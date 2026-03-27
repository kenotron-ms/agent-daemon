package mirror

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// CommandFetcher runs a shell command and captures its stdout as JSON data.
// This is the simplest fetcher — works with `gh api`, `curl`, custom scripts, etc.
type CommandFetcher struct {
	// Shell to use for executing commands. Defaults to "sh".
	Shell string
	// Timeout for command execution. Defaults to 30s.
	Timeout time.Duration
}

// NewCommandFetcher returns a CommandFetcher with sensible defaults.
func NewCommandFetcher() *CommandFetcher {
	return &CommandFetcher{
		Shell:   "sh",
		Timeout: 30 * time.Second,
	}
}

// Fetch executes the connector's Command in a shell and returns the stdout as JSON.
func (f *CommandFetcher) Fetch(conn *Connector) (*FetchResult, error) {
	if conn.Command == "" {
		return nil, fmt.Errorf("command fetcher: connector %s has no command configured", conn.ID)
	}

	timeout := f.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	shell := f.Shell
	if shell == "" {
		shell = "sh"
	}

	cmd := exec.CommandContext(ctx, shell, "-c", conn.Command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("command fetcher: %s (stderr: %s)", err, stderr.String())
	}

	output := bytes.TrimSpace(stdout.Bytes())

	// Validate it's valid JSON
	if !json.Valid(output) {
		// Wrap non-JSON output as a JSON string
		wrapped, _ := json.Marshal(string(output))
		output = wrapped
	}

	return &FetchResult{
		Data:      json.RawMessage(output),
		FetchedAt: time.Now(),
	}, nil
}