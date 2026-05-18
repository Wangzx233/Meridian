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

Meridian is a web console for managing project-scoped Codex CLI work across
servers. It is not a new agent runtime, not an IDE, and not a replacement for
Codex. A small runner on each target machine invokes the real local Codex CLI
inside the real project directory.

Meridian is useful when one terminal is no longer enough: multiple machines,
multiple projects, long-running tasks, resumable Codex sessions, visible run
history, and explicit user-selected context.

## Quick Start

Docker Compose is the recommended path for open-source users. It starts
PostgreSQL, starts the backend, automatically applies database migrations,
builds runner artifacts for the in-app installer, and serves the web UI.

```bash
git clone https://github.com/Wangzx233/Meridian.git
cd Meridian
docker compose up -d --build
```

Open Meridian:

```text
http://<server-ip>:18080
```

For a local-only trial on the same machine, `http://127.0.0.1:18080` also works.
The default Compose bind address is `0.0.0.0`, so a self-hosted server is
reachable without extra bind-address configuration. On first browser visit,
create the initial admin account.

To customize the port, database password, external database, or auth settings:

```bash
cp .env.example .env
vi .env
docker compose up -d --build
```

Use HTTPS for shared or internet-facing deployments. If Meridian is only exposed
through a local reverse proxy, set `MERIDIAN_HTTP_BIND=127.0.0.1` in `.env`.

## Connect A Runner

Install Codex CLI on every machine that will execute tasks. Then use Meridian
itself to install the runner:

1. Open the web UI and create or select a server.
2. Click the runner install button in the top-right corner.
3. Set the Control URL to the URL reachable from the target machine.
4. Copy the Linux, macOS, or Windows command shown by the UI and run it on the
   target machine.
5. Create a project for that server and set `workdir` to the real project
   directory.

Do not use `127.0.0.1` in the installer for a remote runner unless Meridian is
running on that same machine. The backend image and the source deployment flow
both build the runner artifacts used by these install endpoints; you should not
need to handcraft runner download commands.

## Manual Source Deployment

Use this path when you do not want Docker Compose to run the app stack. The
backend applies migrations on startup by default, so a first deployment does not
need a separate migration command.

```bash
sh ./scripts/build-runner-artifacts.sh

cd frontend
npm ci
npm run build
cd ..

DATABASE_URL='postgres://user:password@db-host:5432/meridian?sslmode=disable' \
BACKEND_ADDR='0.0.0.0:8080' \
RUNNER_ARTIFACT_DIR="$PWD/artifacts/runner" \
go run ./backend/cmd/server
```

Serve `frontend/dist` with your web server and proxy `/api` plus WebSocket
traffic to the backend. After the UI is reachable, use the same top-right runner
install menu to connect target machines.

More deployment options, external database settings, and Windows notes are in
[Deployment Guide](docs/deployment.md).

## Development

Use the source workflow when developing Meridian itself:

```bash
docker run --name meridian-postgres \
  -e POSTGRES_DB=meridian_dev \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 55433:5432 \
  -d postgres:16-alpine

sh ./scripts/build-runner-artifacts.sh

DATABASE_URL='postgres://postgres:postgres@127.0.0.1:55433/meridian_dev?sslmode=disable' \
BACKEND_ADDR='127.0.0.1:18080' \
RUNNER_ARTIFACT_DIR="$PWD/artifacts/runner" \
go run ./backend/cmd/server
```

In another shell:

```bash
cd frontend
npm ci
VITE_API_PROXY_TARGET='http://127.0.0.1:18080' \
VITE_CONTROL_URL='http://127.0.0.1:18080' \
npm run dev
```

Open `http://127.0.0.1:5173`.

## Capabilities

| Capability | Description |
| --- | --- |
| Multi-server control | Manage machines that can run the local runner and track their connection state. |
| Real project directories | Each project points to a real working directory on one server. |
| Long-lived tasks | A task can span many Codex turns. A successful run does not complete the task. |
| Codex session resume | Store the Codex CLI session id and resume later turns in the same task. |
| Explicit context | Users manually attach small, visible context items. |
| Live run output | Stream Codex run events from the runner back to the web console. |
| Project tools | Browse files, do lightweight edits, and run project-local terminal commands. |
| Runner distribution | Serve Linux, macOS, and Windows runner installers from the backend. |

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

## Basic Usage

1. Open the web UI.
2. Create a server and install its runner from the top-right install menu.
3. Create a project under that server and set the real `workdir`.
4. Create a task.
5. Send one instruction per turn.
6. Use Output, Terminal, and Files to inspect the project and run state.
7. Continue until the work is actually complete.
8. Manually mark the task done.

## Repository Layout

```text
backend/   Go control-plane API
runner/    Go runner agent that connects to the control plane and invokes Codex CLI
frontend/  React + TypeScript + Vite web UI
db/        PostgreSQL migrations
docs/      Requirements, architecture, API contract, deployment, and release docs
scripts/   Local helper scripts
```

## Checks

```bash
go test ./...
go vet ./...
(cd frontend && npm ci && npm run build)
sh ./scripts/build-runner-artifacts.sh
```

## Current Limitations

- Authentication is a simple login gate only; there is no self-registration or
  fine-grained permission model.
- Runner install endpoints are intended for trusted environments.
- Runner artifacts are not signed.
- Codex CLI must be installed separately on runner machines.
- Context is manually selected; v1 does not recommend or inject context
  automatically.
- A successful Codex run does not complete a task. The user must manually mark
  the task done.

## Related Documents

- [Deployment Guide](docs/deployment.md)
- [Contributing](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [Release Checklist](docs/release-checklist.md)
- [Requirements](docs/requirements.md)
- [Architecture](docs/architecture.md)
- [API Contract](docs/api-contract.md)
