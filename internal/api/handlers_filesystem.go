package api

import (
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

// pickFolder opens the native OS directory picker dialog and returns the
// selected path. macOS only — other platforms return {"supported": false}.
//
// GET /api/filesystem/pick-folder          — open the dialog
// GET /api/filesystem/pick-folder?check=1  — probe support without opening dialog
func (s *Server) pickFolder(w http.ResponseWriter, r *http.Request) {
	supported := runtime.GOOS == "darwin"

	// Capability probe: just report whether the feature is available.
	if r.URL.Query().Get("check") != "" {
		writeJSON(w, http.StatusOK, map[string]any{"supported": supported})
		return
	}

	if !supported {
		writeJSON(w, http.StatusOK, map[string]any{"supported": false})
		return
	}

	// Opens the native macOS Finder directory chooser.
	// Returns POSIX path on success, exits 1 if the user clicks Cancel.
	prompt := r.URL.Query().Get("prompt")
	if prompt == "" {
		prompt = "Select Project Folder"
	}

	cmd := exec.CommandContext(r.Context(), "osascript", "-e",
		`POSIX path of (choose folder with prompt "`+prompt+`")`,
	)
	out, err := cmd.Output()
	if err != nil {
		// User cancelled — osascript exits 1 with "User canceled." on stderr.
		writeJSON(w, http.StatusOK, map[string]any{"cancelled": true})
		return
	}

	// Trim trailing newline and the trailing slash osascript adds.
	path := strings.TrimRight(strings.TrimSpace(string(out)), "/")
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "supported": true})
}
