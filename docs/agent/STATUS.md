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
| RBAC Skill 架构 | `$admin-rbac` 按当前 PostgreSQL 权限事实、Redis `ready/invalidating` 状态、版本化快照、本地缓存确认、token lease 失效和动态路由链路重写；新增无界 PostgreSQL 故障回源、旧 publisher/token、跨平台和双实例容量检查 |
| 前端下拉组件 | 全部生产 `el-select`/`el-option` 迁移为 `el-select-v2 + options`；`AGENTS.md`、设计基线及 `check:architecture` 固化禁用规则，保留数值/字符串、多选、可创建与 header 插槽行为 |
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
| 下一步/阻塞 | 原实现与终审已完成；维护者随后定案邮箱验证码 TTL 必须唯一来自 Mail 配置（当前 5 分钟），重发只复用现有 Redis rate limit。下一工作单元按 `docs/agent/plans/2026-09-07-mail-verification-contract-closeout.md` 收口 Auth/Mail/前端时间语义和 60 秒后重发；SMS 接入前 `phone` 仍不可用。`-race` 仍受 `CGO_ENABLED=0` 且无 C compiler 阻塞。 |

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
| Critical | 六位验证码可被并发枚举，错误请求还会放大 LoginLog 写入 | `auth/login/redis.go` 使用 Lua 原子校验并按账号 10 次/IP 30 次、10 分钟窗口限流；账号/IP 均为 HMAC key，无 PII；达限在 User/LoginLog 前返回 `429/10007` | 账号/IP 阈值、正确码不删除、共享 Redis 可重复运行及 Service 短路测试通过 |
| Important | Policy 新增 `loginTypes` 后仍复用 v1 Redis namespace；仅调整顺序也会错误递增版本 | `auth/platform/redis.go` 使用 schema/key/load-lock v2；`service.go` 比较前规范化登录方式 | 旧快照绕开、反序 no-op、双实例恢复测试通过 |
| Important | `VerifyCodeReady` 每次公开 `login-config` 查询配置和全部模板，发送前又重复查询 | `message/mail/readiness.go` 新增 Redis readiness 状态机；missing 才经 singleflight + 5 秒 token lock 单次回源，corrupt/invalidating/Redis error 闭合；配置/模板状态写入使用 tokenized CAS 发布 | 双实例 64 路仅 1 组 PostgreSQL 查询、100 次热命中零查询、跨实例失效/旧 owner 拒绝测试通过 |
| Important | Mail/provider 失败时复用已取消请求 context，可能残留 Auth code/lease；Mail readiness 写库后的发布亦可能被取消 | Auth 清理使用 1 秒 `WithoutCancel` 有界 context；Mail rollback 1 秒、提交后发布 5 秒且保留 request values | 取消后 code/lease 清理、readiness rollback/publish 测试通过 |
| Important | `challengeId` 超过数据库 128 字符、管理端空 `loginTypes` 仍可保存、登录配置失败后无法恢复 | 请求增加 `max=128`；表单校验 1-3 个合法且不重复；登录页增加 i18n 失败态与重试按钮，成功后只渲染服务端方式 | Handler、认证平台页、登录页 9 项测试通过 |
| Important | GoLand 的 `PACKAGE + admin/server/...` 将 CLI wildcard 当作单个包校验，无法启动后端全量测试 | `.run/Admin Server Tests.run.xml` 改为从 `$PROJECT_DIR$/server` 执行 `go test ./...` 的 Shell Script 配置；`Admin Full Tests` 同步引用新类型 | 3 个 `.run` XML 解析通过；`go list ./...` 识别 39 个包；最终全量仍由维护者执行 |
| Checked | Public routes、管理端 action RBAC、Access version、hidden action 分离 | `cmd/api/main.go`/测试、Access fixture 与现有 Middleware 映射核对 | `login-config`/`send-code`/`login` 路由基线及 RBAC/Access 测试通过 |
| Checked | context 必须贯穿 Service/Repository/Redis/Mail，不得下传 Gin 或换成 `context.Background()` | Auth 链路使用 request context；Mail Handler 全部改传 `c.Request.Context()`；补偿/发布只使用 `context.WithoutCancel(ctx)` 加硬超时，不创建无界后台 context | 取消请求原错误返回 200 的测试先 RED，修复后返回 `503/10006`；生产调用链静态扫描通过 |

迁移与验证记录：

- 已执行 `docs/database/2026-09-05-auth-platform-login-types.sql`，输出为 `BEGIN`、`ALTER TABLE`、回填、约束/索引重建、`COMMIT`，exit 0；未打印 DSN，未更新 `docs/database/current.sql`。
- Task 12 使用两个 Auth Service/两个 Redis client 验证 policy 单次回源、ready 零额外查询、单 client 故障闭合；验证 32 路验证码 Consume 仅一个 winner、Redis Put/Consume/Release 故障，以及 PostgreSQL identity 回滚/login-log best-effort。
- 实现基线曾完成 `go fmt ./...`、`go vet ./...`、`go test ./...`、`go build ./...`；终审兜底后定向执行 `go test ./cmd/api ./internal/module/auth/login ./internal/module/auth/platform ./internal/module/message/mail -count=1`，四包均通过。
- 前端实现基线曾完成 63 文件/446 项 Vitest 与 `pnpm build`；终审兜底后登录/认证平台/API 4 个文件共 36 项 Vitest、受影响文件 Prettier、`pnpm lint`、`pnpm check:architecture`、`pnpm typecheck` 均通过，最终全量复跑交维护者从 `.run` 执行。全仓 `pnpm format:check` 仍因 3 个未被本任务修改的历史文件失败：`role-view.ts`、`mail.test.ts`、`menus/index.test.ts`。
- `go test -race` 未执行：当前 `CGO_ENABLED=0` 且机器没有 `gcc/clang`；Vite build 仍有既有的 >500 kB chunk 提示。本轮终审兜底未 commit，历史计划文件无本轮修改。
- `.run` 提供 `Admin Server Tests`、`Admin Web Tests` 和一键并行的 `Admin Full Tests`；后端使用 Shell Script 在 `server` 工作目录执行真实 `go test ./...`，避免 GoLand `PACKAGE` 模式拒绝 wildcard。3 个 XML 均可解析，`go list ./...` 识别 39 个包；未代维护者执行最后一轮测试。

### 邮箱验证码契约收口（待执行）

| 项目 | 内容 |
| --- | --- |
| 计划 | `docs/agent/plans/2026-09-07-mail-verification-contract-closeout.md` |
| 固定契约 | Mail `message_mail_config.ttl_minutes` 是邮箱验证码唯一 TTL 来源，当前 5 分钟；重发只使用现有 `business_email_minute=1/60s` Redis rate limit；不新增 cooldown 状态。 |
| 实施范围 | Mail readiness v2 携带 TTL、限流前置预检、Auth lease-owner 原子替换、`resendAfterSeconds` HTTP/前端契约、真实 Redis 双实例故障探针。 |
| 不在范围 | 数据库迁移、SMS/phone 开放、系统设置、身份生命周期、AuthPlatform 字段、`registered_user` 授权、并发协议时间配置化。 |
| 验收 | 5 分钟 TTL 全链路同源；60 秒内 429 且旧码可用，60 秒后新码替换旧码；Redis/PG/provider 故障无假成功；readiness 热读零 PostgreSQL，missing 双实例单回源；Go/Vitest 全量通过。 |

## 项目规则维护（2026-09-07）

- `$admin-rbac` 已通过 `skill-creator/scripts/quick_validate.py`；其读写路径与当前 permission 模块核对，明确
  Redis state 确认、本地/Redis snapshot、PostgreSQL fresh source、事务内 access version、tokenized
  invalidation lease 和提交后 CAS 发布顺序。现有 Redis 故障 PostgreSQL 回源若后续调整，必须补跨实例有界
  恢复或改为显式依赖失败，禁止无界每请求回源。
- 前端 5 个生产 SFC（个人资料、认证平台、对象存储配置/规则、邮件收件规则）共 10 个旧下拉已迁移为
  `el-select-v2`；架构脚本会拒绝新的原生 `el-select`。定向 4 文件 47 项 Vitest、typecheck、lint、
  `check:architecture` 和生产 build 通过；完整前端 63 文件 449 项 Vitest 全绿，build 仍有既有的 >500 kB
  chunk 提示。

## 状态条目模板

复制以下四行到“后续事项”或新建当前工作条目即可：

```text
目标：
范围：
验收：
下一步/阻塞：
```
