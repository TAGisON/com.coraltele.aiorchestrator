// Package sarvamllm implements the sarvam-llm Think gateway (OpenAI-compatible chat).
package sarvamllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvam"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const ID port.GatewayID = "sarvam-llm"

// DefaultModel is recorded for lab docs / tests (Sarvam recommended conversational model).
const DefaultModel = sarvam.DefaultChatModel // sarvam-105b-conversations

// Gateway is Think over Sarvam /v1/chat/completions.
type Gateway struct {
	Cfg        sarvam.Config
	HTTPClient *http.Client
	Model      string // empty → DefaultModel
}

func New(cfg sarvam.Config) *Gateway {
	return &Gateway{Cfg: cfg, HTTPClient: sarvam.DefaultHTTPClient(), Model: DefaultModel}
}

func (g *Gateway) ID() port.GatewayID { return ID }

func (g *Gateway) Capabilities() port.Capability {
	return port.Capability{Streaming: true, Batch: true, Cancel: true}
}

func (g *Gateway) client() *http.Client {
	if g.HTTPClient != nil {
		return g.HTTPClient
	}
	return sarvam.DefaultHTTPClient()
}

func (g *Gateway) model() string {
	if g.Model != "" {
		return g.Model
	}
	return DefaultModel
}

func (g *Gateway) Complete(ctx context.Context, req port.ThinkRequest) (port.ThinkResult, error) {
	if !g.Cfg.Configured() {
		return port.ThinkResult{}, &port.GatewayError{Code: port.CodeAuth, Message: "sarvam api key missing"}
	}
	body := g.buildBody(req, false)
	raw, err := g.postChat(ctx, body)
	if err != nil {
		return port.ThinkResult{}, err
	}
	return parseChatResult(raw)
}

func (g *Gateway) CompleteStream(ctx context.Context, req port.ThinkRequest) (port.ThinkStream, error) {
	// Prefer non-SSE path: complete then emit one token chunk (satisfies channel contract).
	// SSE is available when Stream=true on the wire; lab uses buffered Complete for reliability.
	res, err := g.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan string, 1)
	ch <- res.Text
	close(ch)
	st := &thinkStream{tokens: ch, res: res}
	return st, nil
}

func (g *Gateway) buildBody(req port.ThinkRequest, stream bool) map[string]any {
	msgs := make([]map[string]string, 0, len(req.Messages)+len(req.GroundingChunks)+1)
	if len(req.GroundingChunks) > 0 {
		msgs = append(msgs, map[string]string{
			"role":    "system",
			"content": "Grounding:\n" + strings.Join(req.GroundingChunks, "\n"),
		})
	}
	for _, m := range req.Messages {
		role := m.Role
		if role == "" {
			role = "user"
		}
		msgs = append(msgs, map[string]string{"role": role, "content": m.Content})
	}
	body := map[string]any{
		"model":    g.model(),
		"messages": msgs,
		"stream":   stream,
	}
	// Skill descriptors → OpenAI tools when present; SkillProposal left nil if unused (plan).
	if len(req.Skills) > 0 {
		tools := make([]map[string]any, 0, len(req.Skills))
		for _, sk := range req.Skills {
			fn := map[string]any{"name": sk.Name, "description": sk.Description}
			if len(sk.InputSchema) > 0 {
				var schema any
				if json.Unmarshal(sk.InputSchema, &schema) == nil {
					fn["parameters"] = schema
				}
			}
			tools = append(tools, map[string]any{"type": "function", "function": fn})
		}
		body["tools"] = tools
	}
	return body
}

func (g *Gateway) postChat(ctx context.Context, body map[string]any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.Cfg.ChatURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(sarvam.HeaderAPIKey, g.Cfg.APIKey)
	httpReq.Header.Set("Authorization", "Bearer "+g.Cfg.APIKey)

	resp, err := g.client().Do(httpReq)
	if err != nil {
		return nil, sarvam.MapDialError(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, sarvam.MapHTTPStatus(resp.StatusCode, string(raw))
	}
	return raw, nil
}

func parseChatResult(raw []byte) (port.ThinkResult, error) {
	var out struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return port.ThinkResult{}, &port.GatewayError{Code: port.CodeInternal, Message: "bad chat json", Cause: err}
	}
	if len(out.Choices) == 0 {
		return port.ThinkResult{}, &port.GatewayError{Code: port.CodeInternal, Message: "empty chat choices"}
	}
	msg := out.Choices[0].Message
	res := port.ThinkResult{Text: msg.Content}
	if len(msg.ToolCalls) > 0 {
		tc := msg.ToolCalls[0]
		res.SkillProposal = &port.SkillProposal{
			Name: tc.Function.Name,
			Args: []byte(tc.Function.Arguments),
		}
	}
	return res, nil
}

type thinkStream struct {
	tokens chan string
	res    port.ThinkResult
	cancel atomic.Bool
}

func (t *thinkStream) Tokens() <-chan string { return t.tokens }
func (t *thinkStream) Result(ctx context.Context) (port.ThinkResult, error) {
	return t.res, nil
}
func (t *thinkStream) Cancel(ctx context.Context) error {
	t.cancel.Store(true)
	return nil
}

// Register adds sarvam-llm when configured.
func Register(reg port.Registry, g *Gateway) error {
	if g == nil {
		cfg, err := sarvam.LoadConfig()
		if err != nil {
			return err
		}
		g = New(cfg)
	}
	if !g.Cfg.Configured() {
		return fmt.Errorf("sarvam-llm: api key not configured")
	}
	return reg.Register(port.Registration{
		ID:           ID,
		Port:         port.PortThink,
		Capabilities: g.Capabilities(),
		Instance:     g,
		Probe: func(ctx context.Context) port.Health {
			if !g.Cfg.Configured() {
				return port.Health{Healthy: false, LastError: "api key missing"}
			}
			return port.Health{Healthy: true, LastOK: time.Now()}
		},
	})
}
