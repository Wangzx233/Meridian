# Documentation Map Guide

Use this document when adding, splitting, merging, or reorganizing repository
documentation.

The documentation tree should behave like the product itself: keep context
small, explicit, and selected for the current task. `AGENTS.md` is a context
router, not a context warehouse.

## Design Rules

- Use map pages to route readers to the right focused document.
- Organize by task intent first, and by code module only when that is the
  clearest user path.
- Keep the normal hierarchy to three levels: entry map, focused guide, detailed
  reference or ADR.
- Avoid map pages inside map pages unless a real workflow needs the extra level.
- Keep a map page small enough to scan in one screen when practical.
- Do not copy canonical content between docs. Link to the source of truth.
- Put "Use this document when..." at the top of focused guides.
- Keep operation steps and decision rationale separate.

## Topic Types

Use these lightweight topic types when deciding where content belongs:

- `concept`: product intent, boundaries, vocabulary, and mental models.
- `procedure`: steps for development, release, Git, operations, or recovery.
- `reference`: APIs, protocols, data shapes, environment variables, commands.
- `troubleshooting`: symptoms, causes, checks, and fixes.
- `decision`: long-lived architectural or workflow rationale stored as an ADR.

A page may contain more than one type, but it should have one primary job. If a
page starts serving several unrelated jobs, split it and update the nearest map.

## Current Tree

```text
AGENTS.md
  docs/agent-guides/
    product-scope.md
    codex-execution.md
    development-workflow.md
    documentation-map.md
    concurrent-tasks.md
  docs/
    requirements.md
    architecture.md
    api-contract.md
    deployment.md
    release-checklist.md
    adr/
README.md
CONTRIBUTING.md
SECURITY.md
```

## Source Of Truth Rules

- Product boundary and task semantics: `docs/agent-guides/product-scope.md` and
  `docs/requirements.md`.
- Codex invocation and run lifecycle: `docs/agent-guides/codex-execution.md`.
- Control-plane and runner architecture: `docs/architecture.md`.
- API, SSE, and runner WebSocket contract: `docs/api-contract.md`.
- Deployment, external databases, and environment variables:
  `docs/deployment.md`.
- Development workflow and checks: `docs/agent-guides/development-workflow.md`
  and `CONTRIBUTING.md`.
- Concurrent task and Git behavior: `docs/agent-guides/concurrent-tasks.md`.
- Stable decision rationale: `docs/adr/`.

If two documents need the same fact, keep the fact in one authoritative document
and link to it from the other.

## ADR Rules

Use an ADR when:

- There are two or more plausible engineering options.
- The reason for a choice matters to future tasks.
- A workflow rule would otherwise be repeated in several guides.
- A product boundary decision needs durable context.

Do not use ADRs for temporary status updates, release notes, or routine task
summaries.

ADR filename format:

```text
docs/adr/NNNN-short-kebab-title.md
```

ADR body format:

```text
# NNNN. Title

Date: YYYY-MM-DD

## Status

Accepted | Superseded | Proposed

## Context

## Decision

## Consequences

## Related
```

## Navigation Tests

Before finishing a documentation restructure, test the map with real questions:

- "I need to change runner WebSocket messages. Where do I go?"
- "I need to resolve a push rejection. Where do I go?"
- "I need to add a new context type. Where do I go?"
- "I need to understand why the project uses map-style agent docs. Where do I
  go?"

If the answer is not obvious from `AGENTS.md`, update the map.

## External Practices

This guide adapts these public documentation practices:

- Diataxis: tutorials, how-to guides, reference, and explanation organized
  around user needs.
- GitLab CTRT: concept, task, reference, and troubleshooting topic types.
- GitHub Docs content model: map topics for routing, articles for focused
  content, and reuse by linking to the source of truth.
- Google Cloud ADR guidance: record options, requirements, decisions, and
  rationale close to the codebase.

References:

- https://diataxis.fr/
- https://docs.gitlab.com/development/documentation/topic_types/
- https://docs.github.com/en/contributing/style-guide-and-content-model/about-the-content-model
- https://docs.cloud.google.com/architecture/architecture-decision-records
