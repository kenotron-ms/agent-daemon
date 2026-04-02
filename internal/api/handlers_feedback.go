package api

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// ghBin finds the gh CLI binary, searching common install locations in
// addition to PATH — necessary when loom runs as a launchd/systemd service
// that has a stripped PATH (e.g. /opt/homebrew/bin is absent).
func ghBin() (string, error) {
	if p, err := exec.LookPath("gh"); err == nil {
		return p, nil
	}
	candidates := []string{
		"/opt/homebrew/bin/gh",    // macOS Apple Silicon
		"/usr/local/bin/gh",       // macOS Intel / Linux
		"/home/linuxbrew/.linuxbrew/bin/gh",
	}
	// Also check $HOME/.local/bin and whatever's on PATH
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, home+"/.local/bin/gh")
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", &exec.Error{Name: "gh", Err: exec.ErrNotFound}
}

type feedbackRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type feedbackResponse struct {
	URL string `json:"url"`
}

// POST /api/feedback
// Files a GitHub issue on kenotron-ms/amplifier-app-loom via the gh CLI.
func (s *Server) createFeedback(w http.ResponseWriter, r *http.Request) {
	var req feedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	args := []string{
		"issue", "create",
		"--repo", "kenotron-ms/amplifier-app-loom",
		"--title", req.Title,
	}
	if req.Body != "" {
		args = append(args, "--body", req.Body)
	} else {
		args = append(args, "--body", "*(no description provided)*")
	}

	bin, err := ghBin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "gh CLI not found — install it from https://cli.github.com")
		return
	}
	out, err := exec.CommandContext(r.Context(), bin, args...).Output()
	if err != nil {
		// Surface the stderr if available for better diagnostics
		msg := "failed to create issue: gh CLI error"
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			msg = "gh: " + strings.TrimSpace(string(exitErr.Stderr))
		}
		writeError(w, http.StatusInternalServerError, msg)
		return
	}

	url := strings.TrimSpace(string(out))
	writeJSON(w, http.StatusCreated, feedbackResponse{URL: url})
}
