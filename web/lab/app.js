const API = ""; // same origin

const presets = {
  fakes: {
    id: "lab-fakes",
    modes: { listen: true, speak: true, think: true },
    audio: { canonical_sample_rate_hz: 16000 },
    routers: {
      listen: { providers: ["fake-listen"] },
      speak: { providers: ["fake-speak"] },
      think: { providers: ["fake-think"] },
    },
  },
  sarvam: {
    id: "lab-sarvam-failover",
    modes: { listen: true, speak: true, think: true },
    audio: { canonical_sample_rate_hz: 16000 },
    routers: {
      listen: { providers: ["sarvam-stt", "fake-listen"] },
      speak: { providers: ["sarvam-tts", "fake-speak"] },
      think: { providers: ["sarvam-llm", "fake-think"] },
    },
  },
  captions: {
    id: "job-captions",
    modes: { listen: true, speak: false, think: false },
    audio: { canonical_sample_rate_hz: 16000 },
    language: { behaviour: "captions", primary: "en-IN" },
    routers: {
      listen: { providers: ["fake-listen"] },
    },
  },
};

let sse = null;
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

async function loadGateways() {
  const data = await api("GET", "/v1/gateways");
  document.getElementById("gatewaysJson").textContent = JSON.stringify(data, null, 2);
}

function applyPreset() {
  const key = document.getElementById("preset").value;
  const doc = structuredClone(presets[key]);
  document.getElementById("pubId").value = doc.id;
  document.getElementById("profId").value = doc.id;
  document.getElementById("profDoc").value = JSON.stringify(doc, null, 2);
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
  await loadSessions();
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

async function inspectSession() {
  const id = document.getElementById("inspId").value.trim();
  if (!id) return;
  const sess = await api("GET", `/v1/sessions/${encodeURIComponent(id)}`);
  document.getElementById("inspSession").textContent = JSON.stringify(sess, null, 2);
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
