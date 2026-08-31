# Admin 业务域命名与动态菜单实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有 `sys_* + system:* + views/system/*` 管理功能一次性迁移为 account/auth/rbac/audit 真实业务域，并交付以 PostgreSQL 为唯一事实来源的多根菜单、动态页面、角色授权、菜单搜索和本地 Lucide 图标。

**Architecture:** 启动阶段先由 `database.PrepareDomainNames` 在 PostgreSQL 事务内重命名九张表及其对象，再由各模块按新表名完成 PrepareSchema、AutoMigrate 和 EnsureSchema。菜单协议迁移复用原 row ID 更新 code/parent/path/componentPath，所有在线请求继续遵守 `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`；前端使用静态 `/access/menus` 冷启动绑定和数据库多根菜单树。

**Tech Stack:** Go 1.26、Gin、GORM、PostgreSQL、Redis、Asynq、Vue 3、TypeScript 6、Pinia、Vue Router、vue-i18n、Element Plus、Lucide Vue、Vitest。

---

## 执行约束

- 规范来源：`docs/superpowers/specs/2026-08-24-menu-protocol-dynamic-page-design.md`。
- 本计划是一次性破坏性协议切换；不实现旧表、旧权限、旧页面或旧任务类型兼容。
- 保留已有账号、角色、菜单 row ID、角色授权和操作日志数据。
- 使用 TDD：失败测试 -> 确认目标原因失败 -> 最小实现 -> 定向测试通过 -> 再整理代码。
- PostgreSQL 迁移和 schema 测试必须使用真实 PostgreSQL 的独立 schema，不允许 SQLite 或内存替代。
- 不使用 Docker，不引入 Adapter、Manager、Factory、BaseService、BaseRepository 或 DI 容器。
- TypeScript 禁止显式 `any`、`any[]`、`as any`、`Record<string, any>` 和 `@ts-ignore`。
- 计划中的提交标题仅是 owner-controlled checkpoint。除非用户明确授权，不执行 `git add`、`git commit`、`rebase`、`push`、`fetch` 或 `pull`。
- 不停止来源不明的用户进程；执行维护窗口前先确认 API、Worker 和前端服务归属。
- 不使用 `FLUSHDB`。Redis 删除只允许命中本文列出的项目 key/type。

## 文件职责

### 数据库与启动

- Create `server/internal/database/domain_names.go`：九张表和相关 PostgreSQL 对象的一次性显式改名。
- Create `server/internal/database/domain_names_test.go`：旧 schema、重复执行和冲突状态的真实 PostgreSQL 测试。
- Create `server/internal/database/testschema/schema.go`：仅供测试使用的独立 PostgreSQL schema 连接辅助函数。
- Modify `server/cmd/api/main.go`：固定 domain rename、PrepareSchema、AutoMigrate、EnsureSchema 和 foundation 顺序。

### 后端业务模块

- `server/internal/module/*/model.go`：只定义新表映射和显式字段。
- `server/internal/module/*/schema.go`：只维护新约束、索引和模块自身结构迁移。
- `server/internal/module/*/repository.go`：全部原始 SQL 使用新表名。
- `server/internal/module/{menu,role,user,auth,authplatform,operationlog}/protocol.go`：各模块拥有自己的权限常量。
- Create `server/internal/module/menu/foundation.go`：受保护基础节点、首次空库种子 DTO 和保护判断。
- `server/internal/module/menu/schema.go`：菜单 name/i18n/icon 和 old code -> new code 的一次性数据迁移。
- Create `server/cmd/api/menu_foundation.go`：composition root 使用各模块常量组装当前内置菜单。
- `server/internal/module/access/*`、`accessstate/*`：从新 RBAC 表生成快照并拒绝旧缓存协议。
- `server/internal/module/operationlog/task.go`：只生产和消费 `audit:operation-log:v2`。

### 前端协议与页面

- `web/src/api/*.contract.ts`：严格解析新字段和多根树，不接受旧权限或路径。
- `web/src/router/index.ts`、`access-routes.ts`：静态 `/access/menus` 精确绑定，其余页面动态注册。
- `web/src/layout/*`：只从 access tree 渲染侧栏、面包屑和 RouteTabs。
- Move `web/src/views/system/{users,sessions}` -> `web/src/views/account/*`。
- Move `web/src/views/system/{menus,roles,auth-platforms}` -> `../../../web/src/views/permission/*`。
- Keep `web/src/views/system/operation-logs`，只更新权限码和导航键。
- Move 对应 `web/tests/views/*` 到相同业务域目录。
- Rename `web/src/styles/system-pages.scss` -> `management-pages.scss`，class 改为 `management-page*`。
- Create `web/src/icons/menu-icons.ts`：本地 Lucide 菜单图标唯一目录和解析器。
- Create `../../../web/src/views/permission/menus/filter-menu-tree.ts`：菜单搜索和祖先/后代保留算法。

---

### Task 1: 建立独立 PostgreSQL 测试环境和领域表名迁移

**Files:**
- Create: `server/internal/database/testschema/schema.go`
- Create: `server/internal/database/domain_names.go`
- Create: `server/internal/database/domain_names_test.go`
- Test: `server/internal/database/domain_names_test.go`

- [ ] **Step 1: 创建独立 schema 测试辅助函数**

`testschema.Open` 接收 DSN 和安全前缀，创建随机 schema，并为 pgx/GORM 设置独立
`search_path`：

```go
package testschema

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"admin/server/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var prefixPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func Open(t *testing.T, dsn, prefix string) (*gorm.DB, context.Context) {
	t.Helper()
	if !prefixPattern.MatchString(prefix) {
		t.Fatalf("invalid PostgreSQL test schema prefix %q", prefix)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	root, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	if err := root.GORM.WithContext(ctx).Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatalf("create PostgreSQL test schema: %v", err)
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse PostgreSQL DSN: %v", err)
	}
	config.RuntimeParams["search_path"] = schema
	sqlDB := stdlib.OpenDB(*config)
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("ping isolated PostgreSQL schema: %v", err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open isolated GORM: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = root.GORM.WithContext(cleanupCtx).Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
		_ = root.Close()
	})
	return db, ctx
}
```

只允许经过正则校验的内部测试前缀进入标识符拼接；业务值继续使用 SQL 参数。

- [ ] **Step 2: 写九表改名失败测试**

在独立 schema 创建九张 legacy 表、九个历史序列、主键、现有 CHECK/FK 和全部独立索引，并插入
可追踪 ID。核心测试：

```go
func TestPrepareDomainNamesRenamesTablesAndOwnedObjects(t *testing.T) {
	db, ctx := openDomainNamesDatabase(t, "complete")
	createLegacyDomainSchema(t, db)

	if err := database.PrepareDomainNames(ctx, db); err != nil {
		t.Fatal(err)
	}

	for oldName, newName := range expectedTableRenames {
		assertRelationMissing(t, db, oldName)
		assertRelationExists(t, db, newName)
	}
	assertNoOwnedObjectPrefix(t, db, "sys_")
	assertRowPreserved(t, db, "user_account", 101)
	assertRowPreserved(t, db, "rbac_menu", 301)
	assertRowPreserved(t, db, "audit_operation_log", 901)
}
```

再增加：

- `TestPrepareDomainNamesIsIdempotentForCompleteNewSchema`：第二次只验证，不执行 DDL；
- `TestPrepareDomainNamesAllowsCompletelyEmptySchema`：空库返回 nil，交给 AutoMigrate；
- `TestPrepareDomainNamesRejectsOldAndNewTableTogether`：同一 pair 并存时失败；
- `TestPrepareDomainNamesRejectsMixedGeneration`：部分 old、部分 new 时失败；
- `TestPrepareDomainNamesRollsBackAllRenames`：制造目标对象冲突后，九张旧表全部仍存在。

- [ ] **Step 3: 运行测试并确认目标失败**

```powershell
cd D:\admin\server
go test ./internal/database -run TestPrepareDomainNames -count=1
```

预期：编译失败，提示 `database.PrepareDomainNames` 尚不存在。

- [ ] **Step 4: 实现显式表映射和状态机**

`domain_names.go` 固定映射：

```go
type domainRename struct {
	Old string
	New string
}

var domainTableRenames = []domainRename{
	{Old: "sys_user", New: "user_account"},
	{Old: "sys_user_session", New: "auth_session"},
	{Old: "sys_menu", New: "rbac_menu"},
	{Old: "sys_role", New: "rbac_role"},
	{Old: "sys_user_role", New: "rbac_user_role"},
	{Old: "sys_role_menu", New: "rbac_role_menu"},
	{Old: "sys_access_version", New: "rbac_access_version"},
	{Old: "sys_auth_platform", New: "auth_platform"},
	{Old: "sys_operation_log", New: "audit_operation_log"},
}

func PrepareDomainNames(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("prepare domain names requires a database")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := inspectDomainRenameState(tx, domainTableRenames)
		if err != nil {
			return err
		}
		switch state {
		case domainSchemaEmpty, domainSchemaCurrent:
			return verifyDomainObjectNames(tx)
		case domainSchemaLegacy:
			return renameLegacyDomainObjects(tx)
		default:
			return fmt.Errorf("domain schema mixes legacy and current names")
		}
	})
}
```

`inspectDomainRenameState` 必须逐 pair 查询 `to_regclass(current_schema() || '.' || ?)`；不根据
单张表猜整个 schema 状态。

- [ ] **Step 5: 实现约束、索引和序列显式映射**

九个序列必须全部显式映射；`rbac_access_version.user_id` 当前虽不再自增，现有开发库仍有
历史 sequence，迁移不能把它遗留为 `sys_*` 对象：

```go
var domainSequenceRenames = []domainRename{
	{Old: "sys_user_id_seq", New: "user_account_id_seq"},
	{Old: "sys_user_session_id_seq", New: "auth_session_id_seq"},
	{Old: "sys_menu_id_seq", New: "rbac_menu_id_seq"},
	{Old: "sys_role_id_seq", New: "rbac_role_id_seq"},
	{Old: "sys_user_role_id_seq", New: "rbac_user_role_id_seq"},
	{Old: "sys_role_menu_id_seq", New: "rbac_role_menu_id_seq"},
	{Old: "sys_access_version_user_id_seq", New: "rbac_access_version_user_id_seq"},
	{Old: "sys_auth_platform_id_seq", New: "auth_platform_id_seq"},
	{Old: "sys_operation_log_id_seq", New: "audit_operation_log_id_seq"},
}
```

约束使用完整、闭合的 old/new 映射。除下面列出的对象外，迁移不得根据 catalog 名称做前缀
替换。九个主键约束：

```text
sys_user_pkey           -> user_account_pkey
sys_user_session_pkey   -> auth_session_pkey
sys_menu_pkey           -> rbac_menu_pkey
sys_role_pkey           -> rbac_role_pkey
sys_user_role_pkey      -> rbac_user_role_pkey
sys_role_menu_pkey      -> rbac_role_menu_pkey
sys_access_version_pkey -> rbac_access_version_pkey
sys_auth_platform_pkey  -> auth_platform_pkey
sys_operation_log_pkey  -> audit_operation_log_pkey
```

业务 CHECK/FK 约束的闭合映射：

```text
ck_sys_user_is_enabled                    -> ck_user_account_is_enabled
ck_sys_user_session_version               -> ck_auth_session_version
fk_sys_user_session_user                  -> fk_auth_session_user
ck_sys_menu_type                          -> ck_rbac_menu_type
ck_sys_menu_shape                         -> ck_rbac_menu_shape
ck_sys_menu_render_shape                  -> ck_rbac_menu_render_shape
ck_sys_menu_sort_order                    -> ck_rbac_menu_sort_order
ck_sys_menu_is_enabled                    -> ck_rbac_menu_is_enabled
ck_sys_menu_is_hidden                     -> ck_rbac_menu_is_hidden
fk_sys_menu_parent                        -> fk_rbac_menu_parent
ck_sys_role_is_default                    -> ck_rbac_role_is_default
ck_sys_role_is_enabled                    -> ck_rbac_role_is_enabled
fk_sys_user_role_user                     -> fk_rbac_user_role_user
fk_sys_user_role_role                     -> fk_rbac_user_role_role
fk_sys_role_menu_role                     -> fk_rbac_role_menu_role
fk_sys_role_menu_menu                     -> fk_rbac_role_menu_menu
ck_sys_access_version_version             -> ck_rbac_access_version_version
fk_sys_access_version_user                -> fk_rbac_access_version_user
ck_sys_auth_platform_code                 -> ck_auth_platform_code
ck_sys_auth_platform_policy_version       -> ck_auth_platform_policy_version
ck_sys_auth_platform_access_ttl_seconds   -> ck_auth_platform_access_ttl_seconds
ck_sys_auth_platform_refresh_ttl_seconds  -> ck_auth_platform_refresh_ttl_seconds
ck_sys_auth_platform_session_cache_ttl_seconds -> ck_auth_platform_session_cache_ttl_seconds
ck_sys_auth_platform_access_cache_ttl_seconds  -> ck_auth_platform_access_cache_ttl_seconds
ck_sys_auth_platform_bind_device          -> ck_auth_platform_bind_device
ck_sys_auth_platform_bind_ip              -> ck_auth_platform_bind_ip
ck_sys_auth_platform_max_sessions         -> ck_auth_platform_max_sessions
ck_sys_auth_platform_allow_register       -> ck_auth_platform_allow_register
ck_sys_auth_platform_is_enabled           -> ck_auth_platform_is_enabled
ck_sys_auth_platform_is_builtin           -> ck_auth_platform_is_builtin
ck_sys_operation_log_is_success           -> ck_audit_operation_log_is_success
ck_sys_operation_log_latency_ms           -> ck_audit_operation_log_latency_ms
```

PostgreSQL 18 暴露的命名 NOT NULL 约束也属于迁移对象。以下花括号是闭合集合，不是通配符；
实现时展开成逐项 `domainConstraintRename`，old/new 名称都写成编译期常量：

```text
sys_user_{id,username,email,password_hash,is_enabled,created_at,updated_at}_not_null
  -> user_account_{同名字段}_not_null
sys_user_session_{id,user_id,platform,device_id,refresh_token_hash,version,client_ip,user_agent,refresh_expires_at,created_at,updated_at}_not_null
  -> auth_session_{同名字段}_not_null
sys_menu_{id,menu_type,code,i18n_key,sort_order,is_enabled,is_hidden,created_at,updated_at}_not_null
  -> rbac_menu_{同名字段}_not_null
sys_role_{id,code,name,is_default,is_enabled,created_at,updated_at}_not_null
  -> rbac_role_{同名字段}_not_null
sys_user_role_{id,user_id,role_id,created_at,updated_at}_not_null
  -> rbac_user_role_{同名字段}_not_null
sys_role_menu_{id,role_id,menu_id,created_at,updated_at}_not_null
  -> rbac_role_menu_{同名字段}_not_null
sys_access_version_{user_id,version,created_at,updated_at}_not_null
  -> rbac_access_version_{同名字段}_not_null
sys_auth_platform_{id,code,name,policy_version,access_ttl_seconds,refresh_ttl_seconds,session_cache_ttl_seconds,access_cache_ttl_seconds,bind_device,bind_ip,max_sessions,allow_register,is_enabled,is_builtin,created_at,updated_at}_not_null
  -> auth_platform_{同名字段}_not_null
sys_operation_log_{id,event_id,request_id,method,route,module,action,client_ip,user_agent,status_code,is_success,latency_ms,created_at,updated_at}_not_null
  -> audit_operation_log_{同名字段}_not_null
```

独立索引的闭合映射；主键 backing index 随 `RENAME CONSTRAINT` 同步改名，不在这里重复执行：

```text
ux_sys_user_username_active              -> ux_user_account_username_active
ux_sys_user_email_active                 -> ux_user_account_email_active
ux_sys_user_session_refresh_hash         -> ux_auth_session_refresh_hash
ix_sys_user_session_user_created         -> ix_auth_session_user_created
ix_sys_user_session_user_platform_active -> ix_auth_session_user_platform_active
ux_sys_user_session_current              -> ux_auth_session_current
ux_sys_menu_code_active                  -> ux_rbac_menu_code_active
ux_sys_menu_page_path_active             -> ux_rbac_menu_page_path_active
ix_sys_menu_parent_active                -> ix_rbac_menu_parent_active
ux_sys_role_code_active                  -> ux_rbac_role_code_active
ux_sys_role_name_active                  -> ux_rbac_role_name_active
ux_sys_role_default_active               -> ux_rbac_role_default_active
ux_sys_user_role_active                  -> ux_rbac_user_role_active
ux_sys_role_menu_active                  -> ux_rbac_role_menu_active
ux_sys_auth_platform_code_active         -> ux_auth_platform_code_active
ux_sys_operation_log_event_id            -> ux_audit_operation_log_event_id
ux_sys_operation_log_request_id          -> ux_audit_operation_log_request_id
ix_sys_operation_log_request_id          -> ix_audit_operation_log_request_id
ix_sys_operation_log_created_at          -> ix_audit_operation_log_created_at
ix_sys_operation_log_user_created        -> ix_audit_operation_log_user_created
ix_sys_operation_log_action_created      -> ix_audit_operation_log_action_created
```

实现分别用 `ALTER TABLE ... RENAME CONSTRAINT`、`ALTER INDEX ... RENAME TO`、
`ALTER SEQUENCE ... RENAME TO` 和 `ALTER TABLE ... RENAME TO`。标识符只来自编译期常量，
每次执行前查询精确 old/new 对象状态；不得对 catalog 结果做字符串前缀替换。迁移顺序固定为
约束 -> 独立索引 -> sequence -> table；可选的历史过渡对象
`ck_sys_menu_render_shape`、`ux_sys_user_session_current`、`ux_sys_operation_log_request_id` 不存在时
跳过，但 old/new 同时存在时仍然失败。其余当前对象缺失或目标对象冲突时整体回滚。

- [ ] **Step 6: 运行数据库迁移测试**

```powershell
cd D:\admin\server
go fmt ./internal/database/...
go test ./internal/database -run TestPrepareDomainNames -count=1
```

预期：六个迁移场景全部通过，测试 schema 在 cleanup 中删除。

**Owner-controlled commit checkpoint:** `feat!: 增加业务域表名迁移`

---

### Task 2: 将 Model、Schema 和 Repository 全面切换到新表名

**Files:**
- Modify: `server/cmd/api/main.go`
- Modify: `server/internal/module/user/model.go`
- Modify: `server/internal/module/user/repository.go`
- Modify: `server/internal/module/user/repository_test.go`
- Modify: `server/internal/module/user/service_test.go`
- Modify: `server/internal/module/auth/session_model.go`
- Modify: `server/internal/module/auth/schema.go`
- Modify: `server/internal/module/auth/schema_test.go`
- Modify: `server/internal/module/auth/session_repository.go`
- Modify: `server/internal/module/auth/session_repository_test.go`
- Modify: `server/internal/module/auth/session_admin.go`
- Modify: `server/internal/module/auth/session_admin_integration_test.go`
- Modify: `server/internal/module/role/model.go`
- Modify: `server/internal/module/role/schema.go`
- Modify: `server/internal/module/role/schema_test.go`
- Modify: `server/internal/module/role/repository.go`
- Modify: `server/internal/module/role/repository_test.go`
- Modify: `server/internal/module/role/service_test.go`
- Modify: `server/internal/module/menu/model.go`
- Modify: `server/internal/module/menu/schema.go`
- Modify: `server/internal/module/menu/schema_test.go`
- Modify: `server/internal/module/menu/repository.go`
- Modify: `server/internal/module/menu/repository_test.go`
- Modify: `server/internal/module/menu/test_models_test.go`
- Modify: `server/internal/module/access/model.go`
- Modify: `server/internal/module/access/schema.go`
- Modify: `server/internal/module/access/schema_test.go`
- Modify: `server/internal/module/access/repository.go`
- Modify: `server/internal/module/access/repository_test.go`
- Modify: `server/internal/module/authplatform/model.go`
- Modify: `server/internal/module/authplatform/schema.go`
- Modify: `server/internal/module/authplatform/schema_test.go`
- Modify: `server/internal/module/authplatform/repository.go`
- Modify: `server/internal/module/authplatform/repository_test.go`
- Modify: `server/internal/module/authplatform/service_test.go`
- Modify: `server/internal/module/operationlog/model.go`
- Modify: `server/internal/module/operationlog/schema.go`
- Modify: `server/internal/module/operationlog/schema_test.go`
- Modify: `server/internal/module/operationlog/repository.go`
- Modify: `server/internal/module/operationlog/integration_test.go`
- Test: all `server/internal/module/*/*_test.go` files containing a legacy table literal

- [ ] **Step 1: 将 TableName 和 schema 测试断言先改为目标名称**

目标 Model 映射：

```go
func (User) TableName() string         { return "user_account" }
func (Session) TableName() string      { return "auth_session" }
func (Menu) TableName() string         { return "rbac_menu" }
func (Role) TableName() string         { return "rbac_role" }
func (UserRole) TableName() string     { return "rbac_user_role" }
func (RoleMenu) TableName() string     { return "rbac_role_menu" }
func (Version) TableName() string      { return "rbac_access_version" }
func (Platform) TableName() string     { return "auth_platform" }
func (OperationLog) TableName() string { return "audit_operation_log" }
```

各 schema test 同步断言新表、新 `ck_*`/`fk_*`/`ux_*`/`ix_*` 名称，并断言对应旧表名不存在。

- [ ] **Step 2: 运行模块测试并确认因旧实现失败**

```powershell
cd D:\admin\server
go test ./internal/module/user ./internal/module/auth ./internal/module/role ./internal/module/menu ./internal/module/access ./internal/module/authplatform ./internal/module/operationlog -count=1
```

预期：TableName、schema 对象和原始 SQL 仍引用 `sys_*`，测试按目标原因失败。

- [ ] **Step 3: 更新 Model tag 和 TableName**

除表名和约束名外不改变业务字段。GORM tag 中的 CHECK 名同步，例如：

```go
IsEnabled yesno.Value `gorm:"column:is_enabled;type:smallint;not null;default:1;check:ck_user_account_is_enabled,is_enabled IN (0,1)"`

Version int64 `gorm:"column:version;not null;default:1;check:ck_auth_session_version,version >= 1"`

IsDefault yesno.Value `gorm:"column:is_default;type:smallint;not null;default:0;check:ck_rbac_role_is_default,is_default IN (0,1)"`
```

每个 Model 继续直接声明 `CreatedAt` 和 `UpdatedAt` 的非空 TIMESTAMPTZ；不得顺便引入
BaseModel 或删除现有 `deleted_at`。

- [ ] **Step 4: 更新各模块 Schema DDL**

按以下关系修改所有 DDL 和 catalog 检查：

```text
auth:         user_account + auth_session
role:         rbac_role + rbac_user_role -> user_account
menu:         rbac_menu + rbac_role_menu -> rbac_role/rbac_menu
access:       rbac_access_version -> user_account
authplatform: auth_platform
operationlog: audit_operation_log
```

示例：

```sql
CREATE UNIQUE INDEX IF NOT EXISTS ux_user_account_username_active
ON user_account (lower(username)) WHERE deleted_at IS NULL;

ALTER TABLE rbac_role_menu ADD CONSTRAINT fk_rbac_role_menu_menu
FOREIGN KEY (menu_id) REFERENCES rbac_menu(id) ON DELETE RESTRICT;

ALTER TABLE rbac_access_version ADD CONSTRAINT fk_rbac_access_version_user
FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT;
```

所有 `to_regclass`、`information_schema` 和 `pg_constraint.conrelid` 查询也改为新表名。

- [ ] **Step 5: 定向更新 Repository 原始 SQL**

逐文件修改，不做不受控的全仓替换。完成后运行：

```powershell
cd D:\admin
rg -n "sys_(user|user_session|menu|role|user_role|role_menu|access_version|auth_platform|operation_log)" server\internal\module -g "*.go"
```

预期：只允许 `database/domain_names.go`、一次性 menu legacy migration 和对应迁移测试出现旧名；
Model、Schema 当前 DDL、Repository、Service 和普通测试夹具不得出现旧名。

- [ ] **Step 6: 把领域迁移接到 AutoMigrate 之前**

`server/cmd/api/main.go` 在打开 PostgreSQL 后立即执行：

```go
if err := database.PrepareDomainNames(processContext, postgres.GORM); err != nil {
	return fmt.Errorf("prepare domain database names: %w", err)
}
if err := auth.PrepareSessionSchema(processContext, postgres.GORM); err != nil {
	return fmt.Errorf("prepare authentication schema: %w", err)
}
if err := operationlog.PrepareSchema(processContext, postgres.GORM); err != nil {
	return fmt.Errorf("prepare operation log schema: %w", err)
}
```

Task 3 会在同一位置补 `menu.PrepareSchema`。当前 `database.AutoMigrate` 模型列表保持完整，
但全部 Model 已返回新表名。

- [ ] **Step 7: 运行新表名后端测试**

```powershell
cd D:\admin\server
go fmt ./...
go test ./internal/database ./internal/module/user ./internal/module/auth ./internal/module/role ./internal/module/menu ./internal/module/access ./internal/module/authplatform ./internal/module/operationlog ./cmd/api ./cmd/worker -count=1
```

预期：全部通过；任何共享 public schema 污染应先改为 `testschema.Open`，不能通过删除开发数据
掩盖测试隔离问题。

**Owner-controlled commit checkpoint:** `refactor!: 统一后端业务域表名`

---

### Task 3: 原地迁移菜单协议、权限所有权和缓存任务版本

**Files:**
- Modify: `server/cmd/api/main.go`
- Create: `server/cmd/api/menu_foundation.go`
- Modify: `server/cmd/api/main_test.go`
- Modify: `server/internal/module/menu/model.go`
- Modify: `server/internal/module/menu/protocol.go`
- Create: `server/internal/module/menu/foundation.go`
- Modify: `server/internal/module/menu/schema.go`
- Modify: `server/internal/module/menu/schema_migration_test.go`
- Modify: `server/internal/module/menu/schema_test.go`
- Modify: `server/internal/module/menu/request.go`
- Modify: `server/internal/module/menu/response.go`
- Modify: `server/internal/module/menu/tree.go`
- Modify: `server/internal/module/menu/tree_test.go`
- Modify: `server/internal/module/menu/repository.go`
- Modify: `server/internal/module/menu/repository_test.go`
- Modify: `server/internal/module/menu/service.go`
- Modify: `server/internal/module/menu/service_test.go`
- Modify: `server/internal/module/menu/errors.go`
- Modify: `server/internal/module/menu/errors_test.go`
- Modify: `server/internal/module/menu/handler_test.go`
- Modify: `server/internal/module/role/protocol.go`
- Modify: `server/internal/module/role/route.go`
- Modify: `server/internal/module/role/permission.go`
- Modify: `server/internal/module/role/response.go`
- Modify: `server/internal/module/role/*_test.go`
- Modify: `server/internal/module/user/protocol.go`
- Modify: `server/internal/module/user/protocol_test.go`
- Modify: `server/internal/module/user/route.go`
- Modify: `server/internal/module/user/handler_test.go`
- Create: `server/internal/module/auth/protocol.go`
- Modify: `server/internal/module/auth/session_admin_route.go`
- Modify: `server/internal/module/auth/session_admin_handler_test.go`
- Modify: `server/internal/module/authplatform/protocol.go`
- Modify: `server/internal/module/authplatform/route.go`
- Modify: `server/internal/module/authplatform/handler_test.go`
- Create: `server/internal/module/operationlog/protocol.go`
- Modify: `server/internal/module/operationlog/route.go`
- Modify: `server/internal/module/operationlog/handler_test.go`
- Modify: `server/internal/module/operationlog/task.go`
- Modify: `server/internal/module/operationlog/task_test.go`
- Modify: `server/internal/module/access/service.go`
- Modify: `server/internal/module/access/service_test.go`
- Modify: `server/internal/module/access/redis.go`
- Modify: `server/internal/module/access/redis_test.go`
- Modify: `server/internal/module/accessstate/state.go`
- Modify: `server/internal/module/accessstate/state_test.go`
- Modify: `server/internal/shared/i18n/catalog.go`

- [ ] **Step 1: 写完整 legacy menu rekey 失败测试**

重写 `openLegacyMenuSchema`，在独立 schema 创建当前 25 个 legacy 节点、角色关系和 access
version。fixture 将 `system:menu:*` 四行及其一组 role-menu 关系用同一个 `retiredAt` 软删除，
另建一条删除时间不同的历史 role-menu 关系，证明迁移不会泛化恢复。测试调用顺序：

```go
if err := database.PrepareDomainNames(ctx, db); err != nil {
	t.Fatal(err)
}
if err := menu.PrepareSchema(ctx, db); err != nil {
	t.Fatal(err)
}
if err := database.AutoMigrate(ctx, db, currentModels()...); err != nil {
	t.Fatal(err)
}
if err := menu.EnsureSchema(ctx, db); err != nil {
	t.Fatal(err)
}
```

断言：

- legacy `system` row ID 原地变为 `access`；
- 新 `account` 和新 `system` 是另外两行；
- 所有旧 page/action ID 不变；
- `rbac_role_menu.menu_id` 不变；
- 被错误软删除的四个 `system:menu:*` row 原 ID 恢复为有效数据；
- 只有 `deleted_at == retiredAt` 的对应 role-menu row 原 ID 被恢复，其他已删除授权保持删除；
- 所有 `rbac_access_version.version` 精确 `+1`；
- 三个 root、六个 page 和全部 action 的 code/parent/path/componentPath 精确匹配 Spec；
- action `i18n_key IS NULL`、`icon IS NULL`、`is_hidden = 1`；
- `name` 非空，未知 code 或未知 icon 会列出值并回滚；
- 重复 `PrepareSchema` 不再次推进 version 或覆盖管理员展示字段。

- [ ] **Step 2: 写权限所有权和超级管理员失败测试**

每个模块 route test 捕获 `requirePermission` 参数，精确断言本模块常量。示例：

```go
func TestRoleRoutesUseRBACPermissions(t *testing.T) {
	want := []string{
		role.PermissionList,
		role.PermissionCreate,
		role.PermissionUpdate,
		role.PermissionStatus,
		role.PermissionDefault,
		role.PermissionDelete,
		role.PermissionAuthorize,
		role.PermissionAuthorize,
	}
	// 使用现有 route registration helper 收集并比较。
}
```

`access/service_test.go` 新增：数据库 source 不包含菜单管理节点时，超级管理员的
`PermissionCodes` 也不包含任何 `rbac:menu:*`；source 包含节点时才包含。

- [ ] **Step 3: 运行后端协议测试并确认失败**

```powershell
cd D:\admin\server
go test ./internal/module/menu ./internal/module/role ./internal/module/user ./internal/module/auth ./internal/module/authplatform ./internal/module/operationlog ./internal/module/access ./internal/module/accessstate -count=1
```

预期：缺少 name、旧 code/path、权限常量仍集中在 menu、旧缓存版本和旧 task type 导致失败。

- [ ] **Step 4: 将权限常量归还业务模块**

精确常量：

```go
// menu/protocol.go
const (
	PermissionList   = "rbac:menu:list"
	PermissionCreate = "rbac:menu:create"
	PermissionUpdate = "rbac:menu:update"
	PermissionDelete = "rbac:menu:delete"
)

// role/protocol.go
const (
	PermissionList      = "rbac:role:list"
	PermissionCreate    = "rbac:role:create"
	PermissionUpdate    = "rbac:role:update"
	PermissionStatus    = "rbac:role:status"
	PermissionDefault   = "rbac:role:default"
	PermissionDelete    = "rbac:role:delete"
	PermissionAuthorize = "rbac:role:authorize"
)

// user/protocol.go
const (
	PermissionList   = "account:user:list"
	PermissionUpdate = "account:user:update"
	PermissionStatus = "account:user:status"
	PermissionDelete = "account:user:delete"
	PermissionRoles  = "account:user:roles"
)

// auth/protocol.go
const (
	PermissionSessionList   = "auth:session:list"
	PermissionSessionRevoke = "auth:session:revoke"
)

// authplatform/protocol.go
const (
	PermissionList   = "auth:platform:list"
	PermissionCreate = "auth:platform:create"
	PermissionUpdate = "auth:platform:update"
	PermissionStatus = "auth:platform:status"
	PermissionDelete = "auth:platform:delete"
)

// operationlog/protocol.go
const PermissionList = "audit:operation-log:list"
const TaskType = "audit:operation-log:v2"
```

route.go 只引用同包常量。删除 menu 中所有 Role/User/AuthPlatform/Session/OperationLog 权限
常量；删除其他模块对 `menu.Permission*` 的引用。

- [ ] **Step 5: 增加 menu name 和可空 i18nKey 协议**

Model 和 Service 类型：

```go
type Menu struct {
	ID            int64
	ParentID      *int64
	MenuType      Type
	Name          string
	Code          string
	I18nKey       *string
	Path          *string
	ComponentPath *string
	Icon          *string
	SortOrder     int
	IsEnabled     yesno.Value
	IsHidden      yesno.Value
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt
}

type CreateInput struct {
	ParentID *int64
	MenuType Type
	Name string
	Code string
	I18nKey *string
	Path *string
	ComponentPath *string
	Icon *string
	SortOrder int
	IsEnabled yesno.Value
	IsHidden yesno.Value
}
```

`UpdateInput` 同样增加 `Name`、把 `I18nKey` 改为 `*string`，但继续不允许更新 code。
request 使用 `nullableString` 要求 `i18nKey` 字段显式出现；response 返回
`name`、`i18nKey: string | null` 和 `isProtected`。

`validMenuName` 使用 trim、UTF-8 rune 1-128；directory/page 要求合法 i18nKey，action 要求
nil。PostgreSQL `ck_rbac_menu_shape` 同步校验 name、i18n_key 和三种 shape。

- [ ] **Step 6: 实现 old code -> new code 原地迁移**

`menu.PrepareSchema` 在 `rbac_menu` 存在时执行事务。核心 code 映射必须完整：

```go
var legacyPermissionCodes = map[string]string{
	"system:menu:list": "rbac:menu:list",
	"system:menu:create": "rbac:menu:create",
	"system:menu:update": "rbac:menu:update",
	"system:menu:delete": "rbac:menu:delete",
	"system:role:list": "rbac:role:list",
	"system:role:create": "rbac:role:create",
	"system:role:update": "rbac:role:update",
	"system:role:status": "rbac:role:status",
	"system:role:default": "rbac:role:default",
	"system:role:delete": "rbac:role:delete",
	"system:role:authorize": "rbac:role:authorize",
	"system:user:list": "account:user:list",
	"system:user:update": "account:user:update",
	"system:user:status": "account:user:status",
	"system:user:delete": "account:user:delete",
	"system:user:roles": "account:user:roles",
	"system:session:list": "auth:session:list",
	"system:session:revoke": "auth:session:revoke",
	"system:auth-platform:list": "auth:platform:list",
	"system:auth-platform:create": "auth:platform:create",
	"system:auth-platform:update": "auth:platform:update",
	"system:auth-platform:status": "auth:platform:status",
	"system:auth-platform:delete": "auth:platform:delete",
	"system:operation-log:list": "audit:operation-log:list",
}
```

`name` 回填不读取 i18n 语言包，也不把 code 机械转成人类文案。当前 25 个 legacy 节点使用
以下闭合映射；缺少任一当前节点映射时列出 code 并回滚：

```go
var legacyMenuNames = map[string]string{
	"system":                         "权限与认证",
	"system:menu:list":               "菜单管理",
	"system:menu:create":             "新增菜单",
	"system:menu:update":             "修改菜单",
	"system:menu:delete":             "删除菜单",
	"system:role:list":               "角色管理",
	"system:role:create":             "新增角色",
	"system:role:update":             "修改角色",
	"system:role:status":             "修改角色状态",
	"system:role:default":            "设置默认角色",
	"system:role:delete":             "删除角色",
	"system:role:authorize":          "配置角色权限",
	"system:user:list":               "用户管理",
	"system:user:update":             "修改用户",
	"system:user:status":             "修改用户状态",
	"system:user:delete":             "删除用户",
	"system:user:roles":              "分配用户角色",
	"system:session:list":            "会话管理",
	"system:session:revoke":          "踢出会话",
	"system:auth-platform:list":      "认证平台",
	"system:auth-platform:create":    "新增认证平台",
	"system:auth-platform:update":    "编辑认证平台",
	"system:auth-platform:status":    "变更认证平台状态",
	"system:auth-platform:delete":    "删除认证平台",
	"system:operation-log:list":      "操作日志",
}
```

当前开发库的已知图标按 legacy code 校验并迁移，不能只按旧图标字符串全局替换，因为两个
`List` 的目标语义不同：

```go
type legacyIconTarget struct {
	Old string
	New string
}

var legacyMenuIcons = map[string]legacyIconTarget{
	"system":                    {Old: "Setting", New: "lucide:shield-check"},
	"system:menu:list":          {Old: "Menu", New: "lucide:panel-left"},
	"system:role:list":          {Old: "UserFilled", New: "lucide:user-cog"},
	"system:user:list":          {Old: "User", New: "lucide:user-round-cog"},
	"system:auth-platform:list": {Old: "Key", New: "lucide:key-round"},
	"system:session:list":       {Old: "List", New: "lucide:monitor-smartphone"},
	"system:operation-log:list": {Old: "List", New: "lucide:scroll-text"},
}
```

这七个节点只接受表中写明的旧值；其余 action 必须为 `NULL`。先完成旧值校验，再更新 icon；
遇到其他非空 icon 时错误必须包含 code 和原值。复用的 `system` row 同时更新为
`code=access`、`name=权限与认证`、`i18n_key=navigation.access`、`sort_order=200`；新插入的
`account` 和 `system` root 使用 Spec 第 6.1 节的 name/i18nKey/icon/sortOrder。

page 目标：

```go
var migratedPages = map[string]pageTarget{
	"account:user:list": {ParentCode: "account", Path: "/account/users", ComponentPath: "account/users", I18nKey: "navigation.accountUsers", Icon: "lucide:user-round-cog", SortOrder: 10},
	"auth:session:list": {ParentCode: "account", Path: "/account/sessions", ComponentPath: "account/sessions", I18nKey: "navigation.accountSessions", Icon: "lucide:monitor-smartphone", SortOrder: 20},
	"rbac:menu:list": {ParentCode: "access", Path: "/access/menus", ComponentPath: "access/menus", I18nKey: "navigation.accessMenus", Icon: "lucide:panel-left", SortOrder: 10},
	"rbac:role:list": {ParentCode: "access", Path: "/access/roles", ComponentPath: "access/roles", I18nKey: "navigation.accessRoles", Icon: "lucide:user-cog", SortOrder: 20},
	"auth:platform:list": {ParentCode: "access", Path: "/access/auth-platforms", ComponentPath: "access/auth-platforms", I18nKey: "navigation.accessAuthPlatforms", Icon: "lucide:key-round", SortOrder: 30},
	"audit:operation-log:list": {ParentCode: "system", Path: "/system/operation-logs", ComponentPath: "system/operation-logs", I18nKey: "navigation.systemOperationLogs", Icon: "lucide:scroll-text", SortOrder: 10},
}
```

复用旧 `system` ID 作为 access；插入 account/system root 后按 parent code 更新 page，再按 page
更新 action parent。先校验目标 code 不被其他 ID 占用，再更新 code，必要时使用事务内临时
code 避免唯一索引碰撞。最后：

```sql
UPDATE rbac_access_version
SET version = version + 1, updated_at = CURRENT_TIMESTAMP;
```

只有实际发生 legacy rekey 时推进一次。保留 `view_key -> component_path` 历史迁移，但删除
`retireLegacyMenuManagementNode` 及其软删除行为。rekey 查询必须使用 unscoped 数据，保证当前
已被错误删除的 `system:menu:*` 四行仍按原 ID 更新。在清除这些 menu row 的 `deleted_at` 之前，
先用精确时间相等恢复关联授权：

```sql
UPDATE rbac_role_menu AS role_menu
SET deleted_at = NULL,
    updated_at = CURRENT_TIMESTAMP
FROM rbac_menu AS menu
WHERE role_menu.menu_id = menu.id
  AND menu.code IN (
    'system:menu:list',
    'system:menu:create',
    'system:menu:update',
    'system:menu:delete'
  )
  AND menu.deleted_at IS NOT NULL
  AND role_menu.deleted_at = menu.deleted_at;
```

随后只恢复这四个受保护 menu row。不得使用 `menu_id IN (...) AND deleted_at IS NOT NULL` 泛化
恢复授权；删除时间不同的历史关系和其他普通软删除数据必须保持不变。

- [ ] **Step 7: 实现受保护基础节点和空库首次种子**

`menu/foundation.go` 定义：

```go
type FoundationDefinition struct {
	ParentCode string
	MenuType Type
	Name string
	Code string
	I18nKey *string
	Path *string
	ComponentPath *string
	Icon *string
	SortOrder int
	IsEnabled yesno.Value
	IsHidden yesno.Value
	Protected bool
}

func IsProtectedCode(code string) bool {
	return code == "access" || code == PermissionList ||
		code == PermissionCreate || code == PermissionUpdate || code == PermissionDelete
}
```

`cmd/api/menu_foundation.go` 使用 menu/role/user/auth/authplatform/operationlog 常量构造三个 root、
六个 page 和全部 action。空表时在一个事务中创建完整 catalog；非空表只恢复/创建五个
protected 定义，不复活被管理员删除的普通节点。

Service 的 `EnsureFoundation(ctx, definitions)` 继续使用 repository 锁、access invalidating
lease 和 version 推进。基础节点无变化时零写入、零 version 变化。Update 只允许受保护节点
展示字段变化；UpdateStatus/Delete/结构变化返回 `CodeMenuProtected`。

- [ ] **Step 8: 删除超级管理员权限硬编码并更新角色名称 DTO**

删除 `access.buildSnapshot` 中：

```go
permissionCodes = append(permissionCodes,
	menu.PermissionList,
	menu.PermissionCreate,
	menu.PermissionUpdate,
	menu.PermissionDelete,
)
```

`role.PermissionTreeNode` 改为：

```go
type PermissionTreeNode struct {
	ID int64
	ParentID *int64
	MenuType menu.Type
	Code string
	Name string
	IsEnabled yesno.Value
	Children []PermissionTreeNode
}
```

`access.SourceMenu.I18nKey` 同步改为 `*string`，Repository 才能扫描 action 的 `NULL`。构建
access tree 时只对 directory/page 解引用：这两类为 nil 时明确返回协议错误，再写入仍保持
`I18nKey string` 的 `access.MenuNode`/HTTP DTO；action 不进入 menu tree，只进入
`permissionCodes`。角色授权树展示 name，不再返回 action i18nKey。

- [ ] **Step 9: 提升缓存协议并重命名 Asynq task**

```go
// accessstate/state.go
const SchemaVersion = 2

// access/redis.go
const accessSnapshotSchemaVersion = 3
```

旧 state 必须解码失败并由现有 repair 流程使用 PostgreSQL version 重建。operationlog
`task.go` 全部使用 `TaskType`；payload 的 `schemaVersion = 2`、闭合 DTO、脱敏和未知字段拒绝
规则保持不变。

- [ ] **Step 10: 固定 API 启动顺序**

```go
database.PrepareDomainNames
-> auth.PrepareSessionSchema
-> operationlog.PrepareSchema
-> menu.PrepareSchema
-> database.AutoMigrate
-> role/authplatform/auth/menu/access/operationlog EnsureSchema
-> open Redis and Queue
-> construct access invalidator/menu service
-> menuService.EnsureFoundation(processContext, menuFoundation())
-> build router
```

当前代码中提前打开 Redis/Queue 的逻辑必须下移到全部 Schema/AutoMigrate/EnsureSchema 成功
之后，不允许保留提前连接。foundation 必须在 router 和 HTTP server 之前完成；任何步骤失败
都关闭已经打开的资源并返回错误，不启动部分 API。

- [ ] **Step 11: 运行后端完整定向测试**

```powershell
cd D:\admin\server
go fmt ./...
go test ./internal/module/menu ./internal/module/role ./internal/module/user ./internal/module/auth ./internal/module/authplatform ./internal/module/operationlog ./internal/module/access ./internal/module/accessstate ./cmd/api ./cmd/worker -count=1
```

预期：迁移、权限所有权、foundation、角色名称、缓存协议和 task type 测试全部通过。

**Owner-controlled commit checkpoint:** `feat!: 切换业务域权限与菜单协议`

---

### Task 4: 同步前端业务域目录、路由、权限与 DTO

**Files:**
- Move: `web/src/views/system/users` -> `web/src/views/account/users`
- Move: `web/src/views/system/sessions` -> `web/src/views/account/sessions`
- Move: `web/src/views/system/menus` -> `../../../web/src/views/permission/menus`
- Move: `web/src/views/system/roles` -> `../../../web/src/views/permission/roles`
- Move: `web/src/views/system/auth-platforms` -> `../../../web/src/views/permission/auth-platforms`
- Move matching tests under `web/tests/views/system/*` to `web/tests/views/account/*` or `web/tests/views/access/*`
- Move: `web/src/styles/system-pages.scss` -> `web/src/styles/management-pages.scss`
- Modify: `web/src/styles/index.scss`
- Modify: all moved Vue/test imports
- Modify: `web/src/api/menu-fields.ts`
- Modify: `web/src/api/menu.contract.ts`
- Modify: `web/src/api/role.contract.ts`
- Modify: `web/src/api/access.contract.ts`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/router/access-routes.ts`
- Modify: `web/src/layout/components/AppAside.vue`
- Modify: `web/src/layout/breadcrumbs.ts`
- Modify: `web/src/layout/components/RouteTabs.vue`
- Modify: `web/src/layout/index.vue`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Test: `web/tests/router/index.test.ts`
- Test: `web/tests/router/access-routes.test.ts`
- Test: `web/tests/layout/breadcrumbs.test.ts`
- Test: `web/tests/layout/components/AccessMenuNode.test.ts`
- Test: `web/tests/layout/components/RouteTabs.test.ts`
- Test: `web/tests/layout/index.test.ts`
- Test: `web/tests/api/*.test.ts`

- [ ] **Step 1: 先移动测试并改成新路径/权限断言**

目标测试路径：

```text
web/tests/views/account/users/index.test.ts
web/tests/views/account/sessions/index.test.ts
web/tests/views/access/menus/index.test.ts
web/tests/views/access/roles/index.test.ts
web/tests/views/access/roles/components/RolePermissionMatrix.test.ts
web/tests/views/access/roles/role-permission-matrix.test.ts
web/tests/views/access/auth-platforms/index.test.ts
web/tests/views/system/operation-logs/index.test.ts
```

路由测试固定映射：

```ts
const expectedPages = [
  ['account:user:list', '/account/users', 'account/users'],
  ['auth:session:list', '/account/sessions', 'account/sessions'],
  ['rbac:menu:list', '/access/menus', 'access/menus'],
  ['rbac:role:list', '/access/roles', 'access/roles'],
  ['auth:platform:list', '/access/auth-platforms', 'access/auth-platforms'],
  ['audit:operation-log:list', '/system/operation-logs', 'system/operation-logs'],
] as const
```

增加多根 fixture，断言 account/access/system 都能递归注册和渲染。

- [ ] **Step 2: 运行前端测试确认旧路径失败**

```powershell
cd D:\admin\web
pnpm vitest run tests/router/index.test.ts tests/router/access-routes.test.ts tests/layout tests/api
```

预期：静态 route、componentPath、DTO closed keys 和权限判断仍为旧协议而失败。

- [ ] **Step 3: 移动页面并更新通用样式名称**

移动页面后更新相对 import。所有管理页面 class：

```text
system-page          -> management-page
system-page__filters -> management-page__filters
system-page__actions -> management-page__actions
```

`styles/index.scss` 改为：

```scss
@use './variables';
@use './management-pages';
```

`operation-logs` 虽然仍在 system 目录，也使用领域无关的 `management-page` 公共样式。

- [ ] **Step 4: 更新菜单和角色严格 DTO**

```ts
export interface ManagedMenuNode {
  id: number
  parentId: number | null
  menuType: ManagedMenuType
  name: string
  code: string
  i18nKey: string | null
  path: string | null
  componentPath: string | null
  icon: string | null
  sortOrder: number
  isEnabled: YesNo
  isHidden: YesNo
  isProtected: YesNo
  createdAt: string
  updatedAt: string
  children: ManagedMenuNode[]
}

export interface RolePermissionTreeNode {
  id: number
  parentId: number | null
  menuType: RolePermissionMenuType
  code: string
  name: string
  isEnabled: YesNo
  children: RolePermissionTreeNode[]
}
```

Create/Update menu DTO 增加 name，i18nKey 改为 `string | null`。closed parser 拒绝旧 role
`i18nKey`、缺少 name/isProtected 和 action 非 null i18nKey。

- [ ] **Step 5: 修改静态菜单管理绑定**

`router/index.ts`：

```ts
{
  path: 'access/menus',
  name: 'access-menus',
  component: () => import('../views/access/menus/index.vue'),
  meta: {
    requiresAuth: true,
    i18nKey: 'navigation.accessMenus',
    requiredPermission: 'rbac:menu:list',
  },
}
```

`access-routes.ts` 唯一静态 binding：

```ts
const staticPageBinding = {
  code: 'rbac:menu:list',
  path: '/access/menus',
  componentPath: 'access/menus',
  routeName: 'access-menus',
} as const
```

精确匹配时复用 record；code/path/componentPath/routeName 任一不一致都抛 `ProtocolError`。
其他数据库页面继续 `addRoute`，失败时倒序清理。

- [ ] **Step 6: 删除布局特例并支持三根菜单**

`AppAside.vue` 删除手工 `/system/menus` 项，只保留 Dashboard 和：

```vue
<AccessMenuNode
  v-for="node in access.menuTree"
  :key="node.code"
  :node="node"
/>
```

`breadcrumbs.ts` 删除 `/system/menus` 特例。RouteTabs 先按当前 path 查完整 access tree，再对
Dashboard 使用静态 meta。多根遍历必须从全部 roots 开始，不假设唯一 `system`。

- [ ] **Step 7: 更新所有页面权限判断和导航 i18n**

页面权限精确替换：

```text
users          -> account:user:*
sessions       -> auth:session:*
menus          -> rbac:menu:*
roles          -> rbac:role:*
auth-platforms -> auth:platform:*
operation-logs -> audit:operation-log:list
```

导航 key：

```text
navigation.account
navigation.accountUsers
navigation.accountSessions
navigation.access
navigation.accessMenus
navigation.accessRoles
navigation.accessAuthPlatforms
navigation.system
navigation.systemOperationLogs
```

中英文 key 集合必须完全一致。删除旧 `navigation.systemUsers/systemSessions/systemMenus/
systemRoles/systemAuthPlatforms`，不保留 alias。

- [ ] **Step 8: 运行路由、布局、DTO 和页面定向测试**

```powershell
cd D:\admin\web
pnpm vitest run tests/api tests/router tests/layout tests/views/account tests/views/access tests/views/system/operation-logs
pnpm build
```

预期：全部通过，`import.meta.glob` 能找到所有新 componentPath，TypeScript 无旧目录 import。

**Owner-controlled commit checkpoint:** `refactor!: 统一前端业务域命名`

---

### Task 5: 建立完全本地的 Lucide 菜单图标协议

**Files:**
- Modify: `web/package.json`
- Modify: `web/pnpm-lock.yaml`
- Create: `web/src/icons/menu-icons.ts`
- Modify: `web/src/api/menu-fields.ts`
- Modify: `web/src/api/menu.contract.ts`
- Modify: `web/src/api/access.contract.ts`
- Modify: `web/src/components/DIcon/src/types.ts`
- Modify: `web/src/components/DIcon/src/index.vue`
- Modify: `web/src/components/IconSelect/src/types.ts`
- Modify: `web/src/components/IconSelect/src/index.vue`
- Modify: `web/src/layout/components/AccessMenuNode.vue`
- Test: `web/tests/api/menu-fields.test.ts`
- Test: `web/tests/api/menu.contract.test.ts`
- Test: `web/tests/api/access.contract.test.ts`
- Test: `web/tests/components/DIcon/src/index.test.ts`
- Test: `web/tests/components/IconSelect/src/index.test.ts`
- Test: `web/tests/layout/components/AccessMenuNode.test.ts`
- Modify: `server/internal/module/menu/tree.go`
- Modify: `server/internal/module/menu/tree_test.go`

- [ ] **Step 1: 写本地解析和禁止混合协议失败测试**

测试固定要求：

```ts
expect(isMenuIcon('lucide:shield-check')).toBe(true)
expect(isMenuIcon('Setting')).toBe(false)
expect(isMenuIcon('mdi:shield')).toBe(false)
expect(isMenuIcon('lucide:not-in-registry')).toBe(false)
```

挂载 `DIcon` 时 mock `global.fetch` 并断言零调用；每个 foundation icon 都实际渲染 SVG；未知
图标触发明确错误状态。IconSelect 测试搜索中文语义、英文 code、选择、清除和空态。

后端 `tree_test.go` 同样拒绝非 registry icon，防止绕过前端直接写入数据库。

- [ ] **Step 2: 运行图标测试并确认失败**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/menu-fields.test.ts tests/api/menu.contract.test.ts tests/api/access.contract.test.ts tests/components/DIcon/src/index.test.ts tests/components/IconSelect/src/index.test.ts
```

预期：当前 DIcon 仍支持 Element Plus/Iconify 字符串，测试失败。

- [ ] **Step 3: 切换依赖到 Lucide Vue**

```powershell
cd D:\admin\web
pnpm remove @iconify/vue
pnpm add lucide-vue-next
```

不从 CDN、Iconify API 或远程 JSON 加载图标。

- [ ] **Step 4: 创建唯一前端图标目录**

`menu-icons.ts` 静态 import 并导出稳定 code：

```ts
import {
  Activity, Bell, Bot, BrainCircuit, CircleDollarSign, Cloud, CloudUpload,
  Cpu, Database, FileStack, Folder, Gauge, HardDrive, House, Images,
  KeyRound, LayoutDashboard, ListTree, LockKeyhole, MessageSquareMore,
  MonitorSmartphone, PanelLeft, ScrollText, Server, Settings2, ShieldCheck,
  Sparkles, UserCog, UserRound, UserRoundCog, Users, UsersRound, WalletCards,
} from 'lucide-vue-next'
import type { Component } from 'vue'

export const menuIcons = {
  'lucide:activity': { component: Activity, label: '活动' },
  'lucide:bell': { component: Bell, label: '通知' },
  'lucide:bot': { component: Bot, label: 'AI 助手' },
  'lucide:brain-circuit': { component: BrainCircuit, label: 'AI 模型' },
  'lucide:circle-dollar-sign': { component: CircleDollarSign, label: '支付' },
  'lucide:cloud': { component: Cloud, label: '云服务' },
  'lucide:cloud-upload': { component: CloudUpload, label: '上传' },
  'lucide:cpu': { component: Cpu, label: '算力' },
  'lucide:database': { component: Database, label: '数据库' },
  'lucide:file-stack': { component: FileStack, label: '文件' },
  'lucide:folder': { component: Folder, label: '目录' },
  'lucide:gauge': { component: Gauge, label: '仪表盘' },
  'lucide:hard-drive': { component: HardDrive, label: '存储' },
  'lucide:house': { component: House, label: '首页' },
  'lucide:images': { component: Images, label: '图片' },
  'lucide:key-round': { component: KeyRound, label: '认证密钥' },
  'lucide:layout-dashboard': { component: LayoutDashboard, label: '控制台' },
  'lucide:list-tree': { component: ListTree, label: '层级列表' },
  'lucide:lock-keyhole': { component: LockKeyhole, label: '安全' },
  'lucide:message-square-more': { component: MessageSquareMore, label: '对话' },
  'lucide:monitor-smartphone': { component: MonitorSmartphone, label: '会话设备' },
  'lucide:panel-left': { component: PanelLeft, label: '菜单' },
  'lucide:scroll-text': { component: ScrollText, label: '操作日志' },
  'lucide:server': { component: Server, label: '服务器' },
  'lucide:settings-2': { component: Settings2, label: '系统设置' },
  'lucide:shield-check': { component: ShieldCheck, label: '权限认证' },
  'lucide:sparkles': { component: Sparkles, label: '智能能力' },
  'lucide:user-cog': { component: UserCog, label: '角色' },
  'lucide:user-round': { component: UserRound, label: '用户' },
  'lucide:user-round-cog': { component: UserRoundCog, label: '用户管理' },
  'lucide:users': { component: Users, label: '用户组' },
  'lucide:users-round': { component: UsersRound, label: '用户与账号' },
  'lucide:wallet-cards': { component: WalletCards, label: '钱包' },
} as const satisfies Record<string, { component: Component; label: string }>

export type MenuIconName = keyof typeof menuIcons

export function isMenuIconName(value: string): value is MenuIconName {
  return Object.prototype.hasOwnProperty.call(menuIcons, value)
}
```

- [ ] **Step 5: 重写 DIcon 和 IconSelect**

`DIcon` 字符串分支只接受 `MenuIconName` 并同步解析：

```ts
const resolvedComponent = computed<Component | null>(() => {
  if (props.component !== undefined) return props.component
  if (props.icon === undefined || !isMenuIconName(props.icon)) return null
  return menuIcons[props.icon].component
})
```

无异步 import、无 Iconify `<Icon>`、无联网。invalid 状态使用明确 title/console error；协议 parser
应在业务入口更早拒绝未知字符串。

IconSelect 的 options 由 `Object.entries(menuIcons)` 生成，搜索 `code + label`。选择后 emit 精确
`MenuIconName`；清除由父表单设置 null，不使用空字符串。

- [ ] **Step 6: 后端使用同一明确 allowlist**

`menu/tree.go` 建立与前端 code 完全一致的 `map[string]struct{}`，`validMenuIcon` 只接受集合成员：

```go
var menuIconNames = map[string]struct{}{
	"lucide:activity": {},
	"lucide:bell": {},
	"lucide:bot": {},
	"lucide:brain-circuit": {},
	"lucide:circle-dollar-sign": {},
	"lucide:cloud": {},
	"lucide:cloud-upload": {},
	"lucide:cpu": {},
	"lucide:database": {},
	"lucide:file-stack": {},
	"lucide:folder": {},
	"lucide:gauge": {},
	"lucide:hard-drive": {},
	"lucide:house": {},
	"lucide:images": {},
	"lucide:key-round": {},
	"lucide:layout-dashboard": {},
	"lucide:list-tree": {},
	"lucide:lock-keyhole": {},
	"lucide:message-square-more": {},
	"lucide:monitor-smartphone": {},
	"lucide:panel-left": {},
	"lucide:scroll-text": {},
	"lucide:server": {},
	"lucide:settings-2": {},
	"lucide:shield-check": {},
	"lucide:sparkles": {},
	"lucide:user-cog": {},
	"lucide:user-round": {},
	"lucide:user-round-cog": {},
	"lucide:users": {},
	"lucide:users-round": {},
	"lucide:wallet-cards": {},
}
```

这不是动态图标注册器，而是数据库协议 allowlist。旧值转换只使用 Task 3 中按 legacy code
定义的七条 `legacyMenuIcons`，不得再创建 `map[oldIcon]newIcon`；未知 legacy 值让 menu
migration 失败并列出 code/value，尤其不能把两个 `List` 映射为同一图标。

- [ ] **Step 7: 运行图标测试和构建**

```powershell
cd D:\admin\server
go test ./internal/module/menu -count=1

cd D:\admin\web
pnpm vitest run tests/api/menu-fields.test.ts tests/api/menu.contract.test.ts tests/api/access.contract.test.ts tests/components/DIcon/src/index.test.ts tests/components/IconSelect/src/index.test.ts tests/layout/components/AccessMenuNode.test.ts
pnpm build
```

预期：全部通过，构建依赖中不再存在 `@iconify/vue`。

**Owner-controlled commit checkpoint:** `feat!: 统一本地菜单图标协议`

---

### Task 6: 完成菜单搜索、稳定展开、Dialog 和角色名称界面

**Files:**
- Create: `../../../web/src/views/permission/menus/filter-menu-tree.ts`
- Create: `web/tests/views/access/menus/filter-menu-tree.test.ts`
- Modify: `../../../web/src/views/permission/menus/index.vue`
- Modify: `web/tests/views/access/menus/index.test.ts`
- Modify: `../../../web/src/views/permission/roles/role-permission-matrix.ts`
- Modify: `../../../web/src/views/permission/roles/components/RolePermissionMatrix.vue`
- Modify: `../../../web/src/views/permission/roles/components/RolePermissionDiffDialog.vue`
- Modify: `../../../web/src/views/permission/roles/index.vue`
- Modify: matching role tests under `web/tests/views/access/roles`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Test: `web/tests/api/menu.contract.test.ts`
- Test: `web/tests/api/role.contract.test.ts`
- Test: all menu/role view tests

- [ ] **Step 1: 写纯函数搜索失败测试**

固定行为：

```ts
it('keeps matches, ancestors, and matched-node descendants without siblings', () => {
  const result = filterManagedMenuTree(menuTree(), 'rbac:menu:create')
  expect(codes(result)).toEqual(['access', 'rbac:menu:list', 'rbac:menu:create'])

  const parentResult = filterManagedMenuTree(menuTree(), '权限与认证')
  expect(codes(parentResult)).toEqual([
    'access',
    'rbac:menu:list', 'rbac:menu:create', 'rbac:menu:update', 'rbac:menu:delete',
    'rbac:role:list',
    'auth:platform:list',
  ])
})
```

再覆盖 name、code、path 不区分英文大小写、trim、空关键词返回完整树、无结果空数组，并断言
输入节点和 children 不被原地修改。

- [ ] **Step 2: 实现搜索纯函数**

```ts
export function filterManagedMenuTree(
  nodes: readonly ManagedMenuNode[],
  rawKeyword: string,
): ManagedMenuNode[] {
  const keyword = rawKeyword.trim().toLocaleLowerCase()
  if (keyword === '') return nodes.map(cloneNode)
  return nodes.flatMap((node) => {
    const ownValues = [node.name, node.code, node.path ?? '']
    const ownMatch = ownValues.some((value) => value.toLocaleLowerCase().includes(keyword))
    if (ownMatch) return [cloneNode(node)]
    const children = filterManagedMenuTree(node.children, keyword)
    return children.length === 0 ? [] : [{ ...node, children }]
  })
}
```

`cloneNode` 递归复制 children；不得修改 API store 中的权威树。

- [ ] **Step 3: 写菜单页面交互失败测试**

必须断言：

- 首次加载只展开 account/access/system 三个 root；
- 不存在 `default-expand-all`；
- 搜索 name/code/path 显示过滤树并自动展开结果祖先；
- 清空搜索恢复搜索前 expanded IDs；
- 全部展开/收起操作真实更新每个 row key；
- 表格和父节点选项显示数据库 name，不调用 `t(name)`；
- action i18nKey/null shape 正确；
- protected 节点结构、状态和删除命令禁用并显示原因；
- Dialog 根据 directory/page/action 渲染精确字段；
- payload 不包含旧 code/path 或兼容字段；
- loading、空态、失败和重试入口存在。

- [ ] **Step 4: 重构菜单页面状态**

```ts
const sourceMenus = ref<ManagedMenuNode[]>([])
const keyword = ref('')
const expandedIDs = ref<Set<number>>(new Set())
const expansionBeforeSearch = ref<Set<number> | null>(null)

const displayedMenus = computed(() => filterManagedMenuTree(sourceMenus.value, keyword.value))
const expandedRowKeys = computed(() => [...expandedIDs.value])
```

首次成功加载设置 root IDs。进入搜索前复制展开集合；搜索时将过滤结果中所有有 children 的
节点和匹配路径祖先加入 expandedIDs；清空后恢复副本。使用 Element Plus
`:expand-row-keys="expandedRowKeys"` 和 row-key，不使用初次渲染专用 `default-expand-all`。

工具栏包含关键词输入、清除、新增根目录、展开/收起、刷新；不增加页面 hero、说明卡片或
自动轮询。

- [ ] **Step 5: 完成按类型 Dialog 和保护规则**

表单类型：

```ts
interface MenuFormState {
  parentId: number | null
  menuType: ManagedMenuType
  name: string
  code: string
  i18nKey: string | null
  path: string | null
  componentPath: string | null
  icon: MenuIconName | null
  sortOrder: number
  isEnabled: YesNo
  isHidden: YesNo
}
```

directory/page 保证 i18nKey string；action 设置 i18nKey/path/componentPath/icon 为 null 且
isHidden=1。编辑 code 只读。protected 节点只允许 name/i18nKey/icon/sortOrder；action 只允许
name/sortOrder。前端禁用只是提示，后端错误仍必须展示。

表格使用 border、浅色表头、稳定列宽、树名称列和 fixed right 操作列；权限码、path、
componentPath 使用等宽文本。移动端 Dialog 单列，任何文案不得溢出按钮或控件。

- [ ] **Step 6: 角色授权全部显示数据库 name**

矩阵转换类型使用：

```ts
export interface RolePermissionMatrixAction {
  id: number
  code: string
  name: string
  isEnabled: YesNo
}
```

group/page/action 均直接渲染 name，同时把 code 作为次要信息；不得 `t(node.i18nKey)`。切换
locale 时只有页面控件变化，数据库业务名称保持。

- [ ] **Step 7: 更新中英文文案并运行菜单/角色测试**

新增并保持双语 key 集合一致：

```text
menu.search.placeholder
menu.search.clear
menu.expandAll
menu.collapseAll
menu.form.directoryName
menu.form.pageName
menu.form.actionName
menu.form.protectedHint
menu.form.iconInvalid
```

协议示例必须使用 `/account/users`、`account/users`、`account:user:list` 和
`navigation.accountUsers`，不再展示旧 system 示例。

```powershell
cd D:\admin\web
pnpm vitest run tests/api/menu.contract.test.ts tests/api/role.contract.test.ts tests/views/access/menus tests/views/access/roles tests/i18n/index.test.ts
pnpm build
```

预期：全部通过。

**Owner-controlled commit checkpoint:** `feat: 完善菜单检索与角色授权界面`

---

### Task 7: 执行开发环境切换、清零旧协议并完成全量验收

**Files:**
- Inspect: `server/internal/database/domain_names.go`
- Inspect: `server/internal/module/{user,auth,menu,role,access,authplatform,operationlog}`
- Inspect: `web/src`, `web/tests`, `web/package.json`, `web/pnpm-lock.yaml`
- Inspect: approved Spec and this Plan
- Temporary create/remove: `server/.tmp/cleanup_operation_log_tasks/main.go`
- No persistent maintenance file; the one-time helper must be removed before completion

- [ ] **Step 1: 确认工作区和进程边界**

```powershell
cd D:\admin
git status --short
Get-CimInstance Win32_Process | Where-Object {
  $_.CommandLine -match 'D:\\admin' -and $_.Name -match 'go|node|pnpm'
} | Select-Object ProcessId, Name, CommandLine
```

确认 diff 只包含本计划范围。只停止本次执行启动或用户已明确允许停止的 API/Worker/Web
进程；记录 PID，避免误停其他项目。

- [ ] **Step 2: 备份开发 PostgreSQL**

在当前 PowerShell 会话设置 `POSTGRES_DSN`，不得输出其值：

```powershell
$taskPgDsn = ($env:POSTGRES_DSN -replace '(?i)(^|\s)TimeZone=\S+', '$1').Trim()
$taskBackup = Join-Path $env:TEMP ("admin-before-domain-rename-{0}.dump" -f (Get-Date -Format 'yyyyMMdd-HHmmss'))
pg_dump --format=custom --file=$taskBackup --dbname=$taskPgDsn
Get-Item -LiteralPath $taskBackup | Select-Object FullName, Length, LastWriteTime
```

`taskPgDsn` 只去掉 GORM 专用、libpq 不识别的 `TimeZone` 选项，不输出转换后的 DSN。预期：
`pg_dump` 退出码 0，备份文件长度大于 0。若环境没有 `pg_dump`，停止真实数据库迁移并报告
阻塞；不能因为项目未上线就跳过后声称已备份。

- [ ] **Step 3: 运行新 API 一次触发显式迁移**

先执行后端构建，再在当前专用 PowerShell 前台启动 API；启动日志必须越过 foundation 并开始
监听：

```powershell
cd D:\admin\server
go build ./...
go run ./cmd/api
```

记录开始监听的日志后，用 `Ctrl+C` 停止本次 `go run`，确认该命令及其子进程退出，再进入
Step 4。`go run ./cmd/api` 是观察型阻塞命令，不能与后续 Redis/Asynq 清理并行，也不能把
仍在运行的 API 当作该步骤完成。

若迁移失败，保留事务错误和数据库现场，不手工创建缺失表或兼容视图。修复代码/fixture 后
从对应 Task 的失败测试重新执行。

- [ ] **Step 4: 定向清理 authz 缓存**

API/Worker 停止后，只删除两个模式：

```powershell
$taskPatterns = @('authz:access-state:*', 'authz:access:*')
foreach ($taskPattern in $taskPatterns) {
  $taskKeys = @(redis-cli -u $env:REDIS_URL --scan --pattern $taskPattern)
  foreach ($taskKey in $taskKeys) {
    if ($taskKey -notlike $taskPattern) { throw "unexpected Redis key: $taskKey" }
    redis-cli -u $env:REDIS_URL DEL $taskKey | Out-Null
  }
}
```

禁止 `FLUSHDB`，禁止删除 `auth:*` 会话 key。

- [ ] **Step 5: 检查并清除旧 operation-log task type**

用 `apply_patch` 创建一次性 `server/.tmp/cleanup_operation_log_tasks/main.go`：

```go
package main

import (
	"fmt"
	"os"

	"github.com/hibiken/asynq"
)

const (
	legacyType = "system:operation-log:v2"
	pageSize   = 100
)

type taskLister func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error)

type queueState struct {
	name string
	list taskLister
}

func main() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		fatalf("REDIS_URL is required")
	}
	redisOptions, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		fatalf("parse REDIS_URL: %v", err)
	}
	inspector := asynq.NewInspector(redisOptions)
	defer func() {
		if err := inspector.Close(); err != nil {
			fatalf("close inspector: %v", err)
		}
	}()

	queues, err := inspector.Queues()
	if err != nil {
		fatalf("list queues: %v", err)
	}
	deletable := []queueState{
		{name: "pending", list: inspector.ListPendingTasks},
		{name: "scheduled", list: inspector.ListScheduledTasks},
		{name: "retry", list: inspector.ListRetryTasks},
		{name: "archived", list: inspector.ListArchivedTasks},
		{name: "completed", list: inspector.ListCompletedTasks},
	}

	for _, queueName := range queues {
		activeCount, err := countLegacy(inspector.ListActiveTasks, queueName)
		if err != nil {
			fatalf("scan active queue %s: %v", queueName, err)
		}
		if activeCount != 0 {
			fatalf("queue %s still has %d active %s tasks", queueName, activeCount, legacyType)
		}
		for _, state := range deletable {
			deleted, err := deleteLegacy(inspector, state.list, queueName)
			if err != nil {
				fatalf("clean %s queue %s: %v", state.name, queueName, err)
			}
			fmt.Printf("queue=%s state=%s deleted=%d\n", queueName, state.name, deleted)
		}
	}

	for _, queueName := range queues {
		states := append([]queueState{{name: "active", list: inspector.ListActiveTasks}}, deletable...)
		for _, state := range states {
			count, err := countLegacy(state.list, queueName)
			if err != nil {
				fatalf("verify %s queue %s: %v", state.name, queueName, err)
			}
			if count != 0 {
				fatalf("queue %s state %s still has %d %s tasks", queueName, state.name, count, legacyType)
			}
		}
	}
}

func deleteLegacy(inspector *asynq.Inspector, list taskLister, queueName string) (int, error) {
	deleted := 0
	for page := 1; ; {
		tasks, err := list(queueName, asynq.Page(page), asynq.PageSize(pageSize))
		if err != nil {
			return 0, err
		}
		deletedOnPage := 0
		for _, task := range tasks {
			if task.Type != legacyType {
				continue
			}
			if err := inspector.DeleteTask(queueName, task.ID); err != nil {
				return 0, fmt.Errorf("delete task %s: %w", task.ID, err)
			}
			deleted++
			deletedOnPage++
		}
		if deletedOnPage != 0 {
			continue
		}
		if len(tasks) < pageSize {
			return deleted, nil
		}
		page++
	}
}

func countLegacy(list taskLister, queueName string) (int, error) {
	count := 0
	for page := 1; ; page++ {
		tasks, err := list(queueName, asynq.Page(page), asynq.PageSize(pageSize))
		if err != nil {
			return 0, err
		}
		for _, task := range tasks {
			if task.Type == legacyType {
				count++
			}
		}
		if len(tasks) < pageSize {
			return count, nil
		}
	}
}

func fatalf(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
```

API/Worker 必须保持停止。运行并删除明确临时目录：

```powershell
cd D:\admin\server
go run .\.tmp\cleanup_operation_log_tasks
Remove-Item -LiteralPath 'D:\admin\server\.tmp\cleanup_operation_log_tasks\main.go'
Remove-Item -LiteralPath 'D:\admin\server\.tmp\cleanup_operation_log_tasks'
if ((Get-ChildItem -LiteralPath 'D:\admin\server\.tmp' -Force | Measure-Object).Count -eq 0) {
  Remove-Item -LiteralPath 'D:\admin\server\.tmp'
}
```

预期：程序输出每个 queue/state 的删除计数并退出 0，二次扫描确认 active、pending、scheduled、
retry、archived、completed 中旧 type 均为 0。只逐 ID 删除 `system:operation-log:v2`，禁止 purge
queue。最终 `rg` 不得在运行时代码残留旧 task type 或 cleanup 分支。

- [ ] **Step 6: 查询真实 PostgreSQL 迁移结果**

```sql
SELECT tablename
FROM pg_tables
WHERE schemaname = current_schema()
  AND tablename IN (
    'user_account','auth_session','rbac_menu','rbac_role','rbac_user_role',
    'rbac_role_menu','rbac_access_version','auth_platform','audit_operation_log'
  )
ORDER BY tablename;

SELECT id, parent_id, menu_type, name, code, i18n_key, path, component_path,
       icon, sort_order, is_enabled, is_hidden, deleted_at
FROM rbac_menu
ORDER BY sort_order, code, id;

SELECT role_id, menu_id, deleted_at
FROM rbac_role_menu
ORDER BY role_id, menu_id, id;

SELECT object_type, object_name
FROM (
  SELECT 'relation' AS object_type, class.relname AS object_name
  FROM pg_class AS class
  JOIN pg_namespace AS namespace ON namespace.oid = class.relnamespace
  WHERE namespace.nspname = current_schema()
    AND class.relkind IN ('r', 'p', 'i', 'I', 'S')
  UNION ALL
  SELECT 'constraint' AS object_type, constraint_row.conname AS object_name
  FROM pg_constraint AS constraint_row
  WHERE constraint_row.connamespace = current_schema()::regnamespace
) AS objects
WHERE object_name LIKE '%sys\_%' ESCAPE '\'
ORDER BY object_type, object_name;
```

使用 `psql --dbname=$taskPgDsn -v ON_ERROR_STOP=1` 执行以上 SQL。预期第一条精确返回九行，
catalog 查询返回零行；三个 root 和六个 page 精确匹配 Spec，role-menu menu_id 仍指向原 ID，
只有与错误软删除时间匹配的菜单管理授权恢复为有效。

- [ ] **Step 7: 扫描旧协议和禁止写法**

```powershell
cd D:\admin
rg -n "sys_(user|user_session|menu|role|user_role|role_menu|access_version|auth_platform|operation_log)" server web
rg -n "system:(user|session|menu|role|auth-platform|operation-log):" server web
rg -n '"/system/(users|sessions|menus|roles|auth-platforms)' web
rg -n "system/(users|sessions|menus|roles|auth-platforms)" web
rg -n "system:operation-log:v2|mdi:|@iconify/vue" server web
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" web\src web\tests -g "*.ts" -g "*.vue"
```

旧值只允许出现在 `database/domain_names.go`、menu 一次性 legacy migration、迁移测试和本
计划/Spec 的历史映射说明中。除这两个显式启动迁移外，普通请求运行时代码、当前 fixture、
前端页面和 package dependency 不得残留。

- [ ] **Step 8: 运行后端完整验证**

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

预期：四条命令全部退出 0。PostgreSQL/Redis 环境错误必须单独报告，不能跳过后宣称通过。

- [ ] **Step 9: 运行前端完整验证**

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

预期：Vitest、vue-tsc 和 Vite build 全部成功。warning 单独记录。

- [ ] **Step 10: 启动本地服务并进行双角色浏览器验收**

使用空闲端口启动 API、Worker 和 Vite；Windows 后台进程使用隐藏窗口并记录 PID。至少验收：

```text
超级管理员：
- 三个根目录顺序、名称、Lucide 图标正确
- 六个页面位于正确根目录
- 菜单页默认只展开 roots，可搜索 name/code/path
- protected 节点只能修改展示字段
- 角色矩阵显示数据库 name + code

普通角色：
- 无 list 权限时侧栏不可见、API 403
- list 与 action 权限独立控制页面和按钮
- 隐藏 page 不在侧栏，但有权限时新路径可直接访问

路由：
- /access/menus 精确复用静态页面
- 旧 /system/users、/system/menus 等不跳转
- breadcrumb、RouteTabs、刷新和直接访问一致
```

完成后向用户提供实际 Vite URL。不得停止用户原先启动的其他服务；本次启动的后台进程在
交付中列出 PID 和是否仍运行。

- [ ] **Step 11: 最终差异和文档一致性检查**

```powershell
cd D:\admin
git diff --check
git status --short
git diff --stat
git diff -- docs/superpowers/specs/2026-08-24-menu-protocol-dynamic-page-design.md docs/superpowers/plans/2026-08-25-menu-protocol-dynamic-page.md
```

确认没有 `.env`、密钥、备份 dump、`web/dist`、日志、测试缓存、临时 cleanup helper 或截图
进入工作区；未经授权没有提交或重写历史。

## 实施完成标准

- 九张表和相关 PostgreSQL 对象全部使用 account/auth/rbac/audit 真实业务域名称。
- Model、Repository SQL、权限常量、route、DTO、页面目录、componentPath、i18n 和测试可顺着同一映射查找。
- 三个根目录和六个当前页面使用新 code/path/componentPath，旧协议没有运行时兼容。
- 菜单和角色授权 ID 保持，`rbac_menu`/`rbac_role_menu` 是唯一事实来源。
- 超级管理员权限只来自有效数据库 page/action。
- 菜单管理拥有 name/i18nKey 分离、protected foundation、搜索、稳定展开和完整 Dialog。
- 菜单图标完全本地、统一为 Lucide，不依赖 Iconify API。
- operation log Worker 只处理 `audit:operation-log:v2`。
- 后端 fmt/vet/test/build、前端 Vitest/build、真实 PostgreSQL 检查和双角色浏览器验收全部完成。
- Git 历史仍由仓库所有者控制。
