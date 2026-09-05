package control_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/flow"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestMetaCatalog(t *testing.T) {
	srv, _ := testServer(t)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/meta/catalog", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["schema_id"] != flow.SchemaID {
		t.Fatalf("schema_id %v", body["schema_id"])
	}
	nodes, _ := body["node_types"].([]any)
	if len(nodes) != len(flow.NodeTypes()) {
		t.Fatalf("node_types %v", nodes)
	}
	clocks, _ := body["clocks"].([]any)
	foundChat := false
	for _, c := range clocks {
		if c == "chat" {
			foundChat = true
		}
	}
	if !foundChat || len(clocks) < 3 {
		t.Fatalf("clocks %v", clocks)
	}
	finals, _ := body["disposition_final"].([]any)
	if len(finals) != len(store.DispositionFinalAllowlist) {
		t.Fatalf("disposition_final len %d", len(finals))
	}
	audits, _ := body["audit_event_types"].([]any)
	if len(audits) != len(store.AuditEventTypes()) {
		t.Fatalf("audit_event_types len %d want %d", len(audits), len(store.AuditEventTypes()))
	}
	foundGraph := false
	for _, a := range audits {
		if a == store.AuditGraphEdge {
			foundGraph = true
		}
	}
	if !foundGraph {
		t.Fatalf("audit_event_types missing graph.edge: %v", audits)
	}
}
