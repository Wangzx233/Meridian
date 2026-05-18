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

Meridian is a web console for managing task work across machines, projects, and long-lived Codex CLI sessions. It puts devices, projects, tasks, run output, history, and selected context into one browser-based control surface.

Meridian does not replace Codex and does not reimplement an agent runtime. The real work is still executed by the Codex CLI installed on the target machine.

Meridian runs a small device agent on each machine and uses that agent to manage local Codex CLI runs.

## When To Use Meridian

- Codex CLI runs on more than one machine and needs a single entry point.
- Several projects live on different machines or working directories.
- A task needs explicit context from earlier work instead of starting from scratch.
- Many Codex tasks may be running, waiting, or failing at the same time.
- Run status, output, history, and failure details should be visible from the browser.

## How It Differs

| Alternative | Main Experience | Meridian Difference |
| --- | --- | --- |
| Hermes / OpenClaw | General agents, personal automation, and cross-tool orchestration. | Meridian does not create a new agent runtime; it manages project tasks around Codex CLI. |
| IDE + AI | Coding inside an editor, often with remote development support. | Meridian switches between machines, projects, and tasks from the web instead of opening a separate editor or shell for every task. |
| Codex App / CLI | Direct interaction with Codex. Apps may also connect to servers. | Target machines do not need public IPs, and the workbench state lives in Meridian rather than in a specific app session. |

Meridian is focused on management rather than intelligence: browser access, machine switching, project switching, and resumable tasks.

## Interface Preview

<p align="center">
  <img src="UI.png" alt="Meridian web console interface preview">
</p>

## Quick Start

Docker Compose is the recommended deployment path. It starts PostgreSQL, starts the backend, applies database migrations automatically, builds runner artifacts, and serves the web UI.

```bash
git clone https://github.com/Wangzx233/Meridian.git
cd Meridian
docker compose up -d --build
```

Open:

```text
http://<server-ip>:18080
```

For a local trial on the same machine, `http://127.0.0.1:18080` also works. Compose listens on all addresses by default, so a server install is normally reachable through the server IP. The first browser visit starts the initial admin setup.

To customize the port, database password, external database, or auth settings:

```bash
cp .env.example .env
vi .env
docker compose up -d --build
```

Use HTTPS behind a reverse proxy for shared or internet-facing deployments. If Meridian is only exposed through a local reverse proxy, set `MERIDIAN_HTTP_BIND=127.0.0.1` in `.env`.

## Connect The First Machine

Install Codex CLI on every machine that will execute tasks. Then use the installer shown in Meridian:

1. Open the web UI and create or select a machine.
2. Click the runner install button in the top-right corner.
3. Set the Control URL to an address reachable from the target machine.
4. Copy the Linux, macOS, or Windows command shown by the UI and run it on the target machine.
5. Create a project under that machine and set `workdir` to the real project directory.

Do not use `127.0.0.1` for a remote machine unless Meridian runs on that same machine. Docker Compose and source deployments both provide the runner files used by the install endpoints, so download commands normally do not need to be written by hand.

## Common Workflow

1. Open the web UI.
2. Create a machine and install its runner from the top-right install menu.
3. Create a project under that machine and set the real `workdir`.
4. Create a task.
5. Send one instruction per turn.
6. Use Output, Terminal, and Files to inspect project and run state.
7. Continue adding turns until the work is complete.
8. Mark the task done manually.

## Other Deployment Paths

Source deployment is available when Docker Compose should not run the whole app stack. The backend applies migrations on startup by default, so a first deployment does not need a separate migration command. The runner is still connected through the installer shown in the top-right menu.

Source deployment, external databases, environment variables, reverse proxy notes, and Windows notes are documented in [Deployment Guide](docs/deployment.md).

## Capabilities

| Capability | Description |
| --- | --- |
| Multi-machine control | Manage machines that run the device agent and track their connection state. |
| Real project directories | Each project points to a real working directory on one machine. |
| Long-lived tasks | A task can span many Codex turns; a successful run does not complete the task. |
| Codex session resume | Store the Codex CLI session id and resume later turns in the same task. |
| Explicit context | Users manually attach small, visible context items. |
| Live run output | Stream Codex run events from the device agent back to the web console. |
| Project tools | Browse files, make lightweight edits, and run project-local terminal commands. |
| Runner distribution | Serve Linux, macOS, and Windows runner installers from the backend. |

## Architecture

```text
Browser UI
  -> Go backend control plane
  -> PostgreSQL task/run/event store

Go backend control plane
  <-> device agent WebSocket
  <-> target device agent
  -> local Codex CLI in the project workdir
```

## Current Limitations

- Authentication is a simple login gate; there is no self-registration or fine-grained permission model.
- Runner install endpoints are intended for trusted environments.
- Runner artifacts are not signed.
- Codex CLI must be installed separately on runner machines.
- Context is manually selected; v1 does not recommend or inject context automatically.
- A successful Codex run does not complete a task. The task must be marked done manually.

## Related Documents

- [Deployment Guide](docs/deployment.md): source deployment, external databases, environment variables, and reverse proxy setup.
- [Contributing](CONTRIBUTING.md): local development setup, checks, and PR expectations.
- [Security Policy](SECURITY.md): security boundaries, vulnerability reports, and deployment guidance.
- [Requirements](docs/requirements.md): product requirements and scope.
- [Architecture](docs/architecture.md): control plane, device agent, and data model.
- [API Contract](docs/api-contract.md): HTTP, SSE, and WebSocket protocol.
- [Release Checklist](docs/release-checklist.md): release preparation.
