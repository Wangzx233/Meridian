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

Meridian 是一个通过 Web 管理项目级 Codex CLI 工作的控制台。它不是新的
agent runtime，不是 IDE，也不替代 Codex。真正执行任务的仍然是目标设备上的
本地 Codex CLI，Meridian 只负责把多设备、多项目、多任务、运行历史和上下文选择
集中管理起来。

## 快速开始

开源用户优先使用 Docker Compose。它会启动 PostgreSQL、启动后端、在后端启动时
自动应用数据库迁移、构建设备代理 artifacts，并服务 Web UI。

```bash
git clone https://github.com/Wangzx233/Meridian.git
cd Meridian
docker compose up -d --build
```

打开：

```text
http://<server-ip>:18080
```

如果只在本机试用，也可以打开 `http://127.0.0.1:18080`。Compose 默认监听
`0.0.0.0`，自托管服务器不需要再手动设置公开监听地址。第一次浏览器访问会进入
初始化管理员账号流程。

如需修改端口、数据库密码、外部数据库或认证配置：

```bash
cp .env.example .env
vi .env
docker compose up -d --build
```

公开或共享部署建议放到 HTTPS 后面。如果 Meridian 只通过本机反向代理暴露，可以在
`.env` 中设置 `MERIDIAN_HTTP_BIND=127.0.0.1`。

## 连接设备代理

每台执行任务的设备都需要先安装 Codex CLI。然后直接使用 Meridian 页面里的安装脚本：

1. 打开 Web UI，创建或选择一台设备。
2. 点击右上角的设备代理安装按钮。
3. 将 Control URL 设置为目标设备能访问到的 Meridian 地址。
4. 复制 UI 里给出的 Linux、macOS 或 Windows 命令，并在目标设备上运行。
5. 创建设备下的项目，把 `workdir` 设置为真实项目目录。

远程设备不要使用 `127.0.0.1`，除非 Meridian 也运行在同一台设备上。后端镜像和源码
部署流程都会构建安装端点所需的设备代理 artifacts，通常不需要手写下载命令。

## 手动源码部署

不使用 Docker Compose 部署应用栈时，可以用源码方式部署。后端默认在启动时自动迁移
数据库，所以首次部署不需要单独执行数据库迁移命令。

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

用你的 Web 服务器服务 `frontend/dist`，并把 `/api` 和 WebSocket 流量代理到后端。
UI 可访问后，仍然通过右上角安装菜单连接目标设备代理。

更多部署选项、外部数据库配置和 Windows 备注见
[部署指南](docs/deployment.md)。

## 开发

开发 Meridian 本身时使用源码流程：

```bash
docker run --name meridian-postgres \
  -e POSTGRES_DB=meridian_dev \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 55433:5432 \
  -d postgres:16-alpine

sh ./scripts/build-runner-artifacts.sh

DATABASE_URL='postgres://postgres:postgres@127.0.0.1:55433/meridian_dev?sslmode=disable' \
BACKEND_ADDR='127.0.0.1:18080' \
RUNNER_ARTIFACT_DIR="$PWD/artifacts/runner" \
go run ./backend/cmd/server
```

另开一个 shell：

```bash
cd frontend
npm ci
VITE_API_PROXY_TARGET='http://127.0.0.1:18080' \
VITE_CONTROL_URL='http://127.0.0.1:18080' \
npm run dev
```

打开 `http://127.0.0.1:5173`。

## 核心能力

| 能力 | 说明 |
| --- | --- |
| 多设备 | 管理安装了设备代理的机器，并跟踪在线状态。 |
| 真实项目目录 | 每个项目绑定到某台设备上的真实工作目录。 |
| 长任务 | 一个任务可以包含多次 Codex turn，成功运行不等于任务完成。 |
| Codex 会话恢复 | 保存 Codex CLI session id，后续 turn 可继续同一任务上下文。 |
| 显式上下文 | 用户手动选择少量、可见的上下文项。 |
| 实时输出 | 设备代理将 Codex run event 流式传回控制台。 |
| 项目工具 | 支持项目文件浏览、轻量编辑和项目目录内终端命令。 |
| 设备代理分发 | 后端提供 Linux、macOS、Windows 设备代理安装端点。 |

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

## 基本使用流程

1. 打开 Web UI。
2. 创建设备，并用右上角安装菜单安装设备代理。
3. 在设备下创建项目，并设置真实 `workdir`。
4. 创建任务。
5. 每次发送一条用户指令。
6. 在 Output、Terminal 和 Files 中查看项目与运行状态。
7. 持续追加 turn，直到确认工作完成。
8. 手动将任务标记为 done。

## 仓库结构

```text
backend/   Go 控制平面 API
runner/    Go 设备代理，连接控制平面并调用 Codex CLI
frontend/  React + TypeScript + Vite Web UI
db/        PostgreSQL migrations
docs/      需求、架构、API、部署和发布文档
scripts/   本地辅助脚本
```

## 检查

```bash
go test ./...
go vet ./...
(cd frontend && npm ci && npm run build)
sh ./scripts/build-runner-artifacts.sh
```

## 当前限制

- 认证是简单登录门禁，没有自助注册或细粒度权限模型。
- 设备代理安装端点面向可信环境。
- 设备代理 artifacts 暂未签名。
- Codex CLI 需要在执行任务的设备上单独安装。
- 第一版只支持手动上下文选择，不做自动推荐或注入。
- 成功的 Codex run 不会自动完成任务，必须由用户手动标记 done。

## 相关文档

- [部署指南](docs/deployment.md)
- [贡献指南](CONTRIBUTING.md)
- [安全策略](SECURITY.md)
- [发布清单](docs/release-checklist.md)
- [需求文档](docs/requirements.md)
- [架构文档](docs/architecture.md)
- [API Contract](docs/api-contract.md)
