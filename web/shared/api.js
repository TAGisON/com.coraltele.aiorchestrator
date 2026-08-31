/** Shared Control API client + error helpers for Admin / Supervisor / User UIs. */
const API = "";

function $(id) {
  return document.getElementById(id);
}

function showBanner(el, kind, html) {
  if (!el) return;
  el.hidden = !html;
  el.className = "banner" + (kind ? " " + kind : "");
  el.innerHTML = html || "";
}

async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  let res;
  try {
    res = await fetch(API + path, opts);
  } catch (err) {
    const e = new Error("network_unreachable: " + (err && err.message ? err.message : String(err)));
    e.code = "network_unreachable";
    e.status = 0;
    throw e;
  }
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch (_) {
    data = { raw: text };
  }
  if (!res.ok) {
    const msg = (data && data.error && data.error.message) || res.statusText || "request failed";
    const e = new Error(msg);
    e.status = res.status;
    e.code = data && data.error && data.error.code;
    e.details = data && data.error && data.error.details;
    e.body = data;
    throw e;
  }
  return data;
}

function formatApiError(err) {
  if (!err) return "unknown error";
  const bits = [];
  if (err.status) bits.push("HTTP " + err.status);
  if (err.code) bits.push(err.code);
  bits.push(err.message || String(err));
  if (err.details && err.details.hint) bits.push("hint: " + err.details.hint);
  return bits.join(" · ");
}

function explainBlocker(code) {
  const map = {
    database_unreachable: "Database is unreachable — control writes will fail.",
    tenant_engines_missing: "Tenant engines not configured — Admin must PUT listen/think/speak.",
    tenant_engines_lookup_failed: "Could not read tenant engines from the store.",
    no_profiles: "No behaviour profiles published — Admin must create and publish a profile.",
    gateway_registry_incomplete: "Process gateway registry missing listen/think/speak adapters.",
    store_memory_not_durable: "Running on in-memory store — data is lost on restart.",
    sarvam_credential_missing: "Engines use Sarvam but no API key in DB — Talk may fail at vendor.",
  };
  if (map[code]) return map[code];
  if (String(code).startsWith("gateway_not_registered:")) {
    return "Configured engine id is not registered in this process: " + String(code).split(":")[1];
  }
  return code;
}

async function loadPlatformStatus() {
  return api("GET", "/v1/platform/status");
}

function renderReadiness(listEl, status) {
  if (!listEl || !status) return;
  const items = [];
  const push = (ok, label, detail) => {
    items.push(
      `<li><span class="mark pill ${ok ? "ok" : "bad"}">${ok ? "OK" : "BLOCK"}</span><div><strong>${label}</strong><div class="hint" style="margin:0">${detail || ""}</div></div></li>`
    );
  };
  push(!!status.db, "Database", status.db ? status.store_backend : "unreachable");
  push(!!status.engines_configured, "Tenant engines", status.engines_configured
    ? `${status.engines.listen} / ${status.engines.think} / ${status.engines.speak}`
    : "not configured");
  const pc = status.profiles && status.profiles.count;
  push(pc > 0, "Profiles", pc > 0 ? `${pc} profile(s)` : "none published");
  const gw = status.gateways || {};
  push((gw.listen || 0) > 0 && (gw.think || 0) > 0 && (gw.speak || 0) > 0, "Gateway registry",
    `listen=${gw.listen || 0} think=${gw.think || 0} speak=${gw.speak || 0}`);
  (status.warnings || []).forEach((w) => {
    items.push(
      `<li><span class="mark pill warn">WARN</span><div><strong>${w}</strong><div class="hint" style="margin:0">${explainBlocker(w)}</div></div></li>`
    );
  });
  (status.blockers || []).forEach((b) => {
    if (["database_unreachable", "tenant_engines_missing", "no_profiles", "gateway_registry_incomplete"].includes(b) ||
        String(b).startsWith("gateway_not_registered:")) {
      return; // already reflected above
    }
    items.push(
      `<li><span class="mark pill bad">BLOCK</span><div><strong>${b}</strong><div class="hint" style="margin:0">${explainBlocker(b)}</div></div></li>`
    );
  });
  listEl.innerHTML = items.join("") || "<li>No status</li>";
}

window.CoralUI = {
  $,
  api,
  showBanner,
  formatApiError,
  explainBlocker,
  loadPlatformStatus,
  renderReadiness,
};
