package port

import "context"

type SpeakRequest struct {
	SessionID  SessionID
	Text       string
	SSML       bool
	VoiceID    string
	SampleRate SampleRateHz
	Language   string
}

// SpeakStream synthesizes and emits PCM until mark or cancel.
type SpeakStream interface {
	Frames() <-chan PCMFrame
	Done() <-chan struct{}
	Cancel(ctx context.Context) error
}

type Speak interface {
	ID() GatewayID
	Capabilities() Capability
	Speak(ctx context.Context, req SpeakRequest) (SpeakStream, error)
}
