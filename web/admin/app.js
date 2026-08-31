const { $, api, showBanner, formatApiError, loadPlatformStatus, renderReadiness } = CoralUI;

const ccBase = {
  modes: { listen: true, speak: true, think: true, talk: true },
  audio: { canonical_sample_rate_hz: 16000 },
  language: { behaviour: "none", auto_detect: true, mid_call_switch: true, allowed: ["en-IN", "hi-IN"] },
  hot_swap_allowed: ["language.primary"],
  fallback: {
    listen_down: { speak_canned: "clip-apology-en", skill: "warm_transfer" },
    think_down: { speak_canned: "clip-apology-en", skill: "warm_transfer" },
    speak_down: { text_sink: true, skill: "warm_transfer" },
  },
  skills: {
    allowed: ["warm_transfer"],
    definitions: { warm_transfer: { gateway: "coral-transfer", authority: "act", confirm: true } },
  },
};

const presets = {
  "cc-sales": {
    ...structuredClone(ccBase),
    id: "cc-sales",
    metadata: { display_name: "CC Sales", family: "contact-agent" },
    persona: {
      name: "Sales Mira",
      instructions: "Concise sales contact agent. Qualify interest; never invent pricing.",
      voice: { "fake-speak": "lab-voice-sales", "sarvam-tts": "shubh" },
    },
    response: {
      ladder: ["clip", "template", "llm"],
      clips: {
        "clip-apology-en": { text: "Sorry — connecting you to a sales specialist." },
        "greeting-en": { text: "Hi — Sales desk. How can I help today?", when: { regex: "(?i)\\b(hi|hello|hey)\\b" } },
        "pricing-en": { text: "I can share list pricing after we confirm your product interest.", when: { regex: "(?i)\\b(price|pricing|cost|quote)\\b" } },
      },
      templates: { clarify: { text: "Which product line are you asking about?", when: { regex: "(?i)\\b(what|huh|which)\\b" } } },
    },
  },
  "cc-rnd": {
    ...structuredClone(ccBase),
    id: "cc-rnd",
    metadata: { display_name: "CC R&D", family: "contact-agent" },
    persona: {
      name: "R&D Kai",
      instructions: "R&D desk agent. Prefer technical clarity; escalate speculative claims.",
      voice: { "fake-speak": "lab-voice-rnd", "sarvam-tts": "meera" },
    },
    response: {
      ladder: ["clip", "template", "llm"],
      clips: {
        "clip-apology-en": { text: "Sorry — transferring you to an engineer." },
        "greeting-en": { text: "R&D desk — describe the symptom or API error.", when: { regex: "(?i)\\b(hi|hello|hey)\\b" } },
        "bug-en": { text: "Please share the error code and last successful step.", when: { regex: "(?i)\\b(bug|crash|error|stack)\\b" } },
      },
      templates: { clarify: { text: "Is this reproducible in lab or only production?", when: { regex: "(?i)\\b(what|huh|where)\\b" } } },
    },
  },
  "cc-after-hours": {
    ...structuredClone(ccBase),
    id: "cc-after-hours",
    metadata: { display_name: "CC after-hours", family: "contact-agent" },
    persona: {
      name: "Night Desk",
      instructions: "After-hours agent. Contain urgent issues; warm-transfer when needed.",
      voice: { "fake-speak": "lab-voice-night", "sarvam-tts": "anushka" },
    },
    response: {
      ladder: ["clip", "template", "llm"],
      clips: {
        "clip-apology-en": { text: "Connecting you to the on-call desk." },
        "greeting-en": { text: "After-hours desk — what is urgent?", when: { regex: "(?i)\\b(hi|hello|hey)\\b" } },
      },
      templates: { clarify: { text: "Is this blocking production right now?", when: { regex: "(?i)\\b(what|urgent)\\b" } } },
    },
  },
  fakes: {
    id: "offline-fakes",
    metadata: { display_name: "Offline fakes", family: "contact-agent" },
    modes: { listen: true, speak: true, think: true, talk: true },
    audio: { canonical_sample_rate_hz: 16000 },
    persona: {
      name: "Fake Agent",
      instructions: "Offline fake Think replies for local testing.",
      voice: { "fake-speak": "lab-voice" },
    },
    response: {
      ladder: ["clip", "llm"],
      clips: {
        "clip-apology-en": { text: "Sorry — fake fallback." },
        "greeting-en": { text: "Hello from fakes.", when: { regex: "(?i)\\b(hi|hello|hey)\\b" } },
      },
    },
    routers: {
      listen: { providers: ["fake-listen"] },
      think: { providers: ["fake-think"] },
      speak: { providers: ["fake-speak"] },
    },
  },
};

function setJson(id, data) {
  $(id).textContent = typeof data === "string" ? data : JSON.stringify(data, null, 2);
}

async function refreshStatus() {
  try {
    const st = await loadPlatformStatus();
    renderReadiness($("readyList"), st);
    setJson("statusJson", st);
    if (!st.ready_for_sessions) {
      showBanner($("banner"), "bad",
        `<strong>Not ready for sessions.</strong> Fix blockers in Admin, then open User. <code>${(st.blockers || []).join(", ")}</code>`);
    } else if ((st.warnings || []).length) {
      showBanner($("banner"), "",
        `<strong>Degraded but startable.</strong> Warnings: <code>${st.warnings.join(", ")}</code>`);
    } else {
      showBanner($("banner"), "ok", `<strong>Ready.</strong> Users can start sessions.`);
    }
  } catch (e) {
    showBanner($("banner"), "bad", formatApiError(e));
    setJson("statusJson", { error: formatApiError(e) });
  }
}

async function loadGateways() {
  try {
    const data = await api("GET", "/v1/gateways");
    const g = data.gateways || {};
    const fill = (listId, rows) => {
      $(listId).innerHTML = (rows || []).map((r) => `<option value="${r.id}"></option>`).join("");
    };
    fill("gwListen", g.listen);
    fill("gwThink", g.think);
    fill("gwSpeak", g.speak);
  } catch (_) { /* non-fatal */ }
}

async function loadEngines() {
  try {
    const data = await api("GET", "/v1/tenant/engines");
    $("engListen").value = data.listen || "";
    $("engThink").value = data.think || "";
    $("engSpeak").value = data.speak || "";
    setJson("enginesJson", data);
  } catch (e) {
    $("engListen").value = "";
    $("engThink").value = "";
    $("engSpeak").value = "";
    setJson("enginesJson", { error: formatApiError(e), note: "Expected on fresh install until Save engines." });
  }
}

async function saveEngines() {
  try {
    const body = {
      listen: $("engListen").value.trim(),
      think: $("engThink").value.trim(),
      speak: $("engSpeak").value.trim(),
    };
    const data = await api("PUT", "/v1/tenant/engines", body);
    setJson("enginesJson", data);
    await refreshStatus();
  } catch (e) {
    setJson("enginesJson", { error: formatApiError(e), details: e.details || null });
    showBanner($("banner"), "bad", "Engines save failed: " + formatApiError(e));
  }
}

async function loadConfigBundle() {
  try {
    const data = await api("GET", "/v1/tenant/config");
    setJson("credJson", { credentials: data.credentials || [] });
    setJson("settingsJson", { settings: data.settings || [] });
  } catch (e) {
    setJson("credJson", { error: formatApiError(e) });
  }
}

async function saveCred() {
  try {
    const id = $("credId").value.trim();
    const key = $("credKey").value;
    if (!id || !key) throw Object.assign(new Error("gateway_id and api_key required"), { code: "bad_request" });
    const data = await api("PUT", "/v1/tenant/credentials/" + encodeURIComponent(id), { api_key: key });
    $("credKey").value = "";
    setJson("credJson", data);
    await refreshStatus();
    await loadConfigBundle();
  } catch (e) {
    setJson("credJson", { error: formatApiError(e) });
    showBanner($("banner"), "bad", "Credential save failed: " + formatApiError(e));
  }
}

async function saveSetting() {
  try {
    const key = $("setKey").value.trim();
    const value = $("setVal").value;
    if (!key) throw Object.assign(new Error("key required"), { code: "bad_request" });
    const data = await api("PUT", "/v1/tenant/settings/" + encodeURIComponent(key), { value });
    setJson("settingsJson", data);
    await loadConfigBundle();
  } catch (e) {
    setJson("settingsJson", { error: formatApiError(e) });
  }
}

function applyPreset() {
  const key = $("preset").value;
  const doc = structuredClone(presets[key]);
  $("profId").value = doc.id;
  $("profName").value = (doc.metadata && doc.metadata.display_name) || doc.id;
  $("profDoc").value = JSON.stringify(doc, null, 2);
}

async function publishProfile() {
  try {
    const id = $("profId").value.trim();
    const name = $("profName").value.trim() || id;
    let doc;
    try {
      doc = JSON.parse($("profDoc").value);
    } catch (pe) {
      throw Object.assign(new Error("profile JSON invalid: " + pe.message), { code: "bad_request" });
    }
    doc.id = id;
    try {
      await api("POST", "/v1/profiles", { id, display_name: name, tenant_id: "default" });
    } catch (e) {
      if (e.status !== 409) throw e;
    }
    const pub = await api("POST", "/v1/profiles/" + encodeURIComponent(id) + "/versions", doc);
    setJson("profilesJson", { published: pub });
    await listProfiles();
    await refreshStatus();
    showBanner($("banner"), "ok", `Published profile <code>${id}</code>.`);
  } catch (e) {
    setJson("profilesJson", { error: formatApiError(e), body: e.body || null });
    showBanner($("banner"), "bad", "Publish failed: " + formatApiError(e));
  }
}

async function listProfiles() {
  try {
    const data = await api("GET", "/v1/profiles");
    setJson("profilesJson", data);
  } catch (e) {
    setJson("profilesJson", { error: formatApiError(e) });
  }
}

$("btnRefreshStatus").onclick = () => refreshStatus();
$("btnSaveEngines").onclick = () => saveEngines();
$("btnLoadEngines").onclick = () => loadEngines();
$("btnFakeEngines").onclick = () => {
  $("engListen").value = "fake-listen";
  $("engThink").value = "fake-think";
  $("engSpeak").value = "fake-speak";
};
$("btnSarvamEngines").onclick = () => {
  $("engListen").value = "sarvam-stt";
  $("engThink").value = "sarvam-llm";
  $("engSpeak").value = "sarvam-tts";
};
$("btnSaveCred").onclick = () => saveCred();
$("btnSaveSetting").onclick = () => saveSetting();
$("btnApplyPreset").onclick = () => applyPreset();
$("btnPublish").onclick = () => publishProfile();
$("btnListProfiles").onclick = () => listProfiles();

applyPreset();
loadGateways().then(() => loadEngines()).then(() => loadConfigBundle()).then(() => listProfiles()).then(() => refreshStatus());
