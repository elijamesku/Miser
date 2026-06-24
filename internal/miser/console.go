package miser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strings"
)

type ConsoleConfig struct {
	Provider    string
	AccountID   string
	Integration string
	LogPath     string
	CachePath   string
}

func RenderConsoleHTML(config ConsoleConfig) string {
	var b strings.Builder
	fmt.Fprintln(&b, "<!doctype html>")
	fmt.Fprintln(&b, `<html lang="en">`)
	fmt.Fprintln(&b, `<head>`)
	fmt.Fprintln(&b, `<meta charset="utf-8">`)
	fmt.Fprintln(&b, `<meta name="viewport" content="width=device-width, initial-scale=1">`)
	fmt.Fprintln(&b, `<title>Miser Console</title>`)
	fmt.Fprintln(&b, `<style>`)
	fmt.Fprintln(&b, consoleCSS())
	fmt.Fprintln(&b, `</style>`)
	fmt.Fprintln(&b, `</head>`)
	fmt.Fprintln(&b, `<body>`)
	fmt.Fprintln(&b, `<div class="app">`)
	fmt.Fprintln(&b, `<aside class="rail">`)
	fmt.Fprintln(&b, `<div class="brand"><span>M</span><strong>Miser</strong></div>`)
	fmt.Fprintln(&b, `<nav>`)
	fmt.Fprintln(&b, `<a class="active" href="/">Console</a>`)
	fmt.Fprintln(&b, `<a href="/miser/api/requests">Requests</a>`)
	fmt.Fprintln(&b, `<a href="/healthz">Health</a>`)
	fmt.Fprintln(&b, `</nav>`)
	fmt.Fprintln(&b, `<div class="rail-footer">`)
	fmt.Fprintf(&b, `<small>Provider</small><strong>%s</strong>`, html.EscapeString(defaultString(config.Provider, "openai")))
	fmt.Fprintln(&b, `</div>`)
	fmt.Fprintln(&b, `</aside>`)

	fmt.Fprintln(&b, `<main>`)
	fmt.Fprintln(&b, `<header>`)
	fmt.Fprintln(&b, `<div>`)
	fmt.Fprintln(&b, `<p class="eyebrow">Live AI spend control</p>`)
	fmt.Fprintln(&b, `<h1>Miser Console</h1>`)
	fmt.Fprintf(&b, `<p class="muted">%s%s</p>`, html.EscapeString(labelIfSet("Account", config.AccountID)), html.EscapeString(labelIfSet("Integration", config.Integration)))
	fmt.Fprintln(&b, `</div>`)
	fmt.Fprintln(&b, `<div class="status"><span></span>Proxy live</div>`)
	fmt.Fprintln(&b, `</header>`)

	fmt.Fprintln(&b, `<section class="metrics">`)
	consoleMetric(&b, "Intercepted", "0", "requests", "requestsCount")
	consoleMetric(&b, "Saved", "$0.00", "exact cache", "savedAmount")
	consoleMetric(&b, "Cache", "0.0%", "hit rate", "cacheRate")
	consoleMetric(&b, "Spend", "$0.00", "after Miser", "spendAmount")
	fmt.Fprintln(&b, `</section>`)

	fmt.Fprintln(&b, `<section class="workspace">`)
	fmt.Fprintln(&b, `<section class="chat">`)
	fmt.Fprintln(&b, `<div class="panel-head"><div><h2>Playground</h2><p>Test a request through the Miser proxy.</p></div><select id="model"><option>gpt-4o-mini</option><option>gpt-4o</option><option>gpt-4.1-mini</option></select></div>`)
	fmt.Fprintln(&b, `<div id="messages" class="messages"><div class="bubble assistant">Send a message and watch Miser explain the decision beside it.</div></div>`)
	fmt.Fprintln(&b, `<form id="chatForm" class="composer"><textarea id="prompt" rows="3" placeholder="Ask something, classify a ticket, summarize a note..."></textarea><button type="submit">Send</button></form>`)
	fmt.Fprintln(&b, `</section>`)

	fmt.Fprintln(&b, `<aside class="trace">`)
	fmt.Fprintln(&b, `<div class="panel-head"><div><h2>Decision Trace</h2><p>What Miser did with the latest request.</p></div><button id="refresh" type="button">Refresh</button></div>`)
	fmt.Fprintln(&b, `<div class="decision" id="decision">`)
	fmt.Fprintln(&b, `<div><small>Action</small><strong>Waiting for traffic</strong></div>`)
	fmt.Fprintln(&b, `<div><small>Reason</small><p>No intercepted request yet.</p></div>`)
	fmt.Fprintln(&b, `</div>`)
	fmt.Fprintln(&b, `<h3>Recent Requests</h3>`)
	fmt.Fprintln(&b, `<div id="requests" class="request-list"></div>`)
	fmt.Fprintln(&b, `</aside>`)
	fmt.Fprintln(&b, `</section>`)

	fmt.Fprintln(&b, `<section class="inspector">`)
	fmt.Fprintln(&b, `<div class="panel-head"><div><h2>Request Inspector</h2><p>Original request, Miser decision, final route, and savings.</p></div><code>/v1/chat/completions</code></div>`)
	fmt.Fprintln(&b, `<pre id="inspector">{ "state": "waiting_for_request" }</pre>`)
	fmt.Fprintln(&b, `</section>`)
	fmt.Fprintln(&b, `</main>`)
	fmt.Fprintln(&b, `</div>`)
	fmt.Fprintln(&b, `<script>`)
	fmt.Fprintln(&b, consoleJS())
	fmt.Fprintln(&b, `</script>`)
	fmt.Fprintln(&b, `</body></html>`)
	return b.String()
}

func consoleMetric(b *strings.Builder, label, value, caption, id string) {
	fmt.Fprintf(b, `<article><span>%s</span><strong id="%s">%s</strong><small>%s</small></article>`, html.EscapeString(label), html.EscapeString(id), html.EscapeString(value), html.EscapeString(caption))
}

func loadProxyLogRows(path string, limit int) ([]map[string]interface{}, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var rows []map[string]interface{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row map[string]interface{}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func consoleCSS() string {
	return `
:root {
  color-scheme: dark;
  --bg: #0d1117;
  --rail: #111820;
  --panel: #151b23;
  --panel-2: #0f151d;
  --line: #2b3440;
  --text: #e6edf3;
  --muted: #8b949e;
  --soft: #202a36;
  --accent: #4ea1ff;
  --good: #3fb950;
  --warn: #d29922;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font: 14px/1.45 ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
.app {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 236px 1fr;
}
.rail {
  background: var(--rail);
  border-right: 1px solid var(--line);
  padding: 18px 14px;
  display: flex;
  flex-direction: column;
  gap: 22px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 6px;
}
.brand span {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 7px;
  background: var(--accent);
  color: #06111f;
  font-weight: 800;
}
.brand strong { font-size: 16px; }
nav { display: grid; gap: 4px; }
nav a {
  color: var(--muted);
  text-decoration: none;
  padding: 9px 10px;
  border-radius: 7px;
}
nav a.active, nav a:hover {
  background: var(--soft);
  color: var(--text);
}
.rail-footer {
  margin-top: auto;
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 12px;
  background: var(--panel-2);
}
small, .muted, .panel-head p, .decision p { color: var(--muted); }
.rail-footer small, .rail-footer strong { display: block; }
main {
  min-width: 0;
  padding: 22px;
}
header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 16px;
}
.eyebrow {
  margin: 0 0 5px;
  color: var(--accent);
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}
h1, h2, h3, p { margin-top: 0; }
h1 { margin-bottom: 4px; font-size: 28px; }
h2 { margin-bottom: 2px; font-size: 16px; }
h3 { margin: 18px 0 10px; font-size: 13px; color: var(--muted); text-transform: uppercase; }
.status {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: var(--panel);
  white-space: nowrap;
}
.status span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--good);
}
.metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 10px;
}
.metrics article, .chat, .trace, .inspector {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
}
.metrics article { padding: 14px; }
.metrics span, .metrics small { display: block; color: var(--muted); }
.metrics strong { display: block; margin: 6px 0 2px; font-size: 24px; }
.workspace {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(360px, .9fr);
  gap: 10px;
  align-items: stretch;
}
.chat, .trace, .inspector {
  min-width: 0;
  overflow: hidden;
}
.panel-head {
  min-height: 66px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  padding: 14px;
  border-bottom: 1px solid var(--line);
}
select, button, textarea {
  border: 1px solid var(--line);
  background: var(--panel-2);
  color: var(--text);
  border-radius: 7px;
  font: inherit;
}
select, button { height: 34px; padding: 0 10px; }
button {
  background: var(--accent);
  border-color: var(--accent);
  color: #06111f;
  font-weight: 700;
  cursor: pointer;
}
.messages {
  height: 270px;
  overflow: auto;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.bubble {
  max-width: 82%;
  padding: 10px 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--soft);
  white-space: pre-wrap;
}
.bubble.user {
  margin-left: auto;
  background: #1d2a38;
}
.composer {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
  padding: 14px;
  border-top: 1px solid var(--line);
}
textarea {
  width: 100%;
  resize: vertical;
  min-height: 58px;
  padding: 10px;
}
.decision {
  display: grid;
  gap: 10px;
  padding: 14px;
  border-bottom: 1px solid var(--line);
}
.decision div {
  background: var(--panel-2);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 10px;
}
.decision strong { display: block; margin-top: 4px; }
.request-list {
  max-height: 255px;
  overflow: auto;
  padding: 0 14px 14px;
}
.request {
  border: 1px solid var(--line);
  background: var(--panel-2);
  border-radius: 8px;
  padding: 10px;
  margin-bottom: 8px;
  cursor: pointer;
}
.request:hover { border-color: var(--accent); }
.request-top {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}
.pill {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 3px 8px;
  background: var(--soft);
  color: var(--muted);
  font-size: 12px;
}
.pill.hit { color: var(--good); }
.pill.miss { color: var(--warn); }
.inspector { margin-top: 10px; }
pre {
  margin: 0;
  padding: 14px;
  max-height: 320px;
  overflow: auto;
  background: var(--panel-2);
  color: #c9d1d9;
}
code { color: var(--muted); }
@media (max-width: 980px) {
  .app { grid-template-columns: 1fr; }
  .rail { display: none; }
  .metrics, .workspace { grid-template-columns: 1fr; }
  main { padding: 14px; }
  header { display: block; }
}
`
}

func consoleJS() string {
	return `
const state = { requests: [] };
const $ = (id) => document.getElementById(id);

function money(value) {
  return '$' + Number(value || 0).toFixed(4);
}

function pct(value) {
  return (Number(value || 0) * 100).toFixed(1) + '%';
}

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
}

function decisionFor(row) {
  if (!row) return { action: 'Waiting for traffic', reason: 'No intercepted request yet.', saved: 0 };
  const hit = row.cache_status === 'hit';
  const cacheable = row.miser_cache_eligible === true;
  const before = hit ? Number(row.cache_saved_usd || 0) : Number(row.cost_usd || 0);
  const saved = hit ? before : 0;
  if (hit) {
    return {
      action: 'Exact cache hit',
      reason: 'Miser matched the request fingerprint and returned the cached provider response before spending again.',
      saved
    };
  }
  if (cacheable) {
    return {
      action: 'Pass through + cache write',
      reason: 'First time seeing this request. Miser sent it upstream, logged token cost, and stored the response for an identical repeat.',
      saved: 0
    };
  }
  return {
    action: 'Pass through',
    reason: 'This request was not cache eligible, usually because it was streaming or not a supported chat/responses endpoint.',
    saved: 0
  };
}

async function loadRequests() {
  const res = await fetch('/miser/api/requests', { cache: 'no-store' });
  state.requests = await res.json();
  renderRequests();
}

function renderRequests() {
  const rows = state.requests || [];
  const spend = rows.reduce((sum, row) => sum + Number(row.cost_usd || 0), 0);
  const hits = rows.filter(row => row.cache_status === 'hit').length;
  const requests = rows.length;
  const saved = rows.filter(row => row.cache_status === 'hit').reduce((sum, row) => sum + Number(row.cache_saved_usd || 0), 0);
  $('requestsCount').textContent = String(requests);
  $('spendAmount').textContent = money(spend);
  $('cacheRate').textContent = requests ? pct(hits / requests) : '0.0%';
  $('savedAmount').textContent = money(saved);

  const latest = rows[0];
  renderDecision(latest);
  if (latest) renderInspector(latest);

  $('requests').innerHTML = rows.slice(0, 30).map((row, index) => {
    const decision = decisionFor(row);
    const cacheClass = row.cache_status === 'hit' ? 'hit' : 'miss';
    return '<article class="request" data-index="' + index + '">' +
      '<div class="request-top"><strong>' + escapeHTML(row.model || 'unknown') + '</strong><span class="pill ' + cacheClass + '">' + escapeHTML(row.cache_status || 'pass') + '</span></div>' +
      '<div><span class="pill">' + escapeHTML(row.http_path || row.workflow || 'request') + '</span> <span class="pill">' + money(row.cost_usd) + '</span></div>' +
      '<small>' + escapeHTML(decision.action) + ' - ' + escapeHTML(row.request_fingerprint || '') + '</small>' +
    '</article>';
  }).join('') || '<p class="muted">No intercepted requests yet.</p>';

  document.querySelectorAll('.request').forEach(node => {
    node.addEventListener('click', () => {
      const row = state.requests[Number(node.dataset.index)];
      renderDecision(row);
      renderInspector(row);
    });
  });
}

function renderDecision(row) {
  const d = decisionFor(row);
  $('decision').innerHTML =
    '<div><small>Action</small><strong>' + escapeHTML(d.action) + '</strong></div>' +
    '<div><small>Reason</small><p>' + escapeHTML(d.reason) + '</p></div>' +
    '<div><small>Route</small><strong>' + escapeHTML(row?.provider || 'provider') + ' / ' + escapeHTML(row?.model || 'model') + '</strong></div>' +
    '<div><small>Saved</small><strong>' + money(d.saved) + '</strong></div>';
}

function renderInspector(row) {
  const d = decisionFor(row);
  const payload = {
    original_request: {
      path: row.http_path,
      model: row.model,
      prompt_fingerprint: row.request_fingerprint,
      prompt_chars: row.prompt_chars
    },
    miser_decision: {
      action: d.action,
      reason: d.reason,
      cache_status: row.cache_status,
      cache_eligible: row.miser_cache_eligible
    },
    final_provider_request: {
      provider: row.provider,
      model: row.model,
      status: row.http_status
    },
    result: {
      cost_after_miser: Number(row.cost_usd || 0),
      estimated_saved: d.saved,
      input_tokens: row.input_tokens,
      output_tokens: row.output_tokens,
      latency_ms: row.latency_ms,
      cost_basis: row.cost_basis
    }
  };
  $('inspector').textContent = JSON.stringify(payload, null, 2);
}

function appendMessage(role, text) {
  const div = document.createElement('div');
  div.className = 'bubble ' + role;
  div.textContent = text;
  $('messages').appendChild(div);
  $('messages').scrollTop = $('messages').scrollHeight;
}

function assistantText(payload) {
  const choice = payload?.choices?.[0];
  return choice?.message?.content || choice?.text || payload?.output_text || JSON.stringify(payload, null, 2);
}

$('chatForm').addEventListener('submit', async (event) => {
  event.preventDefault();
  const prompt = $('prompt').value.trim();
  if (!prompt) return;
  $('prompt').value = '';
  appendMessage('user', prompt);
  appendMessage('assistant', 'Thinking through Miser...');
  const bubbles = document.querySelectorAll('.bubble.assistant');
  const pending = bubbles[bubbles.length - 1];
  try {
    const res = await fetch('/v1/chat/completions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: $('model').value,
        messages: [{ role: 'user', content: prompt }]
      })
    });
    const payload = await res.json();
    pending.textContent = assistantText(payload);
    await loadRequests();
  } catch (err) {
    pending.textContent = 'Request failed: ' + err.message;
  }
});

$('refresh').addEventListener('click', loadRequests);
loadRequests();
setInterval(loadRequests, 3000);
`
}
