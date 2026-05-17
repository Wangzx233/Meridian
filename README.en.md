# Meridian

<p align="center">
  <img src="frontend/public/favicon.svg" alt="Meridian icon" width="96" height="96">
</p>

English | [中文](README.md)

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Node.js](https://img.shields.io/badge/Node.js-20.19%2B-339933?logo=nodedotjs&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=111)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16%2B-4169E1?logo=postgresql&logoColor=white)
![CI](https://img.shields.io/badge/CI-GitHub_Actions-2088FF?logo=githubactions&logoColor=white)

Meridian is a task workbench for running Codex CLI in real project directories.

It is not a new AI agent, not an IDE, and not an app that replaces Codex.
Meridian does one narrower, more auditable job: it turns Codex CLI executions
scattered across servers and project directories into long-lived tasks that can
be queued, observed, resumed, continued, and manually completed.

## What It Actually Does

Codex CLI is strong at doing real coding work inside a project directory. Once
you have multiple servers, multiple projects, and multiple unfinished tasks,
plain terminals become hard to manage:

- You need to remember which machine and working directory each task belongs to.
- You need to keep the Codex session id so later turns can resume.
- You need to know whether a run succeeded, failed, was canceled, or is simply
  waiting for the user to continue.
- You need context sources to stay explicit instead of silently injecting hidden
  material.
- You need runner install, runner status, run output, and project file actions
  in one control surface.

Meridian provides that task control plane. The user selects
`Server -> Project -> Task` in the web UI and sends one instruction per turn.
The runner on the target server invokes the local Codex CLI in the real project
directory. The backend stores tasks, runs, events, context items, and Codex
session ids. A task is complete only when the user manually marks it done.

The runner invokes the real Codex CLI directly:

```bash
codex --cd <project_workdir> exec --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --json -
codex --cd <project_workdir> exec resume --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --json <session_id> -
```

### Why Not Hermes / OpenClaw

Projects like Hermes and OpenClaw are closer to general-purpose agents or
personal automation platforms. They typically emphasize model and tool
integrations, long-term memory, skill extension, messaging channels, autonomous
execution, and broad personal or business automation.

Meridian intentionally does not take that path. It does not build a new agent
runtime, choose models for you, create an automatic skill system, build a
knowledge graph, or turn tasks into autonomous workflows. It manages one
project-scoped Codex CLI workflow: which server, which project directory, which
task, which Codex session, and what happened in every run.

If you want a general assistant that learns, talks across channels, and connects
to many tools, Hermes / OpenClaw may be a better fit. If you already want Codex
CLI as the coding executor and need a multi-project, multi-server, resumable
task console, Meridian is aimed at that gap.

### Why Not IDE + AI

IDE + AI is best for one developer sitting in an editor. Its strengths are
file-local context, completion, navigation, review, and interactive editing.

Meridian solves a different problem: running Codex CLI tasks in real project
directories on real servers, then continuing those tasks over time. It does not
try to become a full browser IDE or move the project into the web app. You can
keep using your own IDE; Meridian manages Codex turns that need server-local
execution, observation, and resume.

### Why Not Codex App / Plain Codex CLI

If you only run Codex once on one project from one machine, plain Codex CLI or a
Codex app is simpler.

Meridian becomes useful when:

- You maintain multiple projects across different servers or working
  directories.
- A task needs multiple Codex turns instead of one run.
- You need run history, failure details, event streams, and final states.
- You want runner install, runner update, artifact distribution, and run entry
  points in one place.
- You need a clear distinction between "run succeeded" and "task is done."

### Differentiation

| Meridian choice | Difference |
| --- | --- |
| Reuse the real Codex CLI | No replacement agent and no change to Codex CLI's core behavior. |
| Project directory first | Every run executes in the configured real `workdir`. |
| Long-lived tasks | A successful run does not close the task; users can keep adding turns. |
| Store Codex session ids | Later turns in the same task can resume the original Codex session. |
| Explicit context selection | Context items are small and visible; v1 avoids automatic recommendation or hidden injection. |
| Runner runs on target servers | Supports multiple servers and projects instead of binding to the current local IDE. |
| Control plane records state | Tasks, runs, events, and runner status are visible in one system. |
| Infrastructure first | CI, vet, frontend build, runner artifact build, and release checklist are part of the project workflow. |

## Quick Start

### Docker Compose

The recommended open-source trial path is Docker Compose. It starts
PostgreSQL, applies migrations, builds runner artifacts, starts the backend,
and serves the web UI with `/api` and WebSocket traffic proxied to the backend.

```bash
docker compose up --build
```

Open:

```text
http://127.0.0.1:18080
```

On the first browser visit, create the initial admin account. The local Compose
profile stores PostgreSQL data in the `postgres_data` Docker volume.

To customize the port, database password, or auth settings:

```powershell
Copy-Item .env.example .env
```

Then edit `.env` and restart:

```bash
docker compose up --build
```

For a shared or internet-facing deployment, set `MERIDIAN_HTTP_BIND=0.0.0.0`,
put Meridian behind HTTPS, and set `WORKBENCH_AUTH_COOKIE_SECURE=true`.

### Source Development

Use the source workflow when developing Meridian itself or when you want to run
the frontend and backend separately.

Required tools:

- Go 1.25 or newer.
- Node.js 20.19 or newer, plus npm.
- PostgreSQL with the `pgcrypto` extension available.
- Codex CLI installed on every machine where a runner will execute tasks.
- PowerShell is the easiest path for Windows local development because the
  runner artifact script is PowerShell-based.

Start PostgreSQL:

```powershell
docker run --name meridian-postgres `
  -e POSTGRES_DB=meridian_dev `
  -e POSTGRES_USER=postgres `
  -e POSTGRES_PASSWORD=postgres `
  -p 55433:5432 `
  -d postgres:16-alpine
```

Set the database URL and apply migrations:

```powershell
$env:DATABASE_URL = "postgres://postgres:postgres@127.0.0.1:55433/meridian_dev?sslmode=disable"
go run ./backend/cmd/migrate up
```

If migrations live outside the repository root, set `MIGRATIONS_DIR`:

```powershell
$env:MIGRATIONS_DIR = "D:\go\workplace\db\migrations"
```

Build runner artifacts for the install endpoints:

```powershell
.\scripts\build-runner-artifacts.ps1
```

If `go` is not on `PATH`, set `GO_EXE`:

```powershell
$env:GO_EXE = "C:\Users\DELL\sdk\go1.26.1\bin\go.exe"
.\scripts\build-runner-artifacts.ps1
```

Start the backend:

```powershell
$env:DATABASE_URL = "postgres://postgres:postgres@127.0.0.1:55433/meridian_dev?sslmode=disable"
$env:BACKEND_ADDR = "127.0.0.1:18080"
$env:RUNNER_ARTIFACT_DIR = "D:\go\workplace\artifacts\runner"
go run ./backend/cmd/server
```

The API is available at:

```text
http://127.0.0.1:18080/api/v1
```

Start the frontend in another shell:

```powershell
cd frontend
npm install
$env:VITE_API_PROXY_TARGET = "http://127.0.0.1:18080"
$env:VITE_CONTROL_URL = "http://127.0.0.1:18080"
npm run dev
```

Open:

```text
http://127.0.0.1:5173
```

## Capabilities

| Capability | Description |
| --- | --- |
| Multi-server control | Manage machines that can run the local runner and track their connection state. |
| Real project directories | Each project points to a real working directory on one server. |
| Long-lived tasks | A task can span many Codex turns. A successful run does not complete the task. |
| Codex session resume | Store the Codex CLI session id and resume later turns in the same task. |
| Explicit context | Users manually attach a small set of source-visible context items. |
| Live run output | Stream Codex run events from the runner back to the web console. |
| Project tools | Browse files, do lightweight edits, and run project-local terminal commands. |
| Runner distribution | Build Windows, Linux, and macOS runner artifacts for backend install endpoints. |

## Architecture

```text
Browser UI
  -> Go backend control plane
  -> PostgreSQL task/run/event store

Go backend control plane
  <-> runner WebSocket
  <-> target server runner
  -> local Codex CLI in the project workdir
```

## Repository Layout

```text
backend/   Go control-plane API
runner/    Go runner agent that connects to the control plane and invokes Codex CLI
frontend/  React + TypeScript + Vite web UI
db/        PostgreSQL migrations
docs/      Requirements, architecture, API contract, and release checklist
scripts/   Local helper scripts
```

## Basic Usage

1. Open the web UI.
2. Create a server.
3. Install and start a runner on that server.
4. Select the server, then create a project under it.
5. Set the project `workdir` to the real project directory.
6. Create a task.
7. Use Output for Codex logs, Terminal for a project-local shell, and Files for
   project file inspection and lightweight editing.
8. Send one instruction per turn.
9. Continue the task until you are satisfied.
10. Manually mark the task done.

## Server / Project / Runner Model

| Concept | Role |
| --- | --- |
| `Server` | A machine that can run the runner. The important value is `runner_id`. |
| `Project` | A real working directory on one server. Codex runs in this directory. |
| `Task` | A real work objective, usually completed over multiple turns. |
| `Run` / `Turn` | One user instruction and one Codex CLI execution. |
| `ContextItem` | A manually selected rule, note, log, summary, command, or file hint. |
| `CodexSession` | The Codex CLI session id used for resume. |

The runner reconnects after network drops or backend restarts. The backend also
gives a disconnected runner a short grace period before marking the server
offline.

Each turn can optionally override the Codex model, reasoning effort, and service
tier:

```bash
--model gpt-5.5 --config model_reasoning_effort="high" --config service_tier="fast"
```

Leaving them blank uses the runner machine's local Codex config.

## Installing A Runner

Use the install button in the top-right of the web UI, or call the install
endpoints directly. For a new machine, do not pass `runner_id`; the installer
derives a unique id from that machine. Only pass `runner_id=<runner-id>` when
reinstalling the same server record.

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -NoProfile -Command "iex ((iwr -UseBasicParsing -Uri 'http://<control-host>:<port>/api/v1/runner/install.ps1').Content)"
```

For a machine-level SYSTEM task, run PowerShell as Administrator and add
`run_as=system`:

```powershell
powershell -ExecutionPolicy Bypass -NoProfile -Command "iex ((iwr -UseBasicParsing -Uri 'http://<control-host>:<port>/api/v1/runner/install.ps1?run_as=system').Content)"
```

Linux or macOS:

```bash
curl -fsSL 'http://<control-host>:<port>/api/v1/runner/install.sh' | sh
```

On Linux, the installer uses systemd when it is available. In containers or
other non-systemd environments, it falls back to a standalone `nohup`
background process under `/opt/codex-task-workbench/runner` and writes
`run-runner.sh`, `runner.pid`, `runner.log`, and `runner.err.log`. Standalone
mode does not survive container or host restarts; rerun the installer or start
`run-runner.sh` manually after a restart.

Do not use `127.0.0.1` on a remote runner machine unless the control plane runs
on the same machine.

The bootstrap scripts install:

| Platform | Install method |
| --- | --- |
| Windows user mode | `%LOCALAPPDATA%\CodexTaskWorkbench\runner` plus a Startup folder command. |
| Windows SYSTEM mode | A Scheduled Task named `CodexTaskWorkbenchRunner`. |
| Linux | A systemd service named `codex-task-workbench-runner.service`. |
| macOS | A launchd daemon named `com.codex-task-workbench.runner`. |

If Codex CLI is not on the target machine's default path, pass a custom path:

```bash
curl -fsSL 'http://<control-host>:<port>/api/v1/runner/install.sh?codex_path=/usr/local/bin/codex' | sh
```

## Manual Runner Start

```powershell
$env:CONTROL_URL = "http://127.0.0.1:18080"
$env:RUNNER_ID = "local_runner"
$env:CODEX_PATH = "codex"
go run ./runner/cmd/runner
```

Linux or macOS:

```bash
CONTROL_URL=http://127.0.0.1:18080 \
RUNNER_ID=local_runner \
CODEX_PATH=codex \
go run ./runner/cmd/runner
```

## Key Environment Variables

### Backend

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | required | PostgreSQL connection string. |
| `BACKEND_ADDR` | `:8080` | HTTP listen address. |
| `RUNNER_ARTIFACT_DIR` | `artifacts/runner` | Directory served by runner artifact endpoints. |
| `CODEX_BYPASS_APPROVALS_AND_SANDBOX` | `true` | Adds Codex CLI bypass sandbox/approval flags. Set `false` to preserve Codex defaults. |
| `WORKBENCH_AUTH_USERS` | empty | Comma-separated login accounts such as `admin:password,ops:password2`. Empty enters browser first-run setup. |
| `WORKBENCH_AUTH_SESSION_SECRET` | required when auth users are set | Secret used to sign browser session cookies. |
| `WORKBENCH_RUNNER_TOKEN` | required when auth users are set | Bearer token for runner install scripts, runner WebSocket connections, and artifact downloads. |
| `WORKBENCH_AUTH_COOKIE_SECURE` | `true` | Whether browser session cookies require HTTPS. Set `false` only for local HTTP development. |

### Runner

| Variable | Default | Purpose |
| --- | --- | --- |
| `CONTROL_URL` | `http://localhost:8080` | Control-plane base URL. |
| `RUNNER_ID` | machine hostname | Runner identity matched with a server record. |
| `CODEX_PATH` | `codex` | Codex CLI executable path. |
| `RUNNER_TOKEN` | empty | Bearer token for connecting to an authenticated control plane. |
| `CODEX_BYPASS_APPROVALS_AND_SANDBOX` | `true` | Whether the runner ensures Codex runs without sandbox or approval prompts. |
| `HEARTBEAT_INTERVAL` | `10s` | Runner heartbeat interval. |
| `RUNNER_ENV` | empty | Extra environment variables separated by `;`. |

### Frontend

| Variable | Default | Purpose |
| --- | --- | --- |
| `VITE_API_PROXY_TARGET` | `http://127.0.0.1:8080` | Dev-server proxy target for `/api`. |
| `VITE_API_BASE_URL` | `/api/v1` | Browser API base URL. |
| `VITE_CONTROL_URL` | browser origin | Default URL shown in runner install commands. |

## Development Checks

Run before committing or merging:

```powershell
go test ./...
go vet ./...
cd frontend
npm run build
```

Build runner artifacts when runner behavior, install scripts, release packaging,
or backend artifact-serving behavior changes:

```powershell
.\scripts\build-runner-artifacts.ps1
```

GitHub Actions runs:

- `go test ./...`
- `go vet ./...`
- `npm run build`
- runner artifact build

## Releases

Release preparation is documented in [docs/release-checklist.md](docs/release-checklist.md).

Pushing a tag like `v0.1.0` triggers the GitHub release workflow:

```powershell
git tag v0.1.0
git push origin v0.1.0
```

The release workflow reruns the quality gate, builds runner artifacts, writes
`SHA256SUMS.txt`, and publishes the files to GitHub Release.

## Deployment Notes

For most self-hosted installs, use the root `docker-compose.yml`:

1. Starts bundled PostgreSQL by default.
2. Runs `migrate up` before the backend starts.
3. Builds and serves runner artifacts from the backend image.
4. Serves the built frontend through Nginx.
5. Proxies `/api`, SSE, and WebSocket upgrade traffic to the backend.

Use `.env.example` as the deployment template. The default bind address is
`127.0.0.1` for local safety; change `MERIDIAN_HTTP_BIND` only when a reverse
proxy, firewall, and HTTPS plan are in place.

Source-based deployment is still supported:

1. Provision PostgreSQL.
2. Apply migrations with `go run ./backend/cmd/migrate up`.
3. Build runner artifacts with `scripts/build-runner-artifacts.ps1`.
4. Start the backend with `DATABASE_URL`, `BACKEND_ADDR`, and
   `RUNNER_ARTIFACT_DIR` set.
5. Build the frontend with `npm run build`.
6. Serve `frontend/dist` with a static web server or reverse proxy.
7. Proxy `/api` and WebSocket upgrade traffic to the backend.

Public or shared deployments should use HTTPS and make runner install commands
use the public control-plane URL. If `WORKBENCH_AUTH_USERS` is empty, the first
browser visit enters admin setup mode.

## Current Limitations

- Authentication is a simple login gate only; there is no self-registration or
  fine-grained permission model.
- Runner install endpoints are intended for trusted environments.
- Runner artifacts are not signed.
- Codex CLI must be installed separately on runner machines.
- The first version keeps context manual; it does not recommend or inject
  context automatically.
- A successful Codex run does not complete a task. The user must manually mark
  the task done.

## Related Documents

- [Contributing](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [Release Checklist](docs/release-checklist.md)
- [Requirements](docs/requirements.md)
- [Architecture](docs/architecture.md)
- [API Contract](docs/api-contract.md)
