package api

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// findDir resolves a folder name to its full absolute path.
// Called after the browser's showDirectoryPicker() returns a handle.name.
//
// GET /api/filesystem/find-dir?name=loom
//
// macOS  — uses mdfind (Spotlight, ~100ms, no CGO)
// Linux  — uses find (slower but functional)
// other  — returns empty list
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
		// mdfind uses Spotlight — fast, no subprocess startup overhead
		query := `kMDItemKind == "Folder" && kMDItemFSName == "` + name + `"`
		out, err := exec.CommandContext(r.Context(), "mdfind", "-onlyin", home, query).Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				// Filter out system/library noise — keep user-space dirs only
				if strings.Contains(line, "/Library/") ||
					strings.Contains(line, "/Application Support/") ||
					strings.Contains(line, "/.Trash/") ||
					strings.Contains(line, "/Cache") {
					continue
				}
				// Prefer root project dirs (those that contain .git or common manifests)
				if looksLikeProject(line) {
					paths = append(paths, line)
				}
			}
		}

	case "linux":
		out, err := exec.CommandContext(r.Context(), "find", home,
			"-maxdepth", "6", "-type", "d", "-name", name,
			"-not", "-path", "*/.*").Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && looksLikeProject(line) {
					paths = append(paths, line)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"paths": paths})
}

// looksLikeProject returns true when a directory looks like a project root.
// Heuristic: it contains .git, go.mod, package.json, Cargo.toml, pyproject.toml, etc.
func looksLikeProject(dir string) bool {
	markers := []string{".git", "go.mod", "package.json", "Cargo.toml",
		"pyproject.toml", "setup.py", "pom.xml", "build.gradle", "Makefile"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}
