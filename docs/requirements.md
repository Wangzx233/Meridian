# Meridian Requirements

## 1. Summary

Meridian is a web application for managing multi-turn agent work sessions across
multiple servers and projects.

The system exists to answer one practical need:

> From a browser, choose the correct project on the correct server, send a
> focused instruction to the current executor, optionally attach a small amount
> of manual context, continue the same work session across many turns, and keep
> an auditable task record.

The first version should make the user's existing Codex CLI workflow easier to
manage without changing Codex's role or replacing Codex with a custom agent.

## 2. Product Positioning

This is a project task workbench, not a generic agent workspace.

The primary user already relies on Codex to do most implementation work. The
system should therefore:

- Help place Codex in the right project and directory.
- Preserve Codex CLI behavior and session continuity.
- Store task history, selected context, outputs, and summaries.
- Avoid adding large, noisy, automatic context that distracts Codex.

## 3. Goals

- Manage projects that live on different servers.
- Start and continue Codex CLI sessions from a web UI.
- Model tasks as long-lived work objectives, not one-shot jobs.
- Support repeated user-to-Codex turns inside one task.
- Let the user manually select context items for each turn.
- Stream Codex output back to the browser.
- Persist run history and final messages.
- Preserve Codex session ids so tasks can be resumed.
- Let the user manually mark tasks complete.
- Allow task memories or summaries to be saved after completion.

## 4. Non-Goals

The first version must not try to become:

- A full IDE.
- A general AI chat app.
- A replacement for Codex.
- A custom OpenAI API agent framework.
- A full DevOps/monitoring/deployment platform.
- A complex knowledge graph.
- A multi-model orchestration platform.
- A system that automatically recommends or injects context.

## 5. Core User Flow

1. User opens the web console.
2. User selects a project.
3. User creates or opens a task.
4. User writes the next instruction for Codex.
5. User optionally checks context items to attach to this turn.
6. User sends the turn.
7. Control plane creates a run.
8. Runner receives the run.
9. Runner changes into the configured project working directory.
10. Runner invokes Codex CLI.
11. Codex output streams back to the browser.
12. When Codex finishes, the run is marked succeeded, failed, or canceled.
13. The task returns to `waiting_user`.
14. User sends additional turns until satisfied.
15. User manually marks the task done.
16. User may save a task memory or summary.

## 6. Task Semantics

A task is a long-lived container for work.

Examples:

- Fix order export timeout.
- Add billing retry logic.
- Investigate production API errors.
- Refactor report query performance.

A task may require many Codex turns:

- Ask Codex to inspect and plan.
- Ask Codex to implement.
- Ask Codex to run tests.
- Ask Codex to fix failures.
- Ask Codex to summarize changes.

A task is not completed automatically. The user must mark it done.

## 7. Turn / Run Semantics

A run, also called a turn, is one user instruction and one Codex CLI execution.

Sending a turn creates a run in `queued` status and immediately moves the task
to `running`. A task must not have more than one `queued` or `running` run at
the same time, even if the user clicks twice, refreshes the browser, or a runner
reconnects.

Each run records:

- User message.
- Selected context items.
- Generated prompt sent to Codex.
- Execution mode: `new` or `resume`.
- Codex session id.
- Raw event stream or output.
- Final Codex message.
- Status.
- Start and end time.

Run success only means that Codex finished that turn. It does not imply the task
is complete.

## 8. Codex Session Continuity

Each task should normally map to one Codex session.

The first run of a task starts a new session:

```bash
codex --cd <project_workdir> exec --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --json -
```

Subsequent runs resume the same session:

```bash
codex --cd <project_workdir> exec resume --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --json <session_id> -
```

The UI should expose two clear modes when appropriate:

- Continue existing session.
- Start a new session for this task.

The default for an existing task should be to continue the existing session.

The system should use a simple first-version session id rule:

- Store the Codex session id on each run when it is observed from Codex output.
- After the first successful run of a task, copy the observed session id to the
  task.
- If no session id is available for a task, the UI must not silently attempt a
  resume. It should ask the user to start a new session for that task.

## 9. Context Handling

First version context is manual only.

The system should list available context items and allow the user to check the
items to attach to the current turn. It should not automatically recommend,
rank, or inject context.

Context item types:

- `project_rule`
- `task_summary`
- `decision`
- `log_snippet`
- `verify_command`
- `file_hint`
- `note`

Context items can be scoped globally, to a server, to a project, or to a task:

- `global` scoped items are available across all servers and projects.
- `server` scoped items are available to all projects on one server.
- `project` scoped items are available to all tasks in the project.
- `task` scoped items are available only inside one task.

Context should be attached per run. A context item selected in a previous run
should not be automatically re-sent in later resumed runs unless the user selects
it again.

Each run should preserve a snapshot of the selected context content, title, type,
and order. This keeps old run records auditable even if a context item is edited
later.

This avoids noisy repeated prompts because Codex already has the previous
session context.

## 10. Prompt Construction

The generated prompt should be short and explicit.

### New Session Template

```text
Current task:
<task title and description>

User instruction for this turn:
<message>

Selected context:
<manually selected context items>

Instructions:
- First inspect the current repository before deciding.
- Historical context is background only.
- Current repository code is authoritative if it conflicts with context.
- Complete this turn and explain changes, verification, and next steps.
```

### Resume Session Template

```text
Continue the current task.

User instruction for this turn:
<message>

Additional context selected for this turn:
<manually selected context items>

Instructions:
- Continue from the existing Codex session.
- Do not repeat already completed work unless needed.
- Current repository code is authoritative.
```

## 11. Server Requirements

A server represents a machine where the runner and Codex CLI can execute.

Server fields:

- Name.
- Runner identity.
- Status.
- Last heartbeat time.
- Optional tags.

The control plane should not use direct SSH fan-out as the primary execution
model. Each server should run a runner that connects back to the control plane.

## 12. Project Requirements

A project represents a configured working directory on a server.

Project fields:

- Name.
- Server id.
- Work directory.
- Default branch.
- Optional rules path, usually `AGENTS.md`.
- Optional default verification commands.

The runner must execute Codex in the configured working directory.

## 13. Runner Requirements

The runner is a small process installed on each server.

It should:

- Register with the control plane.
- Maintain heartbeat.
- Receive run requests.
- Invoke Codex CLI.
- Stream output or JSONL events to the control plane.
- Report run completion status.
- Support cancellation.
- Return errors clearly.

It should not:

- Call OpenAI APIs directly in the first version.
- Implement its own agent loop.
- Summarize source code.
- Recommend context.
- Store all task data permanently.

## 14. UI Requirements

The UI should have these screens:

- Login.
- Server list.
- Project list.
- Task list per project.
- Task session page.
- Context item management.
- Run detail view.

The task session page is the most important screen. It should show:

- Project and server.
- Task title and status.
- Codex session id when known.
- Previous turns.
- Current message input.
- Manual context checkbox list.
- Send button.
- Cancel current run button.
- Mark task done button.
- A compact language toggle for English and Chinese UI labels.

## 15. Task Memory

When marking a task done, the user may save a task memory. The memory should be
lightweight: a short note is enough, and structured fields are optional aids
rather than required sections.

Suggested memory fields:

- Problem.
- Root cause.
- Changes.
- Files touched.
- Decisions.
- Verification.
- Related tasks.
- Source commit.
- Stale conditions.

The user may write this manually. The UI may also offer an explicit "draft
memory" action that asks Codex, through the normal runner/Codex CLI path, to
produce a parseable draft for the form. Drafting must be visible as a normal
turn, must not save the memory automatically, and must not inject the memory into
future context automatically.

The task session UI may group manual context selection and task memory summary
as two views under the context panel. This keeps completion controls close to
the context workflow without shrinking them into a constrained footer.

For the smallest MVP, this can be implemented as a simple user-written note plus
optional fields before introducing project-specific memory templates.

## 16. Safety Requirements

The first version should keep safety simple:

- Do not store plaintext secrets in context items.
- Do not auto-attach logs or environment files.
- Do not automatically run production deployment commands.
- Default workbench runs bypass Codex CLI sandbox and approval prompts so
  non-interactive server tasks do not get stuck waiting for manual execution.
  Operators can set `CODEX_BYPASS_APPROVALS_AND_SANDBOX=false` to restore Codex
  sandbox and approval behavior.
- Make the configured project directory visible before each run.
- Treat all historical context as less authoritative than current code.

## 17. MVP Scope

The MVP is complete when the user can:

- Add a server with an active runner.
- Add a project pointing to a server directory.
- Create a task.
- Send a first Codex turn.
- Resume the same task with more turns.
- Manually attach context per turn.
- Watch Codex output stream in the browser.
- See run history.
- Mark the task done.
- Save a simple task summary or memory.

## 18. Success Criteria

The product is successful if it preserves the feel and capability of direct
Codex CLI usage while adding:

- Clear project/task organization.
- Browser access from different work computers.
- Reliable resume of Codex sessions.
- Explicit context selection.
- Persistent task and run records.
