// Package coralcrm implements the coral-crm Skill stub (not customer Salesforce).
package coralcrm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const ID port.GatewayID = "coral-crm"

// Gateway is a minimal act/inform CRM stub (create_ticket / status).
type Gateway struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (g *Gateway) ID() port.GatewayID { return ID }

func (g *Gateway) Capabilities() port.Capability {
	return port.Capability{Batch: true}
}

func (g *Gateway) Execute(ctx context.Context, req port.SkillRequest) (port.SkillResult, error) {
	var args map[string]any
	_ = json.Unmarshal(req.Args, &args)
	action := req.Name
	if action == "" {
		action = "status"
	}
	if args != nil {
		if v, ok := args["action"].(string); ok && v != "" {
			action = v
		}
	}
	payload := map[string]any{
		"session_id": string(req.SessionID),
		"tenant_id":  req.TenantID,
		"action":     action,
		"args":       args,
	}
	body, _ := json.Marshal(payload)
	if g.BaseURL == "" {
		out, _ := json.Marshal(map[string]any{"ok": true, "stub": true, "action": action})
		return port.SkillResult{OK: true, Output: out}, nil
	}
	client := g.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/skills/crm", bytes.NewReader(body))
	if err != nil {
		return port.SkillResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return port.SkillResult{}, &port.GatewayError{Code: port.CodeUnavailable, Message: err.Error(), Retryable: true, Cause: err}
	}
	defer resp.Body.Close()
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	out, _ := json.Marshal(map[string]any{"ok": ok, "status": resp.StatusCode, "action": action})
	return port.SkillResult{OK: ok, Output: out}, nil
}

// Register adds coral-crm to the registry.
func Register(reg port.Registry, g *Gateway) error {
	if g == nil {
		g = &Gateway{}
	}
	return reg.Register(port.Registration{
		ID:           ID,
		Port:         port.PortSkill,
		Capabilities: g.Capabilities(),
		Instance:     g,
		Probe: func(ctx context.Context) port.Health {
			return port.Health{Healthy: true, LastOK: time.Now()}
		},
	})
}
