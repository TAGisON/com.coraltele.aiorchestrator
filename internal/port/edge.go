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
