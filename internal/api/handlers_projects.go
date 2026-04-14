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
    