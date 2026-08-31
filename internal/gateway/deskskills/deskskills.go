// Package deskskills implements the Contact Desk skill contracts (§9) as a single
// Skill gateway. Stub mode is deterministic and self-contained so a desk can be
// configured, published and tested end to end before a CRM/ACD connector exists.
package deskskills

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// ID is the gateway id desks bind to by default.
const ID port.GatewayID = "desk-skills"

// Status values returned to the desk engine (branch keys).
const (
	StatusOK          = "ok"
	StatusFail        = "fail"
	StatusDuplicate   = "duplicate"
	StatusTimeout     = "timeout"
	StatusUnavailable = "unavailable"
)

// Ticket is one stub complaint record.
type Ticket struct {
	ID         string            `json:"ticket_id"`
	TenantID   string            `json:"tenant_id"`
	SessionID  string            `json:"session_id"`
	Status     string            `json:"status"`
	Attributes map[string]string `json:"attributes"`
	Notes      []string          `json:"notes,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

// Email is one stub outbound email record.
type Email struct {
	To        string    `json:"to"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	TicketID  string    `json:"ticket_id"`
	SessionID string    `json:"session_id"`
	SentAt    time.Time `json:"sent_at"`
}

// Transfer is one stub warm transfer record.
type Transfer struct {
	SessionID  string            `json:"session_id"`
	Target     string            `json:"target"`
	Owner      string            `json:"owner"`
	Priority   string            `json:"priority"`
	Summary    string            `json:"summary"`
	Attributes map[string]string `json:"attributes"`
	At         time.Time         `json:"at"`
}

// Gateway holds the stub ledger and operator-controlled failure injection.
type Gateway struct {
	mu        sync.Mutex
	seq       int
	tickets   []Ticket
	emails    []Email
	transfers []Transfer
	enquiries []map[string]string
	failures  map[string]string // skill → forced status
	agentDown map[string]bool   // queue target → unavailable
	knowledge map[string]string
}

// New builds a gateway with the Coral product knowledge blurbs preloaded.
func New() *Gateway {
	return &Gateway{
		failures:  map[string]string{},
		agentDown: map[string]bool{},
		knowledge: defaultKnowledge(),
	}
}

func (g *Gateway) ID() port.GatewayID { return ID }

func (g *Gateway) Capabilities() port.Capability { return port.Capability{Batch: true} }

// SetFailure forces a status for one skill ("" clears). Used by the Admin console
// and tests to prove the never-invent laws (§12, §13).
func (g *Gateway) SetFailure(skill, status string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if strings.TrimSpace(status) == "" {
		delete(g.failures, skill)
		return
	}
	g.failures[skill] = status
}

// Failures returns the active injection map.
func (g *Gateway) Failures() map[string]string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.failuresLocked()
}

func (g *Gateway) failuresLocked() map[string]string {
	out := map[string]string{}
	for k, v := range g.failures {
		out[k] = v
	}
	return out
}

// SetAgentAvailable toggles a transfer target's availability.
func (g *Gateway) SetAgentAvailable(target string, available bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if available {
		delete(g.agentDown, target)
		return
	}
	g.agentDown[target] = true
}

// Ledger returns a snapshot for the Admin console.
func (g *Gateway) Ledger() map[string]any {
	g.mu.Lock()
	defer g.mu.Unlock()
	return map[string]any{
		"tickets":   append([]Ticket(nil), g.tickets...),
		"emails":    append([]Email(nil), g.emails...),
		"transfers": append([]Transfer(nil), g.transfers...),
		"enquiries": append([]map[string]string(nil), g.enquiries...),
		"failures":  g.failuresLocked(),
	}
}

// Reset clears the ledger (lab reruns).
func (g *Gateway) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.tickets = nil
	g.emails = nil
	g.transfers = nil
	g.enquiries = nil
	g.failures = map[string]string{}
	g.agentDown = map[string]bool{}
}

func (g *Gateway) Execute(ctx context.Context, req port.SkillRequest) (port.SkillResult, error) {
	args := map[string]any{}
	if len(req.Args) > 0 {
		_ = json.Unmarshal(req.Args, &args)
	}
	out, err := g.run(ctx, req, args)
	if err != nil {
		return port.SkillResult{}, err
	}
	body, mErr := json.Marshal(out)
	if mErr != nil {
		return port.SkillResult{}, mErr
	}
	status, _ := out["status"].(string)
	return port.SkillResult{OK: status == StatusOK, Output: body}, nil
}

func (g *Gateway) run(ctx context.Context, req port.SkillRequest, args map[string]any) (map[string]any, error) {
	if forced := g.forced(req.Name); forced != "" {
		if forced == StatusTimeout {
			return nil, &port.GatewayError{Code: port.CodeUnavailable, Message: req.Name + " timeout (injected)", Retryable: true}
		}
		return map[string]any{"status": forced, "message": "injected " + forced}, nil
	}
	switch req.Name {
	case "resolve_caller":
		return g.resolveCaller(args), nil
	case "search_knowledge":
		return g.searchKnowledge(args), nil
	case "find_open_complaint":
		return g.findOpenComplaint(req, args), nil
	case "transfer_to_queue":
		return g.transferToQueue(req, args), nil
	case "create_service_complaint":
		return g.createComplaint(req, args), nil
	case "send_complaint_email":
		return g.sendEmail(req, args), nil
	case "register_sales_enquiry":
		return g.registerEnquiry(args), nil
	case "schedule_callback":
		return g.scheduleCallback(args), nil
	case "push_disposition":
		return map[string]any{"status": StatusOK}, nil
	case "scrub_outbound_consent":
		return g.scrubConsent(args), nil
	default:
		return map[string]any{"status": StatusFail, "message": "unknown skill " + req.Name}, nil
	}
}

func (g *Gateway) forced(skill string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.failures[skill]
}

func (g *Gateway) resolveCaller(args map[string]any) map[string]any {
	phone := str(args["phone"])
	if phone == "" {
		return map[string]any{"status": StatusFail, "message": "no ani"}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := len(g.tickets) - 1; i >= 0; i-- {
		t := g.tickets[i]
		if t.Attributes["phone"] == phone && t.Attributes["name"] != "" {
			return map[string]any{
				"status":        StatusOK,
				"customer_name": t.Attributes["name"],
				"known_caller":  "true",
			}
		}
	}
	return map[string]any{"status": StatusOK, "known_caller": "false"}
}

func (g *Gateway) searchKnowledge(args map[string]any) map[string]any {
	key := strings.ToLower(strings.TrimSpace(str(args["product"])))
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(str(args["query"])))
	}
	g.mu.Lock()
	answer, ok := g.knowledge[key]
	g.mu.Unlock()
	if !ok || answer == "" {
		return map[string]any{"status": StatusFail, "message": "no approved content"}
	}
	return map[string]any{"status": StatusOK, "kb_answer": answer, "kb_source": "coral-products"}
}

func (g *Gateway) findOpenComplaint(req port.SkillRequest, args map[string]any) map[string]any {
	product := str(args["product"])
	email := strings.ToLower(str(args["email"]))
	phone := str(args["phone"])
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := len(g.tickets) - 1; i >= 0; i-- {
		t := g.tickets[i]
		if t.Status != "open" {
			continue
		}
		if t.Attributes["product"] != product {
			continue
		}
		sameCaller := (email != "" && strings.ToLower(t.Attributes["email"]) == email) ||
			(phone != "" && t.Attributes["phone"] == phone)
		if sameCaller {
			return map[string]any{
				"status":             StatusDuplicate,
				"existing_ticket_id": t.ID,
			}
		}
	}
	return map[string]any{"status": StatusOK, "existing_ticket_id": ""}
}

func (g *Gateway) transferToQueue(req port.SkillRequest, args map[string]any) map[string]any {
	target := str(args["target"])
	owner := str(args["owner"])
	g.mu.Lock()
	down := g.agentDown[target]
	g.mu.Unlock()
	if down {
		return map[string]any{"status": StatusUnavailable, "message": owner + " unavailable"}
	}
	attrs := strMap(args["attributes"])
	rec := Transfer{
		SessionID:  string(req.SessionID),
		Target:     target,
		Owner:      owner,
		Priority:   strOr(str(args["priority"]), "normal"),
		Summary:    str(args["summary"]),
		Attributes: attrs,
		At:         time.Now().UTC(),
	}
	g.mu.Lock()
	g.transfers = append(g.transfers, rec)
	g.mu.Unlock()
	return map[string]any{
		"status":          StatusOK,
		"transfer_target": target,
		"transfer_owner":  owner,
	}
}

func (g *Gateway) createComplaint(req port.SkillRequest, args map[string]any) map[string]any {
	email := str(args["email"])
	product := str(args["product"])
	if email == "" || product == "" {
		return map[string]any{"status": StatusFail, "message": "email and product required"}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := len(g.tickets) - 1; i >= 0; i-- {
		t := g.tickets[i]
		if t.Status == "open" && t.Attributes["product"] == product &&
			strings.EqualFold(t.Attributes["email"], email) {
			return map[string]any{"status": StatusDuplicate, "existing_ticket_id": t.ID}
		}
	}
	g.seq++
	id := fmt.Sprintf("CTL-%06d", 100000+(g.seq*7919)%899999)
	attrs := map[string]string{
		"name":            str(args["name"]),
		"email":           email,
		"phone":           str(args["phone"]),
		"product":         product,
		"problem":         str(args["problem"]),
		"impact":          str(args["impact"]),
		"troubleshooting": str(args["troubleshooting"]),
		"priority":        strOr(str(args["priority"]), "normal"),
		"language":        str(args["language"]),
	}
	g.tickets = append(g.tickets, Ticket{
		ID:         id,
		TenantID:   req.TenantID,
		SessionID:  string(req.SessionID),
		Status:     "open",
		Attributes: attrs,
		CreatedAt:  time.Now().UTC(),
	})
	return map[string]any{"status": StatusOK, "ticket_id": id}
}

func (g *Gateway) sendEmail(req port.SkillRequest, args map[string]any) map[string]any {
	to := str(args["email"])
	ticket := str(args["ticket_id"])
	if to == "" || ticket == "" {
		return map[string]any{"status": StatusFail, "message": "email and ticket_id required"}
	}
	body := strings.Join([]string{
		"Ticket ID: " + ticket,
		"Customer name: " + str(args["name"]),
		"Product: " + str(args["product"]),
		"Complaint: " + str(args["problem"]),
		"Status: open",
		"Support: Coral Telecom Limited",
	}, "\n")
	g.mu.Lock()
	g.emails = append(g.emails, Email{
		To:        to,
		Subject:   "Coral Telecom Service Complaint – Ticket " + ticket,
		Body:      body,
		TicketID:  ticket,
		SessionID: string(req.SessionID),
		SentAt:    time.Now().UTC(),
	})
	g.mu.Unlock()
	return map[string]any{"status": StatusOK, "email_sent": true}
}

func (g *Gateway) registerEnquiry(args map[string]any) map[string]any {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	id := fmt.Sprintf("ENQ-%06d", 100000+(g.seq*4703)%899999)
	rec := map[string]string{
		"enquiry_id":  id,
		"name":        str(args["name"]),
		"email":       str(args["email"]),
		"phone":       str(args["phone"]),
		"requirement": str(args["requirement"]),
	}
	g.enquiries = append(g.enquiries, rec)
	return map[string]any{"status": StatusOK, "enquiry_id": id}
}

func (g *Gateway) scheduleCallback(args map[string]any) map[string]any {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	id := fmt.Sprintf("CB-%06d", 100000+(g.seq*3571)%899999)
	return map[string]any{"status": StatusOK, "callback_id": id}
}

func (g *Gateway) scrubOutboundKey(args map[string]any) string {
	return str(args["phone"])
}

func (g *Gateway) scrubConsent(args map[string]any) map[string]any {
	phone := g.scrubOutboundKey(args)
	if phone == "" {
		return map[string]any{"status": StatusFail, "message": "phone required"}
	}
	// Stub policy: numbers ending in 0 are on the do-not-call list.
	if strings.HasSuffix(phone, "0") {
		return map[string]any{"status": StatusOK, "consent": "blocked", "reason": "dnc_list"}
	}
	return map[string]any{"status": StatusOK, "consent": "allowed"}
}

func defaultKnowledge() map[string]string {
	return map[string]string{
		"ip_phone": "Coral Telecom IP Phones are SIP desk phones for enterprise telephony, with HD voice, PoE, programmable keys and central provisioning from the Coral Call Server.",
		"media_gateway": "The Coral Media Gateway connects IP telephony to PRI, E1 and FXO or FXS trunks, with support for SIP, transcoding and survivable local switching.",
		"call_server": "The Coral Call Server is the enterprise IP PBX providing extension management, call routing, conferencing, voicemail integration and redundancy options.",
		"call_center": "Coral Call Center is the inbound and outbound contact centre suite with skill based routing, queues, agent desktop, supervisor monitoring and reporting.",
		"cloud_box":   "Coral Cloud Box is the compact cloud connected communication server for branch offices, bundling call control, voice mail and remote management.",
		"vms":         "Coral VMS is the voice mail and voice recording system with mailbox management, auto attendant menus and call recording retention controls.",
	}
}

// SetKnowledge overrides an answer (Admin console KB stub).
func (g *Gateway) SetKnowledge(product, answer string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.knowledge[strings.ToLower(strings.TrimSpace(product))] = answer
}

// Products lists knowledge keys.
func (g *Gateway) Products() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]string, 0, len(g.knowledge))
	for k := range g.knowledge {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func str(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.4f", t), "0"), ".")
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func strOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func strMap(v any) map[string]string {
	out := map[string]string{}
	m, ok := v.(map[string]any)
	if !ok {
		return out
	}
	for k, val := range m {
		if s := str(val); s != "" {
			out[k] = s
		}
	}
	return out
}

// Register adds the desk-skills gateway to the registry.
func Register(reg port.Registry, g *Gateway) (*Gateway, error) {
	if g == nil {
		g = New()
	}
	err := reg.Register(port.Registration{
		ID:           ID,
		Port:         port.PortSkill,
		Capabilities: g.Capabilities(),
		Instance:     g,
		Probe: func(ctx context.Context) port.Health {
			return port.Health{Healthy: true, LastOK: time.Now()}
		},
	})
	return g, err
}
