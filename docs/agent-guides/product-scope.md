# Product Scope Guide

Use this document when a task may affect product boundaries, feature scope, or
the meaning of core concepts.

## Product Intent

Meridian is a web console for managing project-scoped agent work sessions:

- The user selects a server, project, and task.
- The user writes one instruction per turn.
- The user may manually attach a small set of context items to that turn.
- A runner on the target server invokes the local Codex CLI in the project's
  real working directory.
- The task stays open across multiple Codex turns until the user manually marks
  it done.

## Current Product Shape

The application uses a control-plane plus runner architecture:

- Browser UI: React, TypeScript, Vite, TanStack Query.
- Control plane: Go HTTP API, WebSocket/SSE streaming, PostgreSQL persistence.
- Runner: Go process installed on target servers.
- Database: PostgreSQL migrations under `db/migrations`.

Current capabilities include:

- Browser login and first-run setup.
- Server records matched to connected runner identities.
- Runner install scripts, artifact download, reconnect, heartbeat, capability
  reporting, and self-update for capable runner versions.
- Server-scoped projects with real working directories.
- Long-lived tasks with one Codex instruction per turn.
- Manual context items scoped globally, to a server, to a project, or to a task.
- Run creation, cancellation, event storage, replay, and browser streaming.
- Codex session id capture and resume.
- Optional per-run Codex model, reasoning effort, and service tier overrides.
- Manual image input for Codex turns.
- Project file browsing, file read/write actions, project-local terminal, and
  short project-local command execution through the connected runner.
- In-app run-finished notifications.

## Core Principles

- Do not implement a replacement AI agent.
- Do not call OpenAI APIs from the runner in the first version.
- Do not add automatic context recommendation in the first version.
- Do not assume a task is complete when one Codex run succeeds.
- Preserve Codex CLI behavior as much as possible, except for explicit runner
  flags documented in the execution model.
- Keep context small, explicit, user-selected, and source-visible.
- Treat the current repository code as more authoritative than historical
  context summaries.
- Prefer simple, auditable workflow over hidden automation.

## Non-Goals

Do not build these in the first version unless explicitly requested:

- Automatic context recommendation.
- Knowledge graph.
- Multi-model orchestration.
- Full web IDE.
- Full observability dashboard.
- Automated deployment platform.
- Complex multi-tenant permission system.
- Custom model/tool agent runtime.
- Server-side SSH fan-out as the primary execution model.

File browsing, lightweight file editing, project-local terminal access, and
short project-local commands are workbench helpers for trusted project servers.
They must not expand into a full IDE or remote administration platform.

## Important Concepts

- `Server`: a machine capable of running the runner and Codex CLI.
- `Runner`: the process that connects from a server to the control plane.
- `Project`: a configured working directory on a server.
- `Task`: a real work objective, usually completed over multiple turns.
- `Run` or `Turn`: one user instruction sent to Codex and one Codex execution.
- `ContextItem`: a manually selected snippet such as a rule, note, log,
  previous task summary, verification command, or file hint.
- `CodexSession`: the session id owned by Codex CLI and used for resume.
