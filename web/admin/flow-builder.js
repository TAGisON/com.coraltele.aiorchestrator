/* Admin flow graph builder (A.4). Catalog-driven enums; syncs a coral.flow.v1 doc object. */
(function (global) {
  function emptyDoc() {
    return {
      schema_id: "coral.flow.v1",
      entry_node_id: "",
      default_locale: "en-IN",
      nodes: [],
      edges: [],
      prompts: {},
      matrix: [],
      binding_refs: [],
    };
  }

  function clone(doc) {
    return JSON.parse(JSON.stringify(doc));
  }

  function ensureArrays(doc) {
    if (!doc.nodes) doc.nodes = [];
    if (!doc.edges) doc.edges = [];
    if (!doc.prompts) doc.prompts = {};
    if (!doc.matrix) doc.matrix = [];
    if (!doc.binding_refs) doc.binding_refs = [];
    if (!doc.schema_id) doc.schema_id = "coral.flow.v1";
    return doc;
  }

  function nodeIds(doc) {
    return (doc.nodes || []).map((n) => n.id).filter(Boolean);
  }

  function fillSelect(sel, values, selected) {
    AdminUI.fillSelect(sel, values, selected);
  }

  function renderNodes(doc, catalog, container, onChange) {
    container.innerHTML = "";
    (doc.nodes || []).forEach((n, idx) => {
      const card = document.createElement("div");
      card.className = "card";
      card.style.marginTop = "0.5rem";
      card.innerHTML =
        '<div class="field-row">' +
        '<div><label class="field">Node id</label><input class="n-id" type="text" /></div>' +
        '<div><label class="field">Type</label><select class="n-type"></select></div>' +
        '<div><label class="field">Tool</label><select class="n-tool"></select></div>' +
        '<div><label class="field">prompt_ref</label><input class="n-pref" type="text" /></div>' +
        '</div>' +
        '<div class="field-row">' +
        '<div><label class="field">binding_ref</label><input class="n-bref" type="text" list="binding-refs-list" /></div>' +
        '<div><label class="field">matrix_intent</label><input class="n-mint" type="text" /></div>' +
        '<div><label class="field">repair max_retries</label><input class="n-retries" type="number" min="0" /></div>' +
        '<div><label class="field">unclear_prompt_ref</label><input class="n-uref" type="text" /></div>' +
        '</div>' +
        '<div class="actions"><button type="button" class="btn secondary n-rm">Remove node</button></div>';
      card.querySelector(".n-id").value = n.id || "";
      fillSelect(card.querySelector(".n-type"), catalog.node_types || [], n.type || "Speak");
      fillSelect(card.querySelector(".n-tool"), catalog.tools || [], n.tool || "");
      card.querySelector(".n-pref").value = n.prompt_ref || "";
      card.querySelector(".n-bref").value = n.binding_ref || "";
      card.querySelector(".n-mint").value = n.matrix_intent || "";
      const repair = n.repair || {};
      card.querySelector(".n-retries").value = repair.max_retries != null ? repair.max_retries : "";
      card.querySelector(".n-uref").value = repair.unclear_prompt_ref || "";
      const sync = () => {
        n.id = card.querySelector(".n-id").value.trim();
        n.type = card.querySelector(".n-type").value;
        const tool = card.querySelector(".n-tool").value;
        if (tool) n.tool = tool;
        else delete n.tool;
        const pref = card.querySelector(".n-pref").value.trim();
        if (pref) n.prompt_ref = pref;
        else delete n.prompt_ref;
        const bref = card.querySelector(".n-bref").value.trim();
        if (bref) n.binding_ref = bref;
        else delete n.binding_ref;
        const mint = card.querySelector(".n-mint").value.trim();
        if (mint) n.matrix_intent = mint;
        else delete n.matrix_intent;
        const retries = card.querySelector(".n-retries").value;
        const uref = card.querySelector(".n-uref").value.trim();
        if (retries !== "" || uref) {
          n.repair = {};
          if (retries !== "") n.repair.max_retries = parseInt(retries, 10);
          if (uref) n.repair.unclear_prompt_ref = uref;
        } else delete n.repair;
        onChange();
      };
      card.querySelectorAll("input, select").forEach((el) => {
        el.addEventListener("change", sync);
        el.addEventListener("blur", sync);
      });
      card.querySelector(".n-rm").onclick = () => {
        doc.nodes.splice(idx, 1);
        onChange(true);
      };
      container.appendChild(card);
    });
  }

  function renderEdges(doc, catalog, container, onChange) {
    container.innerHTML = "";
    const ids = nodeIds(doc);
    (doc.edges || []).forEach((e, idx) => {
      const card = document.createElement("div");
      card.className = "card";
      card.style.marginTop = "0.5rem";
      card.innerHTML =
        '<div class="field-row">' +
        '<div><label class="field">Edge id</label><input class="e-id" type="text" /></div>' +
        '<div><label class="field">Kind</label><select class="e-kind"></select></div>' +
        '<div><label class="field">From</label><select class="e-from"></select></div>' +
        '<div><label class="field">To</label><select class="e-to"></select></div>' +
        '</div>' +
        '<div class="field-row">' +
        '<div><label class="field">intent</label><input class="e-intent" type="text" /></div>' +
        '<div><label class="field">option</label><input class="e-option" type="text" /></div>' +
        '<div><label class="field">matrix_intent</label><input class="e-mint" type="text" /></div>' +
        '</div>' +
        '<div class="actions"><button type="button" class="btn secondary e-rm">Remove edge</button></div>';
      card.querySelector(".e-id").value = e.id || "";
      fillSelect(card.querySelector(".e-kind"), catalog.edge_kinds || [], e.kind || "next");
      fillSelect(card.querySelector(".e-from"), ids, e.from || "");
      fillSelect(card.querySelector(".e-to"), ids, e.to || "");
      card.querySelector(".e-intent").value = e.intent || "";
      card.querySelector(".e-option").value = e.option || "";
      card.querySelector(".e-mint").value = e.matrix_intent || "";
      const sync = () => {
        e.id = card.querySelector(".e-id").value.trim();
        e.kind = card.querySelector(".e-kind").value;
        e.from = card.querySelector(".e-from").value;
        e.to = card.querySelector(".e-to").value;
        const intent = card.querySelector(".e-intent").value.trim();
        if (intent) e.intent = intent;
        else delete e.intent;
        const opt = card.querySelector(".e-option").value.trim();
        if (opt) e.option = opt;
        else delete e.option;
        const mint = card.querySelector(".e-mint").value.trim();
        if (mint) e.matrix_intent = mint;
        else delete e.matrix_intent;
        onChange();
      };
      card.querySelectorAll("input, select").forEach((el) => {
        el.addEventListener("change", sync);
        el.addEventListener("blur", sync);
      });
      card.querySelector(".e-rm").onclick = () => {
        doc.edges.splice(idx, 1);
        onChange(true);
      };
      container.appendChild(card);
    });
  }

  function renderMatrix(doc, catalog, container, onChange) {
    container.innerHTML = "";
    (doc.matrix || []).forEach((row, idx) => {
      const card = document.createElement("div");
      card.className = "card";
      card.style.marginTop = "0.5rem";
      card.innerHTML =
        '<div class="field-row">' +
        '<div><label class="field">intent</label><input class="m-intent" type="text" /></div>' +
        '<div><label class="field">owner</label><input class="m-owner" type="text" /></div>' +
        '<div><label class="field">target</label><input class="m-target" type="text" /></div>' +
        '<div><label class="field">number</label><input class="m-num" type="text" /></div>' +
        '</div>' +
        '<div class="field-row">' +
        '<div><label class="field">action</label><select class="m-action"></select></div>' +
        '<div><label class="field">disposition_code</label><select class="m-disp"></select></div>' +
        '</div>' +
        '<div class="actions"><button type="button" class="btn secondary m-rm">Remove row</button></div>';
      card.querySelector(".m-intent").value = row.intent || "";
      card.querySelector(".m-owner").value = row.owner || "";
      card.querySelector(".m-target").value = row.target || "";
      card.querySelector(".m-num").value = row.number || "";
      fillSelect(card.querySelector(".m-action"), catalog.matrix_actions || ["transfer"], row.action || "transfer");
      fillSelect(card.querySelector(".m-disp"), catalog.disposition_final || [], row.disposition_code || "");
      const sync = () => {
        row.intent = card.querySelector(".m-intent").value.trim();
        row.owner = card.querySelector(".m-owner").value.trim();
        row.target = card.querySelector(".m-target").value.trim();
        row.number = card.querySelector(".m-num").value.trim();
        row.action = card.querySelector(".m-action").value;
        row.disposition_code = card.querySelector(".m-disp").value;
        onChange();
      };
      card.querySelectorAll("input, select").forEach((el) => {
        el.addEventListener("change", sync);
        el.addEventListener("blur", sync);
      });
      card.querySelector(".m-rm").onclick = () => {
        doc.matrix.splice(idx, 1);
        onChange(true);
      };
      container.appendChild(card);
    });
  }

  function renderPrompts(doc, container, onChange) {
    container.innerHTML = "";
    Object.keys(doc.prompts || {}).forEach((key) => {
      const locales = doc.prompts[key] || {};
      const card = document.createElement("div");
      card.className = "card";
      card.style.marginTop = "0.5rem";
      const firstLoc = Object.keys(locales)[0] || "en-IN";
      card.innerHTML =
        '<div class="field-row">' +
        '<div><label class="field">prompt key</label><input class="p-key" type="text" /></div>' +
        '<div><label class="field">locale</label><input class="p-loc" type="text" /></div>' +
        '</div>' +
        '<label class="field">text</label><input class="p-text" type="text" />' +
        '<div class="actions"><button type="button" class="btn secondary p-rm">Remove prompt</button></div>';
      card.querySelector(".p-key").value = key;
      card.querySelector(".p-loc").value = firstLoc;
      card.querySelector(".p-text").value = locales[firstLoc] || "";
      const sync = () => {
        const nk = card.querySelector(".p-key").value.trim();
        const loc = card.querySelector(".p-loc").value.trim() || "en-IN";
        const text = card.querySelector(".p-text").value;
        if (!nk) return;
        if (nk !== key) {
          delete doc.prompts[key];
          key = nk;
        }
        doc.prompts[key] = doc.prompts[key] || {};
        doc.prompts[key][loc] = text;
        onChange();
      };
      card.querySelectorAll("input").forEach((el) => {
        el.addEventListener("change", sync);
        el.addEventListener("blur", sync);
      });
      card.querySelector(".p-rm").onclick = () => {
        delete doc.prompts[key];
        onChange(true);
      };
      container.appendChild(card);
    });
  }

  global.FlowBuilder = {
    emptyDoc,
    clone,
    ensureArrays,
    nodeIds,
    renderNodes,
    renderEdges,
    renderMatrix,
    renderPrompts,
  };
})(typeof window !== "undefined" ? window : globalThis);
