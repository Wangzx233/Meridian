# Meridian Architecture

## 1. Architecture Overview

The system uses a control-plane and runner architecture.

```text
Browser Web UI
  |
  | HTTP + WebSocket/SSE
  v
Control Plane API
  |
  | PostgreSQL
  v
Database

Control Plane API
  |
  | persistent runner connection
  v
Runner Agent on Server
  |
  | local process execution
  v
Codex CLI in Project Directory
```

The browser and control plane manage tasks, context, and history. The runner
executes Codex CLI on the server where the project actually lives.

## 2. Components

### Web UI

Responsibilities:

- Show servers, projects, tasks, context items, and runs.
- Provide the task session page.
- Let the user manually select context for each turn.
- Stream Codex output.
- Let the user cancel a running turn.
- Let the user mark tasks done.

Suggested stack:

- React.
- TypeScript.
- Vite.
- TanStack Query.
- Monaco or another diff viewer later if needed.

### Control Plane API

Responsibilities:

- Authentication.
- Project and task CRUD.
- Context item CRUD.
- Run creation and state transitions.
- Prompt construction.
- Runner registration and heartbeat.
- Event ingestion from runners.
- Streaming events to browser clients.
- Persistence to PostgreSQL.

Suggested stack:

- Go.
- PostgreSQL.
- WebSocket or SSE for browser event streaming.
- WebSocket or long-poll/gRPC stream for runner communication.

### Runner Agent

Responsibilities:

- Connect to control plane.
- Register identity and capabilities.
- Keep heartbeat alive.
- Receive run requests.
- Invoke Codex CLI.
- Stream Codex JSONL/stdout/stderr events back.
- Support cancellation.
- Report final status.
- Expose a run-scoped local send-back helper only when the control plane
  explicitly enables it for that run.
- Self-update from deployed runner artifacts when the registered capability is
  available.

The runner should be thin. It should not call OpenAI APIs directly in the first
version.

Suggested stack:

- Go.
- Installed as a systemd service on Linux servers.
- Uses local `codex` executable from PATH or configured absolute path.

## 3. Execution Flow

### First Turn of a Task

1. User opens a task with no Codex session.
2. User writes an instruction and selects optional context.
3. Control plane builds a new-session prompt.
4. Control plane creates a run with `mode = new`.
5. Runner receives the run.
6. Runner executes:

```bash
codex --cd <project_workdir> exec --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --json -
```

7. Runner writes the prompt to stdin.
8. Runner streams JSONL output to the control plane.
9. Control plane stores events.
10. Browser receives events.
11. On completion, control plane stores final message and Codex session id.
12. Task moves to `waiting_user`.

### Later Turn of a Task

1. User writes another instruction.
2. User optionally selects context for this turn.
3. Control plane builds a resume prompt.
4. Control plane creates a run with `mode = resume`.
5. Runner executes:

```bash
codex --cd <project_workdir> exec resume --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --json <session_id> -
```

6. Runner writes the prompt to stdin.
7. Events stream back and are stored.
8. Task returns to `waiting_user` after the run ends.

## 4. Data Model

### servers

```text
id
name
alias
runner_id
status
last_heartbeat_at
created_at
updated_at
```

`name` is the registered or runner-reported host name. `alias` is an optional
human display name used by the console, runner update summaries, and workbench
notifications; runner heartbeats do not overwrite it.

### projects

```text
id
name
server_id
workdir
default_branch
rules_path
created_at
updated_at
```

### tasks

```text
id
project_id
title
description
status
codex_session_id
created_at
updated_at
completed_at
archived_at
```

### runs

```text
id
task_id
mode                  -- new | resume
status                -- queued | running | succeeded | failed | canceled
user_message
generated_prompt
final_message
codex_session_id
assigned_runner_id
exit_code
error_message
cancel_requested_at
runner_started_at
started_at
ended_at
created_at
```

### context_items

```text
id
server_id             -- nullable; set when scope = server/project/task
project_id            -- nullable; set when scope = project/task
task_id               -- nullable; set when scope = task
scope                 -- global | server | project | task
type                  -- project_rule | task_summary | decision | log_snippet | verify_command | file_hint | note
title
content
tags
created_at
updated_at
```

Context items are reusable at the selected owner level. The manual context
picker for a task shows global items, items for that task's server, items for
that project, and items bound to the task itself.

### run_context_items

```text
run_id
context_item_id
order_index
type_snapshot
title_snapshot
content_snapshot
```

The snapshots preserve the exact context shown to Codex for historical runs,
even if the source context item is edited later.

### run_events

```text
id
run_id
seq
event_type
stream                -- jsonl | stdout | stderr | system
payload
occurred_at
created_at
```

### task_memories

```text
id
task_id
project_id
problem
root_cause
changes
files
decisions
verification
related_tasks
source_commit
stale_conditions
created_at
updated_at
```

## 5. State Machines

### Task State

```text
open -> running -> waiting_user -> running
waiting_user -> done
done -> archived
open -> archived
waiting_user -> archived
```

Rules:

- A task can only have one active run at a time.
- Active runs are runs with status `queued` or `running`.
- Creating a queued run should move the task to `running` in the same database
  transaction.
- The database should enforce this with a constraint so repeated clicks,
  retries, or runner reconnects cannot create two active runs for one task.
- A task remains `running` while the active run is queued or running.
- A task becomes `waiting_user` when a run succeeds, fails, or is canceled,
  unless the task is already done or archived.
- Only explicit user action marks a task `done`.

### Run State

```text
queued -> running -> succeeded
queued -> running -> failed
queued -> running -> canceled
queued -> canceled
```

## 6. Prompt Construction

Prompt construction belongs in the control plane, not the runner.

The runner receives the final prompt as opaque text and sends it to Codex.

### New Session Prompt

```text
Current task:
<title>

Description:
<description>

User instruction for this turn:
<user_message>

Selected context:
<context items>

Instructions:
- First inspect the current repository before deciding.
- Historical context is background only.
- Current repository code is authoritative if it conflicts with context.
- Complete this turn and explain changes, verification, and next steps.
```

### Resume Prompt

```text
Continue the current task.

User instruction for this turn:
<user_message>

Additional context selected for this turn:
<context items>

Instructions:
- Continue from the existing Codex session.
- Do not repeat already completed work unless needed.
- Current repository code is authoritative.
```

## 7. Runner Protocol

The concrete first-version HTTP, browser stream, and runner WebSocket contract
is defined in `docs/api-contract.md`.

The protocol should support:

- Runner registration.
- Heartbeat.
- Run assignment.
- Event streaming from runner to control plane.
- Cancellation request from control plane to runner.
- Shutdown request from control plane to runner when a server is deleted.
- Final status report.

For MVP, a simple WebSocket connection is acceptable.

Example message categories:

```text
runner.register
runner.heartbeat
run.assign
run.event
run.completed
run.cancel
runner.shutdown
```

## 8. Codex CLI Integration

The runner should use the installed Codex CLI.

Requirements:

- Configurable path to `codex`.
- Configurable environment variables.
- Working directory must be the project's configured `workdir`.
- Use `--json` so events can be parsed and stored.
- Use stdin for prompts.
- By default, pass `--dangerously-bypass-approvals-and-sandbox` so
  non-interactive server tasks do not stall on Codex approval prompts or
  sandbox path limits. Operators can set
  `CODEX_BYPASS_APPROVALS_AND_SANDBOX=false` to preserve Codex's sandbox and
  approval behavior.
- When a run has send-back notices enabled, prepend a runner-managed
  `send-back` helper to that Codex process `PATH` and pass a
  runner-local callback URL/token through environment variables. The callback
  must bind to `127.0.0.1`, and the callback URL/token must not be included in
  prompts or persisted as context.

Commands:

```bash
codex --cd <project_workdir> exec --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --json -
codex --cd <project_workdir> exec resume --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --json <session_id> -
```

Avoid `--ephemeral` for normal task runs because sessions need to be resumed.

### Session ID Capture

For the first version, session id handling should stay simple:

- Parse the Codex JSONL stream and store the session id on the run when it is
  observed.
- After a successful first run, copy the observed session id to the task.
- Resume runs should use the task's stored session id.
- If a task has no stored session id, the control plane should not create an
  implicit resume run. The UI should require the user to start a new session for
  that task.

## 9. Security Notes

Initial security should be simple but explicit:

- Require login for browser and HTTP API access once authentication is
  configured.
- Allow first-run browser setup only when no account exists. After the first
  account is created, keep accounts manually managed and do not add self-service
  registration or per-user authorization in the first version.
- Protect runner endpoints with a separate bearer token so installed runners can
  connect without browser cookies.
- Do not store plaintext secrets as context.
- Keep runner authentication tokens scoped per runner.
- Show the target server and workdir before sending a turn.
- Keep the workbench behind login and trusted runner tokens when Codex sandbox
  and approval controls are bypassed.

## 10. Future Extensions

Possible later additions:

- Context recommendation.
- Task relationship graph.
- Automatic task memory drafting by Codex.
- Diff viewer.
- Web terminal.
- Approval workflow for risky commands.
- Runner capability policies.
- Integration with Hermes as an optional executor.
- Team permissions.

These are intentionally outside the first version.
