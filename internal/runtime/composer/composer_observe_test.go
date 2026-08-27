package composer_test

import (
	"context"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/bus"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/composer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestComposer_AuditOnTurn(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	obs := &observe.Observer{Repo: mem, Meta: observe.SessionMeta{
		SessionID: "sess-audit", TenantID: "t1",
		ProfileID: "talk", ProfileVersion: 1, Clock: "live",
	}}
	talk, err := composer.NewTalk(talkProfile(), reg, bus.New(), session.NewMemory(), "live", "sess-audit")
	if err != nil {
		t.Fatal(err)
	}
	talk.Obs = obs
	if err := talk.InjectFinal(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	audits, err := mem.ListAuditEvents(context.Background(), "sess-audit")
	if err != nil {
		t.Fatal(err)
	}
	foundTurn := false
	for _, a := range audits {
		if a.EventType == store.AuditTurnCompleted {
			foundTurn = true
		}
	}
	if !foundTurn {
		t.Fatalf("want turn.completed audit, got %#v", audits)
	}
	ams, _ := mem.ListAnalyticsEvents(context.Background(), "sess-audit")
	foundMetric := false
	for _, m := range ams {
		if m.Metric == store.MetricTurnCompleted {
			foundMetric = true
		}
	}
	if !foundMetric {
		t.Fatalf("want turn_completed metric, got %#v", ams)
	}
}
