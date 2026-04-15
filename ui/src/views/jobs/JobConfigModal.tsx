import { useEffect, useRef, useState } from 'react'
import { Job, getJob, updateJob } from '../../api/jobs'

interface Props {
  job: Job
  onClose: () => void
  onSaved: (updated: Job) => void
}

export default function JobConfigModal({ job, onClose, onSaved }: Props) {
  const [raw, setRaw]       = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving]  = useState(false)
  const [error, setError]    = useState<string | null>(null)
  const textaRef = useRef<HTMLTextAreaElement>(null)

  // Fetch full job (includes amplifier/shell/claudeCode configs)
  useEffect(() => {
    setLoading(true)
    getJob(job.id)
      .then(full => {
        setRaw(JSON.stringify(full, null, 2))
        setLoading(false)
        setTimeout(() => textaRef.current?.focus(), 50)
      })
      .catch(e => { setError(String(e)); setLoading(false) })
  }, [job.id])

  const handleSave = async () => {
    setError(null)
    let parsed: Job
    try {
      parsed = JSON.parse(raw)
    } catch (e) {
      setError('Invalid JSON — ' + String(e))
      return
    }
    setSaving(true)
    try {
      const updated = await updateJob(job.id, parsed)
      onSaved(updated)
    } catch (e) {
      setError(String(e))
      setSaving(false)
    }
  }

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <>
      {/* Backdrop */}
      <div
        onClick={onClose}
        style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.55)', zIndex: 1000 }}
      />

      {/* Modal */}
      <div style={{
        position: 'fixed', top: '50%', left: '50%',
        transform: 'translate(-50%, -50%)',
        width: 700, maxHeight: '82vh',
        display: 'flex', flexDirection: 'column',
        background: 'var(--bg-modal)',
        border: '1px solid var(--border-dark)',
        borderRadius: 6, zIndex: 1001,
        boxShadow: '0 24px 64px rgba(0,0,0,0.55)',
      }}>
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center',
          padding: '0 16px', height: 40, flexShrink: 0,
          borderBottom: '1px solid var(--border)',
        }}>
          <span style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-primary)' }}>
            Edit — {job.name}
          </span>
          <button onClick={onClose} style={{
            marginLeft: 'auto', fontSize: 18, lineHeight: 1,
            background: 'none', border: 'none',
            color: 'var(--text-muted)', cursor: 'pointer', padding: '0 2px',
          }}>×</button>
        </div>

        {/* Body */}
        <div style={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column', padding: 12, gap: 10 }}>
          {loading ? (
            <span style={{ fontSize: 11, color: 'var(--text-muted)', padding: 4 }}>Loading…</span>
          ) : (
            <textarea
              ref={textaRef}
              value={raw}
              onChange={e => { setRaw(e.target.value); setError(null) }}
              spellCheck={false}
              style={{
                flex: 1,
                fontFamily: 'var(--font-mono)', fontSize: 11.5, lineHeight: 1.65,
                background: 'var(--bg-terminal)', color: 'var(--text-terminal)',
                border: error ? '1px solid var(--red)' : '1px solid var(--border)',
                borderRadius: 4, padding: '10px 12px', resize: 'none', outline: 'none',
              }}
            />
          )}
          {error && (
            <div style={{
              fontSize: 11, color: 'var(--red)', flexShrink: 0,
              padding: '6px 10px', background: 'rgba(239,68,68,0.08)',
              borderRadius: 4, border: '1px solid rgba(239,68,68,0.2)',
            }}>{error}</div>
          )}
        </div>

        {/* Footer */}
        <div style={{
          display: 'flex', justifyContent: 'flex-end', alignItems: 'center', gap: 8,
          padding: '10px 16px', flexShrink: 0,
          borderTop: '1px solid var(--border)',
        }}>
          <span style={{ fontSize: 10, color: 'var(--text-very-muted)', marginRight: 'auto' }}>
            Full JSON — save patches only the fields you change
          </span>
          <button onClick={onClose} style={{
            fontSize: 11, padding: '5px 14px',
            background: 'transparent', border: '1px solid var(--border-dark)',
            borderRadius: 3, color: 'var(--text-muted)', cursor: 'pointer',
          }}>Cancel</button>
          <button
            onClick={handleSave}
            disabled={saving || loading}
            style={{
              fontSize: 11, padding: '5px 14px',
              background: saving ? 'var(--bg-pane-title)' : 'var(--amber)',
              border: 'none', borderRadius: 3,
              color: saving ? 'var(--text-muted)' : '#000',
              cursor: saving ? 'not-allowed' : 'pointer',
              fontWeight: 500,
            }}
          >{saving ? 'Saving…' : 'Save'}</button>
        </div>
      </div>
    </>
  )
}