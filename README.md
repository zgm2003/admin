# Admin

一个采用线性数据流的 Vue 3 + Go 管理后台基础项目。

普通数据库请求固定经过：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

首期由 Vue Admin、Go API、Go Worker、PostgreSQL、Redis 和 Asynq 组成。项目不使用 Docker；Git 仓库按基础质量加固计划初始化和绑定远程，但不会自动提交或推送。

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

开发阶段只有 API 启动时执行 `AutoMigrate`；Worker 连接 PostgreSQL 更新任务状态，但不执行迁移。因此首次启动或模型变化后，应先启动 API，再启动 Worker。

## 健康检查

- `GET /health`：只证明 API 进程存活，不访问 PostgreSQL 或 Redis。
- `GET /ready`：同时探测 PostgreSQL 和 Redis；任一不可用时返回 HTTP 503。

Redis 就绪表示 Asynq 的队列存储可访问，不代表 Worker 一定正在消费。首期直接检查 Worker 进程和日志，不伪造 Worker 心跳。

## 示例任务

```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:16301/api/v1/example-tasks -ContentType 'application/json' -Body '{"message":"foundation-check"}'
```

API 先在 PostgreSQL 创建 `pending` 记录，再向 Asynq 投递只包含 `taskId` 的消息。Worker 从 PostgreSQL 读取任务内容，并将状态更新为 `running`、`completed` 或 `failed`。PostgreSQL 是任务事实来源，Redis 不是。

首期示例采用尽力投递，不承诺 PostgreSQL 与 Redis 之间的原子一致性。需要可靠投递的正式业务任务出现后，再按设计引入事务 Outbox。

## 验证

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

完整设计见 `docs/superpowers/specs/2026-08-17-admin-foundation-design.md` 和 `docs/superpowers/specs/2026-08-17-admin-foundation-hardening-design.md`。当前仓库远程由项目所有者确认，根 `.gitignore` 持续生效；项目不会自动创建提交或推送历史。
