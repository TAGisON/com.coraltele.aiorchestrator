package control_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/deskskills"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/ingest"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// deskAPI drives the Control API exactly as the consoles do.
type deskAPI struct {
	t   *testing.T
	srv *control.Server
	gw  *deskskills.Gateway
}

func newDeskAPI(t *testing.T) *deskAPI {
	t.Helper()
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	gw, err := deskskills.Register(reg, nil)
	if err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	if err := ingest.Register(reg, ingest.New(mem)); err != nil {
		t.Fatal(err)
	}
	seedFakeTenantEngines(t, mem)
	srv := control.NewWithRuntime(mem, reg, nil, control.Config{OwnerInstance: "desk-api"}, nil)
	return &deskAPI{t: t, srv: srv, gw: gw}
}

func (a *deskAPI) do(method, path string, body any) (int, map[string]any) {
	a.t.Helper()
	var reader *bytes.Buffer
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = bytes.NewBuffer(raw)
	} else {
		reader = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rr, req)
	out := map[string]any{}
	if rr.Body.Len() > 0 {
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
	}
	return rr.Code, out
}

func (a *deskAPI) mustDo(method, path string, body any, wantStatus int) map[string]any {
	a.t.Helper()
	code, out := a.do(method, path, body)
	if code != wantStatus {
		raw, _ := json.Marshal(out)
		a.t.Fatalf("%s %s = %d, want %d: %s", method, path, code, wantStatus, raw)
	}
	return out
}

// installAndPublish runs the operator's happy path: preset → checklist → publish.
func (a *deskAPI) installAndPublish() map[string]any {
	a.t.Helper()
	a.mustDo(http.MethodPost, "/v1/desk-presets/coral-tfn", map[string]any{"tenant_id": "default"}, http.StatusCreated)

	check := a.mustDo(http.MethodGet, "/v1/desks/coral-tfn/checklist", nil, http.StatusOK)
	list, _ := check["checklist"].(map[string]any)
	if list == nil || list["publishable"] != true {
		raw, _ := json.Marshal(check)
		a.t.Fatalf("preset should be publishable: %s", raw)
	}
	return a.mustDo(http.MethodPost, "/v1/desks/coral-tfn/publish",
		map[string]any{"published_by": "admin-console"}, http.StatusCreated)
}

func TestDeskInstallPublishAndVersion(t *testing.T) {
	a := newDeskAPI(t)
	pub := a.installAndPublish()

	if pub["desk_version"] != float64(1) || pub["profile_version"] != float64(1) {
		raw, _ := json.Marshal(pub)
		t.Fatalf("first publish should be version 1/1: %s", raw)
	}
	if hash, _ := pub["content_hash"].(string); len(hash) != 64 {
		t.Fatalf("publish should record a content hash, got %v", pub["content_hash"])
	}

	got := a.mustDo(http.MethodGet, "/v1/desks/coral-tfn", nil, http.StatusOK)
	deskRec, _ := got["desk"].(map[string]any)
	if deskRec["status"] != "published" || deskRec["current_version"] != float64(1) {
		raw, _ := json.Marshal(deskRec)
		t.Fatalf("desk record after publish: %s", raw)
	}

	versions := a.mustDo(http.MethodGet, "/v1/desks/coral-tfn/versions", nil, http.StatusOK)
	if items, _ := versions["versions"].([]any); len(items) != 1 {
		t.Fatalf("expected one published version, got %v", versions["versions"])
	}

	listed := a.mustDo(http.MethodGet, "/v1/desks", nil, http.StatusOK)
	if items, _ := listed["desks"].([]any); len(items) != 1 {
		t.Fatalf("desk list should show the Coral desk, got %v", listed["desks"])
	}
}

func TestDeskPublishRejectsIncompleteDesk(t *testing.T) {
	a := newDeskAPI(t)
	a.mustDo(http.MethodPost, "/v1/desks", map[string]any{
		"id": "empty-desk", "name": "", "tenant_id": "default",
	}, http.StatusCreated)

	out := a.mustDo(http.MethodPost, "/v1/desks/empty-desk/publish", map[string]any{}, http.StatusUnprocessableEntity)
	errObj, _ := out["error"].(map[string]any)
	if errObj == nil || !strings.Contains(errObj["message"].(string), "checklist") {
		raw, _ := json.Marshal(out)
		t.Fatalf("publish of an empty desk should fail the checklist: %s", raw)
	}
}

// text call: the console's manual test surface, end to end over HTTP.
func (a *deskAPI) startCall(language string) string {
	a.t.Helper()
	out := a.mustDo(http.MethodPost, "/v1/desk-calls", map[string]any{
		"desk_id": "coral-tfn", "language": language, "ani": "919812345678",
	}, http.StatusCreated)
	id, _ := out["id"].(string)
	if id == "" {
		a.t.Fatal("desk call should return an id")
	}
	return id
}

func (a *deskAPI) turn(callID, text string) map[string]any {
	a.t.Helper()
	return a.mustDo(http.MethodPost, "/v1/desk-calls/"+callID+"/turn", map[string]any{"text": text}, http.StatusOK)
}

func lastReply(snapshot map[string]any) string {
	turns, _ := snapshot["turns"].([]any)
	if len(turns) == 0 {
		return ""
	}
	last, _ := turns[len(turns)-1].(map[string]any)
	reply, _ := last["assistant"].(string)
	return reply
}

func TestDeskTextCallEnglishEndToEnd(t *testing.T) {
	a := newDeskAPI(t)
	a.installAndPublish()
	call := a.startCall("en-IN")

	a.turn(call, "my ip phone is not working and I want to register a complaint")
	a.turn(call, "yes powered on")
	a.turn(call, "network is fine")
	a.turn(call, "calls keep dropping")
	a.turn(call, "multiple users")
	a.turn(call, "register a complaint")
	a.turn(call, "Ramesh Kumar")
	a.turn(call, "ramesh@coral.com")
	a.turn(call, "yes")
	created := a.turn(call, "yes all correct")

	attrs, _ := created["attributes"].(map[string]any)
	ticket, _ := attrs["ticket_id"].(string)
	if !strings.HasPrefix(ticket, "CTL-") {
		raw, _ := json.Marshal(attrs)
		t.Fatalf("expected a backend ticket id: %s", raw)
	}
	if !strings.Contains(lastReply(created), ticket) {
		t.Fatalf("desk should speak the ticket id, said: %s", lastReply(created))
	}

	// Confidential attributes are masked in the console payload (§11).
	if email, _ := attrs["customer_email"].(string); email != "r***@coral.com" {
		t.Errorf("email should be masked in console reads, got %q", email)
	}
	if ani, _ := attrs["ani"].(string); !strings.HasPrefix(ani, "*") {
		t.Errorf("ANI should be masked in console reads, got %q", ani)
	}

	final := a.turn(call, "no thank you")
	if final["ended"] != true || final["disposition"] != "ticket_created" {
		raw, _ := json.Marshal(final)
		t.Fatalf("call should end as ticket_created: %s", raw)
	}

	ledger := a.mustDo(http.MethodGet, "/v1/desk-skills/ledger", nil, http.StatusOK)
	tickets, _ := ledger["tickets"].([]any)
	emails, _ := ledger["emails"].([]any)
	if len(tickets) != 1 || len(emails) != 1 {
		raw, _ := json.Marshal(ledger)
		t.Fatalf("ledger should hold one ticket and one email: %s", raw)
	}
}

func TestDeskTextCallHindiEndToEnd(t *testing.T) {
	a := newDeskAPI(t)
	a.installAndPublish()
	call := a.startCall("")

	first := a.turn(call, "मुझे शिकायत दर्ज करानी है")
	if first["language"] != "hi-IN" {
		t.Fatalf("Hindi opener should switch the call language, got %v", first["language"])
	}
	a.turn(call, "मीडिया गेटवे")
	a.turn(call, "हाँ चालू है")
	a.turn(call, "नहीं जुड़ रहा")
	a.turn(call, "दोनों")
	a.turn(call, "सभी कॉल्स")
	a.turn(call, "शिकायत दर्ज करें")
	a.turn(call, "सुरेश शर्मा")
	emailTurn := a.turn(call, "suresh@coral.com")
	if emailTurn["language"] != "hi-IN" {
		t.Fatalf("an email address must not flip the call to English, got %v", emailTurn["language"])
	}
	a.turn(call, "हाँ सही है")
	created := a.turn(call, "हाँ बिल्कुल सही")

	attrs, _ := created["attributes"].(map[string]any)
	if ticket, _ := attrs["ticket_id"].(string); !strings.HasPrefix(ticket, "CTL-") {
		raw, _ := json.Marshal(created)
		t.Fatalf("Hindi journey should also create a ticket: %s", raw)
	}
	if !strings.Contains(lastReply(created), "successfully registered") {
		t.Fatalf("confirmation should speak ticket registration (EN-authored), said: %s", lastReply(created))
	}
}

func TestDeskTextCallSilenceAndLanguageSwitch(t *testing.T) {
	a := newDeskAPI(t)
	a.installAndPublish()
	call := a.startCall("en-IN")

	first := a.mustDo(http.MethodPost, "/v1/desk-calls/"+call+"/silence", nil, http.StatusOK)
	if !strings.Contains(lastReply(first), "still on the call") {
		t.Fatalf("first silence should nudge, said: %s", lastReply(first))
	}
	a.mustDo(http.MethodPost, "/v1/desk-calls/"+call+"/silence", nil, http.StatusOK)
	third := a.mustDo(http.MethodPost, "/v1/desk-calls/"+call+"/silence", nil, http.StatusOK)
	if third["ended"] != true || third["disposition"] != "abandoned_silence" {
		raw, _ := json.Marshal(third)
		t.Fatalf("third silence should end the call: %s", raw)
	}

	// A fresh call proves the operator language override.
	call2 := a.startCall("en-IN")
	switched := a.mustDo(http.MethodPost, "/v1/desk-calls/"+call2+"/language",
		map[string]any{"language": "hi-IN"}, http.StatusOK)
	if switched["language"] != "hi-IN" {
		t.Fatalf("operator language switch failed: %v", switched["language"])
	}
	out := a.turn(call2, "technical support")
	if !strings.Contains(lastReply(out), "Coral Telecom") {
		t.Fatalf("desk should keep talking after the switch, said: %s", lastReply(out))
	}
}

// §12: injected backend failure must not produce a ticket id anywhere.
func TestDeskTicketFailureThroughAPI(t *testing.T) {
	a := newDeskAPI(t)
	a.installAndPublish()
	a.mustDo(http.MethodPost, "/v1/desk-skills/failures",
		map[string]any{"skill": "create_service_complaint", "status": "fail"}, http.StatusOK)

	call := a.startCall("en-IN")
	a.turn(call, "I want to register a complaint about my ip phone")
	a.turn(call, "yes powered on")
	a.turn(call, "network is fine")
	a.turn(call, "calls keep dropping")
	a.turn(call, "multiple users")
	a.turn(call, "register a complaint")
	a.turn(call, "Ramesh Kumar")
	a.turn(call, "ramesh@coral.com")
	a.turn(call, "yes")
	failed := a.turn(call, "yes all correct")

	reply := lastReply(failed)
	if strings.Contains(reply, "CTL-") {
		t.Fatalf("desk invented a ticket id after a backend failure: %s", reply)
	}
	if !strings.Contains(reply, "unable to register the complaint") {
		t.Fatalf("desk should admit the failure, said: %s", reply)
	}
	attrs, _ := failed["attributes"].(map[string]any)
	if tid, _ := attrs["ticket_id"].(string); tid != "" {
		t.Fatalf("ticket_id must stay empty, got %q", tid)
	}

	ledger := a.mustDo(http.MethodGet, "/v1/desk-skills/ledger", nil, http.StatusOK)
	if tickets, _ := ledger["tickets"].([]any); len(tickets) != 0 {
		t.Fatalf("no ticket should reach the backend, got %v", tickets)
	}

	a.mustDo(http.MethodPost, "/v1/desk-skills/reset", nil, http.StatusOK)
	after := a.mustDo(http.MethodGet, "/v1/desk-skills/ledger", nil, http.StatusOK)
	failures, _ := after["failures"].(map[string]any)
	if len(failures) != 0 {
		t.Fatalf("reset should clear injected failures, got %v", failures)
	}
}

func TestDeskCatalogDescribesGUIVocabulary(t *testing.T) {
	a := newDeskAPI(t)
	cat := a.mustDo(http.MethodGet, "/v1/desk-catalog", nil, http.StatusOK)
	for _, key := range []string{"step_types", "validations", "branches", "repairs",
		"dispositions", "purposes", "products", "skills", "languages", "prompt_slots"} {
		items, _ := cat[key].([]any)
		if len(items) == 0 {
			t.Errorf("catalog.%s should be populated for the Configurator GUI", key)
		}
	}
	langs, _ := cat["languages"].([]any)
	if len(langs) != 2 || langs[0] != "en-IN" || langs[1] != "hi-IN" {
		t.Errorf("catalog must offer Indian multilingual defaults, got %v", langs)
	}
}

func TestTenantPropertiesRoundTrip(t *testing.T) {
	a := newDeskAPI(t)
	got := a.mustDo(http.MethodGet, "/v1/tenant/properties", nil, http.StatusOK)
	props, _ := got["properties"].(map[string]any)
	if props["max_concurrent_sessions"] != "20" {
		t.Fatalf("default concurrency should be present, got %v", props)
	}

	saved := a.mustDo(http.MethodPut, "/v1/tenant/properties", map[string]any{
		"properties": map[string]string{"max_concurrent_sessions": "5", "retention_transcript_days": "30"},
	}, http.StatusOK)
	props, _ = saved["properties"].(map[string]any)
	if props["max_concurrent_sessions"] != "5" || props["retention_transcript_days"] != "30" {
		t.Fatalf("properties did not persist: %v", props)
	}
}

func TestComplianceErasureLifecycle(t *testing.T) {
	a := newDeskAPI(t)
	created := a.mustDo(http.MethodPost, "/v1/compliance/erasure", map[string]any{
		"subject_ref": "session:abc123", "scope": "all", "requested_by": "dpo",
	}, http.StatusCreated)
	id, _ := created["id"].(string)
	if id == "" || created["status"] != "queued" {
		raw, _ := json.Marshal(created)
		t.Fatalf("erasure request should be queued: %s", raw)
	}

	listed := a.mustDo(http.MethodGet, "/v1/compliance/erasure", nil, http.StatusOK)
	if items, _ := listed["erasure_requests"].([]any); len(items) != 1 {
		t.Fatalf("erasure request should be listed, got %v", listed)
	}

	done := a.mustDo(http.MethodPost, "/v1/compliance/erasure/"+id+"/complete", nil, http.StatusOK)
	if done["status"] != "completed" {
		t.Fatalf("erasure should complete, got %v", done["status"])
	}
}

func TestDispositionOverrideRejectsUnknownCode(t *testing.T) {
	a := newDeskAPI(t)
	code, _ := a.do(http.MethodPatch, "/v1/sessions/s-1/disposition", map[string]any{"final": "made_up"})
	if code != http.StatusBadRequest {
		t.Fatalf("unknown disposition should be rejected, got %d", code)
	}
	a.mustDo(http.MethodPatch, "/v1/sessions/s-1/disposition",
		map[string]any{"final": "ticket_created", "actor": "supervisor"}, http.StatusOK)
}
