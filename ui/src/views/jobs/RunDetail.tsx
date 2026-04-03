import { useEffect, useMemo, useState } from 'react'
import Convert from 'ansi-to-html'
import { Job, JobRun, listJobRuns, triggerJob } from '../../api/jobs'
import { useRunStream } from './useRunStream'

const ansiConvert = new Convert({ escapeXML: true, newline: false })

interface Props { job: Job }

export default function RunDetail({ job }: Props) {
  const [runs, setRuns]           = useState<JobRun[]>([])
  const [activeRunId, setActiveRunId] = useState<string | null>(null)
  const logOutput = useRunStream(activeRunId)
  const logHtml   = useMemo(() => ansiConvert.toHtml(logOutput), [logOutput])

  const refreshRuns = async (jobId: string) => {
    try {
      const rs = await listJobRuns(jobId)
      const safe = rs ?? []
      setRuns(safe)
      if (safe.length > 0) setActiveRunId(safe[0].id)
    } catch (e) { console.error('listJobRuns:', e) }
  }

  useEffect(() => { refreshRuns(job.id) }, [job.id])

  const handleTrigger = async () => {
    try {
      await triggerJob(job.id)
      setTimeout(() => refreshRuns(job.id), 800)
    } catch (e) { console.error('triggerJob:', e) }
  }

  const statusStyle = (status: string): React.CSSProperties => {
    const map: Record<string, React.CSSProperties> = {
      running:   { background: 'rgba(245,158,11,0.08)', color: 'var(--amber)', border: '1px solid rgba(245,158,11,0.20)' },
      succeeded: { background: 'rgba(76,175,116,0.08)', color: 'var(--green)', border: '1px solid rgba(76,175,116,0.20)' },
      failed:    { background: 'rgba(229,57,53,0.08)',  color: 'var(--red)',   border: '1px solid rgba(229,57,53,0.20)' },
      cancelled: { background: 'var(--bg-pane-title)',  color: 'var(--text-very-muted)', border: '1px solid var(--border)' },
    }
    return {
      ...(map[status] ?? map.cancelled),
      fontSize: 10, padding: '2px 7px', borderRadius: 3, fontFamily: 'var(--font-mono)',
      display: 'inline-block',
    }
  }

  const activeRun = runs.find(r => r.id === activeRunId) ?? null

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: 'var(--bg-right)' }}>
      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 10,
        padding: '0 16px',
        height: 36,
        background: 'var(--bg-pane-title)',
        borderBottom: '1px solid var(--border)',
        flexShrink: 0,
      }}>
        <span style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-primary)' }}>{job.name}</span>
        {activeRun && <span style={statusStyle(activeRun.status)}>{activeRun.status}</span>}
        <button
          onClick={handleTrigger}
          style={{
            marginLeft: 'auto',
            fontSize: 11, padding: '4px 12px',
            background: 'var(--bg-modal)',
            border: '1px solid var(--border-dark)',
            borderRadius: 3,
            color: 'var(--text-primary)',
            cursor: 'pointer',
          }}
          onMouseEnter={e => (e.currentTarget as HTMLElement).style.background = 'var(--bg-pane-title)'}
          onMouseLeave={e => (e.currentTarget as HTMLElement).style.background = 'var(--bg-modal)'}
        >▶ Run Now</button>
      </div>

      {/* Run history tabs */}
      {runs.length > 0 && (
        <div style={{
          display: 'flex', gap: 4,
          padding: '4px 12px',
          background: 'var(--bg-right)',
          borderBottom: '1px solid var(--border)',
          flexShrink: 0,
          overflowX: 'auto',
        }}>
          {runs.slice(0, 10).map((run, i) => (
            <button
              key={run.id}
              onClick={() => setActiveRunId(run.id)}
              style={{
                fontSize: 10, padding: '2px 8px',
                borderRadius: 3, flexShrink: 0,
                background: activeRunId === run.id ? 'var(--bg-pane-title)' : 'transparent',
                color: activeRunId === run.id ? 'var(--text-primary)' : 'var(--text-very-muted)',
                border: '1px solid transparent',
                borderColor: activeRunId === run.id ? 'var(--border)' : 'transparent',
                cursor: 'pointer',
              }}
            >#{runs.length - i}</button>
          ))}
        </div>
      )}

      {/* Log output */}
      <div style={{
        flex: 1, overflowY: 'auto',
        padding: 16,
        background: 'var(--bg-terminal)',
      }} className="canvas-scroll">
        {logOutput
          ? <pre
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: 11.5,
                color: 'var(--text-terminal)',
                whiteSpace: 'pre-wrap',
                lineHeight: 1.65,
                margin: 0,
              }}
              dangerouslySetInnerHTML={{ __html: logHtml }}
            />
          : <span style={{
              fontFamily: 'var(--font-mono)',
              fontSize: 11, color: 'rgba(200,196,188,0.4)',
            }}>No output yet — click ▶ Run Now to trigger a run.</span>
        }
      </div>
    </div>
  )
}
