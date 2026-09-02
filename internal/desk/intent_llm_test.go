package desk

import (
	"context"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
)

func TestParseIntentID(t *testing.T) {
	d := Doc{Intents: []Intent{{ID: "sales_enquiry", Active: true}}}
	if got := parseIntentID(`{"intent_id":"sales_enquiry"}`, d); got != "sales_enquiry" {
		t.Fatalf("got %q", got)
	}
}

func TestClassifyIntentBridgeUsesThink(t *testing.T) {
	classifier := func(ctx context.Context, d Doc, text, activeLang string) (string, bool) {
		return "sales_enquiry", true
	}
	d := PresetCoralTFN("default")
	d.Normalize()
	id, score := ClassifyIntentBridge(context.Background(), d, "random tamil phrase", classifier, "hi-IN")
	if id != "sales_enquiry" || score < d.CX.IntentAcceptScore {
		t.Fatalf("id=%q score=%v", id, score)
	}
}

func TestThinkLocaleSynthesizer(t *testing.T) {
	th := &fake.Think{}
	syn := NewThinkLocaleSynthesizer(th, "sess-1")
	out, ok := syn(context.Background(), "How can I help?", "ta-IN")
	if !ok || out == "" {
		t.Fatalf("ok=%v out=%q", ok, out)
	}
}
