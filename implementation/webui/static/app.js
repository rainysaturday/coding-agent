/* Coding Agent Web UI - single-page chat frontend. */
(function () {
  "use strict";

  var SESSION_KEY = "codingAgentSession";
  var session = localStorage.getItem(SESSION_KEY) || "";

  // DOM refs
  var form = document.getElementById("chat-form");
  var input = document.getElementById("prompt-input");
  var sendBtn = document.getElementById("send-btn");
  var cancelBtn = document.getElementById("cancel-btn");
  var output = document.getElementById("output");
  var statusBar = document.getElementById("status-bar");
  var statsEl = document.getElementById("stats");
  var historyEl = document.getElementById("history-list");

  var state = {
    running: false,
    readOnly: false,
    contextSize: 0,
    maxContext: 0,
    historyCount: 0,
  };

  var history = [];

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text !== undefined) e.textContent = text;
    return e;
  }

  function appendMessage(role, text) {
    var div = el("div", "msg msg-" + role);
    var head = el("div", "msg-head", role === "user" ? "You" : "Assistant");
    var body = el("div", "msg-body", text);
    div.appendChild(head);
    div.appendChild(body);
    output.appendChild(div);
    output.scrollTop = output.scrollHeight;
    return body;
  }

  function appendToolCall(name, args) {
    var div = el("div", "msg msg-tool");
    var head = el("div", "msg-head", "[Tool Call] " + name);
    var body = el("div", "msg-body msg-body-mono", args || "");
    div.appendChild(head);
    div.appendChild(body);
    output.appendChild(div);
    output.scrollTop = output.scrollHeight;
  }

  function appendToolResult(text) {
    var div = el("div", "msg msg-tool-result");
    var head = el("div", "msg-head", "Tool Result");
    var body = el("div", "msg-body msg-body-mono", text || "");
    div.appendChild(head);
    div.appendChild(body);
    output.appendChild(div);
    output.scrollTop = output.scrollHeight;
  }

  function setStatus(text) {
    statusBar.textContent = text || "";
  }

  function renderState(st) {
    if (!st) return;
    state = st;
    var ctx = st.contextSize + " / " + st.maxContext + " tokens";
    setStatus(
      "Model: " + (st.model || "?") +
      "  |  Context: " + ctx +
      (st.readOnly ? "  |  [Read-only]" : "") +
      (st.goalActive ? "  |  Goal: " + st.goal : "")
    );
    sendBtn.disabled = st.running;
    cancelBtn.disabled = !st.running;
  }

  function renderStats(stats) {
    if (!stats) return;
    var parts = [
      "Input: " + stats.inputTokens,
      "Output: " + stats.outputTokens,
      "Tok/s: " + (stats.tokensPerSecond ? stats.tokensPerSecond.toFixed(1) : "0"),
      "Tools: " + stats.toolCalls + (stats.failedToolCalls ? " (" + stats.failedToolCalls + " failed)" : ""),
      "Iters: " + stats.iterations
    ];
    if (stats.compressionCount) parts.push("Compressions: " + stats.compressionCount);
    statsEl.textContent = parts.join("  ·  ");
  }

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
    try {
      history = JSON.parse(localStorage.getItem("codingAgentHistory") || "[]");
    } catch (e) { history = []; }
    renderHistory();
  }

  function saveHistory() {
    try {
      localStorage.setItem("codingAgentHistory", JSON.stringify(history.slice(0, 50)));
    } catch (e) {}
  }

  function pushHistory(prompt) {
    if (!prompt) return;
    history = history.filter(function (h) { return h !== prompt; });
    history.unshift(prompt);
    saveHistory();
    renderHistory();
  }

  function showCommandOutput(text) {
    var div = el("div", "msg msg-command");
    var body = el("div", "msg-body msg-body-mono", text || "");
    div.appendChild(body);
    output.appendChild(div);
    output.scrollTop = output.scrollHeight;
  }

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
    post("/api/chat", { session: session, prompt: prompt }).then(function (res) {
      if (res && res.error) {
        setStatus(res.error);
        appendMessage("error", res.error);
      }
      if (res && res.output !== undefined) {
        // Command result
        showCommandOutput(res.output);
        renderState(res.state);
      }
    }).catch(function (err) {
      appendMessage("error", "Request failed: " + err.message);
    });
  }

  function connectSSE() {
    var es = new EventSource("/api/events?session=" + encodeURIComponent(session));
    es.addEventListener("state", function (ev) {
      renderState(JSON.parse(ev.data));
    });
    es.addEventListener("chunk", function (ev) {
      var c = JSON.parse(ev.data);
      if (c.isToolCall) {
        appendToolCall(c.text.replace(/^\[Tool Call\]\s*/, ""), "");
        return;
      }
      if (c.contentType === 1) { // reasoning
        var r = output.lastElementChild;
        if (!r || r.dataset.reasoning !== "1") {
          r = el("div", "msg msg-reasoning");
          r.dataset.reasoning = "1";
          var head = el("div", "msg-head", "Reasoning");
          r.appendChild(head);
          r.dataset.body = "";
          r.appendChild(el("div", "msg-body"));
          output.appendChild(r);
        }
        r.lastElementChild.textContent = r.dataset.body + c.text;
        r.dataset.body = r.dataset.body + c.text;
        output.scrollTop = output.scrollHeight;
        return;
      }
      var last = output.lastElementChild;
      if (c.text === "\n") {
        if (last && last.dataset.assistant === "1") {
          last.lastElementChild.textContent += "\n";
          last.lastElementChild.scrollTop = last.lastElementChild.scrollHeight;
        }
        return;
      }
      if (!last || last.dataset.assistant !== "1") {
        last = el("div", "msg msg-assistant");
        last.dataset.assistant = "1";
        last.appendChild(el("div", "msg-head", "Assistant"));
        last.appendChild(el("div", "msg-body"));
        output.appendChild(last);
      }
      last.lastElementChild.textContent += c.text;
      output.scrollTop = output.scrollHeight;
    });
    es.addEventListener("result", function (ev) {
      var r = JSON.parse(ev.data);
      renderStats(r.stats);
    });
    es.addEventListener("stats", function (ev) {
      renderStats(JSON.parse(ev.data));
    });
    es.addEventListener("error", function (ev) {
      appendMessage("error", (JSON.parse(ev.data || "{}").message) || "An error occurred.");
    });
    es.addEventListener("done", function (ev) {
      var d = JSON.parse(ev.data);
      if (!d.ok) appendMessage("error", "Run finished with errors.");
    });
    es.onerror = function () {
      // EventSource reconnects automatically; update status only.
      setStatus("Reconnecting…");
    };
  }

  function init() {
    loadHistory();
    form.addEventListener("submit", function (e) {
      e.preventDefault();
      var prompt = input.value.trim();
      if (!prompt || state.running) return;
      sendPrompt(prompt);
    });

    cancelBtn.addEventListener("click", function () {
      post("/api/cancel", { session: session }).then(function () {
        setStatus("Cancelling…");
      });
    });

    // Fetch initial state, then connect SSE.
    fetch("/api/state?session=" + encodeURIComponent(session)).then(function (res) {
      return res.json();
    }).then(function (st) {
      renderState(st);
      connectSSE();
    }).catch(function () {
      setStatus("Failed to load state.");
      connectSSE();
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
