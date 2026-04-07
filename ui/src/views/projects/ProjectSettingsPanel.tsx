// ui/src/views/projects/ProjectSettingsPanel.tsx
import { useEffect, useRef, useState } from 'react'
import {
  type AppBundle,
  listBundles,
} from '../../api/bundles'
import {
  type ProjectSettings,
  getProjectSettings,
  updateProjectSettings,
} from '../../api/projects'

interface Props {
  projectId: string
}

// ── Collapsible section wrapper ───────────────────────────────────────────────

function Section({
  title,
  summary,
  children,
  defaultOpen = false,
}: {
  title: string
  summary?: string
  children: React.ReactNode
  defaultOpen?: boolean
}) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div style={{ borderBottom: '1px solid var(--border)' }}>
      <button
        onClick={() => setOpen((v) => !v)}
        style={{
          display: 'flex',
          alignItems: 'center',
          width: '100%',
          padding: '8px 12px',
          background: 'none',
          border: 'none',
          cursor: 'pointer',
          gap: 6,
        }}
      >
        <span
          style={{
            fontSize: 9,
            fontWeight: 700,
            letterSpacing: '0.08em',
            color: 'var(--text-very-muted)',
            textTransform: 'uppercase',
            flex: 1,
            textAlign: 'left',
          }}
        >
          {title}
        </span>
        {summary && !open && (
          <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>{summary}</span>
        )}
        <span style={{ fontSize: 10, color: 'var(--text-very-muted)', marginLeft: 4 }}>
          {open ? '▼' : '▶'}
        </span>
      </button>
      {open && (
        <div style={{ padding: '0 12px 12px 12px' }}>{children}</div>
      )}
    </div>
  )
}

// ── Pill toggle ───────────────────────────────────────────────────────────────

function Toggle({ value, onChange }: { value: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      onClick={() => onChange(!value)}
      style={{
        width: 28,
        height: 16,
        borderRadius: 9999,
        background: value ? '#4CAF74' : '#E8E0D4',
        border: 'none',
        cursor: 'pointer',
        position: 'relative',
        flexShrink: 0,
        transition: 'background 150ms',
      }}
    >
      <div
        style={{
          position: 'absolute',
          top: 2,
          width: 12,
          height: 12,
          borderRadius: '50%',
          background: '#fff',
          transition: 'left 150ms',
          left: value ? 14 : 2,
        }}
      />
    </button>
  )
}

// ── Field label ───────────────────────────────────────────────────────────────

function FieldLabel({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        fontSize: 10,
        fontWeight: 600,
        letterSpacing: '0.06em',
        textTransform: 'uppercase',
        color: 'var(--text-very-muted)',
        marginBottom: 4,
        marginTop: 8,
      }}
    >
      {children}
    </div>
  )
}

// ── Shared input style ────────────────────────────────────────────────────────

const inputStyle: React.CSSProperties = {
  width: '100%',
  boxSizing: 'border-box',
  padding: '4px 8px',
  fontSize: 12,
  background: 'var(--bg-input)',
  border: '1px solid var(--border)',
  borderRadius: 2,
  color: 'var(--text-primary)',
  fontFamily: 'monospace',
  outline: 'none',
}

// ── Bundle Section ────────────────────────────────────────────────────────────
//
// SCOPE SEMANTICS:
//   bundle.app is a LIST that REPLACES across scopes — it does NOT merge.
//   - No project override → project inherits the global bundle.app list entirely.
//   - Project override present → project's bundle.app completely replaces global.
//
// UI:
//   Each bundle row shows:
//     [GLOBAL dot] = amber if globally enabled (AppBundle.enabled)
//     [PROJECT toggle] = green if in project list (or mirrors global when no override)
//   Banner shows "Inheriting global" or "Project override active" + Reset button.

function BundleSection({
  settings,
  appBundles,
  onChange,
}: {
  settings: ProjectSettings
  appBundles: AppBundle[]
  onChange: (s: ProjectSettings) => void
}) {
  const bundle = settings.bundle ?? {}
  const projectAppSpecs: string[] | undefined = bundle.app

  function setActive(active: string) {
    onChange({ ...settings, bundle: { ...bundle, active: active || undefined } })
  }

  function toggleProjectBundle(installSpec: string) {
    const baseline =
      projectAppSpecs ?? appBundles.filter((b) => b.enabled).map((b) => b.installSpec)
    const isCurrentlyOn = baseline.includes(installSpec)
    const next = isCurrentlyOn
      ? baseline.filter((s) => s !== installSpec)
      : [...baseline, installSpec]
    onChange({ ...settings, bundle: { ...bundle, app: next } })
  }

  function resetToGlobal() {
    const { app: _removed, ...rest } = bundle
    onChange({ ...settings, bundle: Object.keys(rest).length ? rest : undefined })
  }

  const hasProjectOverride = projectAppSpecs !== undefined

  return (
    <Section title="Bundle" defaultOpen>
      <FieldLabel>Active bundle</FieldLabel>
      <select
        value={bundle.active ?? ''}
        onChange={(e) => setActive(e.target.value)}
        style={{ ...inputStyle, fontFamily: 'monospace' }}
      >
        <option value="">(default — foundation)</option>
        {appBundles.map((b) => (
          <option key={b.id} value={b.name}>
            {b.name}
          </option>
        ))}
      </select>
      {bundle.active && (
        <div
          style={{
            fontSize: 10,
            color: 'var(--text-very-muted)',
            marginTop: 4,
            fontFamily: 'monospace',
            wordBreak: 'break-all',
          }}
        >
          {appBundles.find((b) => b.name === bundle.active)?.installSpec ?? bundle.active}
        </div>
      )}

      <FieldLabel>App bundles</FieldLabel>

      {/* Scope banner */}
      <div
        style={{
          fontSize: 10,
          color: hasProjectOverride ? 'var(--text-primary)' : 'var(--text-muted)',
          background: hasProjectOverride ? 'rgba(245,158,11,0.08)' : 'transparent',
          border: hasProjectOverride
            ? '1px solid rgba(245,158,11,0.25)'
            : '1px solid var(--border)',
          borderRadius: 2,
          padding: '4px 8px',
          marginBottom: 8,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 8,
        }}
      >
        <span>
          {hasProjectOverride
            ? '⚡ Project override active — replaces global list'
            : '↳ Inheriting global selections — toggle to override'}
        </span>
        {hasProjectOverride && (
          <button
            onClick={resetToGlobal}
            style={{
              fontSize: 10,
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--text-muted)',
              padding: 0,
              textDecoration: 'underline',
            }}
          >
            Reset to global
          </button>
        )}
      </div>

      {appBundles.length === 0 && (
        <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>
          No app bundles installed.
        </div>
      )}

      {/* Column headers */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '2px 0 4px',
          borderBottom: '1px solid var(--border)',
        }}
      >
        <span
          style={{
            fontSize: 9,
            color: 'var(--text-very-muted)',
            width: 28,
            textAlign: 'center',
            letterSpacing: '0.06em',
          }}
        >
          GLOBAL
        </span>
        <span
          style={{
            fontSize: 9,
            color: 'var(--text-very-muted)',
            width: 28,
            textAlign: 'center',
            letterSpacing: '0.06em',
          }}
        >
          PROJECT
        </span>
        <span
          style={{ fontSize: 9, color: 'var(--text-very-muted)', letterSpacing: '0.06em' }}
        >
          BUNDLE
        </span>
      </div>

      {appBundles.map((b) => {
        const projectOn =
          projectAppSpecs !== undefined ? projectAppSpecs.includes(b.installSpec) : b.enabled
        return (
          <div
            key={b.id}
            style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '4px 0' }}
          >
            {/* Global indicator — read-only amber/sand dot */}
            <div
              title={b.enabled ? 'Globally enabled' : 'Not in global list'}
              style={{ width: 28, display: 'flex', justifyContent: 'center' }}
            >
              <div
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  background: b.enabled ? '#F59E0B' : '#E8E0D4',
                  border: b.enabled ? 'none' : '1px solid #D0C8BC',
                }}
              />
            </div>

            {/* Project toggle */}
            <div
              style={{
                width: 28,
                display: 'flex',
                justifyContent: 'center',
                opacity: hasProjectOverride ? 1 : 0.5,
              }}
            >
              <Toggle value={projectOn} onChange={() => toggleProjectBundle(b.installSpec)} />
            </div>

            <span
              style={{
                fontSize: 12,
                fontFamily: 'monospace',
                color: 'var(--text-primary)',
                flex: 1,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {b.name}
            </span>
          </div>
        )
      })}
    </Section>
  )
}

// ── Providers Section ─────────────────────────────────────────────────────────

function ProvidersSection({
  settings,
  onChange,
}: {
  settings: ProjectSettings
  onChange: (s: ProjectSettings) => void
}) {
  const providers = settings.config?.providers ?? []

  function updateProvider(index: number, key: string, value: string) {
    const next = providers.map((p, i) => {
      if (i !== index) return p
      const config = { ...(p.config ?? {}) }
      if (value) config[key] = value
      else delete config[key]
      return { ...p, config }
    })
    onChange({ ...settings, config: { ...(settings.config ?? {}), providers: next } })
  }

  const summary = providers.length > 0 ? `${providers.length} configured` : 'none'

  return (
    <Section title="Providers" summary={summary}>
      {providers.length === 0 && (
        <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>
          No providers configured at project scope. Project inherits global provider settings.
        </div>
      )}
      {providers.map((p, i) => (
        <div key={i} style={{ marginBottom: 12 }}>
          <div
            style={{
              fontSize: 12,
              fontFamily: 'monospace',
              color: 'var(--text-primary)',
              fontWeight: 600,
              marginBottom: 4,
            }}
          >
            {p.module}
          </div>
          <FieldLabel>Default model</FieldLabel>
          <input
            style={inputStyle}
            value={(p.config?.['default_model'] as string) ?? ''}
            onChange={(e) => updateProvider(i, 'default_model', e.target.value)}
            placeholder="e.g. claude-sonnet-4-6"
          />
          <FieldLabel>API key override</FieldLabel>
          <input
            style={inputStyle}
            type="password"
            value={(p.config?.['api_key'] as string) ?? ''}
            onChange={(e) => updateProvider(i, 'api_key', e.target.value)}
            placeholder="${ENV_VAR} or literal key"
          />
        </div>
      ))}
    </Section>
  )
}

// ── Routing Section ───────────────────────────────────────────────────────────

function RoutingSection({
  settings,
  onChange,
}: {
  settings: ProjectSettings
  onChange: (s: ProjectSettings) => void
}) {
  const routing = settings.routing ?? {}
  const MATRICES = ['balanced', 'fast', 'quality', 'economy']

  return (
    <Section title="Routing" summary={routing.matrix ?? 'default'}>
      <FieldLabel>Matrix</FieldLabel>
      <select
        value={routing.matrix ?? ''}
        onChange={(e) =>
          onChange({
            ...settings,
            routing: { ...routing, matrix: e.target.value || undefined },
          })
        }
        style={inputStyle}
      >
        <option value="">(inherit global)</option>
        {MATRICES.map((m) => (
          <option key={m} value={m}>
            {m}
          </option>
        ))}
      </select>
    </Section>
  )
}

// ── Filesystem Section ────────────────────────────────────────────────────────

function FilesystemSection({
  settings,
  onChange,
}: {
  settings: ProjectSettings
  onChange: (s: ProjectSettings) => void
}) {
  const fsTool = settings.modules?.tools?.find((t) => t.module === 'tool-filesystem')
  const cfg = fsTool?.config ?? {}
  const writePaths = cfg.allowed_write_paths ?? []
  const readPaths = cfg.allowed_read_paths ?? []
  const deniedPaths = cfg.denied_write_paths ?? []
  const totalPaths = writePaths.length + readPaths.length + deniedPaths.length
  const summary = totalPaths > 0 ? `${totalPaths} path${totalPaths !== 1 ? 's' : ''}` : 'default'

  function updateFsPaths(
    field: 'allowed_write_paths' | 'allowed_read_paths' | 'denied_write_paths',
    raw: string,
  ) {
    const paths = raw
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean)
    const newCfg = { ...cfg, [field]: paths.length ? paths : undefined }
    const tools = (settings.modules?.tools ?? []).filter((t) => t.module !== 'tool-filesystem')
    tools.push({ module: 'tool-filesystem', config: newCfg })
    onChange({ ...settings, modules: { ...(settings.modules ?? {}), tools } })
  }

  return (
    <Section title="Filesystem" summary={summary}>
      <FieldLabel>Allowed write paths</FieldLabel>
      <textarea
        style={{ ...inputStyle, height: 60, resize: 'vertical' }}
        value={writePaths.join('\n')}
        onChange={(e) => updateFsPaths('allowed_write_paths', e.target.value)}
        placeholder="One path per line"
      />
      <FieldLabel>Allowed read paths</FieldLabel>
      <textarea
        style={{ ...inputStyle, height: 60, resize: 'vertical' }}
        value={readPaths.join('\n')}
        onChange={(e) => updateFsPaths('allowed_read_paths', e.target.value)}
        placeholder="One path per line"
      />
      <FieldLabel>Denied write paths</FieldLabel>
      <textarea
        style={{ ...inputStyle, height: 60, resize: 'vertical' }}
        value={deniedPaths.join('\n')}
        onChange={(e) => updateFsPaths('denied_write_paths', e.target.value)}
        placeholder="One path per line"
      />
    </Section>
  )
}

// ── Notifications Section ─────────────────────────────────────────────────────

function NotificationsSection({
  settings,
  onChange,
}: {
  settings: ProjectSettings
  onChange: (s: ProjectSettings) => void
}) {
  const desktop = settings.config?.notifications?.desktop ?? {}
  const enabled = desktop.enabled !== false

  function setEnabled(v: boolean) {
    onChange({
      ...settings,
      config: {
        ...(settings.config ?? {}),
        notifications: {
          ...(settings.config?.notifications ?? {}),
          desktop: { ...desktop, enabled: v },
        },
      },
    })
  }

  function setField(field: keyof typeof desktop, value: unknown) {
    onChange({
      ...settings,
      config: {
        ...(settings.config ?? {}),
        notifications: {
          ...(settings.config?.notifications ?? {}),
          desktop: { ...desktop, [field]: value || undefined },
        },
      },
    })
  }

  const summary = enabled ? 'on' : 'off'

  return (
    <Section title="Notifications" summary={summary}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
        <Toggle value={enabled} onChange={setEnabled} />
        <span style={{ fontSize: 12, color: 'var(--text-primary)' }}>Desktop notifications</span>
      </div>
      {enabled && (
        <>
          <FieldLabel>Min iterations before notifying</FieldLabel>
          <input
            style={inputStyle}
            type="number"
            min={0}
            value={desktop.min_iterations ?? ''}
            onChange={(e) =>
              setField('min_iterations', e.target.value ? Number(e.target.value) : undefined)
            }
            placeholder="(inherit)"
          />
          <FieldLabel>Sound</FieldLabel>
          <input
            style={inputStyle}
            value={desktop.sound ?? ''}
            onChange={(e) => setField('sound', e.target.value)}
            placeholder="e.g. Glass, Ping (macOS)"
          />
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 8 }}>
            <Toggle
              value={desktop.suppress_if_focused ?? false}
              onChange={(v) => setField('suppress_if_focused', v)}
            />
            <span style={{ fontSize: 12, color: 'var(--text-primary)' }}>
              Suppress when app is focused
            </span>
          </div>
        </>
      )}
    </Section>
  )
}

// ── Overrides Section ─────────────────────────────────────────────────────────

function OverridesSection({
  settings,
  onChange,
}: {
  settings: ProjectSettings
  onChange: (s: ProjectSettings) => void
}) {
  const overrides = settings.overrides ?? {}
  const count = Object.keys(overrides).length
  const summary = count > 0 ? `${count} override${count !== 1 ? 's' : ''}` : 'none'

  const [raw, setRaw] = useState(() => JSON.stringify(overrides, null, 2))
  const [parseError, setParseError] = useState<string | null>(null)

  function applyRaw() {
    try {
      const parsed = JSON.parse(raw)
      setParseError(null)
      onChange({ ...settings, overrides: parsed })
    } catch (e) {
      setParseError(String(e))
    }
  }

  return (
    <Section title="Overrides" summary={summary}>
      <div style={{ fontSize: 10, color: 'var(--text-very-muted)', marginBottom: 6 }}>
        Per-module source and config overrides. Edit as JSON.
      </div>
      <textarea
        style={{ ...inputStyle, height: 120, resize: 'vertical', fontSize: 11 }}
        value={raw}
        onChange={(e) => setRaw(e.target.value)}
        onBlur={applyRaw}
        spellCheck={false}
      />
      {parseError && (
        <div style={{ fontSize: 10, color: '#e57373', marginTop: 4 }}>{parseError}</div>
      )}
    </Section>
  )
}

// ── Sources Section ───────────────────────────────────────────────────────────

function SourcesSection({
  settings,
  onChange,
}: {
  settings: ProjectSettings
  onChange: (s: ProjectSettings) => void
}) {
  const sources = settings.sources ?? {}
  const modCount = Object.keys(sources.modules ?? {}).length
  const summary =
    modCount > 0 ? `${modCount} module override${modCount !== 1 ? 's' : ''}` : 'none'

  const [raw, setRaw] = useState(() => JSON.stringify(sources, null, 2))
  const [parseError, setParseError] = useState<string | null>(null)

  function applyRaw() {
    try {
      const parsed = JSON.parse(raw)
      setParseError(null)
      onChange({ ...settings, sources: parsed })
    } catch (e) {
      setParseError(String(e))
    }
  }

  return (
    <Section title="Sources" summary={summary}>
      <div style={{ fontSize: 10, color: 'var(--text-very-muted)', marginBottom: 6 }}>
        Point modules at local checkouts for dev. Edit as JSON.
      </div>
      <textarea
        style={{ ...inputStyle, height: 80, resize: 'vertical', fontSize: 11 }}
        value={raw}
        onChange={(e) => setRaw(e.target.value)}
        onBlur={applyRaw}
        spellCheck={false}
      />
      {parseError && (
        <div style={{ fontSize: 10, color: '#e57373', marginTop: 4 }}>{parseError}</div>
      )}
    </Section>
  )
}

// ── Root panel ────────────────────────────────────────────────────────────────

export function ProjectSettingsPanel({ projectId }: Props) {
  const [settings, setSettings] = useState<ProjectSettings>({})
  const [appBundles, setAppBundles] = useState<AppBundle[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    setLoading(true)
    Promise.all([getProjectSettings(projectId), listBundles()])
      .then(([s, b]) => {
        setSettings(s)
        setAppBundles(b)
        setLoading(false)
      })
      .catch((e) => {
        setError(String(e))
        setLoading(false)
      })
  }, [projectId])

  // Debounced auto-save: 800 ms after last change
  function handleChange(next: ProjectSettings) {
    setSettings(next)
    if (saveTimer.current) clearTimeout(saveTimer.current)
    saveTimer.current = setTimeout(() => {
      setSaving(true)
      updateProjectSettings(projectId, next)
        .then(() => setSaving(false))
        .catch((e) => {
          setError(String(e))
          setSaving(false)
        })
    }, 800)
  }

  if (loading) {
    return (
      <div style={{ padding: 16, fontSize: 12, color: 'var(--text-muted)' }}>
        Loading settings…
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: 16, fontSize: 12, color: '#e57373' }}>
        {error}
      </div>
    )
  }

  return (
    <div
      style={{
        overflowY: 'auto',
        height: '100%',
        fontSize: 12,
        color: 'var(--text-primary)',
        background: 'var(--bg-panel)',
      }}
    >
      {saving && (
        <div
          style={{
            position: 'sticky',
            top: 0,
            background: 'var(--bg-input)',
            borderBottom: '1px solid var(--border)',
            padding: '4px 12px',
            fontSize: 10,
            color: 'var(--text-very-muted)',
          }}
        >
          Saving…
        </div>
      )}
      <BundleSection settings={settings} appBundles={appBundles} onChange={handleChange} />
      <ProvidersSection settings={settings} onChange={handleChange} />
      <RoutingSection settings={settings} onChange={handleChange} />
      <FilesystemSection settings={settings} onChange={handleChange} />
      <NotificationsSection settings={settings} onChange={handleChange} />
      <OverridesSection settings={settings} onChange={handleChange} />
      <SourcesSection settings={settings} onChange={handleChange} />
    </div>
  )
}
