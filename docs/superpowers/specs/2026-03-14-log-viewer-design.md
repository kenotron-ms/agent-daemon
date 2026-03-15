# Log Viewer for Activity Runs

**Date:** 2026-03-14  
**Status:** Approved

## Overview

Add an enhanced inline log viewer to the Activity tab that supports both completed and live-streaming run output. When a user clicks "logs" on a run card, the card expands in-place to reveal a styled log panel. For runs that are currently executing, the panel streams output in real-time via Server-Sent Events (SSE).

## Goals

- View stdout+stderr output for completed runs in a readable, scrollable panel
- Watch live output from actively running jobs without waiting for completion
- Keep the activity list visible — no modal, no navigation away
- Copy log output with one click

## UI Design

### Run Card — Collapsed (default)

Unchanged from today except the "logs" link shows a `▾` chevron.

```
✓  build:release   2 minutes ago · 34s                    logs ▾
```

### Run Card — Expanded (completed run)

The card grows in-place to reveal the log panel below the header row:

```
✓  build:release   2 minutes ago · 34s                    hide ▴
┌─────────────────────────────────────────────────────────────────┐
│ stdout + stderr                                          copy   │
│─────────────────────────────────────────────────────────────────│
│ + Installing dependencies...                                    │
│ ✓ node_modules ready                                            │
│ ✓ TypeScript compiled (0 errors)                                │
│ ✓ Build complete in 34.2s                                       │
└─────────────────────────────────────────────────────────────────┘
```

- Max height: `240px`, scrollable
- Font: monospace, `11px`
- Dark background (`#0d1117`), light text (`#c9d1d9`)
- Copy button copies full content to clipboard

### Run Card — Expanded (live/running)

Running jobs get a blue accent border and a `● live` badge. The panel auto-expands on load (no click required for running jobs). Output auto-scrolls to the bottom as chunks arrive.

```
●  sync:data   started 12s ago                         ● live  hide ▴
┌─────────────────────────────────────────────────────────────────┐
│ streaming output                                         copy   │
│─────────────────────────────────────────────────────────────────│
│ ✓ Connected to database                                         │
│ → Fetching records (batch 1/10)...                              │
│ ✓ Batch 1 complete (1,240 rows)                                 │
│ → Fetching records (batch 2/10)... ▌                            │
└─────────────────────────────────────────────────────────────────┘
```

- Blue border (`#1a3a5c`) and glow on the card while running
- "streaming output" label instead of "stdout + stderr"
- Cursor blink (`▌`) on the last line while connected
- When the run completes, the SSE connection closes, the live badge disappears, and the panel transitions to the completed state (blue border removed)

## Backend Design

### New SSE Endpoint

```
GET /api/runs/{id}/stream
```

**Completed run:** Returns the stored `run.Output` as a single `data:` event followed by a `event: done` marker, then closes. This lets the frontend use a single code path (EventSource) for both live and historical output.

**Running run:** Streams buffered output first (chunks accumulated since run start), then streams new chunks as they arrive. Closes with `event: done` when the run completes or errors.

**SSE event format:**
```
data: {"chunk":"output text here\n"}

event: done
data: {"status":"success","exit_code":0}
```

### In-Memory Output Broadcaster

New component: `internal/scheduler/broadcaster.go`

```go
type Broadcaster struct { ... }

func (b *Broadcaster) Register(runID string)
func (b *Broadcaster) Write(runID, chunk string)       // called by executors
func (b *Broadcaster) Subscribe(runID string) (buffered []string, ch <-chan string, ok bool)
func (b *Broadcaster) Unsubscribe(runID string, ch chan string)
func (b *Broadcaster) Complete(runID string)           // signals done, closes channels
func (b *Broadcaster) Remove(runID string)             // cleanup after all subscribers gone
```

- `Register` is called at run start; `Complete` at run end; `Remove` after a grace period
- `Write` appends to an in-memory buffer AND fans out to all active subscriber channels
- `Subscribe` returns the accumulated buffer (for late subscribers) plus a channel for new chunks
- Thread-safe via `sync.RWMutex`

The `Broadcaster` instance lives on the `Daemon` struct, passed into both the scheduler (for writes) and the API server (for SSE reads).

### Executor Changes

Each executor (`exec_shell.go`, `exec_claude_code.go`, `exec_amplifier.go`) currently uses `cmd.CombinedOutput()` which blocks until completion. This needs to switch to streaming capture:

1. Replace `cmd.CombinedOutput()` with `cmd.StdoutPipe()` + `cmd.StderrPipe()`
2. Read from pipes in a goroutine, writing chunks to `broadcaster.Write(runID, chunk)`
3. Accumulate chunks into a string buffer (for DB persistence at end — unchanged)
4. Return the accumulated string as before

The executor interface signature does not change. The broadcaster is injected via the `Runner` struct.

### API Server

- Add route: `GET /api/runs/{id}/stream` → `handlers_stream.go`
- `Server` struct receives the `Broadcaster` reference at construction time

### Store / DB

No changes. Output is still written to the `runs` BoltDB bucket at run completion, exactly as today.

## Frontend Design

### `renderRunCard` changes (`web/app.js`)

Replace the existing simple toggle with the new panel structure:

```html
<!-- Toolbar replaces the old <a class="run-log-toggle"> -->
<div class="log-panel" id="log-{id}">
  <div class="log-toolbar">
    <span class="log-label">stdout + stderr</span>
    <button class="log-copy-btn" onclick="copyLog('{id}')">copy</button>
  </div>
  <pre class="log-output" id="logout-{id}">{escaped output}</pre>
</div>
```

For running jobs, the panel is rendered open by default with `log-live` class on the card.

### SSE subscription

```js
function openLiveLog(runId) {
  const pre = document.getElementById(`logout-${runId}`);
  const src = new EventSource(`/api/runs/${runId}/stream`);
  src.onmessage = e => {
    const d = JSON.parse(e.data);
    if (d.chunk) {
      pre.textContent += d.chunk;
      pre.scrollTop = pre.scrollHeight;
    }
  };
  src.addEventListener('done', e => {
    src.close();
    const d = JSON.parse(e.data);
    // update card status badge + remove live styling
    finalizeRunCard(runId, d.status);
  });
  liveSources[runId] = src;
}
```

- `liveSources` is a module-level map so open connections can be closed when the panel is hidden or the run card is removed from the DOM
- The existing `loadAll()` 3-second poll continues unchanged — it refreshes the list and adds new run cards; running cards with an open SSE connection skip their output from being overwritten

### Copy button

```js
function copyLog(runId) {
  const text = document.getElementById(`logout-${runId}`).textContent;
  navigator.clipboard.writeText(text);
}
```

### CSS additions (`web/style.css`)

```css
.run-card.live          { border-color: #1a3a5c; box-shadow: 0 0 0 1px rgba(33,150,243,0.2); }
.log-panel              { border-top: 1px solid #222; }
.log-toolbar            { display: flex; justify-content: space-between; padding: 4px 12px; background: #111827; }
.log-label              { color: #555; font-size: 11px; font-family: monospace; }
.log-copy-btn           { background: none; border: none; color: #4a9eff; font-size: 11px; cursor: pointer; padding: 0; }
.log-output             { margin: 0; padding: 12px; background: #0d1117; color: #c9d1d9;
                          font-size: 11px; font-family: 'SF Mono', monospace; line-height: 1.5;
                          max-height: 240px; overflow-y: auto; }
.log-live-badge         { display: inline-flex; align-items: center; gap: 4px; color: #2196F3; font-size: 11px; }
```

## File Inventory

| File | Change |
|---|---|
| `internal/scheduler/broadcaster.go` | **New** — output broadcaster |
| `internal/api/handlers_stream.go` | **New** — SSE endpoint handler |
| `internal/api/server.go` | Add route + broadcaster field |
| `internal/service/daemon.go` | Construct broadcaster, wire to scheduler + API |
| `internal/scheduler/runner.go` | Accept broadcaster, call Register/Complete |
| `internal/scheduler/exec_shell.go` | Switch to streaming pipe capture |
| `internal/scheduler/exec_claude_code.go` | Switch to streaming pipe capture |
| `internal/scheduler/exec_amplifier.go` | Switch to streaming pipe capture |
| `web/app.js` | New log panel, SSE subscription, copy button |
| `web/style.css` | New log panel styles |

## Out of Scope

- Log search / filtering
- Log download (file save)
- Persisting partial output to DB during run (only final output is stored)
- Log rotation or size limits beyond the existing 64 KB cap
