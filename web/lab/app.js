const API = ""; // same origin

/** Contact Agent behaviour presets — engines come from tenant binding, not profile failover. */
const ccBase = {
  modes: { listen: true, speak: true, think: true, talk: true },
  audio: { canonical_sample_rate_hz: 16000 },
  language: {
    behaviour: "none",
    auto_detect: true,
    mid_call_switch: true,
    allowed: ["en-IN", "hi-IN"],
  },
  hot_swap_allowed: ["language.primary"],
  fallback: {
    listen_down: { speak_canned: "clip-apology-en", skill: "warm_transfer" },
    think_down: { speak_canned: "clip-apology-en", skill: "warm_transfer" },
    speak_down: { text_sink: true, skill: "warm_transfer" },
  },
  skills: {
    allowed: ["warm_transfer"],
    definitions: {
      warm_transfer: {
        gateway: "coral-transfer",
        authority: "act",
        confirm: true,
      },
    },
  },
};

const presets = {
  "cc-sales": {
    ...structuredClone(ccBase),
    id: "cc-sales",
    metadata: { display_name: "CC Sales", family: "contact-agent" },
    persona: {
      name: "Sales Mira",
      instructions: "You are a concise sales contact agent. Qualify interest; never invent pricing.",
      voice: { "fake-speak": "lab-voice-sales", "sarvam-tts": "shubh" },
    },
    response: {
      ladder: ["clip", "template", "llm"],
      clips: {
        "clip-apology-en": { text: "Sorry — connecting you to a sales specialist." },
        "greeting-en": {
          text: "Hi — Sales desk. How can I help today?",
          when: { regex: "(?i)\\b(hi|hello|hey)\\b" },
        },
        "pricing-en": {
          text: "I can share list pricing after we confirm your product interest.",
          when: { regex: "(?i)\\b(price|pricing|cost|quote)\\b" },
        },
      },
      templates: {
        clarify: {
          text: "Which product line are you asking about?",
          when: { regex: "(?i)\\b(what|huh|which)\\b" },
        },
      },
    },
  },
  "cc-rnd": {
    ...structuredClone(ccBase),
    id: "cc-rnd",
    metadata: { display_name: "CC R&D", family: "contact-agent" },
    persona: {
      name: "R&D Kai",
      instructions: "You are an R&D desk agent. Prefer technical clarity; escalate speculative claims.",
      voice: { "fake-speak": "lab-voice-rnd", "sarvam-tts": "meera" },
    },
    response: {
      ladder: ["clip", "template", "llm"],
      clips: {
        "clip-apology-en": { text: "Sorry — transferring you to an engineer." },
        "greeting-en": {
          text: "R&D desk — describe the symptom or API error.",
          when: { regex: "(?i)\\b(hi|hello|hey)\\b" },
        },
        "bug-en": {
          text: "Please share the error code and last successful step.",
          when: { regex: "(?i)\\b(bug|crash|error|stack)\\b" },
        },
      },
      templates: {
        clarify: {
          text: "Is this reproducible in lab or only production?",
          when: { regex: "(?i)\\b(what|huh|where)\\b" },
        },
      },
    },
  },
  "cc-after-hours": {
    ...structuredClone(ccBase),
    id: "cc-after-hours",
    metadata: { display_name: "CC after-hours", family: "contact-agent" },
    persona: {
      name: "Night Desk",
      instructions: "After-hours contact agent. Contain urgent issues; warm-transfer when needed.",
      voice: { "fake-speak": "lab-voice-night", "sarvam-tts": "anushka" },
    },
    response: {
      ladder: ["clip", "template", "llm"],
      clips: {
        "clip-apology-en": { text: "Sorry — an on-call agent will take over shortly." },
        "greeting-en": {
          text: "After-hours desk. Leave a brief urgency note.",
          when: { regex: "(?i)\\b(hi|hello|hey)\\b" },
        },
        "urgent-en": {
          text: "Flagged as urgent — holding while we escalate.",
          when: { regex: "(?i)\\b(urgent|emergency|outage)\\b" },
        },
      },
      templates: {
        clarify: {
          text: "Is this blocking callers right now?",
          when: { regex: "(?i)\\b(what|huh|when)\\b" },
        },
      },
    },
  },
  // Legacy / non-CC — profile-owned routers (may include failover lists).
  fakes: {
    id: "lab-fakes",
    metadata: { display_name: "Lab fakes (legacy)", family: "lab" },
    modes: { listen: true, speak: true, think: true },
    audio: { canonical_sample_rate_hz: 16000 },
    persona: { voice: { "fake-speak": "lab-voice" } },
    routers: {
      listen: { providers: ["fake-listen"] },
      speak: { providers: ["fake-speak"] },
      think: { providers: ["fake-think"] },
    },
  },
  sarvam: {
    id: "lab-sarvam-failover",
    metadata: { display_name: "Sarvam+fake failover (legacy non-CC)", family: "lab" },
    modes: { listen: true, speak: true, think: true },
    audio: { canonical_sample_rate_hz: 16000 },
    persona: { voice: { "sarvam-tts": "shubh", "fake-speak": "lab-voice" } },
    routers: {
      listen: { providers: ["sarvam-stt", "fake-listen"] },
      speak: { providers: ["sarvam-tts", "fake-speak"] },
      think: { providers: ["sarvam-llm", "fake-think"] },
    },
  },
  captions: {
    id: "job-captions",
    metadata: { display_name: "Captions listen-only (legacy)", family: "captions" },
    modes: { listen: true, speak: false, think: false },
    audio: { canonical_sample_rate_hz: 16000 },
    language: { behaviour: "captions", primary: "en-IN" },
    routers: {
      listen: { providers: ["fake-listen"] },
    },
  },
};

let sse = null;
let gatewayCatalog = null;
const apiLogEl = () => document.getElementById("apiLog");

function logApi(method, path, status, body) {
  const line = `[${new Date().toISOString()}] ${method} ${path} → ${status}\n${typeof body === "string" ? body : JSON.stringify(body, null, 2)}\n\n`;
  const el = apiLogEl();
  el.textContent = line + el.textContent;
}

async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  const token = localStorage.getItem("CONTROL_AUTH_TOKEN");
  if (token) opts.headers["Authorization"] = "Bearer " + token;
  const res = await fetch(API + path, opts);
  const text = await res.text();
  let data;
  try { data = JSON.parse(text); } catch { data = text; }
  logApi(method, path, res.status, data);
  if (!res.ok) throw Object({ status: res.status, data });
  return data;
}

function clearApiLog() { apiLogEl().textContent = ""; }

document.querySelectorAll("nav button").forEach((btn) => {
  btn.addEventListener("click", () => {
    document.querySelectorAll("nav button").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    document.querySelectorAll(".tab").forEach((t) => { t.hidden = true; });
    document.getElementById("tab-" + btn.dataset.tab).hidden = false;
    if (btn.dataset.tab === "engines") loadEngines().catch(() => {});
    if (btn.dataset.tab === "settings") loadTenantConfig().catch(() => {});
  });
});

function setPill(id, text, ok) {
  const el = document.getElementById(id);
  el.textContent = text;
  el.classList.remove("ok", "bad");
  if (ok === true) el.classList.add("ok");
  if (ok === false) el.classList.add("bad");
}

async function refreshOverview() {
  try {
    const h = await api("GET", "/v1/health");
    const lab = await api("GET", "/v1/lab/status");
    document.getElementById("overviewJson").textContent = JSON.stringify({ health: h, lab }, null, 2);
    setPill("pillHealth", "health " + h.status, h.status === "ok");
    setPill("pillStore", "store " + (lab.store_backend || "?"), lab.store_backend === "postgres" && lab.db);
    setPill("pillSarvam", "sarvam " + (lab.sarvam_configured ? "on" : "off"), lab.sarvam_configured);
  } catch (e) {
    setPill("pillHealth", "health down", false);
    document.getElementById("overviewJson").textContent = JSON.stringify(e, null, 2);
  }
}

function fillGatewayLists(data) {
  gatewayCatalog = data;
  const byPort = { listen: [], think: [], speak: [] };
  const items = data.gateways || data || [];
  const list = Array.isArray(items) ? items : Object.values(items).flat();
  for (const g of list) {
    const id = g.id || g.gateway_id || g;
    const port = (g.port || g.kind || "").toLowerCase();
    if (port.includes("listen") && byPort.listen.indexOf(id) < 0) byPort.listen.push(id);
    if (port.includes("think") && byPort.think.indexOf(id) < 0) byPort.think.push(id);
    if (port.includes("speak") && byPort.speak.indexOf(id) < 0) byPort.speak.push(id);
  }
  const fill = (listId, ids) => {
    const el = document.getElementById(listId);
    el.innerHTML = ids.map((id) => `<option value="${id}"></option>`).join("");
  };
  fill("gwListenList", byPort.listen.length ? byPort.listen : ["fake-listen", "sarvam-stt"]);
  fill("gwThinkList", byPort.think.length ? byPort.think : ["fake-think", "sarvam-llm"]);
  fill("gwSpeakList", byPort.speak.length ? byPort.speak : ["fake-speak", "sarvam-tts"]);
}

async function loadGateways() {
  const data = await api("GET", "/v1/gateways");
  document.getElementById("gatewaysJson").textContent = JSON.stringify(data, null, 2);
  fillGatewayLists(data);
}

async function loadEngines() {
  try {
    if (!gatewayCatalog) await loadGateways().catch(() => {});
    const data = await api("GET", "/v1/tenant/engines");
    document.getElementById("engListen").value = data.listen || "";
    document.getElementById("engThink").value = data.think || "";
    document.getElementById("engSpeak").value = data.speak || "";
    document.getElementById("engSource").textContent = data.source || "?";
    document.getElementById("engTenant").textContent = data.tenant_id || "default";
    document.getElementById("enginesJson").textContent = JSON.stringify(data, null, 2);
  } catch (e) {
    document.getElementById("enginesJson").textContent = JSON.stringify(e, null, 2);
  }
}

async function saveEngines() {
  const body = {
    listen: document.getElementById("engListen").value.trim(),
    think: document.getElementById("engThink").value.trim(),
    speak: document.getElementById("engSpeak").value.trim(),
  };
  const data = await api("PUT", "/v1/tenant/engines", body);
  document.getElementById("engSource").textContent = data.source || "store";
  document.getElementById("engTenant").textContent = data.tenant_id || "default";
  document.getElementById("enginesJson").textContent = JSON.stringify(data, null, 2);
}

function suggestFakeEngines() {
  document.getElementById("engListen").value = "fake-listen";
  document.getElementById("engThink").value = "fake-think";
  document.getElementById("engSpeak").value = "fake-speak";
}

function suggestSarvamEngines() {
  document.getElementById("engListen").value = "sarvam-stt";
  document.getElementById("engThink").value = "sarvam-llm";
  document.getElementById("engSpeak").value = "sarvam-tts";
}

async function loadTenantConfig() {
  try {
    const data = await api("GET", "/v1/tenant/config");
    document.getElementById("settingsJson").textContent = JSON.stringify(data, null, 2);
  } catch (e) {
    document.getElementById("settingsJson").textContent = JSON.stringify(e, null, 2);
  }
}

async function saveCredential() {
  const gatewayId = document.getElementById("credGateway").value.trim();
  const apiKey = document.getElementById("credApiKey").value;
  if (!gatewayId || !apiKey) {
    alert("gateway_id and api_key required");
    return;
  }
  const data = await api("PUT", "/v1/tenant/credentials/" + encodeURIComponent(gatewayId), { api_key: apiKey });
  document.getElementById("credApiKey").value = "";
  document.getElementById("settingsJson").textContent = JSON.stringify(data, null, 2);
  await loadTenantConfig().catch(() => {});
  await refreshOverview().catch(() => {});
}

async function saveSetting() {
  const key = document.getElementById("setKey").value.trim();
  const value = document.getElementById("setValue").value;
  if (!key) {
    alert("key required");
    return;
  }
  const data = await api("PUT", "/v1/tenant/settings/" + encodeURIComponent(key), { value });
  document.getElementById("settingsJson").textContent = JSON.stringify(data, null, 2);
  await loadTenantConfig().catch(() => {});
}

function applyPreset() {
  const key = document.getElementById("preset").value;
  const doc = structuredClone(presets[key]);
  document.getElementById("pubId").value = doc.id;
  document.getElementById("profId").value = doc.id;
  if (doc.metadata && doc.metadata.display_name) {
    document.getElementById("profName").value = doc.metadata.display_name;
  }
  document.getElementById("profDoc").value = JSON.stringify(doc, null, 2);
  if (String(key).startsWith("cc-")) {
    document.getElementById("sessProfile").value = doc.id;
  }
}

async function createProfile() {
  const id = document.getElementById("profId").value.trim();
  const display_name = document.getElementById("profName").value.trim();
  await api("POST", "/v1/profiles", { id, display_name });
  await loadProfiles();
}

async function publishProfile() {
  const id = document.getElementById("pubId").value.trim();
  const doc = JSON.parse(document.getElementById("profDoc").value);
  doc.id = id;
  await api("POST", `/v1/profiles/${encodeURIComponent(id)}/versions`, doc);
  await loadProfiles();
}

async function loadProfiles() {
  const data = await api("GET", "/v1/profiles");
  const tb = document.getElementById("profTable");
  tb.innerHTML = "";
  for (const p of data.profiles || []) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td>${p.id}</td><td>${p.display_name || ""}</td><td>${p.updated_at || ""}</td>
      <td><button class="action secondary" data-id="${p.id}">View latest</button></td>`;
    tr.querySelector("button").onclick = async () => {
      const v = await api("GET", `/v1/profiles/${encodeURIComponent(p.id)}/versions/latest`);
      document.getElementById("profDoc").value = JSON.stringify(v.document, null, 2);
      document.getElementById("pubId").value = p.id;
    };
    tb.appendChild(tr);
  }
}

async function createSession() {
  const profile_id = document.getElementById("sessProfile").value.trim();
  const clock = document.getElementById("sessClock").value;
  const data = await api("POST", "/v1/sessions", { profile_id, profile_version: "latest", clock });
  document.getElementById("inspId").value = data.session_id;
  document.getElementById("helperSid").value = data.session_id;
  document.getElementById("sessBinding").textContent = JSON.stringify({
    session_id: data.session_id,
    gateway_binding: data.gateway_binding,
    state: data.state,
    profile_id: data.profile_id,
  }, null, 2);
  await loadSessions();
}

async function injectText() {
  const id = document.getElementById("helperSid").value.trim() || document.getElementById("inspId").value.trim();
  const text = document.getElementById("injectText").value;
  if (!id) return;
  const data = await api("POST", `/v1/sessions/${encodeURIComponent(id)}/inject`, { text, speak: true });
  document.getElementById("sessBinding").textContent = JSON.stringify(data, null, 2);
}

async function patchLanguage() {
  const id = document.getElementById("helperSid").value.trim() || document.getElementById("inspId").value.trim();
  const primary = document.getElementById("patchLang").value.trim();
  if (!id) return;
  const data = await api("PATCH", `/v1/sessions/${encodeURIComponent(id)}/profile-fields`, {
    "language.primary": primary,
  });
  document.getElementById("sessBinding").textContent = JSON.stringify(data, null, 2);
}

function openInspectHelper() {
  const id = document.getElementById("helperSid").value.trim();
  if (id) document.getElementById("inspId").value = id;
  document.querySelector('nav button[data-tab="inspect"]').click();
  inspectSession().catch(() => {});
}

async function loadSessions() {
  const data = await api("GET", "/v1/sessions");
  const tb = document.getElementById("sessTable");
  tb.innerHTML = "";
  for (const s of data.sessions || []) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td style="font-family:var(--mono);font-size:0.75rem">${s.session_id}</td>
      <td>${s.profile_id}@${s.profile_version}</td><td>${s.state}</td><td>${s.clock}</td>
      <td>
        <button class="action secondary" data-act="insp">Inspect</button>
        <button class="action secondary" data-act="stop">Stop</button>
      </td>`;
    tr.querySelector('[data-act="insp"]').onclick = () => {
      document.getElementById("inspId").value = s.session_id;
      document.getElementById("helperSid").value = s.session_id;
      inspectSession();
      document.querySelector('nav button[data-tab="inspect"]').click();
    };
    tr.querySelector('[data-act="stop"]').onclick = () => stopSession(s.session_id);
    tb.appendChild(tr);
  }
}

async function stopSession(id) {
  if (!id) return;
  await api("POST", `/v1/sessions/${encodeURIComponent(id)}/stop`, { reason: "lab" });
  await loadSessions();
  await inspectSession();
}

function summarizeSession(sess) {
  const gb = sess.gateway_binding || {};
  const parts = [
    `gateway_binding: listen=${gb.listen || "—"} think=${gb.think || "—"} speak=${gb.speak || "—"}`,
    `detected_language=${sess.detected_language || "—"}`,
    `active_language=${sess.active_language || "—"}`,
  ];
  document.getElementById("inspSummary").textContent = parts.join(" · ");
}

async function inspectSession() {
  const id = document.getElementById("inspId").value.trim();
  if (!id) return;
  document.getElementById("helperSid").value = id;
  const sess = await api("GET", `/v1/sessions/${encodeURIComponent(id)}`);
  document.getElementById("inspSession").textContent = JSON.stringify(sess, null, 2);
  summarizeSession(sess);
  try {
    const audit = await api("GET", `/v1/sessions/${encodeURIComponent(id)}/audit`);
    document.getElementById("inspAudit").textContent = JSON.stringify(audit, null, 2);
  } catch (e) {
    document.getElementById("inspAudit").textContent = JSON.stringify(e, null, 2);
  }
  try {
    const an = await api("GET", `/v1/sessions/${encodeURIComponent(id)}/analytics`);
    document.getElementById("inspAnalytics").textContent = JSON.stringify(an, null, 2);
  } catch (e) {
    document.getElementById("inspAnalytics").textContent = JSON.stringify(e, null, 2);
  }
  await loadTranscript().catch(() => {});
  await loadDisposition().catch(() => {});
}

async function loadTranscript() {
  const id = document.getElementById("inspId").value.trim();
  if (!id) return;
  try {
    const data = await api("GET", `/v1/sessions/${encodeURIComponent(id)}/transcript`);
    document.getElementById("inspTranscript").textContent = JSON.stringify(data, null, 2);
  } catch (e) {
    document.getElementById("inspTranscript").textContent = JSON.stringify(e, null, 2);
  }
}

async function loadDisposition() {
  const id = document.getElementById("inspId").value.trim();
  if (!id) return;
  try {
    const data = await api("GET", `/v1/sessions/${encodeURIComponent(id)}/disposition`);
    document.getElementById("inspDisposition").textContent = JSON.stringify(data, null, 2);
  } catch (e) {
    document.getElementById("inspDisposition").textContent = JSON.stringify(e, null, 2);
  }
}

function toggleSSE() {
  const id = document.getElementById("inspId").value.trim();
  const el = document.getElementById("inspSSE");
  if (sse) {
    sse.close();
    sse = null;
    el.textContent = (el.textContent || "") + "\n[SSE closed]\n";
    return;
  }
  if (!id) return;
  el.textContent = "";
  sse = new EventSource(`/v1/sessions/${encodeURIComponent(id)}/events`);
  sse.onmessage = (ev) => {
    el.textContent += ev.data + "\n";
  };
  sse.onerror = () => {
    el.textContent += "[SSE error / closed]\n";
  };
}

async function createPlayback() {
  const profile_id = document.getElementById("pbProfile").value.trim();
  const file_uri = document.getElementById("pbUri").value.trim();
  const data = await api("POST", "/v1/jobs/playback", { profile_id, profile_version: "latest", file_uri });
  document.getElementById("pbOut").textContent = JSON.stringify(data, null, 2);
}

applyPreset();
refreshOverview();
loadProfiles().catch(() => {});
loadSessions().catch(() => {});
loadGateways().catch(() => {});
loadEngines().catch(() => {});
