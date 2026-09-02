package desk

// Coral Telecom toll-free desk preset — English-authored TFN script as a desk
// document: canonical EN prompts, India runtime languages via synthesis, four
// intents, guided troubleshooting trees, ticket + email actions, routing matrix.

// PresetCoralTFN builds the Coral Telecom TFN desk for a tenant.
func PresetCoralTFN(tenantID string) Doc {
	d := Doc{
		SchemaVersion: SchemaVersion,
		ID:            "coral-tfn",
		TenantID:      tenantID,
		Name:          "Coral Telecom Toll-Free Desk",
		Direction:     DirectionInbound,
		Purpose:       "support",
		// Canonical authoring locale is English. Callers are served in any allowlisted
		// Indian language via STT lock + locale synthesis (not duplicated HI tabs).
		// Author in English only; CX.runtime_languages = India set below.
		Languages:       []string{"en-IN"},
		DefaultLanguage: "en-IN",
		Tone:            "professional",
		VoiceID:         "priya",
		Voice:           map[string]string{"sarvam-tts": "priya"},
		CX:              DefaultCX(),
		Prompts:         coralPrompts(),
		Intents:         coralIntents(),
		Paths:           coralPaths(),
		Matrix:          coralMatrix(),
		Skills:          coralSkills(),
		Knowledge: []KnowledgeAttach{
			{Collection: "coral-products", Intents: []string{"product_information", "technical_support"}},
		},
		Retention: &RetentionOverride{TranscriptDays: 90, AttributesDays: 90, AuditDays: 365, RecordingDays: 30},
	}
	d.Normalize()
	// True multilingual runtime: English assets + India allowlist + synthesis.
	welcomeBarge := false
	localeSynth := true
	d.CX.WelcomeBargeAllowed = &welcomeBarge
	d.CX.LocaleSynthesis = &localeSynth
	d.CX.PrimaryLocale = "en-IN"
	d.CX.RuntimeLanguages = append([]string(nil), IndiaDefaultLanguages...)
	d.CX.RTPSettleMs = 400
	d.CX.ListenWhileSpeak = true
	d.CX.BargeIn = true
	d.CX.MinBargeChars = 3
	d.CX.MinBargeMs = 280
	d.CX.BargePartialConfidence = 0.70
	return d
}

func prompt(id, label, en string) Prompt {
	// Canonical English only — runtime synthesizes to active_language (LIVE_TALK §6.3).
	return Prompt{
		ID:    id,
		Label: label,
		Media: "text_tts",
		Text:  map[string]string{"en-IN": en},
	}
}

func coralPrompts() map[string]Prompt {
	list := []Prompt{
		// A greeting is a greeting — short, warm, and it hands the turn straight
		// back to the caller. The capability menu and the "English or Hindi" line
		// live in the clarify prompt, said only if the caller needs steering.
		prompt(PromptWelcome, "Welcome", "Thank you for calling Coral Telecom. How may I help you today?"),
		prompt(PromptClarify, "Clarify menu", "I can help you with Sales Enquiry, Product Information, Technical Support, or Service Complaint. Please tell me what you need help with."),
		prompt("clarify_2", "Clarify open", "No problem. Please briefly describe what you need help with, and I will direct your call to the right team."),
		prompt(PromptSilence1, "Silence nudge 1", "Are you still on the call? I am here to help."),
		prompt(PromptSilence2, "Silence nudge 2", "I can assist you with Sales, Product Information, Technical Support, or Service Complaint. Please tell me what you need."),
		prompt(PromptSilenceGoodbye, "Silence goodbye", "I am unable to hear a response from you. Thank you for calling Coral Telecom Limited. Goodbye."),
		prompt(PromptClosing, "Closing", "Thank you for calling Coral Telecom Limited. We appreciate your call and look forward to serving you. Have a great day. Goodbye."),
		prompt(PromptAnythingElse, "Anything else", "Is there anything else I can help you with today?"),
		prompt(PromptSystemDown, "System down", "I am sorry, I am unable to complete that right now due to a temporary system issue. I can connect you with our team so your issue is addressed."),
		prompt(PromptHold, "Hold", "Please hold while I transfer your call."),

		// Sales (§3)
		prompt("sales_intro", "Sales intro", "Certainly. I understand that you have a sales enquiry. I will connect you with our Sales team. Rahul Gupta will assist you with your requirement."),
		prompt("sales_requirement", "Sales requirement", "Could you briefly tell me your requirement, including the product and quantity if known?"),
		prompt("sales_unavailable", "Sales unavailable", "Our Sales representative is currently unavailable."),
		prompt("sales_callback_offer", "Sales callback offer", "Would you like me to register your enquiry and arrange a callback?"),
		prompt("ask_name", "Ask name", "May I have your name, please?"),
		prompt("ask_email", "Ask email", "Please provide your email address so that we can send you the confirmation details."),
		prompt("sales_registered", "Sales enquiry registered", "Thank you. Your enquiry has been registered with reference number {{enquiry_id}}. Our Sales team will call you back."),
		prompt("sales_no_callback", "Sales no callback", "Understood. You can reach our Sales team any time on this number."),

		// Product information (§4)
		prompt("product_which", "Which product", "Certainly. I can help you with information about Coral Telecom products and solutions. Could you please tell me which product or solution you are interested in? For example IP Phones, Media Gateway, Call Server, Call Center, Cloud Box, or VMS."),
		prompt("product_info_answer", "Product info answer", "{{kb_answer}}"),
		prompt("product_no_info", "Product info unavailable", "I do not have approved details for that product with me right now."),
		prompt("product_connect_offer", "Offer sales connect", "For detailed product information, pricing, configuration, or deployment requirements, I can connect you with our Sales team. Would you like me to connect you with Rahul Gupta?"),

		// Technical support (§5)
		prompt("tech_intro", "Tech intro", "I understand that you require technical support. Before I transfer your call, may I ask a few questions so that our technical team can assist you faster?"),
		prompt("tech_q_product", "Tech product", "Which Coral Telecom product or system are you facing the problem with?"),
		prompt("tech_q_problem", "Tech problem", "Could you briefly describe the problem you are experiencing?"),
		prompt("tech_q_impact", "Tech impact", "Is the issue affecting a single user or device, multiple users, or the entire system?"),
		prompt("tech_q_error", "Tech error", "If you are seeing any error message, alarm, or specific indication, could you please tell me what it says? If not, please say none."),
		prompt("tech_summary", "Tech summary", "Thank you. Let me summarize. You are facing a problem with {{product}}. The issue is {{problem}}. It is affecting {{impact}}. Is that correct?"),
		prompt("tech_transfer_intro", "Tech transfer intro", "Thank you. I will now connect you with Arjun Singh Topwal and share the information you have provided so you do not have to repeat it. Please hold while I transfer your call."),
		prompt("tech_unavailable_offer", "Tech unavailable", "Our technical support engineer is not available at this moment. Would you like me to register a service complaint instead?"),

		// Service complaint (§6)
		prompt("complaint_intro", "Complaint intro", "I am sorry to hear that you are facing a service issue. I will first understand the problem and try some basic troubleshooting. If the issue is still not resolved, you can speak with our technical support team or register a formal service complaint."),
		prompt("complaint_q_product", "Complaint product", "Which Coral Telecom product or service do you need assistance with? IP Phone, Media Gateway, Call Center, Call Server, Cloud Box, VMS, or another product?"),

		// IP Phone tree (§6.2)
		prompt("ipp_l1_power", "IP phone power", "Thank you. You are facing an issue with an IP Phone. Is the phone powered on, and is the display working?"),
		prompt("ipp_l1_retry", "IP phone power retry", "Please check the power adapter, or the network cable if the phone receives power through PoE. After checking, is the phone powering on now?"),
		prompt("ipp_l2_network", "IP phone network", "Does the phone show a network connection, or does it display any network related error?"),
		prompt("ipp_l3_calling", "IP phone calling", "Are you able to make or receive calls from this phone?"),
		prompt("ipp_scope", "IP phone scope", "Is the problem affecting only this IP Phone, or are multiple IP Phones experiencing the same issue?"),
		prompt("ipp_summary", "IP phone summary", "Based on what you shared, I understand that {{problem}} is occurring on the IP Phone, affecting {{impact}}."),

		// Media gateway tree (§6.3)
		prompt("mg_l1_status", "Gateway status", "Thank you. You are facing an issue with the Media Gateway. Is the gateway powered on, and are the status indicators showing normally?"),
		prompt("mg_l2_network", "Gateway network", "Is the Media Gateway reachable from the Call Server or the network?"),
		prompt("mg_l3_service", "Gateway service", "Is the problem related to incoming calls, outgoing calls, or both?"),
		prompt("mg_extra", "Gateway scope", "Is the problem affecting all calls, or only specific numbers or channels?"),
		prompt("mg_summary", "Gateway summary", "Thank you. I understand that the Media Gateway is experiencing {{problem}}, affecting {{impact}}."),

		// Call center tree (§6.4)
		prompt("cc_l1_login", "Call center login", "Thank you. You are facing an issue with the Call Center. Are the agents able to log in to the Call Center application?"),
		prompt("cc_l2_calls", "Call center calls", "If agents are logged in, are calls reaching the agents?"),
		prompt("cc_l3_scope", "Call center scope", "Is the problem affecting all agents or only a specific agent?"),
		prompt("cc_extra", "Call center symptom", "Are calls getting disconnected, remaining in the queue, or not reaching the Call Center at all?"),
		prompt("cc_summary", "Call center summary", "Thank you. I understand that the Call Center is experiencing {{problem}}, affecting {{impact}}."),

		// Call server / cloud box / VMS tree (§6.5)
		prompt("sys_l1_status", "System status", "Thank you. I will help you troubleshoot. Is the system powered on and is the application or service running normally?"),
		prompt("sys_l2_conn", "System connectivity", "Are users or connected devices able to communicate with the system?"),
		prompt("sys_l3_impact", "System impact", "Is this problem affecting a single user, multiple users, or the entire system?"),
		prompt("sys_extra", "System alarm", "Are you seeing any alarm, error message, or service failure indication? If not, please say none."),
		prompt("sys_summary", "System summary", "Thank you. I understand that {{product}} is experiencing {{problem}}, affecting {{impact}}."),

		// Other product (§6.6)
		prompt("other_describe", "Other product name", "Please tell me the name of the product or service."),
		prompt("other_q1", "Other problem", "Please briefly describe the issue you are experiencing."),
		prompt("other_q2", "Other start", "When did the problem start?"),
		prompt("other_q3", "Other scope", "Is the problem affecting one user or multiple users?"),
		prompt("other_summary", "Other summary", "Thank you. I understand that {{product}} is experiencing {{problem}}, affecting {{impact}}."),

		prompt("offer_tech_or_complaint", "Offer tech or complaint", "Would you like me to connect you with Arjun Singh Topwal for technical assistance, or would you like to register a service complaint?"),

		// Complaint registration (§8–§14)
		prompt("complaint_register_intro", "Complaint register intro", "Certainly. I can register a service complaint for you. Before I create the complaint, I need a few details."),
		prompt("email_confirm", "Email confirm", "Let me confirm your email address. I have {{customer_email}}. Is that correct?"),
		prompt("complaint_summary", "Complaint summary", "Let me confirm your complaint details. Customer name {{customer_name}}. Product {{product}}. Problem {{problem}}. Impact {{impact}}. Email {{customer_email}}. Is all this information correct?"),
		prompt("complaint_correction_choice", "Correction choice", "Which information would you like to correct? Name, email, or the problem description?"),
		prompt("complaint_fix_problem", "Fix problem", "Please describe the problem again."),
		prompt("ticket_created", "Ticket created", "Thank you. Your service complaint has been successfully registered. Your complaint ticket number is {{ticket_id}}. Please keep this ticket number for future reference."),
		prompt("email_sent", "Email sent", "I have also sent the complaint details and ticket number to your email address {{customer_email}}."),
		prompt("email_failed", "Email failed", "Your complaint has been registered and your ticket number is {{ticket_id}}. However, I was unable to send the confirmation email at this time. Please keep your ticket number for future reference."),
		prompt("ticket_failed", "Ticket failed", "I am sorry, I am currently unable to register the complaint due to a temporary system issue. I have not created any ticket."),
		prompt("connect_tech_offer", "Connect tech offer", "I can connect you directly with our technical support team so that your issue is addressed. Would you like me to connect you with Arjun Singh Topwal?"),
		prompt("duplicate_found", "Duplicate found", "I found an existing complaint for this issue with ticket number {{existing_ticket_id}}."),
		prompt("duplicate_choice", "Duplicate choice", "Would you like me to connect you with technical support regarding this existing complaint, or would you like to provide additional information?"),
		prompt("duplicate_info", "Duplicate info", "Please tell me the additional information you would like to add to the existing complaint."),
		prompt("duplicate_ack", "Duplicate ack", "Thank you. I have added your update to ticket {{existing_ticket_id}}. Our support team will review it."),

		// Critical (§15)
		prompt("critical_ack", "Critical ack", "I understand that this appears to be a critical service issue. I will prioritize this for technical support."),
		prompt("critical_connect_offer", "Critical connect offer", "Would you like me to connect you immediately with Arjun Singh Topwal?"),

		// Transfers (§7, §21)
		prompt("transfer_sales_intro", "Transfer sales", "Certainly. I will connect you with Rahul Gupta from our Sales team. Please hold while I transfer your call."),
		prompt("transfer_tech_intro", "Transfer tech", "Certainly. I will connect you with Arjun Singh Topwal from our Technical Support team. I will also share the information you have already provided so you do not need to repeat the details. Please hold while I transfer your call."),
		prompt("transfer_service_intro", "Transfer service", "Certainly. I will connect you with Ritu from our Service team. Please hold while I transfer your call."),
		prompt("transfer_done", "Transfer done", "You are being connected now. Thank you for calling Coral Telecom Limited."),
		prompt("transfer_failed", "Transfer failed", "I am sorry, I could not reach the team right now."),
	}
	out := make(map[string]Prompt, len(list))
	for _, p := range list {
		out[p.ID] = p
	}
	return out
}

func coralIntents() []Intent {
	return []Intent{
		{
			ID: "sales_enquiry", Display: "Sales Enquiry", Active: true, PathID: "sales_enquiry",
			Phrases: map[string][]string{
				"en-IN": {"sales", "sales enquiry", "buy", "purchase", "want to buy", "price", "pricing", "quotation", "quote",
					"demo", "deployment", "new project", "tender", "commercial", "dealer", "distributor", "new requirement", "cost"},
				"hi-IN": {"खरीदना", "कीमत", "कोटेशन", "डेमो", "बिक्री", "सेल्स", "टेंडर", "डीलर",
					"kharidna hai", "keemat", "price kya hai", "quotation chahiye", "demo chahiye", "sales team"},
			},
		},
		{
			ID: "product_information", Display: "Product Information", Active: true, PathID: "product_information",
			Phrases: map[string][]string{
				"en-IN": {"product information", "information about", "tell me about", "details about", "specification",
					"features", "what is", "know more", "know about", "brochure", "datasheet", "product", "pbx"},
				"hi-IN": {"जानकारी", "उत्पाद की जानकारी", "प्रोडक्ट", "उत्पाद", "फीचर", "विवरण",
					"jankari chahiye", "product ki jankari", "batao product", "feature kya hai", "produk", "product ke bare"},
			},
		},
		{
			ID: "technical_support", Display: "Technical Support", Active: true, PathID: "technical_support",
			Phrases: map[string][]string{
				"en-IN": {"technical support", "support", "not working", "issue", "problem", "configuration issue",
					"network issue", "call connectivity", "alarm", "error", "engineer", "troubleshoot", "fault",
					"phone not working", "system down", "cannot make calls", "no audio"},
				"hi-IN": {"तकनीकी सहायता", "काम नहीं कर रहा", "समस्या", "खराब", "नेटवर्क समस्या", "इंजीनियर",
					"technical support chahiye", "kaam nahi kar raha", "samasya hai", "problem aa rahi hai", "band ho gaya"},
			},
		},
		{
			ID: "service_complaint", Display: "Service Complaint", Active: true, PathID: "service_complaint",
			Phrases: map[string][]string{
				"en-IN": {"complaint", "register a complaint", "service complaint", "unresolved", "still not resolved",
					"raise a ticket", "log a complaint", "file a complaint", "poor service"},
				"hi-IN": {"शिकायत", "शिकायत दर्ज", "टिकट", "सेवा शिकायत",
					"shikayat", "shikayat darj karni hai", "complaint karni hai", "ticket banao"},
			},
		},
	}
}

func coralMatrix() []MatrixRow {
	// Number (Admin → Routing → Extension) is the dial destination for
	// uuid_transfer. It is NOT hardcoded here — Configurator owns the real
	// queue/agent extensions. Target stays the ACD queue label.
	return []MatrixRow{
		{Intent: "sales_enquiry", Owner: "Rahul Gupta", Target: "sales", Action: "transfer"},
		{Intent: "product_information", Owner: "Rahul Gupta", Target: "sales", Action: "transfer"},
		{Intent: "technical_support", Owner: "Arjun Singh Topwal", Target: "technical_support", Action: "transfer"},
		{Intent: "service_complaint", Owner: "Ritu", Target: "service", Action: "both"},
	}
}

func coralSkills() map[string]SkillBind {
	names := []string{
		"resolve_caller", "search_knowledge", "find_open_complaint", "transfer_to_queue",
		"create_service_complaint", "send_complaint_email", "register_sales_enquiry", "schedule_callback",
		"push_disposition",
	}
	out := map[string]SkillBind{}
	for _, n := range names {
		out[n] = SkillBind{Enabled: true, Mode: "stub", Gateway: DefaultSkillGateway}
	}
	return out
}

func say(id, promptID, next string) Step {
	return Step{ID: id, Type: StepSay, PromptID: promptID, Next: next}
}

func ask(id, promptID, slot, validation, next string) Step {
	return Step{ID: id, Type: StepAsk, PromptID: promptID, SlotKey: slot, Validation: validation, Next: next, Required: true}
}

func reask(s Step) Step {
	s.Reask = true
	return s
}

func confirmStep(id, promptID, onYes, onNo string) Step {
	return Step{ID: id, Type: StepConfirm, SummaryPromptID: promptID, OnYes: onYes, OnNo: onNo}
}

func choiceStep(id, promptID, slot string, opts ...Option) Step {
	return Step{ID: id, Type: StepChoice, PromptID: promptID, SlotKey: slot, Options: opts}
}

func actionStep(id, skill string, args map[string]string, branches map[string]string) Step {
	return Step{ID: id, Type: StepAction, Skill: skill, ArgMap: args, Branches: branches}
}

func endStep(id, closing, disposition string, anythingElse bool) Step {
	return Step{ID: id, Type: StepEnd, ClosingPromptID: closing, DispositionHint: disposition, OfferAnythingElse: anythingElse}
}

func opt(id, label, next string, en, hi []string) Option {
	return Option{ID: id, Label: label, Next: next, Utterances: map[string][]string{"en-IN": en, "hi-IN": hi}}
}

var impactOptions = func(next string) []Option {
	return []Option{
		opt("single_user", "single user", next,
			[]string{"single", "single user", "one user", "only one", "one device", "just one",
				"one agent", "single agent", "one extension", "only this"},
			[]string{"एक", "एक उपयोगकर्ता", "सिर्फ एक", "ek user", "sirf ek", "ek hi", "ek agent"}),
		opt("multiple_users", "multiple users", next,
			[]string{"multiple", "multiple users", "many users", "several", "few users", "some users",
				"some agents", "few agents", "many agents", "multiple agents", "multiple phones"},
			[]string{"कई", "कई उपयोगकर्ता", "बहुत सारे", "kai users", "bahut log", "kuch users", "kai agents"}),
		opt("entire_system", "entire system", next,
			[]string{"entire", "entire system", "whole system", "everyone", "all users", "complete system",
				"everything", "all agents", "every agent", "all the agents", "all extensions", "all of them"},
			[]string{"पूरा सिस्टम", "सब", "सभी", "सारे", "poora system", "sab band", "sabhi users", "sab agents"}),
	}
}

func coralPaths() map[string]Path {
	paths := []Path{
		{
			ID: "sales_enquiry", Label: "Sales enquiry", Entry: "s_intro",
			Steps: []Step{
				say("s_intro", "sales_intro", "s_requirement"),
				ask("s_requirement", "sales_requirement", AttrRequirement, ValidateFreeText, "s_hold"),
				say("s_hold", "transfer_sales_intro", "s_transfer"),
				actionStep("s_transfer", "transfer_to_queue",
					map[string]string{"target": "=sales", "owner": "=Rahul Gupta"},
					map[string]string{BranchOK: "s_done", BranchUnavailable: "s_unavailable", BranchFail: "s_unavailable", BranchTimeout: "s_unavailable"}),
				endStep("s_done", "transfer_done", DispTransferredSales, false),
				say("s_unavailable", "sales_unavailable", "s_offer"),
				confirmStep("s_offer", "sales_callback_offer", "s_name", "s_no_callback"),
				ask("s_name", "ask_name", AttrCustomerName, ValidateFreeText, "s_email"),
				ask("s_email", "ask_email", AttrCustomerEmail, ValidateEmail, "s_register"),
				actionStep("s_register", "register_sales_enquiry",
					map[string]string{"name": AttrCustomerName, "email": AttrCustomerEmail, "requirement": AttrRequirement, "phone": AttrANI},
					map[string]string{BranchOK: "s_registered", BranchFail: "s_register_failed", BranchTimeout: "s_register_failed", BranchUnavailable: "s_register_failed"}),
				say("s_registered", "sales_registered", "s_end"),
				endStep("s_end", "closing", DispCallbackScheduled, true),
				say("s_register_failed", PromptSystemDown, "s_end_failed"),
				endStep("s_end_failed", "closing", DispSystemFailure, false),
				say("s_no_callback", "sales_no_callback", "s_end_info"),
				endStep("s_end_info", "closing", DispResolvedInfo, true),
			},
		},
		{
			ID: "product_information", Label: "Product information", Entry: "p_which",
			Steps: []Step{
				ask("p_which", "product_which", AttrProduct, ValidateProduct, "p_lookup"),
				actionStep("p_lookup", "search_knowledge",
					map[string]string{"product": AttrProduct, "query": AttrProduct},
					map[string]string{BranchOK: "p_answer", BranchFail: "p_no_info", BranchTimeout: "p_no_info", BranchUnavailable: "p_no_info"}),
				say("p_answer", "product_info_answer", "p_offer"),
				say("p_no_info", "product_no_info", "p_offer"),
				confirmStep("p_offer", "product_connect_offer", "p_hold", "p_end_info"),
				say("p_hold", "transfer_sales_intro", "p_transfer"),
				actionStep("p_transfer", "transfer_to_queue",
					map[string]string{"target": "=sales", "owner": "=Rahul Gupta"},
					map[string]string{BranchOK: "p_done", BranchUnavailable: "p_failed", BranchFail: "p_failed", BranchTimeout: "p_failed"}),
				endStep("p_done", "transfer_done", DispTransferredSales, false),
				say("p_failed", "transfer_failed", "p_end_failed"),
				endStep("p_end_failed", "closing", DispSystemFailure, false),
				endStep("p_end_info", "closing", DispResolvedInfo, true),
			},
		},
		{
			ID: "technical_support", Label: "Technical support", Entry: "t_intro",
			Steps: []Step{
				say("t_intro", "tech_intro", "t_product"),
				ask("t_product", "tech_q_product", AttrProduct, ValidateProduct, "t_problem"),
				ask("t_problem", "tech_q_problem", AttrProblem, ValidateFreeText, "t_impact"),
				choiceStep("t_impact", "tech_q_impact", AttrImpact, impactOptions("t_error")...),
				{ID: "t_error", Type: StepAsk, PromptID: "tech_q_error", SlotKey: AttrErrorAlarm,
					Validation: ValidateFreeText, Next: "t_summary", OnNoMatch: RepairNext},
				confirmStep("t_summary", "tech_summary", "t_transfer_intro", "t_problem"),
				say("t_transfer_intro", "tech_transfer_intro", "t_transfer"),
				actionStep("t_transfer", "transfer_to_queue",
					map[string]string{"target": "=technical_support", "owner": "=Arjun Singh Topwal"},
					map[string]string{BranchOK: "t_done", BranchUnavailable: "t_unavailable", BranchFail: "t_unavailable", BranchTimeout: "t_unavailable"}),
				endStep("t_done", "transfer_done", DispTransferredTech, false),
				confirmStep("t_unavailable", "tech_unavailable_offer", "path:complaint_register", "t_end_no"),
				endStep("t_end_no", "closing", DispUnresolvedNoTicket, true),
			},
		},
		{
			ID: "service_complaint", Label: "Service complaint", Entry: "c_intro",
			Steps: []Step{
				say("c_intro", "complaint_intro", "c_product"),
				choiceStep("c_product", "complaint_q_product", AttrProduct,
					opt("ip_phone", "ip phone", "path:ts_ip_phone",
						ProductVocabulary["ip_phone"], []string{"आईपी फोन", "फोन"}),
					opt("media_gateway", "media gateway", "path:ts_media_gateway",
						ProductVocabulary["media_gateway"], []string{"मीडिया गेटवे", "गेटवे"}),
					opt("call_center", "call center", "path:ts_call_center",
						ProductVocabulary["call_center"], []string{"कॉल सेंटर"}),
					opt("call_server", "call server", "path:ts_system",
						ProductVocabulary["call_server"], []string{"कॉल सर्वर"}),
					opt("cloud_box", "cloud box", "path:ts_system",
						ProductVocabulary["cloud_box"], []string{"क्लाउड बॉक्स"}),
					opt("vms", "vms", "path:ts_system",
						ProductVocabulary["vms"], []string{"वीएमएस", "वॉइस मेल"}),
					opt("other", "other", "path:ts_other",
						[]string{"other", "another", "different", "something else"},
						[]string{"अन्य", "दूसरा", "koi aur", "dusra"}),
				),
			},
		},
		troubleshootPath("ts_ip_phone", "IP Phone troubleshooting", []tsQuestion{
			{id: "l1", promptID: "ipp_l1_power", slot: "ts_l1"},
			{id: "l2", promptID: "ipp_l2_network", slot: "ts_l2"},
			{id: "l3", promptID: "ipp_l3_calling", slot: AttrProblem},
			{id: "l4", promptID: "ipp_scope", slot: AttrImpact, choice: true},
		}, "ipp_summary"),
		troubleshootPath("ts_media_gateway", "Media Gateway troubleshooting", []tsQuestion{
			{id: "l1", promptID: "mg_l1_status", slot: "ts_l1"},
			{id: "l2", promptID: "mg_l2_network", slot: "ts_l2"},
			{id: "l3", promptID: "mg_l3_service", slot: AttrProblem},
			{id: "l4", promptID: "mg_extra", slot: AttrImpact},
		}, "mg_summary"),
		troubleshootPath("ts_call_center", "Call Center troubleshooting", []tsQuestion{
			{id: "l1", promptID: "cc_l1_login", slot: "ts_l1"},
			{id: "l2", promptID: "cc_l2_calls", slot: "ts_l2"},
			{id: "l3", promptID: "cc_extra", slot: AttrProblem},
			{id: "l4", promptID: "cc_l3_scope", slot: AttrImpact, choice: true},
		}, "cc_summary"),
		troubleshootPath("ts_system", "Call Server / Cloud Box / VMS troubleshooting", []tsQuestion{
			{id: "l1", promptID: "sys_l1_status", slot: "ts_l1"},
			{id: "l2", promptID: "sys_l2_conn", slot: "ts_l2"},
			{id: "l3", promptID: "sys_extra", slot: AttrProblem},
			{id: "l4", promptID: "sys_l3_impact", slot: AttrImpact, choice: true},
		}, "sys_summary"),
		troubleshootPath("ts_other", "Other product troubleshooting", []tsQuestion{
			{id: "l0", promptID: "other_describe", slot: AttrProduct},
			{id: "l1", promptID: "other_q1", slot: AttrProblem},
			{id: "l2", promptID: "other_q2", slot: "ts_l2"},
			{id: "l3", promptID: "other_q3", slot: AttrImpact, choice: true},
		}, "other_summary"),
		{
			ID: "complaint_register", Label: "Complaint registration", Entry: "r_intro",
			Steps: []Step{
				say("r_intro", "complaint_register_intro", "r_name"),
				ask("r_name", "ask_name", AttrCustomerName, ValidateFreeText, "r_email"),
				ask("r_email", "ask_email", AttrCustomerEmail, ValidateEmail, "r_email_confirm"),
				confirmStep("r_email_confirm", "email_confirm", "r_dupcheck", "r_email"),
				actionStep("r_dupcheck", "find_open_complaint",
					map[string]string{"product": AttrProduct, "email": AttrCustomerEmail, "phone": AttrANI},
					map[string]string{BranchOK: "r_summary", BranchDuplicate: "r_dup_found", BranchFail: "r_summary", BranchTimeout: "r_summary", BranchUnavailable: "r_summary"}),
				confirmStep("r_summary", "complaint_summary", "r_create", "r_correct"),
				choiceStep("r_correct", "complaint_correction_choice", "",
					opt("name", "name", "r_fix_name",
						[]string{"name", "my name"}, []string{"नाम", "naam"}),
					opt("email", "email", "r_fix_email",
						[]string{"email", "mail", "email address"}, []string{"ईमेल", "email"}),
					opt("problem", "problem", "r_fix_problem",
						[]string{"problem", "issue", "description", "complaint"}, []string{"समस्या", "problem", "shikayat"}),
				),
				reask(ask("r_fix_name", "ask_name", AttrCustomerName, ValidateFreeText, "r_summary")),
				reask(ask("r_fix_email", "ask_email", AttrCustomerEmail, ValidateEmail, "r_email_confirm")),
				reask(ask("r_fix_problem", "complaint_fix_problem", AttrProblem, ValidateFreeText, "r_summary")),
				actionStep("r_create", "create_service_complaint",
					map[string]string{
						"name": AttrCustomerName, "email": AttrCustomerEmail, "phone": AttrANI,
						"product": AttrProduct, "problem": AttrProblem, "impact": AttrImpact,
						"troubleshooting": AttrTroubleshoot, "priority": AttrPriority,
					},
					map[string]string{BranchOK: "r_created", BranchDuplicate: "r_dup_found", BranchFail: "r_ticket_failed", BranchTimeout: "r_ticket_failed", BranchUnavailable: "r_ticket_failed"}),
				say("r_created", "ticket_created", "r_email_send"),
				actionStep("r_email_send", "send_complaint_email",
					map[string]string{"email": AttrCustomerEmail, "ticket_id": AttrTicketID, "name": AttrCustomerName,
						"product": AttrProduct, "problem": AttrProblem},
					map[string]string{BranchOK: "r_email_ok", BranchFail: "r_email_fail", BranchTimeout: "r_email_fail", BranchUnavailable: "r_email_fail"}),
				say("r_email_ok", "email_sent", "r_end"),
				say("r_email_fail", "email_failed", "r_end"),
				endStep("r_end", "closing", DispTicketCreated, true),
				say("r_ticket_failed", "ticket_failed", "r_offer_tech"),
				confirmStep("r_offer_tech", "connect_tech_offer", "path:transfer_technical_support", "r_end_failed"),
				endStep("r_end_failed", "closing", DispSystemFailure, false),
				say("r_dup_found", "duplicate_found", "r_dup_choice"),
				choiceStep("r_dup_choice", "duplicate_choice", "",
					opt("connect", "connect", "path:transfer_technical_support",
						[]string{"connect", "technical support", "transfer", "agent", "speak", "yes"},
						[]string{"जोड़", "टेक्निकल", "connect karo", "baat karao", "हाँ", "हां"}),
					opt("add_info", "add information", "r_dup_info",
						[]string{"additional", "add information", "more information", "provide", "update"},
						[]string{"जानकारी", "और जानकारी", "jankari dena", "update"}),
					opt("no", "no", "r_dup_end",
						[]string{"no", "no thanks", "not now", "later"},
						[]string{"नहीं", "नही", "बाद में", "nahi"}),
				),
				ask("r_dup_info", "duplicate_info", "additional_info", ValidateFreeText, "r_dup_ack"),
				say("r_dup_ack", "duplicate_ack", "r_dup_end"),
				endStep("r_dup_end", "closing", DispExistingTicket, true),
			},
		},
		{
			ID: "critical", Label: "Critical escalation", Entry: "k_ack",
			Steps: []Step{
				{ID: "k_ack", Type: StepSay, PromptID: "critical_ack", Next: "k_offer",
					SetAttributes: map[string]string{AttrPriority: "critical"}},
				confirmStep("k_offer", "critical_connect_offer", "k_transfer", "path:complaint_register"),
				say("k_transfer", "transfer_tech_intro", "k_action"),
				actionStep("k_action", "transfer_to_queue",
					map[string]string{"target": "=technical_support", "owner": "=Arjun Singh Topwal", "priority": "=critical"},
					map[string]string{BranchOK: "k_done", BranchFail: "k_failed", BranchTimeout: "k_failed", BranchUnavailable: "k_failed"}),
				endStep("k_done", "transfer_done", DispTransferredTech, false),
				say("k_failed", "transfer_failed", "k_end"),
				endStep("k_end", "closing", DispSystemFailure, false),
			},
		},
		transferPath("transfer_sales_enquiry", "transfer_sales_intro", "sales", "Rahul Gupta", DispTransferredSales),
		transferPath("transfer_product_information", "transfer_sales_intro", "sales", "Rahul Gupta", DispTransferredSales),
		transferPath("transfer_technical_support", "transfer_tech_intro", "technical_support", "Arjun Singh Topwal", DispTransferredTech),
		transferPath("transfer_service_complaint", "transfer_service_intro", "service", "Ritu", DispTransferredService),
		transferPath("transfer_generic", "transfer_tech_intro", "technical_support", "Arjun Singh Topwal", DispTransferredTech),
	}
	out := make(map[string]Path, len(paths))
	for _, p := range paths {
		out[p.ID] = p
	}
	return out
}

type tsQuestion struct {
	id       string
	promptID string
	slot     string
	choice   bool
}

// troubleshootPath builds a 2–3 level troubleshooting tree that ends in a summary
// plus the technical-support / complaint choice (§6.2–§6.6).
func troubleshootPath(id, label string, qs []tsQuestion, summaryPrompt string) Path {
	steps := make([]Step, 0, len(qs)+5)
	for i, q := range qs {
		next := "x_summary"
		if i+1 < len(qs) {
			next = qs[i+1].id
		}
		if q.choice {
			c := choiceStep(q.id, q.promptID, q.slot, impactOptions(next)...)
			c.OnNoMatch = RepairNext
			steps = append(steps, c)
			continue
		}
		step := ask(q.id, q.promptID, q.slot, ValidateFreeText, next)
		step.OnNoMatch = RepairNext
		steps = append(steps, step)
	}
	notes := make([]string, 0, len(qs))
	for _, q := range qs {
		if q.slot == "" || q.slot == AttrImpact || q.slot == AttrProduct {
			continue
		}
		notes = append(notes, "{{"+q.slot+"}}")
	}
	summary := Step{
		ID: "x_summary", Type: StepSay, PromptID: summaryPrompt, Next: "x_offer",
		SetAttributes: map[string]string{AttrTroubleshoot: joinNotes(notes)},
	}
	steps = append(steps,
		summary,
		choiceStep("x_offer", "offer_tech_or_complaint", "",
			opt("tech", "technical support", "path:transfer_technical_support",
				[]string{"technical", "technical support", "connect", "engineer", "arjun", "agent", "transfer", "support"},
				[]string{"तकनीकी", "टेक्निकल", "इंजीनियर", "technical support", "baat karao", "connect karo"}),
			opt("complaint", "complaint", "path:complaint_register",
				[]string{"complaint", "register", "ticket", "raise", "log"},
				[]string{"शिकायत", "टिकट", "shikayat", "complaint darj", "ticket banao"}),
		),
	)
	return Path{ID: id, Label: label, Entry: qs[0].id, Steps: steps}
}

func joinNotes(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "; "
		}
		out += p
	}
	return out
}

func transferPath(id, introPrompt, target, owner, disposition string) Path {
	return Path{
		ID: id, Label: "Transfer " + target, Entry: "x_intro",
		Steps: []Step{
			say("x_intro", introPrompt, "x_action"),
			actionStep("x_action", "transfer_to_queue",
				map[string]string{"target": "=" + target, "owner": "=" + owner},
				map[string]string{BranchOK: "x_done", BranchFail: "x_failed", BranchTimeout: "x_failed", BranchUnavailable: "x_failed"}),
			endStep("x_done", "transfer_done", disposition, false),
			say("x_failed", "transfer_failed", "x_offer"),
			confirmStep("x_offer", "connect_tech_offer", "x_retry", "x_end"),
			actionStep("x_retry", "transfer_to_queue",
				map[string]string{"target": "=" + target, "owner": "=" + owner},
				map[string]string{BranchOK: "x_done", BranchFail: "x_end", BranchTimeout: "x_end", BranchUnavailable: "x_end"}),
			endStep("x_end", "closing", DispSystemFailure, false),
		},
	}
}
