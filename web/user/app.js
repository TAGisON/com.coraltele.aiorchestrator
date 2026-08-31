const { $, api, showBanner, formatApiError, loadPlatformStatus, renderReadiness } = CoralUI;

let sessionId = "";
let eventSource = null;

function setJson(id, data) {
  $(id).textContent = typeof data === "string" ? data : JSON.stringify(data, null, 2);
}

function setActive(on) {
  $("btnStart").disabled = on;
  $("btnStop").disabled = !on;
  $("btnSend").disabled = !on;
  $("btnRefreshTx").disabled = !on;
  $("msg").disabled = !on;
  $("profile").disabled = on;
  $("clock").disabled = on;
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function appendBubble(role, text, meta) {
  const div = document.createElement("div");
  div.className = "bubble " + (role === "user" ? "user" : "assistant");
  div.innerHTML = `<div class="meta">${escapeHtml(meta || role)}</div>${escapeHtml(text || "")}`;
  $("chat").appendChild(div);
  $("chat").scrollTop = $("chat").scrollHeight;
}

async function refreshReady() {
  try {
    const st = await loadPlatformStatus();
    renderReadiness($("readyList"), st);
    if (!st.ready_for_sessions) {
      showBanner($("banner"), "bad",
        `<strong>Cannot start until Admin finishes setup.</strong> ${(st.blockers || []).map(CoralUI.explainBlocker).join(" · ")}`);
      $("btnStart").disabled = true;
      $("startHint").textContent = "Blocked: " + (st.blockers || []).join(", ");
    } else if ((st.warnings || []).length) {
      showBanner($("banner"), "",
        `<strong>Startable with warnings.</strong> ${st.warnings.map(CoralUI.explainBlocker).join(" · ")}`);
      $("btnStart").disabled = !!sessionId;
      $("startHint").textContent = "Ready (degraded). Sarvam or other warnings may cause vendor failures mid-call.";
    } else {
      showBanner($("banner"), "ok", "<strong>Ready.</strong> Pick a profile and start.");
      $("btnStart").disabled = !!sessionId;
      $("startHint").textContent = "Platform reports ready_for_sessions.";
    }
    return st;
  } catch (e) {
    showBanner($("banner"), "bad", "Platform status unavailable: " + formatApiError(e));
    $("btnStart").disabled = true;
    return null;
  }
}

async function loadProfiles() {
  const sel = $("profile");
  sel.innerHTML = "";
  try {
    const data = await api("GET", "/v1/profiles");
    const list = data.profiles || [];
    if (!list.length) {
      sel.innerHTML = `<option value="">(no profiles — publish in Admin)</option>`;
      return;
    }
    list.forEach((p) => {
      const opt = document.createElement("option");
      opt.value = p.id;
      opt.textContent = (p.display_name || p.id) + " (" + p.id + ")";
      sel.appendChild(opt);
    });
  } catch (e) {
    sel.innerHTML = `<option value="">(profiles list failed)</option>`;
    showBanner($("banner"), "bad", formatApiError(e));
  }
}

function closeSSE() {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
}

function openSSE(id) {
  closeSSE();
  try {
    eventSource = new EventSource("/v1/sessions/" + encodeURIComponent(id) + "/events");
    eventSource.addEventListener("turn.completed", (ev) => {
      try {
        const payload = JSON.parse(ev.data);
        if (payload.assistant_text) appendBubble("assistant", payload.assistant_text, "assistant · live");
        else refreshTranscript();
      } catch (_) {
        refreshTranscript();
      }
    });
    eventSource.addEventListener("session.state", (ev) => {
      try {
        const payload = JSON.parse(ev.data);
        if (payload.state && /Completed|Failed|Cancelled/.test(payload.state)) {
          showBanner($("banner"), payload.state === "Failed" ? "bad" : "ok", "Session " + payload.state);
          setActive(false);
          closeSSE();
        }
      } catch (_) { /* ignore */ }
    });
    eventSource.onerror = () => {
      /* browser will retry; surface soft note */
    };
  } catch (_) { /* SSE optional */ }
}

async function startSession() {
  const profileId = $("profile").value.trim();
  if (!profileId) {
    showBanner($("banner"), "bad", "No profile selected. Publish one in Admin first.");
    return;
  }
  $("chat").innerHTML = "";
  try {
    const created = await api("POST", "/v1/sessions", {
      profile_id: profileId,
      profile_version: "latest",
      clock: $("clock").value,
    });
    sessionId = created.session_id;
    setJson("createJson", created);
    const pills = [];
    if (created.gateway_binding) {
      pills.push(`<span class="pill ok">${created.gateway_binding.listen} / ${created.gateway_binding.think} / ${created.gateway_binding.speak}</span>`);
    }
    pills.push(`<span class="pill">${created.state || "created"}</span>`);
    pills.push(`<span class="pill"><code>${sessionId}</code></span>`);
    $("sessionMeta").innerHTML = pills.join(" ");
    setActive(true);
    openSSE(sessionId);
    showBanner($("banner"), "ok", "Session started. Send a message to run a Talk turn.");
    await refreshTranscript();
  } catch (e) {
    setJson("createJson", { error: formatApiError(e), details: e.details || null, body: e.body || null });
    let tip = formatApiError(e);
    if (e.code === "bad_request" && /engines not configured/i.test(e.message || "")) {
      tip += " — open Admin → Tenant engines.";
    }
    if (e.status === 404) tip += " — profile missing; publish in Admin.";
    showBanner($("banner"), "bad", "Start failed: " + tip);
    setActive(false);
    sessionId = "";
  }
}

async function sendTurn() {
  const text = $("msg").value.trim();
  if (!sessionId || !text) return;
  appendBubble("user", text, "you");
  $("msg").value = "";
  try {
    const data = await api("POST", "/v1/sessions/" + encodeURIComponent(sessionId) + "/inject", {
      text,
      speak: true,
    });
    setJson("turnJson", data);
    await refreshTranscript();
  } catch (e) {
    setJson("turnJson", { error: formatApiError(e), body: e.body || null });
    showBanner($("banner"), "bad", "Turn failed: " + formatApiError(e) +
      " (vendor down, missing credential, or session not Running)");
    appendBubble("assistant", "⚠ " + formatApiError(e), "error");
  }
}

async function refreshTranscript() {
  if (!sessionId) return;
  try {
    const tr = await api("GET", "/v1/sessions/" + encodeURIComponent(sessionId) + "/transcript");
    const turns = tr.turns || [];
    $("chat").innerHTML = "";
    turns.forEach((t) => appendBubble(t.role || "assistant", t.text || "", (t.role || "") + " · seq " + t.seq));
  } catch (e) {
    setJson("turnJson", { transcript_error: formatApiError(e) });
  }
}

async function stopSession() {
  if (!sessionId) return;
  try {
    const data = await api("POST", "/v1/sessions/" + encodeURIComponent(sessionId) + "/stop", { reason: "user" });
    setJson("createJson", data);
    showBanner($("banner"), "ok", "Session stopped. Open Supervisor to inspect audit / disposition.");
  } catch (e) {
    showBanner($("banner"), "bad", "Stop failed: " + formatApiError(e));
  }
  closeSSE();
  setActive(false);
  sessionId = "";
  await refreshReady();
}

$("btnReady").onclick = () => refreshReady();
$("btnStart").onclick = () => startSession();
$("btnStop").onclick = () => stopSession();
$("btnSend").onclick = () => sendTurn();
$("btnRefreshTx").onclick = () => refreshTranscript();
$("msg").addEventListener("keydown", (ev) => {
  if (ev.key === "Enter") {
    ev.preventDefault();
    sendTurn();
  }
});

setActive(false);
loadProfiles().then(() => refreshReady());
