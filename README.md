# Meridian

<p align="center">
  <img src="frontend/public/favicon.svg" alt="Meridian icon" width="96" height="96">
</p>

[English](README.en.md) | 中文

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Node.js](https://img.shields.io/badge/Node.js-20.19%2B-339933?logo=nodedotjs&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=111)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16%2B-4169E1?logo=postgresql&logoColor=white)
![CI](https://img.shields.io/badge/CI-GitHub_Actions-2088FF?logo=githubactions&logoColor=white)

Meridian 是一个通过 Web 管理任务的控制台。它把多台设备、多个项目、多个任务放到一个界面里，让你随时打开浏览器，一键切换设备、项目和任务。自动提炼任务摘要，需要时可手动选择传入其他设备、项目、任务上下文。

Meridian 不替代 Codex，也不重新实现 agent。真正执行任务的仍然是目标设备上的 Codex CLI。

Meridian 所做的是在每台设备上运行一个设备代理，来管理设备上的 Codex。



## 适用场景

- 你在不止一台设备上使用 Codex CLI，希望有一个统一入口。
- 你维护多个项目，需要快速切换到对应设备上的项目和任务。
- 手动显式选择其他任务和过往上下文进行传递，每次任务不用从头开始。
- 同时将几个甚至上百个工作交给 Codex，你只需要等待任务完成的通知。
- 你希望在网页里查看运行状态、输出、历史记录和失败原因。


### 和常见方案的区别

| 方案 | 主要体验 | Meridian 的差异 |
| --- | --- | --- |
| Hermes / OpenClaw | 通用 agent、个人自动化、跨工具编排。 | 不做新的 agent runtime，只围绕项目任务管理 Codex CLI。 |
| IDE + AI | 在编辑器里写代码，也可以通过远程开发连接服务器。 | 用 Web 在不同设备、项目、任务之间直接切换。每切换任务不需要再新建窗口、连接服务器、启动codex... |
| Codex App / CLI | 直接和 Codex 交互，APP 也可以连接服务器运行。 | 服务器无需拥有公网 IP 也可控制，工作台保存在 Meridian 不需要相同 APP 账号。 |

Meridian 的重点不是“更聪明”，而是“更好管理”：Web 可访问、设备可切换、项目可切换、任务可继续。

## 快速开始

### Docker Compose

开源用户建议优先使用 Docker Compose。它会启动 PostgreSQL、执行数据库迁移、构建设备代理 artifacts、启动后端，并用前端 Nginx 将 `/api` 和 WebSocket 流量代理到后端。

```bash
docker compose up --build
```

打开：

```text
http://127.0.0.1:18080
```

第一次浏览器访问会进入初始化管理员账号流程。本地 Compose 会把 PostgreSQL 数据保存在 `postgres_data` Docker volume。

如需修改端口、数据库密码或认证配置：

```powershell
Copy-Item .env.example .env
```

然后编辑 `.env` 并重新启动：

```bash
docker compose up --build
```

公开或共享部署时，设置 `MERIDIAN_HTTP_BIND=0.0.0.0`，把 Meridian 放到 HTTPS 反向代理后面，并设置 `WORKBENCH_AUTH_COOKIE_SECURE=true`。

### 源码开发

当你要开发 Meridian 本身，或者希望分别启动前端和后端时，使用源码开发流程。

需要准备：

- Go 1.25 或更新版本。
- Node.js 20.19 或更新版本，以及 npm。
- PostgreSQL，需支持 `pgcrypto` extension。
- 每台执行任务的设备都需要安装 Codex CLI。
- Windows 本地开发建议使用 PowerShell，因为设备代理构建脚本是 PowerShell 脚本。

启动 PostgreSQL：

```powershell
docker run --name meridian-postgres `
  -e POSTGRES_DB=meridian_dev `
  -e POSTGRES_USER=postgres `
  -e POSTGRES_PASSWORD=postgres `
  -p 55433:5432 `
  -d postgres:16-alpine
```

设置数据库连接并执行迁移：

```powershell
$env:DATABASE_URL = "postgres://postgres:postgres@127.0.0.1:55433/meridian_dev?sslmode=disable"
go run ./backend/cmd/migrate up
```

如果 migrations 不在仓库根目录下，设置 `MIGRATIONS_DIR`：

```powershell
$env:MIGRATIONS_DIR = "D:\go\workplace\db\migrations"
```

为安装端点构建设备代理 artifacts：

```powershell
.\scripts\build-runner-artifacts.ps1
```

如果 `go` 不在 `PATH`，设置 `GO_EXE`：

```powershell
$env:GO_EXE = "C:\Users\DELL\sdk\go1.26.1\bin\go.exe"
.\scripts\build-runner-artifacts.ps1
```

启动后端：

```powershell
$env:DATABASE_URL = "postgres://postgres:postgres@127.0.0.1:55433/meridian_dev?sslmode=disable"
$env:BACKEND_ADDR = "127.0.0.1:18080"
$env:RUNNER_ARTIFACT_DIR = "D:\go\workplace\artifacts\runner"
go run ./backend/cmd/server
```

API 地址：

```text
http://127.0.0.1:18080/api/v1
```

另开一个 shell 启动前端：

```powershell
cd frontend
npm install
$env:VITE_API_PROXY_TARGET = "http://127.0.0.1:18080"
$env:VITE_CONTROL_URL = "http://127.0.0.1:18080"
npm run dev
```

打开：

```text
http://127.0.0.1:5173
```

## 核心能力

| 能力 | 说明 |
| --- | --- |
| 多设备 | 管理安装了设备代理（Runner）的设备，并跟踪在线状态。 |
| 项目目录 | 每个项目绑定到某台设备上的真实工作目录。 |
| 长任务 | 一个任务可以包含多次 Codex turn，成功运行不等于任务完成。 |
| Codex 会话恢复 | 保存 Codex CLI session id，后续 turn 可继续同一任务上下文。 |
| 显式上下文 | 用户手动选择少量上下文项，避免自动注入不可见上下文。 |
| 实时输出 | 设备代理将 Codex run event 流式传回控制台。 |
| 项目工具 | 支持项目文件浏览、轻量编辑、项目目录内终端命令。 |
| 设备代理分发 | 构建 Windows、Linux、macOS artifact，并通过后端安装端点下载。 |

## 架构速览

```text
Browser UI
  -> Go backend control plane
  -> PostgreSQL task/run/event store

Go backend control plane
  <-> device agent WebSocket
  <-> target device agent
  -> local Codex CLI in the project workdir
```

## 仓库结构

```text
backend/   Go 控制平面 API
runner/    Go 设备代理，连接控制平面并调用 Codex CLI
frontend/  React + TypeScript + Vite Web UI
db/        PostgreSQL migrations
docs/      需求、架构、API contract、release checklist
scripts/   本地辅助脚本
```

## 基本使用流程

1. 打开 Web UI。
2. 创建设备（Server）。
3. 在对应设备上安装并启动设备代理（Runner）。
4. 选择设备，并在该设备下创建 project。
5. 将 project `workdir` 设置为真实项目目录。
6. 创建 task。
7. 在 Output 查看 Codex 输出，在 Terminal 执行项目目录内命令，在 Files 浏览和轻量编辑项目文件。
8. 每次只发送一条用户指令。
9. 持续追加 turn，直到你确认任务完成。
10. 手动将任务标记为 done。

## 设备 / 项目 / 任务模型

| 概念 | 作用 |
| --- | --- |
| `Server` | 一台可执行 Codex 任务的设备。代码里保留 `Server` 命名，产品语境里可以理解为设备。 |
| `Project` | 绑定到某个设备的真实工作目录。Codex 在该目录中执行。 |
| `Task` | 一个真实工作目标，可能需要多轮 Codex turn 才完成。 |
| `Run` / `Turn` | 一次用户指令和一次 Codex CLI 执行。 |
| `ContextItem` | 用户手动选择的规则、笔记、日志、摘要、命令或文件提示。 |
| `CodexSession` | Codex CLI 拥有的 session id，用于 resume。 |
| `Runner` | 安装在设备上的轻量代理，负责接收任务并在本机项目目录里启动 Codex CLI。 |

设备代理会在网络中断或后端重启后持续重连。后端会给断开的设备代理一个短暂宽限期，避免普通重启时立即把设备标记为离线。

每个 turn 可选覆盖 Codex model、reasoning effort 和 service tier：

```bash
--model gpt-5.5 --config model_reasoning_effort="high" --config service_tier="fast"
```

留空则使用目标设备上的本地 Codex 配置。

## 安装设备代理（Runner）

可以使用 UI 右上角的安装按钮，也可以直接调用安装端点。新设备不要传 `runner_id`，安装脚本会根据设备生成唯一 ID。只有在重装同一设备记录时才传 `runner_id=<runner-id>`。

Windows PowerShell：

```powershell
powershell -ExecutionPolicy Bypass -NoProfile -Command "iex ((iwr -UseBasicParsing -Uri 'http://<control-host>:<port>/api/v1/runner/install.ps1').Content)"
```

如果需要机器级 SYSTEM task，以管理员 PowerShell 运行并添加 `run_as=system`：

```powershell
powershell -ExecutionPolicy Bypass -NoProfile -Command "iex ((iwr -UseBasicParsing -Uri 'http://<control-host>:<port>/api/v1/runner/install.ps1?run_as=system').Content)"
```

Linux 或 macOS：

```bash
curl -fsSL 'http://<control-host>:<port>/api/v1/runner/install.sh' | sh
```

不要在远程设备上使用 `127.0.0.1`，除非控制平面也运行在同一台设备上。

安装脚本会创建：

| 平台 | 安装方式 |
| --- | --- |
| Windows user mode | `%LOCALAPPDATA%\CodexTaskWorkbench\runner` 和 Startup folder 命令。 |
| Windows SYSTEM mode | `CodexTaskWorkbenchRunner` Scheduled Task。 |
| Linux | 优先创建 `codex-task-workbench-runner.service` systemd 服务；容器或非 systemd 环境会 fallback 到 standalone `nohup` 后台进程。 |
| macOS | `com.codex-task-workbench.runner` launchd daemon。 |

Linux standalone fallback 会把 runner 安装到 `/opt/codex-task-workbench/runner`，
并写入 `run-runner.sh`、`runner.pid`、`runner.log` 和 `runner.err.log`。这种模式
不会跨容器/主机重启自动恢复；重启后重新运行安装命令或手动执行 `run-runner.sh`。

如果目标机器上的 Codex CLI 不在默认路径，可传自定义路径：

```bash
curl -fsSL 'http://<control-host>:<port>/api/v1/runner/install.sh?codex_path=/usr/local/bin/codex' | sh
```

## 手动启动设备代理

```powershell
$env:CONTROL_URL = "http://127.0.0.1:18080"
$env:RUNNER_ID = "local_runner"
$env:CODEX_PATH = "codex"
go run ./runner/cmd/runner
```

Linux 或 macOS：

```bash
CONTROL_URL=http://127.0.0.1:18080 \
RUNNER_ID=local_runner \
CODEX_PATH=codex \
go run ./runner/cmd/runner
```

## 重要环境变量

### Backend

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DATABASE_URL` | required | PostgreSQL 连接串。 |
| `BACKEND_ADDR` | `:8080` | HTTP 监听地址。 |
| `RUNNER_ARTIFACT_DIR` | `artifacts/runner` | 设备代理 artifact 下载目录。 |
| `CODEX_BYPASS_APPROVALS_AND_SANDBOX` | `true` | 为 Codex run 添加 bypass sandbox/approval 参数。设为 `false` 可保留 Codex 原行为。 |
| `WORKBENCH_AUTH_USERS` | empty | 逗号分隔账号，如 `admin:password,ops:password2`。为空时进入浏览器首次设置模式。 |
| `WORKBENCH_AUTH_SESSION_SECRET` | auth users 存在时 required | 浏览器 session cookie 签名密钥。 |
| `WORKBENCH_RUNNER_TOKEN` | auth users 存在时 required | 设备代理安装、WebSocket 连接和 artifact 下载的 bearer token。 |
| `WORKBENCH_AUTH_COOKIE_SECURE` | `true` | 是否要求 HTTPS cookie。仅本地 HTTP 开发可设为 `false`。 |

### Runner

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CONTROL_URL` | `http://localhost:8080` | 控制平面 base URL。 |
| `RUNNER_ID` | machine hostname | 与设备记录匹配的设备代理身份。 |
| `CODEX_PATH` | `codex` | Codex CLI 可执行文件路径。 |
| `RUNNER_TOKEN` | empty | 连接开启鉴权的控制平面时使用的 bearer token。 |
| `CODEX_BYPASS_APPROVALS_AND_SANDBOX` | `true` | 设备代理是否确保 Codex 以无 sandbox/approval prompt 模式运行。 |
| `HEARTBEAT_INTERVAL` | `10s` | 设备代理 heartbeat 间隔。 |
| `RUNNER_ENV` | empty | 额外环境变量，使用 `;` 分隔。 |

### Frontend

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `VITE_API_PROXY_TARGET` | `http://127.0.0.1:8080` | 开发服务器 `/api` 代理目标。 |
| `VITE_API_BASE_URL` | `/api/v1` | 浏览器 API base URL。 |
| `VITE_CONTROL_URL` | browser origin | 设备代理安装命令中默认显示的控制平面 URL。 |

## 开发检查

提交或合并前运行：

```powershell
go test ./...
go vet ./...
cd frontend
npm run build
```

当设备代理、安装脚本、release 打包或 artifact 服务逻辑变化时，构建 runner artifacts：

```powershell
.\scripts\build-runner-artifacts.ps1
```

GitHub Actions 会在 pull request 和 `main` push 上运行：

- `go test ./...`
- `go vet ./...`
- `npm run build`
- runner artifact build

## 发布

发布准备清单见 [docs/release-checklist.md](docs/release-checklist.md)。

推送形如 `v0.1.0` 的 tag 会触发 GitHub release workflow：

```powershell
git tag v0.1.0
git push origin v0.1.0
```

Release workflow 会重新运行质量门、构建设备代理 artifacts、生成 `SHA256SUMS.txt`，并发布到 GitHub Release。

## 部署提示

大多数自托管安装建议使用仓库根目录的 `docker-compose.yml`：

1. 默认启动内置 PostgreSQL。
2. 后端启动前自动执行 `migrate up`。
3. 后端镜像内构建并服务设备代理 artifacts。
4. 通过 Nginx 服务构建后的前端。
5. 将 `/api`、SSE 和 WebSocket upgrade 流量代理到后端。

使用 `.env.example` 作为部署配置模板。默认监听 `127.0.0.1` 是为了本地安全；只有在反向代理、防火墙和 HTTPS 方案明确后，才修改 `MERIDIAN_HTTP_BIND`。
Docker Compose 部署如果要使用外部数据库，设置 `MERIDIAN_DATABASE_URL`；
不要在 compose `.env` 里设置 `DATABASE_URL`，避免旧 shell 或 CI 变量覆盖内置数据库连接。
如果外部数据库也是 Docker 容器并位于其他网络，额外设置
`MERIDIAN_DATABASE_DOCKER_NETWORK=<network>`，并在命令中同时加载
`docker-compose.external-db.yml`：

```bash
docker compose --env-file .env -f docker-compose.yml -f docker-compose.external-db.yml up -d --build
```

源码部署仍然支持：

1. 准备 PostgreSQL。
2. 执行 `go run ./backend/cmd/migrate up`。
3. 执行 `scripts/build-runner-artifacts.ps1`。
4. 设置 `DATABASE_URL`、`BACKEND_ADDR`、`RUNNER_ARTIFACT_DIR` 并启动后端。
5. 执行 `npm run build` 构建前端。
6. 用静态服务器或反向代理服务 `frontend/dist`。
7. 将 `/api` 和 WebSocket upgrade 流量代理到后端。

公开或共享部署必须使用 HTTPS，并确保设备代理安装命令使用公网控制平面 URL。`WORKBENCH_AUTH_USERS` 为空时，首次浏览器访问会进入初始化管理员账号流程。

私有 Gitea 部署会把触发部署的 commit 写入后端和前端镜像，并在部署后校验
Compose 内部网络和公网 URL。后端提供 `GET /api/v1/build`，前端提供
`/build.json`。如果公网地址不是 `https://meridian.example.com`，在部署 `.env` 中设置
`MERIDIAN_DEPLOY_VERIFY_URL`。

## 当前限制

- 认证是简单登录门禁，没有自助注册或细粒度权限模型。
- 设备代理安装端点面向可信环境。
- 设备代理 artifacts 暂未签名。
- Codex CLI 需要在执行任务的设备上单独安装。
- 第一版只支持手动上下文选择，不做自动推荐或注入。
- 成功的 Codex run 不会自动完成任务，必须由用户手动标记 done。

## 相关文档

- [贡献指南](CONTRIBUTING.md)
- [安全策略](SECURITY.md)
- [发布清单](docs/release-checklist.md)
- [需求文档](docs/requirements.md)
- [架构文档](docs/architecture.md)
- [API Contract](docs/api-contract.md)
