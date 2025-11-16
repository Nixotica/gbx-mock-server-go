# gbx-mock-server-go

A Go package for creating mock GBX servers (Trackmania). This is especially useful for testing server controllers written in Go. 

## Installation

```bash
go get github.com/Nixotica/gbx-mock-server-go
```

## Usage

```go
import (
    "testing"
    gbxmockserver "github.com/Nixotica/gbx-mock-server-go"
)

func TestMyServerController(t *testing.T) {
    // Create a mock GBX server with auto-assigned port
    mockServer := gbxmockserver.NewWithAutoPort("127.0.0.1")
    
    // Set custom responses for specific methods
    mockServer.SetResponse("GetPlayerList", []string{"Player1", "Player2"})
    
    // Start the server
    if err := mockServer.Start(); err != nil {
        t.Fatalf("Failed to start mock server: %v", err)
    }
    defer mockServer.Stop()
    
    // Your test code here - connect to mockServer.Address()
    // Example: controller := NewController(mockServer.Address())
    
    // Verify method was called
    if !mockServer.WasMethodCalled("GetPlayerList") {
        t.Error("Expected GetPlayerList to be called")
    }
    
    // Check call count
    callCount := mockServer.GetCallCount("GetPlayerList")
    if callCount != 1 {
        t.Errorf("Expected 1 call, got %d", callCount)
    }
    
    // Clear history for next test
    mockServer.ClearMethodCalls()
}
```

## License

MIT License - see [LICENSE](LICENSE) for details.
