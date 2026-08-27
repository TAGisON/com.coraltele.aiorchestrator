package vad_test

import (
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/vad"
)

func TestEnergy_SpeechThenSilence(t *testing.T) {
	d := vad.NewEnergy()
	d.SpeechHold = 2
	d.SilenceHold = 2
	rate := port.SampleRateHz(16000)
	if d.Process(vad.SilenceFrame(rate, 1, 640)) != vad.Silence {
		t.Fatal("expected silence")
	}
	if d.Process(vad.SpeechFrame(rate, 2, 640)) != vad.Silence {
		t.Fatal("need hold")
	}
	if d.Process(vad.SpeechFrame(rate, 3, 640)) != vad.Speech {
		t.Fatal("expected speech")
	}
	_ = d.Process(vad.SilenceFrame(rate, 4, 640))
	if d.Process(vad.SilenceFrame(rate, 5, 640)) != vad.Silence {
		t.Fatal("expected return to silence")
	}
}
