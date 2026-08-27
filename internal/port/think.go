package port

import "context"

type ChatMessage struct {
	Role    string
	Content string
}

type SkillDescriptor struct {
	Name        string
	Description string
	InputSchema []byte
}

type ThinkRequest struct {
	SessionID       SessionID
	Messages        []ChatMessage
	GroundingChunks []string
	Skills          []SkillDescriptor
	Stream          bool
}

type SkillProposal struct {
	Name string
	Args []byte
}

type ThinkResult struct {
	Text          string
	SkillProposal *SkillProposal
}

type ThinkStream interface {
	Tokens() <-chan string
	Result(ctx context.Context) (ThinkResult, error)
	Cancel(ctx context.Context) error
}

type Think interface {
	ID() GatewayID
	Capabilities() Capability
	Complete(ctx context.Context, req ThinkRequest) (ThinkResult, error)
	CompleteStream(ctx context.Context, req ThinkRequest) (ThinkStream, error)
}
