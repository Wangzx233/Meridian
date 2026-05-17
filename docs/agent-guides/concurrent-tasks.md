# Concurrent Tasks Guide

Use this document when several Workbench tasks may read or write the same
repository, or when a commit, push, rebase, or conflict is involved.

This repository is expected to be edited by multiple Workbench tasks at the same
time. Concurrent reading is normal, and concurrent writing must be treated as a
first-class workflow. Automatic pushing is required unless the user explicitly
says not to, so conflicts are expected and must be handled deliberately.

## Preferred Isolation Model

For any task that writes code, prefer one isolated Git worktree or clone per
active task:

- Create a task-scoped branch from the latest `origin/main`.
- Use a branch name such as `task/<task-id>-<short-slug>`.
- Point the Workbench project for that task at the task-specific worktree.
- Keep the original shared checkout available for review and lightweight
  inspection only.
- Push the task branch with `git push -u origin HEAD`.

This keeps each task on its own working tree, index, and branch. The remote
repository remains the coordination point.

## Same-Worktree Fallback

If multiple tasks must write inside the same physical worktree:

- Start by reading `git status --short --branch`.
- Treat all pre-existing modified, staged, or untracked files as owned by
  someone else unless the current task clearly created them.
- Edit only files required for the current task.
- Do not run broad formatters or refactors across unrelated files.
- Do not use `git add -A`, `git add .`, or `git commit -a`.
- Stage and commit only task-owned paths.
- If other paths are already staged, use a path-limited commit such as
  `git commit --only <paths>`.
- If a file contains both current-task edits and unrelated edits, inspect the
  diff carefully and stage only the intended hunks when practical.
- If unrelated changes make the task impossible to complete safely, stop and
  report the exact paths and conflict.

## Push Rejections

If push is rejected because the remote branch moved:

- Fetch the remote branch.
- Rebase or merge the current task branch onto the new remote tip when the
  working tree is isolated or only current-task changes are present.
- Resolve conflicts only after understanding both sides.
- Rerun relevant checks after conflict resolution.
- Push again.

If the rejection or conflict involves unrelated local work, another task's
owned files, or user changes in the same worktree, do not choose a winner
blindly. Preserve both sides and report the conflicting paths, current branch,
remote branch, and recommended next action.

## Conflict Handling Policy

Conflicts are acceptable; silent overwrites are not.

- If the conflict is entirely within current-task files, resolve it, test it,
  commit the resolution, and push.
- If the conflict crosses task ownership, pause and surface the conflict.
- Prefer small commits and narrow ownership so conflicts are easy to review.
- Preserve behavior from both sides unless the task explicitly requires removing
  one side.
- Update docs and tests with the final resolved behavior.

## Product Design For Concurrency

When implementing Workbench features, assume several tasks may target the same
project at once:

- Keep the active-run lock per task, not globally per project.
- Make server, project, task, run, branch, commit, and dirty-worktree state
  visible where it affects user decisions.
- Store enough run metadata to audit which task produced which commit.
- Prefer future support for task-scoped worktrees, branch names, base commits,
  push status, and conflict status over broad project-level locking.
- Do not add automatic context injection to solve Git conflicts. Conflict
  handling should stay explicit and source-visible.
