package api

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ── Directory browser ─────────────────────────────────────────────────────────
//
// GET /api/filesystem/browse          — list home directory
// GET /api/filesystem/browse?path=/x  — list arbitrary directory (dirs only)

type browseEntry struct {
	Name   string `json:"name"`
	Hidden bool   `json:"hidden"`
}

type browseResponse struct {
	Path    string        `json:"path"`
	Home    string        `json:"home"`
	Parent  string        `json:"parent,omitempty"`
	Entries []browseEntry `json:"entries"`
}

func (s *Server) browseDirs(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()

	reqPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if reqPath == "" {
		reqPath = home
	}
	// Expand ~ so frontend can send "~" as a shorthand
	if reqPath == "~" || strings.HasPrefix(reqPath, "~/") {
		reqPath = home + reqPath[1:]
	}

	absPath, err := filepath.Abs(reqPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "path is not a directory")
		return
	}

	dirEntries, err := os.ReadDir(absPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read directory: "+err.Error())
		return
	}

	var entries []browseEntry
	for _, e := range dirEntries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		entries = append(entries, browseEntry{
			Name:   name,
			Hidden: strings.HasPrefix(name, "."),
		})
	}
	if entries == nil {
		entries = []browseEntry{} // always return an array, never null
	}

	parent := ""
	if absPath != filepath.Dir(absPath) { // stops at root
		parent = filepath.Dir(absPath)
	}

	writeJSON(w, http.StatusOK, browseResponse{
		Path:    absPath,
		Home:    home,
		Parent:  parent,
		Entries: entries,
	})
}

// ── Legacy: zenity native picker (local-machine only) ────────────────────────
//
// Kept for reference; no longer wired to any route.

func pickFolderLegacy(path, home string) (string, error) {
	_ = path
	_ = home
	return "", nil
}

// ── findDir: resolve folder name → candidate paths via Spotlight/find ────────
//
// GET /api/filesystem/find-dir?name=loom
func (s *Server) findDir(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeJSON(w, http.StatusOK, map[string]any{"paths": []string{}})
		return
	}
	home, _ := os.UserHomeDir()
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		query := `kMDItemKind == "Folder" && kMDItemFSName == "` + name + `"`
		out, err := exec.CommandContext(r.Context(), "mdfind", "-onlyin", home, query).Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.Contains(line, "/Library/") ||
					strings.Contains(line, "/.Trash/") || strings.Contains(line, "/Cache") {
					continue
				}
				if looksLikeProject(line) {
					paths = append(paths, line)
				}
			}
		}
	case "linux":
		out, err := exec.CommandContext(r.Context(), "find", home,
			"-maxdepth", "6", "-type", "d", "-name", name, "-not", "-path", "*/.*").Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if line = strings.TrimSpace(line); line != "" && looksLikeProject(line) {
					paths = append(paths, line)
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"paths": paths})
}

func looksLikeProject(dir string) bool {
	for _, m := range []string{".git", "go.mod", "package.json", "Cargo.toml",
		"pyproject.toml", "setup.py", "pom.xml", "Makefile"} {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}
