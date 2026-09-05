# 邮件限流策略管理实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在邮件管理页增加固定限流策略的可审计配置能力，并让百万级用户、多实例发送热路径只从 Redis 读取当前策略，避免把 PostgreSQL 放进每次发信的配置查询路径。

**Architecture:** PostgreSQL 是限流策略的唯一持久化事实来源，保存固定策略参数和单调递增版本；Redis 保存带版本的完整策略快照，发送和管理员测试读取限流策略时只读取快照。管理端更新先在 PostgreSQL 事务中提交，再以版本比较的 Redis 脚本发布快照；Redis 缺失时由单实例 singleflight 加有界 Redis 锁回源一次 PostgreSQL，Redis 故障、快照版本无法确认或发布处于 invalidating 状态时闭合失败为 `503/10006`，不使用旧进程缓存、默认值或假成功。

**Tech Stack:** Go、Gin、GORM、PostgreSQL、Redis、`redis_rate/v10`、`golang.org/x/sync/singleflight`、Vue 3、TypeScript、Pinia、Element Plus、Vue I18n、Vites
**Spec:** `docs/agent/design.md`、`docs/agent/architecture.md`、本计划中的“固定契约与验收标准”章节。项目当前规则明确不再创建 `docs/superpowers/specs` 或 `docs/superpowers/plans`。

## Global Constraints

- 容量基线是百万级用户、多实例、高并发；共享请求热路径不得把 PostgreSQL 当作每请求配置中心。
- PostgreSQL -> Redis -> 进程内缓存的权限层级不变；本功能不增加进程内限流策略缓存。
- 普通请求固定沿 `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`。
- Handler 不访问 GORM、Redis 或第三方 SDK；Service 不编写 GORM 查询；Repository 只访问 PostgreSQL。
- HTTP envelope 只允许 `code`、`data`、`message`；API 模块必须从 `unknown` 严格解析 DTO。
- JSON 和 TypeScript 字段使用 lower camel case；URL 使用小写复数资源名；Go 使用 `ID`、`HTTP`、`API` initialism。
- 所有表显式声明非空 `created_at`、`updated_at`，类型为 `TIMESTAMPTZ`；本功能策略表没有真实删除行为，不增加 `deleted_at`。
- Yes/No 仍使用 `shared/yesno` 与 `enums/yes-no`；限流策略不增加启停字段，不能通过关闭固定维度绕过保护。
- API/Worker 启动不执行 DDL、seed、回填或 Redis 全库清理；数据库改动使用维护者执行的一次性幂等 forward SQL。
- 业务错误、依赖故障和权限拒绝必须显式返回；不得返回空数组、旧快照、默认策略或假成功。
- 前端请求层负责接口失败通知一次；页面不重复通知同一接口错误。401/403 的既有认证和权限处理保持不变。
- 本计划不允许新增通用 `Adapter`、`Manager`、`Factory`、`BaseRepository`、`BaseService` 或 DI 容器。

---

## 固定契约与验收标准

### 本计划边界

- 本次只管理固定的 7 条邮件限流策略；不新增平台级覆盖、不新增自定义维度、不提供策略删除/启停、不重置已有 Redis 计数器。
- 本次不改收件规则的业务语义、不改 `lastTestError` 等历史邮件错误摘要的全局文案治理；限流页及本次新增接口不得暴露内部英文错误字符串。
- 本次只把“限流策略读取”移出发信配置查询热路径；收件规则、邮件配置、模板和日志的既有 PostgreSQL 读取行为另立计划，不在本次借机改造。
- 管理端 GET 是低频控制面请求，可以直接读取 PostgreSQL；业务发信和管理员测试是高频数据面请求，命中有效 Redis 快照时不得查询 PostgreSQL。

### 固定策略

策略集合是全局唯一的 7 条记录，值作用于所有平台，但 Redis 计数 key 继续保留平台维度。管理员只能修改 `limit` 和 `windowSeconds`，不能新增、删除、启停、修改 key、模式或维度。

| key | mode | dimension | 初始 limit | 初始 windowSeconds | 当前发送 key |
| --- | --- | --- | ---: | ---: | --- |
| `business_email_minute` | `business` | `platform_scene_email` | 1 | 60 | `mail:send:email:{platform}:{scene}:{email}` |
| `business_email_10m` | `business` | `platform_scene_email` | 5 | 600 | `mail:send:email10:{platform}:{scene}:{email}` |
| `business_ip_minute` | `business` | `platform_ip` | 10 | 60 | `mail:send:ip:{platform}:{ip}` |
| `business_scene_minute` | `business` | `platform_scene` | 30 | 60 | `mail:send:scene:{platform}:{scene}` |
| `admin_test_user_10m` | `admin_test` | `admin_user` | 5 | 600 | `mail:test:user:{adminUserID}` |
| `admin_test_ip_minute` | `admin_test` | `ip` | 10 | 60 | `mail:test:ip:{ip}` |
| `admin_test_email_10m` | `admin_test` | `email` | 3 | 600 | `mail:test:email:{email}` |

允许范围固定为 `1 <= limit <= 100000`、`1 <= windowSeconds <= 86400`。数据库 CHECK、Service 校验和前端校验必须使用同一范围。窗口单位始终是秒，Redis 计数器的 key 和已有窗口内计数不因策略编辑而清除；修改只影响后续 `Allow` 调用。

### HTTP 契约

- `GET /api/admin/v1/mail/rate-limit-policies`：使用 `message:mail:list`，返回：

```json
{
  "version": 3,
  "policies": [
    {
      "key": "business_email_minute",
      "mode": "business",
      "dimension": "platform_scene_email",
      "limit": 1,
      "windowSeconds": 60,
      "updatedAt": "2026-09-04T12:00:00Z"
    }
  ]
}
```

示例只展开第一条以说明字段；实际响应必须包含固定集合中的全部 7 条策略，不能省略、补默认值或返回 `null`。

- `PUT /api/admin/v1/mail/rate-limit-policies/:key`：使用新增 `message:mail:rate-limit:update`，请求体严格为 `{"limit": 1, "windowSeconds": 60}`，返回更新后的同一策略对象和新 `version`。
- `:key` 只接受上表 7 个 URL 安全 key；请求体不得包含 `mode`、`dimension`、`name`、`platformId`、`isEnabled` 或未知字段。
- 成功响应 `code=0`；参数范围/未知 key 为 `10001`；不存在的固定策略为 `10004`；PostgreSQL/Redis/快照状态不可用为 `10006`；权限中间件仍返回通用 `10003`。

### Redis 快照契约

- 快照 key：`mail:rate-limit:policies:v1`。
- 回源锁 key：`mail:rate-limit:policies:load-lock:v1`，随机 token，TTL 5 秒；只能由持有 token 的实例释放。
- 快照 JSON 只允许以下字段：`schemaVersion`、`state`、`version`、`policies`、`mutationToken`；每个 policy value 包含 `limit`、`windowSeconds`、`updatedAt`，`mode`/`dimension` 从固定 key catalog 派生。
- ready 示例：

```json
{
  "schemaVersion": 1,
  "state": "ready",
  "version": 3,
  "policies": {
    "business_email_minute": {"limit": 1, "windowSeconds": 60, "updatedAt": "2026-09-04T12:00:00Z"},
    "business_email_10m": {"limit": 5, "windowSeconds": 600, "updatedAt": "2026-09-04T12:00:00Z"},
    "business_ip_minute": {"limit": 10, "windowSeconds": 60, "updatedAt": "2026-09-04T12:00:00Z"},
    "business_scene_minute": {"limit": 30, "windowSeconds": 60, "updatedAt": "2026-09-04T12:00:00Z"},
    "admin_test_user_10m": {"limit": 5, "windowSeconds": 600, "updatedAt": "2026-09-04T12:00:00Z"},
    "admin_test_ip_minute": {"limit": 10, "windowSeconds": 60, "updatedAt": "2026-09-04T12:00:00Z"},
    "admin_test_email_10m": {"limit": 3, "windowSeconds": 600, "updatedAt": "2026-09-04T12:00:00Z"}
  },
  "mutationToken": null
}
```

- `invalidating` 状态必须包含非空 `mutationToken`，不得包含 `policies`，TTL 30 秒；发送请求看到该状态时返回依赖不可用，不读取旧快照。
- 快照解析使用 `json.Decoder.DisallowUnknownFields`，必须验证 schema、状态、版本、完整 7-key 集合、范围和无重复 key。
- 发布脚本使用版本 CAS：Redis 当前 ready 版本大于待发布版本时返回 `newer`，不得被旧管理请求覆盖；同版本重复发布必须幂等。
- 快照不设置业务 TTL；Redis 重启后的 key 缺失由有界回源恢复。Redis 故障或版本无法读取时直接 `503/10006`。

### RBAC 与审计映射

| 节点/API | 页面权限 | 读取权限 | 写权限 | 隐藏/说明 |
| --- | --- | --- | --- | --- |
| 邮件页“限流策略”页签 | `message:mail:view` | `message:mail:list` | `message:mail:rate-limit:update` | 页签不是独立菜单；更新 action 为 hidden action |
| `GET /mail/rate-limit-policies` | 不适用 | `message:mail:list` | 无 | 后端 Middleware 使用 `PermissionList` |
| `PUT /mail/rate-limit-policies/:key` | 不适用 | 无 | `message:mail:rate-limit:update` | 后端 Middleware 使用同一 action code |

操作日志新增 `PUT /api/admin/v1/mail/rate-limit-policies/:key`，action 为 `mail.rate-limit.update`，捕获请求和响应；只记录 key、次数、窗口、版本等非秘密字段。

### 前端验收

- 邮件页新增“限流策略”页签，不新增左侧菜单；没有 `message:mail:list` 时不发起策略 GET。
- 每条策略显示 i18n 名称、模式、作用维度、次数、窗口和保存操作；没有 `message:mail:rate-limit:update` 时输入框禁用并隐藏保存按钮。
- 次数输入 placeholder：中文 `请输入次数，如 1`，英文 `Enter a count, for example 1`。
- 窗口输入 placeholder：中文 `请输入窗口秒数，如 60`，英文 `Enter window seconds, for example 60`。
- 所有名称、说明、单位、校验、成功提示和空/失败状态接入中英文 i18n；页面不显示后端英文内部错误字符串。

---

## 文件地图

**后端**

- Create: `server/internal/module/message/mail/rate_limit_policy.go`：固定 key、DTO、快照校验、Redis 读写、singleflight 和有界回源。
- Modify: `server/internal/module/message/mail/model.go`：新增 `RateLimitPolicy` GORM Model，显式时间字段。
- Modify: `server/internal/module/message/mail/protocol.go`：新增策略输入、输出、快照和 Service/Store 接口。
- Modify: `server/go.mod`、`server/go.sum`：将 `golang.org/x/sync` 作为策略 Store 使用的直接依赖（仅在 `go mod tidy` 需要变更时落盘）。
- Modify: `server/internal/module/message/mail/repository.go`：新增固定策略列表、加锁更新、版本递增查询。
- Modify: `server/internal/module/message/mail/service.go`：移除 7 个硬编码 limit/window，按快照构造 `LimitRequest`。
- Modify: `server/internal/module/message/mail/handler.go`、`route.go`：新增 GET/PUT Handler 和权限常量。
- Modify: `server/cmd/api/main.go`：显式装配策略 Store；不在启动流程执行 DDL/seed。
- Modify: `server/internal/module/system/operationlog/rules.go`：加入策略更新审计规则。
- Modify: `server/internal/module/system/operationlog/middleware.go`：为策略更新路由注入 allowlist 内的 `policyRef` 路径参数摘要；不放宽通用敏感字段脱敏。
- Modify: `server/internal/shared/i18n/catalog.go`、对应测试：加入策略参数/状态错误的中英文 key。
- Create: `docs/database/2026-09-04-mail-rate-limit-policies.sql`：一次性幂等表结构、CHECK、7 条 seed 和权限 action seed。
- Create: `server/internal/module/message/mail/rate_limit_policy_test.go`：固定目录、快照编解码、缓存和故障路径测试。
- Create: `server/internal/module/message/mail/handler_test.go`：策略接口绑定、权限和 envelope 测试。
- Modify: `server/internal/module/message/mail/repository_test.go`、`service_test.go`、`service_concurrency_test.go`：事务、热路径和并发测试。
- Modify: `server/internal/shared/i18n/catalog_test.go`、`server/internal/module/system/operationlog/rules_test.go`：目录完整性和审计规则测试。
- Modify: `server/internal/module/system/operationlog/middleware_test.go`：验证 `policyRef` 投影和敏感字段仍被遮蔽。

**前端**

- Modify: `web/src/api/message/mail.ts`：新增明确的 `MailRateLimitPolicy`、列表响应、更新输入和严格解析器/API 方法。
- Modify: `web/src/views/message/mail/index.vue`：增加 `rateLimits` 状态、页签、加载分支和权限传递。
- Create: `web/src/views/message/mail/components/rate-limit/index.vue`：固定策略表格、输入校验、placeholder、保存和状态恢复。
- Modify: `web/src/i18n/messages/zh-CN.ts`、`web/src/i18n/messages/en-US.ts`：策略名称、维度、单位、说明、placeholder 和状态文案。
- Create/Modify: `web/tests/api/message/mail.test.ts`、`web/tests/views/message/mail/index.test.ts`、`web/tests/views/message/mail/components/rate-limit/index.test.ts`。

---

## Task 1: 先建立失败的策略协议和数据库契约

**Files:**
- Create: `server/internal/module/message/mail/rate_limit_policy_test.go`
- Modify: `server/internal/module/message/mail/model_test.go`（在现有测试中追加策略模型/权限断言）
- Create: `docs/database/2026-09-04-mail-rate-limit-policies.sql`
- Create: `server/internal/database/mail_rate_limit_migration_test.go`

**Interfaces:**
- Produces the exact constants and schema asserted by later tasks: 7 fixed keys, range `1..100000`/`1..86400`, `RateLimitPolicy` columns, `message:mail:rate-limit:update`。

- [ ] **Step 1: Write failing unit tests for the fixed catalog.**

```go
func TestRateLimitPolicyCatalogIsFixed(t *testing.T) {
    got := FixedRateLimitPolicies()
    if len(got) != 7 { t.Fatalf("policy count = %d, want 7", len(got)) }
    if got[0].Key != "business_email_minute" || got[0].Limit != 1 || got[0].WindowSeconds != 60 {
        t.Fatalf("business email policy = %+v", got[0])
    }
    if got[6].Key != "admin_test_email_10m" || got[6].Limit != 3 || got[6].WindowSeconds != 600 {
        t.Fatalf("admin email policy = %+v", got[6])
    }
}

func TestRateLimitPolicyInputRejectsUnknownKeyAndOutOfRangeValues(t *testing.T) {
    for _, input := range []RateLimitPolicyInput{
        {Key: "custom", Limit: 1, WindowSeconds: 60},
        {Key: "business_email_minute", Limit: 0, WindowSeconds: 60},
        {Key: "business_email_minute", Limit: 100001, WindowSeconds: 60},
        {Key: "business_email_minute", Limit: 1, WindowSeconds: 0},
        {Key: "business_email_minute", Limit: 1, WindowSeconds: 86401},
    } {
        if err := ValidateRateLimitPolicyInput(input); err == nil { t.Fatalf("accepted %+v", input) }
    }
}
```

- [ ] **Step 2: Run the new tests and confirm they fail because the protocol is absent.**

Run: `cd D:\admin\server; go test ./internal/module/message/mail -run 'TestRateLimitPolicyCatalogIsFixed|TestRateLimitPolicyInputRejectsUnknownKeyAndOutOfRangeValues' -count=1`

Expected: FAIL with undefined fixed-catalog symbols.

- [ ] **Step 3: Write the forward migration with exact constraints and seed.**

The SQL must create `message_mail_rate_limit_policy` with `policy_key VARCHAR(64) PRIMARY KEY`, `mode VARCHAR(16) NOT NULL`, `dimension VARCHAR(64) NOT NULL`, `limit_count INTEGER NOT NULL`, `window_seconds INTEGER NOT NULL`, `revision BIGINT NOT NULL DEFAULT 1`, `created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP`, and `updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP`. Add CHECK constraints for the seven keys, `limit_count BETWEEN 1 AND 100000`, `window_seconds BETWEEN 1 AND 86400`, and the exact mode/dimension pair for every key. Add `UNIQUE (policy_key)` only through the primary key; do not add `deleted_at`.

The DDL/seed section must have this shape, with all seven rows expanded rather than generated dynamically:

```sql
BEGIN;
CREATE TABLE IF NOT EXISTS message_mail_rate_limit_policy (
  policy_key VARCHAR(64) PRIMARY KEY,
  mode VARCHAR(16) NOT NULL,
  dimension VARCHAR(64) NOT NULL,
  limit_count INTEGER NOT NULL,
  window_seconds INTEGER NOT NULL,
  revision BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT ck_message_mail_rate_limit_policy_values
    CHECK (limit_count BETWEEN 1 AND 100000 AND window_seconds BETWEEN 1 AND 86400),
  CONSTRAINT ck_message_mail_rate_limit_policy_revision CHECK (revision >= 1),
  CONSTRAINT ck_message_mail_rate_limit_policy_shape CHECK (
    (policy_key = 'business_email_minute' AND mode = 'business' AND dimension = 'platform_scene_email') OR
    (policy_key = 'business_email_10m' AND mode = 'business' AND dimension = 'platform_scene_email') OR
    (policy_key = 'business_ip_minute' AND mode = 'business' AND dimension = 'platform_ip') OR
    (policy_key = 'business_scene_minute' AND mode = 'business' AND dimension = 'platform_scene') OR
    (policy_key = 'admin_test_user_10m' AND mode = 'admin_test' AND dimension = 'admin_user') OR
    (policy_key = 'admin_test_ip_minute' AND mode = 'admin_test' AND dimension = 'ip') OR
    (policy_key = 'admin_test_email_10m' AND mode = 'admin_test' AND dimension = 'email')
  )
);
INSERT INTO message_mail_rate_limit_policy
  (policy_key, mode, dimension, limit_count, window_seconds, revision)
VALUES
  ('business_email_minute', 'business', 'platform_scene_email', 1, 60, 1),
  ('business_email_10m', 'business', 'platform_scene_email', 5, 600, 1),
  ('business_ip_minute', 'business', 'platform_ip', 10, 60, 1),
  ('business_scene_minute', 'business', 'platform_scene', 30, 60, 1),
  ('admin_test_user_10m', 'admin_test', 'admin_user', 5, 600, 1),
  ('admin_test_ip_minute', 'admin_test', 'ip', 10, 60, 1),
  ('admin_test_email_10m', 'admin_test', 'email', 3, 600, 1)
ON CONFLICT (policy_key) DO NOTHING;

DO $$
DECLARE
  admin_platform_id BIGINT;
  mail_page_id BIGINT;
  mail_page_count INTEGER;
  action_id BIGINT;
  action_count INTEGER;
  changed BOOLEAN := FALSE;
BEGIN
  SELECT id INTO admin_platform_id
  FROM auth_platform
  WHERE code = 'admin' AND deleted_at IS NULL AND is_enabled = 1;
  IF admin_platform_id IS NULL THEN
    RAISE EXCEPTION 'active Admin platform is required';
  END IF;

  SELECT count(*) INTO mail_page_count
  FROM permission_menu
  WHERE platform_id = admin_platform_id
    AND code = 'message:mail:view'
    AND menu_type = 'page'
    AND deleted_at IS NULL;
  IF mail_page_count <> 1 THEN
    RAISE EXCEPTION 'exactly one active message:mail:view page is required';
  END IF;
  SELECT id INTO mail_page_id
  FROM permission_menu
  WHERE platform_id = admin_platform_id
    AND code = 'message:mail:view'
    AND menu_type = 'page'
    AND deleted_at IS NULL;

  SELECT count(*) INTO action_count
  FROM permission_menu
  WHERE code = 'message:mail:rate-limit:update' AND deleted_at IS NULL;
  IF action_count > 1 THEN
    RAISE EXCEPTION 'message:mail:rate-limit:update has duplicate active rows';
  END IF;
  IF action_count = 1 THEN
    SELECT id INTO action_id
    FROM permission_menu
    WHERE code = 'message:mail:rate-limit:update' AND deleted_at IS NULL;
    IF EXISTS (
      SELECT 1 FROM permission_menu
      WHERE id = action_id
        AND (platform_id IS DISTINCT FROM admin_platform_id
          OR parent_id IS DISTINCT FROM mail_page_id
          OR menu_type <> 'action'
          OR name <> '修改邮件限流策略'
          OR i18n_key IS NOT NULL
          OR path IS NOT NULL
          OR component_path IS NOT NULL
          OR icon IS NOT NULL
          OR is_enabled <> 1
          OR is_hidden <> 1)
    ) THEN
      RAISE EXCEPTION 'message:mail:rate-limit:update menu shape mismatch';
    END IF;
  ELSE
    INSERT INTO permission_menu
      (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon,
       sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES
      (admin_platform_id, mail_page_id, 'action', '修改邮件限流策略',
       'message:mail:rate-limit:update', NULL, NULL, NULL, NULL, 100, 1, 1,
       CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    RETURNING id INTO action_id;
    changed := TRUE;
  END IF;

  IF changed THEN
    UPDATE permission_access_version
    SET version = version + 1, updated_at = CURRENT_TIMESTAMP;
  END IF;
END $$;
COMMIT;
```

Insert all seven rows with the catalog defaults using `ON CONFLICT (policy_key) DO NOTHING`; never overwrite a value already managed by an administrator. In the same transaction insert the hidden `permission_menu` action `message:mail:rate-limit:update` under the active `message:mail:view` page for the Admin platform, guarded by an exact active-row count check, then increment affected `permission_access_version` rows only when the action is newly inserted. The migration must fail if the expected mail page or Admin platform is absent instead of silently creating an orphan action.

- [ ] **Step 4: Add migration assertions before implementation.**

`mail_rate_limit_migration_test.go` must execute the SQL against PostgreSQL and assert: exactly 7 policy rows (the table has no active/deleted state); all seven keys are present once; constraints reject `limit_count=0`, `window_seconds=0`, an unknown key and a mismatched mode/dimension; `created_at`/`updated_at` are `TIMESTAMPTZ NOT NULL`; rerunning the SQL preserves an edited `limit_count`; the action row is `menu_type='action'`, `is_hidden=1`, has no path/component path, and is not emitted in `menuTree`.

The test setup must use `testschema.Open` with the configured PostgreSQL DSN, `database.AutoMigrate` only for the existing platform/menu/role/access-version fixtures, and an active Admin platform plus the existing `message:mail:view` page before executing this SQL. It must never substitute SQLite or an in-memory database.

- [ ] **Step 5: Run the migration test and record the actual database result.**

Run: `cd D:\admin\server; go test ./internal/database -run TestMailRateLimitMigration -count=1`

Expected: PASS against the configured PostgreSQL test database; SQLite and in-memory substitutes are not acceptable.

## Task 2: Implement the model, repository transaction and versioned Redis Store

**Files:**
- Modify: `server/internal/module/message/mail/model.go`
- Modify: `server/internal/module/message/mail/repository.go`
- Modify: `server/internal/module/message/mail/protocol.go`
- Create: `server/internal/module/message/mail/rate_limit_policy.go`
- Modify: `server/internal/module/message/mail/rate_limit_policy_test.go`
- Modify: `server/internal/module/message/mail/repository_test.go`

**Interfaces:**
- `RateLimitPolicy` exposes `Key`, `Mode`, `Dimension`, `Limit`, `WindowSeconds`, `UpdatedAt`.
- `RateLimitPolicyInput` carries `Key`, `Limit`, `WindowSeconds` for Service validation.
- `RateLimitCatalog` carries `Version` and exactly seven immutable `RateLimitPolicy` values, including `UpdatedAt`; `RateLimitSnapshot` is the Redis wire state that wraps one catalog in `state=ready` or `state=invalidating`.
- Repository methods: `ListRateLimitPolicies(ctx context.Context) (RateLimitCatalog, error)` and `UpdateRateLimitPolicy(ctx context.Context, input RateLimitPolicyInput) (RateLimitCatalog, error)`. Both return the complete seven-row catalog; the update caller selects the changed row by key and never builds a partial snapshot.
- Store methods: `Load(ctx context.Context) (RateLimitCatalog, error)` and `Update(ctx context.Context, input RateLimitPolicyInput) (RateLimitCatalog, error)`. `Load` is the data-plane Redis path; the management Service uses the Repository directly for an authoritative GET.
- `RateLimitSnapshot` and its wire value use the following exact fields; `omitempty` is required so `invalidating` omits `policies` rather than writing `null`:

```go
type RateLimitSnapshot struct {
    SchemaVersion  int                                  `json:"schemaVersion"`
    State          string                               `json:"state"`
    Version        int64                                `json:"version"`
    Policies       map[string]rateLimitPolicyValue     `json:"policies,omitempty"`
    MutationToken  *string                              `json:"mutationToken"`
}
type rateLimitPolicyValue struct {
    Limit          int                                  `json:"limit"`
    WindowSeconds  int                                  `json:"windowSeconds"`
    UpdatedAt      time.Time                            `json:"updatedAt"`
}
```

`encodeRateLimitSnapshot` and `decodeRateLimitSnapshot` are the only Redis payload boundaries.

- [ ] **Step 1: Add repository tests for complete reads and serialized updates.**

```go
func TestUpdateRateLimitPolicyLocksAllRowsAndIncrementsRevision(t *testing.T) {
    catalog, err := repository.UpdateRateLimitPolicy(ctx, RateLimitPolicyInput{
        Key: "business_email_minute", Limit: 2, WindowSeconds: 120,
    })
    if err != nil { t.Fatal(err) }
    row := findPolicy(t, catalog.Policies, "business_email_minute")
    if catalog.Version != 2 || row.Limit != 2 || row.WindowSeconds != 120 { t.Fatalf("catalog=%+v row=%+v", catalog, row) }
    catalogAgain, err := repository.UpdateRateLimitPolicy(ctx, RateLimitPolicyInput{
        Key: "business_ip_minute", Limit: 11, WindowSeconds: 60,
    })
    if err != nil { t.Fatal(err) }
    if catalogAgain.Version != 3 { t.Fatalf("version after second update = %d, want 3", catalogAgain.Version) }
}
```

`findPolicy` is a test-only helper that fails when the key is absent; it must not synthesize a default row.

The test must run two concurrent updates and assert both complete with distinct revisions, the final list has both changes, and no row has a revision lower than 1. A missing key must return `gorm.ErrRecordNotFound`; repository errors must preserve PostgreSQL constraint identity for Service mapping.

- [ ] **Step 2: Run repository tests to verify the missing methods fail.**

Run: `cd D:\admin\server; go test ./internal/module/message/mail -run 'TestUpdateRateLimitPolicyLocksAllRowsAndIncrementsRevision' -count=1`

Expected: FAIL before implementation.

- [ ] **Step 3: Add the explicit GORM Model and repository transaction.**

`RateLimitPolicy` must map `policy_key`, `mode`, `dimension`, `limit_count`, `window_seconds`, `revision`, `created_at`, and `updated_at`; `TableName()` returns `message_mail_rate_limit_policy`. `ListRateLimitPolicies` must query all rows ordered by the fixed catalog order, require exactly seven rows, and return a complete `RateLimitCatalog`. `UpdateRateLimitPolicy` must run one PostgreSQL transaction, lock all seven rows in fixed key order with `FOR UPDATE`, compute `nextRevision = max(revision)+1`, update only the requested row's values/revision/updated_at, re-read all seven rows before commit, and return the complete new catalog. It must not issue DDL or seed.

- [ ] **Step 4: Implement strict snapshot encoding and validation.**

`rate_limit_policy.go` must define constants for the Redis keys and `rateLimitPolicySchemaVersion = 1`. Before decoding, run a token-level duplicate-key scan with the same rejection behavior as `validate.BindJSON`; then decode with `DisallowUnknownFields`, reject trailing JSON, require `state=ready` to have `mutationToken=null`, require `state=invalidating` to have a non-empty token and no policies, require version >= 1, require non-zero RFC3339 timestamps, and require an exact set of seven keys. Keep `mode`, `dimension`, and the fixed catalog order from the catalog; the Redis payload stores `limit`, `windowSeconds`, and `updatedAt`, while static mode/dimension metadata is reconstructed from the catalog.

- [ ] **Step 5: Implement cache load, singleflight and distributed miss lock.**

`Load` first reads the snapshot key. A valid ready snapshot returns without PostgreSQL. A Redis command error returns dependency failure immediately. A missing/expired snapshot enters one process-local `singleflight.Group` call. The group call acquires `mail:rate-limit:policies:load-lock:v1` with a random token and 5-second TTL; the winner reads PostgreSQL once, validates all seven rows, publishes a ready snapshot, and releases only its token. A loser polls the snapshot for at most three attempts at 50 ms intervals; it never starts an unbounded DB query. An explicit `invalidating` state or malformed/version-unknown snapshot returns dependency failure and does not read an old value; only after the key expires and becomes missing may the bounded rebuild run. No result is stored in process memory.

- [ ] **Step 6: Implement version-CAS publication and update recovery.**

`Update` must first read the current ready catalog and acquire a token-protected Redis mutation lease using the established `auth/platform` lease pattern; the temporary prior copy is used only for token-checked rollback and is never a normal send fallback. The lease replaces the ready state with `invalidating` (the lease only blocks readers; PostgreSQL remains the sole policy fact), then executes the repository transaction and publishes its complete catalog with a Redis Lua script. Only one update may hold the lease; a competing update fails closed with `10006` after bounded acquisition, without touching PostgreSQL. The script must refuse to overwrite a newer ready version, atomically set `state=ready`, and keep the invalidating lease for at most 30 seconds. If the database transaction fails, restore the prior ready snapshot with the token; if publication or rollback fails, leave the key non-ready and return dependency failure. Never return success while a known old ready snapshot remains usable. A send seeing invalidating, malformed, or unconfirmable version must fail closed; a missing key may use the bounded singleflight rebuild described above, never a stale process cache. A lower-version publisher receiving `newer` returns the database result as successful only after confirming Redis has a ready version >= its own; otherwise it returns `10006`.

- [ ] **Step 7: Run cache and failure-path tests.**

Run: `cd D:\admin\server; go test ./internal/module/message/mail -run 'TestRateLimitPolicy|Test.*Snapshot|Test.*Redis|Test.*Singleflight' -count=1`

Expected: PASS with tests for cache hit (zero DB reads), one DB read for concurrent local misses, distributed lock loser behavior, malformed snapshot, invalidating state, Redis command failure, older publisher CAS, and preservation of existing counters after policy update.

## Task 3: Replace hard-coded send limits with the policy snapshot

**Files:**
- Modify: `server/internal/module/message/mail/service.go`
- Modify: `server/internal/module/message/mail/service_test.go`
- Modify: `server/internal/module/message/mail/service_concurrency_test.go`

**Interfaces:**
- `RateLimitPolicyStore` exposes `Load(ctx context.Context) (RateLimitCatalog, error)` and `Update(ctx context.Context, input RateLimitPolicyInput) (RateLimitCatalog, error)`.
- Keep the existing constructor dependencies and append the Store explicitly: `NewService(r *Repository, keys *secretkey.KeyRing, sender Sender, rules RuleEvaluator, limiter Limiter, policyStore RateLimitPolicyStore) *Service`. The existing `Limiter` remains responsible only for Redis counters.
- Helper functions produce `[]LimitRequest` from one immutable snapshot and the existing platform/scene/email/IP values.

- [ ] **Step 1: Add failing Service tests proving runtime values come from the Store.**

```go
func TestSendUsesCurrentBusinessPolicySnapshot(t *testing.T) {
    policies := stubRateLimitPolicyStore{snapshot: snapshotWith(
        "business_email_minute", 2, 120,
        "business_email_10m", 7, 900,
        "business_ip_minute", 11, 90,
        "business_scene_minute", 31, 120,
    )}
    limiter := &recordingLimiter{}
    service := newMailServiceForTest(policies, limiter)
    _, err := service.Send(ctx, validBusinessInput())
    if err != nil { t.Fatal(err) }
    if got := limiter.requests[0]; got.Limit != 2 || got.Window != 120*time.Second { t.Fatalf("first request=%+v", got) }
    if got := limiter.requests[3]; got.Limit != 31 || got.Window != 120*time.Second { t.Fatalf("last request=%+v", got) }
}
```

Add an equivalent Admin-test assertion for limits 5/600, 10/60, and 3/600, plus a test that a Store error maps to `CodeDependencyUnavailable` before any counter call.

- [ ] **Step 2: Run Service tests and confirm hard-coded behavior fails the new assertions.**

Run: `cd D:\admin\server; go test ./internal/module/message/mail -run 'TestSendUsesCurrentBusinessPolicySnapshot|TestAdminTestUsesCurrentPolicySnapshot' -count=1`

Expected: FAIL because the current Service has no policy Store and still constructs literal values.

- [ ] **Step 3: Add the Store dependency and immutable limit builders.**

Change the explicit constructor wiring to accept the policy Store. At the start of the business path load one catalog, then build the four requests in the fixed table order. At the start of the Admin-test path load one catalog, then build its three requests. Do not call `Load` once per dimension. Do not query PostgreSQL from Service. Preserve existing recipient-rule evaluation order, existing Redis counter keys, `429/10007` for an actual denied allowance, and `503/10006` for limiter/Store failures.

- [ ] **Step 4: Test concurrency and counter preservation.**

Extend the existing PostgreSQL concurrency test to run simultaneous sends with one policy snapshot per request, assert no request uses a partial snapshot, and update a policy while counters already exist. The counter keys must remain present and their TTL/value must not be deleted by policy update.

- [ ] **Step 5: Run mail Service and concurrency tests.**

Run: `cd D:\admin\server; go test ./internal/module/message/mail -run 'Test(Send|Admin|.*Concurrency|.*Counter)' -count=1`

Expected: PASS; existing recipient-denied, rate-limited and Redis-unavailable tests remain green.

## Task 4: Expose the protected Admin API and operation audit

**Files:**
- Modify: `server/internal/module/message/mail/handler.go`
- Modify: `server/internal/module/message/mail/route.go`
- Modify: `server/internal/module/message/mail/errors.go`
- Modify: `server/internal/module/message/mail/service.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/internal/module/system/operationlog/rules.go`
- Modify: `server/internal/shared/i18n/catalog.go`
- Create: `server/internal/module/message/mail/handler_test.go`
- Modify: `server/internal/module/message/mail/service_test.go`
- Modify: `server/internal/shared/i18n/catalog_test.go`
- Create: `server/internal/module/system/operationlog/rules_test.go`

**Interfaces:**
- Handler methods: `RateLimitPolicies(c *gin.Context)` and `UpdateRateLimitPolicy(c *gin.Context)`.
- Request DTO: `RateLimitPolicyUpdateRequest { Limit int; WindowSeconds int }` with exact JSON fields.
- Response DTOs:

```go
type RateLimitPolicyResponse struct {
    Version int64           `json:"version"`
    Policy  RateLimitPolicy `json:"policy"`
}
type RateLimitPolicyListResponse struct {
    Version  int64            `json:"version"`
    Policies []RateLimitPolicy `json:"policies"`
}
```

- [ ] **Step 1: Add failing handler tests for binding, permission and envelope.**

Cover GET success with seven policies, PUT success with `{"limit":2,"windowSeconds":120}`, malformed JSON, unknown body field, invalid path key, missing permission, and Store dependency failure. Assert the exact envelope keys and status/code pairs: 200/0, 400/10001, 403/10003, 503/10006. Assert GET calls the Repository once and never calls the Redis Store, while PUT passes `c.Request.Context()` through Service and the Handler does not access GORM, Redis or third-party clients directly.

- [ ] **Step 2: Add Service methods and localized application errors.**

Implement `ListRateLimitPolicies(ctx)` and `UpdateRateLimitPolicy(ctx, input)`. Validate the exact key and numeric bounds before Repository/Store access; map not-found to `10004`, invalid input to `10001`, and cache/database failures to `10006`. Add `mail.rateLimitInvalid`, `mail.rateLimitNotFound`, and `mail.rateLimitUnavailable` message keys to both server catalogs, plus mail-local constructors in `errors.go` that preserve HTTP status/code while selecting those keys; no raw `mail repository: ...` or English implementation error may reach the envelope.

`ListRateLimitPolicies` must read PostgreSQL through the Repository on each management GET so the admin view is authoritative; it is not the business send path. `UpdateRateLimitPolicy` must use the Store's repository transaction plus Redis publication and return `{version, policy}` only after Redis confirms a ready version at least as new as the committed revision.

- [ ] **Step 3: Bind routes with the exact action permission.**

Add `GET /rate-limit-policies` with `req(PermissionList)` and `PUT /rate-limit-policies/:key` with `req(PermissionRateLimitUpdate)`. Define `PermissionRateLimitUpdate = "message:mail:rate-limit:update"`. Keep route registration under the existing Admin platform group.

- [ ] **Step 4: Wire the concrete Store explicitly.**

In `server/cmd/api/main.go`, construct `mailRepository`, `mailRateLimitStore := messagemail.NewRateLimitPolicyStore(mailRepository, redisClient)`, and call `messagemail.NewService(mailRepository, keys, storagemail.NewTencentSESClient(nil), messagemail.NewRuleService(mailRepository), mailLimiter, mailRateLimitStore)`. Do not change startup migration behavior or create a second Redis client.

- [ ] **Step 5: Add the operation-log rule and redaction assertion.**

Register `{http.MethodPut, "/api/admin/v1/mail/rate-limit-policies/:key", "mail", "mail.rate-limit.update", true, true}`. Because the global sanitizer intentionally masks fields named `key` and `code`, add this exact allowlist entry in `operationlog/middleware.go`:

```go
var routeParamSummaryAliases = map[string]map[string]string{
    http.MethodPut + " /api/admin/v1/mail/rate-limit-policies/:key": {"key": "policyRef"},
}
```

When this entry matches, project the `:key` path value into request summary field `policyRef`; do not weaken `isSensitiveField` and do not expose `policyRef` in the public API envelope. The projection is added only when the captured request body is a JSON object, and it must preserve the existing `limit`/`windowSeconds` fields. Assert the operation payload has `policyRef`, count, window and version (the nested public `key` may remain `***`) and never contains secrets, email addresses, tokens or DSNs.

Change the middleware helper boundary to `readRequestSummary(context *gin.Context, aliases map[string]string) JSON`; pass the alias selected from `routeParamSummaryAliases` before `Next`, merge it only into a decoded JSON object, then run the existing sanitizer and size limit. All routes without an alias keep byte-for-byte existing summary behavior.

- [ ] **Step 6: Run the protected API tests.**

Run: `cd D:\admin\server; go test ./internal/module/message/mail ./internal/module/system/operationlog ./internal/shared/i18n -count=1`

Expected: PASS with route permission, context propagation, exact envelope and localized error assertions.

## Task 5: Complete the RBAC Access invalidation checks

**Files:**
- Create: `server/internal/module/permission/menu/mail_rate_limit_access_test.go`：覆盖隐藏 action、授权、Access 版本失效和跨平台隔离。

**Interfaces:**
- The action is a hidden `permission_menu` row under `message:mail:view`; it is present in `permissionCodes` only when assigned through a role.

- [ ] **Step 1: Add a failing Access test for the new action.**

Seed the action without granting it to a normal role and assert it is absent from that user's `permissionCodes`; assert the page remains reachable with `message:mail:view` plus `message:mail:list`; assert the PUT returns 403. Grant the action to the role, increment access version through the normal role authorization flow, rebuild the snapshot, and assert the PUT becomes allowed. Assert the action never appears in `menuTree` and cross-platform users cannot see or use it.

- [ ] **Step 2: Verify the migration's idempotent action shape without changing the SQL file.**

The migration from Task 1 must use the live Admin platform ID and existing `message:mail:view` parent; insert only the `message:mail:rate-limit:update` action with `i18n_key`, `path`, `component_path`, and `icon` NULL, `is_hidden=1`, and no page path. It must fail on a missing/duplicate parent or an active code with the wrong shape, must not create a second mail page, must not derive permissions from string suffixes, and must not grant the action automatically to ordinary roles.

- [ ] **Step 3: Run the RBAC integration tests.**

Run: `cd D:\admin\server; go test ./internal/module/permission/... ./internal/module/message/mail -run 'MailRateLimit|Access|Permission' -count=1`

Expected: PASS for no permission, granted permission, hidden action, cross-platform isolation, Redis Access version invalidation and Redis failure closure.

## Task 6: Add the front-end DTO/API and the “限流策略” tab

**Files:**
- Modify: `web/src/api/message/mail.ts`
- Modify: `web/src/views/message/mail/index.vue`
- Create: `web/src/views/message/mail/components/rate-limit/index.vue`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Modify: `web/tests/api/message/mail.test.ts`
- Create/Modify: `web/tests/views/message/mail/index.test.ts`
- Create: `web/tests/views/message/mail/components/rate-limit/index.test.ts`

**Interfaces:**
- `MailRateLimitPolicy`: `{ key: string; mode: 'business' | 'admin_test'; dimension: string; limit: number; windowSeconds: number; updatedAt: string }`.
- `MailRateLimitSnapshot`: `{ version: number; policies: MailRateLimitPolicy[] }`.
- `MailRateLimitUpdateResult`: `{ version: number; policy: MailRateLimitPolicy }`.
- `MailRateLimitPolicyInput`: `{ limit: number; windowSeconds: number }`.
- API functions: `listMailRateLimitPolicies(): Promise<MailRateLimitSnapshot>` and `updateMailRateLimitPolicy(key: string, data: MailRateLimitPolicyInput): Promise<MailRateLimitUpdateResult>`.

- [ ] **Step 1: Add failing API parser tests.**

Assert the parser accepts the exact list/update shape, rejects unknown fields, missing `version`, duplicate/unknown policy keys, non-integer counts/windows, `windowSeconds=0`, and an invalid `updatedAt`. Assert PUT encodes only `limit` and `windowSeconds` and URL-encodes the fixed key.

- [ ] **Step 2: Implement strict DTO parsers and API methods.**

Use existing `expectExactKeys`, `expectInteger`, `expectString` and `ProtocolError`; do not use `any`, `as any`, `Record<string, any>`, silent defaults or `?? []`. Keep the server's exact seven-key set in a typed constant. Parse `version` as a positive integer and each policy's range as `1..100000`/`1..86400`.

- [ ] **Step 3: Add failing view tests for tab and permissions.**

Mount the mail page with `message:mail:list` and assert a “限流策略” tab and seven rows; assert no policy GET when list permission is absent; assert inputs are disabled and save buttons are hidden without `message:mail:rate-limit:update`; assert the two input placeholders are present in the current locale; assert a failed update restores the row's draft and leaves the server value visible after refresh.

- [ ] **Step 4: Implement the tab with explicit loading/error/edit state.**

Add `TabName = 'config' | 'templates' | 'logs' | 'rules' | 'rateLimits'`, `rateLimitPolicies` state and `loadRateLimitPolicies`. Reuse `AppTable` and existing mail tab density. Group rows visually by `mode` using section labels, but do not create nested cards. Each row has i18n name/dimension, `el-input-number` (or an equivalent numeric input) with explicit placeholder, seconds unit, and a save button. The save button sends only the edited row, keeps a per-row saving state, replaces that row with `MailRateLimitUpdateResult.policy`, and leaves errors to `request.ts`.

- [ ] **Step 5: Add all i18n strings in both locales.**

Add keys for the tab, section labels, seven policy names, seven dimensions, “times”, “seconds”, the fixed-policy explanation, count/window placeholders, save success, invalid range, loading failure and empty state. The Chinese text must include `请输入次数，如 1` and `请输入窗口秒数，如 60`; English must include the exact phrases from the front-end acceptance section.

- [ ] **Step 6: Run front-end API and view tests.**

Run: `cd D:\admin\web; pnpm vitest run tests/api/message/mail.test.ts tests/views/message/mail/index.test.ts tests/views/message/mail/components/rate-limit/index.test.ts --pool=threads --maxWorkers=1`

Expected: PASS for strict protocol, tab visibility, placeholders, permission controls, save success, loading, empty and failure states.

## Task 7: Full verification, concurrency audit and status handoff

**Files:**
- Modify: `docs/agent/STATUS.md`
- No production files are added in this task unless a failing verification identifies a directly related defect.

- [ ] **Step 1: Run focused backend verification.**

Run:

```powershell
cd D:\admin\server
go fmt ./internal/module/message/mail ./internal/module/system/operationlog ./internal/shared/i18n
go vet ./...
go test ./internal/module/message/mail ./internal/module/permission/... ./internal/module/system/operationlog ./internal/shared/i18n -count=1
go build ./...
```

Expected: all commands exit 0. Do not run a repository-wide formatter beyond the explicitly listed packages on the CRLF worktree.

- [ ] **Step 2: Run focused front-end verification.**

Run:

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

Expected: all tests pass and production build succeeds. Report any pre-existing typecheck failure separately; do not hide it with `any` or `@ts-ignore`.

- [ ] **Step 3: Execute PostgreSQL/Redis acceptance probes.**

Verify with two API instances or two concurrent test clients:

1. A Redis hit performs zero PostgreSQL policy reads for 1,000 concurrent sends.
2. Deleting the snapshot causes at most one PostgreSQL read per process and one winner across instances; all other callers observe the rebuilt Redis snapshot or `503` within the bounded retry window.
3. Updating one policy changes a subsequent send's `LimitRequest` without deleting any existing counter key.
4. Two concurrent policy updates produce distinct revisions; the lower revision cannot overwrite the higher Redis snapshot.
5. Redis unavailable, malformed, invalidating or version-unknown returns `503/10006`; no old process snapshot or default values are used.
6. Actual allowance denial remains `429/10007`; recipient denial remains its dedicated business error and is not confused with RBAC `403/10003`.

- [ ] **Step 4: Update the single project status entry.**

After implementation and verification, add one completed entry to `docs/agent/STATUS.md` recording the policy table/migration path, Redis snapshot/version protocol, RBAC action, front-end tab, commands actually run, any unrun probes and residual risks. Do not edit historical `docs/superpowers` plans or old dated SQL files.

---

## Self-review checklist

- [ ] All seven existing limit dimensions have a stable key and an explicit default.
- [ ] PostgreSQL is authoritative, but no business send request queries it on a valid Redis hit.
- [ ] Missing Redis has a bounded singleflight/lock path; Redis failure and unconfirmable version fail closed.
- [ ] Concurrent updates cannot publish an older snapshot over a newer one, and counters are never cleared by configuration edits.
- [ ] GET uses existing list permission; PUT uses `message:mail:rate-limit:update` in both frontend visibility and backend Middleware.
- [ ] The new action is hidden, not auto-granted, present in Access only after role authorization, and included in operation logs.
- [ ] Operation logs retain the target policy through the allowlisted `policyRef` summary alias without weakening global secret redaction.
- [ ] API DTOs are exact and lower camel case; frontend has explicit placeholders and complete zh-CN/en-US i18n.
- [ ] Tests cover malformed input, missing policies, Redis failure, cache stampede, concurrent writes, permission denial, UI loading/error/success and build/vet.
- [ ] No placeholder implementation, default fallback, process-local policy cache, unrelated refactor or historical plan edit is included.
