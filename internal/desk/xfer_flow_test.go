package desk_test

import (
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/desk"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/deskskills"
)

func newXferCall(t *testing.T) *call {
	t.Helper()
	gw := deskskills.New()
	doc := desk.PresetCoralXfer("default")
	eng := desk.NewEngine(doc, runner(gw, "xfer-session", "default"))
	eng.SetAttribute(desk.AttrANI, "919800011122")
	c := &call{t: t, eng: eng, gw: gw}
	c.log = append(c.log, "AI: "+eng.Welcome().Text)
	return c
}

func TestCoralXferLanguageAskThenSalesTransfer(t *testing.T) {
	c := newXferCall(t)
	menu := c.eng.ServicesMenu()
	c.log = append(c.log, "AI: "+menu)
	if !strings.Contains(strings.ToLower(menu), "language") {
		c.fail("new ANI should ask language, got %q", menu)
	}
	out := c.say("Hindi")
	if out.End {
		c.fail("language ask should not hangup: %+v", out)
	}
	if c.eng.Language() != "hi-IN" {
		c.fail("want hi-IN locked, got %q", c.eng.Language())
	}
	if !strings.Contains(strings.ToLower(out.Text), "sales") {
		c.fail("after language lock expect menu, got %q", out.Text)
	}
	out = c.say("sales please")
	if out.Transfer == nil || out.Transfer.Number != "5002" {
		c.fail("want sales transfer 5002, got %+v text=%q", out.Transfer, out.Text)
	}
	if !out.End || out.Disposition != desk.DispTransferredSales {
		c.fail("want transferred_sales end, got end=%v disp=%q", out.End, out.Disposition)
	}
}

func TestCoralXferReturningANISkipsLanguageAsk(t *testing.T) {
	c := newXferCall(t)
	c.eng.SetLanguage("en-IN")
	menu := c.eng.ServicesMenu()
	c.log = append(c.log, "AI: "+menu)
	if strings.Contains(strings.ToLower(menu), "which language") {
		c.fail("returning ANI must not re-ask language: %q", menu)
	}
	if !strings.Contains(strings.ToLower(menu), "sales") {
		c.fail("expect department menu, got %q", menu)
	}
	if strings.Contains(strings.ToLower(menu), "understood") {
		c.fail("returning menu must not mash language confirm: %q", menu)
	}
}

func TestCoralXferLanguageSwitchDoesNotTransfer(t *testing.T) {
	c := newXferCall(t)
	c.eng.SetLanguage("en-IN")
	_ = c.eng.ServicesMenu()
	out := c.say("Can you please change my language to Punjabi?")
	if out.Transfer != nil || out.End {
		c.fail("language switch must not transfer/end: %+v", out)
	}
	if c.eng.Language() != "pa-IN" {
		c.fail("want pa-IN, got %q", c.eng.Language())
	}
}

func TestCoralXferLanguageAskIgnoresEcho(t *testing.T) {
	c := newXferCall(t)
	_ = c.eng.ServicesMenu()
	out := c.say("कोरल टेलीकॉम में कॉल करने के लिए धन्यवाद")
	if c.eng.Language() == "hi-IN" && !strings.Contains(strings.ToLower(out.Text), "language") {
		// Echo must not lock Hindi; should re-ask language.
		c.fail("welcome echo must not lock language, got lang=%q text=%q", c.eng.Language(), out.Text)
	}
	if !strings.Contains(strings.ToLower(out.Text), "language") {
		c.fail("expect re-ask language after echo, got %q", out.Text)
	}
}

func TestCoralXferAbuseHangup(t *testing.T) {
	c := newXferCall(t)
	c.eng.SetLanguage("en-IN")
	_ = c.eng.ServicesMenu()
	out := c.say("fuck you")
	if !out.End || out.Disposition != desk.DispAbandonedAbuse {
		c.fail("want abuse hangup, got end=%v disp=%q text=%q", out.End, out.Disposition, out.Text)
	}
}

func TestCoralXferOODThreeStrikeHangup(t *testing.T) {
	c := newXferCall(t)
	c.eng.SetLanguage("en-IN")
	_ = c.eng.ServicesMenu()
	var out desk.Outcome
	for i := 0; i < 3; i++ {
		out = c.say("who won the cricket world cup final last year")
	}
	if !out.End || out.Disposition != desk.DispOutOfScope {
		c.fail("want OOD hangup after 3, got end=%v disp=%q text=%q", out.End, out.Disposition, out.Text)
	}
	if !strings.Contains(strings.ToLower(out.Text), "ending") && !strings.Contains(strings.ToLower(out.Text), "goodbye") {
		c.fail("expect spoken hangup reason, got %q", out.Text)
	}
}

func TestCoralXferCorporateTransfer(t *testing.T) {
	c := newXferCall(t)
	c.eng.SetLanguage("en-IN")
	_ = c.eng.ServicesMenu()
	out := c.say("corporate account please")
	if out.Transfer == nil || out.Transfer.Number != "5003" {
		c.fail("want corporate 5003, got %+v text=%q", out.Transfer, out.Text)
	}
	if out.Disposition != desk.DispTransferredCorporate {
		c.fail("want transferred_corporate, got %q", out.Disposition)
	}
}

func TestAbusiveSpeech(t *testing.T) {
	if !desk.AbusiveSpeech("you are a fucking idiot") {
		t.Fatal("expected abuse")
	}
	if desk.AbusiveSpeech("I need sales support please") {
		t.Fatal("sales must not be abuse")
	}
}
