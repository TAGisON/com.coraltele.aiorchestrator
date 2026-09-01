package desk_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/desk"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/deskskills"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// call scripts one Coral TFN conversation against the real preset and the stub
// connectors, and keeps a transcript so a failure prints the whole call.
type call struct {
	t   *testing.T
	eng *desk.Engine
	gw  *deskskills.Gateway
	log []string
}

func newCall(t *testing.T, opts ...func(*deskskills.Gateway)) *call {
	t.Helper()
	gw := deskskills.New()
	for _, o := range opts {
		o(gw)
	}
	doc := desk.PresetCoralTFN("default")
	eng := desk.NewEngine(doc, runner(gw, "test-session", "default"))
	eng.SetAttribute(desk.AttrANI, "919812345678")
	c := &call{t: t, eng: eng, gw: gw}
	c.log = append(c.log, "AI: "+eng.Welcome().Text)
	return c
}

func runner(g *deskskills.Gateway, sessionID, tenantID string) desk.SkillRunner {
	return func(ctx context.Context, name string, args map[string]any) (map[string]any, bool, error) {
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, false, err
		}
		res, err := g.Execute(ctx, port.SkillRequest{
			SessionID: port.SessionID(sessionID), Name: name, Args: raw, TenantID: tenantID,
		})
		if err != nil {
			return nil, false, err
		}
		out := map[string]any{}
		if len(res.Output) > 0 {
			_ = json.Unmarshal(res.Output, &out)
		}
		return out, res.OK, nil
	}
}

func (c *call) say(text string) desk.Outcome {
	c.t.Helper()
	out := c.eng.Turn(context.Background(), text)
	c.log = append(c.log, "CALLER: "+text, "AI: "+out.Text)
	return out
}

func (c *call) silence() desk.Outcome {
	c.t.Helper()
	out := c.eng.Silence(context.Background())
	c.log = append(c.log, "CALLER: (silence)", "AI: "+out.Text)
	return out
}

func (c *call) transcript() string { return strings.Join(c.log, "\n") }

func (c *call) fail(format string, args ...any) {
	c.t.Helper()
	c.t.Logf("\n%s", c.transcript())
	c.t.Fatalf(format, args...)
}

func (c *call) wantSaid(out desk.Outcome, want string) {
	c.t.Helper()
	if !strings.Contains(strings.ToLower(out.Text), strings.ToLower(want)) {
		c.fail("reply should contain %q, got: %s", want, out.Text)
	}
}

func (c *call) wantNotSaid(out desk.Outcome, unwanted string) {
	c.t.Helper()
	if strings.Contains(strings.ToLower(out.Text), strings.ToLower(unwanted)) {
		c.fail("reply must not contain %q, got: %s", unwanted, out.Text)
	}
}

func (c *call) wantAttr(key, want string) {
	c.t.Helper()
	if got := c.eng.Attributes()[key]; got != want {
		c.fail("attribute %s = %q, want %q", key, got, want)
	}
}

func (c *call) wantAttrSet(key string) string {
	c.t.Helper()
	got := c.eng.Attributes()[key]
	if strings.TrimSpace(got) == "" {
		c.fail("attribute %s should be set", key)
	}
	return got
}

func (c *call) wantAttrEmpty(key string) {
	c.t.Helper()
	if got := c.eng.Attributes()[key]; strings.TrimSpace(got) != "" {
		c.fail("attribute %s should be empty, got %q", key, got)
	}
}

func (c *call) wantDisposition(want string) {
	c.t.Helper()
	if got := c.eng.Disposition(); got != want {
		c.fail("disposition = %q, want %q", got, want)
	}
}

func (c *call) wantLanguage(want string) {
	c.t.Helper()
	if got := c.eng.Language(); got != want {
		c.fail("language = %q, want %q", got, want)
	}
}

// §1–§14: full English service complaint through troubleshooting to a real ticket.
func TestComplaintEnglishCreatesTicketAndEmail(t *testing.T) {
	c := newCall(t)
	c.wantSaid(c.say("my ip phone is not working and I want to register a complaint"),
		"issue with an IP Phone")
	c.say("yes it is powered on")
	c.say("network shows connected")
	c.say("I cannot make outgoing calls")
	summary := c.say("multiple users")
	c.wantSaid(summary, "affecting multiple users")
	c.wantNotSaid(summary, "multiple_users")

	c.say("register a complaint")
	c.say("Ramesh Kumar")
	c.wantSaid(c.say("ramesh at coral dot com"), "ramesh@coral.com")
	confirm := c.say("yes correct")
	c.wantSaid(confirm, "Product IP Phone")

	created := c.say("yes all correct")
	ticket := c.wantAttrSet(desk.AttrTicketID)
	c.wantSaid(created, ticket)
	c.wantSaid(created, "sent the complaint details")
	c.wantAttr(desk.AttrEmailSent, "true")

	c.say("no thank you")
	c.wantDisposition(desk.DispTicketCreated)
	if !c.eng.Ended() {
		c.fail("call should have ended")
	}

	ledger := c.gw.Ledger()
	tickets, _ := ledger["tickets"].([]deskskills.Ticket)
	if len(tickets) != 1 || tickets[0].ID != ticket {
		c.fail("backend should hold exactly the spoken ticket, got %+v", tickets)
	}
	emails, _ := ledger["emails"].([]deskskills.Email)
	if len(emails) != 1 || emails[0].To != "ramesh@coral.com" {
		c.fail("one confirmation email expected, got %+v", emails)
	}
}

// §17: the same journey entirely in Hindi.
func TestComplaintHindiCreatesTicket(t *testing.T) {
	c := newCall(t)
	first := c.say("मुझे शिकायत दर्ज करानी है")
	c.wantLanguage("hi-IN")
	c.wantSaid(first, "किस Coral Telecom उत्पाद")

	c.say("मीडिया गेटवे")
	c.say("हाँ चालू है")
	c.say("नहीं, कॉल सर्वर से नहीं जुड़ रहा")
	c.say("incoming और outgoing दोनों")
	c.wantSaid(c.say("सभी कॉल्स"), "Media Gateway में")

	c.say("शिकायत दर्ज करें")
	c.say("सुरेश शर्मा")

	// An email address is ASCII but must not flip the call back to English.
	emailTurn := c.say("suresh@coral.com")
	c.wantLanguage("hi-IN")
	c.wantSaid(emailTurn, "क्या यह सही है")

	c.wantSaid(c.say("हाँ सही है"), "क्या यह सारी जानकारी सही है")
	created := c.say("हाँ बिल्कुल सही")
	ticket := c.wantAttrSet(desk.AttrTicketID)
	c.wantSaid(created, ticket)
	c.wantSaid(created, "दर्ज कर ली गई है")

	c.say("नहीं धन्यवाद")
	c.wantDisposition(desk.DispTicketCreated)
	c.wantLanguage("hi-IN")
}

// §17: the caller may switch language at any time and the desk keeps its place.
func TestMidCallLanguageSwitch(t *testing.T) {
	c := newCall(t)
	c.say("I need technical support")
	c.wantLanguage("en-IN")

	switched := c.say("Hindi mein baat kijiye")
	c.wantLanguage("hi-IN")
	c.wantSaid(switched, "किस Coral Telecom उत्पाद")

	c.say("कॉल सर्वर")
	back := c.say("please continue in English")
	c.wantLanguage("en-IN")
	c.wantSaid(back, "describe the problem")

	// The product answered in Hindi survives the switch — no repeated question.
	c.wantAttr(desk.AttrProduct, "call_server")
}

// §5 + §21: technical support collects the three levels then warm-transfers.
func TestTechnicalSupportWarmTransfer(t *testing.T) {
	c := newCall(t)
	c.say("I need technical support")
	c.say("call center")
	c.say("agents cannot log in")
	c.wantSaid(c.say("all agents"), "error message")
	c.wantSaid(c.say("none"), "Is that correct?")
	done := c.say("yes that is correct")
	c.wantSaid(done, "Arjun Singh Topwal")

	c.wantDisposition(desk.DispTransferredTech)
	pack := c.eng.HandoffPack()
	if pack.Owner != "Arjun Singh Topwal" || pack.Target != "technical_support" {
		c.fail("handoff should route to technical support, got %+v", pack)
	}
	for _, want := range []string{"product=call_center", "problem=agents cannot log in", "impact=entire_system"} {
		if !strings.Contains(pack.Summary, want) {
			c.fail("handoff summary missing %q: %s", want, pack.Summary)
		}
	}
	if _, ok := pack.Attributes[desk.AttrErrorAlarm]; !ok {
		c.fail("handoff should carry the error answer: %+v", pack.Attributes)
	}
}

// §14.3 rule 3: "all agents" answers a question; only an explicit ask transfers.
func TestAnswerMentioningAgentsDoesNotTransfer(t *testing.T) {
	c := newCall(t)
	c.say("I need technical support")
	c.say("call center")
	out := c.say("agents cannot log in")
	c.wantNotSaid(out, "transfer your call")
	c.wantAttr(desk.AttrProblem, "agents cannot log in")
}

func TestExplicitHumanRequestTransfers(t *testing.T) {
	c := newCall(t)
	c.say("I need technical support")
	out := c.say("just connect me to an engineer please")
	c.wantSaid(out, "Arjun Singh Topwal")
	c.wantDisposition(desk.DispTransferredTech)
}

// §3: sales transfers when the rep is free.
func TestSalesEnquiryTransfers(t *testing.T) {
	c := newCall(t)
	c.say("I want a quotation for fifty ip phones")
	out := c.say("fifty ip phones for our Delhi office")
	c.wantSaid(out, "Rahul Gupta")
	c.wantDisposition(desk.DispTransferredSales)

	transfers, _ := c.gw.Ledger()["transfers"].([]deskskills.Transfer)
	if len(transfers) != 1 || transfers[0].Owner != "Rahul Gupta" {
		c.fail("expected one sales transfer, got %+v", transfers)
	}
}

// §3: when sales is unavailable the desk registers a callback instead.
func TestSalesUnavailableRegistersEnquiry(t *testing.T) {
	c := newCall(t, func(g *deskskills.Gateway) { g.SetAgentAvailable("sales", false) })
	c.say("I want to buy fifty ip phones, please send a quotation")
	c.wantSaid(c.say("fifty ip phones for our Delhi office"), "arrange a callback")
	c.say("yes please")
	c.say("Anil Verma")
	out := c.say("anil@example.com")

	enquiry := c.wantAttrSet(desk.AttrEnquiryID)
	c.wantSaid(out, enquiry)
	c.say("no that is all")
	c.wantDisposition(desk.DispCallbackScheduled)
}

// §4: product information is answered from the knowledge base, never invented.
func TestProductInformationFromKnowledgeBase(t *testing.T) {
	c := newCall(t)
	out := c.say("tell me about your media gateway")
	c.wantSaid(out, "Coral Media Gateway connects IP telephony")
	c.wantSaid(out, "Rahul Gupta")

	c.say("no thanks")
	c.say("no that is all")
	c.wantDisposition(desk.DispResolvedInfo)
}

func TestProductInformationUnknownProductRefuses(t *testing.T) {
	c := newCall(t, func(g *deskskills.Gateway) { g.SetFailure("search_knowledge", deskskills.StatusFail) })
	out := c.say("tell me about your media gateway")
	c.wantSaid(out, "do not have approved details")
	c.wantNotSaid(out, "connects IP telephony")
}

// §12 + §22 rule 15: a backend failure must never produce a ticket number.
func TestTicketFailureNeverInventsTicketID(t *testing.T) {
	c := newCall(t, func(g *deskskills.Gateway) {
		g.SetFailure("create_service_complaint", deskskills.StatusFail)
	})
	c.say("I want to register a complaint about my ip phone")
	c.say("yes powered on")
	c.say("network is fine")
	c.say("calls are dropping")
	c.say("multiple users")
	c.say("register a complaint")
	c.say("Ramesh Kumar")
	c.say("ramesh@coral.com")
	c.say("yes")
	out := c.say("yes all correct")

	c.wantSaid(out, "unable to register the complaint")
	c.wantSaid(out, "not created any ticket")
	c.wantAttrEmpty(desk.AttrTicketID)
	if strings.Contains(out.Text, "CTL-") {
		c.fail("desk spoke a ticket id after a backend failure: %s", out.Text)
	}
	if tickets, _ := c.gw.Ledger()["tickets"].([]deskskills.Ticket); len(tickets) != 0 {
		c.fail("no ticket should exist in the backend, got %+v", tickets)
	}

	// §12: it then offers the technical support fallback.
	c.wantSaid(out, "Arjun Singh Topwal")
	c.wantSaid(c.say("yes please connect"), "Technical Support team")
	c.wantDisposition(desk.DispTransferredTech)
}

// §13: email failure keeps the ticket and says so.
func TestEmailFailureKeepsTicket(t *testing.T) {
	c := newCall(t, func(g *deskskills.Gateway) {
		g.SetFailure("send_complaint_email", deskskills.StatusFail)
	})
	c.say("I want to register a complaint about my ip phone")
	c.say("yes powered on")
	c.say("network is fine")
	c.say("calls are dropping")
	c.say("multiple users")
	c.say("register a complaint")
	c.say("Ramesh Kumar")
	c.say("ramesh@coral.com")
	c.say("yes")
	out := c.say("yes all correct")

	ticket := c.wantAttrSet(desk.AttrTicketID)
	c.wantSaid(out, ticket)
	c.wantSaid(out, "unable to send the confirmation email")
	if tickets, _ := c.gw.Ledger()["tickets"].([]deskskills.Ticket); len(tickets) != 1 {
		c.fail("ticket must survive an email failure, got %+v", tickets)
	}
	c.say("no thanks")
	c.wantDisposition(desk.DispTicketCreated)
}

// §14: an existing open complaint is surfaced instead of a duplicate.
func TestDuplicateComplaintOffersExistingTicket(t *testing.T) {
	c := newCall(t, func(g *deskskills.Gateway) {
		g.SetFailure("find_open_complaint", deskskills.StatusDuplicate)
	})
	c.say("I want to register a complaint about my ip phone")
	c.say("yes powered on")
	c.say("network is fine")
	c.say("calls are dropping")
	c.say("multiple users")
	c.say("register a complaint")
	c.say("Ramesh Kumar")
	c.say("ramesh@coral.com")
	out := c.say("yes")

	c.wantSaid(out, "existing complaint")
	c.wantSaid(out, "additional information")
	if tickets, _ := c.gw.Ledger()["tickets"].([]deskskills.Ticket); len(tickets) != 0 {
		c.fail("duplicate check must not create a ticket, got %+v", tickets)
	}

	c.say("I want to add information")
	c.say("the problem got worse this morning")
	c.say("no that is all")
	c.wantDisposition(desk.DispExistingTicket)
}

// §15: a critical outage is prioritised and escalated.
func TestCriticalOutageEscalates(t *testing.T) {
	c := newCall(t)
	out := c.say("our entire system is down, no calls are working at any location")
	c.wantSaid(out, "critical service issue")
	c.wantAttr(desk.AttrPriority, "critical")

	done := c.say("yes connect me immediately")
	c.wantSaid(done, "Arjun Singh Topwal")
	c.wantDisposition(desk.DispTransferredTech)

	transfers, _ := c.gw.Ledger()["transfers"].([]deskskills.Transfer)
	if len(transfers) != 1 || transfers[0].Priority != "critical" {
		c.fail("transfer should carry critical priority, got %+v", transfers)
	}
}

// §19: the silence ladder nudges twice then says goodbye.
func TestSilenceLadderEndsCall(t *testing.T) {
	c := newCall(t)
	c.wantSaid(c.silence(), "still on the call")
	c.wantSaid(c.silence(), "Sales, Product Information")
	last := c.silence()
	c.wantSaid(last, "unable to hear a response")
	if !last.End {
		c.fail("third silence should end the call")
	}
	c.wantDisposition(desk.DispAbandonedSilence)
}

func TestSilenceResetsWhenCallerSpeaks(t *testing.T) {
	c := newCall(t)
	c.silence()
	c.say("I need technical support")
	c.wantSaid(c.silence(), "still on the call")
	if c.eng.Ended() {
		c.fail("call should still be live")
	}
}

// §16: two unclear turns re-offer the menu rather than guessing.
func TestUnclearRequestClarifies(t *testing.T) {
	c := newCall(t)
	c.wantSaid(c.say("zzz qqq"), "Sales Enquiry, Product Information")
	c.wantSaid(c.say("hmm xyz"), "briefly describe what you need")
}

func TestHinglishProductInformationIntent(t *testing.T) {
	c := newCall(t)
	out := c.say("मुझे प्रोडक्ट के बारे में जानना है")
	if c.eng.Attributes()[desk.AttrIntent] != "product_information" {
		c.fail("intent=%q want product_information", c.eng.Attributes()[desk.AttrIntent])
	}
	c.wantSaid(out, "उत्पाद")
}

// §20: after one request is closed the desk accepts a second one.
func TestSecondRequestInSameCall(t *testing.T) {
	c := newCall(t)
	c.say("tell me about your call server")
	c.say("no thanks")
	next := c.say("yes")
	c.wantSaid(next, "Sales Enquiry, Product Information")
	if c.eng.Ended() {
		c.fail("call should continue after the caller asks for more help")
	}
	out := c.say("I want a quotation for twenty ip phones")
	c.wantSaid(out, "sales enquiry")
}

func TestTransferUnavailableFallsBackToComplaint(t *testing.T) {
	c := newCall(t, func(g *deskskills.Gateway) { g.SetAgentAvailable("technical_support", false) })
	c.say("I need technical support")
	c.say("call server")
	c.say("the system keeps restarting")
	c.say("entire system")
	c.say("none")
	out := c.say("yes correct")
	c.wantSaid(out, "register a service complaint")

	c.say("yes please register a complaint")
	c.say("Ramesh Kumar")
	c.say("ramesh@coral.com")
	c.say("yes")
	created := c.say("yes all correct")
	c.wantSaid(created, c.wantAttrSet(desk.AttrTicketID))
}
