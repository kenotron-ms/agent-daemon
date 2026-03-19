# Agent Daemon Capabilities

This session has access to **agent-daemon** — a local job scheduler running at `http://localhost:7700`.
It runs shell commands, Claude AI prompts, and Amplifier recipes on cron, interval, file-watch, or one-shot triggers.

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
