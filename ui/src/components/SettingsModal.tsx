import { useEffect, useState } from 'react'

const TERMINALS = ['Ghostty', 'Terminal.app', 'iTerm2', 'Warp']
const PROVIDERS = ['anthropic', 'openai'] as const
type Provider = (typeof PROVIDERS)[number]

interface SettingsData {
  aiProvider: Provider
  anthropicKeySet: boolean
  anthropicModel: string
  openAIKeySet: boolean
  openAIModel: string
  aiConfigured: boolean
  preferredTerminal: string
}

interface Props { onClose: () => void }

export default function SettingsModal({ onClose }: Props) {
  // Loaded from server
  const [data, setData] = useState<SettingsData | null>(null)

  // Editable fields
  const [provider, setProvider] = useState<Provider>('anthropic')
  const [anthropicKey, setAnthropicKey] = useState('')
  const [anthropicModel, setAnthropicModel] = useState('')
  const [openaiKey, setOpenaiKey] = useState('')
  const [openaiModel, setOpenaiModel] = useState('')
  const [preferredTerminal, setPreferredTerminal] = useState('Ghostty')

  // Status
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)

  useEffect(() => {
    fetch('/api/settings')
      .then(r => r.json())
      .then((d: SettingsData) => {
        setData(d)
        setProvider(d.aiProvider ?? 'anthropic')
        setAnthropicModel(d.anthropicModel ?? '')
        setOpenaiModel(d.openAIModel ?? '')
        setPreferredTerminal(d.preferredTerminal ?? 'Ghostty')
      })
      .catch(console.error)
  }, [])

  async function handleSave() {
    setSaving(true)
    setSaved(false)
    setTestResult(null)
    try {
      const body: Record<string, string | boolean> = {
        aiProvider: provider,
        anthropicModel,
        openAIModel: openaiModel,
        preferredTerminal,
      }
      if (anthropicKey) body.anthropicKey = anthropicKey
      if (openaiKey) body.openAIKey = openaiKey

      const resp = await fetch('/api/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const updated: SettingsData = await resp.json()
      setData(updated)
      setAnthropicKey('')
      setOpenaiKey('')
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (err) {
      console.error('Failed to save settings:', err)
    } finally {
      setSaving(false)
    }
  }

  async function handleTest() {
    setTesting(true)
    setTestResult(null)
    try {
      const resp = await fetch('/api/settings/test', { method: 'POST' })
      const result = await resp.json()
      setTestResult(result)
    } catch {
      setTestResult({ ok: false, message: 'Request failed' })
    } finally {
      setTesting(false)
    }
  }

  // ── Styles ────────────────────────────────────────────────────────────────

  const labelStyle: React.CSSProperties = {
    display: 'block',
    fontSize: 10,
    fontWeight: 600,
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    color: 'var(--text-very-muted)',
    marginBottom: 6,
  }

  const inputStyle: React.CSSProperties = {
    width: '100%',
    boxSizing: 'border-box',
    padding: '7px 10px',
    fontSize: 13,
    background: 'var(--bg-input)',
    border: '1px solid var(--border)',
    borderRadius: 4,
    color: 'var(--text-primary)',
    outline: 'none',
    fontFamily: 'var(--font-ui)',
  }

  const sectionStyle: React.CSSProperties = {
    marginBottom: 20,
  }

  const dividerStyle: React.CSSProperties = {
    borderTop: '1px solid var(--border)',
    marginBottom: 20,
  }

  const keyStatusStyle = (isSet: boolean): React.CSSProperties => ({
    display: 'inline-block',
    marginLeft: 8,
    fontSize: 10,
    fontWeight: 600,
    color: isSet ? 'var(--green, #4CAF74)' : 'var(--text-very-muted)',
  })

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <div
      style={{
        position: 'fixed', inset: 0,
        background: 'rgba(0,0,0,0.5)',
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
        width: 420,
        maxHeight: '85vh',
        overflowY: 'auto',
        boxShadow: '0 8px 32px rgba(0,0,0,0.5)',
      }}>
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          marginBottom: 16,
        }}>
          <h3 style={{ fontSize: 16, fontWeight: 600, color: 'var(--text-primary)', margin: 0 }}>
            Settings
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

        <div style={dividerStyle} />

        {/* ── AI Provider ─────────────────────────────────────────── */}
        <div style={sectionStyle}>
          <label style={labelStyle}>AI Provider</label>
          <select
            value={provider}
            onChange={e => { setProvider(e.target.value as Provider); setTestResult(null) }}
            style={inputStyle}
          >
            <option value="anthropic">Anthropic</option>
            <option value="openai">OpenAI</option>
          </select>
        </div>

        {/* ── Anthropic ───────────────────────────────────────────── */}
        {provider === 'anthropic' && (
          <>
            <div style={sectionStyle}>
              <label style={labelStyle}>
                Anthropic API Key
                {data && (
                  <span style={keyStatusStyle(data.anthropicKeySet)}>
                    {data.anthropicKeySet ? '● set' : '○ not set'}
                  </span>
                )}
              </label>
              <input
                style={inputStyle}
                type="password"
                value={anthropicKey}
                onChange={e => setAnthropicKey(e.target.value)}
                placeholder={data?.anthropicKeySet ? '(unchanged — enter to replace)' : 'sk-ant-…'}
                autoComplete="off"
              />
            </div>
            <div style={sectionStyle}>
              <label style={labelStyle}>Anthropic Model</label>
              <input
                style={inputStyle}
                value={anthropicModel}
                onChange={e => setAnthropicModel(e.target.value)}
                placeholder="e.g. claude-sonnet-4-5"
              />
            </div>
          </>
        )}

        {/* ── OpenAI ──────────────────────────────────────────────── */}
        {provider === 'openai' && (
          <>
            <div style={sectionStyle}>
              <label style={labelStyle}>
                OpenAI API Key
                {data && (
                  <span style={keyStatusStyle(data.openAIKeySet)}>
                    {data.openAIKeySet ? '● set' : '○ not set'}
                  </span>
                )}
              </label>
              <input
                style={inputStyle}
                type="password"
                value={openaiKey}
                onChange={e => setOpenaiKey(e.target.value)}
                placeholder={data?.openAIKeySet ? '(unchanged — enter to replace)' : 'sk-…'}
                autoComplete="off"
              />
            </div>
            <div style={sectionStyle}>
              <label style={labelStyle}>OpenAI Model</label>
              <input
                style={inputStyle}
                value={openaiModel}
                onChange={e => setOpenaiModel(e.target.value)}
                placeholder="e.g. gpt-4o"
              />
            </div>
          </>
        )}

        {/* Test result */}
        {testResult && (
          <div style={{
            marginBottom: 16,
            padding: '8px 12px',
            borderRadius: 4,
            fontSize: 12,
            background: testResult.ok ? 'rgba(76,175,74,0.1)' : 'rgba(229,115,115,0.1)',
            border: `1px solid ${testResult.ok ? 'rgba(76,175,74,0.3)' : 'rgba(229,115,115,0.3)'}`,
            color: testResult.ok ? 'var(--green, #4CAF74)' : '#e57373',
          }}>
            {testResult.ok ? '✓ ' : '✗ '}{testResult.message}
          </div>
        )}

        <div style={dividerStyle} />

        {/* ── Preferred Terminal ───────────────────────────────────── */}
        <div style={sectionStyle}>
          <label style={labelStyle}>Preferred Terminal</label>
          <select
            value={preferredTerminal}
            onChange={e => setPreferredTerminal(e.target.value)}
            style={inputStyle}
          >
            {TERMINALS.map(t => (
              <option key={t} value={t}>{t}</option>
            ))}
          </select>
        </div>

        {/* ── Actions ─────────────────────────────────────────────── */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8 }}>
          <button
            onClick={handleTest}
            disabled={testing || saving || !data?.aiConfigured && !anthropicKey && !openaiKey}
            style={{
              padding: '7px 14px', fontSize: 12,
              background: 'none',
              border: '1px solid var(--border)',
              borderRadius: 4, cursor: 'pointer',
              color: 'var(--text-muted)',
              opacity: testing ? 0.6 : 1,
            }}
          >
            {testing ? 'Testing…' : 'Test connection'}
          </button>

          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            {saved && (
              <span style={{ fontSize: 11, color: 'var(--green, #4CAF74)' }}>✓ Saved</span>
            )}
            <button
              onClick={onClose}
              style={{
                padding: '7px 16px', fontSize: 13,
                background: 'var(--bg-pane-title)',
                border: '1px solid var(--border)',
                borderRadius: 4, cursor: 'pointer',
                color: 'var(--text-primary)',
              }}
            >Cancel</button>
            <button
              onClick={handleSave}
              disabled={saving}
              style={{
                padding: '7px 16px', fontSize: 13,
                background: 'var(--accent, #4CAF74)',
                border: 'none',
                borderRadius: 4, cursor: saving ? 'not-allowed' : 'pointer',
                color: '#fff',
                fontWeight: 600,
                opacity: saving ? 0.7 : 1,
              }}
            >
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
