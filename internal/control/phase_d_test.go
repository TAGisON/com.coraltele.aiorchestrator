package control_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/token"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/coralcrm"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/coraltransfer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/ingest"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/clock"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func registerPhaseD(t *testing.T, reg *router.MemRegistry, repo store.Repository) {
	t.Helper()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	if err := ingest.Register(reg, ingest.New(repo)); err != nil {
		t.Fatal(err)
	}
	if err := coraltransfer.Register(reg, nil); err != nil {
		t.Fatal(err)
	}
	if err := coralcrm.Register(reg, nil); err != nil {
		t.Fatal(err)
	}
}

func TestKB_IndexLocalRetrieve(t *testing.T) {
	reg := router.NewMemRegistry()
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	registerPhaseD(t, reg, mem)

	g := ingest.New(mem)
	g.IndexLocal(store.KBChunk{
		DocumentID: "local-1",
		Collection: "faq",
		Text:       "Password reset requires identity verification.",
		Ordinal:    0,
	})
	res, err := g.Retrieve(context.Background(), port.KnowledgeQuery{
		Query: "password reset", Collections: []string{"faq"}, TopK: 3,
	})
	if err != nil || !res.Hit {
		t.Fatalf("retrieve hit=%v err=%v", res.Hit, err)
	}
}

func TestProfile_AcceptsIngestAndCoralSkills(t *testing.T) {
	reg := router.NewMemRegistry()
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	registerPhaseD(t, reg, mem)
	doc := profile.Document{}
	doc.Modes.Think = true
	doc.Routers.Knowledge.Providers = []string{"ingest-default"}
	doc.Routers.Think.Providers = []string{"fake-think"}
	doc.Skills.Allowed = []string{"warm_transfer", "create_ticket"}
	doc.Skills.Definitions = map[string]profile.SkillDefinition{
		"warm_transfer": {Gateway: "coral-transfer", Authority: "act"},
		"create_ticket": {Gateway: "coral-crm", Authority: "act"},
	}
	if err := profile.Validate(doc, reg); err != nil {
		t.Fatal(err)
	}
}

func TestSession_EdgeTokenSigned(t *testing.T) {
	reg := router.NewMemRegistry()
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	registerPhaseD(t, reg, mem)
	secret := []byte("phase-d-test-secret")
	srv := control.New(mem, reg, control.Config{EdgeTokenSecret: secret})
	createProfile(t, srv, "edge-lab")
	publishOK(t, srv, "edge-lab", `{
  "id":"edge-lab",
  "modes":{"listen":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "routers":{"listen":{"providers":["fake-listen"]}}
}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"edge-lab","profile_version":"latest","clock":"playback","tenant_id":"t1"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	tok := created["edge_token"].(string)
	claims, err := token.Validate(secret, tok, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if claims.SessionID != created["session_id"] {
		t.Fatalf("session mismatch %q vs %v", claims.SessionID, created["session_id"])
	}
}

func TestPlaybackJob_FileFeedsBus(t *testing.T) {
	reg := router.NewMemRegistry()
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	registerPhaseD(t, reg, mem)
	mgr := session.NewManager(reg)
	srv := control.NewWithRuntime(mem, reg, &control.SessionRuntime{Mgr: mgr}, control.Config{}, nil)
	createProfile(t, srv, "pb-lab")
	publishOK(t, srv, "pb-lab", `{
  "id":"pb-lab",
  "modes":{"listen":true},
  "audio":{"canonical_sample_rate_hz":16000,"frame_ms":20},
  "routers":{"listen":{"providers":["fake-listen"]}}
}`)

	dir := t.TempDir()
	path := filepath.Join(dir, "clip.pcm")
	n := clock.FrameBytes(16000, 20)
	blob := make([]byte, n*4)
	for i := 0; i < len(blob); i += 2 {
		binary.LittleEndian.PutUint16(blob[i:], 200)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{
		"profile_id": "pb-lab", "profile_version": "latest", "file_uri": path,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/playback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("enqueue %d %s", rr.Code, rr.Body.String())
	}
	var enq map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &enq)
	jobID := enq["job_id"].(string)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = srv.StartPlaybackWorker(ctx, mgr)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		rr = httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/jobs/"+jobID, nil))
		var job map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &job)
		if job["state"] == store.JobCompleted {
			return
		}
		if job["state"] == store.JobFailed {
			t.Fatalf("job failed: %v", job)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("playback job did not complete")
}
