package desk

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// IntentClassifier maps caller text to a closed intent id via Think (P1 bridge).
type IntentClassifier func(ctx context.Context, d Doc, text, activeLang string) (intentID string, ok bool)

// LocaleSynthesizer renders canonical prompt text in active_language when no asset exists.
type LocaleSynthesizer func(ctx context.Context, canonical, activeLang string) (text string, ok bool)

var intentJSONRe = regexp.MustCompile(`(?i)"intent_id"\s*:\s*"([^"]+)"`)

// ClassifyIntentBridge runs phrase scoring first, then optional Think enum bridge.
func ClassifyIntentBridge(ctx context.Context, d Doc, text string, llm IntentClassifier, activeLang string) (intentID string, score float64) {
	id, score := ClassifyIntent(d, text)
	if score >= d.CX.IntentAcceptScore {
		return id, score
	}
	if llm == nil {
		return id, score
	}
	llmID, ok := llm(ctx, d, text, strings.TrimSpace(activeLang))
	if !ok {
		return id, score
	}
	llmID = strings.TrimSpace(llmID)
	if llmID == "" || llmID == "unclear" {
		return id, score
	}
	if in, found := d.IntentByID(llmID); found && in.Active {
		return llmID, d.CX.IntentAcceptScore
	}
	return id, score
}

// NewThinkIntentClassifier builds a closed-vocabulary intent classifier via Think.
func NewThinkIntentClassifier(th port.Think, sessionID port.SessionID) IntentClassifier {
	if th == nil {
		return nil
	}
	return func(ctx context.Context, d Doc, text, activeLang string) (string, bool) {
		text = strings.TrimSpace(text)
		if text == "" {
			return "", false
		}
		var ids []string
		for _, in := range d.Intents {
			if in.Active {
				ids = append(ids, in.ID)
			}
		}
		ids = append(ids, "unclear")
		sys := "Classify the caller utterance into exactly one intent_id from this closed list: " +
			strings.Join(ids, ", ") + ". Reply with JSON only: {\"intent_id\":\"...\"}."
		if activeLang != "" {
			sys += " Prefer intents that match speech in " + activeLang +
				". If the utterance is noise, echo, or a different script with no clear department request, return unclear."
		}
		res, err := th.Complete(ctx, port.ThinkRequest{
			SessionID: sessionID,
			Messages: []port.ChatMessage{
				{Role: "system", Content: sys},
				{Role: "user", Content: text},
			},
		})
		if err != nil {
			return "", false
		}
		id := parseIntentID(res.Text, d)
		return id, id != ""
	}
}

func parseIntentID(raw string, d Doc) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if m := intentJSONRe.FindStringSubmatch(raw); len(m) == 2 {
		raw = strings.TrimSpace(m[1])
	}
	raw = strings.Trim(raw, `"{} `)
	if raw == "unclear" {
		return "unclear"
	}
	if in, ok := d.IntentByID(raw); ok && in.Active {
		return raw
	}
	low := strings.ToLower(raw)
	for _, in := range d.Intents {
		if in.Active && strings.EqualFold(in.ID, low) {
			return in.ID
		}
	}
	return ""
}

// NewThinkLocaleSynthesizer renders canonical desk copy in active_language via Think.
func NewThinkLocaleSynthesizer(th port.Think, sessionID port.SessionID) LocaleSynthesizer {
	if th == nil {
		return nil
	}
	return func(ctx context.Context, canonical, activeLang string) (string, bool) {
		canonical = strings.TrimSpace(canonical)
		activeLang = strings.TrimSpace(activeLang)
		if canonical == "" || activeLang == "" {
			return "", false
		}
		if languagesMatch(canonical, activeLang) {
			return canonical, true
		}
		sys := "You render contact-center spoken prompts. Return only the final spoken text in " +
			activeLang + ", preserving meaning. No JSON, no explanation."
		if strings.EqualFold(activeLang, "hi-IN") || strings.EqualFold(activeLang, "hi") {
			sys += " For hi-IN use Hindi or Hinglish (Hindi mixed with English product terms)." +
				" Do not use Marathi, Bengali, or any other language."
		}
		res, err := th.Complete(ctx, port.ThinkRequest{
			SessionID: sessionID,
			Messages: []port.ChatMessage{
				{Role: "system", Content: sys},
				{Role: "user", Content: canonical},
			},
		})
		if err != nil {
			return "", false
		}
		out := strings.TrimSpace(res.Text)
		if out == "" {
			return "", false
		}
		// Strip accidental JSON wrappers from sloppy models.
		var wrap struct {
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(out), &wrap) == nil && strings.TrimSpace(wrap.Text) != "" {
			out = strings.TrimSpace(wrap.Text)
		}
		return out, true
	}
}
