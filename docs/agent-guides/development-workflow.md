# Development Workflow Guide

Use this document when implementing code or docs changes.

## Before Editing

- Inspect the current repository.
- Check `git status --short --branch`.
- Read the files that define the current behavior.
- Do not rely only on old task summaries.
- Identify unrelated modified, staged, or untracked files before editing.

## Implementation Rules

- Keep edits tightly scoped to the current task.
- Do not revert or overwrite unrelated user or task changes.
- Prefer existing code patterns over new abstractions.
- Keep the runner thin; do not move agent logic into it.
- Preserve Codex CLI semantics unless a documented runner flag intentionally
  changes them.
- Keep prompt and context behavior explicit and auditable.
- Use Go for backend and runner code. Use Node only where required by frontend
  tooling.

## Docs Rules

Update docs when changing:

- Setup or environment variables.
- HTTP APIs, SSE, or runner WebSocket protocol.
- Database schema or migrations.
- Deployment or release behavior.
- Runner install, artifact, capability, or update behavior.
- Product scope or workflow expectations.

Prefer map-style documentation:

- Keep entry documents short and link to focused documents for details.
- Organize docs by task intent before repository module.
- Avoid duplicating canonical details. Link to the authoritative source instead.
- Keep the normal tree depth to three levels: entry map, focused guide, detailed
  reference or ADR.
- Put a "Use this document when..." sentence near the top of focused guides.
- Record durable technical decisions as ADRs instead of burying rationale in
  operational guides.

For the full documentation structure rules, read
[Documentation Map](documentation-map.md).

## Recommended Checks

Run checks relevant to the touched files.

Backend and runner:

```powershell
go test ./...
go vet ./...
```

Frontend:

```powershell
cd frontend
npm run build
```

Runner artifacts, when runner behavior, install scripts, release packaging, or
backend artifact-serving behavior changes:

```powershell
.\scripts\build-runner-artifacts.ps1
```

Docs-only changes usually need `git diff --check` plus a careful diff review.

## Commit And Push

- Commit completed current-task changes unless there are no repository changes
  for the task.
- Include only current-task changes in the commit.
- Use clear commit messages that name the task outcome.
- Push the current task branch automatically after committing unless the user
  explicitly says not to.
- After the task branch push succeeds, merge the task branch into `main` and
  push `main`.
- If the current work is already on `main`, push `main` directly.
- Never force-push `main` or any shared branch.
- Do not use destructive cleanup commands for unrelated work.
- If merge or push fails because of conflicts or a moved remote, preserve both
  sides, resolve only current-task conflicts when ownership is clear, rerun
  relevant checks, and push again.

Final responses should report changed files, verification, commit hash, and push
task-branch push status, `main` merge commit or fast-forward status, and `main`
push status.
