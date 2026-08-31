/* Supervisor console */
CoralUI.onReady(function () {
  "use strict";

  var el = CoralUI.el;
  var api = CoralUI.api;
  var showBanner = CoralUI.showBanner;
  var formatApiError = CoralUI.formatApiError;
  var bindClick = CoralUI.bindClick;
  var selected = "";

  function setJson(id, data) {
    var node = el(id);
    if (!node) return;
    node.textContent = typeof data === "string" ? data : JSON.stringify(data, null, 2);
  }

  function statePill(state) {
    var s = String(state || "");
    var cls = "";
    if (/Completed|Cancelled/.test(s)) cls = "ok";
    else if (/Failed/.test(s)) cls = "bad";
    else if (/Running|Attached|Created|Draining/.test(s)) cls = "warn";
    return '<span class="pill ' + cls + '">' + (s || "?") + "</span>";
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  function renderTranscript(data) {
    var turns = (data && data.turns) || [];
    var chat = el("transcriptChat");
    if (!chat) return;
    if (!turns.length) {
      chat.innerHTML = '<div class="hint">No transcript turns.</div>';
      return;
    }
    chat.innerHTML = turns.map(function (t) {
      var role = t.role || "unknown";
      var cls = role === "user" ? "user" : "assistant";
      return '<div class="bubble ' + cls + '"><div class="meta">' + escapeHtml(role) +
        " · seq " + t.seq + (t.turn_id ? " · " + escapeHtml(t.turn_id) : "") +
        "</div>" + escapeHtml(t.text || "") + "</div>";
    }).join("");
  }

  function refreshList() {
    var limit = Number(el("limit").value) || 50;
    return api("GET", "/v1/sessions?limit=" + limit).then(function (data) {
      var rows = data.sessions || [];
      el("sessEmpty").hidden = rows.length > 0;
      el("sessBody").innerHTML = rows.map(function (s) {
        var id = s.session_id;
        var sel = id === selected ? " selected" : "";
        return '<tr class="clickable' + sel + '" data-id="' + id + '">' +
          "<td><code>" + id.slice(0, 12) + "…</code></td>" +
          "<td>" + statePill(s.state) + "</td>" +
          "<td>" + (s.profile_id || "") + " v" + (s.profile_version || "?") + "</td>" +
          "<td>" + (s.updated_at || "") + "</td></tr>";
      }).join("");
      var trs = el("sessBody").querySelectorAll("tr.clickable");
      for (var i = 0; i < trs.length; i++) {
        (function (tr) {
          tr.addEventListener("click", function () {
            selected = tr.getAttribute("data-id");
            el("sid").value = selected;
            inspect(selected);
            refreshList();
          });
        })(trs[i]);
      }
      if (!rows.length) {
        showBanner(el("banner"), "", "No sessions in store. Start one from the User console.");
      } else {
        showBanner(el("banner"), "ok", rows.length + " session(s) loaded.");
      }
    }).catch(function (e) {
      showBanner(el("banner"), "bad", formatApiError(e));
      setJson("sessJson", { error: formatApiError(e) });
    });
  }

  function inspect(id) {
    if (!id) {
      showBanner(el("banner"), "bad", "session_id required");
      return Promise.resolve();
    }
    selected = id;
    el("metaPills").innerHTML = "";

    var pSess = api("GET", "/v1/sessions/" + encodeURIComponent(id)).then(function (sess) {
      setJson("sessJson", sess);
      var pills = [
        statePill(sess.state),
        '<span class="pill">profile ' + (sess.profile_id || "?") + " v" + (sess.profile_version || "?") + "</span>",
      ];
      if (sess.gateway_binding) {
        pills.push('<span class="pill">' + sess.gateway_binding.listen + " / " +
          sess.gateway_binding.think + " / " + sess.gateway_binding.speak + "</span>");
      }
      if (sess.active_language) pills.push('<span class="pill">lang ' + sess.active_language + "</span>");
      if (sess.recording_ref) pills.push('<span class="pill warn">recording_ref set (external)</span>');
      else pills.push('<span class="pill">no recording_ref (orchestrator does not store call audio)</span>');
      el("metaPills").innerHTML = pills.join(" ");
    }).catch(function (e) {
      setJson("sessJson", { error: formatApiError(e) });
      showBanner(el("banner"), "bad", "Session load failed: " + formatApiError(e));
    });

    var pTr = api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/transcript").then(function (tr) {
      setJson("transcriptJson", tr);
      renderTranscript(tr);
    }).catch(function (e) {
      renderTranscript({ turns: [] });
      setJson("transcriptJson", { error: formatApiError(e) });
    });

    var pDisp = api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/disposition").then(function (d) {
      setJson("dispJson", d);
    }).catch(function (e) {
      setJson("dispJson", {
        note: e.status === 404 ? "Disposition not written yet (expected until postcall after stop)." : formatApiError(e),
        error: formatApiError(e),
      });
    });

    var pAudit = api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/audit").then(function (a) {
      setJson("auditJson", a);
    }).catch(function (e) {
      setJson("auditJson", { error: formatApiError(e) });
    });

    var pAn = api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/analytics").then(function (an) {
      setJson("analyticsJson", an);
    }).catch(function (e) {
      setJson("analyticsJson", { error: formatApiError(e) });
    });

    return Promise.all([pSess, pTr, pDisp, pAudit, pAn,
      loadAttributes(false), loadHandoff(), loadSkills()]);
  }

  /* ------------------------------------------------------ desk inspection */

  var esc = CoralUI.esc;
  var shortTime = CoralUI.shortTime;

  function currentSession() {
    var id = el("sid").value.trim() || selected;
    if (!id) showBanner(el("banner"), "bad", "select a session first");
    return id;
  }

  function loadAttributes(reveal) {
    var id = currentSession();
    if (!id) return Promise.resolve();
    var q = "/v1/sessions/" + encodeURIComponent(id) + "/attributes";
    if (reveal) {
      var actor = el("revealActor").value.trim();
      if (!actor) {
        showBanner(el("banner"), "bad", "An actor name is required to reveal confidential attributes — the reveal is audited.");
        return Promise.resolve();
      }
      q += "?reveal=true&actor=" + encodeURIComponent(actor) +
        "&reason=" + encodeURIComponent(el("revealReason").value.trim());
    }
    return api("GET", q).then(function (data) {
      var rows = data.attributes || [];
      el("attrBody").innerHTML = rows.map(function (a) {
        var cls = a.class === "confidential" ? "warn" : "";
        return "<tr><td><code>" + esc(a.key) + "</code></td><td>" + esc(a.value) +
          '</td><td><span class="pill ' + cls + '">' + esc(a.class) + "</span></td></tr>";
      }).join("") || '<tr><td colspan="3" class="hint">No attributes stored for this session.</td></tr>';
      if (data.revealed) {
        showBanner(el("banner"), "", "Confidential values revealed — this access was written to the PII audit.");
      }
    }).catch(function (e) {
      el("attrBody").innerHTML = '<tr><td colspan="3" class="hint">' + esc(formatApiError(e)) + "</td></tr>";
    });
  }

  function loadHandoff() {
    var id = currentSession();
    if (!id) return Promise.resolve();
    return api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/handoff").then(function (data) {
      var h = data.handoff || {};
      var pills = [];
      if (h.target) pills.push('<span class="pill ok">to: ' + esc(h.target) + "</span>");
      if (h.language) pills.push('<span class="pill">' + esc(h.language) + "</span>");
      if (h.ticket_id) pills.push('<span class="pill ok">ticket: ' + esc(h.ticket_id) + "</span>");
      if (h.priority) pills.push('<span class="pill warn">' + esc(h.priority) + "</span>");
      el("handoffSummary").innerHTML = '<div class="row">' + pills.join(" ") + "</div>" +
        (h.summary ? "<p>" + esc(h.summary) + "</p>" : "");
      setJson("handoffJson", data.handoff);
    }).catch(function (e) {
      el("handoffSummary").innerHTML = "";
      setJson("handoffJson", {
        note: e.status === 404 ? "No handoff pack — this session did not run a contact desk." : "",
        error: formatApiError(e),
      });
    });
  }

  function loadDeskState() {
    var id = currentSession();
    if (!id) return Promise.resolve();
    return api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/desk-state").then(function (data) {
      setJson("handoffJson", data);
    }).catch(function (e) {
      setJson("handoffJson", {
        note: e.status === 404 ? "No desk running — live state exists only while the call is up." : "",
        error: formatApiError(e),
      });
    });
  }

  function loadSkills() {
    var id = currentSession();
    if (!id) return Promise.resolve();
    return api("GET", "/v1/sessions/" + encodeURIComponent(id) + "/skills").then(function (data) {
      var rows = data.skill_invocations || [];
      el("skillBody").innerHTML = rows.map(function (inv) {
        var kind = inv.status === "ok" ? "ok" : "bad";
        var out = inv.output || {};
        var summary = out.ticket_id || out.enquiry_id || out.target || out.status || "";
        return "<tr><td><code>" + esc(inv.skill) + '</code></td><td><span class="pill ' + kind + '">' +
          esc(inv.status) + "</span></td><td>" + esc(summary || inv.error || "") + "</td><td>" +
          esc(shortTime(inv.created_at)) + "</td></tr>";
      }).join("") || '<tr><td colspan="4" class="hint">No connector calls on this session.</td></tr>';
      setJson("skillJson", rows.length ? rows : { note: "No skill invocations recorded." });
    }).catch(function (e) {
      setJson("skillJson", { error: formatApiError(e) });
    });
  }

  function loadDispositionCodes() {
    return api("GET", "/v1/desk-catalog").then(function (cat) {
      el("dispPick").innerHTML = (cat.dispositions || []).map(function (d) {
        return '<option value="' + esc(d) + '">' + esc(d) + "</option>";
      }).join("");
    }).catch(function () {
      el("dispPick").innerHTML = '<option value="">catalog unavailable</option>';
    });
  }

  function overrideDisposition() {
    var id = currentSession();
    if (!id) return Promise.resolve();
    return api("PATCH", "/v1/sessions/" + encodeURIComponent(id) + "/disposition", {
      final: el("dispPick").value,
      actor: el("dispActor").value.trim() || "supervisor",
    }).then(function (data) {
      setJson("dispJson", data);
      showBanner(el("banner"), "ok", "Disposition set to <code>" + esc(data.final) + "</code>.");
    });
  }

  function forceStop() {
    var id = el("sid").value.trim();
    if (!id) return Promise.resolve();
    return api("POST", "/v1/sessions/" + encodeURIComponent(id) + "/stop", { reason: "supervisor" })
      .then(function (data) {
        setJson("sessJson", data);
        showBanner(el("banner"), "ok", "Stop requested.");
        return refreshList().then(function () { return inspect(id); });
      })
      .catch(function (e) {
        showBanner(el("banner"), "bad", formatApiError(e));
      });
  }

  try {
    bindClick("btnRefresh", refreshList);
    bindClick("btnLoad", function () { return inspect(el("sid").value.trim()); });
    bindClick("btnStop", forceStop);
    bindClick("btnAttrs", function () { return loadAttributes(false); });
    bindClick("btnReveal", function () { return loadAttributes(true); });
    bindClick("btnHandoff", loadHandoff);
    bindClick("btnDeskState", loadDeskState);
    bindClick("btnSkills", loadSkills);
    bindClick("btnDisp", overrideDisposition);
    loadDispositionCodes();
    refreshList();
  } catch (err) {
    showBanner(el("banner"), "bad", "Supervisor UI failed to start: " + (err && err.message ? err.message : err));
  }
});
