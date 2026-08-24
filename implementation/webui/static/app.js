/* Coding Agent Web UI - single-page chat frontend (vanilla, dependency-free). */
(function () {
  "use strict";

  var SESSION_KEY = "codingAgentSession";
  var THEME_KEY = "codingAgentTheme";
  var session = localStorage.getItem(SESSION_KEY) || "";

  // DOM refs
  var form = document.getElementById("chat-form");
  var input = document.getElementById("prompt-input");
  var sendBtn = document.getElementById("send-btn");
  var cancelBtn = document.getElementById("cancel-btn");
  var newSessionBtn = document.getElementById("new-session-btn");
  var clearBtn = document.getElementById("clear-btn");
  var clearHistoryBtn = document.getElementById("clear-history-btn");
  var compressBtn = document.getElementById("compress-btn");
  var dumpBtn = document.getElementById("dump-btn");
  var readonlyToggle = document.getElementById("readonly-toggle");
  var themeSelect = document.getElementById("theme-select");
  var output = document.getElementById("output");
  var statusBar = document.getElementById("status-bar");
  var statsEl = document.getElementById("stats");
  var historyEl = document.getElementById("history-list");
  var goalForm = document.getElementById("goal-form");
  var goalInput = document.getElementById("goal-input");
  var goalOffBtn = document.getElementById("goal-off-btn");
  var goalActive = document.getElementById("goal-active");
  var ringFg = document.getElementById("ring-fg");
  var ringText = document.getElementById("ring-text");
  var ring = document.getElementById("context-ring");
  var connDot = document.getElementById("conn-dot");

  var RING_C = 2 * Math.PI * 15.9; // circle circumference
  ringFg.style.strokeDasharray = RING_C;
  ringFg.style.strokeDashoffset = RING_C;

  var state = { running: false, readOnly: false, contextSize: 0, maxContext: 1 };
  var history = [];
  var histIndex = -1;
  var streamBuf = null; // current streaming assistant message node

  var COMMANDS = [
    "/stats", "/clear", "/clear-history", "/read-only", "/compress",
    "/goal ", "/goal-off", "/dump"
  ];

  function $(id) { return document.getElementById(id); }
  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text !== undefined) e.textContent = text;
    return e;
  }

  // ---- Rendering helpers ----

  function scrollBottom() { output.scrollTop = output.scrollHeight; }

  function appendMessage(role, text) {
    var div = el("div", "msg msg-" + role);
    div.appendChild(el("div", "msg-head", role === "user" ? "You" : role === "error" ? "Error" : "Assistant"));
    div.appendChild(el("div", "msg-body", text));
    output.appendChild(div);
    scrollBottom();
    return div;
  }

  function toolCard(name, args) {
    var div = el("div", "msg msg-tool");
    div.dataset.tool = "1";
    var head = el("div", "msg-head");
    head.appendChild(el("span", "spinner", ""));
    head.appendChild(document.createTextNode("[Tool Call] " + name));
    var body = el("div", "msg-body msg-body-mono tool-params", args || "");
    div.appendChild(head);
    div.appendChild(body);
    output.appendChild(div);
    scrollBottom();
    return div;
  }

  function appendToolResult(text) {
    var div = el("div", "msg msg-tool-result");
    div.appendChild(el("div", "msg-head", "Tool Result"));
    var body = el("div", "msg-body msg-body-mono", text || "");
    div.appendChild(body);
    var copy = el("button", "copy-btn", "Copy");
    copy.addEventListener("click", function () {
      navigator.clipboard && navigator.clipboard.writeText(text || "");
      copy.textContent = "Copied!";
      setTimeout(function () { copy.textContent = "Copy"; }, 1500);
    });
    div.appendChild(copy);
    output.appendChild(div);
    scrollBottom();
  }

  function reasoningBlock() {
    var details = el("details", "msg msg-reasoning");
    details.dataset.reasoning = "1";
    var sum = el("summary", "msg-head", "Reasoning");
    var body = el("div", "msg-body msg-body-mono");
    details.appendChild(sum);
    details.appendChild(body);
    output.appendChild(details);
    scrollBottom();
    return { details: details, body: body };
  }

  // ---- State & status ----

  function setStatus(text) { statusBar.textContent = text || ""; }

  function setConn(ok) {
    connDot.className = "conn-dot " + (ok ? "conn-on" : "conn-off");
    connDot.title = ok ? "connected" : "reconnecting";
  }

  function ctxWarningLevel(frac) {
    // Mirror the TUI: green (ok), yellow (warn), orange, red.
    if (frac < 0.6) return 0;
    if (frac < 0.8) return 1;
    if (frac < 0.95) return 2;
    return 3;
  }

  function renderContext() {
    var frac = state.maxContextSize > 0 ? state.contextSize / state.maxContextSize : 0;
    if (frac > 1) frac = 1;
    var lvl = ctxWarningLevel(frac);
    var color = ["#a6e3a1", "#f9e2af", "#fab387", "#f38ba8"][lvl];
    var marks = ["✓", "⚠", "⚠⚠", "⚠⚠⚠"][lvl];
    var off = RING_C * (1 - frac);
    ringFg.style.strokeDashoffset = off;
    ringFg.style.stroke = color;
    ring.className = "context-ring lvl-" + lvl;
    ring.title = state.contextSize + " / " + state.maxContextSize + " tokens (" + marks + ")";
    ringText.textContent = Math.round(frac * 100) + "%";
  }

  function renderState(st) {
    if (!st) return;
    state = st;
    renderContext();
    setStatus(
      "Model: " + (st.model || "?") +
      "  ·  Context: " + st.contextSize + " / " + st.maxContextSize + " tokens" +
      (st.readOnly ? "  ·  [Read-only]" : "") +
      (st.goalActive ? "  ·  Goal: " + st.goal : "")
    );
    readonlyToggle.checked = !!st.readOnly;
    if (st.goalActive) goalActive.textContent = st.goal;
    else goalActive.textContent = "";
    sendBtn.disabled = st.running;
    cancelBtn.disabled = !st.running;
  }

  function renderStats(stats) {
    if (!stats) return;
    var parts = [
      "In: " + stats.inputTokens,
      "Out: " + stats.outputTokens,
      "Tok/s: " + (stats.tokensPerSecond ? stats.tokensPerSecond.toFixed(1) : "0"),
      "Tools: " + stats.toolCalls + (stats.failedToolCalls ? " (" + stats.failedToolCalls + " failed)" : ""),
      "Iters: " + stats.iterations
    ];
    if (stats.compressionCount) parts.push("Cmp: " + stats.compressionCount);
    if (stats.startTime) parts.push("Up: " + uptime(stats.startTime));
    statsEl.textContent = parts.join("  ·  ");
  }

  function uptime(start) {
    var s = Math.floor((Date.now() / 1000) - start);
    var h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), sec = s % 60;
    return (h ? h + "h" : "") + m + "m" + sec + "s";
  }

  // ---- History ----

  function renderHistory() {
    historyEl.innerHTML = "";
    if (!history.length) {
      historyEl.appendChild(el("li", "history-empty", "No commands yet."));
      return;
    }
    history.forEach(function (item) {
      var li = el("li", "history-item", item);
      li.addEventListener("click", function () {
        input.value = item;
        input.focus();
      });
      historyEl.appendChild(li);
    });
  }

  function loadHistory() {
    history = [];
    try { history = JSON.parse(localStorage.getItem("codingAgentHistory") || "[]"); }
    catch (e) { history = []; }
    renderHistory();
  }

  function saveHistory() {
    try { localStorage.setItem("codingAgentHistory", JSON.stringify(history.slice(0, 50))); }
    catch (e) {}
  }

  function pushHistory(prompt) {
    if (!prompt) return;
    history = history.filter(function (h) { return h !== prompt; });
    history.unshift(prompt);
    saveHistory();
    renderHistory();
  }

  function navHistory(dir) {
    if (!history.length) return;
    histIndex += dir;
    if (histIndex < 0) { histIndex = -1; input.value = ""; return; }
    if (histIndex >= history.length) histIndex = history.length - 1;
    input.value = history[histIndex];
  }

  // ---- Slash autocomplete ----

  function autocomplete() {
    var val = input.value;
    var at = val.lastIndexOf(" ");
    var token = (at === -1 ? val : val.slice(at + 1));
    if (token[0] !== "/") { hideAutocomplete(); return; }
    var matches = COMMANDS.filter(function (c) { return c.indexOf(token) === 0; });
    if (!matches.length) { hideAutocomplete(); return; }
    input.value = val.slice(0, at + 1) + matches[0];
  }

  function hideAutocomplete() {}

  // ---- API ----

  function post(path, body) {
    return fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }).then(function (res) { return res.json(); });
  }

  function sendPrompt(prompt) {
    pushHistory(prompt);
    appendMessage("user", prompt);
    input.value = "";
    histIndex = -1;
    post("/api/chat", { session: session, prompt: prompt }).then(function (res) {
      if (res && res.error) { setStatus(res.error); appendMessage("error", res.error); }
      if (res && res.output !== undefined) {
        showCommandOutput(res.output);
        renderState(res.state);
      }
    }).catch(function (err) {
      appendMessage("error", "Request failed: " + err.message);
    });
  }

  function showCommandOutput(text) {
    var div = el("div", "msg msg-command");
    div.appendChild(el("div", "msg-body msg-body-mono", text || ""));
    output.appendChild(div);
    scrollBottom();
  }

  function sendCommand(cmd) {
    post("/api/command", { session: session, command: cmd }).then(function (res) {
      if (res && res.output !== undefined) showCommandOutput(res.output);
      if (res && res.state) renderState(res.state);
      if (res && res.error) appendMessage("error", res.error);
    }).catch(function (err) { appendMessage("error", err.message); });
  }

  // ---- SSE ----

  function handleChunk(c) {
    if (c.isToolCall) {
      // Tool call header from streaming prefix "[Tool Call] name"
      var text = c.text.replace(/^\[Tool Call\]\s*/, "").trim();
      if (text) {
        streamBuf = null;
        var card = toolCard(text, "");
        streamBuf = card;
      }
      return;
    }
    if (c.contentType === 1) { // reasoning
      var rb = output.querySelector(".msg-reasoning[data-reasoning='1']");
      if (!rb) { var r = reasoningBlock(); rb = r.details; rb._body = r.body; }
      rb._body.textContent += c.text;
      scrollBottom();
      return;
    }
    if (c.contentType === 2 || c.contentType === 3) { // goal / compression
      if (!streamBuf || !streamBuf.classList.contains("msg-goal")) {
        streamBuf = el("div", "msg msg-goal");
        streamBuf.appendChild(el("div", "msg-head", c.contentType === 2 ? "Goal" : "Compression"));
        streamBuf.appendChild(el("div", "msg-body"));
        output.appendChild(streamBuf);
      }
      streamBuf.lastElementChild.textContent += c.text;
      scrollBottom();
      return;
    }
    // normal content
    if (c.text === "\n") {
      if (streamBuf && streamBuf.classList.contains("msg-assistant")) {
        streamBuf.lastElementChild.textContent += "\n";
        scrollBottom();
      }
      return;
    }
    if (!streamBuf || !streamBuf.classList.contains("msg-assistant")) {
      streamBuf = el("div", "msg msg-assistant");
      streamBuf.appendChild(el("div", "msg-head", "Assistant"));
      streamBuf.appendChild(el("div", "msg-body"));
      output.appendChild(streamBuf);
    }
    streamBuf.lastElementChild.textContent += c.text;
    scrollBottom();
  }

  function connectSSE() {
    var es = new EventSource("/api/events?session=" + encodeURIComponent(session));
    setConn(false);
    es.addEventListener("open", function () { setConn(true); });
    es.addEventListener("state", function (ev) { renderState(JSON.parse(ev.data)); });
    es.addEventListener("chunk", function (ev) { handleChunk(JSON.parse(ev.data)); });
    es.addEventListener("result", function (ev) {
      var r = JSON.parse(ev.data);
      renderStats(r.stats);
    });
    es.addEventListener("stats", function (ev) { renderStats(JSON.parse(ev.data)); });
    es.addEventListener("error", function (ev) {
      var m = JSON.parse(ev.data || "{}").message || "An error occurred.";
      appendMessage("error", m);
    });
    es.addEventListener("done", function (ev) {
      var d = JSON.parse(ev.data);
      streamBuf = null;
      if (!d.ok) appendMessage("error", "Run finished with errors.");
    });
    es.onerror = function () { setConn(false); };
  }

  // ---- Init & events ----

  function init() {
    loadHistory();
    var savedTheme = localStorage.getItem(THEME_KEY) || "";
    themeSelect.value = savedTheme;
    applyTheme(savedTheme);

    form.addEventListener("submit", function (e) {
      e.preventDefault();
      var prompt = input.value.trim();
      if (!prompt || state.running) return;
      sendPrompt(prompt);
    });

    input.addEventListener("keydown", function (e) {
      if (e.key === "ArrowUp") { e.preventDefault(); navHistory(-1); }
      else if (e.key === "ArrowDown") { e.preventDefault(); navHistory(1); }
      else if (e.key === "Tab") {
        if (input.value.indexOf("/") !== -1) { e.preventDefault(); autocomplete(); }
      }
    });

    cancelBtn.addEventListener("click", function () {
      post("/api/cancel", { session: session }).then(function () { setStatus("Cancelling…"); });
    });

    newSessionBtn.addEventListener("click", function () {
      session = "s" + Date.now();
      localStorage.setItem(SESSION_KEY, session);
      output.innerHTML = "";
      streamBuf = null;
      fetch("/api/state?session=" + encodeURIComponent(session)).then(function (r) { return r.json(); })
        .then(renderState);
    });

    clearBtn.addEventListener("click", function () {
      output.innerHTML = "";
      streamBuf = null;
    });
    clearHistoryBtn.addEventListener("click", function () {
      sendCommand("/clear-history");
    });
    compressBtn.addEventListener("click", function () { sendCommand("/compress"); });
    dumpBtn.addEventListener("click", function () { sendCommand("/dump"); });

    readonlyToggle.addEventListener("change", function () {
      sendCommand(readonlyToggle.checked ? "/read-only" : "/read-only");
    });

    themeSelect.addEventListener("change", function () {
      localStorage.setItem(THEME_KEY, themeSelect.value);
      applyTheme(themeSelect.value);
    });

    goalForm.addEventListener("submit", function (e) {
      e.preventDefault();
      var g = goalInput.value.trim();
      if (g) sendCommand("/goal " + g);
      goalInput.value = "";
    });
    goalOffBtn.addEventListener("click", function () { sendCommand("/goal-off"); });

    fetch("/api/state?session=" + encodeURIComponent(session)).then(function (res) { return res.json(); })
      .then(function (st) { renderState(st); connectSSE(); })
      .catch(function () { setStatus("Failed to load state."); connectSSE(); });
  }

  function applyTheme(theme) {
    var palettes = {
      dark: { "--bg":"#1e1e2e","--bg-2":"#181825","--fg":"#cdd6f4","--muted":"#6c7086","--accent":"#89b4fa","--accent-2":"#cba6f7","--user-bg":"#313244","--assistant-bg":"#242438","--tool-bg":"#2a2a3a","--reasoning-bg":"#2e2a1f","--border":"#3b3b4f","--error":"#f38ba8","--success":"#a6e3a1" },
      light: { "--bg":"#eff1f5","--bg-2":"#e6e9ef","--fg":"#4c4f69","--muted":"#8c8fa1","--accent":"#1e66f5","--accent-2":"#8839ef","--user-bg":"#dce0e8","--assistant-bg":"#e6e9ef","--tool-bg":"#ccd0da","--reasoning-bg":"#f2e8d0","--border":"#ccd0da","--error":"#d20f39","--success":"#40a02b" },
      solarized: { "--bg":"#002b36","--bg-2":"#073642","--fg":"#eee8d5","--muted":"#93a1a1","--accent":"#268bd2","--accent-2":"#6c71c4","--user-bg":"#073642","--assistant-bg":"#073642","--tool-bg":"#0a3a46","--reasoning-bg":"#3a2a1a","--border":"#586e75","--error":"#dc322f","--success":"#859900" },
      gruvbox: { "--bg":"#282828","--bg-2":"#3c3836","--fg":"#ebdbb2","--muted":"#928374","--accent":"#83a598","--accent-2":"#d3869b","--user-bg":"#3c3836","--assistant-bg":"#32302f","--tool-bg":"#3c3836","--reasoning-bg":"#3f2d20","--border":"#504945","--error":"#fb4934","--success":"#b8bb26" },
      darkula: { "--bg":"#2b2b2b","--bg-2":"#3c3c3c","--fg":"#a9b7c6","--muted":"#808080","--accent":"#6897bb","--accent-2":"#cc7833","--user-bg":"#3c3c3c","--assistant-bg":"#323232","--tool-bg":"#3c3c3c","--reasoning-bg":"#3a3325","--border":"#555555","--error":"#bc3f3f","--success":"#a5c261" }
    };
    var root = document.documentElement.style;
    if (theme && palettes[theme]) {
      var p = palettes[theme];
      Object.keys(p).forEach(function (k) { root.setProperty(k, p[k]); });
    } else {
      root.removeProperty("--bg"); root.removeProperty("--bg-2"); root.removeProperty("--fg");
      root.removeProperty("--muted"); root.removeProperty("--accent"); root.removeProperty("--accent-2");
      root.removeProperty("--user-bg"); root.removeProperty("--assistant-bg"); root.removeProperty("--tool-bg");
      root.removeProperty("--reasoning-bg"); root.removeProperty("--border"); root.removeProperty("--error");
      root.removeProperty("--success");
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
