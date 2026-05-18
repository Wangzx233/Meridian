# Contributing

Meridian is intentionally narrow: it is a web console for managing project-scoped
agent work sessions. Keep changes aligned with the product scope in
`AGENTS.md`, `README.md`, and the design docs under `docs/`.

## Development Setup

Prerequisites:

- Go 1.25 or newer.
- Node.js 20.19 or newer, plus npm.
- PostgreSQL 16 or newer for integration work.
- Codex CLI installed on machines that run the runner.

Install frontend dependencies before frontend work:

```bash
cd frontend
npm ci
```

For backend and runner development, standard Go module commands work from the
repository root.

## Required Checks

Run these before opening or merging a pull request:

```bash
go test ./...
go vet ./...
cd frontend
npm run build
```

Build runner artifacts when runner behavior, install scripts, release packaging,
or backend artifact-serving behavior changes:

```bash
sh ./scripts/build-runner-artifacts.sh
```

GitHub Actions runs the same core checks on pull requests and pushes to `main`.

## Pull Request Guidelines

- Keep the scope small and tied to one task objective.
- Prefer simple, auditable workflow over automation.
- Do not replace Codex CLI behavior with a custom agent runtime.
- Keep context explicit and user-selected.
- Include tests for backend or runner behavior when the change affects state,
  command execution, authentication, runner coordination, or persistence.
- Update docs when changing setup, environment variables, APIs, deployment, or
  release behavior.

## Database Changes

Schema changes belong in `db/migrations`. Add a forward migration and, when
practical, a matching rollback migration. Keep migrations deterministic and safe
to run once in production.

## Runner Changes

The runner executes on user-controlled servers and invokes the local Codex CLI.
Avoid hidden automation, implicit context collection, and broad host access
beyond the documented runner responsibilities.

When changing runner protocol behavior, update:

- `docs/api-contract.md`
- `docs/architecture.md` when the system shape changes
- `README.md` and `docs/deployment.md` if setup or operations change

## Release Process

Use `docs/release-checklist.md` for release preparation. The GitHub release
workflow publishes tagged releases and uploads runner binaries plus checksums.
