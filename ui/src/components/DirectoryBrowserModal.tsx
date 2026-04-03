import { useEffect, useState, useCallback } from 'react'
import { browseDirs, BrowseEntry } from '../api/projects'

interface Props {
  onSelect: (path: string) => void
  onClose: () => void
}

export default function DirectoryBrowserModal({ onSelect, onClose }: Props) {
  const [path, setPath]             = useState('')      // current browsed path
  const [home, setHome]             = useState('')
  const [parent, setParent]         = useState('')
  const [entries, setEntries]       = useState<BrowseEntry[]>([])
  const [showHidden, setShowHidden] = useState(false)
  const [loading, setLoading]       = useState(true)
  const [error, setError]           = useState('')

  const navigate = useCallback(async (target?: string) => {
    setLoading(true)
    setError('')
    try {
      const result = await browseDirs(target)
      setPath(result.path)
      setHome(result.home)
      setParent(result.parent)
      setEntries(result.entries)
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [])

  // Load home directory on mount
  useEffect(() => { navigate() }, [navigate])

  // Keyboard: Escape closes, Backspace goes up
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
      if (e.key === 'Backspace' && parent && document.activeElement?.tagName !== 'INPUT') {
        navigate(parent)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose, parent, navigate])

  // Build breadcrumb segments from current path
  const segments = path.split('/').filter(Boolean)

  const visible = showHidden ? entries : entries.filter(e => !e.hidden)
  const hiddenCount = entries.filter(e => e.hidden).length

  return (
    <div
      className="fixed inset-0 bg-[rgba(20,16,10,0.20)] flex items-center justify-center z-[60]"
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
    >
      <div className="bg-[#FAF8F4] border border-[#DDD5C8] rounded-md shadow-lg flex flex-col"
           style={{ width: 520, height: 480 }}>

        {/* Header */}
        <div className="flex items-center justify-between px-4 py-2.5 border-b border-[#DDD5C8] shrink-0">
          <span className="text-sm font-semibold text-[#1C1A16]">Open Folder</span>
          <div className="flex items-center gap-3">
            {home && (
              <button
                onClick={() => navigate(home)}
                disabled={path === home}
                className="text-[10px] text-[#8A8278] hover:text-[#1C1A16] disabled:opacity-30"
                title="Go to home directory"
              >
                ~
              </button>
            )}
            <button
              onClick={onClose}
              className="text-[#A09888] hover:text-[#8A8278] text-xl leading-none"
              aria-label="Close"
            >×</button>
          </div>
        </div>

        {/* Breadcrumb */}
        <div className="flex items-center gap-0.5 px-3 py-1.5 border-b border-[#DDD5C8] bg-[#F0EBE3] shrink-0 overflow-x-auto whitespace-nowrap">
          {/* Root / */}
          <button
            onClick={() => navigate('/')}
            className="text-[11px] text-[#8A8278] hover:text-[#1C1A16] shrink-0 px-0.5"
          >/</button>
          {segments.map((seg, i) => {
            const segPath = '/' + segments.slice(0, i + 1).join('/')
            const isLast = i === segments.length - 1
            return (
              <span key={segPath} className="flex items-center shrink-0">
                <span className="text-[11px] text-[#DDD5C8] mx-0.5">/</span>
                <button
                  onClick={() => !isLast && navigate(segPath)}
                  className={[
                    'text-[11px] px-0.5',
                    isLast
                      ? 'text-[#1C1A16] cursor-default'
                      : 'text-[#8A8278] hover:text-[#1C1A16]',
                  ].join(' ')}
                >
                  {seg}
                </button>
              </span>
            )
          })}
        </div>

        {/* Directory list */}
        <div className="flex-1 overflow-y-auto">
          {loading && (
            <div className="flex items-center justify-center h-full text-[#A09888] text-xs">
              Loading…
            </div>
          )}
          {!loading && error && (
            <div className="p-4 text-[#E53935] text-xs font-mono">{error}</div>
          )}
          {!loading && !error && (
            <>
              {/* Parent dir row */}
              {parent && (
                <button
                  onClick={() => navigate(parent)}
                  className="w-full text-left flex items-center gap-2 px-4 py-1.5 text-xs text-[#8A8278] hover:bg-[#E8E0D4] border-b border-[#DDD5C8] transition-colors"
                >
                  <span className="text-[#A09888]">↑</span>
                  <span>..</span>
                </button>
              )}

              {visible.length === 0 && (
                <div className="flex items-center justify-center h-24 text-[#A09888] text-xs">
                  {entries.length === 0 ? 'Empty directory' : 'No visible directories'}
                </div>
              )}

              {visible.map(entry => (
                <button
                  key={entry.name}
                  onDoubleClick={() => navigate(`${path}/${entry.name}`)}
                  onClick={() => navigate(`${path}/${entry.name}`)}
                  className={[
                    'w-full text-left flex items-center gap-2 px-4 py-1.5 text-xs border-b border-[#DDD5C8] hover:bg-[#E8E0D4] transition-colors',
                    entry.hidden ? 'opacity-50' : '',
                  ].join(' ')}
                >
                  <span className="text-[#F59E0B] shrink-0">📁</span>
                  <span className="text-[#1C1A16] truncate">{entry.name}</span>
                </button>
              ))}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="border-t border-[#DDD5C8] px-4 py-2.5 shrink-0 bg-[#F0EBE3]">
          {/* Current path display */}
          <div className="text-[10px] text-[#A09888] font-mono truncate mb-2" title={path}>
            {path || '…'}
          </div>
          <div className="flex items-center gap-2">
            {hiddenCount > 0 && (
              <label className="flex items-center gap-1.5 text-[10px] text-[#8A8278] cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={showHidden}
                  onChange={e => setShowHidden(e.target.checked)}
                  className="w-3 h-3"
                />
                Show hidden ({hiddenCount})
              </label>
            )}
            <div className="ml-auto flex gap-2">
              <button
                onClick={onClose}
                className="px-3 py-1.5 text-xs text-[#8A8278] hover:text-[#1C1A16]"
              >
                Cancel
              </button>
              <button
                onClick={() => path && onSelect(path)}
                disabled={!path || loading}
                className="px-3 py-1.5 text-xs bg-[#4CAF74] hover:bg-[#43A047] text-white rounded disabled:opacity-40 disabled:cursor-not-allowed"
              >
                Select This Folder
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
