/* Admin shell helpers (A.1). */
(function (global) {
  function bindTokenForm() {
    const input = document.getElementById("token");
    const btn = document.getElementById("save-token");
    const out = document.getElementById("token-out");
    if (!input || !btn || !global.OrchAPI) return;
    input.value = OrchAPI.token();
    btn.onclick = () => {
      OrchAPI.setToken(input.value);
      if (out) {
        out.textContent = "Token saved.";
        out.className = "probe ok";
      }
    };
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

  function gatewayIds(gateways, port) {
    const list = (gateways && gateways.gateways && gateways.gateways[port]) || [];
    return list.map((g) => g.id).filter(Boolean);
  }

  function allGatewayIds(gateways) {
    const ports = ["listen", "speak", "think", "translate", "knowledge", "skill"];
    const out = [];
    const seen = {};
    ports.forEach((p) => {
      gatewayIds(gateways, p).forEach((id) => {
        if (!seen[id]) {
          seen[id] = true;
          out.push(id);
        }
      });
    });
    return out;
  }

  function showStatus(el, ok, msg) {
    if (!el) return;
    el.className = "probe " + (ok ? "ok" : "err");
    el.textContent = msg;
  }

  global.AdminUI = { bindTokenForm, fillSelect, gatewayIds, allGatewayIds, showStatus };
})(typeof window !== "undefined" ? window : globalThis);
