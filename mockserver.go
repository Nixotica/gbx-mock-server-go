package gbxmockserver

import (
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// findAvailablePort finds an available port starting from the given port
func findAvailablePort(startPort int, host string) (int, error) {
	if startPort == 0 {
		startPort = 5000 // Default starting port
	}

	for port := startPort; port < startPort+1000; port++ {
		address := fmt.Sprintf("%s:%d", host, port)
		listener, err := net.Listen("tcp", address)
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", startPort, startPort+999)
}

// MethodCall represents a recorded method call
type MethodCall struct {
	Method    string        `json:"method"`
	Params    []interface{} `json:"params"`
	Timestamp time.Time     `json:"timestamp"`
}

// MockServer represents a mock GBX XML-RPC server
type MockServer struct {
	host        string
	port        int
	listener    net.Listener
	responses   map[string]interface{}
	methodCalls []MethodCall
	mutex       sync.RWMutex
	running     bool
}

// Config holds configuration for the mock server
type Config struct {
	Host     string
	Port     int  // Set to 0 to auto-assign an available port
	AutoPort bool // If true, automatically find next available port
}

// New creates a new mock server instance
func New(config Config) *MockServer {
	if config.Host == "" {
		config.Host = "127.0.0.1"
	}

	port := config.Port
	if config.AutoPort || config.Port == 0 {
		// Find an available port
		availablePort, err := findAvailablePort(config.Port, config.Host)
		if err != nil {
			// Fallback to default port if auto-assignment fails
			port = 5000
		} else {
			port = availablePort
		}
	}

	if port == 0 {
		port = 5000
	}

	return &MockServer{
		host:        config.Host,
		port:        port,
		responses:   make(map[string]interface{}),
		methodCalls: make([]MethodCall, 0),
	}
}

// NewWithAutoPort creates a new mock server instance that automatically finds an available port
func NewWithAutoPort(host string) *MockServer {
	if host == "" {
		host = "127.0.0.1"
	}

	return New(Config{
		Host:     host,
		AutoPort: true,
	})
}

// SetResponse sets a custom response for a specific method
func (s *MockServer) SetResponse(method string, response interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.responses[method] = response
}

// GetMethodCalls returns a copy of all recorded method calls
func (s *MockServer) GetMethodCalls() []MethodCall {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a copy to prevent external modification
	calls := make([]MethodCall, len(s.methodCalls))
	copy(calls, s.methodCalls)
	return calls
}

// GetMethodCallsFor returns all calls for a specific method
func (s *MockServer) GetMethodCallsFor(method string) []MethodCall {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var calls []MethodCall
	for _, call := range s.methodCalls {
		if call.Method == method {
			calls = append(calls, call)
		}
	}
	return calls
}

// WasMethodCalled returns true if the specified method was called
func (s *MockServer) WasMethodCalled(method string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, call := range s.methodCalls {
		if call.Method == method {
			return true
		}
	}
	return false
}

// GetCallCount returns the number of times a method was called
func (s *MockServer) GetCallCount(method string) int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	count := 0
	for _, call := range s.methodCalls {
		if call.Method == method {
			count++
		}
	}
	return count
}

// ClearMethodCalls clears the method call history
func (s *MockServer) ClearMethodCalls() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.methodCalls = make([]MethodCall, 0)
}

// Start starts the mock server
func (s *MockServer) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.host, s.port))
	if err != nil {
		return err
	}

	s.listener = listener
	s.running = true

	go s.acceptConnections()
	return nil
}

// Stop stops the mock server
func (s *MockServer) Stop() error {
	s.running = false
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Port returns the port the server is listening on
func (s *MockServer) Port() int {
	return s.port
}

// Address returns the full address (host:port) the server is listening on
func (s *MockServer) Address() string {
	return fmt.Sprintf("%s:%d", s.host, s.port)
}

func (s *MockServer) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				fmt.Printf("Error accepting connection: %v\n", err)
			}
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *MockServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Send initial handshake response when client connects
	handshake := "GBXRemote 2"
	s.sendResponse(conn, []byte(handshake))

	for {
		// Read length header (4 bytes)
		lengthBytes := make([]byte, 4)
		_, err := io.ReadFull(conn, lengthBytes)
		if err != nil {
			return
		}

		// Convert length from little endian (GBX protocol uses little endian)
		length := uint32(lengthBytes[0]) | uint32(lengthBytes[1])<<8 |
			uint32(lengthBytes[2])<<16 | uint32(lengthBytes[3])<<24

		// Read the handle (4 bytes)
		handleBytes := make([]byte, 4)
		_, err = io.ReadFull(conn, handleBytes)
		if err != nil {
			return
		}

		// Read the XML data (length bytes)
		xmlData := make([]byte, length)
		_, err = io.ReadFull(conn, xmlData)
		if err != nil {
			return
		} // Process the request and send response
		response := s.processRequest(xmlData)
		responseWithHandle := append(handleBytes, response...)
		s.sendResponse(conn, responseWithHandle)
	}
}

func (s *MockServer) processRequest(xmlData []byte) []byte {
	// Parse XML-RPC request using a simplified structure
	type XMLRPCValue struct {
		String  string `xml:"string"`
		Int     int    `xml:"i4"`
		Boolean int    `xml:"boolean"`
	}

	type XMLRPCParam struct {
		Value XMLRPCValue `xml:"value"`
	}

	type XMLRPCParams struct {
		Params []XMLRPCParam `xml:"param"`
	}

	type XMLRPCMethodCall struct {
		XMLName    xml.Name     `xml:"methodCall"`
		MethodName string       `xml:"methodName"`
		Params     XMLRPCParams `xml:"params"`
	}

	var methodCall XMLRPCMethodCall
	err := xml.Unmarshal(xmlData, &methodCall)
	if err != nil {
		return s.buildErrorResponse("Parse error: " + err.Error())
	}

	// Extract parameters
	var params []interface{}
	for _, param := range methodCall.Params.Params {
		if param.Value.String != "" {
			params = append(params, param.Value.String)
		} else if param.Value.Int != 0 {
			params = append(params, param.Value.Int)
		} else if param.Value.Boolean != 0 {
			params = append(params, param.Value.Boolean == 1)
		}
	}

	// Record the method call
	s.recordMethodCall(methodCall.MethodName, params)

	// Get method response
	s.mutex.RLock()
	customResponse, hasCustom := s.responses[methodCall.MethodName]
	s.mutex.RUnlock()

	var result interface{}
	if hasCustom {
		result = customResponse
	} else {
		result = s.getDefaultResponse(methodCall.MethodName)
	}

	return s.buildSuccessResponse(result)
}

// recordMethodCall records a method call for tracking purposes
func (s *MockServer) recordMethodCall(method string, params []interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	call := MethodCall{
		Method:    method,
		Params:    params,
		Timestamp: time.Now(),
	}

	s.methodCalls = append(s.methodCalls, call)
}

func (s *MockServer) getDefaultResponse(method string) interface{} {
	switch method {
	case "SetApiVersion":
		return true
	case "EnableCallbacks":
		return true
	case "Authenticate":
		return true
	case "GetVersion":
		return map[string]interface{}{
			"Name":       "MockServer",
			"TitleId":    "Trackmania",
			"Version":    "1.0.0",
			"Build":      "2024-01-01",
			"ApiVersion": "2023-04-24",
		}
	case "GetStatus":
		return map[string]interface{}{
			"Code": 4,
			"Name": "Running",
		}
	case "GetSystemInfo":
		return map[string]interface{}{
			"PublishedIp":    "127.0.0.1",
			"Port":           s.port,
			"P2PPort":        0,
			"Title":          "TmForever",
			"ServerLogin":    "MockServer",
			"ServerPlayerId": 0,
		}
	case "TriggerModeScriptEvent", "TriggerModeScriptEventArray":
		return true
	case "RestartMap", "NextMap":
		return true
	default:
		return true // Default success response
	}
}

func (s *MockServer) buildSuccessResponse(result interface{}) []byte {
	var valueXML string

	switch v := result.(type) {
	case bool:
		if v {
			valueXML = "<value><boolean>1</boolean></value>"
		} else {
			valueXML = "<value><boolean>0</boolean></value>"
		}
	case string:
		valueXML = fmt.Sprintf("<value><string>%s</string></value>", v)
	case int:
		valueXML = fmt.Sprintf("<value><i4>%d</i4></value>", v)
	case map[string]interface{}:
		valueXML = s.buildStructXML(v)
	default:
		valueXML = "<value><string>OK</string></value>"
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
  <params>
    <param>
      %s
    </param>
  </params>
</methodResponse>`, valueXML)

	return []byte(response)
}

func (s *MockServer) buildStructXML(data map[string]interface{}) string {
	var members strings.Builder
	for key, value := range data {
		var valueXML string
		switch v := value.(type) {
		case string:
			valueXML = fmt.Sprintf("<string>%s</string>", v)
		case int:
			valueXML = fmt.Sprintf("<i4>%d</i4>", v)
		case bool:
			if v {
				valueXML = "<boolean>1</boolean>"
			} else {
				valueXML = "<boolean>0</boolean>"
			}
		default:
			valueXML = fmt.Sprintf("<string>%v</string>", v)
		}

		members.WriteString(fmt.Sprintf(`
      <member>
        <name>%s</name>
        <value>%s</value>
      </member>`, key, valueXML))
	}

	return fmt.Sprintf("<value><struct>%s</struct></value>", members.String())
}

func (s *MockServer) buildErrorResponse(message string) []byte {
	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
  <fault>
    <value>
      <struct>
        <member>
          <name>faultCode</name>
          <value><int>-1</int></value>
        </member>
        <member>
          <name>faultString</name>
          <value><string>%s</string></value>
        </member>
      </struct>
    </value>
  </fault>
</methodResponse>`, message)

	return []byte(response)
}

func (s *MockServer) sendResponse(conn net.Conn, response []byte) {
	// Send length header (4 bytes, little endian)
	length := uint32(len(response))
	lengthBytes := []byte{
		byte(length),
		byte(length >> 8),
		byte(length >> 16),
		byte(length >> 24),
	}

	conn.Write(lengthBytes)
	conn.Write(response)
}
