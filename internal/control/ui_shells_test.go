package control_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
	"github.com/coraltele/com.coraltele.aiorchestrator/web"
)

func testServerWithUI(t *testing.T) *control.Server {
	t.Helper()
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	return control.NewWithRuntime(mem, reg, nil, control.Config{}, web.UIFS)
}

func TestConsoleShells_Static(t *testing.T) {
	srv := testServerWithUI(t)
	cases := []struct {
		path    string
		want    int
		contain string
	}{
		{"/", http.StatusOK, "/admin/"},
		{"/admin/", http.StatusOK, "Engines"},
		{"/admin/engines.html", http.StatusOK, "Tenant engines"},
		{"/admin/profiles.html", http.StatusOK, "Publish version"},
		{"/admin/settings.html", http.StatusOK, "Fallback prompts"},
		{"/admin/bindings.html", http.StatusOK, "Inline FAQ"},
		{"/admin/flows.html", http.StatusOK, "Draft document"},
		{"/admin/flows-builder.html", http.StatusOK, "Graph builder"},
		{"/admin/flow-builder.js", http.StatusOK, "FlowBuilder"},
		{"/admin/pin.html", http.StatusOK, "Answer pins"},
		{"/admin/admin.js", http.StatusOK, "AdminUI"},
		{"/supervisor/", http.StatusOK, "Supervisor"},
		{"/chat/", http.StatusOK, "Chat"},
		{"/shared/api.js", http.StatusOK, "getAnswerPins"},
		{"/shared/styles.css", http.StatusOK, "--accent"},
		{"/admin", http.StatusFound, ""},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Fatalf("%s status %d want %d body %s", tc.path, rr.Code, tc.want, rr.Body.String())
		}
		if tc.contain != "" && !strings.Contains(rr.Body.String(), tc.contain) {
			t.Fatalf("%s missing %q", tc.path, tc.contain)
		}
	}
}
