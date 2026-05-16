package events

// ModeScriptCallbackName is the inner identifier of a mode-script event,
// delivered to clients wrapped inside ManiaPlanet.ModeScriptCallbackArray.
//
// Use MockServer.PushModeScriptCallback to dispatch one of these — it does
// the wrapping for you (param 1 is the script-event name, param 2 onwards
// is the JSON-encoded payload). PushCallback with
// ModeScriptCallbackArray lets you control the wire shape directly.
//
// Reference: https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Modescript-documentation
type ModeScriptCallbackName string

// MSWayPoint fires each time a player crosses a checkpoint or finishes the
// race. Payload includes accountid, racetime, checkpointinrace, isendrace,
// speed, and per-checkpoint timings — see PlayerWayPointEventArgs in
// GbxRemoteGo/events for the full shape.
const MSWayPoint ModeScriptCallbackName = "Trackmania.Event.WayPoint"

// MSGiveUp fires when a player respawns to the start without finishing.
// Payload: time, login, accountid.
const MSGiveUp ModeScriptCallbackName = "Trackmania.Event.GiveUp"

// MSStartLine fires when a player crosses the start line. Payload: time,
// login, accountid.
const MSStartLine ModeScriptCallbackName = "Trackmania.Event.StartLine"

// MSScores carries an aggregated score snapshot from the mode script,
// emitted at section boundaries (round, map, match). Payload is a struct
// with section, useteams, winnerteam, players[], teams[].
const MSScores ModeScriptCallbackName = "Trackmania.Scores"

// MSWarmUpStart fires when warmup begins for a section.
const MSWarmUpStart ModeScriptCallbackName = "Trackmania.WarmUp.Start"

// MSWarmUpEnd fires when warmup ends for a section.
const MSWarmUpEnd ModeScriptCallbackName = "Trackmania.WarmUp.End"

// MSWarmUpStartRound fires at the start of each warmup round.
const MSWarmUpStartRound ModeScriptCallbackName = "Trackmania.WarmUp.StartRound"

// MSWarmUpEndRound fires at the end of each warmup round.
const MSWarmUpEndRound ModeScriptCallbackName = "Trackmania.WarmUp.EndRound"

// MSKnockoutElimination fires when one or more players are eliminated in a
// Knockout match. This is emitted by the Knockout mode script itself and
// is not listed on the central Modescript-documentation wiki page —
// confirmed via the GbxRemoteGo dispatcher (gbxclient.go) and live
// Knockout-mode servers.
const MSKnockoutElimination ModeScriptCallbackName = "Trackmania.Knockout.Elimination"
