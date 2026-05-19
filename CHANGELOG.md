# Changelog

All notable public release changes are recorded here.

## v0.1.0 - 2026-05-19

Initial public release of Meridian.

### Added

- Web console for managing Codex CLI work across machines, projects, and long-lived tasks.
- Device agent that connects to the control plane and runs the local Codex CLI in real project directories.
- Task runs with live output streaming, run history, Codex session resume, and manual task completion.
- Explicit user-selected context, task memory summaries, project file browsing, lightweight file editing, and project-local terminal commands.
- Docker Compose deployment with bundled PostgreSQL, automatic backend migrations, frontend Nginx service, and runner install artifacts.
- In-app runner installation commands for Linux, macOS, and Windows.
- Runner artifact build scripts for Linux, macOS, and Windows targets.
- GitHub Actions CI for Go tests, Go vet, frontend build, and runner artifact builds.
- GitHub Release workflow that publishes runner binaries and `SHA256SUMS.txt`.
- Public documentation for quick start, deployment, architecture, API contract, contribution, release, and security.

### Notes

- Meridian is intended for trusted self-hosted environments.
- Codex CLI must be installed separately on machines that execute tasks.
- Runner artifacts are published with checksums but are not signed yet.
- The first public version uses a simple login gate and does not provide self-registration or fine-grained permissions.
- Context selection is manual; Meridian does not automatically recommend or inject hidden context.
- A successful Codex run does not complete a task. Tasks are marked done manually.
