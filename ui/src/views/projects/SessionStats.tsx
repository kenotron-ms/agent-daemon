import { useEffect, useState } from 'react'
import { Session, SessionStats, getSessionStats } from '../../api/projects'

interface Props {
  project: { id: string; name: string; path: string }
  session: Session
}

function StatCell({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div style={{
      padding: 12,
      borderRight: '1px solid var(--border)',
      borderBottom: '1px solid var(--border)',
    }}>
      <div style={{
        fontSize: 10, fontWeight: 600,
        textTransform: 'uppercase', letterSpacing: '0.08em',
        color: 'var(--text-very-muted)', marginBottom: 6,
      }}>{label}</div>
      <div style={{
        fontSize: 22, fontWeight: 700,
        color: 'var(--text-primary)', lineHeight: 1.1,
      }}>{value}</div>
      {sub && (
        <div style={{ fontSize: 10, color: 'var(--text-very-muted)', marginTop: 4 }}>{sub}</div>
      )}
    </div>
  )
}

function SectionHeader({ label, open, onToggle }: { label: string; open: boolean; onToggle: () => void }) {
  return (
    <button
      onClick={onToggle}
      style={{
        width: '100%', textAlign: 'left',
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '8px 0',
        borderBottom: '1px solid var(--border)',
        background: 'none',
        cursor: 'pointer',
        marginBottom: open ? 10 : 0,
      }}
    >
      <span style={{
        fontSize: 10, fontWeight: 600,
        textTransform: 'uppercase', letterSpacing: '0.08em',
        color: 'var(--text-very-muted)',
      }}>{label}</span>
      <span style={{ fontSize: 9, color: 'var(--text-very-muted)' }}>
        {open ? '▾' : '▸'}
      </span>
    </button>
  )
}

export default function SessionStatsPanel({ project, session }: Props) {
  const [stats, setStats]   = useState<SessionStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError]   = useState<string | null>(null)
  const [openSections, setOpenSections] = useState({ session: true, amplifier: true, project: false })

  const toggleSection = (s: keyof typeof openSections) =>
    setOpenSections(prev => ({ ...prev, [s]: !prev[s] }))

  useEffect(() => {
    setLoading(true)
    setError(null)
    getSessionStats(project.id, session.id)
      .then(setStats)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [project.id, session.id])

  const created = new Date(session.createdAt * 1000)
  const age = Math.floor((Date.now() - created.getTime()) / 1000)
  const ageStr = age < 60 ? `${age}s`
    : age < 3600 ? `${Math.floor(age / 60)}m`
    : age < 86400 ? `${Math.floor(age / 3600)}h`
    : `${Math.floor(age / 86400)}d`

  return (
    <div style={{
      display: 'flex', flexDirection: 'column', height: '100%',
      background: 'var(--bg-right)',
      overflowY: 'auto',
    }} className="canvas-scroll">
      {/* Session summary */}
      <div style={{
        padding: '12px 16px',
        borderBottom: '1px solid var(--border)',
        flexShrink: 0,
      }}>
        <div style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-primary)', marginBottom: 4 }}>
          {session.name || 'Session'}
        </div>
        {session.worktreePath && (
          <div style={{
            fontFamily: 'var(--font-mono)', fontSize: 10,
            color: 'var(--text-very-muted)',
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          }}>{session.worktreePath}</div>
        )}
      </div>

      <div style={{ padding: '16px', flex: 1 }}>

        {/* Stats grid */}
        {stats && (
          <div style={{
            display: 'grid', gridTemplateColumns: '1fr 1fr',
            border: '1px solid var(--border)',
            borderRadius: 4,
            overflow: 'hidden',
            marginBottom: 16,
          }}>
            <StatCell
              label="Tokens"
              value={stats.tokens > 0 ? stats.tokens.toLocaleString() : '—'}
              sub={stats.tokens > 0 ? 'total context' : undefined}
            />
            <StatCell
              label="Tool calls"
              value={stats.tools > 0 ? stats.tools : '—'}
              sub={stats.turns ? `${stats.turns} turns` : undefined}
            />
            <StatCell label="Status" value={session.status || 'active'} />
            <StatCell label="Duration" value={ageStr} sub={created.toLocaleTimeString()} />
          </div>
        )}

        {loading && (
          <div style={{ fontSize: 11, color: 'var(--text-very-muted)', marginBottom: 16 }}>
            Loading stats…
          </div>
        )}
        {error && (
          <div style={{ fontSize: 11, color: 'var(--red)', marginBottom: 16 }}>{error}</div>
        )}

        {/* LLM section — model info */}
        {stats?.model && (
          <div style={{
            background: 'var(--amber-subtle)',
            border: '1px solid var(--amber-border)',
            borderRadius: 4,
            padding: '10px 12px',
            marginBottom: 16,
          }}>
            <div style={{
              display: 'flex', alignItems: 'center', gap: 6,
              marginBottom: 8,
            }}>
              <span style={{ color: 'var(--amber)', fontSize: 12 }}>✶</span>
              <span style={{
                fontSize: 10, fontWeight: 600,
                textTransform: 'uppercase', letterSpacing: '0.08em',
                color: 'var(--amber)',
              }}>LLM · Active</span>
            </div>
            <div style={{
              fontSize: 11, fontFamily: 'var(--font-mono)',
              color: 'var(--text-muted)',
            }}>{stats.model}</div>
          </div>
        )}

        {/* Project section */}
        <div style={{ marginBottom: 12 }}>
          <SectionHeader
            label="Project"
            open={openSections.project}
            onToggle={() => toggleSection('project')}
          />
          {openSections.project && (
            <div style={{ paddingTop: 8 }}>
              <div style={{
                display: 'flex', justifyContent: 'space-between',
                fontSize: 11, marginBottom: 6,
              }}>
                <span style={{ color: 'var(--text-muted)' }}>Name</span>
                <span style={{ color: 'var(--text-primary)', fontWeight: 500 }}>{project.name}</span>
              </div>
              <div style={{
                display: 'flex', justifyContent: 'space-between',
                fontSize: 11,
              }}>
                <span style={{ color: 'var(--text-muted)' }}>Path</span>
                <span style={{
                  fontFamily: 'var(--font-mono)', fontSize: 10,
                  color: 'var(--text-muted)',
                  overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                  maxWidth: '60%',
                  direction: 'rtl',
                }}>{project.path}</span>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
