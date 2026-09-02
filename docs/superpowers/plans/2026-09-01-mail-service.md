# 邮件服务模块 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Admin 平台建立腾讯云 SES 邮件基础设施，提供四个固定业务模板、同步发送事实、诊断日志、收件人黑白名单、双层 Redis 限流和四 Tab 管理页面，同时保证凭据、验证码和平台数据隔离。

**Architecture:** 延续 `router -> middleware -> handler -> service -> repository -> PostgreSQL` 的模块链路。邮件 Service 统一编排模板、配置、规则、限流、验证码快照和腾讯云 SES 同步调用；腾讯云调用只封装在 `server/internal/storage/mail`，不引入多供应商抽象。PostgreSQL 保存平台隔离的邮件事实，Redis 保存短期读取缓存和限流计数，API/Worker 启动只注册组件，不执行迁移或默认数据写入。

**Tech Stack:** Go 1.26.5、Gin、GORM、PostgreSQL、go-redis/v9、腾讯云 `tencentcloud-sdk-go` SES、`redis/go-redis_rate/v10`、Vue 3、TypeScript、Element Plus、Pinia、Vue Router、Vue I18n、Vitest。

**Spec:** `docs/superpowers/specs/2026-09-01-mail-service-design.md`

## Global Constraints

- 邮件管理页面挂在 `消息服务 -> 邮件服务`，页面权限固定为 `system:mail:view`，四个 Tab 不创建左侧子菜单。
- `:view` 只控制页面入口；读取使用 `system:mail:list`/`system:mail:detail`，写操作使用各自 action，页面权限不自动扩展读取或操作权限。
- 固定模板只有 `login=47941`、`forget=47942`、`bind_email=47943`、`change_password=47944`；不提供模板新增、删除或场景改名接口。
- 所有邮件表必须有 `platform_id` 外键和平台范围条件；Admin API 固定使用 `auth_platform.code='admin'`，拒绝客户端传入平台 ID。
- API/Worker 启动禁止 AutoMigrate、EnsureSchema、默认模板初始化、菜单写入、回填和 Redis 清理；数据库结构和初始四模板由一次性人工 SQL 完成。
- SecretId/SecretKey 使用独立用途的 AES-256-GCM 派生密钥；验证码只保存密文快照，公开响应、普通日志和操作日志不得出现密钥、密文、验证码或模板变量原文。
- Admin 端可以显示完整邮箱、验证码和腾讯云诊断，但仅限受保护的邮件日志 DTO；公开业务接口不得返回这些字段或规则命中细节。
- 公开业务发送和 Admin 测试发送都必须经过黑白名单和限流；Redis 不可用时两类发送均 fail-closed，不退化为进程内放行。
- 发送使用带 8 秒超时的同步 SES 调用；状态严格为 `pending -> sent/failed`，不做供应商 fallback、队列、outbox 或本地正文渲染。
- 所有实现先写失败测试，再写最小实现，再运行相关测试；每个任务独立提交，标题符合 `<type>: <中文摘要>`，不带 scope、句号或 `wip`。

## 文件责任地图

| 责任 | 文件 |
| --- | --- |
| 邮件模型、常量、表名和结构契约 | `server/internal/module/system/mail/model.go`、`schema.go` |
| 一次性数据库结构和四条初始模板 | `docs/database/2026-09-01-mail-service.sql` |
| 邮件专用密钥派生 | `server/internal/secretkey/secretkey.go` |
| 腾讯云 SES 请求封装 | `server/internal/storage/mail/tencent_ses.go` |
| 配置、模板、日志、验证码快照的持久化 | `server/internal/module/system/mail/repository.go` |
| 发送编排和公开业务接口 | `server/internal/module/system/mail/service.go`、`protocol.go` |
| 收件人规则匹配 | `server/internal/module/system/mail/rules.go` |
| Redis 业务限流和 HTTP 快速拒绝 | `server/internal/module/system/mail/limiter.go`、`server/internal/middleware/ratelimit.go` |
| Admin DTO、处理器和路由 | `server/internal/module/system/mail/request.go`、`response.go`、`handler.go`、`route.go` |
| API 依赖注入和路由注册 | `server/cmd/api/main.go`、`server/cmd/api/main_test.go` |
| 操作日志动作和敏感字段策略 | `server/internal/module/system/operationlog/rules.go` |
| 前端 API、动态路由和四 Tab 页面 | `web/src/api/system/mail.ts`、`web/src/views/message/mail/index.vue`、`web/src/router/permission-routes.ts` |
| 邮件菜单和隐藏 action 菜单 | `docs/database/2026-09-01-mail-menu.sql` |

## 依赖顺序

```text
数据库模型与人工 SQL
        -> MailEncryptionKey
        -> SES client + 固定模板协议
        -> 规则服务 + Redis 限流
        -> Mail service（配置、模板、日志、同步发送）
        -> Admin handler/route/operation log
        -> 前端四 Tab 与权限显示
        -> 菜单人工 SQL
        -> 全量测试与真实人工验收
```

### Task 1: 建立邮件数据模型、约束和一次性 SQL

**Files:**
- Create: `server/internal/module/system/mail/model.go`
- Create: `server/internal/module/system/mail/schema.go`
- Create: `server/internal/module/system/mail/model_test.go`
- Create: `server/internal/module/system/mail/schema_test.go`
- Create: `server/internal/database/mail_service_migration_test.go`
- Create: `docs/database/2026-09-01-mail-service.sql`

**Interfaces:**
- Produces `mail.Config`, `mail.Template`, `mail.Log`, `mail.Verification`, `mail.RecipientRule` GORM models and `TableName()` methods for `system_mail_config`, `system_mail_template`, `system_mail_log`, `system_mail_log_verification`, `system_mail_recipient_rule`.
- Produces constants `SceneLogin`, `SceneForget`, `SceneBindEmail`, `SceneChangePassword`, `StatusPending`, `StatusSent`, `StatusFailed`, `RuleScopeEmail`, `RuleScopeDomain`, `RuleActionAllow`, `RuleActionDeny` and `FixedTemplates()` returning the four immutable scene/ID definitions.
- Produces `ValidateScene(scene string) error` and `ValidateStatus(status string) error`; these are pure validation helpers and do not connect to PostgreSQL.

- [ ] **Step 1: Write failing model and fixed-template tests.**

  In `model_test.go`, assert the exact table names, four template IDs and variables, and reject an unknown scene or status:

  ```go
  func TestFixedTemplatesHaveStableTencentIDs(t *testing.T) {
      got := FixedTemplates()
      want := map[string]int{SceneLogin: 47941, SceneForget: 47942, SceneBindEmail: 47943, SceneChangePassword: 47944}
      for _, item := range got {
          if want[item.Scene] != item.TencentTemplateID {
              t.Fatalf("scene %q has template id %d", item.Scene, item.TencentTemplateID)
          }
      }
      if len(got) != 4 || len(got[0].Variables) != 2 {
          t.Fatalf("fixed template contract changed: %+v", got)
      }
  }
  ```

- [ ] **Step 2: Run the focused test and verify it fails because the mail package does not exist.**

  Run from `D:\admin\server`:

  ```powershell
  go test ./internal/module/system/mail -run 'TestFixedTemplatesHaveStableTencentIDs|TestMailTableNames' -count=1
  ```

  Expected result: FAIL with missing package/types or missing fixed-template definitions.

- [ ] **Step 3: Add models and schema constants without runtime migration.**

  Define the model fields exactly as the spec requires. `Log` must contain no body or plaintext verification field; `Verification` stores `KeyVersion`, `CodeCiphertext`, `ExpiresAt`; every model includes `PlatformID` and `gorm.DeletedAt` where the table is deletable. `schema.go` exposes only table names and fixed scene metadata for tests; it must not export `AutoMigrate`, `EnsureSchema`, `Seed`, or startup hooks.

  The log also stores the caller-supplied `ChallengeID` for idempotency, and the active uniqueness contract is represented in the model tests as:

  ```go
  type Log struct {
      ID int64 `gorm:"column:id;primaryKey;autoIncrement"`
      PlatformID int64 `gorm:"column:platform_id;not null"`
      ChallengeID *string `gorm:"column:challenge_id;type:varchar(128)"`
      Scene string `gorm:"column:scene;type:varchar(32);not null"`
      TemplateID int `gorm:"column:template_id;not null"`
      ToEmail string `gorm:"column:to_email;type:varchar(254);not null"`
      Status string `gorm:"column:status;type:varchar(16);not null"`
      DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
  }
  ```

- [ ] **Step 4: Add the complete PostgreSQL SQL migration.**

  `docs/database/2026-09-01-mail-service.sql` must execute in one transaction and create all five tables, foreign keys to `auth_platform(id)`, `SMALLINT` Yes/No checks, status/scene checks, timestamp columns, soft-delete columns, and these partial unique indexes:

  ```sql
  CREATE TABLE system_mail_config (
      id BIGSERIAL PRIMARY KEY,
      platform_id BIGINT NOT NULL REFERENCES auth_platform(id),
      secret_id_ciphertext TEXT NOT NULL,
      secret_key_ciphertext TEXT NOT NULL,
      secret_id_hint VARCHAR(32) NOT NULL DEFAULT '',
      secret_key_hint VARCHAR(32) NOT NULL DEFAULT '',
      region VARCHAR(64) NOT NULL,
      endpoint VARCHAR(255),
      from_email VARCHAR(254) NOT NULL,
      from_name VARCHAR(128) NOT NULL,
      reply_to VARCHAR(254),
      ttl_minutes SMALLINT NOT NULL CHECK (ttl_minutes BETWEEN 1 AND 60),
      is_enabled SMALLINT NOT NULL DEFAULT 0 CHECK (is_enabled IN (0, 1)),
      last_test_at TIMESTAMPTZ,
      last_test_error VARCHAR(512) NOT NULL DEFAULT '',
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      deleted_at TIMESTAMPTZ
  );
  CREATE TABLE system_mail_template (
      id BIGSERIAL PRIMARY KEY,
      platform_id BIGINT NOT NULL REFERENCES auth_platform(id),
      scene VARCHAR(32) NOT NULL CHECK (scene IN ('login', 'forget', 'bind_email', 'change_password')),
      name VARCHAR(128) NOT NULL,
      subject VARCHAR(255) NOT NULL,
      tencent_template_id INTEGER NOT NULL,
      variables JSONB NOT NULL,
      example_variables JSONB NOT NULL,
      is_enabled SMALLINT NOT NULL DEFAULT 1 CHECK (is_enabled IN (0, 1)),
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      deleted_at TIMESTAMPTZ
  );
  CREATE TABLE system_mail_log (
      id BIGSERIAL PRIMARY KEY,
      platform_id BIGINT NOT NULL REFERENCES auth_platform(id),
      challenge_id VARCHAR(128),
      user_id BIGINT,
      scene VARCHAR(32) NOT NULL,
      template_id INTEGER NOT NULL,
      to_email VARCHAR(254) NOT NULL,
      subject VARCHAR(255) NOT NULL,
      status VARCHAR(16) NOT NULL CHECK (status IN ('pending', 'sent', 'failed')),
      request_id VARCHAR(128) NOT NULL DEFAULT '',
      message_id VARCHAR(128) NOT NULL DEFAULT '',
      error_code VARCHAR(128) NOT NULL DEFAULT '',
      error_summary VARCHAR(512) NOT NULL DEFAULT '',
      latency_ms BIGINT NOT NULL DEFAULT 0,
      sent_at TIMESTAMPTZ,
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      deleted_at TIMESTAMPTZ,
      UNIQUE (id, platform_id)
  );
  CREATE TABLE system_mail_log_verification (
      id BIGSERIAL PRIMARY KEY,
      platform_id BIGINT NOT NULL REFERENCES auth_platform(id),
      mail_log_id BIGINT NOT NULL,
      key_version VARCHAR(16) NOT NULL,
      code_ciphertext TEXT NOT NULL,
      expires_at TIMESTAMPTZ NOT NULL,
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      deleted_at TIMESTAMPTZ,
      FOREIGN KEY (mail_log_id, platform_id) REFERENCES system_mail_log(id, platform_id)
  );
  CREATE TABLE system_mail_recipient_rule (
      id BIGSERIAL PRIMARY KEY,
      platform_id BIGINT NOT NULL REFERENCES auth_platform(id),
      scope VARCHAR(16) NOT NULL CHECK (scope IN ('email', 'domain')),
      pattern VARCHAR(254) NOT NULL,
      action VARCHAR(16) NOT NULL CHECK (action IN ('allow', 'deny')),
      name VARCHAR(128) NOT NULL,
      remark VARCHAR(512) NOT NULL DEFAULT '',
      is_enabled SMALLINT NOT NULL DEFAULT 1 CHECK (is_enabled IN (0, 1)),
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      deleted_at TIMESTAMPTZ
  );
  ```

  ```sql
  CREATE UNIQUE INDEX ux_system_mail_config_platform_active
      ON system_mail_config (platform_id) WHERE deleted_at IS NULL;
  CREATE UNIQUE INDEX ux_system_mail_template_platform_scene_active
      ON system_mail_template (platform_id, scene) WHERE deleted_at IS NULL;
  CREATE UNIQUE INDEX ux_system_mail_rule_platform_scope_pattern_action_active
      ON system_mail_recipient_rule (platform_id, scope, pattern, action) WHERE deleted_at IS NULL;
  CREATE UNIQUE INDEX ux_system_mail_log_platform_challenge_active
      ON system_mail_log (platform_id, challenge_id) WHERE deleted_at IS NULL AND challenge_id IS NOT NULL;
  CREATE UNIQUE INDEX ux_system_mail_verification_log_active
      ON system_mail_log_verification (mail_log_id) WHERE deleted_at IS NULL;
  ```

  The seed portion is also manual SQL and must be limited to the Admin platform:

  ```sql
  INSERT INTO system_mail_template
      (platform_id, scene, name, subject, tencent_template_id, variables, example_variables, is_enabled)
  SELECT p.id, v.scene, v.name, v.subject, v.tencent_template_id, v.variables::jsonb, v.example_variables::jsonb, 1
  FROM auth_platform p
  CROSS JOIN (VALUES
      ('login', '邮箱验证码登录', '登录验证码', 47941, '{"code":"123456","ttl_minutes":"10"}', '{"code":"123456","ttl_minutes":"10"}'),
      ('forget', '找回密码', '找回密码验证码', 47942, '{"code":"123456","ttl_minutes":"10"}', '{"code":"123456","ttl_minutes":"10"}'),
      ('bind_email', '绑定/换绑邮箱', '绑定邮箱验证码', 47943, '{"code":"123456","ttl_minutes":"10"}', '{"code":"123456","ttl_minutes":"10"}'),
      ('change_password', '验证码改密', '修改密码验证码', 47944, '{"code":"123456","ttl_minutes":"10"}', '{"code":"123456","ttl_minutes":"10"}')
  ) AS v(scene, name, subject, tencent_template_id, variables, example_variables)
  WHERE p.code = 'admin' AND p.deleted_at IS NULL
    AND NOT EXISTS (
        SELECT 1 FROM system_mail_template t
        WHERE t.platform_id = p.id AND t.scene = v.scene AND t.deleted_at IS NULL
    );
  ```

  Insert exactly four Admin-platform templates by selecting the live row with `code = 'admin'`; do not insert rows for Canvas and do not use an API startup hook. The verification table must use a composite foreign key `(mail_log_id, platform_id)` to a matching unique key on the log table so a snapshot cannot cross platforms.

- [ ] **Step 5: Write and run the migration contract test.**

  In `mail_service_migration_test.go`, execute the SQL file against the existing PostgreSQL test harness, query `to_regclass`, `pg_indexes`, `pg_constraint`, and the Admin template rows, and assert that a second execution is intentionally rejected or is not part of application startup. Also assert that `main.go` contains no call to `AutoMigrate` or mail seed functions.

  ```powershell
  cd D:\admin\server
  go test ./internal/database -run MailServiceMigration -count=1 -timeout 180s
  ```

- [ ] **Step 6: Run all Task 1 tests and commit the schema-only change.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/system/mail ./internal/database -run 'Mail|mail' -count=1 -timeout 180s
  cd D:\admin
  git add server/internal/module/system/mail server/internal/database/mail_service_migration_test.go docs/database/2026-09-01-mail-service.sql
  git commit -m "feat: 建立邮件服务数据模型和人工迁移脚本"
  ```

### Task 2: 为邮件凭据增加独立 KeyRing 用途

**Files:**
- Modify: `server/internal/secretkey/secretkey.go`
- Modify: `server/internal/secretkey/secretkey_test.go`
- Modify: `server/internal/module/system/mail/crypto.go`
- Create: `server/internal/module/system/mail/crypto_test.go`

**Interfaces:**
- Produces `(*secretkey.KeyRing).MailEncryptionKey() []byte` using HKDF purpose `admin:system:mail-encryption:v1`.
- Produces `mail.EncryptSecret(key []byte, plaintext string) (ciphertext string, keyVersion string, err error)` and `mail.DecryptSecret(key []byte, ciphertext string) (string, error)` using AES-256-GCM, random nonce and an explicit key version prefix.
- Consumes the existing `StorageEncryptionKey` implementation only as a pattern; the mail key must be a different derived value.

- [ ] **Step 1: Add failing tests for separated derivation and ciphertext behavior.**

  Extend the KeyRing test to assert stable copied mail keys that differ from JWT, refresh and COS keys. Add crypto tests that decrypt a round trip, reject tampering, and assert two encryptions of the same code differ because the nonce is random.

- [ ] **Step 2: Run the focused tests and verify missing API/crypto symbols fail.**

  ```powershell
  cd D:\admin\server
  go test ./internal/secretkey ./internal/module/system/mail -run 'MailEncryption|EncryptSecret' -count=1
  ```

  Expected result: FAIL because `MailEncryptionKey`, `EncryptSecret`, and `DecryptSecret` are not defined.

- [ ] **Step 3: Implement the new HKDF purpose and AES-256-GCM helpers.**

  Add a private `mailEncryptionPurpose` constant, a `mailEncryptionKey` field initialized in `secretkey.New`, and a copy-returning accessor. Store ciphertext as `mail:v1:<base64url(nonce+ciphertext)>`; reject an unknown version, short nonce, invalid base64 or authentication failure. Never log plaintext or ciphertext.

  The public accessor and encryption contract must have this shape:

  ```go
  func (k *KeyRing) MailEncryptionKey() []byte {
      return append([]byte(nil), k.mailEncryptionKey...)
  }

  func EncryptSecret(key []byte, plaintext string) (string, string, error) {
      block, err := aes.NewCipher(key)
      if err != nil { return "", "", err }
      gcm, err := cipher.NewGCM(block)
      if err != nil { return "", "", err }
      nonce := make([]byte, gcm.NonceSize())
      if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return "", "", err }
      sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
      return "mail:v1:" + base64.RawURLEncoding.EncodeToString(sealed), "v1", nil
  }
  ```

- [ ] **Step 4: Run tests and inspect sensitive-field behavior.**

  ```powershell
  cd D:\admin\server
  go test ./internal/secretkey ./internal/module/system/mail -run 'MailEncryption|EncryptSecret' -count=1
  rg -n "MailEncryptionKey|CodeCiphertext|SecretIDCiphertext|SecretKeyCiphertext" internal/module/system/mail internal/secretkey
  ```

  Expected result: PASS, with no response DTO or log statement referencing the plaintext value.

- [ ] **Step 5: Commit the isolated cryptography change.**

  ```powershell
  cd D:\admin
  git add server/internal/secretkey/secretkey.go server/internal/secretkey/secretkey_test.go server/internal/module/system/mail/crypto.go server/internal/module/system/mail/crypto_test.go
  git commit -m "feat: 增加邮件凭据独立加密密钥"
  ```

### Task 3: 封装腾讯云 SES 客户端和固定模板发送协议

**Files:**
- Create: `server/internal/storage/mail/tencent_ses.go`
- Create: `server/internal/storage/mail/tencent_ses_test.go`
- Create: `server/internal/module/system/mail/provider.go`
- Create: `server/internal/module/system/mail/provider_test.go`
- Modify: `server/go.mod`
- Modify: `server/go.sum`

**Interfaces:**
- Produces `mailprovider.Client` with constructor `NewTencentSESClient(httpClient *http.Client) *Client` and method `Send(context.Context, mailprovider.SendInput) (mailprovider.SendResult, error)`.
- `SendInput` contains `Region`, `Endpoint`, `SecretID`, `SecretKey`, `FromEmail`, `FromName`, `ReplyTo`, `ToEmail`, `TemplateID`, and `TemplateData map[string]string`; it never contains an arbitrary body or caller-provided provider name.
- `SendResult` contains only `RequestID` and `MessageID`; `ProviderError` contains a bounded code and summary for the Service layer.
- Adds `github.com/tencentcloud/tencentcloud-sdk-go` and uses the SES `SendEmail` API, not a generic provider factory.

- [ ] **Step 1: Write failing client contract tests with an HTTP stub.**

  Assert that the client builds a Tencent SES request with the configured region, source, destination, fixed template ID and JSON variables; an 8-second context timeout is honored; Request ID/Message ID are returned; SDK errors are reduced to a code and maximum-length summary without credentials.

- [ ] **Step 2: Run the provider tests and verify the client/package dependency is absent.**

  ```powershell
  cd D:\admin\server
  go test ./internal/storage/mail ./internal/module/system/mail -run 'SES|Provider' -count=1
  ```

  Expected result: FAIL because the client, interface and SES dependency are not present.

- [ ] **Step 3: Add the SES SDK dependency and implement `Client.Send`.**

  Use `github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common` credentials and `tencentcloud/ses/v20201002`. Configure the endpoint only when the stored value is non-empty, set `ClientProfile.HttpProfile.Timeout = 8 * time.Second`, pass template variables as JSON, and map SDK errors to `ProviderError{Code, Summary}` capped at 512 bytes. Do not include SecretId/SecretKey in returned errors.

  ```go
  request := ses.NewSendEmailRequest()
  request.Template = &ses.Template{TemplateID: common.Uint64Ptr(uint64(input.TemplateID)), TemplateData: common.StringPtr(templateJSON)}
  request.Destination = []*string{common.StringPtr(input.ToEmail)}
  request.FromEmailAddress = common.StringPtr(input.FromEmail)
  request.Subject = common.StringPtr(input.Subject)
  if input.ReplyTo != "" { request.ReplyToAddresses = []*string{common.StringPtr(input.ReplyTo)} }

  type Sender interface {
      Send(context.Context, mailprovider.SendInput) (mailprovider.SendResult, error)
  }
  ```

- [ ] **Step 4: Run tests and verify the exact four template IDs are the only accepted IDs at the Service boundary.**

  ```powershell
  cd D:\admin\server
  go test ./internal/storage/mail ./internal/module/system/mail -run 'SES|Provider|FixedTemplate' -count=1
  ```

- [ ] **Step 5: Commit the provider-only change.**

  ```powershell
  cd D:\admin
  git add server/internal/storage/mail server/internal/module/system/mail/provider.go server/internal/module/system/mail/provider_test.go server/go.mod server/go.sum
  git commit -m "feat: 接入腾讯云 SES 固定模板发送"
  ```

### Task 4: 实现邮件配置、模板、日志和同步发送 Service

**Files:**
- Create: `server/internal/module/system/mail/protocol.go`
- Create: `server/internal/module/system/mail/errors.go`
- Create: `server/internal/module/system/mail/repository.go`
- Create: `server/internal/module/system/mail/request.go`
- Create: `server/internal/module/system/mail/response.go`
- Create: `server/internal/module/system/mail/service.go`
- Create: `server/internal/module/system/mail/repository_test.go`
- Create: `server/internal/module/system/mail/service_test.go`

**Interfaces:**
- Produces `NewRepository(db *gorm.DB) *Repository` with platform-scoped methods for config, template, log and verification rows; every query signature includes `platformID int64`. The log methods are `CreatePendingLog`, `FindSentChallenge`, `MarkSent`, `MarkFailed`, `GetLogDetail`, `ListLogs`, `DeleteLog` and `DeleteLogs`.
- Produces `NewService(repository *Repository, keys *secretkey.KeyRing, sender Sender, rules RuleEvaluator, limiter Limiter) *Service`.
- Produces public business method:

  ```go
  type BusinessSendInput struct {
      PlatformID int64
      UserID     *int64
      ClientIP   string
      ChallengeID string
      Scene      string
      ToEmail    string
      Variables  map[string]string
  }
  func (s *Service) Send(ctx context.Context, input BusinessSendInput) (SendResult, error)
  ```

- Produces Admin method `Test(ctx context.Context, input AdminTestInput) (AdminTestResult, error)` where `AdminTestInput` has `AdminUserID`, `ClientIP`, `ToEmail`, `Scene`, and validated variables; it has no `PlatformID` field.
- Produces config methods `GetConfig`, `SaveConfig`, `DeleteConfig`, `SetConfigStatus`, `TestConnection`, template methods `ListTemplates`, `UpdateTemplate`, `SetTemplateStatus`, and log methods `ListLogs`, `GetLogDetail`, `DeleteLog`, `DeleteLogs`. `MarkFailed` accepts a bounded `ProviderError` and records only its code/summary.

- [ ] **Step 1: Write failing Service tests for the complete send state machine.**

  Cover configuration missing/disabled, template disabled, rule denial, limiter denial, `pending -> sent`, `pending -> failed` on SES error/timeout, challenge-id idempotency, and platform scoping. Assert `Sender.Send` receives the stored Tencent template ID and variables, while the returned public `SendResult` contains no code, secret, provider error message or rule ID.

  ```go
  func TestSendCreatesPendingThenSentWithoutSensitiveResponse(t *testing.T) {
      result, err := service.Send(ctx, BusinessSendInput{PlatformID: 1, ChallengeID: "challenge-1", Scene: SceneLogin, ToEmail: "user@example.com", Variables: map[string]string{"code": "123456", "ttl_minutes": "10"}})
      if err != nil { t.Fatal(err) }
      if result.Status != StatusSent || result.RequestID == "" { t.Fatalf("unexpected result: %+v", result) }
      if strings.Contains(string(mustJSON(result)), "123456") { t.Fatal("public result leaked verification code") }
  }
  ```

- [ ] **Step 2: Run the Service tests and verify missing methods fail.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/system/mail -run 'TestSend|TestConfig|TestTemplate|TestLog' -count=1
  ```

  Expected result: FAIL because repository, Service and protocol methods do not exist.

- [ ] **Step 3: Implement repository methods with mandatory platform predicates and soft-delete handling.**

  Use `WHERE platform_id = ? AND deleted_at IS NULL` for active reads and writes. `CreatePendingLog` inserts before SES; `MarkSent` and `MarkFailed` update only the matching platform and pending row; `FindSentChallenge` enforces `(platform_id, challenge_id)` in a unique active index or equivalent transaction lock. Log deletion is soft deletion and never mutates verification or template rows.

  Every repository method must make the platform argument visible in its signature and query:

  ```go
  func (r *Repository) FindConfig(ctx context.Context, platformID int64) (Config, error) {
      var row Config
      err := r.db.WithContext(ctx).
          Where("platform_id = ? AND deleted_at IS NULL", platformID).
          Take(&row).Error
      return row, err
  }

  func (r *Repository) MarkSent(ctx context.Context, platformID, logID int64, result mailprovider.SendResult) error {
      updates := map[string]any{"status": StatusSent, "request_id": result.RequestID, "message_id": result.MessageID, "updated_at": time.Now().UTC()}
      tx := r.db.WithContext(ctx).Model(&Log{}).Where("id = ? AND platform_id = ? AND status = ? AND deleted_at IS NULL", logID, platformID, StatusPending).Updates(updates)
      if tx.Error != nil { return tx.Error }
      if tx.RowsAffected != 1 { return gorm.ErrRecordNotFound }
      return nil
  }
  ```

- [ ] **Step 4: Implement config and fixed-template validation.**

  Require TTL 1-60 minutes, valid sender/reply-to emails, encrypted credentials on write, empty secret input on update means preserve old ciphertext, and no client platform ID. `UpdateTemplate` may change only name, subject, Tencent template ID, variables, example variables and metadata; reject scene changes, unknown scenes, arbitrary provider IDs, and delete requests.

  Keep the allowed template set explicit at the Service boundary:

  ```go
  func fixedTemplate(scene string) (FixedTemplate, bool) {
      for _, item := range FixedTemplates() {
          if item.Scene == scene { return item, true }
      }
      return FixedTemplate{}, false
  }
  ```

- [ ] **Step 5: Implement synchronous `Service.Send` and `Service.Test`.**

  Normalize the recipient, validate exactly the scene variables, read the platform config/template, call the injected rule evaluator and limiter, insert a `pending` log, encrypt and store a verification snapshot, call `Sender.Send` with an 8-second context, then atomically complete the row as `sent` or `failed`. If the same `ChallengeID` already has a sent row, return the existing safe result without a second SES request. Admin tests use Admin platform internally and never accept a request platform ID.

  Keep the orchestration order observable in a unit test and in code:

  ```go
  decision, err := s.rules.Evaluate(ctx, input.PlatformID, normalizedEmail, SendModeBusiness)
  if err != nil || !decision.Allowed { return SendResult{}, ErrRecipientDenied }
  if allowed, err := s.allowBusinessLimits(ctx, input); err != nil || !allowed { return SendResult{}, ErrRateLimited }
  existing, err := s.repository.FindSentChallenge(ctx, input.PlatformID, input.ChallengeID)
  if err == nil { return safeSendResult(existing), nil }
  logRow, err := s.repository.CreatePendingLog(ctx, pendingLog(input, template))
  if err != nil { return SendResult{}, mapDependency(err) }
  sendCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
  defer cancel()
  providerResult, sendErr := s.sender.Send(sendCtx, providerInput(config, template, normalizedEmail, input.Variables))
  if sendErr != nil {
      _ = s.repository.MarkFailed(ctx, input.PlatformID, logRow.ID, boundedProviderError(sendErr))
      return SendResult{LogID: logRow.ID, Status: StatusFailed}, mapProviderError(sendErr)
  }
  _ = s.repository.MarkSent(ctx, input.PlatformID, logRow.ID, providerResult)
  return SendResult{LogID: logRow.ID, Status: StatusSent, RequestID: providerResult.RequestID, MessageID: providerResult.MessageID}, nil
  ```

- [ ] **Step 6: Add Admin-only diagnostic projection and sensitive-field assertions.**

  `GetLogDetail` may decrypt the verification snapshot only after the caller has `system:mail:detail`; `ListLogs` returns bounded failure summaries and no code. Keep the complete code, email and Tencent error in the protected detail DTO only. Add tests that marshal public results, ordinary `slog` attributes and operation-log payloads and assert they contain none of `secretId`, `secretKey`, `ciphertext`, `code`, `templateVariables` values or Authorization headers.

- [ ] **Step 7: Run all Service tests and commit the core module.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/system/mail -count=1 -timeout 180s
  cd D:\admin
  git add server/internal/module/system/mail
  git commit -m "feat: 实现邮件配置模板和同步发送服务"
  ```

### Task 5: 实现收件人黑白名单规则服务

**Files:**
- Create: `server/internal/module/system/mail/rules.go`
- Create: `server/internal/module/system/mail/rules_test.go`
- Modify: `server/internal/module/system/mail/protocol.go`
- Modify: `server/internal/module/system/mail/repository.go`

**Interfaces:**
- Produces `RuleEvaluator`:

  ```go
  type RuleEvaluator interface {
      Evaluate(ctx context.Context, platformID int64, email string, mode SendMode) (RuleDecision, error)
  }
  type RuleDecision struct { Allowed bool; RuleID int64; Reason string }
  ```

- Produces `NormalizeRecipient(value string) (string, error)` and `NormalizeRule(scope, pattern string) (string, error)`.
- Produces rule CRUD/status methods with `platformID` as a server-supplied argument; `mode` distinguishes `business` and `admin_test` but neither mode bypasses a deny rule or a limiter.

- [ ] **Step 1: Write failing matching tests.**

  Include exact email matching, case/space normalization, `example.com` matching `user@example.com` and `a.b@example.com`, rejection of `badexample.com`, `example.com.evil`, malformed DNS labels and invalid email syntax. Include exact-email-over-domain precedence and deny-over-allow at the same level.

- [ ] **Step 2: Run the rules tests and verify matching helpers are absent.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/system/mail -run 'Rule|Normalize|Domain' -count=1
  ```

  Expected result: FAIL because `NormalizeRecipient`, `RuleEvaluator` and rule repository methods are not implemented.

- [ ] **Step 3: Implement normalization and DNS-label boundary matching.**

  Lowercase and trim email/domain patterns, use `net/mail` for complete email validation, and compare domain candidates by labels (`host == pattern || strings.HasSuffix(host, "."+pattern)`) rather than raw suffixes. Store only normalized values.

  The matcher must preserve the DNS label boundary explicitly:

  ```go
  func matchesDomain(email, pattern string) bool {
      host := strings.ToLower(strings.TrimSpace(emailDomain(email)))
      rule := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(pattern)), ".")
      return host == rule || strings.HasSuffix(host, "."+rule)
  }
  ```

- [ ] **Step 4: Implement deterministic decision order and CRUD conflicts.**

  Query active rules for the platform, select exact email rules before domain rules, choose deny before allow within a level, and default to allow. Map the partial unique index `(platform_id, scope, pattern, action)` to a conflict error. Return a generic denied error to business callers; expose rule ID/reason only to Admin diagnostics.

  Encode the order as a stable sort key instead of relying on database row order:

  ```go
  func chooseRule(exact, domain []RecipientRule) RuleDecision {
      for _, group := range [][]RecipientRule{exact, domain} {
          for _, action := range []string{RuleActionDeny, RuleActionAllow} {
              for _, rule := range group {
                  if rule.Action == action && rule.IsEnabled == yesno.Yes {
                      return RuleDecision{Allowed: action == RuleActionAllow, RuleID: rule.ID, Reason: action}
                  }
              }
          }
      }
      return RuleDecision{Allowed: true, Reason: "default_allow"}
  }
  ```

- [ ] **Step 5: Run focused and core Service tests, then commit.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/system/mail -run 'Rule|Normalize|Domain|TestSend' -count=1
  cd D:\admin
  git add server/internal/module/system/mail/protocol.go server/internal/module/system/mail/repository.go server/internal/module/system/mail/rules.go server/internal/module/system/mail/rules_test.go
  git commit -m "feat: 增加邮件收件人黑白名单规则"
  ```

### Task 6: 增加 Redis 双层限流和 HTTP 快速拒绝

**Files:**
- Create: `server/internal/module/system/mail/limiter.go`
- Create: `server/internal/module/system/mail/limiter_test.go`
- Create: `server/internal/middleware/ratelimit.go`
- Create: `server/internal/middleware/ratelimit_test.go`
- Modify: `server/internal/shared/apperror/error.go`
- Modify: `server/internal/shared/apperror/error_test.go`
- Modify: `server/internal/shared/i18n/catalog.go`
- Modify: `server/internal/shared/i18n/catalog_test.go`
- Modify: `server/go.mod`
- Modify: `server/go.sum`

**Interfaces:**
- Produces `Limiter`:

  ```go
  type LimitRequest struct { Key string; Limit int; Window time.Duration }
  type Limiter interface { Allow(context.Context, LimitRequest) (bool, error) }
  ```

- Produces `NewRedisLimiter(client redis.UniversalClient) *RedisLimiter`, backed by `github.com/redis/go-redis_rate/v10`; Redis errors are returned and never interpreted as allowed.
- Produces `RateLimit(config RateLimitConfig) gin.HandlerFunc` for route-level IP limits. The middleware emits HTTP 429 with the existing response envelope and does not inspect or log request bodies.

- [ ] **Step 1: Write failing tests for all limiter dimensions and fail-closed behavior.**

  Assert public sends use keys `mail:send:email:<platform>:<scene>:<email>`, `mail:send:ip:<platform>:<ip>`, and `mail:send:scene:<platform>:<scene>` with windows 60 seconds/10 minutes/one minute as specified. Assert Admin tests use user/IP/email keys, white-list matches do not skip a check, and a Redis command error returns `allowed=false`.

- [ ] **Step 2: Run limiter tests and verify dependency/API are absent.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/system/mail ./internal/middleware -run 'Limit|Rate' -count=1
  ```

  Expected result: FAIL because `Limiter`, `RedisLimiter`, and `RateLimit` do not exist.

- [ ] **Step 3: Add Redis rate dependency and implement atomic windows.**

  Use `redis_rate.Limiter.Allow(ctx, key, redis_rate.Limit{Rate, Burst, Period})`; map every non-nil Redis error to fail-closed and return the retry duration for a 429 response. Keep limits in package constants so no request field can alter them.

  Add the shared envelope error once, rather than returning an ad-hoc JSON shape from middleware:

  ```go
  const CodeRateLimited = 10007

  func RateLimited(cause error) *Error {
      return newError(http.StatusTooManyRequests, CodeRateLimited, i18n.KeyRateLimited, nil, cause)
  }
  ```

  Add `KeyRateLimited MessageKey = "error.rateLimited"` with `请求过于频繁` and `Too many requests` catalog entries, plus the matching `apperror` and catalog tests.

  The adapter must make an error distinguishable from a normal rejection:

  ```go
  func (l *RedisLimiter) Allow(ctx context.Context, request LimitRequest) (bool, error) {
      result, err := l.limiter.Allow(ctx, request.Key, redis_rate.Limit{Rate: request.Limit, Burst: request.Limit, Period: request.Window})
      if err != nil { return false, fmt.Errorf("redis mail rate limit: %w", err) }
      return result.Allowed > 0, nil
  }
  ```

- [ ] **Step 4: Implement the Gin middleware and inject it only on protected send routes.**

  Read the client IP from Gin, use a route-specific key, call `Allow`, and return a generic rate-limit error on false/error. Do not use an in-memory fallback. Business email/service calls still invoke the Service limiter for recipient, scene and Admin-user dimensions.

  The middleware must reject both Redis errors and exhausted windows with the existing response envelope:

  ```go
  func RateLimit(config RateLimitConfig) gin.HandlerFunc {
      return func(c *gin.Context) {
          allowed, err := config.Limiter.Allow(c.Request.Context(), LimitRequest{Key: config.Key(c), Limit: config.Limit, Window: config.Window})
          if err != nil || !allowed {
              response.Fail(c, apperror.RateLimited(fmt.Errorf("mail rate limit unavailable or exceeded")))
              c.Abort()
              return
          }
          c.Next()
      }
  }
  ```

- [ ] **Step 5: Run tests and commit the rate-limit change.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/system/mail ./internal/middleware -count=1
  cd D:\admin
  git add server/internal/module/system/mail/limiter.go server/internal/module/system/mail/limiter_test.go server/internal/middleware/ratelimit.go server/internal/middleware/ratelimit_test.go server/go.mod server/go.sum
  git commit -m "feat: 增加邮件发送双层限流"
  ```

### Task 7: 接入 Admin Handler、严格 DTO、路由和操作日志

**Files:**
- Create: `server/internal/module/system/mail/handler.go`
- Create: `server/internal/module/system/mail/route.go`
- Modify: `server/internal/module/system/mail/request.go`
- Modify: `server/internal/module/system/mail/response.go`
- Create: `server/internal/module/system/mail/handler_test.go`
- Create: `server/internal/module/system/mail/route_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`
- Modify: `server/internal/module/system/operationlog/rules.go`
- Modify: `server/internal/module/system/operationlog/rules_test.go`
- Modify: `server/internal/module/system/operationlog/middleware.go`
- Modify: `server/internal/module/system/operationlog/middleware_test.go`

**Interfaces:**
- Produces permission constants:

  ```go
  const (
      PermissionView = "system:mail:view"
      PermissionList = "system:mail:list"
      PermissionDetail = "system:mail:detail"
      PermissionConfigUpdate = "system:mail:config:update"
      PermissionConfigDelete = "system:mail:config:delete"
      PermissionTest = "system:mail:test"
      PermissionTemplateUpdate = "system:mail:template:update"
      PermissionTemplateStatus = "system:mail:template:status"
      PermissionLogDelete = "system:mail:log:delete"
      PermissionRuleCreate = "system:mail:rule:create"
      PermissionRuleUpdate = "system:mail:rule:update"
      PermissionRuleStatus = "system:mail:rule:status"
      PermissionRuleDelete = "system:mail:rule:delete"
  )
  ```

- Produces `RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc)` under `/mail`.
- Produces exact endpoints from the spec: `GET /page-init`, `GET/PUT/DELETE /config`, `POST /test`, template list/update/status, log list/detail/delete-one/delete-many, and recipient-rule list/create/update/status/delete.
- Admin platform ID is derived from authenticated identity/platform middleware; request structs reject unknown fields and have no `platformId` field.

- [ ] **Step 1: Write failing route and DTO contract tests.**

  Register routes with permission spies and assert every method/path/permission pair. Send JSON containing `platformId`, unknown fields, template `scene` changes, arbitrary `templateId`, and request bodies to delete endpoints; assert invalid request responses. Assert `/mail/test` accepts only the four scene values and variables required by that scene.

  ```go
  want := []struct { method, path, permission string }{
      {http.MethodGet, "/mail/page-init", PermissionList},
      {http.MethodPut, "/mail/config", PermissionConfigUpdate},
      {http.MethodPost, "/mail/test", PermissionTest},
      {http.MethodGet, "/mail/logs/:id", PermissionDetail},
      {http.MethodPost, "/mail/recipient-rules", PermissionRuleCreate},
  }
  ```

- [ ] **Step 2: Run the API tests and verify the route/handler contract fails.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/system/mail ./cmd/api -run 'Mail|mail' -count=1
  ```

  Expected result: FAIL because the mail Handler, route registration and dependency fields are absent.

- [ ] **Step 3: Implement strict request parsing and response projections.**

  Reuse `shared/validate.BindJSON`, `validate.RequireEmptyBody`, pagination parsing and the existing response envelope. Config responses expose `configured`, metadata and hints only; config GET never returns secret fields. Template responses expose fixed metadata. Log list returns safe fields; log detail returns complete Admin diagnostics only after `PermissionDetail`. Rule DTOs return normalized pattern and action.

  The request structs must use pointers for required fields and have no platform field, for example:

  ```go
  type configUpdateRequest struct {
      Region *string `json:"region"`
      FromEmail *string `json:"fromEmail"`
      SecretID json.RawMessage `json:"secretId"`
      SecretKey json.RawMessage `json:"secretKey"`
      TTLMinutes *int `json:"ttlMinutes"`
  }
  ```

- [ ] **Step 4: Register routes and inject dependencies in `server/cmd/api/main.go`.**

  Construct one mail repository, `RedisLimiter`, SES client and mail Service after the existing COS services. Add `Mail *mail.Handler` to `routerDependencies`, pass it from `run`, and register it inside the existing `adminRoutes` group, which already applies `authclient.Require()` and `authclient.RequireAdminPlatform()`.

  The registration must remain inside the shared Admin group:

  ```go
  mailService := systemmail.NewService(mail.NewRepository(postgres.GORM), keys, sesClient, ruleEvaluator, redisLimiter)
  Mail: systemmail.NewHandler(mailService),
  // buildRouter
  systemmail.RegisterRoutes(adminRoutes, dependencies.Mail, dependencies.Authenticate, dependencies.RequirePermission)
  ```

- [ ] **Step 5: Add operation-log rules with request/response capture disabled for sensitive mail endpoints.**

  Add mutation rules for config save/delete, test send, template update/status, log delete, and rule create/update/status/delete. Set `CaptureRequest=false` and `CaptureResponse=false` for `/mail/test` and log detail/delete routes; for other mutations rely on sanitizer but explicitly add `code`, `templateVariables`, `toEmail`, `secretId`, `secretKey`, `verificationCode` and `ciphertext` to the sensitive-field deny list. Add tests proving login/refresh/logout remain unmatched and no mail payload leaks.

  Extend the existing sanitizer predicate, preserving its normalized comparison:

  ```go
  for _, sensitive := range []string{
      "password", "confirmpassword", "accesstoken", "refreshtoken", "authorization", "cookie",
      "secret", "key", "code", "templatevariables", "toemail", "verificationcode", "ciphertext",
  } {
      if normalized == sensitive || strings.Contains(normalized, sensitive) { return true }
  }
  ```

- [ ] **Step 6: Run focused API, route, startup and operation-log tests.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/system/mail ./internal/module/system/operationlog ./cmd/api -run 'Mail|mail|Operation|Route' -count=1 -timeout 180s
  ```

  Expected result: PASS; `main` still has no migration, default-template seed or menu initialization call.

- [ ] **Step 7: Commit the Admin API boundary.**

  ```powershell
  cd D:\admin
  git add server/internal/module/system/mail server/cmd/api/main.go server/cmd/api/main_test.go server/internal/module/system/operationlog/rules.go server/internal/module/system/operationlog/rules_test.go
  git commit -m "feat: 增加邮件管理 Admin API 和审计规则"
  ```

### Task 8: 实现前端 API、动态路由和四 Tab 页面

**Files:**
- Create: `web/src/api/system/mail.ts`
- Create: `web/src/views/message/mail/index.vue`
- Create: `web/tests/api/system/mail.test.ts`
- Create: `web/tests/views/message/mail/index.test.ts`
- Modify: `web/src/router/permission-routes.ts`
- Modify: `web/src/locales/zh-CN.ts`
- Modify: `web/src/locales/en-US.ts`

**Interfaces:**
- Produces strict TypeScript DTO parsers for every Admin mail endpoint; parsers reject missing or unknown fields and never model secret ciphertext.
- Adds `componentPathMap['message/mail'] = 'message/mail'` and resolves the page as a dynamic route named from `system:mail:view`.
- Produces one `el-tabs` page with lazy tabs `config`, `templates`, `logs`, `rules`; each tab fetches only after activation and gates its requests/buttons with the exact permission codes from Task 7.

- [ ] **Step 1: Write failing API parser and permission-visibility tests.**

  Assert `parseConfig` rejects `secretId`, `secretKey`, unknown keys and missing `configured`; assert the page renders four tabs only when `system:mail:view` exists, hides logs when `system:mail:list`/`system:mail:detail` is absent, and hides each mutation button without its action permission. Assert the edit form initializes secret inputs as empty strings.

- [ ] **Step 2: Run focused Vitest tests and verify missing API/page modules fail.**

  ```powershell
  cd D:\admin\web
  pnpm vitest run tests/api/system/mail.test.ts tests/views/message/mail/index.test.ts --pool=threads --maxWorkers=1
  ```

  Expected result: FAIL because the API module, component path binding and page do not exist.

- [ ] **Step 3: Implement strict API functions matching the backend paths.**

  Add `getMailPageInit`, `getMailConfig`, `saveMailConfig`, `deleteMailConfig`, `sendMailTest`, `listMailTemplates`, `updateMailTemplate`, `updateMailTemplateStatus`, `listMailLogs`, `getMailLogDetail`, `deleteMailLog`, `deleteMailLogs`, and recipient-rule CRUD functions. Use `request<T>` and exact-key validation; do not pass platform IDs from the browser.

  Keep the parser contract explicit and safe:

  ```ts
  const configKeys = ['configured', 'region', 'endpoint', 'fromEmail', 'fromName', 'replyTo', 'ttlMinutes', 'isEnabled', 'lastTestAt', 'lastTestError'] as const
  export function parseMailConfig(value: unknown): MailConfig {
    const record = asRecord(value)
    exactKeys(record, configKeys)
    if (typeof record.configured !== 'boolean' || typeof record.region !== 'string' || !isYesNo(record.isEnabled)) {
      throw new ProtocolError('mail config response is invalid')
    }
    return record as MailConfig
  }
  ```

- [ ] **Step 4: Implement the four tabs and interaction states.**

  Configuration tab supports save/test/delete, masks editable secrets by leaving them blank, and displays safe failure summaries. Template tab displays exactly four fixed scenes and IDs and supports metadata/status only. Logs tab supports pagination/filter/detail/batch soft-delete and a diagnostic failure empty state. Rules tab supports email/domain, allow/deny, normalized pattern, enable/disable and delete. Use existing AppTable/AppDialog/AppSearch patterns and ensure no tab nests a page-level card inside another card.

  The page must use one tab state and lazy loaders, with permission checks at both tab and action level:

  ```ts
  const activeTab = ref<'config' | 'templates' | 'logs' | 'rules'>('config')
  const visibleTabs = computed(() => [
    { name: 'config', visible: access.hasPermission('system:mail:list') },
    { name: 'templates', visible: access.hasPermission('system:mail:list') },
    { name: 'logs', visible: access.hasPermission('system:mail:list') && access.hasPermission('system:mail:detail') },
    { name: 'rules', visible: access.hasPermission('system:mail:list') },
  ].filter(tab => tab.visible))
  watch(activeTab, tab => { if (tab === 'logs') void loadLogs(); if (tab === 'rules') void loadRules() }, { immediate: true })
  ```

- [ ] **Step 5: Register dynamic component mapping and i18n labels.**

  Add the `message/mail` mapping, Chinese/English labels for the directory, page, four tabs, statuses and errors, and tests that a menu node with `componentPath: 'message/mail'` resolves to `web/src/views/message/mail/index.vue` without a static route.

- [ ] **Step 6: Run page/API tests, type-check and commit the frontend.**

  ```powershell
  cd D:\admin\web
  pnpm vitest run tests/api/system/mail.test.ts tests/views/message/mail/index.test.ts tests/router/permission-routes.test.ts --pool=threads --maxWorkers=1
  pnpm build
  cd D:\admin
  git add web/src/api/system/mail.ts web/src/views/message/mail/index.vue web/tests/api/system/mail.test.ts web/tests/views/message/mail/index.test.ts web/src/router/permission-routes.ts web/src/locales/zh-CN.ts web/src/locales/en-US.ts
  git commit -m "feat: 增加邮件服务四 Tab 管理页面"
  ```

### Task 9: 通过人工 SQL 写入消息服务菜单和隐藏 action 权限

**Files:**
- Create: `docs/database/2026-09-01-mail-menu.sql`
- Create: `server/internal/database/mail_menu_migration_test.go`

**Interfaces:**
- Produces a one-time SQL script that inserts the Admin-platform `消息服务` directory and `邮件服务` page with `component_path='message/mail'`, page code `system:mail:view`, and hidden action rows for every `list/detail/action` code in the spec.
- Does not modify `menu.remark`, does not insert Canvas rows, and does not add any runtime migration or default-data function.

- [ ] **Step 1: Write the failing menu contract test.**

  Assert the SQL contains parent directory code `message`, page code `system:mail:view`, component path `message/mail`, four Tab page semantics represented only in frontend, and all twelve action permissions with `is_hidden=1`. Assert no template create/delete permission and no `platformId` request field.

- [ ] **Step 2: Run the migration test and verify the script is absent.**

  ```powershell
  cd D:\admin\server
  go test ./internal/database -run MailMenuMigration -count=1
  ```

  Expected result: FAIL because the one-time menu SQL does not exist.

- [ ] **Step 3: Add transactional Admin-only menu SQL.**

  Select the live Admin platform by `auth_platform.code = 'admin'`, lock/find an existing `message` directory if present, insert the page beneath it, and insert hidden actions with unique `(platform_id, code)` guards. Page rows must have `menu_type='page'`, `code='system:mail:view'`, `path='/message/mail'`, `component_path='message/mail'`; action rows must have no path/component path and must not end in `:view`.

  The one-time script must use the same platform-scoped conflict pattern as existing menu SQL:

  ```sql
  BEGIN;
  WITH admin_platform AS (
      SELECT id FROM auth_platform WHERE code = 'admin' AND deleted_at IS NULL FOR UPDATE
  )
  INSERT INTO permission_menu
      (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden)
  SELECT id, NULL, 'directory', '消息服务', 'message', 'navigation.message', NULL, NULL, 'MessageSquare', 80, 1, 0
  FROM admin_platform
  WHERE NOT EXISTS (
      SELECT 1 FROM permission_menu m WHERE m.platform_id = admin_platform.id AND m.code = 'message' AND m.deleted_at IS NULL
  );
  INSERT INTO permission_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, sort_order, is_enabled, is_hidden)
  SELECT p.id, d.id, 'page', '邮件服务', 'system:mail:view', 'navigation.mail', '/message/mail', 'message/mail', 10, 1, 0
  FROM auth_platform p JOIN permission_menu d ON d.platform_id = p.id AND d.code = 'message' AND d.deleted_at IS NULL
  WHERE p.code = 'admin' AND p.deleted_at IS NULL
    AND NOT EXISTS (SELECT 1 FROM permission_menu m WHERE m.platform_id = p.id AND m.code = 'system:mail:view' AND m.deleted_at IS NULL);
  WITH actions(code, name, sort_order) AS (VALUES
      ('system:mail:list', '读取邮件服务', 5), ('system:mail:detail', '读取邮件详情', 10),
      ('system:mail:config:update', '修改邮件配置', 20), ('system:mail:config:delete', '删除邮件配置', 30),
      ('system:mail:test', '发送测试邮件', 40), ('system:mail:template:update', '修改邮件模板', 50),
      ('system:mail:template:status', '启停邮件模板', 60), ('system:mail:log:delete', '删除邮件日志', 70),
      ('system:mail:rule:create', '新增收件人规则', 80), ('system:mail:rule:update', '修改收件人规则', 90),
      ('system:mail:rule:status', '启停收件人规则', 100), ('system:mail:rule:delete', '删除收件人规则', 110)
  ), page AS (
      SELECT p.id AS platform_id, m.id AS parent_id
      FROM auth_platform p JOIN permission_menu m ON m.platform_id = p.id AND m.code = 'system:mail:view' AND m.deleted_at IS NULL
      WHERE p.code = 'admin' AND p.deleted_at IS NULL
  )
  INSERT INTO permission_menu (platform_id, parent_id, menu_type, name, code, sort_order, is_enabled, is_hidden)
  SELECT page.platform_id, page.parent_id, 'action', action.name, action.code, action.sort_order, 1, 1
  FROM page CROSS JOIN actions action
  WHERE NOT EXISTS (
      SELECT 1 FROM permission_menu m WHERE m.platform_id = page.platform_id AND m.code = action.code AND m.deleted_at IS NULL
  );
  COMMIT;
  ```

  Preserve `parent_id` as the page ID and use the repository's active `(platform_id, code)` uniqueness convention. The script must fail if the Admin platform or parent directory is missing rather than creating either at API startup.

- [ ] **Step 4: Execute the migration contract test and verify route resolution.**

  ```powershell
  cd D:\admin\server
  go test ./internal/database -run 'MailMenuMigration|MailMenuPermission' -count=1 -timeout 180s
  cd D:\admin\web
  pnpm vitest run tests/router/permission-routes.test.ts tests/views/message/mail/index.test.ts --pool=threads --maxWorkers=1
  ```

- [ ] **Step 5: Commit menu data separately from code.**

  ```powershell
  cd D:\admin
  git add docs/database/2026-09-01-mail-menu.sql server/internal/database/mail_menu_migration_test.go
  git commit -m "feat: 增加邮件服务菜单和权限数据脚本"
  ```

### Task 10: 全量验证、敏感信息审计和真实人工验收

**Files:**
- Create: `docs/superpowers/verification/2026-09-01-mail-service-acceptance.md`
- Create: `server/internal/module/system/mail/integration_test.go` for the cross-layer send/diagnostic contract.
- Create: `web/tests/views/message/mail/acceptance.test.ts` for deterministic permission and tab regression coverage.

**Interfaces:**
- Consumes all interfaces from Tasks 1-9 and the manually executed database/menu SQL.
- Produces test evidence and an acceptance checklist; it does not add fallback behavior, startup migration, or unrequested features.

- [ ] **Step 1: Run the complete backend suite and inspect the changed-file set.**

  ```powershell
  cd D:\admin\server
  go test ./... -count=1 -timeout 20m
  cd D:\admin
  git status --short
  git diff --check
  ```

  Expected result: all backend tests pass, the worktree contains only intended mail commits/changes, and `git diff --check` is clean.

- [ ] **Step 2: Run the complete frontend suite and production build.**

  ```powershell
  cd D:\admin\web
  pnpm test -- --pool=threads --maxWorkers=1
  pnpm build
  ```

  Expected result: all Vitest files pass and `vue-tsc` plus Vite build complete successfully.

- [ ] **Step 3: Perform a static sensitive-data audit.**

  ```powershell
  cd D:\admin
  rg -n "SecretID|SecretKey|CodeCiphertext|verificationCode|templateVariables|Authorization|ciphertext" server/internal/module/system/mail server/internal/storage/mail server/internal/module/system/operationlog
  rg -n "AutoMigrate|EnsureSchema|Seed|初始化|INSERT INTO.*system_mail|INSERT INTO.*permission_menu" server/cmd/api server/cmd/worker
  ```

  Confirm the first search only finds encryption/storage/test assertions and the second finds no startup migration, seed or menu write path.

- [ ] **Step 4: Execute real manual acceptance against Tencent SES and both platforms.**

  Verify, in order: save/disable/re-enable Admin SES config; connection test; all four template IDs against Tencent console; successful and failed Admin test sends with complete protected detail; blacklist rejection of a virtual mailbox; allow rule that still hits limits; public email/IP/scene limits; Admin user/IP/email limits; API/Worker restart with no migration/default writes; Admin/Canvas isolation; and absence of secrets, ciphertext, verification code or full provider payload in API responses, ordinary logs and operation logs.

- [ ] **Step 5: Record final evidence and commit only documentation if needed.**

  ```powershell
  cd D:\admin
  git status --short
  git add docs/superpowers/verification/2026-09-01-mail-service-acceptance.md server/internal/module/system/mail/integration_test.go web/tests/views/message/mail/acceptance.test.ts
  git diff --cached
  git commit -m "test: 完成邮件服务全量验证和验收记录"
  ```

  The acceptance file must contain command output, SES test timestamps, platform IDs used, and a pass/fail result for each item in Step 4; do not create an empty commit.

## Self-Review Checklist

- [ ] Fixed four-template contract is covered by model, SQL, Service and frontend tests; no create/delete template endpoint appears anywhere in the plan.
- [ ] Every table and repository operation is platform-scoped, Admin routes derive the Admin platform server-side, and Canvas cannot read or mutate Admin facts.
- [ ] Secret encryption, verification ciphertext, protected diagnostics and operation-log capture rules are covered by implementation and static tests.
- [ ] Public/Admin rate limits use separate dimensions, share Redis state, fail closed, and cannot be bypassed by allow rules.
- [ ] `:view`/`:list`/`:detail` semantics, hidden action menus, dynamic component path and lazy four-tab permission behavior are all covered.
- [ ] Startup persistence boundaries and one-time manual SQL are explicitly tested; no migration or default-data call is planned for API/Worker startup.
- [ ] Every task has exact files, interfaces, a failing test, a failure command, a minimal implementation step, a passing command and a compliant commit command.
