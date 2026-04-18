// ── Registry types (mirrors kenotron-ms/amplifier-registry/bundles.json) ─────

export interface RegistryEntry {
  id: string
  name: string
  namespace: string
  description: string
  type: 'bundle' | 'agent' | 'tool' | 'module'
  category: string
  author: string
  repo: string
  install: string          // e.g. "amplifier bundle add superpowers"
  rating: number | null
  tags: string[]
  featured?: boolean
  community?: boolean
  lastUpdated: string
  llmVerdict?: string
  stars?: number
  forks?: number
  quality?: {
    total: number
    rating: number
  }
  // Private / local registry fields
  private?: boolean
  localPath?: string
  capabilities?: Array<{
    type: string
    name: string
    description: string | null
    version: string | null
    sourceFile: string
  }>
}

// ── Installed bundle types (loom config) ─────────────────────────────────────

export interface AppBundle {
  id: string
  installSpec: string
  name: string
  enabled: boolean
}

// ── API calls ─────────────────────────────────────────────────────────────────

/** Fetch all entries from the public registry (cached server-side for 1 h). */
export async function fetchRegistry(): Promise<RegistryEntry[]> {
  const res = await fetch('/api/registry')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

/** Fetch private bundles from the local index (~/.amplifier/bundle-index/). */
export async function fetchLocalRegistry(): Promise<RegistryEntry[]> {
  const res = await fetch('/api/local-registry')
  if (!res.ok) return []  // graceful degradation if index not seeded
  return res.json()
}

/** List app bundles installed in loom. */
export async function listBundles(): Promise<AppBundle[]> {
  const res = await fetch('/api/bundles')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

/** Add a bundle from a registry entry's install string. */
export async function addBundle(entry: RegistryEntry): Promise<AppBundle> {
  // Strip "amplifier bundle add " prefix to get the bare install spec
  const installSpec = entry.install.replace(/^amplifier bundle add\s+/, '').trim()
  const res = await fetch('/api/bundles', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: entry.id, installSpec, name: entry.name }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? 'Failed to add bundle')
  }
  return res.json()
}

/** Remove an installed bundle by id. */
export async function removeBundle(id: string): Promise<void> {
  const res = await fetch(`/api/bundles/${encodeURIComponent(id)}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 204) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? 'Failed to remove bundle')
  }
}

/** Toggle a bundle's enabled state. */
export async function toggleBundle(id: string): Promise<AppBundle> {
  const res = await fetch(`/api/bundles/${encodeURIComponent(id)}/toggle`, { method: 'POST' })
  if (!res.ok) throw new Error('Failed to toggle bundle')
  return res.json()
}
