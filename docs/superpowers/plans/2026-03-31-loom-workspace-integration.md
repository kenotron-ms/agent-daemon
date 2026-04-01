# Loom Workspace Integration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn loom into a unified developer workspace with Projects (PTY coding sessions), Jobs (existing scheduler), and Mirror (existing connectors) behind a single React SPA served by the existing Go binary.

**Architecture:** Copy grove's `apps/web/src/` into `loom/ui/`, extend the root `App.tsx` with a hub tab bar (Projects | Jobs | Mirror), and implement three new Go packages (`internal/workspaces/`, `internal/pty/`, `internal/files/`) that back the Projects domain. Jobs and Mirror consume their existing API routes unchanged. The SPA is built with Vite and embedded into the Go binary via `//go:embed`.

**Tech Stack:** Go 1.25 · bbolt · `github.com/creack/pty` · `github.com/gorilla/websocket` · React 18 · TypeScript · Vite · xterm.js · Zustand · TailwindCSS · Radix UI

**Spec:** `docs/superpowers/specs/2026-03-31-loom-workspace-integration-design.md`

---

## File Map

### New files
| File | Purpose |
|---|---|
| `ui/package.json` | Frontend dependencies (adapted from grove `apps/web`) |
| `ui/tsconfig.json` | TypeScript config |
| `ui/tsconfig.node.json` | Node TypeScript config for Vite |
| `ui/vite.config.ts` | Vite build config — outputs to `ui/dist/` |
| `ui/index.html` | SPA entry point |
| `ui/src/App.tsx` | **Hub navigation** — Projects \| Jobs \| Mirror tab bar |
| `ui/src/views/projects/` | Grove's WorkspaceApp (copied + auth removed) |
| `ui/src/views/jobs/index.tsx` | Jobs view shell |
| `ui/src/views/jobs/JobList.tsx` | Left panel — job list |
| `ui/src/views/jobs/RunDetail.tsx` | Right panel — run history + SSE log |
| `ui/src/views/mirror/index.tsx` | Mirror view shell |
| `ui/src/views/mirror/ConnectorList.tsx` | Left panel — connector list |
| `ui/src/views/mirror/EntityBrowser.tsx` | Right panel — entity list |
| `ui/src/api/loom.ts` | Typed fetch helpers for all loom API routes |
| `internal/workspaces/workspaces.go` | Projects + Sessions CRUD (bbolt) |
| `internal/workspaces/workspaces_test.go` | Unit tests |
| `internal/pty/pty.go` | PTY process manager + WebSocket bridge |
| `internal/pty/pty_test.go` | Unit tests |
| `internal/files/files.go` | Read-only file browser |
| `internal/files/files_test.go` | Unit tests |
| `internal/api/handlers_projects.go` | HTTP handlers for `/api/projects/*` |

### Modified files
| File | Change |
|---|---|
| `internal/store/bbolt.go` | Add `bucketProjects`, `bucketSessions`, `bucketSessionsByProject` + CRUD methods |
| `internal/store/store.go` | Add workspace methods to `Store` interface |
| `internal/api/server.go` | Add `workspaceStore`, `ptyMgr`, `fileBrowser` fields + `SetWorkspaces()` + new routes |
| `web/embed.go` | Change `//go:embed` to embed `ui/dist/` |
| `Makefile` | Add `make ui` target; make `build` depend on `ui` |
| `go.mod` / `go.sum` | Add `github.com/creack/pty` and `github.com/gorilla/websocket` |

### Deleted files
| File | Reason |
|---|---|
| `web/index.html` | Replaced by `ui/dist/index.html` |
| `web/app.js` | Replaced by `ui/dist/assets/` |
| `web/style.css` | Replaced by `ui/dist/assets/` |

---

## Task 1: Feature branch + worktree

**Files:** none

- [ ] **Step 1: Create the feature branch**

```bash
cd /Users/ken/workspace/ms/loom
git checkout -b feature/workspace-integration
```

Expected: `Switched to a new branch 'feature/workspace-integration'`

- [ ] **Step 2: Verify clean state**

```bash
git status
```

Expected: `nothing to commit, working tree clean` (plus the two untracked dirs `cmd/migrate/` and `ui-studio/` — these are fine to ignore)

---

## Task 2: Frontend scaffold

Sets up the `ui/` directory with Vite + React, wires the build output into the Go embed, and updates the Makefile.

**Files:**
- Create: `ui/package.json`
- Create: `ui/tsconfig.json`
- Create: `ui/tsconfig.node.json`
- Create: `ui/vite.config.ts`
- Create: `ui/index.html`
- Modify: `web/embed.go`
- Modify: `Makefile`

- [ ] **Step 1: Create `ui/package.json`**

```json
{
  "name": "@loom/ui",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "@radix-ui/react-dialog": "^1.1.4",
    "@radix-ui/react-dropdown-menu": "^2.1.4",
    "@radix-ui/react-scroll-area": "^1.2.2",
    "@radix-ui/react-separator": "^1.1.1",
    "@radix-ui/react-tabs": "^1.1.2",
    "@radix-ui/react-tooltip": "^1.1.6",
    "@xterm/addon-fit": "^0.10.0",
    "@xterm/addon-search": "^0.15.0",
    "@xterm/xterm": "^5.5.0",
    "clsx": "^2.1.1",
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-resizable-panels": "^2.1.7",
    "react-router-dom": "^6.28.2",
    "tailwind-merge": "^2.6.0",
    "zustand": "^5.0.3"
  },
  "devDependencies": {
    "@types/react": "^18.3.18",
    "@types/react-dom": "^18.3.5",
    "@vitejs/plugin-react": "^4.3.4",
    "autoprefixer": "^10.4.20",
    "postcss": "^8.5.1",
    "tailwindcss": "^3.4.17",
    "typescript": "~5.7.2",
    "vite": "^5.4.11",
    "vitest": "^2.1.9"
  }
}
```

- [ ] **Step 2: Create `ui/tsconfig.json`**

```json
{
  "files": [],
  "references": [
    { "path": "./tsconfig.app.json" },
    { "path": "./tsconfig.node.json" }
  ]
}
```

- [ ] **Step 3: Create `ui/tsconfig.app.json`**

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"]
}
```

- [ ] **Step 4: Create `ui/tsconfig.node.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2023"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "strict": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 5: Create `ui/vite.config.ts`**

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:7700',
      '/ws': { target: 'ws://localhost:7700', ws: true },
    },
  },
})
```

- [ ] **Step 6: Create `ui/index.html`**

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>loom</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 7: Create `ui/src/main.tsx`** (minimal bootstrap)

```typescript
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
```

- [ ] **Step 8: Create `ui/src/index.css`** (Tailwind directives)

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

* {
  box-sizing: border-box;
}

html, body, #root {
  height: 100%;
  margin: 0;
  padding: 0;
  background: #0d1117;
  color: #e6edf3;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}
```

- [ ] **Step 9: Create `ui/tailwind.config.js`**

```javascript
/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: { extend: {} },
  plugins: [],
}
```

- [ ] **Step 10: Create `ui/postcss.config.js`**

```javascript
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
```

- [ ] **Step 11: Update `web/embed.go`** to embed `ui/dist/`

The file currently embeds `web/*.html`, `web/*.js`, `web/*.css`. We need it to embed the Vite build output. Since the embed path must be relative to the package, we'll keep it in `web/` but have the build copy the dist output there — OR restructure embed.go to point at `ui/dist`. The cleanest approach is to change the package to point at a new location:

Replace the contents of `web/embed.go`:

```go
// Package web holds the embedded static web UI.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var files embed.FS

// FS exposes only the built UI files (ui/dist/ contents).
var FS, _ = fs.Sub(files, "dist")
```

Then update `Makefile` to build the React app into `web/dist/` (so the embed path works):

- [ ] **Step 12: Update `Makefile`**

```makefile
BINARY   = loom
DIST     = dist
MODULE   = github.com/ms/amplifier-app-loom
VERSION  = 0.5.2
LDFLAGS  = -ldflags "-X $(MODULE)/internal/api.Version=$(VERSION) -s -w"

.PHONY: build run install-svc uninstall-svc test clean cross ui

ui:
	cd ui && npm install && npm run build
	rm -rf web/dist
	cp -r ui/dist web/dist

build: ui $(DIST)
	go build $(LDFLAGS) -o $(DIST)/$(BINARY) ./cmd/loom/

$(DIST):
	mkdir -p $(DIST)

run: build
	./$(DIST)/$(BINARY) _serve

install-svc: build
	./$(DIST)/$(BINARY) install
	./$(DIST)/$(BINARY) start

uninstall-svc:
	./$(DIST)/$(BINARY) stop || true
	./$(DIST)/$(BINARY) uninstall

test:
	go test ./...

clean:
	rm -rf $(DIST) web/dist ui/dist

cross: ui $(DIST)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-amd64   ./cmd/loom/
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64  go build $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-arm64   ./cmd/loom/
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o $(DIST)/$(BINARY)-darwin-amd64  ./cmd/loom/
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o $(DIST)/$(BINARY)-darwin-arm64  ./cmd/loom/
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o $(DIST)/$(BINARY)-windows-amd64.exe ./cmd/loom/
	ls -lh $(DIST)/
```

- [ ] **Step 13: Add `web/dist` to `.gitignore`**

```bash
echo "web/dist/" >> .gitignore
echo "ui/dist/" >> .gitignore
echo "ui/node_modules/" >> .gitignore
```

- [ ] **Step 14: Delete the old vanilla JS files**

```bash
rm web/index.html web/app.js web/style.css
```

- [ ] **Step 15: Create a placeholder `ui/src/App.tsx`** (just enough to build)

```typescript
export default function App() {
  return <div style={{ color: 'white', padding: 32 }}>loom — scaffold</div>
}
```

- [ ] **Step 16: Verify frontend builds**

```bash
make ui
```

Expected: `npm install` completes, `vite build` outputs to `ui/dist/`, `cp` copies to `web/dist/`.

- [ ] **Step 17: Verify Go still compiles with new embed target**

```bash
go build ./...
```

Expected: no errors. If `web/dist` is empty, add a `.gitkeep` placeholder: `touch web/dist/.gitkeep` (not needed if `make ui` already ran).

- [ ] **Step 18: Commit**

```bash
git add -A
git commit -m "chore: scaffold ui/ directory and wire Vite build into Go embed"
```

---

## Task 3: Hub navigation (`ui/src/App.tsx`)

Replaces the scaffold App.tsx with the real hub: a top tab bar with Projects, Jobs, and Mirror modes. Each mode renders a placeholder view for now — the real views come in later tasks.

**Files:**
- Modify: `ui/src/App.tsx`
- Create: `ui/src/views/projects/index.tsx` (placeholder)
- Create: `ui/src/views/jobs/index.tsx` (placeholder)
- Create: `ui/src/views/mirror/index.tsx` (placeholder)

- [ ] **Step 1: Write the hub navigation test**

Create `ui/src/App.test.tsx`:

```typescript
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect } from 'vitest'
import App from './App'

describe('App hub navigation', () => {
  it('renders all three tabs', () => {
    render(<App />)
    expect(screen.getByRole('tab', { name: 'Projects' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Jobs' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Mirror' })).toBeInTheDocument()
  })

  it('defaults to Projects tab', () => {
    render(<App />)
    expect(screen.getByRole('tab', { name: 'Projects' })).toHaveAttribute('aria-selected', 'true')
  })

  it('switches to Jobs tab on click', async () => {
    render(<App />)
    await userEvent.click(screen.getByRole('tab', { name: 'Jobs' }))
    expect(screen.getByRole('tab', { name: 'Jobs' })).toHaveAttribute('aria-selected', 'true')
  })
})
```

Install test deps first:
```bash
cd ui && npm install --save-dev @testing-library/react @testing-library/user-event @testing-library/jest-dom jsdom
```

Add to `ui/vite.config.ts`:
```typescript
test: {
  environment: 'jsdom',
  setupFiles: ['./src/test-setup.ts'],
},
```

Create `ui/src/test-setup.ts`:
```typescript
import '@testing-library/jest-dom'
```

- [ ] **Step 2: Run test — verify it fails**

```bash
cd ui && npx vitest run src/App.test.tsx
```

Expected: FAIL — `App` doesn't render tabs yet.

- [ ] **Step 3: Write `ui/src/App.tsx`**

```typescript
import { useState } from 'react'
import ProjectsView from './views/projects'
import JobsView from './views/jobs'
import MirrorView from './views/mirror'

type Tab = 'projects' | 'jobs' | 'mirror'

const TABS: { id: Tab; label: string }[] = [
  { id: 'projects', label: 'Projects' },
  { id: 'jobs',     label: 'Jobs' },
  { id: 'mirror',   label: 'Mirror' },
]

export default function App() {
  const [active, setActive] = useState<Tab>('projects')

  return (
    <div className="flex flex-col h-full bg-[#0d1117]">
      {/* Top nav */}
      <nav className="flex items-center bg-[#161b22] border-b border-[#30363d] px-3 h-9 shrink-0">
        <span className="text-[#8b949e] text-xs font-semibold mr-4">loom</span>
        <div className="flex h-full" role="tablist">
          {TABS.map(tab => (
            <button
              key={tab.id}
              role="tab"
              aria-selected={active === tab.id}
              onClick={() => setActive(tab.id)}
              className={[
                'px-3 h-full text-xs border-b-2 transition-colors',
                active === tab.id
                  ? 'border-[#58a6ff] text-[#e6edf3]'
                  : 'border-transparent text-[#8b949e] hover:text-[#e6edf3]',
              ].join(' ')}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </nav>

      {/* Mode content */}
      <div className="flex-1 overflow-hidden">
        {active === 'projects' && <ProjectsView />}
        {active === 'jobs'     && <JobsView />}
        {active === 'mirror'   && <MirrorView />}
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Create placeholder views**

`ui/src/views/projects/index.tsx`:
```typescript
export default function ProjectsView() {
  return <div className="p-8 text-[#8b949e]">Projects — coming soon</div>
}
```

`ui/src/views/jobs/index.tsx`:
```typescript
export default function JobsView() {
  return <div className="p-8 text-[#8b949e]">Jobs — coming soon</div>
}
```

`ui/src/views/mirror/index.tsx`:
```typescript
export default function MirrorView() {
  return <div className="p-8 text-[#8b949e]">Mirror — coming soon</div>
}
```

- [ ] **Step 5: Run test — verify it passes**

```bash
cd ui && npx vitest run src/App.test.tsx
```

Expected: PASS — 3 tests.

- [ ] **Step 6: Verify full build still works**

```bash
cd /Users/ken/workspace/ms/loom && make ui && go build ./...
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(ui): add hub navigation with Projects | Jobs | Mirror tabs"
```

---

## Task 4: `internal/workspaces/` — Projects + Sessions CRUD

Extends the bbolt store with two new bucket groups and implements the Projects/Sessions service that the API handlers will call.

**Files:**
- Modify: `internal/store/store.go` (add workspace methods to interface)
- Modify: `internal/store/bbolt.go` (add buckets + CRUD implementations)
- Create: `internal/workspaces/workspaces.go`
- Create: `internal/workspaces/workspaces_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/workspaces/workspaces_test.go`:

```go
package workspaces_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"

	"github.com/ms/amplifier-app-loom/internal/workspaces"
)

func openTestDB(t *testing.T) *bolt.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateProject(t *testing.T) {
	svc := workspaces.New(openTestDB(t))
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "loom", "/tmp/loom")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if p.Name != "loom" {
		t.Fatalf("expected name loom, got %s", p.Name)
	}
	if p.Path != "/tmp/loom" {
		t.Fatalf("expected path /tmp/loom, got %s", p.Path)
	}
}

func TestListProjects(t *testing.T) {
	svc := workspaces.New(openTestDB(t))
	ctx := context.Background()

	svc.CreateProject(ctx, "alpha", "/tmp/alpha")
	svc.CreateProject(ctx, "beta", "/tmp/beta")

	projects, err := svc.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
}

func TestDeleteProject(t *testing.T) {
	svc := workspaces.New(openTestDB(t))
	ctx := context.Background()

	p, _ := svc.CreateProject(ctx, "toDelete", "/tmp/del")
	if err := svc.DeleteProject(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	got, err := svc.GetProject(ctx, p.ID)
	if err == nil && got != nil {
		t.Fatal("expected project to be deleted")
	}
}

func TestCreateSession(t *testing.T) {
	svc := workspaces.New(openTestDB(t))
	ctx := context.Background()

	dir := t.TempDir()
	p, _ := svc.CreateProject(ctx, "proj", dir)

	s, err := svc.CreateSession(ctx, p.ID, "main", dir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if s.ProjectID != p.ID {
		t.Fatalf("expected projectID %s, got %s", p.ID, s.ProjectID)
	}
	if s.Status != "idle" {
		t.Fatalf("expected status idle, got %s", s.Status)
	}
}

func TestListSessionsForProject(t *testing.T) {
	svc := workspaces.New(openTestDB(t))
	ctx := context.Background()

	dir := t.TempDir()
	p, _ := svc.CreateProject(ctx, "proj", dir)
	svc.CreateSession(ctx, p.ID, "main", dir)
	svc.CreateSession(ctx, p.ID, "feature", filepath.Join(dir, "feature"))

	sessions, err := svc.ListSessions(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}
```

- [ ] **Step 2: Run test — verify it fails**

```bash
go test ./internal/workspaces/... 2>&1 | head -20
```

Expected: `cannot find package` or compile error — package doesn't exist yet.

- [ ] **Step 3: Create `internal/workspaces/workspaces.go`**

```go
package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketProjects          = []byte("projects")
	bucketSessions          = []byte("sessions")
	bucketSessionsByProject = []byte("sessions_by_project")
)

// Project is a codebase on disk with one or more worktree sessions.
type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Path           string `json:"path"` // absolute path on disk
	CreatedAt      int64  `json:"createdAt"`
	LastActivityAt int64  `json:"lastActivityAt"`
}

// Session is a git worktree within a project, backed by a persistent PTY process.
type Session struct {
	ID           string  `json:"id"`
	ProjectID    string  `json:"projectId"`
	Name         string  `json:"name"`         // e.g. branch name
	WorktreePath string  `json:"worktreePath"` // absolute path to git worktree
	ProcessID    *string `json:"processId"`    // nil when no PTY is running
	CreatedAt    int64   `json:"createdAt"`
	Status       string  `json:"status"` // "idle" | "active" | "stopped"
}

// Service is the workspace CRUD layer backed by bbolt.
type Service struct {
	db *bolt.DB
}

// New creates a Service and initialises the required bbolt buckets.
func New(db *bolt.DB) *Service {
	db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketProjects, bucketSessions, bucketSessionsByProject} {
			tx.CreateBucketIfNotExists(b)
		}
		return nil
	})
	return &Service{db: db}
}

// ── Projects ──────────────────────────────────────────────────────────────────

func (s *Service) CreateProject(_ context.Context, name, path string) (*Project, error) {
	p := &Project{
		ID:             uuid.New().String(),
		Name:           name,
		Path:           path,
		CreatedAt:      time.Now().Unix(),
		LastActivityAt: time.Now().Unix(),
	}
	return p, s.db.Update(func(tx *bolt.Tx) error {
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketProjects).Put([]byte(p.ID), data)
	})
}

func (s *Service) GetProject(_ context.Context, id string) (*Project, error) {
	var p Project
	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketProjects).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project %s not found", id)
		}
		return json.Unmarshal(data, &p)
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) ListProjects(_ context.Context) ([]*Project, error) {
	var projects []*Project
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketProjects).ForEach(func(_, v []byte) error {
			var p Project
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}
			projects = append(projects, &p)
			return nil
		})
	})
	if projects == nil {
		projects = []*Project{}
	}
	return projects, err
}

func (s *Service) UpdateProject(_ context.Context, id, name string) (*Project, error) {
	p, err := s.GetProject(context.Background(), id)
	if err != nil {
		return nil, err
	}
	p.Name = name
	return p, s.db.Update(func(tx *bolt.Tx) error {
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketProjects).Put([]byte(p.ID), data)
	})
}

func (s *Service) DeleteProject(_ context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(bucketProjects).Delete([]byte(id)); err != nil {
			return err
		}
		// also delete all sessions for this project from the index
		prefix := []byte(id + "/")
		idxBucket := tx.Bucket(bucketSessionsByProject)
		c := idxBucket.Cursor()
		for k, _ := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = c.Next() {
			idxBucket.Delete(k)
		}
		return nil
	})
}

func (s *Service) TouchProject(_ context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketProjects).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project %s not found", id)
		}
		var p Project
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		p.LastActivityAt = time.Now().Unix()
		updated, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketProjects).Put([]byte(id), updated)
	})
}

// ── Sessions ──────────────────────────────────────────────────────────────────

func (s *Service) CreateSession(_ context.Context, projectID, name, worktreePath string) (*Session, error) {
	sess := &Session{
		ID:           uuid.New().String(),
		ProjectID:    projectID,
		Name:         name,
		WorktreePath: worktreePath,
		ProcessID:    nil,
		CreatedAt:    time.Now().Unix(),
		Status:       "idle",
	}
	return sess, s.db.Update(func(tx *bolt.Tx) error {
		data, err := json.Marshal(sess)
		if err != nil {
			return err
		}
		if err := tx.Bucket(bucketSessions).Put([]byte(sess.ID), data); err != nil {
			return err
		}
		// update index: projectID/sessionID → ""
		indexKey := []byte(projectID + "/" + sess.ID)
		return tx.Bucket(bucketSessionsByProject).Put(indexKey, []byte(""))
	})
}

func (s *Service) GetSession(_ context.Context, id string) (*Session, error) {
	var sess Session
	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketSessions).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("session %s not found", id)
		}
		return json.Unmarshal(data, &sess)
	})
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Service) ListSessions(_ context.Context, projectID string) ([]*Session, error) {
	var sessions []*Session
	err := s.db.View(func(tx *bolt.Tx) error {
		prefix := []byte(projectID + "/")
		c := tx.Bucket(bucketSessionsByProject).Cursor()
		for k, _ := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = c.Next() {
			sessionID := string(k[len(prefix):])
			data := tx.Bucket(bucketSessions).Get([]byte(sessionID))
			if data == nil {
				continue
			}
			var sess Session
			if err := json.Unmarshal(data, &sess); err != nil {
				return err
			}
			sessions = append(sessions, &sess)
		}
		return nil
	})
	if sessions == nil {
		sessions = []*Session{}
	}
	return sessions, err
}

func (s *Service) UpdateSessionStatus(_ context.Context, id, status string, processID *string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketSessions).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("session %s not found", id)
		}
		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			return err
		}
		sess.Status = status
		sess.ProcessID = processID
		updated, err := json.Marshal(sess)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketSessions).Put([]byte(id), updated)
	})
}

func (s *Service) DeleteSession(_ context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketSessions).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("session %s not found", id)
		}
		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			return err
		}
		if err := tx.Bucket(bucketSessions).Delete([]byte(id)); err != nil {
			return err
		}
		indexKey := []byte(sess.ProjectID + "/" + id)
		return tx.Bucket(bucketSessionsByProject).Delete(indexKey)
	})
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/workspaces/... -v
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add internal/workspaces package (Projects + Sessions CRUD, bbolt)"
```

---

## Task 5: `internal/pty/` — PTY process manager + WebSocket bridge

Manages PTY processes keyed by a unique ID. Each process is a shell started in a project's worktree directory. The WebSocket handler bridges xterm.js to the PTY I/O.

**Files:**
- Create: `internal/pty/pty.go`
- Create: `internal/pty/pty_test.go`

- [ ] **Step 1: Add Go dependencies**

```bash
go get github.com/creack/pty@latest
go get github.com/gorilla/websocket@latest
```

Expected: `go.mod` and `go.sum` updated.

- [ ] **Step 2: Write the failing tests**

Create `internal/pty/pty_test.go`:

```go
package pty_test

import (
	"testing"
	"time"

	loompty "github.com/ms/amplifier-app-loom/internal/pty"
)

func TestSpawnAndKill(t *testing.T) {
	mgr := loompty.NewManager()

	id, err := mgr.Spawn("test-proc", t.TempDir(), []string{"/bin/sh"})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty process ID")
	}

	// process should be alive
	if !mgr.IsAlive(id) {
		t.Fatal("expected process to be alive after spawn")
	}

	if err := mgr.Kill(id); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	// give process a moment to exit
	time.Sleep(50 * time.Millisecond)
	if mgr.IsAlive(id) {
		t.Fatal("expected process to be dead after kill")
	}
}

func TestSpawnDeduplicated(t *testing.T) {
	mgr := loompty.NewManager()
	dir := t.TempDir()

	id1, _ := mgr.Spawn("proc", dir, []string{"/bin/sh"})
	id2, _ := mgr.Spawn("proc", dir, []string{"/bin/sh"})

	// same key → same process ID returned
	if id1 != id2 {
		t.Fatalf("expected deduplicated process, got %s vs %s", id1, id2)
	}

	mgr.Kill(id1)
}
```

- [ ] **Step 3: Run test — verify it fails**

```bash
go test ./internal/pty/... 2>&1 | head -10
```

Expected: `cannot find package` — package doesn't exist yet.

- [ ] **Step 4: Create `internal/pty/pty.go`**

```go
package pty

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// Process wraps a running PTY process.
type Process struct {
	ID  string
	ptm *os.File // PTY master
	cmd interface{ Wait() error }
}

// Manager holds all active PTY processes keyed by a stable ID.
// The same key always returns the same process while it is alive.
type Manager struct {
	mu      sync.Mutex
	procs   map[string]*Process // key → process
	keyToID map[string]string   // stable key → processID
}

// NewManager returns an initialised Manager.
func NewManager() *Manager {
	return &Manager{
		procs:   make(map[string]*Process),
		keyToID: make(map[string]string),
	}
}

// Spawn starts a new PTY process for the given key and working directory.
// If a live process already exists for this key, its ID is returned without
// spawning a new process (deduplication).
func (m *Manager) Spawn(key, workDir string, argv []string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id, ok := m.keyToID[key]; ok {
		if p, alive := m.procs[id]; alive && m.isAliveUnlocked(p) {
			return id, nil
		}
		delete(m.keyToID, key)
	}

	cmd := buildCmd(workDir, argv)
	ptm, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("pty start: %w", err)
	}

	id := key // use key as stable ID for simplicity
	proc := &Process{ID: id, ptm: ptm, cmd: cmd}
	m.procs[id] = proc
	m.keyToID[key] = id

	// reap process when it exits
	go func() {
		cmd.Wait()
		m.mu.Lock()
		delete(m.procs, id)
		m.mu.Unlock()
	}()

	return id, nil
}

// IsAlive reports whether a process with the given ID is currently running.
func (m *Manager) IsAlive(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.procs[id]
	return ok && m.isAliveUnlocked(p)
}

func (m *Manager) isAliveUnlocked(p *Process) bool {
	_, ok := m.procs[p.ID]
	return ok
}

// Kill terminates the process and removes it from the registry.
func (m *Manager) Kill(id string) error {
	m.mu.Lock()
	p, ok := m.procs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("process %s not found", id)
	}
	delete(m.procs, id)
	m.mu.Unlock()
	return p.ptm.Close()
}

// ── WebSocket bridge ──────────────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeWS upgrades an HTTP connection to WebSocket and bridges it to the PTY
// identified by processID. Blocks until the connection is closed.
func (m *Manager) ServeWS(w http.ResponseWriter, r *http.Request, processID string) {
	m.mu.Lock()
	p, ok := m.procs[processID]
	m.mu.Unlock()
	if !ok {
		http.Error(w, "process not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// PTY → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := p.ptm.Read(buf)
			if err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// WebSocket → PTY stdin
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if _, err := io.WriteString(p.ptm, string(msg)); err != nil {
			return
		}
	}
}
```

Create `internal/pty/cmd_unix.go` (build constraint for the PTY fork):

```go
//go:build !windows

package pty

import (
	"os/exec"
)

func buildCmd(workDir string, argv []string) *exec.Cmd {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = workDir
	cmd.Env = append(cmd.Environ(), "TERM=xterm-256color")
	return cmd
}

// Ensure *exec.Cmd satisfies the Wait interface used in Spawn.
func (c *exec.Cmd) Wait() error { return c.Wait() }
```

Wait — `*exec.Cmd` already has a `Wait() error` method. The `cmd interface{ Wait() error }` in `Process` struct is satisfied by `*exec.Cmd` directly. Simplify:

Replace the `cmd interface{ Wait() error }` field in `Process` with `*exec.Cmd`. Also no separate cmd_unix.go needed — use `exec.Command` directly. Update `pty.go`:

The `buildCmd` function can be inlined into `Spawn`:
```go
cmd := exec.Command(argv[0], argv[1:]...)
cmd.Dir = workDir
cmd.Env = append(os.Environ(), "TERM=xterm-256color")
```

Add `os/exec` and `os` to imports. Remove the `cmd interface{ Wait() error }` in `Process`; use `cmd *exec.Cmd` and store it to call `Kill()`:

```go
type Process struct {
	ID  string
	ptm *os.File
	cmd *exec.Cmd
}
```

And in `Kill`:
```go
p.cmd.Process.Kill()
p.ptm.Close()
```

- [ ] **Step 5: Run tests — verify they pass**

```bash
go test ./internal/pty/... -v
```

Expected: `TestSpawnAndKill` PASS, `TestSpawnDeduplicated` PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: add internal/pty package (PTY manager + WebSocket bridge)"
```

---

## Task 6: `internal/files/` — read-only file browser

Serves directory listings and file contents scoped to a project root. All paths are validated to prevent traversal.

**Files:**
- Create: `internal/files/files.go`
- Create: `internal/files/files_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/files/files_test.go`:

```go
package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ms/amplifier-app-loom/internal/files"
)

func makeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0644)
	os.MkdirAll(filepath.Join(root, "internal"), 0755)
	os.WriteFile(filepath.Join(root, "internal", "util.go"), []byte("package internal"), 0644)
	return root
}

func TestListRoot(t *testing.T) {
	root := makeTree(t)
	b := files.New(root)

	entries, err := b.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestListSubdir(t *testing.T) {
	root := makeTree(t)
	b := files.New(root)

	entries, err := b.List("internal")
	if err != nil {
		t.Fatalf("List internal: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "util.go" {
		t.Fatalf("expected util.go, got %s", entries[0].Name)
	}
}

func TestReadFile(t *testing.T) {
	root := makeTree(t)
	b := files.New(root)

	data, err := b.Read("main.go")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(data) != "package main" {
		t.Fatalf("unexpected content: %s", data)
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	root := makeTree(t)
	b := files.New(root)

	_, err := b.List("../../../etc")
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}

	_, err = b.Read("../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/files/... 2>&1 | head -10
```

Expected: `cannot find package`.

- [ ] **Step 3: Create `internal/files/files.go`**

```go
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Entry is a single file or directory listing item.
type Entry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// Browser provides path-scoped file access rooted at a single directory.
type Browser struct {
	root string
}

// New creates a Browser rooted at root. root must be an absolute path.
func New(root string) *Browser {
	return &Browser{root: filepath.Clean(root)}
}

// resolve validates rel and returns its absolute path within root.
// Returns an error if the resolved path escapes root (traversal attempt).
func (b *Browser) resolve(rel string) (string, error) {
	abs := filepath.Clean(filepath.Join(b.root, rel))
	if abs != b.root && !strings.HasPrefix(abs, b.root+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside root", rel)
	}
	return abs, nil
}

// List returns directory entries at rel (relative to root).
// An empty string returns the root directory listing.
func (b *Browser) List(rel string) ([]Entry, error) {
	abs, err := b.resolve(rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		var size int64
		if info != nil && !e.IsDir() {
			size = info.Size()
		}
		result = append(result, Entry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  size,
		})
	}
	return result, nil
}

// Read returns the contents of the file at rel (relative to root).
func (b *Browser) Read(rel string) ([]byte, error) {
	abs, err := b.resolve(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/files/... -v
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add internal/files package (scoped read-only file browser)"
```

---

## Task 7: Wire Projects API into the server

Adds the `workspaces`, `ptyMgr`, and file browser to the Server, registers all new routes, and implements the HTTP handlers.

**Files:**
- Modify: `internal/api/server.go`
- Create: `internal/api/handlers_projects.go`

- [ ] **Step 1: Write a failing handler test**

Create `internal/api/handlers_projects_test.go`:

```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/ms/amplifier-app-loom/internal/api"
	"github.com/ms/amplifier-app-loom/internal/config"
	"github.com/ms/amplifier-app-loom/internal/files"
	"github.com/ms/amplifier-app-loom/internal/pty"
	"github.com/ms/amplifier-app-loom/internal/store"
	"github.com/ms/amplifier-app-loom/internal/workspaces"
)

func newTestServer(t *testing.T) *api.Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	boltStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	cfg := &config.Config{}
	srv := api.NewServer(cfg, boltStore, nil, nil, time.Now(), nil)
	srv.SetWorkspaces(workspaces.New(db), pty.NewManager(), files.New(t.TempDir()))
	return srv
}

func TestCreateAndListProjects(t *testing.T) {
	srv := newTestServer(t)

	// create
	body := `{"name":"myproject","path":"/tmp/myproject"}`
	req := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// list
	req2 := httptest.NewRequest("GET", "/api/projects", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
	var projects []map[string]any
	json.NewDecoder(w2.Body).Decode(&projects)
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
}
```

- [ ] **Step 2: Run test — verify it fails**

```bash
go test ./internal/api/... -run TestCreateAndListProjects 2>&1 | head -20
```

Expected: compile error — `SetWorkspaces` doesn't exist yet / `ServeHTTP` not implemented.

- [ ] **Step 3: Add `SetWorkspaces` and new fields to `internal/api/server.go`**

Add these imports to `server.go`:
```go
"github.com/ms/amplifier-app-loom/internal/files"
"github.com/ms/amplifier-app-loom/internal/pty"
"github.com/ms/amplifier-app-loom/internal/workspaces"
```

Add fields to the `Server` struct:
```go
workspaceStore *workspaces.Service
ptyMgr         *pty.Manager
fileBrowser    *files.Browser
```

Add method after `SetMirror`:
```go
// SetWorkspaces wires the workspace subsystem (projects, PTY, files) into the server.
func (s *Server) SetWorkspaces(ws *workspaces.Service, mgr *pty.Manager, fb *files.Browser) {
	s.workspaceStore = ws
	s.ptyMgr = mgr
	s.fileBrowser = fb
}
```

Add a `ServeHTTP` method so the test can use `httptest`:
```go
// ServeHTTP implements http.Handler so the server can be used in tests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	mux.ServeHTTP(w, r)
}
```

Add new routes inside `registerRoutes` after the mirror routes:
```go
// Projects
mux.HandleFunc("GET /api/projects", s.listProjects)
mux.HandleFunc("POST /api/projects", s.createProject)
mux.HandleFunc("GET /api/projects/{id}", s.getProject)
mux.HandleFunc("PATCH /api/projects/{id}", s.updateProject)
mux.HandleFunc("DELETE /api/projects/{id}", s.deleteProject)

// Sessions
mux.HandleFunc("GET /api/projects/{id}/sessions", s.listSessions)
mux.HandleFunc("POST /api/projects/{id}/sessions", s.createSession)
mux.HandleFunc("DELETE /api/projects/{id}/sessions/{sid}", s.deleteSession)

// Terminal
mux.HandleFunc("POST /api/projects/{id}/sessions/{sid}/terminal", s.spawnTerminal)
mux.HandleFunc("/api/terminal/{processId}", s.handleTerminalWS)

// Files + Stats
mux.HandleFunc("GET /api/projects/{id}/sessions/{sid}/files", s.listFiles)
mux.HandleFunc("GET /api/projects/{id}/sessions/{sid}/files/{path...}", s.readFile)
mux.HandleFunc("GET /api/projects/{id}/sessions/{sid}/stats", s.getSessionStats)
```

- [ ] **Step 4: Create `internal/api/handlers_projects.go`**

```go
package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ms/amplifier-app-loom/internal/workspaces"
)

// ── Projects ──────────────────────────────────────────────────────────────────

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.workspaceStore.ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" || req.Path == "" {
		writeError(w, http.StatusBadRequest, "name and path are required")
		return
	}
	p, err := s.workspaceStore.CreateProject(r.Context(), req.Name, req.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.workspaceStore.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	p, err := s.workspaceStore.UpdateProject(r.Context(), id, req.Name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// kill all PTY processes for sessions under this project
	sessions, _ := s.workspaceStore.ListSessions(r.Context(), id)
	for _, sess := range sessions {
		if sess.ProcessID != nil {
			s.ptyMgr.Kill(*sess.ProcessID)
		}
	}
	if err := s.workspaceStore.DeleteProject(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Sessions ──────────────────────────────────────────────────────────────────

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sessions, err := s.workspaceStore.ListSessions(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	var req struct {
		Name         string `json:"name"`
		WorktreePath string `json:"worktreePath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.WorktreePath == "" {
		writeError(w, http.StatusBadRequest, "worktreePath is required")
		return
	}
	// create git worktree if it doesn't exist
	if _, err := os.Stat(req.WorktreePath); os.IsNotExist(err) {
		p, err := s.workspaceStore.GetProject(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		out, err := exec.CommandContext(r.Context(), "git", "-C", p.Path, "worktree", "add", req.WorktreePath, req.Name).CombinedOutput()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "git worktree add: "+string(out))
			return
		}
	}
	sess, err := s.workspaceStore.CreateSession(r.Context(), projectID, req.Name, req.WorktreePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("sid")
	sess, err := s.workspaceStore.GetSession(r.Context(), sid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if sess.ProcessID != nil {
		s.ptyMgr.Kill(*sess.ProcessID)
	}
	if err := s.workspaceStore.DeleteSession(r.Context(), sid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Terminal ──────────────────────────────────────────────────────────────────

func (s *Server) spawnTerminal(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("sid")
	sess, err := s.workspaceStore.GetSession(r.Context(), sid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	key := sess.ProjectID + "::" + sess.WorktreePath
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	processID, err := s.ptyMgr.Spawn(key, sess.WorktreePath, []string{shell})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.workspaceStore.UpdateSessionStatus(r.Context(), sid, "active", &processID)
	writeJSON(w, http.StatusOK, map[string]string{"processId": processID})
}

func (s *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	processID := r.PathValue("processId")
	s.ptyMgr.ServeWS(w, r, processID)
}

// ── Files ─────────────────────────────────────────────────────────────────────

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("sid")
	sess, err := s.workspaceStore.GetSession(r.Context(), sid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	rel := r.URL.Query().Get("path")
	browser := fileBrowserForSession(sess)
	entries, err := browser.List(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) readFile(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("sid")
	path := r.PathValue("path")
	sess, err := s.workspaceStore.GetSession(r.Context(), sid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	browser := fileBrowserForSession(sess)
	data, err := browser.Read(path)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
}

func fileBrowserForSession(sess *workspaces.Session) interface {
	List(string) (interface{}, error)
	Read(string) ([]byte, error)
} {
	// import cycle avoidance: create a fresh browser scoped to the session's worktree
	// files.New is imported at package level via the files import at the top of the file
	return nil // replaced in real implementation — see note below
}
```

> **Note:** The `fileBrowserForSession` helper above is a placeholder. In the real implementation, import `github.com/ms/amplifier-app-loom/internal/files` at the top of `handlers_projects.go` and replace the helper body with `return files.New(sess.WorktreePath)`. The return type is `*files.Browser`. The handler code for `listFiles` and `readFile` should call `files.New(sess.WorktreePath).List(rel)` and `.Read(path)` directly.

The complete `listFiles` and `readFile` using the files package directly:

```go
func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("sid")
	sess, err := s.workspaceStore.GetSession(r.Context(), sid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	rel := r.URL.Query().Get("path")
	entries, err := files.New(sess.WorktreePath).List(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) readFile(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("sid")
	path := r.PathValue("path")
	sess, err := s.workspaceStore.GetSession(r.Context(), sid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	data, err := files.New(sess.WorktreePath).Read(path)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
}
```

For `getSessionStats`, read `~/.amplifier` session logs:

```go
func (s *Server) getSessionStats(w http.ResponseWriter, r *http.Request) {
	// Minimal implementation: return placeholder stats
	// Full implementation reads events.jsonl from ~/.amplifier/projects/<slug>/sessions/<id>/events.jsonl
	// and counts token usage and tool calls from JSONL event stream.
	writeJSON(w, http.StatusOK, map[string]any{
		"tokens": 0,
		"tools":  0,
		"note":   "stats not yet implemented",
	})
}
```

- [ ] **Step 5: Import the `files` package in `handlers_projects.go`** and clean up the placeholder:

The top of `handlers_projects.go` should import:
```go
import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"

	"github.com/ms/amplifier-app-loom/internal/files"
	"github.com/ms/amplifier-app-loom/internal/workspaces"
)
```

- [ ] **Step 6: Run the test — verify it passes**

```bash
go test ./internal/api/... -run TestCreateAndListProjects -v
```

Expected: PASS.

- [ ] **Step 7: Run all Go tests**

```bash
go test ./...
```

Expected: all tests pass, no regressions.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: wire Projects API into server (CRUD, PTY, files, sessions)"
```

---

## Task 8: Wire workspaces into the daemon startup

The `internal/workspaces/` service and `internal/pty/` manager need to be constructed and passed to the server at daemon startup.

**Files:**
- Modify: `cmd/loom/main.go` (or wherever the daemon constructs the `Server`)

- [ ] **Step 1: Find the daemon startup file**

```bash
grep -rn "NewServer\|SetMirror" cmd/ --include="*.go" | head -20
```

This will show where `NewServer` and `SetMirror` are called — that's where to add `SetWorkspaces`.

- [ ] **Step 2: Add workspace construction next to mirror construction**

In the file that calls `srv.SetMirror(...)`, add immediately after:

```go
// workspaces
wsService := workspaces.New(boltStore.DB())
ptyMgr := pty.NewManager()
srv.SetWorkspaces(wsService, ptyMgr, nil) // fileBrowser is per-session, no global needed
```

Imports to add:
```go
"github.com/ms/amplifier-app-loom/internal/pty"
"github.com/ms/amplifier-app-loom/internal/workspaces"
```

- [ ] **Step 3: Build and verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: wire workspaces/pty into daemon startup"
```

---

## Task 9: Copy grove's Projects view + remove auth

Copies the relevant source files from amplifier-grove's `apps/web/src/` into `loom/ui/src/views/projects/`, removes auth guards, and patches the API base URL.

**Files:**
- Copy from grove: `apps/web/src/components/` → `ui/src/views/projects/components/`
- Copy from grove: `apps/web/src/store/` → `ui/src/store/`
- Copy from grove: `apps/web/src/api/client.ts` → `ui/src/api/client.ts`
- Modify: `ui/src/api/client.ts` (patch base URL)
- Modify: `ui/src/views/projects/index.tsx` (wire in WorkspaceApp, drop auth)

- [ ] **Step 1: Copy grove's source into the projects view**

```bash
GROVE=/Users/ken/workspace/ms/amplifier-grove
UI=/Users/ken/workspace/ms/loom/ui/src

# Copy components (terminal, file browser, project picker, etc.)
cp -r $GROVE/apps/web/src/components $UI/views/projects/components

# Copy store (Zustand state)
cp -r $GROVE/apps/web/src/store $UI/store

# Copy hooks
cp -r $GROVE/apps/web/src/hooks $UI/hooks 2>/dev/null || true

# Copy types
cp -r $GROVE/apps/web/src/types $UI/types 2>/dev/null || true

# Copy api client
cp $GROVE/apps/web/src/api/client.ts $UI/api/client.ts 2>/dev/null || true
mkdir -p $UI/api
cp $GROVE/apps/web/src/api/*.ts $UI/api/ 2>/dev/null || true

# Copy the main workspace app component
cp $GROVE/apps/web/src/WorkspaceApp.tsx $UI/views/projects/WorkspaceApp.tsx 2>/dev/null || \
cp $GROVE/apps/web/src/App.tsx $UI/views/projects/WorkspaceApp.tsx
```

- [ ] **Step 2: Patch the API base URL in `ui/src/api/client.ts`**

Find the line that sets the base URL (it will reference `localhost:3001` or `VITE_API_URL`). Replace with loom's port:

```typescript
// Before (grove):
const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:3001'

// After (loom):
const BASE_URL = import.meta.env.VITE_API_URL ?? ''  // same origin — loom serves SPA + API on :7700
```

Using an empty string means all `/api/...` requests go to the same origin as the page — this works because loom's Go server serves both the SPA and the API on port 7700.

- [ ] **Step 3: Remove auth guards from the WorkspaceApp path**

In grove's `App.tsx` (now copied as `WorkspaceApp.tsx`), remove:
- The `ProtectedRoute` wrapper component
- The `/login` route
- The `/auth/callback` route

The simplified entry point for projects view (`ui/src/views/projects/index.tsx`):

```typescript
import WorkspaceApp from './WorkspaceApp'

// Remove any ProtectedRoute, AuthProvider, or login redirect logic.
// Loom is auth-free — render WorkspaceApp directly.
export default function ProjectsView() {
  return <WorkspaceApp />
}
```

- [ ] **Step 4: Fix import paths**

Grove's files import from `@workspaces/...` package paths (the monorepo shared packages). Search for these and inline or replace:

```bash
cd /Users/ken/workspace/ms/loom/ui
grep -r "@workspaces/" src/ --include="*.ts" --include="*.tsx" -l
```

For each file found, replace `@workspaces/types` imports with local equivalents (the types were copied to `ui/src/types/`):

```bash
# Example fix — adjust paths as needed based on what grep finds
find src/ -type f \( -name "*.ts" -o -name "*.tsx" \) \
  -exec sed -i '' 's|@workspaces/types|../types|g' {} +
find src/ -type f \( -name "*.ts" -o -name "*.tsx" \) \
  -exec sed -i '' 's|@workspaces/ui|../components|g' {} +
```

- [ ] **Step 5: Build and fix TypeScript errors**

```bash
cd /Users/ken/workspace/ms/loom/ui && npm run build 2>&1 | head -40
```

Fix any type errors by adjusting import paths. Common issues:
- Missing types for `node-pty` (not needed client-side — remove any server-side types)
- Missing `@anthropic-ai/` imports (grove's AI SDK — not needed in loom's frontend)
- Remove any `passport`, `jwt`, `better-sqlite3` references (server-only deps)

- [ ] **Step 6: Verify Go build still works**

```bash
cd /Users/ken/workspace/ms/loom && make ui && go build ./...
```

Expected: clean build.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(ui): copy grove WorkspaceApp into Projects view, remove auth"
```

---

## Task 10: Jobs React view

Replaces the vanilla JS Jobs UI with a React view. Consumes the existing `/api/jobs`, `/api/runs`, and SSE stream endpoints without any backend changes.

**Files:**
- Modify: `ui/src/views/jobs/index.tsx`
- Create: `ui/src/views/jobs/JobList.tsx`
- Create: `ui/src/views/jobs/RunDetail.tsx`
- Create: `ui/src/views/jobs/useRunStream.ts`
- Create: `ui/src/api/jobs.ts`

- [ ] **Step 1: Create `ui/src/api/jobs.ts`** — types + fetch helpers for Jobs API

```typescript
export interface Job {
  id: string
  name: string
  description: string
  enabled: boolean
  trigger: { type: string; schedule: string }
  executor: string
  lastRunAt?: string
  lastRunStatus?: string
}

export interface JobRun {
  id: string
  jobId: string
  status: 'running' | 'succeeded' | 'failed' | 'cancelled'
  startedAt: string
  finishedAt?: string
  output?: string
}

export async function listJobs(): Promise<Job[]> {
  const res = await fetch('/api/jobs')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function listJobRuns(jobId: string, limit = 20): Promise<JobRun[]> {
  const res = await fetch(`/api/jobs/${jobId}/runs?limit=${limit}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function triggerJob(jobId: string): Promise<JobRun> {
  const res = await fetch(`/api/jobs/${jobId}/trigger`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
```

- [ ] **Step 2: Create `ui/src/views/jobs/useRunStream.ts`** — SSE hook for live log output

```typescript
import { useEffect, useRef, useState } from 'react'

export function useRunStream(runId: string | null) {
  const [lines, setLines] = useState<string[]>([])
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    if (!runId) {
      setLines([])
      return
    }
    setLines([])
    const es = new EventSource(`/api/runs/${runId}/stream`)
    esRef.current = es

    es.onmessage = (e) => {
      setLines(prev => [...prev, e.data])
    }
    es.onerror = () => es.close()

    return () => {
      es.close()
      esRef.current = null
    }
  }, [runId])

  return lines
}
```

- [ ] **Step 3: Create `ui/src/views/jobs/JobList.tsx`**

```typescript
import { Job } from '../../api/jobs'

interface Props {
  jobs: Job[]
  selectedId: string | null
  onSelect: (id: string) => void
  onNew: () => void
}

export default function JobList({ jobs, selectedId, onSelect, onNew }: Props) {
  const statusDot = (job: Job) => {
    const color = job.lastRunStatus === 'running' ? '#3fb950' : '#8b949e'
    return <span style={{ width: 6, height: 6, borderRadius: '50%', background: color, display: 'inline-block', flexShrink: 0 }} />
  }

  return (
    <div className="flex flex-col h-full bg-[#0d1117] border-r border-[#30363d] w-52 shrink-0">
      <div className="flex items-center justify-between px-3 py-2 border-b border-[#30363d]">
        <span className="text-[#8b949e] text-xs uppercase tracking-wider">Jobs</span>
        <button
          onClick={onNew}
          className="text-xs text-[#58a6ff] hover:text-[#e6edf3]"
        >
          + New
        </button>
      </div>
      <div className="flex-1 overflow-y-auto">
        {jobs.map(job => (
          <button
            key={job.id}
            onClick={() => onSelect(job.id)}
            className={[
              'w-full text-left px-3 py-2 border-b border-[#21262d] hover:bg-[#161b22] transition-colors',
              selectedId === job.id ? 'bg-[#21262d]' : '',
            ].join(' ')}
          >
            <div className="flex items-center justify-between gap-2">
              <span className={`text-xs truncate ${selectedId === job.id ? 'text-[#e6edf3]' : 'text-[#8b949e]'}`}>
                {job.name}
              </span>
              {statusDot(job)}
            </div>
            <div className="text-[10px] text-[#8b949e] mt-0.5">
              {job.trigger.type} {job.trigger.schedule && `· ${job.trigger.schedule}`}
            </div>
          </button>
        ))}
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Create `ui/src/views/jobs/RunDetail.tsx`**

```typescript
import { useEffect, useState } from 'react'
import { Job, JobRun, listJobRuns, triggerJob } from '../../api/jobs'
import { useRunStream } from './useRunStream'

interface Props {
  job: Job
}

export default function RunDetail({ job }: Props) {
  const [runs, setRuns] = useState<JobRun[]>([])
  const [activeRunId, setActiveRunId] = useState<string | null>(null)
  const logLines = useRunStream(activeRunId)

  useEffect(() => {
    listJobRuns(job.id).then(setRuns)
  }, [job.id])

  const activeRun = runs.find(r => r.id === activeRunId) ?? runs[0] ?? null
  if (!activeRunId && runs.length > 0) setActiveRunId(runs[0].id)

  const handleTrigger = async () => {
    const run = await triggerJob(job.id)
    setRuns(prev => [run, ...prev])
    setActiveRunId(run.id)
  }

  const statusBadge = (status: string) => {
    const styles: Record<string, string> = {
      running:   'bg-[#1a3a2a] text-[#3fb950]',
      succeeded: 'bg-[#1a3a2a] text-[#3fb950]',
      failed:    'bg-[#3a1a1a] text-[#f85149]',
      cancelled: 'bg-[#21262d] text-[#8b949e]',
    }
    return (
      <span className={`text-[10px] px-1.5 py-0.5 rounded ${styles[status] ?? styles.cancelled}`}>
        {status}
      </span>
    )
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-3 px-4 py-2 bg-[#161b22] border-b border-[#30363d] shrink-0">
        <span className="text-sm font-semibold text-[#e6edf3]">{job.name}</span>
        {activeRun && statusBadge(activeRun.status)}
        <button
          onClick={handleTrigger}
          className="ml-auto text-xs px-3 py-1 bg-[#21262d] border border-[#30363d] rounded text-[#e6edf3] hover:bg-[#30363d]"
        >
          ▶ Run Now
        </button>
      </div>

      {/* Run history tabs */}
      {runs.length > 0 && (
        <div className="flex gap-1 px-4 py-1.5 bg-[#0d1117] border-b border-[#21262d] shrink-0 overflow-x-auto">
          {runs.slice(0, 10).map((run, i) => (
            <button
              key={run.id}
              onClick={() => setActiveRunId(run.id)}
              className={[
                'text-[10px] px-2 py-0.5 rounded shrink-0',
                activeRunId === run.id
                  ? 'bg-[#21262d] text-[#e6edf3]'
                  : 'text-[#8b949e] hover:text-[#e6edf3]',
              ].join(' ')}
            >
              #{runs.length - i}
            </button>
          ))}
        </div>
      )}

      {/* Log output */}
      <div className="flex-1 overflow-y-auto font-mono text-[11px] text-[#e6edf3] bg-[#0d1117] p-4 leading-relaxed">
        {logLines.length > 0
          ? logLines.map((line, i) => <div key={i}>{line}</div>)
          : <span className="text-[#8b949e]">No output yet</span>
        }
      </div>
    </div>
  )
}
```

- [ ] **Step 5: Update `ui/src/views/jobs/index.tsx`**

```typescript
import { useEffect, useState } from 'react'
import { Job, listJobs } from '../../api/jobs'
import JobList from './JobList'
import RunDetail from './RunDetail'

export default function JobsView() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)

  useEffect(() => {
    listJobs().then(jobs => {
      setJobs(jobs)
      if (jobs.length > 0) setSelectedId(jobs[0].id)
    })
  }, [])

  const selectedJob = jobs.find(j => j.id === selectedId) ?? null

  return (
    <div className="flex h-full">
      <JobList
        jobs={jobs}
        selectedId={selectedId}
        onSelect={setSelectedId}
        onNew={() => {/* TODO: new job modal */}}
      />
      <div className="flex-1 overflow-hidden">
        {selectedJob
          ? <RunDetail job={selectedJob} />
          : <div className="p-8 text-[#8b949e] text-sm">Select a job to view details</div>
        }
      </div>
    </div>
  )
}
```

- [ ] **Step 6: Build and verify**

```bash
cd /Users/ken/workspace/ms/loom && make ui
```

Expected: clean build with no TypeScript errors in the jobs view.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(ui): add Jobs view (job list + run detail + SSE log streaming)"
```

---

## Task 11: Mirror React view

Replaces the vanilla JS Mirror UI. Consumes existing `/api/mirror/connectors`, `/api/mirror/entities`, and `/api/mirror/changes` endpoints.

**Files:**
- Modify: `ui/src/views/mirror/index.tsx`
- Create: `ui/src/views/mirror/ConnectorList.tsx`
- Create: `ui/src/views/mirror/EntityBrowser.tsx`
- Create: `ui/src/api/mirror.ts`

- [ ] **Step 1: Create `ui/src/api/mirror.ts`**

```typescript
export interface Connector {
  id: string
  name: string
  type: string
  health: 'live' | 'idle' | 'error'
  lastSyncAt?: string
  entityCount?: number
}

export interface Entity {
  address: string
  type: string
  data: Record<string, unknown>
  updatedAt: string
}

export interface Change {
  id: string
  connectorId: string
  entityAddress: string
  diff: string
  createdAt: string
}

export async function listConnectors(): Promise<Connector[]> {
  const res = await fetch('/api/mirror/connectors')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function listEntities(connectorId: string): Promise<Entity[]> {
  const res = await fetch(`/api/mirror/entities?connectorId=${connectorId}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function listChanges(connectorId: string, limit = 20): Promise<Change[]> {
  const res = await fetch(`/api/mirror/changes?connectorId=${connectorId}&limit=${limit}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
```

- [ ] **Step 2: Create `ui/src/views/mirror/ConnectorList.tsx`**

```typescript
import { Connector } from '../../api/mirror'

interface Props {
  connectors: Connector[]
  selectedId: string | null
  onSelect: (id: string) => void
}

export default function ConnectorList({ connectors, selectedId, onSelect }: Props) {
  const healthDot = (health: string) => {
    const color = health === 'live' ? '#3fb950' : health === 'error' ? '#f85149' : '#8b949e'
    return <span style={{ width: 6, height: 6, borderRadius: '50%', background: color, display: 'inline-block', flexShrink: 0 }} />
  }

  return (
    <div className="flex flex-col h-full bg-[#0d1117] border-r border-[#30363d] w-52 shrink-0">
      <div className="px-3 py-2 border-b border-[#30363d]">
        <span className="text-[#8b949e] text-xs uppercase tracking-wider">Connectors</span>
      </div>
      <div className="flex-1 overflow-y-auto">
        {connectors.map(c => (
          <button
            key={c.id}
            onClick={() => onSelect(c.id)}
            className={[
              'w-full text-left px-3 py-2 border-b border-[#21262d] hover:bg-[#161b22] transition-colors',
              selectedId === c.id ? 'bg-[#21262d]' : '',
            ].join(' ')}
          >
            <div className="flex items-center justify-between gap-2">
              <span className={`text-xs truncate ${selectedId === c.id ? 'text-[#e6edf3]' : 'text-[#8b949e]'}`}>
                {c.name}
              </span>
              {healthDot(c.health)}
            </div>
            <div className="text-[10px] text-[#8b949e] mt-0.5">{c.type}</div>
          </button>
        ))}
        {connectors.length === 0 && (
          <div className="px-3 py-4 text-[#8b949e] text-xs">No connectors configured</div>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 3: Create `ui/src/views/mirror/EntityBrowser.tsx`**

```typescript
import { useEffect, useState } from 'react'
import { Connector, Entity, listEntities } from '../../api/mirror'

interface Props {
  connector: Connector
}

export default function EntityBrowser({ connector }: Props) {
  const [entities, setEntities] = useState<Entity[]>([])
  const [selected, setSelected] = useState<Entity | null>(null)

  useEffect(() => {
    setEntities([])
    setSelected(null)
    listEntities(connector.id).then(setEntities)
  }, [connector.id])

  return (
    <div className="flex flex-col h-full">
      <div className="px-4 py-2 bg-[#161b22] border-b border-[#30363d] shrink-0">
        <span className="text-sm font-semibold text-[#e6edf3]">{connector.name}</span>
        <span className="ml-2 text-xs text-[#8b949e]">
          {entities.length} entities
          {connector.lastSyncAt && ` · last sync ${new Date(connector.lastSyncAt).toLocaleTimeString()}`}
        </span>
      </div>
      <div className="flex flex-1 overflow-hidden">
        {/* Entity list */}
        <div className="w-72 border-r border-[#30363d] overflow-y-auto shrink-0">
          {entities.map(e => (
            <button
              key={e.address}
              onClick={() => setSelected(e)}
              className={[
                'w-full text-left px-3 py-2 border-b border-[#21262d] hover:bg-[#161b22] transition-colors',
                selected?.address === e.address ? 'bg-[#21262d]' : '',
              ].join(' ')}
            >
              <div className="text-xs text-[#e6edf3] truncate">{e.address}</div>
              <div className="text-[10px] text-[#8b949e]">{e.type}</div>
            </button>
          ))}
        </div>
        {/* Entity detail */}
        <div className="flex-1 overflow-auto p-4">
          {selected ? (
            <pre className="text-xs text-[#e6edf3] whitespace-pre-wrap font-mono">
              {JSON.stringify(selected.data, null, 2)}
            </pre>
          ) : (
            <span className="text-[#8b949e] text-sm">Select an entity</span>
          )}
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Update `ui/src/views/mirror/index.tsx`**

```typescript
import { useEffect, useState } from 'react'
import { Connector, listConnectors } from '../../api/mirror'
import ConnectorList from './ConnectorList'
import EntityBrowser from './EntityBrowser'

export default function MirrorView() {
  const [connectors, setConnectors] = useState<Connector[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)

  useEffect(() => {
    listConnectors().then(cs => {
      setConnectors(cs)
      if (cs.length > 0) setSelectedId(cs[0].id)
    })
  }, [])

  const selected = connectors.find(c => c.id === selectedId) ?? null

  return (
    <div className="flex h-full">
      <ConnectorList
        connectors={connectors}
        selectedId={selectedId}
        onSelect={setSelectedId}
      />
      <div className="flex-1 overflow-hidden">
        {selected
          ? <EntityBrowser connector={selected} />
          : <div className="p-8 text-[#8b949e] text-sm">Select a connector to browse entities</div>
        }
      </div>
    </div>
  )
}
```

- [ ] **Step 5: Full build**

```bash
cd /Users/ken/workspace/ms/loom && make ui && go build ./...
```

Expected: clean build.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(ui): add Mirror view (connector list + entity browser)"
```

---

## Task 12: Final cleanup + integration smoke test

Delete old vanilla JS files, run the full test suite, and do a manual smoke test.

**Files:**
- Delete: `web/index.html`, `web/app.js`, `web/style.css`

- [ ] **Step 1: Delete old vanilla JS files**

```bash
git rm web/index.html web/app.js web/style.css
```

Expected: files staged for deletion.

- [ ] **Step 2: Run full test suite**

```bash
cd /Users/ken/workspace/ms/loom && go test ./...
```

Expected: all tests pass.

- [ ] **Step 3: Run UI tests**

```bash
cd ui && npx vitest run
```

Expected: hub nav tests pass.

- [ ] **Step 4: Full build**

```bash
cd /Users/ken/workspace/ms/loom && make build
```

Expected: binary produced at `dist/loom`.

- [ ] **Step 5: Manual smoke test**

```bash
./dist/loom start
```

Open http://localhost:7700 in a browser.

Verify:
- [ ] Loads the React SPA (not the old vanilla JS UI)
- [ ] "Projects", "Jobs", "Mirror" tabs visible in top nav
- [ ] Clicking "Jobs" shows the job list panel
- [ ] Clicking "Mirror" shows the connector list panel
- [ ] Clicking "Projects" shows the workspace app

```bash
./dist/loom stop
```

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "chore: remove old vanilla JS web/ files, workspace integration complete"
```

---

## Execution

Plan complete and saved to `docs/superpowers/plans/2026-03-31-loom-workspace-integration.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks. Use skill `superpowers:subagent-driven-development`.

**2. Inline Execution** — execute tasks in this session with checkpoints. Use skill `superpowers:executing-plans`.

Which approach?
