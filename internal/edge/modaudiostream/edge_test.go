package modaudiostream_test

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/modaudiostream"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/token"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/gorilla/websocket"
)

func TestResample_Identity(t *testing.T) {
	in := make([]byte, 320) // 20ms @ 8k
	for i := 0; i < len(in); i += 2 {
		binary.LittleEndian.PutUint16(in[i:], uint16(i))
	}
	out := modaudiostream.ResamplePCM(in, 8000, 8000)
	if len(out) != len(in) {
		t.Fatalf("len %d want %d", len(out), len(in))
	}
}

func TestResample_8kTo16k(t *testing.T) {
	in := make([]byte, 160) // 10ms @ 8k = 80 samples
	out := modaudiostream.ResamplePCM(in, 8000, 16000)
	if len(out) < 300 || len(out) > 340 {
		t.Fatalf("unexpected out len %d", len(out))
	}
}

type labBinder struct {
	conn   **modaudiostream.Conn
	onGone chan struct{}
}

func (b *labBinder) BindEdge(claims token.Claims, peerRate port.SampleRateHz) (port.SampleRateHz, int, func(), error) {
	return 16000, 20, func() {
		select {
		case b.onGone <- struct{}{}:
		default:
		}
	}, nil
}

func (b *labBinder) AttachConn(sessionID string, conn *modaudiostream.Conn) error {
	*b.conn = conn
	return nil
}

func TestRoundTrip_PCMAndStreamAudio(t *testing.T) {
	secret := []byte("test-secret")
	tok, err := token.Issue(secret, token.Claims{TenantID: "t", SessionID: "sess-1"}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	var serverConn *modaudiostream.Conn
	binder := &labBinder{conn: &serverConn, onGone: make(chan struct{}, 1)}
	h := modaudiostream.NewHandler(secret, binder)

	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/edge/fs?token=" + tok + "&rate=8000"
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// wait for server conn
	deadline := time.Now().Add(2 * time.Second)
	for serverConn == nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if serverConn == nil {
		t.Fatal("server conn not attached")
	}

	// peer → feeder: send 20ms @ 8k binary
	peerFrame := make([]byte, modaudiostream.FrameBytes(8000, 20))
	for i := 0; i < len(peerFrame); i += 2 {
		binary.LittleEndian.PutUint16(peerFrame[i:], 1000)
	}
	if err := client.WriteMessage(websocket.BinaryMessage, peerFrame); err != nil {
		t.Fatal(err)
	}

	select {
	case fr := <-serverConn.Frames():
		if fr.SampleRate != 16000 {
			t.Fatalf("canonical rate %d", fr.SampleRate)
		}
		if len(fr.Data) < 600 {
			t.Fatalf("canonical frame too small %d", len(fr.Data))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no feeder frame")
	}

	// sink → peer: WritePCM canonical, expect streamAudio JSON
	canFrame := make([]byte, modaudiostream.FrameBytes(16000, 20))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := serverConn.WritePCM(ctx, port.PCMFrame{Data: canFrame, SampleRate: 16000}); err != nil {
		t.Fatal(err)
	}

	_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := client.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Type string `json:"type"`
		Data struct {
			SampleRate int    `json:"sampleRate"`
			AudioData  string `json:"audioData"`
		} `json:"data"`
	}
	if err := json.Unmarshal(msg, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != "streamAudio" {
		t.Fatalf("type %q", decoded.Type)
	}
	if decoded.Data.SampleRate != 8000 {
		t.Fatalf("peer rate %d", decoded.Data.SampleRate)
	}
	raw, err := base64.StdEncoding.DecodeString(decoded.Data.AudioData)
	if err != nil || len(raw) == 0 {
		t.Fatalf("audioData: %v len=%d", err, len(raw))
	}

	// Flush barge-in
	if err := serverConn.Flush(ctx); err != nil {
		t.Fatal(err)
	}

	_ = serverConn.Close(ctx)
}

func TestAuth_RejectsBadToken(t *testing.T) {
	h := modaudiostream.NewHandler([]byte("secret"), &labBinder{conn: new(*modaudiostream.Conn), onGone: make(chan struct{}, 1)})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/edge/fs?token=bad", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
}
