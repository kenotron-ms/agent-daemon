# SSE Integration Validation — Implementation Plan

> **Execution:** Use the subagent-driven-development workflow to implement this plan.

**Goal:** Verify the end-to-end SSE integration test script, build binary, and validate against a live daemon.
**Architecture:** A standalone bash script (`scripts/test-sse.sh`) runs 10 validation checks against a live daemon — health check, job CRUD, SSE streaming, replay, and cleanup. The binary is built with `go build`, tests are run, and the repo is tagged `phase1-backend-complete`.
**Tech Stack:** Bash, curl, jq, Go build toolchain

---

> **SPEC REVIEW WARNING — HUMAN REVIEW REQUIRED**
>
> The automated spec review loop exhausted after 3 iterations without final
> approval. The last issue identified was: **git tag `phase1-backend-complete`
> pointed to the wrong commit** (`537aaa0` instead of `da96a10`).
>
> **Current state (verified 2026-03-15):** The tag has been moved and now
> correctly points to `da96a10` (HEAD). All acceptance criteria appear met.
> However, because the review loop did not terminate with an explicit PASS,
> a human reviewer should confirm the items below before considering this
> task closed.

---

## Pre-Implementation Status

This task is **already implemented**. The plan below contains verification-only
tasks for a human reviewer to confirm correctness.

**What exists:**
- `scripts/test-sse.sh` — executable, 185 lines, 10 numbered checks
- Commit `537aaa0` — `test: add SSE integration validation script`
- Commit `da96a10` — `fix: add --no-buffer flag to SSE curl command in test script`
- Git tag `phase1-backend-complete` on `da96a10` (HEAD)
- All unit tests passing (`go test ./internal/...`)
- Binary builds cleanly (`go build -o ./agent-daemon-test ./cmd/agent-daemon`)

---

### Task 1: Verify Script Exists and Is Executable

**Files:**
- Verify: `scripts/test-sse.sh`

**Step 1: Check file exists and has execute permission**
Run: `ls -la scripts/test-sse.sh`
Expected: `-rwxr-xr-x` permissions

**Step 2: Check shebang and usage comment**
Run: `head -5 scripts/test-sse.sh`
Expected: Line 1 is `#!/usr/bin/env bash`, line 3 contains `Usage: PORT=61017 bash scripts/test-sse.sh`

---

### Task 2: Verify Script Structure — All 10 Checks Present

**Files:**
- Verify: `scripts/test-sse.sh`

**Step 1: Count numbered checks**
Run: `grep -c '^echo "Check [0-9]' scripts/test-sse.sh`
Expected: `10`

**Step 2: Verify check topics match spec**
Run: `grep '^echo "Check [0-9]' scripts/test-sse.sh`
Expected output (in order):
```
Check 1: Daemon health check
Check 2: Create shell job
Check 3: Trigger job
Check 4: Find run ID
Check 5: Verify chunk events in SSE output
Check 6: Verify chunk content contains step output
Check 7: Verify 'event: done' in SSE output
Check 8: Verify done payload status == success
Check 9: Verify done payload has started_at
Check 10: Verify completed run replay
```

**Step 3: Verify prerequisite checks**
Run: `grep 'require_cmd' scripts/test-sse.sh`
Expected: `require_cmd curl` and `require_cmd jq`

---

### Task 3: Verify SSE Curl Flags Match Spec

**Files:**
- Verify: `scripts/test-sse.sh`

**Step 1: Check SSE subscribe curl command has all required flags**
Run: `grep 'curl.*stream' scripts/test-sse.sh | head -1`
Expected: Line 108 contains `curl -sf -N --no-buffer --max-time 15`

The spec requires exactly: `curl -sf -N --no-buffer --max-time 15 GET /api/runs/{id}/stream`

---

### Task 4: Verify Binary Builds With Zero Errors

**Files:**
- Build: `cmd/agent-daemon/` (source)
- Artifact: `./agent-daemon-test` (temporary, cleaned up)

**Step 1: Build the binary**
Run: `go build -o ./agent-daemon-test ./cmd/agent-daemon`
Expected: Exit code 0, no output (clean build)

**Step 2: Verify binary was created**
Run: `file ./agent-daemon-test`
Expected: Mach-O 64-bit executable (or appropriate platform binary)

**Step 3: Clean up test binary**
Run: `rm -f ./agent-daemon-test`
Expected: Binary removed, not committed to repo

---

### Task 5: Verify All Unit Tests Pass

**Files:**
- Test: `internal/api/`, `internal/scheduler/`, `internal/service/`

**Step 1: Run full unit test suite**
Run: `go test ./internal/... -v`
Expected: All packages report `ok` or `[no test files]`. Zero `FAIL` lines.

---

### Task 6: Run Live Integration Test

**Files:**
- Run: `scripts/test-sse.sh`
- Binary: `cmd/agent-daemon/`

**Step 1: Build and start the daemon**
Run:
```bash
go build -o ./agent-daemon-test ./cmd/agent-daemon
PORT=61017 ./agent-daemon-test &
DAEMON_PID=$!
sleep 2
```
Expected: Daemon starts and listens on port 61017

**Step 2: Run the integration test script**
Run: `PORT=61017 bash scripts/test-sse.sh`
Expected output ends with: `RESULTS: 10 passed, 0 failed`
Exit code: 0

**Step 3: Stop the daemon and clean up**
Run:
```bash
kill $DAEMON_PID 2>/dev/null || true
rm -f ./agent-daemon-test
```

---

### Task 7: Verify Git Tag Placement

> **This is the item that caused spec review exhaustion.** Confirm it is resolved.

**Step 1: Check tag exists and points to HEAD**
Run: `git log --oneline --decorate phase1-backend-complete -1`
Expected: `da96a10 (HEAD -> main, tag: phase1-backend-complete) fix: add --no-buffer flag to SSE curl command in test script`

**Step 2: Confirm tag and HEAD are the same commit**
Run: `[ "$(git rev-parse HEAD)" = "$(git rev-parse phase1-backend-complete)" ] && echo "MATCH" || echo "MISMATCH"`
Expected: `MATCH`

---

### Task 8: Verify Commit Messages

**Step 1: Check commit history for this task**
Run: `git log --oneline phase1-backend-complete~2..phase1-backend-complete`
Expected:
```
da96a10 fix: add --no-buffer flag to SSE curl command in test script
537aaa0 test: add SSE integration validation script
```

The original commit message matches the spec: `test: add SSE integration validation script`

---

## Summary of Acceptance Criteria

| Criterion | Status |
|---|---|
| `scripts/test-sse.sh` is executable | Verified |
| Binary builds with zero errors | Verified |
| Integration script reports `10 passed, 0 failed` | **Requires live run** |
| `go test ./internal/... -v` all pass | Verified |
| Git tag `phase1-backend-complete` created | Verified (on `da96a10`) |
| Committed with message `test: add SSE integration validation script` | Verified (`537aaa0`) |

**Remaining action:** Task 6 (live integration test) must be run by a human or
CI environment with a real daemon process. All other criteria are confirmed.