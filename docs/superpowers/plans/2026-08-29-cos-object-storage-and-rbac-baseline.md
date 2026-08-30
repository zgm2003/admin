# COS 对象存储与 RBAC 基线 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变已验收的 Admin 业务行为和 URL 契约的前提下，完成个人资料 RBAC 闭环、RBAC 三层访问缓存、菜单权限基线和并发保护，并交付只面向腾讯云 COS 的配置、上传规则和短时预签名 PUT 能力。

**Architecture:** 继续使用 `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL` 的线性链路。PostgreSQL 保存 COS、规则和权限事实，Redis 负责跨进程 access 版本与快照协调，进程内只保存经过 Redis 版本门控的有界不可变快照；COS SDK 只存在于 `server/internal/storage/cos`，不引入 OSS 或多云抽象。数据库只由维护者手工执行事务化 SQL migration，API 和 Worker 启动只连接依赖并注册运行时组件。

**Tech Stack:** Go 1.26.5、Gin、GORM、PostgreSQL、go-redis/v9、腾讯云 `cos-go-sdk-v5`、Vue 3、TypeScript、Element Plus、Pinia、Vue Router、Vue I18n、SCSS、Vitest。

**Spec:**
- `docs/superpowers/specs/2026-08-29-cos-object-storage-design.md`
- `docs/superpowers/specs/2026-08-29-admin-modular-architecture-design.md`
- `AGENTS.md`

## Global Constraints

- 共享认证、权限和上传凭证使用 `/api/v1`；Admin 管理资源使用 `/api/admin/v1`；健康检查保留 `/health`、`/ready`。
- 普通请求固定为 `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`；Handler 不访问 GORM、Redis、Queue、Storage 或第三方 SDK。
- 前端 HTTP 数据流固定为 `view -> api/<module>.ts -> utils/request.ts -> Go API`；响应 envelope 只能是 `code`、`data`、`message`。
- 只实现腾讯云 COS；不得添加 OSS、S3、Provider、Driver、通用 Adapter、StorageManager、Factory、Manager、BaseService、BaseRepository、DI 容器或运行时注册器。
- 所有页面菜单节点的入口权限以资源级 `:list` 结尾；个人资料固定使用 `account:profile:list`，按钮和写接口分别使用 `account:profile:update`、`account:password:update`。
- `is_hidden=1` 只隐藏侧边栏，不删除 Access 快照、动态路由或权限矩阵中的页面节点；前端隐藏不是后端安全边界。
- RBAC 查找顺序固定为 Redis access version 门控、进程内 L1、Redis 快照、PostgreSQL 重建；Redis 状态无法确认、损坏或为 `invalidating` 时禁止使用旧 L1。
- PostgreSQL 只通过人工执行的 forward SQL migration 改变结构和菜单数据；API/Worker 启动禁止 AutoMigrate、EnsureSchema、PrepareSchema、seed、回填、菜单插入、foundation 修复和 Redis 清理。
- 有真实删除行为的表只使用 `deleted_at TIMESTAMPTZ NULL`；有效数据唯一性使用 `deleted_at IS NULL` 部分唯一索引；Yes/No 使用 `SMALLINT` 的 `0/1` 和 CHECK。
- `storage_upload_rule` 每个 `platform_id` 最多一个有效启用规则，但允许没有启用规则；这个不变量同时由事务锁和 PostgreSQL 部分唯一索引保证。
- COS SecretId/SecretKey 只能在 Service 内短暂出现，数据库保存 AES-256-GCM 密文；响应、普通日志、操作日志和测试输出不得包含明文、密文、签名 URL、Authorization、Cookie 或 Token。
- 上传凭证固定有效期为 10 分钟；客户端不能提交最终 object key、目录、过期时间或放宽规则上限；本期不创建文件元数据表、上传 token 表、STS、分片上传、代理上传或文件管理页面。
- 所有新行为先写失败测试，再写最小实现，再运行相关测试；不使用 Playwright 作为默认验证，不启动或停止 API、Worker、前端、PostgreSQL 或 Redis 服务。
- 每个任务完成后单独提交，提交标题必须符合 `<type>: <中文摘要>`，不得带 scope、句号或 `wip`。

## Scope and Dependencies

8 月 29 日模块化架构迁移、`user_profile` 表、`user_session`、`user_login_log`、`platform_id`、`taskdemo` 删除和启动持久化边界已经在当前分支完成并验证。本计划不重复迁移已经完成的目录，也不重新引入启动期 schema 准备。仅执行下面仍待落地的基线整改和 COS 独立功能：

```text
个人资料权限闭环
        -> Access Redis version 门控 + 进程内 L1
        -> 菜单行 key、Canvas code、默认角色并发基线
        -> 人工 RBAC 菜单 migration
        -> COS 表与 Admin 菜单 migration
        -> storage/cosconfig
        -> storage/uploadrule
        -> storage/cos + /api/v1/storage/upload-credentials
        -> Admin 对象存储两 Tab 页面
        -> 全量验证与人工验收
```

涉及多个独立领域时保持各任务的可测试边界，但它们必须按上面的依赖顺序合入。任何实现中发现需要文件表、业务对象引用、STS、分片上传、Canvas 业务代码、AI、支付、邮件、短信或 WebSocket，立即停止该分支并另开 spec。

### Task 1: 完成个人资料页面与按钮权限闭环

**Files:**
- Modify: `server/internal/module/user/profile/route.go`
- Create: `server/internal/module/user/profile/permissions.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/internal/module/audit/operationlog/rules.go`
- Create: `server/internal/module/user/profile/route_test.go`
- Modify: `server/cmd/api/main_test.go`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/router/access-routes.ts`
- Modify: `web/src/permission.ts`
- Modify: `web/src/layout/components/AppAside.vue`
- Modify: `web/src/layout/breadcrumbs.ts`
- Modify: `web/src/layout/components/RouteTabs.vue`
- Modify: `web/src/views/account/profile/index.vue`
- Modify: `web/tests/router/index.test.ts`
- Modify: `web/tests/router/access-routes.test.ts`
- Modify: `web/tests/layout/breadcrumbs.test.ts`
- Modify: `web/tests/layout/index.test.ts`
- Create: `web/tests/layout/components/AppAside.test.ts`
- Create: `web/tests/views/account/profile/index.test.ts`

**Interfaces:**
- Produces the route registration signature `RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc)`.
- Produces permission constants `PermissionList = "account:profile:list"`, `PermissionUpdate = "account:profile:update"`, and `PermissionPasswordUpdate = "account:password:update"` in the `profile` package.
- Produces dynamic route name `access:account:profile:list`; no static route named `account-profile` remains.
- Consumes the existing `AccessSnapshot.menuTree` and `permissionCodes` without changing the HTTP DTOs for profile data.

- [ ] **Step 1: Write the failing backend route contract test.**

  In `server/internal/module/user/profile/route_test.go`, register the routes with an authentication middleware and a `requirePermission` spy. Assert the exact middleware permission order and endpoint mapping:

  ```go
  want := []struct {
      method     string
      path       string
      permission string
  }{
      {http.MethodGet, "/api/admin/v1/account/profile", PermissionList},
      {http.MethodPut, "/api/admin/v1/account/profile", PermissionUpdate},
      {http.MethodPost, "/api/admin/v1/account/password", PermissionPasswordUpdate},
  }
  ```

  The test must prove that authentication runs before the page/action permission middleware and that no profile route is reachable through authentication alone.

- [ ] **Step 2: Run the focused Go test and verify the contract fails for the missing permission argument.**

  Run from `D:\admin\server`:

  ```powershell
  go test ./internal/module/user/profile -run TestRegisterRoutesRequiresProfilePermissions -count=1
  ```

  Expected result: FAIL because the current `RegisterRoutes` accepts no `requirePermission` function and the three routes only use authentication.

- [ ] **Step 3: Add permission constants and wire the backend middleware.**

  Add `permissions.go` with the three exact constants, change `RegisterRoutes` to the signature above, and bind routes as follows:

  ```go
  accountRoutes.GET("/profile", authenticate, requirePermission(PermissionList), handler.CurrentProfile)
  accountRoutes.PUT("/profile", authenticate, requirePermission(PermissionUpdate), handler.UpdateProfile)
  accountRoutes.POST("/password", authenticate, requirePermission(PermissionPasswordUpdate), handler.ChangePassword)
  ```

  Pass `dependencies.RequirePermission` from `server/cmd/api/main.go`. Update `main_test.go` fixtures so the route table still expects exactly the same three URLs, now with the permission middleware present.

- [ ] **Step 4: Register profile mutation routes in operation-log rules without capturing secrets.**

  Add rules for `PUT /api/admin/v1/account/profile` with action `account.profile.update` and `POST /api/admin/v1/account/password` with action `account.password.change`. Keep request sanitization enabled; the existing sanitizer must turn `currentPassword`, `newPassword`, and `confirmPassword` into `"***"`. Do not add a rule for profile GET, refresh, login, logout, or any upload credential endpoint.

- [ ] **Step 5: Write failing frontend tests for dynamic profile routing and conditional account entry.**

  Extend router tests to assert `createAppRouter().resolve('/account/profile').matched` is empty and `hasRoute('account-profile')` is false. Add an access-route test with a hidden page node:

  ```ts
  const cleanup = registerAccessRoutes(router, [profilePageNode], testViews)
  expect(router.resolve('/account/profile').name).toBe('access:account:profile:list')
  cleanup()
  expect(router.resolve('/account/profile').matched).toHaveLength(0)
  ```

  Add AppAside tests that set `permissionCodes` to include or exclude `account:profile:list`; the profile dropdown item must exist only in the former case, while logout remains visible in both cases.

- [ ] **Step 6: Remove static and hard-coded profile behavior.**

  Remove the `/account/profile` child from `web/src/router/index.ts`. Keep `account/profile` in `componentPathMap` so the Access page can resolve `web/src/views/account/profile/index.vue`. Remove the profile special cases from `resolveBreadcrumbs` and `RouteTabs.vue`; both must find profile metadata from `menuTree` like every other hidden page. In `AppAside.vue`, compute `canOpenProfile = access.hasPermission('account:profile:list')`, render the profile dropdown item conditionally, and navigate with `router.push('/account/profile')` so the dynamic route name is not hard-coded. Keep `web/src/permission.ts` redirect behavior: an authenticated URL absent from the Access tree resolves to Dashboard.

- [ ] **Step 7: Run the frontend and backend focused tests.**

  Run:

  ```powershell
  cd D:\admin\server
  go test ./internal/module/user/profile ./cmd/api -run 'Profile|Route|Operation' -count=1
  cd D:\admin\web
  pnpm vitest run tests/router/index.test.ts tests/router/access-routes.test.ts tests/layout/breadcrumbs.test.ts tests/layout/index.test.ts tests/layout/components/AppAside.test.ts --pool=threads --maxWorkers=1
  ```

  Expected result: all tests PASS; a user without `account:profile:list` is redirected to Dashboard before any profile component or API call is used.

- [ ] **Step 8: Commit the isolated profile permission change.**

  ```powershell
  cd D:\admin
  git add server/internal/module/user/profile/route.go server/internal/module/user/profile/permissions.go server/internal/module/user/profile/route_test.go server/cmd/api/main.go server/cmd/api/main_test.go server/internal/module/audit/operationlog/rules.go web/src/router/index.ts web/src/router/access-routes.ts web/src/permission.ts web/src/layout/components/AppAside.vue web/src/layout/breadcrumbs.ts web/src/layout/components/RouteTabs.vue web/tests/router/index.test.ts web/tests/router/access-routes.test.ts web/tests/layout/breadcrumbs.test.ts web/tests/layout/index.test.ts web/tests/layout/components/AppAside.test.ts
  git commit -m "fix: 收紧个人资料页面与按钮权限"
  ```

### Task 2: Implement Redis-gated bounded in-process Access cache

**Files:**
- Create: `server/internal/module/rbac/access/local_cache.go`
- Create: `server/internal/module/rbac/access/local_cache_test.go`
- Modify: `server/internal/module/rbac/access/service.go`
- Modify: `server/internal/module/rbac/access/service_test.go`
- Modify: `server/internal/module/rbac/access/redis_test.go`
- Modify: `server/cmd/api/main.go`

**Interfaces:**
- Produces the concrete cache key and cache API:

  ```go
  type SnapshotCacheKey struct {
      UserID        int64
      PlatformID    int64
      Platform      string
      PolicyVersion int64
      AccessVersion int64
  }

  type LocalSnapshotCache struct { /* map + mutex, bounded entries, injected clock */ }
  func NewLocalSnapshotCache(maxEntries int) *LocalSnapshotCache
  func (c *LocalSnapshotCache) Read(key SnapshotCacheKey, now time.Time) (CachedSnapshot, bool)
  func (c *LocalSnapshotCache) Put(key SnapshotCacheKey, snapshot CachedSnapshot, expiresAt time.Time)
  func cloneCachedSnapshot(snapshot CachedSnapshot) CachedSnapshot
  ```

- Changes `access.NewService` to accept `*LocalSnapshotCache` between the Redis `SnapshotCache` and logger arguments.
- Consumes the existing `accessstate.Store`, Redis `SnapshotCache`, `CachedSnapshot`, `cloneMenuTree`, and `identity.AccessCacheTTL`.

- [ ] **Step 1: Write failing unit tests for L1 isolation, expiry, and capacity.**

  In `local_cache_test.go`, construct a cache with capacity `2` and an injected clock. Assert all of the following:

  1. A `Put` followed by `Read` returns a deep copy; mutating the returned role slice, permission slice, or nested menu children does not mutate the stored value.
  2. A read at `now == expiresAt` is a miss and removes the entry.
  3. Inserting a third key evicts the oldest `createdAt` entry deterministically.
  4. A key differing only in `PlatformID`, `Platform`, `PolicyVersion`, or `AccessVersion` never hits.

- [ ] **Step 2: Run the cache unit test and verify it fails because the L1 type does not exist.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/rbac/access -run TestLocalSnapshotCache -count=1
  ```

- [ ] **Step 3: Implement the bounded immutable cache.**

  Use `map[SnapshotCacheKey]localSnapshotEntry` guarded by `sync.Mutex`; each entry contains a cloned `CachedSnapshot`, `createdAt`, and `expiresAt`. Reject non-positive capacity in the constructor by using the fixed application capacity `1024`; do not silently create an unbounded cache. `Read` removes expired entries and returns `cloneCachedSnapshot(entry.snapshot)`. `Put` ignores an already expired value, stores a clone, and evicts the entry with the oldest `createdAt`, breaking ties by the lexicographically ordered key fields. No cache method may log the snapshot contents.

- [ ] **Step 4: Write a failing service test proving the Redis version gate precedes L1.**

  Warm a service with `Current`, remove the Redis snapshot key while leaving the Redis access state at `ready`, and call `Current` again. The second call must be served by L1 without another PostgreSQL source call. Then close Redis and call `Current`; the call must use PostgreSQL and must not use the warm L1 entry. Add cases for `invalidating`, missing/corrupt state, and access version change; each must bypass L1 and either return the existing updating error or rebuild from PostgreSQL.

- [ ] **Step 5: Change the service lookup order and wire one bounded cache per API process.**

  `loadSnapshot` must follow this exact sequence:

  ```text
  states.Read(userID)
    -> if Redis state is ready and version > 0, LocalSnapshotCache.Read(full key)
    -> on L1 miss, SnapshotCache.Read(full key)
    -> on L2 miss/error, FindSourceWithVersion from PostgreSQL
    -> re-read Redis state/version before publish
    -> publish L2 only when the version still matches
    -> publish L1 only after the L2 publication succeeds
  ```

  A Redis state read error, unknown state, `invalidating`, invalid snapshot, or version mismatch must never fall through to an old L1 value. A PostgreSQL rebuild may be returned as a fresh result when Redis is unavailable, but no L1 entry may be written in that path. Preserve the existing `CacheResult` values and add only non-sensitive layer/result fields to controlled logs if needed. In `main.go`, instantiate `access.NewLocalSnapshotCache(1024)` and pass it to `access.NewService`.

- [ ] **Step 6: Run access tests and inspect the cache behavior.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/rbac/access -count=1
  ```

  Expected result: existing Redis read-through tests still pass, new L1 tests pass, and the PostgreSQL failure case returns `dependencyUnavailable` rather than stale permissions.

- [ ] **Step 7: Commit the access cache change.**

  ```powershell
  cd D:\admin
  git add server/internal/module/rbac/access/local_cache.go server/internal/module/rbac/access/local_cache_test.go server/internal/module/rbac/access/service.go server/internal/module/rbac/access/service_test.go server/internal/module/rbac/access/redis_test.go server/cmd/api/main.go
  git commit -m "perf: 增加 Redis 门控的进程级权限缓存"
  ```

### Task 3: Fix menu row keys, Canvas permission codes, and default-role concurrency coverage

**Files:**
- Modify: `web/src/views/access/menus/index.vue`
- Modify: `web/tests/views/access/menus/index.test.ts`
- Modify: `web/tests/views/access/roles/index.test.ts`
- Modify: `web/tests/views/access/roles/role-permission-matrix.test.ts`
- Modify: `server/internal/module/rbac/access/service_test.go`
- Modify: `server/internal/module/rbac/access/repository_test.go`
- Modify: `server/internal/module/rbac/menu/foundation_test.go`
- Modify: `server/internal/module/rbac/menu/service_test.go`
- Modify: `server/internal/module/rbac/menu/repository_test.go`
- Create: `server/internal/module/rbac/role/default_concurrency_test.go`
- Modify: `web/tests/api/rbac/menu.test.ts`
- Modify: `web/tests/api/rbac/role.test.ts`

**Interfaces:**
- The menu view state becomes `Set<string>` for both `expandedIDs` and `expansionBeforeSearch`; `expandedRowKeys` is `string[]`.
- Persistent Canvas page code is `canvas:test:list`; its action remains `canvas:test:button`.
- `rbac/role.Service.SetDefault` retains its current transaction API; this task adds real PostgreSQL concurrency evidence and only changes implementation if the test proves a race.

- [ ] **Step 1: Write the failing row-key regression assertions.**

  Update the menu test to expect `['1']` rather than `[1]`. Add cases that click “全部展开”, “全部收起”, search and clear search, and switch Admin/Canvas tabs. Assert every `expand-row-keys` prop contains only strings and that the full directory set is expanded after “全部展开”.

- [ ] **Step 2: Run the focused frontend test and verify the numeric key failure.**

  ```powershell
  cd D:\admin\web
  pnpm vitest run tests/modules/rbac/menu/index.test.ts --pool=threads --maxWorkers=1
  ```

  Expected result: FAIL at the existing numeric `expandRowKeys` assertion or the full-expand behavior.

- [ ] **Step 3: Normalize every menu row key at the state boundary.**

  Change the refs to `Set<string>`. Add one local helper and use it in every expansion path:

  ```ts
  function menuRowKey(id: number): string {
    return String(id)
  }
  ```

  Apply it in `setExpandedForRoots`, `expandAll`, search snapshot/restore, platform reload, and any table event handler. Keep `row-key="id"`; only the controlled `expand-row-keys` state must be string-valued.

- [ ] **Step 4: Update all non-persistent Canvas fixtures and protocol assertions.**

  Run from `D:\admin` before editing:

  ```powershell
  rg --pcre2 -n "canvas:test(?!:list)" server web -g "*.go" -g "*.ts" -g "*.vue"
  ```

  Update only these non-persistent fixtures and assertions: `server/internal/module/rbac/access/repository_test.go`,
  `server/internal/module/rbac/access/service_test.go`, `server/internal/module/rbac/menu/foundation_test.go`,
  `server/internal/module/rbac/menu/service_test.go`, `web/tests/api/rbac/menu.test.ts`,
  `web/tests/api/rbac/role.test.ts`, `web/tests/views/access/menus/index.test.ts`,
  `web/tests/views/access/roles/index.test.ts`, and `web/tests/views/access/roles/role-permission-matrix.test.ts`.
  Replace page fixtures and expected permission arrays with `canvas:test:list`, retain `canvas:test:button`,
  and leave migration/spec documents that mention the old code as historical preflight cases. Do not add a
  runtime fallback that accepts both codes; the persistent row is changed only by the manual migration in Task 4.

- [ ] **Step 5: Add a real PostgreSQL concurrent default-role test.**

  In `default_concurrency_test.go`, create two enabled non-default roles in an isolated schema, start two goroutines with a barrier so both call `SetDefault` concurrently, wait for both transactions, and query `COUNT(*) WHERE is_default=1 AND deleted_at IS NULL`. Assert the count is exactly one, the surviving row is enabled, and a failed transaction leaves the prior default intact. Accept a serialized success result or an explicit conflict, but never accept zero or two defaults. Do not add an optimistic-lock column or a second status field.

- [ ] **Step 6: Run menu, role, and access regression tests.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/rbac/access ./internal/module/rbac/menu ./internal/module/rbac/role -count=1
  cd D:\admin\web
  pnpm vitest run tests/modules/rbac/menu/index.test.ts tests/modules/rbac/role/index.test.ts tests/modules/rbac/role/role-permission-matrix.test.ts --pool=threads --maxWorkers=1
  ```

- [ ] **Step 7: Commit the row-key, Canvas fixture, and concurrency coverage.**

  ```powershell
  cd D:\admin
  git add web/src/views/access/menus/index.vue web/tests/api/rbac/menu.test.ts web/tests/api/rbac/role.test.ts web/tests/views/access/menus/index.test.ts web/tests/views/access/roles/index.test.ts web/tests/views/access/roles/role-permission-matrix.test.ts server/internal/module/rbac/access/repository_test.go server/internal/module/rbac/access/service_test.go server/internal/module/rbac/menu/foundation_test.go server/internal/module/rbac/menu/service_test.go server/internal/module/rbac/menu/repository_test.go server/internal/module/rbac/role/default_concurrency_test.go
  git commit -m "fix: 统一菜单行键并保障默认角色并发唯一"
  ```

### Task 4: Add the manual RBAC menu contract migration

**Files:**
- Create: `docs/database/2026-08-29-admin-rbac-baseline.sql`
- Create: `docs/database/2026-08-29-admin-rbac-baseline.md`
- Create: `server/internal/database/admin_rbac_baseline_migration_test.go`

**Interfaces:**
- Produces a maintenance-only forward SQL file; no Go runtime code imports or executes it.
- Preserves existing menu IDs, parent relationships, and `rbac_role_menu` rows.
- Creates the hidden profile page/actions and migrates the known Canvas page code without creating Canvas business code.

- [ ] **Step 1: Write migration tests against an isolated PostgreSQL schema.**

  Build a fixture containing the Admin and Canvas platforms, the Admin `account` directory, a `canvas:test` page with `canvas:test:button`, and an existing role grant. After executing the SQL, assert:

  ```text
  account:profile:list       page, parent=account, path=/account/profile, is_hidden=1
  account:profile:update     action, parent=account:profile:list, is_hidden=1
  account:password:update    action, parent=account:profile:list, is_hidden=1
  canvas:test:list            same ID, same parent, same role grants
  canvas:test                 absent
  ```

  Execute the script twice and assert the second execution changes no counts or IDs. Add a conflict fixture with both `canvas:test` and `canvas:test:list`; the script must fail and roll back. Add a fixture with a mismatched existing profile code; it must fail instead of overwriting it.

- [ ] **Step 2: Run the migration test and verify the SQL file is missing.**

  ```powershell
  cd D:\admin\server
  go test ./internal/database -run TestAdminRBACBaselineMigration -count=1
  ```

- [ ] **Step 3: Write the transactional migration with explicit preflight checks.**

  The SQL must begin with `BEGIN;` and use `ON_ERROR_STOP` in the documented command. Before any write, verify exactly one active Admin platform, an existing Admin `account` directory, and no conflicting duplicate codes. For Canvas, update only when exactly one old row exists and the target row is absent; if the target already exists without the old row, verify its shape and treat it as already migrated. Insert profile nodes with the fixed fields above, `layout.account.profile` as the page `i18n_key`, null render fields on actions, and `CURRENT_TIMESTAMP` for both timestamps. Use `ON DELETE RESTRICT` already present on `rbac_menu`; preserve role grants and, because menu codes change the effective permission snapshot, advance every active user's `rbac_access_version` in the same transaction. The migration document must also require a maintenance-window, key-scoped deletion of this project's `authz:access-state:*` and `authz:access:*` Redis entries after the SQL succeeds; never use `FLUSHDB`. End with `COMMIT;`.

- [ ] **Step 4: Document execution, rollback, and verification.**

  In the `.md` file, require a PostgreSQL backup and a maintenance window, show:

  ```powershell
  psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f docs/database/2026-08-29-admin-rbac-baseline.sql
  ```

  State that rollback is backup restore, not a guessed down migration. Include SQL that checks profile/COS menu codes, IDs, parent IDs, `is_hidden`, Canvas old-code absence, and role grant counts.

- [ ] **Step 5: Run the isolated migration tests and `git diff --check`.**

  ```powershell
  cd D:\admin\server
  go test ./internal/database -run TestAdminRBACBaselineMigration -count=1
  cd D:\admin
  git diff --check
  ```

- [ ] **Step 6: Commit the manual migration separately from runtime code.**

  ```powershell
  cd D:\admin
  git add docs/database/2026-08-29-admin-rbac-baseline.sql docs/database/2026-08-29-admin-rbac-baseline.md server/internal/database/admin_rbac_baseline_migration_test.go
  git commit -m "docs: 增加 Admin RBAC 菜单基线迁移"
  ```

### Task 5: Add the transactional COS schema and Admin menu migration

**Files:**
- Create: `docs/database/2026-08-29-cos-object-storage.sql`
- Create: `docs/database/2026-08-29-cos-object-storage.md`
- Create: `server/internal/database/cos_object_storage_migration_test.go`

**Interfaces:**
- Produces tables `storage_cos_config` and `storage_upload_rule`, their constraints, indexes, and the Admin `cloud`/`storage:object:list` menu tree with action nodes.
- The migration is the only source of initial COS schema/menu data; API and Worker do not call it.

- [ ] **Step 1: Write failing PostgreSQL schema tests.**

  In an isolated schema, create the prerequisite `auth_platform` and `rbac_menu` tables with the current test model helpers, execute the future migration, and assert the exact columns and types from the COS spec. Assert the following constraints and indexes exist:

  ```text
  fk_storage_upload_rule_platform
  fk_storage_upload_rule_cos_config
  ck_storage_cos_config_is_enabled
  ck_storage_upload_rule_is_enabled
  ck_storage_upload_rule_max_file_size
  ck_storage_upload_rule_max_file_count
  ck_storage_upload_rule_access_mode
  ux_storage_cos_config_name_active
  ix_storage_cos_config_enabled_created_at
  ux_storage_upload_rule_platform_code_active
  ux_storage_upload_rule_platform_enabled
  ix_storage_upload_rule_config_enabled_created_at
  ```

  `ux_storage_upload_rule_platform_enabled` must be `UNIQUE (platform_id) WHERE is_enabled = 1 AND deleted_at IS NULL`; it is the database-level enforcement of the confirmed “one enabled rule per platform, or none” invariant. Test that a second enabled rule for one platform is rejected, a disabled rule is allowed, and a different platform can enable its own rule. Test `ON DELETE RESTRICT` for both foreign keys and soft-delete visibility.

- [ ] **Step 2: Run the schema test and verify it fails because the migration file is missing.**

  ```powershell
  cd D:\admin\server
  go test ./internal/database -run TestCOSObjectStorageMigration -count=1
  ```

- [ ] **Step 3: Write the table DDL exactly once in a transaction.**

  Use this shape, with identity generation matching the existing BIGINT model convention. Every named
  CHECK below is required so the migration test can distinguish a compatible schema from an anonymous
  or drifted constraint:

  ```sql
  CREATE TABLE storage_cos_config (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    app_id VARCHAR(32) NOT NULL,
    secret_id_ciphertext TEXT NOT NULL,
    secret_key_ciphertext TEXT NOT NULL,
    bucket VARCHAR(128) NOT NULL,
    region VARCHAR(64) NOT NULL,
    endpoint VARCHAR(255) NULL,
    bucket_domain VARCHAR(255) NULL,
    is_enabled SMALLINT NOT NULL DEFAULT 1,
    remark VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT ck_storage_cos_config_is_enabled CHECK (is_enabled IN (0, 1))
  );

  CREATE TABLE storage_upload_rule (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    platform_id BIGINT NOT NULL,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    cos_config_id BIGINT NOT NULL,
    path_prefix VARCHAR(255) NOT NULL,
    max_file_size_bytes BIGINT NOT NULL,
    max_file_count INTEGER NOT NULL DEFAULT 1,
    allowed_extensions TEXT[] NOT NULL,
    allowed_mime_types TEXT[] NOT NULL,
    access_mode VARCHAR(16) NOT NULL DEFAULT 'private',
    is_enabled SMALLINT NOT NULL DEFAULT 1,
    remark VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT fk_storage_upload_rule_platform FOREIGN KEY (platform_id) REFERENCES auth_platform(id) ON DELETE RESTRICT,
    CONSTRAINT fk_storage_upload_rule_cos_config FOREIGN KEY (cos_config_id) REFERENCES storage_cos_config(id) ON DELETE RESTRICT,
    CONSTRAINT ck_storage_upload_rule_max_file_size CHECK (max_file_size_bytes > 0),
    CONSTRAINT ck_storage_upload_rule_max_file_count CHECK (max_file_count > 0),
    CONSTRAINT ck_storage_upload_rule_access_mode CHECK (access_mode IN ('private', 'public')),
    CONSTRAINT ck_storage_upload_rule_is_enabled CHECK (is_enabled IN (0, 1))
  );
  ```

  Before creating an existing relation, inspect its columns and constraints. If a same-named table or index exists with a different definition, raise an exception and roll back; do not use a blind `IF NOT EXISTS` that hides drift. Create the indexes with the exact definitions from the spec plus the partial unique enabled-rule index above. Do not create an array GIN index or standalone low-selectivity `is_enabled`/`deleted_at` index.

- [ ] **Step 4: Add the Admin menu nodes in the same maintenance boundary.**

  Verify the Admin `cloud` directory and `storage:object:list` page do not conflict. Insert or verify:

  ```text
  cloud                         directory, i18n=navigation.cloud, icon=lucide:cloud
  storage:object:list           page, parent=cloud, path=/cloud/object-storage,
                                component_path=storage/object,
                                i18n=navigation.storageObject,
                                icon=lucide:cloud-upload
  ```

  Insert these ten action nodes as hidden children of the object-storage page:
  `storage:cos-config:create`, `storage:cos-config:update`, `storage:cos-config:status`,
  `storage:cos-config:test`, `storage:cos-config:delete`, `storage:upload-rule:create`,
  `storage:upload-rule:update`, `storage:upload-rule:status`, `storage:upload-rule:delete`,
  and `storage:object:upload`. Do not insert a Canvas COS node, file-management node, upload-token node,
  or placeholder menu. Preserve all existing role grants; `super_admin` receives the current platform’s
  valid menu through the existing Access calculation rather than a special COS role path.

- [ ] **Step 5: Document migration execution and test idempotency.**

  The `.md` file must specify that the RBAC baseline migration runs before this migration, both require backup and a maintenance window, and rollback is backup restore. Include verification SQL for table columns, foreign keys, checks, indexes, menu parent/platform IDs, and counts. Execute the file twice in the test and assert no duplicate table/index/menu rows. Explicitly state that API/Worker startup never executes it.

- [ ] **Step 6: Run schema and documentation checks.**

  ```powershell
  cd D:\admin\server
  go test ./internal/database -run TestCOSObjectStorageMigration -count=1
  cd D:\admin
  git diff --check
  ```

- [ ] **Step 7: Commit the COS schema migration.**

  ```powershell
  cd D:\admin
  git add docs/database/2026-08-29-cos-object-storage.sql docs/database/2026-08-29-cos-object-storage.md server/internal/database/cos_object_storage_migration_test.go
  git commit -m "feat: 增加 COS 配置与上传规则数据库"
  ```

### Task 6: Add credential encryption and the COS configuration module

**Files:**
- Modify: `server/internal/secretkey/secretkey.go`
- Modify: `server/internal/secretkey/secretkey_test.go`
- Create: `server/internal/module/storage/cosconfig/model.go`
- Create: `server/internal/module/storage/cosconfig/crypto.go`
- Create: `server/internal/module/storage/cosconfig/request.go`
- Create: `server/internal/module/storage/cosconfig/response.go`
- Create: `server/internal/module/storage/cosconfig/errors.go`
- Create: `server/internal/module/storage/cosconfig/repository.go`
- Create: `server/internal/module/storage/cosconfig/service.go`
- Create: `server/internal/module/storage/cosconfig/handler.go`
- Create: `server/internal/module/storage/cosconfig/route.go`
- Create: `server/internal/module/storage/cosconfig/model_test.go`
- Create: `server/internal/module/storage/cosconfig/crypto_test.go`
- Create: `server/internal/module/storage/cosconfig/repository_test.go`
- Create: `server/internal/module/storage/cosconfig/service_test.go`
- Create: `server/internal/module/storage/cosconfig/handler_test.go`
- Create: `server/internal/storage/cos/types.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`
- Modify: `server/internal/module/audit/operationlog/rules.go`

**Interfaces:**
- `secretkey.KeyRing` gains `StorageEncryptionKey() []byte`, derived with HKDF purpose `admin:storage:cos-encryption:v1`; the derived key must differ from JWT and refresh-token keys.
- `cosconfig` exposes only safe values to handlers:

  ```go
  type SafeValue struct {
      ID int64; Name string; AppID string; Bucket string; Region string
      Endpoint *string; BucketDomain *string; IsEnabled yesno.Value
      HasCredentials bool; Remark string; CreatedAt time.Time; UpdatedAt time.Time
  }
  ```

- Define the module-owned inputs and service signatures exactly as follows; JSON names belong only to
  request DTOs and remain lower camel case at the Handler boundary:

  ```go
  type ListQuery struct {
      Page int; PageSize int; Keyword string; IsEnabled *yesno.Value
  }
  type SecretInput struct { Present bool; Value string }
  type CreateInput struct {
      Name string; AppID string; SecretID string; SecretKey string
      Bucket string; Region string; Endpoint *string; BucketDomain *string
      IsEnabled yesno.Value; Remark string
  }
  type UpdateInput struct {
      Name string; AppID string; Bucket string; Region string
      Endpoint *string; BucketDomain *string; SecretID SecretInput; SecretKey SecretInput
      Remark string
  }
  func (s *Service) List(context.Context, ListQuery) (pagination.Result[SafeValue], error)
  func (s *Service) Get(context.Context, int64) (SafeValue, error)
  func (s *Service) Create(context.Context, CreateInput) (int64, error)
  func (s *Service) Update(context.Context, int64, UpdateInput) error
  func (s *Service) UpdateStatus(context.Context, int64, yesno.Value) error
  func (s *Service) TestConnection(context.Context, int64) error
  func (s *Service) Delete(context.Context, int64) error
  func NewService(*Repository, *secretkey.KeyRing, cos.ConnectionTester) *Service
  ```

  `SecretInput.Present=false` means keep the existing ciphertext during update; `Present=true` is a
  replacement and its value must be non-empty after strict decoding. The repository owns PostgreSQL
  locking and exposes `Count`, `List`, `FindByID`, `LockByID`, `Create`, `Update`, `MarkDeleted`, and
  `CountEnabledRules(context.Context, int64) (int64, error)`; it never decrypts credentials or calls COS.
- Task 6 owns the SDK-free contract in `server/internal/storage/cos/types.go`:

  ```go
  type Credentials struct {
      AppID string; SecretID string; SecretKey string
      Bucket string; Region string; Endpoint string
  }

  type ConnectionTester interface {
      TestConnection(context.Context, Credentials) error
  }
  ```

  The connection test accepts only this narrow contract; Task 6 service tests use a local fake implementing
  `ConnectionTester`, and Task 8's concrete `*cos.Client` satisfies it without introducing a provider interface.

- [ ] **Step 1: Write failing key-ring and crypto tests.**

  Assert `StorageEncryptionKey()` is deterministic for the same `APP_SECRET`, different from `JWTSigningKey()` and `RefreshTokenHMACKey()`, and returns a copy. Add AES-256-GCM tests for round-trip, random nonce changes, versioned Base64 output, wrong-key failure, malformed ciphertext failure, and empty plaintext rejection.

- [ ] **Step 2: Run the focused tests and verify missing storage key/crypto behavior.**

  ```powershell
  cd D:\admin\server
  go test ./internal/secretkey ./internal/module/storage/cosconfig -count=1
  ```

- [ ] **Step 3: Implement key derivation and module-local encryption.**

  Add the fixed HKDF purpose to `secretkey`, and keep AES-GCM encoding in `cosconfig/crypto.go`. Encode `v1:<base64(nonce+ciphertext)>`; generate a fresh nonce with `crypto/rand` on every encryption. Decrypt only inside Service methods that need to test a connection or issue a credential. Never put plaintext in `Value`, GORM models returned to handlers, slog fields, or operation payloads.

- [ ] **Step 4: Write strict DTO tests before the repository implementation.**

  Test unknown fields, duplicate JSON keys, missing required fields, invalid URL/host values, empty names, invalid `isEnabled`, and explicit `null`/empty secret replacement. Secret fields use a presence-aware type: missing means “keep the current ciphertext” during update; an explicit JSON value replaces it; `null` and `""` are invalid. No endpoint accepts a secret-clearing operation.

- [ ] **Step 5: Implement the model, repository, and service lifecycle.**

  Map `storage_cos_config` explicitly with `CreatedAt`, `UpdatedAt`, and `gorm.DeletedAt`. Normalize and validate names, AppID, Bucket, Region, endpoint, bucket domain, and remark in Service. Create encrypts both secrets before inserting. Update locks the row, rejects deleted rows, preserves omitted secrets, and updates ciphertext only when a new value is supplied. Status changes lock the row and set `updated_at`; enabling a deleted row is rejected, and disabling a config with any enabled, non-deleted rule reference returns `conflict` without changing either row. Delete locks the config, checks `storage_upload_rule.deleted_at IS NULL AND is_enabled = 1 AND cos_config_id = ?`, returns `conflict` when any row exists, otherwise atomically sets `is_enabled=0`, `deleted_at`, and `updated_at`.

- [ ] **Step 6: Implement handlers, routes, and audit rules.**

  Register exactly:

  ```text
  GET    /api/admin/v1/storage/cos-configs
  POST   /api/admin/v1/storage/cos-configs
  GET    /api/admin/v1/storage/cos-configs/:id
  PUT    /api/admin/v1/storage/cos-configs/:id
  PATCH  /api/admin/v1/storage/cos-configs/:id/status
  POST   /api/admin/v1/storage/cos-configs/:id/test
  DELETE /api/admin/v1/storage/cos-configs/:id
  ```

  Every route uses Admin platform, authentication, and its exact action permission. Responses include only safe metadata and `hasCredentials`. Add operation-log rules for create, update, status, test, and delete with module `storage` and actions `storage.cos-config.*`; sanitizer coverage must prove that no secret or ciphertext enters the task payload.

- [ ] **Step 7: Wire the module without network calls at startup.**

  Instantiate the repository and Service in `server/cmd/api/main.go` using the existing `KeyRing` and a
  nil `cos.ConnectionTester` placeholder until Task 8 wires `cos.NewClient(nil)`. Add route dependencies
  to `routerDependencies` and `buildRouter`. Do not add an env var for database credentials; COS
  credentials are stored encrypted in PostgreSQL and the only root secret remains `APP_SECRET`.

- [ ] **Step 8: Run module tests and commit.**

  ```powershell
  cd D:\admin\server
  go test ./internal/secretkey ./internal/module/storage/cosconfig ./cmd/api -count=1
  cd D:\admin
  git add server/internal/secretkey/secretkey.go server/internal/secretkey/secretkey_test.go server/internal/storage/cos/types.go server/internal/module/storage/cosconfig server/cmd/api/main.go server/cmd/api/main_test.go server/internal/module/audit/operationlog/rules.go
  git commit -m "feat: 增加 COS 配置管理模块"
  ```

### Task 7: Add the upload-rule module and atomic per-platform enablement

**Files:**
- Create: `server/internal/module/storage/uploadrule/model.go`
- Create: `server/internal/module/storage/uploadrule/request.go`
- Create: `server/internal/module/storage/uploadrule/response.go`
- Create: `server/internal/module/storage/uploadrule/errors.go`
- Create: `server/internal/module/storage/uploadrule/repository.go`
- Create: `server/internal/module/storage/uploadrule/service.go`
- Create: `server/internal/module/storage/uploadrule/handler.go`
- Create: `server/internal/module/storage/uploadrule/route.go`
- Create: `server/internal/module/storage/uploadrule/model_test.go`
- Create: `server/internal/module/storage/uploadrule/repository_test.go`
- Create: `server/internal/module/storage/uploadrule/service_test.go`
- Create: `server/internal/module/storage/uploadrule/handler_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`
- Modify: `server/internal/module/audit/operationlog/rules.go`

**Interfaces:**
- Define the module-owned inputs and service outputs before implementing handlers:

  ```go
  type ListQuery struct {
      Page int; PageSize int; PlatformID *int64; CosConfigID *int64
      Keyword string; IsEnabled *yesno.Value
  }
  type CreateInput struct {
      PlatformID int64; Code string; Name string; CosConfigID int64; PathPrefix string
      MaxFileSizeBytes int64; MaxFileCount int; AllowedExtensions []string
      AllowedMimeTypes []string; AccessMode string; IsEnabled yesno.Value; Remark string
  }
  type UpdateInput struct {
      Name string; CosConfigID int64; PathPrefix string; MaxFileSizeBytes int64; MaxFileCount int
      AllowedExtensions []string; AllowedMimeTypes []string; AccessMode string; Remark string
  }
  type PageInit struct {
      Platforms []PlatformOption; Configs []ConfigSummary
  }
  type PlatformOption struct { ID int64; Code string; Name string; IsEnabled yesno.Value }
  type ConfigSummary struct { ID int64; Name string; Bucket string; Region string; IsEnabled yesno.Value }
  type RuleValue struct {
      ID int64; PlatformID int64; PlatformCode string; PlatformName string
      Code string; Name string; CosConfigID int64; CosConfigName string
      PathPrefix string; MaxFileSizeBytes int64; MaxFileCount int
      AllowedExtensions []string; AllowedMimeTypes []string; AccessMode string
      IsEnabled yesno.Value; Remark string; CreatedAt time.Time; UpdatedAt time.Time
  }
  type FileInput struct { FileName string; ContentType string; FileSizeBytes int64 }
  type CredentialInput struct { RuleCode string; Files []FileInput }
  type CredentialItem struct {
      UploadURL string; ObjectKey string; Method string; Headers map[string]string
      ExpiresAt time.Time; PublicURL *string
  }
  type CredentialResponse struct { Items []CredentialItem }
  func (s *Service) List(context.Context, ListQuery) (pagination.Result[RuleValue], error)
  func (s *Service) PageInit(context.Context) (PageInit, error)
  func (s *Service) Get(context.Context, int64) (RuleValue, error)
  func (s *Service) Create(context.Context, CreateInput) (int64, error)
  func (s *Service) Update(context.Context, int64, UpdateInput) error
  func (s *Service) UpdateStatus(context.Context, int64, yesno.Value) error
  func (s *Service) Delete(context.Context, int64) error
  func (s *Service) IssueCredentials(context.Context, auth.Identity, CredentialInput) (CredentialResponse, error)
  ```

  `RuleValue`, `PlatformOption`, and `ConfigSummary` are safe response structs; none contains ciphertext.
  Define the unexported repository-to-Service `UploadTarget` with `PlatformID`, normalized rule limits,
  access mode, bucket metadata, and the two ciphertext strings; it exists only for the immediate signing
  call. `IssueCredentials` is added only when Task 8 supplies the concrete COS client.
- Rule code and platform are immutable after creation because `code` is the business stable reference; update changes name, config, prefix, limits, allowed arrays, access mode, and remark.
- Repository methods include `Count`, `List`, `FindByID`, `LockByID`, `LockActiveByPlatform`, `Create`, `Update`, `MarkDeleted`, `FindPlatformOptions`, `FindConfigSummaries`, and `FindUploadTarget`. `FindUploadTarget` joins the rule to its COS config and returns ciphertext only to the Service, never to a response.

- [ ] **Step 1: Write validation and concurrency tests.**

  Cover lower-cased extensions without leading dots, lower-cased MIME types, at least one extension, safe relative prefixes, rejection of `/`, `..`, control characters, zero/negative values, file size above `5 * 1024 * 1024 * 1024`, invalid access mode, and public mode without `bucket_domain`. Add tests for platform/config mismatch, disabled/deleted platform or config, duplicate `(platform_id, code)`, deletion of enabled rules, and disabled rules disappearing from credential lookup.

- [ ] **Step 2: Write the failing atomic enable test.**

  Create two rules on one platform, enable the first, then concurrently enable the second from two PostgreSQL transactions. Assert the final state contains at most one enabled rule and that a failed operation leaves the previous enabled rule unchanged. A successful enable must disable the previous active rule in the same transaction; a disable operation may leave zero enabled rules.

- [ ] **Step 3: Implement model and repository queries.**

  Map `TEXT[]` using the PostgreSQL array type already selected for the project, include explicit timestamps and `gorm.DeletedAt`, and filter normal reads with `deleted_at IS NULL`. `FindUploadTarget` must make `platform_id = currentPlatformID` the first rule condition, require `is_enabled=1` and non-deleted rows, and join a non-deleted, enabled COS config. `PageInit` returns enabled/non-deleted auth platform options and safe COS config summaries in one response.

- [ ] **Step 4: Implement service validation and transaction semantics.**

  On create, normalize all arrays and validate the target platform/config before insert. A create request with
  `IsEnabled=1` locks all active rules for the platform in ascending ID order, disables the previous active
  rule in the same transaction, and then inserts the new rule; `IsEnabled=0` inserts without changing other
  rules. On update, lock the rule and validate any changed config against the same enabled, non-deleted
  platform/config rules. On enable, lock all active rules for the platform in ascending ID order, lock/validate
  the target config, set other active rules to `is_enabled=0` with the same `updated_at`, then set the target
  to `1`; let `ux_storage_upload_rule_platform_enabled` reject any unexpected race and map that constraint to
  `conflict`. On disable, only set the target to `0`. On delete, require disabled first, then set `deleted_at`
  and `updated_at`. Every state-changing method returns an explicit error and never returns an empty success
  on a database failure.

- [ ] **Step 5: Implement admin routes and operation audit.**

  Register:

  ```text
  GET    /api/admin/v1/storage/upload-rules
  GET    /api/admin/v1/storage/upload-rules/page-init
  POST   /api/admin/v1/storage/upload-rules
  GET    /api/admin/v1/storage/upload-rules/:id
  PUT    /api/admin/v1/storage/upload-rules/:id
  PATCH  /api/admin/v1/storage/upload-rules/:id/status
  DELETE /api/admin/v1/storage/upload-rules/:id
  ```

  Apply the exact `storage:upload-rule:*` permissions. Add audit rules for create, update, status, and delete; capture only normalized non-secret rule metadata. Do not add an operation rule for future credential issuance.

- [ ] **Step 6: Run the module and real PostgreSQL tests.**

  ```powershell
  cd D:\admin\server
  go test ./internal/module/storage/uploadrule ./cmd/api -count=1
  ```

  Expected result: service tests prove the one-enabled-per-platform invariant under concurrency, and handler tests reject unknown fields, duplicate keys, invalid arrays, and secret-like response fields.

- [ ] **Step 7: Commit the upload-rule module.**

  ```powershell
  cd D:\admin
  git add server/internal/module/storage/uploadrule server/cmd/api/main.go server/cmd/api/main_test.go server/internal/module/audit/operationlog/rules.go
  git commit -m "feat: 增加平台上传规则管理"
  ```

### Task 8: Implement the concrete Tencent COS client and presigned upload credentials

**Files:**
- Create: `server/internal/storage/cos/client.go`
- Create: `server/internal/storage/cos/client_test.go`
- Modify: `server/go.mod`
- Modify: `server/go.sum`
- Modify: `server/internal/module/storage/cosconfig/service.go`
- Modify: `server/internal/module/storage/uploadrule/service.go`
- Modify: `server/internal/module/storage/uploadrule/handler.go`
- Modify: `server/internal/module/storage/uploadrule/route.go`
- Create: `server/internal/module/storage/uploadrule/credential_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`

**Interfaces:**
- Adds the official `github.com/tencentyun/cos-go-sdk-v5` dependency at a pinned version selected by `go mod tidy`; no other object-storage SDK is allowed.
- Consumes `Credentials` and `ConnectionTester` from Task 6's SDK-free `storage/cos/types.go` and adds the concrete client types and methods:

  ```go
  type PutRequest struct {
      ObjectKey string; ContentType string; ContentLength int64
      PublicRead bool
  }

  type PutResult struct { URL string; Headers map[string]string }

  type Client struct { /* concrete COS SDK configuration and injected http.Client */ }
  func NewClient(httpClient *http.Client) *Client
  func (c *Client) TestConnection(context.Context, Credentials) error
  func (c *Client) PresignPut(context.Context, Credentials, PutRequest) (PutResult, error)
  ```

- `uploadrule.Service.IssueCredentials(ctx, identity auth.Identity, input CredentialInput) (CredentialResponse, error)` is the only business entry point for signing; the handler never receives a secret or calls the SDK.

- [ ] **Step 1: Write failing client boundary tests.**

  Use a fake HTTP transport or SDK test seam that never contacts Tencent. Assert `TestConnection` performs only a bucket reachability request and maps third-party failures to an internal error. Assert `PresignPut` binds method `PUT`, the service-generated object key, `Content-Type`, `Content-Length`, fixed ten-minute expiry, and `x-cos-acl: public-read` only when `PublicRead` is true. Assert no test log contains SecretId, SecretKey, signature query, or complete URL.

- [ ] **Step 2: Add the official SDK and implement the concrete client.**

  Build the bucket as `<bucket>-<app_id>`, use `region` and optional `endpoint` for the SDK client, and call the SDK’s context-aware bucket request for connection testing. Use its presign API for PUT, pass the exact signed headers, and return a copy of headers. `PresignPut` always uses the package constant `10*time.Minute`; the request type has no expiry field. `NewClient(nil)` is the production construction and tests pass an `http.Client` with a fake `RoundTripper`. Reject non-positive content length and unsafe object keys before the SDK call.

- [ ] **Step 3: Implement object-key generation and credential validation in uploadrule Service.**

  Accept only:

  ```json
  {
    "ruleCode": "avatar",
    "files": [{"fileName": "photo.png", "contentType": "image/png", "fileSizeBytes": 123456}]
  }
  ```

  Resolve the rule using the authenticated `identity.PlatformID`, require `storage:object:upload`, validate batch count, normalized extension, MIME and size, and reject path separators, `..`, control characters, missing extension, and client object-key fields. Generate `path_prefix/YYYY/MM/DD/<random-id>.<extension>` with `crypto/rand`; never reuse the original filename as the key. Decrypt the joined COS credentials only in memory, call `cos.Client.PresignPut`, and zero local byte slices when practical. For `public`, require a valid bucket domain and return `publicUrl = bucketDomain + "/" + escapedObjectKey`; for `private`, omit the read URL and ACL.

- [ ] **Step 4: Add the shared credential route with platform isolation.**

  Register `POST /api/v1/storage/upload-credentials` under the shared authenticated routes with `requirePermission("storage:object:upload")`. The handler must get `auth.Identity` from context, bind strict JSON, call Service with `context.Request.Context()`, and return `{code,data,message}`. It must never add an operation-log rule, because credential issuance is not an Admin mutation. A Canvas identity can see only Canvas rules; an Admin identity cannot use a Canvas rule unless the Admin platform explicitly owns an enabled rule and permission.

- [ ] **Step 5: Add service, handler, and route tests.**

  Assert a valid request returns one item per file with `uploadUrl`, `objectKey`, `method="PUT"`, signed `Content-Type`, `expiresAt` ten minutes in the future, and the public URL only for public rules. Assert platform mismatch, missing permission, disabled/deleted rule, disabled/deleted config, batch overflow, extension/MIME/size violations, and SDK errors return the exact public `invalidRequest`, `forbidden`, `notFound`, `conflict`, or `dependencyUnavailable` envelope. Inspect captured logs and operation payloads for forbidden sensitive values.

- [ ] **Step 6: Wire the concrete client and run focused tests.**

  Construct one `cos.Client` with `cos.NewClient(nil)` in `server/cmd/api/main.go`, pass that same
  pointer as the `cos.ConnectionTester` and credential signer dependency, and keep construction
  side-effect free. Run:

  ```powershell
  cd D:\admin\server
  go test ./internal/storage/cos ./internal/module/storage/cosconfig ./internal/module/storage/uploadrule ./cmd/api -count=1
  ```

- [ ] **Step 7: Commit the concrete COS and credential slice.**

  ```powershell
  cd D:\admin
  git add server/internal/storage/cos server/internal/module/storage/cosconfig/service.go server/internal/module/storage/uploadrule/service.go server/internal/module/storage/uploadrule/handler.go server/internal/module/storage/uploadrule/route.go server/internal/module/storage/uploadrule/credential_test.go server/cmd/api/main.go server/cmd/api/main_test.go server/go.mod server/go.sum
  git commit -m "feat: 增加 COS 预签名上传凭证"
  ```

### Task 9: Build the Admin object-storage page with two internal tabs

**Files:**
- Create: `web/src/api/storage/cosconfig.ts`
- Create: `web/src/api/storage/uploadrule.ts`
- Create: `web/src/views/cloud/storage-object/index.vue`
- Create: `web/tests/api/storage/cosconfig.test.ts`
- Create: `web/tests/api/storage/uploadrule.test.ts`
- Create: `web/tests/views/cloud/storage-object/index.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Modify: `web/tests/i18n/index.test.ts`

**Interfaces:**
- `cosconfig.ts` exposes typed functions for the seven Admin endpoints and parses `unknown` responses with exact-key checks. Its DTOs never contain `secretId`, `secretKey`, ciphertext, SDK config, or signature data in response types.
- `uploadrule.ts` exposes typed functions for list, page-init, create, update, status, and delete; its `CredentialResponse` parser is kept separate from Admin list DTOs if a future business client needs it.
- `index.vue` consumes only the two API files, `useAccessStore().hasPermission`, existing `AppTable`/`AppDialog`/Element Plus components, and the `storage:object:list` menu metadata.

- [ ] **Step 1: Write failing API parser tests.**

  Mock `request` and assert exact URLs, methods, query fields, and request bodies. Feed parsers missing fields, extra fields, wrong numeric/array types, invalid `YesNo`, and a response containing `secretId`, `secretKey`, `ciphertext`, or `signature`; each must throw `ProtocolError`. A valid config response must expose `hasCredentials` only.

- [ ] **Step 2: Implement typed API modules.**

  Use `PageRequest`/`PageResult` and the existing `request<unknown>` pattern. Keep `platformId`, `cosConfigId`, `maxFileSizeBytes`, `maxFileCount`, `allowedExtensions`, `allowedMimeTypes`, and `accessMode` in lower camel case. Do not add `platform` aliases or accept `msg` in the envelope.

- [ ] **Step 3: Write failing page interaction tests.**

  Mount the page with Element Plus, Pinia, and the app i18n. Assert it renders one `el-tabs` with exactly `config` and `rules` panes, calls only the active tab’s list API, hides each button when its action permission is absent, and shows configuration status, `hasCredentials`, rule platform/config summaries, enabled state, and safe empty/error states. Add tests for create/edit dialogs, connection-test success/failure, enable/disable confirmation, deletion conflict, and rule form validation.

- [ ] **Step 4: Implement the two-tab Element Plus page.**

  Use `el-tabs`, `el-table`, `el-form`, `el-dialog`, `el-select`, `el-input-number`, `el-checkbox-group`, `el-radio-group`, `el-switch`, `el-tag`, `el-alert`, `el-empty`, and `el-pagination`. The COS configuration tab supports create, edit, connection test, enable/disable, and soft delete. Secret inputs are password fields that start empty in edit mode; the page never displays a previous value or hint. The upload-rule tab uses `page-init` for platform/config options, supports normalized extension/MIME entry, fixed numeric bounds, public/private warning, and atomic status feedback. Deletion and connection test use Element Plus confirmation/result interactions, not custom string alerts.

- [ ] **Step 5: Add complete Chinese and English catalog entries.**

  Add navigation keys for `navigation.cloud` and `navigation.storageObject`, permission labels for all ten Admin actions, table/form/status/error/success messages for both tabs, public-access risk text, and validation messages in both `zh-CN.ts` and `en-US.ts`. Do not put a locale switch on the login page; locale remains localStorage-persisted through the existing global mechanism. Run catalog validation so every key exists in both languages.

- [ ] **Step 6: Run page tests, type-check, and build.**

  ```powershell
  cd D:\admin\web
  pnpm vitest run tests/api/storage/cosconfig.test.ts tests/api/storage/uploadrule.test.ts tests/modules/storage/object/index.test.ts --pool=threads --maxWorkers=1
  pnpm build
  ```

  Expected result: all storage tests pass, `vue-tsc` reports no `any` or missing DTO fields, and the production build succeeds. Do not use Playwright unless the user separately requests browser automation.

- [ ] **Step 7: Commit the Admin object-storage page.**

  ```powershell
  cd D:\admin
  git add web/src/api/storage web/src/views/cloud/storage-object/index.vue web/tests/api/storage web/tests/views/cloud/storage-object/index.test.ts web/src/i18n/messages/zh-CN.ts web/src/i18n/messages/en-US.ts web/tests/i18n/index.test.ts
  git commit -m "feat: 增加 Admin 对象存储管理页面"
  ```

### Task 10: Run full verification and perform the maintenance handoff

**Files:**
- Verify: `README.md`
- Verify: `docs/database/2026-08-29-admin-modular-architecture.md`
- Create: `docs/superpowers/verification/2026-08-29-cos-object-storage-and-rbac-baseline.md`
- No runtime file changes are allowed in this task unless a preceding test identifies a concrete defect.

**Interfaces:**
- Verifies the completed implementation against both approved specs and the project hard rules.
- Produces a human-executable migration and acceptance checklist, not a new runtime abstraction.

- [ ] **Step 1: Run repository-wide forbidden-reference scans.**

  ```powershell
  cd D:\admin
  rg --pcre2 -n "taskdemo|example-tasks|foundation_task|auth_session|canvas:test(?!:list)|account:profile:view|account:profile:read|storage.*OSS|oss" server web docs README.md -g "*.go" -g "*.ts" -g "*.vue" -g "*.sql" -g "*.md"
  rg -n "AutoMigrate|PrepareSchema|EnsureSchema|EnsureFoundation|InstallReadyIfMissing|DeleteMany|FlushDB|FLUSHDB" server/cmd server/internal -g "*.go"
  rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" web/src web/tests -g "*.ts" -g "*.vue"
  ```

  The first scan may match historical migration/test names that are intentionally documented; classify each result. Runtime code must contain no taskdemo/foundation startup path, old Canvas page code, old profile page suffix, OSS reference, or static profile route. The second scan must not find startup DDL/seed/Redis cleanup. The third scan must be empty.

- [ ] **Step 2: Run all backend checks.**

  ```powershell
  cd D:\admin\server
  gofmt -w internal/module/storage internal/storage/cos internal/module/user/profile internal/module/rbac/access
  go fmt ./...
  go vet ./...
  go test ./... -count=1
  go build ./...
  ```

  Confirm the real PostgreSQL and Redis integration tests were not silently skipped when the local dependencies are available. Record any intentionally skipped integration test and its reason instead of claiming full coverage.

- [ ] **Step 3: Run all frontend checks.**

  ```powershell
  cd D:\admin\web
  pnpm vitest run --pool=threads --maxWorkers=1
  pnpm build
  ```

- [ ] **Step 4: Execute the two manual migrations only after backup and review.**

  In a maintenance window, back up PostgreSQL, stop API/Worker as required by the operator, then run the RBAC baseline SQL followed by the COS SQL with `psql -v ON_ERROR_STOP=1`. Record table/menu/index/foreign-key counts and verify existing user, role, authorization, session, and operation-log IDs remain unchanged. Do not start a service as part of the migration command and do not invoke a Go migration runner.

- [ ] **Step 5: Perform focused manual acceptance.**

  Verify:

  1. A user with `account:profile:list` can open the hidden profile from the sidebar avatar; users without it are redirected from a forced URL to Dashboard, and profile GET/PUT/password POST return the correct permission errors.
  2. Changing locale persists across reloads and Element Plus follows the current locale; profile and COS labels have both translations.
  3. Access requests for Admin and Canvas never contain the other platform’s menus or permission codes; super admin receives all valid menus only for the current platform.
  4. Access L1 hits only after a Redis `ready` version check; Redis outage or `invalidating` never serves an old local snapshot.
  5. Menu “全部展开” expands every directory, “全部收起” collapses them, search/platform changes preserve the correct string keys, and concurrent default-role changes leave exactly one default.
  6. “云服务 -> 对象存储” contains exactly two tabs. Config secrets never render. Enabling a rule atomically disables the previous rule on the same platform; disabling all rules is possible. A config with an enabled rule cannot be deleted.
  7. Credential issuance returns short-lived PUT URLs and generated object keys, binds MIME/length, and never writes a signature, Secret, Authorization, or credential request to operation logs.
  8. Restarting API/Worker does not change schema, insert menus, seed data, backfill access versions, or clear Redis.

- [ ] **Step 6: Update only the verification record and finish without an automatic commit.**

  Write command outputs, migration execution timestamps, and skipped-test reasons to
  `docs/superpowers/verification/2026-08-29-cos-object-storage-and-rbac-baseline.md` using the fixed
  sections `Environment`, `Commands`, `Migration`, `Skipped tests`, and `Acceptance`. Verify that
  `README.md` and `docs/database/2026-08-29-admin-modular-architecture.md` do not claim runtime migration
  or demo-task routes; update only those documentation lines if an inaccurate statement is found. Run
  `git status --short` and `git diff --check`; confirm the working tree contains only intentional
  documentation changes after the task commits. This plan does not authorize an automatic final commit
  or push.

## Self-Review Checklist

- [ ] Every COS spec section has an implementation task: scope/menu, encrypted config, rule lifecycle, one-enabled invariant, upload credential flow, API envelope, audit/error handling, migration boundary, backend tests, frontend tests, and manual acceptance.
- [ ] Every baseline requirement has a task: profile page/action `:list` contract, dynamic route, hidden avatar entry, backend Middleware, Redis version gate, bounded immutable L1, string row keys, Canvas code migration, and real PostgreSQL default-role concurrency.
- [ ] The confirmed per-platform rule invariant is enforced in both Service transaction order and `ux_storage_upload_rule_platform_enabled`; all-stop remains valid.
- [ ] No task introduces a runtime migration, startup seed, old permission compatibility branch, OSS path, file metadata table, STS, or business-platform menu.
- [ ] All cross-task names are consistent: `storage_cos_config`, `storage_upload_rule`, `storage/cosconfig`, `storage/uploadrule`, `storage/cos`, `storage:object:list`, `storage:object:upload`, `account:profile:list`, `account:profile:update`, and `account:password:update`.
- [ ] All test commands are scoped to the changed risk and the final full suite; no completion claim is made without command output.
