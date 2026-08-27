package port

import "context"

type SkillRequest struct {
	SessionID SessionID
	Name      string
	Args      []byte
	TenantID  string
}

type SkillResult struct {
	OK     bool
	Output []byte
}

type Skill interface {
	ID() GatewayID
	Capabilities() Capability
	Execute(ctx context.Context, req SkillRequest) (SkillResult, error)
}
