# 0001. Map-Based Agent Documentation

Date: 2026-05-16

## Status

Accepted

## Context

Meridian is designed around small, explicit, user-selected context.
The same principle should apply to repository instructions. A large
`AGENTS.md` consumes context on every task, repeats information that belongs in
more specific documents, and makes concurrent task work harder because agents
must scan unrelated policy before finding the relevant rule.

The project also expects multiple tasks to read and write the repository at the
same time. Documentation needs to route each task to the smallest useful source
of truth so unrelated tasks do not all depend on one oversized file.

## Decision

`AGENTS.md` is a map, not a manual. It keeps only always-on rules, product
boundaries, and links to focused documents.

Focused details live in `docs/agent-guides/`, `README.md`, `CONTRIBUTING.md`,
`SECURITY.md`, and the existing architecture and API documents. Durable
rationale lives in `docs/adr/`.

Documentation is organized primarily by task intent:

- Product scope and concepts.
- Codex execution and context policy.
- Development workflow.
- Documentation structure.
- Concurrent task and Git behavior.
- API, protocol, architecture, setup, security, and release references.

## Consequences

- Agents spend less default context on repository-wide instructions.
- New or changed rules need a clear source-of-truth document.
- `AGENTS.md` must be maintained as the routing map whenever guides are added,
  removed, or renamed.
- Some tasks may need to open two documents: the map and one focused guide.
- Long-lived "why" explanations should be added as ADRs instead of expanding
  operational guides.

## Related

- [Agent map](../../AGENTS.md)
- [Documentation map guide](../agent-guides/documentation-map.md)
- [Concurrent tasks guide](../agent-guides/concurrent-tasks.md)
