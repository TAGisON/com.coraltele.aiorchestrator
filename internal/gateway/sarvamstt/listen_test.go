package sarvamstt_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvam"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvamstt"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

func TestRecognizeBatchContract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(sarvam.HeaderAPIKey) == "" {
			t.Error("missing api-subscription-key")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transcript":    "batch-hello",
			"language_code": "en-IN",
		})
	}))
	defer srv.Close()

	g := &sarvamstt.Gateway{
		Cfg:        sarvam.Config{APIKey: "test-key", STTRestURL: srv.URL},
		HTTPClient: srv.Client(),
	}
	final, err := g.RecognizeBatch(context.Background(), port.ListenRequest{
		SessionID: "s1", SampleRate: 16000, LanguageHint: "en-IN", Clock: "live",
	}, make([]byte, 320))
	if err != nil {
		t.Fatal(err)
	}
	if final.Text != "batch-hello" {
		t.Fatalf("text=%q", final.Text)
	}
}

func TestOpenStreamWritePCMFinals(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			var envelope map[string]any
			_ = json.Unmarshal(msg, &envelope)
			if audio, ok := envelope["audio"].(map[string]any); ok {
				if enc, _ := audio["encoding"].(string); enc != "audio/wav" {
					t.Errorf("encoding=%q want audio/wav", enc)
				}
				once.Do(func() {
					_ = c.WriteMessage(websocket.TextMessage, []byte(`{"type":"data","data":{"transcript":"hi","language_code":"en-IN"}}`))
				})
			}
			if typ, _ := envelope["type"].(string); typ == "flush" {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	g := &sarvamstt.Gateway{
		Cfg: sarvam.Config{APIKey: "test-key", STTWSURL: wsURL},
		Dial: func(ctx context.Context, u string, header http.Header) (sarvamstt.WSConn, *http.Response, error) {
			d := websocket.Dialer{}
			conn, resp, err := d.DialContext(ctx, u, header)
			return conn, resp, err
		},
	}
	stream, err := g.OpenStream(context.Background(), port.ListenRequest{
		SessionID: "s1", SampleRate: 16000, LanguageHint: "en-IN", Clock: "live",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.WritePCM(context.Background(), port.PCMFrame{Data: []byte{0, 0, 1, 0}, SampleRate: 16000}); err != nil {
		t.Fatal(err)
	}
	select {
	case final := <-stream.Finals():
		if final.Text != "hi" {
			t.Fatalf("final=%q", final.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for final")
	}
	_ = stream.Close(context.Background())
}

func TestMissingKey(t *testing.T) {
	t.Setenv("SARVAM_API_KEY", "")
	t.Setenv("AIORCH_SECRETS_FILE", filepath.Join(t.TempDir(), "missing-secrets.json"))
	sarvam.SetKeyProvider(nil)
	g := &sarvamstt.Gateway{Cfg: sarvam.Config{}}
	_, err := g.RecognizeBatch(context.Background(), port.ListenRequest{}, nil)
	ge, ok := port.AsGatewayError(err)
	if !ok || ge.Code != port.CodeAuth {
		t.Fatalf("want auth got %v", err)
	}
}

func TestRecognizeBatch_EmptyHintSendsUnknown(t *testing.T) {
	var gotLang string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		gotLang = r.FormValue("language_code")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transcript":    "ok",
			"language_code": "hi-IN",
		})
	}))
	defer srv.Close()
	g := &sarvamstt.Gateway{
		Cfg:        sarvam.Config{APIKey: "test-key", STTRestURL: srv.URL},
		HTTPClient: srv.Client(),
	}
	_, err := g.RecognizeBatch(context.Background(), port.ListenRequest{
		SessionID: "s1", SampleRate: 16000, LanguageHint: "", Clock: "live",
	}, make([]byte, 320))
	if err != nil {
		t.Fatal(err)
	}
	if gotLang != "unknown" {
		t.Fatalf("language_code=%q want unknown", gotLang)
	}
}

func TestRecognizeBatch_AutoHintSendsUnknown(t *testing.T) {
	var gotLang string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		gotLang = r.FormValue("language_code")
		_ = json.NewEncoder(w).Encode(map[string]any{"transcript": "ok", "language_code": "en-IN"})
	}))
	defer srv.Close()
	g := &sarvamstt.Gateway{
		Cfg:        sarvam.Config{APIKey: "test-key", STTRestURL: srv.URL},
		HTTPClient: srv.Client(),
	}
	_, err := g.RecognizeBatch(context.Background(), port.ListenRequest{
		SessionID: "s1", SampleRate: 16000, LanguageHint: "auto", Clock: "live",
	}, make([]byte, 320))
	if err != nil {
		t.Fatal(err)
	}
	if gotLang != "unknown" {
		t.Fatalf("language_code=%q want unknown", gotLang)
	}
}

func TestRecognizeBatch_ConcreteHintPassthrough(t *testing.T) {
	var gotLang string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		gotLang = r.FormValue("language_code")
		_ = json.NewEncoder(w).Encode(map[string]any{"transcript": "ok", "language_code": "hi-IN"})
	}))
	defer srv.Close()
	g := &sarvamstt.Gateway{
		Cfg:        sarvam.Config{APIKey: "test-key", STTRestURL: srv.URL},
		HTTPClient: srv.Client(),
	}
	_, err := g.RecognizeBatch(context.Background(), port.ListenRequest{
		SessionID: "s1", SampleRate: 16000, LanguageHint: "hi-IN", Clock: "live",
	}, make([]byte, 320))
	if err != nil {
		t.Fatal(err)
	}
	if gotLang != "hi-IN" {
		t.Fatalf("language_code=%q want hi-IN", gotLang)
	}
}

func TestOpenStream_EmptyHintLanguageCodeQuery(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("language-code")
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = c.Close()
	}))
	defer srv.Close()
	wsURL := "ws" + srv.URL[len("http"):]
	g := &sarvamstt.Gateway{
		Cfg: sarvam.Config{APIKey: "test-key", STTWSURL: wsURL},
		Dial: func(ctx context.Context, u string, header http.Header) (sarvamstt.WSConn, *http.Response, error) {
			d := websocket.Dialer{}
			conn, resp, err := d.DialContext(ctx, u, header)
			return conn, resp, err
		},
	}
	stream, err := g.OpenStream(context.Background(), port.ListenRequest{
		SessionID: "s1", SampleRate: 16000, LanguageHint: "", Clock: "live",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Close(context.Background())
	if gotQuery != "unknown" {
		t.Fatalf("language-code=%q want unknown", gotQuery)
	}
}
