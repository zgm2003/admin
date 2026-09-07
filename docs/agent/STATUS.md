# 项目状态

> 这是当前唯一的进度入口。它记录现在要做什么、已经交付什么和下一步做什么；不回填历史
> `docs/superpowers` plan。

## 当前总目标

| 项目 | 内容 |
| --- | --- |
| 目标 | `agent-context-refactor`：让 Agent 使用小而精、渐进式的项目上下文 |
| 状态 | 已完成 |
| 验收 | 硬规则、任务路由、状态、设计、架构和固定 Skill 均有唯一入口；历史 spec/plan/SQL 不进入默认读取 |
| 最近更新 | 2026-09-04 |

## 已完成基线

- 后端使用 `server/cmd/api`、`server/cmd/worker` 两个显式组合入口，业务模块位于
  `server/internal/module`，同步请求遵循线性分层。
- 当前后端模块包括认证、健康检查、权限（菜单/角色/Access）、用户、消息邮件、对象存储和操作日志；
  具体范围以代码和数据库事实为准。
- 前端生产代码位于 `web/src`，测试位于 `web/tests`；页面、API、Router、Store 和公共组件已有独立目录。
- HTTP envelope、严格 DTO、RBAC 页面/动作权限、PostgreSQL/Redis Access 版本协议和显式时间/删除字段规则已
  固化在 `AGENTS.md`、`design.md`、`architecture.md`。
- `$admin-crud`、`$admin-rbac`、`$admin-database` 已安装到
  `C:\Users\IGT\.codex\skills`，分别封装 CRUD、RBAC 和数据库施工流程。
- 旧方案和一次性 migration 保留在 `docs/superpowers/**`、`docs/database/2026-*.sql`，默认读取路径已移除。

## 本次整改记录

| 项目 | 结果 |
| --- | --- |
| 根硬规则 | `AGENTS.md` 收敛为不可违反的边界和验证要求 |
| 渐进路由 | `docs/agent/README.md` 提供启动顺序、任务矩阵和历史档案策略 |
| 当前设计 | `docs/agent/design.md` 提供产品、UI、协议和权限基线 |
| 当前架构 | `docs/agent/architecture.md` 提供组件、数据流和数据库事实边界 |
| 容量基线 | 共享请求路径按百万级用户、多实例高并发设计；spec/plan 必须记录热点查询、缓存一致性、故障行为和并发验证 |
| 固定流程 | 三个全局 Skill 通过 quick validator |
| 邮件规则与限流 | 修复 `AppTable` 动态插槽导致收件规则状态请求携带 `undefined`；收件规则明确为默认允许、邮箱优先、拒绝优先；Redis 限流异常返回 `503/10006`，真实额度超限返回 `429/10007` |
| 邮件日期展示 | 邮件日志接口将 `sentAt`、`createdAt`、`updatedAt`、`verificationExpiresAt` 显式输出为 UTC RFC3339Nano，空值为 `null`；页面使用供应商实际发送时间 `sentAt` 和验证码过期时间，并按应用语言和本地时区格式化，空值或非法值显示 `-` |
| 邮件错误反馈 | 收件规则拒绝使用 `403/18000`；终态 `401/403` 由请求层通知一次，登录凭据 `10002` 由登录页内联展示且不重复通知 |
| 后端契约与全量验证 | 邮件日志列表复用 `shared/pagination.Result`；权限菜单测试夹具统一遵循 `path = "/" + componentPath`，完整后端测试、静态检查和构建通过 |
| 邮件限流策略管理 | 核心实现和前端页签已落地；已补迁移跨平台 action 隔离、RBAC Access 授权/版本失效/隐藏 action 集成 fixture、Redis 回源版本保护、失败草稿恢复和严格策略 DTO 校验。2026-09-05 已在本地 `admin` PostgreSQL 执行限流策略迁移并校验 7 条策略与隐藏 action；真实 Redis 双实例并发/故障探针仍未完成 |

## 后续事项

- 新业务模块按任务路由加载 `$admin-crud`、`$admin-rbac` 或 `$admin-database`，交付后只更新本文件对应条目。
- 需要长期跟踪的功能在本表新增一行，不再创建一组必须同步维护的历史 spec/plan。
- 数据库迁移成功并确认需要快照时，从真实 PostgreSQL 导出 `docs/database/current.sql`；当前未创建该文件，
  不用历史 SQL 拼接伪造。
- 2026-09-05 执行 `docs/database/2026-09-04-mail-rate-limit-policies.sql`：创建
  `message_mail_rate_limit_policy` 并插入 7 条固定策略，创建隐藏 action
  `message:mail:rate-limit:update`；校验通过，并定向清理 1 个旧的权限状态 Redis 键，
  页面重载后 Access version 16 快照已包含该 action；未更新 `docs/database/current.sql`。
- 只有用户明确要求历史审计、决策恢复或迁移对比时，才读取 `docs/superpowers` 或旧 migration。

## 当前工作

| 项目 | 内容 |
| --- | --- |
| 目标 | `auth-platform-login-types`：把认证平台可配置的邮箱/手机号/密码登录方式连接到 Mail 验证码、Auth、User、Session、RBAC 和前端登录链路 |
| 范围 | `docs/agent/plans/2026-09-05-auth-platform-login-types.md`；AuthPlatform JSONB 策略与 Redis 故障闭合、Mail 认证窄接口、Auth 验证码 Redis 原子消费和 delivery lease、自动注册事务、Session/JWT、登录日志、严格前端 DTO、并发/故障探针 |
| 验收 | Redis ready 命中零 PostgreSQL 配置查询；Redis/数据库/Mail 部分失败显式 `503/10006`；验证码只能消费一次且错误码不删除正确码；并发注册由 PostgreSQL 唯一键收敛并补齐角色/Profile/Access version；登录方式变更递增 policy version 且不撤销既有 Session；全量 Go/Vitest/build 与 review checklist 通过 |
| 下一步/阻塞 | 2026-09-07 完成实现、Task 12 双实例/故障探针、Task 13 review 与兜底、Task 14 migration 和全量验证。Auth 固定拥有 10 分钟验证码 TTL，Mail 使用调用方 TTL；SMS 接入前 `phone` 可保存但不进入公开有效方式，直接请求亦被拒绝。无待修复的 Critical/Important finding；`-race` 仍受 `CGO_ENABLED=0` 且无 C compiler 阻塞 |

### `auth-platform-login-types` review 与兜底（2026-09-07）

| 严重度 | Finding/检查项 | 修复与文件 | 验证结果 |
| --- | --- | --- | --- |
| Critical | 可绕过平台 `loginTypes`，且 SMS 未接入时 `phone` 路径可被直接请求 | `auth/login/service.go` 在访问 User 前强制校验策略并拒绝不可用 phone；空密码哈希在 bcrypt 前拒绝 | Auth Service/Handler 回归通过 |
| Critical | 验证码在确认账号状态前消费会烧毁正确码，禁止注册时还可能暴露错误语义 | `auth/login/service.go` 改为 `Check -> identity decision -> Consume`；新账号且禁止注册统一 `401/10002` | 错码不删正确码、禁止注册不消费、单次消费测试通过 |
| Critical | 新 delivery lease 可覆盖未过期验证码；provider/Redis 部分失败可能残留 code/lease 或假成功 | `auth/login/redis.go` Lua 拒绝覆盖；`auth/login/service.go` 清理失败闭合为 `503/10006`，成功发送后释放 lease | Redis Put/Consume/Release、Mail 失败清理与重复 challenge 测试通过 |
| Critical | Policy 缺失恢复的 200ms 等待短于 5s load lock；首请求取消会拖累同实例 waiter，并用取消 context 释放锁 | `auth/platform/service.go`、`redis.go` 统一 5s 契约，使用 `DoChan`、有界共享 context、调用者独立取消和有界释放 context | `TestCurrentPolicyTwoInstancesShareOneMissingKeyRecovery`、`TestCurrentPolicyCanceledLeaderDoesNotCancelSharedRecovery` 连续 3 轮通过，PostgreSQL 仅回源 1 次 |
| Important | PostgreSQL 回源期间旧 publisher 可能盖过 Redis 中已发布的新 policy/loginTypes | `auth/platform/redis.go` 安装失败后读取胜出快照；policy 更新递增 version | 新快照胜出、ready 热读零 PostgreSQL、invalidating/corrupt/Redis error fail-closed 测试通过 |
| Important | Mail readiness 未包含 Redis 限流依赖，且 Auth/Mail 验证码 TTL 所有权不清 | `message/mail/protocol.go`、`service.go`：窄接口接收 Auth TTL，并把 Redis 限流纳入 readiness；Auth 固定返回自身 10 分钟有效期 | Mail readiness、TTL、发送失败测试通过；Auth 不依赖 Mail Repository/GORM |
| Important | 自动注册、唯一键竞争和事务回滚必须在高并发下保持完整 identity | `user/account/repository.go` 仅锁 enabled default role，在单事务创建账号/Profile/Role/Access version；唯一冲突 requery winner | 并发 winner、Profile INSERT 强制失败全回滚、phone-only `email=''`/空 hash 测试通过 |
| Important | LoginLog 故障不应改变已成功登录；Session/JWT 必须使用当前 policy version，关闭登录方式不能撤销既有 Session | `auth/login/service.go` 登录日志改 best-effort，凭证统一由当前策略签发 | 真实 PostgreSQL login-log INSERT 故障仍返回凭证；Session/AuthState/JWT 回归通过 |
| Important | 请求 DTO 可同时携带互斥凭据，响应漏出 `isNewUser`，登录失败类型记录不准 | `auth/login/request.go`、`response.go`、`handler.go` 使用指针字段和互斥校验，显式输出 `isNewUser`，邮箱失败记录 `loginType=email` | Handler/协议测试通过 |
| Important | 前端可在配置失败时显示默认密码方式，且 boolean/loginTypes/时间戳解析不严格 | `web/src/api/auth/*.ts`、登录页严格解析并在配置失败时不渲染登录表单；每次发送尝试轮换 challenge | Auth API 与登录页测试通过；无 phone/fake data 展示 |
| Important | 登录页超过 SFC 架构阈值；认证平台 `reactive const form` 触发 Vue 隐式 `let` 编译警告 | 样式迁至 `auth/login/LoginPage.css`；认证平台表单改为 `ref<AuthPlatformForm>` | `check:architecture` 通过；登录页 9 项、认证平台页 11 项测试通过且无该警告 |
| Checked | Public routes、管理端 action RBAC、Access version、hidden action 分离 | `cmd/api/main.go`/测试、Access fixture 与现有 Middleware 映射核对 | `login-config`/`send-code`/`login` 路由基线及 RBAC/Access 测试通过 |
| Checked | context 必须贯穿 Service/Repository/Redis/Mail，不得下传 Gin 或换成 `context.Background()` | 审查本轮生产调用链；共享恢复仅使用 `context.WithoutCancel(ctx)` 加硬超时，不创建无界后台 context | 全量 `go vet`、Go 测试及故障探针通过 |

迁移与验证记录：

- 已执行 `docs/database/2026-09-05-auth-platform-login-types.sql`，输出为 `BEGIN`、`ALTER TABLE`、回填、约束/索引重建、`COMMIT`，exit 0；未打印 DSN，未更新 `docs/database/current.sql`。
- Task 12 使用两个 Auth Service/两个 Redis client 验证 policy 单次回源、ready 零额外查询、单 client 故障闭合；验证 32 路验证码 Consume 仅一个 winner、Redis Put/Consume/Release 故障，以及 PostgreSQL identity 回滚/login-log best-effort。
- 后端：`go fmt ./...`、`go vet ./...`、`go test ./...`、`go build ./...` 均 exit 0。
- 前端：本轮文件 Prettier 检查、`pnpm lint`、`pnpm check:architecture`、63 文件/446 项 Vitest、`pnpm build` 均通过；最后一次表单 `ref` 调整后认证平台页 11 项定向测试与 build 通过，最终全量复跑交维护者从 `.run` 执行。全仓 `pnpm format:check` 仍因 3 个未被本任务修改的历史文件失败：`role-view.ts`、`mail.test.ts`、`menus/index.test.ts`。
- `go test -race` 未执行：当前 `CGO_ENABLED=0` 且机器没有 `gcc/clang`；Vite build 仍有既有的 >500 kB chunk 提示。工作区未 commit，历史计划文件无本轮修改。
- `.run` 新增 `Admin Server Tests`、`Admin Web Tests` 和一键并行的 `Admin Full Tests`；XML、Go 全包 pattern 和 Vitest 参数解析均已检查，未代维护者执行最后一轮测试。

## 状态条目模板

复制以下四行到“后续事项”或新建当前工作条目即可：

```text
目标：
范围：
验收：
下一步/阻塞：
```
