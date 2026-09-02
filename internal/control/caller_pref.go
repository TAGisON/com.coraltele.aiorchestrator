package control

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

const (
	prefSourceSTTLock  = "stt_lock"
	prefSourceOperator = "operator"
	prefSourceRestore  = "restore"
)

// normalizeANI keeps digits only so +91-98… and 9198… collide on one row.
func normalizeANI(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// callerANIFromJSON reads the FS/Lua "ani" (or common aliases) from session.caller.
func callerANIFromJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	for _, k := range []string{"ani", "caller_id_number", "from", "phone"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				if n := normalizeANI(s); n != "" {
					return n
				}
			}
		}
	}
	return ""
}

func (r *SessionRuntime) loadCallerPreference(ctx context.Context, tenantID, ani string) (store.CallerPreference, bool) {
	if r == nil || r.Repo == nil || tenantID == "" || ani == "" {
		return store.CallerPreference{}, false
	}
	p, err := r.Repo.GetCallerPreference(ctx, tenantID, ani)
	if errors.Is(err, store.ErrNotFound) {
		return store.CallerPreference{}, false
	}
	if err != nil {
		applog.Warn("load caller preference", "tenant", tenantID, "ani", ani, "err", err)
		return store.CallerPreference{}, false
	}
	if strings.TrimSpace(p.PreferredLanguage) == "" {
		return store.CallerPreference{}, false
	}
	return p, true
}

func (r *SessionRuntime) saveCallerPreference(tenantID, ani, lang, source string) {
	if r == nil || r.Repo == nil {
		return
	}
	ani = normalizeANI(ani)
	lang = strings.TrimSpace(lang)
	if tenantID == "" || ani == "" || lang == "" {
		return
	}
	_, err := r.Repo.UpsertCallerPreference(context.Background(), store.CallerPreference{
		TenantID:          tenantID,
		ANI:               ani,
		PreferredLanguage: lang,
		Source:            source,
	})
	if err != nil {
		applog.Warn("save caller preference", "tenant", tenantID, "ani", ani, "lang", lang, "err", err)
		return
	}
	applog.Info("caller preference saved", "tenant", tenantID, "ani", ani, "lang", lang, "source", source)
}
