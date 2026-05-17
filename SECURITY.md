# Security Policy

Meridian is intended for trusted project environments. The backend, runner install
endpoints, and runner WebSocket should be exposed only behind transport security
and appropriate network controls.

## Supported Versions

Security fixes are applied to the current `main` branch until formal versioned
support is introduced. Tagged releases are snapshots, not long-term support
branches.

## Reporting A Vulnerability

Do not open a public issue with exploit details, secrets, tokens, private URLs,
or runner logs that contain sensitive project data.

Report vulnerabilities privately to the maintainers through the repository host's
private security advisory feature when available. If that is not available, send
a minimal private report to the repository owner with:

- Affected component: backend, runner, frontend, installer, deployment, or docs.
- Reproduction steps.
- Expected impact.
- Any relevant version, commit, or deployment details.

Expect an acknowledgement within 7 days. The fix path may include a private
patch, a coordinated release, and public disclosure after users have had time to
upgrade.

## Secrets And Sensitive Data

- Never commit `.env` files, access tokens, session secrets, database URLs, or
  runner tokens.
- Treat Codex run output, runner logs, and terminal transcripts as potentially
  sensitive project data.
- Rotate `WORKBENCH_AUTH_SESSION_SECRET` and `WORKBENCH_RUNNER_TOKEN` if either
  value is exposed.
- Public deployments should use HTTPS and set `WORKBENCH_AUTH_COOKIE_SECURE=true`.

## Runner Risk Model

The runner can execute Codex CLI and project-local terminal commands in the
configured working directory. Install runners only on machines where that access
is acceptable, and do not share one `runner_id` across multiple machines.

The first version intentionally avoids multi-tenant permission boundaries. Do
not use it as an untrusted shared execution service.
