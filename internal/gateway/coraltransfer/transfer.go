// Package coraltransfer implements the coral-transfer Skill stub.
package coraltransfer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const ID port.GatewayID = "coral-transfer"

// Gateway POSTs warm-transfer payload to Coral base URL when set; else stub success.
type Gateway struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (g *Gateway) ID() port.GatewayID { return ID }

func (g *Gateway) Capabilities() port.Capability {
	return port.Capability{Batch: true}
}

// Payload matches RULES_AND_SKILLS.md §2.
type Payload struct {
	SessionID        string          `json:"session_id"`
	TenantID         string          `json:"tenant_id"`
	Caller           json.RawMessage `json:"caller,omitempty"`
	Intent           string          `json:"intent"`
	Summary          string          `json:"summary"`
	TranscriptExcerpt string         `json:"transcript_excerpt"`
	RecordingRef     string          `json:"recording_ref"`
	ProfileID        string          `json:"profile_id"`
	ProfileVersion   int             `json:"version"`
	EscalationReason string          `json:"escalation_reason"`
}

func (g *Gateway) Execute(ctx context.Context, req port.SkillRequest) (port.SkillResult, error) {
	var args map[string]any
	_ = json.Unmarshal(req.Args, &args)
	p := Payload{
		SessionID: string(req.SessionID),
		TenantID:  req.TenantID,
	}
	if args != nil {
		if v, ok := args["intent"].(string); ok {
			p.Intent = v
		}
		if v, ok := args["summary"].(string); ok {
			p.Summary = v
		}
		if v, ok := args["transcript_excerpt"].(string); ok {
			p.TranscriptExcerpt = v
		}
		if v, ok := args["recording_ref"].(string); ok {
			p.RecordingRef = v
		}
		if v, ok := args["profile_id"].(string); ok {
			p.ProfileID = v
		}
		if v, ok := args["escalation_reason"].(string); ok {
			p.EscalationReason = v
		}
		if v, ok := args["caller"]; ok {
			p.Caller, _ = json.Marshal(v)
		}
		if v, ok := args["version"].(float64); ok {
			p.ProfileVersion = int(v)
		}
	}
	body, _ := json.Marshal(p)
	if g.BaseURL == "" {
		out, _ := json.Marshal(map[string]any{"ok": true, "stub": true, "payload": json.RawMessage(body)})
		return port.SkillResult{OK: true, Output: out}, nil
	}
	client := g.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/skills/warm-transfer", bytes.NewReader(body))
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
	out, _ := json.Marshal(map[string]any{"ok": ok, "status": resp.StatusCode})
	return port.SkillResult{OK: ok, Output: out}, nil
}

// Register adds coral-transfer to the registry.
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
