# Release Checklist

Use this checklist for each public release.

## 1. Prepare

- Confirm the release scope is aligned with `AGENTS.md`.
- Review merged changes since the previous tag.
- Update `README.md`, `docs/api-contract.md`, and `docs/architecture.md` if
  setup, operations, APIs, runner protocol, or deployment behavior changed.
- Confirm new migrations are ordered and safe to apply once.
- Confirm no generated logs, local artifacts, `.env` files, or secrets are
  staged.

## 2. Verify Locally

Run from the repository root:

```bash
go test ./...
go vet ./...
```

Run from `frontend/`:

```bash
npm ci
npm run build
```

Build runner artifacts from the repository root:

```bash
sh ./scripts/build-runner-artifacts.sh
```

Expected runner outputs:

- `artifacts/runner/runner-windows-amd64.exe`
- `artifacts/runner/runner-linux-amd64`
- `artifacts/runner/runner-linux-arm64`
- `artifacts/runner/runner-darwin-amd64`
- `artifacts/runner/runner-darwin-arm64`

## 3. Smoke Test

- Start the backend against a disposable PostgreSQL database with
  `RUNNER_ARTIFACT_DIR` pointing at the built artifacts.
- Start the frontend against that backend.
- Create or select a server, project, and task.
- Connect a runner.
- Run one Codex turn and confirm events stream to the UI.
- Confirm a successful run leaves the task in `waiting_user`, not `done`.
- Confirm manual task completion still requires the user to mark the task done.

## 4. Tag And Publish

Create a version tag on the public GitHub release branch and push it to GitHub:

```bash
git tag v0.1.0
git push github v0.1.0
```

The GitHub release workflow runs the quality gate, builds runner artifacts, and
publishes a GitHub release with runner binaries and `SHA256SUMS.txt`.

Private mirrors can keep the same tag for traceability, but GitHub is the public
release surface.

For a manual republish of an existing tag, run the `Release` workflow from
GitHub Actions and provide the existing tag name.

## 5. Post-Release

- Check the published release assets and checksums.
- Deploy with the normal deployment workflow.
- Confirm the backend applied migrations during startup.
- Reinstall or self-update connected runners when the runner changed.
- Verify the public site login, runner install commands, and one Codex turn.
