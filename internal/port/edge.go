package port

import "context"

type FeederMeta struct {
	PeerID    string
	PeerRate  SampleRateHz
	SessionID SessionID
}

type FeederEvent struct {
	Kind string // "dtmf" | "stop" | "error"
	Data string
}

type Feeder interface {
	ID() GatewayID
	Frames() <-chan PCMFrame
	Events() <-chan FeederEvent
	Close(ctx context.Context) error
}

type Sink interface {
	ID() GatewayID
	WritePCM(ctx context.Context, frame PCMFrame) error
	Flush(ctx context.Context) error
	WaitMark(ctx context.Context) error
	Close(ctx context.Context) error
}

// TransferRequest is a blind transfer of the caller leg to a dialplan extension.
// Dialplan/Context default to "XML"/"calltransfer" at the edge when empty.
type TransferRequest struct {
	Destination string
	Dialplan    string
	Context     string
	// DrainMs bounds how long the edge holds the transfer waiting for queued
	// playout to finish. Zero means the edge default.
	DrainMs int
	// Reason is recorded for audit; it never reaches the dialplan.
	Reason string
}

// CallControl is implemented by sinks bound to a real telephony leg (currently
// only the FreeSWITCH edge). Sinks that are not a call leg — file, browser, test
// doubles — simply do not implement it, so callers must type-assert.
//
// Both operations are asynchronous requests: the edge plays out whatever is
// still queued first, then acts. They do not wait for the call to end.
type CallControl interface {
	// Hangup ends the call once queued playout drains. cause is a FreeSWITCH
	// cause string ("NORMAL_CLEARING" when empty).
	Hangup(ctx context.Context, cause string) error
	// Transfer hands the caller leg to another extension once playout drains.
	Transfer(ctx context.Context, req TransferRequest) error
}
