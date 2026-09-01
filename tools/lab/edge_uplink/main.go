package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	created := post("http://127.0.0.1:8011/v1/sessions", map[string]any{
		"profile_id": "coral-cc", "profile_version": "latest", "clock": "live",
	})
	sid, _ := created["session_id"].(string)
	tok, _ := created["edge_token"].(string)
	fmt.Println("sid", sid)

	u := "ws://127.0.0.1:8011/edge/fs?token=" + url.QueryEscape(tok) + "&rate=16000"
	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	go func() {
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ans := post("http://127.0.0.1:8011/v1/sessions/"+sid+"/answer", map[string]any{})
	fmt.Println("spoken:", ans["spoken"])

	const rate = 16000
	const frame = 320
	for n := 0; n < rate*5/frame; n++ {
		buf := make([]byte, frame*2)
		for i := 0; i < frame; i++ {
			t := float64(n*frame+i) / rate
			// AM speech-ish burst: tone + silence to trip VAD
			amp := 0.0
			phase := n % 50
			if phase < 35 {
				amp = 14000
			}
			v := int16(amp * math.Sin(2*math.Pi*220*t))
			binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
		}
		_ = c.WriteMessage(websocket.BinaryMessage, buf)
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println("sent 5s pcm")
	time.Sleep(4 * time.Second)
	tr := get("http://127.0.0.1:8011/v1/sessions/" + sid + "/transcript")
	b, _ := json.MarshalIndent(tr, "", "  ")
	fmt.Println(string(b))
	_ = post("http://127.0.0.1:8011/v1/sessions/"+sid+"/stop", map[string]any{"reason": "uplink-smoke"})
}

func post(u string, body any) map[string]any {
	raw, _ := json.Marshal(body)
	resp, err := http.Post(u, "application/json", bytes.NewReader(raw))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode >= 300 {
		fmt.Println("POST", u, resp.Status, out)
		os.Exit(1)
	}
	return out
}

func get(u string) map[string]any {
	resp, err := http.Get(u)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}
