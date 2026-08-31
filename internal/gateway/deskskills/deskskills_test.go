package deskskills

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

func exec(t *testing.T, g *Gateway, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s1", Name: name, Args: raw, TenantID: "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{}
	_ = json.Unmarshal(res.Output, &out)
	return out, res.OK
}

func TestNeverInventTicketOnFailure(t *testing.T) {
	g := New()
	g.SetFailure("create_service_complaint", StatusFail)
	out, ok := exec(t, g, "create_service_complaint", map[string]any{
		"name": "Ramesh", "email": "r@c.com", "product": "ip_phone", "problem": "dead",
	})
	if ok {
		t.Fatal("forced fail must not be ok")
	}
	if id, _ := out["ticket_id"].(string); strings.TrimSpace(id) != "" {
		t.Fatalf("must never invent ticket id, got %q", id)
	}
}

func TestEmailFailKeepsTicket(t *testing.T) {
	g := New()
	created, ok := exec(t, g, "create_service_complaint", map[string]any{
		"name": "Ramesh", "email": "r@c.com", "product": "ip_phone", "problem": "dead",
	})
	if !ok {
		t.Fatal("create should succeed")
	}
	ticket, _ := created["ticket_id"].(string)
	if ticket == "" {
		t.Fatal("ticket id required on success")
	}
	g.SetFailure("send_complaint_email", StatusFail)
	mail, mailOK := exec(t, g, "send_complaint_email", map[string]any{
		"email": "r@c.com", "ticket_id": ticket,
	})
	if mailOK {
		t.Fatal("forced email fail")
	}
	if sent, _ := mail["email_sent"].(bool); sent {
		t.Fatal("email_sent must be false")
	}
	if len(g.Ledger()["tickets"].([]Ticket)) != 1 {
		t.Fatal("ticket must remain after email fail")
	}
}

func TestKnowledgeHasIndianProducts(t *testing.T) {
	g := New()
	out, ok := exec(t, g, "search_knowledge", map[string]any{"product": "ip_phone"})
	if !ok {
		t.Fatal("knowledge should hit")
	}
	ans, _ := out["kb_answer"].(string)
	if !strings.Contains(ans, "IP Phone") && !strings.Contains(ans, "SIP") {
		t.Fatalf("expected IP Phone blurb, got %q", ans)
	}
}
