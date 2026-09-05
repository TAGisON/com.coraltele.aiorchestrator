/* User chat console (C.2) — clock=chat via Control create/answer/inject/transcript. */
(function () {
  let sessionId = "";
  let pollTimer = null;
  let pins = [];
  let profiles = [];
  let flows = [];

  function el(id) {
    return document.getElementById(id);
  }

  function show(node, ok, msg) {
    if (!node) return;
    node.className = "probe " + (ok ? "ok" : "err");
    node.textContent = msg;
  }

  function fillSelect(sel, ids, selected) {
    if (!sel) return;
    const cur = selected || sel.value || "";
    sel.innerHTML = "";
    const blank = document.createElement("option");
    blank.value = "";
    blank.textContent = ids && ids.length ? "— select —" : "— none registered —";
    sel.appendChild(blank);
    (ids || []).forEach((id) => {
      const opt = document.createElement("option");
      opt.value = id;
      opt.textContent = id;
      sel.appendChild(opt);
    });
    if (cur && ids && ids.indexOf(cur) >= 0) sel.value = cur;
  }

  function applyPinForProfile() {
    const pid = el("profile").value;
    const pin = pins.find((p) => p.profile_id === pid);
    if (!pin) return;
    if (pin.flow_id) el("flow").value = pin.flow_id;
    if (pin.flow_version) el("flow-ver").value = pin.flow_version;
  }

  function stopPoll() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  function startPoll() {
    stopPoll();
    pollTimer = setInterval(() => {
      if (!sessionId) return;
      refreshTranscript().catch(() => {});
      refreshSession().catch(() => {});
    }, 1500);
  }

	function turnClass(t) {
    const kind = (t.event_kind || "").toLowerCase();
    if (kind === "tool_line") return "tool";
    if (kind === "edge_taken") return "edge";
    if (t.role === "user" || kind === "user_final") return "user";
    if (t.role === "assistant" || kind === "bot_utterance") return "bot";
    return "meta";
  }

  function renderTurns(turns) {
    const box = el("turns");
    box.innerHTML = "";
    if (!turns || !turns.length) {
      const empty = document.createElement("div");
      empty.className = "chat-turn meta";
      empty.textContent = "No turns yet — Start chat answers the welcome node.";
      box.appendChild(empty);
      return;
    }
    turns.forEach((t) => {
      const row = document.createElement("div");
      row.className = "chat-turn " + turnClass(t);
      const meta = document.createElement("div");
      meta.className = "meta-line";
      meta.textContent =
        (t.role || "?") +
        (t.event_kind ? " · " + t.event_kind : "") +
        (t.seq != null ? " · #" + t.seq : "");
      const text = document.createElement("div");
      text.textContent = t.text || "";
      row.appendChild(meta);
      row.appendChild(text);
      box.appendChild(row);
    });
    box.scrollTop = box.scrollHeight;
  }

  async function refreshTranscript() {
    if (!sessionId) return;
    const data = await OrchAPI.getTranscript(sessionId);
    renderTurns(data.turns || []);
  }

  async function refreshSession() {
    if (!sessionId) return;
    const s = await OrchAPI.getSession(sessionId);
    el("sess-state").textContent = s.state || "—";
    const terminal = /completed|stopped|failed|draining/i.test(s.state || "");
    if (terminal) {
      stopPoll();
      try {
        const d = await OrchAPI.getDisposition(sessionId);
        const final = d.final != null ? d.final : "(none)";
        show(el("disp-out"), true, "Disposition final: " + final);
      } catch (_) {
        /* disposition may be absent mid-flight */
      }
    }
  }

  async function loadCatalogs() {
    const [plist, flist, pinRes] = await Promise.all([
      OrchAPI.listProfiles(),
      OrchAPI.listFlows(),
      OrchAPI.getAnswerPins().catch(() => ({ pins: [] })),
    ]);
    profiles = (plist && (plist.profiles || plist.items || plist)) || [];
    if (!Array.isArray(profiles)) profiles = [];
    flows = (flist && (flist.flows || flist.items || flist)) || [];
    if (!Array.isArray(flows)) flows = [];
    pins = (pinRes && pinRes.pins) || [];
    fillSelect(
      el("profile"),
      profiles.map((p) => p.id || p).filter(Boolean),
      el("profile").value
    );
    fillSelect(
      el("flow"),
      flows.map((f) => f.id || f).filter(Boolean),
      el("flow").value
    );
    applyPinForProfile();
    show(el("start-out"), true, "Loaded " + profiles.length + " profiles, " + flows.length + " flows.");
  }

  async function startChat() {
    const profile_id = el("profile").value;
    const flow_id = el("flow").value;
    const flow_version = (el("flow-ver").value || "latest").trim();
    if (!profile_id || !flow_id) {
      show(el("start-out"), false, "profile and flow required");
      return;
    }
    const caller = {};
    const ani = el("caller-ani").value.trim();
    const name = el("caller-name").value.trim();
    if (ani) caller.ani = ani;
    if (name) caller.caller_id_name = name;

    const body = {
      profile_id: profile_id,
      profile_version: "latest",
      flow_id: flow_id,
      flow_version: flow_version === "latest" ? "latest" : Number(flow_version) || flow_version,
      clock: "chat",
    };
    if (Object.keys(caller).length) body.caller = caller;

    show(el("start-out"), true, "Creating session…");
    const created = await OrchAPI.createSession(body);
    sessionId = created.session_id;
    el("sess-id").textContent = sessionId;
    el("sess-state").textContent = created.state || "created";
    el("session-card").hidden = false;
    el("disp-out").textContent = "";
    el("disp-out").className = "probe";

    const ans = await OrchAPI.answerSession(sessionId, {});
    show(
      el("sess-out"),
      true,
      "Answered" + (ans.spoken ? ": " + ans.spoken : ".")
    );
    await refreshTranscript();
    await refreshSession();
    startPoll();
    show(el("start-out"), true, "Session " + sessionId + " (clock=chat)");
  }

  async function sendTurn() {
    const text = el("msg").value.trim();
    if (!text) {
      show(el("sess-out"), false, "text required");
      return;
    }
    if (!sessionId) {
      show(el("sess-out"), false, "no session");
      return;
    }
    await OrchAPI.injectText(sessionId, { text: text, speak: true });
    el("msg").value = "";
    show(el("sess-out"), true, "Sent.");
    await refreshTranscript();
    await refreshSession();
  }

  async function stopChat() {
    if (!sessionId) return;
    await OrchAPI.stopSession(sessionId, { reason: "operator" });
    stopPoll();
    await refreshSession();
    await refreshTranscript();
    show(el("sess-out"), true, "Stopped.");
  }

  function bind() {
    el("token").value = OrchAPI.token();
    el("save-token").onclick = () => {
      OrchAPI.setToken(el("token").value);
      show(el("token-out"), true, "Token saved.");
    };
    el("profile").onchange = applyPinForProfile;
    el("btn-reload").onclick = () =>
      loadCatalogs().catch((e) => show(el("start-out"), false, e.message || String(e)));
    el("btn-start").onclick = () =>
      startChat().catch((e) => show(el("start-out"), false, e.message || String(e)));
    el("btn-send").onclick = () =>
      sendTurn().catch((e) => show(el("sess-out"), false, e.message || String(e)));
    el("btn-refresh").onclick = () =>
      refreshTranscript()
        .then(() => refreshSession())
        .catch((e) => show(el("sess-out"), false, e.message || String(e)));
    el("btn-stop").onclick = () =>
      stopChat().catch((e) => show(el("sess-out"), false, e.message || String(e)));
    el("msg").addEventListener("keydown", (ev) => {
      if (ev.key === "Enter" && !ev.shiftKey) {
        ev.preventDefault();
        el("btn-send").click();
      }
    });
  }

  bind();
  loadCatalogs().catch((e) => show(el("start-out"), false, e.message || String(e)));
})();
