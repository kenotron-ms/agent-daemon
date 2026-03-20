package cli

import (
	"strings"
	"testing"
)

func TestSplitTrimmed(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", nil},
		{"whitespace only", "   ", nil},
		{"single token", "create", []string{"create"}},
		{"two tokens", "create,write", []string{"create", "write"}},
		{"tokens with spaces", "create, write, remove", []string{"create", "write", "remove"}},
		{"trailing comma", "create,", []string{"create"}},
		{"comma only", ",", nil},
		{"spaces between commas", " , ", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitTrimmed(tc.input, ",")
			if len(got) != len(tc.want) {
				t.Fatalf("splitTrimmed(%q) = %v (len %d), want %v (len %d)",
					tc.input, got, len(got), tc.want, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("splitTrimmed(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestValidateAddOpts(t *testing.T) {
	// changed() helper — simulates cmd.Flags().Changed()
	changed := func(flags ...string) map[string]bool {
		m := make(map[string]bool)
		for _, f := range flags {
			m[f] = true
		}
		return m
	}
	none := map[string]bool{}

	tests := []struct {
		name         string
		opts         addOpts
		changedFlags map[string]bool
		wantErr      string
	}{
		{
			name:         "missing name",
			opts:         addOpts{triggerType: "once", executorType: "shell", command: "echo hi"},
			changedFlags: none,
			wantErr:      "--name is required",
		},
		{
			name:         "invalid trigger",
			opts:         addOpts{name: "x", triggerType: "foobar", executorType: "shell", command: "echo"},
			changedFlags: none,
			wantErr:      `invalid trigger "foobar"`,
		},
		{
			name:         "invalid executor",
			opts:         addOpts{name: "x", triggerType: "once", executorType: "bash", command: "echo"},
			changedFlags: none,
			wantErr:      `invalid executor "bash"`,
		},
		{
			name:         "shell missing command",
			opts:         addOpts{name: "x", triggerType: "once", executorType: "shell"},
			changedFlags: none,
			wantErr:      `--command is required for executor "shell"`,
		},
		{
			name:         "command with claude-code",
			opts:         addOpts{name: "x", triggerType: "once", executorType: "claude-code", command: "echo", prompt: "hi"},
			changedFlags: changed("command"),
			wantErr:      "--command is only valid with --executor shell",
		},
		{
			name:         "claude-code missing prompt",
			opts:         addOpts{name: "x", triggerType: "once", executorType: "claude-code"},
			changedFlags: none,
			wantErr:      `--prompt is required for executor "claude-code"`,
		},
		{
			name:         "amplifier missing both",
			opts:         addOpts{name: "x", triggerType: "once", executorType: "amplifier"},
			changedFlags: none,
			wantErr:      `--prompt or --recipe is required for executor "amplifier"`,
		},
		{
			name:         "recipe with shell",
			opts:         addOpts{name: "x", triggerType: "once", executorType: "shell", command: "echo", recipe: "r.yaml"},
			changedFlags: changed("recipe"),
			wantErr:      "--recipe is only valid with --executor amplifier",
		},
		{
			name:         "model with shell",
			opts:         addOpts{name: "x", triggerType: "once", executorType: "shell", command: "echo", model: "opus"},
			changedFlags: changed("model"),
			wantErr:      "--model is only valid with --executor claude-code or amplifier",
		},
		{
			name:         "watch missing watch-path",
			opts:         addOpts{name: "x", triggerType: "watch", executorType: "shell", command: "echo"},
			changedFlags: none,
			wantErr:      "--watch-path is required when --trigger watch",
		},
		{
			name:         "watch-recursive without watch trigger",
			opts:         addOpts{name: "x", triggerType: "once", executorType: "shell", command: "echo", watchRecursive: true},
			changedFlags: changed("watch-recursive"),
			wantErr:      "--watch-recursive requires --trigger watch",
		},
		{
			name:         "invalid watch-mode",
			opts:         addOpts{name: "x", triggerType: "watch", executorType: "shell", command: "echo", watchPath: "/tmp", watchMode: "inotify"},
			changedFlags: changed("watch-mode"),
			wantErr:      `invalid --watch-mode "inotify"`,
		},
		{
			name:         "invalid watch-events token",
			opts:         addOpts{name: "x", triggerType: "watch", executorType: "shell", command: "echo", watchPath: "/tmp", watchMode: "notify", watchEvents: "created"},
			changedFlags: changed("watch-events"),
			wantErr:      `invalid event "created"`,
		},
		{
			name:         "poll mode with rename event",
			opts:         addOpts{name: "x", triggerType: "watch", executorType: "shell", command: "echo", watchPath: "/tmp", watchMode: "poll", watchEvents: "rename"},
			changedFlags: changed("watch-mode", "watch-events"),
			wantErr:      `--watch-mode poll does not support event "rename"`,
		},
		{
			name:         "poll-interval without poll mode",
			opts:         addOpts{name: "x", triggerType: "watch", executorType: "shell", command: "echo", watchPath: "/tmp", watchMode: "notify", watchPollInterval: "2s"},
			changedFlags: changed("watch-poll-interval"),
			wantErr:      "--watch-poll-interval requires --watch-mode poll",
		},
		{
			name:         "invalid watch-debounce duration",
			opts:         addOpts{name: "x", triggerType: "watch", executorType: "shell", command: "echo", watchPath: "/tmp", watchMode: "notify", watchDebounce: "five seconds"},
			changedFlags: changed("watch-debounce"),
			wantErr:      `invalid --watch-debounce "five seconds"`,
		},
		{
			name: "valid shell+watch",
			opts: addOpts{
				name: "watcher", triggerType: "watch", executorType: "shell",
				command: "make", watchPath: "/tmp", watchMode: "notify",
			},
			changedFlags: none,
			wantErr:      "",
		},
		{
			name: "valid claude+cron",
			opts: addOpts{
				name: "daily", triggerType: "cron", executorType: "claude-code",
				prompt: "summarize", schedule: "0 0 9 * * *",
			},
			changedFlags: none,
			wantErr:      "",
		},
		{
			name: "valid amplifier+watch recipe only",
			opts: addOpts{
				name: "proc", triggerType: "watch", executorType: "amplifier",
				recipe: "r.yaml", watchPath: "/tmp", watchMode: "notify",
			},
			changedFlags: none,
			wantErr:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAddOpts(tc.opts, tc.changedFlags)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
			}
		})
	}
}
