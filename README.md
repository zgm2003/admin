# Admin

一个采用线性数据流的 Vue 3 + Go 管理后台基础项目。

普通数据库请求固定经过：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

首期由 Vue Admin、Go API、Go Worker、PostgreSQL、Redis 和 Asynq 组成。项目不使用 Docker；Git 仓库按基础质量加固计划初始化和绑定远程，但不会自动提交或推送。

## AI 施工规范

AI 和开发者共同遵守根 [`AGENTS.md`](AGENTS.md) 的硬规则。任务路由见
[`docs/agent/README.md`](docs/agent/README.md)，当前进度见 [`docs/agent/STATUS.md`](docs/agent/STATUS.md)，
产品与架构基线分别见 [`docs/agent/design.md`](docs/agent/design.md) 和
[`docs/agent/architecture.md`](docs/agent/architecture.md)。按任务触发 `$admin-crud`、`$admin-rbac` 或
`$admin-database`，不要求全量阅读历史文档。

## 本机前提

- Go 1.26
- Node.js 与 pnpm
- PostgreSQL
- Redis

先确认 PostgreSQL 和 Redis 本机服务已经启动，并创建 `admin` 数据库。项目不会在依赖不可用时启动替代服务或使用内存兜底。

## 环境变量

后端读取 `server/.env`：

```dotenv
HTTP_ADDR=:16301
POSTGRES_DSN=host=127.0.0.1 user=postgres password=postgres dbname=admin port=5432 sslmode=disable TimeZone=Asia/Shanghai
REDIS_URL=redis://127.0.0.1:6379/0
CORS_ORIGIN=http://localhost:16300
```

API 使用全部四项。Worker 只读取 `POSTGRES_DSN` 和 `REDIS_URL`。变量缺失、为空或格式错误时进程会明确退出，不提供默认地址。

前端读取 `web/.env`：

```dotenv
VITE_API_BASE_URL=http://localhost:16301
```

浏览器直接访问 Go API；项目不配置 Vite Proxy。

模板分别位于 `server/.env.example` 和 `web/.env.example`。本机凭据只修改被忽略的 `.env`，不要把真实密码写入模板。

## 启动顺序

启动 PostgreSQL 和 Redis 后，分别打开三个终端：

```powershell
cd D:\admin\server
go run ./cmd/api
```

```powershell
cd D:\admin\server
go run ./cmd/worker
```

```powershell
cd D:\admin\web
pnpm dev
```

`pnpm dev` 使用固定端口启动 Vite，并自动打开浏览器。

本地页面为 `http://localhost:16300`，API 为 `http://localhost:16301`。Vite 使用固定端口并开启 `strictPort`，端口被占用时会明确退出，不会自动切换。

API 和 Worker 启动时都不会执行数据库迁移、数据预设或回填。首次启动或数据库结构变化前，必须由维护者先
执行已确认的 forward migration，再启动 API 和 Worker；带日期的 `docs/database/2026-*.sql` 只保留为历史
执行记录，不作为当前 schema 的默认来源。

## 健康检查

- `GET /health`：只证明 API 进程存活，不访问 PostgreSQL 或 Redis。
- `GET /ready`：同时探测 PostgreSQL 和 Redis；任一不可用时返回 HTTP 503。

Redis 就绪表示 Asynq 的队列存储可访问，不代表 Worker 一定正在消费。首期直接检查 Worker 进程和日志，不伪造 Worker 心跳。

## 验证

前端生产代码位于 `web/src`，测试代码统一位于 `web/tests`，并按 `src` 的目录结构镜像组织。测试只由 `web/vite.config.ts` 扫描 `tests/**/*.{test,spec}.{ts,tsx,js,jsx}`，不会进入生产源码目录或 Vite 生产入口。

```text
web/
├─ src/       # Vue 生产源码
└─ tests/     # Vitest 测试，镜像 src 结构
```

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

```powershell
cd D:\admin\web
pnpm vitest run
pnpm build
```

资源受限的 Windows 环境若在 Vitest 并行 worker 启动或页面测试上出现超时，可使用单 worker 完整运行同一套测试：

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
```

早期设计、实施计划和一次性迁移记录保留在 `docs/superpowers/**` 与 `docs/database/2026-*.sql`，仅用于明确的
历史追溯，不是当前 Agent 的默认上下文。当前仓库远程由项目所有者确认，根 `.gitignore` 持续生效；项目不会
自动创建提交或推送历史。
