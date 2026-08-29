package sarvamllm_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvam"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvamllm"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

func TestCompleteContract(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(sarvam.HeaderAPIKey) == "" {
			t.Error("missing api-subscription-key")
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		gotModel, _ = req["model"].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "echo: ping"}},
			},
		})
	}))
	defer srv.Close()

	g := &sarvamllm.Gateway{
		Cfg:        sarvam.Config{APIKey: "test-key", ChatURL: srv.URL},
		HTTPClient: srv.Client(),
		Model:      sarvamllm.DefaultModel,
	}
	res, err := g.Complete(context.Background(), port.ThinkRequest{
		SessionID: "s1",
		Messages:  []port.ChatMessage{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text == "" {
		t.Fatal("empty think")
	}
	if gotModel != sarvamllm.DefaultModel {
		t.Fatalf("model=%q want %q (sarvam-105b-conversations)", gotModel, sarvamllm.DefaultModel)
	}
}

func TestCompleteStreamCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	g := &sarvamllm.Gateway{
		Cfg:        sarvam.Config{APIKey: "test-key", ChatURL: srv.URL},
		HTTPClient: srv.Client(),
	}
	st, err := g.CompleteStream(context.Background(), port.ThinkRequest{
		Messages: []port.ChatMessage{{Role: "user", Content: "x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tok := <-st.Tokens()
	if tok == "" {
		t.Fatal("empty token")
	}
	if err := st.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := st.Result(context.Background())
	if err != nil || res.Text == "" {
		t.Fatalf("result %v %+v", err, res)
	}
}

func TestAuthErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()
	g := &sarvamllm.Gateway{
		Cfg:        sarvam.Config{APIKey: "x", ChatURL: srv.URL},
		HTTPClient: srv.Client(),
	}
	_, err := g.Complete(context.Background(), port.ThinkRequest{
		Messages: []port.ChatMessage{{Role: "user", Content: "x"}},
	})
	ge, ok := port.AsGatewayError(err)
	if !ok || ge.Code != port.CodeAuth {
		t.Fatalf("want auth got %v", err)
	}
	_ = time.Now()
}
