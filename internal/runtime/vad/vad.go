// Package vad provides local energy-threshold VAD for Talk barge-in.
// Interface-shaped so a later Silero/ONNX wrapper can plug without composer changes.
package vad

import (
	"math"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// Decision is the VAD output for one frame.
type Decision int

const (
	Silence Decision = iota
	Speech
)

// Detector classifies PCM frames. Implementations must be safe for single-session use.
type Detector interface {
	Process(frame port.PCMFrame) Decision
	Reset()
}

// Energy is a deterministic RMS energy VAD with simple hysteresis (CI-friendly).
type Energy struct {
	// Threshold is mean absolute sample amplitude (0–32767). Default 500.
	Threshold float64
	// SpeechHold is consecutive speech frames required to emit Speech. Default 2.
	SpeechHold int
	// SilenceHold is consecutive silence frames required to emit Silence after speech. Default 3.
	SilenceHold int

	speechStreak  int
	silenceStreak int
	inSpeech      bool
}

// NewEnergy returns an energy VAD with defaults suitable for synthetic test PCM.
func NewEnergy() *Energy {
	return &Energy{
		Threshold:   500,
		SpeechHold:  2,
		SilenceHold: 3,
	}
}

func (e *Energy) Reset() {
	e.speechStreak = 0
	e.silenceStreak = 0
	e.inSpeech = false
}

func (e *Energy) Process(frame port.PCMFrame) Decision {
	thr := e.Threshold
	if thr <= 0 {
		thr = 500
	}
	sh := e.SpeechHold
	if sh < 1 {
		sh = 1
	}
	silH := e.SilenceHold
	if silH < 1 {
		silH = 1
	}

	energy := rmsAbs(frame.Data)
	if energy >= thr {
		e.speechStreak++
		e.silenceStreak = 0
		if e.speechStreak >= sh {
			e.inSpeech = true
		}
	} else {
		e.silenceStreak++
		e.speechStreak = 0
		if e.silenceStreak >= silH {
			e.inSpeech = false
		}
	}
	if e.inSpeech {
		return Speech
	}
	return Silence
}

func rmsAbs(data []byte) float64 {
	if len(data) < 2 {
		return 0
	}
	n := len(data) / 2
	var sum float64
	for i := 0; i+1 < len(data); i += 2 {
		v := int16(uint16(data[i]) | uint16(data[i+1])<<8)
		sum += math.Abs(float64(v))
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// SpeechFrame builds a loud mono s16le frame for tests (amplitude near full scale).
func SpeechFrame(rate port.SampleRateHz, seq uint64, nBytes int) port.PCMFrame {
	if nBytes < 2 {
		nBytes = 640
	}
	if nBytes%2 != 0 {
		nBytes++
	}
	data := make([]byte, nBytes)
	for i := 0; i+1 < len(data); i += 2 {
		// ~16000 amplitude — well above default threshold
		data[i] = 0x00
		data[i+1] = 0x40
	}
	return port.PCMFrame{Data: data, SampleRate: rate, Seq: seq}
}

// SilenceFrame builds a zero PCM frame for tests.
func SilenceFrame(rate port.SampleRateHz, seq uint64, nBytes int) port.PCMFrame {
	if nBytes < 2 {
		nBytes = 640
	}
	return port.PCMFrame{Data: make([]byte, nBytes), SampleRate: rate, Seq: seq}
}
