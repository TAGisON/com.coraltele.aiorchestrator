package modaudiostream

import (
	"encoding/base64"
	"encoding/json"
)

// streamAudio message schema (EDGE_FS.md) — kept private to this package.
type streamAudioMsg struct {
	Type string `json:"type"`
	Data struct {
		AudioDataType string `json:"audioDataType"`
		SampleRate    int    `json:"sampleRate"`
		Channels      int    `json:"channels"`
		AudioData     string `json:"audioData"`
	} `json:"data"`
}

func encodeStreamAudio(pcm []byte, sampleRate int) ([]byte, error) {
	var m streamAudioMsg
	m.Type = "streamAudio"
	m.Data.AudioDataType = "raw"
	m.Data.SampleRate = sampleRate
	m.Data.Channels = 1
	m.Data.AudioData = base64.StdEncoding.EncodeToString(pcm)
	return json.Marshal(m)
}

// inboundEvent is optional JSON from FS (DTMF/stop/error).
type inboundEvent struct {
	Type  string `json:"type"`
	Digit string `json:"digit,omitempty"`
	Data  string `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func parseInboundEvent(raw []byte) (kind, data string, ok bool) {
	var ev inboundEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return "", "", false
	}
	switch ev.Type {
	case "dtmf":
		return "dtmf", ev.Digit, true
	case "stop":
		return "stop", ev.Data, true
	case "error":
		msg := ev.Error
		if msg == "" {
			msg = ev.Data
		}
		return "error", msg, true
	default:
		return "", "", false
	}
}
