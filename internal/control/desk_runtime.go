package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/desk"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/thinkpath"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// deskController drives the guided path for one session and persists everything
// the Supervisor console and post-call pipeline need (§6.9, §11).
type deskController struct {
	eng       *desk.Engine
	repo      store.Repository
	sessionID string
	tenantID  string

	mu       sync.Mutex
	lastPack desk.Handoff
}

// NewSkillRunner builds a desk skill runner backed by the gateway registry.
func newSkillRunner(reg port.Registry, doc desk.Doc, sessionID, tenantID string) desk.SkillRunner {
	return func(ctx context.Context, name string, args map[string]any) (map[string]any, bool, error) {
		bind, ok := doc.Skills[name]
		if !ok || !bind.Enabled {
			return map[string]any{"status": "unavailable", "message": "skill not enabled"}, false, nil
		}
		gwID := strings.TrimSpace(bind.Gateway)
		if gwID == "" {
			gwID = desk.DefaultSkillGateway
		}
		rec, found := reg.Get(port.GatewayID(gwID))
		if !found {
			return map[string]any{"status": "unavailable", "message": "gateway " + gwID + " not registered"}, false, nil
		}
		sk, cast := rec.Instance.(port.Skill)
		if !cast {
			return map[string]any{"status": "unavailable", "message": "gateway " + gwID + " is not a skill"}, false, nil
		}
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, false, err
		}
		res, err := sk.Execute(ctx, port.SkillRequest{
			SessionID: port.SessionID(sessionID),
			Name:      name,
			Args:      raw,
			TenantID:  tenantID,
		})
		if err != nil {
			return nil, false, err
		}
		out := map[string]any{}
		if len(res.Output) > 0 {
			_ = json.Unmarshal(res.Output, &out)
		}
		if _, has := out["status"]; !has {
			if res.OK {
				out["status"] = desk.BranchOK
			} else {
				out["status"] = desk.BranchFail
			}
		}
		return out, res.OK, nil
	}
}

// newDeskController builds the controller when the pinned profile carries a desk.
func newDeskController(doc profile.Document, reg port.Registry, repo store.Repository, sessionID, tenantID string, profileVersion int) (*deskController, bool) {
	if len(doc.XDesk) == 0 {
		return nil, false
	}
	var d desk.Doc
	if err := json.Unmarshal(doc.XDesk, &d); err != nil {
		applog.Warn("desk document unreadable", "session", sessionID, "err", err)
		return nil, false
	}
	d.Normalize()
	eng := desk.NewEngine(d, newSkillRunner(reg, d, sessionID, tenantID))
	eng.SetAttribute(desk.AttrProfileVersion, fmt.Sprint(profileVersion))
	return &deskController{eng: eng, repo: repo, sessionID: sessionID, tenantID: tenantID}, true
}

// Engine exposes the FSM for supervisor reads.
func (c *deskController) Engine() *desk.Engine { return c.eng }

// Welcome returns the desk opening line.
func (c *deskController) Welcome() (string, bool) {
	out := c.eng.Welcome()
	c.persist(context.Background(), out)
	return out.Text, out.Text != ""
}

// Turn implements thinkpath.Controller.
func (c *deskController) Turn(ctx context.Context, userText string) (thinkpath.ControllerResult, bool) {
	out := c.eng.Turn(ctx, userText)
	c.persist(ctx, out)
	return thinkpath.ControllerResult{
		Text:        out.Text,
		Tier:        out.Tier,
		SkillName:   out.SkillName,
		SkillOK:     out.SkillOK,
		End:         out.End,
		Disposition: out.Disposition,
		StepID:      out.StepID,
	}, true
}

// Silence advances the no-response ladder.
func (c *deskController) Silence(ctx context.Context) desk.Outcome {
	out := c.eng.Silence(ctx)
	c.persist(ctx, out)
	return out
}

func (c *deskController) persist(ctx context.Context, out desk.Outcome) {
	if c.repo == nil {
		return
	}
	attrs := out.Attributes
	if attrs == nil {
		attrs = c.eng.Attributes()
	}
	rows := make([]store.SessionAttribute, 0, len(attrs))
	for k, v := range attrs {
		rows = append(rows, store.SessionAttribute{Key: k, Value: v, Class: desk.ClassOf(k)})
	}
	if err := c.repo.UpsertSessionAttributes(ctx, c.sessionID, rows); err != nil {
		applog.Warn("desk attributes fail-open", "session", c.sessionID, "err", err)
	}
	for _, call := range out.SkillCalls {
		args, _ := json.Marshal(call.Args)
		output, _ := json.Marshal(call.Output)
		_, err := c.repo.AppendSkillInvocation(ctx, store.SkillInvocation{
			SessionID:      c.sessionID,
			TenantID:       c.tenantID,
			Skill:          call.Name,
			IdempotencyKey: idempotencyKey(c.sessionID, call.Name, args),
			Status:         call.Status,
			Args:           args,
			Output:         output,
			Error:          call.Error,
		})
		if err != nil {
			applog.Warn("desk skill ledger fail-open", "session", c.sessionID, "skill", call.Name, "err", err)
		}
	}
	if out.Transfer != nil {
		c.mu.Lock()
		c.lastPack = *out.Transfer
		c.mu.Unlock()
	}
	if out.End && out.Disposition != "" {
		_, err := c.repo.UpsertSessionDisposition(ctx, store.SessionDisposition{
			SessionID:  c.sessionID,
			Suggestion: out.Disposition,
			TemplateID: "contact-desk-disposition",
			Source:     "desk_engine",
		})
		if err != nil {
			applog.Warn("desk disposition fail-open", "session", c.sessionID, "err", err)
		}
	}
}

// HandoffPack returns the warm-transfer payload for the agent screen pop.
func (c *deskController) HandoffPack() desk.Handoff {
	c.mu.Lock()
	last := c.lastPack
	c.mu.Unlock()
	if last.Target != "" {
		return last
	}
	return c.eng.HandoffPack()
}

func idempotencyKey(sessionID, skill string, args []byte) string {
	h := sha256.New()
	h.Write([]byte(sessionID))
	h.Write([]byte{0})
	h.Write([]byte(skill))
	h.Write([]byte{0})
	h.Write(args)
	return hex.EncodeToString(h.Sum(nil)[:12])
}
