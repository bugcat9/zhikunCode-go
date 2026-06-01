# ZhikunCode Go Workspace

这是 ZhikunCode 的 Go 迁移工作区，用来把原有项目逐步拆分为前端应用和 Go 后端服务。

当前目录保持前后端分离：

```text
ZhikunCode-go/
  frontend/                         # React + Vite 前端
  go-backend/                       # Go 后端迁移目标
  python-service/                   # 现有 Python 分析/tokenizer 服务
  docs/go-migration-development-plan.md
```

## 项目目标

本工作区的目标不是直接改动原始 `zhikuncode` 项目，而是在独立目录中推进 Go 后端迁移：

- 保留现有 React 前端，后续逐步接入 Go API。
- 新建 `go-backend`，用 Go 实现后端能力。
- 保留 `python-service`，迁移期间由 Go 后端按需代理调用。
- 优先完成健康检查、配置、Python Service 调用、LLM Client、QueryEngine、工具调用和 Agent Kernel。
- 迁移过程中可以继续和原 Java 后端、Python Service 并行运行。

## 目录说明

```text
frontend/
  src/                # 前端源码
  e2e/                # Playwright 端到端测试
  package.json        # 前端脚本和依赖
  vite.config.ts      # Vite 配置

go-backend/
  cmd/server/         # Go 服务入口
  internal/api/       # HTTP 路由和 Handler
  internal/config/    # 配置加载
  internal/storage/   # SQLite 和迁移
  internal/session/   # 会话与消息
  internal/llm/       # OpenAI-compatible LLM Client
  internal/engine/    # QueryEngine
  internal/tools/     # 工具接口和工具注册表
  internal/permission/# 权限策略和请求
  internal/agent/     # Agent、SubAgent、Coordinator
  internal/python/    # Python Service Client
  internal/workspace/ # 工作区路径边界
  internal/protocol/  # DTO 和错误协议

python-service/
  src/                # Python 服务源码
  cli/                # 命令行入口
  tests/              # Python 测试
  pyproject.toml      # Python 项目配置
  requirements.txt    # Python 依赖
```

## 环境要求

- Node.js，建议使用项目 `.nvmrc` 对应版本。
- npm。
- Go，建议使用 Go 1.22 或更新版本。
- Python 3.10 或更新版本，用于运行 `python-service`。

## 前端开发

进入前端目录：

```bash
cd frontend
npm install
npm run dev
```

默认开发环境变量在 `frontend/.env.development` 中：

```text
VITE_API_URL=http://localhost:8080
VITE_APP_TITLE=AI Code Assistant (Dev)
```

当前前端默认仍指向原 Java 后端 `8080`。当 Go 后端 API 准备好后，可以把 API 地址切到 Go 服务端口，例如 `http://localhost:8081`。

常用前端命令：

```bash
npm run dev
npm run build
npm run test
npm run test:e2e
```

## Python Service 开发

Python Service 已复制到当前工作区，用于保留现有 tokenizer、代码分析、图表生成等能力。Go 后端迁移过程中会通过 HTTP Client 调用它。

进入 Python Service 目录：

```bash
cd python-service
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
```

计划中的服务地址：

```text
http://localhost:8000
```

Go 后端计划代理的 Python 能力：

```text
GET  /api/python/capabilities
POST /api/tokenizer/count
POST /api/code-diagrams/generate
POST /api/code-path/endpoints
POST /api/code-path/trace
```

## Go 后端开发

Go 后端当前处于目录骨架阶段，后续会在 `go-backend` 内补齐 `go.mod`、服务入口和内部模块。

计划中的本地端口：

```text
Go Backend:      http://localhost:8081
Java Backend:    http://localhost:8080
Python Service:  http://localhost:8000
Frontend:        http://localhost:5173
```

计划中的启动方式：

```bash
cd go-backend
go run ./cmd/server
```

计划优先实现的接口：

```text
GET /api/health
GET /api/doctor
GET /api/config
GET /api/python/capabilities
POST /api/tokenizer/count
POST /api/code-diagrams/generate
POST /api/query
POST /api/query/stream
```

## 迁移路线

详细开发计划见：

```text
docs/go-migration-development-plan.md
```

建议按以下顺序推进：

1. 初始化 `go-backend/go.mod`。
2. 实现 `cmd/server/main.go`、配置加载和日志。
3. 实现 `/api/health`、`/api/doctor`。
4. 接入 Python Service capabilities 和 tokenizer。
5. 实现 OpenAI-compatible LLM Client。
6. 实现 `/api/query` 和 QueryEngine v0。
7. 加入 SQLite session/message 存储。
8. 实现 Tool interface、ToolRegistry 和基础文件工具。
9. 实现 SSE stream、permission broker、SubAgent、Coordinator。

## 开发原则

- 前端和 Go 后端保持独立目录，便于单独启动、测试和替换。
- Python Service 保持独立目录，便于迁移期间继续复用已有分析能力。
- Go 后端优先提供兼容前端的 REST/SSE 接口。
- Python Service 作为可选能力保留，通过 Go 后端统一代理。
- 每个阶段先做可运行的最小闭环，再逐步替换 Java 后端能力。
