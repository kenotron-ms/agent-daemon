#!/usr/bin/env node
/**
 * agent-daemon-cli
 *
 * Headless agent-daemon CLI for AI agents. Hybrid approach:
 *   - Service lifecycle (install/uninstall/start/stop): shells out to agent-daemon binary
 *   - Everything else: calls the HTTP API at localhost:7700 (or --port N)
 *
 * Requires Node.js 18+ (uses global fetch, no npm dependencies).
 *
 * USAGE:
 *   node scripts/agent-daemon-cli.mjs <command> [options]
 *
 * DAEMON STATUS & CONTROL:
 *   status                           Show daemon status
 *   pause                            Pause job scheduling
 *   resume                           Resume job scheduling
 *   flush                            Flush pending jobs from queue
 *
 * JOB MANAGEMENT:
 *   list                             List all jobs
 *   get       --id ID                Get a job by ID or prefix
 *   add       --name NAME            Create a new job
 *             --trigger TYPE         cron | loop | once | watch  (default: once)
 *             --schedule EXPR        Cron expression or duration (e.g. "0 9 * * *" or "5m")
 *             --executor TYPE        shell | claude-code | amplifier  (default: shell)
 *             --command CMD          Shell command  (executor=shell)
 *             --prompt TEXT          Prompt text  (executor=amplifier or claude-code)
 *             --steps TEXT           Additional prompt turns; repeat for each step
 *             --recipe PATH          Recipe .yaml path  (executor=amplifier, optional alongside --prompt)
 *             --bundle NAME          Bundle name  (executor=amplifier)
 *             --model MODEL          Model name  (executor=amplifier or claude-code)
 *             --context KEY=VAL      Recipe context variable; repeat for multiple (executor=amplifier)
 *             --max-turns N          Max conversation turns  (executor=claude-code)
 *             --cwd DIR              Working directory
 *             --timeout DUR          Max execution time e.g. 30s, 5m
 *             --retries N            Retries on failure (default: 0)
 *             --description TEXT     Job description
 *             --watch-path PATH      Path to watch  (trigger=watch)
 *             --watch-recursive      Watch subdirectories  (trigger=watch)
 *             --watch-events LIST    Comma-separated: create,write,remove,rename,chmod
 *             --watch-debounce DUR   Quiet window before firing e.g. 500ms
 *   update    --id ID [same flags]   Update an existing job
 *   delete    --id ID --yes          Delete a job (--yes bypasses confirmation)
 *   trigger   --id ID                Trigger a job immediately
 *   enable    --id ID                Enable a disabled job
 *   disable   --id ID                Disable a job
 *   prune                            Delete all disabled jobs
 *
 * RUNS:
 *   runs      [--limit N]            List recent runs (default: 50)
 *   run       --id ID                Get a specific run with full output
 *   job-runs  --id ID [--limit N]    List runs for a specific job
 *   clear-runs                       Clear all run history
 *
 * SERVICE LIFECYCLE (shells out to agent-daemon binary):
 *   install                          Install as system service
 *   uninstall                        Uninstall system service
 *   start                            Start the daemon service
 *   stop                             Stop the daemon service
 *
 * OPTIONS:
 *   --json         Machine-readable JSON output
 *   --port N       Daemon port (default: 7700)
 *   --yes, -y      Skip confirmation prompts
 *   --help, -h     Show this help
 *
 * JSON OUTPUT SCHEMA:
 *   status:     { state, pid, startedAt, activeRuns, queueDepth, jobCount, version }
 *   list:       Job[]
 *   get:        Job
 *   add:        Job  (created)
 *   update:     Job  (updated)
 *   delete:     { deleted: true, id }  |  { deleted: false, reason }
 *   trigger:    { status: "triggered" }
 *   enable:     Job
 *   disable:    Job
 *   prune:      { deleted: N }
 *   runs:       JobRun[]
 *   run:        JobRun
 *   job-runs:   JobRun[]
 *   clear-runs: { cleared: true }
 *   install/uninstall/start/stop: { success: true, output }
 *   errors:     { error: string }  +  exit code 1
 */

import { execSync } from 'child_process';
import { fileURLToPath } from 'url';
import { realpathSync } from 'fs';

const DEFAULT_PORT = 7700;
const SERVICE_COMMANDS = new Set(['install', 'uninstall', 'start', 'stop']);

// ── Arg parsing ───────────────────────────────────────────────────────────────

function parseArgs(argv) {
  const args = argv.slice(2);
  const opts = { _: [] };
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if      (a === '--json')             opts.json = true;
    else if (a === '--yes' || a === '-y') opts.yes = true;
    else if (a === '--help' || a === '-h') opts.help = true;
    else if (a === '--port')             opts.port = Number(args[++i]);
    else if (a === '--id')               opts.id = args[++i];
    else if (a === '--name')             opts.name = args[++i];
    else if (a === '--description')      opts.description = args[++i];
    else if (a === '--trigger')          opts.trigger = args[++i];
    else if (a === '--schedule')         opts.schedule = args[++i];
    else if (a === '--executor')         opts.executor = args[++i];
    else if (a === '--command')          opts.command = args[++i];
    else if (a === '--prompt')           opts.prompt = args[++i];
    else if (a === '--recipe')           opts.recipe = args[++i];
    else if (a === '--bundle')           opts.bundle = args[++i];
    else if (a === '--model')            opts.model = args[++i];
    else if (a === '--steps')            (opts.steps = opts.steps ?? []).push(args[++i]);
    else if (a === '--context')          (opts.context = opts.context ?? []).push(args[++i]);
    else if (a === '--max-turns')        opts.maxTurns = Number(args[++i]);
    else if (a === '--cwd')              opts.cwd = args[++i];
    else if (a === '--timeout')          opts.timeout = args[++i];
    else if (a === '--retries')          opts.retries = Number(args[++i]);
    else if (a === '--limit')            opts.limit = Number(args[++i]);
    else if (a === '--watch-path')       opts.watchPath = args[++i];
    else if (a === '--watch-recursive')  opts.watchRecursive = true;
    else if (a === '--watch-events')     opts.watchEvents = args[++i].split(',');
    else if (a === '--watch-debounce')   opts.watchDebounce = args[++i];
    else if (!a.startsWith('-'))         opts._.push(a);
  }
  return opts;
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

function apiBase(port) {
  return `http://localhost:${port ?? DEFAULT_PORT}/api`;
}

async function apifetch(port, path, method = 'GET', body = null) {
  const url = `${apiBase(port)}${path}`;
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body !== null) opts.body = JSON.stringify(body);

  let res;
  try {
    res = await fetch(url, opts);
  } catch (err) {
    throw new Error(`Daemon not reachable at ${apiBase(port)} — is it running? (${err.message})`);
  }

  const text = await res.text();
  let data;
  try { data = JSON.parse(text); } catch { data = text; }

  if (!res.ok) {
    const msg = (typeof data === 'object' && data?.error) ? data.error : text.slice(0, 200);
    throw Object.assign(new Error(`${method} ${path} → ${res.status}: ${msg}`), { status: res.status, data });
  }
  return data;
}

// Resolve a short ID prefix to a full job UUID
async function resolveID(port, prefix) {
  const jobs = await apifetch(port, '/jobs');
  const matches = jobs.filter(j => j.id === prefix || j.id.startsWith(prefix));
  if (matches.length === 0) throw new Error(`No job found matching '${prefix}'`);
  if (matches.length > 1)   throw new Error(`Ambiguous prefix '${prefix}' matches ${matches.length} jobs — use more characters`);
  return matches[0].id;
}

// ── Service lifecycle (binary) ────────────────────────────────────────────────

function runServiceCmd(cmd) {
  try {
    const output = execSync(`agent-daemon ${cmd}`, { encoding: 'utf8', stdio: ['inherit', 'pipe', 'pipe'] });
    return { success: true, output: output.trim() };
  } catch (err) {
    const msg = (err.stderr ?? err.message ?? '').trim();
    throw new Error(`agent-daemon ${cmd} failed: ${msg}`);
  }
}

// ── Daemon control ────────────────────────────────────────────────────────────

async function cmdStatus({ port, json }) {
  const d = await apifetch(port, '/status');
  if (!json) {
    const icon = d.state === 'paused' ? '⏸' : '●';
    console.log(`${icon} agent-daemon  [${d.state}]  v${d.version}`);
    console.log(`  PID:      ${d.pid}`);
    console.log(`  Uptime:   ${formatUptime(d.startedAt)}`);
    console.log(`  Jobs:     ${d.jobCount}`);
    console.log(`  Running:  ${d.activeRuns}`);
    console.log(`  Queued:   ${d.queueDepth}`);
    console.log(`  UI:       http://localhost:${port ?? DEFAULT_PORT}`);
    return null;
  }
  return d;
}

async function cmdPause({ port, json }) {
  const d = await apifetch(port, '/daemon/pause', 'POST');
  if (!json) { console.log('✓ Scheduler paused.'); return null; }
  return d;
}

async function cmdResume({ port, json }) {
  const d = await apifetch(port, '/daemon/resume', 'POST');
  if (!json) { console.log('✓ Scheduler resumed.'); return null; }
  return d;
}

async function cmdFlush({ port, json }) {
  const d = await apifetch(port, '/daemon/flush', 'POST');
  if (!json) { console.log('✓ Queue flushed.'); return null; }
  return d;
}

// ── Job management ────────────────────────────────────────────────────────────

async function cmdList({ port, json }) {
  const jobs = await apifetch(port, '/jobs');
  if (!json) {
    if (jobs.length === 0) { console.log('No jobs configured.'); return null; }
    printTable(
      ['ID', 'NAME', 'TRIGGER', 'SCHEDULE', 'EXECUTOR', 'ENABLED'],
      jobs.map(j => [
        j.id.slice(0, 8),
        j.name,
        j.trigger?.type  ?? '-',
        j.trigger?.schedule ?? '-',
        j.executor       || 'shell',
        j.enabled        ? 'yes' : 'no',
      ])
    );
    return null;
  }
  return jobs;
}

async function cmdGet({ port, json, id }) {
  if (!id) throw new Error('--id is required');
  const fullId = await resolveID(port, id);
  const job = await apifetch(port, `/jobs/${fullId}`);
  if (!json) { console.log(JSON.stringify(job, null, 2)); return null; }
  return job;
}

// Build the job payload from CLI options — handles all three executor types
function buildJobPayload(opts) {
  const job = {};
  if (opts.name)        job.name        = opts.name;
  if (opts.description) job.description = opts.description;
  if (opts.cwd)         job.cwd         = opts.cwd;
  if (opts.timeout)     job.timeout     = opts.timeout;
  if (opts.retries !== undefined) job.maxRetries = opts.retries;

  if (opts.trigger || opts.schedule) {
    job.trigger = {
      type:     opts.trigger  || 'once',
      schedule: opts.schedule || '',
    };
  }

  // Infer executor from flags if not explicit.
  // Both amplifier and claude-code take --prompt; use --executor to disambiguate.
  // --recipe always means amplifier (only amplifier supports recipes).
  // --prompt without --executor defaults to amplifier.
  const executor = opts.executor
    || (opts.command               ? 'shell'
      : opts.recipe || opts.prompt ? 'amplifier'
      :                              'shell');
  job.executor = executor;

  if (executor === 'shell') {
    job.shell = { command: opts.command || '' };
  } else if (executor === 'claude-code') {
    job.claudeCode = {
      prompt: opts.prompt || '',
      ...(opts.steps    && { steps:    opts.steps }),
      ...(opts.model    && { model:    opts.model }),
      ...(opts.maxTurns && { maxTurns: opts.maxTurns }),
    };
  } else if (executor === 'amplifier') {
    // Parse --context KEY=VAL pairs into an object
    const context = opts.context
      ? Object.fromEntries(opts.context.map(kv => {
          const eq = kv.indexOf('=');
          return eq === -1 ? [kv, ''] : [kv.slice(0, eq), kv.slice(eq + 1)];
        }))
      : undefined;
    job.amplifier = {
      ...(opts.prompt  && { prompt:      opts.prompt }),
      ...(opts.steps   && { steps:       opts.steps }),
      ...(opts.recipe  && { recipePath:  opts.recipe }),
      ...(opts.bundle  && { bundle:      opts.bundle }),
      ...(opts.model   && { model:       opts.model }),
      ...(context      && { context }),
    };
  }

  if (opts.trigger === 'watch' && opts.watchPath) {
    job.watch = {
      path:      opts.watchPath,
      recursive: opts.watchRecursive || false,
      ...(opts.watchEvents   && { events:   opts.watchEvents }),
      ...(opts.watchDebounce && { debounce: opts.watchDebounce }),
    };
  }

  return job;
}

async function cmdAdd(opts) {
  if (!opts.name) throw new Error('--name is required');
  const payload = buildJobPayload(opts);
  payload.enabled = true;
  const job = await apifetch(opts.port, '/jobs', 'POST', payload);
  if (!opts.json) { console.log(`✓ Job created: ${job.name} (id: ${job.id})`); return null; }
  return job;
}

async function cmdUpdate(opts) {
  if (!opts.id) throw new Error('--id is required');
  const fullId = await resolveID(opts.port, opts.id);
  const payload = buildJobPayload(opts);
  const job = await apifetch(opts.port, `/jobs/${fullId}`, 'PUT', payload);
  if (!opts.json) { console.log(`✓ Job updated: ${job.name} (id: ${job.id})`); return null; }
  return job;
}

async function cmdDelete({ port, json, id, yes }) {
  if (!id) throw new Error('--id is required');
  const fullId = await resolveID(port, id);
  if (!yes) {
    const jobs = await apifetch(port, '/jobs');
    const job  = jobs.find(j => j.id === fullId);
    const msg  = `Confirmation required to delete '${job?.name ?? fullId}'. Pass --yes to confirm.`;
    if (json) return { deleted: false, reason: msg };
    console.log(msg);
    return null;
  }
  await apifetch(port, `/jobs/${fullId}`, 'DELETE');
  if (!json) { console.log(`✓ Deleted job '${fullId}'`); return null; }
  return { deleted: true, id: fullId };
}

async function cmdTrigger({ port, json, id }) {
  if (!id) throw new Error('--id is required');
  const fullId = await resolveID(port, id);
  const d = await apifetch(port, `/jobs/${fullId}/trigger`, 'POST');
  if (!json) { console.log(`✓ Triggered job '${fullId}'`); return null; }
  return d;
}

async function cmdEnable({ port, json, id }) {
  if (!id) throw new Error('--id is required');
  const fullId = await resolveID(port, id);
  const job = await apifetch(port, `/jobs/${fullId}/enable`, 'POST');
  if (!json) { console.log(`✓ Enabled job '${job.name}'`); return null; }
  return job;
}

async function cmdDisable({ port, json, id }) {
  if (!id) throw new Error('--id is required');
  const fullId = await resolveID(port, id);
  const job = await apifetch(port, `/jobs/${fullId}/disable`, 'POST');
  if (!json) { console.log(`✓ Disabled job '${job.name}'`); return null; }
  return job;
}

async function cmdPrune({ port, json }) {
  const d = await apifetch(port, '/jobs/prune', 'POST');
  if (!json) { console.log(`✓ Pruned ${d.deleted} disabled job(s)`); return null; }
  return d;
}

// ── Runs ──────────────────────────────────────────────────────────────────────

async function cmdRuns({ port, json, limit }) {
  const qs   = limit ? `?limit=${limit}` : '';
  const runs = await apifetch(port, `/runs${qs}`);
  if (!json) {
    if (runs.length === 0) { console.log('No runs found.'); return null; }
    printTable(
      ['ID', 'JOB', 'STATUS', 'STARTED', 'ENDED'],
      runs.map(r => [
        r.id.slice(0, 8),
        r.jobName,
        r.status,
        fmtTime(r.startedAt),
        r.endedAt ? fmtTime(r.endedAt) : '-',
      ])
    );
    return null;
  }
  return runs;
}

async function cmdRun({ port, json, id }) {
  if (!id) throw new Error('--id is required');
  const run = await apifetch(port, `/runs/${id}`);
  if (!json) { console.log(JSON.stringify(run, null, 2)); return null; }
  return run;
}

async function cmdJobRuns({ port, json, id, limit }) {
  if (!id) throw new Error('--id is required');
  const fullId = await resolveID(port, id);
  const qs     = limit ? `?limit=${limit}` : '';
  const runs   = await apifetch(port, `/jobs/${fullId}/runs${qs}`);
  if (!json) {
    if (runs.length === 0) { console.log('No runs for this job.'); return null; }
    printTable(
      ['ID', 'STATUS', 'STARTED', 'ENDED', 'EXIT'],
      runs.map(r => [
        r.id.slice(0, 8),
        r.status,
        fmtTime(r.startedAt),
        r.endedAt ? fmtTime(r.endedAt) : '-',
        r.exitCode ?? '-',
      ])
    );
    return null;
  }
  return runs;
}

async function cmdClearRuns({ port, json }) {
  await apifetch(port, '/runs', 'DELETE');
  if (!json) { console.log('✓ Run history cleared.'); return null; }
  return { cleared: true };
}

// ── Formatting helpers ────────────────────────────────────────────────────────

function printTable(headers, rows) {
  const widths = headers.map((h, i) =>
    Math.max(h.length, ...rows.map(r => String(r[i] ?? '').length))
  );
  const line = (cells) => cells.map((c, i) => String(c ?? '').padEnd(widths[i])).join('  ');
  console.log(line(headers));
  console.log(widths.map(w => '-'.repeat(w)).join('  '));
  rows.forEach(r => console.log(line(r)));
}

function fmtTime(iso) {
  return new Date(iso).toLocaleString();
}

function formatUptime(startedAt) {
  const s = Math.floor((Date.now() - new Date(startedAt).getTime()) / 1000);
  const m = Math.floor(s / 60), h = Math.floor(m / 60);
  if (h > 0) return `${h}h${String(m % 60).padStart(2, '0')}m${String(s % 60).padStart(2, '0')}s`;
  if (m > 0) return `${m}m${String(s % 60).padStart(2, '0')}s`;
  return `${s}s`;
}

// ── Help ──────────────────────────────────────────────────────────────────────

const HELP = `
agent-daemon-cli — Headless agent-daemon CLI for AI agents

USAGE:
  node scripts/agent-daemon-cli.mjs <command> [options]

DAEMON STATUS & CONTROL:
  status                           Show daemon status
  pause                            Pause job scheduling
  resume                           Resume job scheduling
  flush                            Flush pending jobs from queue

JOB MANAGEMENT:
  list                             List all jobs
  get       --id ID                Get a job by ID or prefix
  add       --name NAME            Create a new job
            --trigger TYPE         cron | loop | once | watch  (default: once)
            --schedule EXPR        Cron expression or Go duration (e.g. "0 9 * * *" or "5m")
            --executor TYPE        shell | claude-code | amplifier  (default: shell)
            --command CMD          Shell command  (executor=shell)
            --prompt TEXT          Prompt text  (executor=claude-code or amplifier)
            --recipe PATH          Recipe .yaml path  (executor=amplifier)
            --bundle NAME          Bundle name  (executor=amplifier)
            --model MODEL          Model name  (executor=claude-code or amplifier)
            --cwd DIR              Working directory
            --timeout DUR          Max execution time e.g. 30s, 5m
            --retries N            Retries on failure (default: 0)
            --description TEXT     Job description
            --watch-path PATH      Path to watch  (trigger=watch)
            --watch-recursive      Watch subdirectories  (trigger=watch)
            --watch-events LIST    Comma-separated events: create,write,remove,rename,chmod
            --watch-debounce DUR   Quiet window before firing e.g. 500ms
  update    --id ID [same flags]   Update an existing job
  delete    --id ID --yes          Delete a job (--yes bypasses confirmation)
  trigger   --id ID                Trigger a job immediately
  enable    --id ID                Enable a disabled job
  disable   --id ID                Disable a job
  prune                            Delete all disabled jobs

RUNS:
  runs      [--limit N]            List recent runs (default: 50)
  run       --id ID                Get a specific run with full output
  job-runs  --id ID [--limit N]    List runs for a specific job
  clear-runs                       Clear all run history

SERVICE LIFECYCLE (shells out to agent-daemon binary):
  install                          Install as system service (launchd/systemd/SCM)
  uninstall                        Uninstall system service
  start                            Start the daemon service
  stop                             Stop the daemon service

OPTIONS:
  --json         Machine-readable JSON output
  --port N       Daemon port (default: 7700)
  --yes, -y      Skip confirmation prompts
  --help, -h     Show this help
`.trim();

// ── Entry point ───────────────────────────────────────────────────────────────

async function main() {
  const opts = parseArgs(process.argv);
  const cmd  = opts._[0];

  if (opts.help || !cmd) {
    console.log(HELP);
    process.exit(cmd ? 0 : 1);
  }

  try {
    // Service lifecycle — shell out to the agent-daemon binary
    if (SERVICE_COMMANDS.has(cmd)) {
      const result = runServiceCmd(cmd);
      if (result.output) console.log(result.output);
      if (opts.json) console.log(JSON.stringify(result, null, 2));
      return;
    }

    // HTTP API commands
    let result;
    switch (cmd) {
      case 'status':      result = await cmdStatus(opts);    break;
      case 'pause':       result = await cmdPause(opts);     break;
      case 'resume':      result = await cmdResume(opts);    break;
      case 'flush':       result = await cmdFlush(opts);     break;
      case 'list':        result = await cmdList(opts);      break;
      case 'get':         result = await cmdGet(opts);       break;
      case 'add':         result = await cmdAdd(opts);       break;
      case 'update':      result = await cmdUpdate(opts);    break;
      case 'delete':      result = await cmdDelete(opts);    break;
      case 'trigger':     result = await cmdTrigger(opts);   break;
      case 'enable':      result = await cmdEnable(opts);    break;
      case 'disable':     result = await cmdDisable(opts);   break;
      case 'prune':       result = await cmdPrune(opts);     break;
      case 'runs':        result = await cmdRuns(opts);      break;
      case 'run':         result = await cmdRun(opts);       break;
      case 'job-runs':    result = await cmdJobRuns(opts);   break;
      case 'clear-runs':  result = await cmdClearRuns(opts); break;
      default:
        console.error(`Unknown command: ${cmd}\nRun with --help for usage.`);
        process.exit(1);
    }

    if (result !== null && result !== undefined && opts.json) {
      console.log(JSON.stringify(result, null, 2));
    }
  } catch (err) {
    if (opts.json) {
      console.log(JSON.stringify({ error: err.message }));
    } else {
      console.error(`Error: ${err.message}`);
    }
    process.exit(1);
  }
}

if (realpathSync(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
