package port

import "context"

type TranslateRequest struct {
	SessionID SessionID
	Text      string
	Source    string
	Target    string
}

type Translate interface {
	ID() GatewayID
	Capabilities() Capability
	Translate(ctx context.Context, req TranslateRequest) (string, error)
}
