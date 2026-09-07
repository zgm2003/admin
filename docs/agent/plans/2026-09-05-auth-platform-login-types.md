# 认证平台登录方式与验证码认证实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans，按任务逐项执行。步骤使用 checkbox 跟踪。当前项目未授权自动提交、推送或改写历史；执行者只保留可审查的工作区 diff。

**Goal:** 将认证平台配置的 email、phone、password 登录方式连接到当前 Auth、Mail、User、Session 和前端登录链路，并在百万级用户、多实例高并发下保证验证码一次性消费、并发注册一致性和 Redis/数据库故障闭合。

**Architecture:** auth_platform.login_types 是 PostgreSQL 中的策略事实，策略更新递增 policyVersion 并通过现有 Redis policy snapshot 发布。Auth 只依赖 Mail 暴露的认证窄接口；Auth 自己持有验证码 Redis key、HMAC 摘要、delivery lease、检查和原子消费，Mail 只负责渠道 readiness、模板、限流、供应商发送和邮件审计。验证码登录成功后复用现有 Session/auth-state/JWT 流程，首次身份创建在 User 一个事务中完成账号、默认角色、角色关系、Profile 和 Access version。

**Tech Stack:** Go 1.26、Gin、GORM、PostgreSQL、Redis、golang.org/x/sync/singleflight、Vue 3、TypeScript strict、Pinia、Element Plus、Vue I18n、Vitest。

**Spec:** docs/agent/design.md、docs/agent/architecture.md 和本计划的“固定契约与验收标准”。项目当前路由规定不创建 docs/superpowers/specs；本计划保存在 docs/agent/plans/。

## Global Constraints

- 容量基线是百万级用户、多实例、高并发；认证热路径不得把 PostgreSQL 当作每请求配置中心，不得依赖单进程唯一状态或无界缓存。
- PostgreSQL -> Redis -> 进程内缓存的权限层级保持不变；Redis 故障、版本无法确认、快照损坏或 invalidating 不返回旧缓存、空配置、默认策略或假成功。
- Redis policy key 命中有效 ready 快照时不得查询 PostgreSQL；只有 key 缺失且 Redis 可用时，才允许有界 singleflight + Redis 分布式锁回源一次。
- 普通调用固定为 router -> middleware -> handler -> service -> repository -> model -> PostgreSQL；Handler 不访问 GORM、Redis、Mail Repository 或第三方 SDK。
- 所有入口取得的 context.Context 必须贯穿 Service、Repository、GORM、Redis、Mail 和 Session；禁止 context.Background() 替代请求上下文，禁止把 *gin.Context 传入 Service。
- HTTP envelope 只有 code、data、message；前端 API 从 unknown 严格解析 DTO，不使用 any、as any 或 @ts-ignore。
- JSON/TypeScript 使用 lower camel case；Go 导出 initialism 使用 ID、HTTP、API；URL 使用小写资源名。
- Yes/No 使用 shared/yesno；维护表显式声明非空 created_at/updated_at 的 TIMESTAMPTZ；DDL 仅通过维护者执行的幂等 forward migration。
- 不新增 Adapter、Manager、Factory、Base*、DI 容器或跨模块万能接口。Auth 不导入 Mail Repository/GORM；Mail 不导入 Auth 包。
- 行为变化先写失败测试，确认失败后做最小实现，再运行定向测试；未授权不得 commit、push、fetch、pull、rebase、amend 或回退其他开发者改动。
- 当前工作区已有 web/tests/views/cloud/storage-object/index.test.ts 的未提交改动，执行者必须保留，不能纳入本任务。

---

## 固定契约与验收标准

### 登录方式存储与顺序

auth_platform.login_types 使用 JSONB NOT NULL，值是至少一个、最多三个的字符串数组，只允许 email、phone、password。服务层拒绝重复项并按固定顺序重排为 email -> phone -> password。现有行迁移为 ["email","password"]，不改变既有邮箱密码登录行为；phone 只有在未来 SMS 窄接口接入后才会成为有效方式。

Go 契约（结构体 JSON tag 按项目 lower camel case 现有写法实现）：

~~~go
type LoginType string

const (
    LoginTypeEmail    LoginType = "email"
    LoginTypePhone    LoginType = "phone"
    LoginTypePassword LoginType = "password"
)

type LoginTypeOption struct {
    Value LoginType
    Label string
}

type LoginConfig struct {
    LoginTypes    []LoginTypeOption
    AllowRegister bool
}
~~~

Policy.LoginTypes []LoginType 从 JSONB 解析得到。策略 JSON 解析使用 json.Decoder.DisallowUnknownFields；缺失、损坏、非法值和不完整数组均为依赖不可用，而不是回退默认登录方式。

### Auth 与 Mail 窄接口

Mail 认证接口只暴露邮箱能力，禁止暴露 Repository、GORM、Redis client 或 Mail model：

~~~go
type EmailVerifyCodeInput struct {
    PlatformID   int64
    UserID       *int64
    ClientIP     string
    ChallengeID  string
    Scene        string
    ToEmail      string
    Code         string
    TTLMinutes   int
}

type EmailVerifyCodeResult struct {
    LogID        int64
    ChallengeID  string
    ExpiresAt    time.Time
}

type VerifyCodeSender interface {
    VerifyCodeReady(context.Context, int64, string) (bool, error)
    SendEmailVerifyCode(context.Context, EmailVerifyCodeInput) (EmailVerifyCodeResult, error)
}

type SendCodeResult struct {
    ChallengeID string
    ExpiresAt   time.Time
}
~~~

VerifyCodeReady 的参数是 platformID 和 Mail scene；false 表示配置/模板未启用，错误表示 Mail/Redis/数据库依赖不可用。当前 Mail 只支持 SceneLogin；没有 SMS 实现时不能伪造 phone ready 或发送成功。

Auth 验证码存储接口：

~~~go
type VerificationCodeStore interface {
    AcquireDelivery(context.Context, string, string, time.Duration) (bool, error)
    Put(context.Context, string, string, string, time.Duration) error
    Check(context.Context, string, string) (bool, error)
    Consume(context.Context, string, string) (bool, error)
    DeleteIfOwned(context.Context, string, string) error
    ReleaseDelivery(context.Context, string, string) error
}

type VerifiedIdentityInput struct {
    IdentityKind string
    Account      string
    Username     string
    PasswordHash string
}
~~~

验证码 key 只含平台、scene、login type 和 HMAC 后的账号标识，不写入邮箱/手机号明文。Redis value 至少包含 digest、leaseToken 和过期语义；Check 只读，Consume 使用 Lua 在比较摘要、检查存在性和删除之间原子完成。

### HTTP 契约

公共路由全部位于 /api/v1/auth，由 X-Auth-Platform 决定平台：

~~~text
GET  /api/v1/auth/login-config
POST /api/v1/auth/send-code
POST /api/v1/auth/login
~~~

严格发码请求：

~~~json
{"account":"user@example.com","loginType":"email","scene":"login","challengeId":"client-id"}
~~~

challengeId 可为空，为空时由服务端生成；scene 只能为 login。登录请求严格为：

~~~json
{"loginType":"email","loginAccount":"user@example.com","code":"123456"}
~~~

或：

~~~json
{"loginType":"phone","loginAccount":"+8615671628271","code":"123456"}
~~~

或：

~~~json
{"loginType":"password","loginAccount":"user@example.com","password":"password"}
~~~

不保留 username、email、phone 旧字段别名；未知字段、缺失字段和登录方式与字段不匹配均为 10001。成功凭证为：

~~~json
{"accessToken":"...","expiresIn":900,"isNewUser":false}
~~~

平台策略或渠道依赖不可用为 503/10006；显式配置但不允许的登录方式为 403/10003；验证码错误、过期、重放和不存在账号且不允许注册统一为认证失败语义，不泄露账号是否存在。登录日志写入失败不阻断已完成的登录。

### 认证与并发不变量

- CurrentPolicy Redis 命中时零 PostgreSQL 查询；Redis 故障、损坏、版本无法确认、invalidating 返回 503/10006，不返回进程旧缓存。key 缺失只允许有界 singleflight + Redis 锁回源。
- 平台登录方式和 allowRegister 的任何实际变化都递增 policyVersion；只影响新的认证请求，不撤销已有 Session。已有 Session 仍按原有 PolicyVersion/auth-state 规则校验。
- 验证码发送失败必须删除自己写入的验证码并按 token 释放 delivery lease；删除失败记录日志并返回依赖不可用，不能返回假成功。
- 同一 key 的错误验证码不删除正确验证码；过期验证码不能成功；同一验证码只有一个 Consume 成功。
- 并发首次登录依赖 PostgreSQL 部分唯一索引作为最终防线；唯一键竞争后按邮箱或手机号重新查询用户，不能直接把冲突映射成失败。
- 自动注册事务同时创建 user_account、默认角色关系、permission_access_version(version=1) 和 user_profile，不使用 LOCK TABLE user_account。
- 没有 SMS 渠道时 phone 不进入有效 loginTypes；显式 phone 请求不能触发空数组、假发送或空用户创建。

## 文件地图

后端：
- docs/database/2026-09-05-auth-platform-login-types.sql：添加 auth_platform.login_types JSONB、CHECK、既有数据回填，并重建邮箱部分唯一索引以允许手机号用户使用 email=''。
- server/internal/secretkey/secretkey.go、secretkey_test.go：增加独立 VerificationCodeHMACKey HKDF purpose。
- server/internal/module/auth/platform/model.go、request.go、response.go、protocol.go、service.go、redis.go、schema.go 及测试：登录方式模型、严格 DTO、策略转换、快照校验和故障闭合。
- server/internal/module/message/mail/protocol.go、service.go、errors.go 及测试：实现 VerifyCodeSender，只复用 Send 的 Mail 业务能力。
- server/internal/module/auth/login/request.go、protocol.go、response.go、handler.go、route.go、service.go、redis.go（新增）及测试：登录配置、发码、验证码 store、发送 lease、验证码登录和统一 Session 签发。
- server/internal/module/user/account/model.go、repository.go、service.go、errors.go 及测试：手机号查询、空密码处理、无整表锁自动注册事务和唯一键竞争重查。
- server/internal/module/user/loginlog/model.go、service.go 及测试：email/phone 登录类型和 best-effort 记录策略。
- server/cmd/api/main.go、main_test.go：显式装配 Mail 窄接口、验证码 store 和新路由。
- server/internal/shared/i18n/catalog.go、catalog_test.go：登录方式、验证码和不可用状态文案。

前端：
- web/src/api/auth/login.ts、测试：LoginType、登录配置、发码、登录请求和严格响应解析。
- web/src/api/auth/platform.ts、测试：管理端创建/更新/list 的 loginTypes 字段。
- web/src/views/permission/auth-platforms/auth-platform-form.ts、auth-platform-view.ts、components/AuthPlatformDialog/*、对应测试：登录方式多选及至少一项校验。
- web/src/views/auth/login/index.vue、样式和测试：动态方式切换、邮箱验证码发送倒计时、密码/验证码表单、错误恢复和 isNewUser 透传。
- web/src/i18n/messages/zh-CN.ts、en-US.ts：登录方式、验证码和表单校验文案。

---

## Task 1: 先建立认证平台契约和迁移失败测试

Files:
- Create server/internal/module/auth/platform/login_type_test.go
- Modify server/internal/module/auth/platform/model.go、request.go、response.go、protocol.go、schema_test.go
- Create server/internal/database/auth_platform_login_types_migration_test.go
- Create docs/database/2026-09-05-auth-platform-login-types.sql
- Modify server/internal/module/auth/platform/service_test.go、repository_test.go

Interfaces: 产出 LoginType 常量、Policy.LoginTypes、canonical normalization 和 JSONB schema。

- [ ] Step 1: 写失败规范化测试。

~~~go
func TestNormalizeLoginTypesUsesCanonicalOrderAndRejectsDuplicates(t *testing.T) {
    got, err := normalizeLoginTypes([]LoginType{LoginTypePassword, LoginTypeEmail, LoginTypePhone})
    if err != nil || !reflect.DeepEqual(got, []LoginType{LoginTypeEmail, LoginTypePhone, LoginTypePassword}) {
        t.Fatalf("normalizeLoginTypes() = %#v, %v", got, err)
    }
    if _, err := normalizeLoginTypes([]LoginType{LoginTypeEmail, LoginTypeEmail}); err == nil {
        t.Fatal("duplicate login type was accepted")
    }
}
~~~

- [ ] Step 2: 运行失败测试。

运行：cd D:\admin\server; go test ./internal/module/auth/platform -run TestNormalizeLoginTypesUsesCanonicalOrderAndRejectsDuplicates -count=1

预期：FAIL，因为常量和 normalizer 尚不存在。

- [ ] Step 3: 增加模型、DTO 和迁移。Platform 增加 json.RawMessage LoginTypes；CreateInput、UpdateInput、Policy 增加 []LoginType LoginTypes；创建和更新 request 使用指针字段并要求 loginTypes。迁移必须幂等，增加 login_types JSONB NOT NULL DEFAULT '["email","password"]'，回填缺失或非法 legacy 值，增加数组长度 1..3 和允许值检查，然后删除并重建名为 ux_user_account_email_active、谓词为 email <> '' AND deleted_at IS NULL 的部分唯一索引。不能删除用户数据；schema.go 的 builtinAdmin 和 builtinCanvas 也必须显式写入 email,password 默认值。

- [ ] Step 4: 让 ValidatePlatform 和 policyFromModel 对损坏 JSON 显式失败，不静默默认。newListResponse 输出 canonical loginTypes。写 service/repository 测试验证策略变化递增 policyVersion、无变化不递增。

- [ ] Step 5: 运行定向测试和迁移测试。

运行：cd D:\admin\server; go test ./internal/module/auth/platform ./internal/database -run 'LoginType|AuthPlatformLoginTypes|PlatformSchema' -count=1

预期：PASS。维护者再用本地 PostgreSQL 执行 forward SQL，验证 admin 行保持 email,password，且邮箱索引谓词排除空字符串。

---

## Task 2: 收紧 AuthPlatform Redis 策略读取

Files:
- Modify server/internal/module/auth/platform/service.go、redis.go、errors.go
- Modify server/internal/module/auth/platform/service_test.go、redis_test.go

Interfaces: CurrentPolicy(context.Context, string) (Policy, error) 仍是 Auth 唯一策略依赖，但保证固定的缓存/故障语义。

- [ ] Step 1: 写 Redis error、corrupt snapshot、invalidating snapshot 和 ready 命中零查询测试。用带查询计数的 fake policy repository；错误断言 HTTP 503、CodeDependencyUnavailable，ready 命中断言 repository 调用为 0。

~~~go
func TestCurrentPolicyDoesNotUsePostgresWhenRedisSnapshotIsReady(t *testing.T) {
    store := newReadyPolicyStore(t)
    repository := &countingPolicyRepository{}
    service := newPolicyService(repository, store)
    _, err := service.CurrentPolicy(context.Background(), "admin")
    if err != nil || repository.findCalls != 0 {
        t.Fatalf("err=%v findCalls=%d", err, repository.findCalls)
    }
}
~~~

- [ ] Step 2: 运行失败测试，确认旧实现仍在 Redis 失败时回源 PostgreSQL。

运行：cd D:\admin\server; go test ./internal/module/auth/platform -run 'TestCurrentPolicy' -count=1

预期：新增 ready 命中/错误闭合断言失败。

- [ ] Step 3: 实现有界缺失 key 恢复：Redis read error 立即 dependency unavailable；key missing 时按 code 使用 singleflight，持有 5 秒 tokenized Redis lock 的实例只查 PostgreSQL 一次，校验 row，install ready，按 token 释放锁并最多重读一次；等待实例最多轮询 200ms，超时 503/10006。不能返回进程旧值。

- [ ] Step 4: malformed JSON、schema/version mismatch、不完整 loginTypes 和 invalidating 全部 dependency unavailable。已有 mutation API 可继续用 sessionUpdating；认证读路径必须满足 10006。

- [ ] Step 5: 运行 race 和故障测试。

运行：cd D:\admin\server; go test -race ./internal/module/auth/platform -run 'CurrentPolicy|PolicyStore' -count=1

预期：PASS；32 个并发缺失 key 调用最多一次 PostgreSQL 回源。

---

## Task 3: 对齐并实现 Mail 认证窄接口

Files:
- 开始前重新查看 server/internal/module/message/mail/protocol.go、service.go、model.go、repository.go，并确认另一 AI 的 Mail diff。
- Modify server/internal/module/message/mail/protocol.go、service.go、errors.go
- Modify server/internal/module/message/mail/service_test.go、service_concurrency_test.go、handler_test.go

Interfaces: 实现固定契约的 VerifyCodeSender；不导入 auth/login，不暴露 Mail repository 类型。

- [ ] Step 1: 写 fake sender/repository 的失败测试：缺少/禁用配置或 login template 时 VerifyCodeReady=false；repository/Redis policy failure 返回 error；SendEmailVerifyCode 把 scene、code、TTL、challenge ID、user ID 传给现有 Send。

- [ ] Step 2: 运行测试确认接口不存在。

运行：cd D:\admin\server; go test ./internal/module/message/mail -run 'VerifyCodeReady|SendEmailVerifyCode' -count=1

预期：FAIL，缺少接口/方法。

- [ ] Step 3: 实现 Mail service 方法。NormalizeRecipient；scene 只能是 SceneLogin；代码必须为六位数字；TTL 为正数；调用现有 Send，Variables 写入 code 和 ttl_minutes，保留 ChallengeID 幂等。phone 不声称 ready。

- [ ] Step 4: 验证 provider、Redis limiter、数据库失败均为 503/10006，结果不能报告 sent，不暴露 credentials/SQL。

- [ ] Step 5: 运行 Mail 定向及并发测试。

运行：cd D:\admin\server; go test ./internal/module/message/mail -run 'VerifyCode|Send|ConcurrentChallenge' -count=1

预期：PASS。若另一 AI 的最终接口不同，先补兼容测试并在本任务内部统一，Auth 只接受统一后的窄接口。

---

## Task 4: Auth 自有验证码 Redis Store 与 delivery lease

Files:
- Create server/internal/module/auth/login/redis.go
- Extend server/internal/module/auth/login/redis_test.go
- Modify server/internal/secretkey/secretkey.go、secretkey_test.go
- Modify server/internal/module/auth/login/protocol.go、errors.go

Interfaces: NewVerificationCodeStore(*redis.Client, []byte) 实现 VerificationCodeStore。

- [ ] Step 1: 写 Redis integration 失败测试，覆盖 AcquireDelivery token ownership、Put token mismatch、Check wrong code、Consume single winner、DeleteIfOwned、expired key 和 Redis unavailable。

~~~go
func TestVerificationCodeConsumeIsSingleUse(t *testing.T) {
    store := newVerificationStoreForTest(t)
    key := "auth:verify-code:v1:admin:login:email:test"
    if err := store.Put(context.Background(), key, "digest-a", "lease-a", time.Minute); err != nil {
        t.Fatal(err)
    }
    first, err := store.Consume(context.Background(), key, "digest-a")
    if err != nil || !first { t.Fatalf("first consume = %v,%v", first, err) }
    second, err := store.Consume(context.Background(), key, "digest-a")
    if err != nil || second { t.Fatalf("replay consume = %v,%v", second, err) }
}
~~~

- [ ] Step 2: 运行测试确认 store 不存在。

运行：cd D:\admin\server; go test ./internal/module/auth/login -run 'VerificationCode|Delivery' -count=1

预期：FAIL，缺少 store symbols。

- [ ] Step 3: 增加 HKDF purpose admin:auth:verification-code-hmac:v1，暴露 copy-returning VerificationCodeHMACKey()，测试它与其他 purpose 派生值不同。

- [ ] Step 4: 用 Lua 实现 compare-and-delete。Consume 只在 digest 完全匹配时删除；错误 digest 返回 false 且保留正确码。Put 必须校验当前 delivery lease token，并拒绝覆盖另一个 token 持有的未过期验证码。DeleteIfOwned 和 ReleaseDelivery 比较 lease token。key 只使用 HMAC 账号摘要，TTL 限制在 1..15 分钟，lease TTL 10 秒。

- [ ] Step 5: 运行 race 测试。

运行：cd D:\admin\server; go test -race ./internal/module/auth/login -run 'VerificationCode|Delivery' -count=1

预期：PASS；Redis 不可用时只返回依赖错误，-short 只跳过真实 Redis 探针，不跳过 key-format/unit 测试。

---

## Task 5: User 身份查找和无整表锁自动注册事务

Files:
- Modify server/internal/module/user/account/model.go、repository.go、service.go、errors.go
- Modify server/internal/module/user/account/repository_test.go、service_test.go、model_test.go
- 如需插入 Profile，只能在同一事务内用 SQL helper，不能让 account import profile（profile 已依赖 account）。

Interfaces:

~~~go
func (r *Repository) FindCredentialByPhone(context.Context, string) (Credential, error)
func (r *Repository) FindCredentialByIdentity(context.Context, string, string) (Credential, error)
func (r *Repository) CreateVerifiedIdentity(context.Context, VerifiedIdentityInput) (User, error)
~~~

VerifiedIdentityInput 包含受控字符串 IdentityKind（仅允许 email 或 phone）、normalized Account、deterministic Username 和 PasswordHash（passwordless 为空）。Account 包不能导入 Auth 包；Auth 在 Service 层把 LoginType 映射成 IdentityKind。现有手工注册调用不改变。

- [ ] Step 1: 写失败测试：phone lookup、phone 用户 email=''、空 PasswordHash、Profile、Access version=1、恰好一个 enabled default role，以及新方法不执行 LOCK TABLE user_account。

- [ ] Step 2: 运行旧实现确认失败。

运行：cd D:\admin\server; go test ./internal/module/user/account -run 'CredentialByPhone|CreateVerifiedIdentity|Profile|AccessVersion' -count=1

预期：FAIL，缺少 phone lookup 和专用事务。

- [ ] Step 3: 增加 active-row lookup。email 使用 email <> '' AND deleted_at IS NULL；phone 使用 phone = ? AND deleted_at IS NULL；select password_hash/is_enabled。

- [ ] Step 4: 在一个 Transaction 内用 FOR UPDATE 锁 exactly-one enabled default role，插入 user_account、permission_user_role、permission_access_version(version=1) 和 user_profile（空 avatar/default gender，同一 UTC microsecond timestamp）。不锁 user_account 整表；由 PostgreSQL unique index 处理竞争。

- [ ] Step 5: 保留并识别 ErrEmailConflict、ErrPhoneConflict、ErrUsernameConflict；Auth 遇到邮箱/手机号冲突必须 requery，不用 SQL 字符串判断。

- [ ] Step 6: 运行 32 并发账号测试。

运行：cd D:\admin\server; go test -race ./internal/module/user/account -run 'CreateVerifiedIdentity|Concurrent|Unique' -count=1

预期：PASS；同一身份只有一个 active user、role、Profile、Access version。

---

## Task 6: Auth effective login config 和 send-code

Files:
- Modify server/internal/module/auth/login/protocol.go、request.go、response.go、service.go、errors.go
- Modify server/internal/module/auth/login/service_test.go、handler_test.go、redis_test.go

Interfaces:

~~~go
type SendCodeInput struct {
    Account     string
    LoginType   LoginType
    Scene       string
    ChallengeID string
    Client      authclient.Client
}

type LoginInput struct {
    LoginType    LoginType
    LoginAccount string
    Password     string
    Code         string
    Client       authclient.Client
}

func (s *Service) LoginConfig(context.Context, authclient.Client) (LoginConfig, error)
func (s *Service) SendCode(context.Context, SendCodeInput) (SendCodeResult, error)
~~~

- [ ] Step 1: 写失败测试：canonical email,phone,password 顺序；无 SMS sender 时过滤 phone；Mail disabled 过滤但不伪造；readiness error 为 10006；空 effective set 为 10006。

- [ ] Step 2: 运行测试确认方法缺失。

运行：cd D:\admin\server; go test ./internal/module/auth/login -run 'LoginConfig|SendCode' -count=1

预期：FAIL。

- [ ] Step 3: 实现 LoginConfig。CurrentPolicy 只加载一次；email 调 Mail VerifyCodeReady；phone 要求真实 SMS sender（当前无，因此过滤）；password 直接加入。readiness error 为 503/10006，false 过滤，最终空集合为 503/10006。Localized label 在 Handler 用 request context locale 构造，Service 不硬编码文案。

- [ ] Step 4: 实现 SendCode 顺序：validate scene=login -> assert configured/effective type -> normalize email 或 phone -> readiness -> crypto/rand 六位码 -> HMAC digest/key -> acquire 10 秒 lease -> Put digest -> SendEmailVerifyCode；发送任一错误先 DeleteIfOwned 和 ReleaseDelivery，再返回依赖/provider error。只返回 expiresAt/TTL 和 challenge metadata，不返回 code。

- [ ] Step 5: 写部分失败测试。Mail 失败、Redis Put 失败、Redis release 失败、重复 ChallengeID 和并发发送都不能返回 sent success。清理失败日志并映射 10006。

- [ ] Step 6: 运行 race 测试。

运行：cd D:\admin\server; go test -race ./internal/module/auth/login -run 'LoginConfig|SendCode|Cleanup|Challenge' -count=1

预期：PASS；code store 只保存 digest，账号/IP/scene 限流仍由 Mail 既有分布式限流保护。

---

## Task 7: Code/password login 与统一 Session 签发

Files:
- Modify server/internal/module/auth/login/service.go、response.go、handler.go
- Modify server/internal/module/auth/login/service_test.go、handler_test.go、password_test.go、token_test.go
- Modify server/internal/module/user/loginlog/model.go、service.go、测试

Interfaces: Credential 增加 IsNewUser bool，refresh credential 固定 false。把已有成功 password login 尾部提取为：

~~~go
func (s *Service) issueCredential(context.Context, authclient.Client, authplatform.Policy, user.Credential, string, bool) (Credential, error)
~~~

该 helper 负责 auth-state readiness、session limit/revocation、snapshot publish、JWT issue 和 best-effort login log。

- [ ] Step 1: 写失败测试：空 PasswordHash 的 password login 必须拒绝；existing email code、phone code、首次 email/phone code（isNewUser=true）、disabled user、wrong/expired/replay code、policy disabled。

- [ ] Step 2: 运行旧实现确认失败。

运行：cd D:\admin\server; go test ./internal/module/auth/login -run 'Password|Code|NewUser|Replay|Disabled' -count=1

预期：FAIL。

- [ ] Step 3: 实现 Login dispatcher。加载 policy 一次，校验 effective login type，规范化 account。Password 只查 email，空 hash 在 bcrypt 前拒绝；code 先 Check 再查 user，检查 enabled；账号不存在时先检查 allowRegister 再 Consume，之后调用 CreateVerifiedIdentity。Consume 失败不能进入 Session。

- [ ] Step 4: 处理并发 registration race。CreateVerifiedIdentity 返回 ErrEmailConflict/ErrPhoneConflict 时 requery 同一 normalized identity；winner enabled 则 issueCredential(isNewUser=false)，缺失/disabled 映射认证错误。不得重用已消费 code 或创建第二份 role/Profile/version。

- [ ] Step 5: recordLoginEvent 改为 best-effort。持久化失败用 request context 结构化 warning 并 return nil；success、invalid credentials、disabled、replay、policy denial 都记录。增加 LoginEmail、LoginPhone constants。

- [ ] Step 6: 运行定向测试。

运行：cd D:\admin\server; go test -race ./internal/module/auth/login ./internal/module/user/loginlog -run 'Login|Refresh|Session|Code|NewUser' -count=1

预期：PASS；snapshot 使用当前 policy version，new user access version 为 1，禁用方式不撤销既有 Session，login-log outage 不改变成功响应。

---

## Task 8: 严格 Handler、路由和显式装配

Files:
- Modify server/internal/module/auth/login/handler.go、route.go、request.go、response.go
- Modify server/cmd/api/main.go、main_test.go
- Modify server/internal/shared/i18n/catalog.go、catalog_test.go

Interfaces: Handler 增加 LoginConfig、SendCode 和更新后的 Login；全部传入 c.Request.Context()。/api/v1/auth 下注册 GET config、POST send-code、POST login，再保留 refresh/logout/me。

- [ ] Step 1: 写失败 Handler/route 测试。断言 route methods、旧 email/password 和 unknown fields 被拒绝、lower camel envelope、localized labels、send response 不含 code、login response 含 isNewUser。

- [ ] Step 2: 运行测试确认旧 Handler 不满足。

运行：cd D:\admin\server; go test ./internal/module/auth/login ./cmd/api -run 'Route|LoginConfig|SendCode|Strict|NewUser' -count=1

预期：FAIL。

- [ ] Step 3: 使用 pointer-backed request fields 和 validate.BindJSON；Service 做 login-type-specific 校验；labels 使用 i18n.LocaleFromContext。

- [ ] Step 4: main.go 显式装配 verificationStore := auth.NewVerificationCodeStore(redisClient, keys.VerificationCodeHMACKey())，调用 SetVerifyCodeSender(mailService) 和 SetVerificationCodeStore(verificationStore)。Auth 不接收 mailRepository 或 redis.UniversalClient。

- [ ] Step 5: 运行 Handler/assembly 测试。

运行：cd D:\admin\server; go test ./internal/module/auth/login ./cmd/api -run 'Route|Handler|Assembly' -count=1

预期：PASS；公共路由不附加管理端权限中间件。

---

## Task 9: AuthPlatform 管理端 API/UI

Files:
- Modify web/src/api/auth/platform.ts
- Modify web/src/views/permission/auth-platforms/auth-platform-form.ts、auth-platform-view.ts、components/AuthPlatformDialog/types.ts、components/AuthPlatformDialog/index.vue
- Modify web/tests/api/auth/platform.test.ts、web/tests/views/permission/auth-platforms/index.test.ts
- Modify web/src/i18n/messages/zh-CN.ts、en-US.ts

Interfaces: list/create/update DTO 都增加 loginTypes: LoginType[]；parser 接受且只接受 email|phone|password，拒绝 duplicate/empty/unknown，并发送 lower camel case loginTypes。

- [ ] Step 1: 写失败 Vitest：list/create/update 的 canonical loginTypes、malformed array 抛 ProtocolError、dialog 至少一项、builtin Admin 仍强制 allowRegister=No。

- [ ] Step 2: 运行定向 Vitest 确认缺字段失败。

运行：cd D:\admin\web; pnpm vitest run tests/api/auth/platform.test.ts tests/views/permission/auth-platforms/index.test.ts --pool=threads --maxWorkers=1

预期：FAIL。

- [ ] Step 3: 用现有 protocol helpers 和 typed parseLoginTypes 实现 strict parser；UI 使用 i18n 多选，空数组禁用保存，不默认 fake method list。

- [ ] Step 4: 运行测试和 build。

运行：cd D:\admin\web; pnpm vitest run tests/api/auth/platform.test.ts tests/views/permission/auth-platforms/index.test.ts --pool=threads --maxWorkers=1; pnpm build

预期：PASS，strict TypeScript 编译通过。

---

## Task 10: 动态登录页和验证码模式

Files:
- Modify web/src/api/auth/login.ts、web/tests/api/auth/login.test.ts
- Modify web/src/views/auth/login/index.vue（页面内 scoped style 一并调整）、web/tests/views/auth/login/index.test.ts
- Modify web/src/store/auth.ts only if AccessCredential needs typed isNewUser；不存储验证码。
- Modify web/src/i18n/messages/zh-CN.ts、en-US.ts

Interfaces: getLoginConfig、sendLoginCode、login(LoginInput) 使用 discriminated union。AccessCredential 为 accessToken:string、expiresIn:number、isNewUser:boolean。

- [ ] Step 1: 写失败 page/API 测试：config failure、password mode、email/code mode、phone 未返回时不显示、server TTL countdown、resend、wrong-code inline error、10006 notification pass-through、safe redirect。

- [ ] Step 2: 运行定向测试确认旧页面失败。

运行：cd D:\admin\web; pnpm vitest run tests/api/auth/login.test.ts tests/views/auth/login/index.test.ts --pool=threads --maxWorkers=1

预期：FAIL。

- [ ] Step 3: API 从 unknown 严格解析 config option、send result（challengeId、expiresAt）和 credential；拒绝 extra keys、非整数 TTL、非 boolean isNewUser、未由服务端返回的 phone option。

- [ ] Step 4: 页面 mount 加载 config，选择第一个 server method，只显示对应控件；发码使用稳定 challenge ID 和 server TTL；send failure 停止/重置倒计时；pending 阻止重复提交；切换模式清理旧错误；不记录/显示 code，保留 auth-store 和 safe redirect 顺序。

- [ ] Step 5: 运行 page tests。

运行：cd D:\admin\web; pnpm vitest run tests/api/auth/login.test.ts tests/views/auth/login/index.test.ts --pool=threads --maxWorkers=1

预期：PASS；窄屏无重叠，无隐藏 fake phone 控件，无凭据错误重复通知。

---

## Task 11: RBAC、Access version、Session 和 LoginLog 回归

Files:
- Modify server/internal/module/auth/platform/service_test.go、server/internal/module/auth/login/service_test.go
- Modify server/internal/module/permission/access/*_test.go、server/internal/module/user/session/*_test.go
- Modify server/internal/module/user/loginlog/*_test.go
- 如新增操作日志路由分类，只修改 server/internal/module/system/operationlog/*_test.go

Interfaces: 公共登录路由不新增权限码；管理端编辑继续 auth:platform:update；页面进入/list/update 权限保持分离。登录方式 mutation 发布更高 policy snapshot，但不修改 user Access version 或撤销既有 Session。

- [ ] Step 1: 写两个 policy service、两个 Redis client 的 integration tests：update version CAS、stale publisher rejection、invalidating read failure、Access snapshot continuity、禁用方式后既有 Session 仍有效。
- [ ] Step 2: RBAC 断言：未认证只可到 public auth config/send/login；无 auth:platform:update 的管理请求返回 10003；hidden action 不进 menuTree；public login endpoint 不使用 admin permission middleware。
- [ ] Step 3: LoginLog 断言：success/failure 的 loginType 为 email|phone|password；强制 user_login_log insert error 时成功 credential 仍返回并写结构化 warning。
- [ ] Step 4: 运行回归测试。

运行：cd D:\admin\server; go test -race ./internal/module/auth/... ./internal/module/permission/access ./internal/module/user/session ./internal/module/user/loginlog ./internal/module/system/operationlog -count=1

预期：PASS；Redis/PostgreSQL integration skip 必须在 STATUS.md 记录准确原因。

---

## Task 12: 多实例并发和故障探针

Files:
- Create server/internal/module/auth/login/concurrency_integration_test.go
- Create server/internal/module/auth/platform/concurrency_integration_test.go
- 如现有探针约定需要外部进程，Create scripts/auth-login-concurrency.ps1
- Modify docs/agent/STATUS.md

Interfaces: 探针只使用公共 service/HTTP contract 和本地 PostgreSQL/Redis fixture；Mail sender 为可控 fake，不连接第三方供应商。

- [ ] Step 1: 32 请求同一 email/phone 的首次登录，使用两个 Auth service instance 和两个 Redis client，使用不同有效 code。断言一个 active user、一个 role、一个 Profile、一个 Access version；所有 registration loser requery winner。
- [ ] Step 2: 32 个 Consume 同一 code，断言恰好一个成功；wrong code 不删除正确 code；TTL 后全部失败。
- [ ] Step 3: Redis 在 policy read、code Put、code Consume、lease release 阶段故障；PostgreSQL 在 identity create 和 login-log insert 阶段故障。断言依赖错误为 10006、无 stale policy/code success、无半用户，且 login-log only failure 不阻断登录。
- [ ] Step 4: ready policy snapshot 下 100 次 login-config 读取断言 policy PostgreSQL 查询为 0；missing key recovery window 内每个 code 最多一次回源。
- [ ] Step 5: 本地服务可用时执行：

运行：cd D:\admin\server; go test -race ./internal/module/auth/login ./internal/module/auth/platform -run 'Concurrency|Probe|Fault|Budget' -count=1

预期：PASS。服务不可用时不能用 skip 隐藏失败，必须在 STATUS.md 写明命令、阻塞和剩余风险。

---

## Task 13: 执行者交接与强制 review checkpoint

Files:
- Review all files changed by Tasks 1-12; no unrelated cleanup.
- Update docs/agent/STATUS.md

Interfaces: 执行者返回 diff summary、测试输出、migration command/result 和明确 unrun list；主 reviewer 在接受前逐条检查。

- [ ] Step 1: 停止实现并捕获状态。

运行：cd D:\admin; git status --short; git diff --stat; git diff --check

预期：既有 storage-object test edit 未触碰；没有 .env、secret、build artifact 或无关文档进入改动。

- [ ] Step 2: 按严重度 review 并记录 file/line evidence：
  - CurrentPolicy 对 Redis error/corrupt/invalidating 不回退，不返回进程旧缓存。
  - policy update 递增 policyVersion，旧 publisher 不能覆盖新 loginTypes。
  - Mail 只经 VerifyCodeSender；Auth 不依赖 Mail Repository/GORM。
  - code key 无 PII；错误码不删除正确码；Consume Lua 原子且一次性。
  - provider/Redis partial failure 清理 owned code/lease；cleanup failure 可见为 10006。
  - phone-only 用户 email=''、空 hash、Profile、Access version；password login 拒绝空 hash。
  - 自动注册只锁 enabled default role，不锁 user_account 整表。
  - email/phone unique competition requery winner，不返回假冲突。
  - Session/auth-state/JWT 使用当前 policy version；关闭方式不撤销已有 Session。
  - LoginLog 失败不能改变成功登录响应。
  - 每个 Service/Repository/Redis/Mail call 传 request context。
  - 前端 DTO strict，dynamic UI 不显示不可用 phone 或 fake data。
  - Public routes、management RBAC、Access version、hidden action 分离。
- [ ] Step 3: 每个 review finding 先补 regression test 再修复；排序为 auth bypass、data integrity/concurrency、availability/fault、protocol/RBAC、style。不得绕过失败测试。
- [ ] Step 4: 每个 review fix 后重跑对应 task test，在 STATUS.md 写 finding、file、test、result；不修改历史计划。

---

## Task 14: 全量验证和 Agent 框架维护

Files:
- Modify docs/agent/STATUS.md
- 除非稳定规则确实变化，不修改 docs/agent/README.md、design.md、architecture.md。

- [ ] Step 1: 后端格式、静态检查、测试和构建。

~~~powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
~~~

预期：全部 exit 0；若缺 PostgreSQL/Redis，保留首个失败 package 和原始原因。

- [ ] Step 2: 前端验证。

~~~powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
~~~

预期：全部 exit 0，strict DTO 编译通过。

- [ ] Step 3: 维护者执行 migration，DSN 不打印：

~~~powershell
$dsn = ((Get-Content server/.env | Where-Object { $_ -match '^POSTGRES_DSN=' }) -replace '^POSTGRES_DSN=','') -replace ' TimeZone=.*$',''
& 'D:\FlyEnv-Data\env\postgresql\bin\psql.exe' "$dsn" -v ON_ERROR_STOP=1 -f docs/database/2026-09-05-auth-platform-login-types.sql
~~~

只查询结构事实：auth_platform.login_types、CHECK、active email 部分索引谓词、exactly-one default role 和 fixture user 的 Profile/Access rows；日志不得出现 DSN 或 secrets。

- [ ] Step 4: 更新 STATUS.md：review/verification 通过后才从当前工作移到已完成；写实际命令、结果、迁移日期、跳过的真实探针、SMS 依赖和未改动的 storage-object test。
- [ ] Step 5: 最终状态检查。

运行：cd D:\admin; git status --short; git diff --check

预期：只有本任务文件和实现变更加既有 storage-object test edit；未经单独授权不创建 commit。

---

## 计划自审

- 规格覆盖：登录方式策略、Mail readiness/send 窄接口、Redis 验证码原子消费、发送失败清理、并发注册、默认角色竞态、Profile/Access version、Session/Policy version、RBAC、LoginLog、前端 strict DTO、迁移、并发/故障探针和 Agent 状态均有任务。
- 失败闭合：Redis 故障/快照损坏/版本不明/invalidating、Mail/provider 失败、验证码 replay/wrong/expired、PostgreSQL unique competition、Profile/Access 写入失败、LoginLog 失败均有明确响应或事务语义。
- 类型一致：LoginType、LoginConfig、VerifyCodeSender、VerificationCodeStore、SendCodeInput、LoginInput 和 Credential.IsNewUser 在契约章节定义，后续任务只引用这些名字。
- 范围控制：SMS 未接入前 phone 只作为可配置但不可用的方式被过滤；本计划不增加短信供应商、不改已有 Session 撤销策略、不把 Mail Repository 暴露给 Auth。
