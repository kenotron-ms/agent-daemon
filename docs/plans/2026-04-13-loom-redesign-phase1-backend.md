# Phase 1: Backend Surgery — Implementation Plan

> **For execution:** Use `/execute-plan` mode.

**Prerequisite:** None — this is Phase 1.
**Goal:** Remove PTY/session backend entirely, add Workspace field, add two new endpoints.
**Architecture:** The xterm.js web terminal stack (PTY manager, WebSocket bridge, session CRUD) is surgically removed. Two new endpoints are added: one reads Amplifier's on-disk session store, the other launches native terminal apps. The `Project` struct gains a `Workspace` grouping label.
**Tech Stack:** Go, bbolt, macOS exec/osascript

---

### Task 1: Remove PTY/session/terminal routes, handlers, and wiring from the API layer

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/handlers_projects.go`
- Modify: `internal/service/daemon.go`
- Modify: `internal/api/handlers_projects_test.go`

**Action:**

This task severs every reference to the `internal/pty` package and removes all session/terminal handler functions. These four files must be updated together because they're coupled — changing one without the others leaves a broken build.

**In `internal/api/server.go`:**

1. Remove the `loompty` import line:
```go
// DELETE this line:
loompty "github.com/ms/amplifier-app-loom/internal/pty"
```

2. Remove the `ptyMgr` and `watchedSessions` fields from the `Server` struct (lines 36-37):
```go
// DELETE these two lines from the Server struct:
ptyMgr          *loompty.Manager
watchedSessions sync.Map // sessionID → struct{}: tracks in-flight name watchers
```

3. Change the `SetWorkspaces` method signature and body (lines 63-67). Replace:
```go
// SetWorkspaces wires the workspace subsystem (projects, PTY) into the server.
func (s *Server) SetWorkspaces(ws *workspaces.Service, mgr *loompty.Manager) {
	s.workspaceStore = ws
	s.ptyMgr = mgr
}
```
with:
```go
// SetWorkspaces wires the workspace subsystem (projects) into the server.
func (s *Server) SetWorkspaces(ws *workspaces.Service) {
	s.workspaceStore = ws
}
```

4. In `registerRoutes`, remove the Sessions block (lines 177-180):
```go
// DELETE:
	// Sessions
	mux.HandleFunc("GET /api/projects/{id}/sessions", s.listSessions)
	mux.HandleFunc("POST /api/projects/{id}/sessions", s.createSession)
	mux.HandleFunc("DELETE /api/projects/{id}/sessions/{sid}", s.deleteSession)
```

5. Remove the Terminal block (lines 182-185):
```go
// DELETE:
	// Terminal
	mux.HandleFunc("POST /api/projects/{id}/sessions/{sid}/terminal", s.spawnTerminal)
	mux.HandleFunc("/api/terminal/{processId}", s.handleTerminalWS)
	mux.HandleFunc("POST /api/terminal/{processId}/resize", s.resizeTerminal)
```

6. Replace the Files + Stats block (lines 187-190):
```go
// DELETE:
	// Files + Stats
	mux.HandleFunc("GET /api/projects/{id}/sessions/{sid}/files", s.listFiles)
	mux.HandleFunc("GET /api/projects/{id}/sessions/{sid}/files/{path...}", s.readFile)
	mux.HandleFunc("GET /api/projects/{id}/sessions/{sid}/stats", s.getSessionStats)
```
with project-based file routes (insert right after the project settings routes):
```go
	// Files (project-scoped, no longer session-scoped)
	mux.HandleFunc("GET /api/projects/{id}/files", s.listFiles)
	mux.HandleFunc("GET /api/projects/{id}/files/{path...}", s.readFile)
```

**In `internal/api/handlers_projects.go`:**

Replace the entire file content with:
```go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/ms/amplifier-app-loom/internal/files"
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
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
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
	if err := s.workspaceStore.DeleteProject(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Files ─────────────────────────────────────────────────────────────────────

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.workspaceStore.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	rel := r.URL.Query().Get("path")
	entries, err := files.New(p.Path).List(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) readFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	path := r.PathValue("path")
	p, err := s.workspaceStore.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	data, err := files.New(p.Path).Read(path)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data) //nolint:errcheck
}
```

**In `internal/service/daemon.go`:**

1. Remove the `loompty` import (line 14):
```go
// DELETE this line:
loompty "github.com/ms/amplifier-app-loom/internal/pty"
```

2. Replace line 129:
```go
// REPLACE:
			srv.SetWorkspaces(ws, loompty.NewManager())
// WITH:
			srv.SetWorkspaces(ws)
```

**In `internal/api/handlers_projects_test.go`:**

1. Remove the `loompty` import (line 16):
```go
// DELETE this line:
loompty "github.com/ms/amplifier-app-loom/internal/pty"
```

2. Replace line 46:
```go
// REPLACE:
	srv.SetWorkspaces(ws, loompty.NewManager())
// WITH:
	srv.SetWorkspaces(ws)
```

**Build check:** `go build ./...` — Expected: no errors
**Commit:** `git add -A && git commit -m "refactor: remove PTY/session/terminal routes and handlers from API layer"`

---

### Task 2: Delete `internal/pty/` directory

**Files:**
- Delete: `internal/pty/pty.go`
- Delete: `internal/pty/pty_test.go`

**Action:**
```bash
rm -rf internal/pty/
```

**Build check:** `go build ./...` — Expected: no errors (no code imports this package after Task 1)
**Commit:** `git add -A && git commit -m "chore: delete internal/pty package (PTY manager + WebSocket bridge)"`

---

### Task 3: Delete `internal/amplifier/prepare_session.go` and `prepare_session.py`

**Files:**
- Delete: `internal/amplifier/prepare_session.go`
- Delete: `internal/amplifier/prepare_session.py`

**Action:**
```bash
rm internal/amplifier/prepare_session.go internal/amplifier/prepare_session.py
```

**Build check:** `go build ./...` — Expected: no errors (no code calls `PrepareSession` after Task 1)
**Commit:** `git add -A && git commit -m "chore: delete amplifier PrepareSession + embedded Python script"`

---

### Task 4: Remove Session struct, session CRUD, and session buckets from workspaces.go

**Files:**
- Modify: `internal/workspaces/workspaces.go`

**Action:**

Replace the entire file content with:
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

var bucketProjects = []byte("projects")

// Project is a codebase on disk managed by Loom.
type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Path           string `json:"path"` // absolute path on disk
	CreatedAt      int64  `json:"createdAt"`
	LastActivityAt int64  `json:"lastActivityAt"`
}

// Service is the workspace CRUD layer backed by bbolt.
type Service struct {
	db *bolt.DB
}

// New creates a Service and initialises the required bbolt buckets.
func New(db *bolt.DB) (*Service, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketProjects); err != nil {
			return fmt.Errorf("create bucket %q: %w", bucketProjects, err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("initialise workspaces buckets: %w", err)
	}
	return &Service{db: db}, nil
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
	var p Project
	return &p, s.db.Update(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketProjects).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project %s not found", id)
		}
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		p.Name = name
		updated, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketProjects).Put([]byte(id), updated)
	})
}

func (s *Service) DeleteProject(_ context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketProjects).Delete([]byte(id))
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
```

**What was removed:**
- `bytes` import
- `bucketSessions` and `bucketSessionsByProject` variables
- `Session` struct (lines 30-39)
- Session bucket initialization in `New()` (lines 49 loop)
- All session functions: `CreateSession`, `GetSession`, `ListSessions`, `RenameSession`, `SetAmplifierSessionID`, `UpdateSessionStatus`, `DeleteSession` (lines 177-318)
- Session cleanup from `DeleteProject` (lines 138-152)

**Build check:** `go build ./...` — Expected: no errors
**Commit:** `git add -A && git commit -m "refactor: remove Session struct, session CRUD, and session buckets from workspaces"`

---

### Task 5: Add `Workspace` field to `Project` struct

**Files:**
- Modify: `internal/workspaces/workspaces.go`
- Modify: `internal/api/handlers_projects.go`

**Action:**

**In `internal/workspaces/workspaces.go`:**

1. Add `Workspace` field to the `Project` struct. Replace:
```go
type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Path           string `json:"path"` // absolute path on disk
	CreatedAt      int64  `json:"createdAt"`
	LastActivityAt int64  `json:"lastActivityAt"`
}
```
with:
```go
// Project is a codebase on disk managed by Loom.
type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Path           string `json:"path"`      // absolute path on disk
	Workspace      string `json:"workspace"` // grouping label (default: "Default")
	CreatedAt      int64  `json:"createdAt"`
	LastActivityAt int64  `json:"lastActivityAt"`
}
```

2. In `GetProject`, add the default after the nil check. Replace:
```go
	if err != nil {
		return nil, err
	}
	return &p, nil
}
```
with:
```go
	if err != nil {
		return nil, err
	}
	if p.Workspace == "" {
		p.Workspace = "Default"
	}
	return &p, nil
}
```

3. In `ListProjects`, default the workspace on read. Replace:
```go
			projects = append(projects, &p)
```
with:
```go
			if p.Workspace == "" {
				p.Workspace = "Default"
			}
			projects = append(projects, &p)
```

4. Update `UpdateProject` to accept and apply workspace. Replace:
```go
func (s *Service) UpdateProject(_ context.Context, id, name string) (*Project, error) {
	var p Project
	return &p, s.db.Update(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketProjects).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project %s not found", id)
		}
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		p.Name = name
		updated, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketProjects).Put([]byte(id), updated)
	})
}
```
with:
```go
func (s *Service) UpdateProject(_ context.Context, id, name, workspace string) (*Project, error) {
	var p Project
	return &p, s.db.Update(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketProjects).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project %s not found", id)
		}
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		p.Name = name
		if workspace != "" {
			p.Workspace = workspace
		}
		if p.Workspace == "" {
			p.Workspace = "Default"
		}
		updated, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketProjects).Put([]byte(id), updated)
	})
}
```

**In `internal/api/handlers_projects.go`:**

Replace the `updateProject` handler. Replace:
```go
func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	p, err := s.workspaceStore.UpdateProject(r.Context(), id, req.Name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}
```
with:
```go
func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name      string `json:"name"`
		Workspace string `json:"workspace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	p, err := s.workspaceStore.UpdateProject(r.Context(), id, req.Name, req.Workspace)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}
```

**Build check:** `go build ./...` — Expected: no errors
**Commit:** `git add -A && git commit -m "feat: add Workspace field to Project struct with 'Default' fallback"`

---

### Task 6: Add `PreferredTerminal` to Config

**Files:**
- Modify: `internal/config/config.go`

**Action:**

Add the `PreferredTerminal` field to the `Config` struct. In `internal/config/config.go`, after the `OnboardingComplete` field (line 43), add:

```go
	// PreferredTerminal is the macOS terminal app used for "New Session" and
	// session resume. Supported: "Terminal.app", "iTerm2", "Warp", "Ghostty".
	PreferredTerminal string `json:"preferredTerminal,omitempty"`
```

The default is applied at use-time in the handler (falls back to `"Terminal.app"` when empty), so no change to `Defaults()` is needed.

**Build check:** `go build ./...` — Expected: no errors
**Commit:** `git add -A && git commit -m "feat: add PreferredTerminal config field"`

---

### Task 7: Create `internal/amplifier/sessions.go` — ListProjectSessions

**Files:**
- Create: `internal/amplifier/sessions.go`

**Action:**

Create the file with this content:
```go
package amplifier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// AmplifierSession represents a session from Amplifier's on-disk store.
type AmplifierSession struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ProjectPath string    `json:"projectPath"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ListProjectSessions reads ~/.amplifier/projects/<slug>/sessions/ and returns
// all sessions for the given project path, sorted by recency (newest first).
// Returns an empty slice (not nil) when no sessions are found.
func ListProjectSessions(projectPath string) ([]AmplifierSession, error) {
	dir, err := sessionsDir(projectPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []AmplifierSession
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name(), "metadata.json"))
		if err != nil {
			continue
		}
		var m Meta
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		sessions = append(sessions, AmplifierSession{
			ID:          m.SessionID,
			Name:        m.Name,
			ProjectPath: m.WorkingDir,
			CreatedAt:   m.Created,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	if sessions == nil {
		sessions = []AmplifierSession{}
	}
	return sessions, nil
}
```

**Note:** This file reuses the `Meta` struct and `sessionsDir()` helper already defined in the existing `internal/amplifier/session.go` (same package).

**Build check:** `go build ./...` — Expected: no errors
**Commit:** `git add -A && git commit -m "feat: add ListProjectSessions — reads Amplifier's on-disk session store"`

---

### Task 8: Create `internal/api/handlers_terminal.go` and register new routes

**Files:**
- Create: `internal/api/handlers_terminal.go`
- Modify: `internal/api/server.go`

**Action:**

**Create `internal/api/handlers_terminal.go`:**
```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ms/amplifier-app-loom/internal/amplifier"
)

// handleListAmplifierSessions returns Amplifier sessions for a project,
// read from Amplifier's on-disk session store (~/.amplifier/projects/…/sessions/).
func (s *Server) handleListAmplifierSessions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.workspaceStore.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	sessions, err := amplifier.ListProjectSessions(p.Path)
	if err != nil {
		// Session store may be missing — return empty list, not an error.
		writeJSON(w, http.StatusOK, []amplifier.AmplifierSession{})
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

// handleOpenTerminal launches or focuses a native terminal for a project.
//
// Request body:
//
//	{"mode": "new"}                           — open a new terminal at the project path
//	{"mode": "resume", "sessionId": "<uuid>"} — focus existing or resume session
func (s *Server) handleOpenTerminal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.workspaceStore.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req struct {
		Mode      string `json:"mode"`      // "new" | "resume"
		SessionID string `json:"sessionId"` // required when mode=resume
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	terminal := s.cfg.PreferredTerminal
	if terminal == "" {
		terminal = "Terminal.app"
	}

	switch req.Mode {
	case "new":
		cmd := exec.Command("open", "-a", terminal, p.Path)
		if err := cmd.Run(); err != nil {
			writeError(w, http.StatusInternalServerError,
				fmt.Sprintf("failed to open %s: %s", terminal, err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "opened"})

	case "resume":
		if req.SessionID == "" {
			writeError(w, http.StatusBadRequest, "sessionId required for resume mode")
			return
		}
		// Check if a process with this session ID is already running.
		check := exec.Command("bash", "-c",
			fmt.Sprintf("ps aux | grep '%s' | grep -v grep", req.SessionID))
		if check.Run() == nil {
			// Process found — focus the terminal window via AppleScript.
			script := fmt.Sprintf(`tell application "%s" to activate`, terminal)
			exec.Command("osascript", "-e", script).Run() //nolint:errcheck
			writeJSON(w, http.StatusOK, map[string]string{"status": "focused"})
			return
		}
		// Not running — open a new terminal with amplifier --resume.
		ampBin := resolveAmplifier()
		script := fmt.Sprintf(
			`tell application "%s"
	activate
	do script "cd '%s' && '%s' run --resume '%s'"
end tell`, terminal, p.Path, ampBin, req.SessionID)
		if err := exec.Command("osascript", "-e", script).Run(); err != nil {
			writeError(w, http.StatusInternalServerError,
				fmt.Sprintf("failed to resume session: %s", err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})

	default:
		writeError(w, http.StatusBadRequest, `mode must be "new" or "resume"`)
	}
}

// resolveAmplifier finds the amplifier binary. GUI apps and launchd services
// inherit a minimal PATH that misses user-installed tools in ~/.local/bin.
func resolveAmplifier() string {
	if p, err := exec.LookPath("amplifier"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".local", "bin", "amplifier"),
		"/usr/local/bin/amplifier",
		"/opt/homebrew/bin/amplifier",
		filepath.Join(home, "go", "bin", "amplifier"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "amplifier"
}
```

**In `internal/api/server.go`**, add two new routes. After the file routes block (which now reads `GET /api/projects/{id}/files...`), insert:
```go
	// Amplifier sessions + terminal launch
	mux.HandleFunc("GET /api/projects/{id}/amplifier-sessions", s.handleListAmplifierSessions)
	mux.HandleFunc("POST /api/projects/{id}/open-terminal", s.handleOpenTerminal)
```

**Build check:** `go build ./...` — Expected: no errors
**Commit:** `git add -A && git commit -m "feat: add GET amplifier-sessions and POST open-terminal endpoints"`

---

### Task 9: Remove unused Go module dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Action:**

Run:
```bash
go mod tidy
```

This removes `github.com/creack/pty` and `github.com/gorilla/websocket` from `go.mod` since nothing imports them after deleting `internal/pty/`.

**Build check:** `go build ./...` — Expected: no errors
**Test check:** `go test ./...` — Expected: all tests pass
**Commit:** `git add -A && git commit -m "chore: go mod tidy — remove creack/pty and gorilla/websocket"`

---

### Task 10: Browser verification

**Files:** none (verification only)

**Action:**

Start the server:
```bash
go run ./cmd/loom &
```
Wait 3 seconds for startup, then:
```bash
agent-browser open http://localhost:7700
```
```bash
agent-browser snapshot -ic
```
Expected in snapshot output: Projects, Jobs, Mirror, Bundles tabs visible. No panic in server terminal output.

```bash
agent-browser screenshot /tmp/phase1-verify.png
```
```bash
agent-browser close
```

Kill the background server process.

**Commit:** `git add -A && git commit -m "feat: phase 1 backend surgery complete"`