# loom

A cross-platform scheduled job runner with a web UI and AI assistant. Run shell commands, Claude Code sessions, or Amplifier recipes on a schedule — or whenever a file changes.

## Features

- **Multiple trigger types**
  - `cron` — standard cron with seconds (e.g. `0 */5 * * * *`)
  - `loop` — repeating interval (e.g. `30s`, `5m`, `1h`)
  - `once` — run once then auto-disable, with optional delay (`10m`, `2h`)
  - `watch` — fire when a file or directory changes (OS-level notify or polling)
  - `connector` — fire when a mirror connector detects a change in an external service
- **Multiple executor types**
  - `shell` — run any shell command
  - `claude-code` — run `claude -p` with multi-step/resume support
  - `amplifier` — run `amplifier run` or a YAML recipe file
- **Web UI** at `http://localhost:7700` — add, edit, enable/disable, and run jobs
- **AI assistant** — describe jobs in plain English ("every 5 minutes, ask claude code to check for lint errors")
- **System tray app** (macOS/Windows/Linux with CGO) — start/stop/pause/open UI from the menu bar
- **Job queue** — bounded concurrency (configurable, default: 4 parallel), deduplication, configurable retries and timeouts
- **System service** — install as a LaunchAgent (macOS), systemd unit (Linux), or Windows Service
- **Persistent storage** — embedded bbolt database, no external dependencies

## Installation

### macOS / Linux — one-liner

```sh
curl -fsSL https://raw.githubusercontent.com/kenotron-ms/amplifier-app-loom/main/install.sh | sh
```

This detects your OS and architecture, downloads the latest binary from GitHub Releases, installs it to `/usr/local/bin`, and tells you if you need to update your `PATH`.

### Windows — PowerShell

```powershell
irm https://raw.githubusercontent.com/kenotron-ms/amplifier-app-loom/main/install.ps1 | iex
```

Installs to `%LOCALAPPDATA%\Programs\loom` and adds it to your user `PATH` automatically. To use a different directory:

```powershell
$env:INSTALL_DIR="C:\tools"; irm .../install.ps1 | iex
```

### Manual download

Pre-built binaries are on the [GitHub Releases](https://github.com/kenotron-ms/amplifier-app-loom/releases) page:

| Platform | Binary |
|---|---|
| macOS (Apple Silicon) | `loom-darwin-arm64` |
| macOS (Intel) | `loom-darwin-amd64` |
| Linux (amd64) | `loom-linux-amd64` |
| Linux (arm64) | `loom-linux-arm64` |
| Windows (amd64) | `loom-windows-amd64.exe` |

Download, `chmod +x` (Unix), and place in any directory on your `PATH`.

### Build from source

```sh
git clone https://github.com/kenotron-ms/amplifier-app-loom.git
cd amplifier-app-loom
make build          # native binary (with tray support if CGO available)
make cross          # all platforms → dist/
```

## Quick start

```sh
# Install as a user-level service (no sudo required)
loom install

# Start the daemon
loom start

# Open the web UI
open http://localhost:7700

# Check status
loom status

# Stop
loom stop

# Uninstall
loom uninstall
```

For a system-level service (starts at boot, requires `sudo`):

```sh
sudo loom install --system
sudo loom start --system
```

## CLI reference

```
loom <command> [flags]

Service management:
  install    Install as a system service (--system for boot-level)
  uninstall  Remove the system service
  start      Start the daemon
  stop       Stop the daemon
  status     Show daemon status
  update     Update loom to the latest release (stops service, swaps binary, restarts)

Scheduler control:
  pause      Pause job dispatching (running jobs continue)
  resume     Resume job dispatching
  flush      Clear the pending job queue

Job management:
  list       List all jobs
  add        Add a job (--name, --trigger, --schedule, --command, ...)
  remove     Remove a job by ID or ID prefix
  prune      Delete all disabled jobs (--dry-run, -y)

Configuration:
  config absorb-env  Auto-detect AI API keys from environment and save them to the daemon config

Other:
  tray       Launch the system tray app
```

## Configuration

The daemon supports both Anthropic and OpenAI for the AI assistant feature. Set one before installing — or use `loom config absorb-env` afterwards to auto-detect keys from your environment:

```sh
# Anthropic
export ANTHROPIC_API_KEY=sk-ant-...
loom install

# — or — OpenAI
export OPENAI_API_KEY=sk-...
loom install

# — or — detect and persist whatever keys are already in the environment
loom config absorb-env
```

`loom config absorb-env` searches `$ANTHROPIC_API_KEY`, `$OPENAI_API_KEY`, `~/.amplifier/keys.env`, `~/.anthropic/api_key`, `~/.env`, and common shell dotfiles, then saves any found keys into the daemon's config database. Useful after a system-level install where the service doesn't inherit your shell environment.

Default port is `7700`. The database is stored at:
- macOS: `~/Library/Application Support/loom/loom.db`
- Linux: `~/.local/share/loom/loom.db`
- Windows: `%APPDATA%\loom\loom.db`

## Watch trigger

Monitor a file or directory and run a job whenever it changes:

```json
{
  "trigger": { "type": "watch" },
  "watch": {
    "path": "/path/to/project",
    "recursive": true,
    "events": ["create", "write", "remove"],
    "mode": "notify",
    "debounce": "500ms"
  }
}
```

- `mode: "notify"` uses OS-level events (inotify/FSEvents/kqueue) — efficient, recommended
- `mode: "poll"` checks for changes on a timer — works on network drives and containers
- `debounce` waits for a quiet period before firing to avoid rapid re-triggers

## License

MIT
