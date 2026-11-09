# Admin

一个采用线性数据流的 Vue 3 + Go 管理后台基础项目。

普通数据库请求固定经过：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

首期由 Vue Admin、Go API、Go Worker、PostgreSQL、Redis 和 Asynq 组成。项目不使用 Docker；Git 仓库按基础质量加固计划初始化和绑定远程，但不会自动提交或推送。

部署边界固定为两个独立单元：`server/` 产出 API 和 Worker，可部署在 A 机器；`web/` 产出静态前端，可部署在 B 机器。两者只通过 HTTP API 通信，不依赖仓库根目录的 `shared/`、共享文件系统或跨机器运行时目录。`server/internal/shared/` 仅是 Go 服务内部的稳定公共源码包，随 server 编译，不是第三个部署单元。

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
CORS_ORIGIN=http://localhost:16300,http://127.0.0.1:16300
TRUSTED_PROXIES=none
APP_SECRET=replace_with_at_least_64_random_characters_before_running_api_server
AUTH_COOKIE_SECURE=0
```

API 使用以上变量；`TRUSTED_PROXIES` 必须明确填写 `none` 或逗号分隔的 IP/CIDR；`APP_SECRET` 必须是至少 64 个 ASCII 字符的随机值；`AUTH_COOKIE_SECURE` 在本地 HTTP 使用 `0`，HTTPS 使用 `1`。Worker 的连接配置读取 `POSTGRES_DSN` 和 `REDIS_URL`。变量缺失、为空或格式错误时进程会明确退出，不提供默认地址。

支付沙箱/生产环境和完整 HTTPS `notifyUrl` 在支付配置页面维护。浏览器回跳 `returnUrl` 由充值页面提交时自动取当前页面 URL（保留路径和 query，去掉 fragment），后端作为支付宝下单参数传递，不要求额外的支付 env 或 CORS 白名单。生产下单入口仍待到账验收后开放；历史订单回调和查单沿订单绑定的配置及凭据版本处理。

前端读取 `web/.env`：

```dotenv
API_PROXY_TARGET=http://localhost:16301
```

浏览器始终使用当前站点的 `/api/...`，HTTP 与 WebSocket 都走同源入口。开发时由 Vite 将完整路径转发到
`API_PROXY_TARGET`；该变量仅供开发代理读取，不打包到浏览器，必须是无路径、凭据、query 或 fragment 的
HTTP(S) origin。生产构建不依赖该变量，由 Nginx 反向代理到 Go API。

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

### Nginx 部署

在 `web/` 运行 `pnpm build` 后，部署 `web/dist/`，将 [`web/nginx.conf.example`](web/nginx.conf.example)
放入 Nginx 的 `http {}` 配置（例如 `conf.d/admin.conf`）。替换 `HOST` 为对外域名、`TARGET` 为 dist 的绝对
目录；API 在其他机器时修改 `proxy_pass`。执行 `nginx -t` 检查后再加载配置。

模板保留完整 `/api/...` 路径、WebSocket Upgrade 和前端路由刷新回退，API 错误不会回退成 HTML。
配置 HTTPS 时在同一站点加入证书与 TLS 监听，并将后端 `AUTH_COOKIE_SECURE` 设为 `1`；
`CORS_ORIGIN` 使用浏览器实际访问的完整 origin（协议、域名和非默认端口），`TRUSTED_PROXIES` 按实际代理
IP/CIDR 配置。代理透传浏览器 Origin，不改写认证来源校验。

Canvas 对外前端直接复用 `D:\github-project\infinite-canvas\web`，接入本项目的 Canvas 平台认证：

```powershell
cd D:\admin\canvas
pnpm install
pnpm dev
```

访问 `http://localhost:16302`；本地 `/api` 代理到 `http://localhost:16301`。运行 `pnpm test` 验证认证、个人中心和
画布逻辑，运行 `pnpm build` 执行类型检查及生产构建。AI 渠道和画布素材暂时沿用参考项目的本地配置与存储。

API 和 Worker 启动时都不会执行数据库迁移、数据预设或回填。首次启动或数据库结构变化前，必须由维护者先
执行已确认的 forward migration，再启动 API 和 Worker；带日期的 `docs/database/2026-*.sql` 只保留为历史
执行记录，不作为当前 schema 的默认来源。

## 健康检查

- `GET /api/health`：只证明 API 进程存活，不访问 PostgreSQL 或 Redis。
- `GET /api/ready`：同时探测 PostgreSQL 和 Redis；任一不可用时返回 HTTP 503。

首页提供服务状态、服务器资源、项目介绍与当前授权的运维入口。服务器指标来自响应请求的 API 实例，包含
CPU、主机内存、工作目录所在文件系统的磁盘使用量及 Go 运行时；不代表 Worker 或整个集群。仅手动刷新，
每实例合并采集并缓存 5 秒，采集预算 3 秒，失败不显示旧指标。页面跟随系统品牌、语言、明暗和主色设置。

首次启用服务器菜单，由维护者停止全部 API/Worker 后执行（脚本不会操作服务）：

```powershell
cd D:\admin\server
.\migrate-system-server.ps1 -ServicesStopped -AuditDirectory ..\docs\database\backups\system-server
```

工具在事务内执行 `docs/database/2026-11-06-system-server.sql`，核验 page 形状后同步 Redis 菜单版本，
记录 SQL 摘要与前后版本；可安全重跑。非 Windows 可在 `server` 下直接执行
`go run ./cmd/migrate-system-server -services-stopped -audit-dir TARGET`。失败时按审计修复后重跑，再启动服务。
首页服务器指标对所有已登录的管理端用户开放，接口 `GET /api/admin/v1/system/server` 只要求登录，
不新增详情 action。独立 `/system/server` 页面仍使用 `system:server:view`，普通角色需显式授权页面；
该迁移只增加可选的系统菜单入口，不是首页读取指标的前置条件。运维配置和写操作继续使用各自的权限。

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
