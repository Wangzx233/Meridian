# Codex Execution Guide

Use this document when a task affects Codex invocation, run creation, task
states, context handling, or prompt construction.

## Runner Execution Model

The runner invokes Codex CLI directly with argv-style process execution, not by
constructing a shell command string.

New session:

```bash
codex --cd <project_workdir> exec --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --json -
```

Resume existing session:

```bash
codex --cd <project_workdir> exec resume --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --json <session_id> -
```

The first command starts a new Codex session. The second continues an existing
task session. Store the Codex session id returned or observed from the run and
associate it with the task.

Optional Codex settings are passed through to the real CLI before `exec`, for
example:

```bash
--model gpt-5.5 --config model_reasoning_effort="high" --config service_tier="fast"
```

Prompt construction belongs in the control plane. The runner receives the final
prompt as opaque text, writes it to Codex stdin, streams Codex JSONL/stdout/stderr
events back, and reports the final status.

## Task Lifecycle

Task states:

- `open`: created but no Codex turn is running.
- `running`: a Codex turn is queued or currently executing.
- `waiting_user`: the latest Codex turn ended and the user can continue.
- `done`: the user manually marked the task complete.
- `archived`: hidden from normal active views.

Run states:

- `queued`
- `running`
- `succeeded`
- `failed`
- `canceled`

A successful run does not mean the task is done. Only explicit user action marks
a task `done`.

The system must enforce at most one active run per task, where active means
`queued` or `running`. This invariant is per task, not per project. Multiple
tasks may run against the same project at the same time.

## Context Policy

First-version context is manual only.

- Context items can be global, server-scoped, project-scoped, or task-scoped.
- The user chooses which context items are attached to each run.
- A context item selected for a previous run must not be automatically resent on
  later runs unless the user selects it again.
- Run creation snapshots selected context so historical runs remain auditable.
- Do not store plaintext secrets in context items.
- Do not auto-attach logs, environment files, or repository-wide summaries.

## Prompt Templates

New session prompt:

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

Resume prompt:

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
