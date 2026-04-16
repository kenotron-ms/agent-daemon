import { useEffect, useMemo, useRef, useState, useCallback } from 'react'
import Convert from 'ansi-to-html'
import { Job, JobRun, listJobRuns, triggerJob, cancelRun, deleteRun, clearJobRuns } from '../../api/jobs'
import { useRunStream } from './useRunStream'
import JobConfigModal from './JobConfigModal'

const ansiConvert = new Convert({ escapeXML: true, newline: false })

interface Props {
  job: Job
  onUpdate: (updated: Job) => void
}

function formatRunDate(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  const month = d.toLocaleDateString('en-US', { month: 'short' })
  const day = d.getDate()
  const year = d.getFullYear()
  const time = d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })
  const datePart = year !== now.getFullYear()
    ? `${month} ${day}, ${year}`
    : `${month} ${day}`
  return `${datePart} · ${time}`
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

function triggerBadge(run: JobRun, job: Job): { label: string; bg: string; color: string } {
  // Explicit source field wins (set on all new runs by the backend)
  if (run.source === 'manual')
    return { label: 'Manual', bg: 'var(--bg-pane-title)', color: 'var(--text-muted)' }
  if (run.source === 'scheduled')
    return { label: 'Scheduled', bg: '#14b8a6', color: '#fff' }
  // Fallback for old runs without a source field: infer from job trigger type
  if (job.trigger.type === 'cron' || job.trigger.type === 'loop')
    return { label: 'Scheduled', bg: '#14b8a6', color: '#fff' }
  return { label: 'Manual', bg: 'var(--bg-pane-title)', color: 'var(--text-muted)' }
}

export default function RunDetail({ job, onUpdate }: Props) {
  const [runs, setRuns]               = useState<JobRun[]>([])
  const [activeRunId, setActiveRunId] = useState<string | null>(null)
  const [editOpen, setEditOpen]       = useState(false)
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

  useEffect(() => {
    if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight
  }, [logOutput])

  const handleTrigger = async () => {
    try {
      await triggerJob(job.id)
      setTimeout(() => refreshRuns(job.id), 800)
    } catch (e) { console.error('triggerJob:', e) }
  }

  const handleCancel = async () => {
    if (!activeRunId) return
    try {
      await cancelRun(activeRunId)
      setTimeout(() => refreshRuns(job.id), 600)
    } catch (e) { console.error('cancelRun:', e) }
  }

  const [hoverRunId, setHoverRunId] = useState<string | null>(null)

  const handleDeleteRun = async (runId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await deleteRun(runId)
      const remaining = runs.filter(r => r.id !== runId)
      setRuns(remaining)
      if (activeRunId === runId) setActiveRunId(remaining[0]?.id ?? null)
    } catch (e) { console.error('deleteRun:', e) }
  }

  const handleClearAll = async () => {
    try {
      await clearJobRuns(job.id)
      setRuns([])
      setActiveRunId(null)
    } catch (e) { console.error('clearJobRuns:', e) }
  }

  const handleSaved = useCallback((updated: Job) => {
    setEditOpen(false)
    onUpdate(updated)
  }, [onUpdate])

  const activeRun = runs.find(r => r.id === activeRunId) ?? null

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: 'var(--bg-right)' }}>
      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 10,
        padding: '0 16px', height: 36,
        background: 'var(--bg-pane-title)', borderBottom: '1px solid var(--border)', flexShrink: 0,
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
      {activeRun?.status === 'running' && (
        <button
          onClick={handleCancel}
          style={{
            marginLeft: 8, fontSize: 11, padding: '4px 12px',
            background: 'var(--bg-modal)', border: '1px solid var(--border-dark)',
            borderRadius: 3, color: 'var(--red)', cursor: 'pointer',
          }}
          onMouseEnter={e => (e.currentTarget as HTMLElement).style.background = 'var(--bg-pane-title)'}
          onMouseLeave={e => (e.currentTarget as HTMLElement).style.background = 'var(--bg-modal)'}
        >⏹ Cancel</button>
      )}
      <button
        onClick={() => setEditOpen(true)}
        style={{
          marginLeft: 8, fontSize: 11, padding: '4px 12px',
          background: 'var(--bg-modal)', border: '1px solid var(--border-dark)',
          borderRadius: 3, color: 'var(--text-muted)', cursor: 'pointer',
        }}
          onMouseEnter={e => (e.currentTarget as HTMLElement).style.background = 'var(--bg-pane-title)'}
          onMouseLeave={e => (e.currentTarget as HTMLElement).style.background = 'var(--bg-modal)'}
      >⚙ Edit</button>
      </div>

    {editOpen && (
      <JobConfigModal
        job={job}
        onClose={() => setEditOpen(false)}
        onSaved={handleSaved}
      />
    )}

      {/* Master-detail split */}
      <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>

        {/* Left column — Run List (280px) */}
        <div style={{
          width: 280, flexShrink: 0, display: 'flex', flexDirection: 'column',
          borderRight: '1px solid var(--border)', background: 'var(--bg-right)',
        }}>
          {runs.length > 0 && (
            <div style={{
              display: 'flex', alignItems: 'center', justifyContent: 'flex-end',
              padding: '4px 10px', borderBottom: '1px solid var(--border)',
              background: 'var(--bg-pane-title)', flexShrink: 0,
            }}>
              <button
                onClick={handleClearAll}
                style={{
                  fontSize: 10, padding: '2px 8px',
                  background: 'transparent', border: '1px solid var(--border-dark)',
                  borderRadius: 3, color: 'var(--text-very-muted)', cursor: 'pointer',
                }}
                onMouseEnter={e => (e.currentTarget as HTMLElement).style.color = 'var(--red)'}
                onMouseLeave={e => (e.currentTarget as HTMLElement).style.color = 'var(--text-very-muted)'}
              >Clear All</button>
            </div>
          )}
          <div style={{ flex: 1, overflowY: 'auto' }} className="canvas-scroll">
          {runs.length === 0 && (
            <div style={{ padding: '24px 16px', fontSize: 11, color: 'var(--text-very-muted)', textAlign: 'center' }}>
              No runs yet — click ▶ Run Now to trigger a run.
            </div>
          )}
          {runs.map(run => {
            const badge        = triggerBadge(run, job)
            const isHovered    = hoverRunId === run.id
            const isActive     = activeRunId === run.id
            const isRunning    = run.status === 'running'
            const isSuccess    = run.status === 'success'
            const isCancelled  = run.status === 'cancelled'
            const isFailed     = run.status === 'failed' || run.status === 'timeout'
            return (
              <div
                key={run.id}
                style={{ position: 'relative' }}
                onMouseEnter={() => setHoverRunId(run.id)}
                onMouseLeave={() => setHoverRunId(null)}
              >
              <button
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
              >
                {/* Status dot */}
                <span style={{
                  width: 8, height: 8, borderRadius: '50%', marginTop: 4, flexShrink: 0,
                  background: isRunning   ? 'var(--amber)'
                    : isSuccess           ? 'var(--green)'
                    : isFailed            ? 'var(--red)'
                    : isCancelled         ? 'var(--text-muted)'
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
                    <span style={{ fontSize: 10, color: 'var(--text-very-muted)', fontFamily: 'var(--font-mono)' }}>
                      {isRunning ? 'running…' : isCancelled ? 'cancelled' : formatDuration(run.startedAt, run.endedAt)}
                    </span>
                  </div>
                </div>
              </button>
              {isHovered && run.status !== 'running' && (
                <button
                  onClick={e => handleDeleteRun(run.id, e)}
                  title="Delete this run"
                  style={{
                    position: 'absolute', top: 6, right: 8,
                    width: 18, height: 18, borderRadius: 3,
                    background: 'var(--bg-pane-title)', border: '1px solid var(--border-dark)',
                    color: 'var(--text-very-muted)', cursor: 'pointer',
                    fontSize: 11, lineHeight: '16px', padding: 0, display: 'flex',
                    alignItems: 'center', justifyContent: 'center',
                  }}
                  onMouseEnter={e => (e.currentTarget as HTMLElement).style.color = 'var(--red)'}
                  onMouseLeave={e => (e.currentTarget as HTMLElement).style.color = 'var(--text-very-muted)'}
                >×</button>
              )}
              </div>
            )
          })}
          </div>
        </div>

        {/* Right column — Log Viewer (flex-grow) */}
        <div
          ref={logRef}
          style={{ flex: 1, overflowY: 'auto', padding: 16, background: 'var(--bg-terminal)' }}
          className="canvas-scroll"
        >
          {activeRun
            ? logOutput
              ? <pre style={{
                  fontFamily: 'var(--font-mono)', fontSize: 11.5,
                  color: 'var(--text-terminal)', whiteSpace: 'pre-wrap',
                  lineHeight: 1.65, margin: 0,
                }} dangerouslySetInnerHTML={{ __html: logHtml }} />
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
