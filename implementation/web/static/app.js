// Coding Agent web UI — vanilla JS, no external dependencies.
(function () {
  "use strict";

  const log = document.getElementById("log");
  const input = document.getElementById("input");
  const send = document.getElementById("send");
  const cancelBtn = document.getElementById("cancel");
  const ctxEl = document.getElementById("ctx");
  const goalEl = document.getElementById("goal");
  const statsBody = document.getElementById("stats-body");
  const themeSel = document.getElementById("theme");

  themeSel.value = document.body.dataset.theme || "dark";
  themeSel.addEventListener("change", () => {
    location.search = "?theme=" + encodeURIComponent(themeSel.value);
  });

  // ---- session --------------------------------------------------------
  let session = localStorage.getItem("ca.session") || "";

  function saveSession(id) {
    if (id) { session = id; localStorage.setItem("ca.session", id); }
  }

  // ---- history (persisted across refresh) ------------------------------
  let history = JSON.parse(localStorage.getItem("ca.history") || "[]");
  let histIdx = -1;

  function saveHistory() {
    localStorage.setItem("ca.history", JSON.stringify(history.slice(-200)));
  }

  function pushHistory(p) {
    if (!p) return;
    if (history[history.length - 1] !== p) history.push(p);
    saveHistory();
  }

  // ---- rendering -------------------------------------------------------
  function el(cls, text) {
    const d = document.createElement("div");
    d.className = cls;
    if (text !== undefined) d.textContent = text;
    log.appendChild(d);
    log.scrollTop = log.scrollHeight;
    return d;
  }

  let streamEl = null;   // current streaming content element
  let reasoningEl = null;
  let toolCard = null;   // current live tool card

  function closeStream() { streamEl = null; reasoningEl = null; }
  function closeToolCard() { toolCard = null; }

  function appendInto(node, text) {
    node.textContent += text;
    log.scrollTop = log.scrollHeight;
  }

  function renderEvent(ev) {
    switch (ev.type) {
      case "content":
        reasoningEl = null;
        if (!streamEl) streamEl = el("msg content");
        appendInto(streamEl, ev.text || "");
        break;
      case "reasoning":
        streamEl = null;
        if (!reasoningEl) reasoningEl = el("msg reasoning");
        appendInto(reasoningEl, ev.text || "");
        break;
      case "tool_args":
        closeStream();
        if (!toolCard) {
          toolCard = el("card");
          toolCard.innerHTML = '<div class="tool-name"></div><div class="tool-args"></div>';
        }
        toolCard.querySelector(".tool-name").textContent = "[Tool Call] " + (ev.tool || "");
        toolCard.querySelector(".tool-args").textContent = ev.args || "";
        break;
      case "tool_start":
        closeStream();
        if (!toolCard) {
          toolCard = el("card");
          toolCard.innerHTML = '<div class="tool-name"></div><div class="tool-args"></div>';
          toolCard.querySelector(".tool-name").textContent = "[Tool Call] " + (ev.tool || "");
          toolCard.querySelector(".tool-args").textContent = ev.args || "";
        }
        break;
      case "tool_result": {
        closeToolCardStart();
        if (!toolCard) {
          toolCard = el("card");
          toolCard.innerHTML = '<div class="tool-name"></div>';
          toolCard.querySelector(".tool-name").textContent = "[Tool] " + (ev.tool || "");
        }
        const r = document.createElement("div");
        r.className = "tool-result " + (ev.failed ? "fail" : "ok");
        r.textContent = (ev.failed ? "Tool '" + ev.tool + "' failed:" : "Tool '" + ev.tool + "' executed successfully:") +
          (ev.result ? "\n" + ev.result : "");
        toolCard.appendChild(r);
        toolCard.classList.add(ev.failed ? "fail" : "ok");
        closeToolCard();
        break;
      }
      case "error":
        closeStream(); closeToolCard();
        el("msg error", ev.text || "error");
        break;
      case "goal":
      case "compressed":
        closeStream(); closeToolCard();
        el("msg goal", ev.text || "");
        break;
      case "subagent":
        closeStream(); closeToolCard();
        el("msg info", "[Subagent] " + (ev.text || ""));
        break;
      case "state":
        applyState(ev.state);
        break;
      case "done":
        closeStream(); closeToolCard();
        setRunning(false);
        refreshState();
        break;
      case "iteration":
        closeStream(); closeToolCard();
        break;
    }
  }

  function closeToolCardStart() { /* tool card may still be open from args */ }

  // ---- state -----------------------------------------------------------
  function applyState(st) {
    if (!st) return;
    const tok = st.context_tokens, lim = st.context_limit, pct = parseFloat(st.context_percent);
    ctxEl.textContent = "Tokens: " + tok + " / " + lim + " (" + pct.toFixed(1) + "%)";
    ctxEl.className = "ctx " + (pct > 90 ? "critical" : pct > 75 ? "high" : pct >= 50 ? "warn" : "ok");
    if (st.goal) { goalEl.textContent = "Goal: " + st.goal; goalEl.classList.remove("hidden"); }
    else { goalEl.classList.add("hidden"); }
    const s = st.stats || {};
    statsBody.innerHTML = "";
    const rows = [
      ["Input tokens", s.input_tokens],
      ["Output tokens", s.output_tokens],
      ["Tokens/sec", s.tokens_per_second],
      ["Tool calls", s.tool_calls],
      ["Failed calls", s.failed_tool_calls],
      ["Iterations", s.iterations],
      ["Compressions", s.compressions],
      ["Uptime", s.uptime],
    ];
    for (const [k, v] of rows) {
      const dt = document.createElement("dt"); dt.textContent = k;
      const dd = document.createElement("dd"); dd.textContent = v === undefined ? "-" : v;
      statsBody.appendChild(dt); statsBody.appendChild(dd);
    }
    if (st.running) setRunning(true);
  }

  function setRunning(running) {
    cancelBtn.classList.toggle("hidden", !running);
    send.disabled = running;
    input.disabled = false;
  }

  function refreshState() {
    if (!session) return;
    fetch("/api/state?session=" + encodeURIComponent(session))
      .then(r => r.json())
      .then(j => { saveSession(j.session); applyState(j.state); })
      .catch(() => {});
  }
  setInterval(refreshState, 4000);

  // ---- SSE ---------------------------------------------------------------
  let es = null;
  function connectSSE() {
    if (es) es.close();
    es = new EventSource("/api/events?session=" + encodeURIComponent(session));
    es.onmessage = (m) => {
      try { renderEvent(JSON.parse(m.data)); } catch (e) { /* ignore */ }
    };
    es.onerror = () => { /* EventSource reconnects automatically */ };
  }

  // ---- actions -----------------------------------------------------------
  function submit() {
    const text = input.value.trim();
    if (!text) return;
    input.value = "";
    histIdx = -1;
    pushHistory(text);
    closeStream(); closeToolCard();

    if (text.startsWith("/")) {
      fetch("/api/command", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ session, command: text }),
      })
        .then(r => r.json())
        .then(j => {
          saveSession(j.session);
          if (j.output === "cleared") { log.innerHTML = ""; }
          else if (j.output) el("msg system", j.output);
          applyState(j.state);
        })
        .catch(e => el("msg error", "command failed: " + e));
      return;
    }

    el("msg user", text);
    setRunning(true);
    fetch("/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ session, prompt: text }),
    })
      .then(r => r.json())
      .then(j => saveSession(j.session))
      .catch(e => { el("msg error", "request failed: " + e); setRunning(false); });
  }

  send.addEventListener("click", submit);
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") { submit(); return; }
    if (e.key === "ArrowUp") {
      if (history.length === 0) return;
      histIdx = histIdx < 0 ? history.length - 1 : Math.max(0, histIdx - 1);
      input.value = history[histIdx];
      e.preventDefault();
    } else if (e.key === "ArrowDown") {
      if (histIdx < 0) return;
      histIdx = Math.min(history.length - 1, histIdx + 1);
      if (histIdx === history.length - 1 && input.value === history[histIdx]) {
        // keep
      }
      if (histIdx >= history.length - 1) { histIdx = -1; input.value = ""; }
      else input.value = history[histIdx];
      e.preventDefault();
    }
  });

  cancelBtn.addEventListener("click", () => {
    fetch("/api/cancel", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ session }),
    }).catch(() => {});
  });

  // ---- boot ---------------------------------------------------------------
  function boot() {
    fetch("/api/state?session=" + encodeURIComponent(session))
      .then(r => r.json())
      .then(j => {
        saveSession(j.session);
        applyState(j.state);
        return fetch("/api/history?session=" + encodeURIComponent(session));
      })
      .then(r => r.json())
      .then(j => {
        if (j.history && j.history.length) {
          localStorage.setItem("ca.history", JSON.stringify(j.history));
          history = j.history;
        }
      })
      .catch(() => {})
      .finally(() => { connectSSE(); refreshState(); input.focus(); });
  }
  boot();
})();
