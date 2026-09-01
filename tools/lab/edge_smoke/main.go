package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	created, err := postJSON("http://127.0.0.1:8011/v1/sessions", map[string]any{
		"profile_id": "coral-tfn", "profile_version": "latest", "clock": "live",
	})
	if err != nil {
		fatal(err)
	}
	sid, _ := created["session_id"].(string)
	tok, _ := created["edge_token"].(string)
	fmt.Println("sid", sid)

	u := "ws://127.0.0.1:8011/edge/fs?token=" + url.QueryEscape(tok) + "&rate=16000"
	c, resp, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		fmt.Println("dial err:", err)
		if resp != nil {
			b, _ := io.ReadAll(resp.Body)
			fmt.Println("status", resp.StatusCode, string(b))
		}
		os.Exit(1)
	}
	defer c.Close()
	fmt.Println("ws connected")

	_ = c.WriteMessage(websocket.BinaryMessage, make([]byte, 640))

	ans, err := postJSON("http://127.0.0.1:8011/v1/sessions/"+sid+"/answer", map[string]any{})
	if err != nil {
		fmt.Println("answer err", err)
	} else {
		fmt.Println("spoken:", ans["spoken"])
	}

	_ = c.SetReadDeadline(time.Now().Add(8 * time.Second))
	for i := 0; i < 40; i++ {
		mt, msg, err := c.ReadMessage()
		if err != nil {
			fmt.Println("read:", err)
			break
		}
		if mt != websocket.TextMessage {
			continue
		}
		if bytes.Contains(msg, []byte("streamAudio")) {
			fmt.Println("GOT_AUDIO bytes", len(msg))
			break
		}
		s := string(msg)
		if len(s) > 100 {
			s = s[:100] + "..."
		}
		fmt.Println("text:", s)
	}
	_, _ = postJSON("http://127.0.0.1:8011/v1/sessions/"+sid+"/stop", map[string]any{"reason": "smoke"})
}

func postJSON(u string, body any) (map[string]any, error) {
	raw, _ := json.Marshal(body)
	resp, err := http.Post(u, "application/json", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, b)
	}
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out, nil
}

func fatal(err error) {
	fmt.Println(err)
	os.Exit(1)
}
