package desk

// PresetCoralXfer is a minimal inbound transfer desk: welcome → language (new
// callers) → department menu → live matrix transfer or one-line domain FAQ.
// Canonical prompts are English; runtime speaks Hinglish/hi-IN by default and
// synthesizes other allowlisted languages after preference lock.
func PresetCoralXfer(tenantID string) Doc {
	welcomeBarge := false
	localeSynth := true
	d := Doc{
		SchemaVersion:   SchemaVersion,
		ID:              "coral-xfer",
		TenantID:        tenantID,
		Name:            "Coral Telecom Simple Transfer",
		Direction:       DirectionInbound,
		Purpose:         "support",
		Languages:       []string{"en-IN"},
		DefaultLanguage: "hi-IN", // Hinglish-first default speak/lookup fallback
		Tone:            "professional",
		VoiceID:         "priya",
		Voice:           map[string]string{"sarvam-tts": "priya"},
		CX: CXPolicy{
			BargeIn:                true,
			ListenWhileSpeak:       true,
			SilenceNudge1Ms:        9000,
			SilenceNudge2Ms:        10000,
			SilenceHangupMs:        12000,
			AskTimeoutMs:           9000,
			MaxRetries:             2,
			MaxTurnFailures:        3,
			IntentAcceptScore:      0.55,
			IntentConfirmScore:     0.40,
			WelcomeBargeAllowed:    &welcomeBarge,
			MinBargeChars:          3,
			MinBargeMs:             280,
			BargePartialConfidence: 0.70,
			RTPSettleMs:            400,
			LocaleSynthesis:        &localeSynth,
			PrimaryLocale:          "en-IN",
			RuntimeLanguages:       append([]string(nil), IndiaDefaultLanguages...),
		},
		Prompts: xferPrompts(),
		Intents: xferIntents(),
		Paths:   xferPaths(),
		Matrix:  xferMatrix(),
		Skills: map[string]SkillBind{
			"transfer_to_queue": {Enabled: true, Mode: "stub", Gateway: DefaultSkillGateway},
			"search_knowledge":  {Enabled: true, Mode: "stub", Gateway: DefaultSkillGateway},
		},
		Knowledge: []KnowledgeAttach{
			{Collection: "coral-products", Intents: []string{"domain_faq"}},
		},
		Retention: &RetentionOverride{TranscriptDays: 90, AttributesDays: 90, AuditDays: 365, RecordingDays: 30},
	}
	d.Normalize()
	d.CX.WelcomeBargeAllowed = &welcomeBarge
	d.CX.LocaleSynthesis = &localeSynth
	d.CX.PrimaryLocale = "en-IN"
	d.CX.RuntimeLanguages = append([]string(nil), IndiaDefaultLanguages...)
	d.DefaultLanguage = "hi-IN"
	return d
}

func xferPrompt(id, label, en string) Prompt {
	return Prompt{
		ID: id, Label: label, Media: "text_tts",
		Text: map[string]string{"en-IN": en},
	}
}

func xferPrompts() map[string]Prompt {
	list := []Prompt{
		xferPrompt(PromptWelcome, "Welcome",
			"Thank you for calling Coral Telecom."),
		xferPrompt("ask_language", "Ask preferred language",
			"Which language would you prefer for this call? For example Hindi, English, or another Indian language."),
		xferPrompt("language_confirmed", "Language confirmed",
			"Understood. I will continue in your preferred language."),
		xferPrompt(PromptClarify, "Department menu",
			"I can transfer you to Sales, Corporate, or Support. Which department do you need, or how can I help?"),
		xferPrompt("clarify_2", "Clarify again",
			"Please say Sales, Corporate, or Support, or ask a short question about Coral Telecom products."),
		xferPrompt("ood_hangup", "Out of scope hangup",
			"I can only help with Coral Telecom Sales, Corporate, or Support. Since I could not understand your request after several tries, I am ending this call. Goodbye."),
		xferPrompt(PromptSilence1, "Silence 1",
			"Are you still on the line? I am here to help with Sales, Corporate, or Support."),
		xferPrompt(PromptSilence2, "Silence 2",
			"I have not heard a response. Please say Sales, Corporate, Support, or briefly describe what you need."),
		xferPrompt(PromptSilenceGoodbye, "Silence goodbye",
			"I am ending the call because there was no response. Thank you for calling Coral Telecom. Goodbye."),
		xferPrompt("abuse_hangup", "Abuse hangup",
			"I cannot continue this call due to inappropriate language. Goodbye."),
		xferPrompt(PromptClosing, "Closing",
			"Thank you for calling Coral Telecom. Goodbye."),
		xferPrompt(PromptHold, "Hold",
			"Please hold while I transfer your call."),
		xferPrompt(PromptSystemDown, "System down",
			"I am unable to transfer your call right now due to a system issue. Please try again shortly. Goodbye."),
		xferPrompt("ack_sales", "Ack sales",
			"Connecting you to Sales now."),
		xferPrompt("ack_corporate", "Ack corporate",
			"Connecting you to Corporate now."),
		xferPrompt("ack_support", "Ack support",
			"Connecting you to Support now."),
		xferPrompt("transfer_done", "Transfer done",
			"You are being connected now. Thank you for calling Coral Telecom."),
		xferPrompt("domain_answer", "Domain FAQ",
			"{{kb_answer}}"),
		xferPrompt("domain_no_info", "No FAQ",
			"I do not have approved details for that. I can transfer you to Sales, Corporate, or Support."),
		xferPrompt(PromptAnythingElse, "Anything else",
			"Is there anything else I can help you with?"),
	}
	out := map[string]Prompt{}
	for _, p := range list {
		out[p.ID] = p
	}
	return out
}

func xferIntents() []Intent {
	return []Intent{
		{
			ID: "sales", Display: "Sales", Active: true, PathID: "path_sales",
			Phrases: map[string][]string{
				"en-IN": {"sales", "sale", "quotation", "quote", "buy", "purchase", "pricing", "price", "order"},
				"hi-IN": {"sales", "सेल्स", "खरीदना", "कोटेशन", "कीमत", "order"},
			},
		},
		{
			ID: "corporate", Display: "Corporate", Active: true, PathID: "path_corporate",
			Phrases: map[string][]string{
				"en-IN": {"corporate", "enterprise", "company account", "business", "office"},
				"hi-IN": {"corporate", "कॉर्पोरेट", "बिज़नेस", "कंपनी"},
			},
		},
		{
			ID: "support", Display: "Support", Active: true, PathID: "path_support",
			Phrases: map[string][]string{
				"en-IN": {"support", "technical support", "tech support", "help desk", "complaint", "not working", "issue", "problem"},
				"hi-IN": {"support", "सपोर्ट", "तकनीकी", "शिकायत", "समस्या", "काम नहीं"},
			},
		},
		{
			ID: "domain_faq", Display: "Product FAQ", Active: true, PathID: "path_faq",
			Phrases: map[string][]string{
				"en-IN": {"what is", "tell me about", "product", "ip phone", "media gateway", "call center", "call server", "vms", "cloud box"},
				"hi-IN": {"क्या है", "बताओ", "प्रोडक्ट", "आईपी फोन", "मीडिया गेटवे"},
			},
		},
	}
}

func xferMatrix() []MatrixRow {
	// Lab dial destinations — Configurator may change Extension per site.
	return []MatrixRow{
		{Intent: "sales", Owner: "Sales Desk", Target: "sales", Number: "5002", Action: "transfer"},
		{Intent: "corporate", Owner: "Corporate Desk", Target: "corporate", Number: "5003", Action: "transfer"},
		{Intent: "support", Owner: "Support Desk", Target: "support", Number: "5004", Action: "transfer"},
		{Intent: "domain_faq", Owner: "Sales Desk", Target: "sales", Number: "5002", Action: "transfer"},
	}
}

func xferPaths() map[string]Path {
	xfer := func(id, ackPrompt, target, owner, disposition string) Path {
		return Path{
			ID: id, Label: id, Entry: "ack",
			Steps: []Step{
				say("ack", ackPrompt, "do_xfer"),
				actionStep("do_xfer", "transfer_to_queue",
					map[string]string{"target": "=" + target, "owner": "=" + owner},
					map[string]string{BranchOK: "done", BranchFail: "fail", BranchTimeout: "fail", BranchUnavailable: "fail"}),
				endStep("done", "transfer_done", disposition, false),
				endStep("fail", PromptSystemDown, DispSystemFailure, false),
			},
		}
	}
	return map[string]Path{
		"path_sales":     xfer("path_sales", "ack_sales", "sales", "Sales Desk", DispTransferredSales),
		"path_corporate": xfer("path_corporate", "ack_corporate", "corporate", "Corporate Desk", DispTransferredCorporate),
		"path_support":   xfer("path_support", "ack_support", "support", "Support Desk", DispTransferredTech),
		"path_faq": {
			ID: "path_faq", Label: "Domain FAQ", Entry: "search",
			Steps: []Step{
				actionStep("search", "search_knowledge",
					map[string]string{"query": AttrIntent, "product": AttrProduct, "language": AttrLanguage},
					map[string]string{BranchOK: "answer", BranchFail: "noinfo", BranchTimeout: "noinfo", BranchUnavailable: "noinfo"}),
				say("answer", "domain_answer", "more"),
				endStep("more", PromptAnythingElse, DispResolvedInfo, true),
				say("noinfo", "domain_no_info", "more_noinfo"),
				endStep("more_noinfo", PromptAnythingElse, DispResolvedInfo, true),
			},
		},
	}
}
