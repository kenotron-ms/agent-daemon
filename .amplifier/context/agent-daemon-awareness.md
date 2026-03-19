# Agent Daemon Capabilities

This session has access to **agent-daemon** — a local job scheduler running at `http://localhost:7700`.
It runs shell commands, Claude AI prompts, and Amplifier recipes on cron, interval, file-watch, or one-shot triggers.

## Installation

**One-liner (macOS / Linux):**
```bash
curl -fsSL https://raw.githubusercontent.com/kenotron-ms/agent-daemon/main/.amplifier/scripts/install.sh | bash
```

This installs the binary, registers it as a background service (auto-starts on login), and on macOS launches the menu bar tray app.

**What the installer does:**
1. Downloads the latest binary for the current OS/arch from GitHub Releases
2. Installs to `/usr/local/bin/agent-daemon`
3. Runs `agent-daemon install` — registers as a user-level launchd agent (macOS) or systemd user service (Linux)
4. Runs `agent-daemon start` — starts the daemon immediately
5. **macOS only:** launches `agent-daemon tray` (menu bar icon) and adds it to Login Items

**Manual service commands** (if already installed):
```bash
agent-daemon install   # register as background service
agent-daemon start     # start the service
agent-daemon stop      # stop the service
agent-daemon uninstall # remove the service
agent-daemon tray      # launch the macOS menu bar app
```

**Check it's running:**
```bash
agent-daemon status
# or open http://localhost:7700
```

---

## Using from Amplifier

Load the skill before running any agent-daemon command:

```
load_skill(skill_name="agent-daemon-cli")
```

## Scripts Path

The CLI script lives inside the agent-daemon bundle cache. Resolve it once per session:

```bash
export AGENT_DAEMON_ROOT=$(ls -dt ~/.amplifier/cache/amplifier-bundle-agent-daemon-* 2>/dev/null | head -1)
# local dev fallback (when running from the bundle repo itself)
[ -f "scripts/agent-daemon-cli.mjs" ] && export AGENT_DAEMON_ROOT="."
```

All commands: `node "$AGENT_DAEMON_ROOT/scripts/agent-daemon-cli.mjs" <command> --json`

## Executor Types

| Executor | What it runs |
|----------|-------------|
| `shell` | A shell command (default) |
| `claude-code` | An AI prompt via `claude -p` |
| `amplifier` | An Amplifier prompt or `.yaml` recipe |
