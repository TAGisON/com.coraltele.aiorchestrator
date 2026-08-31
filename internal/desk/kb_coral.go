package desk

// CoralProductKnowledge is the approved bilingual product pack seeded into the
// coral-products collection when the Coral TFN preset is installed.
func CoralProductKnowledge() string {
	return stringsJoin(
		"Coral Telecom IP Phones are SIP desk phones for enterprise telephony, with HD voice, PoE, programmable keys and central provisioning from the Coral Call Server.",
		"Coral Telecom के IP Phone SIP डेस्क फोन हैं — HD आवाज़, PoE, programmable keys और Coral Call Server से केंद्रीय provisioning।",
		"The Coral Media Gateway connects IP telephony to PRI, E1 and FXO or FXS trunks, with support for SIP, transcoding and survivable local switching.",
		"Coral Media Gateway IP टेलीफोनी को PRI, E1 और FXO/FXS trunks से जोड़ता है — SIP, transcoding और स्थानीय switching के साथ।",
		"The Coral Call Server is the enterprise IP PBX providing extension management, call routing, conferencing, voicemail integration and redundancy options.",
		"Coral Call Server एंटरप्राइज़ IP PBX है — एक्सटेंशन प्रबंधन, कॉल रूटिंग, कॉन्फ्रेंसिंग, वॉइसमेल और redundancy।",
		"Coral Call Center is the inbound and outbound contact centre suite with skill based routing, queues, agent desktop, supervisor monitoring and reporting.",
		"Coral Call Center इनबाउंड और आउटबाउंड कॉन्टैक्ट सेंटर सूट है — skill based routing, queues, एजेंट डेस्कटॉप, सुपरवाइज़र मॉनिटरिंग और रिपोर्टिंग।",
		"Coral Cloud Box is the compact cloud connected communication server for branch offices, bundling call control, voice mail and remote management.",
		"Coral Cloud Box शाखा कार्यालयों के लिए compact cloud-connected संचार सर्वर है — कॉल कंट्रोल, वॉइसमेल और रिमोट प्रबंधन।",
		"Coral VMS is the voice mail and voice recording system with mailbox management, auto attendant menus and call recording retention controls.",
		"Coral VMS वॉइस मेल और कॉल रिकॉर्डिंग सिस्टम है — मेलबॉक्स, ऑटो अटेंडेंट और रिकॉर्डिंग retention।",
	)
}

func stringsJoin(parts ...string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\n\n"
		}
		out += p
	}
	return out
}
