// Package gbxmockserver runs an in-process GBX XML-RPC server intended for
// testing Trackmania controller code. It speaks the framed XML-RPC wire
// format that a real dedicated server speaks, records every method call,
// answers requests with static or dynamic responders, and pushes
// server-initiated callbacks to connected clients.
//
// Reference docs for the protocol simulated by this package:
//   - Methods:   https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Methods
//   - Callbacks: https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Callbacks
//   - Modescript: https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Modescript-documentation
//   - Connect:   https://wiki.trackmania.io/en/dedicated-server/XML-RPC/HowToConnect
package gbxmockserver

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// MethodCall is a recorded inbound request. Params carry the decoded
// argument values; use the Value accessors (AsString, AsInt, …) to read
// them.
type MethodCall struct {
	Method    string    `json:"method"`
	Params    []Value   `json:"-"`
	Timestamp time.Time `json:"timestamp"`
}

// Responder produces a response value for a single inbound call. Returning
// a non-nil error causes the mock to emit an XML-RPC fault — implement
// `FaultCode() int` on the error to control the fault code (default -1).
type Responder func(call MethodCall) (any, error)

// FaultError is an error type that carries an explicit XML-RPC fault code.
type FaultError struct {
	Code    int
	Message string
}

func (e *FaultError) Error() string  { return e.Message }
func (e *FaultError) FaultCode() int { return e.Code }

// Identity overrides the values reported by GetVersion / GetStatus /
// GetSystemInfo. Empty fields fall back to TM2020-shaped defaults.
//
// Field names mirror the wire-level struct keys documented at
// https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Methods so a
// consumer who wants to assert against client-side parsed structs can
// match them by inspection.
type Identity struct {
	Name           string
	TitleId        string
	Version        string
	Build          string
	ApiVersion     string
	StatusCode     int
	StatusName     string
	PublishedIp    string
	ServerLogin    string
	ServerPlayerId int
}

// Config holds initialization options for New.
type Config struct {
	Host     string
	Port     int  // Set to 0 to auto-assign an available port.
	AutoPort bool // If true, automatically find next available port.
	Identity Identity
}

// MockServer is a single mock GBX XML-RPC endpoint.
type MockServer struct {
	host     string
	port     int
	identity Identity

	listener net.Listener

	mu          sync.RWMutex
	responses   map[string]Value
	responders  map[string]Responder
	methodCalls []MethodCall

	connsMu  sync.Mutex
	conns    map[*managedConn]struct{}
	cbHandle uint32
}

// New creates a MockServer. Call Start to begin accepting connections.
func New(config Config) *MockServer {
	if config.Host == "" {
		config.Host = "127.0.0.1"
	}

	port := config.Port
	if config.AutoPort || config.Port == 0 {
		if available, err := findAvailablePort(config.Port, config.Host); err == nil {
			port = available
		} else {
			port = 5000
		}
	}
	if port == 0 {
		port = 5000
	}

	return &MockServer{
		host:       config.Host,
		port:       port,
		identity:   applyIdentityDefaults(config.Identity),
		responses:  make(map[string]Value),
		responders: make(map[string]Responder),
	}
}

// NewWithAutoPort is a convenience wrapper around New with AutoPort=true.
func NewWithAutoPort(host string) *MockServer {
	if host == "" {
		host = "127.0.0.1"
	}
	return New(Config{Host: host, AutoPort: true})
}

func applyIdentityDefaults(id Identity) Identity {
	if id.Name == "" {
		id.Name = "MockServer"
	}
	if id.TitleId == "" {
		id.TitleId = "Trackmania"
	}
	if id.Version == "" {
		id.Version = "3.3.0"
	}
	if id.Build == "" {
		id.Build = "2024-01-01"
	}
	if id.ApiVersion == "" {
		id.ApiVersion = "2023-04-24"
	}
	if id.StatusCode == 0 {
		id.StatusCode = 4
	}
	if id.StatusName == "" {
		id.StatusName = "Running"
	}
	if id.PublishedIp == "" {
		id.PublishedIp = "127.0.0.1"
	}
	if id.ServerLogin == "" {
		id.ServerLogin = "MockServer"
	}
	return id
}

// SetResponse stores a static response for the named method. The value
// flows through ValueOf, so any Go type the value tree understands (string,
// int, bool, float, time.Time, []byte, slice, map, or tagged struct) is
// accepted.
func (s *MockServer) SetResponse(method string, response any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.responses[method] = ValueOf(response)
}

// SetResponseFunc registers a dynamic responder for the named method.
// Responders take precedence over static responses set via SetResponse.
// Returning a non-nil error emits an XML-RPC fault.
func (s *MockServer) SetResponseFunc(method string, fn Responder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.responders[method] = fn
}

// GetMethodCalls returns a copy of every recorded inbound call.
func (s *MockServer) GetMethodCalls() []MethodCall {
	s.mu.RLock()
	defer s.mu.RUnlock()
	calls := make([]MethodCall, len(s.methodCalls))
	copy(calls, s.methodCalls)
	return calls
}

// GetMethodCallsFor returns the recorded calls for one method name.
func (s *MockServer) GetMethodCallsFor(method string) []MethodCall {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []MethodCall
	for _, c := range s.methodCalls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// WasMethodCalled reports whether the named method was ever invoked.
func (s *MockServer) WasMethodCalled(method string) bool {
	return s.GetCallCount(method) > 0
}

// GetCallCount returns the number of times method was invoked.
func (s *MockServer) GetCallCount(method string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, c := range s.methodCalls {
		if c.Method == method {
			count++
		}
	}
	return count
}

// ClearMethodCalls discards the recorded call history.
func (s *MockServer) ClearMethodCalls() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.methodCalls = nil
}

// Start binds the listener and begins accepting connections.
func (s *MockServer) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.host, s.port))
	if err != nil {
		return err
	}
	s.listener = listener
	go s.acceptConnections()
	return nil
}

// Stop closes the listener. Outstanding client connections will drop on
// their next read.
func (s *MockServer) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Port returns the bound port.
func (s *MockServer) Port() int { return s.port }

// Address returns "host:port".
func (s *MockServer) Address() string { return fmt.Sprintf("%s:%d", s.host, s.port) }

func (s *MockServer) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConnection(conn)
	}
}

func (s *MockServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	managed := &managedConn{conn: conn}
	unregister := s.register(managed)
	defer unregister()

	if err := writeHandshake(managed); err != nil {
		return
	}

	for {
		var lengthBytes [4]byte
		if _, err := io.ReadFull(conn, lengthBytes[:]); err != nil {
			return
		}
		length := uint32(lengthBytes[0]) | uint32(lengthBytes[1])<<8 |
			uint32(lengthBytes[2])<<16 | uint32(lengthBytes[3])<<24

		var handleBytes [4]byte
		if _, err := io.ReadFull(conn, handleBytes[:]); err != nil {
			return
		}
		handle := uint32(handleBytes[0]) | uint32(handleBytes[1])<<8 |
			uint32(handleBytes[2])<<16 | uint32(handleBytes[3])<<24

		body := make([]byte, length)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}

		response := s.processRequest(body)
		if err := writeFrame(managed, handle, response); err != nil {
			return
		}
	}
}

func (s *MockServer) processRequest(xmlData []byte) []byte {
	method, params, err := parseMethodCall(xmlData)
	if err != nil {
		return encodeFault(-1, err.Error())
	}

	call := MethodCall{
		Method:    method,
		Params:    params,
		Timestamp: time.Now(),
	}

	s.mu.Lock()
	s.methodCalls = append(s.methodCalls, call)
	responder, hasResponder := s.responders[method]
	stored, hasStored := s.responses[method]
	s.mu.Unlock()

	if hasResponder {
		result, rerr := responder(call)
		if rerr != nil {
			code := -1
			if fc, ok := rerr.(interface{ FaultCode() int }); ok {
				code = fc.FaultCode()
			}
			return encodeFault(code, rerr.Error())
		}
		return encodeMethodResponse(ValueOf(result))
	}

	if hasStored {
		return encodeMethodResponse(stored)
	}

	return encodeMethodResponse(s.defaultResponse(method))
}

func (s *MockServer) defaultResponse(method string) Value {
	switch method {
	case "GetVersion":
		return StructValue(map[string]Value{
			"Name":       StringValue(s.identity.Name),
			"TitleId":    StringValue(s.identity.TitleId),
			"Version":    StringValue(s.identity.Version),
			"Build":      StringValue(s.identity.Build),
			"ApiVersion": StringValue(s.identity.ApiVersion),
		})
	case "GetStatus":
		return StructValue(map[string]Value{
			"Code": IntValue(int64(s.identity.StatusCode)),
			"Name": StringValue(s.identity.StatusName),
		})
	case "GetSystemInfo":
		return StructValue(map[string]Value{
			"PublishedIp":            StringValue(s.identity.PublishedIp),
			"Port":                   IntValue(int64(s.port)),
			"P2PPort":                IntValue(0),
			"TitleId":                StringValue(s.identity.TitleId),
			"ServerLogin":            StringValue(s.identity.ServerLogin),
			"ServerPlayerId":         IntValue(int64(s.identity.ServerPlayerId)),
			"ConnectionDownloadRate": IntValue(0),
			"ConnectionUploadRate":   IntValue(0),
			"IsServer":               BoolValue(true),
			"IsDedicated":            BoolValue(true),
		})
	default:
		return BoolValue(true)
	}
}

func findAvailablePort(startPort int, host string) (int, error) {
	if startPort == 0 {
		startPort = 5000
	}
	for port := startPort; port < startPort+1000; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", startPort, startPort+999)
}
