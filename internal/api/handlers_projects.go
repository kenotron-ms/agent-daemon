package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/ms/amplifier-app-loom/internal/amplifier"
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
	sessions, _ := s.workspaceStore.ListSessions(r.Context(), id)
	for _, sess := range sessions {
		if sess.ProcessID != nil {
			s.ptyMgr.Kill(*sess.ProcessID) //nolint:errcheck
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
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	// A session runs in the project's root directory by default.
	// Worktree association is a separate, optional operation — not required to create a session.
	p, err := s.workspaceStore.GetProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	// Auto-name: derive from git branch when no name is supplied.
	// Eventually this will be replaced by amplifier's own session-naming hook —
	// amplifier auto-names sessions based on conversation context (e.g. "Loom macOS App Icon & DMG").
	if req.Name == "" {
		if out, err2 := exec.CommandContext(r.Context(),
			"git", "-C", p.Path, "rev-parse", "--abbrev-ref", "HEAD",
		).Output(); err2 == nil {
			req.Name = strings.TrimSpace(string(out))
		}
		if req.Name == "" || req.Name == "HEAD" {
			req.Name = "main"
		}
	}
	sess, err := s.workspaceStore.CreateSession(r.Context(), projectID, req.Name, p.Path)
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
		s.ptyMgr.Kill(*sess.ProcessID) //nolint:errcheck
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
	// Key by session ID — each session gets its own independent shell process.
	// Clicking the same session multiple times reuses the same PTY (dedup by session ID).
	key := sess.ID
	// Resume the paired amplifier session if one is stored; otherwise start fresh.
	ampCmd := []string{"amplifier", "run", "--mode", "chat"}
	if sess.AmplifierSessionID != "" {
		ampCmd = append(ampCmd, "--resume", sess.AmplifierSessionID)
	}
	processID, err := s.ptyMgr.Spawn(key, sess.WorktreePath, ampCmd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.workspaceStore.UpdateSessionStatus(r.Context(), sid, "active", &processID) //nolint:errcheck
	writeJSON(w, http.StatusOK, map[string]string{"processId": processID})

	// One goroutine per loom session handles both concerns:
	//   1. Capture the amplifier session ID after a fresh start (stored in DB)
	//   2. Watch for the amplifier auto-name and rename the loom session
	isNew := sess.AmplifierSessionID == ""
	if _, loaded := s.watchedSessions.LoadOrStore(sid, struct{}{}); !loaded {
		go func() {
			defer s.watchedSessions.Delete(sid)
			s.watchSession(sid, sess.WorktreePath, isNew)
		}()
	}
}

func (s *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	processID := r.PathValue("processId")
	s.ptyMgr.ServeWS(w, r, processID)
}

func (s *Server) resizeTerminal(w http.ResponseWriter, r *http.Request) {
	processID := r.PathValue("processId")
	var req struct {
		Cols uint16 `json:"cols"`
		Rows uint16 `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.ptyMgr.Resize(processID, req.Cols, req.Rows); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	w.Write(data) //nolint:errcheck
}

// ── Stats ─────────────────────────────────────────────────────────────────────

func (s *Server) getSessionStats(w http.ResponseWriter, r *http.Request) {
	// Placeholder: full implementation reads ~/.amplifier/.../events.jsonl
	writeJSON(w, http.StatusOK, map[string]any{
		"tokens": 0,
		"tools":  0,
	})
}

// watchSession is a unified goroutine started once per loom session on first spawn.
//
//   isNew=true  → also capture the amplifier session UUID after fresh start
//   isNew=false → amplifier session ID already stored, just watch for name updates
//
// Exits when the loom session is deleted from the DB.
func (s *Server) watchSession(sessionID, worktreePath string, isNew bool) {
	// Give amplifier a few seconds to create its session record on disk.
	time.Sleep(4 * time.Second)

	after := time.Now().Add(-8 * time.Second) // include sessions created just before us
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	ctx := context.Background()

	idCaptured := !isNew // skip capture step for resume sessions

	for range ticker.C {
		sess, err := s.workspaceStore.GetSession(ctx, sessionID)
		if err != nil {
			return // session was deleted
		}

		meta, err := amplifier.NewestSession(worktreePath, after)
		if err != nil || meta == nil {
			continue
		}

		// 1. Capture the amplifier session ID (once, for new sessions only)
		if !idCaptured && meta.SessionID != "" {
			if err := s.workspaceStore.SetAmplifierSessionID(ctx, sessionID, meta.SessionID); err == nil {
				slog.Info("amplifier session ID captured",
					"loom_session", sessionID, "amp_session", meta.SessionID)
				idCaptured = true
			}
		}

		// 2. Rename loom session when amplifier's naming hook fires
		if meta.Name != "" && meta.Name != sess.Name {
			if err := s.workspaceStore.RenameSession(ctx, sessionID, meta.Name); err == nil {
				slog.Info("session renamed from amplifier hook",
					"session", sessionID, "name", meta.Name)
			}
		}
	}
}
