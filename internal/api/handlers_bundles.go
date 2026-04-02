package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ms/amplifier-app-loom/internal/config"
)

// ── Registry proxy ────────────────────────────────────────────────────────────

const registryURL = "https://raw.githubusercontent.com/kenotron-ms/amplifier-registry/main/bundles.json"

var (
	registryCache      []json.RawMessage
	registryCacheAt    time.Time
	registryCacheMu    sync.Mutex
	registryCacheTTL   = time.Hour
)

// GET /api/registry
// Fetches the public bundle registry (cached for 1 hour).
func (s *Server) getRegistry(w http.ResponseWriter, r *http.Request) {
	registryCacheMu.Lock()
	defer registryCacheMu.Unlock()

	if registryCache != nil && time.Since(registryCacheAt) < registryCacheTTL {
		writeJSON(w, http.StatusOK, registryCache)
		return
	}

	resp, err := http.Get(registryURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch registry: "+err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to read registry response")
		return
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(body, &entries); err != nil {
		writeError(w, http.StatusBadGateway, "registry response is not valid JSON")
		return
	}

	registryCache = entries
	registryCacheAt = time.Now()
	writeJSON(w, http.StatusOK, registryCache)
}

// ── Installed bundle management ───────────────────────────────────────────────

type addBundleRequest struct {
	ID          string `json:"id"`
	InstallSpec string `json:"installSpec"` // argument after "amplifier bundle add "
	Name        string `json:"name,omitempty"`
}

// GET /api/bundles
func (s *Server) listBundles(w http.ResponseWriter, r *http.Request) {
	bundles := s.cfg.AppBundles
	if bundles == nil {
		bundles = []config.AppBundle{}
	}
	writeJSON(w, http.StatusOK, bundles)
}

// POST /api/bundles
// Body: {id, installSpec, name}
func (s *Server) addBundle(w http.ResponseWriter, r *http.Request) {
	var req addBundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.InstallSpec = strings.TrimSpace(req.InstallSpec)
	req.ID = strings.TrimSpace(req.ID)
	if req.InstallSpec == "" {
		writeError(w, http.StatusBadRequest, "installSpec is required")
		return
	}
	if req.ID == "" {
		req.ID = req.InstallSpec
	}

	// Deduplicate by ID
	for _, b := range s.cfg.AppBundles {
		if b.ID == req.ID {
			writeError(w, http.StatusConflict, "bundle already installed")
			return
		}
	}

	bundle := config.AppBundle{
		ID:          req.ID,
		InstallSpec: req.InstallSpec,
		Name:        req.Name,
		Enabled:     true,
	}
	s.cfg.AppBundles = append(s.cfg.AppBundles, bundle)

	if err := s.store.SaveConfig(r.Context(), s.cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	writeJSON(w, http.StatusCreated, bundle)
}

// DELETE /api/bundles/{id}
func (s *Server) removeBundle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	found := false
	filtered := s.cfg.AppBundles[:0]
	for _, b := range s.cfg.AppBundles {
		if b.ID != id {
			filtered = append(filtered, b)
		} else {
			found = true
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "bundle not found")
		return
	}
	s.cfg.AppBundles = filtered
	if err := s.store.SaveConfig(r.Context(), s.cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/bundles/{id}/toggle
// Flips the Enabled flag for the given bundle.
func (s *Server) toggleBundle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for i, b := range s.cfg.AppBundles {
		if b.ID == id {
			s.cfg.AppBundles[i].Enabled = !s.cfg.AppBundles[i].Enabled
			if err := s.store.SaveConfig(r.Context(), s.cfg); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save config")
				return
			}
			writeJSON(w, http.StatusOK, s.cfg.AppBundles[i])
			return
		}
	}
	writeError(w, http.StatusNotFound, "bundle not found")
}
