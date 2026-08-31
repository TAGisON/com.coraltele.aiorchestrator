package desk

import (
	"fmt"
	"sort"
	"strings"
)

// CheckItem is one publish-checklist row shown in the Configurator GUI (§16).
type CheckItem struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
	Blocker bool   `json:"blocker"`
}

// CheckResult is the full checklist plus a publishable verdict.
type CheckResult struct {
	Items      []CheckItem `json:"items"`
	Publishable bool       `json:"publishable"`
	Warnings   []string    `json:"warnings,omitempty"`
}

// Validate runs the publish checklist and structural compiler gates (§16, §6.9).
func Validate(d Doc) CheckResult {
	d.Normalize()
	var items []CheckItem
	var warnings []string

	add := func(id, label string, ok bool, blocker bool, detail string) {
		items = append(items, CheckItem{ID: id, Label: label, OK: ok, Blocker: blocker, Detail: detail})
	}

	add("name", "Desk name set", strings.TrimSpace(d.Name) != "", true, "")
	add("purpose", "Purpose set", validPurpose(d.Purpose), true, "purpose must be one of "+strings.Join(Purposes, ", "))
	add("direction", "Direction set", d.Direction == DirectionInbound || d.Direction == DirectionOutbound, true, "")

	welcome := d.PromptText(PromptWelcome, d.DefaultLanguage)
	add("welcome", "Welcome prompt set", strings.TrimSpace(welcome) != "", true, "")

	missingLocales := missingPromptLocales(d)
	if len(missingLocales) > 0 {
		warnings = append(warnings, "prompts missing translations: "+strings.Join(missingLocales, ", ")+" (runtime falls back to "+d.DefaultLanguage+")")
	}
	add("languages", "At least one language configured", len(d.Languages) > 0, true, "")

	activeIntents := 0
	intentsWithPhrases := 0
	intentsWithPaths := 0
	var intentDetail []string
	for _, in := range d.Intents {
		if !in.Active {
			continue
		}
		activeIntents++
		phrases := 0
		for _, list := range in.Phrases {
			phrases += len(list)
		}
		if phrases > 0 {
			intentsWithPhrases++
		} else {
			intentDetail = append(intentDetail, in.ID+": no example phrases")
		}
		if p, ok := d.Paths[in.PathID]; ok && len(p.Steps) > 0 {
			intentsWithPaths++
		} else {
			intentDetail = append(intentDetail, in.ID+": no guided path")
		}
	}
	add("intents_active", "At least one active intent", activeIntents > 0, true, "")
	add("intent_phrases", "Every active intent has example phrases", activeIntents > 0 && intentsWithPhrases == activeIntents, true, strings.Join(intentDetail, "; "))
	add("intent_paths", "Every active intent has a guided path", activeIntents > 0 && intentsWithPaths == activeIntents, true, "")

	structural := StructuralErrors(d)
	add("paths_valid", "Guided paths resolve (no dangling steps)", len(structural) == 0, true, strings.Join(structural, "; "))

	matrixOK := true
	var matrixDetail []string
	for _, in := range d.Intents {
		if !in.Active {
			continue
		}
		if _, ok := d.MatrixFor(in.ID); !ok {
			matrixOK = false
			matrixDetail = append(matrixDetail, in.ID)
		}
	}
	add("matrix", "Routing matrix covers active intents", matrixOK, true, strings.Join(matrixDetail, ", "))

	ticket := d.Skills["create_service_complaint"]
	transfer := d.Skills["transfer_to_queue"]
	add("actions", "Ticket skill enabled or transfer-only explicit", ticket.Enabled || transfer.Enabled, true,
		"enable create_service_complaint or transfer_to_queue")

	silenceOK := strings.TrimSpace(d.PromptText(PromptSilence1, d.DefaultLanguage)) != "" &&
		strings.TrimSpace(d.PromptText(PromptSilenceGoodbye, d.DefaultLanguage)) != "" &&
		d.CX.SilenceNudge1Ms > 0
	add("silence", "Silence prompts and timeouts set", silenceOK, true, "")

	kbNeeded := false
	for _, p := range d.Paths {
		for _, s := range p.Steps {
			if s.Skill == "search_knowledge" {
				kbNeeded = true
			}
		}
	}
	add("knowledge", "Knowledge pack attached when a path answers from KB", !kbNeeded || len(d.Knowledge) > 0, true, "")

	voiceOK := strings.TrimSpace(d.VoiceID) != "" || len(d.Voice) > 0
	add("voice", "Voice set for Speak", voiceOK, true, "")

	add("clarify", "Fallback clarify prompt set", strings.TrimSpace(d.PromptText(PromptClarify, d.DefaultLanguage)) != "", true, "")

	if d.Direction == DirectionOutbound {
		consentOK := d.Consent != nil && (!d.Consent.Required || strings.TrimSpace(d.Consent.Skill) != "")
		add("consent", "Outbound consent policy set", consentOK, true, "outbound desks must declare a consent decision")
	}

	res := CheckResult{Items: items, Warnings: warnings, Publishable: true}
	for _, it := range items {
		if it.Blocker && !it.OK {
			res.Publishable = false
		}
	}
	return res
}

func validPurpose(p string) bool {
	for _, v := range Purposes {
		if v == p {
			return true
		}
	}
	return false
}

func missingPromptLocales(d Doc) []string {
	var out []string
	ids := make([]string, 0, len(d.Prompts))
	for id := range d.Prompts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := d.Prompts[id]
		for _, lang := range d.Languages {
			if strings.TrimSpace(p.Text[lang]) == "" && strings.TrimSpace(p.WavRef[lang]) == "" {
				out = append(out, id+"/"+lang)
			}
		}
	}
	return out
}

// StructuralErrors reports dangling references that must fail publish (§6.9 compiler law).
func StructuralErrors(d Doc) []string {
	d.Normalize()
	var errs []string
	pathIDs := make([]string, 0, len(d.Paths))
	for id := range d.Paths {
		pathIDs = append(pathIDs, id)
	}
	sort.Strings(pathIDs)

	for _, pid := range pathIDs {
		p := d.Paths[pid]
		steps := map[string]Step{}
		for _, s := range p.Steps {
			if s.ID == "" {
				errs = append(errs, fmt.Sprintf("%s: step with empty id", pid))
				continue
			}
			if _, dup := steps[s.ID]; dup {
				errs = append(errs, fmt.Sprintf("%s: duplicate step id %s", pid, s.ID))
			}
			steps[s.ID] = s
		}
		if p.Entry != "" {
			if _, ok := steps[p.Entry]; !ok {
				errs = append(errs, fmt.Sprintf("%s: entry step %s missing", pid, p.Entry))
			}
		}
		ref := func(from, target, kind string) {
			if strings.TrimSpace(target) == "" {
				return
			}
			if isCrossPathRef(target) {
				name := strings.TrimPrefix(target, "path:")
				if _, ok := d.Paths[name]; !ok {
					errs = append(errs, fmt.Sprintf("%s.%s: %s references unknown path %s", pid, from, kind, name))
				}
				return
			}
			if _, ok := steps[target]; !ok {
				errs = append(errs, fmt.Sprintf("%s.%s: %s references unknown step %s", pid, from, kind, target))
			}
		}
		for i, s := range p.Steps {
			switch s.Type {
			case StepSay, StepAsk, StepConfirm, StepChoice, StepAction, StepEnd:
			default:
				errs = append(errs, fmt.Sprintf("%s.%s: unknown step type %q", pid, s.ID, s.Type))
				continue
			}
			ref(s.ID, s.Next, "next")
			ref(s.ID, s.OnYes, "on_yes")
			ref(s.ID, s.OnNo, "on_no")
			for _, o := range s.Options {
				ref(s.ID, o.Next, "option "+o.ID)
			}
			for br, target := range s.Branches {
				switch br {
				case BranchOK, BranchFail, BranchDuplicate, BranchTimeout, BranchUnavailable:
				default:
					errs = append(errs, fmt.Sprintf("%s.%s: unknown branch %q", pid, s.ID, br))
				}
				ref(s.ID, target, "branch "+br)
			}
			switch s.Type {
			case StepSay:
				if s.PromptID == "" {
					errs = append(errs, fmt.Sprintf("%s.%s: Say needs prompt_id", pid, s.ID))
				}
			case StepAsk:
				if s.SlotKey == "" {
					errs = append(errs, fmt.Sprintf("%s.%s: Ask needs slot_key", pid, s.ID))
				}
				if s.PromptID == "" {
					errs = append(errs, fmt.Sprintf("%s.%s: Ask needs prompt_id", pid, s.ID))
				}
			case StepConfirm:
				if s.SummaryPromptID == "" && s.PromptID == "" {
					errs = append(errs, fmt.Sprintf("%s.%s: Confirm needs summary_prompt_id", pid, s.ID))
				}
			case StepChoice:
				if len(s.Options) == 0 {
					errs = append(errs, fmt.Sprintf("%s.%s: Choice needs options", pid, s.ID))
				}
			case StepAction:
				if s.Skill == "" {
					errs = append(errs, fmt.Sprintf("%s.%s: Action needs skill", pid, s.ID))
				} else if b, ok := d.Skills[s.Skill]; !ok || !b.Enabled {
					errs = append(errs, fmt.Sprintf("%s.%s: skill %s not enabled on this desk", pid, s.ID, s.Skill))
				}
			}
			// Non-terminal steps must reach somewhere.
			if s.Type != StepEnd && s.Next == "" && s.OnYes == "" && len(s.Options) == 0 && len(s.Branches) == 0 {
				if i == len(p.Steps)-1 {
					errs = append(errs, fmt.Sprintf("%s.%s: last step is not End and has no next", pid, s.ID))
				}
			}
			for _, id := range []string{s.PromptID, s.RepromptID, s.SummaryPromptID, s.ClosingPromptID} {
				if strings.TrimSpace(id) == "" {
					continue
				}
				if _, ok := d.Prompts[id]; !ok {
					errs = append(errs, fmt.Sprintf("%s.%s: unknown prompt %s", pid, s.ID, id))
				}
			}
		}
	}

	for _, in := range d.Intents {
		if !in.Active {
			continue
		}
		if in.PathID == "" {
			errs = append(errs, "intent "+in.ID+": no path_id")
			continue
		}
		if _, ok := d.Paths[in.PathID]; !ok {
			errs = append(errs, "intent "+in.ID+": unknown path "+in.PathID)
		}
	}
	return errs
}

func isCrossPathRef(target string) bool {
	return strings.HasPrefix(target, "path:")
}
