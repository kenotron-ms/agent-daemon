# Phase 2: Projects Frontend — Implementation Plan

> **For execution:** Use `/execute-plan` mode.

**Prerequisite:** Phase 1 complete (Go backend endpoints `GET /api/projects/{id}/amplifier-sessions` and `POST /api/projects/{id}/open-terminal` exist; `workspace` field added to Project struct).
**Goal:** Replace the 3-pane `WorkspaceApp` with a `ProjectsGrid` card view and tabbed `ProjectDetail` page (Sessions, Settings, Files).
**Architecture:** State-based routing in `App.tsx` — `selectedProjectId` switches between grid and detail views. Dark-themed inline styles scoped to the projects view only (other views untouched). New components call Phase 1 backend endpoints for session listing and terminal launch.
**Tech Stack:** React 18, TypeScript, CSS grid/flexbox, inline styles

---

### Task 1: Add API types and functions

**Files:**
- Modify: `ui/src/api/projects.ts`

**Step 1: Add `workspace` field to the `Project` interface**

At `ui/src/api/projects.ts:1-7`, add the `workspace` field:

```typescript
export interface Project {
  id: string
  name: string
  path: string
  workspace?: string
  createdAt: number
  lastActivityAt: number
}
```

**Step 2: Add `AmplifierSession` interface and three new API functions**

Append to the end of `ui/src/api/projects.ts` (after line 254):

```typescript
// ── Amplifier sessions (Phase 2 — reads from Amplifier's session store) ──────

export interface AmplifierSession {
  id: string
  name: string
  createdAt: string
  lastActiveAt: string
}

export async function getProject(id: string): Promise<Project> {
  const res = await fetch(`/api/projects/${id}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function listAmplifierSessions(projectId: string): Promise<AmplifierSession[]> {
  const res = await fetch(`/api/projects/${projectId}/amplifier-sessions`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function openTerminal(
  projectId: string,
  mode: 'new' | 'resume',
  sessionId?: string,
): Promise<void> {
  const res = await fetch(`/api/projects/${projectId}/open-terminal`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ mode, sessionId }),
  })
  if (!res.ok) throw new Error(await res.text())
}
```

**Step 3: Build check**
```
cd ui && npm run build
```
Expected: no TypeScript errors (new types/functions are valid but unused)

**Step 4: Commit**
```
git add -A && git commit -m "feat: add API types for amplifier sessions and terminal launch"
```

---

### Task 2: Create ProjectCard component

**Files:**
- Create: `ui/src/views/projects/ProjectCard.tsx`

**Step 1: Write the component**

Create `ui/src/views/projects/ProjectCard.tsx`:

```tsx
import { type Project, openTerminal } from '../../api/projects'

interface Props {
  project: Project
  sessionCount: number
  onSelect: (id: string) => void
}

function shortenPath(fullPath: string): string {
  return fullPath.replace(/^\/Users\/[^/]+/, '~')
}

export default function ProjectCard({ project, sessionCount, onSelect }: Props) {
  const hasActive = sessionCount > 0

  async function handleNewSession(e: React.MouseEvent) {
    e.stopPropagation()
    try {
      await openTerminal(project.id, 'new')
    } catch (err) {
      console.error('Failed to open terminal:', err)
    }
  }

  return (
    <div
      onClick={() => onSelect(project.id)}
      style={{
        background: '#1c1f27',
        border: '1px solid #252832',
        borderRadius: 8,
        boxShadow: '0 2px 8px rgba(0,0,0,0.4)',
        padding: 16,
        cursor: 'pointer',
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
        transition: 'border-color 0.15s ease',
      }}
      onMouseEnter={e => (e.currentTarget.style.borderColor = '#3a3f4b')}
      onMouseLeave={e => (e.currentTarget.style.borderColor = '#252832')}
    >
      {/* Top row: project name + status dot */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span style={{
          fontSize: 14,
          fontWeight: 600,
          color: '#ffffff',
          flex: 1,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
        }}>
          {project.name}
        </span>
        <span style={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          background: hasActive ? '#4CAF74' : '#6b7280',
          flexShrink: 0,
        }} />
      </div>

      {/* Shortened path */}
      <div style={{
        fontFamily: 'var(--font-mono)',
        fontSize: 12,
        color: '#4b5563',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
      }}>
        {shortenPath(project.path)}
      </div>

      {/* Session count badge */}
      <div>
        <span style={{
          display: 'inline-block',
          fontSize: 11,
          fontWeight: 500,
          padding: '2px 8px',
          borderRadius: 9999,
          background: hasActive ? 'rgba(76,175,116,0.15)' : 'rgba(107,114,128,0.15)',
          color: hasActive ? '#4CAF74' : '#6b7280',
        }}>
          {sessionCount} {sessionCount === 1 ? 'session' : 'sessions'}
        </span>
      </div>

      {/* New Session ghost button */}
      <button
        onClick={handleNewSession}
        style={{
          width: '100%',
          padding: '6px 0',
          marginTop: 4,
          fontSize: 12,
          fontWeight: 500,
          color: '#9ca3af',
          background: 'transparent',
          border: '1px solid #252832',
          borderRadius: 6,
          cursor: 'pointer',
          transition: 'all 0.15s ease',
        }}
        onMouseEnter={e => {
          e.currentTarget.style.borderColor = '#14b8a6'
          e.currentTarget.style.color = '#14b8a6'
        }}
        onMouseLeave={e => {
          e.currentTarget.style.borderColor = '#252832'
          e.currentTarget.style.color = '#9ca3af'
        }}
      >
        New Session
      </button>
    </div>
  )
}
```

**Step 2: Build check**
```
cd ui && npm run build
```
Expected: no TypeScript errors

**Step 3: Commit**
```
git add -A && git commit -m "feat: add ProjectCard component"
```

---

### Task 3: Create ProjectsGrid component

**Files:**
- Create: `ui/src/views/projects/ProjectsGrid.tsx`

**Step 1: Write the component**

Create `ui/src/views/projects/ProjectsGrid.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { type Project, listProjects, listAmplifierSessions } from '../../api/projects'
import ProjectCard from './ProjectCard'

interface Props {
  onSelectProject: (id: string) => void
}

export default function ProjectsGrid({ onSelectProject }: Props) {
  const [projects, setProjects] = useState<Project[]>([])
  const [sessionCounts, setSessionCounts] = useState<Record<string, number>>({})
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    listProjects()
      .then(async (ps) => {
        setProjects(ps)
        const counts: Record<string, number> = {}
        await Promise.all(
          ps.map(async (p) => {
            try {
              const sessions = await listAmplifierSessions(p.id)
              counts[p.id] = sessions.length
            } catch {
              counts[p.id] = 0
            }
          }),
        )
        setSessionCounts(counts)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  // Group projects by workspace label
  const groups = new Map<string, Project[]>()
  for (const p of projects) {
    const ws = p.workspace || 'Default'
    if (!groups.has(ws)) groups.set(ws, [])
    groups.get(ws)!.push(p)
  }

  if (loading) {
    return (
      <div style={{ background: '#12141a', height: '100%', padding: 24, color: '#6b7280' }}>
        Loading projects...
      </div>
    )
  }

  return (
    <div style={{ background: '#12141a', height: '100%', overflowY: 'auto', padding: 24 }}>
      {/* Top bar with Add Project placeholder */}
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 24 }}>
        <button
          style={{
            fontSize: 12,
            fontWeight: 500,
            padding: '6px 16px',
            color: '#9ca3af',
            background: 'transparent',
            border: '1px solid #252832',
            borderRadius: 6,
            cursor: 'pointer',
          }}
        >
          Add Project
        </button>
      </div>

      {/* Workspace groups */}
      {Array.from(groups.entries()).map(([workspace, wsProjects]) => (
        <div key={workspace} style={{ marginBottom: 32 }}>
          {/* Section header */}
          <div style={{
            fontSize: 11,
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
            color: '#6b7280',
            marginBottom: 12,
          }}>
            {workspace}
          </div>

          {/* 3-column card grid */}
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(3, 1fr)',
            gap: 16,
          }}>
            {wsProjects.map(p => (
              <ProjectCard
                key={p.id}
                project={p}
                sessionCount={sessionCounts[p.id] ?? 0}
                onSelect={onSelectProject}
              />
            ))}
          </div>
        </div>
      ))}

      {/* Empty state */}
      {projects.length === 0 && (
        <div style={{ color: '#6b7280', textAlign: 'center', paddingTop: 64, fontSize: 13 }}>
          No projects found. Add a project to get started.
        </div>
      )}
    </div>
  )
}
```

**Step 2: Build check**
```
cd ui && npm run build
```
Expected: no TypeScript errors

**Step 3: Commit**
```
git add -A && git commit -m "feat: add ProjectsGrid component with workspace grouping"
```

---

### Task 4: Wire ProjectsGrid into the app

**Files:**
- Modify: `ui/src/views/projects/index.tsx`
- Modify: `ui/src/App.tsx`

**Step 1: Update `index.tsx` to export `ProjectsGrid`**

Replace the entire contents of `ui/src/views/projects/index.tsx` with:

```tsx
import ProjectsGrid from './ProjectsGrid'

export default function ProjectsView() {
  return <ProjectsGrid onSelectProject={(id) => console.log('Selected:', id)} />
}
```

> Note: This is a temporary wiring — `onSelectProject` just logs for now. Task 8 will add the real navigation to `ProjectDetail`.

**Step 2: Build check**
```
cd ui && npm run build
```
Expected: no TypeScript errors. The `WorkspaceApp` import is gone from `index.tsx`. `WorkspaceApp.tsx` still exists on disk but is no longer imported.

**Step 3: Commit**
```
git add -A && git commit -m "feat: wire ProjectsGrid as default projects view"
```

---

### Task 5: Checkpoint A — Browser verification of grid view

> Verify: the Projects tab shows workspace section headers and project cards in a 3-column grid on a dark background.

**Step 1: Ensure the dev server is running**

```
cd ui && npm run dev &
```

(If already running, skip this.)

**Step 2: Open the app and navigate to the Projects tab**

```
agent-browser open http://localhost:7700
agent-browser snapshot -ic
```

Click the Projects tab (if not already selected) using the interactive ref from the snapshot.

**Step 3: Verify the grid renders**

```
agent-browser snapshot -ic
```

Expected:
- Dark background (`#12141a`) fills the projects content area
- Workspace section headers visible (uppercase, gray text like "DEFAULT")
- Project cards visible in a 3-column grid with dark card backgrounds
- Each card shows: project name (white), shortened path (gray monospace), session badge, "New Session" button
- "Add Project" button in top-right

**Step 4: Take a screenshot for the record**

```
agent-browser screenshot /tmp/phase2-checkpoint-a.png
agent-browser close
```

---

### Task 6: Create SessionsList component

**Files:**
- Create: `ui/src/views/projects/SessionsList.tsx`

**Step 1: Write the component**

Create `ui/src/views/projects/SessionsList.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { type AmplifierSession, listAmplifierSessions, openTerminal } from '../../api/projects'

interface Props {
  projectId: string
}

function formatTimestamp(ts: string): string {
  const date = new Date(ts)
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

export default function SessionsList({ projectId }: Props) {
  const [sessions, setSessions] = useState<AmplifierSession[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    listAmplifierSessions(projectId)
      .then(setSessions)
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [projectId])

  async function handleNewSession() {
    try {
      await openTerminal(projectId, 'new')
    } catch (err) {
      console.error('Failed to open terminal:', err)
    }
  }

  async function handleOpenSession(sessionId: string) {
    try {
      await openTerminal(projectId, 'resume', sessionId)
    } catch (err) {
      console.error('Failed to resume session:', err)
    }
  }

  if (loading) {
    return <div style={{ padding: 16, color: '#6b7280' }}>Loading sessions...</div>
  }

  return (
    <div style={{ padding: 16 }}>
      {/* New Session button */}
      <button
        onClick={handleNewSession}
        style={{
          fontSize: 13,
          fontWeight: 500,
          padding: '8px 20px',
          marginBottom: 16,
          color: '#14b8a6',
          background: 'transparent',
          border: '1px solid #14b8a6',
          borderRadius: 6,
          cursor: 'pointer',
          transition: 'background 0.15s ease',
        }}
        onMouseEnter={e => (e.currentTarget.style.background = 'rgba(20,184,166,0.08)')}
        onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
      >
        New Session
      </button>

      {/* Empty state */}
      {sessions.length === 0 ? (
        <div style={{ color: '#6b7280', fontSize: 13, paddingTop: 16 }}>
          No Amplifier sessions found for this project.
        </div>
      ) : (
        /* Session rows */
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          {sessions.map(s => (
            <div
              key={s.id}
              style={{
                display: 'flex',
                alignItems: 'center',
                padding: '10px 12px',
                borderBottom: '1px solid #252832',
                gap: 12,
              }}
            >
              <span style={{
                flex: 1,
                fontSize: 13,
                color: '#e5e7eb',
                fontWeight: 500,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}>
                {s.name || s.id}
              </span>
              <span style={{ fontSize: 12, color: '#6b7280', flexShrink: 0 }}>
                {formatTimestamp(s.lastActiveAt || s.createdAt)}
              </span>
              <button
                onClick={() => handleOpenSession(s.id)}
                style={{
                  fontSize: 12,
                  fontWeight: 500,
                  padding: '4px 12px',
                  color: '#14b8a6',
                  background: 'transparent',
                  border: '1px solid #252832',
                  borderRadius: 4,
                  cursor: 'pointer',
                  flexShrink: 0,
                  transition: 'border-color 0.15s ease',
                }}
                onMouseEnter={e => (e.currentTarget.style.borderColor = '#14b8a6')}
                onMouseLeave={e => (e.currentTarget.style.borderColor = '#252832')}
              >
                Open
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
```

**Step 2: Build check**
```
cd ui && npm run build
```
Expected: no TypeScript errors

**Step 3: Commit**
```
git add -A && git commit -m "feat: add SessionsList component for Amplifier sessions"
```

---

### Task 7: Create ProjectDetail component

**Files:**
- Create: `ui/src/views/projects/ProjectDetail.tsx`

> **Note on FileViewer:** `FileViewer` currently takes `{ projectId: string; sessionId: string }` (see `ui/src/views/projects/FileViewer.tsx:57-60`). The component is kept unchanged per design. The Files tab renders `FileViewer` with the first available Amplifier session's ID. If Phase 1 updated the file browsing routes to work at the project level, the `sessionId` prop may be vestigial. If no Amplifier sessions exist, the Files tab shows a fallback message.

**Step 1: Write the component**

Create `ui/src/views/projects/ProjectDetail.tsx`:

```tsx
import { useEffect, useState } from 'react'
import {
  type Project,
  type AmplifierSession,
  getProject,
  listAmplifierSessions,
} from '../../api/projects'
import SessionsList from './SessionsList'
import { ProjectSettingsPanel } from './ProjectSettingsPanel'
import FileViewer from './FileViewer'

interface Props {
  projectId: string
  onBack: () => void
}

type Tab = 'sessions' | 'settings' | 'files'

const TABS: { id: Tab; label: string }[] = [
  { id: 'sessions', label: 'Sessions' },
  { id: 'settings', label: 'Settings' },
  { id: 'files', label: 'Files' },
]

export default function ProjectDetail({ projectId, onBack }: Props) {
  const [project, setProject] = useState<Project | null>(null)
  const [activeTab, setActiveTab] = useState<Tab>('sessions')
  const [sessions, setSessions] = useState<AmplifierSession[]>([])

  useEffect(() => {
    getProject(projectId).then(setProject).catch(console.error)
    listAmplifierSessions(projectId).then(setSessions).catch(() => setSessions([]))
  }, [projectId])

  return (
    <div style={{
      background: '#12141a',
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
    }}>
      {/* Header: back button + project name */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        padding: '16px 24px',
        borderBottom: '1px solid #252832',
        flexShrink: 0,
      }}>
        <button
          onClick={onBack}
          style={{
            fontSize: 13,
            color: '#9ca3af',
            background: 'transparent',
            border: 'none',
            cursor: 'pointer',
            padding: '4px 8px',
          }}
          onMouseEnter={e => (e.currentTarget.style.color = '#ffffff')}
          onMouseLeave={e => (e.currentTarget.style.color = '#9ca3af')}
        >
          &larr; Back
        </button>
        <h1 style={{
          fontSize: 18,
          fontWeight: 600,
          color: '#ffffff',
          margin: 0,
        }}>
          {project?.name ?? 'Loading...'}
        </h1>
      </div>

      {/* Tab bar */}
      <div style={{
        display: 'flex',
        gap: 0,
        padding: '0 24px',
        borderBottom: '1px solid #252832',
        flexShrink: 0,
      }}>
        {TABS.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            style={{
              padding: '10px 16px',
              fontSize: 13,
              fontWeight: 500,
              color: activeTab === tab.id ? '#ffffff' : '#6b7280',
              background: 'transparent',
              border: 'none',
              borderBottom: activeTab === tab.id
                ? '2px solid #14b8a6'
                : '2px solid transparent',
              cursor: 'pointer',
              transition: 'color 0.15s ease',
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div style={{ flex: 1, overflowY: 'auto' }}>
        {activeTab === 'sessions' && (
          <SessionsList projectId={projectId} />
        )}
        {activeTab === 'settings' && (
          <ProjectSettingsPanel projectId={projectId} />
        )}
        {activeTab === 'files' && (
          sessions.length > 0
            ? <FileViewer projectId={projectId} sessionId={sessions[0].id} />
            : (
              <div style={{ padding: 16, color: '#6b7280', fontSize: 13 }}>
                No sessions available for file browsing.
              </div>
            )
        )}
      </div>
    </div>
  )
}
```

**Step 2: Build check**
```
cd ui && npm run build
```
Expected: no TypeScript errors

**Step 3: Commit**
```
git add -A && git commit -m "feat: add ProjectDetail component with Sessions/Settings/Files tabs"
```

---

### Task 8: Wire ProjectDetail into App.tsx

**Files:**
- Modify: `ui/src/views/projects/index.tsx`
- Modify: `ui/src/App.tsx`

**Step 1: Update `index.tsx` to accept and forward `onSelectProject`**

Replace the entire contents of `ui/src/views/projects/index.tsx` with:

```tsx
import ProjectsGrid from './ProjectsGrid'

export default ProjectsGrid
```

**Step 2: Update `App.tsx` to manage project selection state**

In `ui/src/App.tsx`, make three edits:

**(a) Add the import for `ProjectDetail` and `useState` (line 1-2):**

Change:

```tsx
import { useEffect, useState } from 'react'
import ProjectsView from './views/projects'
```

To:

```tsx
import { useEffect, useState } from 'react'
import ProjectsGrid from './views/projects'
import ProjectDetail from './views/projects/ProjectDetail'
```

**(b) Add `selectedProjectId` state inside the `App` component (after line 61):**

After:
```tsx
  const [active, setActive]             = useState<Tab>('projects')
  const [showFeedback, setShowFeedback] = useState(false)
```

Add:
```tsx
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
```

**(c) Replace the projects tab render (line 160):**

Change:
```tsx
        {active === 'projects' && <ProjectsView />}
```

To:
```tsx
        {active === 'projects' && (
          selectedProjectId
            ? <ProjectDetail
                projectId={selectedProjectId}
                onBack={() => setSelectedProjectId(null)}
              />
            : <ProjectsGrid onSelectProject={setSelectedProjectId} />
        )}
```

**Step 3: Build check**
```
cd ui && npm run build
```
Expected: no TypeScript errors. The app now routes between grid and detail views based on `selectedProjectId`.

**Step 4: Commit**
```
git add -A && git commit -m "feat: wire ProjectDetail routing in App.tsx"
```

---

### Task 9: Checkpoint B — Browser verification of detail view

> Verify: clicking a project card navigates to the detail view with Sessions/Settings/Files tabs and a back button.

**Step 1: Open the app**

```
agent-browser open http://localhost:7700
agent-browser snapshot -ic
```

**Step 2: Click a project card body**

Identify a project card from the snapshot and click its body area (not the "New Session" button):

```
agent-browser click @eN
```

(Replace `@eN` with the actual element ref from the snapshot.)

**Step 3: Verify detail view renders**

```
agent-browser snapshot -ic
```

Expected:
- Dark background (`#12141a`) fills the detail view
- "&larr; Back" button visible in top-left
- Project name displayed as a heading
- Tab bar visible with three tabs: "Sessions", "Settings", "Files"
- "Sessions" tab is active (teal underline `#14b8a6`)
- Sessions tab content area visible below

**Step 4: Verify back navigation**

Click the "&larr; Back" button:

```
agent-browser click @eN
agent-browser snapshot -ic
```

Expected: returns to the project card grid.

**Step 5: Screenshot**

```
agent-browser screenshot /tmp/phase2-checkpoint-b.png
agent-browser close
```

---

### Task 10: Checkpoint C — Browser verification of sessions list

> Verify: the Sessions tab shows the session list (or empty state) and the "New Session" button works without JS errors.

**Step 1: Navigate to a project's Sessions tab**

```
agent-browser open http://localhost:7700
agent-browser snapshot -ic
```

Click a project card to enter the detail view:

```
agent-browser click @eN
agent-browser snapshot -ic
```

**Step 2: Verify the Sessions tab content**

Expected (one of):
- **If sessions exist:** A list of session rows, each showing session name, timestamp, and "Open" button
- **If no sessions:** The text "No Amplifier sessions found for this project."

In both cases: "New Session" button visible at the top of the tab content.

**Step 3: Click "New Session" and verify no errors**

```
agent-browser click @eN
```

(Click the "New Session" button ref.)

Expected: no JS console errors. The button calls `POST /api/projects/{id}/open-terminal` with `{mode:"new"}`. If Phase 1 backend is running, a native terminal may launch. If the endpoint returns an error, the console will log it gracefully (no unhandled exceptions).

**Step 4: Screenshot**

```
agent-browser screenshot /tmp/phase2-checkpoint-c.png
agent-browser close
```

---

### Task 11: Delete old files

**Files:**
- Delete: `ui/src/views/projects/terminal/XTermTerminal.tsx`
- Delete: `ui/src/views/projects/terminal/TerminalPanel.tsx`
- Delete: `ui/src/views/projects/terminal/useTerminalSocket.ts`
- Delete: `ui/src/views/projects/terminal/` (directory)
- Delete: `ui/src/views/projects/WorkspaceApp.tsx`
- Delete: `ui/src/views/projects/SessionStats.tsx`

**Step 1: Verify no remaining imports of the files being deleted**

```
cd ui && grep -r "WorkspaceApp\|SessionStats\|terminal/TerminalPanel\|terminal/XTermTerminal\|terminal/useTerminalSocket" src/ --include="*.tsx" --include="*.ts"
```

Expected: **zero matches**. If any matches appear, those files have stale imports that must be removed before deleting.

**Step 2: Delete the files**

```
rm -rf ui/src/views/projects/terminal
rm ui/src/views/projects/WorkspaceApp.tsx
rm ui/src/views/projects/SessionStats.tsx
```

**Step 3: Build check**

```
cd ui && npm run build
```

Expected: no TypeScript errors. All deleted files were unreferenced.

**Step 4: Commit**

```
git add -A && git commit -m "chore: remove WorkspaceApp, SessionStats, and terminal/ (replaced by ProjectsGrid)"
```

---

## Summary

| Task | What | Files |
|------|------|-------|
| 1 | API types + functions | Modify `ui/src/api/projects.ts` |
| 2 | ProjectCard component | Create `ui/src/views/projects/ProjectCard.tsx` |
| 3 | ProjectsGrid component | Create `ui/src/views/projects/ProjectsGrid.tsx` |
| 4 | Wire grid into app | Modify `index.tsx`, `App.tsx` |
| 5 | **Checkpoint A** | Browser: verify grid renders |
| 6 | SessionsList component | Create `ui/src/views/projects/SessionsList.tsx` |
| 7 | ProjectDetail component | Create `ui/src/views/projects/ProjectDetail.tsx` |
| 8 | Wire detail into App.tsx | Modify `index.tsx`, `App.tsx` |
| 9 | **Checkpoint B** | Browser: verify detail + tabs |
| 10 | **Checkpoint C** | Browser: verify sessions + New Session |
| 11 | Delete old files | Remove `terminal/`, `WorkspaceApp.tsx`, `SessionStats.tsx` |
