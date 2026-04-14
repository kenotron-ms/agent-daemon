# Phase 3: Jobs View Redesign — Implementation Plan

> **For execution:** Use `/execute-plan` mode.

**Prerequisite:** Phase 1 and Phase 2 complete.
**Goal:** Add hover-delete to job cards; replace tab-per-run with master-detail split panel.
**Tech Stack:** React 18, TypeScript, CSS flexbox, SSE

**Architecture:** The existing Jobs view has a 200px left sidebar (`JobList.tsx`) listing jobs and a right panel (`RunDetail.tsx`) that shows runs as horizontal numbered tabs above a log viewer. This plan adds an X-on-hover delete button to `JobList`, then replaces the horizontal tab bar in `RunDetail` with a vertical two-column split: a 280px run list on the left showing formatted timestamps, trigger badges, status dots with pulsing animation, and durations — and a flex-grow log viewer on the right that reuses the existing `useRunStream` SSE hook without modification.

**Relevant backend routes (already exist — no backend changes needed):**
- `DELETE /api/jobs/{id}` → `{"status":"deleted"}`
- `GET /api/jobs/{id}/runs?limit=20` → `JobRun[]`
- `GET /api/runs/{id}/stream` → SSE: `data: {"chunk":"..."}` messages + `event: done`
- `GET /api/runs/{id}` → single `JobRun`

**Key Go types (source of truth for JSON field names):**
```go
// internal/types/types.go
type JobRun struct {
    ID        string     `json:"id"`
    JobID     string     `json:"jobId"`
    JobName   string     `json:"jobName"`
    StartedAt time.Time  `json:"startedAt"`
    EndedAt   *time.Time `json:"endedAt,omitempty"`  // NOT "finishedAt"
    Status    RunStatus  `json:"status"`              // "running"|"success"|"failed"|"timeout"|"skipped"|"pending"
    ExitCode  int        `json:"exitCode"`
    Output    string     `json:"output"`
    Attempt   int        `json:"attempt"`
}
```

---

### Task 1: Update API client — add `deleteJob`, align `JobRun` with backend

**Files:**
- Modify: `ui/src/api/jobs.ts`

**Why:** The current `JobRun` TypeScript interface has `finishedAt` but the Go backend sends `endedAt`. Status values also differ (`succeeded` in TS vs `success` in Go). Fix the interface and add the missing `deleteJob` function needed for the X button.

**Implementation:** Replace the entire file with:

```typescript
export interface Job {
  id: string
  name: string
  description: string
  enabled: boolean
  trigger: { type: string; schedule: string }
  executor: string
  lastRunAt?: string
  lastRunStatus?: string
}

export interface JobRun {
  id: string
  jobId: string
  jobName: string
  status: 'pending' | 'running' | 'success' | 'failed' | 'timeout' | 'skipped'
  startedAt: string
  endedAt?: string
  exitCode: number
  output?: string
  attempt: number
}

export async function listJobs(): Promise<Job[]> {
  const res = await fetch('/api/jobs')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function listJobRuns(jobId: string, limit = 20): Promise<JobRun[]> {
  const res = await fetch(`/api/jobs/${jobId}/runs?limit=${limit}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function triggerJob(jobId: string): Promise<JobRun> {
  const res = await fetch(`/api/jobs/${jobId}/trigger`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deleteJob(jobId: string): Promise<void> {
  const res = await fetch(`/api/jobs/${jobId}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await res.text())
}
```

**Build check:** `cd ui && npm run build` — Expected: no TypeScript errors

**Commit:** `git add -A && git commit -m "feat(jobs): add deleteJob API, align JobRun interface with Go backend"`

---

### Task 2: Add pulsing-dot CSS keyframe for in-progress runs

**Files:**
- Modify: `ui/src/index.css`

**Why:** The master-detail run list (Task 5) needs a pulsing animation on status dots for in-progress runs. Add it now so it's available.

**Implementation:** Append this block at the very end of `ui/src/index.css`, after the existing `.hljs-type` rule on the last line:

```css

/* ── Pulsing status dot for in-progress runs ────────────────────── */
@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}
```

**Build check:** `cd ui && npm run build` — Expected: no errors

**Commit:** `git add -A && git commit -m "feat(jobs): add pulse-dot keyframe animation for running status"`

---

### Task 3: Add hover X delete button to job cards

**Files:**
- Modify: `ui/src/views/jobs/JobList.tsx`

**Why:** Each job card needs an X icon that appears on hover, positioned top-right. Clicking it calls the parent's `onDelete` callback. The X must not crowd the card when not hovering.

**Implementation:** Replace the entire file with:

```tsx
import { useState } from 'react'
import { Job } from '../../api/jobs'

interface Props {
  jobs: Job[]
  selectedId: string | null
  onSelect: (id: string) => void
  onNew: () => void
  onDelete: (id: string) => void
}

function StatusDot({ status }: { status: string }) {
  const isRunning = status === 'running'
  return (
    <span style={{
      width: 6, height: 6, borderRadius: '50%',
      background: isRunning ? 'var(--amber)' : 'var(--text-very-muted)',
      display: 'inline-block', flexShrink: 0,
    }} />
  )
}

export default function JobList({ jobs, selectedId, onSelect, onNew, onDelete }: Props) {
  const [hoveredId, setHoveredId] = useState<string | null>(null)

  return (
    <div style={{
      width: 200,
      flexShrink: 0,
      display: 'flex',
      flexDirection: 'column',
      background: 'var(--bg-sidebar)',
      borderRight: '1px solid var(--border)',
      height: '100%',
    }}>
      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '0 12px',
        height: 32,
        borderBottom: '1px solid var(--border)',
        flexShrink: 0,
      }}>
        <span style={{
          fontSize: 10, fontWeight: 600,
          textTransform: 'uppercase', letterSpacing: '0.08em',
          color: 'var(--text-very-muted)',
        }}>Jobs</span>
        <button
          onClick={onNew}
          style={{
            fontSize: 14, lineHeight: 1,
            color: 'var(--text-muted)',
            background: 'none', border: 'none', cursor: 'pointer',
            padding: '0 2px',
          }}
          onMouseEnter={e => (e.currentTarget as HTMLElement).style.color = 'var(--amber)'}
          onMouseLeave={e => (e.currentTarget as HTMLElement).style.color = 'var(--text-muted)'}
          title="New job"
        >+</button>
      </div>

      {/* Job list */}
      <div style={{ flex: 1, overflowY: 'auto' }} className="canvas-scroll">
        {jobs.map(job => (
          <div
            key={job.id}
            style={{ position: 'relative' }}
            onMouseEnter={() => setHoveredId(job.id)}
            onMouseLeave={() => setHoveredId(null)}
          >
            <button
              onClick={() => onSelect(job.id)}
              style={{
                width: '100%', textAlign: 'left',
                padding: '7px 12px 7px 14px',
                display: 'flex', alignItems: 'flex-start', gap: 8,
                background: selectedId === job.id ? 'var(--bg-sidebar-active)' : 'transparent',
                borderLeft: selectedId === job.id ? '2px solid var(--amber)' : '2px solid transparent',
                borderBottom: '1px solid var(--border)',
                borderTop: 'none', borderRight: 'none',
                cursor: 'pointer',
                transition: 'background 0.12s ease',
              }}
              onMouseEnter={e => {
                if (selectedId !== job.id)
                  (e.currentTarget as HTMLElement).style.background = 'rgba(0,0,0,0.03)'
              }}
              onMouseLeave={e => {
                if (selectedId !== job.id)
                  (e.currentTarget as HTMLElement).style.background = 'transparent'
              }}
            >
              <StatusDot status={job.lastRunStatus ?? 'idle'} />
              <div style={{ minWidth: 0, flex: 1 }}>
                <div style={{
                  fontSize: 12,
                  fontWeight: selectedId === job.id ? 500 : 400,
                  color: selectedId === job.id ? 'var(--text-primary)' : 'var(--text-muted)',
                  overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                }}>{job.name}</div>
                <div style={{
                  fontSize: 10, color: 'var(--text-very-muted)', marginTop: 2,
                }}>
                  {job.trigger.type}
                  {job.trigger.schedule && ` · ${job.trigger.schedule}`}
                </div>
              </div>
            </button>
            {/* X delete button — visible only on hover */}
            <button
              onClick={(e) => {
                e.stopPropagation()
                onDelete(job.id)
              }}
              style={{
                position: 'absolute',
                top: 6, right: 6,
                width: 18, height: 18,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                background: 'var(--bg-pane-title)',
                border: '1px solid var(--border)',
                borderRadius: 3,
                color: 'var(--text-muted)',
                fontSize: 11, lineHeight: 1,
                cursor: 'pointer',
                opacity: hoveredId === job.id ? 1 : 0,
                pointerEvents: hoveredId === job.id ? 'auto' : 'none',
                transition: 'opacity 0.12s ease',
                padding: 0,
              }}
              onMouseEnter={e => {
                (e.currentTarget as HTMLElement).style.color = 'var(--red)'
                (e.currentTarget as HTMLElement).style.borderColor = 'var(--red)'
              }}
              onMouseLeave={e => {
                (e.currentTarget as HTMLElement).style.color = 'var(--text-muted)'
                (e.currentTarget as HTMLElement).style.borderColor = 'var(--border)'
              }}
              title="Remove job"
            >×</button>
          </div>
        ))}
        {jobs.length === 0 && (
          <div style={{ padding: '16px 14px', fontSize: 11, color: 'var(--text-very-muted)' }}>
            No jobs yet
          </div>
        )}
      </div>
    </div>
  )
}
```

**Key changes from the original:**
- Added `onDelete` prop to the `Props` interface
- Wrapped each job `<button>` in a `<div style={{ position: 'relative' }}>` that tracks hover via `hoveredId` state
- Added the X button: `position: absolute; top: 6px; right: 6px; opacity: 0` (becomes `opacity: 1` when hovered)
- X button calls `e.stopPropagation()` so clicking it doesn't also select the job
- `pointerEvents: 'none'` when hidden prevents accidental clicks on invisible X
- X turns red on hover over the button itself

**Build check:** `cd ui && npm run build` — Expected: TypeScript error about missing `onDelete` prop in `index.tsx` (fixed in Task 4)

**Commit:** Do NOT commit yet — wait for Task 4 to wire up the parent.

---

### Task 4: Wire delete handler in JobsView parent

**Files:**
- Modify: `ui/src/views/jobs/index.tsx`

**Why:** JobList now expects an `onDelete` prop. Wire it up with a `window.confirm` dialog and the `deleteJob` API call from Task 1.

**Implementation:** Replace the entire file with:

```tsx
import { useCallback, useEffect, useState } from 'react'
import { Job, deleteJob, listJobs } from '../../api/jobs'
import JobList from './JobList'
import RunDetail from './RunDetail'
import ChatView from '../chat'

export default function JobsView() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const loadJobs = useCallback(async () => {
    const js = await listJobs().catch(() => [] as Job[])
    setJobs(js)
  }, [])

  useEffect(() => { loadJobs() }, [loadJobs])

  const selectedJob = jobs.find(j => j.id === selectedId) ?? null

  const handleSelect = (id: string) => setSelectedId(id)
  const handleNew = () => setSelectedId(null)

  const handleDelete = async (id: string) => {
    if (!window.confirm('Remove this job? This cannot be undone.')) return
    try {
      await deleteJob(id)
      setJobs(prev => prev.filter(j => j.id !== id))
      if (selectedId === id) setSelectedId(null)
    } catch (e) {
      console.error('deleteJob:', e)
    }
  }

  return (
    <div style={{ display: 'flex', height: '100%', background: 'var(--bg-page)' }}>
      <JobList
        jobs={jobs}
        selectedId={selectedId}
        onSelect={handleSelect}
        onNew={handleNew}
        onDelete={handleDelete}
      />
      <div className="flex-1 overflow-hidden">
        {selectedJob
          ? <RunDetail job={selectedJob} />
          : <ChatView onResponse={loadJobs} />
        }
      </div>
    </div>
  )
}
```

**Key changes from the original:**
- Added `deleteJob` to the import from `../../api/jobs`
- Added `handleDelete` function: shows `window.confirm`, calls `deleteJob(id)`, removes job from local state, clears selection if the deleted job was selected
- Passes `onDelete={handleDelete}` to `<JobList>`

**Build check:** `cd ui && npm run build` — Expected: no TypeScript errors

**Commit:** `git add -A && git commit -m "feat(jobs): add hover X button for job deletion with confirmation"`

---

### Task 5: Checkpoint A — hover X and delete confirmation

**Browser verification:**

```
agent-browser open http://localhost:7700
agent-browser snapshot -ic
# Click the Jobs tab
agent-browser click @eN       # (the Jobs tab button)
agent-browser snapshot -ic
agent-browser screenshot /tmp/phase3-checkpoint-a.png
agent-browser close
```

**Expected results:**
- Jobs tab shows the sidebar with job cards
- Hovering over a job card reveals a small X button at top-right
- Clicking the X shows a browser confirm dialog: "Remove this job? This cannot be undone."
- **Do NOT actually confirm deletion** during testing — click Cancel
- X button is invisible when not hovering (opacity: 0)

---

### Task 6: Redesign RunDetail as master-detail split

**Files:**
- Modify: `ui/src/views/jobs/RunDetail.tsx`
- No changes to: `ui/src/views/jobs/useRunStream.ts` (reused as-is)

**Why:** Replace the horizontal numbered tab bar (`#3 #2 #1`) with a two-column split: left column is a scrollable run list with rich info (timestamp, trigger badge, status dot, duration), right column is the log viewer. SSE streaming via `useRunStream` is reused without modification.

**Implementation:** Replace the entire file with:

```tsx
import { useEffect, useMemo, useRef, useState } from 'react'
import Convert from 'ansi-to-html'
import { Job, JobRun, listJobRuns, triggerJob } from '../../api/jobs'
import { useRunStream } from './useRunStream'

const ansiConvert = new Convert({ escapeXML: true, newline: false })

interface Props { job: Job }

function formatRunDate(iso: string): string {
  const d = new Date(iso)
  const month = d.toLocaleDateString('en-US', { month: 'short' })
  const day = d.getDate()
  const time = d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })
  return `${month} ${day} · ${time}`
}

function formatDuration(startIso: string, endIso?: string): string {
  const start = new Date(startIso).getTime()
  const end = endIso ? new Date(endIso).getTime() : Date.now()
  const secs = Math.max(0, Math.round((end - start) / 1000))
  if (secs < 60) return `${secs}s`
  const mins = Math.floor(secs / 60)
  const rem = secs % 60
  if (mins < 60) return rem > 0 ? `${mins}m ${rem}s` : `${mins}m`
  const hrs = Math.floor(mins / 60)
  const remM = mins % 60
  return remM > 0 ? `${hrs}h ${remM}m` : `${hrs}h`
}

function triggerBadge(type: string): { label: string; bg: string; color: string } {
  if (type === 'cron' || type === 'loop')
    return { label: 'Scheduled', bg: '#14b8a6', color: '#fff' }
  return { label: 'Manual', bg: 'var(--bg-pane-title)', color: 'var(--text-muted)' }
}

export default function RunDetail({ job }: Props) {
  const [runs, setRuns]               = useState<JobRun[]>([])
  const [activeRunId, setActiveRunId] = useState<string | null>(null)
  const logOutput = useRunStream(activeRunId)
  const logHtml   = useMemo(() => ansiConvert.toHtml(logOutput), [logOutput])
  const logRef    = useRef<HTMLDivElement>(null)

  const refreshRuns = async (jobId: string) => {
    try {
      const rs = await listJobRuns(jobId)
      const safe = rs ?? []
      setRuns(safe)
      if (safe.length > 0) setActiveRunId(safe[0].id)
    } catch (e) { console.error('listJobRuns:', e) }
  }

  useEffect(() => { refreshRuns(job.id) }, [job.id])

  // Auto-scroll log to bottom as new SSE chunks arrive
  useEffect(() => {
    if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight
  }, [logOutput])

  const handleTrigger = async () => {
    try {
      await triggerJob(job.id)
      setTimeout(() => refreshRuns(job.id), 800)
    } catch (e) { console.error('triggerJob:', e) }
  }

  const activeRun = runs.find(r => r.id === activeRunId) ?? null
  const badge = triggerBadge(job.trigger.type)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: 'var(--bg-right)' }}>
      {/* ── Header ──────────────────────────────────────── */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 10,
        padding: '0 16px', height: 36,
        background: 'var(--bg-pane-title)',
        borderBottom: '1px solid var(--border)',
        flexShrink: 0,
      }}>
        <span style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-primary)' }}>{job.name}</span>
        <button
          onClick={handleTrigger}
          style={{
            marginLeft: 'auto', fontSize: 11, padding: '4px 12px',
            background: 'var(--bg-modal)', border: '1px solid var(--border-dark)',
            borderRadius: 3, color: 'var(--text-primary)', cursor: 'pointer',
          }}
          onMouseEnter={e => (e.currentTarget as HTMLElement).style.background = 'var(--bg-pane-title)'}
          onMouseLeave={e => (e.currentTarget as HTMLElement).style.background = 'var(--bg-modal)'}
        >▶ Run Now</button>
      </div>

      {/* ── Master-detail split ─────────────────────────── */}
      <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>

        {/* Left column — Run List (280px) */}
        <div style={{
          width: 280, flexShrink: 0, overflowY: 'auto',
          borderRight: '1px solid var(--border)', background: 'var(--bg-right)',
        }} className="canvas-scroll">
          {runs.length === 0 && (
            <div style={{ padding: '24px 16px', fontSize: 11, color: 'var(--text-very-muted)', textAlign: 'center' }}>
              No runs yet — click ▶ Run Now to trigger a run.
            </div>
          )}
          {runs.map(run => {
            const isActive  = activeRunId === run.id
            const isRunning = run.status === 'running'
            const isSuccess = run.status === 'success'
            const isFailed  = run.status === 'failed' || run.status === 'timeout'
            return (
              <button
                key={run.id}
                onClick={() => setActiveRunId(run.id)}
                style={{
                  width: '100%', textAlign: 'left', padding: '10px 14px',
                  display: 'flex', alignItems: 'flex-start', gap: 10,
                  background: isActive ? 'var(--bg-sidebar-active)' : 'transparent',
                  borderLeft: isActive ? '2px solid var(--amber)' : '2px solid transparent',
                  borderBottom: '1px solid var(--border)',
                  borderTop: 'none', borderRight: 'none',
                  cursor: 'pointer', transition: 'background 0.12s ease',
                }}
                onMouseEnter={e => {
                  if (!isActive) (e.currentTarget as HTMLElement).style.background = 'rgba(0,0,0,0.03)'
                }}
                onMouseLeave={e => {
                  if (!isActive) (e.currentTarget as HTMLElement).style.background = 'transparent'
                }}
              >
                {/* Status dot */}
                <span style={{
                  width: 8, height: 8, borderRadius: '50%', marginTop: 4, flexShrink: 0,
                  background: isRunning ? 'var(--amber)'
                    : isSuccess ? 'var(--green)'
                    : isFailed  ? 'var(--red)'
                    : 'var(--text-very-muted)',
                  animation: isRunning ? 'pulse-dot 1.5s ease-in-out infinite' : 'none',
                }} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  {/* Timestamp */}
                  <div style={{
                    fontSize: 11.5,
                    fontWeight: isActive ? 500 : 400,
                    color: isActive ? 'var(--text-primary)' : 'var(--text-muted)',
                  }}>
                    {formatRunDate(run.startedAt)}
                  </div>
                  {/* Badge + duration */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 3 }}>
                    <span style={{
                      fontSize: 9, fontWeight: 600, padding: '1px 5px', borderRadius: 3,
                      background: badge.bg, color: badge.color,
                      textTransform: 'uppercase', letterSpacing: '0.04em',
                    }}>{badge.label}</span>
                    <span style={{
                      fontSize: 10, color: 'var(--text-very-muted)', fontFamily: 'var(--font-mono)',
                    }}>
                      {isRunning ? 'running…' : formatDuration(run.startedAt, run.endedAt)}
                    </span>
                  </div>
                </div>
              </button>
            )
          })}
        </div>

        {/* Right column — Log Viewer (flex-grow, dark surface) */}
        <div
          ref={logRef}
          style={{ flex: 1, overflowY: 'auto', padding: 16, background: 'var(--bg-terminal)' }}
          className="canvas-scroll"
        >
          {activeRun
            ? logOutput
              ? <pre
                  style={{
                    fontFamily: 'var(--font-mono)', fontSize: 11.5,
                    color: 'var(--text-terminal)', whiteSpace: 'pre-wrap',
                    lineHeight: 1.65, margin: 0,
                  }}
                  dangerouslySetInnerHTML={{ __html: logHtml }}
                />
              : <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'rgba(200,196,188,0.4)' }}>
                  Waiting for output…
                </span>
            : <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'rgba(200,196,188,0.4)' }}>
                Select a run to view its log
              </span>
          }
        </div>
      </div>
    </div>
  )
}
```

**What changed from the original `RunDetail.tsx`:**

| Aspect | Before | After |
|---|---|---|
| Run selection | Horizontal tab bar `#3 #2 #1` (up to 10 tabs) | Vertical 280px scrollable list with rich rows |
| Run row info | Just a number `#{runs.length - i}` | Formatted timestamp, trigger badge, status dot, duration |
| Status dot | No per-run status dots | Color-coded dot (green/red/amber) with pulsing keyframe for `running` |
| Header status badge | Inline `statusStyle(activeRun.status)` badge | Removed from header (status visible per-row instead) |
| Log area | Full width below tabs | Right column, flex-grow |
| Log auto-scroll | None | `useEffect` scrolls `logRef` to bottom on `logOutput` change |
| Empty state | "No output yet — click ▶ Run Now" | Left: "No runs yet…", Right: "Select a run to view its log" |
| `useRunStream` | Used identically | No change — same hook, same SSE connection |
| `ansi-to-html` | Used identically | No change |
| Trigger badge | Not shown | Derived from `job.trigger.type`: `cron`/`loop` → "Scheduled" (teal `#14b8a6`), else → "Manual" (gray) |
| Duration | Not shown | Calculated from `startedAt` / `endedAt`, formatted as "1m 23s" |

**Build check:** `cd ui && npm run build` — Expected: no TypeScript errors

**Commit:** `git add -A && git commit -m "feat(jobs): replace run tabs with master-detail split panel"`

---

### Task 7: Checkpoint B — master-detail run history

**Browser verification:**

```
agent-browser open http://localhost:7700
agent-browser snapshot -ic
# Click the Jobs tab
agent-browser click @eN
agent-browser snapshot -ic
# Click a job in the left sidebar to select it
agent-browser click @eN       # (a job card)
agent-browser snapshot -ic
agent-browser screenshot /tmp/phase3-checkpoint-b.png
agent-browser close
```

**Expected results:**
- After selecting a job, the right panel shows a two-column split
- Left column (~280px): scrollable run list, each row has a formatted timestamp ("Apr 13 · 3:42 PM"), a teal "SCHEDULED" or gray "MANUAL" badge, a colored status dot, and a duration
- Right column: dark terminal-like surface (`--bg-terminal`), showing the log for the selected (first) run
- First run is auto-selected when a job is clicked
- Clicking a different run row switches the log viewer to that run's output
- No horizontal tabs remain — they have been fully replaced by the vertical run list

---

### Task 8: Checkpoint C — SSE streaming for in-progress run

**Browser verification:**

```
agent-browser open http://localhost:7700
agent-browser snapshot -ic
# Click the Jobs tab, then select a job
agent-browser click @eN       # Jobs tab
agent-browser snapshot -ic
agent-browser click @eN       # A job card
agent-browser snapshot -ic
# Click "▶ Run Now" to trigger a live run
agent-browser click @eN       # The Run Now button
# Wait 2 seconds for the run to appear
agent-browser snapshot -ic
agent-browser screenshot /tmp/phase3-checkpoint-c.png
agent-browser close
```

**Expected results:**
- After clicking "▶ Run Now", a new run appears at the top of the run list within ~1 second
- The new run's status dot is amber and pulsing (CSS `pulse-dot` animation)
- The new run shows "running…" instead of a duration
- The right-panel log viewer streams live output (text appearing incrementally via SSE)
- The log auto-scrolls to the bottom as new chunks arrive
- When the run completes, the status dot changes to green (success) or red (failed), the pulsing stops, and a duration appears