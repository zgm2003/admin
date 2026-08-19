# Menu Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. This repository is intentionally executed inline on `main`; do not dispatch subagents and do not create commits without explicit repository-owner instruction.

**Goal:** Implement the approved menu-management vertical slice: a PostgreSQL-backed directory/page/action tree with protected core menus, transactional tree mutations, strict HTTP contracts, and an Element Plus management page at `/system/menus`.

**Architecture:** Keep the existing linear flow `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`. The menu Service owns tree rules and transaction intent; the concrete menu Repository owns PostgreSQL queries, row locks, and writes. Frontend data flows `view -> api/menu.ts -> utils/request.ts -> menu API`, with strict runtime DTO parsing and no global menu Store or generic tree abstraction.

**Tech Stack:** Go 1.26, Gin, GORM, PostgreSQL, PostgreSQL integration tests; Vue 3, TypeScript, Vue Router, Pinia access state, Element Plus, vue-i18n, Vitest, Vue Test Utils, pnpm.

---

## Existing boundaries to preserve

- `server/internal/module/menu/model.go` already maps `sys_menu` and `sys_role_menu` with explicit timestamps, `gorm.DeletedAt`, and Yes/No values.
- `server/internal/module/menu/schema.go` adds idempotent constraints and indexes after `AutoMigrate`; `schema_test.go` uses real PostgreSQL.
- `server/internal/shared/validate.BindJSON` rejects unknown fields and trailing JSON documents.
- `server/internal/module/access/middleware.go` exposes `access.RequirePermission`; its Repository already reads enabled menus and direct role grants.
- `server/cmd/api/main.go` owns database setup, system initialization, service construction, and route registration.
- `web/src/api/access.contract.ts` is the reference for closed-record runtime validation.
- `web/src/access/menu-icons.ts`, `route-views.ts`, `web/src/i18n`, and `web/src/store/access.ts` are the existing frontend protocol and permission surfaces.

The repository owner controls Git history. Every task ends with a focused test and diff review; this plan never commits, pushes, fetches, pulls, starts, or stops services.

### Task 1: Extend the PostgreSQL menu schema

**Files:**
- Modify: `server/internal/module/menu/schema.go`
- Modify: `server/internal/module/menu/schema_test.go`
- Test: `server/internal/module/menu/schema_test.go`

- [x] **Step 1: Add failing assertions for the new constraints and index**

Extend the test maps with these exact objects:

```go
checks := map[string][]string{
    "ck_sys_menu_render_shape": {"CHECK", "directory", "path", "action", "icon"},
    "ck_sys_menu_sort_order":   {"CHECK", "sort_order", "0"},
}
indexes := map[string][]string{
    "ux_sys_menu_page_path_active": {"CREATE UNIQUE INDEX", "(path)", "menu_type", "page", "deleted_at IS NULL"},
}
```

The render-shape check must require directory `path IS NULL AND view_key IS NULL`, page non-empty `path` and `view_key`, and action `path IS NULL AND view_key IS NULL AND icon IS NULL`. The sort check requires `sort_order >= 0`.

- [x] **Step 2: Run the schema test and confirm it fails for missing objects**

Run from `D:\admin\server`:

```powershell
go test ./internal/module/menu -run TestMenuSchema -count=1
```

Expected: FAIL because the new constraint and page-path index do not exist. A PostgreSQL connection failure is an environment failure, not the expected red state.

- [x] **Step 3: Add the idempotent schema definitions**

Append to `menuConstraints`:

```go
{
    name:  "ck_sys_menu_render_shape",
    table: "sys_menu",
    ddl: `ALTER TABLE sys_menu ADD CONSTRAINT ck_sys_menu_render_shape CHECK (
        (menu_type = 'directory' AND path IS NULL AND view_key IS NULL)
        OR (menu_type = 'page' AND path IS NOT NULL AND btrim(path) <> '' AND view_key IS NOT NULL AND btrim(view_key) <> '')
        OR (menu_type = 'action' AND path IS NULL AND view_key IS NULL AND icon IS NULL)
    )`,
},
{
    name:  "ck_sys_menu_sort_order",
    table: "sys_menu",
    ddl:   `ALTER TABLE sys_menu ADD CONSTRAINT ck_sys_menu_sort_order CHECK (sort_order >= 0)`,
},
```

Append to `menuIndexes`:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS ux_sys_menu_page_path_active
ON sys_menu (path)
WHERE deleted_at IS NULL AND menu_type = 'page'
```

Do not use AutoMigrate to drop or rewrite an existing constraint.

- [x] **Step 4: Run the focused test again**

Run:

```powershell
go test ./internal/module/menu -run TestMenuSchema -count=1
```

Expected: PASS with all named constraints and indexes present.

### Task 2: Define menu protocol constants and localized errors

**Files:**
- Create: `server/internal/module/menu/errors.go`
- Create: `server/internal/module/menu/protocol.go`
- Create: `server/internal/module/menu/errors_test.go`
- Modify: `server/internal/shared/i18n/catalog.go`
- Modify: `server/internal/shared/i18n/catalog_test.go`

- [x] **Step 1: Write failing protocol and error tests**

Test these stable permission/core constants:

```go
const (
    PermissionList        = "system:menu:list"
    PermissionCreate      = "system:menu:create"
    PermissionUpdate      = "system:menu:update"
    PermissionDelete      = "system:menu:delete"
    BuiltinSystemCode     = "system"
    BuiltinMenuListCode   = "system:menu:list"
)
```

Test the exact error contract:

```text
14000 menu tree invalid       500
14001 menu not found          404
14002 menu code conflict      409
14003 menu path conflict      409
14004 invalid parent          400
14005 cycle detected          400
14006 builtin protected       409
14007 parent disabled         409
14008 structure conflict      409
14009 invalid menu fields     400
```

Every error test checks HTTP status, business code, message key, params, and `errors.Is` cause preservation.

- [x] **Step 2: Run the tests and confirm the symbols are absent**

Run:

```powershell
go test ./internal/module/menu ./internal/shared/i18n -run 'Test(Menu|Catalog)' -count=1
```

Expected: compile failure for the missing constants/errors.

- [x] **Step 3: Implement the finite backend protocol**

`protocol.go` contains the core codes, permissions, allowed menu title keys, exact view key `system-menus`, and icon keys `Cpu`, `Folder`, `Key`, `Menu`, `Setting`, `User`.

```go
func IsMenuTitleKey(value string) bool
func IsMenuViewKey(value string) bool
func IsMenuIconKey(value string) bool
func IsBuiltinCode(value string) bool
```

Use package constants/maps only. Do not create a runtime registry, Manager, Adapter, or plugin system.

- [x] **Step 4: Implement menu errors and matching catalogs**

`errors.go` constructs concrete `*apperror.Error` values with the status/code table above. Add matching catalog keys:

```text
menu.notFound
menu.codeConflict       {{code}}
menu.pathConflict       {{path}}
menu.invalidParent
menu.cycleDetected
menu.builtinProtected   {{code}}
menu.parentDisabled     {{code}}
menu.structureConflict  {{code}}
menu.invalidFields
menu.treeInvalid
```

Chinese and English use identical key and parameter sets. Internal causes never enter response messages.

- [x] **Step 5: Run focused tests**

Run:

```powershell
go test ./internal/module/menu ./internal/shared/i18n -count=1
```

Expected: PASS, including `ValidateCatalogs()`.

### Task 3: Implement and test core-menu bootstrap

**Files:**
- Create: `server/internal/module/menu/builtin.go`
- Create: `server/internal/module/menu/builtin_test.go`
- Create: `server/internal/module/menu/repository.go`
- Create: `server/internal/module/menu/service.go`
- Modify: `server/cmd/api/main.go`

- [x] **Step 1: Write failing PostgreSQL bootstrap tests**

Use the existing real-PostgreSQL fixture style and one transaction per test. Cover:

1. empty menu data creates the five exact core rows;
2. a second call inserts nothing;
3. a child physically removed inside the isolated test transaction is recreated without rewriting existing correct rows;
4. mutated type, parent, i18n key, path, view key, disabled state, soft-delete history, or duplicate history fails;
5. operator-adjusted icon and sort order survive a later initialization.

The exact core tree is:

```text
system                  directory  parent NULL       navigation.system       path/view NULL             icon Setting sort 100
system:menu:list        page       system            navigation.systemMenus  /system/menus system-menus icon Menu    sort 10
system:menu:create      action     system:menu:list  permission.menuCreate  path/view/icon NULL                     sort 10
system:menu:update      action     system:menu:list  permission.menuUpdate  path/view/icon NULL                     sort 20
system:menu:delete      action     system:menu:list  permission.menuDelete  path/view/icon NULL                     sort 30
```

- [x] **Step 2: Run the bootstrap tests and confirm the initializer is absent**

Run:

```powershell
go test ./internal/module/menu -run 'TestEnsureBuiltin' -count=1
```

Expected: compile failure for missing `EnsureBuiltin` support.

- [x] **Step 3: Add concrete locked repository support**

Implement:

```go
func NewRepository(db *gorm.DB) *Repository
func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error
func (r *Repository) LockMenuTableForBuiltin(ctx context.Context) error
func (r *Repository) FindBuiltinRecords(ctx context.Context, codes []string) ([]Menu, error)
func (r *Repository) Create(ctx context.Context, value *Menu) error
```

`LockMenuTableForBuiltin` executes `LOCK TABLE sys_menu IN SHARE ROW EXCLUSIVE MODE` inside the transaction. `FindBuiltinRecords` uses `Unscoped()` so deleted history cannot be mistaken for an empty slot. Create the minimal concrete `Service` and `NewService(*Repository)` in `service.go`; later tasks extend the same type.

- [x] **Step 4: Implement `Service.EnsureBuiltin`**

Use this sequence:

```text
transaction
-> lock sys_menu
-> load all five codes unscoped
-> reject duplicate/history/corrupt records
-> create missing rows parent before child
-> validate existing immutable fields and enabled state
-> commit
```

Missing rows may be created. Existing rows are never silently restored, moved, renamed, enabled, or overwritten. Existing icon and sort order are allowed and preserved.

- [x] **Step 5: Wire bootstrap before HTTP listen**

In `server/cmd/api/main.go`, construct the menu Repository/Service after `menu.EnsureSchema`, call `EnsureBuiltin(processContext)`, and return a wrapped error before `ListenAndServe` on failure. Do not add Handler/router dependencies until Task 7 creates those types.

- [x] **Step 6: Run focused bootstrap tests**

Run:

```powershell
go test ./internal/module/menu ./cmd/api -run 'Test(EnsureBuiltin|BuildRouter)' -count=1
```

Expected: PASS for bootstrap behavior and API construction.

### Task 4: Add repository tree reads and transactional write primitives

**Files:**
- Modify: `server/internal/module/menu/repository.go`
- Create: `server/internal/module/menu/repository_test.go`

- [x] **Step 1: Write failing repository integration tests**

Cover these concrete operations:

- active reads include enabled and disabled rows, exclude soft-deleted rows, and sort by `sort_order, code, id`;
- locked reads operate inside a transaction;
- create writes nullable and non-null fields plus database timestamps;
- update can set nullable parent/path/view/icon to SQL NULL;
- bulk status updates all requested IDs and `updated_at`;
- role-menu soft deletion touches only active links;
- menu soft deletion writes one supplied timestamp to all IDs;
- `ux_sys_menu_code_active` and `ux_sys_menu_page_path_active` violations convert to menu sentinel errors.

- [x] **Step 2: Run tests and confirm the methods are missing**

Run:

```powershell
go test ./internal/module/menu -run 'TestRepository' -count=1
```

Expected: compile/assertion failures for unimplemented repository methods.

- [x] **Step 3: Implement the concrete methods**

Use these signatures:

```go
type UpdateValues struct {
    ParentID  *int64
    MenuType  Type
    I18nKey   string
    Path      *string
    ViewKey   *string
    Icon      *string
    SortOrder int
}

func (r *Repository) FindActiveMenus(ctx context.Context) ([]Menu, error)
func (r *Repository) LockActiveMenus(ctx context.Context) ([]Menu, error)
func (r *Repository) UpdateMenu(ctx context.Context, id int64, values UpdateValues, updatedAt time.Time) error
func (r *Repository) UpdateMenuStatus(ctx context.Context, ids []int64, value yesno.Value, updatedAt time.Time) error
func (r *Repository) HasActiveDirectGrant(ctx context.Context, menuID int64) (bool, error)
func (r *Repository) SoftDeleteRoleMenus(ctx context.Context, menuIDs []int64, deletedAt time.Time) error
func (r *Repository) SoftDeleteMenus(ctx context.Context, menuIDs []int64, deletedAt time.Time) error
```

`UpdateMenu` uses an explicit map so nil values become SQL NULL. Every query uses `WithContext(ctx)`; writes maintain `updated_at`. `SoftDeleteRoleMenus` and `SoftDeleteMenus` update both `updated_at` and `deleted_at` to the supplied UTC timestamp. Inspect `*pgconn.PgError.ConstraintName` for uniqueness conversion and preserve all other causes.

Define the sentinels in `repository.go` and preserve them with `%w`:

```go
var (
    ErrMenuCodeConflict = errors.New("active menu code conflicts")
    ErrMenuPathConflict = errors.New("active menu page path conflicts")
)
```

- [x] **Step 4: Run repository tests**

Run:

```powershell
go test ./internal/module/menu -run 'TestRepository' -count=1
```

Expected: PASS with no `context.Background()`, Gin, Redis, Queue, or memory fallback inside Repository.

### Task 5: Implement the menu tree Service and mutation validation

**Files:**
- Modify: `server/internal/module/menu/service.go`
- Create: `server/internal/module/menu/tree.go`
- Create: `server/internal/module/menu/service_test.go`

- [x] **Step 1: Define the Service contract and write failing tests**

Use these public types:

```go
type CreateInput struct {
    ParentID  *int64
    MenuType  Type
    Code      string
    I18nKey   string
    Path      *string
    ViewKey   *string
    Icon      *string
    SortOrder int
    IsEnabled yesno.Value
}

type UpdateInput struct {
    ParentID  *int64
    MenuType  Type
    I18nKey   string
    Path      *string
    ViewKey   *string
    Icon      *string
    SortOrder int
}

type ManagedMenu struct {
    ID        int64
    ParentID  *int64
    MenuType  Type
    Code      string
    I18nKey   string
    Path      *string
    ViewKey   *string
    Icon      *string
    SortOrder int
    IsEnabled yesno.Value
    IsBuiltin bool
    CreatedAt time.Time
    UpdatedAt time.Time
    Children  []ManagedMenu
}
```

The Service API is:

```go
func NewService(repository *Repository) *Service
func (s *Service) EnsureBuiltin(ctx context.Context) error
func (s *Service) List(ctx context.Context) ([]ManagedMenu, error)
func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error)
func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error
func (s *Service) UpdateStatus(ctx context.Context, id int64, value yesno.Value) error
func (s *Service) Delete(ctx context.Context, id int64) error
```

Write tests for sorted tree output, explicit root null, root non-directory, missing/invalid parents, self/descendant cycles, action children, unknown title/view/icon keys, invalid code/path/sort/YesNo, duplicate code/path, static path collisions, enabled nodes below disabled ancestors, normal code immutability, core protection, and directory conversion with a direct role grant.

Core update cases must prove: directory/page allow only registered icon and sort changes; action allows only sort; all core nodes reject type, parent, i18nKey, path, viewKey, status, and deletion changes. Create cases must prove an enabled child requires an entirely enabled ancestor chain, while a disabled child may be created below a disabled parent if the structural relationship is valid.

- [x] **Step 2: Run Service tests to establish the red state**

Run:

```powershell
go test ./internal/module/menu -run 'Test(Service|BuildMenu|ValidateMenu)' -count=1
```

Expected: compile or assertion failures for the missing tree/Service behavior.

- [x] **Step 3: Implement private tree indexing in `tree.go`**

Use one loaded slice and package-private helpers:

```go
type menuIndex struct {
    byID     map[int64]Menu
    children map[int64][]int64
    roots    []int64
}

func buildMenuIndex(menus []Menu) (menuIndex, error)
func (index menuIndex) descendants(id int64) ([]int64, error)
func (index menuIndex) ancestors(id int64) ([]int64, error)
func (index menuIndex) validateEnabledAncestors() error
func (index menuIndex) buildManagedTree() ([]ManagedMenu, error)
```

Reject duplicate IDs, orphans, root non-directories, illegal type pairs, action children, invalid stored shapes, and cycles. Use visited ID sets. Sort sibling IDs by `sort_order, code, id`; leaf `Children` is always a non-nil empty slice. Do not issue recursive N+1 SQL and do not export a generic TreeService.

- [x] **Step 4: Implement `List`, `Create`, and `Update` as tree operations**

`List` performs one repository read, builds/validates the complete index, and returns the managed tree. Corrupt persisted data maps to `14000`; PostgreSQL failure maps to dependency unavailable.

Create flow:

```text
normalize and validate input
-> transaction
-> lock active menus
-> validate parent/type/enabled ancestors/code/path/protocol
-> insert one row
-> commit and return ID
```

Update flow:

```text
transaction
-> lock active menus
-> find target
-> enforce builtin immutable fields and all code immutability
-> replace target in an in-memory candidate tree
-> validate complete candidate tree and enabled ancestors
-> reject direct grant if target becomes directory
-> write only the target row
-> commit
```

Validation must trim strings, then reject blank/invalid values rather than generating names. Code has maximum length 128 and matches `^[a-z][a-z0-9]*(?::[a-z][a-z0-9]*)*$`; i18nKey/viewKey have maximum length 128; path has maximum length 255; icon has maximum length 64; sort order is non-negative. Path is absolute, contains no query/hash, and cannot be `/`, `/login`, or `/dashboard`. Map sentinel conflicts to `14002/14003`; map tree/business failures to the exact `14001-14009` constructor; map unknown database failures to dependency unavailable.

- [x] **Step 5: Run Service tests and verify rollback**

Run:

```powershell
go test ./internal/module/menu -run 'Test(Service|BuildMenu|ValidateMenu)' -count=1
```

Expected: PASS, including a forced write failure that leaves the original row unchanged. Create a named CHECK constraint inside the outer test transaction that rejects the candidate `i18n_key`, invoke `Service.Update`, then query the original row and assert every field is unchanged. Do not add a production test hook.

### Task 6: Implement transactional status changes and subtree deletion

**Files:**
- Modify: `server/internal/module/menu/service.go`
- Modify: `server/internal/module/menu/tree.go`
- Modify: `server/internal/module/menu/service_test.go`
- Modify: `server/internal/module/menu/repository_test.go`

- [x] **Step 1: Add failing state-transition tests**

Cover:

- disabling a directory sets the complete subtree to `0` but keeps active role-menu links;
- enabling a child under a disabled ancestor returns `14007` and changes nothing;
- enabling a parent changes only the parent and leaves disabled children unchanged;
- same-state status calls are idempotent;
- builtin disable/delete returns `14006`;
- normal subtree deletion soft-deletes all menu rows and active role-menu links with one timestamp;
- deleted rows disappear from `List` but remain under `Unscoped()`;
- injected role-menu/menu write failures roll back the entire delete.

For the delete rollback case, add `CHECK (deleted_at IS NULL)` to `sys_menu` inside the outer test transaction. `SoftDeleteRoleMenus` runs first and `SoftDeleteMenus` fails second; after the Service returns, assert both menu rows and role-menu rows are still active. Rolling back the outer test transaction removes the temporary constraint.

- [x] **Step 2: Run focused tests and confirm missing behavior**

Run:

```powershell
go test ./internal/module/menu -run 'Test(Service.*Status|Service.*Delete|Repository.*Delete)' -count=1
```

Expected: FAIL until bulk state and delete logic exists.

- [x] **Step 3: Implement `UpdateStatus`**

Use exactly:

```text
transaction
-> lock all active menus
-> find target and validate YesNo/core protection
-> No: collect target + descendants and bulk update all to No
-> Yes: validate complete ancestor chain and update target only
-> commit
```

Do not enable descendants and do not modify `sys_role_menu`. Set `updated_at` explicitly for changed rows.

- [x] **Step 4: Implement `Delete`**

Build the locked index, collect target and descendants iteratively, detect cycles, and reject any subtree containing a builtin code. Use one `deletedAt := time.Now().UTC()` and one transaction:

```go
repository.SoftDeleteRoleMenus(ctx, ids, deletedAt)
repository.SoftDeleteMenus(ctx, ids, deletedAt)
```

Already-deleted/missing returns `14001`. Do not use physical `ON DELETE CASCADE` or row-by-row recursive deletion.

- [x] **Step 5: Run the menu Service/Repository suite**

Run:

```powershell
go test ./internal/module/menu -run 'Test(Service|Repository)' -count=1
```

Expected: PASS, including role-menu preservation, role-menu deletion, timestamp equality, and rollback assertions.

### Task 7: Add strict HTTP DTOs, handlers, routes, and app wiring

**Files:**
- Create: `server/internal/module/menu/request.go`
- Create: `server/internal/module/menu/response.go`
- Create: `server/internal/module/menu/handler.go`
- Create: `server/internal/module/menu/route.go`
- Create: `server/internal/module/menu/handler_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`
- Modify: `server/internal/module/access/repository_test.go`

- [x] **Step 1: Write failing Handler and route tests**

Test:

- missing `parentId` differs from explicit `"parentId": null`;
- `parentId=0`, string numbers, wrong nulls, missing/unknown fields, multiple JSON values, `msg`, and invalid Yes/No return 400 with exactly `code/data/message`;
- GET returns the complete managed tree;
- POST returns 201 and `{ "id": ... }`;
- PUT returns 200 and its payload has no `code`/`isEnabled`;
- PATCH accepts only `{ "isEnabled": 0|1 }`;
- DELETE requires an empty body and returns `{ "id": ... }`;
- Service errors retain status/code/localized message;
- all five routes bind the exact permission code;
- unauthenticated requests stop at 401 and authenticated unauthorized requests stop at 403.

Add `TestRepositoryBuiltinMenusAppearForSuperAdmin`: initialize the five core menus, assign only the built-in `super_admin` role to an enabled user, call `access.Service.Current`, and assert the snapshot contains directory `system`, page `system:menu:list`, and all four page/action permission codes without `sys_role_menu` grants.

- [x] **Step 2: Run tests and confirm missing HTTP symbols**

Run:

```powershell
go test ./internal/module/menu ./internal/module/access ./cmd/api -run 'Test(Menu|BuildRouter|Repository.*Builtin)' -count=1
```

Expected: compile failures for Handler/request/response/routes.

- [x] **Step 3: Implement strict request decoding**

Use `validate.BindJSON`. Missing and explicit null require presence-aware local types:

```go
type nullableInt64 struct {
    Value   *int64
    Present bool
}

type nullableString struct {
    Value   *string
    Present bool
}
```

Their `UnmarshalJSON` sets `Present=true` for both a value and `null`; an omitted key stays false. Create requires every field including `parentId/path/viewKey/icon`; use pointers for scalar `sortOrder` and `isEnabled` so zero is distinguishable from missing. Update excludes code/status. Status accepts only `isEnabled`. DTO conversion rejects missing values, invalid IDs and Yes/No, then returns the exact Service input.

- [x] **Step 4: Implement handlers and exact response DTOs**

Define only the real Handler boundary interface:

```go
type menuService interface {
    List(context.Context) ([]ManagedMenu, error)
    Create(context.Context, CreateInput) (int64, error)
    Update(context.Context, int64, UpdateInput) error
    UpdateStatus(context.Context, int64, yesno.Value) error
    Delete(context.Context, int64) error
}
```

Handlers use `context.Request.Context()`, positive path IDs, `validate.RequireEmptyBody` for DELETE, and `response.OK/Fail`. Responses use lower camel case, RFC3339 timestamps, nullable JSON `null`, computed `isBuiltin`, and non-nil `children` arrays.

- [x] **Step 5: Register the five explicit permission routes**

Use:

```go
func RegisterRoutes(
    routes *gin.RouterGroup,
    handler *Handler,
    authenticate gin.HandlerFunc,
    requirePermission func(string) gin.HandlerFunc,
)
```

Register GET/POST `/menus`, PUT/DELETE `/menus/:id`, PATCH `/menus/:id/status`, in the order `authenticate -> requirePermission -> handler`, with `PermissionList/Create/Update/Delete`. In `main.go`, construct menu Repository/Service/Handler and pass `access.RequirePermission(accessService, code)` via the callback. Keep `/access` unchanged.

- [x] **Step 6: Run HTTP composition tests**

Run:

```powershell
go test ./internal/module/menu ./internal/module/access ./cmd/api -count=1
```

Expected: PASS. Do not start the API process.

### Task 8: Add frontend title/view/icon protocol and i18n keys

**Files:**
- Create: `web/src/access/menu-title-keys.ts`
- Create: `web/src/access/menu-title-keys.test.ts`
- Modify: `web/src/access/protocol.ts`
- Modify: `web/src/access/protocol.test.ts`
- Modify: `web/src/access/route-views.ts`
- Modify: `web/src/api/access.contract.ts`
- Modify: `web/src/api/access.contract.test.ts`
- Modify: `web/src/router/access-routes.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Create: `web/src/views/system/menus/index.vue`

- [x] **Step 1: Write failing registry and access-contract tests**

Test that `menuTitleKeys` contains exactly the five initial title/permission keys, `routeViews` contains `system-menus`, access directory path is always null, page path/view are non-null, action never appears in `menuTree`, and dynamic menu titles must be registered menu titles rather than arbitrary application message keys. Update access fixtures to use the system keys. Catalogs must retain exact key parity.

- [x] **Step 2: Run focused frontend protocol tests**

Run from `D:\admin\web`:

```powershell
pnpm vitest run src/access src/api/access.contract.test.ts src/router/access-routes.test.ts src/i18n/index.test.ts
```

Expected: FAIL for absent registries, view loader, and translations.

- [x] **Step 3: Implement typed registries**

Create:

```ts
export const menuTitleKeys = [
  'navigation.system',
  'navigation.systemMenus',
  'permission.menuCreate',
  'permission.menuUpdate',
  'permission.menuDelete',
] as const satisfies readonly AppMessageKey[]

export type MenuTitleKey = (typeof menuTitleKeys)[number]
export function isMenuTitleKey(value: string): value is MenuTitleKey
```

Define the view explicitly:

```ts
export const routeViews = {
  'system-menus': () => import('../views/system/menus/index.vue'),
} as const satisfies RouteViewMap

export type RouteViewKey = keyof typeof routeViews
```

Retain the existing `hasRouteViewKey` and `isMenuIconKey` predicates in `protocol.ts`, now backed by the typed map; do not introduce a second predicate name. Never concatenate an import path or return a default view/icon.

Create the real route target before registering it:

```vue
<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <section class="menu-management-page" aria-labelledby="menu-management-title">
    <h1 id="menu-management-title">{{ t('menu.title') }}</h1>
  </section>
</template>
```

This is the first working page slice needed by the dynamic import, not a fallback component. Tasks 10-11 extend the same file with its table and form.

- [x] **Step 4: Tighten access parsing and add both locale catalogs**

Change dynamic menu `titleKey` to `MenuTitleKey`, require directory `path === null`, and validate title/view/icon registries. Add menu page, column, type, form, confirmation, success, and validation strings to both locale files with identical keys; keep `fallbackLocale: false`.

- [x] **Step 5: Run protocol tests and scan for forbidden TypeScript**

Run:

```powershell
pnpm vitest run src/access src/api/access.contract.test.ts src/router/access-routes.test.ts src/i18n/index.test.ts
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src -g "*.ts" -g "*.vue"
```

Expected: tests PASS and no new AnyScript matches.

### Task 9: Implement the strict frontend menu API contract

**Files:**
- Create: `web/src/api/menu.contract.ts`
- Create: `web/src/api/menu.contract.test.ts`
- Create: `web/src/api/menu.ts`
- Create: `web/src/api/menu.test.ts`
- Modify: `web/src/enums/yes-no.ts`
- Modify: `web/src/enums/yes-no.test.ts`

- [x] **Step 1: Write failing closed-record parser tests**

Test nodes with exactly:

```text
id, parentId, menuType, code, i18nKey, path, viewKey, icon,
sortOrder, isEnabled, isBuiltin, createdAt, updatedAt, children
```

Reject missing/extra fields, wrong nullability, IDs below 1, unknown types/title/view/icon, negative sort, Yes/No outside 0/1, invalid timestamps, duplicate IDs/codes/page paths, illegal directory/action shapes, invalid root/parent types, a child's `parentId` not matching its enclosing node, roots with non-null parent IDs, siblings not ordered by `sortOrder, code, id`, and non-array/null children. Also test exact mutation results `{ id }` and `{ id, isEnabled }`.

- [x] **Step 2: Run tests and confirm the API files are absent**

Run:

```powershell
pnpm vitest run src/api/menu.contract.test.ts src/api/menu.test.ts
```

Expected: FAIL because the menu contract/API files do not exist.

- [x] **Step 3: Implement explicit DTOs and strict parsers**

Define:

```ts
export type ManagedMenuType = 'directory' | 'page' | 'action'

export interface ManagedMenuNode {
  id: number
  parentId: number | null
  menuType: ManagedMenuType
  code: string
  i18nKey: MenuTitleKey
  path: string | null
  viewKey: RouteViewKey | null
  icon: MenuIconKey | null
  sortOrder: number
  isEnabled: YesNo
  isBuiltin: boolean
  createdAt: string
  updatedAt: string
  children: ManagedMenuNode[]
}
```

Also define exact `CreateMenuInput`, `UpdateMenuInput`, `{ id: number }`, and `MenuStatusResult`. Parse from `unknown` using a closed-record helper, positive integer/timestamp checks, the three protocol registries, and one recursive parser with ID/code/path sets. Do not use `as any`, `?? []`, optional-field guessing, or default icon/title values.

- [x] **Step 4: Implement exact API calls and response parsing**

Implement:

```ts
getMenus(): Promise<ManagedMenuNode[]>
createMenu(input: CreateMenuInput): Promise<{ id: number }>
updateMenu(id: number, input: UpdateMenuInput): Promise<{ id: number }>
updateMenuStatus(id: number, isEnabled: YesNo): Promise<MenuStatusResult>
deleteMenu(id: number): Promise<{ id: number }>
```

Use `request<unknown>` and exact GET/POST `/api/v1/menus`, PUT/DELETE `/api/v1/menus/:id`, and PATCH `/api/v1/menus/:id/status`. DELETE sends no body. This module never imports or calls the access API.

Add and test the finite runtime predicate in `enums/yes-no.ts`:

```ts
export function isYesNo(value: unknown): value is YesNo {
  return value === YesNo.No || value === YesNo.Yes
}
```

- [x] **Step 5: Run focused API tests**

Run:

```powershell
pnpm vitest run src/api/menu.contract.test.ts src/api/menu.test.ts
```

Expected: PASS for strict parsing, exact methods/URLs/payloads, and malformed response rejection.

### Task 10: Build the menu management tree page

**Files:**
- Modify: `web/src/views/system/menus/index.vue`
- Create: `web/src/views/system/menus/index.test.ts`

- [x] **Step 1: Write failing load/render/permission tests**

Mock `getMenus` and mount with Element Plus, `appI18n`, and a fresh Pinia. Test:

- one load on mount;
- directory/page/action rows render translated title, type, code, path/view, icon, sort, enabled, and builtin columns;
- disabled rows remain visible and leaves use `children: []`;
- an API rejection shows explicit error state instead of a fake successful empty tree;
- `system:menu:create/update/delete` independently control their commands;
- builtin rows expose only permitted edit behavior;
- the page does not create a body-level scroll owner.

- [x] **Step 2: Run the component test and confirm the view is absent**

Run:

```powershell
pnpm vitest run src/views/system/menus/index.test.ts
```

Expected: FAIL because the initial route-target page does not yet load or render the management tree.

- [x] **Step 3: Implement local page state and Element Plus tree table**

Use local `ref`/`computed` state only: menus, loading, error, drawer state, editing ID, form state, and expanded row keys. Do not create a menu Store. Render an `el-table` tree with stable `row-key="id"` and the fields above. Use Element Plus variables/components and existing `menuIcons`; do not introduce a new theme or visible body scrollbar.

- [x] **Step 4: Implement permission-controlled commands**

Read `useAccessStore().hasPermission`:

```text
system:menu:create -> add root and valid child commands
system:menu:update -> edit and status commands
system:menu:delete -> delete command
```

Directory can add directory/page, page can add action, action cannot add children. Use icon buttons where a familiar Element Plus icon exists and provide tooltip/accessible labels.

- [x] **Step 5: Run the view shell tests**

Run:

```powershell
pnpm vitest run src/views/system/menus/index.test.ts
```

Expected: PASS for loading, rendering, errors, disabled state, builtin state, and permission visibility.

### Task 11: Implement the typed drawer form and page mutations

**Files:**
- Modify: `web/src/views/system/menus/index.vue`
- Modify: `web/src/views/system/menus/index.test.ts`

- [x] **Step 1: Add failing form and mutation tests**

Test:

- root creation submits `parentId: null`, never 0;
- each type shows only its valid fields and sends explicit null for disallowed nullable fields;
- code is required on create, read-only on edit, absent from update payload;
- changing type clears incompatible path/view/icon values;
- parent options exclude self/descendants and filter allowed parent types;
- title/view/icon values come only from registries;
- core immutable fields/status/delete are disabled while icon/sort remain editable where allowed;
- create/update/status/delete call the exact API and then `getMenus` exactly once;
- no mutation calls `getAccess`, `access.load`, route cleanup, or `window.location.reload`;
- success tells the operator to manually refresh for global navigation;
- failure preserves the current table and shows the explicit error.

- [x] **Step 2: Run tests and confirm the drawer behavior is missing**

Run:

```powershell
pnpm vitest run src/views/system/menus/index.test.ts
```

Expected: FAIL on the new interaction assertions.

- [x] **Step 3: Implement a discriminated typed form**

Use:

```ts
interface MenuFormState {
  parentId: number | null
  menuType: ManagedMenuType
  code: string
  i18nKey: MenuTitleKey
  path: string | null
  viewKey: RouteViewKey | null
  icon: MenuIconKey | null
  sortOrder: number
  isEnabled: YesNo
}
```

Initialize nullable fields explicitly. Compute valid parents from the current tree and exclude the editing subtree using an iterative ID set. Do not use `Record<string, unknown>`, type suppression, or convert null to 0.

- [x] **Step 4: Implement form submission and confirmations**

Call only:

```ts
await createMenu({ parentId, menuType, code, i18nKey, path, viewKey, icon, sortOrder, isEnabled })
await updateMenu(id, { parentId, menuType, i18nKey, path, viewKey, icon, sortOrder })
await updateMenuStatus(id, nextValue)
await deleteMenu(id)
await loadMenus()
```

Use translated `ElMessageBox.confirm` text before subtree disable/delete. On success close the drawer where relevant, reload only the management tree, and show an `ElNotification.success` manual-refresh notice. Do not optimistically mutate the tree.

- [x] **Step 5: Run interaction tests and forbidden-pattern checks**

Run:

```powershell
pnpm vitest run src/views/system/menus/index.test.ts
rg -n "getAccess|access\.load|window\.location\.reload|\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src/views/system/menus src/api/menu.ts src/api/menu.contract.ts -g "*.ts" -g "*.vue"
```

Expected: tests PASS and the scan has no output.

### Task 12: Complete cross-module verification and handoff

**Files:**
- Test: `server/internal/module/menu/...`
- Test: `server/cmd/api/...`
- Test: `web/src/access/...`
- Test: `web/src/api/menu...`
- Test: `web/src/views/system/menus/...`

No source modification is planned in this verification task. If a command fails, return to the task that owns that behavior and make only the required correction.

- [x] **Step 1: Re-read the approved spec against the implementation diff**

Check each requirement:

```text
schema: deleted_at, TIMESTAMPTZ, Yes/No, page-path uniqueness
tree: directory/page/action matrix, null root, no cycles/orphans
core: five rows, idempotent startup, corruption failure, immutable protection
mutation: candidate validation, subtree disable, parent-chain enable, transactional delete
API: five routes, exact permissions, strict DTOs, code/data/message
frontend: registered title/view/icon, typed DTOs, Element Plus tree, local form state
refresh: only GET /menus after mutation, no /access reload or hot route update
scope: no user/role/Redis/generic CRUD implementation
```

- [x] **Step 2: Run complete backend verification**

From `D:\admin\server`:

```powershell
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

Expected: every command exits 0. PostgreSQL tests use the configured real instance; never substitute SQLite or an in-memory fake.

- [x] **Step 3: Run complete frontend verification**

From `D:\admin\web`:

```powershell
pnpm vitest run
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src -g "*.ts" -g "*.vue"
```

Expected: Vitest/build exit 0 and the AnyScript scan has no new matches. Identify any pre-existing unrelated match rather than hiding it.

- [x] **Step 4: Run repository hygiene checks**

From `D:\admin`:

```powershell
git diff --check
git status --short
```

Expected: no whitespace errors. Preserve unrelated changes and do not commit/push/fetch/pull or manage services.

- [x] **Step 5: Report verified evidence and remaining risk**

List changed files, exact command results, skipped verification and its reason, and residual risks. State that backend permission checks see committed menu changes immediately, while the current browser sidebar and dynamic-route snapshot change only after manual refresh.

## Spec coverage review

This plan covers schema/indexes (Task 1), stable errors/protocol (Task 2), core initialization (Task 3), PostgreSQL primitives (Task 4), tree validation/mutations (Tasks 5-6), strict API and permissions (Task 7), frontend registries/i18n (Task 8), typed API (Task 9), tree UI and form workflow (Tasks 10-11), operator/user refresh behavior (Task 11), and full acceptance verification (Task 12). It does not introduce user/role management, Redis caching, a generic CRUD/tree abstraction, or a fallback path.
