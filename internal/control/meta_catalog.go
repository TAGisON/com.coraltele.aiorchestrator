package control

import (
	"net/http"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/flow"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// handleMetaCatalog implements GET /v1/meta/catalog (U.1 / OD-13-3).
func (s *Server) handleMetaCatalog(w http.ResponseWriter, r *http.Request) {
	_ = s
	_ = r
	g := flow.Catalog()
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_id":              g.SchemaID,
		"node_types":             g.NodeTypes,
		"edge_kinds":             g.EdgeKinds,
		"tools":                  g.Tools,
		"matrix_actions":         g.MatrixActions,
		"binding_kinds":          []string{store.BindingKindKnowledge, store.BindingKindCRM},
		"binding_statuses":       []string{store.BindingStatusActive, store.BindingStatusDisabled},
		"knowledge_modes":        []string{"inline_faq", "http_retrieve"},
		"clocks":                 []string{"live", "playback", "chat"},
		"disposition_final":      append([]string(nil), store.DispositionFinalAllowlist...),
		"disposition_sources": []string{
			store.DispositionSourceLiveTool,
			store.DispositionSourceLiveGraph,
			store.DispositionSourceOpsPatch,
			store.DispositionSourcePostcallWorker,
		},
		"transcript_event_kinds": []string{
			store.EventKindUserFinal,
			store.EventKindBotUtterance,
			store.EventKindEdgeTaken,
			store.EventKindToolLine,
			store.EventKindNote,
		},
		"audit_event_types": store.AuditEventTypes(),
	})
}
