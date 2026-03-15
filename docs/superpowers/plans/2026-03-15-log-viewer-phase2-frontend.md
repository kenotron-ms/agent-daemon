# Log Viewer — Phase 2: Frontend Implementation Plan

> **Execution:** Use the subagent-driven-development workflow to implement this plan.

**Goal:** Replace the basic run card with a full inline log viewer — expanded panels, copy button, live streaming badge, and SSE subscription.  
**Architecture:** `renderRunCard` produces three card variants (success, failed/timeout, running). `loadRuns()` does incremental per-card updates instead of blowing away the full list. `openLiveLog` attaches an `EventSource` to the SSE endpoint per running card; `finalizeRunCard` transitions a running card to its final state when the SSE `done` event fires.  
**Tech Stack:** Vanilla JS (no framework, no test harness), CSS custom properties, `EventSource` API

---

## Prerequisites

Phase 1 (backend) must be complete and the `GET /api/runs/{id}/stream` SSE endpoint must be working. Verify with:

```bash
# Start the daemon, then in another terminal:
PORT=61017 bash scripts/test-sse.sh
# Expected: Results: 10 passed, 0 failed
```

Do not start Phase 2 until Phase 1 is green.

---

## Codebase orientation

Read these two files before making any changes:

```
web/app.js     — existing renderRunCard, loadRuns, renderRuns, toggleRunLog, esc(), timeAgo(), durationMs()
web/style.css  — existing .run-card rule, CSS custom properties (--green, --red, --blue, etc.)
```

Key facts:
- Container element ID for the runs list: **`runs-list`** (confirmed in `clearActivity()`)
- `esc(str)` — HTML-escapes a string; already defined, use it for all output
- `timeAgo(isoStr)` — takes an ISO timestamp **string**, calls `new Date()` internally
- `durationMs(start, end)` — takes two **Date objects**, not strings
- API returns camelCase: `run.startedAt`, `run.endedAt`, `run.jobName`, `run.jobId`
- SSE `event: done` payload uses snake_case: `started_at`, `ended_at` (set by the backend)
- Current `.run-card` CSS uses `display: flex; align-items: flex-start; gap: 10px;` — this must change to `display: block` to accommodate the new vertical card structure
- No JS test framework exists — all verification is manual browser inspection

---

## Task 1: CSS additions — log panel, live badge, updated run card layout

**Files:**
- Modify: `web/style.css`

---

### Step 1: Update the existing `.run-card` rule

The current `.run-card` rule in `style.css` looks like:

```css
.run-card {
  display: flex; align-items: flex-start; gap: 10px;
  background: var(--bg2); border: 1px solid var(--border);
  border-radius: var(--radius); padding: 10px 12px; margin-bottom: 4px;
}
```

Replace only the `display: flex; align-items: flex-start; gap: 10px;` part with `display: block;`. The rest stays the same. The updated rule must be:

```css
.run-card {
  display: block;
  background: var(--bg2); border: 1px solid var(--border);
  border-radius: var(--radius); padding: 0; margin-bottom: 4px;
  overflow: hidden;
}
```

> **Why `padding: 0` here?** Padding is now handled by `.run-header` so the log panel can extend edge-to-edge.

### Step 2: Append all new CSS rules

At the very end of `web/style.css`, after the `.onboarding-notice a` rule, append this block:

```css

/* ── Run card v2 ──────────────────────────────────────────────────────────── */

.run-header {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 12px;
}

.run-name   { font-weight: 600; font-size: 13px; flex: 1; min-width: 0;
              overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.run-time   { font-size: 11px; color: var(--text2); white-space: nowrap; flex-shrink: 0; }

.log-toggle {
  font-size: 11px; color: var(--accent); text-decoration: none;
  flex-shrink: 0; white-space: nowrap;
}
.log-toggle:hover { text-decoration: underline; }

/* Live card — blue accent border while running */
.run-card.live { border-color: #1a3a5c; box-shadow: 0 0 0 1px rgba(33,150,243,0.2); }

/* Failed / timeout card — red left border */
.run-card.run-failed { border-left: 3px solid var(--red); }

/* ── Log panel ───────────────────────────────────────────────────────────── */

.log-panel              { border-top: 1px solid #222; }
.log-panel.hidden       { display: none; }

.log-toolbar {
  display: flex; justify-content: space-between; align-items: center;
  padding: 4px 12px; background: #111827;
}

.log-label    { color: #555; font-size: 11px; font-family: monospace; }
.log-copy-btn {
  background: none; border: none; color: #4a9eff;
  font-size: 11px; cursor: pointer; padding: 0;
}
.log-copy-btn:hover { text-decoration: underline; }

.log-output {
  margin: 0; padding: 12px;
  background: #0d1117; color: #c9d1d9;
  font-size: 11px; font-family: 'SF Mono', 'Fira Code', monospace; line-height: 1.5;
  max-height: 240px; overflow-y: auto;
  white-space: pre-wrap; word-break: break-all;
}

/* ── Live badge ──────────────────────────────────────────────────────────── */

.live-badge {
  display: inline-flex; align-items: center; gap: 4px;
  color: #2196F3; font-size: 11px; flex-shrink: 0;
}

/* ── Blinking cursor inside streaming pre ────────────────────────────────── */

.log-cursor             { animation: blink 1s step-end infinite; }
@keyframes blink        { 0%, 100% { opacity: 1; } 50% { opacity: 0; } }
```

### Step 3: Verify in the browser

With the daemon running, open the web UI and navigate to the Activity tab.

**What to look for:**

1. Existing run cards still display — they won't look perfect yet (the new CSS assumes the new HTML structure), but they should not crash the page.
2. Open DevTools → Console: no JavaScript errors.
3. Run cards may appear taller/narrower than before — that's expected until Task 2 replaces the HTML templates.

If the page is broken or cards disappear entirely, check the CSS syntax. Common mistake: missing semicolons or unclosed braces.

### Step 4: Commit

```bash
git add web/style.css
git commit -m "feat: add log panel, live badge, and run card v2 CSS"
```

---

## Task 2: Replace renderRunCard, add toggleLog and copyLog

**Files:**
- Modify: `web/app.js`

This task replaces `renderRunCard` with three distinct templates and adds `toggleLog` and `copyLog`. It also removes `toggleRunLog` (the old toggle function) since it's superseded.

No new global state variables yet — `liveSources` and `failedSources` come in Task 4.

---

### Step 1: Replace renderRunCard

Find the existing `renderRunCard` function in `web/app.js`:

```js
function renderRunCard(run) {
  const icons = { success: '✓', failed: '✗', timeout: '⏱', running: '●', pending: '○', skipped: '—' };
  const colors = { success: 'var(--green)', failed: 'var(--red)', timeout: 'var(--yellow)', running: 'var(--blue)', pending: 'var(--text2)', skipped: 'var(--text2)' };
  const icon = icons[run.status] || '?';
  const color = colors[run.status] || 'var(--text2)';
  const when = run.endedAt ? timeAgo(run.endedAt) : 'running…';
  const duration = run.endedAt ? durationMs(new Date(run.startedAt), new Date(run.endedAt)) : '';
  const hasOutput = run.output && run.output.trim().length > 0;
  const outputId = `out-${run.id}`;
  return `
  <div class="run-card">
    <div class="run-status-icon" style="color:${color}">${icon}</div>
    <div class="run-info">
      <div class="run-job-name">${esc(run.jobName || run.jobId)}</div>
      <div class="run-meta">${when}${duration ? ' · ' + duration : ''}${run.attempt > 1 ? ` · attempt ${run.attempt}` : ''}${hasOutput ? ` <a href="#" class="run-log-toggle" onclick="toggleRunLog('${outputId}',this);return false">logs</a>` : ''}</div>
      ${hasOutput ? `<pre class="run-output hidden" id="${outputId}">${esc(run.output)}</pre>` : ''}
    </div>
  </div>`;
}
```

Replace the entire function with:

```js
function renderRunCard(run) {
  const name = esc(run.jobName || run.jobId);
  const attemptSuffix = run.attempt > 1 ? ` · attempt ${run.attempt}` : '';

  if (run.status === 'running') {
    const when = `started ${timeAgo(run.startedAt)}`;
    return `
<div class="run-card live" id="run-${run.id}">
  <div class="run-header">
    <span class="run-status-icon" id="status-icon-${run.id}" style="color:#2196F3">●</span>
    <span class="run-name">${name}${attemptSuffix}</span>
    <span class="run-time" id="run-time-${run.id}">${when}</span>
    <span class="live-badge" id="live-badge-${run.id}">● live</span>
    <a class="log-toggle" href="#" onclick="toggleLog('${run.id}', this); return false;">hide ▴</a>
  </div>
  <div class="log-panel" id="log-${run.id}">
    <div class="log-toolbar">
      <span class="log-label" id="log-label-${run.id}">streaming output</span>
      <button class="log-copy-btn" onclick="copyLog('${run.id}')">copy</button>
    </div>
    <pre class="log-output" id="logout-${run.id}"><span id="cursor-${run.id}" class="log-cursor">▌</span></pre>
  </div>
</div>`;
  }

  // Completed, failed, or timeout.
  const icon  = run.status === 'success' ? '✓' : '✕';
  const color = run.status === 'success' ? 'var(--green)' : 'var(--red)';
  const when  = run.endedAt
    ? `${timeAgo(run.startedAt)} · ${durationMs(new Date(run.startedAt), new Date(run.endedAt))}`
    : timeAgo(run.startedAt);
  const failedClass = run.status !== 'success' ? ' run-failed' : '';

  return `
<div class="run-card${failedClass}" id="run-${run.id}">
  <div class="run-header">
    <span class="run-status-icon" id="status-icon-${run.id}" style="color:${color}">${icon}</span>
    <span class="run-name">${name}${attemptSuffix}</span>
    <span class="run-time" id="run-time-${run.id}">${when}</span>
    <a class="log-toggle" href="#" onclick="toggleLog('${run.id}', this); return false;">logs ▾</a>
  </div>
  <div class="log-panel hidden" id="log-${run.id}">
    <div class="log-toolbar">
      <span class="log-label" id="log-label-${run.id}">stdout + stderr</span>
      <button class="log-copy-btn" onclick="copyLog('${run.id}')">copy</button>
    </div>
    <pre class="log-output" id="logout-${run.id}">${esc(run.output || '')}</pre>
  </div>
</div>`;
}
```

### Step 2: Add toggleLog and copyLog

Find the existing `toggleRunLog` function:

```js
function toggleRunLog(id, link) {
  const el = document.getElementById(id);
  if (!el) return;
  const hidden = el.classList.toggle('hidden');
  link.textContent = hidden ? 'logs' : 'hide logs';
}
```

Replace it with:

```js
function toggleLog(id, link) {
  const panel = document.getElementById(`log-${id}`);
  if (!panel) return;
  const hidden = panel.classList.toggle('hidden');
  link.textContent = hidden ? 'logs ▾' : 'hide ▴';
}

function copyLog(runId) {
  const pre = document.getElementById(`logout-${runId}`);
  if (!pre) return;
  // Collect text from all child nodes, skipping the blinking cursor span.
  const text = Array.from(pre.childNodes)
    .filter(n => !(n.nodeType === Node.ELEMENT_NODE && n.id === `cursor-${runId}`))
    .map(n => n.textContent)
    .join('');
  navigator.clipboard.writeText(text).catch(() => {});  // silent failure on HTTP or permission denial
}
```

### Step 3: Verify in the browser

**Setup:** Open the web UI Activity tab with the daemon running and at least 2–3 completed runs visible.

**What to look for:**

1. **Completed run card layout:**  
   Each run card shows: `[✓ icon] [job name] [time ago · duration] [logs ▾]` all on one horizontal line.

2. **Log panel toggle:**  
   Click `logs ▾` → panel expands showing a dark `#0d1117` pre block with the output text.  
   The link text changes to `hide ▴`.  
   Click `hide ▴` → panel collapses. Link returns to `logs ▾`.

3. **Copy button:**  
   Open a log panel. Click `copy`. Paste somewhere — should contain the raw output text with no ▌ cursor character.

4. **Failed run card:**  
   A failed run shows `✕` icon and has a red left border (`border-left: 3px solid var(--red)`).

5. **No console errors** in DevTools.

If cards are invisible or layout is broken, check that the CSS from Task 1 was saved correctly. The most common issue is `.run-card { display: block; padding: 0; }` not being applied — verify it in DevTools computed styles.

### Step 4: Commit

```bash
git add web/app.js
git commit -m "feat: new renderRunCard templates, toggleLog, copyLog"
```

---

## Task 3: loadRuns() refactor — incremental per-card updates

**Files:**
- Modify: `web/app.js`

This task replaces the `loadRuns()` + `renderRuns()` full-innerHTML approach with incremental per-card logic that preserves open panels and running SSE connections across polls.

---

### Step 1: Replace loadRuns and renderRuns

Find the existing `loadRuns` and `renderRuns` functions:

```js
async function loadRuns() {
  try {
    const runs = await api('GET', '/api/runs?limit=30');
    renderRuns(runs);
  } catch {}
}
```

```js
function renderRuns(runs) {
  const container = document.getElementById('runs-list');
  if (!runs.length) {
    container.innerHTML = '<div class="empty">No activity yet.</div>';
    return;
  }
  container.innerHTML = runs.map(renderRunCard).join('');
}
```

Replace **both** with the following single function. Do not keep `renderRuns` — it will no longer be used:

```js
// ── Runs ──────────────────────────────────────────────────────────────────────

async function loadRuns() {
  let runs;
  try {
    runs = await api('GET', '/api/runs?limit=30');
  } catch {
    return;
  }

  const list = document.getElementById('runs-list');

  if (!runs.length) {
    // Only replace content if there are truly no runs — preserve any existing
    // cards momentarily visible during a clear-then-repopulate transition.
    if (!list.querySelector('.run-card')) {
      list.innerHTML = '<div class="empty">No activity yet.</div>';
    }
    return;
  }

  // Remove stale empty-state placeholder if runs just appeared.
  const empty = list.querySelector('.empty');
  if (empty) empty.remove();

  // The API returns runs newest-first. Iterate oldest-first so each
  // insertAdjacentHTML('afterbegin') leaves the newest card at the top.
  for (const run of [...runs].reverse()) {
    const existing = document.getElementById(`run-${run.id}`);

    if (existing) {
      // ── Existing card: update in-place, never replace outerHTML ────────────
      // Replacing outerHTML would collapse user-opened log panels and kill SSE.

      // Always update elapsed time.
      const timeEl = document.getElementById(`run-time-${run.id}`);
      if (timeEl) {
        timeEl.textContent = run.status === 'running'
          ? `started ${timeAgo(run.startedAt)}`
          : `${timeAgo(run.startedAt)} · ${durationMs(new Date(run.startedAt), new Date(run.endedAt))}`;
      }

      // For non-live completed cards: sync status icon in case onerror
      // falsely marked the card as failed before the run actually finished.
      if (!liveSources[run.id] && run.status !== 'running') {
        const icon = document.getElementById(`status-icon-${run.id}`);
        if (icon) {
          icon.textContent = run.status === 'success' ? '✓' : '✕';
          icon.style.color = run.status === 'success' ? 'var(--green)' : 'var(--red)';
        }
        const card = document.getElementById(`run-${run.id}`);
        if (card) {
          card.classList.remove('run-failed');
          if (run.status !== 'success') card.classList.add('run-failed');
        }
      }

      // Start SSE if the run is still going and we don't already have a source.
      if (!liveSources[run.id] && !failedSources.has(run.id) && run.status === 'running') {
        openLiveLog(run.id);
      }
      continue;
    }

    // ── New card: render and prepend ──────────────────────────────────────────
    list.insertAdjacentHTML('afterbegin', renderRunCard(run));
    if (run.status === 'running') openLiveLog(run.id);
  }

  // ── Remove cards no longer in the API response ────────────────────────────
  // Close any live SSE connection first so we don't leak EventSources.
  list.querySelectorAll('.run-card').forEach(el => {
    if (!runs.find(r => `run-${r.id}` === el.id)) {
      const runId = el.id.replace('run-', '');
      if (liveSources[runId]) {
        liveSources[runId].close();
        delete liveSources[runId];
      }
      el.remove();
    }
  });
}
```

### Step 2: Add the module-level state variables

The new `loadRuns` references `liveSources` and `failedSources` which don't exist yet. Add them near the top of the `// ── Runs ──` section, right before the new `loadRuns` function:

```js
const liveSources   = {};          // runId → EventSource (open SSE connections)
const failedSources = new Set();   // runIds that errored via SSE — never reconnect
```

Place these two lines immediately **before** `async function loadRuns()`.

> Note: `openLiveLog` and `finalizeRunCard` are referenced by `loadRuns` but implemented in Task 4. The page will log `ReferenceError: openLiveLog is not defined` on the console until Task 4 is complete. That's expected — the page won't crash, it'll just skip the SSE subscription for running cards.

### Step 3: Verify in the browser

Open the Activity tab with the daemon running.

**What to look for:**

1. **Runs display correctly** — cards appear, newest at top, same as before.

2. **Panel state preserved across polls** — expand a completed run's log panel (click `logs ▾`). Wait 3–6 seconds for the next poll. The panel must stay open. If it collapses on poll, the incremental logic is not working — `renderRuns` was probably not fully removed or there's a typo in `getElementById`.

3. **Time updates in place** — for a running job (trigger one if needed), the "started X ago" time in the card header updates every 3 seconds without the card flashing.

4. **Console error for openLiveLog** — you will see `ReferenceError: openLiveLog is not defined` in the console. This is **expected at this stage** and will be fixed in Task 4.

### Step 4: Commit

```bash
git add web/app.js
git commit -m "feat: incremental loadRuns — preserve panel state across polls"
```

---

## Task 4: openLiveLog, finalizeRunCard, liveSources, failedSources

**Files:**
- Modify: `web/app.js`

This task implements the SSE client: subscribing to `/api/runs/{id}/stream`, appending chunks to the pre, and transitioning the card to its final state when the run completes.

---

### Step 1: Add openLiveLog and finalizeRunCard

In `web/app.js`, find the comment and blank line just after the `copyLog` function (which ends with `navigator.clipboard...`). Add these two functions immediately after `copyLog`:

```js
// ── SSE live log ──────────────────────────────────────────────────────────────

function openLiveLog(runId) {
  const pre    = document.getElementById(`logout-${runId}`);
  const cursor = document.getElementById(`cursor-${runId}`);
  if (!pre) return;  // card may have been removed by the time this fires

  const src = new EventSource(`/api/runs/${runId}/stream`);
  liveSources[runId] = src;

  src.onmessage = e => {
    let d;
    try { d = JSON.parse(e.data); } catch { return; }
    if (!d.chunk) return;

    // Insert text before the cursor span so the cursor stays at the end.
    const atBottom = pre.scrollHeight - pre.scrollTop <= pre.clientHeight + 4;
    pre.insertBefore(document.createTextNode(d.chunk), cursor);
    if (atBottom) pre.scrollTop = pre.scrollHeight;
  };

  src.addEventListener('done', e => {
    src.close();
    delete liveSources[runId];
    let payload;
    try { payload = JSON.parse(e.data); } catch { payload = {}; }
    finalizeRunCard(runId, payload.status || 'failed', payload.started_at, payload.ended_at);
  });

  src.onerror = () => {
    src.close();
    delete liveSources[runId];
    failedSources.add(runId);   // prevent loadRuns from reconnecting after error
    finalizeRunCard(runId, 'failed');
  };
}

function finalizeRunCard(runId, status, startedAt, endedAt) {
  // All selectors use optional chaining — this may be called twice if both
  // onerror and a stale done event fire (harmless).

  // 1. Remove live class from card.
  document.getElementById(`run-${runId}`)?.classList.remove('live');

  // 2. Update status icon.
  const icon = document.getElementById(`status-icon-${runId}`);
  if (icon) {
    icon.textContent  = status === 'success' ? '✓' : '✕';
    icon.style.color  = status === 'success' ? 'var(--green)' : 'var(--red)';
  }

  // 3. Remove the live badge.
  document.getElementById(`live-badge-${runId}`)?.remove();

  // 4. Change log panel label from "streaming output" to "stdout + stderr".
  const label = document.getElementById(`log-label-${runId}`);
  if (label) label.textContent = 'stdout + stderr';

  // 5. Remove blinking cursor.
  document.getElementById(`cursor-${runId}`)?.remove();

  // 6. Add run-failed class for non-success outcomes.
  if (status !== 'success') {
    document.getElementById(`run-${runId}`)?.classList.add('run-failed');
  }

  // 7. Update elapsed time if timing data is available.
  if (startedAt && endedAt) {
    const timeEl = document.getElementById(`run-time-${runId}`);
    if (timeEl) {
      timeEl.textContent = `${timeAgo(startedAt)} · ${durationMs(new Date(startedAt), new Date(endedAt))}`;
    }
  }

  // 8. Update log toggle link text to collapsed state.
  //    The panel stays open (user can close it manually).
  const toggleLink = document.querySelector(`#run-${runId} .log-toggle`);
  if (toggleLink && toggleLink.textContent.includes('hide')) {
    // Panel is currently open — keep it open, but update the link wording.
    // (It already says "hide ▴" so no change needed.)
  }
}
```

### Step 2: Verify there are no reference errors in the console

Open DevTools → Console. Reload the Activity tab. The `ReferenceError: openLiveLog is not defined` error from Task 3 should be **gone** now.

If you still see it, the function wasn't saved in the right place or there's a syntax error. Check the console error message carefully — it will show the line number.

### Step 3: Verify with a live job

Trigger a slow job so you can observe streaming. In the UI, click **▶ Run now** on any shell job, or create a quick test job:

1. Click **+ Add Job**
2. Name it `sse-test`, executor `shell`, trigger `once`, command:  
   `for i in 1 2 3 4 5; do echo "step $i"; sleep 0.5; done`
3. Save and click **▶ Run now**
4. Switch to the **Activity** tab immediately

**What to look for (in order):**

| # | What you should see |
|---|---------------------|
| 1 | A new card appears at the top with a **●** blue icon and **`● live`** badge |
| 2 | The log panel is open (no click required) with a blinking **▌** cursor |
| 3 | Lines appear one by one: `step 1`, `step 2`, etc. — text inserted before the cursor |
| 4 | The panel auto-scrolls as lines arrive |
| 5 | After ~3 seconds, streaming stops |
| 6 | The **●** icon becomes **✓** (green), `● live` badge disappears, blinking cursor disappears |
| 7 | The label changes from `streaming output` to `stdout + stderr` |
| 8 | The time field updates from `started Xs ago` to `Xs ago · Ys` |
| 9 | The panel stays open — you can now click `hide ▴` to collapse it |
| 10 | Clicking `copy` copies the full output without the ▌ character |

If streaming doesn't start, open DevTools → Network tab and look for the `stream` request. It should be a long-lived connection with `text/event-stream` content type. If it 404s, Phase 1 backend is not complete. If it returns immediately, check that the daemon was restarted after Phase 1 changes.

### Step 4: Verify panel state survives polling

After a run completes and its panel is open:

1. Wait 6+ seconds (two poll cycles)
2. The panel must remain open
3. The log content must not change or flicker
4. DevTools → Network should show no new `stream` requests for completed runs

### Step 5: Verify failedSources guard

To test the `onerror` path: stop the daemon while a job is running (Ctrl+C the daemon process). 

**What to see:**
- The SSE connection errors
- The card transitions to **✕** failed state (via `finalizeRunCard('failed')`)
- After restarting the daemon, the next `loadRuns` poll **does not** reconnect SSE for that run (because `failedSources` has the ID)
- The `loadAll` poll does eventually correct the status icon to `✓` (if the run actually succeeded before the daemon stopped) by syncing with the store

### Step 6: Commit

```bash
git add web/app.js
git commit -m "feat: SSE live log streaming — openLiveLog, finalizeRunCard"
```

---

## Task 5: Final smoke test — three jobs, verify all scenarios

No code changes. This task is purely manual verification that everything works together.

---

### Scenario A: One job running, two completed — all three visible simultaneously

1. **Setup:** Ensure at least 2 completed runs are in the Activity tab from prior testing.
2. **Trigger a slow job:**  
   `for i in $(seq 1 8); do echo "line $i"; sleep 0.4; done`
3. **While it runs**, verify:
   - The running card is at the top with blue border and live badge
   - The two completed cards below it are stable — no flicker, no re-render
   - The running card's elapsed time updates every 3s without resetting the log content
4. **After it completes**, verify:
   - Blue border gone, live badge gone, cursor gone
   - Final time shown (e.g. `12s ago · 3.2s`)
   - All three cards' log panels can be independently opened/closed

### Scenario B: Copy button works correctly

1. Open a completed run's log panel (click `logs ▾`)
2. Click `copy`
3. Paste into a text editor
4. Verify: the text is the raw output, no `▌` character, no HTML entities like `&lt;`

### Scenario C: Two jobs running at the same time

1. Trigger two different slow jobs in quick succession (within 1 second of each other):
   - Job A: `for i in 1 2 3; do echo "A-$i"; sleep 0.5; done`
   - Job B: `for i in 1 2 3; do echo "B-$i"; sleep 0.5; done`
2. Watch the Activity tab:
   - Both cards appear with live panels
   - `A-1`, `A-2`, `A-3` stream into card A's panel
   - `B-1`, `B-2`, `B-3` stream into card B's panel — no cross-contamination
3. Both cards finalize independently

### Scenario D: Reload page mid-run

1. Trigger a slow job (`for i in $(seq 1 10); do echo $i; sleep 0.5; done`)
2. While it's running (around step 4–5), **reload the browser page**
3. After reload:
   - The running card should appear immediately
   - `openLiveLog` should be called by `loadRuns`
   - The log panel should show buffered output from before the reload (the backend broadcaster replays its buffer on new subscription)
   - Streaming continues to completion

### Scenario E: Completed run — 404 variant

1. Clear all activity (`DELETE /api/runs` via the "Clear" button)
2. Trigger and complete a job
3. Subscribe to the stream URL in the browser address bar:  
   `http://localhost:61017/api/runs/<run-id>/stream`
4. You should see the stored output followed by `event: done` — rendered as plain text since browsers don't format SSE

### Commit the smoke test results

```bash
git add -A
git commit -m "test: manual smoke test complete — log viewer Phase 2 verified"
git tag phase2-frontend-complete
```

---

## Phase 2 complete — full feature verification checklist

Before tagging the release, run through this checklist:

| Check | Method |
|-------|--------|
| Phase 1 tests pass | `go test ./internal/... -v` — all green |
| Phase 1 integration | `PORT=61017 bash scripts/test-sse.sh` — 10/10 |
| Completed run panels expand/collapse | Manual — Task 2 |
| Copy button excludes cursor | Manual — Task 2 |
| Incremental updates preserve open panels | Manual — Task 3 |
| Running job streams live | Manual — Task 4 scenario |
| Finalization updates icon + time + label | Manual — Task 4 |
| Two concurrent jobs don't mix output | Manual — Task 5 scenario C |
| Page reload replays broadcaster buffer | Manual — Task 5 scenario D |
| No console errors after all tasks | DevTools Console — zero errors |

---

## Appendix: full diff summary

### web/style.css changes

1. Existing `.run-card` rule: `display: flex; align-items: flex-start; gap: 10px; padding: 10px 12px;` → `display: block; padding: 0; overflow: hidden;`
2. New rules appended at end of file: `.run-header`, `.run-name`, `.run-time`, `.log-toggle`, `.run-card.live`, `.run-card.run-failed`, `.log-panel`, `.log-panel.hidden`, `.log-toolbar`, `.log-label`, `.log-copy-btn`, `.log-output`, `.live-badge`, `.log-cursor`, `@keyframes blink`

### web/app.js changes

| Old | New |
|-----|-----|
| `renderRunCard(run)` — single template | `renderRunCard(run)` — 3 templates (running / success / failed) |
| `renderRuns(runs)` — full innerHTML replace | **removed** |
| `loadRuns()` — calls renderRuns | `loadRuns()` — incremental per-card |
| `toggleRunLog(id, link)` | **removed** → replaced by `toggleLog(id, link)` |
| *(not present)* | `copyLog(runId)` — new |
| *(not present)* | `const liveSources = {}` — new module-level |
| *(not present)* | `const failedSources = new Set()` — new module-level |
| *(not present)* | `openLiveLog(runId)` — new |
| *(not present)* | `finalizeRunCard(runId, status, startedAt, endedAt)` — new |
