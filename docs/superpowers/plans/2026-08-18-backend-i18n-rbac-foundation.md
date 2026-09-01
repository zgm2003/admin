# Backend i18n and RBAC Foundation Implementation Plan

> 历史计划说明（2026-09-01）：所有页面入口权限统一使用 `:view`；列表、详情和写接口使用
> 独立 action permission，页面权限不得自动派生 API 权限。本文旧 `system:*` 示例保留为
> 当时实施记录，不是当前权限码契约；后续实现以最新 Agent 规则和功能 spec 为准。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add backend Chinese/English error localization, PostgreSQL-backed RBAC access calculation and authorization, and a typed frontend access bootstrap that installs dynamic menus, routes, and button permissions once per authenticated application load.

**Architecture:** Keep the backend flow linear: `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`. `sys_role_menu` stores only direct page/action grants; the access service derives ancestors for menu visibility and effective page permission, while each protected backend request checks the current Access Snapshot. The frontend performs `auth -> GET /access -> validate -> store -> register routes -> navigate`, with Dashboard remaining a static protected route and no fallback component or guessed view import.

**Tech Stack:** Go 1.26, Gin, GORM, PostgreSQL, Vue 3, TypeScript 6, Pinia, Vue Router 4, Element Plus, vue-i18n, Vitest, Vue Test Utils.

---

## Execution Rules

- Work directly in `D:\admin`; the repository owner has chosen inline execution without subagents or a worktree.
- Do not start or stop the user's API, Worker, frontend, PostgreSQL, or Redis processes.
- Do not use Docker or Docker Compose.
- Do not commit, fetch, pull, or push. Keep each task inspectable with `git diff`.
- Follow TDD inside every behavior task: add a failing test, run it, implement the smallest behavior, rerun it.
- Keep HTTP responses exactly `code`, `data`, and `message`.
- Pass `context.Context` through every backend I/O call; never pass `*gin.Context` below middleware or handler.
- Do not introduce `platform`, `infra`, generic adapters, Manager, Factory, BaseService, BaseRepository, a DI container, a policy engine, or a runtime registry.
- Do not introduce explicit TypeScript `any`, `any[]`, `as any`, `Record<string, any>`, `@ts-ignore`, or a silent fallback.
- Do not seed fake production menus. PostgreSQL tests create fixtures inside transactions and roll them back.
- This historical plan predates the Redis-gated process cache; use the latest Agent rules for the
  current three-layer Access cache and do not implement a second cache policy here.

## File Map

### Create: backend

```text
server/internal/shared/i18n/catalog.go
server/internal/shared/i18n/catalog_test.go
server/internal/middleware/language.go
server/internal/middleware/language_test.go
server/internal/module/menu/model.go
server/internal/module/menu/schema.go
server/internal/module/menu/schema_test.go
server/internal/module/access/repository.go
server/internal/module/access/repository_test.go
server/internal/module/access/service.go
server/internal/module/access/service_test.go
server/internal/module/access/response.go
server/internal/module/access/handler.go
server/internal/module/access/handler_test.go
server/internal/module/access/middleware.go
server/internal/module/access/middleware_test.go
server/internal/module/access/route.go
```

### Modify: backend

```text
server/internal/shared/apperror/error.go
server/internal/shared/response/response.go
server/internal/shared/response/response_test.go
server/internal/middleware/cors.go
server/internal/middleware/cors_test.go
server/internal/module/auth/service.go
server/internal/module/auth/bootstrap_service.go
server/internal/module/auth/handler_test.go
server/internal/module/taskdemo/route.go
server/internal/module/taskdemo/handler_test.go
server/cmd/api/main.go
server/cmd/api/main_test.go
```

### Create: frontend

```text
web/src/access/menu-icons.ts
web/src/access/protocol.ts
web/src/access/protocol.test.ts
web/src/access/route-views.ts
web/src/api/access.contract.ts
web/src/api/access.contract.test.ts
web/src/api/permission.ts
web/src/api/access.test.ts
web/src/store/permission.ts
web/src/store/access.test.ts
web/src/router/access-routes.ts
web/src/router/access-routes.test.ts
web/src/layout/components/AccessMenuNode.vue
web/src/layout/components/AccessMenuNode.test.ts
```

### Modify: frontend

```text
web/src/i18n/index.ts
web/src/i18n/index.test.ts
web/src/i18n/messages/zh-CN.ts
web/src/i18n/messages/en-US.ts
web/src/router/index.ts
web/src/router/index.test.ts
web/src/permission.ts
web/src/layout/components/AppAside.vue
web/src/layout/index.vue
web/src/layout/index.test.ts
web/src/utils/request.ts
web/src/utils/request.test.ts
```

## Task 1: Add backend locale negotiation and translated errors

**Files:**
- Create: `server/internal/shared/i18n/catalog.go`
- Create: `server/internal/shared/i18n/catalog_test.go`
- Create: `server/internal/middleware/language.go`
- Create: `server/internal/middleware/language_test.go`
- Modify: `server/internal/shared/apperror/error.go`
- Modify: `server/internal/shared/response/response.go`
- Modify: `server/internal/shared/response/response_test.go`
- Modify: `server/internal/middleware/cors.go`
- Modify: `server/internal/middleware/cors_test.go`
- Modify: `server/internal/module/auth/service.go`
- Modify: `server/internal/module/auth/bootstrap_service.go`
- Modify: `server/cmd/api/main.go`

- [x] **Step 1: Write failing catalog and interpolation tests**

Create `server/internal/shared/i18n/catalog_test.go` with table tests for catalog equality, exact interpolation parameters, missing/extra parameters, and both locales:

```go
func TestCatalogsHaveTheSameKeysAndParameters(t *testing.T) {
	if err := i18n.ValidateCatalogs(); err != nil {
		t.Fatalf("ValidateCatalogs() error = %v", err)
	}
}

func TestTranslateUsesTheRequestedLocale(t *testing.T) {
	for _, test := range []struct {
		locale i18n.Locale
		want   string
	}{
		{locale: i18n.ZhCN, want: "请求参数错误"},
		{locale: i18n.EnUS, want: "Invalid request"},
	} {
		got, err := i18n.Translate(test.locale, i18n.KeyInvalidRequest, nil)
		if err != nil || got != test.want {
			t.Fatalf("Translate(%q) = %q,%v, want %q", test.locale, got, err, test.want)
		}
	}
}

func TestTranslateRejectsUnexpectedParameters(t *testing.T) {
	_, missingErr := i18n.Translate(i18n.ZhCN, i18n.KeyPermissionDenied, nil)
	if missingErr == nil {
		t.Fatal("Translate accepted missing interpolation parameters")
	}
	_, err := i18n.Translate(i18n.ZhCN, i18n.KeyInvalidRequest, map[string]string{"field": "email"})
	if err == nil {
		t.Fatal("Translate accepted an unexpected interpolation parameter")
	}
}
```

- [x] **Step 2: Run the catalog tests and verify the missing-package failure**

Run from `D:\admin\server`:

```powershell
go test ./internal/shared/i18n
```

Expected: FAIL because `internal/shared/i18n` does not exist.

- [x] **Step 3: Implement the fixed locale and message catalogs**

Create `catalog.go` with these public contracts and no language fallback:

```go
type Locale string
type MessageKey string

const (
	ZhCN Locale = "zh-CN"
	EnUS Locale = "en-US"

	KeyInternal              MessageKey = "error.internal"
	KeyInvalidRequest        MessageKey = "error.invalidRequest"
	KeyUnauthorized          MessageKey = "error.unauthorized"
	KeyForbidden             MessageKey = "error.forbidden"
	KeyNotFound              MessageKey = "error.notFound"
	KeyConflict              MessageKey = "error.conflict"
	KeyDependencyUnavailable MessageKey = "error.dependencyUnavailable"
	KeyUsernameConflict      MessageKey = "auth.usernameConflict"
	KeyEmailConflict         MessageKey = "auth.emailConflict"
	KeySuperAdminExists      MessageKey = "auth.superAdminExists"
	KeyPermissionDenied      MessageKey = "access.permissionDenied"
	KeyAccessSnapshotInvalid MessageKey = "access.snapshotInvalid"
)

var catalogs = map[Locale]map[MessageKey]string{
	ZhCN: {
		KeyInternal: "服务内部错误", KeyInvalidRequest: "请求参数错误",
		KeyUnauthorized: "未登录或登录已失效", KeyForbidden: "无权执行该操作",
		KeyNotFound: "请求的资源不存在", KeyConflict: "数据冲突",
		KeyDependencyUnavailable: "服务暂未就绪",
		KeyUsernameConflict: "用户名已存在",
		KeyEmailConflict: "邮箱已存在",
		KeySuperAdminExists: "超级管理员已存在",
		KeyPermissionDenied: "无权执行 {{permission}}",
		KeyAccessSnapshotInvalid: "访问权限数据无效",
	},
	EnUS: {
		KeyInternal: "Internal server error", KeyInvalidRequest: "Invalid request",
		KeyUnauthorized: "Authentication is required or has expired", KeyForbidden: "Permission denied",
		KeyNotFound: "Resource not found", KeyConflict: "Data conflict",
		KeyDependencyUnavailable: "Service is temporarily unavailable",
		KeyUsernameConflict: "Username already exists",
		KeyEmailConflict: "Email already exists",
		KeySuperAdminExists: "A super administrator already exists",
		KeyPermissionDenied: "Permission denied: {{permission}}",
		KeyAccessSnapshotInvalid: "Access permission data is invalid",
	},
}
```

Implement `ParseLocale`, `WithLocale`, `LocaleFromContext`, `ValidateCatalogs`, and `Translate`. `ValidateCatalogs` compares exact keys and `{{parameter}}` names. `Translate` rejects an unsupported locale, unknown key, missing parameter, or extra parameter; it never switches languages.

- [x] **Step 4: Convert application errors from literal text to message keys**

Change `apperror.Error` to:

```go
type Error struct {
	HTTPStatus int
	Code       int
	MessageKey i18n.MessageKey
	Params     map[string]string
	Cause      error
}

func (e *Error) Error() string { return string(e.MessageKey) }
```

Keep existing codes `10000-10006`. Keep `InvalidRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `DependencyUnavailable`, and `Internal` signatures unchanged. Change Conflict to accept a key and parameters:

```go
func Conflict(key i18n.MessageKey, params map[string]string, cause error) *Error {
	return newError(http.StatusConflict, CodeConflict, key, params, cause)
}
```

Add `ForbiddenWithParams(key i18n.MessageKey, params map[string]string, cause error)` for permission middleware. Keep `Forbidden(cause)` mapped to the generic forbidden key for existing callers.

Update the three literal conflict call sites exactly:

```go
apperror.Conflict(i18n.KeySuperAdminExists, nil, nil)
apperror.Conflict(i18n.KeyUsernameConflict, nil, err)
apperror.Conflict(i18n.KeyEmailConflict, nil, err)
```

The username and email conflicts have separate keys so neither service code nor interpolation parameters contain translated field names. Do not put public translated text back into `service.go`.

- [x] **Step 5: Make `response.Fail` the only translation point**

Resolve the locale from `context.Request.Context()`, translate `MessageKey` and `Params`, and preserve the exact envelope:

```go
message, translateErr := i18n.Translate(
	i18n.LocaleFromContext(context.Request.Context()),
	appErr.MessageKey,
	appErr.Params,
)
if translateErr != nil {
	appErr = apperror.Internal(errors.Join(appErr, translateErr))
	message, _ = i18n.Translate(i18n.LocaleFromContext(context.Request.Context()), appErr.MessageKey, nil)
}
context.AbortWithStatusJSON(appErr.HTTPStatus, Envelope[any]{
	Code: appErr.Code, Data: nil, Message: message,
})
```

The known internal-error key is the only safe public conversion for an invalid application key. It is not a locale fallback.

- [x] **Step 6: Write failing language middleware tests**

Cover no header, Chinese, English, a browser-style weighted header, and an unsupported first language:

```go
func TestLanguageNegotiation(t *testing.T) {
	tests := []struct {
		name           string
		acceptLanguage string
		wantLanguage   string
		wantMessage    string
	}{
		{name: "missing defaults to Chinese", wantLanguage: "zh-CN", wantMessage: "请求参数错误"},
		{name: "Chinese", acceptLanguage: "zh-CN", wantLanguage: "zh-CN", wantMessage: "请求参数错误"},
		{name: "weighted English", acceptLanguage: "en-US,en;q=0.9", wantLanguage: "en-US", wantMessage: "Invalid request"},
		{name: "unsupported returns Chinese error", acceptLanguage: "fr-FR", wantLanguage: "zh-CN", wantMessage: "请求参数错误"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(Language())
			router.GET("/", func(context *gin.Context) {
				response.Fail(context, apperror.InvalidRequest(errors.New("invalid test request")))
			})
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Accept-Language", test.acceptLanguage)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest || recorder.Header().Get("Content-Language") != test.wantLanguage {
				t.Fatalf("status=%d language=%q", recorder.Code, recorder.Header().Get("Content-Language"))
			}
			var envelope struct { Message string `json:"message"` }
			if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil || envelope.Message != test.wantMessage {
				t.Fatalf("message=%q decodeErr=%v, want %q", envelope.Message, err, test.wantMessage)
			}
		})
	}
}
```

Every assertion checks `Content-Language`, HTTP status, non-zero code on failure, and translated `message`.

- [x] **Step 7: Implement and register the language middleware**

Create `middleware.Language()` that reads the first language range, strips an optional `;q=`, requires exact `zh-CN` or `en-US`, places the locale in the request `context.Context`, and sets `Content-Language`. Missing header selects Chinese. Unsupported input stores Chinese only for the error response, returns `apperror.InvalidRequest`, and aborts.

Call `i18n.ValidateCatalogs()` near the start of `run`. Register Language after request ID, access logging, and recovery so invalid-language requests are logged and recovered normally:

```go
router.Use(
	projectmiddleware.RequestID(),
	projectmiddleware.CORS(dependencies.CORSOrigin),
	projectmiddleware.AccessLog(dependencies.Logger),
	projectmiddleware.Recovery(dependencies.Logger),
	projectmiddleware.Language(),
)
```

Add `Accept-Language` to CORS `AllowHeaders` and `Content-Language` to `ExposeHeaders`. Extend `cors_test.go` with an OPTIONS preflight that requests `accept-language` and asserts it is allowed, then assert a normal response exposes `Content-Language`.

- [x] **Step 8: Run backend i18n and existing error tests**

```powershell
go test ./internal/shared/i18n ./internal/shared/response ./internal/middleware ./internal/module/auth
```

Expected: all packages pass; English requests receive English errors, missing headers receive Chinese, and every response keeps exactly three envelope fields.

## Task 2: Add the menu and direct role-grant schema

**Files:**
- Create: `server/internal/module/menu/model.go`
- Create: `server/internal/module/menu/schema.go`
- Create: `server/internal/module/menu/schema_test.go`
- Modify: `server/cmd/api/main.go`

- [x] **Step 1: Write the PostgreSQL schema integration test**

Create `menu/schema_test.go`. Follow the existing authentication schema test and assert these columns and PostgreSQL types:

```text
sys_menu.id                 bigint NOT NULL
sys_menu.parent_id          bigint NULL
sys_menu.menu_type          character varying(16) NOT NULL
sys_menu.code               character varying(128) NOT NULL
sys_menu.i18n_key           character varying(128) NOT NULL
sys_menu.path               character varying(255) NULL
sys_menu.view_key           character varying(128) NULL
sys_menu.icon               character varying(64) NULL
sys_menu.sort_order         integer NOT NULL
sys_menu.is_enabled         smallint NOT NULL
sys_menu.created_at         timestamp with time zone NOT NULL
sys_menu.updated_at         timestamp with time zone NOT NULL
sys_menu.deleted_at         timestamp with time zone NULL
sys_role_menu.id            bigint NOT NULL
sys_role_menu.role_id       bigint NOT NULL
sys_role_menu.menu_id       bigint NOT NULL
sys_role_menu.created_at    timestamp with time zone NOT NULL
sys_role_menu.updated_at    timestamp with time zone NOT NULL
sys_role_menu.deleted_at    timestamp with time zone NULL
```

Also assert check constraints, three `ON DELETE RESTRICT` foreign keys, the active code index, parent traversal index, and active `(role_id, menu_id)` unique index.

- [x] **Step 2: Run the schema test and verify the missing models**

```powershell
go test ./internal/module/menu -run TestMenuSchema -count=1
```

Expected: FAIL because the menu package and tables do not exist.

- [x] **Step 3: Define explicit GORM models**

Create `menu/model.go` with no embedded base model:

```go
type Type string

const (
	TypeDirectory Type = "directory"
	TypePage      Type = "page"
	TypeAction    Type = "action"
)

type Menu struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	ParentID  *int64         `gorm:"column:parent_id"`
	MenuType  Type           `gorm:"column:menu_type;type:varchar(16);not null"`
	Code      string         `gorm:"column:code;type:varchar(128);not null"`
	I18nKey   string         `gorm:"column:i18n_key;type:varchar(128);not null"`
	Path      *string        `gorm:"column:path;type:varchar(255)"`
	ViewKey   *string        `gorm:"column:view_key;type:varchar(128)"`
	Icon      *string        `gorm:"column:icon;type:varchar(64)"`
	SortOrder int            `gorm:"column:sort_order;not null;default:0"`
	IsEnabled yesno.Value    `gorm:"column:is_enabled;type:smallint;not null;default:1"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

type RoleMenu struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	RoleID    int64          `gorm:"column:role_id;not null"`
	MenuID    int64          `gorm:"column:menu_id;not null"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}
```

Return table names `sys_menu` and `sys_role_menu` explicitly.

- [x] **Step 4: Add concrete schema DDL**

`menu.EnsureSchema` creates these named constraints and indexes idempotently:

```text
ck_sys_menu_type
ck_sys_menu_shape
ck_sys_menu_is_enabled
fk_sys_menu_parent
fk_sys_role_menu_role
fk_sys_role_menu_menu
ux_sys_menu_code_active
ix_sys_menu_parent_active
ux_sys_role_menu_active
```

`ck_sys_menu_shape` permits a directory with nullable path and null view key, requires non-empty path/view key for a page, and requires null path/view key for an action. Foreign keys use `ON DELETE RESTRICT`. Use a package-local `ensureConstraint`; do not move authentication schema code into a generic schema manager.

- [x] **Step 5: Add startup migration in dependency order**

Append `&menu.Menu{}` and `&menu.RoleMenu{}` after role tables in `database.AutoMigrate`, then call `menu.EnsureSchema` after `auth.EnsureSchema`. Keep system role creation after both schemas succeed. Do not create menu rows.

- [x] **Step 6: Verify the real PostgreSQL schema**

```powershell
go test ./internal/module/menu -run TestMenuSchema -count=1
go test ./internal/module/auth -run TestAuthenticationSchema -count=1
```

Expected: both integration tests pass against PostgreSQL; no fake menu data is created.

## Task 3: Query direct grants and check effective permissions

**Files:**
- Create: `server/internal/module/access/repository.go`
- Create: `server/internal/module/access/repository_test.go`

- [x] **Step 1: Write transactional repository tests**

Build a fixture inside one rollback transaction:

```text
system(directory)
`-- users(page, system:user:list)
    |-- create(action, system:user:create)
    `-- delete(action, system:user:delete)
```

Create two enabled roles and one enabled user. Store only the create action in `sys_role_menu`. Tests must prove:

```go
hasCreate, _ := repository.HasPermission(ctx, userID, "system:user:create")
hasList, _ := repository.HasPermission(ctx, userID, "system:user:list")
hasDelete, _ := repository.HasPermission(ctx, userID, "system:user:delete")
if !hasCreate || !hasList || hasDelete {
	t.Fatalf("permissions create=%v list=%v delete=%v, want true true false", hasCreate, hasList, hasDelete)
}
```

Add cases for multi-role union, soft-deleted relationship, disabled role, disabled menu, deleted user, and `super_admin`. Super admin succeeds only when the requested permission code exists on an enabled page/action.

- [x] **Step 2: Run the repository tests and verify the missing implementation**

```powershell
go test ./internal/module/access -run 'TestRepository' -count=1
```

Expected: FAIL because `access.Repository` does not exist.

- [x] **Step 3: Define repository records and load the access source**

Use these concrete types:

```go
type Source struct {
	RoleCodes     []string
	Menus         []menu.Menu
	GrantedMenuIDs []int64
	SuperAdmin    bool
}

type Repository struct { db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository
func (r *Repository) FindSource(ctx context.Context, userID int64) (Source, error)
func (r *Repository) HasPermission(ctx context.Context, userID int64, code string) (bool, error)
```

`FindSource` first requires an enabled, undeleted user, then performs explicit queries in this order: active user roles, all enabled active menu nodes, direct active role-menu IDs. Sort role codes in SQL. Return an error if the user has no active role. Mark `SuperAdmin` from the role code; do not synthesize role-menu rows.

- [x] **Step 4: Implement the recursive permission query**

`HasPermission` uses one PostgreSQL recursive CTE beginning at the required enabled page/action and walking down to descendants. Use `UNION`, not `UNION ALL`, so corrupted cycles terminate instead of looping. It returns true when an active role directly grants any node in that set. A separate branch grants super admin only after the requested enabled permission node exists. Every join filters `deleted_at IS NULL`, enabled user, enabled role, and enabled menu.

Required semantic results:

```text
direct action create -> create true
direct action create -> ancestor page view true
direct action create -> sibling action delete false
direct page view      -> page view true
direct page view      -> child actions false
```

Return database errors with context; never convert errors to `false, nil`.

- [x] **Step 5: Verify direct-grant persistence and effective checks**

```powershell
go test ./internal/module/access -run 'TestRepository' -count=1
```

Expected: all repository tests pass, and fixture assertions confirm only explicitly granted menu IDs exist in `sys_role_menu`.

## Task 4: Build and validate the current access snapshot

**Files:**
- Create: `server/internal/module/access/service.go`
- Create: `server/internal/module/access/service_test.go`

- [x] **Step 1: Write service tests with a focused store stub**

Define a store stub implementing `FindSource` and `HasPermission`. Test an action-only grant with intentionally unsorted input. The exact expected result is:

```go
Snapshot{
	RoleCodes: []string{"ai_tester", "registered_user"},
	MenuTree: []MenuNode{{
		Code: "system", MenuType: menu.TypeDirectory, TitleKey: "navigation.system",
		Children: []MenuNode{{
			Code: "system:user:list", MenuType: menu.TypePage,
			Path: stringPointer("/system/users"), ViewKey: stringPointer("systemUsers"),
			TitleKey: "navigation.systemUsers", Children: []MenuNode{},
		}},
	}},
	PermissionCodes: []string{"system:user:create", "system:user:list"},
}
```

Also test page-only grant, multi-role duplicate grants, super admin, missing parent, parent cycle, directory direct grant, invalid parent type, action with children, duplicate path, and database failure.

- [x] **Step 2: Run tests and verify the missing service**

```powershell
go test ./internal/module/access -run 'TestService' -count=1
```

Expected: FAIL because the service and snapshot types do not exist.

- [x] **Step 3: Define the service boundary and stable response values**

Use a small consumer-owned store interface for tests:

```go
type store interface {
	FindSource(context.Context, int64) (Source, error)
	HasPermission(context.Context, int64, string) (bool, error)
}

type MenuNode struct {
	Code     string
	MenuType menu.Type
	Path     *string
	ViewKey  *string
	TitleKey string
	Icon     *string
	Children []MenuNode
}

type Snapshot struct {
	RoleCodes       []string
	MenuTree        []MenuNode
	PermissionCodes []string
}
```

`Current(ctx, userID)` and `Allowed(ctx, userID, permissionCode)` validate positive IDs and non-empty permission codes before calling the repository.

- [x] **Step 4: Implement ancestor closure without persisting ancestors**

Build `menusByID`, start from direct grant IDs or every enabled page/action for super admin, and walk `ParentID` upward. Track a per-chain visited set to reject cycles and a global selected set to deduplicate. Reject direct directory grants. A selected action contributes its code plus the selected ancestor page code; it never selects child or sibling actions.

Validate selected relationships and fields before returning:

```text
directory parent -> directory/page only
page parent      -> action only
action           -> no children
page             -> non-empty path and viewKey
directory/action -> null viewKey
selected page paths and all selected codes -> unique
```

Sort role codes and permission codes lexicographically. Sort tree siblings by `sort_order`, then code. Initialize every children field as `[]MenuNode{}`, never nil.

- [x] **Step 5: Map failures explicitly**

Define `CodeAccessSnapshotInvalid = 14000`. Repository I/O errors become `apperror.DependencyUnavailable`. Corrupt graph or invalid protocol data becomes an `apperror.Error` with HTTP 500, code 14000, `i18n.KeyAccessSnapshotInvalid`, and the internal cause retained. `Allowed` maps a clean false result to false without an error; middleware decides the 403.

- [x] **Step 6: Run all access service tests**

```powershell
go test ./internal/module/access -run 'TestService' -count=1
```

Expected: all snapshot, sorting, corruption, and fail-closed tests pass.

## Task 5: Expose `/access`, add authorization middleware, and wire routes

**Files:**
- Create: `server/internal/module/access/response.go`
- Create: `server/internal/module/access/handler.go`
- Create: `server/internal/module/access/handler_test.go`
- Create: `server/internal/module/access/middleware.go`
- Create: `server/internal/module/access/middleware_test.go`
- Create: `server/internal/module/access/route.go`
- Modify: `server/internal/module/taskdemo/route.go`
- Modify: `server/internal/module/taskdemo/handler_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`

- [x] **Step 1: Write handler and route tests**

Test `GET /api/v1/access` with a stored authenticated identity and a service snapshot. Assert status 200, exact envelope keys, sorted arrays, and the closed node fields `code`, `menuType`, `path`, `viewKey`, `titleKey`, `icon`, `children`. Assert missing identity is 401 and service failure preserves its code.

Update the API router test to require exactly one `GET /api/v1/access`. Add an unauthenticated request assertion proving the endpoint is protected.

- [x] **Step 2: Write permission middleware tests**

Use a stub with `Allowed(context.Context, int64, string) (bool, error)` and cover:

```text
missing auth identity -> 401, service not called
allowed true          -> next handler runs
allowed false         -> 403
repository error      -> 503/500 from the service, next handler does not run
empty permission code -> panic at route construction or explicit startup error
```

- [x] **Step 3: Run focused HTTP tests and verify failure**

```powershell
go test ./internal/module/access ./cmd/api -run 'TestAccess|TestRequirePermission|TestBuildRouter' -count=1
```

Expected: FAIL because handlers, middleware, and route wiring do not exist.

- [x] **Step 4: Implement the access Handler and route**

`Handler.Current` extracts `auth.Identity`, passes `identity.UserID` and the request context into `service.Current`, maps service values into JSON-tagged response structs, and calls `response.OK`. Register only:

```go
func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc) {
	routes.GET("/access", authenticate, handler.Current)
}
```

Do not merge `/access` into `/auth/me`.

- [x] **Step 5: Implement concrete authorization middleware**

```go
func RequirePermission(service permissionService, permissionCode string) gin.HandlerFunc {
	if strings.TrimSpace(permissionCode) == "" {
		panic("permission code is required")
	}
	return func(context *gin.Context) {
		identity, ok := auth.IdentityFromContext(context)
		if !ok {
			response.Fail(context, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
			return
		}
		allowed, err := service.Allowed(context.Request.Context(), identity.UserID, permissionCode)
		if err != nil { response.Fail(context, err); return }
		if !allowed {
			response.Fail(context, apperror.ForbiddenWithParams(
				i18n.KeyPermissionDenied,
				map[string]string{"permission": permissionCode},
				fmt.Errorf("permission %s is required", permissionCode),
			))
			return
		}
		context.Next()
	}
}
```

Business modules will declare their own permission constants and pass them at route registration. Do not create a central permission-code catalog.

- [x] **Step 6: Wire concrete dependencies in `cmd/api`**

Construct one access repository, service, and handler from PostgreSQL. Extend `routerDependencies` with `Access *access.Handler`. Register access routes after auth routes. The access route uses the same `auth.Authenticate(authService)` middleware instance.

Protect `POST /api/v1/example-tasks` with authentication because it is a Dashboard operation. Do not invent a menu or RBAC permission for Dashboard:

```go
func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc) {
	routes.POST("/example-tasks", authenticate, handler.Create)
}
```

Health/readiness remain public. Auth endpoints keep their existing authentication/origin rules.

- [x] **Step 7: Verify HTTP and middleware behavior**

```powershell
go test ./internal/module/access ./internal/module/taskdemo ./cmd/api -count=1
```

Expected: `/access` is authenticated, permission middleware fails closed, Dashboard task submission requires authentication, and no fake business route or menu exists.

## Task 6: Add the strict frontend access contract and store

**Files:**
- Create: `web/src/access/menu-icons.ts`
- Create: `web/src/access/route-views.ts`
- Create: `web/src/access/protocol.ts`
- Create: `web/src/access/protocol.test.ts`
- Create: `web/src/api/access.contract.ts`
- Create: `web/src/api/access.contract.test.ts`
- Create: `web/src/api/access.ts`
- Create: `web/src/api/access.test.ts`
- Create: `../../../web/src/store/permission.ts`
- Create: `web/src/store/access.test.ts`
- Modify: `web/src/i18n/index.ts`
- Modify: `web/src/i18n/index.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

- [x] **Step 1: Add failing closed-contract tests**

Test a valid directory-only snapshot and reject:

```text
null arrays
missing or extra object keys
duplicate roleCodes or permissionCodes
unsorted arrays
unknown menuType
action inside menuTree
page with null path/viewKey
directory with non-null viewKey
duplicate menu code or page path
unknown titleKey
unknown icon
unknown viewKey
```

Use `unknown` fixtures and never cast them to the DTO.

- [x] **Step 2: Run tests and verify missing frontend modules**

Run from `D:\admin\web`:

```powershell
pnpm vitest run src/api/access.contract.test.ts src/access/protocol.test.ts src/store/access.test.ts
```

Expected: FAIL because the access files do not exist.

- [x] **Step 3: Define explicit title, icon, and view protocols**

Export `isAppMessageKey(value: string): value is AppMessageKey` from `i18n/index.ts` using `Object.prototype.hasOwnProperty.call(zhCN, value)`.

Create an explicit Element Plus icon map:

```ts
export const menuIcons = {
  Cpu,
  Folder,
  Key,
  Menu: MenuIcon,
  Setting,
  User,
} as const

export type MenuIconKey = keyof typeof menuIcons
```

Create the real view map with no fake views:

```ts
export type RouteViewLoader = () => Promise<Component>
export type RouteViewMap = Readonly<Record<string, RouteViewLoader>>
export const routeViews: RouteViewMap = {}
```

`protocol.ts` exports `isMenuIconKey` and `hasRouteViewKey`. The access parser calls them while narrowing unknown node fields, so an icon absent from `menuIcons` or a page view key absent from `routeViews` is rejected before an `AccessSnapshot` exists. The map stays empty until the next system-management slice adds real views and mappings.

- [x] **Step 4: Implement strict DTO parsing**

Define:

```ts
export type MenuType = 'directory' | 'page'

export interface AccessMenuNode {
  code: string
  menuType: MenuType
  path: string | null
  viewKey: string | null
  titleKey: AppMessageKey
  icon: MenuIconKey | null
  children: AccessMenuNode[]
}

export interface AccessSnapshot {
  roleCodes: string[]
  menuTree: AccessMenuNode[]
  permissionCodes: string[]
}
```

`parseAccessSnapshot(value: unknown)` uses closed-record checks at every level, validates positive structure, exact sorted/deduplicated arrays, known title keys, known icon keys, known page view keys, and menu shape. `getAccess` calls `request<unknown>` and returns only the parsed result.

- [x] **Step 5: Implement a concrete Pinia access store with one in-flight Promise**

Use a setup store so the Promise remains private and non-reactive:

```ts
export type AccessStatus = 'idle' | 'loading' | 'ready' | 'error'

export const useAccessStore = defineStore('access', () => {
  const roleCodes = ref<string[]>([])
  const menuTree = ref<AccessMenuNode[]>([])
  const permissionCodes = ref<string[]>([])
  const status = ref<AccessStatus>('idle')
  const errorMessage = ref('')
  let loadPromise: Promise<void> | null = null
  let generation = 0

  function hasPermission(code: string): boolean {
    return permissionCodes.value.includes(code)
  }

  function applySnapshot(snapshot: AccessSnapshot): void {
    roleCodes.value = [...snapshot.roleCodes]
    menuTree.value = snapshot.menuTree
    permissionCodes.value = [...snapshot.permissionCodes]
    status.value = 'ready'
    errorMessage.value = ''
  }

  function fail(error: unknown): void {
    roleCodes.value = []
    menuTree.value = []
    permissionCodes.value = []
    status.value = 'error'
    errorMessage.value = error instanceof ProtocolError
      ? appI18n.global.t('access.invalidProtocol')
      : error instanceof ApiError && error.message !== ''
      ? error.message
      : appI18n.global.t('access.loadFailed')
  }

  function reset(): void {
    generation += 1
    loadPromise = null
    roleCodes.value = []
    menuTree.value = []
    permissionCodes.value = []
    status.value = 'idle'
    errorMessage.value = ''
  }

  async function load(): Promise<void> {
    if (status.value === 'ready') return
    if (loadPromise !== null) return loadPromise
    status.value = 'loading'
    const requestGeneration = generation
    const pending = getAccess()
      .then((snapshot) => {
        if (generation === requestGeneration) applySnapshot(snapshot)
      })
      .catch((error: unknown) => {
        if (generation === requestGeneration) fail(error)
        throw error
      })
    loadPromise = pending
    pending.finally(() => {
      if (loadPromise === pending) loadPromise = null
    }).catch(() => undefined)
    return pending
  }

  return { roleCodes, menuTree, permissionCodes, status, errorMessage, hasPermission, applySnapshot, fail, load, reset }
})
```

Two simultaneous `load()` calls share the same Promise and call `getAccess` once. On failure, clear all arrays, set `error`, save a localized public message, and rethrow. `reset()` clears the Promise and all user-specific values.

Add a deferred-Promise test that calls `load()`, then `reset()`, then resolves the old request. The store must remain idle and empty. This prevents a late response from the previous account restoring stale permissions.

- [x] **Step 6: Add access messages to both frontend catalogs**

Add exactly matching keys:

```text
access.loadFailed
access.invalidProtocol
access.permissionDenied
```

Keep `enUS satisfies Record<AppMessageKey, string>` and `fallbackLocale: false`.

- [x] **Step 7: Verify contracts, store concurrency, and strict types**

```powershell
pnpm vitest run src/i18n/index.test.ts src/api/access.contract.test.ts src/api/access.test.ts src/access/protocol.test.ts src/store/access.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: all tests pass, concurrent load calls issue one API request, and TypeScript has no diagnostics.

## Task 7: Register dynamic routes once during authenticated bootstrap

**Files:**
- Create: `web/src/router/access-routes.ts`
- Create: `web/src/router/access-routes.test.ts`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/router/index.test.ts`
- Modify: `web/src/permission.ts`

- [x] **Step 1: Write dynamic route registration tests**

Use an in-memory router, a typed page fixture, and an injected test view map. Assert:

```text
directories do not register routes
pages register below named admin-layout
page titleKey becomes route meta
unknown viewKey throws ProtocolError
duplicate path or route name throws ProtocolError
the returned cleanup function removes every installed route
```

The production `routeViews` remains empty; the test injects `{ systemUsers: async () => TestView }`.

- [x] **Step 2: Write guard timing tests before implementation**

Extend `router/index.test.ts` mocks with `getAccess`. Cover these exact sequences:

```text
cold dynamic URL: refresh -> me -> access -> register -> original URL
authenticated Dashboard: access once -> Dashboard
click between two installed pages: no additional access call
two concurrent protected navigations: one access call
access failure on Dashboard: Dashboard remains, store error
access failure on dynamic URL: redirect Dashboard
logout/anonymous navigation: dynamic route cleanup
public Login: no access call
```

- [x] **Step 3: Run router tests and verify failures**

```powershell
pnpm vitest run src/router/access-routes.test.ts src/router/index.test.ts
```

Expected: FAIL because the access route builder and access bootstrap do not exist.

- [x] **Step 4: Name the static layout and build flat page routes**

Set the static protected layout route name to `admin-layout`. `registerAccessRoutes(router, menuTree, views = routeViews)` traverses directory nodes, converts only pages to absolute child routes, resolves each view key explicitly, and calls:

```ts
const remove = router.addRoute('admin-layout', {
  path: node.path,
  name: `access:${node.code}`,
  component: views[node.viewKey],
  meta: { requiresAuth: true, titleKey: node.titleKey },
})
```

Validate before adding any route. If validation succeeds, register all pages and return one cleanup function that calls every remove callback in reverse order. Do not use `import.meta.glob`, a directory RouterView, or an empty component.

If Vue Router throws after registration begins, `registerAccessRoutes` removes every callback already created in that call before rethrowing. The guard catches the error, calls `access.fail(error)`, and follows the same Dashboard-only failure path as an `/access` loading error.

- [x] **Step 5: Extend the existing guard in one linear flow**

Keep route cleanup inside the `installPermissionGuard` closure:

```text
validate matched route metadata
→ public route handling
→ restore auth when required
→ load access once
→ register all dynamic page routes once
→ retry unmatched original URL once
→ allow Dashboard or matched business route
```

Treat an unmatched frontend URL as a protected candidate, because a cold dynamic URL is unmatched before access routes are installed. After registration, resolve it again; if still unmatched, go to Dashboard. If access loading fails, allow only static Dashboard and redirect other protected targets there. Never retry access on a menu click when store status is `ready` or `error`.

When auth becomes anonymous, call the route cleanup function and `access.reset()` before returning Login. This guarantees logout removes the previous account's routes without a global route registry.

- [x] **Step 6: Verify exact bootstrap ordering**

```powershell
pnpm vitest run src/router/access-routes.test.ts src/router/index.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: cold dynamic navigation follows `refresh -> me -> access -> route registration -> original URL`, while subsequent menu navigation does not call `/access`.

## Task 8: Render dynamic menus and finish request/logout failure behavior

**Files:**
- Create: `web/src/layout/components/AccessMenuNode.vue`
- Create: `web/src/layout/components/AccessMenuNode.test.ts`
- Modify: `web/src/layout/components/AppAside.vue`
- Modify: `web/src/layout/index.vue`
- Modify: `web/src/layout/index.test.ts`
- Modify: `web/src/utils/request.ts`
- Modify: `web/src/utils/request.test.ts`

- [x] **Step 1: Write menu rendering tests**

Mount `AccessMenuNode` and `AppAside` with a ready access store. Assert:

```text
Dashboard is always the first static item
directory renders el-sub-menu and never navigates
page renders el-menu-item with its absolute path
title uses the active frontend locale
known icon renders through menuIcons
collapsed and mobile Aside receive the same access tree
empty menuTree still shows Dashboard
access error keeps RouterView mounted and shows one non-closable error alert
```

- [x] **Step 2: Implement the recursive concrete menu component**

`AccessMenuNode.vue` accepts one `AccessMenuNode`. It renders directory children recursively and a page as one `el-menu-item`. Resolve icons only from `menuIcons`; protocol validation guarantees the key exists. Use the menu code for directory indexes and page path for navigation. Do not add a menu store wrapper or global component registration.

Update `AppAside.vue` to read `useAccessStore().menuTree`, retain the hardcoded Dashboard item, use `route.path` as `default-active`, and render one `AccessMenuNode` per root.

In `layout/index.vue`, render one non-closable `el-alert` with `data-testid="access-error"` and `access.errorMessage` when status is `error`. Place it inside Main before RouterView so the static Dashboard remains mounted. Do not show an empty menu as a successful access state and do not add a second notification for an API error already handled by the request client.

- [x] **Step 3: Write request language and final 401 tests**

Extend request tests to assert every normal request and raw refresh request sends the current `Accept-Language`. Add an injected final-unauthorized callback and prove:

```text
three concurrent 401 responses -> one refresh request, one notification, one final callback
403 -> notification only, no refresh, no callback
401 recovered by refresh -> no notification and no callback
```

- [x] **Step 4: Add request locale headers and one final unauthorized action**

Apply `Accept-Language: readLocale()` in request interceptors for both the normal and raw refresh clients. Extend the internal request-client builder with one concrete `onUnauthorized: () => void` callback; the exported test factory accepts it as an optional third argument.

For the default application client, the callback clears the access store and calls `window.location.assign` with `/login?redirect=<encoded current path>`. The full-page navigation removes installed dynamic routes without importing Vue Router into the request module or creating an event bus. It runs only after Refresh itself returns 401. The existing coordinated refresh Promise prevents duplicate callback and notification execution. A 403 never clears auth and never redirects.

- [x] **Step 5: Clear access state during explicit logout**

In `layout/index.vue`, after the logout API succeeds:

```ts
access.reset()
auth.setAnonymous()
await router.replace({ name: 'login' })
```

The guard observes anonymous state and invokes its local dynamic-route cleanup. Keep logout failure behavior unchanged so a failed logout does not pretend local success.

- [x] **Step 6: Verify menu, request, and logout behavior**

```powershell
pnpm vitest run src/layout/components/AccessMenuNode.test.ts src/layout/index.test.ts src/utils/request.test.ts src/router/index.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: dynamic menus render from the access tree, Dashboard remains static with an explicit access-error alert on failure, final 401 clears access and redirects once, 403 stays on the page, and explicit logout removes user-specific access state.

## Task 9: Run complete verification and scope audit

**Files:**
- Verify every file listed in this plan

- [x] **Step 1: Format and run all backend checks**

Run from `D:\admin\server`:

```powershell
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

Expected: every command exits 0. PostgreSQL integration tests validate the real schema and transactional access fixtures.

- [x] **Step 2: Run the complete frontend suite and build**

Run from `D:\admin\web`:

```powershell
pnpm vitest run
pnpm exec vue-tsc -b --pretty false
pnpm build
```

Expected: every test passes, strict TypeScript emits no diagnostics, and Vite builds successfully. Report a chunk-size warning separately if it remains informational.

- [x] **Step 3: Scan forbidden TypeScript and architecture names**

Run from `D:\admin`:

```powershell
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore|Platform|Adapter|Manager|Factory|BaseService|BaseRepository|import\.meta\.glob" server web/src -g "*.go" -g "*.ts" -g "*.vue"
```

Expected: no newly introduced forbidden match. Inspect any pre-existing proper noun before changing it.

- [x] **Step 4: Audit the exact contracts**

```powershell
rg -n 'json:"msg"|"msg"\s*:' server web/src -g "*.go" -g "*.ts" -g "*.vue"
rg -n "is_del|is_deleted" server -g "*.go"
rg -n "GET\(\"/access\"|RequirePermission|Content-Language|Accept-Language|permissionCodes|viewKey" server web/src -g "*.go" -g "*.ts" -g "*.vue"
```

Expected: no `msg` compatibility field or duplicate deletion flag; positive scan shows the explicit i18n/access contracts.

- [x] **Step 5: Check repository scope and document status**

```powershell
git diff --check
git status --short --branch
git diff --stat
git diff -- server web/src docs/superpowers/specs/2026-08-18-backend-i18n-rbac-foundation-design.md docs/superpowers/plans/2026-08-18-backend-i18n-rbac-foundation.md
```

Confirm no Redis permission cache, process cache, fake production menu, system-management CRUD page, service process change, commit, fetch, pull, or push occurred.

## Self-Review Against the Approved Spec

- Backend locale parsing, Chinese default, unsupported-language rejection, `Content-Language`, catalog parity, exact interpolation, startup validation, and translation only in `response.Fail` are covered by Task 1.
- Explicit `TIMESTAMPTZ`, `deleted_at`, Yes/No, nullable root parent, menu shape, indexes, and restrictive foreign keys are covered by Task 2.
- Direct-grant-only persistence, page/action semantics, ancestor permission, sibling isolation, multi-role union, disabled/deleted filtering, and super-admin behavior are covered by Tasks 3-4.
- Sorted non-null `/access`, authentication without an RBAC permission, graph corruption errors, and PostgreSQL fail-closed behavior are covered by Tasks 4-5.
- Route-level permission middleware and concrete module-owned permission-code usage are established in Task 5 without adding a fake protected resource.
- Strict unknown-data parsing, known i18n/icon/view protocols, no TypeScript `any`, one access Promise, and button-level `hasPermission` are covered by Task 6.
- Static Dashboard, no directory components, explicit view map, cold-route timing, one `/access` request, no menu-click reload, and dynamic-route cleanup are covered by Task 7.
- Dynamic sidebar rendering, locale request headers, final 401 redirect, 403 stay-in-place, access failure state, and logout cleanup are covered by Task 8.
- Redis L2, permission versions, and process L1 remain documented future boundaries and are deliberately absent from implementation tasks.

The plan contains no placeholder task, undefined file reference, compatibility response field, or speculative abstraction. Backend names use `ID`/`HTTP` conventions, JSON and TypeScript use lower camel case, PostgreSQL uses lower snake case, and the direct-grant/effective-permission semantics remain consistent across repository, service, API, store, and router tasks.
