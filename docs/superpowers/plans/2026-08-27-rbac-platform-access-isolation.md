# RBAC 多平台菜单归属与 Access 隔离 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使用 `rbac_menu.platform_id`、运行时 `Identity.PlatformID` 和平台过滤的 Access 权限源，建立可验证的 Admin/Canvas 菜单与权限隔离。

**Architecture:** 认证 Service 从已加载的认证 Policy 把平台 ID 写入运行时 Identity，JWT 协议保持平台 code 不变。菜单表通过认证平台外键、父子组合外键和平台内部分唯一索引表达归属；Access Repository 直接按 `PlatformID` 过滤，角色仍为全局资源并通过菜单 ID 跨平台授权。

**Tech Stack:** Go、Gin、GORM、PostgreSQL、Redis、Vue 3、TypeScript、Pinia、Element Plus、Vitest、pnpm

**Spec:** `docs/superpowers/specs/2026-08-27-rbac-platform-access-isolation-design.md`

> 页面入口权限统一遵守项目 Agent 规则：所有 `menuType=page` 使用 `:view`，包括 Canvas 根页；
> 本历史计划中的 Canvas code 已统一为 `canvas:test:view`。API/Worker 不负责 foundation 或
> seed，数据由维护者 migration/测试 fixture 准备。

## Global Constraints

- `rbac_menu` 只存 `platform_id BIGINT NOT NULL`，不冗余平台 code。
- `platform_id` 创建后不可修改；更新菜单请求不接受 `platformId`。
- 现有有效和历史菜单全部回填为内置 `admin` 平台 ID，菜单 ID 和角色授权关系保持。
- 根节点允许目录或页面，根操作禁止；页面可直接包含 action。
- 父子菜单必须同平台，并由 `(parent_id, platform_id)` 组合外键保证。
- 用户和角色保持全局；`rbac_role_menu`、`rbac_user_role`、`rbac_access_version` 不增加平台字段。
- `/api/v1/access` 只返回 `identity.PlatformID` 对应平台的菜单和权限，包括超级管理员。
- Canvas 只保留 `canvas:test:view` 根 page 和 `canvas:test:button` action 的隔离测试数据；不在
  API/Worker 启动时创建认证平台、菜单 foundation 或 seed。
- Access Version 继续按用户维护，菜单或角色授权变更使用现有全局失效流程。
- JWT 继续携带平台 code，不增加 `platformId` claim；Platform ID 只存在于认证后的运行时 Identity。
- Admin 管理接口继续使用 `/api/admin/v1`，Access 继续使用 `/api/v1`。
- 不实现 COS、邮件、短信、支付、AI、WebSocket 或 Canvas 业务前端。
- TypeScript 不使用显式 `any`、类型断言兜底或必填字段静默默认值。
- 每个行为变化执行失败测试、确认目标失败、最小实现、通过、再重构。
- 下列 Commit 步骤只在维护者再次明确授权提交时执行；未授权时跳过并保留工作区改动。

---

## File Map

### Runtime identity and Access

- `server/internal/module/auth/service.go`：增加运行时 `Identity.PlatformID` 并从 Policy 填充。
- `server/internal/module/auth/middleware_test.go`、`service_test.go`：锁定 Platform ID 传播，不改变 JWT。
- `server/internal/module/access/repository.go`：按平台 ID 读取菜单和角色授权。
- `server/internal/module/access/service.go`：把 Platform ID 传入权限源并允许根级页面。
- `server/internal/module/access/*_test.go`：Admin/Canvas、多角色、超级管理员和缓存隔离证据。

### Menu database and management

- `server/internal/module/menu/model.go`：增加 `PlatformID int64`。
- `server/internal/module/menu/migration.go`、`schema.go`：显式回填、约束和索引迁移。
- `server/internal/module/menu/tree.go`：平台内校验和根级页面规则。
- `server/internal/module/menu/repository.go`：平台目录、列表和写入。
- `server/internal/module/menu/service.go`、`request.go`、`response.go`、`handler.go`：管理 DTO 和平台不可变规则。
- `server/internal/module/menu/*_test.go`：真实 PostgreSQL Schema、迁移、Service 和 HTTP 契约。

### Authentication-platform deletion guard

- `server/internal/module/authplatform/repository.go`：查询平台的有效菜单引用。
- `server/internal/module/authplatform/service.go`、测试和 i18n：有效菜单存在时拒绝软删除平台。

### Role permissions

- `server/internal/module/role/permission.go`、`response.go`：按平台返回权限树并接受根级页面。
- `server/internal/module/role/repository.go`、`service.go`：读取平台元数据并校验跨平台授权集合。
- `server/internal/module/role/*_test.go`：多平台权限树、保存和 Access 失效。

### Frontend

- `web/src/api/menu.ts`、`role.ts`：闭合平台 DTO。
- `../../../web/src/views/permission/menus/index.vue`：顶部平台 Tabs、当前平台创建、编辑只读和根级页面。
- `../../../web/src/views/permission/roles/role-permission-matrix.ts`：平台分组和根级页面矩阵。
- `../../../web/src/views/permission/roles/index.vue`、components：平台分段授权 UI。
- `web/src/i18n/messages/zh-CN.ts`、`en-US.ts`：平台与约束文案。

### Composition and documentation

- `server/cmd/api/main.go`、测试：先建立 Admin/Canvas 认证平台预设，在通用 AutoMigrate 前回填旧菜单平台列，再分别建立 Admin/Canvas 菜单 foundation。
- `docs/superpowers/specs/2026-08-27-rbac-platform-access-isolation-design.md`：实施后更新状态。

---

### Task 1: 将认证 Policy ID 传播到运行时 Identity

**Files:**
- Modify: `server/internal/module/auth/service.go`
- Modify: `server/internal/module/auth/service_test.go`
- Modify: `server/internal/module/auth/middleware_test.go`
- Modify: `server/internal/module/access/handler_test.go`
- Modify: `server/internal/module/access/middleware_test.go`

**Interfaces:**
- Produces: `auth.Identity.PlatformID int64`
- Consumes: `authplatform.Policy.ID`
- Preserves: `TokenIdentity{UserID, SessionID, Platform, Version}` and JWT claims

- [ ] **Step 1: Add failing authentication tests for runtime Platform ID**

Extend the warm-cache and PostgreSQL-fallback authentication assertions:

```go
if identity.PlatformID != policy.ID || identity.Platform != policy.Code {
	t.Fatalf("runtime platform identity = %+v, policy = %+v", identity, policy)
}
```

Add a middleware fixture with `PlatformID: 17` and assert the exact Identity survives `context.Set`/`IdentityFromContext`.

- [ ] **Step 2: Run focused tests and confirm the field is missing**

```powershell
cd D:\admin\server
go test ./internal/module/auth ./internal/module/access -run 'Test(AuthenticateUsesWarmRedisWithoutPostgreSQL|AuthenticateFallsBackToPostgreSQLAndRebuildsRedis|AuthenticateMiddlewareStoresIdentity|AccessHandlerReturnsClosedSnapshot)' -count=1
```

Expected: FAIL because `Identity.PlatformID` does not exist or is zero.

- [ ] **Step 3: Add Platform ID only to runtime Identity**

Implement:

```go
type Identity struct {
	UserID          int64
	SessionID       int64
	PlatformID      int64
	Platform        string
	Version         int64
	PolicyVersion   int64
	AccessCacheTTL  time.Duration
	CacheResult     string
}

func identityFromAuthority(authority SessionAuthority, policy authplatform.Policy, cacheResult string) Identity {
	return Identity{
		UserID: authority.UserID, SessionID: authority.Session.ID,
		PlatformID: policy.ID, Platform: authority.Session.Platform,
		Version: authority.Session.Version, PolicyVersion: policy.PolicyVersion,
		AccessCacheTTL: policy.AccessCacheTTL, CacheResult: cacheResult,
	}
}
```

Update runtime Identity test fixtures. Do not add Platform ID to JWT claims, session rows, cookies or public authentication responses.

- [ ] **Step 4: Validate complete runtime identities**

Require `PlatformID >= 1` wherever a fully authenticated `Identity` is validated for Access. Tests that intentionally exercise missing identity must continue to fail with the existing public error.

- [ ] **Step 5: Run focused authentication and Access tests**

```powershell
cd D:\admin\server
go test ./internal/module/auth ./internal/module/access -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit only if explicitly authorized**

```powershell
git add server/internal/module/auth/service.go server/internal/module/auth/service_test.go server/internal/module/auth/middleware_test.go server/internal/module/access/handler_test.go server/internal/module/access/middleware_test.go
git commit -m "refactor: 传播认证平台运行时标识"
```

### Task 2: 建立 menu.platform_id 的 PostgreSQL 迁移与约束

**Files:**
- Modify: `server/internal/module/menu/model.go`
- Modify: `server/internal/module/menu/migration.go`
- Modify: `server/internal/module/menu/schema.go`
- Modify: `server/internal/module/menu/schema_test.go`
- Modify: `server/internal/module/menu/schema_migration_test.go`
- Modify: `server/internal/module/menu/test_database_test.go`
- Modify: `server/cmd/api/main.go`

**Interfaces:**
- Produces: `menu.Menu.PlatformID int64`
- Produces: `fk_rbac_menu_platform`, `uq_rbac_menu_id_platform`, `fk_rbac_menu_parent_platform`
- Produces: platform-scoped active code/path and tree indexes
- Consumes: built-in `auth_platform.code = 'admin'` created by `authplatform.EnsureSchema`

- [ ] **Step 1: Write failing real-PostgreSQL schema assertions**

Require:

```text
rbac_menu.platform_id BIGINT NOT NULL
fk_rbac_menu_platform -> auth_platform(id) ON DELETE RESTRICT
uq_rbac_menu_id_platform UNIQUE(id, platform_id)
fk_rbac_menu_parent_platform FOREIGN KEY(parent_id, platform_id)
```

Assert the old single-column `fk_rbac_menu_parent`, global code/path indexes and old parent index are absent. Assert the new definitions contain:

```text
(platform_id, code) WHERE deleted_at IS NULL
(platform_id, path) WHERE deleted_at IS NULL AND menu_type = 'page'
(platform_id, parent_id, sort_order, id) WHERE deleted_at IS NULL
```

- [ ] **Step 2: Write a migration preservation test**

Create an old-shape database with active and soft-deleted menus plus role grants, run auth-platform Schema then menu migration/Schema, and assert:

```go
if active.PlatformID != admin.ID || deleted.PlatformID != admin.ID {
	t.Fatalf("historical menus were not assigned to admin")
}
if grant.MenuID != originalMenuID {
	t.Fatalf("role grant changed during migration")
}
```

Run migration twice and require the second run to be a no-op.

- [ ] **Step 3: Confirm the schema tests fail for the missing field and constraints**

```powershell
cd D:\admin\server
go test ./internal/module/menu -run 'Test(MenuSchema|MenuSchemaMigration)' -count=1
```

Expected: FAIL because `platform_id` and new constraints/indexes do not exist.

- [ ] **Step 4: Add the model field and explicit migration order**

Add:

```go
PlatformID int64 `gorm:"column:platform_id;not null"`
```

Ensure startup ordering is:

```text
prepare old menu names
-> AutoMigrate authplatform.Platform only
-> authplatform.EnsureSchema creates/validates Admin
-> menu.PreparePlatformSchema backfills an existing rbac_menu before NOT NULL mapping
-> AutoMigrate remaining models including menu.Menu
-> menu.EnsureSchema replaces constraints and indexes
```

`PreparePlatformSchema` must no-op when `rbac_menu` does not yet exist. For an old table it locks `auth_platform` and `rbac_menu`, requires exactly one active built-in Admin platform, adds `platform_id` nullable, backfills all active and deleted menu rows, then sets `NOT NULL` in one transaction. This prevents GORM from attempting to add a non-null column to populated history before backfill.

- [ ] **Step 5: Replace constraints and indexes**

Use explicit idempotent DDL:

```sql
ALTER TABLE rbac_menu
  ADD CONSTRAINT fk_rbac_menu_platform
  FOREIGN KEY (platform_id) REFERENCES auth_platform(id) ON DELETE RESTRICT;

ALTER TABLE rbac_menu
  ADD CONSTRAINT uq_rbac_menu_id_platform UNIQUE (id, platform_id);

ALTER TABLE rbac_menu
  ADD CONSTRAINT fk_rbac_menu_parent_platform
  FOREIGN KEY (parent_id, platform_id)
  REFERENCES rbac_menu(id, platform_id) ON DELETE RESTRICT;
```

Drop the exact obsolete indexes before creating the platform-scoped replacements. Map the retained conflict constraint names in Repository error handling.

- [ ] **Step 6: Prove the database rejects cross-platform parentage**

In a rollback transaction, create Admin and Canvas root pages and attempt to insert a Canvas child using the Admin parent ID. Require PostgreSQL constraint `fk_rbac_menu_parent_platform` to reject it.

- [ ] **Step 7: Run menu schema and startup tests**

```powershell
cd D:\admin\server
go test ./internal/module/menu ./cmd/api -run 'Test(MenuSchema|MenuSchemaMigration|Run)' -count=1 -p=1
```

Expected: PASS.

- [ ] **Step 8: Commit only if explicitly authorized**

```powershell
git add server/internal/module/menu/model.go server/internal/module/menu/migration.go server/internal/module/menu/schema.go server/internal/module/menu/schema_test.go server/internal/module/menu/schema_migration_test.go server/internal/module/menu/test_database_test.go server/cmd/api/main.go
git commit -m "feat: 建立菜单平台外键与迁移"
```

### Task 3: 使菜单领域支持平台目录和根级页面

**Files:**
- Modify: `server/internal/module/menu/tree.go`
- Modify: `server/internal/module/menu/tree_test.go`
- Modify: `server/internal/module/menu/repository.go`
- Modify: `server/internal/module/menu/repository_test.go`
- Modify: `server/internal/module/menu/foundation.go`
- Modify: `server/internal/module/menu/foundation_test.go`

**Interfaces:**
- Produces: `menu.PlatformOption{ID, Code, Name, IsEnabled}`
- Produces: `Repository.FindPlatformOptions(context.Context) ([]PlatformOption, error)`
- Produces: `Repository.FindPlatform(context.Context, int64) (PlatformOption, error)`
- Produces: platform-aware `buildMenuIndex` and root page acceptance
- Consumes: `Menu.PlatformID` from Task 2

- [ ] **Step 1: Write failing tree tests for Canvas root page**

Create `canvas:test:view` as a root page and `canvas:test:button` as its action. Require `buildMenuIndex`, `buildManagedTree` and enabled-ancestor validation to pass. Add negative cases for root action, cross-platform parent, and duplicate code/path inside one platform; add positive duplicate code/path across two platforms.

- [ ] **Step 2: Run focused tree tests**

```powershell
cd D:\admin\server
go test ./internal/module/menu -run 'Test.*(RootPage|Platform|Duplicate)' -count=1
```

Expected: FAIL because roots must currently be directories and uniqueness is global.

- [ ] **Step 3: Make tree identity platform-aware**

Index uniqueness with a typed key:

```go
type platformValueKey struct {
	PlatformID int64
	Value      string
}
```

Require `PlatformID >= 1`, allow root directory/page, reject root action, and verify `parent.PlatformID == child.PlatformID` before recording the edge.

- [ ] **Step 4: Add exact platform repository reads**

Implement platform option queries against active and non-deleted `auth_platform`. `FindPlatform` must return disabled platforms for Admin cleanup but never deleted rows. `FindActiveMenus` and lock methods retain all platforms unless an explicit list filter is supplied.

- [ ] **Step 5: Make Admin foundation resolve and preserve Admin Platform ID**

`EnsureFoundation` must resolve `authplatform.BuiltinAdminCode`, assign its ID to every foundation candidate, include Platform ID in historical code locks, and never claim or rewrite a same-code menu from another platform.

- [ ] **Step 6: Run menu domain and foundation tests**

```powershell
cd D:\admin\server
go test ./internal/module/menu -run 'Test(BuildMenu|Repository|EnsureFoundation)' -count=1 -p=1
```

Expected: PASS.

- [ ] **Step 7: Commit only if explicitly authorized**

```powershell
git add server/internal/module/menu/tree.go server/internal/module/menu/tree_test.go server/internal/module/menu/repository.go server/internal/module/menu/repository_test.go server/internal/module/menu/foundation.go server/internal/module/menu/foundation_test.go
git commit -m "feat: 支持多平台菜单树与根级页面"
```

### Task 4: 在 Access 权限源实施平台隔离

**Files:**
- Modify: `server/internal/module/access/repository.go`
- Modify: `server/internal/module/access/repository_test.go`
- Modify: `server/internal/module/access/service.go`
- Modify: `server/internal/module/access/service_test.go`
- Modify: `server/internal/module/access/redis.go`
- Modify: `server/internal/module/access/redis_test.go`

**Interfaces:**
- Produces: `FindSourceWithVersion(context.Context, userID, platformID int64) (Source, error)`
- Consumes: `auth.Identity.PlatformID`
- Preserves: public `/api/v1/access` response fields and cache key platform code

- [ ] **Step 1: Write a PostgreSQL fixture with Admin and Canvas menus**

Inside a rollback transaction, create Canvas platform plus:

```text
    canvas:test:view       page   root
    canvas:test:button     action parent=canvas:test:view
```

Create one user with two roles, grant Admin menus through one role and Canvas test/action through the other. Test Repository twice with Admin and Canvas Platform IDs.

- [ ] **Step 2: Assert platform isolation fails before implementation**

Require Admin source to exclude Canvas IDs and Canvas source to exclude Admin IDs. Require Canvas grants to be deduplicated across roles. Run:

```powershell
cd D:\admin\server
go test ./internal/module/access -run 'TestRepository.*Platform' -count=1
```

Expected: FAIL because Repository currently reads all menus and has no platform parameter.

- [ ] **Step 3: Filter every menu source query by Platform ID**

Change the source-store contract and queries:

```go
type sourceStore interface {
	FindSourceWithVersion(context.Context, int64, int64) (Source, error)
}
```

Both the all-menu query and direct-grant query must use the same positive Platform ID. Role codes remain global and deduplicated.

- [ ] **Step 4: Allow root pages in Access snapshot validation**

Change only the root rule:

```go
if item.ParentID == nil && item.MenuType == MenuAction {
	return fmt.Errorf("root menu %d is an action", id)
}
```

Keep page shape, action shape, ancestor completion, cycle detection and permission sorting strict.

- [ ] **Step 5: Lock super-admin isolation and multi-role merging**

Tests must prove super-admin Admin identity gets all Admin pages/actions but no Canvas permissions, while the same user with Canvas identity gets only Canvas page/action. A normal user with multiple roles receives a sorted unique union for the requested platform.

- [ ] **Step 6: Validate Platform ID in cached snapshots without changing public DTO**

Add `PlatformID` to the internal cached snapshot, validate it against the runtime identity on reads, and bump the internal access snapshot schema version. Keep Redis key human-readable by platform code and include policy/user version as before.

- [ ] **Step 7: Run complete Access tests**

```powershell
cd D:\admin\server
go test ./internal/module/access -count=1 -p=1
```

Expected: PASS.

- [ ] **Step 8: Commit only if explicitly authorized**

```powershell
git add server/internal/module/access/repository.go server/internal/module/access/repository_test.go server/internal/module/access/service.go server/internal/module/access/service_test.go server/internal/module/access/redis.go server/internal/module/access/redis_test.go
git commit -m "feat: 隔离不同平台的访问权限快照"
```

### Task 5: 闭合菜单管理 API 的平台协议

**Files:**
- Modify: `server/internal/module/menu/service.go`
- Modify: `server/internal/module/menu/request.go`
- Modify: `server/internal/module/menu/response.go`
- Modify: `server/internal/module/menu/handler.go`
- Modify: `server/internal/module/menu/service_test.go`
- Modify: `server/internal/module/menu/handler_test.go`
- Modify: `server/internal/shared/i18n/keys.go`
- Modify: `server/internal/shared/i18n/catalog.go`

**Interfaces:**
- Produces: `ListQuery{PlatformID *int64}`
- Produces: `Catalog{Platforms []PlatformOption; MenuTree []ManagedMenu}`
- Produces: `CreateInput.PlatformID int64`
- Preserves: `UpdateInput` without Platform ID

- [ ] **Step 1: Write failing strict HTTP contract tests**

Require `GET /api/admin/v1/menus?platformId=2` to pass one filter and return:

```json
{
  "platforms":[{"id":1,"code":"admin","name":"Admin","isEnabled":1}],
  "menuTree":[]
}
```

Require create JSON to contain `platformId`; missing, zero, repeated query values and unknown fields return 400. Require update JSON containing `platformId` to return 400.

- [ ] **Step 2: Run Handler tests and confirm contract failure**

```powershell
cd D:\admin\server
go test ./internal/module/menu -run 'TestHandler.*(List|Create|Update|Platform)' -count=1
```

Expected: FAIL because list returns a bare tree and create has no platform field.

- [ ] **Step 3: Implement list/catalog DTOs and query parsing**

Handler parses only optional positive `platformId`. Service loads platform options, rejects an unknown filter, reads the selected menu rows, and returns nodes carrying `platformId`, `platformCode` and `platformName`.

- [ ] **Step 4: Enforce platform rules in create and edit transactions**

Inside the menu mutation transaction, Create locks the target `auth_platform` row with a lock mode that conflicts with platform deletion, confirms it is not deleted, then validates root page/directory shape, parent type and parent Platform ID. Update retains the stored Platform ID, validates any new parent belongs to it, and never writes `platform_id`.

Conflict checks use `(PlatformID, Code)` and `(PlatformID, Path)`, including repository constraint mappings.

- [ ] **Step 5: Preserve platform-aware invalidation behavior**

Keep the existing conservative global user Access Version advance for menu mutations. Tests must assert no-op edits do not advance and real Canvas/Admin changes do advance active users once.

- [ ] **Step 6: Run complete menu tests**

```powershell
cd D:\admin\server
go test ./internal/module/menu -count=1 -p=1
```

Expected: PASS.

- [ ] **Step 7: Commit only if explicitly authorized**

```powershell
git add server/internal/module/menu server/internal/shared/i18n/keys.go server/internal/shared/i18n/catalog.go
git commit -m "feat: 增加菜单管理平台协议"
```

### Task 6: 保护认证平台删除并按平台组织角色权限

**Files:**
- Modify: `server/internal/module/authplatform/repository.go`
- Modify: `server/internal/module/authplatform/service.go`
- Modify: `server/internal/module/authplatform/service_test.go`
- Modify: `server/internal/module/role/permission.go`
- Modify: `server/internal/module/role/response.go`
- Modify: `server/internal/module/role/repository.go`
- Modify: `server/internal/module/role/service.go`
- Modify: `server/internal/module/role/service_test.go`
- Modify: `server/internal/module/role/handler_test.go`

**Interfaces:**
- Produces: `authplatform.Repository.HasActiveMenus(context.Context, platformID int64) (bool, error)`
- Produces: `role.PermissionPlatform{ID, Code, Name, IsEnabled, MenuTree}`
- Produces: `Permissions.Platforms []PermissionPlatform`
- Preserves: role permission save body `{menuIds: number[]}`

- [ ] **Step 1: Write failing platform deletion protection test**

Create a non-built-in platform with one active menu and require `Service.Delete` to return a stable conflict without revoking/deleting the platform. Soft-delete the menu and require deletion to proceed through the existing session invalidation flow.

- [ ] **Step 2: Implement the repository check and Service guard**

Inside the platform mutation transaction, lock the `auth_platform` row for update and then count `rbac_menu` rows with matching Platform ID and `deleted_at IS NULL`. Return a translated conflict when any remain. Menu creation must acquire a conflicting lock on the same row before insertion, so create/delete races serialize and recheck committed facts. Do not cascade menu deletion.

- [ ] **Step 3: Write failing role permission tests for platform groups**

Build Admin and Canvas menus, including the Canvas root page. Require:

```go
permissions.Platforms[0].Code == "admin"
permissions.Platforms[1].Code == "canvas"
```

Each platform tree must contain only its own nodes. Existing direct grants remain one sorted `MenuIDs` array. Saving mixed Admin/Canvas IDs is allowed because roles are global; directory IDs, missing IDs and root action IDs remain invalid.

- [ ] **Step 4: Make the permission index platform-aware**

Allow root directory/page, reject root action, validate parent Platform ID, scope duplicate code checks by Platform ID, and build one tree per platform. Platform metadata comes from non-deleted auth platforms and includes disabled platforms for Admin cleanup.

- [ ] **Step 5: Return a closed platform-grouped permission response**

Replace top-level `menuTree` with:

```json
"platforms": [{
  "id": 1,
  "code": "admin",
  "name": "Admin",
  "isEnabled": 1,
  "menuTree": []
}]
```

Keep `role` and `menuIds`. Exact-key Handler tests must reject missing platform fields.

- [ ] **Step 6: Run auth-platform and role tests**

```powershell
cd D:\admin\server
go test ./internal/module/authplatform ./internal/module/role -count=1 -p=1
```

Expected: PASS.

- [ ] **Step 7: Commit only if explicitly authorized**

```powershell
git add server/internal/module/authplatform server/internal/module/role
git commit -m "feat: 按平台组织角色菜单授权"
```

### Task 7: 在菜单管理页面接入平台维度和根级页面

**Files:**
- Modify: `web/src/api/menu.ts`
- Modify: `web/src/api/menu.test.ts`
- Modify: `../../../web/src/views/permission/menus/index.vue`
- Modify: `../../../web/src/views/permission/menus/index.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

**Interfaces:**
- Consumes: `MenuCatalogResponse{platforms, menuTree}` from Task 5
- Produces: exact TypeScript DTOs with `platformId`, `platformCode`, `platformName`

- [ ] **Step 1: Write failing API parser tests**

Require closed parsing of the catalog and reject missing `platformId`, invalid platform option status, unknown keys and a bare-array legacy response. Require `getMenus({platformId: 2})` to send exactly one query parameter.

- [ ] **Step 2: Run API tests and confirm the old DTO fails**

```powershell
cd D:\admin\web
pnpm vitest run src/api/menu.test.ts --pool=threads --maxWorkers=1
```

Expected: FAIL because current API accepts a bare menu array and has no platform DTO.

- [ ] **Step 3: Implement strict DTOs and API calls**

Use:

```ts
export interface MenuPlatformOption {
  id: number
  code: string
  name: string
  isEnabled: YesNo
}

export interface MenuCatalogResponse {
  platforms: MenuPlatformOption[]
  menuTree: ManagedMenuNode[]
}
```

`CreateMenuInput` includes `platformId`; `UpdateMenuInput` does not.

- [ ] **Step 4: Write failing page behavior tests**

Cover top-level platform Tabs, switching Tabs to reload the selected platform, creating a root page with `parentId: null` under the active platform, filtering parent choices to the active platform, showing platform read-only during edit, and never placing `platformId` in update JSON.

- [ ] **Step 5: Implement the menu UI**

Add `el-tabs` as the first control above Search, with one Tab per non-deleted platform and built-in Admin selected by default. The active Tab is the only platform context: switching it reloads that platform tree, and Create sends its Platform ID without a duplicate platform select in the dialog. Show the platform as read-only during edit. Permit page type with no parent; continue forbidding action without a page parent.

- [ ] **Step 6: Run menu frontend tests**

```powershell
cd D:\admin\web
pnpm vitest run src/api/menu.test.ts src/views/access/menus/index.test.ts --pool=threads --maxWorkers=1
```

Expected: PASS.

- [ ] **Step 7: Commit only if explicitly authorized**

```powershell
git add web/src/api/menu.ts web/src/api/menu.test.ts web/src/views/access/menus/index.vue web/src/views/access/menus/index.test.ts web/src/i18n/messages/zh-CN.ts web/src/i18n/messages/en-US.ts
git commit -m "feat: 增加菜单平台筛选与创建"
```

### Task 8: 在角色授权页面按平台呈现权限矩阵

**Files:**
- Modify: `web/src/api/role.ts`
- Modify: `web/src/api/role.test.ts`
- Modify: `../../../web/src/views/permission/roles/role-permission-matrix.ts`
- Modify: `../../../web/src/views/permission/roles/role-permission-matrix.test.ts`
- Modify: `../../../web/src/views/permission/roles/components/RolePermissionMatrix.vue`
- Modify: `../../../web/src/views/permission/roles/components/RolePermissionMatrix.test.ts`
- Modify: `../../../web/src/views/permission/roles/index.vue`
- Modify: `../../../web/src/views/permission/roles/index.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

**Interfaces:**
- Consumes: `RolePermissionsResponse.platforms` from Task 6
- Produces: platform-aware `RoleMatrixGroup`
- Preserves: `updateRolePermissions({menuIds})`

- [ ] **Step 1: Write failing closed DTO tests**

Require every permission platform to have exact `id/code/name/isEnabled/menuTree`, reject legacy top-level `menuTree`, and preserve mixed-platform `menuIds`.

- [ ] **Step 2: Write failing matrix tests for a root Canvas page**

Input one Admin directory tree and one Canvas tree whose root is a page. Require both to produce groups, Canvas to produce one row with its action, and all selection/diff/direct-ID functions to stay globally ID-based and sorted.

- [ ] **Step 3: Run role API and matrix tests**

```powershell
cd D:\admin\web
pnpm vitest run src/api/role.test.ts src/views/access/roles/role-permission-matrix.test.ts --pool=threads --maxWorkers=1
```

Expected: FAIL because current parser expects top-level `menuTree` and matrix rejects root pages.

- [ ] **Step 4: Implement platform-aware matrix construction**

Build groups from each platform. Directory roots retain their current grouping; a root page becomes a platform group with that page as a row. Prefix labels with platform name/code where ambiguity is possible, while selection continues using menu IDs only.

- [ ] **Step 5: Present platforms with top tabs inside the permission dialog**

Use `el-tabs` at the top of the permission editor, matching the old project's established interaction and avoiding nested cards. The matrix below shows only the active platform; “select all” and “clear” operate on that platform, while Save merges selections from every Tab. Disabled platforms remain visible with a status indicator so stale grants can be removed.

- [ ] **Step 6: Run focused role page tests**

```powershell
cd D:\admin\web
pnpm vitest run src/api/role.test.ts src/views/access/roles --pool=threads --maxWorkers=1
```

Expected: PASS.

- [ ] **Step 7: Commit only if explicitly authorized**

```powershell
git add web/src/api/role.ts web/src/api/role.test.ts web/src/views/access/roles web/src/i18n/messages/zh-CN.ts web/src/i18n/messages/en-US.ts
git commit -m "feat: 按平台展示角色权限矩阵"
```

### Task 9: 完整验证、范围审计与文档收口

**Files:**
- Modify: `docs/superpowers/specs/2026-08-27-rbac-platform-access-isolation-design.md`
- Modify: tests or implementation files only when a verification failure proves a defect in this plan

**Interfaces:**
- Consumes: Tasks 1-8
- Produces: verified RBAC platform isolation baseline

- [ ] **Step 1: Format and run complete backend verification**

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test -count=1 -p=1 ./...
go build ./...
```

Expected: all packages pass against real PostgreSQL/Redis.

- [ ] **Step 2: Run complete frontend verification**

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

Expected: all tests and production build pass. Existing chunk-size warnings may remain if unchanged.

- [ ] **Step 3: Run exact architecture and scope audits**

```powershell
cd D:\admin
rg -n "\bplatform\s+string.*rbac_menu|column:platform;|json:\"platform\"" server/internal/module/menu
rg -n "PlatformID" server/internal/module/auth server/internal/module/access server/internal/module/menu
rg -n "canvas" server web -g "*.go" -g "*.ts" -g "*.vue"
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" web/src -g "*.ts" -g "*.vue"
git diff --check
git status --short
```

Expected:

- no menu storage field named string `platform`;
- Platform ID appears only in the approved runtime/menu boundaries;
- Canvas runtime occurrences are limited to the explicit authentication-platform and two-menu startup preset plus test fixtures;
- no AnyScript matches;
- no whitespace errors.

- [ ] **Step 4: Verify database and algorithm invariants from fresh output**

Confirm test output proves:

1. historical Admin IDs and role grants survive migration;
2. PostgreSQL rejects cross-platform parent relationships;
3. Canvas root page/action produces a valid Canvas snapshot;
4. Admin snapshot excludes Canvas for normal and super-admin users;
5. multi-role current-platform grants are merged and deduplicated;
6. Access rebuild consumes `Identity.PlatformID` without an extra platform lookup;
7. active-menu platforms cannot be soft-deleted;
8. the startup seed contains exactly the built-in Canvas platform, root test page and test button, without a Canvas business frontend.

- [ ] **Step 5: Update the spec status after every gate passes**

Change:

```text
状态：已确认，待实施
```

to:

```text
状态：已实施并完成验证
```

Do not update the status when any required command is unrun or failing.

- [ ] **Step 6: Commit only if explicitly authorized**

Because this changes database and Access contracts, use a breaking commit with migration guidance:

```powershell
git add server web docs/superpowers/specs/2026-08-27-rbac-platform-access-isolation-design.md docs/superpowers/plans/2026-08-27-rbac-platform-access-isolation.md
git commit -m "feat!: 建立多平台菜单与权限隔离" -m "rbac_menu 新增 platform_id 并回填为内置 Admin 平台；管理端菜单响应改为平台目录，角色权限响应改为平台分组。现有数据库由启动迁移保留菜单 ID 和角色授权，客户端需适配 platformId、platforms 和 menuTree 新契约。"
```

## Completion Criteria

- [ ] `rbac_menu.platform_id`、平台外键、父子组合外键和平台内唯一索引真实存在。
- [ ] Admin 历史菜单和角色授权无 ID 变化或数据丢失。
- [ ] 菜单平台不可在线修改，父子菜单不可能跨平台。
- [ ] Canvas 内置平台、根 test 页面和 test action 由启动 foundation 幂等预设，且未引入 Canvas 业务前端。
- [ ] `/api/v1/access` 对普通用户、多角色和超级管理员均按当前 Platform ID 隔离。
- [ ] 菜单管理和角色权限页面完整表达平台维度。
- [ ] 认证平台有有效菜单时不可软删除。
- [ ] 后端、前端、构建和静态审计全部通过。
