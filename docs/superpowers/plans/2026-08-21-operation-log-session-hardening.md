# 操作日志与会话撤销加固 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 消除管理端会话撤销的旧 Token 窗口，并保证每次操作都能以有界、不可由客户端去重绕过的方式进入审计队列。

**Architecture:** 会话 Service 在 PostgreSQL 写入前获取 authstate invalidating lease，Repository 仍只访问 PostgreSQL；提交后发布新 generation，再删除 snapshot。操作日志使用服务端 eventId 做队列和数据库幂等，requestId 只做链路关联；OperationLog 包裹 Recovery 以采集 panic 的 500 响应。

**Tech Stack:** Go、Gin、GORM、PostgreSQL、Redis、Asynq。

---

不自动提交 Git；当前 `web/src/access/menu-icons.ts` 和 `web/src/access/protocol.test.ts` 的用户改动不在本计划范围内。

### Task 1: 锁定会话撤销契约

**Files:**
- Modify: `server/internal/module/auth/session_admin_handler.go`
- Modify: `server/internal/module/auth/session_admin_handler_test.go`
- Modify: `server/internal/module/auth/session_admin.go`
- Modify: `server/internal/module/auth/session_repository_test.go`
- Modify: `server/internal/shared/i18n/catalog.go`
- Modify: `server/internal/shared/i18n/catalog_test.go`

- [ ] 写失败测试：单条不存在返回 404/`session.notFound`，单条当前会话返回 403/`session.currentProtected`，批量仍跳过当前会话、已撤销和不存在项。
- [ ] 运行 `go test ./internal/module/auth ./internal/shared/i18n -run "Test.*Session.*(NotFound|Current|Bulk)|TestCatalogs" -count=1`，确认因目标行为缺失而失败。
- [ ] 为 Service 明确区分单条和批量调用；Repository 增加只读目标查询，单条错误在 PostgreSQL 写入及 Redis mutation 前返回。
- [ ] 增加 `KeySessionNotFound` 的中英文 catalog；当前会话使用既有稳定 key。
- [ ] 重跑定向测试，确认通过。

### Task 2: 将 invalidating lease 前移到 PostgreSQL 撤销前

**Files:**
- Modify: `server/internal/module/auth/session_admin.go`
- Modify: `server/internal/module/auth/session_admin_integration_test.go`
- Modify: `server/internal/module/auth/session_repository_test.go`

- [ ] 写失败集成测试：Repository 执行撤销时 Redis sessions state 必须已经是 `invalidating`；模拟 PostgreSQL 成功后 generation 发布失败，旧 Token 不得从旧 snapshot 认证成功。
- [ ] 运行对应 auth 测试，确认旧顺序使测试失败。
- [ ] Service 先查询目标并归组，一次 `Acquire` 所有 sessions facts，启动 renewal 后调用 Repository 事务；失败时 rollback，成功后 commit fresh generations，最后 DeleteMany。
- [ ] 保留 PostgreSQL 已提交后的显式 Redis 错误；不得恢复旧 ready generation，不添加同步 PostgreSQL 兜底写或补偿队列。
- [ ] 重跑 auth/authstate 定向测试，确认通过。

### Task 3: 用 eventId 隔离链路标识与审计幂等

**Files:**
- Modify: `server/internal/module/operationlog/middleware.go`
- Modify: `server/internal/module/operationlog/task.go`
- Modify: `server/internal/module/operationlog/model.go`
- Modify: `server/internal/module/operationlog/repository.go`
- Modify: `server/internal/module/operationlog/schema.go`
- Modify: `server/internal/module/operationlog/schema_test.go`
- Modify: `server/internal/module/operationlog/middleware_test.go`
- Modify: `server/internal/module/operationlog/task_test.go`
- Modify: `server/internal/module/operationlog/integration_test.go`
- Modify: `server/internal/module/operationlog/response.go`

- [ ] 写失败测试：相同 requestId 的两次操作拥有不同 eventId；TaskID 使用 eventId；payload 缺 eventId 被拒绝；Repository 以 event_id 幂等；schema 中 event_id 唯一而 request_id 非唯一。
- [ ] 运行 operationlog 定向测试并确认失败原因正确。
- [ ] TaskPayload 升级为 schemaVersion 2 并新增 EventID；Middleware 每次匹配规则时服务端生成；Model/Item/response 明确传递 eventId。
- [ ] `EnsureSchema` 可重复迁移旧表：添加并回填 event_id、设置 NOT NULL、替换索引；Repository 冲突目标改为 event_id。
- [ ] 重跑 operationlog 定向与真实 PostgreSQL schema/integration 测试。

### Task 4: 让请求和响应摘要严格有界

**Files:**
- Modify: `server/internal/module/operationlog/middleware.go`
- Modify: `server/internal/module/operationlog/middleware_test.go`
- Modify: `server/internal/module/operationlog/task_test.go`

- [ ] 写失败测试：超过 32 KiB 的请求与响应得到合法 JSON、带 truncated、总长度不超过限制；多字节 UTF-8 保持有效；summaryWriter 的缓存长度保持有界。
- [ ] 运行定向测试，确认旧实现分别因超限和无界缓存失败。
- [ ] 实现有界 writer；超限时只记录固定 truncated JSON，不保存未经完整解析和脱敏的 prefix。
- [ ] 重跑摘要和 payload 校验测试，确认通过。

### Task 5: 审计 panic 失败操作

**Files:**
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`
- Modify: `server/internal/module/operationlog/middleware_test.go`

- [ ] 写失败测试：登记的操作路由 panic 后，客户端收到统一 500 envelope，Enqueuer 收到 statusCode=500、isSuccess=0 的唯一任务。
- [ ] 运行定向测试，确认当前中间件顺序无法入队。
- [ ] 将 OperationLog 放到 Recovery 外层，保持 RequestID、CORS、AccessLog 的既有相对顺序。
- [ ] 重跑 operationlog、middleware 和 cmd/api 测试。

### Task 6: 全量验证与文档一致性

**Files:**
- Modify: `docs/superpowers/specs/2026-08-21-operation-log-session-management-design.md`
- Modify: `docs/superpowers/plans/2026-08-21-operation-log-session-management.md`

- [ ] 在原设计与原计划中标注已经由本加固设计替代的 request_id 唯一键和撤销顺序，避免后续 Agent 复刻旧行为。
- [ ] 运行 `go fmt ./...`、`go vet ./...`、`go test ./...`、`go build ./...`。
- [ ] 运行 `git diff --check`，检查没有环境文件、密钥、构建产物或无关前端改动进入本次 diff。
- [ ] 对照 6 个已确认行为逐项复核测试名称和实现路径，不自动 commit、push、fetch 或 pull。

## 自检

- 六个确认缺口均有失败测试、最小实现和定向回归步骤。
- Session Repository 只访问 PostgreSQL；authstate 和 snapshot 调用仍在 Service。
- OperationLog eventId 是具体字段，没有引入通用幂等服务或 Adapter。
- schemaVersion、Go/JSON/PostgreSQL 命名在计划内保持一致。
- 没有 TODO、兼容字段、静默兜底或与本次问题无关的功能。
