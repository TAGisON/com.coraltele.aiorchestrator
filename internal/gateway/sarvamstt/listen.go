// Package sarvamstt implements the sarvam-stt Listen gateway (saaras:v3).
package sarvamstt

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvam"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const ID port.GatewayID = "sarvam-stt"

// DialFunc opens a WebSocket (injectable for tests).
type DialFunc func(ctx context.Context, u string, header http.Header) (WSConn, *http.Response, error)

// WSConn is the subset of websocket.Conn used by the stream.
type WSConn interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
	SetWriteDeadline(t time.Time) error
}

// Gateway is Listen over Sarvam REST + legacy speech-to-text WebSocket.
type Gateway struct {
	Cfg        sarvam.Config
	HTTPClient *http.Client
	Dial       DialFunc // nil → gorilla dialer
}

func New(cfg sarvam.Config) *Gateway {
	return &Gateway{Cfg: cfg, HTTPClient: sarvam.DefaultHTTPClient()}
}

func (g *Gateway) ID() port.GatewayID { return ID }

func (g *Gateway) Capabilities() port.Capability {
	return port.Capability{
		Streaming:   true,
		Batch:       true,
		Partials:    true,
		Cancel:      true,
		SampleRates: []port.SampleRateHz{8000, 16000},
	}
}

func (g *Gateway) client() *http.Client {
	if g.HTTPClient != nil {
		return g.HTTPClient
	}
	return sarvam.DefaultHTTPClient()
}

func (g *Gateway) refreshCfg() {
	if cfg, err := sarvam.LoadConfig(); err == nil {
		g.Cfg = cfg
	}
}

func (g *Gateway) RecognizeBatch(ctx context.Context, req port.ListenRequest, pcm []byte) (port.ListenFinal, error) {
	g.refreshCfg()
	if !g.Cfg.Configured() {
		return port.ListenFinal{}, &port.GatewayError{Code: port.CodeAuth, Message: "sarvam api key missing"}
	}
	rate := int(req.SampleRate)
	if rate == 0 {
		rate = 16000
	}
	sendPCM := pcm
	sendRate := rate
	if rate != 8000 && rate != 16000 {
		sendPCM = sarvam.ResamplePCM(pcm, rate, 16000)
		sendRate = 16000
	}
	wav := wrapPCMAsWAV(sendPCM, sendRate)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "audio.wav")
	if err != nil {
		return port.ListenFinal{}, err
	}
	if _, err := part.Write(wav); err != nil {
		return port.ListenFinal{}, err
	}
	_ = w.WriteField("model", sarvam.DefaultSTTModel)
	_ = w.WriteField("mode", sarvam.DefaultSTTMode)
	lang := sarvam.STTLanguageCode(req.LanguageHint)
	_ = w.WriteField("language_code", lang)
	_ = w.WriteField("input_audio_codec", "wav")
	if err := w.Close(); err != nil {
		return port.ListenFinal{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.Cfg.STTRestURL, &body)
	if err != nil {
		return port.ListenFinal{}, err
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	httpReq.Header.Set(sarvam.HeaderAPIKey, g.Cfg.APIKey)

	resp, err := g.client().Do(httpReq)
	if err != nil {
		return port.ListenFinal{}, sarvam.MapDialError(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return port.ListenFinal{}, sarvam.MapHTTPStatus(resp.StatusCode, string(raw))
	}
	var out struct {
		Transcript   string  `json:"transcript"`
		LanguageCode string  `json:"language_code"`
		LanguageProb float32 `json:"language_probability"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return port.ListenFinal{}, &port.GatewayError{Code: port.CodeInternal, Message: "bad stt json", Cause: err}
	}
	conf := out.LanguageProb
	if conf == 0 {
		conf = 1
	}
	langOut := out.LanguageCode
	if langOut == "" {
		langOut = lang
	}
	return port.ListenFinal{Text: out.Transcript, Confidence: conf, Language: langOut}, nil
}

func (g *Gateway) OpenStream(ctx context.Context, req port.ListenRequest) (port.ListenStream, error) {
	g.refreshCfg()
	if !g.Cfg.Configured() {
		return nil, &port.GatewayError{Code: port.CodeAuth, Message: "sarvam api key missing"}
	}
	rate := int(req.SampleRate)
	if rate == 0 {
		rate = 16000
	}
	wsRate := rate
	if rate != 8000 && rate != 16000 {
		wsRate = 16000
	}
	lang := sarvam.STTLanguageCode(req.LanguageHint)

	q := url.Values{}
	q.Set("model", sarvam.DefaultSTTModel)
	q.Set("mode", sarvam.DefaultSTTMode)
	q.Set("language-code", lang)
	q.Set("sample_rate", strconv.Itoa(wsRate))
	q.Set("input_audio_codec", "pcm_s16le")
	q.Set("flush_signal", "true")
	q.Set("high_vad_sensitivity", "true")
	q.Set("vad_signals", "true")
	u := g.Cfg.STTWSURL
	if strings.Contains(u, "?") {
		u += "&" + q.Encode()
	} else {
		u += "?" + q.Encode()
	}

	header := http.Header{}
	header.Set(sarvam.HeaderAPIKey, g.Cfg.APIKey)
	header.Set("Api-Subscription-Key", g.Cfg.APIKey)

	dial := g.Dial
	if dial == nil {
		dial = defaultDial
	}
	conn, _, err := dial(ctx, u, header)
	if err != nil {
		return nil, sarvam.MapDialError(err)
	}

	s := &listenStream{
		conn:     conn,
		partials: make(chan port.ListenPartial, 8),
		finals:   make(chan port.ListenFinal, 8),
		lang:     lang,
		reqRate:  rate,
		wsRate:   wsRate,
	}
	go s.readLoop()
	return s, nil
}

func defaultDial(ctx context.Context, u string, header http.Header) (WSConn, *http.Response, error) {
	d := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, resp, err := d.DialContext(ctx, u, header)
	if err != nil {
		return nil, resp, err
	}
	return conn, resp, nil
}

type listenStream struct {
	conn     WSConn
	partials chan port.ListenPartial
	finals   chan port.ListenFinal
	lang     string
	reqRate  int
	wsRate   int

	mu     sync.Mutex
	closed atomic.Bool
}

func (s *listenStream) WritePCM(ctx context.Context, frame port.PCMFrame) error {
	if s.closed.Load() {
		return &port.GatewayError{Code: port.CodeCancelled, Message: "stream closed"}
	}
	select {
	case <-ctx.Done():
		return sarvam.MapDialError(ctx.Err())
	default:
	}
	pcm := frame.Data
	rate := s.reqRate
	if frame.SampleRate > 0 {
		rate = int(frame.SampleRate)
	}
	if rate != s.wsRate {
		pcm = sarvam.ResamplePCM(pcm, rate, s.wsRate)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return &port.GatewayError{Code: port.CodeCancelled, Message: "stream closed"}
	}
	return s.writeAudioLocked(pcm)
}

func (s *listenStream) writeAudioLocked(pcm []byte) error {
	// LiveKit / Sarvam Python SDK: raw pcm_s16le bytes with encoding label "audio/wav"
	// while connection input_audio_codec=pcm_s16le.
	payload := map[string]any{
		"audio": map[string]any{
			"data":        base64.StdEncoding.EncodeToString(pcm),
			"sample_rate": s.wsRate,
			"encoding":    "audio/wav",
		},
	}
	raw, _ := json.Marshal(payload)
	_ = s.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if err := s.conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		return sarvam.MapDialError(err)
	}
	return nil
}

func (s *listenStream) Partials() <-chan port.ListenPartial { return s.partials }
func (s *listenStream) Finals() <-chan port.ListenFinal     { return s.finals }

func (s *listenStream) Close(ctx context.Context) error {
	if s.closed.Swap(true) {
		return nil
	}
	s.mu.Lock()
	_ = s.conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"flush"}`))
	_ = s.conn.Close()
	s.mu.Unlock()
	return nil
}

func (s *listenStream) readLoop() {
	defer close(s.partials)
	defer close(s.finals)
	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			s.closed.Store(true)
			return
		}
		var envelope struct {
			Type string `json:"type"`
			Data struct {
				Transcript   string  `json:"transcript"`
				LanguageCode string  `json:"language_code"`
				Message      string  `json:"message"`
				Error        string  `json:"error"`
				Code         string  `json:"code"`
				SignalType   string  `json:"signal_type"`
				Confidence   float32 `json:"confidence"`
				LanguageProb float32 `json:"language_probability"`
			} `json:"data"`
		}
		if err := json.Unmarshal(msg, &envelope); err != nil {
			continue
		}
		switch envelope.Type {
		case "data", "transcript":
			lang := envelope.Data.LanguageCode
			if lang == "" {
				lang = s.lang
			}
			conf := envelope.Data.Confidence
			if conf == 0 {
				conf = envelope.Data.LanguageProb
			}
			if conf == 0 {
				conf = 1
			}
			text := envelope.Data.Transcript
			if text == "" {
				continue
			}
			select {
			case s.finals <- port.ListenFinal{Text: text, Confidence: conf, Language: lang}:
			default:
			}
		case "events", "speech_start", "speech_end":
			// VAD signals — Listen port only surfaces finals
		case "error":
			msg := envelope.Data.Message
			if msg == "" {
				msg = envelope.Data.Error
			}
			if msg == "" {
				msg = "sarvam stt error"
			}
			applog.Warn("sarvam stt stream error", "code", envelope.Data.Code, "message", msg)
			s.closed.Store(true)
			return
		default:
			if envelope.Type != "" {
				applog.Info("sarvam stt message", "type", envelope.Type, "signal", envelope.Data.SignalType)
			}
		}
	}
}

func wrapPCMAsWAV(pcm []byte, sampleRate int) []byte {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	const channels = 1
	const bits = 16
	byteRate := sampleRate * channels * bits / 8
	blockAlign := channels * bits / 8
	dataSize := len(pcm)
	buf := make([]byte, 44+dataSize)
	copy(buf[0:], "RIFF")
	binaryPutUint32(buf[4:], uint32(36+dataSize))
	copy(buf[8:], "WAVE")
	copy(buf[12:], "fmt ")
	binaryPutUint32(buf[16:], 16)
	binaryPutUint16(buf[20:], 1) // PCM
	binaryPutUint16(buf[22:], channels)
	binaryPutUint32(buf[24:], uint32(sampleRate))
	binaryPutUint32(buf[28:], uint32(byteRate))
	binaryPutUint16(buf[32:], uint16(blockAlign))
	binaryPutUint16(buf[34:], bits)
	copy(buf[36:], "data")
	binaryPutUint32(buf[40:], uint32(dataSize))
	copy(buf[44:], pcm)
	return buf
}

func binaryPutUint16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func binaryPutUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

// Register adds sarvam-stt. API key may be supplied later via DB credentials.
func Register(reg port.Registry, g *Gateway) error {
	if g == nil {
		cfg, err := sarvam.LoadConfig()
		if err != nil {
			return err
		}
		g = New(cfg)
	}
	return reg.Register(port.Registration{
		ID:           ID,
		Port:         port.PortListen,
		Capabilities: g.Capabilities(),
		Instance:     g,
		Probe: func(ctx context.Context) port.Health {
			g.refreshCfg()
			if !g.Cfg.Configured() {
				return port.Health{Healthy: false, LastError: "api key missing"}
			}
			return port.Health{Healthy: true, LastOK: time.Now()}
		},
	})
}
