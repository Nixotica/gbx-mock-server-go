# gbx-mock-server-go

A Go library for simulating Trackmania dedicated-server GBX XML-RPC
endpoints in tests. It speaks the framed XML-RPC wire format a real
dedicated server speaks, records every method call, answers requests with
static or dynamic responders, and **pushes server-initiated callbacks** to
connected clients — so you can exercise the full event-driven controller
flow without booting an actual game server.

Wire format and callback / method names follow the official documentation
at <https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Callbacks> and
<https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Modescript-documentation>.

## Install

```bash
go get github.com/Nixotica/gbx-mock-server-go
```

## Quick start

```go
import (
    "testing"

    gbxmockserver "github.com/Nixotica/gbx-mock-server-go"
)

func TestController(t *testing.T) {
    mock := gbxmockserver.NewWithAutoPort("127.0.0.1")
    if err := mock.Start(); err != nil {
        t.Fatalf("start: %v", err)
    }
    defer mock.Stop()

    // Wire your controller against mock.Address(), then drive it.

    if mock.GetCallCount("ChatSendServerMessage") == 0 {
        t.Error("controller never spoke to the chat")
    }
}
```

## Pushing callbacks

A real Trackmania server sends callbacks (PlayerConnect, EndRound, …)
unsolicited over the same TCP connection. The mock can do the same — let
the controller register its handlers as it normally would, then drive
events from the test:

```go
import (
    gbxmockserver "github.com/Nixotica/gbx-mock-server-go"
    "github.com/Nixotica/gbx-mock-server-go/events"
)

mock.PushCallback(events.PlayerConnect, "test-login", false)
mock.PushCallback(events.PlayerChat, 42, "test-login", "/help", false, 0)
```

The exported `events.CallbackName` constants are derived from the wiki
list of callbacks; each constant's godoc carries the wire signature so you
know what to pass as `args`.

### Mode-script callbacks

Script callbacks ride inside `ManiaPlanet.ModeScriptCallbackArray` with the
inner name as the first argument and a JSON payload string as the second.
`PushModeScriptCallback` wraps that for you:

```go
import gbxevents "github.com/MRegterschot/GbxRemoteGo/events"

mock.PushModeScriptCallback(events.MSWayPoint, gbxevents.PlayerWayPointEventArgs{
    AccountId: "racer-1",
    RaceTime:  45123,
    IsEndRace: true,
})
```

`payload` may be a `string`, a `[]string`, or any value that
`encoding/json` can marshal. The library handles the JSON encoding and the
outer wrapping.

## Dynamic responses

`SetResponse` stores a static value. For parameter-aware behaviour or
state-machine responses, use `SetResponseFunc`:

```go
mock.SetResponseFunc("SetScriptName", func(call gbxmockserver.MethodCall) (any, error) {
    name, _ := call.Params[0].AsString()
    if !strings.HasPrefix(name, "Modes/") {
        return nil, &gbxmockserver.FaultError{Code: -32602, Message: "unknown script"}
    }
    return true, nil
})
```

A `nil` error wraps the return value in an XML-RPC `<methodResponse>`. A
non-nil error becomes an XML-RPC fault; implement `FaultCode() int` on the
error (or use the bundled `*FaultError`) to control the fault code.

`SetResponseFunc` takes precedence over `SetResponse` when both are set
for the same method.

## Value model

Inbound parameters and outbound responses flow through a typed `Value`
tree that covers the full XML-RPC value set: string, int, double, bool,
dateTime, base64, nil, array, struct. Use the `AsX` accessors:

```go
call := mock.GetMethodCallsFor("Authenticate")[0]
login, _ := call.Params[0].AsString()
password, _ := call.Params[1].AsString()
```

`SetResponse` accepts native Go values (`string`, `int`, `bool`,
`float64`, `time.Time`, `[]byte`, slices, maps, or structs tagged with
`xmlrpc:"..."`) and converts them via `ValueOf`. Struct field tags mirror
GbxRemoteGo's convention, so event-args structs from that library can be
passed straight through.

## Identity / default responses

The mock has built-in defaults for `GetVersion`, `GetStatus`, and
`GetSystemInfo` so the GBX handshake completes without explicit setup.
Override identity fields via `Config.Identity`:

```go
mock := gbxmockserver.New(gbxmockserver.Config{
    AutoPort: true,
    Identity: gbxmockserver.Identity{
        Name:    "MyMock",
        TitleId: "TMNext",
        Version: "3.3.0",
    },
})
```

Empty fields fall back to TM2020-shaped defaults — no more `TmForever`.

## v0.x compatibility note

This is a 0.x library; breaking changes are made deliberately where they
sharpen the API. The current release replaces `MethodCall.Params
[]interface{}` with `[]Value` — call sites that read params positionally
migrate to `call.Params[i].AsString()` / `AsInt()` / etc.

## License

MIT — see [LICENSE](LICENSE).
