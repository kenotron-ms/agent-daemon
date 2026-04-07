package api

import (
	"encoding/json"
	"net/http"

	"github.com/ms/amplifier-app-loom/internal/amplifier"
)

// GET /api/projects/{id}/settings
func (s *Server) getProjectSettings(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.workspaceStore.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found: "+err.Error())
		return
	}
	settings, err := amplifier.ReadProjectSettings(p.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read settings: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// PUT /api/projects/{id}/settings
func (s *Server) updateProjectSettings(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.workspaceStore.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found: "+err.Error())
		return
	}
	var settings amplifier.ProjectSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := amplifier.WriteProjectSettings(p.Path, settings); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write settings: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}
