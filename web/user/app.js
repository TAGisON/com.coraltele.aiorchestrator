/* User console — voice call over /edge/fs (browser as feeder+sink) */
CoralUI.onReady(function () {
  "use strict";

  var el = CoralUI.el;
  var api = CoralUI.api;
  var showBanner = CoralUI.showBanner;
  var formatApiError = CoralUI.formatApiError;
  var loadPlatformStatus = CoralUI.loadPlatformStatus;
  var renderReadiness = CoralUI.renderReadiness;
  var bindClick = CoralUI.bindClick;
  var explainBlocker = CoralUI.explainBlocker;

  var sessionId = "";
  var eventSource = null;
  var edgeWs = null;
  var mediaStream = null;
  var audioCtx = null;
  var processor = null;
  var playCtx = null;
  var playTime = 0;
  var peerRate = 16000;
  var callActive = false;
  var micBytesSent = 0;
  var micStatusTimer = null;
  var liveCaptionEl = null;
  var liveCaptionText = "";

  function setJson(id, data) {
    var node = el(id);
    if (!node) return;
    node.textContent = typeof data === "string" ? data : JSON.stringify(data, null, 2);
  }

  function setCallStatus(label, kind) {
    var node = el("callStatus");
    if (!node) return;
    var cls = "pill";
    if (kind === "ok") cls += " ok";
    if (kind === "bad") cls += " bad";
    if (kind === "warn") cls += " warn";
    node.innerHTML = '<span class="' + cls + '">' + escapeHtml(label) + "</span>";
  }

  function setActive(on) {
    callActive = on;
    el("btnStart").disabled = on;
    el("btnStop").disabled = !on;
    el("btnSend").disabled = !on;
    el("btnRefreshTx").disabled = !on;
    el("msg").disabled = !on;
    el("profile").disabled = on;
    el("clock").disabled = on;
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  function appendBubble(role, text, meta) {
    var div = document.createElement("div");
    div.className = "bubble " + (role === "user" ? "user" : "assistant");
    div.innerHTML = '<div class="meta">' + escapeHtml(meta || role) + "</div>" + escapeHtml(text || "");
    el("chat").appendChild(div);
    el("chat").scrollTop = el("chat").scrollHeight;
  }

  function setLiveCaption(text) {
    var chat = el("chat");
    if (!chat) return;
    liveCaptionText = text || "";
    if (!text) {
      if (liveCaptionEl && liveCaptionEl.parentNode) {
        liveCaptionEl.parentNode.removeChild(liveCaptionEl);
      }
      liveCaptionEl = null;
      return;
    }
    if (!liveCaptionEl) {
      liveCaptionEl = document.createElement("div");
      liveCaptionEl.className = "bubble user live-partial";
      chat.appendChild(liveCaptionEl);
    }
    liveCaptionEl.innerHTML = '<div class="meta">you · live</div>' + escapeHtml(text);
    chat.scrollTop = chat.scrollHeight;
  }

  function refreshReady() {
    return loadPlatformStatus().then(function (st) {
      renderReadiness(el("readyList"), st);
      if (!st.ready_for_sessions) {
        showBanner(el("banner"), "bad",
          "<strong>Cannot start until Admin finishes setup.</strong> " +
          (st.blockers || []).map(explainBlocker).join(" · "));
        el("btnStart").disabled = true;
        el("startHint").textContent = "Blocked: " + (st.blockers || []).join(", ");
      } else if ((st.warnings || []).length) {
        showBanner(el("banner"), "",
          "<strong>Startable with warnings.</strong> " + st.warnings.map(explainBlocker).join(" · "));
        el("btnStart").disabled = !!sessionId;
        el("startHint").textContent = "Ready (degraded). Allow mic; vendor warnings may cause mid-call failures.";
      } else {
        showBanner(el("banner"), "ok", "<strong>Ready.</strong> Pick a profile and Start call.");
        el("btnStart").disabled = !!sessionId;
        el("startHint").textContent = "Platform ready — Start call uses mic + speakers via /edge/fs.";
      }
      return st;
    }).catch(function (e) {
      showBanner(el("banner"), "bad", "Platform status unavailable: " + formatApiError(e));
      el("btnStart").disabled = true;
      return null;
    });
  }

  function loadProfiles() {
    var sel = el("profile");
    sel.innerHTML = "";
    return api("GET", "/v1/profiles").then(function (data) {
      var list = data.profiles || [];
      if (!list.length) {
        sel.innerHTML = '<option value="">(no profiles — publish in Admin)</option>';
        return;
      }
      list.forEach(function (p) {
        var opt = document.createElement("option");
        opt.value = p.id;
        opt.textContent = (p.display_name || p.id) + " (" + p.id + ")";
        sel.appendChild(opt);
      });
    }).catch(function (e) {
      sel.innerHTML = '<option value="">(profiles list failed)</option>';
      showBanner(el("banner"), "bad", formatApiError(e));
    });
  }

  function closeSSE() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }

  function openSSE(id) {
    closeSSE();
    setLiveCaption("");
    try {
      eventSource = new EventSource("/v1/sessions/" + encodeURIComponent(id) + "/events");
      eventSource.addEventListener("turn.completed", function () {
        setLiveCaption("");
        refreshTranscript();
      });
      eventSource.addEventListener("caption", function (ev) {
        try {
          var payload = JSON.parse(ev.data);
          if (payload.partial) {
            setLiveCaption(payload.text || "");
          } else {
            setLiveCaption("");
            refreshTranscript();
          }
        } catch (_) { /* ignore */ }
      });
      eventSource.addEventListener("session.state", function (ev) {
        try {
          var payload = JSON.parse(ev.data);
          if (payload.state && /Completed|Failed|Cancelled/.test(payload.state)) {
            showBanner(el("banner"), payload.state === "Failed" ? "bad" : "ok", "Session " + payload.state);
            teardownMedia();
            setActive(false);
            closeSSE();
            setCallStatus("Idle", "");
            refreshTranscript();
          }
        } catch (_) { /* ignore */ }
      });
    } catch (_) { /* SSE optional */ }
  }

  function edgeWsUrl(token) {
    var proto = location.protocol === "https:" ? "wss:" : "ws:";
    return proto + "//" + location.host + "/edge/fs?token=" + encodeURIComponent(token) + "&rate=" + peerRate;
  }

  function floatTo16BitPCM(float32, inRate, outRate) {
    var ratio = inRate / outRate;
    var outLen = Math.floor(float32.length / ratio);
    var buf = new ArrayBuffer(outLen * 2);
    var view = new DataView(buf);
    var i;
    for (i = 0; i < outLen; i++) {
      var src = Math.floor(i * ratio);
      var s = Math.max(-1, Math.min(1, float32[src] || 0));
      view.setInt16(i * 2, s < 0 ? s * 0x8000 : s * 0x7fff, true);
    }
    return buf;
  }

  function ensurePlayCtx() {
    if (!playCtx) {
      var AC = window.AudioContext || window.webkitAudioContext;
      playCtx = new AC();
      playTime = playCtx.currentTime;
    }
    if (playCtx.state === "suspended") {
      playCtx.resume();
    }
    return playCtx;
  }

  function playPCM16(base64, sampleRate) {
    try {
      var ctx = ensurePlayCtx();
      var raw = atob(base64);
      var n = raw.length;
      var bytes = new Uint8Array(n);
      var i;
      for (i = 0; i < n; i++) bytes[i] = raw.charCodeAt(i);
      var samples = n / 2;
      var ab = ctx.createBuffer(1, samples, sampleRate || peerRate);
      var ch = ab.getChannelData(0);
      var view = new DataView(bytes.buffer);
      for (i = 0; i < samples; i++) {
        ch[i] = view.getInt16(i * 2, true) / 0x8000;
      }
      var src = ctx.createBufferSource();
      src.buffer = ab;
      src.connect(ctx.destination);
      var startAt = Math.max(ctx.currentTime + 0.02, playTime);
      src.start(startAt);
      playTime = startAt + ab.duration;
    } catch (err) {
      setJson("turnJson", { play_error: String(err && err.message ? err.message : err) });
    }
  }

  // Must unlock both AudioContexts inside the Start-call click (before awaits), or Chrome
  // keeps them suspended — mic uplink silent and TTS playback inaudible even when edge sends audio.
  function unlockAudioContexts() {
    var AC = window.AudioContext || window.webkitAudioContext;
    if (!audioCtx) {
      try {
        audioCtx = new AC({ sampleRate: peerRate });
      } catch (_) {
        audioCtx = new AC();
      }
    }
    if (!playCtx) {
      try {
        playCtx = new AC({ sampleRate: peerRate });
      } catch (_) {
        playCtx = new AC();
      }
      playTime = playCtx.currentTime;
    }
    var jobs = [];
    if (audioCtx.state === "suspended") {
      jobs.push(audioCtx.resume().catch(function () { /* still try capture */ }));
    }
    if (playCtx.state === "suspended") {
      jobs.push(playCtx.resume().catch(function () { /* still try playback */ }));
    }
    return jobs.length ? Promise.all(jobs) : Promise.resolve();
  }

  function startMicCapture(ws) {
    return unlockAudioContexts().then(function () {
      if (!mediaStream) {
        throw new Error("microphone stream missing");
      }
      var source = audioCtx.createMediaStreamSource(mediaStream);
      // ScriptProcessor: lab-friendly; 4096 frames at ctx rate → downsample to peerRate
      var bufferSize = 4096;
      processor = audioCtx.createScriptProcessor(bufferSize, 1, 1);
      micBytesSent = 0;
      processor.onaudioprocess = function (ev) {
        if (!ws || ws.readyState !== 1) return;
        if (audioCtx && audioCtx.state === "suspended") {
          audioCtx.resume();
          return;
        }
        // Mute uplink while bot TTS is playing — otherwise STT hears the greeting
        // from speakers and treats it as the user (acoustic echo).
        if (playCtx && playTime > playCtx.currentTime + 0.12) {
          return;
        }
        var input = ev.inputBuffer.getChannelData(0);
        var pcm = floatTo16BitPCM(input, audioCtx.sampleRate, peerRate);
        try {
          ws.send(pcm);
          micBytesSent += pcm.byteLength;
        } catch (_) { /* ignore */ }
      };
      source.connect(processor);
      var mute = audioCtx.createGain();
      mute.gain.value = 0;
      processor.connect(mute);
      mute.connect(audioCtx.destination);
      if (micStatusTimer) clearInterval(micStatusTimer);
      micStatusTimer = setInterval(function () {
        if (!callActive) return;
        if (micBytesSent > 0) {
          setCallStatus("On call · mic uplink " + Math.round(micBytesSent / 1024) + " KB", "ok");
        } else {
          setCallStatus("On call · mic silent (check permission / AudioContext)", "warn");
        }
      }, 2000);
    });
  }

  function teardownMedia() {
    if (micStatusTimer) {
      clearInterval(micStatusTimer);
      micStatusTimer = null;
    }
    micBytesSent = 0;
    try {
      if (processor) {
        processor.disconnect();
        processor.onaudioprocess = null;
        processor = null;
      }
    } catch (_) { /* ignore */ }
    try {
      if (audioCtx) {
        audioCtx.close();
        audioCtx = null;
      }
    } catch (_) { /* ignore */ }
    try {
      if (playCtx) {
        playCtx.close();
        playCtx = null;
        playTime = 0;
      }
    } catch (_) { /* ignore */ }
    if (edgeWs) {
      try { edgeWs.close(); } catch (_) { /* ignore */ }
      edgeWs = null;
    }
    if (mediaStream) {
      mediaStream.getTracks().forEach(function (t) { t.stop(); });
      mediaStream = null;
    }
  }

  function connectEdge(token) {
    return new Promise(function (resolve, reject) {
      var url = edgeWsUrl(token);
      var ws;
      try {
        ws = new WebSocket(url);
      } catch (err) {
        reject(err);
        return;
      }
      ws.binaryType = "arraybuffer";
      var opened = false;
      var timer = setTimeout(function () {
        if (!opened) {
          try { ws.close(); } catch (_) { /* ignore */ }
          reject(new Error("edge WebSocket connect timeout"));
        }
      }, 8000);
      ws.onopen = function () {
        opened = true;
        clearTimeout(timer);
        edgeWs = ws;
        startMicCapture(ws).then(function () {
          resolve(ws);
        }).catch(function (err) {
          reject(err);
        });
      };
      ws.onmessage = function (ev) {
        if (typeof ev.data !== "string") return;
        try {
          var msg = JSON.parse(ev.data);
          if (msg && msg.type === "streamAudio" && msg.data && msg.data.audioData) {
            playPCM16(msg.data.audioData, msg.data.sampleRate || peerRate);
          }
        } catch (_) { /* ignore */ }
      };
      ws.onerror = function () {
        if (!opened) {
          clearTimeout(timer);
          reject(new Error("edge WebSocket error"));
        }
      };
      ws.onclose = function () {
        if (callActive && sessionId) {
          setCallStatus("Edge disconnected", "warn");
        }
      };
    });
  }

  function startCall() {
    var profileId = el("profile").value.trim();
    if (!profileId) {
      showBanner(el("banner"), "bad", "No profile selected. Publish one in Admin first.");
      return Promise.resolve();
    }
    if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
      showBanner(el("banner"), "bad", "This browser cannot access the microphone (needs localhost or HTTPS).");
      return Promise.resolve();
    }
    el("chat").innerHTML = "";
    setLiveCaption("");
    setCallStatus("Requesting mic…", "warn");
    // Unlock playback/mic AudioContext while still in the click gesture chain.
    return unlockAudioContexts().then(function () {
      return navigator.mediaDevices.getUserMedia({
        audio: {
          // Speakers + AEC often cancel the user's voice while TTS plays (lab).
          echoCancellation: false,
          noiseSuppression: false,
          autoGainControl: true,
          channelCount: 1,
        },
        video: false,
      });
    }).then(function (stream) {
      mediaStream = stream;
      setCallStatus("Starting session…", "warn");
      return api("POST", "/v1/sessions", {
        profile_id: profileId,
        profile_version: "latest",
        clock: el("clock").value,
      });
    }).then(function (created) {
      sessionId = created.session_id;
      setJson("createJson", created);
      var pills = [];
      if (created.gateway_binding) {
        pills.push('<span class="pill ok">' + created.gateway_binding.listen + " / " +
          created.gateway_binding.think + " / " + created.gateway_binding.speak + "</span>");
      }
      pills.push('<span class="pill">' + (created.state || "created") + "</span>");
      pills.push("<span class=\"pill\"><code>" + sessionId + "</code></span>");
      el("sessionMeta").innerHTML = pills.join(" ");
      if (!created.edge_token) {
        throw new Error("session create missing edge_token");
      }
      openSSE(sessionId);
      setCallStatus("Connecting audio edge…", "warn");
      return connectEdge(created.edge_token).then(function () {
        return created;
      });
    }).then(function () {
      setCallStatus("Answering…", "warn");
      return api("POST", "/v1/sessions/" + encodeURIComponent(sessionId) + "/answer", {});
    }).then(function (ans) {
      setJson("turnJson", ans);
      setActive(true);
      setCallStatus("On call", "ok");
      showBanner(el("banner"), "ok",
        "Call live — speak after the greeting (headphones help). Status shows mic uplink KB while audio is sent.");
      if (ans && ans.spoken) {
        appendBubble("assistant", ans.spoken, "opening");
      }
      return refreshTranscript();
    }).catch(function (e) {
      setJson("createJson", { error: formatApiError(e), details: e.details || null, body: e.body || null });
      var tip = formatApiError(e);
      if (/Permission|NotAllowed|getUserMedia/i.test(String(e && e.message ? e.message : e))) {
        tip = "Microphone permission denied — allow mic for this site and try again.";
      }
      showBanner(el("banner"), "bad", "Start call failed: " + tip);
      teardownMedia();
      setActive(false);
      sessionId = "";
      setCallStatus("Idle", "");
      return refreshReady();
    });
  }

  function sendTurn() {
    var text = el("msg").value.trim();
    if (!sessionId || !text) return Promise.resolve();
    appendBubble("user", text, "you (text)");
    el("msg").value = "";
    return api("POST", "/v1/sessions/" + encodeURIComponent(sessionId) + "/inject", {
      text: text,
      speak: true,
    }).then(function (data) {
      setJson("turnJson", data);
      return refreshTranscript();
    }).catch(function (e) {
      setJson("turnJson", { error: formatApiError(e), body: e.body || null });
      showBanner(el("banner"), "bad", "Turn failed: " + formatApiError(e));
      appendBubble("assistant", "⚠ " + formatApiError(e), "error");
    });
  }

  function refreshTranscript() {
    if (!sessionId) return Promise.resolve();
    return api("GET", "/v1/sessions/" + encodeURIComponent(sessionId) + "/transcript").then(function (tr) {
      var turns = tr.turns || [];
      var partial = liveCaptionText;
      el("chat").innerHTML = "";
      liveCaptionEl = null;
      turns.forEach(function (t) {
        appendBubble(t.role || "assistant", t.text || "", (t.role || "") + " · seq " + t.seq);
      });
      if (partial) setLiveCaption(partial);
    }).catch(function (e) {
      setJson("turnJson", { transcript_error: formatApiError(e) });
    });
  }

  function endCall() {
    if (!sessionId) {
      teardownMedia();
      setActive(false);
      setCallStatus("Idle", "");
      return Promise.resolve();
    }
    var sid = sessionId;
    teardownMedia();
    return api("POST", "/v1/sessions/" + encodeURIComponent(sid) + "/stop", { reason: "user" })
      .then(function (data) {
        setJson("createJson", data);
        showBanner(el("banner"), "ok", "Call ended. Open Supervisor for audit / disposition.");
      })
      .catch(function (e) {
        showBanner(el("banner"), "bad", "End call failed: " + formatApiError(e));
      })
      .then(function () {
        closeSSE();
        setActive(false);
        sessionId = "";
        setCallStatus("Idle", "");
        return refreshReady();
      });
  }

  /* ------------------------------------------------- text call on a desk */

  var esc = CoralUI.esc;
  var deskCallId = "";

  /* Quick-fire phrases so a reviewer can walk the Coral journeys, including
     Hindi, without typing Devanagari on an English keyboard. */
  var QUICK_PHRASES = [
    { label: "Sales", text: "I want to buy IP phones for my office" },
    { label: "Tech support", text: "My IP phone is not working" },
    { label: "Complaint", text: "I want to register a complaint" },
    { label: "Product info", text: "Tell me about your media gateway" },
    { label: "हिंदी: शिकायत", text: "मुझे शिकायत दर्ज करानी है" },
    { label: "हिंदी: सपोर्ट", text: "मेरा आईपी फ़ोन काम नहीं कर रहा है" },
    { label: "हाँ", text: "हाँ" },
    { label: "Yes", text: "yes" },
    { label: "No", text: "no" },
    { label: "Switch to English", text: "English mein baat karo" },
    { label: "Agent please", text: "connect me to an agent" },
  ];

  function renderQuick() {
    el("deskQuick").innerHTML = QUICK_PHRASES.map(function (q, i) {
      return '<button type="button" class="secondary" data-quick="' + i + '">' + esc(q.label) + "</button>";
    }).join("");
    Array.prototype.forEach.call(el("deskQuick").querySelectorAll("[data-quick]"), function (btn) {
      btn.addEventListener("click", function () {
        if (!deskCallId) return;
        el("deskMsg").value = QUICK_PHRASES[Number(btn.getAttribute("data-quick"))].text;
        deskSend();
      });
    });
  }

  function loadDeskPicker() {
    return api("GET", "/v1/desks").then(function (data) {
      var rows = (data.desks || []).filter(function (d) { return d.current_version > 0; });
      var all = data.desks || [];
      var use = rows.length ? rows : all;
      el("deskPick").innerHTML = use.map(function (d) {
        var suffix = d.current_version > 0 ? " (v" + d.current_version + ")" : " (draft)";
        return '<option value="' + esc(d.id) + '">' + esc(d.name || d.id) + esc(suffix) + "</option>";
      }).join("") || '<option value="">no desks — install one in Admin</option>';
    }).catch(function () {
      el("deskPick").innerHTML = '<option value="">desk list unavailable</option>';
    });
  }

  function setDeskActive(on) {
    el("btnDeskSend").disabled = !on;
    el("btnDeskSilence").disabled = !on;
    el("deskMsg").disabled = !on;
    el("deskMsg").placeholder = on ? "Type what the caller says…" : "Start a text call first…";
    el("deskPick").disabled = on;
    el("deskLang").disabled = on;
  }

  function renderDeskCall(snap) {
    var pills = [
      '<span class="pill">' + esc(snap.id) + "</span>",
      '<span class="pill">' + esc(snap.language) + "</span>",
    ];
    var st = snap.state || {};
    if (st.path_id) pills.push('<span class="pill">path: ' + esc(st.path_id) + "</span>");
    if (snap.attributes && snap.attributes.ticket_id) {
      pills.push('<span class="pill ok">ticket: ' + esc(snap.attributes.ticket_id) + "</span>");
    }
    if (snap.attributes && snap.attributes.transfer_target) {
      pills.push('<span class="pill ok">transfer: ' + esc(snap.attributes.transfer_target) + "</span>");
    }
    if (snap.ended) {
      pills.push('<span class="pill warn">call ended · ' + esc(snap.disposition || "unset") + "</span>");
    }
    el("deskMeta").innerHTML = pills.join(" ");

    var turns = snap.turns || [];
    el("deskChat").innerHTML = turns.map(function (t) {
      var out = "";
      if (t.user) {
        out += '<div class="bubble user"><div class="meta">caller</div>' + esc(t.user) + "</div>";
      }
      var meta = ["assistant", t.language].concat(t.intent ? [t.intent] : []).join(" · ");
      var skills = (t.skills || []).map(function (s) {
        var kind = s.status === "ok" ? "ok" : "bad";
        return '<span class="pill ' + kind + '">' + esc(s.name) + " → " + esc(s.status) + "</span>";
      }).join(" ");
      out += '<div class="bubble assistant"><div class="meta">' + esc(meta) + "</div>" +
        esc(t.assistant) + (skills ? '<div style="margin-top:0.4rem">' + skills + "</div>" : "") + "</div>";
      return out;
    }).join("");
    el("deskChat").scrollTop = el("deskChat").scrollHeight;

    setJson("deskStateJson", {
      attributes: snap.attributes,
      handoff: snap.handoff,
      state: snap.state,
      disposition: snap.disposition,
    });

    if (snap.ended) setDeskActive(false);
  }

  function deskStart() {
    var id = el("deskPick").value;
    if (!id) {
      showBanner(el("banner"), "bad", "No desk available. Install the Coral TFN preset in Admin first.");
      return Promise.resolve();
    }
    return api("POST", "/v1/desk-calls", {
      desk_id: id,
      language: el("deskLang").value,
      ani: el("deskAni").value.trim(),
    }).then(function (snap) {
      deskCallId = snap.id;
      setDeskActive(true);
      renderDeskCall(snap);
      el("deskMsg").focus();
      showBanner(el("banner"), "ok", "Text call started on <code>" + esc(id) + "</code>.");
    });
  }

  function deskSend() {
    var text = el("deskMsg").value.trim();
    if (!deskCallId || !text) return Promise.resolve();
    el("deskMsg").value = "";
    return api("POST", "/v1/desk-calls/" + encodeURIComponent(deskCallId) + "/turn", { text: text })
      .then(renderDeskCall)
      .catch(function (e) {
        showBanner(el("banner"), "bad", formatApiError(e));
      });
  }

  function deskSilence() {
    if (!deskCallId) return Promise.resolve();
    return api("POST", "/v1/desk-calls/" + encodeURIComponent(deskCallId) + "/silence", {})
      .then(renderDeskCall);
  }

  try {
    setActive(false);
    setDeskActive(false);
    renderQuick();
    setCallStatus("Idle", "");
    bindClick("btnDeskStart", deskStart);
    bindClick("btnDeskSend", deskSend);
    bindClick("btnDeskSilence", deskSilence);
    el("deskMsg").addEventListener("keydown", function (ev) {
      if (ev.key === "Enter") {
        ev.preventDefault();
        deskSend();
      }
    });
    bindClick("btnReady", refreshReady);
    bindClick("btnStart", startCall);
    bindClick("btnStop", endCall);
    bindClick("btnSend", sendTurn);
    bindClick("btnRefreshTx", refreshTranscript);
    el("msg").addEventListener("keydown", function (ev) {
      if (ev.key === "Enter") {
        ev.preventDefault();
        sendTurn();
      }
    });
    loadDeskPicker().then(loadProfiles).then(refreshReady);
  } catch (err) {
    showBanner(el("banner"), "bad", "User UI failed to start: " + (err && err.message ? err.message : err));
  }
});
