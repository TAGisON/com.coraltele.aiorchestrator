/* Shared Control API client (U.2/A.1). Bearer from localStorage orch_token when AuthToken is set. */
(function (global) {
  const TOKEN_KEY = "orch_token";

  function token() {
    try {
      return (global.localStorage && global.localStorage.getItem(TOKEN_KEY)) || "";
    } catch (_) {
      return "";
    }
  }

  function setToken(value) {
    try {
      if (!global.localStorage) return;
      const v = (value || "").trim();
      if (v) global.localStorage.setItem(TOKEN_KEY, v);
      else global.localStorage.removeItem(TOKEN_KEY);
    } catch (_) {}
  }

  function errMessage(data, fallback) {
    if (!data) return fallback;
    if (typeof data.error === "object" && data.error && data.error.message) return data.error.message;
    if (data.message) return data.message;
    if (typeof data.error === "string") return data.error;
    return fallback;
  }

  async function request(method, path, body, opts) {
    opts = opts || {};
    const headers = { Accept: "application/json" };
    const t = token();
    if (t) headers.Authorization = "Bearer " + t;
    const init = { method, headers };
    if (body !== undefined) {
      if (opts.rawBody) {
        if (opts.contentType) headers["Content-Type"] = opts.contentType;
        init.body = body;
      } else {
        headers["Content-Type"] = "application/json";
        init.body = typeof body === "string" ? body : JSON.stringify(body);
      }
    }
    const res = await fetch(path, init);
    const text = await res.text();
    let data = null;
    if (text) {
      try {
        data = JSON.parse(text);
      } catch (_) {
        data = { raw: text };
      }
    }
    if (!res.ok) {
      const err = new Error(errMessage(data, res.statusText || "request failed"));
      err.status = res.status;
      err.data = data;
      throw err;
    }
    return data;
  }

  global.OrchAPI = {
    TOKEN_KEY,
    token,
    setToken,
    get: (path) => request("GET", path),
    post: (path, body) => request("POST", path, body),
    put: (path, body) => request("PUT", path, body),
    patch: (path, body) => request("PATCH", path, body),
    del: (path) => request("DELETE", path),
    putRaw: (path, body, contentType) => request("PUT", path, body, { rawBody: true, contentType: contentType }),
    catalog: () => request("GET", "/v1/meta/catalog"),
    platformStatus: () => request("GET", "/v1/platform/status"),
    gateways: () => request("GET", "/v1/gateways"),
    getEngines: () => request("GET", "/v1/tenant/engines"),
    putEngines: (body) => request("PUT", "/v1/tenant/engines", body),
    listCredentials: () => request("GET", "/v1/tenant/credentials"),
    putCredential: (gatewayId, body) => request("PUT", "/v1/tenant/credentials/" + encodeURIComponent(gatewayId), body),
    listSettings: () => request("GET", "/v1/tenant/settings"),
    putSetting: (key, value) => request("PUT", "/v1/tenant/settings/" + encodeURIComponent(key), { value: value }),
    listFallback: () => request("GET", "/v1/tenant/fallback"),
    putFallback: (scenario, wavBytes) =>
      request("PUT", "/v1/tenant/fallback/" + encodeURIComponent(scenario), wavBytes, {
        rawBody: true,
        contentType: "audio/wav",
      }),
    listBindings: (qs) => request("GET", "/v1/bindings" + (qs ? "?" + qs : "")),
    getBinding: (id) => request("GET", "/v1/bindings/" + encodeURIComponent(id)),
    putBinding: (id, body) => request("PUT", "/v1/bindings/" + encodeURIComponent(id), body),
    listFlows: (qs) => request("GET", "/v1/flows" + (qs ? "?" + qs : "")),
    getFlow: (id) => request("GET", "/v1/flows/" + encodeURIComponent(id)),
    createFlow: (body) => request("POST", "/v1/flows", body),
    getFlowDraft: (id) => request("GET", "/v1/flows/" + encodeURIComponent(id) + "/draft"),
    putFlowDraft: (id, doc) =>
      request("PUT", "/v1/flows/" + encodeURIComponent(id) + "/draft", typeof doc === "string" ? doc : JSON.stringify(doc)),
    publishFlow: (id, doc) =>
      request("POST", "/v1/flows/" + encodeURIComponent(id) + "/versions", typeof doc === "string" ? doc : JSON.stringify(doc)),
    getFlowVersion: (id, ver) =>
      request("GET", "/v1/flows/" + encodeURIComponent(id) + "/versions/" + encodeURIComponent(String(ver))),
    getAnswerPins: () => request("GET", "/v1/tenant/answer-pins"),
    putAnswerPins: (body) => request("PUT", "/v1/tenant/answer-pins", body),
    createSession: (body) => request("POST", "/v1/sessions", body),
    getSession: (id) => request("GET", "/v1/sessions/" + encodeURIComponent(id)),
    answerSession: (id, body) =>
      request("POST", "/v1/sessions/" + encodeURIComponent(id) + "/answer", body || {}),
    injectText: (id, body) =>
      request("POST", "/v1/sessions/" + encodeURIComponent(id) + "/inject", body),
    stopSession: (id, body) =>
      request("POST", "/v1/sessions/" + encodeURIComponent(id) + "/stop", body || {}),
    getTranscript: (id) =>
      request("GET", "/v1/sessions/" + encodeURIComponent(id) + "/transcript"),
    getDisposition: (id) =>
      request("GET", "/v1/sessions/" + encodeURIComponent(id) + "/disposition"),
    listSessions: (qs) => request("GET", "/v1/sessions" + (qs ? "?" + qs : "")),
    getAudit: (id) =>
      request("GET", "/v1/sessions/" + encodeURIComponent(id) + "/audit"),
    listProfiles: () => request("GET", "/v1/profiles"),
    createProfile: (body) => request("POST", "/v1/profiles", body),
    getProfileVersion: (id, ver) =>
      request("GET", "/v1/profiles/" + encodeURIComponent(id) + "/versions/" + encodeURIComponent(ver || "latest")),
    publishProfile: (id, doc) =>
      request("POST", "/v1/profiles/" + encodeURIComponent(id) + "/versions", doc),
  };
})(typeof window !== "undefined" ? window : globalThis);
