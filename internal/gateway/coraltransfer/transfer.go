// Package coraltransfer implements the coral-transfer Skill stub.
package coraltransfer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const ID port.GatewayID = "coral-transfer"

// TransferFunc performs the telephony transfer for a live session. It is
// injected by control at boot so this gateway never imports the runtime.
type TransferFunc func(ctx context.Context, sessionID string, req port.TransferRequest) error

// Gateway executes a warm transfer: it notifies Coral (when BaseURL is set) and
// then moves the caller's leg to the destination extension.
//
// Notification and the leg move are separate concerns — a Coral outage must not
// strand a caller who was promised a human, so the leg is transferred whether or
// not the notification succeeds, and the notification result is reported in the
// skill output for the audit trail.
type Gateway struct {
	BaseURL    string
	HTTPClient *http.Client

	// Transfer moves the caller leg. When nil the gateway only notifies Coral
	// and reports transferred=false — used by playback//lab clocks that have no
	// telephony leg.
	Transfer TransferFunc

	// DefaultDialplan / DefaultContext feed
	// `uuid_transfer <uuid> <dest> <dialplan> <context>` when the caller does not
	// specify them. Empty values fall back to "XML" / "calltransfer" at the edge.
	DefaultDialplan string
	DefaultContext  string
	// DefaultDestination is used when the skill args carry no destination, e.g. a
	// shared "human agent" queue number.
	DefaultDestination string
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
	dest := g.resolveDestination(args)

	result := map[string]any{
		"destination": dest,
		"transferred": false,
		"notified":    false,
	}

	// 1. Best-effort notification to Coral so the receiving agent has context.
	if g.BaseURL != "" {
		body, _ := json.Marshal(p)
		status, err := g.notify(ctx, body)
		result["notify_status"] = status
		result["notified"] = err == nil && status >= 200 && status < 300
		if err != nil {
			// Log-and-continue: the caller still needs to reach a human.
			result["notify_error"] = err.Error()
		}
	} else {
		result["notify_skipped"] = "coral base url not configured"
	}

	// 2. Move the leg. This is the part the caller actually experiences.
	if g.Transfer == nil {
		result["transfer_skipped"] = "no telephony leg bound to this session"
		out, _ := json.Marshal(result)
		// Not an error: playback and lab clocks legitimately have no leg.
		return port.SkillResult{OK: true, Output: out}, nil
	}
	if dest == "" {
		result["error"] = "no transfer destination configured"
		out, _ := json.Marshal(result)
		return port.SkillResult{OK: false, Output: out}, &port.GatewayError{
			Code:    port.CodeBadRequest,
			Message: "coral-transfer: destination required (skill arg \"destination\" or a configured default)",
		}
	}

	err := g.Transfer(ctx, string(req.SessionID), port.TransferRequest{
		Destination: dest,
		Dialplan:    stringArg(args, "dialplan", g.DefaultDialplan),
		Context:     stringArg(args, "context", g.DefaultContext),
		Reason:      firstNonEmpty(p.EscalationReason, p.Intent, "warm_transfer"),
	})
	if err != nil {
		result["error"] = err.Error()
		out, _ := json.Marshal(result)
		return port.SkillResult{OK: false, Output: out}, &port.GatewayError{
			Code:    port.CodeUnavailable,
			Message: "coral-transfer: " + err.Error(),
			Cause:   err,
		}
	}

	result["transferred"] = true
	out, _ := json.Marshal(result)
	return port.SkillResult{OK: true, Output: out}, nil
}

func (g *Gateway) notify(ctx context.Context, body []byte) (int, error) {
	client := g.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.BaseURL+"/skills/warm-transfer", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// resolveDestination accepts the aliases operators and LLM tool calls actually emit.
func (g *Gateway) resolveDestination(args map[string]any) string {
	for _, key := range []string{"destination", "number", "extension", "dest", "transfer_to"} {
		if v := stringArg(args, key, ""); v != "" {
			return v
		}
	}
	return strings.TrimSpace(g.DefaultDestination)
}

func stringArg(args map[string]any, key, fallback string) string {
	if args != nil {
		if v, ok := args[key].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
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
