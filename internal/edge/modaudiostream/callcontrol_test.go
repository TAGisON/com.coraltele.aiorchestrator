package modaudiostream_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/modaudiostream"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/token"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/gorilla/websocket"
)

// dialEdge stands up the handler, dials it, and returns both ends.
func dialEdge(t *testing.T) (*websocket.Conn, *modaudiostream.Conn, func()) {
	t.Helper()
	secret := []byte("test-secret")
	tok, err := token.Issue(secret, token.Claims{TenantID: "t", SessionID: "sess-cc"}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	var serverConn *modaudiostream.Conn
	binder := &labBinder{conn: &serverConn, onGone: make(chan struct{}, 1)}
	srv := httptest.NewServer(modaudiostream.NewHandler(secret, binder))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/edge/fs?token=" + tok + "&rate=8000"
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for serverConn == nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if serverConn == nil {
		client.Close()
		srv.Close()
		t.Fatal("server conn not attached")
	}
	return client, serverConn, func() { client.Close(); srv.Close() }
}

// readControl reads text frames until one has the wanted type, so queued media
// frames do not confuse the assertion.
func readControl(t *testing.T, client *websocket.Conn, want string) map[string]any {
	t.Helper()
	_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
	for i := 0; i < 50; i++ {
		_, msg, err := client.ReadMessage()
		if err != nil {
			t.Fatalf("read %s: %v", want, err)
		}
		var m map[string]any
		if json.Unmarshal(msg, &m) != nil {
			continue
		}
		if m["type"] == want {
			return m
		}
	}
	t.Fatalf("never saw a %q control message", want)
	return nil
}

func TestHangupSendsControlVerb(t *testing.T) {
	client, conn, done := dialEdge(t)
	defer done()

	ctx := context.Background()
	if err := conn.Hangup(ctx, "NORMAL_TEMPORARY_FAILURE"); err != nil {
		t.Fatalf("Hangup: %v", err)
	}
	m := readControl(t, client, "hangup")
	if m["cause"] != "NORMAL_TEMPORARY_FAILURE" {
		t.Fatalf("cause = %v", m["cause"])
	}
	if m["drainMs"] == nil {
		t.Fatal("drainMs must be sent so the module bounds its wait")
	}
}

func TestHangupDefaultsCause(t *testing.T) {
	client, conn, done := dialEdge(t)
	defer done()

	if err := conn.Hangup(context.Background(), ""); err != nil {
		t.Fatalf("Hangup: %v", err)
	}
	if m := readControl(t, client, "hangup"); m["cause"] != "NORMAL_CLEARING" {
		t.Fatalf("cause = %v, want NORMAL_CLEARING", m["cause"])
	}
}

func TestTransferSendsDialplanAndContext(t *testing.T) {
	client, conn, done := dialEdge(t)
	defer done()

	err := conn.Transfer(context.Background(), port.TransferRequest{
		Destination: "1001",
		Reason:      "escalation",
	})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	m := readControl(t, client, "transfer")
	if m["dest"] != "1001" {
		t.Fatalf("dest = %v", m["dest"])
	}
	// These are the uuid_transfer defaults the FreeSWITCH side expects.
	if m["dialplan"] != "XML" {
		t.Fatalf("dialplan = %v, want XML", m["dialplan"])
	}
	if m["context"] != "calltransfer" {
		t.Fatalf("context = %v, want calltransfer", m["context"])
	}
}

func TestTransferHonoursExplicitDialplanContext(t *testing.T) {
	client, conn, done := dialEdge(t)
	defer done()

	err := conn.Transfer(context.Background(), port.TransferRequest{
		Destination: "2002",
		Dialplan:    "inline",
		Context:     "support",
	})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	m := readControl(t, client, "transfer")
	if m["dialplan"] != "inline" || m["context"] != "support" {
		t.Fatalf("overrides ignored: %+v", m)
	}
}

func TestTransferRequiresDestination(t *testing.T) {
	_, conn, done := dialEdge(t)
	defer done()

	if err := conn.Transfer(context.Background(), port.TransferRequest{}); err == nil {
		t.Fatal("empty destination must be rejected before hitting the wire")
	}
}

// The module refuses to retarget an action that is already armed, so the edge
// must not send a second one.
func TestOnlyOneCallControlActionPerConnection(t *testing.T) {
	client, conn, done := dialEdge(t)
	defer done()

	if err := conn.Hangup(context.Background(), "NORMAL_CLEARING"); err != nil {
		t.Fatalf("first Hangup: %v", err)
	}
	readControl(t, client, "hangup")

	if err := conn.Hangup(context.Background(), "NORMAL_CLEARING"); err == nil {
		t.Fatal("second action must be refused")
	}
	if err := conn.Transfer(context.Background(), port.TransferRequest{Destination: "1"}); err == nil {
		t.Fatal("transfer after hangup must be refused")
	}
}

// The RCA fix: a stalled Listen consumer must never block the socket reader,
// because mod_audio_stream services send and receive on one thread — blocking
// the reader back-pressures its uplink and stalls TTS delivery too.
func TestUplinkNeverBlocksWhenConsumerStalls(t *testing.T) {
	client, conn, done := dialEdge(t)
	defer done()

	// Never read conn.Frames(): the 64-slot channel fills immediately.
	frame := make([]byte, modaudiostream.FrameBytes(8000, 20))
	sent := 0
	deadline := time.Now().Add(3 * time.Second)
	for i := 0; i < 400 && time.Now().Before(deadline); i++ {
		_ = client.SetWriteDeadline(time.Now().Add(time.Second))
		if err := client.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			t.Fatalf("write %d stalled: %v", i, err)
		}
		sent++
	}
	if sent < 400 {
		t.Fatalf("only %d/400 uplink frames accepted before the deadline — reader is blocking", sent)
	}

	// Drops must be counted, not silent.
	waitFor(t, 2*time.Second, func() bool { return conn.Stats().UplinkDropped > 0 })

	// And the connection must still be usable for downlink control.
	if err := conn.Hangup(context.Background(), "NORMAL_CLEARING"); err != nil {
		t.Fatalf("downlink still works? %v", err)
	}
	readControl(t, client, "hangup")
}

// WaitMark must not hang when writes fail: pending accounting is released for
// every frame popped from the queue, successfully written or not.
func TestWaitMarkReturnsAfterPeerDisappears(t *testing.T) {
	client, conn, done := dialEdge(t)
	defer done()

	ctx := context.Background()
	canFrame := make([]byte, modaudiostream.FrameBytes(16000, 20)*25) // 500 ms
	if err := conn.WritePCM(ctx, port.PCMFrame{Data: canFrame, SampleRate: 16000}); err != nil {
		t.Fatal(err)
	}
	client.Close()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Either the queue drains or the connection closes; both must return, and
	// neither may hit the context deadline.
	err := conn.WaitMark(waitCtx)
	if err == context.DeadlineExceeded {
		t.Fatal("WaitMark hung after the peer went away")
	}
}

func waitFor(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}
