# User Management And User Role Assignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Execute inline in the current `main` workspace; do not create a worktree, dispatch subagents, or create commits unless the repository owner explicitly changes those instructions.

**Goal:** Complete the user-management side of RBAC with strict paged user queries, username editing, status and soft-delete operations, multi-role assignment, session revocation, and the corresponding dynamic Vue page.

**Architecture:** Preserve the backend path `router -> authentication middleware -> permission middleware -> user Handler -> user Service -> user Repository -> model -> PostgreSQL`. PostgreSQL remains authoritative; the Service performs business validation and ordered mutations, the Repository owns GORM/SQL/locks, and Redis is touched only after a successful PostgreSQL disable/delete transaction to remove the existing current-session pointer. Frontend data remains `views/system/users/index.vue -> api/user.ts -> utils/request.ts -> Go API`, with every response parsed from `unknown` and page state kept local.

**Tech Stack:** Go 1.26, Gin, GORM, PostgreSQL, Redis, real PostgreSQL integration tests; Vue 3, strict TypeScript, Element Plus, Pinia, vue-i18n, Vitest, Vue Test Utils, pnpm.

---

## Existing Boundaries To Preserve

- `server/internal/module/user/model.go` already owns `sys_user`; `server/internal/module/role/model.go` owns `sys_role` and `sys_user_role`; `server/internal/module/auth/session_model.go` owns `sys_user_session`. This feature adds no table or column.
- `auth.Service`, bootstrap-admin, and authentication tests already consume `user.Repository.CreateWithRole`, `FindCredentialByUsername`, and `FindCurrent`. Keep these methods compatible while extending the same concrete Repository.
- `auth` already imports `user`. To avoid a Go import cycle, `user.Handler` must not import `auth`. `server/cmd/api/main.go` injects a concrete `func(*gin.Context) (int64, bool)` that delegates to `auth.IdentityFromContext`; the Handler passes only the actor user ID into the Service.
- All user mutations acquire locks in one order: fixed `super_admin` role row first, target user second, then user-role/session rows. No mutation may introduce a different order.
- `server/internal/module/access` remains the only runtime permission-union implementation. User-role saves change PostgreSQL immediately but do not call `/api/v1/access`, mutate the Access Store, reload the browser, or revoke sessions.
- `server/internal/shared/pagination`, `server/internal/shared/validate`, `server/internal/shared/response`, `web/src/types/pagination.ts`, and `web/src/utils/request.ts` remain the shared transport boundaries. Do not duplicate envelope, JSON, pagination, notification, or 401 logic.
- The only new test seam for Redis is the exact operation used here:

```go
type sessionPointerStore interface {
    Delete(context.Context, string) error
}
```

  Do not add a cache adapter, cache manager, interface package, or Redis fallback behavior.
- The repository owner controls Git history and running services. Each task ends with focused verification and `git diff --check`; this plan does not start or stop services and does not commit, push, pull, or fetch.

### Task 1: Unify Username Validation And The Current-Session Pointer Key

**Files:**
- Create: `server/internal/module/user/protocol.go`
- Create: `server/internal/module/user/protocol_test.go`
- Create: `server/internal/module/user/session_pointer.go`
- Create: `server/internal/module/user/session_pointer_test.go`
- Modify: `server/internal/module/auth/service.go`
- Modify: `server/internal/module/auth/service_test.go`

- [ ] **Step 1: Write failing user-protocol tests**

Define the exact public helper contract in `user/protocol_test.go`:

```go
func TestNormalizeUsername(t *testing.T) {
    for _, test := range []struct {
        input string
        want  string
        valid bool
    }{
        {input: "  张三_01  ", want: "张三_01", valid: true},
        {input: "alice-01", want: "alice-01", valid: true},
        {input: "ab", valid: false},
        {input: strings.Repeat("a", 65), valid: false},
        {input: "bad name", valid: false},
        {input: "bad@email", valid: false},
    } {
        got, err := NormalizeUsername(test.input)
        if test.valid && (err != nil || got != test.want) {
            t.Fatalf("NormalizeUsername(%q) = %q,%v", test.input, got, err)
        }
        if !test.valid && err == nil {
            t.Fatalf("NormalizeUsername(%q) accepted invalid input", test.input)
        }
    }
}

func TestCurrentSessionPointerKey(t *testing.T) {
    if got := CurrentSessionPointerKey(42); got != "auth:current-session:42" {
        t.Fatalf("CurrentSessionPointerKey(42) = %q", got)
    }
}
```

Add a source-level assertion in `auth/service_test.go` that `auth/service.go` no longer declares `currentSessionPointerKey` and keep all existing login/authenticate/logout key assertions.

- [ ] **Step 2: Run the focused tests and confirm RED**

Run from `D:\admin\server`:

```powershell
go test ./internal/module/user ./internal/module/auth -run 'TestNormalizeUsername|TestCurrentSessionPointerKey|TestRegister|TestLogin|TestAuthenticate|TestLogout' -count=1
```

Expected: FAIL because `user.NormalizeUsername` and `user.CurrentSessionPointerKey` do not exist.

- [ ] **Step 3: Implement the concrete user helpers**

Create only these public functions:

```go
func NormalizeUsername(value string) (string, error) {
    value = strings.TrimSpace(value)
    count := utf8.RuneCountInString(value)
    if count < 3 || count > 64 {
        return "", fmt.Errorf("username must contain 3 to 64 Unicode characters")
    }
    for _, character := range value {
        if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '_' && character != '-' {
            return "", fmt.Errorf("username contains an unsupported character")
        }
    }
    return value, nil
}

func CurrentSessionPointerKey(userID int64) string {
    return "auth:current-session:" + strconv.FormatInt(userID, 10)
}
```

Keep them in the two named user files; do not create a generic validation or key-building package.

- [ ] **Step 4: Make authentication consume the shared rules**

In `auth.validateAccountInput`, call `user.NormalizeUsername` and wrap failures with `apperror.InvalidRequest`. Replace all key construction in login, authenticate, refresh, and logout with:

```go
user.CurrentSessionPointerKey(userID)
```

Remove the private auth key function and its now-unused imports. Email/password validation stays in `auth`.

- [ ] **Step 5: Verify authentication compatibility**

Run:

```powershell
go test ./internal/module/user ./internal/module/auth -count=1
git diff --check
```

Expected: PASS; registration normalization and every existing Redis key remain byte-for-byte compatible.

### Task 2: Define User Types, Stable Errors, And Backend I18n

**Files:**
- Create: `server/internal/module/user/errors.go`
- Create: `server/internal/module/user/errors_test.go`
- Create: `server/internal/module/user/service.go`
- Modify: `server/internal/shared/i18n/catalog.go`
- Modify: `server/internal/shared/i18n/catalog_test.go`

- [ ] **Step 1: Write failing error-contract tests**

Assert this exact public contract in `user/errors_test.go`:

```text
16000 user.notFound                 HTTP 404
16001 user.usernameConflict         HTTP 409
16002 user.selfOperation            HTTP 409
16003 user.superAdminProtected      HTTP 403
16004 user.lastSuperAdmin           HTTP 409
16005 user.invalidRoles             HTTP 400
16006 user.roleNotFound             HTTP 404
16007 user.dataInvalid              HTTP 500
```

Each test must assert HTTP status, numeric code, message key, exact parameter set (empty for all eight), and internal-cause preservation through `errors.Is`.

- [ ] **Step 2: Add the stable service/domain types to the failing compile target**

Define these exact types in the initial `service.go`; methods arrive in later tasks:

```go
type ListQuery struct {
    Page      int
    PageSize  int
    Keyword   string
    IsEnabled *yesno.Value
    RoleID    *int64
}

type RoleSummary struct {
    ID        int64
    Code      string
    Name      string
    IsEnabled yesno.Value
}

type ListItem struct {
    ID        int64
    Username  string
    Email     string
    IsEnabled yesno.Value
    Roles     []RoleSummary
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Summary struct {
    ID int64; Username string; Email string; IsEnabled yesno.Value
}

type Roles struct {
    User Summary
    Roles []RoleSummary
    RoleIDs []int64
}

type UpdateInput struct { Username string }
type UpdatedUsername struct { ID int64; Username string; UpdatedAt time.Time }
```

Use normal multi-line fields in source; the compact form above only fixes names and types.

- [ ] **Step 3: Run protocol/i18n tests and confirm RED**

Run:

```powershell
go test ./internal/module/user ./internal/shared/i18n -run 'TestUser|TestCatalog' -count=1
```

Expected: FAIL because error constants, constructors, and catalog keys are missing.

- [ ] **Step 4: Implement error constructors and matching catalogs**

Follow `role/errors.go` and define:

```go
func userNotFound(cause error) *apperror.Error
func userUsernameConflict(cause error) *apperror.Error
func userSelfOperation(cause error) *apperror.Error
func userSuperAdminProtected(cause error) *apperror.Error
func userLastSuperAdmin(cause error) *apperror.Error
func userInvalidRoles(cause error) *apperror.Error
func userRoleNotFound(cause error) *apperror.Error
func userDataInvalid(cause error) *apperror.Error
```

Add all eight keys to both backend locales. Chinese and English catalogs must have identical key and interpolation sets; none of these messages expose a username, email, role ID, SQL constraint, or internal cause.

- [ ] **Step 5: Verify the stable error namespace**

Run:

```powershell
go test ./internal/module/user ./internal/shared/i18n -run 'TestUser|TestCatalog' -count=1
git diff --check
```

Expected: PASS with codes `16000` through `16007` and strict bilingual catalogs.

### Task 3: Implement User Listing, Role Options, And Corruption Detection

**Files:**
- Modify: `server/internal/module/user/repository.go`
- Modify: `server/internal/module/user/repository_test.go`
- Modify: `server/internal/module/user/service.go`
- Create: `server/internal/module/user/service_test.go`

- [ ] **Step 1: Add failing real-PostgreSQL list tests**

Extend the existing transaction fixture and cover:

```text
count and page are stable by created_at ASC, id ASC
keyword matches username or email case-insensitively
%, _, and backslash are treated as literal characters
isEnabled filters 0 and 1
roleId uses an effective sys_user_role EXISTS predicate
unknown positive roleId returns an empty page
multiple roles never duplicate a user or total count
roles sort by code ASC, id ASC
disabled roles remain in the response
no effective relation is ErrUserDataInvalid
relation to soft-deleted/missing role is ErrUserDataInvalid
empty page returns a non-nil empty slice
role options include every non-deleted role and sort by code ASC, id ASC
```

Introduce repository sentinels with concrete meanings:

```go
var ErrUserDataInvalid = errors.New("user or user-role data is invalid")
```

Keep existing `ErrUsernameConflict` and `ErrEmailConflict` behavior unchanged.

- [ ] **Step 2: Run list tests and confirm RED**

Run:

```powershell
go test ./internal/module/user -run 'Test(Count|List|FindRoleOptions|ListService)' -count=1
```

Expected: FAIL because the list/query methods do not exist. A PostgreSQL connection failure is an environment failure, not the expected red state.

- [ ] **Step 3: Add count, page, batched-role, and option queries**

Add these concrete Repository methods:

```go
func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error)
func (r *Repository) List(ctx context.Context, query ListQuery) ([]ListItem, error)
func (r *Repository) FindRoleOptions(ctx context.Context) ([]RoleSummary, error)
```

`Count` and the page base query share `applyUserListFilter`. Role filtering must use:

```sql
EXISTS (
  SELECT 1 FROM sys_user_role AS user_role
  WHERE user_role.user_id = app_user.id
    AND user_role.role_id = ?
    AND user_role.deleted_at IS NULL
)
```

Escape keyword search with the same explicit replacement order as role listing:

```go
strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
```

After loading one page of users, execute one batched relationship query for all page IDs. Use active `sys_user_role` rows and an unscoped/left role join so a missing or soft-deleted target is detected rather than hidden. Assemble `Roles` in memory, reject users with zero valid relationships, and never issue one query per user.

- [ ] **Step 4: Implement list and role-option Service methods**

Add:

```go
type sessionPointerStore interface {
    Delete(context.Context, string) error
}

type Service struct {
    repository *Repository
    pointers   sessionPointerStore
}

func NewService(repository *Repository, pointers sessionPointerStore) *Service
func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[ListItem], error)
func (s *Service) RoleOptions(ctx context.Context) ([]RoleSummary, error)
```

List validation must trim keyword, limit it to 254 Unicode characters, require page `>=1`, pageSize `1..100`, valid optional Yes/No, and positive optional role ID. Map `ErrUserDataInvalid` to code `16007`; map PostgreSQL errors to Dependency Unavailable. Preserve a non-nil empty list.

- [ ] **Step 5: Run list verification**

Run:

```powershell
go test ./internal/module/user -run 'Test(Count|List|FindRoleOptions|ListService|RoleOptions)' -count=1
git diff --check
```

Expected: PASS; list count and rows agree under all filters and no N+1 query is introduced.

### Task 4: Add Ordered Mutation Repository Primitives

**Files:**
- Modify: `server/internal/module/user/repository.go`
- Modify: `server/internal/module/user/repository_test.go`

- [ ] **Step 1: Write failing transaction-primitive tests**

Using real PostgreSQL transactions, test these exact behaviors:

```text
Transaction rolls back every prior write when the callback fails
LockSuperAdminRole locks the one active fixed role by code
LockUser excludes soft-deleted users
LockUserUnscoped distinguishes absent and soft-deleted rows
IsEffectiveSuperAdmin requires enabled user, active relation, active enabled role
HasActiveRole detects an active binding even when the target user is disabled
FindUser and FindUserRoles provide non-locking reads for the role-query endpoint
LockRoles returns every non-deleted role ordered by id
LockUserRoles returns every active relation ordered by role_id,id
CountEffectiveSuperAdmins follows the spec's full predicate
UpdateUsername maps ux_sys_user_username_active to ErrUsernameConflict
UpdateStatus and TouchUser maintain explicit updated_at
SoftDeleteUserRoleIDs timestamps only selected active rows
CreateUserRoles inserts new history rows rather than reviving deleted rows
RevokeActiveSessions sets revoked_at and updated_at to one supplied UTC time
SoftDeleteUser timestamps updated_at and deleted_at together
```

- [ ] **Step 2: Run the primitive tests and confirm RED**

Run:

```powershell
go test ./internal/module/user -run 'TestRepository(Transaction|Lock|Update|SoftDelete|CreateUserRoles|Revoke|CountEffective)' -count=1
```

Expected: FAIL for missing methods.

- [ ] **Step 3: Implement the fixed Repository surface**

Add this surface; Repository methods perform SQL/GORM only and contain no actor authorization branches:

```go
func (r *Repository) Transaction(context.Context, func(*Repository) error) error
func (r *Repository) LockSuperAdminRole(context.Context) (role.Role, error)
func (r *Repository) LockUser(context.Context, int64) (User, error)
func (r *Repository) LockUserUnscoped(context.Context, int64) (User, error)
func (r *Repository) IsEffectiveSuperAdmin(context.Context, int64, int64) (bool, error)
func (r *Repository) HasActiveRole(context.Context, int64, int64) (bool, error)
func (r *Repository) FindUser(context.Context, int64) (User, error)
func (r *Repository) FindUserRoles(context.Context, int64) ([]role.UserRole, error)
func (r *Repository) LockRoles(context.Context) ([]role.Role, error)
func (r *Repository) LockUserRoles(context.Context, int64) ([]role.UserRole, error)
func (r *Repository) CountEffectiveSuperAdmins(context.Context, int64) (int64, error)
func (r *Repository) UpdateUsername(context.Context, int64, string, time.Time) error
func (r *Repository) UpdateStatus(context.Context, int64, yesno.Value, time.Time) error
func (r *Repository) TouchUser(context.Context, int64, time.Time) error
func (r *Repository) SoftDeleteUserRoleIDs(context.Context, []int64, time.Time) error
func (r *Repository) CreateUserRoles(context.Context, []role.UserRole) error
func (r *Repository) RevokeActiveSessions(context.Context, int64, time.Time) error
func (r *Repository) SoftDeleteUser(context.Context, int64, time.Time) error
```

`LockSuperAdminRole` must use `FOR UPDATE` and validate that the fixed role is non-deleted, enabled, and uniquely present. `IsEffectiveSuperAdmin` is for actor authority and last-admin counting; `HasActiveRole` is for target protection and remains true for a disabled user that still owns `super_admin`. `RevokeActiveSessions` may update zero rows and must use raw `sys_user_session` SQL so `user` does not import `auth` and create an import cycle.

- [ ] **Step 4: Prove rollback and lock order with focused integration tests**

Add a forced trigger failure after a user status/relationship update and assert the user, relations, and sessions all retain their previous values. Add a two-transaction lock test showing a second `LockSuperAdminRole` waits/cancels before it can lock a target user; this documents the common first lock used by every mutation.

- [ ] **Step 5: Run Repository verification**

Run:

```powershell
go test ./internal/module/user -run 'TestRepository(Transaction|Lock|Update|SoftDelete|CreateUserRoles|Revoke|CountEffective)' -count=1
git diff --check
```

Expected: PASS; every I/O method honors `context.Context`, timestamps are explicit, and failed callbacks leave no partial write.

### Task 5: Implement Username Editing And Super-Administrator Target Protection

**Files:**
- Modify: `server/internal/module/user/service.go`
- Modify: `server/internal/module/user/service_test.go`
- Modify: `server/internal/module/user/repository_test.go`

- [ ] **Step 1: Write failing username-mutation tests**

Cover:

```text
actor may update their own username
same normalized username is idempotent and preserves updated_at
the shared Unicode/length/character rules are applied
unknown or deleted target is 16000
ordinary actor cannot update a super_admin target (16003)
effective super_admin may update another super_admin
case-insensitive conflict is 16001
soft-deleted username can be reused
repository failure rolls back and maps to dependency unavailable
super_admin role is locked before target user
```

- [ ] **Step 2: Run the service test and confirm RED**

Run:

```powershell
go test ./internal/module/user -run 'TestServiceUpdateUsername' -count=1
```

Expected: FAIL because `Service.Update` is missing.

- [ ] **Step 3: Implement the ordered username transaction**

Add:

```go
func (s *Service) Update(
    ctx context.Context,
    actorUserID int64,
    targetUserID int64,
    input UpdateInput,
) (UpdatedUsername, error)
```

The method validates positive IDs and calls `NormalizeUsername`, then executes exactly:

```text
repository.Transaction
-> LockSuperAdminRole
-> LockUser(target)
-> IsEffectiveSuperAdmin(actor)
-> HasActiveRole(target, superAdminRoleID)
-> reject ordinary actor against super target
-> return current username/time when normalized value is unchanged
-> UpdateUsername with time.Now().UTC()
```

Map `ErrUsernameConflict` to `16001`, `gorm.ErrRecordNotFound` for the target to `16000`, explicit data corruption to `16007`, and all other repository failures to Dependency Unavailable.

- [ ] **Step 4: Verify username semantics and registration reuse**

Run:

```powershell
go test ./internal/module/user ./internal/module/auth -run 'Test(ServiceUpdateUsername|NormalizeUsername|Register)' -count=1
git diff --check
```

Expected: PASS; registration and management cannot diverge on username validity.

### Task 6: Implement User-Role Query And Minimal-Difference Assignment

**Files:**
- Modify: `server/internal/module/user/service.go`
- Modify: `server/internal/module/user/service_test.go`
- Modify: `server/internal/module/user/repository_test.go`

- [ ] **Step 1: Write failing user-role query tests**

Test `Service.Roles(ctx, targetUserID)` returns the user summary, all non-deleted role options sorted by `code,id`, and current role IDs sorted numerically and deduplicated. It must return `16000` for an unknown/deleted user and `16007` for no relation, duplicate active relation, or a relation to a missing/soft-deleted role. A disabled role is valid and remains visible/selected.

- [ ] **Step 2: Write failing role-save and protection tests**

Cover all of these cases:

```text
incoming role IDs are sorted and deduplicated
empty set -> 16005
only disabled roles -> 16005
missing or soft-deleted requested role -> 16006
disabled role plus one enabled role is valid
same normalized set is idempotent and preserves updated_at/history
removed relation is soft-deleted with the operation time
new relation creates a fresh row; old history is not revived
write failure rolls back removals and additions
actor cannot change their own roles -> 16002
ordinary actor cannot change an existing super_admin target -> 16003
ordinary actor cannot grant or remove super_admin -> 16003
effective super_admin can grant/remove super_admin on another user
removing the last effective super_admin -> 16004
two concurrent removals from two different super admins cannot both commit
role changes do not revoke sessions and do not touch Redis
```

- [ ] **Step 3: Run role tests and confirm RED**

Run:

```powershell
go test ./internal/module/user -run 'TestService(Roles|UpdateRoles|ConcurrentSuperAdmin)' -count=1
```

Expected: FAIL for missing service methods.

- [ ] **Step 4: Implement strict role query**

Add:

```go
func (s *Service) Roles(ctx context.Context, targetUserID int64) (Roles, error)
```

Use `FindUser`, `FindRoleOptions`, and `FindUserRoles` without mutation, validate the complete relationship set against all non-deleted roles, and build `roleIds` from validated active relationships. Never silently drop an invalid relationship and never invent the default role.

- [ ] **Step 5: Implement the fixed role-assignment transaction**

Add:

```go
func (s *Service) UpdateRoles(
    ctx context.Context,
    actorUserID int64,
    targetUserID int64,
    requestedRoleIDs []int64,
) (int64, error)
```

Reject a nil request at the request layer; the Service sorts/deduplicates a non-nil slice and validates positive IDs. Execute:

```text
LockSuperAdminRole
-> LockUser(target)
-> reject actor == target
-> determine actor effective-super status and target's active super_admin binding
-> LockRoles
-> validate every requested ID exists and at least one is enabled
-> LockUserRoles and validate stored relationships
-> determine additions/removals and whether super_admin is touched
-> apply actor/target protection
-> when removing super_admin, CountEffectiveSuperAdmins must remain >= 1
-> soft-delete removed relationship IDs
-> create additions
-> TouchUser only when the normalized set changed
```

Use one UTC timestamp for every changed row. The return value is the normalized role count. Do not call the pointer store from this method.

- [ ] **Step 6: Run role-assignment verification**

Run:

```powershell
go test ./internal/module/user -run 'TestService(Roles|UpdateRoles|ConcurrentSuperAdmin)' -count=1
go test ./internal/module/access -count=1
git diff --check
```

Expected: PASS; concurrent mutation preserves at least one effective super admin and existing access-union tests still pass.

### Task 7: Implement Status Changes, Soft Delete, Session Revocation, And Redis Cleanup

**Files:**
- Modify: `server/internal/module/user/service.go`
- Modify: `server/internal/module/user/service_test.go`
- Modify: `server/internal/module/user/repository_test.go`

- [ ] **Step 1: Write failing status tests**

Cover:

```text
enable updates only is_enabled/updated_at and never writes Redis
repeat enable is idempotent and preserves updated_at
enable requires at least one active relationship to a non-deleted role, but that role may be disabled
disable rejects actor == target with 16002
ordinary actor cannot disable super target with 16003
disable of last effective super admin returns 16004 and rolls back
disable revokes every unrevoked session in the same transaction
repeat disable re-runs session revocation and pointer deletion
PostgreSQL failure never deletes the pointer
Redis delete occurs only after commit
Redis delete failure returns Dependency Unavailable while PostgreSQL remains disabled/revoked
```

- [ ] **Step 2: Write failing deletion tests**

Cover:

```text
unknown ID returns 16000
actor cannot delete self with 16002
ordinary actor cannot delete super target with 16003
deleting last effective super admin returns 16004
user, active user-role rows, and all unrevoked sessions change in one transaction
one UTC operation time is used across changed rows
forced relationship/session/user failure rolls back every preceding write
first successful delete removes the Redis pointer after commit
already-soft-deleted ID skips database writes but retries pointer delete
Redis failure after delete is retryable by DELETE on the same ID
```

- [ ] **Step 3: Run safety tests and confirm RED**

Run:

```powershell
go test ./internal/module/user -run 'TestService(UpdateStatus|Delete)' -count=1
```

Expected: FAIL because the methods are missing.

- [ ] **Step 4: Implement status transitions**

Add:

```go
func (s *Service) UpdateStatus(
    ctx context.Context,
    actorUserID int64,
    targetUserID int64,
    value yesno.Value,
) error
```

Every status transaction starts `LockSuperAdminRole -> LockUser`. Validate actor authority with `IsEffectiveSuperAdmin` and target protection with `HasActiveRole`. Before enabling, lock/validate the target's active relationships against non-deleted roles; at least one relationship is required, but all related roles may currently be disabled. Enable then performs only an idempotent status update. Disable checks self/super/last-super rules; when already disabled it skips the status timestamp write but still revokes any unrevoked sessions. It commits that transaction and then calls:

```go
s.pointers.Delete(ctx, CurrentSessionPointerKey(targetUserID))
```

after commit. A nil pointer dependency or Delete error is Dependency Unavailable; never report success after failed Redis cleanup.

- [ ] **Step 5: Implement idempotent soft deletion**

Add:

```go
func (s *Service) Delete(ctx context.Context, actorUserID, targetUserID int64) error
```

The transaction order is `LockSuperAdminRole -> LockUserUnscoped(target)`. If no physical row exists, return `16000`; if `DeletedAt.Valid`, commit no database writes and proceed to pointer deletion. Otherwise enforce self/super/last-super rules, soft-delete active user-role rows, revoke active sessions, and soft-delete the user with one UTC timestamp. Delete the Redis pointer after transaction success in both newly-deleted and already-deleted cases.

- [ ] **Step 6: Verify PostgreSQL truth and retry behavior**

Run:

```powershell
go test ./internal/module/user -run 'TestService(UpdateStatus|Delete)' -count=1
go test ./internal/module/auth -run 'Test(Authenticate|Refresh)' -count=1
git diff --check
```

Expected: PASS; PostgreSQL always reaches the safe state before Redis cleanup, and old access/refresh sessions cannot become valid again after enable.

### Task 8: Expose Strict User HTTP Contracts And Wire The API

**Files:**
- Create: `server/internal/module/user/request.go`
- Create: `server/internal/module/user/response.go`
- Create: `server/internal/module/user/handler.go`
- Create: `server/internal/module/user/handler_test.go`
- Create: `server/internal/module/user/route.go`
- Create: `server/internal/module/user/route_test.go`
- Modify: `server/internal/module/menu/protocol.go`
- Modify: `server/internal/middleware/access_log.go`
- Modify: `server/internal/middleware/access_log_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`

- [ ] **Step 1: Write failing Handler success-contract tests**

Use a typed stub service and injected actor reader returning user ID `41`. Assert exact status/envelope/data for all seven routes:

```text
GET    /users?page=1&pageSize=20
GET    /users/role-options
PUT    /users/7                 {"username":"alice_new"}
PATCH  /users/7/status          {"isEnabled":0}
DELETE /users/7
GET    /users/7/roles
PUT    /users/7/roles           {"roleIds":[5,2,5]}
```

Assert mutation stub calls receive actor `41`, target `7`, and only the declared input fields. Timestamps must serialize with `UTC().Format(time.RFC3339Nano)`; empty lists must be `[]`, and delete data must be `{}`.

- [ ] **Step 2: Add strict failure matrices**

Test:

```text
list requires page/pageSize exactly once
unknown/repeated query parameters are 400
keyword over 254 Unicode characters is 400
isEnabled outside 0/1 and roleId <= 0 are 400
URL IDs reject zero, negative, text, overflow, and signs
JSON rejects missing fields, unknown fields, wrong types, trailing JSON, duplicate top-level JSON values, and empty body
roleIds accepts duplicates for Service normalization but rejects non-positive values
DELETE rejects any body including {}
missing actor identity fails closed and never invokes Service
service errors preserve their status/code/message envelope
```

- [ ] **Step 3: Run Handler tests and confirm RED**

Run:

```powershell
go test ./internal/module/user -run 'TestHandler|TestRegisterRoutes' -count=1
```

Expected: FAIL because request, response, Handler, and route files are absent.

- [ ] **Step 4: Implement explicit request and response types**

The service boundary in `handler.go` is:

```go
type userService interface {
    List(context.Context, ListQuery) (pagination.Result[ListItem], error)
    RoleOptions(context.Context) ([]RoleSummary, error)
    Update(context.Context, int64, int64, UpdateInput) (UpdatedUsername, error)
    UpdateStatus(context.Context, int64, int64, yesno.Value) error
    Delete(context.Context, int64, int64) error
    Roles(context.Context, int64) (Roles, error)
    UpdateRoles(context.Context, int64, int64, []int64) (int64, error)
}
```

Constructor:

```go
func NewHandler(
    service userService,
    actorUserID func(*gin.Context) (int64, bool),
) *Handler
```

Use pointer request fields to distinguish missing from zero/empty, `validate.BindJSON` for JSON, and `validate.RequireEmptyBody` for DELETE. `parseListQuery` must use an explicit allowlist and `parseUserID` must accept only positive base-10 int64 values.

Keep response DTOs private and map Service values exactly:

```text
listResponse             list,total,page,pageSize
listItemResponse         id,username,email,isEnabled,roles,createdAt,updatedAt
roleSummaryResponse      id,code,name,isEnabled
roleOptionsResponse      roles
updatedUsernameResponse  id,username,updatedAt
statusResponse           id,isEnabled
rolesResponse            user,roles,roleIds
roleResultResponse       id,roleCount
emptyResponse            no fields
```

Convert Yes/No values to JSON numbers and all timestamps with `UTC().Format(time.RFC3339Nano)`. If the actor callback is absent or returns an invalid ID, fail with the existing Unauthorized error before invoking the Service.

- [ ] **Step 5: Define the finite permission constants and register exact routes**

Add these constants to `menu/protocol.go`:

```go
const (
    PermissionUserList   = "system:user:list"
    PermissionUserUpdate = "system:user:update"
    PermissionUserStatus = "system:user:status"
    PermissionUserDelete = "system:user:delete"
    PermissionUserRoles  = "system:user:roles"
)
```

The route test must capture the registered permission strings and thereby prove these constant values before binding them exactly:

```go
routes.GET("/users", authenticate, requirePermission(menu.PermissionUserList), handler.List)
routes.GET("/users/role-options", authenticate, requirePermission(menu.PermissionUserList), handler.RoleOptions)
routes.PUT("/users/:id", authenticate, requirePermission(menu.PermissionUserUpdate), handler.Update)
routes.PATCH("/users/:id/status", authenticate, requirePermission(menu.PermissionUserStatus), handler.UpdateStatus)
routes.DELETE("/users/:id", authenticate, requirePermission(menu.PermissionUserDelete), handler.Delete)
routes.GET("/users/:id/roles", authenticate, requirePermission(menu.PermissionUserRoles), handler.Roles)
routes.PUT("/users/:id/roles", authenticate, requirePermission(menu.PermissionUserRoles), handler.UpdateRoles)
```

Keep `/users/role-options` before parameter routes.

- [ ] **Step 6: Attach concrete mutation fields to the existing access log**

Add one directly used middleware helper:

```go
func SetAccessLogOperation(
    context *gin.Context,
    operation string,
    actorUserID int64,
    targetUserID int64,
)
```

`AccessLog` appends `operation`, `actorUserId`, and `targetUserId` when set. Preserve its existing rule for unrelated endpoints; additionally retain `appErr.Cause` for a request carrying this explicit user-operation context, including expected 4xx business rejections. Do not broaden cause logging for authentication requests that can contain credentials. The four mutation Handlers call the helper after strict ID/identity parsing and before Service invocation with:

```text
user.username.update
user.status.update
user.roles.update
user.delete
```

Extend `access_log_test.go` to assert request ID, operation, actor, target, status/error code, and internal cause on a failing mutation. Assert the response still does not expose the cause and logs contain no password, Token, Cookie, Authorization, password hash, or request body. This enriches the existing HTTP access log only; it is not the deferred persistent operation-log feature.

- [ ] **Step 7: Compose the concrete Service and actor reader in main**

Add `User *user.Handler` to `routerDependencies`. Reuse the one existing `userRepository` and Redis client:

```go
userService := user.NewService(userRepository, redisClient)
userHandler := user.NewHandler(userService, func(context *gin.Context) (int64, bool) {
    identity, ok := auth.IdentityFromContext(context)
    return identity.UserID, ok
})
```

Register user routes after access/menu/role routing. This callback is only an import-cycle boundary; it contains no business rule and is not named Adapter.

- [ ] **Step 8: Verify the complete backend HTTP slice**

Run:

```powershell
go test ./internal/module/user ./internal/middleware ./cmd/api -count=1
go test ./internal/module/auth ./internal/module/access ./internal/module/menu ./internal/module/role -count=1
git diff --check
```

Expected: PASS; `main_test.go` sees each route exactly once and route tests prove the exact permission code and middleware order.

### Task 9: Extend The Built-In Menu Tree With User Management

**Files:**
- Modify: `server/internal/module/menu/protocol.go`
- Modify: `server/internal/module/menu/builtin.go`
- Modify: `server/internal/module/menu/builtin_test.go`
- Modify: `server/internal/module/access/repository_test.go`

- [ ] **Step 1: Write failing exact-tree tests**

Update the built-in menu count and assert these five exact records under the existing `system` directory:

```text
system:user:list    page   navigation.systemUsers  /system/users  system-users  User  sort 30
system:user:update  action permission.userUpdate   parent=list                sort 10
system:user:status  action permission.userStatus   parent=list                sort 20
system:user:delete  action permission.userDelete   parent=list                sort 30
system:user:roles   action permission.userRoles    parent=list                sort 40
```

Extend corruption tests so wrong type, parent, i18n key, path, view key, or enabled state is rejected. Extend the operator-field test to prove icon and sort remain operator-controlled after creation, matching current `EnsureBuiltin` semantics.

- [ ] **Step 2: Add failing RBAC visibility assertions**

In `access/repository_test.go`, assert:

```text
super_admin receives the user page and all four actions implicitly
an ordinary role granted only system:user:list receives the page, not actions
an ordinary role granted system:user:roles receives its page ancestor plus that action
disabled roles do not contribute user permissions
```

Use the existing access snapshot/ancestor tests; do not create a second permission algorithm.

- [ ] **Step 3: Run menu/access tests and confirm RED**

Run:

```powershell
go test ./internal/module/menu ./internal/module/access -run 'TestEnsureBuiltin|Test.*UserPermission' -count=1
```

Expected: FAIL because the built-in definitions and finite title/view registries do not yet contain user management.

- [ ] **Step 4: Add finite registries and built-in definitions**

Add these backend protocol values:

```text
menuTitleKeys: navigation.systemUsers, permission.userUpdate,
               permission.userStatus, permission.userDelete, permission.userRoles
menuViewKeys:  system-users
builtinCodes:  all five PermissionUser* constants
```

Append the page after roles (`sortOrder: 30`) and its actions in the order above. `EnsureBuiltin` must create missing rows and strictly validate immutable fields without rewriting existing records. Do not add a create-user action.

- [ ] **Step 5: Verify the core RBAC tree**

Run:

```powershell
go test ./internal/module/menu ./internal/module/access -count=1
git diff --check
```

Expected: PASS; user management appears in the same system tree and the existing leaf/ancestor access behavior remains unchanged.

### Task 10: Register The User Frontend Protocol And Bilingual Copy

**Files:**
- Modify: `web/src/access/menu-title-keys.ts`
- Modify: `web/src/access/menu-title-keys.test.ts`
- Modify: `web/src/access/route-views.ts`
- Modify: `web/src/access/protocol.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Create: `web/src/views/system/users/index.vue`

- [ ] **Step 1: Write failing finite-registry tests**

Assert that `isMenuTitleKey` accepts exactly these additions:

```text
navigation.systemUsers
permission.userUpdate
permission.userStatus
permission.userDelete
permission.userRoles
```

Assert `routeViews['system-users']` exists and resolves a Vue component. Add a minimal placeholder `index.vue` only after the failing test so the dynamic import can resolve:

```vue
<template><main class="user-management" /></template>
```

- [ ] **Step 2: Write failing locale-shape and copy assertions**

Add tests using the existing locale/key validation that both locales contain the same user-management keys. The page will use this fixed namespace:

```text
user.title, user.keyword, user.status, user.role, user.search, user.reset, user.refresh
user.id, user.username, user.email, user.roles, user.createdAt, user.updatedAt, user.actions
user.enabled, user.disabled, user.roleDisabled, user.noRoles
user.edit, user.editTitle, user.assignRoles, user.assignRolesTitle
user.selectAll, user.clear, user.save, user.cancel
user.loadFailed, user.roleLoadFailed, user.saveFailed
user.usernameRequired, user.invalidUsername, user.enabledRoleRequired
user.selfStatusBlocked, user.selfRolesBlocked, user.selfDeleteBlocked
user.superAdminBlocked, user.superAdminRoleBlocked
user.enableConfirm, user.disableConfirm, user.deleteConfirm, user.superAdminImpact
user.updateSuccess, user.statusSuccess, user.rolesSuccess, user.deleteSuccess
```

Chinese copy must explicitly say disabling logs the user out and re-enabling requires login; deletion must say the account/relations/sessions are not restored and the email may register a new account. English must convey the same facts. Do not put implementation explanations or keyboard instructions in the UI.

- [ ] **Step 3: Run protocol tests and confirm RED**

Run from `D:\admin\web`:

```powershell
pnpm vitest run src/access/menu-title-keys.test.ts src/access/protocol.test.ts
```

Expected: FAIL for missing title/view/locale keys.

- [ ] **Step 4: Extend registries, dynamic view mapping, and locale objects**

Add:

```ts
'system-users': () => import('../views/system/users/index.vue').then((module) => module.default)
```

Keep `menuTitleKeys` an `as const` finite list and preserve `AppMessageKey` checking. Use the existing `User` icon already present in `menu-icons.ts`; do not create or import another icon implementation.

- [ ] **Step 5: Verify frontend protocol registration**

Run:

```powershell
pnpm vitest run src/access/menu-title-keys.test.ts src/access/protocol.test.ts
pnpm build
```

Expected: PASS; the dynamic route resolves and both locale objects satisfy the same strict type.

### Task 11: Implement Strict User Response Parsing

**Files:**
- Create: `web/src/api/user.contract.ts`
- Create: `web/src/api/user.contract.test.ts`

- [ ] **Step 1: Define explicit DTO/input types and failing happy-path tests**

Define:

```ts
export interface UserListQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
  roleId?: number
}

export interface UserRoleSummary {
  id: number
  code: string
  name: string
  isEnabled: YesNo
}

export interface UserListItem {
  id: number
  username: string
  email: string
  isEnabled: YesNo
  roles: UserRoleSummary[]
  createdAt: string
  updatedAt: string
}

export interface UserRolesResponse {
  user: { id: number; username: string; email: string; isEnabled: YesNo }
  roles: UserRoleSummary[]
  roleIds: number[]
}

export interface UpdateUserInput { username: string }
export interface UpdateUserRolesInput { roleIds: number[] }
```

Also define typed results for username (`id`, `username`, `updatedAt`), status (`id`, `isEnabled`), role options (`roles`), role save (`id`, `roleCount`), and empty deletion.

- [ ] **Step 2: Write the complete malformed-protocol matrix**

Every exported parser takes `unknown`. Test rejection of:

```text
missing or extra object fields at every nesting level
non-object roots and non-array list/roles/roleIds
non-positive/non-safe-integer IDs and counts
Yes/No values outside 0/1
blank or untrimmed username/email/code/name
invalid RFC3339 timestamps
duplicate user IDs in a page
duplicate role IDs/codes in role option arrays
user list roles not sorted by code,id
role options not sorted by code,id
roleIds not strictly increasing or absent from the role options
list page/total/pageSize invalid
delete response containing any field
```

Keep disabled roles valid. Require a non-empty `roles` array for each list user and current-role response because the backend treats missing relationships as corruption.

- [ ] **Step 3: Run contract tests and confirm RED**

Run:

```powershell
pnpm vitest run src/api/user.contract.test.ts
```

Expected: FAIL because `user.contract.ts` does not exist.

- [ ] **Step 4: Implement closed-record parsers from unknown**

Export these parsers:

```ts
parseUserPage
parseUserRoleOptions
parseUpdatedUsername
parseUserStatusResult
parseEmptyUserResult
parseUserRoles
parseUserRoleResult
```

Follow `role.contract.ts`: use `Record<string, unknown>` only after an object check, compare exact sorted keys, validate every scalar, and construct fresh typed objects. Do not use explicit `any`, optional chaining to hide missing fields, compatibility aliases, or default values for malformed required data.

- [ ] **Step 5: Run parser and TypeScript checks**

Run:

```powershell
pnpm vitest run src/api/user.contract.test.ts
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src/api/user.contract.ts src/api/user.contract.test.ts
```

Expected: tests/build PASS and `rg` returns no matches (exit code `1` is the expected no-match result).

### Task 12: Implement Thin User API Functions

**Files:**
- Create: `web/src/api/user.ts`
- Create: `web/src/api/user.test.ts`

- [ ] **Step 1: Write failing request-shape tests**

Mock `request` and assert exact calls:

```ts
getUsers({ page: 2, pageSize: 50, keyword: 'alice', isEnabled: YesNo.No, roleId: 7 })
// GET /api/v1/users with exactly those params

getUserRoleOptions()
// GET /api/v1/users/role-options, no params/data

updateUser(7, { username: 'alice_new' })
// PUT /api/v1/users/7 with { username }

updateUserStatus(7, YesNo.No)
// PATCH /api/v1/users/7/status with { isEnabled }

deleteUser(7)
// DELETE /api/v1/users/7, no body

getUserRoles(7)
// GET /api/v1/users/7/roles, no body

updateUserRoles(7, { roleIds: [2, 5] })
// PUT /api/v1/users/7/roles with { roleIds }
```

For every function, return malformed mock data once and assert a `ProtocolError` rather than compatibility data.

- [ ] **Step 2: Run API tests and confirm RED**

Run:

```powershell
pnpm vitest run src/api/user.test.ts
```

Expected: FAIL because `user.ts` is absent.

- [ ] **Step 3: Implement request-then-parse functions**

Each function must call:

```ts
const data = await request<unknown>({ /* exact method/url/params/data */ })
return parseSpecificUserResult(data)
```

Do not catch errors, notify, refresh auth, normalize envelope fields, or add a user API class. `request.ts` already owns those concerns.

- [ ] **Step 4: Verify the thin API boundary**

Run:

```powershell
pnpm vitest run src/api/user.test.ts src/api/user.contract.test.ts
pnpm build
git diff --check
```

Expected: PASS with all payloads lower camel case and no request body on GET/DELETE.

### Task 13: Add The Auth Store Username Synchronization Action

**Files:**
- Modify: `web/src/store/auth.ts`
- Modify: `web/src/store/auth.test.ts`

- [ ] **Step 1: Write failing store tests**

Add:

```ts
it('updates only the authenticated current username', () => {
  const store = useAuthStore()
  store.setAuthenticated({ userId: 7, username: 'old', email: 'user@example.com' })
  expect(store.updateUsername(7, 'new')).toBe(true)
  expect(store.user).toEqual({ userId: 7, username: 'new', email: 'user@example.com' })
  expect(store.updateUsername(8, 'ignored')).toBe(false)
  expect(store.user?.username).toBe('new')
})
```

Also assert it returns false without an authenticated user and preserves token, expiry, email, and status.

- [ ] **Step 2: Run the store test and confirm RED**

Run:

```powershell
pnpm vitest run src/store/auth.test.ts
```

Expected: FAIL because `updateUsername` is missing.

- [ ] **Step 3: Implement the narrow action**

Add an action with this signature:

```ts
updateUsername(userId: number, username: string): boolean
```

It updates `this.user` only when it exists and its `userId` matches. Construct a fresh `CurrentUser` object and return whether an update occurred. It must not call an API or touch Access Store state.

- [ ] **Step 4: Verify store behavior**

Run:

```powershell
pnpm vitest run src/store/auth.test.ts
pnpm build
```

Expected: PASS; Header consumers of `auth.user.username` update reactively without reloading access.

### Task 14: Build The Paged User List And Filter Surface

**Files:**
- Modify: `web/src/views/system/users/index.vue`
- Create: `web/src/views/system/users/index.test.ts`

- [ ] **Step 1: Write failing initial-load and rendering tests**

Mock all functions from `api/user.ts`, mount with Pinia/i18n/Element Plus, and assert initial load sends:

```ts
getUserRoleOptions()
getUsers({ page: 1, pageSize: 20 })
```

Render one multi-role user and assert columns for ID, username, email, role tags, Yes/No status, created time, updated time, and commands. Enabled and disabled roles both render; the disabled role has explicit disabled text/style and exposes its code in adjacent text or Tooltip. Empty successful data renders an Element Plus empty state, while a rejected request renders `user.loadFailed` and does not replace previously loaded rows with a fake empty list.

- [ ] **Step 2: Write failing filter, paging, and refresh tests**

Drive the real controls and assert:

```text
search trims keyword and sends keyword/isEnabled/roleId with page reset to 1
reset clears all three filters and requests page 1
current-page change preserves filters
page-size change preserves filters and resets page 1
refresh preserves the applied query and current page
role options include disabled roles and show their state
role-option load failure is an explicit error, not an empty select
loading flags prevent duplicate commands without shifting layout
```

- [ ] **Step 3: Write failing permission and protection tests**

Apply Access Store snapshots one permission at a time and prove:

```text
system:user:list renders the page/filter/refresh surface
system:user:update alone controls edit command rendering
system:user:status alone controls status command rendering
system:user:delete alone controls delete command rendering
system:user:roles alone controls role command rendering
```

With an authenticated current user, assert status/delete/roles are disabled for their own row, while edit remains available. For an ordinary actor (`roleCodes` excludes `super_admin`), all mutation commands against a row containing role code `super_admin` are disabled with an explicit reason. A super-admin actor may operate a different super-admin row. These are interaction guards only; tests must not imply backend trust.

- [ ] **Step 4: Run the page tests and confirm RED**

Run:

```powershell
pnpm vitest run src/views/system/users/index.test.ts
```

Expected: FAIL because the placeholder page has no list behavior.

- [ ] **Step 5: Implement page-local state and explicit loading**

Use typed refs matching the contract:

```ts
const rows = ref<UserListItem[]>([])
const roleOptions = ref<UserRoleSummary[]>([])
const total = ref(0)
const query = ref<UserListQuery>({ page: 1, pageSize: 20 })
const keyword = ref('')
const statusFilter = ref<'' | YesNo>('')
const roleFilter = ref<'' | number>('')
const loading = ref(false)
const roleOptionsLoading = ref(false)
const loadError = ref('')
const roleOptionsError = ref('')
```

Create concrete `loadUsers` and `loadRoleOptions` functions. Catch `unknown`, map `ApiError`/`ProtocolError` to localized text as existing role page does, retain prior valid data on refresh failure, and never fabricate response fields. Keep all state in this view.

- [ ] **Step 6: Implement the Element Plus page structure**

Build one unframed page surface:

```text
compact search row -> table -> pagination
```

Use `el-input`, `el-select`, icon/text command buttons where appropriate, `el-table`, tags, tooltips, `el-alert`, and `el-pagination`. Do not nest cards, add marketing copy, or introduce a second page scroll owner. Give table/action columns stable widths and allow compact controls to wrap without overflow.

- [ ] **Step 7: Enforce the page scroll boundary**

Use styles consistent with the role/menu pages:

```css
.user-management {
  min-height: 0;
  display: flex;
  flex-direction: column;
}
```

The page must not set `overflow-y: auto` on `body`, layout wrappers, or another full-height container. Main remains the only page scroll owner. Add a DOM/style assertion matching the existing role-page scroll test.

- [ ] **Step 8: Verify list behavior**

Run:

```powershell
pnpm vitest run src/views/system/users/index.test.ts
pnpm build
git diff --check
```

Expected: PASS; filtering, paging, error retention, exact permissions, and body-scroll constraints are covered.

### Task 15: Add Username And Role Assignment Dialogs

**Files:**
- Modify: `web/src/views/system/users/index.vue`
- Modify: `web/src/views/system/users/index.test.ts`

- [ ] **Step 1: Write failing username Dialog tests**

Assert:

```text
edit opens a teleported el-dialog for the selected user
username is editable and email is read-only
frontend validation uses trimmed 3..64 Unicode letters/digits/_/-
submit sends only { username }
save failure retains input, target, error, and open Dialog
success closes Dialog and refreshes the current applied query
editing current user calls auth.updateUsername with the server-returned normalized username
editing another user does not mutate Auth Store
no username mutation calls access.load or /api/v1/access
```

- [ ] **Step 2: Write failing role Dialog loading and selection tests**

Assert the role command calls `getUserRoles(target.id)` and renders a flat checkbox list with name, code, and enabled state. Cover:

```text
all non-deleted roles appear, including disabled roles
current roleIds initialize selection exactly
select all selects enabled and disabled roles
clear produces an allowed edit state but disables save with enabled-role explanation
disabled roles remain selectable and removable
load failure shows explicit error/retry and does not fabricate empty roles
self row cannot open the Dialog
ordinary actor sees super_admin but cannot toggle it
ordinary actor's existing super_admin selection is preserved by all/clear operations
super-admin actor can toggle super_admin on another user
```

- [ ] **Step 3: Write failing role save tests**

Assert `updateUserRoles(targetID, { roleIds })` receives unique numerically sorted IDs. A selection with at least one enabled role may include any number of disabled roles. Saving an unchanged set may still call the idempotent API; the backend owns final normalization. On failure, keep the Dialog, selections, and error. On success, close and refresh the current user page. Assert no save calls `access.load`, mutates dynamic routes, or invokes `window.location.reload`.

- [ ] **Step 4: Run Dialog tests and confirm RED**

Run:

```powershell
pnpm vitest run src/views/system/users/index.test.ts -t 'username|role dialog|role selection|role save'
```

Expected: FAIL because the Dialog workflows do not exist.

- [ ] **Step 5: Implement the small edit Dialog**

Keep concrete state in the page:

```ts
interface UserFormState { username: string }
const editVisible = ref(false)
const editingUser = ref<UserListItem | null>(null)
const userForm = ref<UserFormState>({ username: '' })
const editSaving = ref(false)
const editError = ref('')
```

Use an Element Plus Dialog with a fixed responsive width, read-only email display, inline validation, and explicit cancel/save commands. Use the API's returned `username` and `updatedAt`, not the pre-submit input, when updating local/auth state.

- [ ] **Step 6: Implement the flat role assignment Dialog**

Keep typed state:

```ts
const roleDialogVisible = ref(false)
const roleTarget = ref<UserListItem | null>(null)
const roleData = ref<UserRolesResponse | null>(null)
const selectedRoleIDs = ref<number[]>([])
const roleLoading = ref(false)
const roleSaving = ref(false)
const roleError = ref('')
```

Compute `hasEnabledSelection`, `isSuperAdminActor`, and protected super-admin behavior from strict DTOs and Access Store role codes. `selectAllRoles` and `clearRoles` must preserve the current `super_admin` checkbox for a non-super actor because that actor cannot grant or remove it. Submit a copied sorted/deduplicated array.

- [ ] **Step 7: Constrain Dialog scrolling**

Teleport both Dialogs to `document.body`. Set a viewport-relative maximum Dialog body height with body-only vertical scrolling so long role lists do not push header/footer off-screen. Do not make the page or global body a new scroll owner.

- [ ] **Step 8: Verify both Dialog workflows**

Run:

```powershell
pnpm vitest run src/views/system/users/index.test.ts
pnpm build
git diff --check
```

Expected: PASS; failed mutations preserve user work and successful current-username edits update the Header state immediately.

### Task 16: Add Status And Delete Confirmation Workflows

**Files:**
- Modify: `web/src/views/system/users/index.vue`
- Modify: `web/src/views/system/users/index.test.ts`

- [ ] **Step 1: Write failing status confirmation tests**

For enabled -> disabled, assert the confirmation states that the user is logged out immediately and must log in after re-enable. For disabled -> enabled, assert it does not claim roles or sessions are restored. When the target has `super_admin`, append the localized super-admin impact text. Confirm calls `updateUserStatus(id, nextYesNo)` and refreshes the current query; cancel performs no mutation. API failure keeps the current list visible and exposes the mutation error.

- [ ] **Step 2: Write failing delete confirmation tests**

Assert the confirmation states:

```text
user and active role relations are soft-deleted
old sessions are revoked and not restored
there is no restore entry
the original email may register a new account with a new user ID
super-admin identity impact is explicit when applicable
```

Confirm calls `deleteUser(id)` without data. After success, refresh the current page; if deletion makes the page index invalid, move to the last valid page and issue one corrected request. Cancel and failure retain the row. Protected self/super-target controls never open confirmation.

- [ ] **Step 3: Run mutation tests and confirm RED**

Run:

```powershell
pnpm vitest run src/views/system/users/index.test.ts -t 'status|delete'
```

Expected: FAIL because confirmation workflows are missing.

- [ ] **Step 4: Implement explicit confirmations and mutation state**

Use `ElMessageBox.confirm` and the existing page error mapper. Keep one typed mutation lock so duplicate clicks cannot overlap. Do not optimistically change status or remove a row before the server succeeds; PostgreSQL and the next page response remain authoritative.

- [ ] **Step 5: Prove manual-refresh access policy**

Add a source/behavior assertion that none of the four user mutations call:

```ts
access.load()
window.location.reload()
```

Role changes rely on the next backend permission query immediately and the target user's manual browser refresh for menu/button reconstruction. Status/delete rely on server-side session revocation and normal 401 handling.

- [ ] **Step 6: Verify complete user-page behavior**

Run:

```powershell
pnpm vitest run src/views/system/users/index.test.ts
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src/views/system/users src/api/user.ts src/api/user.contract.ts src/store/auth.ts
git diff --check
```

Expected: tests/build PASS; the forbidden-any scan has no matches and canceled/failed mutations never corrupt the displayed state.

### Task 17: Complete Backend, Frontend, And RBAC Acceptance

**Files:**
- Modify only when a verification failure proves a defect in files already named by this plan.
- Review: `docs/superpowers/specs/2026-08-20-user-management-design.md`
- Review: `docs/superpowers/plans/2026-08-20-user-management.md`

- [ ] **Step 1: Run the complete backend verification gate**

Run from `D:\admin\server`:

```powershell
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

Expected: all commands exit `0`. PostgreSQL tests must run against the configured local database; do not count skipped/connection-failed integration coverage as feature success.

- [ ] **Step 2: Run the complete frontend verification gate**

Run from `D:\admin\web`:

```powershell
pnpm vitest run
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src -g "*.ts" -g "*.vue"
```

Expected: Vitest/build exit `0`; `rg` prints no forbidden business TypeScript matches and therefore exits `1`.

- [ ] **Step 3: Run repository hygiene checks**

Run from `D:\admin`:

```powershell
git diff --check
git status --short
rg -n "FIXME|XXX|placeholder|platform|BaseService|BaseRepository|CacheManager|window\.location\.reload" server/internal/module/user web/src/views/system/users web/src/api/user.ts web/src/api/user.contract.ts -g "*.go" -g "*.ts" -g "*.vue"
```

Expected: `git diff --check` exits `0`; status contains only intentional feature/spec/plan changes; the scan has no placeholder, forbidden abstraction, or forced-reload match. Do not alter unrelated user changes.

- [ ] **Step 4: Perform the RBAC browser acceptance with owner-run services**

The repository owner starts the existing API and frontend commands. In the browser, verify this sequence without starting/stopping services from the plan executor:

```text
super_admin opens /system/users from the dynamic sidebar
-> list/filter/pagination and disabled-role labels render
-> assign two roles to an ordinary user
-> ordinary user manually refreshes
-> menus/buttons equal the union of enabled roles
-> remove one role
-> next protected API request follows PostgreSQL immediately
-> target manually refreshes and the frontend snapshot matches
-> disable target
-> old Access and Refresh requests are rejected
-> enable target
-> target must log in again
-> delete target
-> old identity/roles/sessions do not return
-> register the old email again
-> a new user ID and current default role are created
```

- [ ] **Step 5: Attempt direct-request authorization bypasses**

Using the authenticated browser/API client, verify an ordinary delegated administrator receives the documented errors when directly constructing requests to disable/delete/change their own roles, mutate another super-admin user, grant `super_admin`, or remove the last effective super admin. Confirm no failed request leaves a partial user, relation, or session change.

- [ ] **Step 6: Record the phase checkpoint without committing**

Review the diff against all 18 acceptance criteria in the approved spec. Record verification commands/results in the execution notes, leave the workspace on `main`, and hand control of commit/push and running services to the repository owner. The next separate design work is:

```text
RBAC technical-debt recovery
-> PostgreSQL-authoritative RBAC Redis cache
-> PostgreSQL-authoritative authentication Redis hardening
-> structured operation logging
-> proven frontend component reuse
-> UI and formatting baseline redesign
```

Do not implement any of those items in this plan.

## Spec Coverage Review

| Approved requirement | Implementation task(s) |
| --- | --- |
| Shared username normalization and soft-delete reuse | 1, 5 |
| One authoritative current-session Redis key | 1, 7 |
| Stable user errors and bilingual backend messages | 2 |
| Paged keyword/status/role filtering without duplicate users | 3 |
| Complete role data, disabled roles, corruption detection, no N+1 | 3, 6 |
| Fixed mutation lock order and rollback behavior | 4-7 |
| Self and super-admin target protection | 5-8, 14-16 |
| At least one effective super admin under concurrency | 6, 7 |
| Multi-role minimal-difference save and enabled-role invariant | 6 |
| Disable/delete session revocation and post-commit Redis cleanup | 7 |
| Strict HTTP routes, JSON/query/ID contracts, exact permissions | 8 |
| Request ID plus actor/target/operation/internal-cause access logging without sensitive data | 8 |
| Built-in dynamic menu and existing access-union behavior | 9, 10 |
| Strict TypeScript DTOs and thin API flow | 11, 12 |
| Current username Header synchronization | 13, 15 |
| List, filters, paging, permissions, error retention, scrolling | 14 |
| Flat role selector, select-all/clear, disabled/super roles | 15 |
| Explicit status/delete consequences and manual refresh policy | 16 |
| Full automated gates and browser RBAC acceptance | 17 |
| Technical-debt and UI work kept out of this slice | 17 and non-goals below |

## Deliberate Non-Goals

- No admin-created users, user restore, email/password editing, profiles, export, or batch operations.
- No session list, active kick-out command, login log, operation-log table, or empty audit hook.
- This historical slice did not implement RBAC caching. Current work must follow the later three-layer
  Access baseline: Redis access-version gating plus a bounded process-local immutable snapshot; no
  unbounded or ungated cache and no change to the documented authentication fallback semantics.
- No `AppDialog`, `AppTable`, `usePagedList`, generic search form, generic CRUD, BaseService, BaseRepository, Manager, Factory, Adapter, DI container, or registry.
- No forced access refresh, route hot swap, or browser reload after a role mutation.
- No Docker, worktree, automatic commit, fetch, pull, push, or service lifecycle command.
- No UI theme/density/responsive redesign in this feature; that begins only after RBAC acceptance and technical-debt recovery have their own approved specs.
