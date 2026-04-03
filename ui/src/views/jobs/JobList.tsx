import { Job } from '../../api/jobs'

interface Props {
  jobs: Job[]
  selectedId: string | null
  onSelect: (id: string) => void
  onNew: () => void
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

export default function JobList({ jobs, selectedId, onSelect, onNew }: Props) {
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
          <button
            key={job.id}
            onClick={() => onSelect(job.id)}
            style={{
              width: '100%', textAlign: 'left',
              padding: '7px 12px 7px 14px',
              display: 'flex', alignItems: 'flex-start', gap: 8,
              background: selectedId === job.id ? 'var(--bg-sidebar-active)' : 'transparent',
              borderLeft: selectedId === job.id ? '2px solid var(--amber)' : '2px solid transparent',
              borderBottom: '1px solid var(--border)',
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
