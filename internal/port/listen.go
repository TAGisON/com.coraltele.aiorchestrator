package port

import "context"

type ListenRequest struct {
	SessionID    SessionID
	SampleRate   SampleRateHz
	LanguageHint string
	Clock        string // "live" | "playback"
}

type ListenPartial struct {
	Text       string
	Confidence float32
	Language   string
}

type ListenFinal struct {
	Text       string
	Confidence float32
	Language   string
	StartMS    int64
	EndMS      int64
}

// ListenStream is opened for live (streaming) recognition.
type ListenStream interface {
	WritePCM(ctx context.Context, frame PCMFrame) error
	Partials() <-chan ListenPartial
	Finals() <-chan ListenFinal
	Close(ctx context.Context) error
}

type Listen interface {
	ID() GatewayID
	Capabilities() Capability
	OpenStream(ctx context.Context, req ListenRequest) (ListenStream, error)
	RecognizeBatch(ctx context.Context, req ListenRequest, pcm []byte) (ListenFinal, error)
}
