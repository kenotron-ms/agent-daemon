package api

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ncruces/zenity"
)

// pickFolder opens the native OS directory picker via the ncruces/zenity library.
//
// GET /api/filesystem/pick-folder         — open the dialog
// GET /api/filesystem/pick-folder?check=1 — probe availability without opening
func (s *Server) pickFolder(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("check") != "" {
		writeJSON(w, http.StatusOK, map[string]any{"supported": true})
		return
	}
	path, err := zenity.SelectFile(
		zenity.Title("Select Project Folder"),
		zenity.Directory(),
		zenity.Context(r.Context()),
	)
	if err == zenity.ErrCanceled {
		writeJSON(w, http.StatusOK, map[string]any{"cancelled": true})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path})
}

// findDir resolves a folder name to candidate absolute paths via Spotlight/find.
// Used as a fallback when the native picker is unavailable.
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
