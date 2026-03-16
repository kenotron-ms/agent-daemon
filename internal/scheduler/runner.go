package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ms/agent-daemon/internal/config"
	"github.com/ms/agent-daemon/internal/store"
	"github.com/ms/agent-daemon/internal/types"
)

const maxOutputBytes = 64 * 1024 // 64KB cap on stored output

type Runner struct {
	store           store.Store
	broadcaster     *Broadcaster
	userCtx         *config.UserContext
	shellPATHOnce   sync.Once
	cachedShellPATH string
}

func NewRunner(s store.Store, b *Broadcaster, userCtx *config.UserContext) *Runner {
	return &Runner{store: s, broadcaster: b, userCtx: userCtx}
}

// shellPATH returns the user's full login-shell PATH, captured by spawning the
// shell stored in UserContext with -l (login) and -i (interactive) flags so all
// rc files are sourced — nvm, homebrew shims, cargo, pyenv, ~/.local/bin, etc.
//
// Using the stored shell and HOME (rather than os.Getenv which is stripped by
// launchd) ensures we get the exact same PATH the user sees in their terminal.
// Result is cached after the first call.
func (r *Runner) shellPATH() string {
	r.shellPATHOnce.Do(func() {
		shell := "/bin/zsh"
		var extraEnv []string
		if r.userCtx != nil {
			if r.userCtx.Shell != "" {
				shell = r.userCtx.Shell
			}
			if r.userCtx.HomeDir != "" {
				// Set HOME so nvm/brew find their directories correctly.
				extraEnv = append(extraEnv, "HOME="+r.userCtx.HomeDir)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Use a sentinel so any stray stdout from .zshrc/.bashrc doesn't corrupt the value.
		cmd := exec.CommandContext(ctx, shell, "-l", "-i", "-c",
			"printf 'AGENTD_SHELL_PATH=%s' \"$PATH\"")
		if len(extraEnv) > 0 {
			cmd.Env = append(os.Environ(), extraEnv...)
		}
		out, err := cmd.Output() // captures stdout only; stderr noise discarded
		if err != nil {
			slog.Warn("failed to capture user shell PATH", "shell", shell, "err", err)
			return
		}
		const sentinel = "AGENTD_SHELL_PATH="
		s := string(out)
		if idx := strings.LastIndex(s, sentinel); idx >= 0 {
			r.cachedShellPATH = s[idx+len(sentinel):]
		}
	})
	return r.cachedShellPATH
}

// resolveBinary returns the absolute path to name by searching the user's
// login-shell PATH first, then falling back to the daemon's inherited PATH.
func (r *Runner) resolveBinary(name string) (string, error) {
	if p := r.shellPATH(); p != "" {
		for _, dir := range filepath.SplitList(p) {
			candidate := filepath.Join(dir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s: not found in user shell PATH or daemon PATH", name)
}

// baseEnv returns the daemon's environment with the user's HOME, USER, SHELL,
// and PATH overlaid so job processes run as if launched from the user's shell.
func (r *Runner) baseEnv() []string {
	if r.userCtx == nil {
		return os.Environ()
	}
	overrides := map[string]string{
		"HOME":  r.userCtx.HomeDir,
		"USER":  r.userCtx.Username,
		"SHELL": r.userCtx.Shell,
		"PATH":  r.shellPATH(),
	}
	env := os.Environ()
	result := make([]string, 0, len(env))
	for _, e := range env {
		key := strings.SplitN(e, "=", 2)[0]
		if _, overridden := overrides[key]; !overridden {
			result = append(result, e)
		}
	}
	for k, v := range overrides {
		if v != "" {
			result = append(result, k+"="+v)
		}
	}
	return result
}

// commandFor builds an exec.Cmd for the named binary. It resolves the absolute
// path via the user's shell PATH (so nvm/brew/cargo-installed tools are found)
// and sets the user's full environment on the command.
func (r *Runner) commandFor(ctx context.Context, name string, args ...string) *exec.Cmd {
	bin, err := r.resolveBinary(name)
	if err != nil {
		bin = name // let Start() fail with a clear "not found" message
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = r.baseEnv()
	return cmd
}

// Execute dispatches to the correct executor based on job type.
func (r *Runner) Execute(job *types.Job) {
	maxRetries := job.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			time.Sleep(backoff)
		}
		done := r.runAttempt(job, attempt+1)
		if done {
			return
		}
	}
}

// runAttempt runs one attempt. Returns true if we should stop retrying.
func (r *Runner) runAttempt(job *types.Job, attempt int) (stopRetrying bool) {
	ctx := context.Background()
	var cancel context.CancelFunc
	if job.Timeout != "" {
		if d, err := time.ParseDuration(job.Timeout); err == nil && d > 0 {
			ctx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
	}

	run := &types.JobRun{
		ID:        uuid.New().String(),
		JobID:     job.ID,
		JobName:   job.Name,
		StartedAt: time.Now(),
		Status:    types.RunStatusRunning,
		Attempt:   attempt,
	}
	_ = r.store.SaveRun(context.Background(), run)

	if r.broadcaster != nil {
		r.broadcaster.Register(run.ID)
		defer r.broadcaster.Complete(run.ID)
	}

	slog.Info("job starting", "job", job.Name, "executor", job.ResolvedExecutor(), "attempt", attempt)

	var output string
	var exitCode int
	var runErr error

	switch job.ResolvedExecutor() {
	case types.ExecutorClaudeCode:
		output, exitCode, runErr = r.execClaudeCode(ctx, job, run.ID)
	case types.ExecutorAmplifier:
		output, exitCode, runErr = r.execAmplifier(ctx, job, run.ID)
	default: // ExecutorShell + backward compat
		output, exitCode, runErr = r.execShell(ctx, job, run.ID)
	}

	now := time.Now()
	run.EndedAt = &now
	// If the executor produced no output but did return an error (e.g. binary
	// not found, bad CWD), surface the error text so the UI shows something
	// useful instead of a blank error.
	if output == "" && runErr != nil {
		output = runErr.Error()
	}
	run.Output = capOutput(output)
	run.ExitCode = exitCode

	if ctx.Err() == context.DeadlineExceeded {
		run.Status = types.RunStatusTimeout
		slog.Warn("job timed out", "job", job.Name, "attempt", attempt)
		return true // no retry on timeout
	}
	if runErr != nil {
		run.Status = types.RunStatusFailed
		slog.Warn("job failed", "job", job.Name, "attempt", attempt, "err", runErr)
		_ = r.store.SaveRun(context.Background(), run)
		return false // allow retry
	}

	run.Status = types.RunStatusSuccess
	slog.Info("job succeeded", "job", job.Name, "attempt", attempt)
	_ = r.store.SaveRun(context.Background(), run)
	return true
}

func capOutput(s string) string {
	b := []byte(s)
	if len(b) > maxOutputBytes {
		b = b[len(b)-maxOutputBytes:]
	}
	return string(b)
}
