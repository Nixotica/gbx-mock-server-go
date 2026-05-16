package gbxmockserver

import (
	"fmt"
	"testing"
	"time"

	"github.com/MRegterschot/GbxRemoteGo/gbxclient"
)

func TestMockServer(t *testing.T) {
	// Create and start mock server
	mock := New(Config{
		Host: "127.0.0.1",
		Port: 5001, // Use different port for tests
	})

	// Set a custom response
	mock.SetResponse("GetVersion", map[string]interface{}{
		"Name":    "TestServer",
		"Version": "1.0.0-test",
	})

	if err := mock.Start(); err != nil {
		t.Fatal("Failed to start mock server:", err)
	}
	defer mock.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client and connect
	client := gbxclient.NewGbxClient("127.0.0.1", 5001, gbxclient.Options{})
	if err := client.Connect(); err != nil {
		t.Fatal("Failed to connect to mock server:", err)
	}
	defer client.Disconnect()

	// Test authentication flow
	if err := client.SetApiVersion("2023-04-24"); err != nil {
		t.Fatal("Failed to set API version:", err)
	}

	if err := client.EnableCallbacks(true); err != nil {
		t.Fatal("Failed to enable callbacks:", err)
	}

	if err := client.Authenticate("test", "test"); err != nil {
		t.Fatal("Failed to authenticate:", err)
	}

	// Test custom response
	version, err := client.GetVersion()
	if err != nil {
		t.Fatal("Failed to get version:", err)
	}

	if version.Name != "TestServer" {
		t.Errorf("Expected Name='TestServer', got '%s'", version.Name)
	}

	if version.Version != "1.0.0-test" {
		t.Errorf("Expected Version='1.0.0-test', got '%s'", version.Version)
	}

	// Test default response
	status, err := client.GetStatus()
	if err != nil {
		t.Fatal("Failed to get status:", err)
	}

	if status.Code != 4 {
		t.Errorf("Expected status code 4, got %d", status.Code)
	}

	// Test script event methods
	err = client.TriggerModeScriptEvent("Trackmania.Pause.SetActive", "true")
	if err != nil {
		t.Fatal("Failed to trigger script event:", err)
	}

	err = client.TriggerModeScriptEventArray("Trackmania.ForceEndRound", []string{})
	if err != nil {
		t.Fatal("Failed to trigger script event array:", err)
	}
}

func TestMockServerCustomResponses(t *testing.T) {
	mock := New(Config{
		Host: "127.0.0.1",
		Port: 5002,
	})

	// Test different response types
	mock.SetResponse("TestString", "Hello World")
	mock.SetResponse("TestInt", 42)
	mock.SetResponse("TestBool", true)
	mock.SetResponse("TestStruct", map[string]interface{}{
		"Field1": "value1",
		"Field2": 123,
		"Field3": false,
	})

	if err := mock.Start(); err != nil {
		t.Fatal("Failed to start mock server:", err)
	}
	defer mock.Stop()

	time.Sleep(100 * time.Millisecond)

	client := gbxclient.NewGbxClient("127.0.0.1", 5002, gbxclient.Options{})
	if err := client.Connect(); err != nil {
		t.Fatal("Failed to connect:", err)
	}
	defer client.Disconnect()

	// Basic auth
	client.SetApiVersion("2023-04-24")
	client.EnableCallbacks(true)
	client.Authenticate("test", "test")

	// These would be tested if we had generic Call method exposed
	// For now, we test that the mock server handles unknown methods gracefully
	err := client.TriggerModeScriptEvent("TestString", "")
	if err != nil {
		t.Fatal("Mock server should handle unknown methods gracefully:", err)
	}
}

func TestMockServerAutoPort(t *testing.T) {
	// Test creating a server with auto-port
	mock1 := NewWithAutoPort("127.0.0.1")

	if err := mock1.Start(); err != nil {
		t.Fatal("Failed to start first mock server:", err)
	}
	defer mock1.Stop()

	port1 := mock1.Port()
	t.Logf("First server got port: %d", port1)

	// Create a second server, should get a different port
	mock2 := NewWithAutoPort("127.0.0.1")

	if err := mock2.Start(); err != nil {
		t.Fatal("Failed to start second mock server:", err)
	}
	defer mock2.Stop()

	port2 := mock2.Port()
	t.Logf("Second server got port: %d", port2)

	if port1 == port2 {
		t.Error("Both servers got the same port, auto-port assignment failed")
	}

	// Test that both servers work
	client1 := gbxclient.NewGbxClient("127.0.0.1", port1, gbxclient.Options{})
	if err := client1.Connect(); err != nil {
		t.Fatal("Failed to connect to first server:", err)
	}
	defer client1.Disconnect()

	client2 := gbxclient.NewGbxClient("127.0.0.1", port2, gbxclient.Options{})
	if err := client2.Connect(); err != nil {
		t.Fatal("Failed to connect to second server:", err)
	}
	defer client2.Disconnect()

	// Test both connections work
	if err := client1.SetApiVersion("2023-04-24"); err != nil {
		t.Fatal("Failed to set API version on first server:", err)
	}

	if err := client2.SetApiVersion("2023-04-24"); err != nil {
		t.Fatal("Failed to set API version on second server:", err)
	}
}

func TestMockServerConfigAutoPort(t *testing.T) {
	// Test using Config with AutoPort = true
	mock := New(Config{
		Host:     "127.0.0.1",
		AutoPort: true,
	})

	if err := mock.Start(); err != nil {
		t.Fatal("Failed to start mock server with AutoPort:", err)
	}
	defer mock.Stop()

	port := mock.Port()
	address := mock.Address()

	t.Logf("Server with AutoPort got port: %d, address: %s", port, address)

	if port == 0 {
		t.Error("AutoPort failed to assign a port")
	}

	expectedAddress := fmt.Sprintf("127.0.0.1:%d", port)
	if address != expectedAddress {
		t.Errorf("Expected address %s, got %s", expectedAddress, address)
	}
}

func TestMockServerMethodTracking(t *testing.T) {
	mock := New(Config{
		Host: "127.0.0.1",
		Port: 5003,
	})

	if err := mock.Start(); err != nil {
		t.Fatal("Failed to start mock server:", err)
	}
	defer mock.Stop()

	time.Sleep(100 * time.Millisecond)

	client := gbxclient.NewGbxClient("127.0.0.1", 5003, gbxclient.Options{})
	if err := client.Connect(); err != nil {
		t.Fatal("Failed to connect:", err)
	}
	defer client.Disconnect()

	// Basic auth
	client.SetApiVersion("2023-04-24")
	client.EnableCallbacks(true)
	client.Authenticate("test", "test")

	// Clear any existing calls from authentication
	mock.ClearMethodCalls()

	// Test method tracking
	client.TriggerModeScriptEvent("Trackmania.Pause.SetActive", "true")
	client.RestartMap()
	client.GetVersion()

	// Verify method calls were tracked
	if !mock.WasMethodCalled("TriggerModeScriptEvent") {
		t.Error("Expected TriggerModeScriptEvent to be called")
	}

	if !mock.WasMethodCalled("RestartMap") {
		t.Error("Expected RestartMap to be called")
	}

	if !mock.WasMethodCalled("GetVersion") {
		t.Error("Expected GetVersion to be called")
	}

	// Test call count
	if count := mock.GetCallCount("TriggerModeScriptEvent"); count != 1 {
		t.Errorf("Expected TriggerModeScriptEvent to be called 1 time, got %d", count)
	}

	// Test getting calls for specific method
	calls := mock.GetMethodCallsFor("TriggerModeScriptEvent")
	if len(calls) != 1 {
		t.Errorf("Expected 1 call for TriggerModeScriptEvent, got %d", len(calls))
	}

	if len(calls) > 0 {
		call := calls[0]
		if call.Method != "TriggerModeScriptEvent" {
			t.Errorf("Expected method name 'TriggerModeScriptEvent', got '%s'", call.Method)
		}

		if len(call.Params) != 2 {
			t.Errorf("Expected 2 parameters, got %d", len(call.Params))
		}

		if len(call.Params) >= 2 {
			if s, _ := call.Params[0].AsString(); s != "Trackmania.Pause.SetActive" {
				t.Errorf("Expected first param 'Trackmania.Pause.SetActive', got '%v'", call.Params[0])
			}
			if s, _ := call.Params[1].AsString(); s != "true" {
				t.Errorf("Expected second param 'true', got '%v'", call.Params[1])
			}
		}
	}

	// Test getting all calls
	allCalls := mock.GetMethodCalls()
	if len(allCalls) != 3 {
		t.Errorf("Expected 3 total calls, got %d", len(allCalls))
	}

	// Test clearing calls
	mock.ClearMethodCalls()
	if len(mock.GetMethodCalls()) != 0 {
		t.Error("Expected no calls after clearing")
	}
}
