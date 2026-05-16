package gbxmockserver

import (
	"errors"
	"testing"
	"time"

	gbxevents "github.com/MRegterschot/GbxRemoteGo/events"
	"github.com/MRegterschot/GbxRemoteGo/gbxclient"

	"github.com/Nixotica/gbx-mock-server-go/events"
)

const eventWaitTimeout = 2 * time.Second

func startMockAndConnect(t *testing.T) (*MockServer, *gbxclient.GbxClient) {
	t.Helper()

	mock := New(Config{Host: "127.0.0.1", AutoPort: true})
	if err := mock.Start(); err != nil {
		t.Fatalf("start mock: %v", err)
	}
	t.Cleanup(func() { _ = mock.Stop() })

	client := gbxclient.NewGbxClient("127.0.0.1", mock.Port(), gbxclient.Options{})
	if err := client.Connect(); err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect() })

	if err := client.SetApiVersion("2023-04-24"); err != nil {
		t.Fatalf("SetApiVersion: %v", err)
	}
	if err := client.EnableCallbacks(true); err != nil {
		t.Fatalf("EnableCallbacks: %v", err)
	}
	return mock, client
}

// TestSetResponseFunc exercises a parameter-aware responder: the same method
// produces different return values depending on the inbound arguments.
func TestSetResponseFunc(t *testing.T) {
	mock, client := startMockAndConnect(t)

	mock.SetResponseFunc("EchoLookup", func(call MethodCall) (any, error) {
		key, _ := call.Params[0].AsString()
		switch key {
		case "ping":
			return "pong", nil
		case "answer":
			return 42, nil
		default:
			return nil, errors.New("unknown key: " + key)
		}
	})

	pong, err := client.Call("EchoLookup", "ping")
	if err != nil {
		t.Fatalf("EchoLookup(ping): %v", err)
	}
	if s, _ := pong.(string); s != "pong" {
		t.Errorf("got %#v want \"pong\"", pong)
	}

	answer, err := client.Call("EchoLookup", "answer")
	if err != nil {
		t.Fatalf("EchoLookup(answer): %v", err)
	}
	if n, _ := answer.(int); n != 42 {
		t.Errorf("got %#v want int(42)", answer)
	}

	if _, err := client.Call("EchoLookup", "missing"); err == nil {
		t.Errorf("EchoLookup(missing) should fault, got nil err")
	}
}

// TestResponderFaultCode verifies a Responder that returns *FaultError
// preserves its code through the wire fault.
func TestResponderFaultCode(t *testing.T) {
	mock, client := startMockAndConnect(t)

	mock.SetResponseFunc("MustFail", func(MethodCall) (any, error) {
		return nil, &FaultError{Code: -503, Message: "service unavailable"}
	})

	_, err := client.Call("MustFail")
	if err == nil {
		t.Fatal("expected fault, got nil")
	}
	if err.Error() == "" {
		t.Fatalf("empty fault message: %v", err)
	}
}

// TestPushCallback drives a real gbxclient end-to-end and verifies that a
// PushCallback delivers the expected event to the client's typed handler.
func TestPushCallback(t *testing.T) {
	mock, client := startMockAndConnect(t)

	received := make(chan gbxevents.PlayerConnectEventArgs, 1)
	client.OnPlayerConnect = append(client.OnPlayerConnect, gbxclient.GbxCallbackStruct[gbxevents.PlayerConnectEventArgs]{
		Key: "test",
		Call: func(args gbxevents.PlayerConnectEventArgs) {
			received <- args
		},
	})

	waitForClient(t, mock, 1)

	delivered := mock.PushCallback(events.PlayerConnect, "test-login", true)
	if delivered != 1 {
		t.Fatalf("PushCallback delivered=%d want 1", delivered)
	}

	select {
	case got := <-received:
		if got.Login != "test-login" {
			t.Errorf("Login got %q", got.Login)
		}
		if !got.IsSpectator {
			t.Errorf("IsSpectator got false")
		}
	case <-time.After(eventWaitTimeout):
		t.Fatal("timed out waiting for PlayerConnect callback")
	}
}

// TestPushModeScriptCallback drives a WayPoint script callback through the
// real client and asserts the typed struct arrives populated. The client
// dispatches IsEndRace=true to OnPlayerFinish.
func TestPushModeScriptCallback(t *testing.T) {
	mock, client := startMockAndConnect(t)

	received := make(chan gbxevents.PlayerWayPointEventArgs, 1)
	client.OnPlayerFinish = append(client.OnPlayerFinish, gbxclient.GbxCallbackStruct[gbxevents.PlayerWayPointEventArgs]{
		Key: "test",
		Call: func(args gbxevents.PlayerWayPointEventArgs) {
			received <- args
		},
	})

	waitForClient(t, mock, 1)

	payload := gbxevents.PlayerWayPointEventArgs{
		Login:     "racer",
		AccountId: "acc-1",
		RaceTime:  45123,
		IsEndRace: true,
	}
	if delivered := mock.PushModeScriptCallback(events.MSWayPoint, payload); delivered != 1 {
		t.Fatalf("PushModeScriptCallback delivered=%d want 1", delivered)
	}

	select {
	case got := <-received:
		if got.AccountId != "acc-1" || got.RaceTime != 45123 || !got.IsEndRace {
			t.Errorf("got %#v", got)
		}
	case <-time.After(eventWaitTimeout):
		t.Fatal("timed out waiting for WayPoint callback")
	}
}

// TestPushCallbackMultipleClients exercises the connection registry: two
// clients each get their own copy of a single PushCallback.
func TestPushCallbackMultipleClients(t *testing.T) {
	mock := New(Config{Host: "127.0.0.1", AutoPort: true})
	if err := mock.Start(); err != nil {
		t.Fatalf("start mock: %v", err)
	}
	defer mock.Stop()

	makeClient := func() (*gbxclient.GbxClient, chan gbxevents.PlayerConnectEventArgs) {
		c := gbxclient.NewGbxClient("127.0.0.1", mock.Port(), gbxclient.Options{})
		if err := c.Connect(); err != nil {
			t.Fatalf("connect: %v", err)
		}
		t.Cleanup(func() { _ = c.Disconnect() })
		_ = c.SetApiVersion("2023-04-24")
		_ = c.EnableCallbacks(true)
		ch := make(chan gbxevents.PlayerConnectEventArgs, 1)
		c.OnPlayerConnect = append(c.OnPlayerConnect, gbxclient.GbxCallbackStruct[gbxevents.PlayerConnectEventArgs]{
			Call: func(args gbxevents.PlayerConnectEventArgs) { ch <- args },
		})
		return c, ch
	}

	_, ch1 := makeClient()
	_, ch2 := makeClient()
	waitForClient(t, mock, 2)

	if delivered := mock.PushCallback(events.PlayerConnect, "shared", false); delivered != 2 {
		t.Fatalf("delivered=%d want 2", delivered)
	}

	for i, ch := range []chan gbxevents.PlayerConnectEventArgs{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Login != "shared" {
				t.Errorf("client %d login got %q", i, got.Login)
			}
		case <-time.After(eventWaitTimeout):
			t.Fatalf("client %d timed out", i)
		}
	}
}

// TestIdentityOverride confirms that Config.Identity threads through to
// GetVersion / GetSystemInfo defaults so consumers can present any title.
func TestIdentityOverride(t *testing.T) {
	mock := New(Config{
		Host:     "127.0.0.1",
		AutoPort: true,
		Identity: Identity{
			Name:    "ECM-Mock",
			TitleId: "TMNext",
		},
	})
	if err := mock.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer mock.Stop()

	client := gbxclient.NewGbxClient("127.0.0.1", mock.Port(), gbxclient.Options{})
	if err := client.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()
	_ = client.SetApiVersion("2023-04-24")
	_ = client.EnableCallbacks(true)

	v, err := client.GetVersion()
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if v.Name != "ECM-Mock" || v.TitleId != "TMNext" {
		t.Errorf("identity not threaded: %#v", v)
	}

	si, err := client.GetSystemInfo()
	if err != nil {
		t.Fatalf("GetSystemInfo: %v", err)
	}
	if si.TitleId != "TMNext" {
		t.Errorf("system info TitleId got %q, want TMNext", si.TitleId)
	}
}

// waitForClient blocks until the mock has registered want connections or
// the timeout fires.
func waitForClient(t *testing.T, mock *MockServer, want int) {
	t.Helper()
	deadline := time.Now().Add(eventWaitTimeout)
	for time.Now().Before(deadline) {
		if mock.ConnectedClients() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("only %d/%d clients connected after %s", mock.ConnectedClients(), want, eventWaitTimeout)
}
