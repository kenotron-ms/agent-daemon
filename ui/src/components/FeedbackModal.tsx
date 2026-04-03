import { useState } from 'react'
import { submitFeedback } from '../api/feedback'

interface Props { onClose: () => void }
type State = 'idle' | 'submitting' | 'done' | 'error'

export default function FeedbackModal({ onClose }: Props) {
  const [title, setTitle]     = useState('')
  const [body, setBody]       = useState('')
  const [state, setState]     = useState<State>('idle')
  const [issueUrl, setIssueUrl] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  const canSubmit = title.trim().length > 0 && state === 'idle'

  async function handleSubmit() {
    if (!canSubmit) return
    setState('submitting')
    try {
      const result = await submitFeedback({ title: title.trim(), body: body.trim() })
      setIssueUrl(result.url)
      setState('done')
    } catch (e: unknown) {
      setErrorMsg((e as Error).message)
      setState('error')
    }
  }

  const inputStyle: React.CSSProperties = {
    width: '100%',
    padding: '7px 10px',
    fontSize: 13,
    background: 'var(--bg-input)',
    border: '1px solid var(--border)',
    borderRadius: 3,
    color: 'var(--text-primary)',
    outline: 'none',
    fontFamily: 'var(--font-ui)',
    opacity: state === 'submitting' ? 0.5 : 1,
  }

  const labelStyle: React.CSSProperties = {
    display: 'block',
    fontSize: 10,
    fontWeight: 600,
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    color: 'var(--text-very-muted)',
    marginBottom: 6,
  }

  return (
    <div
      style={{
        position: 'fixed', inset: 0,
        background: 'rgba(20,16,10,0.18)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        zIndex: 50,
      }}
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
    >
      <div style={{
        background: 'var(--bg-modal)',
        border: '1px solid var(--border)',
        borderRadius: 6,
        padding: 24,
        width: 400,
        boxShadow: '0 8px 24px rgba(0,0,0,0.12)',
      }}>
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          marginBottom: 16,
        }}>
          <h3 style={{ fontSize: 16, fontWeight: 600, color: 'var(--text-primary)', margin: 0 }}>
            Send Feedback
          </h3>
          <button
            onClick={onClose}
            style={{
              fontSize: 18, color: 'var(--text-very-muted)',
              background: 'none', border: 'none', cursor: 'pointer', lineHeight: 1,
            }}
            onMouseEnter={e => (e.currentTarget as HTMLElement).style.color = 'var(--text-muted)'}
            onMouseLeave={e => (e.currentTarget as HTMLElement).style.color = 'var(--text-very-muted)'}
            aria-label="Close"
          >×</button>
        </div>

        <div style={{ borderTop: '1px solid var(--border)', marginBottom: 20 }} />

        {state === 'done' ? (
          <div style={{ textAlign: 'center', padding: '16px 0' }}>
            <div style={{ fontSize: 28, color: 'var(--green)', marginBottom: 10 }}>✓</div>
            <p style={{ fontSize: 13, color: 'var(--text-primary)', marginBottom: 6 }}>Issue filed!</p>
            <a
              href={issueUrl}
              target="_blank"
              rel="noopener noreferrer"
              style={{ fontSize: 11, color: 'var(--amber)', wordBreak: 'break-all' }}
            >{issueUrl}</a>
            <div style={{ marginTop: 20 }}>
              <button
                onClick={onClose}
                style={{
                  padding: '7px 16px', fontSize: 13,
                  background: 'var(--bg-pane-title)',
                  border: '1px solid var(--border)',
                  borderRadius: 3, cursor: 'pointer',
                  color: 'var(--text-primary)',
                }}
              >Close</button>
            </div>
          </div>
        ) : (
          <>
            <div style={{ marginBottom: 14 }}>
              <label style={labelStyle}>
                Title <span style={{ color: 'var(--red)' }}>*</span>
              </label>
              <input
                autoFocus
                value={title}
                onChange={e => setTitle(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && handleSubmit()}
                disabled={state === 'submitting'}
                placeholder="Short description of the issue or idea"
                style={inputStyle}
                onFocus={e => (e.currentTarget as HTMLElement).style.borderColor = 'var(--amber)'}
                onBlur={e => (e.currentTarget as HTMLElement).style.borderColor = 'var(--border)'}
              />
            </div>

            <div style={{ marginBottom: 20 }}>
              <label style={labelStyle}>Details</label>
              <textarea
                value={body}
                onChange={e => setBody(e.target.value)}
                disabled={state === 'submitting'}
                rows={5}
                placeholder="Steps to reproduce, expected vs actual behavior, etc."
                style={{ ...inputStyle, resize: 'none' }}
                onFocus={e => (e.currentTarget as HTMLElement).style.borderColor = 'var(--amber)'}
                onBlur={e => (e.currentTarget as HTMLElement).style.borderColor = 'var(--border)'}
              />
            </div>

            {state === 'error' && (
              <div style={{
                fontSize: 11, color: 'var(--red)',
                fontFamily: 'var(--font-mono)',
                marginBottom: 14,
              }}>{errorMsg}</div>
            )}

            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
              <button
                onClick={onClose}
                disabled={state === 'submitting'}
                style={{
                  padding: '7px 14px', fontSize: 13,
                  color: 'var(--text-muted)',
                  background: 'none', border: 'none', cursor: 'pointer',
                  opacity: state === 'submitting' ? 0.4 : 1,
                }}
              >Cancel</button>
              <button
                onClick={handleSubmit}
                disabled={!canSubmit}
                style={{
                  padding: '7px 16px', fontSize: 13,
                  background: 'var(--bg-modal)',
                  border: '1px solid var(--border-dark)',
                  borderRadius: 4,
                  color: 'var(--text-primary)',
                  cursor: canSubmit ? 'pointer' : 'default',
                  opacity: canSubmit ? 1 : 0.4,
                }}
              >{state === 'submitting' ? 'Filing…' : 'Submit Issue'}</button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
