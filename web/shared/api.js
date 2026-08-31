/** Shared Control API client + error helpers for Admin / Supervisor / User UIs. */
(function (global) {
  "use strict";

  var API = "";

  function el(id) {
    return document.getElementById(id);
  }

  var ESCAPES = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" };

  /** Escape untrusted text before it goes into innerHTML. */
  function esc(value) {
    return String(value == null ? "" : value).replace(/[&<>"]/g, function (c) {
      return ESCAPES[c];
    });
  }

  function shortTime(value) {
    if (!value) return "";
    var d = new Date(value);
    return isNaN(d.getTime()) ? String(value) : d.toLocaleString();
  }

  function showBanner(node, kind, html) {
    if (!node) return;
    node.hidden = !html;
    node.className = "banner" + (kind ? " " + kind : "");
    node.innerHTML = html || "";
  }

  function api(method, path, body) {
    var opts = { method: method, headers: {} };
    if (body !== undefined) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(body);
    }
    return fetch(API + path, opts).then(function (res) {
      return res.text().then(function (text) {
        var data = null;
        try {
          data = text ? JSON.parse(text) : null;
        } catch (_) {
          data = { raw: text };
        }
        if (!res.ok) {
          var msg = (data && data.error && data.error.message) || res.statusText || "request failed";
          var err = new Error(msg);
          err.status = res.status;
          err.code = data && data.error && data.error.code;
          err.details = data && data.error && data.error.details;
          err.body = data;
          throw err;
        }
        return data;
      });
    }).catch(function (err) {
      if (err && err.status) throw err;
      var e = new Error("network_unreachable: " + (err && err.message ? err.message : String(err)));
      e.code = "network_unreachable";
      e.status = 0;
      throw e;
    });
  }

  function formatApiError(err) {
    if (!err) return "unknown error";
    var bits = [];
    if (err.status) bits.push("HTTP " + err.status);
    if (err.code) bits.push(err.code);
    bits.push(err.message || String(err));
    if (err.details && err.details.hint) bits.push("hint: " + err.details.hint);
    return bits.join(" · ");
  }

  function explainBlocker(code) {
    var map = {
      database_unreachable: "Database is unreachable — control writes will fail.",
      tenant_engines_missing: "Tenant engines not configured — Admin must PUT listen/think/speak.",
      tenant_engines_lookup_failed: "Could not read tenant engines from the store.",
      no_profiles: "No behaviour profiles published — Admin must create and publish a profile.",
      gateway_registry_incomplete: "Process gateway registry missing listen/think/speak adapters.",
      store_memory_not_durable: "Running on in-memory store — data is lost on restart.",
      sarvam_credential_missing: "Engines use Sarvam but no API key in DB — Talk may fail at vendor.",
    };
    if (map[code]) return map[code];
    if (String(code).indexOf("gateway_not_registered:") === 0) {
      return "Configured engine id is not registered in this process: " + String(code).split(":")[1];
    }
    return code;
  }

  function loadPlatformStatus() {
    return api("GET", "/v1/platform/status");
  }

  function renderReadiness(listEl, status) {
    if (!listEl || !status) return;
    var items = [];
    function push(ok, label, detail) {
      items.push(
        '<li><span class="mark pill ' + (ok ? "ok" : "bad") + '">' + (ok ? "OK" : "BLOCK") +
          "</span><div><strong>" + label + '</strong><div class="hint" style="margin:0">' +
          (detail || "") + "</div></div></li>"
      );
    }
    push(!!status.db, "Database", status.db ? status.store_backend : "unreachable");
    push(!!status.engines_configured, "Tenant engines", status.engines_configured
      ? status.engines.listen + " / " + status.engines.think + " / " + status.engines.speak
      : "not configured");
    var pc = status.profiles && status.profiles.count;
    push(pc > 0, "Profiles", pc > 0 ? pc + " profile(s)" : "none published");
    var gw = status.gateways || {};
    push((gw.listen || 0) > 0 && (gw.think || 0) > 0 && (gw.speak || 0) > 0, "Gateway registry",
      "listen=" + (gw.listen || 0) + " think=" + (gw.think || 0) + " speak=" + (gw.speak || 0));
    (status.warnings || []).forEach(function (w) {
      items.push(
        '<li><span class="mark pill warn">WARN</span><div><strong>' + w +
          '</strong><div class="hint" style="margin:0">' + explainBlocker(w) + "</div></div></li>"
      );
    });
    (status.blockers || []).forEach(function (b) {
      if (["database_unreachable", "tenant_engines_missing", "no_profiles", "gateway_registry_incomplete"].indexOf(b) >= 0 ||
          String(b).indexOf("gateway_not_registered:") === 0) {
        return;
      }
      items.push(
        '<li><span class="mark pill bad">BLOCK</span><div><strong>' + b +
          '</strong><div class="hint" style="margin:0">' + explainBlocker(b) + "</div></div></li>"
      );
    });
    listEl.innerHTML = items.join("") || "<li>No status</li>";
  }

  function onReady(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  function bindClick(id, fn) {
    var node = el(id);
    if (!node) {
      console.error("missing element #" + id);
      return;
    }
    node.addEventListener("click", function (ev) {
      ev.preventDefault();
      try {
        var ret = fn(ev);
        if (ret && typeof ret.then === "function") {
          ret.catch(function (err) {
            console.error(id, err);
            showBanner(el("banner"), "bad", formatApiError(err));
          });
        }
      } catch (err) {
        console.error(id, err);
        showBanner(el("banner"), "bad", String(err && err.message ? err.message : err));
      }
    });
  }

  global.CoralUI = {
    el: el,
    api: api,
    esc: esc,
    shortTime: shortTime,
    showBanner: showBanner,
    formatApiError: formatApiError,
    explainBlocker: explainBlocker,
    loadPlatformStatus: loadPlatformStatus,
    renderReadiness: renderReadiness,
    onReady: onReady,
    bindClick: bindClick,
  };
})(window);
