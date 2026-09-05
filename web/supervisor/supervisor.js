/* Supervisor console (S.1) — read-only session list + detail. */
(function () {
  let selectedId = "";

  function el(id) {
    return document.getElementById(id);
  }

  function show(node, ok, msg) {
    if (!node) return;
    node.className = "probe " + (ok === true ? "ok" : ok === false ? "err" : "");
    node.textContent = msg;
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
      empty.textContent = "No transcript turns.";
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
  }

  function shortId(id) {
    if (!id) return "—";
    return id.length > 12 ? id.slice(0, 8) + "…" : id;
  }

  function flowLabel(s) {
    if (!s.flow_id) return "—";
    return s.flow_id + (s.flow_version != null ? "@" + s.flow_version : "");
  }

  function renderList(sessions) {
    const body = el("sess-body");
    body.innerHTML = "";
    if (!sessions || !sessions.length) {
      show(el("list-out"), true, "No sessions yet.");
      return;
    }
    show(el("list-out"), true, sessions.length + " session(s)");
    sessions.forEach((s) => {
      const tr = document.createElement("tr");
      tr.className = s.session_id === selectedId ? "row-active" : "";
      tr.innerHTML =
        "<td><button type=\"button\" class=\"linkish\">" +
        shortId(s.session_id) +
        "</button></td>" +
        "<td>" +
        (s.clock || "—") +
        "</td>" +
        "<td>" +
        (s.state || "—") +
        "</td>" +
        "<td>" +
        (s.profile_id || "—") +
        "</td>" +
        "<td>" +
        flowLabel(s) +
        "</td>" +
        "<td>" +
        (s.recording_ref ? "yes" : "—") +
        "</td>" +
        "<td>" +
        (s.created_at || "—") +
        "</td>";
      tr.querySelector("button").onclick = () => openDetail(s.session_id);
      body.appendChild(tr);
    });
  }

  async function refreshList() {
    const data = await OrchAPI.listSessions();
    renderList(data.sessions || []);
  }

  function recMetaLines(s) {
    const lines = [];
    lines.push("recording_ref: " + (s.recording_ref || "(none)"));
    lines.push("started_at: " + (s.recording_started_at || "—"));
    lines.push("stopped_at: " + (s.recording_stopped_at || "—"));
    lines.push("stop_reason: " + (s.recording_stop_reason || "—"));
    lines.push("bytes: " + (s.recording_bytes != null ? s.recording_bytes : "—"));
    return lines.join("\n");
  }

  async function openDetail(id) {
    selectedId = id;
    el("detail-card").hidden = false;
    el("detail-id").textContent = id;
    el("detail-err").textContent = "";
    el("detail-err").className = "probe";
    try {
      const [sess, tr, disp] = await Promise.all([
        OrchAPI.getSession(id),
        OrchAPI.getTranscript(id),
        OrchAPI.getDisposition(id).catch((e) => {
          if (e.status === 404) return null;
          throw e;
        }),
      ]);
      const summary = {
        session_id: sess.session_id,
        clock: sess.clock,
        state: sess.state,
        profile_id: sess.profile_id,
        profile_version: sess.profile_version,
        flow_id: sess.flow_id,
        flow_version: sess.flow_version,
        active_language: sess.active_language,
        media_phase: sess.media_phase,
      };
      show(el("summary-out"), true, JSON.stringify(summary, null, 2));
      if (disp) {
        show(
          el("disp-out"),
          true,
          "final=" +
            (disp.final != null ? disp.final : "(none)") +
            "\nsuggestion=" +
            (disp.suggestion || "—") +
            "\nsource=" +
            (disp.source || "—")
        );
      } else {
        show(el("disp-out"), true, "(no disposition)");
      }
      show(el("rec-out"), true, recMetaLines(sess));
      renderTurns(tr.turns || []);
      await refreshList();
    } catch (e) {
      show(el("detail-err"), false, e.message || String(e));
    }
  }

  function bind() {
    el("token").value = OrchAPI.token();
    el("save-token").onclick = () => {
      OrchAPI.setToken(el("token").value);
      show(el("token-out"), true, "Token saved.");
    };
    el("btn-refresh").onclick = () =>
      refreshList().catch((e) => show(el("list-out"), false, e.message || String(e)));
    el("btn-reload-detail").onclick = () => {
      if (selectedId) openDetail(selectedId);
    };
  }

  bind();
  refreshList().catch((e) => show(el("list-out"), false, e.message || String(e)));
})();
