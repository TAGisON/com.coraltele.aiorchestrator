package desk_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/desk"
)

func TestPresetCoralTFNIsPublishable(t *testing.T) {
	d := desk.PresetCoralTFN("default")
	res := desk.Validate(d)
	if !res.Publishable {
		for _, it := range res.Items {
			if it.Blocker && !it.OK {
				t.Errorf("blocker %s (%s): %s", it.ID, it.Label, it.Detail)
			}
		}
		t.Fatal("Coral TFN preset must pass the publish checklist")
	}
	if len(res.Warnings) > 0 {
		t.Fatalf("preset should have no translation warnings: %v", res.Warnings)
	}
}

func TestPresetCoralXferIsPublishable(t *testing.T) {
	d := desk.PresetCoralXfer("default")
	if errs := desk.StructuralErrors(d); len(errs) > 0 {
		t.Fatalf("coral-xfer structural: %v", errs)
	}
	res := desk.Validate(d)
	if !res.Publishable {
		for _, it := range res.Items {
			if it.Blocker && !it.OK {
				t.Errorf("blocker %s (%s): %s", it.ID, it.Label, it.Detail)
			}
		}
		t.Fatal("coral-xfer preset must pass the publish checklist")
	}
	if d.DefaultLanguage != "hi-IN" {
		t.Fatalf("default language want hi-IN got %q", d.DefaultLanguage)
	}
	if d.VoiceID != "ritu" && d.Voice["sarvam-tts"] != "ritu" {
		t.Fatalf("want ritu voice, got voice_id=%q voice=%v", d.VoiceID, d.Voice)
	}
	if d.CX.WelcomeBargeAllowed == nil || *d.CX.WelcomeBargeAllowed {
		t.Fatal("welcome barge must be off")
	}
	wantNum := map[string]string{"sales": "5002", "corporate": "5003", "support": "5004"}
	for intent, num := range wantNum {
		got := d.TransferNumberFor(intent, "")
		if got != num {
			t.Errorf("matrix %s want %s got %s", intent, num, got)
		}
	}
}

func TestPresetHasNoStructuralErrors(t *testing.T) {
	if errs := desk.StructuralErrors(desk.PresetCoralTFN("default")); len(errs) > 0 {
		t.Fatalf("preset paths must resolve: %v", errs)
	}
}

func TestPresetCoversScriptRoutingMatrix(t *testing.T) {
	d := desk.PresetCoralTFN("default")
	want := map[string]string{
		"sales_enquiry":       "Rahul Gupta",
		"product_information": "Rahul Gupta",
		"technical_support":   "Arjun Singh Topwal",
		"service_complaint":   "Ritu",
	}
	for intent, owner := range want {
		row, ok := d.MatrixFor(intent)
		if !ok {
			t.Fatalf("routing matrix missing %s", intent)
		}
		if row.Owner != owner {
			t.Errorf("%s owner = %q, want %q", intent, row.Owner, owner)
		}
	}
}

func TestCoralProductKnowledgeIsBilingual(t *testing.T) {
	text := desk.CoralProductKnowledge()
	if !strings.Contains(text, "IP Phone") || !strings.Contains(text, "Call Center") {
		t.Fatal("English product blurbs missing")
	}
	hasDevanagari := false
	for _, r := range text {
		if r >= 0x0900 && r <= 0x097F {
			hasDevanagari = true
			break
		}
	}
	if !hasDevanagari {
		t.Fatal("Hindi product knowledge must include Devanagari")
	}
}

func TestPresetPromptsExistInBothLanguages(t *testing.T) {
	d := desk.PresetCoralTFN("default")
	for id := range d.Prompts {
		if strings.TrimSpace(d.PromptText(id, "en-IN")) == "" {
			t.Errorf("prompt %s missing canonical en-IN text", id)
		}
	}
}

func TestCompileEmbedsDeskAndLanguages(t *testing.T) {
	raw, err := desk.Compile(desk.PresetCoralTFN("default"))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var doc struct {
		ID       string `json:"id"`
		Language struct {
			Primary       string   `json:"primary"`
			Allowed       []string `json:"allowed"`
			AutoDetect    bool     `json:"auto_detect"`
			MidCallSwitch bool     `json:"mid_call_switch"`
		} `json:"language"`
		Skills struct {
			Allowed []string `json:"allowed"`
		} `json:"skills"`
		Persona struct {
			VoiceID string `json:"voice_id"`
		} `json:"persona"`
		XDesk json.RawMessage `json:"x_desk"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("compiled profile unreadable: %s", err)
	}
	if doc.ID != "coral-tfn" {
		t.Errorf("profile id = %q", doc.ID)
	}
	if doc.Language.Primary != "en-IN" {
		t.Errorf("language primary = %q, want en-IN", doc.Language.Primary)
	}
	if len(doc.Language.Allowed) < 2 {
		t.Errorf("compiled profile should expose multilingual allowlist, got %+v", doc.Language)
	}
	if !doc.Language.AutoDetect || !doc.Language.MidCallSwitch {
		t.Error("Indian multilingual requires auto-detect and mid-call switching")
	}
	if doc.Persona.VoiceID == "" {
		t.Error("compiled profile needs a voice")
	}
	if len(doc.Skills.Allowed) == 0 {
		t.Error("compiled profile should allow the desk skills")
	}
	back, ok := desk.FromProfileDocument(raw)
	if !ok {
		t.Fatal("x_desk should round-trip out of the profile")
	}
	if back.ID != "coral-tfn" || len(back.Paths) == 0 {
		t.Errorf("round-tripped desk incomplete: %+v", back.ID)
	}
}

func TestCompileRejectsDanglingStep(t *testing.T) {
	d := desk.PresetCoralTFN("default")
	p := d.Paths["sales_enquiry"]
	steps := append([]desk.Step(nil), p.Steps...)
	steps[0].Next = "nowhere"
	p.Steps = steps
	d.Paths["sales_enquiry"] = p

	if _, err := desk.Compile(d); err == nil {
		t.Fatal("compile must reject a dangling step reference")
	}
	if desk.Validate(d).Publishable {
		t.Fatal("checklist must fail for a dangling step reference")
	}
}

func TestContentHashIsStableAndSensitive(t *testing.T) {
	a := desk.PresetCoralTFN("default")
	b := desk.PresetCoralTFN("default")
	ha, err := desk.ContentHash(a)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	hb, _ := desk.ContentHash(b)
	if ha != hb {
		t.Fatal("identical desks must hash identically")
	}
	b.Name = "Changed"
	hc, _ := desk.ContentHash(b)
	if ha == hc {
		t.Fatal("a changed desk must hash differently")
	}
}

func TestOutboundDeskRequiresConsent(t *testing.T) {
	d := desk.PresetCoralTFN("default")
	d.Direction = desk.DirectionOutbound
	res := desk.Validate(d)
	if res.Publishable {
		t.Fatal("outbound desk without a consent policy must not publish")
	}
	d.Consent = &desk.ConsentConfig{Required: true, Skill: "scrub_outbound_consent"}
	if !desk.Validate(d).Publishable {
		t.Fatal("outbound desk with a consent policy should publish")
	}
}

func TestYesNoAcrossLanguages(t *testing.T) {
	cases := []struct {
		text    string
		yes, ok bool
	}{
		{"yes", true, true},
		{"yes that is correct", true, true},
		{"no thank you", false, true},
		{"हाँ", true, true},
		{"हाँ सही है", true, true},
		{"हाँ बिल्कुल सही", true, true},
		{"जी हाँ", true, true},
		{"नहीं", false, true},
		{"नहीं धन्यवाद", false, true},
		{"galat hai", false, true},
		{"theek hai", true, true},
		{"my phone is broken", false, false},
	}
	for _, tc := range cases {
		yes, ok := desk.YesNo(tc.text)
		if yes != tc.yes || ok != tc.ok {
			t.Errorf("YesNo(%q) = (%v,%v), want (%v,%v)", tc.text, yes, ok, tc.yes, tc.ok)
		}
	}
}

func TestLanguageDetectionAndSwitching(t *testing.T) {
	langs := []string{"en-IN", "hi-IN"}
	if got := desk.DetectLanguage("मुझे शिकायत दर्ज करानी है", langs); got != "hi-IN" {
		t.Errorf("Devanagari should detect hi-IN, got %q", got)
	}
	if got := desk.DetectLanguage("technical support chahiye", langs); got != "hi-IN" {
		t.Errorf("roman Hindi should detect hi-IN, got %q", got)
	}
	if got := desk.DetectLanguage("I need technical support", langs); got != "en-IN" {
		t.Errorf("English should detect en-IN, got %q", got)
	}

	if got := desk.LanguageSwitchRequest("English mein baat karo", langs); got != "en-IN" {
		t.Errorf("explicit English request = %q", got)
	}
	if got := desk.LanguageSwitchRequest("हिंदी में बात कीजिए", langs); got != "hi-IN" {
		t.Errorf("explicit Hindi request = %q", got)
	}

	// A slot answer is not evidence of a language change.
	for _, quiet := range []string{"suresh@coral.com", "Ramesh Kumar", "9812345678", "CTL-123456"} {
		if got := desk.SwitchLanguageEvidence(quiet, langs, "hi-IN"); got != "" {
			t.Errorf("%q must not switch a Hindi call to %q", quiet, got)
		}
	}
	if got := desk.SwitchLanguageEvidence("please continue in English now", langs, "hi-IN"); got != "en-IN" {
		t.Errorf("a real English sentence should switch, got %q", got)
	}
	if got := desk.SwitchLanguageEvidence("हाँ सही है", langs, "en-IN"); got != "hi-IN" {
		t.Errorf("Devanagari should switch an English call, got %q", got)
	}
}

func TestHumanRequestPrecision(t *testing.T) {
	route := []string{
		"connect me to an agent", "transfer me please", "I want to speak to someone",
		"agent", "agent please", "mujhe agent se baat karao", "I need an engineer",
	}
	for _, text := range route {
		if !desk.HumanRequest(text) {
			t.Errorf("HumanRequest(%q) should be true", text)
		}
	}
	stay := []string{
		"agents cannot log in", "all agents are affected", "the engineer already visited yesterday",
		"our call center agents are unable to receive calls", "transfer of calls is failing",
	}
	for _, text := range stay {
		if desk.HumanRequest(text) {
			t.Errorf("HumanRequest(%q) should be false", text)
		}
	}
}

func TestCriticalDetection(t *testing.T) {
	for _, text := range []string{
		"our entire system is down", "all users cannot make calls",
		"multiple locations affected", "पूरा सिस्टम बंद है",
	} {
		if !desk.CriticalRequest(text) {
			t.Errorf("CriticalRequest(%q) should be true", text)
		}
	}
	if desk.CriticalRequest("one phone is not working") {
		t.Error("a single-phone fault is not critical")
	}
}

func TestValidateSlotSpokenEmailAndPhone(t *testing.T) {
	cases := []struct{ kind, in, want string }{
		{desk.ValidateEmail, "ramesh at coral dot com", "ramesh@coral.com"},
		{desk.ValidateEmail, "my email is ramesh.kumar@coral.co.in", "ramesh.kumar@coral.co.in"},
		{desk.ValidatePhone, "nine eight one two three four five six seven eight", ""},
		{desk.ValidatePhone, "98123 45678", "9812345678"},
		{desk.ValidateProduct, "मीडिया गेटवे", "media_gateway"},
		{desk.ValidateProduct, "our call centre", "call_center"},
		{desk.ValidateYesNo, "हाँ", "yes"},
	}
	for _, tc := range cases {
		got, ok := desk.ValidateSlot(tc.kind, tc.in)
		if tc.want == "" {
			if ok {
				t.Errorf("ValidateSlot(%s,%q) should reject, got %q", tc.kind, tc.in, got)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Errorf("ValidateSlot(%s,%q) = (%q,%v), want %q", tc.kind, tc.in, got, ok, tc.want)
		}
	}
}

func TestPIIMasking(t *testing.T) {
	cases := []struct{ key, in, want string }{
		{desk.AttrCustomerEmail, "ramesh@coral.com", "r***@coral.com"},
		{desk.AttrCustomerPhone, "919812345678", "*******5678"},
		{desk.AttrANI, "919812345678", "*******5678"},
		{desk.AttrTicketID, "CTL-826491", "CTL-826491"},
		{desk.AttrProduct, "ip_phone", "ip_phone"},
	}
	for _, tc := range cases {
		if got := desk.Mask(tc.key, tc.in); got != tc.want {
			t.Errorf("Mask(%s) = %q, want %q", tc.key, got, tc.want)
		}
	}
	if desk.ClassOf(desk.AttrCustomerName) != "confidential" {
		t.Error("customer name must be confidential")
	}
	if desk.ClassOf(desk.AttrProduct) != "none" {
		t.Error("product is not PII")
	}
}

func TestDisplayValueIsLocalised(t *testing.T) {
	if got := desk.DisplayValue(desk.AttrProduct, "ip_phone", "en-IN"); got != "IP Phone" {
		t.Errorf("English product display = %q", got)
	}
	if got := desk.DisplayValue(desk.AttrProduct, "ip_phone", "hi-IN"); got != "आईपी फोन" {
		t.Errorf("Hindi product display = %q", got)
	}
	if got := desk.DisplayValue(desk.AttrProblem, "calls are dropping", "en-IN"); got != "calls are dropping" {
		t.Errorf("free text must pass through, got %q", got)
	}
}

func TestPromptFallsBackToDefaultLanguage(t *testing.T) {
	d := desk.PresetCoralTFN("default")
	p := d.Prompts[desk.PromptClosing]
	p.Text = map[string]string{"en-IN": "Goodbye."}
	d.Prompts[desk.PromptClosing] = p
	if got := d.PromptText(desk.PromptClosing, "hi-IN"); got != "Goodbye." {
		t.Errorf("missing Hindi text should fall back to the default language, got %q", got)
	}
}

func TestDispositionTaxonomyIsClosed(t *testing.T) {
	seen := map[string]bool{}
	for _, code := range desk.Dispositions {
		if seen[code] {
			t.Errorf("duplicate disposition %s", code)
		}
		seen[code] = true
	}
	for _, required := range []string{
		desk.DispTicketCreated, desk.DispExistingTicket, desk.DispTransferredTech,
		desk.DispTransferredSales, desk.DispTransferredService, desk.DispTransferredCorporate,
		desk.DispAbandonedSilence, desk.DispAbandonedAbuse, desk.DispSystemFailure,
	} {
		if !seen[required] {
			t.Errorf("disposition taxonomy missing %s", required)
		}
	}
}

func TestTransferNumberFor(t *testing.T) {
	d := desk.Doc{Matrix: []desk.MatrixRow{
		{Intent: "sales_enquiry", Target: "sales", Number: "5002"},
		{Intent: "technical_support", Target: "5004"}, // legacy digits-in-queue
		{Intent: "service_complaint", Target: "service"},
	}}
	if got := d.TransferNumberFor("sales_enquiry", "sales"); got != "5002" {
		t.Fatalf("number column: got %q", got)
	}
	if got := d.TransferNumberFor("technical_support", "5004"); got != "5004" {
		t.Fatalf("dialable target: got %q", got)
	}
	if got := d.TransferNumberFor("service_complaint", "service"); got != "" {
		t.Fatalf("queue label alone must not dial, got %q", got)
	}
	if got := d.TransferNumberFor("product_information", "sales"); got != "5002" {
		t.Fatalf("same-target fallback: got %q", got)
	}
}

func TestNormalizeSetsWelcomeBargeDefault(t *testing.T) {
	d := desk.Doc{
		ID: "x", Languages: []string{"en-IN"}, DefaultLanguage: "en-IN",
		CX: desk.CXPolicy{SilenceNudge1Ms: 9000, AskTimeoutMs: 9000},
	}
	d.Normalize()
	if d.CX.WelcomeBargeAllowed == nil || *d.CX.WelcomeBargeAllowed {
		t.Fatal("Normalize must default welcome_barge_allowed=false")
	}
	if d.CX.PrimaryLocale != "en-IN" {
		t.Fatalf("primary_locale=%q", d.CX.PrimaryLocale)
	}
	if d.CX.LocaleSynthesis == nil || !*d.CX.LocaleSynthesis {
		t.Fatal("Normalize must default locale_synthesis=true")
	}
}
