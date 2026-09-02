/* Admin console — no product presets; operator authors config. */
CoralUI.onReady(function () {
  "use strict";

  var el = CoralUI.el;
  var api = CoralUI.api;
  var showBanner = CoralUI.showBanner;
  var formatApiError = CoralUI.formatApiError;
  var loadPlatformStatus = CoralUI.loadPlatformStatus;
  var renderReadiness = CoralUI.renderReadiness;
  var bindClick = CoralUI.bindClick;

  function setJson(id, data) {
    var node = el(id);
    if (!node) return;
    node.textContent = typeof data === "string" ? data : JSON.stringify(data, null, 2);
  }

  /** Minimal blank Contact Agent profile — structure only, no Sales/R&D content. */
  function blankProfile(id, displayName) {
    id = id || "my-agent";
    displayName = displayName || id;
    return {
      id: id,
      metadata: { display_name: displayName, family: "contact-agent" },
      modes: { listen: true, speak: true, think: true, talk: true },
      audio: { canonical_sample_rate_hz: 16000 },
      language: {
        behaviour: "none",
        auto_detect: true,
        mid_call_switch: true,
        allowed: ["en-IN", "hi-IN"],
      },
      hot_swap_allowed: ["language.primary"],
      persona: {
        name: "Agent",
        instructions: "Describe how this agent should behave.",
        voice: { "fake-speak": "default-voice", "sarvam-tts": "shubh" },
      },
      response: {
        ladder: ["clip", "template", "llm"],
        clips: {
          "clip-apology-en": { text: "Sorry — one moment." },
        },
        templates: {},
      },
      fallback: {
        listen_down: { speak_canned: "clip-apology-en" },
        think_down: { speak_canned: "clip-apology-en" },
        speak_down: { text_sink: true },
      },
      skills: { allowed: [], definitions: {} },
    };
  }

  function refreshStatus() {
    return loadPlatformStatus().then(function (st) {
      renderReadiness(el("readyList"), st);
      setJson("statusJson", st);
      if (!st.ready_for_sessions) {
        showBanner(el("banner"), "bad",
          "<strong>Not ready for sessions.</strong> Fix blockers, then open User. <code>" +
          ((st.blockers || []).join(", ") || "") + "</code>");
      } else if ((st.warnings || []).length) {
        showBanner(el("banner"), "",
          "<strong>Degraded but startable.</strong> Warnings: <code>" + st.warnings.join(", ") + "</code>");
      } else {
        showBanner(el("banner"), "ok", "<strong>Ready.</strong> Users can start sessions.");
      }
    }).catch(function (e) {
      showBanner(el("banner"), "bad", formatApiError(e));
      setJson("statusJson", { error: formatApiError(e) });
    });
  }

  function loadGateways() {
    return api("GET", "/v1/gateways").then(function (data) {
      var g = data.gateways || {};
      function fill(listId, rows) {
        var node = el(listId);
        if (!node) return;
        node.innerHTML = (rows || []).map(function (r) {
          return '<option value="' + r.id + '"></option>';
        }).join("");
      }
      fill("gwListen", g.listen);
      fill("gwThink", g.think);
      fill("gwSpeak", g.speak);
    }).catch(function () { /* non-fatal */ });
  }

  function loadEngines() {
    return api("GET", "/v1/tenant/engines").then(function (data) {
      el("engListen").value = data.listen || "";
      el("engThink").value = data.think || "";
      el("engSpeak").value = data.speak || "";
      setJson("enginesJson", data);
    }).catch(function (e) {
      el("engListen").value = "";
      el("engThink").value = "";
      el("engSpeak").value = "";
      setJson("enginesJson", { error: formatApiError(e), note: "Expected on fresh install until Save engines." });
    });
  }

  function saveEngines() {
    var body = {
      listen: el("engListen").value.trim(),
      think: el("engThink").value.trim(),
      speak: el("engSpeak").value.trim(),
    };
    return api("PUT", "/v1/tenant/engines", body).then(function (data) {
      setJson("enginesJson", data);
      showBanner(el("banner"), "ok", "Engines saved.");
      return refreshStatus();
    }).catch(function (e) {
      setJson("enginesJson", { error: formatApiError(e), details: e.details || null });
      showBanner(el("banner"), "bad", "Engines save failed: " + formatApiError(e));
    });
  }

  function loadConfigBundle() {
    return api("GET", "/v1/tenant/config").then(function (data) {
      setJson("credJson", { credentials: data.credentials || [] });
      setJson("settingsJson", { settings: data.settings || [] });
    }).catch(function (e) {
      setJson("credJson", { error: formatApiError(e) });
    });
  }

  function saveCred() {
    var id = el("credId").value.trim();
    var key = el("credKey").value;
    if (!id || !key) {
      showBanner(el("banner"), "bad", "gateway_id and api_key required");
      return Promise.resolve();
    }
    return api("PUT", "/v1/tenant/credentials/" + encodeURIComponent(id), { api_key: key }).then(function (data) {
      el("credKey").value = "";
      setJson("credJson", data);
      showBanner(el("banner"), "ok", "Credential saved (masked on read).");
      return refreshStatus().then(loadConfigBundle);
    }).catch(function (e) {
      setJson("credJson", { error: formatApiError(e) });
      showBanner(el("banner"), "bad", "Credential save failed: " + formatApiError(e));
    });
  }

  function saveSetting() {
    var key = el("setKey").value.trim();
    var value = el("setVal").value;
    if (!key) {
      showBanner(el("banner"), "bad", "key required");
      return Promise.resolve();
    }
    return api("PUT", "/v1/tenant/settings/" + encodeURIComponent(key), { value: value }).then(function (data) {
      setJson("settingsJson", data);
      showBanner(el("banner"), "ok", "Setting saved. Note: coral.base_url for transfer/CRM is read at process boot — restart after changing it.");
      return loadConfigBundle();
    }).catch(function (e) {
      setJson("settingsJson", { error: formatApiError(e) });
      showBanner(el("banner"), "bad", formatApiError(e));
    });
  }

  function loadBlankProfile() {
    var id = el("profId").value.trim() || "my-agent";
    var name = el("profName").value.trim() || id;
    el("profId").value = id;
    el("profName").value = name;
    el("profDoc").value = JSON.stringify(blankProfile(id, name), null, 2);
    showBanner(el("banner"), "ok", "Blank template loaded — edit JSON, then Create + publish.");
  }

  function listProfiles() {
    return api("GET", "/v1/profiles").then(function (data) {
      setJson("profilesJson", data);
    }).catch(function (e) {
      setJson("profilesJson", { error: formatApiError(e) });
    });
  }

  function publishProfile() {
    var id = el("profId").value.trim();
    var name = el("profName").value.trim() || id;
    if (!id) {
      showBanner(el("banner"), "bad", "profile id required");
      return Promise.resolve();
    }
    var doc;
    try {
      doc = JSON.parse(el("profDoc").value);
    } catch (pe) {
      showBanner(el("banner"), "bad", "profile JSON invalid: " + pe.message);
      return Promise.resolve();
    }
    doc.id = id;
    return api("POST", "/v1/profiles", { id: id, display_name: name, tenant_id: "default" })
      .catch(function (e) {
        if (e.status !== 409) throw e;
      })
      .then(function () {
        return api("POST", "/v1/profiles/" + encodeURIComponent(id) + "/versions", doc);
      })
      .then(function (pub) {
        setJson("profilesJson", { published: pub });
        showBanner(el("banner"), "ok", "Published profile <code>" + id + "</code>.");
        return listProfiles().then(refreshStatus);
      })
      .catch(function (e) {
        setJson("profilesJson", { error: formatApiError(e), body: e.body || null });
        showBanner(el("banner"), "bad", "Publish failed: " + formatApiError(e));
      });
  }

  function kbUpload() {
    var collection = el("kbCollection").value.trim();
    var text = el("kbText").value;
    var uri = el("kbUri").value.trim();
    if (!collection) {
      showBanner(el("banner"), "bad", "collection required");
      return Promise.resolve();
    }
    if (!text && !uri) {
      showBanner(el("banner"), "bad", "paste document text or set a uri");
      return Promise.resolve();
    }
    return api("POST", "/v1/kb/documents", {
      collection: collection,
      tenant_id: "default",
      uri: uri,
      text: text,
    }).then(function (data) {
      setJson("kbJson", data);
      if (data.id) el("kbId").value = data.id;
      showBanner(el("banner"), data.status === "ready" ? "ok" : "bad",
        "KB document " + (data.id || "") + " status=" + data.status);
    }).catch(function (e) {
      setJson("kbJson", { error: formatApiError(e) });
      showBanner(el("banner"), "bad", "KB upload failed: " + formatApiError(e));
    });
  }

  function kbGet() {
    var id = el("kbId").value.trim();
    if (!id) {
      showBanner(el("banner"), "bad", "document id required");
      return Promise.resolve();
    }
    return api("GET", "/v1/kb/documents/" + encodeURIComponent(id)).then(function (data) {
      setJson("kbJson", data);
    }).catch(function (e) {
      setJson("kbJson", { error: formatApiError(e) });
      showBanner(el("banner"), "bad", formatApiError(e));
    });
  }

  /* ---------------------------------------------------------------- desks */

  var selectedDesk = "";
  var deskDoc = null;
  var deskDirty = false;
  var currentPromptId = "";
  var esc = CoralUI.esc;
  var shortTime = CoralUI.shortTime;

  function listDesks() {
    return api("GET", "/v1/desks").then(function (data) {
      var rows = data.desks || [];
      el("deskEmpty").hidden = rows.length > 0;
      el("deskBody").innerHTML = rows.map(function (d) {
        var pill = d.status === "published" ? "ok" : "warn";
        return '<tr class="clickable' + (d.id === selectedDesk ? " selected" : "") + '" data-desk="' + esc(d.id) + '">' +
          "<td><strong>" + esc(d.id) + "</strong><br /><span class=\"hint\">" + esc(d.name) + "</span></td>" +
          "<td>" + esc(d.direction) + "</td>" +
          '<td><span class="pill ' + pill + '">' + esc(d.status) + "</span></td>" +
          "<td>" + esc(d.current_version || 0) + "</td>" +
          "<td>" + esc(shortTime(d.updated_at)) + "</td></tr>";
      }).join("");
      Array.prototype.forEach.call(el("deskBody").querySelectorAll("tr[data-desk]"), function (tr) {
        tr.addEventListener("click", function () {
          selectDesk(tr.getAttribute("data-desk"));
        });
      });
      if (!selectedDesk && rows.length) selectDesk(rows[0].id);
    }).catch(function (e) {
      showBanner(el("banner"), "bad", "Desk list failed: " + formatApiError(e));
    });
  }

  function selectDesk(id) {
    selectedDesk = id;
    el("deskId").value = id;
    return loadDesk(id).then(runChecklist);
  }

  function markDirty(on) {
    deskDirty = !!on;
    var n = el("edDirty");
    if (n) n.hidden = !deskDirty;
  }

  function flushPromptEdits() {
    if (!deskDoc || !currentPromptId || !deskDoc.prompts || !deskDoc.prompts[currentPromptId]) return;
    deskDoc.prompts[currentPromptId].text = deskDoc.prompts[currentPromptId].text || {};
    deskDoc.prompts[currentPromptId].text["en-IN"] = el("edPromptEN").value;
    deskDoc.prompts[currentPromptId].text["hi-IN"] = el("edPromptHI").value;
  }

  function collectEditor() {
    if (!deskDoc) return;
    flushPromptEdits();
    deskDoc.name = el("edName").value.trim() || deskDoc.name;
    deskDoc.purpose = el("edPurpose").value;
    deskDoc.direction = el("edDirection").value;
    deskDoc.default_language = el("edDefaultLang").value;
    deskDoc.voice_id = el("edVoice").value.trim() || deskDoc.voice_id;
    deskDoc.cx = deskDoc.cx || {};
    deskDoc.cx.barge_in = el("edBarge").value === "true";
    deskDoc.cx.listen_while_speak = el("edLWS").value === "true";
    deskDoc.cx.silence_nudge1_ms = Number(el("edSil1").value) || deskDoc.cx.silence_nudge1_ms;
    deskDoc.cx.silence_nudge2_ms = Number(el("edSil2").value) || deskDoc.cx.silence_nudge2_ms;
    deskDoc.cx.silence_hangup_ms = Number(el("edSilH").value) || deskDoc.cx.silence_hangup_ms;
    deskDoc.cx.ask_timeout_ms = Number(el("edAskT").value) || deskDoc.cx.ask_timeout_ms;
    Array.prototype.forEach.call(document.querySelectorAll("[data-intent-en]"), function (ta) {
      var id = ta.getAttribute("data-intent-en");
      (deskDoc.intents || []).forEach(function (it) {
        if (it.id !== id) return;
        it.phrases = it.phrases || {};
        it.phrases["en-IN"] = ta.value.split("\n").map(function (s) { return s.trim(); }).filter(Boolean);
      });
    });
    Array.prototype.forEach.call(document.querySelectorAll("[data-intent-hi]"), function (ta) {
      var id = ta.getAttribute("data-intent-hi");
      (deskDoc.intents || []).forEach(function (it) {
        if (it.id !== id) return;
        it.phrases = it.phrases || {};
        it.phrases["hi-IN"] = ta.value.split("\n").map(function (s) { return s.trim(); }).filter(Boolean);
      });
    });
    Array.prototype.forEach.call(document.querySelectorAll("[data-mx]"), function (inp) {
      var i = Number(inp.getAttribute("data-mx"));
      var field = inp.getAttribute("data-field");
      if (deskDoc.matrix && deskDoc.matrix[i]) deskDoc.matrix[i][field] = inp.value;
    });
  }

  function fillEditor(doc) {
    deskDoc = doc || {};
    el("deskEditor").hidden = false;
    el("edName").value = deskDoc.name || "";
    el("edPurpose").value = deskDoc.purpose || "support";
    el("edDirection").value = deskDoc.direction || "inbound";
    el("edDefaultLang").value = deskDoc.default_language || "en-IN";
    el("edVoice").value = deskDoc.voice_id || "";
    var cx = deskDoc.cx || {};
    el("edBarge").value = cx.barge_in === false ? "false" : "true";
    el("edLWS").value = cx.listen_while_speak === false ? "false" : "true";
    el("edSil1").value = cx.silence_nudge1_ms || 6000;
    el("edSil2").value = cx.silence_nudge2_ms || 6000;
    el("edSilH").value = cx.silence_hangup_ms || 8000;
    el("edAskT").value = cx.ask_timeout_ms || 8000;
    var prompts = deskDoc.prompts || {};
    var ids = Object.keys(prompts).sort();
    el("edPromptPick").innerHTML = ids.map(function (id) {
      return "<option value=\"" + esc(id) + "\">" + esc((prompts[id].label || id) + " (" + id + ")") + "</option>";
    }).join("");
    if (ids.length) showPrompt(ids[0]);
    el("edIntentBody").innerHTML = (deskDoc.intents || []).map(function (it) {
      var en = ((it.phrases || {})["en-IN"] || []).join("\n");
      var hi = ((it.phrases || {})["hi-IN"] || []).join("\n");
      return "<tr><td><strong>" + esc(it.display || it.id) + "</strong><div class=\"hint\">" + esc(it.id) +
        "</div></td><td><textarea data-intent-en=\"" + esc(it.id) + "\" class=\"prompt-box\">" + esc(en) +
        "</textarea></td><td><textarea data-intent-hi=\"" + esc(it.id) + "\" class=\"prompt-box\">" + esc(hi) +
        "</textarea></td></tr>";
    }).join("");
    el("edMatrixBody").innerHTML = (deskDoc.matrix || []).map(function (row, i) {
      // Queue = ACD/queue label (target). Extension = dial destination (number)
      // used by uuid_transfer. Older drafts that put digits only in Queue still
      // transfer via TransferNumberFor's numeric-target fallback.
      return "<tr><td>" + esc(row.intent) + "</td>" +
        "<td><input data-mx=\"" + i + "\" data-field=\"owner\" value=\"" + esc(row.owner || "") + "\" /></td>" +
        "<td><input data-mx=\"" + i + "\" data-field=\"target\" value=\"" + esc(row.target || "") + "\" placeholder=\"sales\" /></td>" +
        "<td><input data-mx=\"" + i + "\" data-field=\"number\" value=\"" + esc(row.number || "") + "\" placeholder=\"extension\" /></td>" +
        "<td><select data-mx=\"" + i + "\" data-field=\"action\">" +
        ["transfer", "ticket", "both"].map(function (a) {
          return "<option value=\"" + a + "\"" + (row.action === a ? " selected" : "") + ">" + a + "</option>";
        }).join("") + "</select></td></tr>";
    }).join("");
    markDirty(false);
  }

  function showPrompt(id) {
    flushPromptEdits();
    currentPromptId = id;
    el("edPromptPick").value = id;
    var p = (deskDoc.prompts || {})[id] || { text: {} };
    el("edPromptEN").value = (p.text && p.text["en-IN"]) || "";
    el("edPromptHI").value = (p.text && p.text["hi-IN"]) || "";
  }

  function switchTab(name) {
    Array.prototype.forEach.call(document.querySelectorAll("#deskTabs .tab"), function (btn) {
      btn.classList.toggle("active", btn.getAttribute("data-tab") === name);
    });
    ["overview", "prompts", "intents", "matrix", "cx", "sim"].forEach(function (id) {
      var pane = el("tab-" + id);
      if (pane) pane.hidden = id !== name;
    });
  }

  function saveDraft() {
    if (!selectedDesk || !deskDoc) {
      showBanner(el("banner"), "bad", "select a desk first");
      return Promise.resolve();
    }
    collectEditor();
    return api("PUT", "/v1/desks/" + encodeURIComponent(selectedDesk) + "/draft", { document: deskDoc })
      .then(function (data) {
        markDirty(false);
        if (data.checklist) renderChecklist(data.checklist);
        showBanner(el("banner"), "ok", "Draft saved. Review the checklist, then Publish desk.");
      });
  }

  function runSim() {
    if (!selectedDesk) return Promise.resolve();
    collectEditor();
    var turns = el("edSimTurns").value.split("\n").map(function (s) { return s.trim(); }).filter(Boolean);
    return api("POST", "/v1/desks/" + encodeURIComponent(selectedDesk) + "/simulate", {
      language: el("edSimLang").value,
      turns: turns,
    }).then(function (data) {
      el("edSimChat").innerHTML = (data.steps || []).map(function (s) {
        var html = "";
        if (s.user) html += "<div class=\"bubble user\"><div class=\"meta\">caller · " + esc(s.language || "") +
          "</div>" + esc(s.user) + "</div>";
        html += "<div class=\"bubble assistant\"><div class=\"meta\">assistant · " + esc(s.language || "") +
          (s.intent ? " · " + esc(s.intent) : "") + "</div>" + esc(s.assistant || "") + "</div>";
        return html;
      }).join("");
      setJson("edSimJson", {
        ended: data.ended, disposition: data.disposition,
        attributes: data.attributes, handoff: data.handoff,
      });
    });
  }

  function loadDesk(id) {
    return api("GET", "/v1/desks/" + encodeURIComponent(id)).then(function (data) {
      var doc = data.document || {};
      var pills = [];
      function pill(label, value, kind) {
        pills.push('<span class="pill ' + (kind || "") + '">' + esc(label) + ": " + esc(value) + "</span>");
      }
      pill("languages", (doc.languages || []).join(" / ") || "en-IN / hi-IN");
      pill("intents", (doc.intents || []).filter(function (i) { return i.active; }).length);
      pill("paths", Object.keys(doc.paths || {}).length);
      pill("prompts", Object.keys(doc.prompts || {}).length);
      if (data.published) {
        pill("published", "v" + data.published.version + " → profile v" + data.published.profile_version, "ok");
      } else {
        pill("published", "never", "warn");
      }
      el("deskSummary").innerHTML = pills.join(" ");
      setJson("deskJson", {
        desk: data.desk,
        published: data.published || null,
        languages: doc.languages,
        default_language: doc.default_language,
        matrix: doc.matrix,
        skills: Object.keys(doc.skills || {}),
      });
      fillEditor(doc);
    }).catch(function (e) {
      setJson("deskJson", { error: formatApiError(e) });
    });
  }

  function renderChecklist(check) {
    var items = (check && check.items) || [];
    el("deskChecklist").innerHTML = items.map(function (it) {
      var kind = it.ok ? "ok" : (it.blocker ? "bad" : "warn");
      var mark = it.ok ? "OK" : (it.blocker ? "BLOCK" : "WARN");
      return '<li><span class="mark pill ' + kind + '">' + mark + "</span><div><strong>" +
        esc(it.label) + '</strong><div class="hint" style="margin:0">' + esc(it.detail || "") + "</div></div></li>";
    }).join("") || "<li>No checklist</li>";
    (check && check.warnings ? check.warnings : []).forEach(function (w) {
      el("deskChecklist").innerHTML +=
        '<li><span class="mark pill warn">WARN</span><div>' + esc(w) + "</div></li>";
    });
  }

  function runChecklist() {
    var id = el("deskId").value.trim();
    if (!id) return Promise.resolve();
    return api("GET", "/v1/desks/" + encodeURIComponent(id) + "/checklist").then(function (data) {
      renderChecklist(data.checklist);
    }).catch(function (e) {
      el("deskChecklist").innerHTML = '<li><span class="mark pill bad">ERR</span><div>' +
        esc(formatApiError(e)) + "</div></li>";
    });
  }

  function installCoral() {
    return api("POST", "/v1/desk-presets/coral-tfn", { tenant_id: "default" }).then(function (data) {
      selectedDesk = data.desk.id;
      renderChecklist(data.checklist);
      showBanner(el("banner"), "ok",
        "Installed <code>" + esc(data.desk.id) + "</code> as a draft. Review the checklist, then Publish desk.");
      return listDesks();
    });
  }

  function publishDesk() {
    var id = el("deskId").value.trim();
    if (!id) {
      showBanner(el("banner"), "bad", "select a desk first");
      return Promise.resolve();
    }
    collectEditor();
    return api("PUT", "/v1/desks/" + encodeURIComponent(id) + "/draft", { document: deskDoc }).then(function () {
      return api("POST", "/v1/desks/" + encodeURIComponent(id) + "/publish", { published_by: "admin-console" });
    })
      .then(function (pub) {
        showBanner(el("banner"), "ok",
          "Published desk <code>" + esc(id) + "</code> v" + pub.desk_version +
          " → profile <code>" + esc(pub.profile_id) + "</code> v" + pub.profile_version +
          ". Users can start a call on it now.");
        renderChecklist(pub.checklist);
        return listDesks().then(listProfiles).then(refreshStatus);
      })
      .catch(function (e) {
        if (e.details && e.details.checklist) renderChecklist(e.details.checklist);
        showBanner(el("banner"), "bad", "Publish failed: " + formatApiError(e));
      });
  }

  /* ----------------------------------------------------- connector sandbox */

  function loadLedger() {
    return api("GET", "/v1/desk-skills/ledger").then(function (data) {
      var counts = [
        ["tickets", (data.tickets || []).length],
        ["emails", (data.emails || []).length],
        ["transfers", (data.transfers || []).length],
        ["enquiries", (data.enquiries || []).length],
      ].map(function (p) {
        return '<span class="pill">' + p[0] + ": " + p[1] + "</span>";
      });
      var failures = data.failures || {};
      Object.keys(failures).forEach(function (k) {
        counts.push('<span class="pill bad">' + esc(k) + " → " + esc(failures[k]) + "</span>");
      });
      el("ledgerCounts").innerHTML = counts.join(" ");
      setJson("ledgerJson", data);
    }).catch(function (e) {
      setJson("ledgerJson", { error: formatApiError(e) });
    });
  }

  function setFailure() {
    return api("POST", "/v1/desk-skills/failures", {
      skill: el("failSkill").value,
      status: el("failStatus").value,
    }).then(function () {
      showBanner(el("banner"), "ok", "Connector behaviour updated.");
      return loadLedger();
    });
  }

  function setAgent() {
    return api("POST", "/v1/desk-skills/agents", {
      target: el("agentTarget").value,
      available: el("agentAvail").value === "true",
    }).then(function (data) {
      showBanner(el("banner"), "ok",
        "Queue <code>" + esc(data.target) + "</code> is now " + (data.available ? "available" : "unavailable") + ".");
      return loadLedger();
    });
  }

  function resetLedger() {
    return api("POST", "/v1/desk-skills/reset", {}).then(function () {
      el("failStatus").value = "";
      el("agentAvail").value = "true";
      showBanner(el("banner"), "ok", "Sandbox ledger cleared.");
      return loadLedger();
    });
  }

  /* --------------------------------------------------------- properties */

  var propKeys = [];

  function loadProps() {
    return api("GET", "/v1/tenant/properties").then(function (data) {
      var props = data.properties || {};
      propKeys = data.known || Object.keys(props);
      el("propRows").innerHTML = '<div class="row">' + propKeys.map(function (k) {
        return "<label>" + esc(k) +
          ' <input data-prop="' + esc(k) + '" value="' + esc(props[k] == null ? "" : props[k]) + '" /></label>';
      }).join("") + "</div>";
      setJson("propsJson", { active_sessions: data.active_sessions, properties: props });
    }).catch(function (e) {
      setJson("propsJson", { error: formatApiError(e) });
    });
  }

  function saveProps() {
    var body = {};
    Array.prototype.forEach.call(document.querySelectorAll("[data-prop]"), function (input) {
      body[input.getAttribute("data-prop")] = input.value.trim();
    });
    return api("PUT", "/v1/tenant/properties", { properties: body }).then(function (data) {
      setJson("propsJson", { active_sessions: data.active_sessions, properties: data.properties });
      showBanner(el("banner"), "ok", "Tenant properties saved.");
    });
  }

  /* --------------------------------------------------------- compliance */

  function loadErasure() {
    return api("GET", "/v1/compliance/erasure").then(function (data) {
      var rows = data.erasure_requests || [];
      el("eraseBody").innerHTML = rows.map(function (rq) {
        var action = rq.status === "completed"
          ? ""
          : '<button type="button" class="secondary" data-erase="' + esc(rq.id) + '">Complete</button>';
        return "<tr><td>" + esc(rq.subject_ref) + "</td><td>" +
          '<span class="pill ' + (rq.status === "completed" ? "ok" : "warn") + '">' + esc(rq.status) + "</span></td><td>" +
          esc(shortTime(rq.requested_at)) + "</td><td>" + action + "</td></tr>";
      }).join("") || '<tr><td colspan="4" class="hint">No erasure requests.</td></tr>';
      Array.prototype.forEach.call(el("eraseBody").querySelectorAll("[data-erase]"), function (btn) {
        btn.addEventListener("click", function () {
          api("POST", "/v1/compliance/erasure/" + encodeURIComponent(btn.getAttribute("data-erase")) + "/complete", {})
            .then(function () {
              showBanner(el("banner"), "ok", "Erasure completed and session data purged.");
              return loadErasure();
            })
            .catch(function (e) { showBanner(el("banner"), "bad", formatApiError(e)); });
        });
      });
    }).catch(function (e) {
      showBanner(el("banner"), "bad", formatApiError(e));
    });
  }

  function queueErasure() {
    var subject = el("eraseSubject").value.trim();
    if (!subject) {
      showBanner(el("banner"), "bad", "subject_ref required");
      return Promise.resolve();
    }
    return api("POST", "/v1/compliance/erasure", {
      subject_ref: subject, scope: "all", requested_by: el("eraseBy").value.trim() || "admin-console",
    }).then(function () {
      el("eraseSubject").value = "";
      showBanner(el("banner"), "ok", "Erasure request queued.");
      return loadErasure();
    });
  }

  function loadPII() {
    return api("GET", "/v1/compliance/pii-access?limit=25").then(function (data) {
      setJson("piiJson", data.pii_access && data.pii_access.length
        ? data.pii_access
        : { note: "No confidential attribute reveals recorded yet." });
    }).catch(function (e) {
      setJson("piiJson", { error: formatApiError(e) });
    });
  }

  try {
    bindClick("btnInstallCoral", installCoral);
    bindClick("btnListDesks", listDesks);
    bindClick("btnChecklist", runChecklist);
    bindClick("btnPublishDesk", publishDesk);
    bindClick("btnSaveDraft", saveDraft);
    bindClick("btnSimRun", runSim);
    el("edPromptPick").addEventListener("change", function () {
      showPrompt(el("edPromptPick").value);
    });
    ["edPromptEN", "edPromptHI", "edName", "edVoice", "edSil1", "edSil2", "edSilH", "edAskT"].forEach(function (id) {
      el(id).addEventListener("input", function () { markDirty(true); });
    });
    ["edPurpose", "edDirection", "edDefaultLang", "edBarge", "edLWS"].forEach(function (id) {
      el(id).addEventListener("change", function () { markDirty(true); });
    });
    Array.prototype.forEach.call(document.querySelectorAll("#deskTabs .tab"), function (btn) {
      btn.addEventListener("click", function () { switchTab(btn.getAttribute("data-tab")); });
    });
    bindClick("btnSetFailure", setFailure);
    bindClick("btnSetAgent", setAgent);
    bindClick("btnLedger", loadLedger);
    bindClick("btnResetLedger", resetLedger);
    bindClick("btnSaveProps", saveProps);
    bindClick("btnLoadProps", loadProps);
    bindClick("btnErase", queueErasure);
    bindClick("btnLoadErase", function () { return loadErasure().then(loadPII); });

    bindClick("btnRefreshStatus", refreshStatus);
    bindClick("btnSaveEngines", saveEngines);
    bindClick("btnLoadEngines", loadEngines);
    bindClick("btnFakeEngines", function () {
      el("engListen").value = "fake-listen";
      el("engThink").value = "fake-think";
      el("engSpeak").value = "fake-speak";
      showBanner(el("banner"), "ok", "Filled fake engine ids — click Save engines to persist.");
    });
    bindClick("btnSarvamEngines", function () {
      el("engListen").value = "sarvam-stt";
      el("engThink").value = "sarvam-llm";
      el("engSpeak").value = "sarvam-tts";
      showBanner(el("banner"), "ok", "Filled Sarvam engine ids — save engines and credential.");
    });
    bindClick("btnSaveCred", saveCred);
    bindClick("btnSaveSetting", saveSetting);
    bindClick("btnBlankProfile", loadBlankProfile);
    bindClick("btnPublish", publishProfile);
    bindClick("btnListProfiles", listProfiles);
    bindClick("btnKbUpload", kbUpload);
    bindClick("btnKbGet", kbGet);

    loadBlankProfile();
    loadGateways()
      .then(loadEngines)
      .then(loadConfigBundle)
      .then(listProfiles)
      .then(listDesks)
      .then(loadLedger)
      .then(loadProps)
      .then(loadErasure)
      .then(loadPII)
      .then(refreshStatus);
  } catch (err) {
    console.error(err);
    showBanner(el("banner"), "bad", "Admin UI failed to start: " + (err && err.message ? err.message : err));
  }
});
