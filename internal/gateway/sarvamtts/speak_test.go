package sarvamtts_test

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvam"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvamtts"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

func TestSpeakFramesContract(t *testing.T) {
	// 40 ms of silence at 16 kHz mono s16le
	pcm := make([]byte, 16000*2/25)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(sarvam.HeaderAPIKey) == "" {
			t.Error("missing api-subscription-key")
		}
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != sarvam.DefaultTTSModel {
			t.Errorf("model=%v", req["model"])
		}
		if req["speaker"] != sarvam.DefaultTTSSpeaker {
			t.Errorf("speaker=%v", req["speaker"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"audios": []string{base64.StdEncoding.EncodeToString(pcm)},
		})
	}))
	defer srv.Close()

	g := &sarvamtts.Gateway{
		Cfg:        sarvam.Config{APIKey: "test-key", TTSURL: srv.URL},
		HTTPClient: srv.Client(),
	}
	ss, err := g.Speak(context.Background(), port.SpeakRequest{
		SessionID: "s1", Text: "hello", SampleRate: 16000, Language: "en-IN",
	})
	if err != nil {
		t.Fatal(err)
	}
	fr := <-ss.Frames()
	if len(fr.Data) == 0 {
		t.Fatal("expected frame")
	}
	<-ss.Done()
}

func TestSpeakCancelStopsDelivery(t *testing.T) {
	// Large PCM so cancel can win before all frames emit.
	pcm := make([]byte, 16000*2) // 1s
	for i := 0; i < len(pcm); i += 2 {
		binary.LittleEndian.PutUint16(pcm[i:], 1000)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"audios": []string{base64.StdEncoding.EncodeToString(pcm)},
		})
	}))
	defer srv.Close()

	g := &sarvamtts.Gateway{
		Cfg:        sarvam.Config{APIKey: "test-key", TTSURL: srv.URL},
		HTTPClient: srv.Client(),
	}
	ss, err := g.Speak(context.Background(), port.SpeakRequest{
		SessionID: "s1", Text: "x", SampleRate: 16000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ss.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ss.Done():
	case <-time.After(time.Second):
		t.Fatal("done not closed after cancel")
	}
}

func TestSSMLUnsupported(t *testing.T) {
	g := &sarvamtts.Gateway{Cfg: sarvam.Config{APIKey: "k"}}
	_, err := g.Speak(context.Background(), port.SpeakRequest{Text: "<speak>x</speak>", SSML: true})
	ge, ok := port.AsGatewayError(err)
	if !ok || ge.Code != port.CodeUnsupported {
		t.Fatalf("want unsupported got %v", err)
	}
}

func TestSpeakVoiceIDOverridesDefaultSpeaker(t *testing.T) {
	pcm := make([]byte, 16000*2/25)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["speaker"] != "anushka" {
			t.Errorf("speaker=%v want anushka (from VoiceID)", req["speaker"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"audios": []string{base64.StdEncoding.EncodeToString(pcm)},
		})
	}))
	defer srv.Close()

	g := &sarvamtts.Gateway{
		Cfg:        sarvam.Config{APIKey: "test-key", TTSURL: srv.URL},
		HTTPClient: srv.Client(),
		Speaker:    sarvam.DefaultTTSSpeaker, // would be shubh without VoiceID
	}
	ss, err := g.Speak(context.Background(), port.SpeakRequest{
		SessionID: "s1", Text: "hello", SampleRate: 16000, Language: "hi-IN",
		VoiceID: "Anushka",
	})
	if err != nil {
		t.Fatal(err)
	}
	<-ss.Frames()
	<-ss.Done()
}
