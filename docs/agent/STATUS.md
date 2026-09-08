# 项目状态

> 这是当前唯一的进度入口。它记录现在要做什么、已经交付什么和下一步做什么；不回填历史
> `docs/superpowers` plan。

## 本轮交付状态（2026-09-08，优先于下方历史记录）

- 最新归属修正已落实到代码：前后端管理模块 `permission/authplatform`，Model 映射
  `permission_auth_platform`，管理 URL `/api/admin/v1/permission/authplatform`、权限码
  `permission:authplatform:*`、导航键 `navigation.permissionAuthplatform`、页面文案 `permission.authplatform.*`。
  `auth/login`、公共 `/api/v1/auth/policy` 与现有平台策略 Redis key 不变；不把前端客户端平台常量
  `src/auth/platform.ts` 误当成管理模块。业务表尚未改名，现状仍为 `auth_platform`。
- 当前 forward migration 已同步原位改表、约束/索引/序列改名、页面路径和权限码转换；隔离 PostgreSQL
  验证跨平台 ID、外键、序列绑定与原有授权不变、重复执行和后续冲突整体回滚。必须排空旧 API、Worker
  及维护脚本后，由维护者执行迁移，再启用新版本。此脚本包含前一轮邮件两策略转换，不单独跳过其中步骤。
- Agent/三个 Skill 统一为“先确认业务归属并校正后端，再统一各层”，不再以现有后端目录自动推断归属。
- 认证平台归属调整验证：`go fmt ./...`、`go vet ./...`、`go build ./...`、
  `go test -p 1 ./... -count=1` 全部通过；补充的原位改表后失败回滚测试用
  `go test ./internal/architecture -count=1` 通过。`go test -race -p 1 ./internal/module/permission/authplatform ./internal/module/auth/login -count=1`
  两包完整通过。IDE build 无错误，三个 Skill validator 通过。
- 此次前端 `pnpm lint`、`pnpm check:architecture`、`pnpm build` 通过；全量 Vitest 为 470/471 通过，
  会话页第一项超过默认 5 秒（全量下 8272ms），并非断言失败。使用原超时独立复跑
  `pnpm vitest run tests/views/user/session/index.test.ts --pool=threads --maxWorkers=1 --reporter=verbose`，
  6/6 通过，原超时项 1230ms。未修改测试超时；未将本次全量记录标为全绿。
- 当前依据仅为维护者本次最终决策；下方旧日期记录中的 plan 链接、7 条邮件策略、固定 60 秒和旧路径
  均为历史执行记录，不是当前契约，不恢复已经删除的 plan/spec。
- 已实现：首次设密条件写入与并发单胜者、重置密码撤销会话、forget readiness 失效、refresh 设密标记；
  后端分层归位及 AST 门禁；Views/API/权限/i18n 按 user 等业务域和单数资源统一；三个 Skill 同步。
- 邮件实现仅两条平台+规范化邮箱共享策略，覆盖业务场景与管理测试；TTL/次数/窗口来自 Mail。
  旧场景额度按 GCRA 剩余债务原子归并，任一窗口拒绝不扣本次请求，损坏状态不产生部分写入。
- 业务数据库迁移尚未执行：见 `docs/database/2026-09-08-module-naming-mail-policy.sql`。
  已在隔离 PostgreSQL schema 验证幂等、跨平台 ID/授权保留、次数保留和编码冲突整体回滚。
  必须排空旧 API 后迁移并统一部署，不能新旧版本混跑；不清库、不重置旧 Redis 邮箱额度。
- 未闭环的架构容量项：Access/Auth 正常 missing 的跨实例回源上限；菜单修改时全用户收集、锁与失效开销。
  Redis error/corrupt 路径现已显式失败且零故障回源，但这不等于冷启动或百万用户写路径已验收。
- 交互待核验：发码成功后的前端倒计时仍使用 Mail 短窗口长度，多次额度配置下可能比服务端可重发时间保守；
  没有新增 Auth 时间配置，但尚未完成按剩余额度返回精确可重发时间的契约。
- 上轮实际验证（认证平台归属调整前）：`go fmt ./...`、`go vet ./...`、`go build ./...`、第二轮
  `go test -p 1 ./... -count=1` 全部通过；随后新增的迁移冲突回滚测试独立通过。
  `pnpm vitest run --pool=threads --maxWorkers=1 --reporter=verbose` 65 文件/471 项全部通过；
  `pnpm lint`、`pnpm check:architecture`、`pnpm build`（含类型检查）通过，三个 Skill validator 通过。
  首次前端全量运行约九分钟无单文件结果后已停止，以上是重新运行的最终结果。
- `git diff --check` 通过；暂存区 `git diff --cached --check` 仍报 role-view.ts 末尾空行，工作区版本已修正，
  未修改其他会话的暂存状态。保留既有 i18n flatten 警告及构建大包提示；未执行浏览器业务端到端验收。
  现有开发服务 `http://localhost:16300` HTTP 200，但业务库未迁移，不能据此宣称新版菜单已联通。
  上述验证对应当前代码，不覆盖刚修正的认证平台业务归属；未 commit 或执行业务库迁移。
- 当前环境已具备 GCC 且 `CGO_ENABLED=1`。本轮 `go test -race -p 1` 对 auth/login、user/account、
  message/mail、permission/access 使用 `Test.*(Concurrent|TwoInstances|TwoServices|FirstPassword|SetPassword|Fault|RedisFailure|Corrupt)`
  筛选的定向测试四包通过；未跑全仓 race。下方旧记录中的无编译器阻塞已过时。

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

## 邮箱验证码 TTL 与重发契约收口（2026-09-07）

> 来源计划：`docs/agent/plans/2026-09-07-mail-verification-contract-closeout.md`。已按失败测试 → 最小实现落地并全量验证，未提交。

**实际接口与 Redis namespace**

- Mail 窄接口改为三段式：`VerifyCodeReady(...) (VerifyCodeReadiness{Ready, TTLMinutes}, error)`、`PrepareEmailVerifyCode(...)`（readiness + 收件规则 + 业务限流各一次）、`SendPreparedEmailVerifyCode(...)`（不重复计数）。
- readiness 快照 schema/key/load-lock 升 v2：`mail:verify-code-readiness:v2:<platform>:<scene>`，ready payload 携带 `ttlMinutes`；v1 不解析、不回退。
- Auth 验证码 key 升 v2：`auth:verify-code:v2:`；Put Lua 只以当前 delivery lease 为写权限，允许原子覆盖旧码；TTL 允许 1–60 分钟。
- 限流 key 未换 namespace，发布期间额度连续。

**5 分钟 TTL 与 60 秒重发验证证据**

- `message_mail_config.ttl_minutes=5` 是唯一 TTL 来源：`loadFromRepository` 读取 `config.TTLMinutes`，`TestPrepareEmailVerifyCodeReturnsConfigTTLAndResendWindow` 断言 TTL=5 且 `business_email_minute.window_seconds=60`。
- Auth 已删除 `verificationCodeTTL = 10 * time.Minute` 常量；`SendCode` 用一次捕获的 `now + preparation.TTLMinutes` 计算 `expiresAt`，同一值写入 Redis/模板/verification log/响应。
- 前端 `SendCodeResult` 增加严格整数 `resendAfterSeconds`（1–86400），登录页倒计时直接取该值，不再由 `expiresAt - Date.now()` 推算。

**双实例查询预算与 winner 数**

- 64 路 v2 missing 只回源一组 PG（config:1/template:1）；100 次 ready 热读零 PG 查询（`TestVerifyCodeReadinessTwoInstancesRecoverMissingSnapshotOnce`）。
- 修复双实例协调竞态：拿到 load-lock 后、回源前复查快照，闭合落后实例的冗余回源。
- 双 Auth Service 同邮箱并发 SendCode 仅一个进入 provider（`TestSendCodeTwoServicesSingleProviderWinner`）。

**各故障点公开错误码**

- 额度超限 `429/10007`；Redis 读取故障 `503/10006`；readiness/依赖不可用 `503/10006`。
- Prepare 失败只释放 lease、不删除旧码；code Put 后发送失败删除本次 code 并释放 lease；任一 owned cleanup 失败提升为 `503/10006`，不返回假成功。

**执行者基线曾运行的命令（终审轮未复跑，最终以维护者执行为准）**

- 后端：`go fmt ./...`、`go vet ./...`、`go test ./...`（36 包全过）、`go build ./...`。
- 前端：`pnpm vitest run --pool=threads --maxWorkers=1`（63 文件 451 用例全过）、`pnpm lint`、`pnpm check:architecture`、`pnpm typecheck`、`pnpm build`。
- 2026-09-07 终审轮：以上全量命令均未运行，由维护者执行最后一轮全量验证。

**终审兜底（2026-09-07，独立审查者）**

| 严重度 | Finding | 修复 | RED/GREEN |
| --- | --- | --- | --- |
| Critical | Auth 验证码 Redis value 解析不严格：`Check` 用普通 `json.Unmarshal` 只比较 digest，接受缺 `leaseToken`、未知字段（含 PII 走私）与 trailing data；`Consume`/`CheckAttempt`/`DeleteIfOwned` Lua 只比较 `decoded.digest`，corrupt payload 可能被当作有效码消费或删除 | `auth/login/redis.go`：新增 `decodeVerificationCodeValue`（token 级重复 key 扫描 + `DisallowUnknownFields` + EOF trailing 检查 + digest/leaseToken 非空校验）用于 `Check`；三个 Lua 脚本统一 `pcall(cjson.decode)` + 字段数恰为 2 + digest/leaseToken 非空字符串校验，异常返回 `corrupt` 并在 Go 侧 fail-closed | `TestVerificationCodeRejectsCorruptValueFields` 先 RED（`Check accepted corrupt value "{"digest":"digest-a"}"`），修复后 GREEN |
| Important | `VerifyCodeReady` 在读取 readiness 前先 `loadRateLimitCatalog`，登录配置热路径因无关限流目录回源 Redis/PostgreSQL，且 catalog 故障会误杀 ready 结果（旧契约残留） | `message/mail/service.go` 删除该调用，ready 热读只读 readiness v2 快照；限流目录归属 Prepare | 新增 `TestVerifyCodeReadyHotReadDoesNotLoadRateLimitCatalog` 先 RED（catalog 错误导致 `ready={false 0}` 依赖错误），修复后 GREEN；`TestVerifyCodeReadyFailsWhenRateLimitPolicyIsUnavailable` 更名为 `...ReadinessStoreIsUnavailable` 修正语义 |

**终审轮实际运行的定向命令（全部通过）**

- `go test ./internal/module/auth/login -run 'TestVerificationCode|TestSendCode|TestHandlerSendCode' -count=1`（25 项）
- `go test ./internal/module/message/mail -run 'Test.*VerifyCodeReadiness|TestVerifyCodeReady|TestPrepareEmailVerifyCode|TestSendPreparedEmailVerifyCode|Test.*TwoInstances|Test.*Concurrent|Test.*Fault' -count=1`（22 项）
- `go test ./internal/module/auth/login ./internal/module/message/mail -run 'Test.*TwoInstances|Test.*Integration|Test.*Concurrent|Test.*Fault' -count=1`
- `gofmt -l` 对 auth/login 与 message/mail 输出为空（redis.go 终审改动已格式化）。
- `pnpm vitest run tests/api/auth/login.test.ts tests/views/auth/login/index.test.ts --pool=threads --maxWorkers=1`（2 文件 24 项）、`pnpm typecheck`、`pnpm check:architecture`（0 基线项）。
- `git diff --check` 通过。

**终审独立验证的关键行为**

- 双实例：`TestSendCodeTwoServicesSingleProviderWinner`（双 Auth Service/双 Redis client 单 provider winner）、`TestVerificationCodeTwoClientConsumeHasSingleWinner`、`TestPrepareEmailVerifyCodeTwoServicesShareRedisRateWindow`（双 Mail Service/双 Redis client 共享真实限流窗口，429 后窗口恢复重发成功）、`TestVerifyCodeReadinessTwoInstancesRecoverMissingSnapshotOnce`（双实例单回源 + 热读零 PostgreSQL）。
- TTL 同源：`TestSendCodeUsesPreparationTTLAndResendWindow`、`TestPrepareEmailVerifyCodeReturnsConfigTTLAndResendWindow`（TTL=5、resend=60 来自 `business_email_minute.window_seconds`）、`TestSendPreparedEmailVerifyCodePersistsPreparationTTL`。
- 429/503：`TestSendCodeReturnsRateLimitedWhenPrepareIsRateLimited`（429 时 `releaseCalls=1`、`deleteCalls=0`，旧码不被烧毁）、`TestVerificationCodePutReplacesPriorCodeUnderCurrentLease`（窗口后新码替换旧码）、`TestSendCodeFailsClosedWhenCleanupFails`、`TestSendCodeGenerationFailureReportsLeaseCleanupFailure`、`TestSendCodeSuccessfulSendFailsClosedWhenLeaseReleaseFails`。
- 静态自查：`rg` 确认无 `verificationCodeTTL` 残留、无 cooldown key、生产链路无 `context.Background()`；phone 登录在 `service.go` 显式 `Forbidden`；前端 `SendCodeResult` 严格三字段 DTO（1–86400 整数校验）、倒计时仅用 `resendAfterSeconds`、无 `el-select`/`any`/`@ts-ignore`。

**未运行项与剩余风险**

- `-race` 未运行（本机无 C 编译器）。
- 旧 v1 readiness/验证码 key 不迁移，按原 TTL 自然过期。
- `SettingDrawer` 因拆出 `SettingDrawer.css` 新增一个文件，属此前 UI 工作，非本计划范围，保留为可 review diff。

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

### 当前优先工作：统一命名、邮件最高规则与后端维护（2026-09-08）

以下条目是当前执行入口；后文 `auth-platform-login-types` 与 TTL 收口条目保留前置工作的记录，不代表本轮仍需重新实施。

| 项目 | 内容 |
| --- | --- |
| 目标 | `password-recovery-first-set`：按现有独立契约兜底找回密码和首次设置密码，先完成架构与行为收口 |
| 决策来源 | 维护者本次对话的最终方案；旧 plan/spec 已删除，不恢复、不作为当前依据 |
| 状态 | 方案已确认，实施及全量回归中；尚未宣称整体交付完成 |
| 优先级 | 后端 Handler -> Service -> Repository -> Model 与前端 View -> API -> Request 边界优先；UI 视觉统一整改延期，必要的协议及交互正确性仍需验证 |
| 兜底范围 | 认证接口与测试替身一致性、首次设密原子条件写入、forget readiness 失效、passwordSetRequired 生命周期、验证码消费与会话撤销的并发/部分失败验证 |
| 已定契约 | 所有发邮件场景（包括管理测试）服从 Mail 管理配置；邮箱额度按平台+规范化邮箱共享，验证码内容按 scene 隔离。60 秒/5 分钟不是硬规则，不新增 Auth TTL/cooldown。存量 Redis 窗口必须衔接，不能直接换 key 重置额度 |
| 验收 | 保留重置后不自动登录、首次设密不强制拦截及保留会话的既定行为；定向行为测试、共享接口测试编译与构建通过；真实依赖验证和未运行项单独记录 |
| 下一步 | 前端对齐后端 user 等业务域，所有资源使用单数；闭合 API/菜单/权限/i18n/测试。邮件只保留每分钟与每 10 分钟两条平台+邮箱策略，删除其他五条。UI 改版不在本轮范围 |

本轮数据库迁移：`docs/database/2026-09-08-module-naming-mail-policy.sql`。已在真实 PostgreSQL 的隔离
schema 上验证两次执行幂等、跨平台菜单改名、原菜单 ID/角色授权保留、access version 仅递增一次及邮件
配置次数保留。尚未在业务 schema 执行，遵守维护者执行 forward migration 的规则。

发布约束：停止并排空所有旧 API 实例后执行 SQL，再启动新版前后端；不能新旧版本混跑。Access snapshot
使用 `authz:permission:v5:`，access state 使用 `authz:permission-state:v3:`，Mail policy 使用 v2，旧 key
不回读、不全库清理。邮箱限流新 key 使用 `mail:send:email:v2:` / `email10:v2:`（Redis 实际带 `rate:`
前缀），每次至多读取两组各四个已知旧场景 key，以 GCRA 剩余债务原子归并，拒绝请求不扣任一窗口。
已删除的 IP/场景/管理员专属策略停止参与判断，旧专属 key 自然过期；它们不是新版邮箱策略的新额度来源。
旧 key 无 hash tag，该跨 key 归并针对当前 standalone Redis；不支持直接迁至 Redis Cluster 后再归并。

后端维护验收清单（本轮进行中）：跨层命名约束与三个 Skill；DTO/Handler/Service/Repository/Model/shared
责任归位与自动检查；首次设密原子写入；邮件全场景共享策略与旧 Redis 衔接；readiness 全场景失效；
refresh 首次设密标记；真实 PostgreSQL/Redis 并发与故障回归。另需评估 Access/Auth Redis 故障无界回源、
菜单全用户失效的容量风险，不能用文件移动代替这些问题的结论。

本轮维护原则：根规则只放硬边界，任务路由负责按需加载，三个项目 Skill 分别维护 CRUD、数据库和 RBAC 施工流程；只将可复用的已确认约束沉淀到 Skill，不把本次密码业务细节扩成通用框架。

本轮已执行验证：`go build ./...` 通过；公共 shared/config/secretkey 定向测试通过；auth/login、message/mail、auth/platform、user/profile 的 `-short` 测试通过（真实依赖测试跳过）。`go test ./... -run '^$'` 因 API/Access 测试替身缺少新增认证接口方法而失败。前端 `pnpm check:architecture` 通过；全量 Vitest 已报告一个存储页面用例失败，随后长时间未完成，本轮启动的进程已停止，无完整通过结论。工作区同时存在其他会话改动，以上仅代表执行当时的结果。

### 前置工作记录：认证平台登录方式

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

## 找回密码与首次设置密码（2026-09-08）

> 用户授权牵头实现找回密码/首次设置密码契约；探索与交互方案已与用户对齐后端到端落地，未提交。

**后端契约**

- 公开路由 `POST /api/v1/auth/password/forgot`（发码，scene=forget，返回 SendCodeResult）与
  `POST /api/v1/auth/password/reset`（email+code+newPassword+confirmPassword，成功 data 为空对象）。
- 登录、refresh、Me 响应新增 `passwordSetRequired` 布尔字段：凭据 `PasswordHash == ""` 时为 true。
- 认证路由 `POST /api/admin/v1/account/password/set`：仅允许无密码账号首次设置（已有密码返回
  `10005` Conflict），使用 `account:password:update` 权限码，成功后不吊销当前会话；改密仍走
  `ChangePasswordAndRevokeSessions` 吊销全部会话。
- Repository 新增 `SetPasswordHash`；`authenticationService` 接口新增 `ForgotPassword`/`ResetPassword`，
  相关测试桩已同步。

**前端契约与页面**

- `AccessCredential`/`CurrentUser` 严格 DTO 纳入 `passwordSetRequired`（`expectExactKeys` + 布尔校验），
  `utils/request.ts` 的 `isAccessCredential` 探测同步。
- 新增 `/forgot-password` 页面（复用 LoginPage.css 布局）：邮箱 → 发码（60s 倒计时、challenge 轮换）→
  设置新密码 → 成功回登录页并经 `?account=` 预填；登录页新增忘记密码入口链接。
- profile 密码卡片双形态：`passwordSetRequired` 为 true 时隐藏当前密码框、改调 `setPassword`、成功文案
  提示会话保持；store 在 set 成功后清除标记。
- zh-CN/en-US 文案齐备；所有引用 `AccessCredential`/`CurrentUser` 的测试 stub 已同步新字段。

**验证证据**

- 后端：`go fmt`、`go vet`、`go build`、`go test ./...`（36 包）全部通过。
- 前端：全量 Vitest 64 文件 461 项中 460 项通过，唯一失败（store updateProfile 期望形状）修复后定向重跑
  通过；`pnpm build`（含 vue-tsc）通过，仅存既有 >500 kB chunk 提示。
- 首轮全量中 storage-object/roles 4 项 5s 超时为机器负载抖动，隔离重跑全部通过。
- `go test -race` 仍未执行（机器无 gcc/clang，与既有记录一致）。

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
