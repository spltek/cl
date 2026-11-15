/* =============================================================================
   CL — COMMAND LAUNCHER
   A faithful, keyboard-driven replica of the cl picker, plus the interactive
   placeholder resolver and the usual page chrome.
   ============================================================================= */
(() => {
  "use strict";

  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  /* ---------------------------------------------------------------------------
     DATA — seed commands, mirroring the README examples
     --------------------------------------------------------------------------- */
  const SEED = [
    {
      name: "build",
      cmd: "npm run build -- --watch",
      out: ["vite v5.4.1 building for production...", "✓ 142 modules transformed · built in 1.92s"],
    },
    {
      name: "ssh server",
      cmd: "ssh {{user}}@{{host:prod.example.com}}",
      out: ["Last login: Tue Nov 12 09:41:07 on ttys002", "admin@prod:~$"],
    },
    { name: "deploy", cmd: "kubectl apply -f production/overlays/us-east-1/kustomization.yaml --prune --selector app=api", out: [] },
    { name: "git push", cmd: "git push {{remote:origin}} {{branch:main}}", out: ["Enumerating objects: 9, done.", "To github.com:spltek/cl.git · 4b0c2f1..9de117a  main -> main"] },
    { name: "go to project", cmd: "cd ~/projects/target-folder && exec $SHELL", out: ["spawned a fresh shell in ~/projects/target-folder", "target-folder $"] },
    {
      name: "logs api",
      cmd: "kubectl logs {{pod}} -n {{namespace:default}} --tail=200",
      out: ["api-7d9f8c6b5-x2vqn  09:41:12  INFO  server listening on :8080", "api-7d9f8c6b5-x2vqn  09:41:19  INFO  GET /healthz 200 1.4ms"],
    },
    {
      name: "psql main",
      cmd: "psql -h {{host:localhost}} -p {{port:5432}} -U {{user}} -d {{db:postgres}}",
      out: ["psql (16.2)", 'Type "help" for help.', "postgres=#"],
    },
    { name: "docker node", cmd: "docker run -p {{port:3000}}:3000 {{image:node:18}}", out: ["Unable to find image 'node:18' locally", "18: Pulling from library/node · Status: Downloaded newer image"] },
  ];

  const PH_RE = /\{\{(\w+)(?::([^}]*))?\}\}/g;

  function readPlaceholders(cmd) {
    const items = [];
    const seen = new Set();
    let m;
    PH_RE.lastIndex = 0;
    while ((m = PH_RE.exec(cmd))) {
      if (seen.has(m[1])) continue;
      seen.add(m[1]);
      items.push({ name: m[1], def: m[2] === undefined ? null : m[2] });
    }
    return items;
  }

  function hintFor(cmd) {
    const items = readPlaceholders(cmd);
    if (!items.length) return "";
    return (
      "(" +
      items.map((p) => (p.def === null ? p.name : `${p.name}[default:${p.def}]`)).join(", ") +
      ")"
    );
  }

  function resolveCommand(cmd, values) {
    return cmd.replace(/\{\{(\w+)(?::([^}]*))?\}\}/g, (_, name, def) => {
      const v = values[name];
      if (v !== undefined && v !== "") return v;
      return def === undefined ? "" : def;
    });
  }

  /* ---------------------------------------------------------------------------
     TERMINAL STATE
     --------------------------------------------------------------------------- */
  const st = {
    cmds: SEED.map((c) => ({ ...c })),
    filter: "",
    sel: 0,
    show: false,
    mode: "list", // list | addName | addCmd | edit | rename | confirmSave | confirmRename | confirmDelete | ph
    buf: "", // text buffer for the current prompt
    err: "",
    target: null, // index into st.cmds
    addName: "",
    pending: "", // new value awaiting y/N
    ph: null, // { name, cmd, items, i, vals }
    out: [],
    focused: false,
  };

  const ui = {
    root: $("#term"),
    screen: $("#termScreen"),
    invoke: $("#tInvoke"),
    invokeCaret: $("#tInvokeCaret"),
    filter: $("#tFilter"),
    filterCaret: $("#tFilterCaret"),
    list: $("#tList"),
    empty: $("#tEmpty"),
    ask: $("#tAsk"),
    askQ: $("#tAskQ"),
    askIn: $("#tAskIn"),
    askHint: $("#tAskHint"),
    out: $("#tOut"),
    status: $("#tStatus"),
  };

  const MODE_STATUS = {
    list: "demo · click to focus",
    addName: "add · name",
    addCmd: "add · command",
    edit: "edit · value",
    rename: "rename · name",
    confirmSave: "confirm save",
    confirmRename: "confirm rename",
    confirmDelete: "confirm delete",
    ph: "placeholder",
  };

  function visible() {
    const q = st.filter.trim().toLowerCase();
    if (!q) return st.cmds.map((c, i) => ({ c, i }));
    return st.cmds.map((c, i) => ({ c, i })).filter(({ c }) => c.name.toLowerCase().includes(q));
  }

  function selectedEntry() {
    const list = visible();
    if (!list.length) return null;
    st.sel = Math.max(0, Math.min(st.sel, list.length - 1));
    return list[st.sel];
  }

  function pushOut(kind, payload) {
    st.out.push({ kind, ...payload });
    if (st.out.length > 8) st.out.splice(0, st.out.length - 8);
  }

  function runEntry(entry, values) {
    const resolved = resolveCommand(entry.c.cmd, values || {});
    const vals = values ? Object.keys(values).map((k) => values[k]) : [];
    pushOut("exec", { name: entry.c.name, vals });
    const lines =
      entry.c.out && entry.c.out.length
        ? entry.c.out.slice()
        : [`[demo] "${entry.c.name}" launched · exit 0`];
    lines.forEach((line) => pushOut("res", { text: line }));
    if (st.show) pushOut("res", { text: "$ " + resolved });
  }

  function startPlaceholders(entry) {
    const items = readPlaceholders(entry.c.cmd);
    if (!items.length) {
      runEntry(entry, {});
      setMode("list");
      return;
    }
    st.ph = { entry, items, i: 0, vals: {} };
    st.mode = "ph";
    st.buf = items[0].def === null ? "" : items[0].def;
    st.err = "";
    render();
  }

  function setMode(mode) {
    st.mode = mode;
    st.err = "";
    render();
  }

  /* ---------------------------------------------------------------------------
     ACTIONS
     --------------------------------------------------------------------------- */
  function move(delta) {
    const list = visible();
    if (!list.length) return;
    st.sel = (st.sel + delta + list.length) % list.length;
    render();
  }

  function runSelected() {
    const entry = selectedEntry();
    if (!entry) return;
    startPlaceholders(entry);
  }

  function beginAdd() {
    st.mode = "addName";
    st.buf = "";
    st.err = "";
    render();
  }

  function beginEdit() {
    const entry = selectedEntry();
    if (!entry) return;
    st.target = entry.i;
    st.mode = "edit";
    st.buf = entry.c.cmd;
    st.err = "";
    render();
  }

  function beginRename() {
    const entry = selectedEntry();
    if (!entry) return;
    st.target = entry.i;
    st.mode = "rename";
    st.buf = entry.c.name;
    st.err = "";
    render();
  }

  function beginDelete() {
    const entry = selectedEntry();
    if (!entry) return;
    st.target = entry.i;
    st.mode = "confirmDelete";
    st.err = "";
    render();
  }

  function toggleShow() {
    st.show = !st.show;
    render();
    syncShowToggle();
  }

  function nameTaken(name, exceptIndex) {
    const n = name.trim().toLowerCase();
    return st.cmds.some((c, i) => i !== exceptIndex && c.name.toLowerCase() === n);
  }

  function cancel() {
    st.mode = "list";
    st.buf = "";
    st.err = "";
    st.ph = null;
    render();
  }

  function commitAdd() {
    const name = st.addName.trim();
    const cmd = st.buf.trim();
    if (!cmd) {
      st.err = "command cannot be empty";
      render();
      return;
    }
    st.cmds.push({ name, cmd, out: [] });
    st.addName = "";
    st.mode = "list";
    st.filter = "";
    st.buf = "";
    st.err = "";
    st.sel = st.cmds.length - 1;
    pushOut("info", { text: `saved "${name}" → commands.json` });
    render();
  }

  function commitEdit() {
    const value = st.buf.trim();
    if (!value) {
      st.err = "command cannot be empty";
      render();
      return;
    }
    st.cmds[st.target].cmd = value;
    pushOut("info", { text: `updated "${st.cmds[st.target].name}" → commands.json` });
    st.mode = "list";
    st.buf = "";
    render();
  }

  function commitRename() {
    const name = st.buf.trim();
    if (!name) {
      st.err = "name cannot be empty";
      render();
      return;
    }
    if (nameTaken(name, st.target)) {
      st.err = `"${name}" is already in use`;
      render();
      return;
    }
    const old = st.cmds[st.target].name;
    st.cmds[st.target].name = name;
    pushOut("info", { text: `renamed "${old}" → "${name}"` });
    st.mode = "list";
    st.buf = "";
    selectByName(name);
    render();
  }

  function commitDelete() {
    const c = st.cmds[st.target];
    pushOut("info", { text: `deleted "${c.name}" from commands.json` });
    st.cmds.splice(st.target, 1);
    st.mode = "list";
    st.target = null;
    st.sel = Math.min(st.sel, Math.max(0, visible().length - 1));
    render();
  }

  function selectByName(name) {
    const idx = visible().findIndex(({ c }) => c.name === name);
    if (idx >= 0) st.sel = idx;
    else if (st.filter) {
      st.filter = "";
      const all = visible().findIndex(({ c }) => c.name === name);
      st.sel = Math.max(0, all);
    }
  }

  function submitPrompt() {
    switch (st.mode) {
      case "addName": {
        const name = st.buf.trim();
        if (!name) {
          st.err = "name cannot be empty";
          render();
          return;
        }
        if (nameTaken(name, -1)) {
          st.err = `"${name}" is already in use`;
          render();
          return;
        }
        st.addName = name;
        st.mode = "addCmd";
        st.buf = "";
        st.err = "";
        render();
        return;
      }
      case "addCmd":
        commitAdd();
        return;
      case "edit":
        st.pending = st.buf.trim();
        st.mode = "confirmSave";
        render();
        return;
      case "rename":
        st.pending = st.buf.trim();
        st.mode = "confirmRename";
        render();
        return;
      case "ph": {
        const item = st.ph.items[st.ph.i];
        const value = st.buf.trim();
        if (!value && item.def === null) {
          st.err = `${item.name} is required`;
          render();
          return;
        }
        st.ph.vals[item.name] = value || item.def || "";
        st.ph.i += 1;
        st.err = "";
        if (st.ph.i >= st.ph.items.length) {
          const entry = st.ph.entry;
          const values = { ...st.ph.vals };
          st.ph = null;
          st.mode = "list";
          runEntry(entry, values);
          render();
          return;
        }
        const next = st.ph.items[st.ph.i];
        st.buf = next.def === null ? "" : next.def;
        render();
        return;
      }
      default:
        return;
    }
  }

  function answerConfirm(yes) {
    if (!yes) {
      st.pending = "";
      cancel();
      return;
    }
    if (st.mode === "confirmSave") {
      st.buf = st.pending;
      st.pending = "";
      commitEdit();
    } else if (st.mode === "confirmRename") {
      st.buf = st.pending;
      st.pending = "";
      commitRename();
    } else if (st.mode === "confirmDelete") {
      commitDelete();
    }
  }

  /* ---------------------------------------------------------------------------
     KEYBOARD
     --------------------------------------------------------------------------- */
  function onDemoKey(e) {
    if (e.metaKey && !e.ctrlKey) return;
    const key = e.key;
    const ctrl = e.ctrlKey;
    const mod = ctrl || e.metaKey;

    if (st.mode === "list") {
      if (mod) {
        const k = key.toLowerCase();
        const actions = {
          a: beginAdd,
          e: beginEdit,
          r: beginRename,
          d: beginDelete,
          s: toggleShow,
        };
        if (actions[k]) {
          e.preventDefault();
          e.stopPropagation();
          actions[k]();
        }
        return;
      }
      if (key === "ArrowDown") {
        e.preventDefault();
        move(1);
        return;
      }
      if (key === "ArrowUp") {
        e.preventDefault();
        move(-1);
        return;
      }
      if (key === "Enter") {
        e.preventDefault();
        runSelected();
        return;
      }
      if (key === "Escape") {
        e.preventDefault();
        if (st.filter) {
          st.filter = "";
          st.sel = 0;
          render();
        } else {
          ui.screen.blur();
        }
        return;
      }
      if (key === "Backspace") {
        e.preventDefault();
        if (st.filter) st.filter = st.filter.slice(0, -1);
        st.sel = 0;
        render();
        return;
      }
      if (key.length === 1 && !mod && !e.altKey) {
        e.preventDefault();
        st.filter += key;
        st.sel = 0;
        render();
      }
      return;
    }

    if (st.mode === "confirmSave" || st.mode === "confirmRename" || st.mode === "confirmDelete") {
      if (key === "y" || key === "Y") {
        e.preventDefault();
        answerConfirm(true);
        return;
      }
      if (key === "n" || key === "N" || key === "Enter") {
        e.preventDefault();
        answerConfirm(false);
        return;
      }
      if (key === "Escape") {
        e.preventDefault();
        answerConfirm(false);
      }
      return;
    }

    // text prompts: addName / addCmd / edit / rename / ph
    if (key === "Enter") {
      e.preventDefault();
      submitPrompt();
      return;
    }
    if (key === "Escape") {
      e.preventDefault();
      cancel();
      return;
    }
    if (key === "Backspace") {
      e.preventDefault();
      st.buf = st.buf.slice(0, -1);
      st.err = "";
      render();
      return;
    }
    if (key.length === 1 && !mod && !e.altKey) {
      e.preventDefault();
      st.buf += key;
      st.err = "";
      render();
    }
  }

  /* ---------------------------------------------------------------------------
     RENDER
     --------------------------------------------------------------------------- */
  const caret = () =>
    '<span class="t-caret" aria-hidden="true" style="vertical-align:-0.14em"></span>';

  function render() {
    if (!ui.root) return;

    // invocation + filter
    ui.invoke.textContent = st.filter;
    ui.filter.textContent = st.filter;
    if (ui.invokeCaret) ui.invokeCaret.hidden = true;
    if (ui.filterCaret) ui.filterCaret.hidden = st.mode !== "list" || !st.focused;

    // list
    const list = visible();
    ui.list.innerHTML = "";
    ui.list.classList.toggle("show-cmds", st.show);
    list.forEach(({ c }, idx) => {
      const li = document.createElement("li");
      li.className = "term__item" + (idx === st.sel ? " is-sel" : "");
      li.setAttribute("role", "option");
      li.setAttribute("aria-selected", idx === st.sel ? "true" : "false");
      li.dataset.idx = String(idx);

      const name = document.createElement("span");
      name.className = "row-name";
      name.textContent = c.name;
      li.appendChild(name);

      const hint = hintFor(c.cmd);
      if (hint) {
        const hintEl = document.createElement("span");
        hintEl.className = "row-hint";
        hintEl.textContent = " " + hint;
        li.appendChild(hintEl);
      }

      const cmdEl = document.createElement("span");
      cmdEl.className = "row-cmd";
      cmdEl.textContent = c.cmd;
      li.appendChild(cmdEl);

      ui.list.appendChild(li);
    });
    ui.empty.hidden = list.length > 0;

    const selEl = ui.list.querySelector(".is-sel");
    if (selEl) selEl.scrollIntoView({ block: "nearest" });

    // ask box
    ui.ask.hidden = st.mode === "list";
    ui.askQ.innerHTML = "";
    ui.askIn.innerHTML = "";
    if (st.mode !== "list") {
      let question = "";
      let label = "";
      let hint = "enter continue · esc cancel";
      let showInput = true;

      switch (st.mode) {
        case "addName":
          question = 'Add command — <span class="q-name">name</span>';
          label = "name";
          hint = "spaces allowed · enter continue · esc cancel";
          break;
        case "addCmd":
          question = `Add <span class="q-name">${escapeHtml(st.addName)}</span> — shell command`;
          label = "command";
          hint = "enter save · esc cancel";
          break;
        case "edit":
          question = `Edit <span class="q-name">${escapeHtml(st.cmds[st.target].name)}</span>`;
          label = "command";
          break;
        case "rename":
          question = `Rename <span class="q-name">${escapeHtml(st.cmds[st.target].name)}</span>`;
          label = "name";
          break;
        case "confirmSave":
          question = `Save &quot;${escapeHtml(st.cmds[st.target].name)}&quot; → <span class="q-name">${escapeHtml(st.pending)}</span> ? [y/N]`;
          showInput = false;
          hint = "y save · n / esc discard";
          break;
        case "confirmRename":
          question = `Rename &quot;${escapeHtml(st.cmds[st.target].name)}&quot; → <span class="q-name">${escapeHtml(st.pending)}</span> ? [y/N]`;
          showInput = false;
          hint = "y rename · n / esc discard";
          break;
        case "confirmDelete":
          question = `Delete <span class="q-name">${escapeHtml(st.cmds[st.target].name)}</span> <span class="t-dim">(${escapeHtml(
            st.cmds[st.target].cmd
          )})</span> ? [y/N]`;
          showInput = false;
          hint = "y delete · N / esc keep";
          break;
        case "ph": {
          const item = st.ph.items[st.ph.i];
          question = `<span class="t-dim">${escapeHtml(st.ph.entry.c.cmd)}</span>`;
          label = item.name;
          hint =
            (item.def === null ? "required" : `default: ${escapeHtml(item.def)}`) +
            ` · ${st.ph.i + 1}/${st.ph.items.length} · enter ${
              st.ph.i + 1 >= st.ph.items.length ? "run" : "continue"
            } · esc cancel`;
          break;
        }
      }

      ui.askQ.innerHTML = question;
      if (showInput) {
        ui.askIn.dataset.label = label;
        ui.askIn.textContent = st.buf;
        ui.askIn.insertAdjacentHTML("beforeend", caret());
      } else {
        delete ui.askIn.dataset.label;
        ui.ask.hidden = false;
      }
      ui.askIn.classList.toggle("is-err", Boolean(st.err));
      ui.askHint.innerHTML = st.err
        ? `<span class="err">${escapeHtml(st.err)}</span>`
        : escapeHtml(hint);
    } else {
      delete ui.askIn.dataset.label;
    }

    // output
    ui.out.innerHTML = "";
    st.out.forEach((line) => {
      const el = document.createElement("span");
      if (line.kind === "exec") {
        el.className = "out-line";
        el.innerHTML =
          '<span class="out-sig">></span> <span class="out-sig">cl-exec:</span> ' +
          `<span class="out-name">${escapeHtml(line.name)}</span>` +
          (line.vals && line.vals.length
            ? ` <span class="out-vals">(${line.vals.map(escapeHtml).join(", ")})</span>`
            : "");
      } else if (line.kind === "err") {
        el.className = "out-line out-err";
        el.textContent = line.text;
      } else if (line.kind === "info") {
        el.className = "out-line out-vals";
        el.textContent = "· " + line.text;
      } else {
        el.className = "out-line out-res";
        el.textContent = line.text;
      }
      ui.out.appendChild(el);
    });
    ui.out.scrollTop = ui.out.scrollHeight;

    // footer status
    if (ui.status) {
      const count = list.length;
      ui.status.textContent =
        (st.mode === "list" ? `${count} of ${st.cmds.length} commands` : MODE_STATUS[st.mode]) +
        (st.show ? " · commands shown" : "") +
        (st.focused ? " · focused" : "");
    }
  }

  function escapeHtml(value) {
    return String(value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  /* ---------------------------------------------------------------------------
     WIRING — terminal
     --------------------------------------------------------------------------- */
  if (ui.screen) {
    ui.screen.addEventListener("keydown", onDemoKey);
    ui.screen.addEventListener("focus", () => {
      st.focused = true;
      ui.screen.classList.add("is-focused");
      render();
    });
    ui.screen.addEventListener("blur", () => {
      st.focused = false;
      ui.screen.classList.remove("is-focused");
      render();
    });
    ui.root.addEventListener("pointerdown", (e) => {
      if (e.target.closest("button")) return;
      if (document.activeElement !== ui.screen) {
        // keep the click from blurring the just-focused element
        e.preventDefault();
        ui.screen.focus();
      }
    });
    ui.list.addEventListener("click", (e) => {
      const li = e.target.closest(".term__item");
      if (!li) return;
      st.sel = Number(li.dataset.idx);
      ui.screen.focus();
      render();
    });
    ui.list.addEventListener("dblclick", () => runSelected());

    $$("[data-touch]", ui.root).forEach((btn) => {
      btn.addEventListener("click", () => {
        ui.screen.focus();
        const t = btn.dataset.touch;
        if (t === "up") move(-1);
        else if (t === "down") move(1);
        else if (t === "enter") runSelected();
        else if (t === "escape") cancel();
        else if (t === "ctrl+s") toggleShow();
        else if (t === "ctrl+a") beginAdd();
      });
    });
  }

  function syncShowToggle() {
    const box = $("#showToggle");
    const label = $("#viewState");
    const view = $("#viewList");
    if (box) box.checked = st.show;
    if (label) label.textContent = st.show ? "names + commands" : "names only";
    if (view) view.setAttribute("data-show", st.show ? "true" : "false");
  }

  render();
  syncShowToggle();

  /* ---------------------------------------------------------------------------
     PLACEHOLDER LAB
     --------------------------------------------------------------------------- */
  const TPLS = {
    ssh: {
      name: "ssh server",
      cmd: "ssh {{user}}@{{host:prod.example.com}}",
      out: "admin@prod.example.com's password: · Connected to prod.example.com",
    },
    git: {
      name: "git push",
      cmd: "git push {{remote:origin}} {{branch:main}}",
      out: "To github.com:spltek/cl.git · 4b0c2f1..9de117a  main -> main",
    },
    psql: {
      name: "psql main",
      cmd: "psql -h {{host:localhost}} -p {{port:5432}} -U {{user}} -d {{db:postgres}}",
      out: 'psql (16.2) · Type "help" for help. · postgres=#',
    },
    docker: {
      name: "docker node",
      cmd: "docker run -p {{port:3000}}:3000 {{image:node:18}}",
      out: "Unable to find image 'node:18' locally · pulling from library/node",
    },
  };

  const lab = {
    fields: $("#phFields"),
    resolved: $("#phResolved"),
    name: $("#phName"),
    annName: $("#phAnnounceName"),
    annVals: $("#phAnnounceVals"),
    out: $("#phOut"),
  };

  const labState = { key: "ssh", vals: {} };

  function labValues() {
    const tpl = TPLS[labState.key];
    const items = readPlaceholders(tpl.cmd);
    const vals = {};
    items.forEach((p) => {
      const v = labState.vals[p.name];
      vals[p.name] = v !== undefined && v !== "" ? v : p.def === null ? "" : p.def;
    });
    return { tpl, items, vals };
  }

  function paintResolved() {
    const { tpl, items, vals } = labValues();
    if (!lab.resolved) return;

    lab.resolved.textContent = "";
    let cursor = 0;
    const src = tpl.cmd;
    const re = /\{\{(\w+)(?::([^}]*))?\}\}/g;
    let m;
    while ((m = re.exec(src))) {
      if (m.index > cursor) {
        lab.resolved.appendChild(document.createTextNode(src.slice(cursor, m.index)));
      }
      const name = m[1];
      const value = vals[name];
      const span = document.createElement("span");
      if (value) {
        span.className = "ph-val";
        span.textContent = value;
      } else {
        span.className = "ph-empty";
        span.textContent = `{{${name}}}`;
      }
      lab.resolved.appendChild(span);
      cursor = m.index + m[0].length;
    }
    if (cursor < src.length) lab.resolved.appendChild(document.createTextNode(src.slice(cursor)));

    const announceVals = items.map((p) => (vals[p.name] ? vals[p.name] : "?")).join(", ");
    if (lab.annName) lab.annName.textContent = "> cl-exec: " + tpl.name;
    if (lab.annVals) lab.annVals.textContent = announceVals ? `(${announceVals})` : "";
    if (lab.out) lab.out.textContent = resolveCommand(tpl.out, vals);
  }

  function paintFields() {
    if (!lab.fields) return;
    const { tpl, items, vals } = labValues();
    if (lab.name) lab.name.textContent = tpl.name;
    lab.fields.textContent = "";
    items.forEach((p) => {
      const row = document.createElement("div");
      row.className = "ph-field";

      const label = document.createElement("label");
      const id = "ph-" + p.name;
      label.htmlFor = id;
      label.textContent = p.name;

      const input = document.createElement("input");
      input.type = "text";
      input.id = id;
      input.value = vals[p.name] || "";
      input.placeholder = p.def === null ? "required" : "default: " + p.def;
      input.autocomplete = "off";
      input.spellcheck = false;
      input.addEventListener("input", () => {
        labState.vals[p.name] = input.value;
        paintResolved();
      });

      row.append(label, input);
      lab.fields.appendChild(row);
    });
    paintResolved();
  }

  $$("[data-tpl]").forEach((btn) => {
    btn.addEventListener("click", () => {
      $$("[data-tpl]").forEach((b) => {
        b.classList.toggle("is-active", b === btn);
        b.setAttribute("aria-selected", b === btn ? "true" : "false");
      });
      labState.key = btn.dataset.tpl;
      labState.vals = {};
      paintFields();
    });
  });
  paintFields();

  /* ---------------------------------------------------------------------------
     STORAGE TABS
     --------------------------------------------------------------------------- */
  const OS = {
    mac: {
      path: "~/Library/Application Support/cl",
      note: "The release installer drops the binary in <code>~/.local/bin</code> — no sudo, no system-wide changes.",
    },
    linux: {
      path: "~/.local/share/cl",
      note: "The XDG data directory. Homebrew and the curl installer both land the binary in <code>~/.local/bin</code>.",
    },
    win: {
      path: "%AppData%\\cl",
      note: "Installed into <code>%LOCALAPPDATA%\\cl\\bin</code>; the installer adds it to your user PATH.",
    },
  };

  $$("[data-os]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const conf = OS[btn.dataset.os];
      if (!conf) return;
      $$("[data-os]").forEach((b) => {
        b.classList.toggle("is-active", b === btn);
        b.setAttribute("aria-selected", b === btn ? "true" : "false");
      });
      const pathEl = $("#osPath");
      const noteEl = $("#osNote");
      if (pathEl) pathEl.textContent = conf.path;
      if (noteEl) noteEl.innerHTML = conf.note;
      const copyBtn = $(".path-box [data-copy]");
      if (copyBtn) copyBtn.dataset.copy = conf.path;
    });
  });

  /* ---------------------------------------------------------------------------
     INSTALL TABS
     --------------------------------------------------------------------------- */
  const INST = {
    brew: {
      cmd: "brew install spltek/tap/cl",
      desc: "The fastest path on macOS and Linux. Updates arrive through <code>brew upgrade</code>.",
      req: "Homebrew on macOS or Linux",
    },
    curl: {
      cmd: "curl -fsSL https://raw.githubusercontent.com/spltek/cl/main/install.sh | sh",
      desc: "Downloads the right binary for your OS/arch into <code>~/.local/bin</code>. Override the destination with <code>CL_INSTALL_DIR</code>.",
      req: "curl · macOS or Linux",
    },
    win: {
      cmd: "iwr https://raw.githubusercontent.com/spltek/cl/main/install.ps1 -UseBasicParsing | iex",
      desc: "Installs into <code>%LOCALAPPDATA%\\cl\\bin</code> and appends it to your user PATH. No elevation required.",
      req: "PowerShell · RemoteSigned policy",
    },
    src: {
      cmd: "make install-local",
      desc: "Builds the binary, installs it to <code>~/.local/bin</code> and prints the one command that activates it in your current shell.",
      req: "Go 1.25+ and make",
    },
  };

  function selectInstall(key) {
    const conf = INST[key];
    if (!conf) return;
    $$("[data-inst]").forEach((b) => {
      b.classList.toggle("is-active", b.dataset.inst === key);
      b.setAttribute("aria-selected", b.dataset.inst === key ? "true" : "false");
    });
    const cmdEl = $("#instCmd");
    const descEl = $("#instDesc");
    const reqEl = $("#instReq");
    if (cmdEl) cmdEl.textContent = conf.cmd;
    if (descEl) descEl.innerHTML = conf.desc;
    if (reqEl) reqEl.textContent = conf.req;
    const copyBtn = $("#instPanel [data-copy]");
    if (copyBtn) copyBtn.dataset.copy = conf.cmd;
  }

  $$("[data-inst]").forEach((btn) => {
    btn.addEventListener("click", () => selectInstall(btn.dataset.inst));
  });

  /* ---------------------------------------------------------------------------
     COPY TO CLIPBOARD
     --------------------------------------------------------------------------- */
  async function copyText(text) {
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        return true;
      }
    } catch (err) {
      /* fall through to execCommand */
    }
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.cssText = "position:fixed;top:-1000px;left:-1000px;opacity:0";
    document.body.appendChild(ta);
    ta.select();
    let ok = false;
    try {
      ok = document.execCommand("copy");
    } catch (err) {
      ok = false;
    }
    ta.remove();
    return ok;
  }

  document.addEventListener("click", async (e) => {
    const btn = e.target.closest("[data-copy]");
    if (!btn) return;
    const text = btn.dataset.copy || "";
    if (!text) return;
    const ok = await copyText(text);
    if (!ok) return;
    if (!btn.dataset.originalLabel) {
      btn.dataset.originalLabel = btn.getAttribute("aria-label") || "Copy";
    }
    btn.classList.add("is-copied");
    btn.setAttribute("aria-label", "Copied");
    window.setTimeout(() => {
      btn.classList.remove("is-copied");
      btn.setAttribute("aria-label", btn.dataset.originalLabel || "Copy");
    }, 1400);
  });

  /* ---------------------------------------------------------------------------
     SHOW-COMMANDS SYNC + GLOBAL CTRL+S
     --------------------------------------------------------------------------- */
  const showBox = $("#showToggle");
  if (showBox) {
    showBox.addEventListener("change", () => {
      st.show = showBox.checked;
      render();
      syncShowToggle();
    });
  }

  let pickerVisible = false;
  const pickerSection = $("#picker");
  if (pickerSection && "IntersectionObserver" in window) {
    const io = new IntersectionObserver(
      (entries) => {
        pickerVisible = entries.some((en) => en.isIntersecting);
      },
      { threshold: 0.2 }
    );
    io.observe(pickerSection);
  }

  document.addEventListener("keydown", (e) => {
    if (!(e.ctrlKey || e.metaKey) || e.altKey) return;
    if (e.key.toLowerCase() !== "s") return;
    const tag = (e.target.tagName || "").toLowerCase();
    if (tag === "input" || tag === "textarea" || e.target.isContentEditable) return;
    if (!st.focused && !pickerVisible) return;
    e.preventDefault();
    toggleShow();
  });

  /* ---------------------------------------------------------------------------
     NAV, PROGRESS RAIL, REVEAL, ATMOSPHERE
     --------------------------------------------------------------------------- */
  const nav = $("#nav");
  const navProgress = $("#navProgress");
  const rail = $("#rail");
  const burger = $("#burger");
  const navLinks = $("#navLinks");

  function onScroll() {
    const y = window.scrollY || window.pageYOffset;
    if (nav) nav.classList.toggle("is-stuck", y > 14);
    const max = document.documentElement.scrollHeight - window.innerHeight;
    const pct = max > 0 ? Math.min(1, Math.max(0, y / max)) : 0;
    if (navProgress) navProgress.style.width = pct * 100 + "%";
    if (rail) rail.style.height = pct * 100 + "%";
  }
  window.addEventListener("scroll", onScroll, { passive: true });
  window.addEventListener("resize", onScroll, { passive: true });
  onScroll();

  if (burger && nav) {
    burger.addEventListener("click", () => {
      const open = nav.classList.toggle("is-open");
      burger.setAttribute("aria-expanded", open ? "true" : "false");
    });
    if (navLinks) {
      navLinks.addEventListener("click", (e) => {
        if (!e.target.closest("a")) return;
        nav.classList.remove("is-open");
        burger.setAttribute("aria-expanded", "false");
      });
    }
    document.addEventListener("keydown", (e) => {
      if (e.key === "Escape" && nav.classList.contains("is-open")) {
        nav.classList.remove("is-open");
        burger.setAttribute("aria-expanded", "false");
      }
    });
    window.addEventListener("resize", () => {
      if (window.innerWidth > 1010 && nav.classList.contains("is-open")) {
        nav.classList.remove("is-open");
        burger.setAttribute("aria-expanded", "false");
      }
    });
  }

  const sectionLinks = $$(".nav__links a")
    .map((a) => {
      const href = a.getAttribute("href");
      const el = href && href.startsWith("#") ? $(href) : null;
      return el ? { a, el } : null;
    })
    .filter(Boolean);

  if (sectionLinks.length && "IntersectionObserver" in window) {
    const navIO = new IntersectionObserver(
      (entries) => {
        entries.forEach((en) => {
          if (!en.isIntersecting) return;
          const match = sectionLinks.find((s) => s.el === en.target);
          if (!match) return;
          sectionLinks.forEach((s) => s.a.classList.toggle("is-active", s === match));
        });
      },
      { rootMargin: "-45% 0px -45% 0px" }
    );
    sectionLinks.forEach((s) => navIO.observe(s.el));
  }

  const revealEls = $$("[data-reveal]");
  if (!reduceMotion && "IntersectionObserver" in window) {
    const revIO = new IntersectionObserver(
      (entries) => {
        entries.forEach((en) => {
          if (!en.isIntersecting) return;
          en.target.classList.add("is-visible");
          revIO.unobserve(en.target);
        });
      },
      { threshold: 0.1, rootMargin: "0px 0px -40px 0px" }
    );
    revealEls.forEach((el, i) => {
      el.style.transitionDelay = (i % 3) * 70 + "ms";
      revIO.observe(el);
    });
  } else {
    revealEls.forEach((el) => el.classList.add("is-visible"));
  }

  if (!reduceMotion) {
    $$(".card").forEach((card) => {
      card.addEventListener("mousemove", (e) => {
        const r = card.getBoundingClientRect();
        card.style.setProperty("--mx", ((e.clientX - r.left) / r.width) * 100 + "%");
        card.style.setProperty("--my", ((e.clientY - r.top) / r.height) * 100 + "%");
      });
    });

    const spot = $("#spot");
    if (spot) {
      let sx = 0;
      let sy = 0;
      let cx = 0;
      let cy = 0;
      window.addEventListener(
        "mousemove",
        (e) => {
          sx = (e.clientX / window.innerWidth - 0.5) * 90;
          sy = (e.clientY / window.innerHeight - 0.5) * 60;
        },
        { passive: true }
      );
      const tick = () => {
        cx += (sx - cx) * 0.06;
        cy += (sy - cy) * 0.06;
        spot.style.transform = `translate(calc(-50% + ${cx.toFixed(2)}px), calc(-50% + ${cy.toFixed(2)}px))`;
        requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    }
  }

  // marquee: clone the track until it spans twice the viewport
  $$("[data-marquee]").forEach((track) => {
    const original = track.innerHTML;
    let guard = 0;
    while (track.scrollWidth < window.innerWidth * 2 && guard < 4) {
      track.innerHTML += original;
      guard += 1;
    }
  });

  function paintClock() {
    const now = new Date();
    const pad = (n) => String(n).padStart(2, "0");
    const txt = `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`;
    $$("[data-clock]").forEach((el) => {
      el.textContent = txt;
    });
  }
  paintClock();
  window.setInterval(paintClock, 1000);

  const yearEl = $("#year");
  if (yearEl) yearEl.textContent = String(new Date().getFullYear());

  // demo film: swap the poster facade for the YouTube iframe on demand, so the
  // page makes no third-party requests — and loads no player — until asked.
  const filmStage = $("[data-film-stage]");
  const filmPlay = $("[data-film-play]");
  if (filmStage && filmPlay) {
    const VIDEO_ID = "Oqw5H6Nlw00";
    filmPlay.addEventListener(
      "click",
      () => {
        const iframe = document.createElement("iframe");
        iframe.className = "film__iframe";
        iframe.src =
          "https://www.youtube-nocookie.com/embed/" +
          VIDEO_ID +
          "?autoplay=1&rel=0&modestbranding=1&playsinline=1&color=white";
        iframe.title = "cl — command launcher demo (4:04)";
        iframe.allow =
          "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share; fullscreen";
        iframe.referrerPolicy = "strict-origin-when-cross-origin";
        iframe.setAttribute("frameborder", "0");
        iframe.setAttribute("allowfullscreen", "");
        filmStage.textContent = "";
        filmStage.appendChild(iframe);
        filmStage.classList.add("is-live");
        iframe.focus();
      },
      { once: true }
    );
  }

  /* __PART2__ */
})();
