package gbxmockserver

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/Nixotica/gbx-mock-server-go/events"
)

// managedConn wraps an accepted connection with the write mutex needed to
// serialize response writes against asynchronous callback pushes.
type managedConn struct {
	conn    net.Conn
	writeMu sync.Mutex
}

// register adds c to the server's connection registry. The returned
// unregister func removes it when the connection ends.
func (s *MockServer) register(c *managedConn) (unregister func()) {
	s.connsMu.Lock()
	if s.conns == nil {
		s.conns = make(map[*managedConn]struct{})
	}
	s.conns[c] = struct{}{}
	s.connsMu.Unlock()
	return func() {
		s.connsMu.Lock()
		delete(s.conns, c)
		s.connsMu.Unlock()
	}
}

// ConnectedClients returns the number of clients currently connected to
// the mock server.
func (s *MockServer) ConnectedClients() int {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	return len(s.conns)
}

// PushCallback delivers a protocol-level callback to every client currently
// connected to the mock server. args are converted via ValueOf, so callers
// can pass native Go values (string, int, bool, time.Time, []byte, slices,
// maps, or structs tagged with `xmlrpc:"..."`).
//
// The arg list must match the wire signature of the callback as documented
// at https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Callbacks —
// each events.CallbackName constant has the signature in its docstring.
//
// Returns the number of clients the frame was attempted on. Write failures
// to a single client do not affect delivery to peers and are not surfaced
// here — tests can verify reception via the client side.
func (s *MockServer) PushCallback(name events.CallbackName, args ...any) int {
	params := make([]Value, len(args))
	for i, a := range args {
		params[i] = ValueOf(a)
	}
	body := encodeMethodCall(string(name), params)
	return s.broadcast(body)
}

// PushModeScriptCallback delivers a mode-script event. It wraps the inner
// name and JSON-encoded payload in a ManiaPlanet.ModeScriptCallbackArray
// frame, matching how a real Trackmania server delivers script callbacks.
//
// The script-event names and their payload shapes are documented at
// https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Modescript-documentation —
// see the events.ModeScriptCallbackName constants for the supported set.
//
// payload may be:
//   - a string — passed through as the single JSON argument
//   - a []string — each element becomes one JSON argument
//   - any other value — marshaled with encoding/json into a single argument
//
// Returns the number of clients the frame was attempted on.
func (s *MockServer) PushModeScriptCallback(name events.ModeScriptCallbackName, payload any) int {
	parts := modeScriptArgs(payload)
	strArgs := make([]Value, len(parts))
	for i, a := range parts {
		strArgs[i] = StringValue(a)
	}
	return s.PushCallback(events.ModeScriptCallbackArray, string(name), ArrayValue(strArgs...))
}

func modeScriptArgs(payload any) []string {
	switch v := payload.(type) {
	case nil:
		return nil
	case string:
		return []string{v}
	case []string:
		return v
	case []byte:
		return []string{string(v)}
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return []string{fmt.Sprintf("%v", v)}
		}
		return []string{string(raw)}
	}
}

// broadcast sends body — already an XML-RPC <methodCall> document — to
// every currently-connected client, framed with a fresh callback handle.
func (s *MockServer) broadcast(body []byte) int {
	s.connsMu.Lock()
	targets := make([]*managedConn, 0, len(s.conns))
	for c := range s.conns {
		targets = append(targets, c)
	}
	s.connsMu.Unlock()

	delivered := 0
	for _, c := range targets {
		handle := s.nextCallbackHandle()
		if writeFrame(c, handle, body) == nil {
			delivered++
		}
	}
	return delivered
}

// nextCallbackHandle returns a strictly-increasing handle in the range
// [1, 0x80000000), wrapping at the top so the high bit is never set —
// gbxclient distinguishes callbacks from responses by that bit.
func (s *MockServer) nextCallbackHandle() uint32 {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	s.cbHandle++
	if s.cbHandle == 0 || s.cbHandle >= 0x80000000 {
		s.cbHandle = 1
	}
	return s.cbHandle
}

// writeFrame writes a single GBX frame (length, handle, payload) under the
// connection's write mutex.
func writeFrame(c *managedConn, handle uint32, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	length := uint32(len(payload))
	header := []byte{
		byte(length), byte(length >> 8), byte(length >> 16), byte(length >> 24),
		byte(handle), byte(handle >> 8), byte(handle >> 16), byte(handle >> 24),
	}
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	_, err := c.conn.Write(payload)
	return err
}

// writeHandshake sends the unframed "GBXRemote 2" banner on connect. The
// banner is length-prefixed but carries no handle, so it bypasses the
// normal frame helper.
func writeHandshake(c *managedConn) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	banner := []byte("GBXRemote 2")
	length := uint32(len(banner))
	header := []byte{
		byte(length), byte(length >> 8), byte(length >> 16), byte(length >> 24),
	}
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	_, err := c.conn.Write(banner)
	return err
}
