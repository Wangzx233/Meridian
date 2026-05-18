# Meridian Agent Map

This file is intentionally short. Treat it as a map, not a full manual.
Open only the referenced document that matches the current task. Context should
stay small, explicit, and source-visible.

## Always Follow

- Inspect the current repository before deciding, including
  `git status --short --branch`.
- Current code is authoritative. Historical summaries are background only.
- Do not revert, overwrite, or commit unrelated user or task changes.
- Keep changes scoped to the current task.
- Update docs when behavior, APIs, setup, deployment, or workflow changes.
- Commit and push current-task changes, then merge them into `main` and push
  `main`, unless the user explicitly says not to.
- Multiple Workbench tasks may read and write this project at the same time.
  Expect conflicts and handle them explicitly.

## Product Boundary

This project is a web console for managing project-scoped Codex CLI work
sessions across servers. It is not a general AI chat app, full IDE, DevOps
platform, or custom agent framework.

First-version constraints:

- The runner invokes the real local Codex CLI.
- The runner does not call OpenAI APIs directly.
- Context is manual and user-selected.
- A successful Codex run does not mark a task done.
- File browsing/editing, terminal, and short commands are trusted workbench
  helpers, not a full IDE or remote administration platform.

Details: [Product Scope](docs/agent-guides/product-scope.md)

## Documentation Map

| If the task is about | Open |
| --- | --- |
| Product intent, scope, concepts, non-goals | [docs/agent-guides/product-scope.md](docs/agent-guides/product-scope.md) |
| Codex execution, task/run lifecycle, context policy | [docs/agent-guides/codex-execution.md](docs/agent-guides/codex-execution.md) |
| Control-plane, runner, streaming, data model | [docs/architecture.md](docs/architecture.md) |
| HTTP API, SSE, runner WebSocket protocol | [docs/api-contract.md](docs/api-contract.md) |
| Implementation workflow, checks, docs update rules | [docs/agent-guides/development-workflow.md](docs/agent-guides/development-workflow.md) |
| Documentation structure, map pages, ADR rules | [docs/agent-guides/documentation-map.md](docs/agent-guides/documentation-map.md) |
| Concurrent tasks, Git conflicts, commit and push rules | [docs/agent-guides/concurrent-tasks.md](docs/agent-guides/concurrent-tasks.md) |
| Quick start and local setup | [README.md](README.md) |
| Deployment details and environment variables | [docs/deployment.md](docs/deployment.md) |
| Contribution and CI expectations | [CONTRIBUTING.md](CONTRIBUTING.md) |
| Architectural decisions and rationale | [docs/adr/README.md](docs/adr/README.md) |
| Release preparation | [docs/release-checklist.md](docs/release-checklist.md) |
| Security posture | [SECURITY.md](SECURITY.md) |

## Repository Map

- `backend/`: Go control-plane API.
- `runner/`: Go runner agent.
- `frontend/`: React, TypeScript, Vite UI.
- `db/migrations/`: PostgreSQL migrations.
- `docs/`: requirements, architecture, API, deployment, release, and agent guides.
- `scripts/`: local helper scripts.

## Concurrency Default

For writing tasks, prefer a task-specific Git worktree and branch from the
latest `origin/main`. Push the task branch first, then merge it into `main` and
push `main`. If forced to use the same worktree as other tasks, stage and commit
only current-task paths. Do not use `git add .`, `git add -A`, or
`git commit -a`.

When a push is rejected or a conflict appears, preserve both sides, identify
the owned paths, resolve only current-task conflicts, rerun relevant checks, and
push again. If conflict ownership is unclear, report the exact paths and stop.

Full workflow: [Concurrent Tasks](docs/agent-guides/concurrent-tasks.md)

## Final Report

When finishing a turn, report:

- Files changed.
- Verification performed.
- Commit hash.
- Push status.
