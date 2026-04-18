package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ms/amplifier-app-loom/internal/amplifier"
	"github.com/ms/amplifier-app-loom/internal/config"
)

// ── Registry proxy ────────────────────────────────────────────────────────────

// registryURL is the public community registry. Override with AMPLIFIER_REGISTRY_URL
// to point at a local server during development (e.g. python3 -m http.server 8765).
var registryURL = func() string {
	if u := os.Getenv("AMPLIFIER_REGISTRY_URL"); u != "" {
		return u
	}
	return "https://raw.githubusercontent.com/kenotron-ms/amplifier-registry/main/bundles.json"
}()

var (
	registryCache    []json.RawMessage
	registryCacheAt  time.Time
	registryCacheMu  sync.Mutex
	registryCacheTTL = time.Hour
)

// GET /api/registry
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
		writeError(w, http.StatusBadGateway, "failed to read registry")
		return
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(body, &entries); err != nil {
		writeError(w, http.StatusBadGateway, "registry not valid JSON")
		return
	}
	registryCache = entries
	registryCacheAt = time.Now()
	writeJSON(w, http.StatusOK, registryCache)
}

// ── App bundle management ─────────────────────────────────────────────────────
//
// Source of truth: ~/.amplifier/settings.yaml → bundle.app (list of spec URIs).
// Loom's config.AppBundles is a metadata cache (id, name) used only for display.
//
//   Adding:  amplifier bundle add --app <spec>     + store metadata in loom config
//   Removing: amplifier bundle remove <spec> --app + remove metadata from loom config
//   Toggle:  add/remove from bundle.app via CLI    + update Enabled in loom config
//   GET:     read bundle.app for real enabled state; merge with loom metadata

type addBundleRequest struct {
	ID          string `json:"id"`
	InstallSpec string `json:"installSpec"`
	Name        string `json:"name,omitempty"`
}

// GET /api/bundles
func (s *Server) listBundles(w http.ResponseWriter, r *http.Request) {
	bundles := s.cfg.AppBundles
	if bundles == nil {
		bundles = []config.AppBundle{}
	}

	// Reconcile Enabled state against ~/.amplifier/settings.yaml
	if appSpecs, err := amplifier.ReadAppBundles(); err == nil {
		inApp := make(map[string]bool, len(appSpecs))
		for _, sp := range appSpecs {
			inApp[strings.TrimSpace(sp)] = true
		}
		for i, b := range bundles {
			bundles[i].Enabled = inApp[strings.TrimSpace(b.InstallSpec)]
		}
	}

	writeJSON(w, http.StatusOK, bundles)
}

// POST /api/bundles
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

	for _, b := range s.cfg.AppBundles {
		if b.ID == req.ID {
			writeError(w, http.StatusConflict, "bundle already installed")
			return
		}
	}

	// Register with amplifier as an app bundle
	if err := ampBundleAddApp(req.InstallSpec); err != nil {
		writeError(w, http.StatusInternalServerError,
			fmt.Sprintf("amplifier bundle add --app failed: %v\nMake sure `amplifier` is installed.", err))
		return
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

	var spec string
	var wasEnabled bool
	filtered := make([]config.AppBundle, 0, len(s.cfg.AppBundles))
	for _, b := range s.cfg.AppBundles {
		if b.ID != id {
			filtered = append(filtered, b)
		} else {
			spec = b.InstallSpec
			wasEnabled = b.Enabled
		}
	}
	if spec == "" {
		writeError(w, http.StatusNotFound, "bundle not found")
		return
	}

	if wasEnabled {
		ampBundleRemoveApp(spec) //nolint:errcheck
	}

	s.cfg.AppBundles = filtered
	if err := s.store.SaveConfig(r.Context(), s.cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/bundles/{id}/toggle
func (s *Server) toggleBundle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	idx := -1
	for i, b := range s.cfg.AppBundles {
		if b.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeError(w, http.StatusNotFound, "bundle not found")
		return
	}

	b := &s.cfg.AppBundles[idx]
	var cliErr error
	if b.Enabled {
		cliErr = ampBundleRemoveApp(b.InstallSpec)
		if cliErr == nil {
			b.Enabled = false
		}
	} else {
		cliErr = ampBundleAddApp(b.InstallSpec)
		if cliErr == nil {
			b.Enabled = true
		}
	}

	if cliErr != nil {
		writeError(w, http.StatusInternalServerError,
			fmt.Sprintf("amplifier bundle command failed: %v", cliErr))
		return
	}

	if err := s.store.SaveConfig(r.Context(), s.cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	writeJSON(w, http.StatusOK, *b)
}

// ── amplifier CLI helpers ─────────────────────────────────────────────────────

func ampBundleAddApp(spec string) error {
	return runAmpCmd("bundle", "add", "--app", spec)
}

func ampBundleRemoveApp(spec string) error {
	return runAmpCmd("bundle", "remove", spec, "--app")
}

// runAmpCmd runs the amplifier binary with the given arguments, searching
// common install locations because the daemon may have a stripped PATH.
func runAmpCmd(args ...string) error {
	bin := resolveAmplifier()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "TERM=dumb")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// resolveAmplifier finds the amplifier binary across common install locations.
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

// ── Private local registry ────────────────────────────────────────────────────
// Reads ~/.amplifier/bundle-index/index.json — maintained by the local-index CLI
// (registry/.github/scripts/local-index.mjs).  Returns an empty array (not an
// error) if the index has not been initialised yet.

// localCapability mirrors capabilities[] entries from local-index.mjs.
type localCapability struct {
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Version     *string `json:"version,omitempty"`
	SourceFile  string  `json:"sourceFile"`
	Inferred    bool    `json:"inferred,omitempty"`
}

// localRepoEntry mirrors repos{} values from local-index.mjs index.json.
type localRepoEntry struct {
	RepoPath     string            `json:"repoPath"`
	Name         string            `json:"name"`
	Remote       string            `json:"remote"` // "org/repo" or ""
	SHA          string            `json:"sha"`
	ScannedAt    string            `json:"scannedAt"`
	Capabilities []localCapability `json:"capabilities"`
}

// localIndexFile mirrors the top-level local-index.mjs index.json.
type localIndexFile struct {
	Version  int                       `json:"version"`
	LastScan string                    `json:"lastScan"`
	Repos    map[string]localRepoEntry `json:"repos"`
}

// capTypePriority controls which capability type becomes the "primary" for a repo.
var capTypePriority = map[string]int{
	"bundle": 0, "behavior": 1, "agent": 2,
	"recipe": 3, "package": 4, "tool": 5,
}

// localRepoToEntry transforms a local repo into the RegistryEntry shape the UI
// expects.  Returns nil if the repo has no capabilities.
func localRepoToEntry(repo localRepoEntry) json.RawMessage {
	if len(repo.Capabilities) == 0 {
		return nil
	}

	// Pick the most significant (lowest-priority-number) capability.
	primary := repo.Capabilities[0]
	for _, c := range repo.Capabilities[1:] {
		if capTypePriority[c.Type] < capTypePriority[primary.Type] {
			primary = c
		}
	}

	namespace, repoURL, install := "", "", ""
	if repo.Remote != "" {
		if idx := strings.Index(repo.Remote, "/"); idx > 0 {
			namespace = repo.Remote[:idx]
		}
		repoURL = "https://github.com/" + repo.Remote
		install = "amplifier bundle add git+https://github.com/" + repo.Remote + "@main"
	} else {
		repoURL = "file://" + repo.RepoPath
		install = "amplifier bundle add git+file://" + repo.RepoPath
	}

	// Use remote slug as ID; fall back to basename of local path.
	id := repo.Remote
	if id == "" {
		id = filepath.Base(repo.RepoPath)
	}

	description := ""
	if primary.Description != nil {
		description = *primary.Description
	}

	lastUpdated := ""
	if len(repo.ScannedAt) >= 10 {
		lastUpdated = repo.ScannedAt[:10]
	}

	entry := map[string]any{
		"id":           id,
		"name":         primary.Name,
		"namespace":    namespace,
		"description":  description,
		"type":         primary.Type,
		"category":     "dev",
		"author":       namespace,
		"repo":         repoURL,
		"install":      install,
		"rating":       nil,
		"tags":         []string{},
		"featured":     false,
		"lastUpdated":  lastUpdated,
		"private":      true,
		"localPath":    repo.RepoPath,
		"capabilities": repo.Capabilities,
	}

	b, err := json.Marshal(entry)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}

// GET /api/local-registry
func (s *Server) getLocalRegistry(w http.ResponseWriter, r *http.Request) {
	home, err := os.UserHomeDir()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot determine home dir")
		return
	}

	data, err := os.ReadFile(filepath.Join(home, ".amplifier", "bundle-index", "index.json"))
	if err != nil {
		// Not initialised yet — return empty array, not an error.
		writeJSON(w, http.StatusOK, []json.RawMessage{})
		return
	}

	var idx localIndexFile
	if err := json.Unmarshal(data, &idx); err != nil {
		writeError(w, http.StatusInternalServerError, "malformed local bundle index")
		return
	}

	entries := make([]json.RawMessage, 0, len(idx.Repos))
	for _, repo := range idx.Repos {
		if e := localRepoToEntry(repo); e != nil {
			entries = append(entries, e)
		}
	}
	writeJSON(w, http.StatusOK, entries)
}
