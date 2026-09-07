# 邮箱验证码 TTL 与重发契约收口实施计划

> **给执行者：** 严格按任务顺序施工，每个行为变化必须先写失败测试并确认失败，再做最小实现。全程不提交、
> 不推送、不回退其他开发者改动；只保留可 review 的工作区 diff。完成每个 Task 后记录实际命令和结果，最后交由
> 审查者按终审清单补兜底。

**目标：** 将邮箱验证码 TTL 收敛到 `message_mail_config.ttl_minutes` 这一个事实来源，修复“60 秒限流已结束但旧
验证码仍阻塞重发”的协议冲突，并让前端严格区分验证码有效期与重发等待时间。

**范围：** Mail verification readiness、Mail 认证窄接口、Auth 验证码 Redis Store、`POST /api/v1/auth/send-code`
响应、登录页倒计时、双实例并发/故障测试和状态记录。

**不在范围：** 系统设置、SMS、phone 登录开放、独立注册/找回密码/首次设置密码、`registered_user` 授权、
AuthPlatform 字段、数据库迁移、provider timeout、delivery lease/load lock/mutation lease/cleanup timeout 配置化。

**技术栈：** Go 1.26、Gin、GORM、PostgreSQL、Redis、`go-redis/redis_rate`、Vue 3、TypeScript strict、Vitest。

---

## 一、固定业务契约

### 1. 三种时间只能各管一件事

| 时间 | 唯一来源 | 当前值 | 责任 |
| --- | --- | --- | --- |
| 邮箱验证码 TTL | `message_mail_config.ttl_minutes` | 5 分钟 | Redis code、模板变量、Mail verification log、API `expiresAt` |
| 重发等待 | 现有 `business_email_minute` rate-limit policy | 1 次/60 秒 | 后端 Redis 强制；前端只显示等待 |
| delivery lease | Auth 代码常量 | 10 秒 | 防多实例同时处理同一邮箱发码，不是重发冷却 |

- 不在 AuthPlatform、system setting、环境变量或前端再增加邮箱 TTL。
- 不新增 cooldown key、cooldown Store、数据库字段或状态机。
- `provider timeout=8s`、`load lock=5s`、`mutation lease=30s/10s renew`、`cleanup timeout=1s` 保留为代码常量。
- Redis rate-limit policy 的现有 7 个固定 key、数据库表和管理页保持不变。

### 2. 目标发码顺序

```text
Auth policy/loginType check
  -> normalize email and derive PII-free verification key
  -> acquire 10s delivery lease
  -> Mail PrepareEmailVerifyCode
       -> readiness v2 (ready + ttlMinutes)
       -> recipient rule
       -> existing Redis rate limits exactly once
       -> return ttlMinutes + resendAfterSeconds
  -> generate six-digit code
  -> Auth Put replaces prior code only while current delivery lease is owned
  -> Mail SendPreparedEmailVerifyCode (must not rate-limit a second time)
  -> release owned lease
  -> return challengeId + expiresAt + resendAfterSeconds
```

失败顺序固定：

- Mail 预检返回 429 时尚未写新 code，旧验证码保持有效；释放本次 lease。
- readiness/Redis/rule/provider/PostgreSQL 依赖错误返回明确错误，不得伪装限流或成功。
- code Put 后任何发送失败，只删除本次 lease token 持有的新 code，再释放本次 lease。
- owned code/lease 清理失败返回 `503/10006` 并记录错误，不能返回原 provider 错误或假成功。
- 请求取消后的清理继续只用 `context.WithoutCancel(ctx) + 1s hard timeout`。

### 3. Mail 认证窄接口

接口名称可以按当前包风格微调，但语义不得改变：

```go
type VerifyCodeReadiness struct {
    Ready      bool
    TTLMinutes int
}

type EmailVerifyCodePrepareInput struct {
    PlatformID int64
    ClientIP   string
    Scene      string
    ToEmail    string
}

type EmailVerifyCodePreparation struct {
    TTLMinutes        int
    ResendAfterSeconds int
}

type EmailVerifyCodeInput struct {
    PlatformID  int64
    UserID      *int64
    ClientIP    string
    ChallengeID string
    Scene       string
    ToEmail     string
    Code        string
    ExpiresAt   time.Time
    Preparation EmailVerifyCodePreparation
}

type VerifyCodeSender interface {
    VerifyCodeReady(context.Context, int64, string) (VerifyCodeReadiness, error)
    PrepareEmailVerifyCode(context.Context, EmailVerifyCodePrepareInput) (EmailVerifyCodePreparation, error)
    SendPreparedEmailVerifyCode(context.Context, EmailVerifyCodeInput) (EmailVerifyCodeResult, error)
}
```

约束：

- Auth 只能依赖此窄接口，不能依赖 Mail Repository、GORM、Redis client、Config 或 Template model。
- `TTLMinutes` 必须来自 readiness v2 的 PostgreSQL-backed snapshot；Auth 不允许传入自己的 TTL。
- `ResendAfterSeconds` 来自当前 `business_email_minute.window_seconds`，默认 60；它只控制前端按钮等待，后端仍以
  所有现有限流维度为最终事实。
- Prepare 负责 recipient rule 和业务限流；SendPrepared 不得再次计数，否则一次请求会消耗两次额度。
- Prepare 成功即代表本次请求已经消耗 rate-limit 额度；后续 code/provider/数据库失败不回滚计数，不为限流实现
  补偿状态或第二套 Redis key。这是有意的故障保护行为。
- Preparation 是当前请求的 Mail 决策结果。配置在请求中途变化时，本次请求继续使用该结果；后续请求读取新
  readiness version。
- Auth 在 Prepare 成功后只捕获一次 `now`，由 Mail 返回的 TTL 计算 `ExpiresAt` 并传回 Mail；Mail 在 I/O 前校验
  该时间仍有效且不超过 preparation TTL，持久化同一个 verification `expires_at`，结果原样返回这个时间。

### 4. HTTP 与前端契约

成功响应：

```json
{
  "challengeId": "client-or-server-id",
  "expiresAt": "2026-09-07T10:05:00Z",
  "resendAfterSeconds": 60
}
```

- `expiresAt` 是验证码失效时间，不参与重发按钮倒计时。
- `resendAfterSeconds` 是正整数，前端 API 必须严格校验完整 key 集合、整数和合理范围（1–86400）。
- 登录页成功发送后直接用 `resendAfterSeconds` 启动倒计时；不得用时间戳相减、`Math.min(..., 900)` 或硬编码 60。
- 失败发送不启动倒计时，仍轮换下一次 challenge ID；通知继续由 `request.ts` 统一处理。

### 5. Redis 与部署契约

- Mail readiness payload 增加 `ttlMinutes`，schema 和 key namespace 从 v1 升到 v2；旧 v1 payload 不解析、不回退。
- Auth verification code key namespace 从 v1 升到 v2，避免滚动发布时旧 Put 语义阻塞新协议。
- rate-limit keys **不得**换 namespace，确保发布期间 60 秒额度连续生效。
- 旧 v1 验证码变为不可达并按原 TTL 自然过期；不扫描、不批量删除 Redis。
- code value 继续只含 HMAC digest 与 lease token，不增加邮箱、手机号、明文验证码或用户 PII。
- Put Lua 必须先确认 delivery lease 当前值等于调用 token，然后允许原子覆盖旧 code；没有 lease、token 不匹配、
  Redis 错误均失败，不能写入。
- Auth Store TTL 允许范围改为 1–60 分钟，与 Mail 数据库约束一致；超范围仍在访问 Redis 前拒绝。

### 6. 容量与故障预算

- `GET /login-config` readiness v2 ready 热命中 PostgreSQL 查询数为 0。
- SendCode 每次真实发送允许访问 PostgreSQL 写日志和 provider；readiness ready 不额外查配置/模板来决定 TTL。
- missing 风暴继续使用同实例 singleflight + 5 秒 tokenized Redis load lock，两个实例总共只允许一个 PostgreSQL
  readiness rebuild owner。
- Redis error/corrupt/invalidating 均 fail-closed；只有 missing 才允许有界回源。
- 至少使用两个 Auth Service、两个 Mail Service/Redis client 验证重发、单发送 winner 和故障清理。

---

## 二、文件地图

后端 Mail：

- `server/internal/module/message/mail/protocol.go`
- `server/internal/module/message/mail/readiness.go`
- `server/internal/module/message/mail/readiness_test.go`
- `server/internal/module/message/mail/service.go`
- `server/internal/module/message/mail/service_test.go`
- `server/internal/module/message/mail/service_concurrency_test.go`
- 必要时局部修改 `limiter.go` 测试接口，但不得新增另一套 limiter

后端 Auth：

- `server/internal/module/auth/login/protocol.go`
- `server/internal/module/auth/login/redis.go`
- `server/internal/module/auth/login/redis_test.go`
- `server/internal/module/auth/login/service.go`
- `server/internal/module/auth/login/service_test.go`
- `server/internal/module/auth/login/response.go`
- `server/internal/module/auth/login/handler.go`
- `server/internal/module/auth/login/handler_test.go`
- `server/internal/module/auth/login/concurrency_integration_test.go`

前端：

- `web/src/api/auth/login.ts`
- `web/tests/api/auth/login.test.ts`
- `web/src/views/auth/login/index.vue`
- `web/tests/views/auth/login/index.test.ts`

状态：

- `docs/agent/STATUS.md`

明确不修改：

- `docs/database/*.sql`
- `auth_platform` Model/Repository/migration
- Mail rate-limit policy 表、管理 API 与管理页面
- SMS、phone login、独立注册和 `registered_user` 权限数据
- `docs/learning/admin-core-architecture.html`

---

## 三、执行任务

## Task 1：Mail readiness v2 携带唯一 TTL

**先写失败测试：**

- ready snapshot 必须包含 `ready=true` 和 `ttlMinutes=5`。
- disabled config/template 返回 `ready=false`，不能伪造可用 TTL。
- TTL 为 0、负数、超过 60、缺失、unknown field、重复 JSON key、trailing data 均视为 corrupt，返回依赖错误。
- v1 payload/key 不能被 v2 解析；v2 missing 双实例并发只回源一组 config/template 查询。
- 100 次 ready 热读不增加 PostgreSQL 查询。
- config TTL 从 5 改 6 时，写路径先 invalidating，提交后用 mutation token 发布 v2 值；旧 owner 不能覆盖。

**实现：**

- 将 readiness snapshot schema/key/load-lock namespace 升到 v2。
- ready payload 增加 `TTLMinutes *int` 或等价严格字段；invalidating 不携带 ready/TTL。
- rebuild 从 `message_mail_config.ttl_minutes` 取值；仅 config/template 均启用时发布 ready+TTL。
- `VerifyCodeReady` 返回结构化 readiness；LoginConfig 只读取 `Ready`。

**定向验证：**

```powershell
cd D:\admin\server
go test ./internal/module/message/mail -run 'Test.*VerifyCodeReadiness|TestVerifyCodeReady' -count=1
```

## Task 2：Mail 预检先消费现有限流，发送阶段不重复计数

**先写失败测试：**

- Prepare 对 ready 邮箱返回 config TTL=5 和 `business_email_minute.window_seconds=60`。
- Prepare 依次校验 readiness、recipient rule、现有全部 business limit；任何依赖错误显式失败。
- 第二次同邮箱 Prepare 在额度窗口内返回 `429/10007`。
- Prepare 被限流后不创建 mail log、不调用 provider。
- SendPrepared 不再次调用 limiter；一份 preparation 只产生一次发送日志/provider call。
- `TTLMinutes` 同时进入模板变量和 `message_mail_log_verification.expires_at`。
- prepared input 的 `ExpiresAt` 必须仍在未来且不超过 preparation TTL；Mail log 使用该精确值并原样返回。
- 非 login scene、非法邮箱、非法准备结果、六位码非法继续返回 10001。

**实现：**

- 从当前 `send(..., SendModeBusiness)` 中抽取最小的 recipient/rate preflight，供 Prepare 使用。
- 普通业务 Mail Send 保持原行为；只有认证窄接口的 prepared send 跳过已执行的业务限流。
- 不新增 Redis key，不增加 reservation/cooldown 状态，不改变现有 7 条 policy。
- 从加载到的 catalog 取得 `business_email_minute.window_seconds` 作为 `ResendAfterSeconds`。

**定向验证：**

```powershell
cd D:\admin\server
go test ./internal/module/message/mail -run 'TestPrepareEmailVerifyCode|TestSendPreparedEmailVerifyCode|TestSend' -count=1
```

## Task 3：Auth Put 在当前 lease 下原子替换旧验证码

**先写失败测试：**

- key 有旧 code、当前 delivery lease 属于新 token 时，Put 原子覆盖并刷新为新 TTL。
- lease missing、lease mismatch 时不改变旧 code。
- Redis Eval 错误不改变成功语义并向上返回。
- v1 key 与 v2 key 隔离，key 仍不含明文 email/phone。
- TTL=1m、5m、60m 接受；小于 1m、超过 60m 在 Redis 前拒绝。
- 替换后旧 digest Check/Consume 失败，新 digest 只有一个 Consume winner。

**实现：**

- verification code namespace 升到 v2。
- Put Lua 删除“旧 code leaseToken 必须等于新 token”的错误约束；只以当前 delivery lease ownership 为写权限。
- 保留 JSON payload 严格解析、HMAC key 和 owned cleanup。

**定向验证：**

```powershell
cd D:\admin\server
go test ./internal/module/auth/login -run 'TestVerificationCodeStore' -count=1
```

## Task 4：Auth SendCode 使用 Mail 准备结果并完整补偿

**先写失败测试：**

- Mail 返回 TTL=5 时，Auth Put TTL、API ExpiresAt 和 Mail input 都是 5 分钟；代码中不再存在 10 分钟业务常量。
- 60 秒内第二次请求在 Put 前得到 429，原 code 仍可 Check/Consume。
- 额度允许后第二次请求可覆盖尚有 4 分钟的旧 code；旧码失败，新码成功。
- Prepare/readiness/rate-limit 失败只释放 lease，不删除旧 code。
- Put 失败释放 lease；prepared send 失败删除本次 code 并释放 lease。
- code/lease 任一清理失败最终为 `503/10006`；不得返回成功。
- 成功后 lease release 失败仍为 `503/10006`。
- request context 传到 Mail/Redis；补偿 context 只有 WithoutCancel+1s。

**实现：**

- 删除 `verificationCodeTTL = 10 * time.Minute`。
- 发码逻辑严格按本计划固定顺序编排。
- `expiresAt` 使用一次捕获的 `now.UTC().Add(preparation TTL)`，同一值传给 Mail 并进入响应，不使用另一套常量。
- `SendCodeResult/Response` 增加 `ResendAfterSeconds int`，Handler 精确输出 lower camel case。

**定向验证：**

```powershell
cd D:\admin\server
go test ./internal/module/auth/login -run 'TestServiceSendCode|TestHandlerSendCode|TestVerification' -count=1
```

## Task 5：前端严格区分 expiresAt 与 resendAfterSeconds

**先写失败测试：**

- DTO 缺少、多出或错误类型的 `resendAfterSeconds` 时抛 `ProtocolError`。
- 0、负数、非整数、超过 86400 被拒绝。
- 返回 `expiresAt=5m`、`resendAfterSeconds=60` 时按钮显示 60 秒而不是 300 秒。
- 60 秒结束允许再次发送，并使用新的 challenge ID。
- 发送失败不启动倒计时，页面不重复通知。

**实现：**

- `SendCodeResult` 增加严格整数 `resendAfterSeconds`。
- 登录页删除 `expiresAt - Date.now()` 和 900 秒 cap；直接使用服务端返回的重发秒数。
- 不增加下拉组件；若触及选项控件仍只能使用 `el-select-v2`。

**定向验证：**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/auth/login.test.ts tests/views/auth/login/index.test.ts --pool=threads --maxWorkers=1
pnpm typecheck
pnpm check:architecture
```

## Task 6：真实 Redis 双实例与部分失败兜底

测试放在现有集成/并发测试文件，不创建只验证 mock 的“并发测试”。至少覆盖：

1. 两个 Mail readiness Store/两个 Redis client，64 路 v2 missing 只回源一组 PostgreSQL 查询。
2. ready 热读 100 次零 PostgreSQL 查询。
3. 两个 Auth Service 同邮箱同时 SendCode，只允许一个进入 provider；另一个明确失败。
4. 首次成功后，在 rate window 内重发返回 429，旧验证码仍可消费。
5. 使用测试专属短窗口等待 Redis 额度恢复后重发成功，新验证码替代旧验证码；禁止让测试真实 sleep 60 秒，
   生产 1/60s 数值由 policy/readiness 单测验证。
6. 分别在 Prepare rate read、code Put、provider send、code cleanup、lease release 注入故障，检查 429/503 分离和
   owned cleanup。
7. PostgreSQL mail-log INSERT/verification INSERT/MarkSent 失败不能产生假成功。

测试必须使用独立 Service 实例和独立 Redis client；可共享真实 Redis server 和测试 PostgreSQL。每个 key 使用
唯一测试前缀/账号，清理仅处理本测试创建的精确 key，不做 pattern scan 或 FlushDB。

**定向验证：**

```powershell
cd D:\admin\server
go test ./internal/module/auth/login ./internal/module/message/mail -run 'Test.*TwoInstances|Test.*Integration|Test.*Concurrent|Test.*Fault' -count=1
```

## Task 7：终审、全量验证与状态记录

执行者先自查：

- [ ] `rg -n "verificationCodeTTL|10 \* time.Minute|TTLMinutes:.*verification" server/internal/module/auth/login`
  不再命中 Auth-owned 邮箱 TTL。
- [ ] `rg -n "cooldown|resend.*key" server web/src` 没有新增冷却存储协议。
- [ ] `rg -n "context.Background\(\)"` 在本轮生产调用链无新增命中。
- [ ] Mail limiter 对一次成功验证码发送只计数一次。
- [ ] Auth 不导入 Mail Repository/GORM/Redis model。
- [ ] v2 readiness corrupt/error/invalidating 均闭合，missing 回源有界。
- [ ] 前端不从 `expiresAt` 计算 resend countdown。
- [ ] 没有修改数据库 migration、SMS、注册或角色授权。

格式与全量验证：

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...

cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm lint
pnpm check:architecture
pnpm typecheck
pnpm build

cd D:\admin
git diff --check
git status --short
```

最后更新 `docs/agent/STATUS.md`，记录：

- 实际接口和 Redis namespace；
- 5 分钟 TTL 与 60 秒重发的验证证据；
- 双实例查询预算和 winner 数；
- 每个故障点的公开错误码；
- 实际执行的全量命令与未运行项；
- 剩余风险。

不要修改本计划勾选状态来代替 `STATUS.md` 的交付记录，不要 commit。

---

## 四、审查者终审清单

执行者交付后，审查者必须重新读完整 diff，并逐项给出 finding 或证据：

1. Mail DB TTL 是唯一业务事实，Auth/前端没有第二个邮箱 TTL。
2. readiness v2 的 ready/false/invalidating/corrupt/missing 结构严格，旧 v1 不能误读。
3. 60 秒内限流发生在 code replacement 前，原验证码不会被 429 请求烧毁。
4. 60 秒后确实能替换未过期验证码，不再返回 503 `code-exists`。
5. Put 只认当前 delivery lease owner，旧 owner/过期 token 不能覆盖。
6. Prepare 只用现有 `redis_rate` 和 policy，无 cooldown 状态；SendPrepared 不重复计数。
7. Redis/provider/PostgreSQL 部分失败无假成功，owned cleanup 失败提升为 503。
8. `expiresAt`、模板 TTL、verification log 和 Redis TTL 同源；`resendAfterSeconds` 只表示前端等待。
9. 公开响应、TypeScript DTO 与页面完全严格，无兼容 fallback、fake data 或硬编码 60。
10. request context 贯穿；只有补偿使用有界 WithoutCancel，不使用无界 Background。
11. 热 read 零 PostgreSQL 查询，missing 双实例只一个 rebuild owner。
12. 没有修改 SMS、独立注册、AuthPlatform、数据库 migration 或 `registered_user` 授权。
