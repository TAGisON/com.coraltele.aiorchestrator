package profile_test

import (
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
)

func TestValidate_UnknownGateway(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	doc := profile.Document{}
	doc.Modes.Listen = true
	doc.Routers.Listen.Providers = []string{"not-a-real-gateway"}
	err := profile.Validate(doc, reg)
	ve, ok := err.(*profile.ValidationError)
	if !ok || ve == nil {
		t.Fatalf("want ValidationError, got %v", err)
	}
	if ve.Details["gateway_id"] != "not-a-real-gateway" {
		t.Fatalf("details: %+v", ve.Details)
	}
}

func TestValidate_WrongPort(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	doc := profile.Document{}
	doc.Modes.Speak = true
	doc.Persona.VoiceID = "lab-voice"
	doc.Routers.Speak.Providers = []string{"fake-listen"} // listen id on speak rail
	err := profile.Validate(doc, reg)
	if _, ok := err.(*profile.ValidationError); !ok {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestValidate_HappyFakes(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	doc := profile.Document{}
	doc.Modes.Listen = true
	doc.Modes.Speak = true
	doc.Modes.Think = true
	doc.Audio.CanonicalSampleRateHz = 16000
	doc.Persona.Voice = map[string]string{"fake-speak": "lab-voice"}
	doc.Routers.Listen.Providers = []string{"fake-listen"}
	doc.Routers.Speak.Providers = []string{"fake-speak"}
	doc.Routers.Think.Providers = []string{"fake-think"}
	if err := profile.Validate(doc, reg); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_NoModes(t *testing.T) {
	reg := router.NewMemRegistry()
	err := profile.Validate(profile.Document{}, reg)
	if _, ok := err.(*profile.ValidationError); !ok {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestValidate_TalkSpeakRequiresVoice(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	doc := profile.Document{}
	doc.Modes.Talk = true
	doc.Modes.Speak = true
	doc.Routers.Speak.Providers = []string{"fake-speak"}
	err := profile.Validate(doc, reg)
	ve, ok := err.(*profile.ValidationError)
	if !ok || ve == nil {
		t.Fatalf("want ValidationError, got %v", err)
	}
	if ve.Details["field"] != "persona.voice" {
		t.Fatalf("details %+v", ve.Details)
	}
	doc.Persona.VoiceID = "shubh"
	if err := profile.Validate(doc, reg); err != nil {
		t.Fatal(err)
	}
}

func TestResolveVoiceID_MapThenScalar(t *testing.T) {
	var doc profile.Document
	doc.Persona.Voice = map[string]string{"sarvam-tts": "anushka", "fake-speak": "lab-voice"}
	doc.Persona.VoiceID = "fallback"
	if got := profile.ResolveVoiceID(doc, "sarvam-tts"); got != "anushka" {
		t.Fatalf("map hit=%q", got)
	}
	if got := profile.ResolveVoiceID(doc, "other-tts"); got != "fallback" {
		t.Fatalf("scalar fallback=%q", got)
	}
}

func TestParse_PersonaVoiceStringAlias(t *testing.T) {
	doc, err := profile.Parse([]byte(`{
  "id":"p","modes":{"speak":true},
  "persona":{"voice":"priya-hi"}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Persona.VoiceID != "priya-hi" {
		t.Fatalf("voice_id=%q", doc.Persona.VoiceID)
	}
}
