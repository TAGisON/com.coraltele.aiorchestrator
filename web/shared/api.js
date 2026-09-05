/* Shared Control API client (U.2). Bearer from localStorage orch_token when AuthToken is set. */
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

  async function request(method, path, body) {
    const headers = { Accept: "application/json" };
    const t = token();
    if (t) headers.Authorization = "Bearer " + t;
    const opts = { method, headers };
    if (body !== undefined) {
      headers["Content-Type"] = "application/json";
      opts.body = typeof body === "string" ? body : JSON.stringify(body);
    }
    const res = await fetch(path, opts);
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
      const err = new Error((data && (data.message || data.error)) || res.statusText || "request failed");
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
    catalog: () => request("GET", "/v1/meta/catalog"),
    platformStatus: () => request("GET", "/v1/platform/status"),
  };
})(typeof window !== "undefined" ? window : globalThis);
