# Deployment Guide

Use this document when the README quick start is not enough: external
databases, local-only bind addresses, source deployment details, or Windows
developer notes.

## Docker Compose

The root `docker-compose.yml` is the default self-hosted deployment path.

- Binds the web UI to `0.0.0.0:18080` by default.
- Starts bundled PostgreSQL unless an external database override is used.
- Starts the backend after PostgreSQL is healthy.
- Applies database migrations automatically when the backend starts.
- Builds and serves runner artifacts from the backend image.
- Serves the frontend through Nginx and proxies `/api` plus WebSocket traffic to
  the backend.

Use a local-only bind when another reverse proxy is the only public entrypoint:

```bash
MERIDIAN_HTTP_BIND=127.0.0.1 docker compose up -d --build
```

For persistent configuration:

```bash
cp .env.example .env
vi .env
docker compose up -d --build
```

## External Database

Compose intentionally ignores host-level `DATABASE_URL` so old shell or CI
values cannot override the bundled database connection. Use
`MERIDIAN_DATABASE_URL` for Compose deployments:

```env
MERIDIAN_DATABASE_URL=postgres://meridian:password@db-host:5432/meridian?sslmode=disable
```

If the external database is another Docker container on a separate network,
also set the network name and include the external database Compose overlay:

```env
MERIDIAN_DATABASE_DOCKER_NETWORK=database_net
```

```bash
docker compose --env-file .env \
  -f docker-compose.yml \
  -f docker-compose.external-db.yml \
  up -d --build
```

## Source Deployment

The backend starts migrations by default. The separate migrate command is only
for maintenance workflows where you intentionally want to run migrations before
starting the server.

```bash
sh ./scripts/build-runner-artifacts.sh

cd frontend
npm ci
npm run build
cd ..

DATABASE_URL='postgres://user:password@db-host:5432/meridian?sslmode=disable' \
BACKEND_ADDR='0.0.0.0:8080' \
RUNNER_ARTIFACT_DIR="$PWD/artifacts/runner" \
go run ./backend/cmd/server
```

Serve `frontend/dist` with a static web server or reverse proxy. Proxy `/api`
and WebSocket upgrade traffic to the backend.

To disable automatic migrations:

```bash
MERIDIAN_AUTO_MIGRATE=false go run ./backend/cmd/server
```

To run migrations manually:

```bash
DATABASE_URL='postgres://user:password@db-host:5432/meridian?sslmode=disable' \
go run ./backend/cmd/migrate up
```

Set `MIGRATIONS_DIR` only when migrations are not in the repository default
`db/migrations` directory.

## Runner Artifacts

Linux and macOS contributors should use:

```bash
sh ./scripts/build-runner-artifacts.sh
```

Windows contributors can use the PowerShell equivalent:

```powershell
.\scripts\build-runner-artifacts.ps1
```

Both scripts write:

- `artifacts/runner/runner-windows-amd64.exe`
- `artifacts/runner/runner-linux-amd64`
- `artifacts/runner/runner-linux-arm64`
- `artifacts/runner/runner-darwin-amd64`
- `artifacts/runner/runner-darwin-arm64`

Set `RUNNER_VERSION=<commit-or-tag>` when building artifacts from source if the
web UI should show a release-specific runner version after install or
self-update. Compose image builds set the runner version from
`MERIDIAN_BUILD_COMMIT` automatically.

The web UI's top-right runner install menu uses these artifacts through the
backend install endpoints. Prefer that UI flow over copying endpoint URLs from
documentation.

## Environment Variables

### Compose

| Variable | Default | Purpose |
| --- | --- | --- |
| `MERIDIAN_HTTP_BIND` | `0.0.0.0` | Host address for published Compose ports. |
| `MERIDIAN_HTTP_PORT` | `18080` | Public frontend port. |
| `MERIDIAN_DATABASE_URL` | bundled PostgreSQL | External database URL for Compose. |
| `MERIDIAN_DATABASE_DOCKER_NETWORK` | empty | External Docker network for an external database container. |
| `MERIDIAN_AUTO_MIGRATE` | `true` | Whether the backend applies migrations on startup. |
| `WORKBENCH_AUTH_USERS` | empty | Empty enables browser first-run admin setup. |
| `WORKBENCH_AUTH_SESSION_SECRET` | required when auth users are set | Browser session signing secret. |
| `WORKBENCH_RUNNER_TOKEN` | required when auth users are set | Bearer token for runner installers and runner WebSocket connections. |
| `WORKBENCH_AUTH_COOKIE_SECURE` | `false` | Set `true` when serving over HTTPS. |

### Source Backend

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | required | PostgreSQL connection string. |
| `BACKEND_ADDR` | `:8080` | Backend listen address. |
| `MIGRATIONS_DIR` | `db/migrations` | Migration directory. |
| `RUNNER_ARTIFACT_DIR` | `artifacts/runner` | Directory served by runner install endpoints. |
| `MERIDIAN_AUTO_MIGRATE` | `true` | Whether the backend applies migrations on startup. |
| `CODEX_BYPASS_APPROVALS_AND_SANDBOX` | `true` | Adds Codex CLI bypass sandbox/approval flags. |

### Source Frontend

| Variable | Default | Purpose |
| --- | --- | --- |
| `VITE_API_PROXY_TARGET` | `http://127.0.0.1:8080` | Dev server `/api` proxy target. |
| `VITE_API_BASE_URL` | `/api/v1` | Browser API base URL. |
| `VITE_CONTROL_URL` | browser origin | Default URL shown in runner install commands. |

## Public Deployment Notes

- Put public deployments behind HTTPS.
- Set `WORKBENCH_AUTH_COOKIE_SECURE=true` when using HTTPS.
- Use a Control URL reachable from target runner machines.
- Do not use `127.0.0.1` for remote runner installers unless Meridian runs on
  the same machine.
- The runner install endpoints are intended for trusted environments.
