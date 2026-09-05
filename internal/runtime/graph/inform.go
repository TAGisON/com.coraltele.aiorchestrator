package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// InformLookup resolves an Inform binding_ref + query to spoken answer text.
// Returns err on miss/disabled/unsupported — cursor fails closed to repair.
type InformLookup func(bindingRef, query, locale string) (answer string, err error)

// BindingInformLookup builds a live store-backed lookup (P2.10).
// allowRefs must be the published doc.binding_refs set.
func BindingInformLookup(repo store.Repository, tenantID string, allowRefs []string, defaultLocale string) InformLookup {
	allowed := make(map[string]struct{}, len(allowRefs))
	for _, r := range allowRefs {
		if s := strings.TrimSpace(r); s != "" {
			allowed[s] = struct{}{}
		}
	}
	return func(bindingRef, query, locale string) (string, error) {
		ref := strings.TrimSpace(bindingRef)
		if ref == "" {
			return "", fmt.Errorf("empty binding_ref")
		}
		if _, ok := allowed[ref]; !ok {
			return "", fmt.Errorf("binding_ref %q not allowlisted on flow", ref)
		}
		if repo == nil {
			return "", fmt.Errorf("binding store unavailable")
		}
		b, err := repo.GetBinding(context.Background(), ref)
		if err != nil {
			return "", fmt.Errorf("binding %q: %w", ref, err)
		}
		if b.TenantID != "" && tenantID != "" && b.TenantID != tenantID {
			return "", fmt.Errorf("binding %q tenant mismatch", ref)
		}
		if b.Kind != store.BindingKindKnowledge {
			return "", fmt.Errorf("binding %q kind %q not knowledge", ref, b.Kind)
		}
		if b.Status != store.BindingStatusActive {
			return "", fmt.Errorf("binding %q status %q", ref, b.Status)
		}
		return answerFromKnowledgeConfig(b.Config, query, locale, defaultLocale)
	}
}

type knowledgeConfig struct {
	Mode    string          `json:"mode"`
	Entries []inlineFAQEntry `json:"entries"`
}

type inlineFAQEntry struct {
	ID        string            `json:"id"`
	Questions []string          `json:"questions"`
	Text      map[string]string `json:"text"`
}

func answerFromKnowledgeConfig(raw json.RawMessage, query, locale, defaultLocale string) (string, error) {
	var cfg knowledgeConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("binding config: %w", err)
	}
	mode := strings.TrimSpace(cfg.Mode)
	switch mode {
	case "", "inline_faq":
		return matchInlineFAQ(cfg.Entries, query, locale, defaultLocale)
	case "http_retrieve":
		return "", fmt.Errorf("http_retrieve not available (fail closed)")
	default:
		return "", fmt.Errorf("unknown knowledge mode %q", mode)
	}
}

func matchInlineFAQ(entries []inlineFAQEntry, query, locale, defaultLocale string) (string, error) {
	q := normalize(query)
	if q == "" {
		return "", fmt.Errorf("empty FAQ query")
	}
	for _, e := range entries {
		hit := false
		for _, qu := range e.Questions {
			key := normalize(qu)
			if key == "" {
				continue
			}
			if q == key || strings.Contains(q, key) || strings.Contains(key, q) {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		if t := strings.TrimSpace(e.Text[locale]); t != "" {
			return t, nil
		}
		if t := strings.TrimSpace(e.Text[defaultLocale]); t != "" {
			return t, nil
		}
		return "", fmt.Errorf("FAQ entry %q missing locale text", e.ID)
	}
	return "", fmt.Errorf("no FAQ match")
}
