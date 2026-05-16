// Package events declares the typed constants used as callback identifiers
// when pushing events from the mock server to connected clients.
//
// Names are sourced from the official Trackmania dedicated-server wiki:
// https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Callbacks
//
// and cross-checked against the strings recognized by the GbxRemoteGo
// client library so that pushed callbacks dispatch correctly on the
// consumer side.
package events

// CallbackName is the wire identifier of a protocol-level callback —
// anything that a Trackmania dedicated server sends directly to its
// connected clients, distinct from script callbacks wrapped inside
// ModeScriptCallbackArray.
//
// Reference: https://wiki.trackmania.io/en/dedicated-server/XML-RPC/Callbacks
type CallbackName string

// Wire signatures next to each constant come from the wiki entry linked
// above. They describe the params a Trackmania server emits; pass those
// same params (in order) to MockServer.PushCallback.

// BeginMap fires when the server starts loading a new map.
//
//	ManiaPlanet.BeginMap(SMapInfo Map)
const BeginMap CallbackName = "ManiaPlanet.BeginMap"

// BeginMatch fires when a match starts on the current map.
//
//	ManiaPlanet.BeginMatch()
const BeginMatch CallbackName = "ManiaPlanet.BeginMatch"

// BillUpdated reports a state change for an in-game planet transaction.
//
//	ManiaPlanet.BillUpdated(int BillId, int State, string StateName, int TransactionId)
//
// Note: not handled by GbxRemoteGo's built-in callback dispatcher, but
// documented on the wiki and emitted by real servers.
const BillUpdated CallbackName = "ManiaPlanet.BillUpdated"

// Echo carries an arbitrary echo message broadcast to all connected XML-RPC
// clients — useful for cross-controller signalling.
//
//	ManiaPlanet.Echo(string Internal, string Public)
const Echo CallbackName = "ManiaPlanet.Echo"

// EndMap fires when the current map ends.
//
//	ManiaPlanet.EndMap(SMapInfo Map)
const EndMap CallbackName = "ManiaPlanet.EndMap"

// EndMatch fires when a match ends, carrying the final rankings.
//
//	ManiaPlanet.EndMatch(SPlayerRanking Rankings[], int WinnerTeam)
const EndMatch CallbackName = "ManiaPlanet.EndMatch"

// MapListModified reports a change in the server's rotation.
//
//	ManiaPlanet.MapListModified(int CurMapIndex, int NextMapIndex, bool IsListModified)
const MapListModified CallbackName = "ManiaPlanet.MapListModified"

// ModeScriptCallback delivers a script callback with two string args. Modern
// scripts almost always use ModeScriptCallbackArray instead; this exists
// for compatibility.
//
//	ManiaPlanet.ModeScriptCallback(string Param1, string Param2)
const ModeScriptCallback CallbackName = "ManiaPlanet.ModeScriptCallback"

// ModeScriptCallbackArray delivers a script callback with a name plus a
// variable-length array of string params. PushModeScriptCallback wraps
// values into this frame automatically.
//
//	ManiaPlanet.ModeScriptCallbackArray(string Param1, string Params[])
const ModeScriptCallbackArray CallbackName = "ManiaPlanet.ModeScriptCallbackArray"

// PlayerAlliesChanged fires when a player changes their ally list.
//
//	ManiaPlanet.PlayerAlliesChanged(string Login)
const PlayerAlliesChanged CallbackName = "ManiaPlanet.PlayerAlliesChanged"

// PlayerChat fires for each in-game chat message.
//
//	ManiaPlanet.PlayerChat(int PlayerUid, string Login, string Text, bool IsRegistredCmd, int Options)
const PlayerChat CallbackName = "ManiaPlanet.PlayerChat"

// PlayerConnect fires when a player joins the server.
//
//	ManiaPlanet.PlayerConnect(string Login, bool IsSpectator)
const PlayerConnect CallbackName = "ManiaPlanet.PlayerConnect"

// PlayerDisconnect fires when a player leaves the server.
//
//	ManiaPlanet.PlayerDisconnect(string Login, string DisconnectionReason)
const PlayerDisconnect CallbackName = "ManiaPlanet.PlayerDisconnect"

// PlayerInfoChanged fires when a player's spectator/team/etc. info changes.
//
//	ManiaPlanet.PlayerInfoChanged(SPlayerInfo PlayerInfo)
const PlayerInfoChanged CallbackName = "ManiaPlanet.PlayerInfoChanged"

// PlayerManialinkPageAnswer fires when a player submits a manialink form.
//
//	ManiaPlanet.PlayerManialinkPageAnswer(int PlayerUid, string Login, string Answer, SEntryVal Entries[])
const PlayerManialinkPageAnswer CallbackName = "ManiaPlanet.PlayerManialinkPageAnswer"

// ServerStart fires when the server boots.
//
//	ManiaPlanet.ServerStart()
const ServerStart CallbackName = "ManiaPlanet.ServerStart"

// ServerStop fires just before the server shuts down.
//
//	ManiaPlanet.ServerStop()
const ServerStop CallbackName = "ManiaPlanet.ServerStop"

// StatusChanged fires when the server's lifecycle status transitions.
//
//	ManiaPlanet.StatusChanged(int StatusCode, string StatusName)
const StatusChanged CallbackName = "ManiaPlanet.StatusChanged"

// TunnelDataReceived delivers a raw P2P tunnel payload from a player.
//
//	ManiaPlanet.TunnelDataReceived(int PlayerUid, string Login, base64 Data)
const TunnelDataReceived CallbackName = "ManiaPlanet.TunnelDataReceived"

// VoteUpdated fires when an in-game callvote changes state.
//
//	ManiaPlanet.VoteUpdated(string StateName, string Login, string CmdName, string CmdParam)
const VoteUpdated CallbackName = "ManiaPlanet.VoteUpdated"

// PlayerIncoherence reports a desync / cheat-suspect on a player.
//
//	Trackmania.PlayerIncoherence(int PlayerUid, string Login)
//
// Note: the wiki spells this "TrackMania.PlayerIncoherence" (capital M)
// alongside other legacy TMUF callbacks, but modern dedicated servers
// (and GbxRemoteGo's dispatcher) emit/expect lowercase "Trackmania." here.
// PushCallback uses the modern spelling.
const PlayerIncoherence CallbackName = "Trackmania.PlayerIncoherence"

// LegacyPlayerCheckpoint is the TMUF-era checkpoint callback. Modern
// Trackmania (2020) replaces this with the Trackmania.Event.WayPoint script
// callback; see ModeScriptCallbackName.MSWayPoint.
//
//	TrackMania.PlayerCheckpoint(int PlayerUid, string Login, int TimeOrScore, int CurLap, int CheckpointIndex)
const LegacyPlayerCheckpoint CallbackName = "TrackMania.PlayerCheckpoint"

// LegacyPlayerFinish is the TMUF-era race-finish callback. Modern
// Trackmania (2020) replaces this with Trackmania.Event.WayPoint with
// IsEndRace=true.
//
//	TrackMania.PlayerFinish(int PlayerUid, string Login, int TimeOrScore)
const LegacyPlayerFinish CallbackName = "TrackMania.PlayerFinish"
