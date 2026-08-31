const { $, api, showBanner, formatApiError } = CoralUI;

let selected = "";

function setJson(id, data) {
  $(id).textContent = typeof data === "string" ? data : JSON.stringify(data, null, 2);
}

function statePill(state) {
  const s = String(state || "");
  let cls = "";
  if (/Completed|Cancelled/.test(s)) cls = "ok";
  else if (/Failed/.test(s)) cls = "bad";
  else if (/Running|Attached|Created|Draining/.test(s)) cls = "warn";
  return `<span class="pill ${cls}">${s || "?"}</span>`;
}

async function refreshList() {
  try {
    const limit = Number($("limit").value) || 50;
    const data = await api("GET", "/v1/sessions?limit=" + limit);
    const rows = data.sessions || [];
    $("sessEmpty").hidden = rows.length > 0;
    $("sessBody").innerHTML = rows.map((s) => {
      const id = s.session_id;
      const sel = id === selected ? " selected" : "";
      return `<tr class="clickable${sel}" data-id="${id}">
        <td><code>${id.slice(0, 12)}…</code></td>
        <td>${statePill(s.state)}</td>
        <td>${s.profile_id || ""} v${s.profile_version || "?"}</td>
        <td>${s.updated_at || ""}</td>
      </tr>`;
    }).join("");
    $("sessBody").querySelectorAll("tr.clickable").forEach((tr) => {
      tr.onclick = () => {
        selected = tr.getAttribute("data-id");
        $("sid").value = selected;
        inspect(selected);
        refreshList();
      };
    });
    if (!rows.length) {
      showBanner($("banner"), "", "No sessions in store. Use User console to create one.");
    } else {
      showBanner($("banner"), "ok", `${rows.length} session(s) loaded.`);
    }
  } catch (e) {
    showBanner($("banner"), "bad", formatApiError(e));
    setJson("sessJson", { error: formatApiError(e) });
  }
}

function renderTranscript(data) {
  const turns = (data && data.turns) || [];
  const chat = $("transcriptChat");
  if (!turns.length) {
    chat.innerHTML = `<div class="hint">No transcript turns.</div>`;
    return;
  }
  chat.innerHTML = turns.map((t) => {
    const role = t.role || "unknown";
    const cls = role === "user" ? "user" : "assistant";
    return `<div class="bubble ${cls}"><div class="meta">${role} · seq ${t.seq}${t.turn_id ? " · " + t.turn_id : ""}</div>${escapeHtml(t.text || "")}</div>`;
  }).join("");
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

async function inspect(id) {
  if (!id) {
    showBanner($("banner"), "bad", "session_id required");
    return;
  }
  selected = id;
  $("metaPills").innerHTML = "";
  try {
    const sess = await api("GET", "/v1/sessions/" + encodeURIComponent(id));
    setJson("sessJson", sess);
    const pills = [
      statePill(sess.state),
      `<span class="pill">profile ${sess.profile_id || "?"} v${sess.profile_version || "?"}</span>`,
    ];
    if (sess.gateway_binding) {
      pills.push(`<span class="pill">${sess.gateway_binding.listen} / ${sess.gateway_binding.think} / ${sess.gateway_binding.speak}</span>`);
    }
    if (sess.active_language) pills.push(`<span class="pill">lang ${sess.active_language}</span>`);
    if (sess.recording_ref) pills.push(`<span class="pill warn">recording_ref set (external)</span>`);
    else pills.push(`<span class="pill">no recording_ref (orchestrator does not store call audio)</span>`);
    $("metaPills").innerHTML = pills.join(" ");
  } catch (e) {
    setJson("sessJson", { error: formatApiError(e) });
    showBanner($("banner"), "bad", "Session load failed: " + formatApiError(e));
  }

  try {
    const tr = await api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/transcript");
    setJson("transcriptJson", tr);
    renderTranscript(tr);
  } catch (e) {
    renderTranscript({ turns: [] });
    setJson("transcriptJson", { error: formatApiError(e) });
  }

  try {
    const d = await api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/disposition");
    setJson("dispJson", d);
  } catch (e) {
    setJson("dispJson", {
      note: e.status === 404 ? "Disposition not written yet (expected until postcall after stop)." : formatApiError(e),
      error: formatApiError(e),
    });
  }

  try {
    const a = await api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/audit");
    setJson("auditJson", a);
  } catch (e) {
    setJson("auditJson", { error: formatApiError(e) });
  }

  try {
    const an = await api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/analytics");
    setJson("analyticsJson", an);
  } catch (e) {
    setJson("analyticsJson", { error: formatApiError(e) });
  }
}

async function forceStop() {
  const id = $("sid").value.trim();
  if (!id) return;
  try {
    const data = await api("POST", "/v1/sessions/" + encodeURIComponent(id) + "/stop", { reason: "supervisor" });
    setJson("sessJson", data);
    await refreshList();
    await inspect(id);
    showBanner($("banner"), "ok", "Stop requested.");
  } catch (e) {
    showBanner($("banner"), "bad", formatApiError(e));
  }
}

$("btnRefresh").onclick = () => refreshList();
$("btnLoad").onclick = () => inspect($("sid").value.trim());
$("btnStop").onclick = () => forceStop();

refreshList();
