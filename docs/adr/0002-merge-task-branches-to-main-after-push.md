# 0002. Merge Task Branches To Main After Push

Date: 2026-05-18

## Status

Accepted

## Context

Codex Task Workbench supports multiple tasks editing the same repository at the
same time. Task-scoped branches and worktrees reduce local conflicts, but if
completed work only remains on task branches, `main` stops representing the
integrated project state.

The user expects automatic repository updates to make completed task work
available from `main`.

## Decision

Completed task changes should be pushed to the task branch first, then merged
into `main`, then `main` should be pushed.

If the work is already committed on `main`, push `main` directly. If merge or
push conflicts occur, preserve both sides, resolve only current-task conflicts
when ownership is clear, rerun relevant checks, and push again. If ownership is
unclear, stop and report the conflicting paths.

## Consequences

- Task branches remain useful as isolation, backup, and review points.
- `main` remains the default integrated branch for completed task work.
- Agents must report both task-branch push status and `main` push status.
- Conflicts may happen during the merge-to-main step, so merge handling must be
  explicit and non-destructive.

## Related

- [Concurrent tasks guide](../agent-guides/concurrent-tasks.md)
- [Development workflow guide](../agent-guides/development-workflow.md)
