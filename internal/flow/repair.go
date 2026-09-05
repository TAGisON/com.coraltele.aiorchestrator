package flow

import "encoding/json"

// RepairPolicy is the optional per-listen-node repair object (03 / G.5).
type RepairPolicy struct {
	MaxRetries       *int   `json:"max_retries"`
	UnclearPromptRef string `json:"unclear_prompt_ref"`
}

// ParseRepair unmarshals node.repair. Ok false when absent/empty.
func ParseRepair(raw json.RawMessage) (RepairPolicy, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return RepairPolicy{}, false, nil
	}
	var p RepairPolicy
	if err := json.Unmarshal(raw, &p); err != nil {
		return RepairPolicy{}, false, err
	}
	return p, true, nil
}

// EffectiveMaxRetries returns retries before on_exhausted (default 3 when policy present).
func (p RepairPolicy) EffectiveMaxRetries() int {
	if p.MaxRetries == nil {
		return 3
	}
	if *p.MaxRetries < 0 {
		return 0
	}
	return *p.MaxRetries
}
