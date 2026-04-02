import { useEffect, useState } from 'react'
import {
  RegistryEntry, AppBundle,
  fetchRegistry, listBundles, addBundle, removeBundle, toggleBundle,
} from '../../api/bundles'

// ── helpers ───────────────────────────────────────────────────────────────────

const TYPE_COLORS: Record<string, string> = {
  bundle: 'bg-[#388bfd]/20 text-[#58a6ff]',
  agent:  'bg-[#8957e5]/20 text-[#a371f7]',
  tool:   'bg-[#3fb950]/20 text-[#56d364]',
  module: 'bg-[#db6d28]/20 text-[#ffa657]',
}

function Stars({ rating }: { rating: number }) {
  const full    = Math.floor(rating)
  const hasHalf = rating - full >= 0.4
  return (
    <span className="flex items-center gap-0.5 text-[#e3b341] text-[10px]">
      {Array.from({ length: 5 }, (_, i) => {
        if (i < full)    return <span key={i}>★</span>
        if (i === full && hasHalf) return <span key={i} className="opacity-60">★</span>
        return <span key={i} className="text-[#30363d]">★</span>
      })}
      <span className="text-[#8b949e] ml-0.5">{rating.toFixed(1)}</span>
    </span>
  )
}

const CATEGORIES = ['all', 'dev', 'infra', 'knowledge', 'integration', 'research', 'ui']
const TYPES      = ['all', 'bundle', 'agent', 'tool', 'module']

// ── BundleCard ────────────────────────────────────────────────────────────────

function BundleCard({
  entry, installed, busy, onAdd, onRemove,
}: {
  entry: RegistryEntry
  installed: boolean
  busy: boolean
  onAdd: () => void
  onRemove: () => void
}) {
  return (
    <div className="flex flex-col bg-[#161b22] border border-[#30363d] rounded-lg p-3 hover:border-[#484f58] transition-colors">
      {/* Header */}
      <div className="flex items-start justify-between gap-2 mb-1.5">
        <div className="min-w-0">
          <span className="text-xs font-semibold text-[#e6edf3] block truncate">{entry.name}</span>
          <span className="text-[9px] text-[#484f58]">{entry.namespace}</span>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          {entry.featured && (
            <span className="text-[8px] px-1 py-0.5 rounded bg-[#e3b341]/20 text-[#e3b341]">featured</span>
          )}
          <span className={`text-[9px] px-1.5 py-0.5 rounded capitalize ${TYPE_COLORS[entry.type] ?? 'bg-[#21262d] text-[#8b949e]'}`}>
            {entry.type}
          </span>
        </div>
      </div>

      {/* Description */}
      <p className="text-[10px] text-[#8b949e] leading-relaxed mb-2 flex-1 line-clamp-2">
        {entry.description}
      </p>

      {/* LLM verdict */}
      {entry.llmVerdict && (
        <p className="text-[9px] text-[#484f58] italic mb-2 line-clamp-2">
          "{entry.llmVerdict}"
        </p>
      )}

      {/* Rating + tags */}
      <div className="flex items-center justify-between gap-2 mb-2">
        <Stars rating={entry.rating} />
        <div className="flex gap-1 flex-wrap justify-end">
          {entry.tags.slice(0, 3).map(t => (
            <span key={t} className="text-[8px] px-1 py-0.5 rounded bg-[#21262d] text-[#484f58]">
              {t}
            </span>
          ))}
        </div>
      </div>

      {/* Action */}
      <div className="flex items-center justify-between gap-2">
        <a
          href={entry.repo}
          target="_blank"
          rel="noopener noreferrer"
          className="text-[10px] text-[#58a6ff] hover:underline truncate font-mono"
          title={entry.repo}
        >
          {entry.repo.replace('https://github.com/', '')}
        </a>
        {installed ? (
          <button
            onClick={onRemove}
            disabled={busy}
            className="text-[10px] px-2 py-0.5 rounded bg-[#f85149]/10 text-[#f85149] hover:bg-[#f85149]/20 disabled:opacity-40 shrink-0"
          >
            Remove
          </button>
        ) : (
          <button
            onClick={onAdd}
            disabled={busy}
            className="text-[10px] px-2 py-0.5 rounded bg-[#238636] hover:bg-[#2ea043] text-white disabled:opacity-40 shrink-0"
          >
            {busy ? 'Adding…' : 'Add'}
          </button>
        )}
      </div>
    </div>
  )
}

// ── Main view ─────────────────────────────────────────────────────────────────

export default function BundlesView() {
  const [registry,  setRegistry]  = useState<RegistryEntry[]>([])
  const [installed, setInstalled] = useState<AppBundle[]>([])
  const [loading,   setLoading]   = useState(true)
  const [search,    setSearch]    = useState('')
  const [category,  setCategory]  = useState('all')
  const [typeFilter, setTypeFilter] = useState('all')
  const [busy, setBusy]           = useState<Record<string, boolean>>({})
  const [toast, setToast]         = useState('')

  useEffect(() => {
    Promise.all([fetchRegistry(), listBundles()])
      .then(([reg, inst]) => { setRegistry(reg); setInstalled(inst) })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  const installedIds = new Set(installed.map(b => b.id))

  const showToast = (msg: string) => {
    setToast(msg)
    setTimeout(() => setToast(''), 3000)
  }

  async function handleAdd(entry: RegistryEntry) {
    setBusy(b => ({ ...b, [entry.id]: true }))
    try {
      const bundle = await addBundle(entry)
      setInstalled(prev => [...prev, bundle])
      showToast(`✓ Added: ${entry.name}`)
    } catch (e: unknown) {
      showToast(`✗ ${(e as Error).message}`)
    } finally {
      setBusy(b => ({ ...b, [entry.id]: false }))
    }
  }

  async function handleRemove(id: string) {
    setBusy(b => ({ ...b, [id]: true }))
    try {
      await removeBundle(id)
      setInstalled(prev => prev.filter(b => b.id !== id))
      showToast('Bundle removed')
    } catch (e: unknown) {
      showToast(`✗ ${(e as Error).message}`)
    } finally {
      setBusy(b => ({ ...b, [id]: false }))
    }
  }

  async function handleToggle(id: string) {
    try {
      const updated = await toggleBundle(id)
      setInstalled(prev => prev.map(b => b.id === id ? updated : b))
    } catch (e: unknown) {
      showToast(`✗ ${(e as Error).message}`)
    }
  }

  // Filter registry
  const q = search.toLowerCase()
  const filtered = registry.filter(e => {
    if (category !== 'all' && e.category !== category) return false
    if (typeFilter !== 'all' && e.type !== typeFilter) return false
    if (q) {
      return e.name.toLowerCase().includes(q) ||
        e.description.toLowerCase().includes(q) ||
        e.namespace.toLowerCase().includes(q) ||
        e.tags.some(t => t.toLowerCase().includes(q))
    }
    return true
  })

  const featured = filtered.filter(e => e.featured && !installedIds.has(e.id))
  const rest     = filtered.filter(e => !e.featured && !installedIds.has(e.id))
  const installedEntries = filtered.filter(e => installedIds.has(e.id))

  return (
    <div className="flex h-full bg-[#0d1117] overflow-hidden">

      {/* ── Left sidebar: installed bundles ────────────────────────────── */}
      <div className="w-60 shrink-0 flex flex-col border-r border-[#30363d] overflow-hidden">
        <div className="flex items-center justify-between px-3 py-2 border-b border-[#30363d]">
          <span className="text-[#8b949e] text-[10px] uppercase tracking-wider">Installed</span>
          <span className="text-[10px] text-[#484f58]">{installed.length}</span>
        </div>

        {/* Install loom button */}
        <div className="px-3 py-2 border-b border-[#21262d]">
          <button
            onClick={async () => {
              showToast('Running: amplifier bundle add …')
              try {
                const res = await fetch('/api/bundles', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({
                    id: 'amplifier-app-loom',
                    installSpec: 'git+https://github.com/kenotron-ms/amplifier-app-loom@main',
                    name: 'Loom',
                  }),
                })
                if (res.ok || res.status === 409) {
                  showToast('✓ Loom registered as app bundle')
                  const updated = await listBundles()
                  setInstalled(updated)
                }
              } catch { showToast('Run: loom bundle install') }
            }}
            className="w-full text-left text-[10px] px-2 py-1.5 rounded bg-[#21262d] text-[#8b949e] hover:bg-[#30363d] hover:text-[#e6edf3] transition-colors"
            title="Run: loom bundle install"
          >
            + Install loom as app bundle
          </button>
        </div>

        <div className="flex-1 overflow-y-auto">
          {installed.length === 0 && !loading && (
            <div className="px-3 py-4 text-[10px] text-[#484f58] text-center">
              No bundles installed yet.{'\n'}Browse the registry →
            </div>
          )}
          {installed.map(b => (
            <div key={b.id} className="flex items-center gap-2 px-3 py-2 border-b border-[#21262d] group">
              <div className="flex-1 min-w-0">
                <div className="text-[11px] text-[#e6edf3] truncate">{b.name || b.id}</div>
                <div className="text-[9px] text-[#484f58] truncate">{b.installSpec}</div>
              </div>
              {/* Toggle */}
              <button
                onClick={() => handleToggle(b.id)}
                title={b.enabled ? 'Disable' : 'Enable'}
                className={`w-7 h-4 rounded-full transition-colors shrink-0 ${
                  b.enabled ? 'bg-[#238636]' : 'bg-[#21262d]'
                }`}
              >
                <div className={`w-3 h-3 rounded-full bg-white mx-auto transition-transform ${
                  b.enabled ? 'translate-x-1.5' : '-translate-x-1.5'
                }`} />
              </button>
              {/* Remove */}
              <button
                onClick={() => handleRemove(b.id)}
                className="opacity-0 group-hover:opacity-100 text-[#484f58] hover:text-[#f85149] text-xs shrink-0"
                title="Remove"
              >×</button>
            </div>
          ))}
        </div>
      </div>

      {/* ── Right: Registry browser ─────────────────────────────────────── */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Registry header */}
        <div className="flex items-center gap-3 px-4 py-2 border-b border-[#21262d] bg-gradient-to-r from-[#0d1117] to-[#161b22] shrink-0">
          <span className="text-[10px] text-[#484f58] uppercase tracking-wider">Amplifier Registry</span>
          <a
            href="https://kenotron-ms.github.io/amplifier-registry/"
            target="_blank"
            rel="noopener noreferrer"
            className="text-[11px] font-medium text-[#f0883e] hover:text-[#ffa657] hover:underline transition-colors"
          >
            🌐 Browse Site
          </a>
          <a
            href="https://github.com/kenotron-ms/amplifier-registry"
            target="_blank"
            rel="noopener noreferrer"
            className="text-[11px] font-medium text-[#58a6ff] hover:text-[#79c0ff] hover:underline transition-colors"
          >
            ⎇ GitHub Source
          </a>
        </div>

        {/* Search + filters */}
        <div className="px-4 py-3 border-b border-[#30363d] shrink-0 space-y-2">
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search registry…"
            className="w-full px-3 py-1.5 text-xs bg-[#0d1117] border border-[#30363d] rounded text-[#e6edf3] placeholder:text-[#484f58] focus:outline-none focus:border-[#58a6ff]"
          />
          <div className="flex gap-4">
            {/* Category */}
            <div className="flex gap-1 flex-wrap">
              {CATEGORIES.map(c => (
                <button
                  key={c}
                  onClick={() => setCategory(c)}
                  className={`text-[9px] px-2 py-0.5 rounded capitalize ${
                    category === c
                      ? 'bg-[#388bfd]/20 text-[#58a6ff]'
                      : 'text-[#484f58] hover:text-[#8b949e]'
                  }`}
                >
                  {c}
                </button>
              ))}
            </div>
            <div className="flex gap-1">
              {TYPES.map(t => (
                <button
                  key={t}
                  onClick={() => setTypeFilter(t)}
                  className={`text-[9px] px-2 py-0.5 rounded capitalize ${
                    typeFilter === t
                      ? `${TYPE_COLORS[t] ?? 'bg-[#388bfd]/20 text-[#58a6ff]'}`
                      : 'text-[#484f58] hover:text-[#8b949e]'
                  }`}
                >
                  {t}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Card grid */}
        <div className="flex-1 overflow-y-auto px-4 py-3 space-y-4">
          {loading && (
            <div className="flex items-center justify-center h-32 text-[#484f58] text-xs">
              Loading registry…
            </div>
          )}

          {/* Already installed — shown at top if matches filter */}
          {installedEntries.length > 0 && (
            <section>
              <h3 className="text-[10px] text-[#484f58] uppercase tracking-wider mb-2">Installed</h3>
              <div className="grid grid-cols-2 xl:grid-cols-3 gap-3">
                {installedEntries.map(e => (
                  <BundleCard
                    key={e.id}
                    entry={e}
                    installed={true}
                    busy={!!busy[e.id]}
                    onAdd={() => handleAdd(e)}
                    onRemove={() => handleRemove(e.id)}
                  />
                ))}
              </div>
            </section>
          )}

          {/* Featured */}
          {featured.length > 0 && (
            <section>
              <h3 className="text-[10px] text-[#e3b341] uppercase tracking-wider mb-2">⭐ Featured</h3>
              <div className="grid grid-cols-2 xl:grid-cols-3 gap-3">
                {featured.map(e => (
                  <BundleCard
                    key={e.id}
                    entry={e}
                    installed={installedIds.has(e.id)}
                    busy={!!busy[e.id]}
                    onAdd={() => handleAdd(e)}
                    onRemove={() => handleRemove(e.id)}
                  />
                ))}
              </div>
            </section>
          )}

          {/* All others */}
          {rest.length > 0 && (
            <section>
              {featured.length > 0 && (
                <h3 className="text-[10px] text-[#484f58] uppercase tracking-wider mb-2">
                  Community ({rest.length})
                </h3>
              )}
              <div className="grid grid-cols-2 xl:grid-cols-3 gap-3">
                {rest.map(e => (
                  <BundleCard
                    key={e.id}
                    entry={e}
                    installed={installedIds.has(e.id)}
                    busy={!!busy[e.id]}
                    onAdd={() => handleAdd(e)}
                    onRemove={() => handleRemove(e.id)}
                  />
                ))}
              </div>
            </section>
          )}

          {!loading && filtered.length === 0 && (
            <div className="flex items-center justify-center h-32 text-[#484f58] text-xs">
              No bundles match your filters.
            </div>
          )}
        </div>
      </div>

      {/* Toast */}
      {toast && (
        <div className="fixed bottom-4 right-4 z-50 px-3 py-2 bg-[#161b22] border border-[#30363d] rounded text-xs text-[#e6edf3] shadow-lg">
          {toast}
        </div>
      )}
    </div>
  )
}
