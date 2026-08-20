# Role Management Phase One Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Execute inline on `main`; do not dispatch subagents and do not create commits without explicit repository-owner instruction.

**Goal:** Deliver the approved role-management and role-authorization foundation so administrators can maintain roles, switch the future-user default role, and assign normalized page/action permissions without weakening the existing RBAC invariants.

**Architecture:** Preserve the linear backend path `router -> authentication middleware -> permission middleware -> role Handler -> role Service -> role Repository -> model -> PostgreSQL`. The role Service owns system-role protection, default-role invariants, mutation order, and permission normalization; the concrete Repository owns PostgreSQL reads, locks, transactions, and writes. Frontend data remains `views/system/roles/index.vue -> api/role.ts -> utils/request.ts -> role API`, with runtime parsing from `unknown`, page-local state, and no generic CRUD framework.

**Tech Stack:** Go 1.26, Gin, GORM, PostgreSQL and real PostgreSQL integration tests; Vue 3, strict TypeScript, Element Plus, vue-i18n, Pinia access state, Vitest, Vue Test Utils, pnpm.

---

## Existing Boundaries To Preserve

- `server/internal/module/role/model.go` already maps `sys_role` and `sys_user_role` with explicit `TIMESTAMPTZ`, `gorm.DeletedAt`, and shared Yes/No values.
- `server/internal/module/role/repository.go` is already consumed by authentication registration and bootstrap-admin. Keep `FindDefault`, `FindByCode`, and `HasActiveUserWithRole` compatible while moving management rules into the role Service.
- `server/internal/module/menu` owns `sys_menu`, `sys_role_menu`, menu tree validation, built-in menu initialization, and the finite menu title/view/icon protocol.
- To avoid a `menu <-> role` import cycle, the seven role-management permission-code constants live in `menu/protocol.go`; the role router imports those constants. System role codes and role field rules remain in `role/protocol.go`.
- `server/internal/module/access` remains the only runtime permission-union and ancestor-derivation implementation. Role authorization stores direct page/action grants only and does not create a second access algorithm.
- `server/internal/shared/pagination` and `web/src/types/pagination.ts` are the existing pagination contracts and must be reused.
- `server/internal/shared/validate.BindJSON` and `RequireEmptyBody` provide strict body handling. Role list query allowlisting is explicit in the role Handler because Gin binding alone does not reject unknown query keys.
- `web/src/utils/request.ts` already validates the exact `code/data/message` envelope, notifies API/protocol errors, and performs 401 refresh coordination. Role code must not duplicate that behavior.
- `web/src/views/system/menus/index.vue` is the local reference for permission-gated commands, error preservation, Element Plus interaction, and the manual-refresh access policy.

The repository owner controls Git history and running services. Every task ends with focused verification and a diff review; this plan never commits, pushes, fetches, pulls, starts, or stops a service.

### Task 1: Move Role Schema Ownership Into The Role Module

**Files:**
- Create: `server/internal/module/role/schema.go`
- Create: `server/internal/module/role/schema_test.go`
- Modify: `server/internal/module/auth/schema.go`
- Modify: `server/internal/module/auth/schema_test.go`
- Modify: `server/internal/module/role/repository_test.go`
- Modify: `server/internal/module/user/repository_test.go`
- Modify: `server/internal/module/access/repository_test.go`
- Modify: `server/internal/module/auth/session_repository_test.go`
- Modify: `server/cmd/api/main.go`

- [ ] **Step 1: Write failing ownership and PostgreSQL schema assertions**

Create `role/schema_test.go` following the SQL assertions in `auth/schema_test.go`; because those helpers are private to `auth_test`, keep small package-local inspection helpers in `role/schema_test.go`. Assert these exact role-owned objects:

```text
ux_sys_role_code_active       UNIQUE (code) WHERE deleted_at IS NULL
ux_sys_role_name_active       UNIQUE (name) WHERE deleted_at IS NULL
ux_sys_role_default_active    UNIQUE (is_default) WHERE is_default = 1 AND deleted_at IS NULL
ux_sys_user_role_active       UNIQUE (user_id, role_id) WHERE deleted_at IS NULL
fk_sys_user_role_user         FOREIGN KEY user_id -> sys_user(id) ON DELETE RESTRICT
fk_sys_user_role_role         FOREIGN KEY role_id -> sys_role(id) ON DELETE RESTRICT
ck_sys_role_is_default        CHECK is_default IN (0, 1)
ck_sys_role_is_enabled        CHECK is_enabled IN (0, 1)
```

Update `auth/schema_test.go` so authentication owns only user/session objects. Add a source-level assertion that `auth/schema.go` no longer contains `ux_sys_role_`, `ux_sys_user_role_`, or `fk_sys_user_role_`.

- [ ] **Step 2: Run the schema tests and confirm the role ownership test is red**

Run from `D:\admin\server`:

```powershell
go test ./internal/module/role ./internal/module/auth -run 'Test(Role|Authentication)Schema' -count=1
```

Expected: FAIL because `role.EnsureSchema` and `ux_sys_role_name_active` do not exist yet. A PostgreSQL connection/configuration failure is an environment failure, not the expected red state.

- [ ] **Step 3: Implement the idempotent role schema**

Create this public boundary in `role/schema.go`:

```go
func EnsureSchema(ctx context.Context, db *gorm.DB) error
```

It must reject a nil database, call `WithContext(ctx)`, create the four indexes above with `IF NOT EXISTS`, and inspect `pg_constraint` before adding each named foreign key. The Yes/No CHECK constraints remain the model/`AutoMigrate` contract; `EnsureSchema` must not duplicate them. Move, rather than duplicate, role/user-role index and foreign-key DDL out of `auth/schema.go`. Do not drop or recreate existing objects and do not introduce a schema manager.

- [ ] **Step 4: Wire schema order and update test fixtures**

In `server/cmd/api/main.go`, call schema ownership in this order after `AutoMigrate`:

```go
if err := role.EnsureSchema(processContext, postgres.GORM); err != nil {
    return fmt.Errorf("ensure role schema: %w", err)
}
if err := auth.EnsureSchema(processContext, postgres.GORM); err != nil {
    return fmt.Errorf("ensure authentication schema: %w", err)
}
if err := menu.EnsureSchema(processContext, postgres.GORM); err != nil {
    return fmt.Errorf("ensure menu schema: %w", err)
}
```

Every PostgreSQL fixture that currently relies on `auth.EnsureSchema` for role constraints must explicitly call `role.EnsureSchema` first. Do not make bootstrap-admin run migrations: it continues to require an initialized API database.

- [ ] **Step 5: Run focused schema verification**

Run:

```powershell
go test ./internal/module/role ./internal/module/auth ./internal/module/user ./internal/module/access -run 'Test(Role|Authentication)Schema|TestFindDefault|TestRepository' -count=1
git diff --check
```

Expected: PASS; role objects are created once by `role.EnsureSchema`, authentication tests still pass, and the diff has no whitespace errors.

### Task 2: Define The Finite Role Protocol And Localized Errors

**Files:**
- Create: `server/internal/module/role/protocol.go`
- Create: `server/internal/module/role/errors.go`
- Create: `server/internal/module/role/errors_test.go`
- Modify: `server/internal/shared/i18n/catalog.go`
- Modify: `server/internal/shared/i18n/catalog_test.go`

- [ ] **Step 1: Write failing protocol and error-contract tests**

Test these system-role constants and helpers:

```go
const (
    CodeSuperAdmin     = "super_admin"
    CodeRegisteredUser = "registered_user"
)

func IsSystemCode(string) bool
func IsValidCode(string) bool
```

`IsValidCode` must match `^[a-z][a-z0-9_]{2,63}$`; `IsSystemCode` accepts exactly the two fixed codes. Test this exact business error table:

```text
15000 role not found                 404
15001 role code conflict             409
15002 role name conflict             409
15003 system role protected          409
15004 default role protected         409
15005 role has effective users       409
15006 invalid role state             400
15007 invalid permission menu        400
15008 super admin authorization      409
15009 role/default/grant corruption  500
```

Each test asserts HTTP status, business code, message key, exact parameter names, and preservation of the internal cause through `errors.Is`.

- [ ] **Step 2: Run the tests and verify missing symbols fail compilation**

Run:

```powershell
go test ./internal/module/role ./internal/shared/i18n -run 'Test(Role|Catalog)' -count=1
```

Expected: FAIL for absent protocol/error definitions.

- [ ] **Step 3: Implement role protocol and error constructors**

Keep finite constants and validation in `role/protocol.go`. Implement concrete constructors in `role/errors.go`, following `menu/errors.go`, including parameterized conflicts:

```go
func roleNotFound(cause error) *apperror.Error
func roleCodeConflict(code string, cause error) *apperror.Error
func roleNameConflict(name string, cause error) *apperror.Error
func roleSystemProtected(code string, cause error) *apperror.Error
func roleDefaultProtected(code string, cause error) *apperror.Error
func roleUsersAttached(code string, cause error) *apperror.Error
func roleInvalidState(cause error) *apperror.Error
func roleInvalidPermission(cause error) *apperror.Error
func roleSuperAdminAuthorization(cause error) *apperror.Error
func roleDataInvalid(cause error) *apperror.Error
```

Do not expose SQL/table/cause text through response parameters.

- [ ] **Step 4: Add matching Chinese and English backend catalogs**

Add identical keys and parameter sets:

```text
role.notFound
role.codeConflict          {{code}}
role.nameConflict          {{name}}
role.systemProtected       {{code}}
role.defaultProtected      {{code}}
role.usersAttached         {{code}}
role.invalidState
role.invalidPermission
role.superAdminAuthorization
role.dataInvalid
```

Keep `ValidateCatalogs` strict; do not add fallback translations.

- [ ] **Step 5: Run focused protocol/i18n tests**

Run:

```powershell
go test ./internal/module/role ./internal/shared/i18n -run 'Test(Role|Catalog)' -count=1
git diff --check
```

Expected: PASS with codes `15000` through `15009` stable and both catalogs structurally equal.

### Task 3: Make System-Role Bootstrap Preserve The Default-Role Invariant

**Files:**
- Modify: `server/internal/module/role/repository.go`
- Create: `server/internal/module/role/service.go`
- Modify: `server/internal/module/role/repository_test.go`
- Create: `server/internal/module/role/service_test.go`
- Modify: `server/cmd/api/main.go`

- [ ] **Step 1: Add failing startup-integrity tests**

Cover all of these states with real PostgreSQL transactions:

```text
neither fixed role exists                 -> create both in one transaction
both fixed roles are valid                -> idempotent, preserve IDs/timestamps
only super_admin exists                   -> fail, create nothing
only registered_user exists               -> fail, create nothing
either fixed code has soft-delete history -> fail
either fixed code has duplicate history   -> fail
fixed name changed                         -> fail
super_admin disabled or default            -> fail
registered_user non-default and disabled   -> valid
custom enabled default selected            -> valid and not reset
custom default exists before fixed roles   -> fail atomically; do not create fixed roles
zero or multiple active defaults           -> fail
default role disabled or super_admin       -> fail
```

For the “only one exists” cases, assert the transaction does not silently insert the missing role.

- [ ] **Step 2: Run the startup tests and confirm current behavior is red**

Run:

```powershell
go test ./internal/module/role -run 'TestEnsureSystemRoles|TestSystemRole' -count=1
```

Expected: FAIL because the current Repository silently creates a missing fixed role and forces `registered_user` to remain default/enabled.

- [ ] **Step 3: Add repository primitives without business branching**

Keep these concrete operations in `repository.go`:

```go
func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error
func (r *Repository) LockRoleTable(ctx context.Context) error
func (r *Repository) FindSystemRoleRecords(ctx context.Context) ([]Role, error) // Unscoped
func (r *Repository) LockActiveRoles(ctx context.Context) ([]Role, error)       // id ASC, FOR UPDATE
func (r *Repository) Create(ctx context.Context, value *Role) error
```

`LockRoleTable` uses `LOCK TABLE sys_role IN SHARE ROW EXCLUSIVE MODE` for startup serialization. All methods pass `ctx`, wrap causes, and do not decide which state is valid.

- [ ] **Step 4: Implement Service-owned startup validation**

Add:

```go
type Service struct {
    repository *Repository
}

func NewService(repository *Repository) *Service
func (s *Service) EnsureSystemRoles(ctx context.Context) error
```

Inside one transaction: lock the table; read both fixed codes Unscoped; create both only when neither has any history; otherwise require exactly one active record for each fixed code and no history; validate fixed names and `super_admin`; then validate exactly one active, enabled, non-super-admin default across all active roles. Never rename, restore, enable, clear, or guess data.

- [ ] **Step 5: Switch API startup to the role Service**

Construct one Repository and one Service:

```go
roleRepository := role.NewRepository(postgres.GORM)
roleService := role.NewService(roleRepository)
if err := roleService.EnsureSystemRoles(processContext); err != nil {
    return fmt.Errorf("ensure system roles: %w", err)
}
```

Keep `FindDefault`, `FindByCode`, and `HasActiveUserWithRole` on the Repository for existing auth/bootstrap consumers. Remove the old Repository-owned `EnsureSystemRoles` after updating tests and callers.

- [ ] **Step 6: Verify startup and authentication compatibility**

Run:

```powershell
go test ./internal/module/role ./internal/module/auth ./internal/module/user ./internal/module/access ./cmd/api -count=1
git diff --check
```

Expected: PASS; registration still resolves the current enabled default role, bootstrap-admin still finds `super_admin`, and startup no longer overwrites an administrator-selected default.

### Task 4: Extend The Core Menu Protocol With Role-Management Permissions

**Files:**
- Modify: `server/internal/module/menu/protocol.go`
- Modify: `server/internal/module/menu/builtin.go`
- Modify: `server/internal/module/menu/builtin_test.go`
- Modify: `server/internal/module/menu/service_test.go`
- Modify: `server/internal/module/access/repository_test.go`

- [ ] **Step 1: Write failing finite-protocol and built-in-tree assertions**

Add exact permission constants to `menu/protocol.go` tests:

```go
const (
    PermissionRoleList      = "system:role:list"
    PermissionRoleCreate    = "system:role:create"
    PermissionRoleUpdate    = "system:role:update"
    PermissionRoleStatus    = "system:role:status"
    PermissionRoleDefault   = "system:role:default"
    PermissionRoleDelete    = "system:role:delete"
    PermissionRoleAuthorize = "system:role:authorize"
)
```

Extend the expected core tree to 12 nodes: existing `system` plus menu-management nodes plus role page/action nodes. Assert `/system/roles`, `system-roles`, `navigation.systemRoles`, `UserFilled`, sort `20`, and action sorts `10..60` exactly.

- [ ] **Step 2: Run the menu/access tests and verify the protocol is red**

Run:

```powershell
go test ./internal/module/menu ./internal/module/access -run 'TestEnsureBuiltin|TestRepositoryBuiltin' -count=1
```

Expected: FAIL because the role-management built-ins and registry values are absent.

- [ ] **Step 3: Extend the finite menu registries and definitions**

Add these values without creating another initializer:

```text
title: navigation.systemRoles
titles: permission.roleCreate, roleUpdate, roleStatus, roleSetDefault, roleDelete, roleAuthorize
view: system-roles
icon: UserFilled
```

Append the role page followed by its six action definitions to `builtinMenuDefinitions()`. Include all seven codes in `builtinCodes`; core-node edit/delete/disable protection then applies automatically. Preserve the current rule that later startup does not overwrite operator-adjustable icon/sort values.

- [ ] **Step 4: Prove super-admin access and ordinary direct grants**

Extend access PostgreSQL tests:

```text
super_admin with zero sys_role_menu rows -> receives role page and all six role actions
ordinary role granted system:role:list   -> receives role page and system directory only
ordinary role granted role action        -> receives that action, role page, and system directory
```

Do not modify access production algorithms unless a test exposes a real defect; the existing super-admin bypass and ancestor derivation should satisfy these cases.

- [ ] **Step 5: Run focused core-RBAC verification**

Run:

```powershell
go test ./internal/module/menu ./internal/module/access -count=1
git diff --check
```

Expected: PASS with exact built-in initialization and no direct `sys_role_menu` grants for `super_admin`.

### Task 5: Implement Stable Role Listing And Strict Filters

**Files:**
- Modify: `server/internal/module/role/repository.go`
- Modify: `server/internal/module/role/service.go`
- Modify: `server/internal/module/role/repository_test.go`
- Modify: `server/internal/module/role/service_test.go`

- [ ] **Step 1: Define the list types and write failing tests**

Add these Service-facing types:

```go
type ListQuery struct {
    Page      int
    PageSize  int
    Keyword   string
    IsEnabled *yesno.Value
}

type ListItem struct {
    ID              int64
    Code            string
    Name            string
    IsDefault       yesno.Value
    IsEnabled       yesno.Value
    UserCount       int64
    PermissionCount int64
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

Write real PostgreSQL tests for count/query filter parity, `created_at ASC, id ASC`, literal `%` and `_`, enabled filtering, empty pages as non-nil slices, disabled-user counting, exclusion of soft-deleted users/relations, direct grant counts including disabled menus, and exclusion of deleted menus/grants.

- [ ] **Step 2: Run list tests and confirm missing methods fail**

Run:

```powershell
go test ./internal/module/role -run 'TestRepositoryList|TestServiceList' -count=1
```

Expected: compile failure for absent list methods.

- [ ] **Step 3: Add count and page queries to the Repository**

Use one private filter builder and expose:

```go
func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error)
func (r *Repository) List(ctx context.Context, query ListQuery) ([]ListItem, error)
```

Both queries filter active roles identically. Escape backslash, `%`, and `_` and use `ILIKE ? ESCAPE '\\'` against `code` and `name`. Count users through active `sys_user_role` plus non-deleted users regardless of user enabled state. Count active role-menu rows joined to non-deleted page/action menus regardless of menu enabled state. Aggregate before pagination so joins cannot duplicate rows.

- [ ] **Step 4: Implement Service validation and public pagination result**

Add:

```go
func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[ListItem], error)
```

Require `page >= 1`, `1 <= pageSize <= 100`, valid optional Yes/No, and a trimmed keyword of at most 64 Unicode characters. Call `Count` and `List` with the same normalized query. Return `List: make([]ListItem, 0)` for no rows; do not turn dependency failures into empty success.

- [ ] **Step 5: Run focused list verification**

Run:

```powershell
go test ./internal/module/role -run 'TestRepositoryList|TestServiceList' -count=1
git diff --check
```

Expected: PASS with stable pagination metadata and exact counts.

### Task 6: Implement Role Creation And Name Editing

**Files:**
- Modify: `server/internal/module/role/repository.go`
- Modify: `server/internal/module/role/service.go`
- Modify: `server/internal/module/role/repository_test.go`
- Modify: `server/internal/module/role/service_test.go`

- [ ] **Step 1: Write failing creation and update tests**

Define:

```go
type CreateInput struct {
    Code string
    Name string
}

type UpdateInput struct {
    Name string
}
```

Cover valid creation, code regex boundaries, Unicode name length `1..64`, surrounding-space normalization, reserved system-code rejection, `is_default=0`, `is_enabled=1`, zero initial grants, immutable code, system-role rename protection, same-name idempotency, missing/deleted role, and code/name conflicts including PostgreSQL constraint-name mapping.

- [ ] **Step 2: Run profile tests and confirm they fail**

Run:

```powershell
go test ./internal/module/role -run 'Test(Service|Repository)(Create|Update)' -count=1
```

Expected: compile failure for absent profile mutation methods.

- [ ] **Step 3: Add precise Repository writes and conflict mapping**

Expose:

```go
var ErrRoleCodeConflict = errors.New("active role code conflicts")
var ErrRoleNameConflict = errors.New("active role name conflicts")

func (r *Repository) LockActiveRole(ctx context.Context, id int64) (Role, error)
func (r *Repository) UpdateName(ctx context.Context, id int64, name string, updatedAt time.Time) error
```

Reuse `Create` from Task 3. Inspect `*pgconn.PgError.ConstraintName` for `ux_sys_role_code_active` and `ux_sys_role_name_active`; preserve all other causes. `UpdateName` explicitly sets `updated_at` and requires exactly one active row.

- [ ] **Step 4: Implement minimal Service rules**

Add:

```go
func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error)
func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error
```

Normalize with `strings.TrimSpace`, validate by rune count, reject fixed codes, set exact initial Yes/No values, and map repository conflicts to `15001/15002`. Update locks the target in a transaction, rejects both system roles, returns success without writing for an identical name, and never accepts a code field.

- [ ] **Step 5: Run focused profile verification**

Run:

```powershell
go test ./internal/module/role -run 'Test(Service|Repository)(Create|Update)' -count=1
git diff --check
```

Expected: PASS; creation adds no role-menu rows and edits cannot alter code/status/default state.

### Task 7: Implement Transactional Status And Default-Role Changes

**Files:**
- Modify: `server/internal/module/role/repository.go`
- Modify: `server/internal/module/role/service.go`
- Modify: `server/internal/module/role/repository_test.go`
- Modify: `server/internal/module/role/service_test.go`
- Modify: `server/internal/module/auth/service_test.go`

- [ ] **Step 1: Write failing state-transition tests**

Cover:

```text
disable super_admin                         -> 15003
disable current default                     -> 15004
disable non-default registered_user         -> success
disable custom role with users/grants       -> success, preserve both relations
enable disabled role                        -> success, restore access participation
same status                                 -> idempotent without timestamp rewrite
set enabled non-super role as default        -> exactly one default
set disabled/deleted/missing role            -> 15006/15000
set super_admin as default                   -> 15003
set same default                             -> idempotent
set zero-permission role as default           -> success
concurrent default changes                    -> serialize, exactly one winner state
injected second default update failure        -> full rollback
```

Add an auth registration test proving a user created after a switch receives the new default, while pre-existing user-role rows remain unchanged.

- [ ] **Step 2: Run state tests and confirm methods are absent**

Run:

```powershell
go test ./internal/module/role ./internal/module/auth -run 'Test.*(Status|Default|Register.*Default)' -count=1
```

Expected: compile failure for state mutation methods.

- [ ] **Step 3: Add exact Repository update primitives**

Expose:

```go
func (r *Repository) UpdateStatus(ctx context.Context, id int64, value yesno.Value, updatedAt time.Time) error
func (r *Repository) ClearDefault(ctx context.Context, id int64, updatedAt time.Time) error
func (r *Repository) SetDefault(ctx context.Context, id int64, updatedAt time.Time) error
```

All run on the transaction-bound Repository, write explicit columns/timestamps, and verify affected-row counts. Default switching first uses `LockActiveRoles` ordered by ID; it does not issue unordered per-role locks.

- [ ] **Step 4: Implement Service state rules**

Add:

```go
func (s *Service) UpdateStatus(ctx context.Context, id int64, value yesno.Value) error
func (s *Service) SetDefault(ctx context.Context, id int64) error
```

Status changes lock the target, enforce fixed/default protection, preserve user-role and role-menu rows, and treat same state as no-op success. Default changes lock all active roles, validate the full current default invariant before mutation, reject `super_admin` and disabled targets, clear only the old default, set only the target, then validate the in-memory result has exactly one enabled default before commit.

- [ ] **Step 5: Verify authentication semantics and state transactions**

Run:

```powershell
go test ./internal/module/role ./internal/module/auth ./internal/module/access -run 'Test.*(Status|Default|Register|DisabledRole)' -count=1
git diff --check
```

Expected: PASS. Disabling a user's last enabled role causes existing authentication/session checks to reject the user on the next request with 401; no session revocation or browser refresh is added here.

### Task 8: Implement Protected Transactional Role Deletion

**Files:**
- Modify: `server/internal/module/role/repository.go`
- Modify: `server/internal/module/role/service.go`
- Modify: `server/internal/module/role/repository_test.go`
- Modify: `server/internal/module/role/service_test.go`

- [ ] **Step 1: Write failing deletion and rollback tests**

Use real PostgreSQL transactions to cover:

```text
delete missing/deleted role                    -> 15000
delete super_admin or registered_user          -> 15003
delete current default                         -> 15004
active user-role + enabled user                -> 15005
active user-role + disabled user               -> 15005
active relation + soft-deleted user             -> does not block after user workflow cleanup rule
soft-deleted user-role relation                 -> does not block
unbound custom role                             -> soft-delete role and active grants
deleted role keeps historical user-role rows    -> no physical delete/migration
grant deletion failure                          -> role remains active
role deletion failure after grant soft-delete   -> grant deletion rolls back
```

For the second rollback case, install a temporary transaction-local constraint that rejects role `deleted_at` changes after role-menu writes, then assert all rows remain active after rollback.

- [ ] **Step 2: Run deletion tests and verify they fail**

Run:

```powershell
go test ./internal/module/role -run 'Test.*Delete' -count=1
```

Expected: compile failure for absent deletion methods.

- [ ] **Step 3: Add Repository count and soft-delete primitives**

Expose:

```go
func (r *Repository) CountEffectiveUsers(ctx context.Context, roleID int64) (int64, error)
func (r *Repository) SoftDeleteRoleMenus(ctx context.Context, roleID int64, deletedAt time.Time) error
func (r *Repository) SoftDeleteRole(ctx context.Context, roleID int64, deletedAt time.Time) error
```

`CountEffectiveUsers` joins active relations to non-deleted users and deliberately does not filter user `is_enabled`. Both soft-delete writes set the same UTC value into `updated_at` and `deleted_at`, operate only on active rows, and never physically delete.

- [ ] **Step 4: Implement ordered Service deletion**

Add:

```go
func (s *Service) Delete(ctx context.Context, id int64) error
```

Inside one transaction: lock active role; reject both fixed roles; reject current default; count effective users and reject if nonzero; obtain one `deletedAt := time.Now().UTC()`; soft-delete active role-menu rows; soft-delete the role. Do not migrate users, assign the default, revoke sessions, or delete historical relations.

- [ ] **Step 5: Run focused deletion verification**

Run:

```powershell
go test ./internal/module/role -run 'Test.*Delete' -count=1
git diff --check
```

Expected: PASS with timestamp equality and both rollback paths proven.

### Task 9: Implement Direct-Grant Query And Minimal-Set Authorization

**Files:**
- Modify: `server/internal/module/role/repository.go`
- Modify: `server/internal/module/role/service.go`
- Create: `server/internal/module/role/permission.go`
- Create: `server/internal/module/role/permission_test.go`
- Modify: `server/internal/module/role/repository_test.go`
- Modify: `server/internal/module/role/service_test.go`

- [ ] **Step 1: Define authorization types and write failing algorithm tests**

Use role-owned response types while reusing `menu.Type`:

```go
type Summary struct {
    ID        int64
    Code      string
    Name      string
    IsDefault yesno.Value
    IsEnabled yesno.Value
}

type PermissionTreeNode struct {
    ID        int64
    ParentID  *int64
    MenuType  menu.Type
    Code      string
    I18nKey   string
    IsEnabled yesno.Value
    Children  []PermissionTreeNode
}

type Permissions struct {
    Role     Summary
    MenuTree []PermissionTreeNode
    MenuIDs  []int64
}
```

Write pure algorithm tests for a multi-root tree, stable sibling order, duplicate IDs/codes, roots that are not directories, orphans, cycles, illegal parent-child types, invalid stored values, and these normalization cases:

```text
[page]                         -> [page]
[action]                       -> [action]
[page, action-of-page]         -> [action]
[page, action-a, action-b]     -> [action-a, action-b]
[page-with-no-selected-action] -> [page]
[]                             -> []
```

Input duplicate IDs are invalid rather than silently deduplicated. Directory input is invalid rather than silently removed by the backend.

- [ ] **Step 2: Write failing PostgreSQL authorization tests**

Cover query/save behavior:

```text
tree includes enabled and disabled non-deleted menus
tree excludes soft-deleted menus
disabled role can query and save grants
super_admin query/save rejected with 15008
grant to directory/missing/deleted menu rejected with 15007
grant to disabled page/action accepted
stored grant to directory/missing/deleted menu causes 15009 on query/save
unchanged active grant keeps ID/created_at/updated_at
removed grant receives updated_at == deleted_at
re-adding a previously soft-deleted pair creates a new active history row
empty array clears every active direct grant
insert/write failure rolls back removals and role updated_at
```

- [ ] **Step 3: Run authorization tests and verify the red state**

Run:

```powershell
go test ./internal/module/role -run 'Test.*(Permission|Authorize|Grant)' -count=1
```

Expected: compile failure for absent types and methods.

- [ ] **Step 4: Add Repository lock/read/write primitives**

Expose transaction-bound methods:

```go
func (r *Repository) FindActiveRole(ctx context.Context, id int64) (Role, error)
func (r *Repository) LockActiveMenus(ctx context.Context) ([]menu.Menu, error)
func (r *Repository) FindActiveRoleMenus(ctx context.Context, roleID int64) ([]menu.RoleMenu, error)
func (r *Repository) SoftDeleteRoleMenuIDs(ctx context.Context, ids []int64, deletedAt time.Time) error
func (r *Repository) CreateRoleMenus(ctx context.Context, values []menu.RoleMenu) error
func (r *Repository) TouchRole(ctx context.Context, roleID int64, updatedAt time.Time) error
```

`LockActiveMenus` uses `FOR UPDATE` with stable `sort_order, code, id`, serializing authorization with menu delete/type changes. `FindActiveRoleMenus` orders by `menu_id, id`. Empty write slices return success without generating invalid SQL. Repository methods report PostgreSQL failures but never normalize authorization.

- [ ] **Step 5: Implement private menu-tree validation and normalization**

In `permission.go`, build `id -> menu`, `parent -> children`, roots, and the page owner for every action in O(n), then stable-sort siblings. Keep helpers private to package role:

```go
func buildPermissionIndex(rows []menu.Menu) (permissionIndex, error)
func (index permissionIndex) tree() ([]PermissionTreeNode, error)
func (index permissionIndex) normalizeRequested(ids []int64) ([]int64, error)
func (index permissionIndex) validateStored(grants []menu.RoleMenu) ([]int64, error)
```

Do not import or copy access snapshot behavior: this algorithm validates the management tree and minimizes direct grants; access remains responsible for runtime ancestors and multi-role union.

- [ ] **Step 6: Implement query and save Service methods**

Add:

```go
func (s *Service) Permissions(ctx context.Context, roleID int64) (Permissions, error)
func (s *Service) UpdatePermissions(ctx context.Context, roleID int64, menuIDs []int64) (int64, error)
```

Query reads the role, rejects `super_admin`, loads all active menus and grants, fails on corruption, and returns non-nil sorted `MenuTree`/`MenuIDs`.

Save uses one transaction: lock role; reject `super_admin`; lock active menus; validate/normalize request; read and validate current grants; compute stable set differences; soft-delete removed rows; create only additions; preserve unchanged rows; touch the role; return the normalized direct-grant count. An unchanged normalized set is idempotent success without rewriting grants or role timestamp.

- [ ] **Step 7: Run authorization and access-regression verification**

Run:

```powershell
go test ./internal/module/role ./internal/module/menu ./internal/module/access -run 'Test.*(Permission|Authorize|Grant|Snapshot|Allowed)' -count=1
git diff --check
```

Expected: PASS. Direct grants are minimal, runtime access still derives page/directory ancestors, and a disabled menu's retained grant becomes effective again when the menu is re-enabled.

### Task 10: Expose Strict Role HTTP Endpoints And Wire The API

**Files:**
- Create: `server/internal/module/role/request.go`
- Create: `server/internal/module/role/response.go`
- Create: `server/internal/module/role/handler.go`
- Create: `server/internal/module/role/handler_test.go`
- Create: `server/internal/module/role/route.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`

- [ ] **Step 1: Write failing Handler contract tests**

Create a typed fake `roleHTTPService` matching:

```go
type roleService interface {
    List(context.Context, ListQuery) (pagination.Result[ListItem], error)
    Create(context.Context, CreateInput) (int64, error)
    Update(context.Context, int64, UpdateInput) error
    UpdateStatus(context.Context, int64, yesno.Value) error
    SetDefault(context.Context, int64) error
    Delete(context.Context, int64) error
    Permissions(context.Context, int64) (Permissions, error)
    UpdatePermissions(context.Context, int64, []int64) (int64, error)
}
```

Assert exact closed response fields, UTC RFC3339Nano times, `[]` instead of `null`, all eight routes, request-context propagation, authentication-before-permission order, and exact permission constants from `menu/protocol.go`.

- [ ] **Step 2: Add strict request failure cases**

Test `400 / 10001` without Service calls for:

```text
missing/zero/negative/non-integer page or pageSize
pageSize > 100
unknown or repeated query key
isEnabled outside 0/1 or repeated
keyword repeated or longer than 64 runes after trim
role ID 0, negative, overflow, or non-integer
missing JSON fields, wrong types, unknown fields, snake_case, msg, trailing JSON
status without isEnabled or outside 0/1
permissions without menuIds, null, duplicates, zero/negative/fractional IDs
default/delete with any body bytes, including {}
```

Success mutation data is exact:

```json
create:      {"id":8}
update:      {}
status:      {"id":8,"isEnabled":0}
default:     {"id":8,"isDefault":1}
delete:      {}
permissions: {"id":8,"permissionCount":3}
```

- [ ] **Step 3: Run Handler tests and confirm missing HTTP files fail**

Run:

```powershell
go test ./internal/module/role ./cmd/api -run 'Test(RoleHandler|RoleRoutes|BuildRouter)' -count=1
```

Expected: compile failure for missing Handler/routes.

- [ ] **Step 4: Implement explicit requests and responses**

Use pointer/presence fields for JSON values that may legitimately be zero or empty. Use `validate.BindJSON` for mutation bodies and `validate.RequireEmptyBody` for default/delete. Implement a role-specific query parser that checks `context.Request.URL.Query()` against exactly `page`, `pageSize`, `keyword`, and `isEnabled`, rejects more than one value per key, then builds `ListQuery`.

Response constructors must return only the approved fields. Build every `children`, `menuTree`, `menuIds`, and paged `list` as a non-nil slice; never mask a Service error with an empty response.

- [ ] **Step 5: Register routes with exact permission middleware**

Implement:

```go
func RegisterRoutes(
    routes *gin.RouterGroup,
    handler *Handler,
    authenticate gin.HandlerFunc,
    requirePermission func(string) gin.HandlerFunc,
)
```

Bind:

```text
GET    /roles                  menu.PermissionRoleList
POST   /roles                  menu.PermissionRoleCreate
PUT    /roles/:id              menu.PermissionRoleUpdate
PATCH  /roles/:id/status       menu.PermissionRoleStatus
PATCH  /roles/:id/default      menu.PermissionRoleDefault
DELETE /roles/:id              menu.PermissionRoleDelete
GET    /roles/:id/permissions  menu.PermissionRoleAuthorize
PUT    /roles/:id/permissions  menu.PermissionRoleAuthorize
```

Handler parses HTTP only and never receives GORM. Service receives request context, never Gin context.

- [ ] **Step 6: Wire the shared role Service into API startup/router**

Add `Role *role.Handler` to `routerDependencies`, construct `role.NewHandler(roleService)`, and register after access/menu routes. Update `main_test.go` fake Service and route-count assertions for all eight paths; verify unauthenticated `/api/v1/roles?page=1&pageSize=20` returns 401 before list parsing reaches the Handler.

- [ ] **Step 7: Run complete backend verification for the vertical slice**

Run:

```powershell
go fmt ./...
go vet ./...
go test ./...
go build ./...
git diff --check
```

Expected: all commands exit `0`; real PostgreSQL transaction tests pass and every role endpoint uses exact envelope/error/permission behavior.

### Task 11: Register Role UI Protocol Values And Frontend I18n

**Files:**
- Modify: `web/src/access/menu-title-keys.ts`
- Modify: `web/src/access/menu-title-keys.test.ts`
- Modify: `web/src/access/menu-icons.ts`
- Modify: `web/src/access/protocol.test.ts`
- Modify: `web/src/access/route-views.ts`
- Modify: `web/src/api/access.contract.test.ts`
- Modify: `web/src/router/index.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Modify: `web/src/i18n/index.test.ts`

- [ ] **Step 1: Write failing finite-registry tests**

Assert exact additions:

```text
navigation.systemRoles
permission.roleCreate
permission.roleUpdate
permission.roleStatus
permission.roleSetDefault
permission.roleDelete
permission.roleAuthorize
system-roles -> views/system/roles/index.vue
UserFilled
```

Extend access/router fixtures to accept `system-roles` and reject unregistered role views/icons/titles. Keep registries finite; do not allow arbitrary imports or raw Element Plus component names.

- [ ] **Step 2: Run frontend protocol tests and confirm they are red**

Run from `D:\admin\web`:

```powershell
pnpm vitest run src/access src/api/access.contract.test.ts src/router/index.test.ts src/i18n/index.test.ts
```

Expected: FAIL for missing title/view/icon and locale keys.

- [ ] **Step 3: Extend registries and both locale files**

Import `UserFilled` into the existing icon object and add the role view with a literal dynamic import. Add all navigation/action titles plus page strings for columns, filters, dialogs, validation, errors, statuses, confirmations, tooltips, zero-permission warning, loading/empty states, and success notices.

Chinese and English must have identical complete key sets. Keep `fallbackLocale: false`; do not show raw i18n keys as fallback.

- [ ] **Step 4: Run protocol and locale verification**

Run:

```powershell
pnpm vitest run src/access src/api/access.contract.test.ts src/router/index.test.ts src/i18n/index.test.ts
pnpm build
```

Expected: PASS; the dynamic access snapshot can resolve the role page without a default component/icon/title.

### Task 12: Implement The Strict Frontend Role Contract And API

**Files:**
- Create: `web/src/api/role.contract.ts`
- Create: `web/src/api/role.contract.test.ts`
- Create: `web/src/api/role.ts`
- Create: `web/src/api/role.test.ts`

- [ ] **Step 1: Define public TypeScript types and failing parser tests**

Use these exact public types:

```ts
export interface RoleListQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
}

export interface RoleListItem {
  id: number
  code: string
  name: string
  isDefault: YesNo
  isEnabled: YesNo
  userCount: number
  permissionCount: number
  createdAt: string
  updatedAt: string
}

export type RolePermissionMenuType = 'directory' | 'page' | 'action'

export interface RolePermissionTreeNode {
  id: number
  parentId: number | null
  menuType: RolePermissionMenuType
  code: string
  i18nKey: MenuTitleKey
  isEnabled: YesNo
  children: RolePermissionTreeNode[]
}

export interface CreateRoleInput {
  code: string
  name: string
}

export interface UpdateRoleInput {
  name: string
}

export interface RoleStatusResult {
  id: number
  isEnabled: YesNo
}

export interface RoleDefaultResult {
  id: number
  isDefault: YesNo
}

export interface RolePermissionsResponse {
  role: {
    id: number
    code: string
    name: string
    isDefault: YesNo
    isEnabled: YesNo
  }
  menuTree: RolePermissionTreeNode[]
  menuIds: number[]
}

export interface UpdateRolePermissionsInput {
  menuIds: number[]
}

export interface RolePermissionResult {
  id: number
  permissionCount: number
}
```

`parseEmptyResult` must accept only `{}` and return a new empty object; it must reject `null`, arrays, and extra keys. `RolePermissionsResponse.menuIds` must be ascending and unique; its `menuTree` must be a non-null array containing all non-deleted directory/page/action nodes, including disabled ones. The nested role object is a closed record with exactly `id`, `code`, `name`, `isDefault`, and `isEnabled`.

Tests must reject missing/extra fields, unsafe/fractional/non-positive IDs, negative/fractional counts, invalid Yes/No, invalid timestamps, null arrays, duplicate IDs/codes/menuIds, unknown menu type/title, bad parent nesting, illegal child types, cycles represented through duplicated nesting, and unstable sibling/menuId order.

- [ ] **Step 2: Run contract tests and confirm missing module failure**

Run:

```powershell
pnpm vitest run src/api/role.contract.test.ts
```

Expected: FAIL because the module does not exist.

- [ ] **Step 3: Implement closed-record parsers from unknown**

Export:

```ts
export function parseRolePage(value: unknown): PageResult<RoleListItem>
export function parseRoleIDResult(value: unknown): { id: number }
export function parseEmptyResult(value: unknown): Record<string, never>
export function parseRoleStatusResult(value: unknown): RoleStatusResult
export function parseRoleDefaultResult(value: unknown): RoleDefaultResult
export function parseRolePermissions(value: unknown): RolePermissionsResponse
export function parseRolePermissionResult(value: unknown): RolePermissionResult
```

Use `Record<string, unknown>` only after a real object guard. Every parser checks exact keys; timestamps use the existing RFC3339 rule; trees validate parent/type/order and registered i18n keys. Do not use `as any`, optional chaining, `?? []`, or unchecked assertions to make malformed data pass.

- [ ] **Step 4: Write failing API request-shape tests**

Mock `request` and assert:

```text
GET roles sends params with required page/pageSize and only present filters
POST sends only code/name
PUT sends only name
PATCH status sends only isEnabled
PATCH default sends no data
DELETE sends no data
GET permissions sends no data
PUT permissions sends exactly menuIds
every response is parsed and malformed response rejects ProtocolError
```

- [ ] **Step 5: Implement the thin API functions**

Export:

```ts
getRoles(query: RoleListQuery)
createRole(input: CreateRoleInput)
updateRole(id: number, input: UpdateRoleInput)
updateRoleStatus(id: number, isEnabled: YesNo)
setDefaultRole(id: number)
deleteRole(id: number)
getRolePermissions(id: number)
updateRolePermissions(id: number, input: UpdateRolePermissionsInput)
```

Each function calls `request<unknown>` and immediately invokes its exact parser. Do not add access-store refresh, retries, notifications, or compatibility parsing in this file.

- [ ] **Step 6: Run API and TypeScript verification**

Run:

```powershell
pnpm vitest run src/api/role.contract.test.ts src/api/role.test.ts
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src/api/role.contract.ts src/api/role.ts src/api/role.contract.test.ts src/api/role.test.ts
```

Expected: tests/build PASS and the forbidden-`any` search returns no matches.

### Task 13: Build The Paged Role List And Profile Dialog

**Files:**
- Create: `web/src/views/system/roles/index.vue`
- Create: `web/src/views/system/roles/index.test.ts`

- [ ] **Step 1: Write failing list/load/filter/page tests**

Mount with Pinia, i18n, and stubs following the existing menu-page test style. Assert:

```text
on mount -> getRoles({page: 1, pageSize: 20}) exactly once
rows render all approved columns and UTC strings
keyword/status query resets page to 1
reset clears filters and requests page 1
pagination sends selected page/pageSize
refresh preserves current query
load error remains visible and does not replace prior rows with fake []
retry uses the current query
empty successful list renders the explicit empty state
```

The page must not call `getAccess`, `access.load`, `window.location.reload`, or router mutation APIs.

- [ ] **Step 2: Run the page tests and confirm the component is missing**

Run:

```powershell
pnpm vitest run src/views/system/roles/index.test.ts
```

Expected: FAIL because `index.vue` does not exist.

- [ ] **Step 3: Implement page-local list state and stable layout**

In `<script setup lang="ts">`, keep explicit refs:

```ts
const rows = ref<RoleListItem[]>([])
const total = ref(0)
const query = ref<RoleListQuery>({ page: 1, pageSize: 20 })
const loading = ref(false)
const loadError = ref('')
const mutationError = ref('')
```

`loadRoles` sets loading and clears only the current load error; it replaces rows/total only after a valid parsed success. The template uses an unframed toolbar/search band, `el-table`, and `el-pagination`, while Main remains the only page scrollbar. Use icon buttons with tooltips for compact row commands and Element Plus theme variables already present in the app.

- [ ] **Step 4: Write failing command-permission and profile-dialog tests**

Set access snapshots with each individual permission and assert:

```text
system:role:create  -> create command
system:role:update  -> edit command
system:role:status  -> status command
system:role:default -> set-default command
system:role:delete  -> delete command
system:role:authorize -> authorization command
```

Missing commands are not rendered. Test create dialog fields `code/name`, edit dialog read-only code plus editable name, code regex/name validation, submission loading, double-submit prevention, failed input preservation, create success returning to page 1, edit success preserving page/filter, and success refresh calling only `getRoles`.

- [ ] **Step 5: Implement the small Element Plus profile Dialog**

Use one local `el-dialog` around 520px for create/edit. Define an explicit form type:

```ts
interface RoleFormState {
  code: string
  name: string
}
```

Create sends `{code, name}`; edit sends `{name}` only. Normalize/validate before API calls but let backend enforce all rules. On API failure, keep the dialog and values. On success, close only after the list reload succeeds; if reload fails, preserve the management error rather than showing a fake successful table.

- [ ] **Step 6: Add protected-state tooltips and mutation confirmations**

Derive command state solely from response fields and fixed codes:

```text
super_admin: edit/status/default/delete/authorize unavailable
registered_user: edit/delete unavailable; authorization available
current default: disable/delete unavailable
disabled non-default role: enable and authorize available; set-default unavailable
custom role with userCount > 0: delete disabled with reason
```

Status/default/delete confirmations include `userCount`, future-user-only default semantics, and soft-deletion text. A zero-permission target (`permissionCount === 0`) receives the extra Dashboard-only warning before default assignment. Frontend disabling is UX only; no backend protection is removed.

- [ ] **Step 7: Run list/profile UI verification**

Run:

```powershell
pnpm vitest run src/views/system/roles/index.test.ts
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" src/views/system/roles src/api/role*
```

Expected: tests/build PASS and the forbidden-type search has no matches.

### Task 14: Add The Role Authorization Dialog And Mutation Workflows

**Files:**
- Modify: `web/src/views/system/roles/index.vue`
- Modify: `web/src/views/system/roles/index.test.ts`

- [ ] **Step 1: Write failing authorization loading/error tests**

Assert:

```text
authorize command opens a large dialog and calls getRolePermissions(id)
header shows exact role name/code
loading does not render a fake empty tree
load failure shows explicit error and retry
disabled menus remain visible and visibly disabled-status tagged
soft-deleted menus are absent because contract never receives them
super_admin has no authorization command
disabled role can open and save authorization
```

Opening one role after another must reset prior tree selection/error/loading state before the next response arrives.

- [ ] **Step 2: Write failing direct-selection and payload tests**

Build fixtures with directory -> page -> multiple actions and assert:

```text
initial menuIds select exact direct grants only
select page does not select its actions
select action does not add page to submitted IDs
directory interaction may select descendants for UX but directory ID is never submitted
page plus selected action normalizes to action only
page without selected action remains in payload
multiple actions remain separate
empty selection submits []
submitted menuIds are unique and ascending
```

Do not depend on Element Plus default parent/child check propagation. Configure the tree with `check-strictly` and implement explicit directory bulk behavior so page/action direct selection remains independent.

- [ ] **Step 3: Implement stable authorization state and tree helpers**

Use explicit state:

```ts
const permissionDialogVisible = ref(false)
const permissionLoading = ref(false)
const permissionSaving = ref(false)
const permissionError = ref('')
const permissionData = ref<RolePermissionsResponse | null>(null)
const selectedMenuIDs = ref<Set<number>>(new Set<number>())
```

Add typed local helpers for flattening, finding a page owner for an action, collecting directory descendants, and normalizing selected page/action IDs. These helpers stay in the view file unless test isolation makes a page-local component necessary. Do not promote them to a global tree composable.

- [ ] **Step 4: Implement the large authorization Dialog**

Use an `el-dialog` around 900px with responsive max width, stable viewport-constrained body height, fixed header/footer, and internal vertical scrolling. Tree nodes show translated title, code, type, and disabled status. Zero selection is a normal valid state, not an error.

Save calls:

```ts
await updateRolePermissions(roleID, { menuIds: normalizedMenuIDs })
```

Keep selection/dialog on failure. On success, reload the current role page to refresh `permissionCount`; close only after that reload succeeds. Do not call `/api/v1/access`, reset the access Store, hot-change routes, or reload the browser.

- [ ] **Step 5: Complete status/default/delete mutation tests**

Verify every successful non-create mutation preserves current page and filters. When deletion removes the final item on a non-first page, compute whether the current page exceeds `ceil((total - 1) / pageSize)`; decrement only then, then issue exactly one list reload. Cancellation causes no API call. API failure leaves current rows unchanged and shows the public error.

- [ ] **Step 6: Verify scrolling and manual-refresh behavior**

Add structural assertions/classes proving:

```text
role page does not set body/document scrolling
Main remains the page scroll owner
permission dialog body owns its internal overflow
dialog header/footer remain outside the scrolling tree body
no window.location.reload or access.load/reset call exists in the workflow
```

- [ ] **Step 7: Run full role frontend verification**

Run:

```powershell
pnpm vitest run src/views/system/roles/index.test.ts src/api/role.contract.test.ts src/api/role.test.ts src/access src/i18n/index.test.ts
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" src -g "*.ts" -g "*.vue"
git diff --check
```

Expected: all tests/build PASS, forbidden-type search returns no matches, and the authorization dialog preserves direct-grant semantics.

### Task 15: Prove RBAC End-To-End And Complete The Phase

**Files:**
- Modify only when a verification test exposes a spec violation in files already listed above
- Verify: `server/internal/module/role/*_test.go`
- Verify: `server/internal/module/access/*_test.go`
- Verify: `server/internal/module/auth/*_test.go`
- Verify: `server/internal/module/menu/*_test.go`
- Verify: `web/src/api/role*.test.ts`
- Verify: `web/src/views/system/roles/index.test.ts`
- Verify: `docs/superpowers/specs/2026-08-19-role-management-design.md`

- [x] **Step 1: Add the final PostgreSQL RBAC acceptance scenarios**

If not already proven by focused tests, add one integration test per flow:

```text
1. custom role granted role-list page -> access snapshot exposes role page, not actions
2. custom role granted role-authorize action -> snapshot exposes action + role page + system directory
3. same role granted page + action -> persisted direct set contains action only
4. user with two enabled roles -> permission union includes both role contributions
5. disable one role -> only the other role contributes on next request
6. disable user's last enabled role -> authentication rejects next request with 401
7. disable a granted menu -> stored grant remains but access excludes it
8. re-enable menu -> stored grant contributes again
9. change a role's grants -> permission middleware reads committed PostgreSQL state immediately
10. super_admin -> all enabled role-management permissions without direct grants
```

These tests use the real Repository/Service path and PostgreSQL. Do not replace PostgreSQL with an in-memory database or mocks for transaction assertions.

- [x] **Step 2: Run focused RBAC acceptance first**

Run from `D:\admin\server`:

```powershell
go test ./internal/module/role ./internal/module/access ./internal/module/auth ./internal/module/menu -run 'Test.*(Role|Permission|Access|Default|Disabled|Builtin)' -count=1
```

Expected: PASS. Fix only concrete failures within the approved spec; do not add Redis, audit logs, user management, sessions UI, or generic CRUD abstractions.

- [x] **Step 3: Run the complete backend quality gate**

Run:

```powershell
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

Expected: every command exits `0` with no failing packages.

- [x] **Step 4: Run the complete frontend quality gate**

Run from `D:\admin\web`:

```powershell
pnpm vitest run
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" src -g "*.ts" -g "*.vue"
```

Expected: Vitest/build exit `0`; the forbidden-TypeScript search returns no matches.

- [x] **Step 5: Inspect the final scope and hygiene**

Run from `D:\admin`:

```powershell
rg -n "BaseService|BaseRepository|Platform|Adapter|Manager|Factory|TODO|TBD|fallback|msg" server/internal/module/role web/src/api/role* web/src/views/system/roles
git diff --check
git status --short
git diff --stat
```

Expected: no forbidden abstraction, placeholder, compatibility field, or fallback implementation in the new role slice; `git diff --check` exits `0`. Review `git status` without reverting user changes and report every changed/untracked file. Do not commit or start services.

- [x] **Step 6: Manually reconcile every acceptance requirement**

Read `docs/superpowers/specs/2026-08-19-role-management-design.md` and record evidence for all 17 acceptance items. Specifically confirm system-role immutability, exactly one enabled default, zero-permission behavior, disabled-role/grant preservation, direct-grant minimality, manual-refresh UX, strict DTO/i18n, and recorded post-RBAC technical debt. Any unmet item keeps the phase incomplete.

## Spec Coverage Review

- Schema ownership, active name/code/default uniqueness, foreign keys, `TIMESTAMPTZ`, soft deletion, and Yes/No checks: Task 1.
- Stable role errors, protocol, and backend bilingual catalogs: Task 2.
- First-start creation, partial-data rejection, fixed role protection, and default invariant on restart: Task 3.
- Role-management built-in page/actions, core protection, and super-admin bypass: Task 4.
- Required pagination, literal keyword filtering, counts, and stable ordering: Task 5.
- Custom role creation, immutable code, name edits, and conflict mapping: Task 6.
- Role enable/disable behavior, relationship preservation, default switching, and future-user-only semantics: Task 7.
- Protected transactional soft deletion and rollback: Task 8.
- Full non-deleted permission tree, disabled menu visibility, direct page/action grants, normalization, corruption detection, and transactional save: Task 9.
- Eight strict HTTP APIs, middleware order, exact permissions, envelopes, request contexts, and app wiring: Task 10.
- Frontend title/view/icon registries and complete two-language UI protocol: Task 11.
- Runtime DTO validation from `unknown`, exact API payloads, and no forbidden TypeScript `any`: Task 12.
- Paged list, filters, permission-gated commands, protected states, profile dialog, and confirmations: Task 13.
- Authorization dialog, strict page/action independence, empty permissions, disabled nodes, mutation paging, internal dialog scrolling, and no access/browser refresh: Task 14.
- Multi-role union, runtime permission effect, disabled/re-enabled behavior, complete quality gates, technical-debt boundary, and all acceptance criteria: Task 15.

## Deliberate Non-Goals

This plan does not implement user management, user-role assignment UI, batch operations, role copying, Redis/process permission caches, audit logs, forced logout, online-session management, global `AppDialog`/`AppTable`, a paged-list composable, UI redesign, or generic CRUD abstractions. Those remain in the spec's recorded order: user management and full RBAC acceptance first, then technical-debt recovery, then UI refinement.
