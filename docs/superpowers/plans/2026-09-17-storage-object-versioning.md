# Configuration Cache Generation Completion and Storage Object Versioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 COS 对象物理版本与私有读取，并把 System Setting、Dictionary、Mail、SMS 和两类限流策略统一到 PostgreSQL generation + durable outbox 配置缓存协议。

**Architecture:** PostgreSQL 用逻辑 COS 配置 + 不可变物理 version 保存对象位置事实，新 object key 自描述平台、规则、配置和物理 version；迁移前旧 key 与已保存对象引用直接废弃。配置型缓存按明确 namespace/scope 复用现有 generation/state/snapshot/cacheFill/outbox 协议，各模块保留业务 codec 和 Repository，不另造通用 CacheManager。普通管理 CRUD 统一 last-write-wins，generation 只在服务端承担缓存正确性。

**Tech Stack:** Go 1.26.5（以 `server/go.mod` 为准）、Gin、GORM、PostgreSQL、Redis、Tencent COS Go SDK、Vue 3、TypeScript strict、Pinia、Element Plus、Vitest。

**Specs:**
- `docs/superpowers/specs/2026-09-17-storage-upload-rule-design.md`
- `docs/superpowers/specs/2026-09-17-config-cache-generation-completion-design.md`

## Global Constraints

- 开始前读取 `AGENTS.md`、`docs/agent/README.md`、`docs/agent/STATUS.md`、`docs/agent/design.md`、`docs/agent/architecture.md` 和上方 Spec；不得扫描 `docs/local-work/**`。
- 第一条命令是 `git status --short`；现有改动只读核对并保留，不得 reset、checkout、clean、覆盖或顺手格式化无关文件。
- 不执行 `nvm use`；Node 已是 `v24.12.0`。
- 本计划列出的测试命令由维护者执行。执行 Agent 只编写测试和记录命令，未获维护者当次明确授权不得运行 Go/Vitest/build/vet。
- 不 commit、push、pull、fetch、rebase、amend 或修改历史。每个 Task 结束只汇报 diff 和等待维护者检查。
- 不在 API/Worker 启动时执行 DDL、seed、数据回填或 Redis 清理。
- 不新增 `CacheManager`、通用 `Adapter`/`Factory`、DI 容器、运行时注册器或跨模块隐式 hook。
- PostgreSQL 是唯一事实；实现前必须用只读 `pg_catalog` 查询真实表、约束、索引和数据。`docs/database/current.sql` 只作最近快照。
- 数据库变更分两个停机阶段，只能由维护者依次执行：先运行
  `docs/database/2026-09-17-storage-object-versioning.ps1 -OldAPIStopped`，再运行
  `docs/database/2026-09-17-config-cache-generation-completion.ps1 -OldAPIStopped`。两个 runner 各自负责本阶段的备份、SQL、
  固定 Redis cleanup、只读验证和回滚 cleanup；执行 Agent 不自行迁移真实库、不清 Redis、不重启 API/Worker。
- migration 不修改菜单、角色授权或权限码，不递增 `menu_version`；现有 `system/cacheGeneration` 页面自然展示新 namespace。
- HTTP envelope 仍只有 `code/data/message`；请求错误继续由 `web/src/utils/request.ts` 单次通知，页面不得重复弹接口错误。
- 本任务不把对象内容代理经 Go API，不新增逐对象登记表，不迁移或兼容现有 COS 对象。migration 清空已知数据库旧引用，
  但不调用 COS API 删除 Bucket 中的对象字节。
- 普通管理 DTO 不暴露或提交 generation/revision/version/expected 字段；同 scope 并发写由 Service 在 500ms 总预算内每 20ms 有界重试，最终 last-write-wins。
- `generation` 仅承担缓存 state/snapshot/lease/outbox 协议；COS 物理 `version` 仅承担历史对象定位。不得用 generation 伪装编辑乐观锁。
- 本计划分为共享协议、COS、System Setting、Dictionary、Mail、SMS、rate-limit 与 migration 阶段；每阶段是独立检查点，不得为“一口气”交付而合并成巨型事务或跨模块 Manager。
- 已写入的 `cacheGeneration.InitializeTx`、COS SQL/runner/migration test 原地修订并保留，不删除后重建，不清理其他模型已写内容。

---

### Task 1: 固定基线、真实 schema 预检与契约清单

**Files:**
- Read: `docs/superpowers/specs/2026-09-17-storage-upload-rule-design.md`
- Read: `docs/database/current.sql`
- Read: `server/internal/module/storage/cosConfig/*`
- Read: `server/internal/module/storage/uploadRule/*`
- Read: `server/internal/module/system/dictionary/*`
- Read: `server/internal/module/message/mail/runtime.go`
- Read: `server/internal/module/message/mail/readiness*.go`
- Read: `server/internal/module/message/mail/{config,template,recipientRule,rateLimitPolicy}/*`
- Read: `server/internal/module/message/sms/runtime.go`
- Read: `server/internal/module/message/sms/{config,template,recipientRule,rateLimitPolicy}/*`
- Read: `server/internal/module/permission/authPlatform/*`
- Read: `server/internal/shared/cacheGeneration/*`
- Read: `server/internal/shared/cacheFill/*`
- Read: `server/cmd/api/main.go`
- Read: `server/cmd/worker/main.go`
- Read: `web/src/api/storage/*`
- Read: `web/src/views/storage/object/*`
- Read: `web/src/components/UpMedia/index.vue`
- Read: `web/src/layout/components/AppAside/index.vue`

**Interfaces:**
- Consumes: 已批准 Spec 与真实 PostgreSQL。
- Produces: 实施记录中的 schema 基线、数据冲突结果、文件归属和测试命令清单。

- [ ] **Step 1: 记录工作区，不改变任何文件**

Run:

```powershell
cd D:\admin
git status --short
```

Expected: 记录所有已有修改；后续每个 diff 都必须能区分本任务文件与原有文件。

- [ ] **Step 2: 对真实 PostgreSQL 做只读结构核验**

使用 `server/.env` 或进程环境中的 `POSTGRES_DSN`，只查询以下事实并把结果写入工作日志，不打印 DSN。除 storage 三表外，
必须记录 `message_mail_rate_limit_policy`、`message_sms_rate_limit_policy` 的列/约束/索引、全部活动平台 ID，以及现有
`system_config_cache_generation/outbox` scopes：

```sql
SELECT table_name, column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name IN ('storage_cos_config','storage_upload_rule','storage_upload_rule_code')
ORDER BY table_name, ordinal_position;

SELECT conrelid::regclass AS table_name, conname, pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conrelid IN (
  'public.storage_cos_config'::regclass,
  'public.storage_upload_rule'::regclass,
  'public.storage_upload_rule_code'::regclass
)
ORDER BY conrelid::regclass::text, conname;

SELECT tablename, indexname, indexdef
FROM pg_indexes
WHERE schemaname = 'public'
  AND tablename IN ('storage_cos_config','storage_upload_rule','storage_upload_rule_code')
ORDER BY tablename, indexname;
```

Expected: 与 Spec 的“迁移前”字段一致；不一致时先修订计划中的具体 DDL，再写代码。

- [ ] **Step 3: 运行只读数据冲突预检**

```sql
SELECT platform_id, count(*) AS enabled_count
FROM storage_upload_rule
WHERE is_enabled = 1 AND deleted_at IS NULL
GROUP BY platform_id
HAVING count(*) > 1;

SELECT r.cos_config_id, count(*) AS all_rule_references
FROM storage_upload_rule AS r
GROUP BY r.cos_config_id
ORDER BY r.cos_config_id;

SELECT count(*) AS non_empty_user_avatars
FROM user_profile
WHERE avatar <> '';

SELECT value AS brand_default_avatar
FROM system_setting
WHERE setting_key = 'app.brand.default_avatar' AND deleted_at IS NULL;

SELECT generation
FROM system_config_cache_generation
WHERE namespace = 'system.setting' AND scope_key = 'global';

SELECT rule_id, code
FROM storage_upload_rule_code
WHERE (
    octet_length(code) NOT BETWEEN 1 AND 64
    OR code <> lower(btrim(code))
    OR code !~ '^[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*$'
    OR position('..' in code) > 0
    OR ('/' || code || '/') LIKE '%/.admin-storage/%'
  );
```

Expected: 多启用规则与保留段两类结果均为空才允许继续；若非空，停止实施并把精确冲突行交给维护者，不自动选赢家或改 code。
规则引用统计用于核对配置删除与新协议历史读取边界。同平台相同 code 指向不同配置/访问模式不再做歧义预检，这是允许的数据。
记录两类非空对象引用的数量/值但不读取对象内容；再用 `rg` 核对 `UpMedia`/`requestObjectURL` 的持久化调用面仍只有用户头像和
品牌默认头像。品牌默认头像活动行与 `system.setting/global` generation 必须各恰好一行且 generation >= 1；若出现新的 object key
持久化字段，先把其清空策略与回滚断言补进 Spec/Plan，不能静默遗漏。

- [ ] **Step 4: 固定接口变化表**

实现过程中按此表逐项核对：

| Surface | Before | After |
| --- | --- | --- |
| COS update | 可改 `appId` | `appId` 只读；无 generation/expected 字段 |
| Rule update | 可改 `cosConfigId/accessMode` | platform/config/access 创建后只读；无 revision/expected 字段 |
| Object URL request | `{ruleCode, objectKey}` + 上传权限 | `{objectKey}` + 仅认证 |
| Object URL response | `{url}`，private 可能空串 | `{url, expiresAt: string|null}`，private 为签名 URL |
| New object key | `<code>/YYYY/...` | `<code>/.admin-storage/v2/p.../r.../c.../v.../YYYY/...` |
| Dictionary cache | Redis-only string generation | `system.dictionary/global` PG generation + outbox |
| Mail runtime/readiness | 两套 Redis-only 协议 | `message.mail/global` 的 generation variants |
| SMS runtime/readiness | Redis-only generation/mutation | `message.sms/global` 的 generation variants |
| Mail rate-limit | 表 revision/catalog version + snapshot state | 复用 `message.mail/global`，DTO 无 version |
| SMS rate-limit | 表/DTO revision + mutation key | 复用 `message.sms/global`，DTO 无 revision |

Checkpoint: 只汇报基线，不做提交。

---

### Task 2: 先写数据库 migration 失败测试

**Files:**
- Modify existing work in place: `server/internal/database/storage_object_versioning_migration_test.go`
- Read: `server/internal/database/system_config_cache_generation_migration_test.go`

**Interfaces:**
- Consumes: Task 1 的真实 schema 形状。
- Produces: `TestStorageObjectVersioningMigration`、`TestStorageObjectVersioningMigrationRejectsMultipleEnabledRules`、
  `TestStorageObjectVersioningMigrationRejectsReservedCode`。

- [ ] **Step 1: 建立最小迁移前 fixture**

fixture 必须创建三张现有 storage 表、`permission_auth_platform`、`system_setting`、`user_profile`、
`system_config_cache_generation`、`system_config_cache_outbox`，并插入：

```text
平台 1：
  config 10（当前物理字段 bucket-a/ap-guangzhou）
  rule 20（public、enabled、codes avatar/file）
  config 12
  rule 21（private、disabled、code avatar，指向 config 12）
平台 2：
  config 11
  rule 22（private、enabled、code avatar）
对象引用：
  user_profile.avatar 为旧格式 key
  app.brand.default_avatar 为旧格式 key
```

平台 1 的重复 `avatar` 故意使用不同 config/access mode；成功 fixture 必须证明这种数据不会因旧对象兼容需求而阻止迁移。

- [ ] **Step 2: 写成功与幂等断言**

测试执行新 SQL 两次，并断言：

```go
// 结构：
// storage_cos_config 不再有 bucket/region/endpoint/bucket_domain。
// storage_cos_config 有 current_version，没有逻辑 revision。
// storage_cos_config_version 的 PK 为 (cos_config_id, version)。
// storage_upload_rule 没有 revision，有平台活动规则部分唯一索引。
// storage_upload_rule_code 无 platform_id，有 updated_at 和规则内活动唯一索引。
//
// 数据：
// 每个旧 config 恰好一个 version=1，内容等于迁移前物理字段。
// config.current_version=1。
// storage.cosconfig/<id> 各恰好一条 generation=1 和一条 generation=1 outbox。
// user_profile.avatar 与 app.brand.default_avatar 均已清空。
// 只有品牌默认头像真实发生清空时，system.setting/global generation 恰好 +1 且新增同代 outbox。
// 不存在 storage_object_legacy_route 表。
//
// 幂等：
// 第二次执行不增加 version、generation、outbox，不再次推进 storage 或 system.setting generation。
// permission_auth_platform.menu_version 完全不变。
```

- [ ] **Step 3: 写冲突整体回滚断言**

分别构造：

1. 同平台两条活动规则；
2. 已有保留路径段 `.admin-storage` 的活动编码。

Expected: 每个 fixture 单独执行 SQL 都返回错误；事务后分别断言不存在新表、新列、版本行、storage generation/outbox，头像与
品牌默认头像原值也保持不变，证明整个事务回滚，而不只检查错误字符串。

- [ ] **Step 4: 写约束和索引行为断言**

断言 migration 后：

- 同平台第二条 `is_enabled=1` 触发 `23505`；
- 同一规则第二条活动同 code 触发 `23505`；
- 不同规则、不同平台可保存相同 code；
- `storage_cos_config.current_version` 指向不存在版本时在 COMMIT 触发 FK；
- 删除被引用版本、配置或平台被 RESTRICT；
- `storage_cos_config_version.version`、`storage_cos_config.current_version` 小于 1，以及非法 Yes/No、非法 access mode均触发 CHECK；
- migration 后明确断言 `storage_upload_rule.revision` 列不存在。

- [ ] **Step 5: 把测试命令交给维护者，不自行执行**

Maintainer command:

```powershell
cd D:\admin\server
go test ./internal/database -run StorageObjectVersioningMigration -count=1
```

Expected before Task 3: FAIL，原因是 migration 文件不存在或结构尚未实现。

---

### Task 3: 实现 COS 幂等 SQL、固定 Redis 清理命令与第一阶段唯一 runner

**Files:**
- Modify existing work in place: `docs/database/2026-09-17-storage-object-versioning.sql`
- Modify existing work in place: `docs/database/2026-09-17-storage-object-versioning.ps1`
- Modify existing work in place: `server/cmd/storage-object-versioning-migration/main.go`
- Modify existing work in place: `server/cmd/storage-object-versioning-migration/main_test.go`
- Test: `server/internal/database/storage_object_versioning_migration_test.go`

**Interfaces:**
- Consumes: Task 2 测试。
- Produces: COS 第一阶段的 `-OldAPIStopped` 唯一执行入口；固定 Redis patterns；新 schema 与初始 generation/outbox。

- [ ] **Step 1: 在一个事务中写 preflight 和 DDL**

SQL 固定使用：

```sql
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

-- 先锁 storage 三表及相关平台/配置事实，检查多启用规则和保留段。
-- 任一冲突 RAISE EXCEPTION，不能自动停用或改名。

CREATE TABLE IF NOT EXISTS storage_cos_config_version (
    cos_config_id BIGINT NOT NULL,
    version BIGINT NOT NULL,
    bucket VARCHAR(128) NOT NULL,
    region VARCHAR(64) NOT NULL,
    endpoint VARCHAR(255),
    bucket_domain VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT pk_storage_cos_config_version PRIMARY KEY (cos_config_id, version),
    CONSTRAINT fk_storage_cos_config_version_config
      FOREIGN KEY (cos_config_id) REFERENCES storage_cos_config(id) ON DELETE RESTRICT,
    CONSTRAINT ck_storage_cos_config_version_value CHECK (version >= 1)
);
```

随后按 Task 2 的形状：

1. 在旧物理列仍存在时用 PL/pgSQL 动态 SQL 回填 version 1；重跑时不得静态引用已删除列。
2. 为 `storage_cos_config` 增加 `current_version` 和 CHECK，不增加逻辑 `revision`。
3. 添加 `fk_storage_cos_config_current_version`，定义为
   `FOREIGN KEY (id,current_version) REFERENCES storage_cos_config_version(cos_config_id,version) DEFERRABLE INITIALLY DEFERRED`。
4. 不为 `storage_upload_rule` 增加 revision，只建立 `ux_storage_upload_rule_platform_enabled`。
5. 为 code 表回填 `updated_at=created_at`，设 NOT NULL，删除平台 FK、旧平台唯一索引和 `platform_id`，建立
   `ux_storage_upload_rule_code_rule_code`。
6. code CHECK 固定为 trim/lower 后 `1..64` ASCII 字节、允许 `/`、每段
   `[a-z0-9][a-z0-9._-]*`、禁止 `..` 与内部段 `.admin-storage`；不得新增 GIN 或 `pg_trgm`。
7. 为每个 COS config 幂等插入
   `storage.cosconfig/<id> generation=1` 与同代 outbox。
8. 清空所有非空 `user_profile.avatar`。锁定 `app.brand.default_avatar` 与 `system.setting/global` generation 行；仅当默认头像非空时，
   清空 value、推进 system setting generation 一次并插入同代 outbox。缺少/重复品牌设置或 generation 行时迁移失败。
9. 明确断言 `to_regclass('public.storage_object_legacy_route') IS NULL`；本 migration 不创建任何旧对象路由表。
10. 最后删除 `storage_cos_config` 的四个旧物理字段并 COMMIT。

- [ ] **Step 2: 写固定 Redis cleanup**

cleanup command 只接受环境中的 `REDIS_URL`，不接受调用方传 pattern。固定列表：

```go
var forwardCleanupPatterns = []string{
    "storage:cos-config:v1:*",
    "storage:upload-rule:v1:*",
    "storage:object-url:v1:*",
    "config-cache:state:v1:system.setting:global",
    "config-cache:snapshot:v1:system.setting:global:*",
    "config-cache:fill:v1:system.setting:global:*",
}

var rollbackCleanupPatterns = []string{
    "config-cache:state:v1:storage.cosconfig:*",
    "config-cache:snapshot:v1:storage.cosconfig:*",
    "config-cache:fill:v1:storage.cosconfig:*",
    "storage:object-route:v2:*",
    "config-cache:state:v1:system.setting:global",
    "config-cache:snapshot:v1:system.setting:global:*",
    "config-cache:fill:v1:system.setting:global:*",
}
```

命令只接受固定枚举 `-mode forward|rollback`，不得接受自定义 pattern。逐 pattern 用 SCAN 小批量定位并删除，打印 pattern 与
remaining 数，不打印 Redis URL 或 payload。禁止 `KEYS`、`FLUSHDB`、`FLUSHALL`。测试用 miniredis 证明：forward 只删三类
旧 storage key 与三个精确的 `system.setting/global` pattern；rollback 只删 `storage.cosconfig` state/snapshot/fill、v2 route 与
`system.setting/global` state/snapshot/fill；两种模式都保留 Session、其它 namespace 和任意外部 key。

- [ ] **Step 3: 写 PowerShell runner**

runner 必须：

1. 无 `-OldAPIStopped` 立即非零退出。
2. 从进程环境或 `server/.env` 读取 DSN/Redis URL，但不打印值。
3. 自动在 `%LOCALAPPDATA%\Admin\backups\storage-object-versioning-<timestamp>` 执行
   `pg_dump --format=custom --schema=public`，输出 SHA256，并以 `pg_restore --list` 验证归档可读。
4. 迁移前记录 storage 行数、非空用户头像数、品牌默认头像是否非空、全部平台 `menu_version`，以及 storage 与
   `system.setting/global` generation/outbox 摘要。
5. 用 `psql -X -v ON_ERROR_STOP=1` 执行同名 SQL。
6. SQL 成功后从 `server` 运行 `go run ./cmd/storage-object-versioning-migration` 两次，第二次验证 remaining=0。
7. 校验表/列/约束/索引/回填、每个 config 的 generation/outbox、旧引用归零、legacy route 表不存在和 `menu_version` 未变化；
   若品牌默认头像原先非空，system setting generation/outbox 必须恰好推进一次，否则必须保持不变。
8. 若 SQL 已提交但 cleanup 失败，固定报文包含“PostgreSQL 已提交、Redis cleanup 未完成；可安全重复执行本 runner”，并
   exit 1。
9. 支持 `-CleanupOnly -OldAPIStopped`，只重试固定 Redis cleanup 和计数。
10. 重跑时比较迁移前后 storage 与 system setting generation/outbox 摘要，不假设 pending 数或 published 状态仍是首次 seed；
    第二次完整执行不得再推进任一 generation 或新增 outbox。
11. 支持互斥的 `-RollbackCleanupOnly -OldAPIStopped`，只调用 cleanup command 的 `-mode rollback`；该模式不得连接或修改
    PostgreSQL，用于恢复 public schema 备份后清除新二进制的 Redis 数据。
12. SQL 使用表锁和破坏旧查询形状的结构验证作为硬防线：提交后必须确认旧物理列与旧 code 平台列已经不存在，使仍存活的
    旧二进制只能明确失败，不能继续签发旧格式 object key。runner 无法可靠证明远端/多实例进程已经停止，因此不得用本地进程名或
    端口扫描冒充硬校验；`-OldAPIStopped` 仍是维护者对部署面的显式门槛。

- [ ] **Step 4: 交给维护者验证红绿，不执行真实迁移**

Maintainer commands:

```powershell
cd D:\admin\server
go test ./cmd/storage-object-versioning-migration -count=1
go test ./internal/database -run StorageObjectVersioningMigration -count=1
```

Expected after implementation: PASS。不要运行 `.ps1`。

Checkpoint: 展示 SQL、runner、cleanup diff 和 migration 测试结果（由维护者提供），不提交。

---

### Task 4: 为动态 scope 增加原子 generation 初始化

**Files:**
- Preserve and finish existing edits: `server/internal/shared/cacheGeneration/repository.go`
- Preserve and finish existing edits: `server/internal/shared/cacheGeneration/repository_test.go`

**Interfaces:**
- Consumes: 现有 `Event`、`Scope` 和 outbox schema。
- Produces:

```go
var ErrGenerationAlreadyExists = errors.New("cache generation row already exists")

func (r *Repository) InitializeTx(
    ctx context.Context,
    tx *gorm.DB,
    scope Scope,
    now time.Time,
) (Event, error)
```

- [ ] **Step 1: 写失败测试**

覆盖 nil tx、非法 scope、首次初始化、调用方事务回滚、顺序重复 scope，以及两个独立 GORM 实例并发初始化同一 scope。
首次初始化必须在同一调用方事务插入 generation=1 和唯一 outbox generation=1；并发结果必须恰好一个成功、一个
`ErrGenerationAlreadyExists`，最终只有一行 generation 和一行 outbox。

- [ ] **Step 2: 实现最小 Repository 方法**

只在 `shared/cacheGeneration` 增加数据库原语，不增加业务注册表或 Manager。只把约束名
`pk_system_config_cache_generation` 的 PostgreSQL `23505` 映射为 `ErrGenerationAlreadyExists`；
`uq_system_config_cache_outbox_generation` 或任何其他 `23505` 表示 schema/事务事实异常，必须保留为依赖错误，不能误报“已初始化”。

- [ ] **Step 3: 交给维护者运行定向测试**

```powershell
cd D:\admin\server
go test ./internal/shared/cacheGeneration -run InitializeTx -count=1
```

Expected: PASS，原有 `AdvanceTx/relay` 测试不改变。

---

### Task 5: 改造 COS Model、DTO 与 Repository 的版本事实

**Files:**
- Modify: `server/internal/module/storage/cosConfig/model.go`
- Modify: `server/internal/module/storage/cosConfig/request.go`
- Modify: `server/internal/module/storage/cosConfig/response.go`
- Modify: `server/internal/module/storage/cosConfig/repository.go`
- Modify: `server/internal/module/storage/cosConfig/model_test.go`
- Modify: `server/internal/module/storage/cosConfig/repository_test.go`
- Modify: `server/internal/module/storage/cosConfig/service_test.go`

**Interfaces:**
- Consumes: Task 3 schema、Task 4 `InitializeTx`。
- Produces:

```go
type Model struct {
    ID int64
    Name string
    AppID string
    SecretIDCiphertext string
    SecretKeyCiphertext string
    CurrentVersion int64
    IsEnabled yesno.Value
    Remark string
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt
}

type Version struct {
    CosConfigID int64
    Version int64
    Bucket string
    Region string
    Endpoint *string
    BucketDomain *string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type SafeValue struct {
    ID int64
    Name string
    AppID string
    Bucket string
    Region string
    Endpoint *string
    BucketDomain *string
    IsEnabled yesno.Value
    HasCredentials bool
    Remark string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

- [ ] **Step 1: 先把 repository fixture 改成新 schema 并写失败断言**

覆盖：

- list/get 只 join `current_version` 返回当前物理字段；管理 DTO 不读取或返回 generation/version；
- create 同事务插入逻辑行、version 1、generation 1、outbox 1；
- name、remark、Secret 或状态的真实改动只推进 generation，不新增物理版本；
- Bucket/Region/Endpoint/Domain 任一变化新增下一物理版本并推进指针；
- 两个实例并发写同一 config 时内部 generation 冲突使失败事务整体回滚，Service 重新取得 lease 后有界重试；
- delete 统计全部规则引用，包括已禁用/已软删规则；
- no-op 不推进 generation、不新增物理版本或 outbox。

- [ ] **Step 2: 收紧请求 DTO**

创建输入保留 `appId` 和物理字段。更新输入删除 `appId`，固定为：

```go
type UpdateInput struct {
    Name string
    SecretID *string
    SecretKey *string
    Bucket string
    Region string
    Endpoint *string
    BucketDomain *string
    Remark string
}
```

Secret 必须“两者都省略”或“两者都为非空字符串”；只传一个或空白均为 400。状态和删除请求不携带 expected 字段。严格 JSON
继续拒绝旧客户端发送 update `appId`、`expectedGeneration` 或 `expectedRevision`。

- [ ] **Step 3: 改 Repository 查询与事务**

- 管理列表/详情通过 current version join，不把物理字段或 version 加回逻辑 Model，也不 join generation。
- 创建先插逻辑行（`current_version=1`），再插 version 1，最后在同一事务调用 `InitializeTx` 建立 generation 1/outbox 1。
- 更新、状态和删除由 Service 传入内部 `baseGeneration`；Repository 事务先 `FOR UPDATE` 锁 COS 逻辑行并计算真实变化，物理变化才插
  新版本，最后调用 `AdvanceTx(baseGeneration)`。若 generation 已变化，整个业务事务回滚，Service 从头重试。
- 每次真实写入返回 `MutationResult{Changed,Generation,OutboxID}`。规范化后 no-op 直接提交只读事务并返回 `Changed=false`，不先推进
  generation，也不写需要哨兵错误回滚的临时 outbox；Service 只 rollback Redis lease。
- Delete 的引用检查不加 `deleted_at` 条件。
- Name 唯一冲突和未找到保留现有公开语义；`ErrUpdating/ErrGenerationChanged` 仅供 Service 判定内部重试，预算耗尽映射依赖繁忙，
  不映射 409。

- [ ] **Step 4: 交给维护者运行 repository 测试**

```powershell
cd D:\admin\server
go test ./internal/module/storage/cosConfig -run 'Model|Repository' -count=1
```

Expected: PASS。

---

### Task 6: 接入 COS generation 快照、lease、outbox 与有界冷回源

**Files:**
- Create: `server/internal/module/storage/cosConfig/cache.go`
- Create: `server/internal/module/storage/cosConfig/cache_test.go`
- Modify: `server/internal/module/storage/cosConfig/service.go`
- Modify: `server/internal/module/storage/cosConfig/service_test.go`
- Modify: `server/internal/module/storage/cosConfig/errors.go`

**Interfaces:**
- Consumes: `cacheGeneration.Repository`、`cacheGeneration.Store`、`cacheFill.Lease`。
- Produces:

```go
const cosConfigGenerationNamespace = "storage.cosconfig"

type RuntimeVersion struct {
    Version int64
    Bucket string
    Region string
    Endpoint *string
    BucketDomain *string
}

type RuntimeConfig struct {
    ID int64
    AppID string
    SecretIDCiphertext string
    SecretKeyCiphertext string
    CurrentVersion int64
    IsEnabled yesno.Value
    Deleted bool
    Versions []RuntimeVersion
}

func (s *Service) Runtime(ctx context.Context, id int64) (RuntimeConfig, error)
func (s *Service) SetCache(cache *Cache)
func (s *Service) SetGenerations(repo *cachegeneration.Repository, store *cachegeneration.Store)
func (s *Service) SetLogger(logger *slog.Logger)
```

- [ ] **Step 1: 写 strict snapshot codec 测试**

payload 固定含 `schemaVersion`、`generation`、逻辑字段和 `versions`。拒绝未知字段、重复字段、trailing JSON、坐标不匹配、
重复/乱序/缺失 current version、空密文和非法物理字段。断言 Redis 中只有密文，没有 Secret 明文。

- [ ] **Step 2: 实现 Cache**

- scope 是 `storage.cosconfig/<decimal id>`，variant 固定 `runtime`；
- snapshot TTL 5 分钟；
- `GetRuntime/PutRuntime` 使用 generation 不可变 key；
- `TryFill/TryRepair` 复用 `shared/cacheFill`；
- 发布前重新读取 state，必须仍为同一 `ready generation`。

- [ ] **Step 3: 写多实例与失败语义测试**

至少两个 Service/Redis client，覆盖：

1. ready 命中零 PostgreSQL；
2. 同一冷 key 只一个 leader 查询 PostgreSQL；
3. fill 期间 generation 前进，旧结果不写入；
4. state missing/corrupt 单 leader 从 PostgreSQL authoritative generation repair；
5. Redis error 不回源 PostgreSQL；
6. invalidating 等待至 500ms 预算后失败闭合；测试为 Service 注入未导出的 fake clock 与 wait hook，wait 每次立即推进 20ms，
   不使用真实 sleep；断言假时间达到 500ms 后返回依赖错误、循环次数有界且 PostgreSQL 查询数为 0；
7. snapshot corrupt 重建；
8. PostgreSQL 提交、`Lease.Commit` 失败仍返回业务成功且 outbox pending；
9. 事务回滚恢复 base ready；
10. 提交结果不确定时按权威 generation 处理，不恢复 stale ready。

- [ ] **Step 4: 实现 COS-specific mutation orchestration**

沿用统一协议但保留在 cosConfig 模块；客户端不提供 generation：

```text
existing config:
  within 500ms budget: read PostgreSQL current -> Acquire exact-generation token lease -> renew
  -> repository transaction (lock/update business -> changed only AdvanceTx(base)/outbox)
  -> stop renew
  -> no-op Rollback lease / changed Commit next generation
  -> MarkPublishedIfUnclaimed
  -> ErrUpdating/ErrGenerationChanged: bounded 20ms wait and restart; no 409

new config:
  repository transaction (config + version1 + InitializeTx/outbox)
  -> Reconcile ready generation1
  -> MarkPublishedIfUnclaimed
```

提交后的 Redis/mark-published 失败只记录不含凭据的日志，不返回失败。创建发布失败由 missing-state repair/outbox relay 恢复。
写预算测试注入未导出的 fake clock 与 wait hook，立即推进 20ms；断言最多 500ms、循环次数有界、没有真实 sleep。Redis I/O 错误不重试
到 PostgreSQL 写入，直接失败闭合。

- [ ] **Step 5: 交给维护者运行缓存与 Service 测试**

```powershell
cd D:\admin\server
go test ./internal/module/storage/cosConfig -run 'Cache|Runtime|Generation|Mutation' -count=1
```

Expected: PASS。

---

### Task 7: 修正上传规则关系模型、互斥启用与 last-write-wins

**Files:**
- Modify: `server/internal/module/storage/uploadRule/model.go`
- Modify: `server/internal/module/storage/uploadRule/request.go`
- Modify: `server/internal/module/storage/uploadRule/response.go`
- Modify: `server/internal/module/storage/uploadRule/repository.go`
- Modify: `server/internal/module/storage/uploadRule/service.go`
- Modify: `server/internal/module/storage/uploadRule/model_test.go`
- Modify: `server/internal/module/storage/uploadRule/repository_test.go`
- Modify: `server/internal/module/storage/uploadRule/service_test.go`

**Interfaces:**
- Consumes: 新 storage schema 与 `cosconfig.Runtime`。
- Produces: 规则内 code 唯一、跨规则可重复、每平台至多一个 enabled、immutable route fields。

- [ ] **Step 1: 先反转旧行为测试**

删除/改写任何“允许同平台多个启用规则”与“平台内 code 冲突”的旧断言，新增：

- 创建 enabled 规则自动停用同平台旧规则；
- 两个 Service 并发启用后恰好一个 enabled；
- 不同规则可保存相同 code；
- 同规则输入重复 code 规范化为一个且保持首次顺序；
- update 可增删改 code；
- update JSON 含 `cosConfigId`、`accessMode` 或 `platformId` 被严格拒绝；
- update/status/delete JSON 含 `revision` 或 `expectedRevision` 被严格拒绝；
- code 任一路径段为 `.admin-storage` 返回 400。

- [ ] **Step 2: 修改 Model 与 DTO**

`RuleCode` 删除 `PlatformID`、增加 `UpdatedAt`；`Model/RuleValue` 不增加 revision。更新输入固定为：

```go
type UpdateInput struct {
    Codes []string
    Name string
    MaxFileSizeBytes int64
    AllowedExtensions []string
    AllowedMimeTypes []string
    Remark string
}
```

状态、删除不带 expected 字段。创建仍含 platform/config/access/isEnabled。

- [ ] **Step 3: 实现稳定锁顺序**

创建 enabled 或启用已有规则时固定：

```text
SELECT permission_auth_platform ... FOR UPDATE
-> 确认平台活动
-> 锁目标 COS config 并确认活动
-> 锁/停用同平台其他活动规则（按 id）
-> 创建或启用目标
```

部分唯一索引是最终兜底。禁用、普通编辑和删除锁目标行，按最后提交事务生效；不比较 revision。唯一冲突保留明确冲突语义，
不把 `23505` 伪装成依赖错误。

- [ ] **Step 4: 保留规范化 code 子表差集更新**

`ReplaceCodes` 只软删移除项、为新增项插入新行，更新 `updated_at`。跨规则重复不报错；同规则竞态由部分唯一索引阻止。
列表和详情继续按 code row `id` 排序；keyword 可用规则名或活动 code `EXISTS` 命中，不加无依据的全文索引。

- [ ] **Step 5: 收紧配置删除规则**

cosConfig 删除检查改为“是否存在任何 `storage_upload_rule.cos_config_id` 引用”，包含已软删规则。已引用配置只能停用。

- [ ] **Step 6: 交给维护者运行规则测试**

```powershell
cd D:\admin\server
go test ./internal/module/storage/uploadRule -run 'Rule|Code|Status|Concurrent' -count=1
```

Expected: PASS。

---

### Task 8: 实现 v2 object key 与不可变 rule route cache

**Files:**
- Create: `server/internal/module/storage/uploadRule/object_key.go`
- Create: `server/internal/module/storage/uploadRule/object_key_test.go`
- Create: `server/internal/module/storage/uploadRule/route_cache.go`
- Create: `server/internal/module/storage/uploadRule/route_cache_test.go`
- Modify: `server/internal/module/storage/uploadRule/repository.go`
- Modify: `server/internal/module/storage/uploadRule/service_test.go`

**Interfaces:**
- Consumes: 创建后不可变的 rule platform/config/access。
- Produces:

```go
type ObjectCoordinates struct {
    PlatformID int64
    RuleID int64
    CosConfigID int64
    Version int64
}

type ObjectRoute struct {
    RuleID int64
    PlatformID int64
    CosConfigID int64
    AccessMode string
}

func generateObjectKey(code string, coordinates ObjectCoordinates, extension string, now time.Time) (string, error)
func parseV2ObjectKey(objectKey string) (ObjectCoordinates, error)
```

- [ ] **Step 1: 写 object key parser/generator 失败测试**

在 `object_key.go` 定义唯一 Go 常量 `internalObjectMarker = ".admin-storage"`，生成器、解析器和后端 code 校验共同使用。
覆盖固定格式、合法的 `article/cover`、64/65 字节边界、以 `.` 开头、空段、Unicode、缺段、重复 marker、零/负 ID、前导零策略、
非法日期、非法 UUID/扩展名、`..`、反斜杠、控制字符，以及旧 `<code>/YYYY/...` key 明确返回无效请求错误。

- [ ] **Step 2: 实现 key 生成与严格解析**

生成格式必须逐字为：

```text
<code>/.admin-storage/v2/p<platformId>/r<ruleId>/c<configId>/v<version>/YYYY/MM/DD/<32 lowercase hex>.<ext>
```

解析不使用 code 推断路由，只提取并校验 `p/r/c/v`；整体 object key 仍传给 COS GET/URL。

- [ ] **Step 3: 写 route cache 测试**

Redis key 固定为：

```text
storage:object-route:v2:rule:<ruleId>
```

payload 严格带 `schemaVersion`，TTL 24 小时。覆盖命中零 PG、单 leader 冷回源、Redis error fail closed、损坏重建、500ms API 预算、
规则已软删仍能加载，以及 route 不存在时明确失败。

- [ ] **Step 4: 实现只读 Repository**

- rule route 查询按 ID 读取 platform/config/access，必须包含软删行，不读取 mutable code/name/status；
- 这些 route 字段不可由运行时管理 API 修改，因此不创建 generation 或 mutation lease。

- [ ] **Step 5: 交给维护者运行定向测试**

```powershell
cd D:\admin\server
go test ./internal/module/storage/uploadRule -run 'ObjectKey|RouteCache' -count=1
```

Expected: PASS。

---

### Task 9: 增加 Presigned GET 并闭合上传/读取 Service

**Files:**
- Modify: `server/internal/storage/cos/types.go`
- Modify: `server/internal/storage/cos/client.go`
- Modify: `server/internal/storage/cos/client_test.go`
- Modify: `server/internal/module/storage/uploadRule/service.go`
- Modify: `server/internal/module/storage/uploadRule/handler.go`
- Modify: `server/internal/module/storage/uploadRule/route.go`
- Modify: `server/internal/module/storage/uploadRule/credential_test.go`
- Modify: `server/internal/module/storage/uploadRule/handler_test.go`
- Modify: `server/internal/module/storage/uploadRule/service_test.go`

**Interfaces:**
- Consumes: Tasks 6-8。
- Produces:

```go
type GetRequest struct {
    ObjectKey string
}

type GetResult struct {
    URL string
    ExpiresAt time.Time
}

const GetPresignValidity = 10 * time.Minute

type Presigner interface {
    PresignPut(context.Context, Credentials, PutRequest) (PutResult, error)
    PresignGet(context.Context, Credentials, GetRequest) (GetResult, error)
}

type ObjectURLResult struct {
    URL string `json:"url"`
    ExpiresAt *time.Time `json:"expiresAt"`
}
```

- [ ] **Step 1: 先写 COS GET 签名测试**

使用固定 credentials 和 object key，断言方法为 GET、签名 URL 有效期约为独立的 `GetPresignValidity=10m`、不添加 public ACL header，
非法 key 与不完整 credentials 返回错误。

- [ ] **Step 2: 实现 `PresignGet`**

复用 `sdkClient` 和与 PUT 相同的 object key 防护，调用 SDK `GetPresignedURL` 的 GET 路径，返回 URL 与精确过期时间；不发起真实 GET。
容量记录必须写入测试/STATUS：周期刷新已取消，因此签名请求量约为“每个标签页实际首次展示的私有对象数 + 图片失败/用户下载时的
按需刷新”，不是每 10 分钟固定请求。即使将来恢复周期刷新，连续活跃请求率也应按
`activeTabs * visiblePrivateObjects / (validity - refreshLead)` 估算；当前设计不承担该放大。

- [ ] **Step 3: 重写上传凭证路径**

`FindUploadTarget` 只返回启用规则的 rule ID、platform ID、config ID、code、限制与 access mode，不再 join COS 物理字段或密文。
Service 调用 `cosConfig.Runtime(configID)`，要求配置活动，取 current physical version，生成 v2 key，再解密当前 Secret 并 PresignPut。
public 仍返回 `publicUrl`，private 不返回 public URL。

- [ ] **Step 4: 重写 object-url 路径**

请求 DTO 只有：

```go
type objectURLRequest struct {
    ObjectKey string `json:"objectKey"`
}
```

流程：

1. 只接受 v2 key；旧格式、缺段或损坏结构直接返回无效请求；
2. parse -> rule route cache -> 登录平台与 key 中 platform/config 坐标交叉校验；
3. 读取 `cosConfig.Runtime` 并找到 key 指定的 physical version；
4. public 用该 version 的 `bucket_domain` 生成 escaped 普通 URL，`ExpiresAt=nil`；
5. private 解密当前 Secret，用该 version 的 bucket/region/endpoint `PresignGet`；
6. 不因规则/配置停用或规则软删拒绝新协议历史对象；
7. 配置一旦被任何规则引用就不可删除，因此不存在 deleted config 历史读取分支。

- [ ] **Step 5: 调整路由权限**

```go
func RegisterCredentialRoute(
    r *gin.RouterGroup,
    h *Handler,
    authMiddleware gin.HandlerFunc,
    req func(string) gin.HandlerFunc,
) {
    r.POST("/storage/upload-credential", authMiddleware, req("storage:object:upload"), h.Credentials)
    r.POST("/storage/object-url", authMiddleware, h.ObjectURL)
}
```

测试必须证明 object-url 仍经过 auth，但不会调用 `req`；未认证、旧格式 key、跨平台、损坏 key、缺物理 version、私有签名失败
均返回明确错误，不返回空 URL。

- [ ] **Step 6: 交给维护者运行后端存储测试**

```powershell
cd D:\admin\server
go test ./internal/storage/cos ./internal/module/storage/cosConfig ./internal/module/storage/uploadRule -count=1
```

Expected: PASS。

---

### Task 10: 在 API 入口显式装配共享 generation 与 storage caches

**Files:**
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`（若现有装配测试在其他文件，修改实际文件）

**Interfaces:**
- Consumes: `cosconfig.Service`、`uploadrule.Service` 新依赖。
- Produces: API 所有 storage 热路径依赖显式且非 nil；Worker 继续复用现有通用 outbox relay。

- [ ] **Step 1: 写装配失败测试或可注入 assembly 断言**

断言缺 Redis cache、generation repository/store、route cache 或 COS signer 时启动返回明确配置错误，不延迟到首个请求 panic。

- [ ] **Step 2: 修改显式装配**

复用现有实例并采用业务中性局部变量名：

```go
configGenerationRepository := cachegeneration.NewRepository(postgres.GORM)
configGenerationStore := cachegeneration.NewStore(redisClient)

cosConfigRepository := cosconfig.NewRepository(postgres.GORM)
cosConfigRepository.SetGenerations(configGenerationRepository)
cosConfigService := cosconfig.NewService(cosConfigRepository, keys, cosClient)
cosConfigCache := cosconfig.NewCache(redisClient)
cosConfigCache.SetStateStore(configGenerationStore)
cosConfigService.SetCache(cosConfigCache)
cosConfigService.SetGenerations(configGenerationRepository, configGenerationStore)
cosConfigService.SetLogger(logger)

uploadRuleRepository := uploadrule.NewRepository(postgres.GORM)
uploadRouteCache := uploadrule.NewRouteCache(redisClient)
uploadRuleService := uploadrule.NewService(
    uploadRuleRepository,
    keys,
    cosClient,
    cosConfigService,
    uploadRouteCache,
)
```

不要为此引入容器或全局注册器。`cmd/worker` 的现有 `cachegeneration.Relay` 已按 namespace 通用处理 outbox，无需新增 COS worker。

- [ ] **Step 3: 交给维护者运行编译/装配检查**

```powershell
cd D:\admin\server
go test ./cmd/api -count=1
go build ./cmd/api ./cmd/worker
```

Expected: PASS。

---

### Task 11: 更新前端 storage DTO 与管理页面

**Files:**
- Modify: `web/src/api/storage/cosConfig.ts`
- Modify: `web/src/api/storage/uploadRule.ts`
- Modify: `web/src/views/storage/object/index.vue`
- Modify: `web/src/views/storage/object/storageForms.ts`
- Modify: `web/src/views/storage/object/storageView.ts`
- Modify: `web/src/views/storage/object/components/types.ts`
- Modify: `web/src/views/storage/object/components/ConfigDialog/index.vue`
- Modify: `web/src/views/storage/object/components/ConfigTable/index.vue`
- Modify: `web/src/views/storage/object/components/RuleDialog/index.vue`
- Modify: `web/src/views/storage/object/components/RuleTable/index.vue`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Modify: `web/tests/api/storage/cosConfig.test.ts`
- Modify: `web/tests/api/storage/uploadRule.test.ts`
- Modify: `web/tests/views/storage/object/index.test.ts`

**Interfaces:**
- Consumes: Tasks 5、7、9 HTTP DTO。
- Produces: strict TS parsers、不可编辑字段 UI、无管理版本字段的 last-write-wins 请求、互斥启用确认。

- [ ] **Step 1: 先更新 API parser 测试**

`CosConfig` 和 `UploadRule` 不增加 currentVersion/generation/revision。COS update/status/delete 与 rule update/status/delete input 均不含
expected 字段。COS update 类型不含 `appId`；rule update 类型不含 `platformId/cosConfigId/accessMode`。为旧字段误传写运行时/类型层
断言，响应 parser 也必须拒绝服务端意外暴露 generation/version/revision。

实现前用 `rg` 固定调用面并把结果写入 Task 记录：`UpdateCosConfigInput/updateCosConfig` 与
`UpdateUploadRuleInput/updateUploadRule` 当前只有 `web/src/views/storage/object/index.vue` 直接调用；若执行时出现新增调用者，必须
一并迁移并补测试。`BrandSettingsPanel` 不复用两类管理 DTO，它通过 `UpMedia` 间接依赖 object-url，归 Task 12 覆盖；
`app.brand.default_avatar` 和用户头像仍只保存 object key，不改变 system setting/profile DTO。

- [ ] **Step 2: 调整 COS dialog/table**

- 编辑态 AppID 用 disabled input 展示但不进入 update payload；
- Bucket/Region/Endpoint/Domain 保持可编辑；
- Secret 两项同时留空表示不轮换，任一填写则两项必填；
- table 不展示物理 version 或缓存 generation；
- 保存、状态、删除发送纯业务字段，成功后重新列表；请求错误仍由请求层单次通知，页面不重复 `ElNotification`。

- [ ] **Step 3: 调整 rule dialog/table**

- 编辑态平台、COS 配置、访问模式 disabled，且不进入 payload；
- codes 保持现有可创建 tag 输入和输入顺序；
- 去掉“创建后编码不可修改”文案，改为“同一规则内去重，可跨规则重复”；
- 创建时选择 enabled 或从 disabled 切换 enabled，若同平台已有启用规则，使用现有 `ElMessageBox` 明确确认会自动停用旧规则；
- 状态、更新、删除不携带 revision/expected 字段，成功后重新列表。

- [ ] **Step 4: 确认 cache generation 页面无需业务特判**

只补测试数据（如果现有测试覆盖 namespace 列表）证明复合 row key 能同时显示：

```text
storage.cosconfig / 1
storage.cosconfig / 2
```

不要新增菜单、路由、权限、过滤常量或专用 COS generation 页面。

- [ ] **Step 5: 交给维护者运行前端管理页测试**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/storage/cosConfig.test.ts tests/api/storage/uploadRule.test.ts tests/views/storage/object/index.test.ts --pool=threads --maxWorkers=1
```

Expected: PASS。

---

### Task 12: 更新 object-url DTO 与私有对象按需预览

**Files:**
- Modify: `web/src/api/storage/upload.ts`
- Modify: `web/tests/api/storage/upload.test.ts`
- Modify: `web/src/components/UpMedia/index.vue`
- Modify: `web/tests/components/UpMedia/src/index.test.ts`
- Modify: `web/src/layout/components/AppAside/index.vue`
- Modify: `web/tests/layout/components/AppAside.test.ts`
- Read: `web/src/views/system/setting/components/BrandSettingsPanel/index.vue`
- Read: `web/src/views/user/profile/components/ProfileDetailsForm/index.vue`
- Test: `web/tests/views/system/setting/index.test.ts`
- Test: `web/tests/views/user/profile/index.test.ts`
- Test: `web/tests/store/brand.test.ts`

**Interfaces:**
- Consumes: `POST /api/v1/storage/object-url {objectKey} -> {url,expiresAt}`。
- Produces:

```ts
export interface ObjectURLResult {
  url: string
  expiresAt: string | null
}

export async function requestObjectURL(objectKey: string): Promise<ObjectURLResult>
```

- [ ] **Step 1: 先写 strict API 测试**

覆盖 public `expiresAt:null`、private ISO UTC 字符串、空 URL、未知字段、缺字段、非法日期。parser 必须拒绝协议损坏，不返回空成功。

- [ ] **Step 2: 改 UpMedia 的多对象生命周期**

为每个 object key 保存 `{url, expiresAt, requestId}`，不创建周期 timer：

- 现有值加载时只传 object key；
- private 在首次绑定、已过期后再次使用，或 `<img>` 加载失败时至多发起一个受 request ID 保护的刷新；
- public `expiresAt=null` 直接复用；
- key 删除、model 替换、组件卸载时使旧 request ID 失效；
- 慢旧响应不得覆盖新 key 的 URL；
- private 文件 PUT 成功后立即请求 object-url，不能因没有 `publicUrl` 留空预览；
- resolver 失败维持该项错误/空预览，不在组件重复通知请求错误。

测试断言成功加载后即使超过 `expiresAt` 也没有后台请求；下一次实际使用或 image error 才刷新；卸载后不请求，竞态响应被忽略。

- [ ] **Step 3: 改侧栏头像生命周期**

移除硬编码 `requestObjectURL('avatar', objectKey)`。侧栏只在 object key 首次出现或图片加载失败时解析；avatar/default avatar
切换、登出和卸载递增 request ID。成功显示后不按 10 分钟周期刷新。

- [ ] **Step 4: 交给维护者运行组件测试**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/storage/upload.test.ts tests/components/UpMedia/src/index.test.ts tests/layout/components/AppAside.test.ts tests/views/system/setting/index.test.ts tests/views/user/profile/index.test.ts tests/store/brand.test.ts --pool=threads --maxWorkers=1
```

Expected: PASS，无 pending timer 或 unhandled rejection；migration 后旧引用为空，新上传只持久化 v2 object key，品牌默认头像与
用户头像预览统一经过新 object-url DTO。

---

### Task 13: 跨实例、故障与性能验收测试

**Files:**
- Modify: `server/internal/module/storage/cosConfig/service_test.go`
- Modify: `server/internal/module/storage/uploadRule/service_test.go`
- Modify: `server/internal/module/storage/uploadRule/credential_test.go`
- Modify: `server/internal/shared/cacheGeneration/relay_test.go`（仅在现有测试不能覆盖新动态 scopes 时）

**Interfaces:**
- Consumes: 完整后端实现。
- Produces: 对 Spec 故障矩阵与查询预算的可重复证明。

- [ ] **Step 1: 加入查询预算探针**

使用包装 `gorm.io/gorm/logger.Interface` 的测试 query counter，在 `Trace` 中只统计执行成功的 `SELECT`，并按 SQL 中的
`storage_cos_config`、`storage_cos_config_version`、`storage_upload_rule` 分类；fixture/warmup 完成后
原子 reset，再开始被测请求。该方式同时覆盖 GORM ORM 查询与 `Raw`，比只注册 Query callback 更完整。断言：

| Scenario | PostgreSQL budget |
| --- | --- |
| ready rule route + ready COS snapshot 的 v2 object-url | 0 |
| 单个冷 rule route | 跨实例合计 1 次 route query |
| 单个冷 COS snapshot | 跨实例合计 1 次 config+versions query |
| Redis I/O error | 0，直接依赖错误 |

把 `service_test.go` 当前 `openRuleDatabase` 中 180-187 行的旧内联 DDL 移到同 package 的测试 helper
`createStorageSchemaV2(t, db, ctx)`，完整创建 config/version、rule/code、generation/outbox 及新约束；
`insertConfig` 同时插入逻辑 config、version 1 和 generation/outbox。不得在部分测试继续保留旧 schema，以免产生两套假事实。

- [ ] **Step 2: 加入 mutation/outbox 崩溃恢复探针**

模拟：

1. 事务提交后进程未调用 `Lease.Commit`；
2. Redis 在同步发布时失败；
3. relay 读取 pending outbox 后恢复；
4. relay 遇到 active invalidating 时 reschedule；
5. lease 过期后 relay 以 PostgreSQL authoritative generation repair；
6. 较旧 outbox 不降低较新 ready generation。

断言无请求能把旧 Bucket snapshot 发布到新 generation。

- [ ] **Step 3: 加入对象历史矩阵**

```text
上传 public v2 at config version 1
修改 Bucket -> config physical version 2
上传 private v2 at version 2
删除上传 code、停用并软删 rule
轮换 Secret
读取第一个 key -> version 1 public URL
读取第二个 key -> version 2 signed GET
跨平台读取任一个 -> denied
旧格式 avatar key -> invalid request，且 PostgreSQL 查询数为 0
```

新协议历史读取不得依赖当前 enabled rule；旧格式 key 不进入 route cache 或 PostgreSQL 回源。

- [ ] **Step 4: 交给维护者运行后端完整定向组**

```powershell
cd D:\admin\server
go test ./internal/shared/cacheGeneration ./internal/shared/cacheFill ./internal/module/storage/cosConfig ./internal/module/storage/uploadRule ./internal/storage/cos ./cmd/api ./cmd/worker -count=1
```

Expected: PASS。

---

### Task 14: 固定共享 MutationResult，并修正 System Setting 写竞争

**Files:**
- Modify: `server/internal/shared/cacheGeneration/protocol.go`
- Modify: `server/internal/shared/cacheGeneration/protocol_test.go`
- Modify: `server/internal/module/system/setting/protocol.go`
- Modify: `server/internal/module/system/setting/service.go`
- Modify: `server/internal/module/system/setting/service_test.go`

**Interfaces:**
- Consumes: 现有 `cachegeneration.Event/Repository/Store/Lease`。
- Produces:

```go
type MutationResult struct {
    Changed    bool
    Generation int64
    OutboxID   int64
}
```

该类型只表达机械提交结果，不执行回调、不持有业务 Repository，也不是 Manager。`setting.MutationResult` 删除并改用
`cachegeneration.MutationResult`；Task 5 的 COS Repository 同样使用此类型。

- [ ] **Step 1: 写 System Setting 并发重试失败测试**

给两个 Service/Redis client 构造同 scope 并发 mutation，覆盖：

1. 第二个请求遇到 `ErrUpdating` 后等待并重新读取 generation；
2. 事务内 `AdvanceTx` 遇到 `ErrGenerationChanged` 时业务写整体回滚，再从完整流程重试；
3. 两个请求最终都成功，数据库结果是最后提交者值，generation 连续推进两次、每代恰好一个 outbox；
4. no-op 不推进 generation；
5. Redis I/O error 不进入 PostgreSQL mutation；
6. 500ms 预算耗尽返回 dependency unavailable，不返回 conflict/409。

为 Service 增加未导出的 `now func() time.Time` 与 `wait func(context.Context,time.Duration) error` 测试 hook；默认分别为
`time.Now` 和 context-aware timer。测试 wait 立即推进 20ms 假时间，不真实 sleep，并断言调用次数不超过 25 次。

- [ ] **Step 2: 实现模块内有界 mutation 循环**

`mutate` 每次 attempt 都从 `generations.Current` 开始并重新 Acquire。只对 `ErrUpdating`、`ErrGenerationChanged` 重试；Redis 网络错误、
非法 state、业务校验、唯一冲突、not found 不重试。已有 lease 的失败 attempt 必须先按权威 generation 执行 commit/rollback 判定，再决定
是否进入下一 attempt；不能遗留 active invalidating。

删除 `mapGenerationError` 把内部竞争映射 409 的分支。业务唯一冲突仍按原语义返回 409，不改变 System Setting HTTP DTO。

- [ ] **Step 3: 交给维护者运行定向测试**

```powershell
cd D:\admin\server
go test ./internal/shared/cacheGeneration ./internal/module/system/setting -run 'MutationResult|Mutation|Concurrent|Budget' -count=1
```

Expected: PASS；已有 system setting generation/outbox 测试仍通过。

---

### Task 15: 将 Dictionary 迁入 `system.dictionary/global`

**Files:**
- Modify: `server/internal/module/system/dictionary/cache.go`
- Modify: `server/internal/module/system/dictionary/cache_test.go`
- Modify: `server/internal/module/system/dictionary/repository.go`
- Modify: `server/internal/module/system/dictionary/repository_test.go`
- Modify: `server/internal/module/system/dictionary/service.go`
- Modify: `server/internal/module/system/dictionary/service_test.go`
- Modify later in Task 23: `server/cmd/api/main.go`

**Interfaces:**
- Scope: `cachegeneration.Scope{Namespace: "system.dictionary", ScopeKey: "global"}`。
- Snapshot variant: `options:<sha256(language + sorted unique codes)>`，variant 只放固定前缀和 hex，不放原始 code。
- Produces:

```go
func (r *Repository) SetGenerations(repo *cachegeneration.Repository, scope cachegeneration.Scope)
func (s *Service) SetGenerations(repo *cachegeneration.Repository, store *cachegeneration.Store)
func (c *optionsCache) SetStateStore(store *cachegeneration.Store)
```

- [ ] **Step 1: 改写 cache codec 与读协议测试**

删除 Redis-only string generation/mutation 语义。`optionsSnapshot.Generation` 改为 `int64`；snapshot key 必须通过
`cachegeneration.SnapshotKey`，fill 准入必须使用 `cachegeneration.StateKey/FillKey`。覆盖 strict JSON、requested code 集合规范化、language、
generation、重复 option value、损坏 snapshot、missing/corrupt state repair 和 publish 前 generation 二次确认。

正常命中断言零 PostgreSQL；同一冷 variant 两个实例合计一次 options query；invalidating 500ms 后失败闭合且零 PostgreSQL；Redis error
直接依赖错误。

- [ ] **Step 2: 把全部写操作收进 Repository 事务**

Repository 增加内部 `mutate(ctx, baseGeneration, now, apply)`：同一事务锁受影响 dictionary/item 行，完成业务校验与写入；只有真实变化
才调用 `AdvanceTx`。Create、Update、UpdateStatus、Delete、CreateItem、UpdateItem、UpdateItemStatus、DeleteItem 都返回
`cachegeneration.MutationResult`。

更新/状态先比较规范化后的当前值；相同值为 no-op。删除的 builtin/子项检查、item 的 parent 检查与唯一冲突都在同一事务完成，消除
Service 预读与实际写之间的 TOCTOU。保留明确 not-found/conflict 语义。

- [ ] **Step 3: 实现 Service mutation lease 与有界重试**

复用 Task 14 的顺序和 500ms/20ms预算，但代码留在 dictionary Service；不得抽通用 callback runner。PostgreSQL 提交后 Redis publish 或
mark-published 失败返回业务成功并记录 scope/generation/outbox/errorClass，不记录 options 内容。

- [ ] **Step 4: 删除旧协议实现但保留迁移清理清单**

运行时代码不再引用：

```text
system:dictionary:generation:v1
system:dictionary:mutation:v1
system:dictionary:options:v1:*
```

旧 key 只由 Task 22 runner 定向清理。不得在正常 mutation 中 pattern delete。

- [ ] **Step 5: 交给维护者运行定向测试**

```powershell
cd D:\admin\server
go test ./internal/module/system/dictionary -count=1
```

Expected: PASS；管理 CRUD 与 options HTTP 契约不增加 generation 字段。

---

### Task 16: 合并 Mail runtime/readiness 到 `message.mail/global`

**Files:**
- Modify: `server/internal/module/message/mail/runtime.go`
- Modify: `server/internal/module/message/mail/runtime_test.go`
- Modify: `server/internal/module/message/mail/readiness.go`
- Modify: `server/internal/module/message/mail/readiness_test.go`
- Delete after callers migrate: `server/internal/module/message/mail/readiness_coordinator.go`
- Modify: `server/internal/module/message/mail/stores.go`
- Modify: `server/internal/module/message/mail/service.go`
- Modify: `server/internal/module/message/mail/service_test.go`
- Modify: `server/internal/module/message/mail/config/{protocol.go,repository.go,service.go,service_test.go}`
- Modify: `server/internal/module/message/mail/template/{protocol.go,repository.go,service.go,service_test.go}`
- Modify: `server/internal/module/message/mail/recipientRule/{protocol.go,repository.go,service.go,service_test.go}`

**Interfaces:**
- Scope: `cachegeneration.Scope{Namespace: "message.mail", ScopeKey: "global"}`。
- Variants: `runtime:<scene>`、`readiness:<scene>`、Task 18 的 `ratelimit:<platformId>`。
- All submodule `RuntimeCoordinator` interfaces become:

```go
type RuntimeCoordinator interface {
    Mutate(context.Context, func(context.Context, int64) (cachegeneration.MutationResult, error)) error
}
```

- [ ] **Step 1: 先写单代际读缓存测试**

把 runtime 和 readiness 测试改为同一 generation state、不同不可变 variants。覆盖四个固定 scene，断言 runtime/readiness 热命中零 PG、
单 variant 冷回源一个 leader、Redis error fail closed、corrupt snapshot 重建、fill 期间 generation 前进则旧结果不发布。Secret 仍只以
ciphertext 进入 runtime snapshot；readiness snapshot 不含凭据。

- [ ] **Step 2: 将 readiness.go 收敛为 variant codec/loader**

保留 `VerifyCodeReadiness`、严格 codec 与业务计算；删除独立 ready/invalidating payload、mutation token、load-lock key 和
`ReadinessCoordinator`。`runtimeStore.LoadReadiness(ctx, scene)` 与 `Load(ctx, scene)` 都读取 `message.mail/global` state 并使用
`shared/cacheFill`。Config/template 不再嵌套 readiness mutation 和 runtime mutation。

- [ ] **Step 3: 让三个业务 Repository 原子推进 generation**

config、template、recipientRule Repository 各自注入同一个 generation Repository/scope。每个真实写方法在自己的 PostgreSQL 事务中
锁业务行、检测 no-op、写业务事实、`AdvanceTx(base)` 并返回 `cachegeneration.MutationResult`。Create 永远 changed；update/status/delete
按规范化事实判断。原有唯一冲突、固定 scene、builtin 与加密规则不变。

- [ ] **Step 4: 实现 Mail mutation 编排**

`runtimeStore.Mutate` 使用 Task 14 的 500ms/20ms有界流程。一次管理写只取得一条 Mail lease，callback 执行一次 Repository 事务，
然后发布一次 generation/outbox；不再出现先 readiness invalidating、再 runtime invalidating 的部分失败窗口。

提交后同步发布失败返回业务成功并由 relay 恢复；回滚恢复 base ready；提交不确定按 PostgreSQL generation 判定。日志不得包含
Secret、模板正文、收件规则明文或目标邮箱。

- [ ] **Step 5: 交给维护者运行 Mail runtime 组**

```powershell
cd D:\admin\server
go test ./internal/module/message/mail ./internal/module/message/mail/config ./internal/module/message/mail/template ./internal/module/message/mail/recipientRule -count=1
```

Expected: PASS；不存在旧 `mail:runtime:*`/`mail:verify-code-readiness:*` 的运行时读写引用。

---

### Task 17: 合并 SMS runtime/readiness 到 `message.sms/global`

**Files:**
- Modify: `server/internal/module/message/sms/runtime.go`
- Modify: `server/internal/module/message/sms/runtime_test.go`
- Modify: `server/internal/module/message/sms/runtime_redis_test.go`
- Modify: `server/internal/module/message/sms/readiness.go`
- Modify: `server/internal/module/message/sms/readiness_test.go`
- Modify: `server/internal/module/message/sms/stores.go`
- Modify: `server/internal/module/message/sms/service.go`
- Modify: `server/internal/module/message/sms/service_test.go`
- Modify: `server/internal/module/message/sms/config/{protocol.go,repository.go,service.go,service_test.go}`
- Modify: `server/internal/module/message/sms/template/{protocol.go,repository.go,service.go,service_test.go}`
- Modify: `server/internal/module/message/sms/recipientRule/{protocol.go,repository.go,service.go,service_test.go}`

**Interfaces:**
- Scope: `cachegeneration.Scope{Namespace: "message.sms", ScopeKey: "global"}`。
- Variants: `runtime`、`readiness:<scene>`、Task 19 的 `ratelimit:<platformId>`。
- `RuntimeCoordinator.Mutate` 使用与 Task 16 相同的 shared `cachegeneration.MutationResult` callback。

- [ ] **Step 1: 反转 Redis-only generation 测试**

删除测试对 `sms:runtime:generation:v1` 自增和 `sms:runtime:mutation:v1` 的依赖，改为 PostgreSQL authoritative generation、统一 state、
不可变 snapshot variants。覆盖 runtime/readiness 的 strict codec、密文、固定模板 catalog、recipient rules、500ms invalidating、
single-leader fill 和 generation 前进丢弃旧结果。

- [ ] **Step 2: 原子化 config/template/recipientRule 写入**

三个 Repository 注入 SMS scope generation Repository。每个写方法在同一事务完成行锁、no-op、业务更新、`AdvanceTx/outbox`；Service
不再先读后通过 context-only callback 写库。保持短信 config singleton、模板固定 scene/provider ID、recipient rule HMAC/密文契约。

- [ ] **Step 3: 实现 SMS mutation 与发布恢复**

`RuntimeCache.Mutate` 改为统一 lease/outbox 流程和有界重试；`Load/LoadReadiness` 只从同一 ready generation 读取 variant。删除旧 Lua
generation/mutation 脚本。同步发布失败业务成功、relay 恢复；Redis 整体失败时发送与配置 mutation 都失败闭合。

- [ ] **Step 4: 交给维护者运行 SMS runtime 组**

```powershell
cd D:\admin\server
go test ./internal/module/message/sms ./internal/module/message/sms/config ./internal/module/message/sms/template ./internal/module/message/sms/recipientRule -count=1
```

Expected: PASS；不存在旧 `sms:runtime:*`/`sms:readiness:*` 的运行时读写引用。

---

### Task 18: 删除 Mail rate-limit revision，并接入 Mail global generation

**Files:**
- Modify: `server/internal/module/message/mail/rateLimitPolicy/model.go`
- Modify: `server/internal/module/message/mail/rateLimitPolicy/protocol.go`
- Modify: `server/internal/module/message/mail/rateLimitPolicy/handler.go`
- Modify: `server/internal/module/message/mail/rateLimitPolicy/handler_test.go`
- Modify: `server/internal/module/message/mail/rateLimitPolicy/repository.go`
- Modify: `server/internal/module/message/mail/rateLimitPolicy/repository_test.go`
- Modify: `server/internal/module/message/mail/rateLimitPolicy/store.go`
- Modify: `server/internal/module/message/mail/rateLimitPolicy/store_test.go`
- Modify: `server/internal/module/message/mail/rateLimitPolicy/service.go`
- Modify: `server/internal/module/message/mail/rateLimitPolicy/service_test.go`
- Modify: `server/internal/module/message/mail/protocol.go`
- Modify: `server/internal/module/message/mail/service_test.go`

**Interfaces:**
- Consumes: Task 16 `message.mail/global` RuntimeCoordinator 和 state store。
- Snapshot variant: `ratelimit:<platformId>`。
- `Catalog`、`PlatformResponse`、`UpdateResponse` 删除 `Version`；`Model` 删除 `Revision`。

- [ ] **Step 1: 先反转版本契约测试**

Repository fixture 不创建 revision；catalog 完整性只校验固定两行、平台、key/mode/dimension、数值与 timestamp。HTTP response 不含
version。strict JSON 测试拒绝残留 version 字段。

- [ ] **Step 2: 将 Store 收敛为 generation variant 读缓存**

Store 不再拥有 ready/invalidating snapshot state、load-lock 或 `Update`。`Load(platformID)` 读取 Mail global state，再按 variant 读取
generation snapshot；miss/corrupt 用 cacheFill 有界回源，发布前重读同 generation。Snapshot 携带 schemaVersion、generation、
platformId、两条 policies，不含 revision/version/mutationToken。

- [ ] **Step 3: 原子更新策略事实与 Mail generation**

Repository `Update` 锁同平台两条固定 policy，比较目标行；no-op 返回 Changed=false；真实变化更新值/timestamp 并在同事务
`AdvanceTx(message.mail/global)`。Service 通过 Task 16 RuntimeCoordinator 执行，last-write-wins，内部竞争有界重试。

`ProvisionDefaults/DeleteForPlatform` 暂保留纯 PostgreSQL tx 方法，Task 20 负责把平台生命周期的双 channel generation 一起推进。

- [ ] **Step 4: 交给维护者运行 Mail rate-limit 测试**

```powershell
cd D:\admin\server
go test ./internal/module/message/mail/rateLimitPolicy -count=1
```

Expected: PASS；运行时代码不再引用 `mail:rate-limit:policies:v3:*` 或 catalog version。

---

### Task 19: 删除 SMS rate-limit revision，并接入 SMS global generation

**Files:**
- Modify: `server/internal/module/message/sms/rateLimitPolicy/model.go`
- Modify: `server/internal/module/message/sms/rateLimitPolicy/protocol.go`
- Modify: `server/internal/module/message/sms/rateLimitPolicy/handler.go`
- Modify: `server/internal/module/message/sms/rateLimitPolicy/handler_test.go`
- Modify: `server/internal/module/message/sms/rateLimitPolicy/repository.go`
- Modify: `server/internal/module/message/sms/rateLimitPolicy/repository_test.go`
- Modify: `server/internal/module/message/sms/rateLimitPolicy/store.go`
- Modify: `server/internal/module/message/sms/rateLimitPolicy/store_test.go`
- Modify: `server/internal/module/message/sms/rateLimitPolicy/service.go`
- Modify: `server/internal/module/message/sms/rateLimitPolicy/service_test.go`

**Interfaces:**
- Consumes: Task 17 `message.sms/global` RuntimeCoordinator 和 state store。
- Snapshot variant: `ratelimit:<platformId>`。
- `Model`、`PolicyResponse` 与 snapshot policy 删除 `Revision`。

- [ ] **Step 1: 先反转数据库/DTO 测试**

fixture 删除 revision；Repository update 不自增任何业务版本；API parser shape 不含 revision。Catalog 完整性仍要求活动平台与两条固定
policy，不使用 max revision 推导版本。

- [ ] **Step 2: 改 Store 与 Repository**

Store 只实现 generation variant 的 `Load`；删除独立 mutation key、snapshot/load-lock Lua。Repository update 使用同平台行锁、no-op
判断和同事务 `AdvanceTx(message.sms/global)`，返回 `cachegeneration.MutationResult`。

- [ ] **Step 3: 改 Service 写编排**

Service 使用 SMS RuntimeCoordinator mutation；不暴露 generation，成功后从 PostgreSQL 返回最新 platform catalog。发布失败遵循业务
成功 + outbox relay，不再因 Redis 发布失败把已提交策略更新报成失败。

- [ ] **Step 4: 交给维护者运行 SMS rate-limit 测试**

```powershell
cd D:\admin\server
go test ./internal/module/message/sms/rateLimitPolicy -count=1
```

Expected: PASS；运行时代码不再引用 `sms:rate-limit:policies:v1:*` 或 revision。

---

### Task 20: 闭合认证平台创建/删除的双渠道 generation

**Files:**
- Modify: `server/internal/module/permission/authPlatform/repository.go`
- Modify: `server/internal/module/permission/authPlatform/repository_test.go`
- Modify: `server/internal/module/permission/authPlatform/service.go`
- Modify: `server/internal/module/permission/authPlatform/service_test.go`
- Modify later in Task 23: `server/cmd/api/main.go`

**Interfaces:**
- Consumes: `message.mail/global`、`message.sms/global`、两组 policy lifecycle PostgreSQL callbacks。
- Produces an explicit service dependency; do not create a Manager/registry:

```go
func (s *Service) SetRateLimitCacheGenerations(
    repository *cachegeneration.Repository,
    store *cachegeneration.Store,
    mailScope cachegeneration.Scope,
    smsScope cachegeneration.Scope,
)
```

- [ ] **Step 1: 写跨 scope 整体结果测试**

覆盖创建平台与软删平台：Mail/SMS policies、两个 global generation/outbox 与平台事实要么全部提交，要么全部回滚。注入第二个 lease
获取失败、第二个 `AdvanceTx` 失败、平台写失败和事务提交不确定；断言没有单边 policy 或单边 generation。

- [ ] **Step 2: 实现固定 lease/数据库锁顺序**

Create/Delete 开始时按 `message.mail` -> `message.sms` 字典序读取 generation 并取得 lease，两个 lease 均开始 renewal 后才进入现有
Repository 事务。若第二个 lease busy/changed，先用原 token rollback 第一个 lease，再按 20ms wait 重启整个双 scope attempt；总预算
仍为 500ms。Repository lifecycle callback 在写完两组 policy 后按同顺序 `AdvanceTx` 两个 scope并返回两个 Event。不得在 Repository
callback 中访问 Redis。

no-op 不适用 Create/Delete；创建/删除都推进两个渠道 generation。平台 ID 是数据库 sequence 新值，不建立动态 generation scope。

- [ ] **Step 3: 完成双 lease finalize**

事务回滚时按逆序 rollback；提交成功按正序 commit 并分别 mark outbox。任一同步 publish 失败不把已提交平台事务报成失败，两个
pending outbox 由 relay 各自恢复。若提交结果不确定，分别读取两个权威 generation，绝不把已推进 scope 恢复旧 ready。

- [ ] **Step 4: 交给维护者运行平台生命周期测试**

```powershell
cd D:\admin\server
go test ./internal/module/permission/authPlatform -run 'RateLimit|Create|Delete|Generation|Concurrent' -count=1
```

Expected: PASS；`menu_version/policy_version` 原有行为不变，Mail/SMS generation 不替代它们。

---

### Task 21: 清理 Mail/SMS 前端管理版本字段

**Files:**
- Modify: `web/src/api/message/mail.ts`
- Modify: `web/src/api/message/sms.ts`
- Modify: `web/src/views/message/mail/rateLimitPolicy/index.vue`
- Modify: `web/src/views/message/sms/index.vue`
- Modify: `web/tests/api/message/mail.test.ts`
- Modify: `web/tests/api/message/sms.test.ts`
- Modify: `web/tests/views/message/mail/rateLimitPolicy/index.test.ts`
- Modify: `web/tests/views/message/sms/index.test.ts`

**Interfaces:**
- Mail platform catalog/update result 删除 `version`。
- SMS policy 删除 `revision`。
- 更新请求继续只含业务坐标和值：Mail/SMS 都是 platform ID + fixed policy key + limit + windowSeconds。

- [ ] **Step 1: 先改 strict parser 测试**

删除“version/revision 必须正整数”断言；新 parser exact-key 集合不含版本字段，并明确拒绝后端残留 version/revision。保留 fixed key、
mode/dimension、范围、timestamp 与完整两条 policy 校验。

- [ ] **Step 2: 改 API 类型和页面状态**

页面不保存、显示或回写版本；保存后以 API 返回 business facts 或重新列表更新。不得新增 hidden generation 字段、409 特判或手工缓存
递增。成功仍使用现有页面 `ElMessage/ElNotification` 约定；请求错误由 request 层单次通知。

- [ ] **Step 3: 交给维护者运行前端定向测试**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/message/mail.test.ts tests/api/message/sms.test.ts tests/views/message/mail/rateLimitPolicy/index.test.ts tests/views/message/sms/index.test.ts --pool=threads --maxWorkers=1
```

Expected: PASS；所有 rate-limit DTO 无 version/revision。

---

### Task 22: 编写配置缓存完成 migration、固定 Redis cleanup 与 runner

**Files:**
- Create: `docs/database/2026-09-17-config-cache-generation-completion.sql`
- Create: `docs/database/2026-09-17-config-cache-generation-completion.ps1`
- Create: `server/cmd/config-cache-generation-completion-migration/main.go`
- Create: `server/cmd/config-cache-generation-completion-migration/main_test.go`
- Create: `server/internal/database/config_cache_generation_completion_migration_test.go`

**Interfaces:**
- Consumes: 已真实存在的 `system_config_cache_generation/outbox` schema。
- Produces: 三个固定 scope seed、删除两列 revision、仅固定 pattern cleanup。

- [ ] **Step 1: 先写隔离 PostgreSQL 失败测试**

fixture 创建当前 Dictionary/Mail/SMS 事实、两个活动平台、完整两条 Mail/SMS policies、既有 `system.setting/global` 和任意历史 outbox。
执行 SQL 两次并断言：

```text
system.dictionary/global generation=1 + outbox generation=1
message.mail/global generation=1 + outbox generation=1
message.sms/global generation=1 + outbox generation=1
message_mail_rate_limit_policy.revision 不存在
message_sms_rate_limit_policy.revision 不存在
两个 revision 索引不存在
policy 行值、created_at、updated_at 完全不变
第二次执行 generation/outbox 不新增、不推进
menu_version/policy_version/session/access facts 完全不变
```

分别构造 Mail/SMS 缺 policy、重复/非法 fixed key、非法数值、缺少系统 generation 表；断言 migration 整体回滚，三 scope 和列删除均未
部分发生。

- [ ] **Step 2: 实现幂等 forward SQL**

一个事务内锁 generation/outbox 与两张 rate policy 表，预检所有活动平台恰好拥有各渠道两条合法固定策略。幂等插入三 scope 的
generation=1 和同代 outbox，再删除 revision indexes/constraints/columns。SQL 不修改业务时间戳、菜单、角色、限流计数或任何 Redis。

- [ ] **Step 3: 实现固定 Redis cleanup command**

forward 固定 patterns 至少包含：

```text
system:dictionary:generation:v1
system:dictionary:mutation:v1
system:dictionary:options:v1:*
mail:runtime:v2:*
mail:runtime:generation:v2
mail:runtime:mutation:v2
mail:runtime:load-lock:v2
mail:verify-code-readiness:v3:*
mail:verify-code-readiness:load-lock:v3:*
mail:rate-limit:policies:v3:*
mail:rate-limit:policies:load-lock:v3:*
sms:runtime:generation:v1*
sms:runtime:mutation:v1*
sms:runtime:v1:*
sms:readiness:v1:*
sms:rate-limit:policies:v1:*
sms:rate-limit:policies:load-lock:v1:*
sms:rate-limit:policies:mutation:v1:*
```

rollback 固定删除新 `config-cache:state:v1:{system.dictionary,message.mail,message.sms}:global` 与各自 snapshot patterns；短 TTL 的
`cachefill:*` 不做全局 pattern 清理，等待最多 6 秒自然到期。命令只接受 `-mode forward|rollback`，SCAN 小批量删除并打印 remaining；
不接受调用方 pattern，禁止 `KEYS/FLUSHDB/FLUSHALL`。测试证明 Session、权限、验证码、限流计数、COS 与 system.setting key 均保留。

- [ ] **Step 4: 实现配置缓存第二阶段唯一 PowerShell runner**

完全复用 COS runner 的安全形状：必须 `-OldAPIStopped`；自动 custom-format public schema 备份 + SHA256 + `pg_restore --list`；记录迁移前
三 scope、outbox、revision 列、policy rows、menu/policy version；`psql -X -v ON_ERROR_STOP=1`；forward cleanup 两次；只读验证；支持
`-CleanupOnly` 和 `-RollbackCleanupOnly`。SQL 已提交但 cleanup 失败必须非零退出并明确可安全重试。

- [ ] **Step 5: 交给维护者运行 migration 测试，不执行真实 runner**

```powershell
cd D:\admin\server
go test ./internal/database -run ConfigCacheGenerationCompletionMigration -count=1
go test ./cmd/config-cache-generation-completion-migration -count=1
```

Expected: PASS；不运行 `.ps1`。

---

### Task 23: 显式装配全部 scopes，并完成跨模块故障/查询预算验收

**Files:**
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`
- Modify only if current relay test lacks dynamic-scope coverage: `server/internal/shared/cacheGeneration/relay_test.go`
- Modify: 各模块 Task 14-20 的 service/cache integration tests

**Interfaces:**
- Consumes: 三固定 global scopes、动态 COS scopes、一个通用 outbox Relay。
- Produces: 显式 constructor/setter wiring；不新增 Worker 注册表。

- [ ] **Step 1: 在 API main 创建一次共享依赖**

```go
generationRepository := cachegeneration.NewRepository(postgres.GORM)
generationStore := cachegeneration.NewStore(redisClient)
```

把同一实例显式注入 Setting、Dictionary、Mail、SMS、Mail/SMS rate-limit、AuthPlatform 生命周期和 COS。每个 Cache 仍由所属模块创建。
删除 Mail readiness coordinator 和旧 Redis-only store 装配。`cmd/worker` 现有 Relay 按 namespace 通用处理，无需新增 channel/COS worker。

- [ ] **Step 2: 写启动依赖失败测试**

缺 generation repository/store、业务 Cache、route cache 或 signer 时启动返回明确错误；不得延迟到首请求 panic。断言 namespace/scope 精确为：

```text
system.setting/global
system.dictionary/global
message.mail/global
message.sms/global
storage.cosconfig/<positive config id>
```

- [ ] **Step 3: 加入统一查询预算 probe**

复用 Task 13 的 GORM logger query counter，分别 warm snapshot 后 reset：

| Scenario | PostgreSQL SELECT budget |
| --- | --- |
| setting/dictionary/mail/sms/rate-limit/COS ready 命中 | 0 |
| 单冷 variant，两个实例并发 | 对应 loader 合计 1 组查询 |
| active invalidating 500ms | 0 |
| Redis I/O error | 0 |

对 Mail/SMS 单次发送链路同时验证 runtime、readiness、rate policy 都 ready 时零 PostgreSQL。测试不得把 append-only log 写入算作配置读查询。

- [ ] **Step 4: 加入统一故障矩阵**

每个固定 scope 至少覆盖：提交后 `Lease.Commit` 失败、mark-published 失败、进程在提交后崩溃、relay 遇到 active invalidating 后重排、
lease 到期 repair、Redis 丢 state、corrupt snapshot、旧 outbox 不降低新 ready、旧 fill 不向新 generation 发布。敏感 payload 不进入日志。

- [ ] **Step 5: 交给维护者运行后端完整定向组**

```powershell
cd D:\admin\server
go test ./internal/shared/cacheGeneration ./internal/shared/cacheFill ./internal/module/system/setting ./internal/module/system/dictionary ./internal/module/message/mail/... ./internal/module/message/sms/... ./internal/module/storage/cosConfig ./internal/module/storage/uploadRule ./internal/module/permission/authPlatform ./internal/storage/cos ./cmd/api ./cmd/worker -count=1
```

Expected: PASS；所有配置 scope 通过相同故障语义，但不存在通用业务 CacheManager。

---

### Task 24: 同步稳定文档、格式、静态检查与维护者全量验证

**Files:**
- Modify: `docs/agent/design.md`
- Modify: `docs/agent/architecture.md`
- Modify only: 本计划涉及的 Go/Vue/TS 文件。

**Interfaces:**
- Consumes: 所有实现。
- Produces: 与实现一致的稳定产品/架构事实、格式化 diff 与维护者验证结果，不产生 commit。

- [ ] **Step 1: 同步稳定产品与架构事实**

修改 `docs/agent/design.md` 和 `docs/agent/architecture.md`，精确记录以下完成态，不把执行进度或 migration 结果写进稳定文档：

```text
COS 逻辑配置全局共享且可维护多条，不持有 platform_id；上传规则按认证平台隔离，并通过 cos_config_id 选择全局配置。
COS 物理 version 只定位历史对象所在 Bucket；配置 generation 只保证缓存一致性；普通管理 CRUD last-write-wins。
配置缓存固定 scopes：system.setting/global、system.dictionary/global、message.mail/global、message.sms/global、storage.cosconfig/<configId>。
permission 的 menu/policy/access version、Session/authority generation、验证码、限流计数与队列数据不迁入配置 generation。
```

同时删除现有“COS 上传配置可以按认证平台分别维护”的过期表述。两份文档不得写 migration 已执行、测试已通过或 Redis 已清理；
这些事实只在 Task 25/26 实际完成后进入 `STATUS.md`。

- [ ] **Step 2: 执行 Agent 只运行格式化与 diff whitespace 检查**

```powershell
cd D:\admin\server
go fmt ./internal/shared/cacheGeneration ./internal/module/system/setting ./internal/module/system/dictionary ./internal/module/message/mail/... ./internal/module/message/sms/... ./internal/module/permission/authPlatform ./internal/module/storage/cosConfig ./internal/module/storage/uploadRule ./internal/storage/cos ./cmd/storage-object-versioning-migration ./cmd/config-cache-generation-completion-migration ./cmd/api

cd D:\admin\web
pnpm prettier --write src/api/storage src/api/message src/views/storage/object src/views/message/mail/rateLimitPolicy src/views/message/sms src/components/UpMedia src/layout/components/AppAside tests/api/storage tests/api/message tests/views/storage/object tests/views/message/mail/rateLimitPolicy tests/views/message/sms tests/components/UpMedia tests/layout/components/AppAside.test.ts

cd D:\admin
git diff --check
git status --short
```

Expected: `git diff --check` exit 0；不得格式化无关目录，不运行测试。

- [ ] **Step 3: 维护者运行后端完整验证**

```powershell
cd D:\admin\server
go vet ./...
go test ./... -count=1
go build ./...
```

Expected: 全部 exit 0。

- [ ] **Step 4: 维护者运行前端完整验证**

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm typecheck
pnpm lint
pnpm check:architecture
pnpm build
```

Expected: 全部 exit 0；build 只允许已记录的非阻断性 chunk warning。

Checkpoint: 若有失败，按首个真实根因修复并由维护者重跑失败命令；不得仅提高 timeout 或删除断言。

---

### Task 25: 维护者执行 COS 真实迁移并建立阶段检查点

**Files:**
- Modify after successful real migration: `docs/database/current.sql`
- Modify after successful real migration: `docs/agent/STATUS.md`
- Do not modify: RBAC migration/menu seed files。

**Interfaces:**
- Consumes: 已通过的代码与 migration。
- Produces: 真实 PostgreSQL 新 schema、定向 Redis cleanup、可审计备份与状态记录。

- [ ] **Step 1: 部署前停止旧写流量**

维护者停止旧 API/Worker，确认没有旧 schema 写入者。执行 Agent 不代为重启或停止服务。

- [ ] **Step 2: 维护者运行第一阶段 COS runner**

```powershell
cd D:\admin
pwsh -File .\docs\database\2026-09-17-storage-object-versioning.ps1 -OldAPIStopped
```

Expected:

- 自动备份成功并输出路径/SHA256；
- SQL、两次 fixed-pattern Redis cleanup、只读验证均成功；
- `menu_version` 全部不变；
- 每个 config 有 version、generation 和 outbox；
- 用户头像与品牌默认头像旧引用均为空；若品牌默认头像原值非空，`system.setting/global` generation/outbox 恰好推进一次；
- `storage_object_legacy_route` 不存在；
- exit 0。

若 SQL 已提交而 cleanup 失败，只运行：

```powershell
pwsh -File .\docs\database\2026-09-17-storage-object-versioning.ps1 -OldAPIStopped -CleanupOnly
```

- [ ] **Step 3: 立即重复执行验证幂等**

维护者再次运行完整 runner。Expected: version、storage/system generation、outbox、头像空值与 menu version 均不变化，全部固定
Redis pattern remaining=0。

- [ ] **Step 4: 从真实 PostgreSQL 刷新 `current.sql`**

沿用项目现有 schema-only 导出参数：`--schema=public --schema-only --no-owner --no-privileges`。先导出临时文件，确认
`pg_dump` exit 0 且包含新表/约束，再原子替换 `docs/database/current.sql`；不得手工编辑快照冒充真实导出。

- [ ] **Step 5: 更新 STATUS 实际结果**

记录：

- migration 执行时间、exit code；
- 备份路径、大小、SHA256、`pg_restore --list` 结果；
- config/version/generation/outbox 行数、legacy route 表不存在；
- 清空的用户头像数、品牌默认头像是否被清空，以及 `system.setting/global` generation/outbox 前后值；
- 六个 forward Redis pattern 两轮 remaining；
- 七个 rollback cleanup 固定 pattern 的测试结果；
- `menu_version` 前后值；
- 实际测试命令和结果；
- API/Worker 是否已重启、浏览器/COS 实桶是否已验收；
- 仍存在的风险，尤其是旧 COS 对象字节仍留在 Bucket、但已从应用层不可达且不在本 migration 删除。

- [ ] **Step 6: 明确回滚边界**

本 migration 对旧二进制不向后兼容。需要回滚时必须：

```text
停止新 API/Worker
-> 恢复 runner 创建的 PostgreSQL public schema 备份
-> 运行 -RollbackCleanupOnly，定向清理 storage.cosconfig、system.setting/global 与 storage:object-route:v2:* 缓存
-> 部署旧二进制
-> 验证旧 schema 与 object-url
```

具体命令：

```powershell
pwsh -File .\docs\database\2026-09-17-storage-object-versioning.ps1 -OldAPIStopped -RollbackCleanupOnly
```

PostgreSQL 备份恢复会同时恢复旧头像引用与 generation/outbox 事实；Redis rollback cleanup 删除新 storage namespace/v2 route，
并删除可能按“头像已清空”事实回填的 `system.setting/global` 缓存。旧二进制不读取新 storage key，但必须删除以防后续再次升级时
误用旧快照。forward `-CleanupOnly` 不能替代 rollback 模式。禁止只切回旧 API，也禁止对 Redis 全库清理。

---

### Task 26: 维护者执行配置缓存完成迁移并刷新最终状态

**Files:**
- Modify after successful real migration: `docs/database/current.sql`
- Modify after successful real migration: `docs/agent/STATUS.md`
- Do not modify: RBAC/menu/role seed、限流计数 key 或历史 migration。

**Interfaces:**
- Consumes: Task 25 已成功的 COS schema 与 Task 22 runner。
- Produces: Dictionary/Mail/SMS 完成态 scopes、无 rate-limit revision 的真实 schema、旧协议 Redis key 清理结果。

- [ ] **Step 1: 保持旧进程停止并运行第二个 runner**

Task 25 成功后不要启动旧 API/Worker。维护者执行：

```powershell
cd D:\admin
pwsh -File .\docs\database\2026-09-17-config-cache-generation-completion.ps1 -OldAPIStopped
```

Expected:

- 自动创建独立的第二份 PostgreSQL 备份并输出路径/SHA256；
- `system.dictionary/global`、`message.mail/global`、`message.sms/global` 各有合法 generation 与同代 outbox；
- Mail/SMS rate policy revision 列与索引不存在，固定 policy facts 未改变；
- 旧 Dictionary/Mail/SMS 配置缓存 patterns 两轮 remaining=0；
- Session、权限、验证码、限流计数、队列、COS 和 system.setting Redis key 未删除；
- `menu_version`、`policy_version` 与 permission access version 均不变化；
- exit 0。

SQL 已提交但 cleanup 失败时只重试：

```powershell
pwsh -File .\docs\database\2026-09-17-config-cache-generation-completion.ps1 -OldAPIStopped -CleanupOnly
```

- [ ] **Step 2: 立即重复完整 runner 验证幂等**

Expected: 三 scope generation、outbox 行数、policy facts 与所有权限版本不变化；全部固定 Redis patterns remaining=0。

- [ ] **Step 3: 从最终真实 PostgreSQL 刷新 current.sql**

使用 `pg_dump --schema=public --schema-only --no-owner --no-privileges` 输出临时文件；确认 exit 0，包含
`storage_cos_config_version.version`、`storage_cos_config.current_version`、三固定 scope 依赖表，且不含两张 rate policy revision 列，再原子替换
`docs/database/current.sql`。Task 25 若已临时导出，本步骤必须以第二次 migration 后真实库再次导出为最终快照。

- [ ] **Step 4: 更新 STATUS 完成态记录**

记录两个 runner 各自的执行时间、exit code、备份路径/大小/SHA256/归档条目、重复执行结果；列出所有配置 scopes 当前 generation、
outbox pending/published 摘要、Redis cleanup 每个 pattern 两轮 remaining、revision 列不存在、权限版本未变化、实际测试命令和结果、
API/Worker 重启及浏览器/COS/真实 Mail/SMS 验收状态。未执行项必须明确写未执行，不能写成通过。

- [ ] **Step 5: 明确第二阶段回滚边界**

配置完成 migration 需要回滚时：

```text
停止新 API/Worker
-> 恢复 Task 26 runner 创建的 public schema 备份
-> 运行 config-cache runner -RollbackCleanupOnly
-> 部署 Task 26 之前、Task 25 之后的二进制
```

若两个阶段都要回滚，再继续使用 Task 25 的备份与 COS `-RollbackCleanupOnly`。两个 runner 的备份和 cleanup 不可互换；禁止只恢复
数据库不清新缓存，也禁止全库 Redis 清理。

---

## Final Review Checklist

- [ ] `storage_cos_config` 不再重复保存 Bucket/Region/Endpoint/Domain 或 logical revision；`current_version` 只指向物理版本；
  PostgreSQL generation 只承担缓存协议，`storage_upload_rule` 不含 revision。
- [ ] `storage_upload_rule_code` 被保留且规范化，没有 `TEXT[]` 或跨规则 code 唯一约束。
- [ ] 新 key 自描述；旧格式 key 明确拒绝，没有 legacy route 表、兼容缓存或“当前启用规则/全局配置”猜测。
- [ ] private object-url 仅认证、同平台、短期签名；public 返回无过期普通 URL。
- [ ] 正常热读零 PostgreSQL，冷回源有界，Redis 故障失败闭合。
- [ ] config mutation 事务内推进 generation/outbox；发布失败由 relay 恢复且接口不谎报业务失败。
- [ ] System Setting、Dictionary、Mail、SMS 与 COS 的并发管理写均 last-write-wins；客户端不见 generation/revision/expected 字段。
- [ ] Dictionary 只使用 `system.dictionary/global`；Mail/SMS 分别只使用 `message.mail/global`、`message.sms/global`，readiness 和 rate-limit 都是同代 variants。
- [ ] Mail/SMS rate-limit 表、Go/JSON/TypeScript/UI 不含 revision/catalog version；已有发送限流计数和窗口未被清理。
- [ ] 认证平台创建/删除对 Mail/SMS policy facts 与两个 global generation/outbox 保持整体事务结果。
- [ ] 两个 migration runner 都包含备份、SQL、固定 Redis cleanup、验证、幂等复跑与独立回滚 cleanup。
- [ ] migration 清空用户头像与品牌默认头像旧引用，并正确推进/清理 `system.setting/global` 代际；不调用 COS 删除对象字节。
- [ ] 旧 Dictionary/Mail/SMS Redis-only generation/mutation/readiness 协议已无运行时引用。
- [ ] `design.md` 与 `architecture.md` 已记录全局 COS 配置、平台上传规则、固定配置 scopes 和明确排除项，不再保留按平台维护 COS 配置的过期描述。
- [ ] 无 RBAC/menu 变化，无 `menu_version` 递增。
- [ ] 无 commit；`git status --short` 中原有改动仍在。
