# Meridian API Contract

This document defines the first-version interface contract between the web UI,
control plane, runner, and browser event stream.

The contract is intentionally small. It should let frontend and backend work in
parallel without turning the product into a general chat app, IDE, or custom
agent framework.

## 1. Contract Decisions

- HTTP APIs use JSON over `/api/v1`.
- Resource ids are strings, preferably UUIDs.
- Timestamps use RFC 3339 UTC strings.
- Browser streaming uses Server-Sent Events (SSE).
- Runner communication uses a persistent WebSocket.
- Prompt construction happens in the control plane.
- The runner receives an opaque prompt and executes the real Codex CLI.
- The runner must invoke Codex with argv-style process execution, not through a
  shell command string.
- Complex security and permission policy are outside this first contract.

## 2. Common JSON Shapes

### Error Response

```json
{
  "error": {
    "code": "active_run_exists",
    "message": "Task already has an active run.",
    "details": {
      "task_id": "tsk_123",
      "run_id": "run_123"
    }
  }
}
```

Common error codes:

- `validation_error`
- `not_found`
- `invalid_state`
- `active_run_exists`
- `unauthorized`
- `missing_codex_session`
- `runner_unavailable`
- `runner_unsupported`
- `runner_request_timeout`
- `internal_error`

### Build Info

```text
GET /api/v1/build
```

Returns the backend build commit and is public so deployment automation can
verify that the public control plane is serving the commit that triggered the
deployment.

```json
{
  "commit": "93b7b46..."
}
```

### List Response

```json
{
  "items": [],
  "next_cursor": null
}
```

The MVP may return all items and keep `next_cursor = null`. Cursor pagination
can be added later without changing the top-level shape.

## 3. Resource Shapes

### Server

```json
{
  "id": "srv_123",
  "name": "desktop",
  "runner_id": "runner_desktop",
  "status": "online",
  "runner_connected": true,
  "runner_connection": {
    "hostname": "desktop",
    "version": "0.5.0",
    "codex_path": "codex",
    "connected_at": "2026-05-12T06:00:00Z"
  },
  "runner_capabilities": {
    "codex_exec": true,
    "cancel": true,
    "fs_list": true,
    "project_files": true,
    "project_file_io": true,
    "project_file_upload": true,
    "project_terminal": true,
    "project_command": true,
    "shutdown": true
  },
  "last_heartbeat_at": "2026-05-11T08:00:00Z",
  "created_at": "2026-05-11T08:00:00Z",
  "updated_at": "2026-05-11T08:00:00Z"
}
```

Server status values:

- `online`
- `offline`

`status` is the persisted heartbeat state. The control plane may keep it online
for a short grace period after a websocket disconnect so runner upgrades and
backend deploys do not immediately mark the server offline.
`runner_connected` reflects whether the current backend process has an active
websocket for `runner_id`. `runner_connection` describes that active process and
is omitted when no runner is connected. File browsing, file editing, and PTY
terminal sessions also require the corresponding values in
`runner_capabilities`; a connected runner without `project_file_io`,
`project_file_upload`, or `project_terminal` is an old runner binary.

### Project

```json
{
  "id": "prj_123",
  "server_id": "srv_123",
  "name": "codex-task-workbench",
  "workdir": "D:\\go\\workplace",
  "default_branch": "main",
  "rules_path": "AGENTS.md",
  "created_at": "2026-05-11T08:00:00Z",
  "updated_at": "2026-05-11T08:00:00Z"
}
```

### Task

```json
{
  "id": "tsk_123",
  "project_id": "prj_123",
  "title": "Design API contract",
  "description": "Define API shapes, state semantics, and stream format.",
  "status": "waiting_user",
  "codex_session_id": "codex-session-id",
  "active_run_id": null,
  "created_at": "2026-05-11T08:00:00Z",
  "updated_at": "2026-05-11T08:00:00Z",
  "completed_at": null,
  "archived_at": null
}
```

Task status values:

- `open`
- `running`
- `waiting_user`
- `done`
- `archived`

### Run

```json
{
  "id": "run_123",
  "task_id": "tsk_123",
  "mode": "resume",
  "status": "succeeded",
  "user_message": "Implement the next change.",
  "generated_prompt": "Continue the current task...",
  "codex_model": "gpt-5.5",
  "codex_reasoning_effort": "high",
  "codex_service_tier": "fast",
  "raw_command": false,
  "final_message": "Implemented the change and ran tests.",
  "codex_session_id": "codex-session-id",
  "assigned_runner_id": "runner_desktop",
  "exit_code": 0,
  "error_message": null,
  "cancel_requested_at": null,
  "runner_started_at": "2026-05-11T08:01:00Z",
  "started_at": "2026-05-11T08:00:58Z",
  "ended_at": "2026-05-11T08:04:00Z",
  "created_at": "2026-05-11T08:00:58Z"
}
```

Run mode values:

- `new`
- `resume`

Run status values:

- `queued`
- `running`
- `succeeded`
- `failed`
- `canceled`

`codex_model`, `codex_reasoning_effort`, and `codex_service_tier` are optional
per-run Codex CLI overrides. When omitted or `null`, the runner uses its local
Codex config. Supported reasoning effort values are `low`, `medium`, `high`,
and `xhigh`. Supported service tier override values currently include `fast`.
`raw_command` is reserved for explicit Codex slash-command turns such as
`/compact`, where the stored prompt is sent to Codex without workbench wrapping.

### Context Item

```json
{
  "id": "ctx_123",
  "server_id": "srv_123",
  "project_id": "prj_123",
  "task_id": "tsk_123",
  "scope": "task",
  "type": "decision",
  "title": "Use SSE for browser stream",
  "content": "Browser stream is one-way, so SSE is enough for the MVP.",
  "tags": ["api", "stream"],
  "created_at": "2026-05-11T08:00:00Z",
  "updated_at": "2026-05-11T08:00:00Z"
}
```

Context scope values:

- `global`
- `server`
- `project`
- `task`

Context type values:

- `project_rule`
- `task_summary`
- `decision`
- `log_snippet`
- `verify_command`
- `file_hint`
- `note`

Rules:

- A `project` scoped item must have `task_id = null`.
- A `task` scoped item must have `task_id` set.
- Run creation stores snapshots of selected context item title, type, content,
  and order.

### Run Event

```json
{
  "id": "evt_123",
  "run_id": "run_123",
  "seq": 12,
  "event_type": "codex.event",
  "stream": "jsonl",
  "payload": {
    "raw": {
      "type": "message"
    },
    "text": "Optional display text extracted by the control plane.",
    "session_id": "codex-session-id"
  },
  "occurred_at": "2026-05-11T08:02:00Z",
  "created_at": "2026-05-11T08:02:00Z"
}
```

`seq` is monotonic per run and assigned by the control plane when the event is
stored. Browser clients use it as the replay cursor.

Event type values:

- `run.state`
- `codex.event`
- `process.output`
- `runner.error`
- `run.final`

Stream values:

- `jsonl`
- `stdout`
- `stderr`
- `system`

## 4. HTTP API

### Authentication

Browser and HTTP API access requires a signed HttpOnly session cookie once
authentication has been configured. Accounts can be preconfigured as
comma-separated `WORKBENCH_AUTH_USERS` entries, or the first account can be
created through `/auth/setup` when no account exists yet. Setup creates one
account, stores its password hash in PostgreSQL, generates session and runner
tokens, and then closes setup mode. The first version does not support
self-service registration or per-user authorization after setup.

```text
GET  /api/v1/auth/session
POST /api/v1/auth/login
POST /api/v1/auth/setup
POST /api/v1/auth/logout
```

Login request:

```json
{
  "username": "admin",
  "password": "change-me"
}
```

Session response:

```json
{
  "authenticated": true,
  "username": "admin",
  "runner_token": "token-for-runner-install-commands"
}
```

Initial setup session response:

```json
{
  "authenticated": false,
  "setup_required": true
}
```

Setup request:

```json
{
  "username": "admin",
  "password": "change-me-longer"
}
```

Runner endpoints use the separate `WORKBENCH_RUNNER_TOKEN` bearer token instead
of browser cookies when the token is preconfigured. In browser setup mode, the
generated runner token stored in PostgreSQL is used instead.

### Servers

```text
GET  /api/v1/servers
POST /api/v1/servers
GET  /api/v1/servers/{server_id}
PATCH /api/v1/servers/{server_id}
GET  /api/v1/servers/{server_id}/directories?path={absolute_path}
POST /api/v1/runners/update-all
```

Create server request:

```json
{
  "name": "desktop",
  "alias": "Oracle backup",
  "runner_id": "runner_desktop"
}
```

Server response:

```json
{
  "id": "srv_123",
  "name": "desktop",
  "alias": "Oracle backup",
  "runner_id": "runner_desktop",
  "status": "online",
  "runner_connected": true,
  "last_heartbeat_at": "2026-05-11T08:00:00Z",
  "created_at": "2026-05-11T07:00:00Z",
  "updated_at": "2026-05-11T08:00:00Z"
}
```

Patch server request:

```json
{
  "alias": "Oracle backup"
}
```

Directory listing response:

```json
{
  "path": "D:\\go",
  "parent": "D:\\",
  "entries": [
    {
      "name": "workplace",
      "path": "D:\\go\\workplace",
      "is_dir": true,
      "markers": [".git", "AGENTS.md", "go.mod"]
    }
  ]
}
```

Rules:

- `name` is the runner-reported or registered host name. `alias` is an optional
  human display name; clients should display `alias` when present and fall back
  to `name`.
- Send `alias` as `null` or an empty string to clear it. Runner heartbeats never
  overwrite `alias`.
- The endpoint requires a currently connected runner for the server's
  `runner_id`.
- The runner must report the `fs_list` capability. Older runners can still show
  a recent heartbeat but return `runner_unsupported` for this endpoint.
- If the connected runner does not respond before the request timeout, the API
  returns `runner_request_timeout`.
- It returns directories only; it does not read file contents.
- Empty `path` asks the runner for useful roots such as home, current runner
  directory, drives on Windows, or `/` on Linux.
- Deleting a server first asks the connected runner to shut down when it reports
  the `shutdown` capability. The server record is still deleted when the runner
  is offline, too old to support shutdown, or does not acknowledge the request.

Runner update response:

```json
{
  "requested_at": "2026-05-11T08:04:00Z",
  "accepted": 1,
  "skipped": 1,
  "failed": 0,
  "results": [
    {
      "server_id": "srv_123",
      "server_name": "Oracle backup",
      "runner_id": "runner_desktop",
      "previous_version": "0.4.0",
      "status": "accepted",
      "message": "Runner update started. The websocket may disconnect while the runner restarts."
    }
  ]
}
```

Runner update rules:

- Only currently connected runners can be updated.
- The runner must report `self_update`; older connected runners are skipped and
  need one manual reinstall.
- The update reuses the normal install endpoints and preserves `runner_id`.
  Windows uses `install.ps1`, while Linux and macOS use `install.sh`.
- The runner token is used as a local updater secret and sent to install
  endpoints with the `Authorization` header; clients and runners must not put
  it in installer URLs.
- `accepted` means the runner started its local updater process, not that the
  new runner has already reconnected.

Runner install endpoints:

- `GET /api/v1/runner/install.ps1` installs the Windows amd64 runner. User mode
  is the default; `run_as=system` installs a Scheduled Task.
- `GET /api/v1/runner/install.sh` installs Linux amd64/arm64 runners as a
  systemd service when systemd is available. Linux defaults to `run_as=user`,
  setting the service `User`, `HOME`, `USER`, and `LOGNAME` to the invoking
  user, or to the owner of an absolute `codex_path` when reinstalling from an
  existing root-runner. This keeps Codex config lookup aligned with the user's
  `~/.codex`. Passing `run_as=system` preserves the previous root service
  behavior. In containers or other non-systemd Linux environments, it falls back
  to a standalone `nohup` background process under
  `/opt/codex-task-workbench/runner`, started as the same resolved user when
  `run_as=user`. macOS amd64/arm64 runners install as the launchd daemon
  `com.codex-task-workbench.runner`.
- If `runner_id` is omitted, the installer derives one from the target machine.
  Passing `runner_id` is only for reinstalling the same server identity.

### Projects

```text
GET  /api/v1/projects?server_id={server_id}
POST /api/v1/projects
GET  /api/v1/projects/{project_id}
PATCH /api/v1/projects/{project_id}
DELETE /api/v1/projects/{project_id}
GET  /api/v1/projects/{project_id}/files?path={relative_project_path}
GET  /api/v1/projects/{project_id}/files/content?path={relative_project_path}
PUT  /api/v1/projects/{project_id}/files/content
POST /api/v1/projects/{project_id}/files/upload
POST /api/v1/projects/{project_id}/files/actions
WS   /api/v1/projects/{project_id}/terminal
POST /api/v1/projects/{project_id}/command
```

Create project request:

```json
{
  "server_id": "srv_123",
  "name": "codex-task-workbench",
  "workdir": "D:\\go\\workplace",
  "default_branch": "main",
  "rules_path": "AGENTS.md"
}
```

Project file listing response:

```json
{
  "root": "D:\\go\\workplace",
  "path": "frontend/src",
  "parent": "frontend",
  "entries": [
    {
      "name": "App.tsx",
      "path": "frontend/src/App.tsx",
      "is_dir": false,
      "size": 42000,
      "modified_at": "2026-05-11T08:00:00Z"
    }
  ]
}
```

Rules:

- Deleting a project removes its tasks, runs, run events, selected context
  snapshots, project/task context items, memories, and workbench notifications.
  It does not delete files from the project's `workdir`.
- The endpoint requires a currently connected runner for the project's server.
- The runner must report the `project_files` capability.
- `path` is project-root relative. Empty `path` lists the project root.
- The runner rejects paths that resolve outside `project.workdir`.
- This endpoint lists files and directories, but not file contents.

Project file content response:

```json
{
  "root": "D:\\go\\workplace",
  "path": "frontend/src/App.tsx",
  "name": "App.tsx",
  "size": 42000,
  "modified_at": "2026-05-11T08:00:00Z",
  "content": "import React from \"react\";\n",
  "encoding": "utf-8"
}
```

Project file write request:

```json
{
  "path": "frontend/src/App.tsx",
  "content": "import React from \"react\";\n",
  "create_dirs": true
}
```

Project file upload request:

```http
POST /api/v1/projects/proj_123/files/upload
Content-Type: multipart/form-data

path=docs&create_dirs=true&file=@diagram.png
```

Project file action request:

```json
{
  "action": "rename",
  "path": "notes/todo.md",
  "target_path": "notes/done.md"
}
```

Rules:

- File content endpoints require `project_file_io`.
- File upload requires `project_file_upload`, writes the selected browser file
  into `path` under the project workdir, preserves binary bytes, and currently
  rejects uploads above 10 MiB.
- `action` may be `create`, `rename`, or `delete`; `create` also accepts
  `is_dir`.
- The runner resolves all paths inside `project.workdir` and rejects paths that
  escape it.
- Files above the runner read-size limit are rejected instead of streamed into
  the editor.

Project terminal browser messages:

```json
{ "type": "open", "payload": { "cols": 100, "rows": 30 } }
{ "type": "input", "payload": { "data": "git status\r" } }
{ "type": "resize", "payload": { "cols": 120, "rows": 34 } }
{ "type": "close" }
```

Project terminal server messages:

```json
{ "type": "ready", "terminal_id": "term_123", "workdir": "D:\\go\\workplace" }
{ "type": "output", "data": "On branch main\r\n" }
{ "type": "exit", "exit_code": 0, "error": null }
```

Rules:

- The terminal endpoint upgrades to a browser WebSocket.
- The backend proxies PTY input/output through the connected runner; browsers do
  not connect directly to runner machines.
- The runner starts the platform shell in the configured project `workdir`.
- The endpoint requires the `project_terminal` runner capability.

Project command request:

```json
{
  "command": "git status --short",
  "timeout_secs": 60
}
```

Project command response:

```json
{
  "command": "git status --short",
  "workdir": "D:\\go\\workplace",
  "exit_code": 0,
  "stdout": "",
  "stderr": "",
  "duration_ms": 120,
  "error": null
}
```

Rules:

- The endpoint requires a currently connected runner for the project's server.
- The runner must report the `project_command` capability.
- Commands are launched with the configured project `workdir` as the process
  working directory.
- Command output may be truncated by the runner.
- This is a workbench helper for trusted project servers, not a sandbox or a
  general remote administration API.

### Tasks

```text
GET  /api/v1/projects/{project_id}/tasks?status=open,waiting_user,running
POST /api/v1/projects/{project_id}/tasks
GET  /api/v1/tasks/{task_id}
PATCH /api/v1/tasks/{task_id}
POST /api/v1/tasks/{task_id}/mark-done
POST /api/v1/tasks/{task_id}/archive
```

Create task request:

```json
{
  "title": "Design API contract",
  "description": "Define API shapes, state semantics, and stream format."
}
```

Mark done request:

```json
{
  "summary": "Backward-compatible optional user-written task summary.",
  "memory": {
    "problem": "Optional short memory note or task objective.",
    "changes": "Optional concrete changes made.",
    "verification": "Optional verification performed and results.",
    "files": "Optional important files touched or inspected.",
    "stale_conditions": "Optional risks, caveats, or follow-up conditions."
  }
}
```

Rules:

- A task can be marked done only from `open` or `waiting_user`.
- A `running` task must be allowed to finish or be canceled before it is marked
  done or archived.
- `memory` fields are optional. Empty fields are allowed and do not need to be
  forced by the client.
- `summary` remains accepted for older clients and is stored as the memory
  problem when no structured problem is provided.
- A memory draft button, when present, must create a normal visible Codex run and
  only prefill the user's editable summary form. The draft is not saved until
  the user explicitly saves it to context or marks the task done.
- Saving a summary to context uses the context item API with type
  `task_summary`; marking a task done remains a separate explicit action.
- Marking a task done does not create a pending in-app notification. Email
  delivery, when configured, remains best-effort.
- Each terminal Codex run also creates a pending in-app notification so the user
  can confirm the finished turn even when the task remains open.

### Workbench Notifications

```text
GET  /api/v1/notifications?pending=true
POST /api/v1/notifications/{notification_id}/ack
```

Notification response:

```json
{
  "id": "ntf_123",
  "type": "run_finished",
  "server_id": "srv_123",
  "server_name": "Oracle backup",
  "project_id": "prj_123",
  "project_name": "codex-task-workbench",
  "task_id": "tsk_123",
  "task_title": "Design API contract",
  "run_id": "run_123",
  "run_status": "succeeded",
  "title": "Run succeeded: Design API contract",
  "message": "codex-task-workbench / Design API contract",
  "acknowledged_at": null,
  "created_at": "2026-05-11T08:00:00Z"
}
```

Rules:

- `pending=true` is the default and returns unacknowledged notifications.
- Notification type values are `task_done` and `run_finished`. `run_finished`
  notifications include `run_id` and terminal `run_status`.
- `server_name` is the server alias when present, otherwise the registered
  server name.
- Pending notification queries exclude `task_done`; that type is retained only
  for historical records.
- Opening or dismissing a notification should call the acknowledge endpoint.
- Acknowledged notifications remain stored but are hidden from the normal
  pending notice tray.

### Context Items

```text
GET    /api/v1/projects/{project_id}/context-items?task_id={task_id}
POST   /api/v1/projects/{project_id}/context-items
GET    /api/v1/context-items/{context_item_id}
PATCH  /api/v1/context-items/{context_item_id}
DELETE /api/v1/context-items/{context_item_id}
```

The list endpoint returns context items eligible for a task picker:

- all global items;
- server-scoped items for the project's server;
- project-scoped items for the project;
- task-scoped items for `task_id`, when `task_id` is provided.

Create context item request:

```json
{
  "server_id": "srv_123",
  "project_id": "prj_123",
  "scope": "task",
  "task_id": "tsk_123",
  "type": "decision",
  "title": "Use SSE for browser stream",
  "content": "Browser stream is one-way, so SSE is enough for the MVP.",
  "tags": ["api", "stream"]
}
```

### Runs

```text
GET  /api/v1/tasks/{task_id}/runs
POST /api/v1/tasks/{task_id}/runs
GET  /api/v1/runs/{run_id}
GET  /api/v1/runs/{run_id}/events?after_seq={seq}
GET  /api/v1/runs/{run_id}/events/stream?after_seq={seq}
POST /api/v1/runs/{run_id}/cancel
```

Create run request:

```json
{
  "message": "Implement the documented API contract.",
  "mode": "auto",
  "codex_model": "gpt-5.5",
  "codex_reasoning_effort": "high",
  "codex_service_tier": "fast",
  "raw_command": false,
  "context_item_ids": ["ctx_123", "ctx_456"]
}
```

Accepted create-run `mode` values:

- `auto`
- `new`
- `resume`

Create-run response:

```json
{
  "run": {
    "id": "run_123",
    "task_id": "tsk_123",
    "mode": "resume",
    "status": "queued"
  },
  "task": {
    "id": "tsk_123",
    "status": "running",
    "active_run_id": "run_123"
  }
}
```

Create-run rules:

- `auto` resolves to `resume` when the task has `codex_session_id`; otherwise
  it resolves to `new`.
- `resume` requires `task.codex_session_id`.
- `new` starts a new Codex session for the task.
- Creating a run stores selected context snapshots immediately.
- Creating a run builds and stores `generated_prompt` immediately.
- Creating a run inserts a `queued` run and moves the task to `running` in the
  same database transaction.
- The database must enforce at most one active run per task, where active means
  `queued` or `running`.
- If an active run already exists, return `409 active_run_exists`.
- If a `resume` run is requested without a task session id, return
  `409 missing_codex_session`.
- A run cannot be created for `done` or `archived` tasks.

The `Idempotency-Key` header is optional but recommended for create-run
requests. When present, retrying the same request should return the originally
created run instead of creating a second run.

Cancel request:

```json
{
  "reason": "User canceled from task page."
}
```

Cancel rules:

- Cancel is valid only for `queued` or `running` runs.
- Canceling a `queued` run may mark it `canceled` immediately.
- Canceling a `running` run sends `run.cancel` to the runner.
- A terminal run cannot be canceled again.

## 5. State Semantics

### Task State Machine

```text
open -> running
waiting_user -> running
running -> waiting_user
open -> done
waiting_user -> done
done -> archived
open -> archived
waiting_user -> archived
```

Rules:

- Only run creation moves a task to `running`.
- A task remains `running` while its active run is `queued` or `running`.
- A terminal run moves the task to `waiting_user`, unless the task is already in
  a terminal user state.
- A successful run does not mark the task done.
- Only explicit user action marks a task done.
- `done` and `archived` tasks cannot receive new runs.
- `running` tasks cannot be marked done or archived until the active run ends or
  is canceled.

### Run State Machine

```text
queued -> running
queued -> canceled
running -> succeeded
running -> failed
running -> canceled
```

Rules:

- `queued` means the control plane accepted the turn and is waiting for a
  runner.
- `running` means the runner has started executing the Codex process.
- `succeeded` means Codex exited with code `0`.
- `failed` means Codex exited non-zero or the runner could not complete the
  command.
- `canceled` means the user requested cancellation before the run completed.
- Terminal statuses are immutable.
- `started_at` is set when the run is accepted by the control plane.
- `runner_started_at` is set when the runner reports `run.started`.
- `ended_at` is set for `succeeded`, `failed`, or `canceled`.

### Codex Session Semantics

- First task run normally uses `mode = new`.
- Later task runs normally use `mode = resume`.
- `resume` runs use `task.codex_session_id`.
- The control plane stores the observed Codex session id on each run.
- When a Codex event exposes a session id, the control plane stores it on the
  run and fills an empty `task.codex_session_id` immediately.
- If the user intentionally starts a new session on an existing task, a
  successful `new` run replaces `task.codex_session_id` with the new observed
  session id.
- When creating a run for a task with an empty stored session, the control plane
  also recovers the latest observed session id from that task's historical runs
  before resolving `auto` or `resume`.
- Failed or canceled `new` runs do not replace a non-empty
  `task.codex_session_id`.
- Unknown Codex JSON event fields must be preserved in `payload.raw`.

## 6. Browser SSE Stream

The browser subscribes to one run at a time:

```text
GET /api/v1/runs/{run_id}/events/stream?after_seq=0
Accept: text/event-stream
```

Reconnect rules:

- The client may pass `after_seq`.
- The client may also send the standard `Last-Event-ID` header.
- The server replays stored events with `seq > after_seq`, then keeps streaming
  new events.
- The server should send heartbeat comments while the run is active:

```text
: ping
```

SSE event format:

```text
id: 12
event: codex.event
data: {"run_id":"run_123","task_id":"tsk_123","seq":12,"event_type":"codex.event","stream":"jsonl","payload":{"raw":{"type":"message"},"text":"Hello"},"occurred_at":"2026-05-11T08:02:00Z"}
```

Event names are the same as `event_type`:

- `run.state`
- `codex.event`
- `process.output`
- `runner.error`
- `run.final`

### `run.state`

Emitted when the run status changes.

```json
{
  "run_id": "run_123",
  "task_id": "tsk_123",
  "seq": 1,
  "event_type": "run.state",
  "stream": "system",
  "payload": {
    "status": "running",
    "previous_status": "queued"
  },
  "occurred_at": "2026-05-11T08:01:00Z"
}
```

### `codex.event`

Emitted for each Codex JSONL object.

```json
{
  "run_id": "run_123",
  "task_id": "tsk_123",
  "seq": 2,
  "event_type": "codex.event",
  "stream": "jsonl",
  "payload": {
    "raw": {
      "type": "message"
    },
    "text": "Optional display text extracted for the UI.",
    "session_id": "codex-session-id"
  },
  "occurred_at": "2026-05-11T08:01:01Z"
}
```

The UI should render from normalized fields when present, but it must tolerate
unknown or changing Codex event shapes because the raw object is authoritative.

### `process.output`

Emitted for non-JSON stdout or stderr fallback output.

```json
{
  "run_id": "run_123",
  "task_id": "tsk_123",
  "seq": 3,
  "event_type": "process.output",
  "stream": "stderr",
  "payload": {
    "text": "warning: ..."
  },
  "occurred_at": "2026-05-11T08:01:02Z"
}
```

### `runner.error`

Emitted when the runner reports an execution problem.

```json
{
  "run_id": "run_123",
  "task_id": "tsk_123",
  "seq": 4,
  "event_type": "runner.error",
  "stream": "system",
  "payload": {
    "message": "Codex executable not found.",
    "code": "codex_not_found"
  },
  "occurred_at": "2026-05-11T08:01:03Z"
}
```

### `run.final`

Emitted once for the terminal run result.

```json
{
  "run_id": "run_123",
  "task_id": "tsk_123",
  "seq": 99,
  "event_type": "run.final",
  "stream": "system",
  "payload": {
    "status": "succeeded",
    "exit_code": 0,
    "final_message": "Implemented the change and ran tests.",
    "error_message": null,
    "codex_session_id": "codex-session-id"
  },
  "occurred_at": "2026-05-11T08:04:00Z"
}
```

The server may close the SSE stream after sending `run.final`.

## 7. Runner WebSocket Protocol

The runner connects to:

```text
GET /api/v1/runner/ws
```

Every WebSocket message is one JSON object with this envelope:

```json
{
  "type": "runner.heartbeat",
  "message_id": "msg_123",
  "sent_at": "2026-05-11T08:00:00Z",
  "payload": {}
}
```

### `runner.register`

Direction: runner to control plane.

```json
{
  "type": "runner.register",
  "message_id": "msg_001",
  "sent_at": "2026-05-11T08:00:00Z",
  "payload": {
    "runner_id": "runner_desktop",
    "hostname": "desktop",
    "version": "0.5.0",
    "codex_path": "codex",
    "capabilities": {
      "codex_exec": true,
      "cancel": true,
      "fs_list": true,
      "project_files": true,
      "project_file_io": true,
      "project_file_upload": true,
      "project_terminal": true,
      "project_command": true,
      "codex_options": true,
      "active_runs": true,
      "self_update": true,
      "shutdown": true
    }
  }
}
```

### `runner.heartbeat`

Direction: runner to control plane.

```json
{
  "type": "runner.heartbeat",
  "message_id": "msg_002",
  "sent_at": "2026-05-11T08:00:10Z",
  "payload": {
    "runner_id": "runner_desktop",
    "active_run_ids": ["run_123"]
  }
}
```

### `run.assign`

Direction: control plane to runner.

```json
{
  "type": "run.assign",
  "message_id": "msg_100",
  "sent_at": "2026-05-11T08:01:00Z",
  "payload": {
    "run_id": "run_123",
    "task_id": "tsk_123",
    "project_id": "prj_123",
    "workdir": "D:\\go\\workplace",
    "mode": "resume",
    "codex_session_id": "codex-session-id",
    "codex_model": "gpt-5.5",
    "codex_reasoning_effort": "high",
    "codex_service_tier": "fast",
    "prompt": "Continue the current task...",
    "argv": [
      "codex",
      "--cd",
      "D:\\go\\workplace",
      "--model",
      "gpt-5.5",
      "--config",
      "model_reasoning_effort=\"high\"",
      "--config",
      "service_tier=\"fast\"",
      "exec",
      "resume",
      "--dangerously-bypass-approvals-and-sandbox",
      "--skip-git-repo-check",
      "--json",
      "codex-session-id",
      "-"
    ]
  }
}
```

For `mode = new`, `argv` must be:

```json
["codex", "--cd", "D:\\go\\workplace", "exec", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "--json", "-"]
```

When run options are set, they appear before `exec`, for example
`["codex", "--cd", "...", "--model", "gpt-5.5", "--config",
"model_reasoning_effort=\"high\"", "exec", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "--json", "-"]`.

The runner writes `prompt` to Codex stdin.

### `run.started`

Direction: runner to control plane.

```json
{
  "type": "run.started",
  "message_id": "msg_101",
  "sent_at": "2026-05-11T08:01:01Z",
  "payload": {
    "run_id": "run_123",
    "pid": 12345,
    "started_at": "2026-05-11T08:01:01Z"
  }
}
```

### `run.event`

Direction: runner to control plane.

```json
{
  "type": "run.event",
  "message_id": "msg_102",
  "sent_at": "2026-05-11T08:01:02Z",
  "payload": {
    "run_id": "run_123",
    "source_seq": 1,
    "event_type": "codex.event",
    "stream": "jsonl",
    "event_payload": {
      "raw": {
        "type": "message"
      },
      "text": "Optional display text extracted by the runner.",
      "session_id": "codex-session-id"
    },
    "occurred_at": "2026-05-11T08:01:02Z"
  }
}
```

Rules:

- `source_seq` is monotonic per run from the runner.
- The control plane still assigns the persisted browser-facing `seq`.
- `event_type` must be one of `codex.event`, `process.output`, or
  `runner.error`.
- For JSONL output, `event_payload.raw` contains the parsed Codex JSON object.
- For stdout or stderr fallback output, use `stream = stdout` or `stderr` and
  `event_payload.text`.
- Unknown Codex fields must not be discarded.

### `run.cancel`

Direction: control plane to runner.

```json
{
  "type": "run.cancel",
  "message_id": "msg_200",
  "sent_at": "2026-05-11T08:02:00Z",
  "payload": {
    "run_id": "run_123",
    "reason": "User canceled from task page.",
    "requested_at": "2026-05-11T08:02:00Z"
  }
}
```

### `run.cancel_ack`

Direction: runner to control plane.

```json
{
  "type": "run.cancel_ack",
  "message_id": "msg_201",
  "sent_at": "2026-05-11T08:02:01Z",
  "payload": {
    "run_id": "run_123",
    "accepted": true
  }
}
```

### `run.completed`

Direction: runner to control plane.

```json
{
  "type": "run.completed",
  "message_id": "msg_300",
  "sent_at": "2026-05-11T08:04:00Z",
  "payload": {
    "run_id": "run_123",
    "status": "succeeded",
    "exit_code": 0,
    "error_message": null,
    "final_message": "Implemented the change and ran tests.",
    "codex_session_id": "codex-session-id",
    "ended_at": "2026-05-11T08:04:00Z"
  }
}
```

Allowed completed statuses:

- `succeeded`
- `failed`
- `canceled`

### `runner.update`

Direction: control plane to runner.

```json
{
  "type": "runner.update",
  "message_id": "msg_350",
  "sent_at": "2026-05-11T08:04:05Z",
  "payload": {}
}
```

### `runner.update.response`

Direction: runner to control plane. The response uses the same `message_id` as
the request.

```json
{
  "type": "runner.update.response",
  "message_id": "msg_350",
  "sent_at": "2026-05-11T08:04:05Z",
  "payload": {
    "accepted": true,
    "message": "Runner update started. The websocket may disconnect while the runner restarts."
  }
}
```

When accepted, the runner asynchronously reruns the platform install script
using its current control URL, runner id, Codex path, and runner token as a
local updater secret for `Authorization` headers. Windows runners use
`install.ps1`; Linux and macOS runners use `install.sh`. The current websocket
can disconnect while the binary is replaced and restarted.

### `runner.shutdown`

Direction: control plane to runner.

```json
{
  "type": "runner.shutdown",
  "message_id": "msg_360",
  "sent_at": "2026-05-11T08:04:30Z",
  "payload": {
    "reason": "server_deleted"
  }
}
```

### `runner.shutdown.response`

Direction: runner to control plane. The response uses the same `message_id` as
the request.

```json
{
  "type": "runner.shutdown.response",
  "message_id": "msg_360",
  "sent_at": "2026-05-11T08:04:30Z",
  "payload": {
    "accepted": true,
    "message": "Runner shutdown accepted. The websocket will disconnect."
  }
}
```

When accepted, the runner cancels local active work, disables the local service
or startup entry when possible, writes a local disabled marker, and exits
without reconnecting. Reinstalling the runner clears the disabled marker.

### `fs.list`

Direction: control plane to runner.

```json
{
  "type": "fs.list",
  "message_id": "msg_400",
  "sent_at": "2026-05-11T08:04:10Z",
  "payload": {
    "path": "D:\\go"
  }
}
```

### `fs.list.response`

Direction: runner to control plane. The response uses the same `message_id` as
the request.

```json
{
  "type": "fs.list.response",
  "message_id": "msg_400",
  "sent_at": "2026-05-11T08:04:10Z",
  "payload": {
    "path": "D:\\go",
    "parent": "D:\\",
    "entries": [
      {
        "name": "workplace",
        "path": "D:\\go\\workplace",
        "is_dir": true,
        "markers": [".git", "AGENTS.md", "go.mod"]
      }
    ]
  }
}
```

Rules:

- This message is a project setup helper only; it must not execute commands or
  inspect file contents.
- The runner returns directories only.
- `markers` may include project hints such as `.git`, `AGENTS.md`, `go.mod`,
  `package.json`, `pyproject.toml`, or `Cargo.toml`.

### `project.files`

Direction: control plane to runner.

```json
{
  "type": "project.files",
  "message_id": "msg_500",
  "sent_at": "2026-05-11T08:04:20Z",
  "payload": {
    "workdir": "D:\\go\\workplace",
    "path": "frontend/src"
  }
}
```

### `project.files.response`

Direction: runner to control plane. The response uses the same `message_id` as
the request.

```json
{
  "type": "project.files.response",
  "message_id": "msg_500",
  "sent_at": "2026-05-11T08:04:20Z",
  "payload": {
    "root": "D:\\go\\workplace",
    "path": "frontend/src",
    "parent": "frontend",
    "entries": [
      {
        "name": "App.tsx",
        "path": "frontend/src/App.tsx",
        "is_dir": false,
        "size": 42000
      }
    ]
  }
}
```

Rules:

- The runner resolves `path` inside `workdir` and rejects paths outside it.
- The response includes both files and directories, but not file contents.

### `project.file.read`

Direction: control plane to runner.

```json
{
  "type": "project.file.read",
  "message_id": "msg_510",
  "sent_at": "2026-05-11T08:04:22Z",
  "payload": {
    "workdir": "D:\\go\\workplace",
    "path": "frontend/src/App.tsx",
    "max_bytes": 2097152
  }
}
```

### `project.file.read.response`

Direction: runner to control plane. The response uses the same `message_id` as
the request.

```json
{
  "type": "project.file.read.response",
  "message_id": "msg_510",
  "sent_at": "2026-05-11T08:04:22Z",
  "payload": {
    "root": "D:\\go\\workplace",
    "path": "frontend/src/App.tsx",
    "name": "App.tsx",
    "size": 42000,
    "content": "export function App() {}\n",
    "encoding": "utf-8"
  }
}
```

### `project.file.write`

Direction: control plane to runner.

```json
{
  "type": "project.file.write",
  "message_id": "msg_520",
  "sent_at": "2026-05-11T08:04:23Z",
  "payload": {
    "workdir": "D:\\go\\workplace",
    "path": "notes/todo.md",
    "content": "- verify\n",
    "create_dirs": true
  }
}
```

### `project.file.write.response`

Direction: runner to control plane. The response uses the same `message_id` as
the request.

```json
{
  "type": "project.file.write.response",
  "message_id": "msg_520",
  "sent_at": "2026-05-11T08:04:23Z",
  "payload": {
    "root": "D:\\go\\workplace",
    "path": "notes/todo.md",
    "is_dir": false,
    "size": 9
  }
}
```

### `project.file.upload`

Direction: control plane to runner.

```json
{
  "type": "project.file.upload",
  "message_id": "msg_525",
  "sent_at": "2026-05-11T08:04:23Z",
  "payload": {
    "workdir": "D:\\go\\workplace",
    "path": "assets/logo.png",
    "content_base64": "iVBORw0KGgo=",
    "create_dirs": true
  }
}
```

### `project.file.upload.response`

Direction: runner to control plane. The response uses the same `message_id` as
the request.

```json
{
  "type": "project.file.upload.response",
  "message_id": "msg_525",
  "sent_at": "2026-05-11T08:04:23Z",
  "payload": {
    "root": "D:\\go\\workplace",
    "path": "assets/logo.png",
    "is_dir": false,
    "size": 8
  }
}
```

### `project.file.action`

Direction: control plane to runner.

```json
{
  "type": "project.file.action",
  "message_id": "msg_530",
  "sent_at": "2026-05-11T08:04:24Z",
  "payload": {
    "workdir": "D:\\go\\workplace",
    "action": "create",
    "path": "notes",
    "is_dir": true
  }
}
```

### `project.file.action.response`

Direction: runner to control plane. The response uses the same `message_id` as
the request.

```json
{
  "type": "project.file.action.response",
  "message_id": "msg_530",
  "sent_at": "2026-05-11T08:04:24Z",
  "payload": {
    "root": "D:\\go\\workplace",
    "path": "notes",
    "is_dir": true
  }
}
```

Rules:

- `project.file.write.response`, `project.file.upload.response`, and
  `project.file.action.response` return the affected project-root-relative path
  and metadata.
- The runner must reject writes, uploads, renames, and deletes that escape
  `workdir`.
- The runner must not delete the project root.

### `project.terminal.open`

Direction: control plane to runner.

```json
{
  "type": "project.terminal.open",
  "message_id": "msg_540",
  "sent_at": "2026-05-11T08:04:25Z",
  "payload": {
    "terminal_id": "term_123",
    "workdir": "D:\\go\\workplace",
    "cols": 100,
    "rows": 30
  }
}
```

### `project.terminal.open.response`

Direction: runner to control plane. The response uses the same `message_id` as
the request.

```json
{
  "type": "project.terminal.open.response",
  "message_id": "msg_540",
  "sent_at": "2026-05-11T08:04:25Z",
  "payload": {
    "terminal_id": "term_123",
    "workdir": "D:\\go\\workplace"
  }
}
```

### `project.terminal.output`

Direction: runner to control plane.

```json
{
  "type": "project.terminal.output",
  "message_id": "msg_541",
  "sent_at": "2026-05-11T08:04:26Z",
  "payload": {
    "terminal_id": "term_123",
    "data": "PS D:\\go\\workplace> "
  }
}
```

Terminal input, resize, close, and exit use `project.terminal.input`,
`project.terminal.resize`, `project.terminal.close`, and
`project.terminal.exit` with the same `terminal_id`.

### `project.command`

Direction: control plane to runner.

```json
{
  "type": "project.command",
  "message_id": "msg_600",
  "sent_at": "2026-05-11T08:04:30Z",
  "payload": {
    "workdir": "D:\\go\\workplace",
    "command": "git status --short",
    "timeout_secs": 60
  }
}
```

### `project.command.response`

Direction: runner to control plane. The response uses the same `message_id` as
the request.

```json
{
  "type": "project.command.response",
  "message_id": "msg_600",
  "sent_at": "2026-05-11T08:04:31Z",
  "payload": {
    "command": "git status --short",
    "workdir": "D:\\go\\workplace",
    "exit_code": 0,
    "stdout": "",
    "stderr": "",
    "duration_ms": 120,
    "error": null
  }
}
```

Rules:

- The runner executes from `workdir` using the platform shell.
- The runner applies a bounded timeout and truncates large output.
- This helper is for project-local inspection and verification commands.

## 8. Minimal Implementation Order

Use this contract to split implementation safely:

1. Implement database schema and HTTP CRUD for servers, projects, tasks, and
   context items.
2. Implement run creation with the active-run invariant and prompt snapshots.
3. Implement runner WebSocket registration, heartbeat, assignment, events, and
   completion.
4. Implement browser SSE replay and live stream.
5. Replace fake runner output with real Codex CLI execution.
6. Add cancel support.
