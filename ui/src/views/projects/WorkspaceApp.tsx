import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import {
  Project, Session, FileEntry,
  listProjects, createProject,
  listSessions, createSession, spawnTerminal, listFiles,
  pickFolder, canPickFolder,
} from '../../api/projects'

// ── Terminal cache — one instance per processId, kept alive forever ──────────
//
// xterm.js Terminal.open(container) moves the terminal's DOM to a new container
// without recreating the scrollback buffer. This lets us "switch" terminals by
// just reattaching the cached instance to the single shared container div.

type TermEntry = { term: Terminal; fit: FitAddon; ws: WebSocket }

function useTerminalCache(
  containerRef: React.RefObject<HTMLDivElement | null>,
  processId: string | null,
) {
  const cache = useRef<Map<string, TermEntry>>(new Map())

  useEffect(() => {
    const container = containerRef.current
    if (!container || !processId) return

    // Clear any terminal DOM previously attached to this container.
    // xterm.js appends rather than replaces, so without this two terminals
    // would stack on top of each other when switching sessions.
    container.innerHTML = ''

    const existing = cache.current.get(processId)
    if (existing) {
      // Reattach cached terminal — scrollback and running process preserved
      existing.term.open(container)
      setTimeout(() => existing.fit.fit(), 16)
      const ro = new ResizeObserver(() => existing.fit.fit())
      ro.observe(container)
      return () => ro.disconnect()
    }

    // First time seeing this processId — create terminal + WebSocket
    const term = new Terminal({
      theme: { background: '#0d1117', foreground: '#e6edf3', cursor: '#58a6ff' },
      fontFamily: 'monospace',
      fontSize: 13,
      cursorBlink: true,
      scrollback: 5000,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(container)
    fit.fit()

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${proto}//${window.location.host}/api/terminal/${processId}`)
    ws.binaryType = 'arraybuffer'
    ws.onmessage = (e) => {
      const data = e.data instanceof ArrayBuffer
        ? new TextDecoder().decode(e.data)
        : e.data as string
      term.write(data)
    }
    ws.onclose = () => {
      // Process exited — remove from cache so next spawn creates a fresh terminal
      cache.current.delete(processId)
      term.write('\r\n[Process exited — create a new session to restart]\r\n')
    }
    term.onData((data) => ws.readyState === WebSocket.OPEN && ws.send(data))

    cache.current.set(processId, { term, fit, ws })

    const ro = new ResizeObserver(() => fit.fit())
    ro.observe(container)
    // Only disconnect the resize observer on cleanup — keep terminal + WS alive
    return () => ro.disconnect()
  }, [processId, containerRef])
}

// ── File browser panel ────────────────────────────────────────────────────────

function FileBrowserPanel({ projectId, sessionId }: { projectId: string; sessionId: string }) {
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [path, setPath] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    listFiles(projectId, sessionId, path)
      .then(setEntries)
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [projectId, sessionId, path])

  return (
    <div className="flex flex-col h-full bg-[#161b22] border-l border-[#30363d]">
      <div className="px-3 py-2 border-b border-[#30363d] text-[10px] text-[#8b949e] uppercase tracking-wider">
        Files {path && <span className="text-[#58a6ff]">/{path}</span>}
      </div>
      {path && (
        <button
          onClick={() => setPath(path.split('/').slice(0, -1).join('/'))}
          className="px-3 py-1 text-xs text-[#8b949e] hover:text-[#e6edf3] text-left border-b border-[#21262d]"
        >
          ↑ ..
        </button>
      )}
      <div className="flex-1 overflow-y-auto">
        {loading && <div className="px-3 py-2 text-xs text-[#8b949e]">Loading…</div>}
        {entries.map((e) => (
          <button
            key={e.name}
            onClick={() => e.isDir && setPath(path ? `${path}/${e.name}` : e.name)}
            className="w-full text-left px-3 py-1 text-xs hover:bg-[#21262d] transition-colors"
          >
            <span className={e.isDir ? 'text-[#58a6ff]' : 'text-[#e6edf3]'}>
              {e.isDir ? '📁' : '📄'} {e.name}
            </span>
          </button>
        ))}
      </div>
    </div>
  )
}

// ── Main workspace ────────────────────────────────────────────────────────────

export default function WorkspaceApp() {
  const [projects, setProjects] = useState<Project[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [activeProject, setActiveProject] = useState<Project | null>(null)
  const [activeSession, setActiveSession] = useState<Session | null>(null)
  const [processId, setProcessId] = useState<string | null>(null)
  const [showFiles, setShowFiles] = useState(false)

  // New Project modal
  const [showNewProject, setShowNewProject] = useState(false)
  const [newProjectName, setNewProjectName] = useState('')
  const [newProjectPath, setNewProjectPath] = useState('')
  const [pathPickerSupported, setPathPickerSupported] = useState<boolean | null>(null)

  // Probe once on mount whether the native folder picker is available (no dialog opens)
  useEffect(() => {
    canPickFolder().then(setPathPickerSupported).catch(() => setPathPickerSupported(false))
  }, [])

  // New Session modal
  const [showNewSession, setShowNewSession] = useState(false)
  const [newSessionName, setNewSessionName] = useState('')
  const [sessionError, setSessionError] = useState('')

  const termContainerRef = useRef<HTMLDivElement>(null)

  useTerminalCache(termContainerRef, processId)

  useEffect(() => {
    listProjects()
      .then(ps => {
        setProjects(ps)
        if (ps.length > 0) selectProject(ps[0])
      })
      .catch(console.error)
  }, [])

  async function selectProject(p: Project) {
    setActiveProject(p)
    setActiveSession(null)
    setProcessId(null)
    const ss = await listSessions(p.id).catch(() => [] as Session[])
    setSessions(ss)
    if (ss.length > 0) selectSession(p, ss[0])
  }

  async function selectSession(p: Project, s: Session) {
    setActiveSession(s)
    setProcessId(null)
    try {
      const { processId: pid } = await spawnTerminal(p.id, s.id)
      setProcessId(pid)
    } catch (e) {
      console.error('spawnTerminal:', e)
    }
  }

  async function handleBrowse() {
    try {
      const result = await pickFolder()
      if (result.path) setNewProjectPath(result.path)
    } catch (e) {
      console.error('pickFolder:', e)
    }
  }

  async function handleCreateProject() {
    if (!newProjectName || !newProjectPath) return
    try {
      const p = await createProject(newProjectName, newProjectPath)
      setProjects(ps => [...ps, p])
      setShowNewProject(false)
      setNewProjectName('')
      setNewProjectPath('')
      selectProject(p)
    } catch (e) {
      console.error('createProject:', e)
    }
  }

  async function handleCreateSession() {
    if (!activeProject || !newSessionName) return
    setSessionError('')
    try {
      const s = await createSession(activeProject.id, newSessionName)
      setSessions(ss => [...ss, s])
      setShowNewSession(false)
      setNewSessionName('')
      selectSession(activeProject, s)
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e)
      setSessionError(msg)
    }
  }

  return (
    <div className="flex h-full bg-[#0d1117]">
      {/* Left sidebar */}
      <div className="w-56 border-r border-[#30363d] flex flex-col shrink-0">
        {/* Project list */}
        <div className="flex items-center justify-between px-3 py-2 border-b border-[#30363d]">
          <span className="text-[#8b949e] text-[10px] uppercase tracking-wider">Projects</span>
          <button
            onClick={() => setShowNewProject(true)}
            className="text-[#58a6ff] text-xs hover:text-[#e6edf3]"
            aria-label="New project"
          >+</button>
        </div>
        <div className="flex-1 overflow-y-auto">
          {projects.map(p => (
            <button
              key={p.id}
              onClick={() => selectProject(p)}
              className={[
                'w-full text-left px-3 py-2 border-b border-[#21262d] transition-colors',
                activeProject?.id === p.id ? 'bg-[#21262d]' : 'hover:bg-[#161b22]',
              ].join(' ')}
            >
              <div className={`text-xs truncate ${activeProject?.id === p.id ? 'text-[#e6edf3]' : 'text-[#8b949e]'}`}>
                {p.name}
              </div>
            </button>
          ))}
        </div>

        {/* Session list for active project */}
        {activeProject && (
          <>
            <div className="flex items-center justify-between px-3 py-2 border-t border-b border-[#30363d]">
              <span className="text-[#8b949e] text-[10px] uppercase tracking-wider">Sessions</span>
              <button
                onClick={() => { setShowNewSession(true); setSessionError('') }}
                className="text-[#58a6ff] text-xs hover:text-[#e6edf3]"
                aria-label="New session"
              >+</button>
            </div>
            {sessions.length === 0 && (
              <div className="px-3 py-2 text-[10px] text-[#484f58]">No sessions yet</div>
            )}
            {sessions.map(s => (
              <button
                key={s.id}
                onClick={() => selectSession(activeProject, s)}
                className={[
                  'w-full text-left px-3 py-1.5 border-b border-[#21262d] transition-colors',
                  activeSession?.id === s.id ? 'bg-[#21262d]' : 'hover:bg-[#161b22]',
                ].join(' ')}
              >
                <div className={`text-[11px] truncate ${activeSession?.id === s.id ? 'text-[#e6edf3]' : 'text-[#8b949e]'}`}>
                  {s.name}
                </div>
                <div className="text-[10px] text-[#484f58]">{s.status}</div>
              </button>
            ))}
          </>
        )}
      </div>

      {/* Main area */}
      <div className="flex flex-1 overflow-hidden">
        <div className="flex-1 flex flex-col overflow-hidden">
          {/* Session header — only shown when a session is active */}
          {activeProject && activeSession && (
            <div className="flex items-center gap-2 px-3 py-1.5 bg-[#161b22] border-b border-[#30363d] shrink-0">
              <span className="text-xs text-[#e6edf3] font-medium">{activeProject.name}</span>
              <span className="text-xs text-[#8b949e]">/ {activeSession.name}</span>
              <button
                onClick={() => setShowFiles(!showFiles)}
                className="ml-auto text-[10px] px-2 py-0.5 rounded bg-[#21262d] text-[#8b949e] hover:text-[#e6edf3]"
              >
                {showFiles ? 'Hide Files' : 'Files'}
              </button>
            </div>
          )}

          {/* Content area — terminal container is ALWAYS in DOM so instances persist */}
          <div className="flex-1 overflow-hidden relative">
            {/* Empty state overlay — covers the (invisible) terminal when no session */}
            {(!activeProject || !activeSession) && (
              <div className="absolute inset-0 flex items-center justify-center z-10 bg-[#0d1117]">
                {activeProject ? (
                  <div className="text-center text-[#8b949e]">
                    <div className="text-sm font-medium text-[#e6edf3] mb-1">{activeProject.name}</div>
                    <div className="text-xs mb-3 text-[#484f58]">{activeProject.path}</div>
                    <button
                      onClick={() => { setShowNewSession(true); setSessionError('') }}
                      className="text-xs px-3 py-1.5 bg-[#21262d] border border-[#30363d] rounded text-[#e6edf3] hover:bg-[#30363d]"
                    >
                      + New Session
                    </button>
                  </div>
                ) : (
                  <div className="text-center text-[#8b949e]">
                    <div className="text-sm mb-2">Select or create a project</div>
                    <button
                      onClick={() => setShowNewProject(true)}
                      className="text-xs px-3 py-1.5 bg-[#21262d] border border-[#30363d] rounded text-[#e6edf3] hover:bg-[#30363d]"
                    >
                      + New Project
                    </button>
                  </div>
                )}
              </div>
            )}

            {/* Terminal — always in DOM, hidden until a session's process is ready */}
            <div
              ref={termContainerRef}
              className="absolute inset-0"
              style={{ visibility: (activeProject && activeSession && processId) ? 'visible' : 'hidden' }}
            />
          </div>
        </div>

        {showFiles && activeProject && activeSession && (
          <div className="w-52 shrink-0">
            <FileBrowserPanel projectId={activeProject.id} sessionId={activeSession.id} />
          </div>
        )}
      </div>

      {/* New Project modal */}
      {showNewProject && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-[#161b22] border border-[#30363d] rounded-lg p-5 w-80">
            <h3 className="text-sm font-semibold text-[#e6edf3] mb-4">New Project</h3>
            <input
              className="w-full mb-3 px-3 py-1.5 text-sm bg-[#0d1117] border border-[#30363d] rounded text-[#e6edf3] placeholder:text-[#8b949e] focus:outline-none focus:border-[#58a6ff]"
              placeholder="Project name"
              value={newProjectName}
              onChange={e => setNewProjectName(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleCreateProject()}
              autoFocus
            />
            <div className="flex gap-2 mb-4">
              <input
                className="flex-1 px-3 py-1.5 text-sm bg-[#0d1117] border border-[#30363d] rounded text-[#e6edf3] placeholder:text-[#8b949e] focus:outline-none focus:border-[#58a6ff]"
                placeholder="/absolute/path/to/codebase"
                value={newProjectPath}
                onChange={e => setNewProjectPath(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && handleCreateProject()}
              />
              {pathPickerSupported && (
                <button
                  onClick={handleBrowse}
                  type="button"
                  className="px-3 py-1.5 text-xs bg-[#21262d] border border-[#30363d] rounded text-[#8b949e] hover:text-[#e6edf3] hover:bg-[#30363d] shrink-0"
                >
                  Browse…
                </button>
              )}
            </div>
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setShowNewProject(false)}
                className="px-3 py-1.5 text-xs text-[#8b949e] hover:text-[#e6edf3]"
              >Cancel</button>
              <button
                onClick={handleCreateProject}
                className="px-3 py-1.5 text-xs bg-[#238636] hover:bg-[#2ea043] text-white rounded"
              >Create</button>
            </div>
          </div>
        </div>
      )}

      {/* New Session modal */}
      {showNewSession && activeProject && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-[#161b22] border border-[#30363d] rounded-lg p-5 w-80">
            <h3 className="text-sm font-semibold text-[#e6edf3] mb-1">New Session</h3>
            <p className="text-[10px] text-[#484f58] mb-4">
              Opens a terminal in <span className="text-[#8b949e]">{activeProject.path}</span>
            </p>
            <input
              className="w-full mb-2 px-3 py-1.5 text-sm bg-[#0d1117] border border-[#30363d] rounded text-[#e6edf3] placeholder:text-[#8b949e] focus:outline-none focus:border-[#58a6ff]"
              placeholder="Session name (e.g. main, debug, review)"
              value={newSessionName}
              onChange={e => { setNewSessionName(e.target.value); setSessionError('') }}
              onKeyDown={e => e.key === 'Enter' && handleCreateSession()}
              autoFocus
            />
            {sessionError && (
              <div className="text-[10px] text-[#f85149] bg-[#3a1a1a] rounded px-2 py-1 mb-2">
                {sessionError}
              </div>
            )}
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => { setShowNewSession(false); setNewSessionName(''); setSessionError('') }}
                className="px-3 py-1.5 text-xs text-[#8b949e] hover:text-[#e6edf3]"
              >Cancel</button>
              <button
                onClick={handleCreateSession}
                className="px-3 py-1.5 text-xs bg-[#238636] hover:bg-[#2ea043] text-white rounded"
              >Create Session</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
