package desk

import (
	"regexp"
	"strings"
	"unicode"
)

// Attribute keys the runtime and GUI both know (§6.5).
const (
	AttrDirection      = "direction"
	AttrDeskID         = "desk_id"
	AttrDeskVersion    = "desk_version"
	AttrProfileVersion = "profile_version"
	AttrPurpose        = "purpose"
	AttrLanguage       = "language"
	AttrANI            = "ani"
	AttrIntent         = "intent"
	AttrProduct        = "product"
	AttrProblem        = "problem"
	AttrImpact         = "impact"
	AttrErrorAlarm     = "error_alarm"
	AttrTroubleshoot   = "troubleshoot_notes"
	AttrCustomerName   = "customer_name"
	AttrCustomerEmail  = "customer_email"
	AttrCustomerPhone  = "customer_phone"
	AttrTicketID       = "ticket_id"
	AttrEmailSent      = "email_sent"
	AttrPriority       = "priority"
	AttrTransferTarget = "transfer_target"
	AttrTransferNumber = "transfer_number"
	AttrDisposition    = "disposition"
	AttrSummary        = "summary"
	AttrConsent        = "consent"
	AttrRequirement    = "requirement"
	AttrEnquiryID      = "enquiry_id"
	AttrCallbackID     = "callback_id"
)

// PIIClass per attribute (§6.5). Missing key = "none".
var PIIClass = map[string]string{
	AttrANI:           "confidential",
	AttrCustomerName:  "confidential",
	AttrCustomerEmail: "confidential",
	AttrCustomerPhone: "confidential",
	AttrConsent:       "confidential",
	AttrTicketID:      "internal",
	AttrTroubleshoot:  "internal",
	AttrSummary:       "internal",
}

// ClassOf returns the PII class for an attribute key.
func ClassOf(key string) string {
	if c, ok := PIIClass[key]; ok {
		return c
	}
	return "none"
}

// Mask hides confidential values for default reads (§11).
func Mask(key, value string) string {
	if value == "" {
		return ""
	}
	switch ClassOf(key) {
	case "confidential":
		switch key {
		case AttrCustomerEmail:
			at := strings.Index(value, "@")
			if at > 1 {
				return value[:1] + "***" + value[at:]
			}
			return "***"
		case AttrCustomerPhone, AttrANI:
			if len(value) > 4 {
				return "*******" + value[len(value)-4:]
			}
			return "***"
		default:
			r := []rune(value)
			if len(r) > 1 {
				return string(r[:1]) + strings.Repeat("*", len(r)-1)
			}
			return "*"
		}
	default:
		return value
	}
}

var (
	emailRe      = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`)
	phoneDigitRe = regexp.MustCompile(`\d`)
	// \p{M} keeps Devanagari vowel signs: without it "हाँ" collapses to "हा" and
	// every Hindi yes/no, product and option match fails.
	nonWordRe = regexp.MustCompile(`[^\p{L}\p{N}\p{M}\s@.+-]+`)
	spaceRe      = regexp.MustCompile(`\s+`)
)

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonWordRe.ReplaceAllString(s, " ")
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func tokens(s string) []string {
	n := normalize(s)
	if n == "" {
		return nil
	}
	return strings.Fields(n)
}

// HasDevanagari reports whether text contains Devanagari script.
func HasDevanagari(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Devanagari, r) {
			return true
		}
	}
	return false
}

var romanHindiMarkers = []string{
	"chahiye", "kijiye", "kijie", "karo", "karna", "kripya", "mujhe", "mera", "meri", "aap",
	"nahi", "nahin", "haan", "hai", "hoga", "kya", "kaise", "samasya", "shikayat", "madad",
	"baat", "bataiye", "theek", "bandh", "chalu", "kaam", "nahí",
}

// englishMarkers are function words that only appear in real English sentences.
// An email address or a name is not evidence of a language.
var englishMarkers = []string{
	"the", "is", "are", "am", "was", "not", "no", "yes", "and", "or", "but", "with",
	"my", "your", "our", "i", "we", "you", "it", "this", "that", "please", "want",
	"need", "can", "cannot", "have", "has", "do", "does", "there", "working", "issue",
	"problem", "call", "calls", "phone", "help", "for", "to", "of", "in", "on", "all",
}

func langPair(allowed []string) (hi, en string) {
	for _, l := range allowed {
		switch baseLang(l) {
		case "hi":
			hi = l
		case "en":
			en = l
		}
	}
	if hi == "" {
		hi = "hi-IN"
	}
	if en == "" {
		en = "en-IN"
	}
	return hi, en
}

// DetectLanguage picks a session language from caller text (§13 multilingual rule).
// Returns empty when the text carries no signal.
func DetectLanguage(text string, allowed []string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	hi, en := langPair(allowed)
	if HasDevanagari(text) {
		return hi
	}
	toks := tokens(text)
	for _, t := range toks {
		for _, m := range romanHindiMarkers {
			if t == m {
				return hi
			}
		}
	}
	if len(toks) > 0 {
		return en
	}
	return ""
}

// SwitchLanguageEvidence reports the language a mid-call utterance proves the
// caller moved to, or "" to stay on the locked language. Unlike DetectLanguage it
// never reads a slot answer such as "suresh@coral.com" or "Ramesh Kumar" as a
// switch to English (§17: the caller switches, not the transcript encoding).
func SwitchLanguageEvidence(text string, allowed []string, current string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	hi, en := langPair(allowed)
	if HasDevanagari(text) {
		if current == hi {
			return ""
		}
		return hi
	}
	toks := tokens(text)
	for _, t := range toks {
		for _, m := range romanHindiMarkers {
			if t == m {
				if current == hi {
					return ""
				}
				return hi
			}
		}
	}
	if current == en {
		return ""
	}
	hits := 0
	for _, t := range toks {
		for _, m := range englishMarkers {
			if t == m {
				hits++
				break
			}
		}
	}
	if hits >= 2 && len(toks) >= 3 {
		return en
	}
	return ""
}

// LanguageSwitchRequest detects an explicit caller request to change language.
func LanguageSwitchRequest(text string, allowed []string) string {
	n := normalize(text)
	if n == "" {
		return ""
	}
	hi, en := "", ""
	for _, l := range allowed {
		switch baseLang(l) {
		case "hi":
			hi = l
		case "en":
			en = l
		}
	}
	wantsEnglish := strings.Contains(n, "english") || strings.Contains(n, "angrezi") || strings.Contains(n, "अंग्रेज")
	wantsHindi := strings.Contains(n, "hindi") || strings.Contains(n, "हिंदी") || strings.Contains(n, "हिन्दी")
	if wantsEnglish && en != "" {
		return en
	}
	if wantsHindi && hi != "" {
		return hi
	}
	return ""
}

var yesWords = []string{"yes", "yeah", "yep", "correct", "right", "sure", "ok", "okay", "confirm", "please do",
	"haan", "haa", "ha", "ji", "ji haan", "sahi", "theek", "thik", "bilkul", "हाँ", "हां", "जी", "सही", "ठीक", "बिल्कुल"}

var noWords = []string{"no", "nope", "not correct", "incorrect", "wrong", "dont", "do not", "cancel",
	"nahi", "nahin", "galat", "नहीं", "नही", "गलत"}

// YesNo classifies an affirmative / negative answer in EN or HI. ok=false when unclear.
func YesNo(text string) (yes bool, ok bool) {
	n := normalize(text)
	if n == "" {
		return false, false
	}
	for _, w := range noWords {
		if n == w || strings.HasPrefix(n, w+" ") || strings.Contains(n, " "+w+" ") || strings.HasSuffix(n, " "+w) {
			return false, true
		}
	}
	for _, w := range yesWords {
		if n == w || strings.HasPrefix(n, w+" ") || strings.Contains(n, " "+w+" ") || strings.HasSuffix(n, " "+w) {
			return true, true
		}
	}
	return false, false
}

// humanPhrases are unambiguous asks to be put through to a person (§14.3 rule 3).
var humanPhrases = []string{
	"transfer me", "transfer my call", "connect me", "put me through", "talk to",
	"speak to", "speak with", "let me speak", "i want to speak", "need an engineer",
	"send an engineer", "call an agent", "get me someone", "human please",
	"baat karao", "baat karwao", "baat karwa", "connect karo", "transfer karo",
	"बात कराओ", "बात कराइए", "ट्रांसफर", "जोड़ दीजिए", "जोड़ दो",
}

// humanNouns only mean "route me out" when the caller says little else — otherwise
// "all agents are logged out" would leave the guided path mid-troubleshooting.
var humanNouns = []string{
	"agent", "agents", "human", "person", "representative", "executive", "engineer",
	"someone", "somebody", "transfer", "एजेंट", "व्यक्ति", "इंजीनियर",
}

// humanFillers are words that may surround a bare "agent" without turning the
// utterance into a description of the problem.
var humanFillers = []string{
	"i", "me", "my", "a", "an", "the", "to", "with", "please", "want", "need", "give",
	"get", "put", "through", "now", "kindly", "sir", "madam", "call", "human",
	"mujhe", "chahiye", "se", "ko", "karo", "kripya", "मुझे", "चाहिए", "से", "को", "कृपया",
}

// bareHumanRequest is true when the whole utterance is a request for a person and
// nothing else: "agent please" routes out, "agents cannot log in" does not.
func bareHumanRequest(text string) bool {
	toks := tokens(text)
	if len(toks) == 0 || len(toks) > 4 {
		return false
	}
	found := false
	for _, t := range toks {
		isNoun := false
		for _, noun := range humanNouns {
			if t == noun {
				isNoun, found = true, true
				break
			}
		}
		if isNoun {
			continue
		}
		isFiller := false
		for _, f := range humanFillers {
			if t == f {
				isFiller = true
				break
			}
		}
		if !isFiller {
			return false
		}
	}
	return found
}

// HumanRequest detects an explicit ask for a human agent (system law §14.3).
func HumanRequest(text string) bool {
	n := normalize(text)
	if n == "" {
		return false
	}
	for _, p := range humanPhrases {
		if strings.Contains(n, normalize(p)) {
			return true
		}
	}
	return bareHumanRequest(text)
}

// CriticalRequest detects a critical outage (§15 of the Coral script).
var criticalPhrases = []string{
	"entire system down", "whole system down", "system is down", "all users", "everyone",
	"all locations", "multiple locations", "complete outage", "major outage", "total outage",
	"nothing is working", "no calls", "call center down", "emergency",
	"pura system band", "sab band", "sabhi", "poora system", "पूरा सिस्टम", "सब बंद", "आपातकाल",
}

func CriticalRequest(text string) bool {
	n := normalize(text)
	if n == "" {
		return false
	}
	for _, p := range criticalPhrases {
		if strings.Contains(n, p) {
			return true
		}
	}
	return false
}

// ProductVocabulary maps caller words to the closed product catalog (§6.4).
var ProductVocabulary = map[string][]string{
	"ip_phone":      {"ip phone", "ipphone", "phone", "handset", "desk phone", "आईपी फोन", "फोन", "telephone"},
	"media_gateway": {"media gateway", "gateway", "मीडिया गेटवे", "गेटवे"},
	"call_server":   {"call server", "server", "कॉल सर्वर", "pbx", "epabx"},
	"call_center":   {"call center", "call centre", "contact center", "कॉल सेंटर", "acd"},
	"cloud_box":     {"cloud box", "cloudbox", "cloud", "क्लाउड"},
	"vms":           {"vms", "voice mail", "voicemail", "वॉइस मेल", "वीएमएस"},
}

// MatchProduct resolves a product id from caller text.
func MatchProduct(text string) (string, bool) {
	n := normalize(text)
	if n == "" {
		return "", false
	}
	best, bestLen := "", 0
	for id, words := range ProductVocabulary {
		for _, w := range words {
			if strings.Contains(n, normalize(w)) && len(w) > bestLen {
				best, bestLen = id, len(w)
			}
		}
	}
	if best != "" {
		return best, true
	}
	if strings.Contains(n, "other") || strings.Contains(n, "अन्य") || strings.Contains(n, "dusra") {
		return "other", true
	}
	return "", false
}

// displayVocabulary maps machine slot values to spoken words per locale, so the
// caller hears "IP Phone" while attributes and skills keep the stable id (§6.5).
var displayVocabulary = map[string]map[string]map[string]string{
	AttrProduct: {
		"ip_phone":      {"en-IN": "IP Phone", "hi-IN": "आईपी फोन"},
		"media_gateway": {"en-IN": "Media Gateway", "hi-IN": "मीडिया गेटवे"},
		"call_server":   {"en-IN": "Call Server", "hi-IN": "कॉल सर्वर"},
		"call_center":   {"en-IN": "Call Center", "hi-IN": "कॉल सेंटर"},
		"cloud_box":     {"en-IN": "Cloud Box", "hi-IN": "क्लाउड बॉक्स"},
		"vms":           {"en-IN": "VMS", "hi-IN": "वीएमएस"},
		"other":         {"en-IN": "the product you described", "hi-IN": "आपके बताए उत्पाद"},
	},
	AttrImpact: {
		"single_user":    {"en-IN": "a single user", "hi-IN": "एक उपयोगकर्ता"},
		"multiple_users": {"en-IN": "multiple users", "hi-IN": "कई उपयोगकर्ता"},
		"entire_system":  {"en-IN": "the entire system", "hi-IN": "पूरे सिस्टम"},
	},
	AttrPriority: {
		"critical": {"en-IN": "critical", "hi-IN": "गंभीर"},
		"normal":   {"en-IN": "normal", "hi-IN": "सामान्य"},
	},
}

// DisplayValue returns the spoken form of an attribute value for a locale.
// Unknown keys and free-text values pass through unchanged.
func DisplayValue(key, value, locale string) string {
	byValue, ok := displayVocabulary[key]
	if !ok {
		return value
	}
	byLocale, ok := byValue[value]
	if !ok {
		return value
	}
	if s := strings.TrimSpace(byLocale[locale]); s != "" {
		return s
	}
	for _, l := range []string{locale, "en-IN"} {
		for k, v := range byLocale {
			if baseLang(k) == baseLang(l) && strings.TrimSpace(v) != "" {
				return v
			}
		}
	}
	return value
}

var spokenEmail = strings.NewReplacer(
	" at the rate ", "@", " at rate ", "@", " at ", "@",
	" dot ", ".", " underscore ", "_", " dash ", "-", " hyphen ", "-",
	" @ ", "@", " @", "@", "@ ", "@", " . ", ".", " .", ".", ". ", ".",
)

// parseEmail reads an address the caller either typed or spelled out loud
// ("ramesh at coral dot com") without swallowing the words around it.
func parseEmail(text string) (string, bool) {
	if m := emailRe.FindString(text); m != "" {
		return m, true
	}
	spoken := spokenEmail.Replace(strings.ToLower(text))
	for _, field := range strings.Fields(spoken) {
		if m := emailRe.FindString(field); m != "" {
			return m, true
		}
	}
	if m := emailRe.FindString(strings.ReplaceAll(spoken, " ", "")); m != "" {
		return m, true
	}
	return "", false
}

// ValidateSlot checks and normalizes a slot answer. ok=false → no-match repair.
func ValidateSlot(kind, text string) (value string, ok bool) {
	t := strings.TrimSpace(text)
	if t == "" {
		return "", false
	}
	switch kind {
	case ValidateEmail:
		return parseEmail(t)
	case ValidatePhone:
		digits := strings.Join(phoneDigitRe.FindAllString(t, -1), "")
		if len(digits) >= 7 {
			return digits, true
		}
		return "", false
	case ValidateNumber:
		digits := strings.Join(phoneDigitRe.FindAllString(t, -1), "")
		if digits == "" {
			return "", false
		}
		return digits, true
	case ValidateProduct:
		if id, hit := MatchProduct(t); hit {
			return id, true
		}
		return "", false
	case ValidateYesNo:
		yes, hit := YesNo(t)
		if !hit {
			return "", false
		}
		if yes {
			return "yes", true
		}
		return "no", true
	default:
		if len([]rune(t)) < 2 {
			return "", false
		}
		return t, true
	}
}

// ScoreIntent returns 0..1 for how well text matches an intent's example phrases.
func ScoreIntent(in Intent, text string) float64 {
	n := normalize(text)
	if n == "" {
		return 0
	}
	userTokens := map[string]bool{}
	for _, t := range tokens(text) {
		userTokens[t] = true
	}
	best := 0.0
	for _, list := range in.Phrases {
		for _, phrase := range list {
			p := normalize(phrase)
			if p == "" {
				continue
			}
			if strings.Contains(n, p) {
				score := 1.0
				if len(p) < 4 {
					score = 0.8
				}
				if score > best {
					best = score
				}
				continue
			}
			pt := tokens(phrase)
			if len(pt) == 0 {
				continue
			}
			matched := 0
			for _, t := range pt {
				if userTokens[t] {
					matched++
				}
			}
			if matched == 0 {
				continue
			}
			score := float64(matched) / float64(len(pt))
			if score > best {
				best = score
			}
		}
	}
	return best
}

// ClassifyIntent picks the best active intent and reports the runner-up score.
func ClassifyIntent(d Doc, text string) (intentID string, score float64) {
	for _, in := range d.Intents {
		if !in.Active {
			continue
		}
		s := ScoreIntent(in, text)
		if s > score {
			intentID, score = in.ID, s
		}
	}
	return intentID, score
}

// RenderTemplate substitutes {{attribute}} placeholders (§14 summaries).
func RenderTemplate(text string, attrs map[string]string) string {
	if text == "" {
		return ""
	}
	out := text
	for k, v := range attrs {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	out = regexp.MustCompile(`\{\{[a-z_]+\}\}`).ReplaceAllString(out, "")
	return strings.TrimSpace(spaceRe.ReplaceAllString(out, " "))
}
