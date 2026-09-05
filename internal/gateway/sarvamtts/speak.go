// Package sarvamtts implements the sarvam-tts Speak gateway (Bulbul).
package sarvamtts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvam"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const ID port.GatewayID = "sarvam-tts"

// Gateway is Speak over Sarvam REST text-to-speech (bulbul:v3).
type Gateway struct {
	Cfg        sarvam.Config
	HTTPClient *http.Client
	Speaker    string // empty → shubh
	Model      string // empty → bulbul:v3
}

func New(cfg sarvam.Config) *Gateway {
	return &Gateway{
		Cfg:        cfg,
		HTTPClient: sarvam.DefaultHTTPClient(),
		Speaker:    sarvam.DefaultTTSSpeaker,
		Model:      sarvam.DefaultTTSModel,
	}
}

func (g *Gateway) ID() port.GatewayID { return ID }

func (g *Gateway) Capabilities() port.Capability {
	return port.Capability{
		Streaming:   true,
		Batch:       true,
		Cancel:      true,
		SampleRates: []port.SampleRateHz{8000, 16000, 22050, 24000, 32000, 44100, 48000},
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
		g.Cfg = sarvam.MergeRefresh(g.Cfg, cfg)
	}
}

func (g *Gateway) Speak(ctx context.Context, req port.SpeakRequest) (port.SpeakStream, error) {
	g.refreshCfg()
	if !g.Cfg.Configured() {
		return nil, &port.GatewayError{Code: port.CodeAuth, Message: "sarvam api key missing"}
	}
	if req.SSML {
		return nil, &port.GatewayError{Code: port.CodeUnsupported, Message: "ssml not supported by sarvam-tts lab"}
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, &port.GatewayError{Code: port.CodeBadRequest, Message: "empty text"}
	}

	rate := int(req.SampleRate)
	if rate == 0 {
		rate = 16000
	}
	lang := req.Language
	if lang == "" {
		lang = sarvam.DefaultSTTLanguage
	}
	speaker := g.Speaker
	if speaker == "" {
		speaker = sarvam.DefaultTTSSpeaker
	}
	if req.VoiceID != "" {
		speaker = strings.ToLower(req.VoiceID)
	}
	model := g.Model
	if model == "" {
		model = sarvam.DefaultTTSModel
	}
	// Bulbul v2 speaker names are rejected by bulbul:v3; remap known lab defaults.
	if strings.HasPrefix(strings.ToLower(model), "bulbul:v3") {
		speaker = mapBulbulV2Speaker(speaker)
	}

	payload := map[string]any{
		"text":               text,
		"language_code":      lang,
		"speaker":            speaker,
		"model":              model,
		"speech_sample_rate": rate,
		"output_audio_codec": "linear16",
	}
	rawReq, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.Cfg.TTSURL, bytes.NewReader(rawReq))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(sarvam.HeaderAPIKey, g.Cfg.APIKey)

	resp, err := g.client().Do(httpReq)
	if err != nil {
		return nil, sarvam.MapDialError(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, sarvam.MapHTTPStatus(resp.StatusCode, string(raw))
	}
	var out struct {
		Audios []string `json:"audios"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &port.GatewayError{Code: port.CodeInternal, Message: "bad tts json", Cause: err}
	}
	if len(out.Audios) == 0 || out.Audios[0] == "" {
		return nil, &port.GatewayError{Code: port.CodeInternal, Message: "empty tts audios"}
	}
	audioBytes, err := base64.StdEncoding.DecodeString(out.Audios[0])
	if err != nil {
		return nil, &port.GatewayError{Code: port.CodeInternal, Message: "bad tts base64", Cause: err}
	}
	pcm, decodedRate, err := sarvam.DecodeWAVorPCM(audioBytes)
	if err != nil {
		return nil, &port.GatewayError{Code: port.CodeBadAudio, Message: "tts decode failed", Cause: err}
	}
	outRate := rate
	if decodedRate > 0 {
		outRate = decodedRate
	}
	if outRate != rate && rate > 0 {
		pcm = sarvam.ResamplePCM(pcm, outRate, rate)
		outRate = rate
	}

	st := &speakStream{
		frames: make(chan port.PCMFrame, 8),
		done:   make(chan struct{}),
		rate:   port.SampleRateHz(outRate),
		pcm:    pcm,
	}
	go st.run(ctx)
	return st, nil
}

type speakStream struct {
	frames   chan port.PCMFrame
	done     chan struct{}
	rate     port.SampleRateHz
	pcm      []byte
	cancel   atomic.Bool
	doneOnce sync.Once
}

func (s *speakStream) run(ctx context.Context) {
	defer close(s.frames)
	defer s.finish()

	frameN := sarvam.FrameBytes(int(s.rate), 20)
	if frameN < 2 {
		frameN = 320
	}
	seq := uint64(0)
	for offset := 0; offset < len(s.pcm); {
		if s.cancel.Load() {
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		end := offset + frameN
		if end > len(s.pcm) {
			end = len(s.pcm)
		}
		chunk := append([]byte(nil), s.pcm[offset:end]...)
		offset = end
		seq++
		fr := port.PCMFrame{Data: chunk, SampleRate: s.rate, Seq: seq, At: time.Now()}
		select {
		case s.frames <- fr:
		case <-ctx.Done():
			return
		}
		if s.cancel.Load() {
			return
		}
	}
}

func (s *speakStream) finish() {
	s.doneOnce.Do(func() { close(s.done) })
}

func (s *speakStream) Frames() <-chan port.PCMFrame { return s.frames }
func (s *speakStream) Done() <-chan struct{}        { return s.done }

func (s *speakStream) Cancel(ctx context.Context) error {
	s.cancel.Store(true)
	s.finish()
	return nil
}

// mapBulbulV2Speaker remaps Bulbul v2 speaker names that Sarvam rejects on bulbul:v3.
func mapBulbulV2Speaker(speaker string) string {
	switch strings.ToLower(strings.TrimSpace(speaker)) {
	case "anushka", "manisha", "vidya", "arya":
		return "priya"
	case "abhilash", "karun", "hitesh":
		return "shubh"
	default:
		return speaker
	}
}

// Register adds sarvam-tts. API key may be supplied later via DB credentials.
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
		Port:         port.PortSpeak,
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
