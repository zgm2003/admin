# Admin API 命名空间基线 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将共享身份接口稳定在 `/api/v1`，将全部 Admin 管理接口迁移到受 `admin` 平台强制约束的 `/api/admin/v1`，并一次性同步审计、前端和当前开发文档。

**Architecture:** 组合根建立 shared/admin 两个明确的 Gin Router Group，业务模块继续只注册资源相对路径。Admin Group 在现有客户端 Header 校验后执行具体的 Admin 平台 Middleware；前端业务 API 直接使用明确最终 URL，不增加路径 Factory、重写拦截器或兼容别名。

**Tech Stack:** Go 1.25、Gin、现有 `authclient`/`apperror`/`response`、Vue 3、TypeScript、Axios、Vitest、Go testing

**Spec:** `docs/superpowers/specs/2026-08-27-admin-api-namespace-baseline-design.md`

## Global Constraints

- 共享身份接口只使用 `/api/v1`；Admin 管理接口只使用 `/api/admin/v1`。
- `/api/admin/v1` 必须先通过 `authclient.Require()`，再强制 `X-Auth-Platform: admin`。
- 合法非 Admin 平台访问 Admin API 返回 HTTP `403`、`code=10003`；非法客户端 Header 仍返回 HTTP `400`、`code=10001`。
- 旧管理路径不注册别名、不重定向、不在 Axios 中静默改写，迁移后返回 `404`。
- Refresh Cookie Path 与自动刷新接口保持 `/api/v1/auth`；Auth 与 Access API 不迁移。
- 历史操作日志 route 不回填；只有当前路由、规则和新事件测试改用 `/api/admin/v1`。
- 不修改 Handler、Service、Repository、DTO、数据库 Schema、缓存键或权限码。
- 不新增通用 Router Factory、Adapter、Manager、DI 容器或运行时注册器。
- 不使用 Playwright；按用户约定由人工完成必要验收。
- 不自动提交；每个 Commit 步骤只有在维护者明确授权时执行，且标题遵守 `<type>: <中文摘要>`。

---

## File Map

### Admin 平台请求边界

- `server/internal/module/authclient/middleware.go`：在已校验客户端 Context 上强制 Admin 平台。
- `server/internal/module/authclient/middleware_test.go`：覆盖 Admin 放行和其他合法平台拒绝。

### 后端命名空间与审计

- `server/cmd/api/main.go`：组合 shared/admin 两个 Router Group。
- `server/cmd/api/main_test.go`：锁定完整路由表、旧路径 404 和 Admin 平台边界。
- `server/internal/module/*/handler_test.go`：让管理模块协议测试使用真实 Admin 前缀。
- `server/internal/module/operationlog/rules.go`：共享认证规则保持不变，管理动作规则迁移。
- `server/internal/module/operationlog/*_test.go`、`server/cmd/worker/main_test.go`：当前事件使用新 route；显式 legacy 数据继续保留旧 route。

### Admin Web

- `web/src/api/{auth-platform,menu,role,user,session,operation-log,taskDemo}.ts`：管理请求迁移到 `/api/admin/v1`。
- `web/tests/api/*.test.ts`：精确断言新管理路径，并锁定 Auth、Access、Refresh 仍使用共享路径。
- `web/tests/views/system/operation-logs/index.test.ts`：当前日志 DTO fixture 使用新 route。

### 当前文档

- `docs/learning/admin-architecture-and-crud.md`：当前开发示例改用最终 Admin 路径，共享 `/access` 保持不变。
- `docs/superpowers/specs/2026-08-27-admin-api-namespace-baseline-design.md`：完成后更新状态。

### 明确保留的历史旧路径

- `server/internal/database/domain_names_test.go`：域名迁移测试中的历史操作日志行。
- `server/internal/module/operationlog/schema_test.go`：legacy schema 数据与历史读取 fixture。
- 已完成的历史 spec/plan：保留原始决策记录，由当前命名空间 spec 覆盖冲突路径。

---

### Task 1: 建立 Admin 平台请求 Middleware

**Files:**
- Modify: `server/internal/module/authclient/middleware.go`
- Modify: `server/internal/module/authclient/middleware_test.go`

**Interfaces:**
- Consumes: `authclient.Client` stored by `authclient.Require()`
- Produces: `authclient.RequireAdminPlatform() gin.HandlerFunc`
- Produces: non-Admin platform response `HTTP 403` with `apperror.CodeForbidden`

- [ ] **Step 1: Write the failing Admin platform Middleware test**

Add a table test that runs the real client Middleware before the new Middleware and proves the downstream Handler call count:

```go
func TestRequireAdminPlatformAllowsOnlyAdmin(t *testing.T) {
	tests := []struct {
		platform  string
		wantStatus int
		wantCalls  int
	}{
		{platform: "admin", wantStatus: http.StatusNoContent, wantCalls: 1},
		{platform: "portal", wantStatus: http.StatusForbidden, wantCalls: 0},
	}
	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			calls := 0
			router := gin.New()
			router.Use(authclient.Require(), authclient.RequireAdminPlatform())
			router.GET("/", func(context *gin.Context) {
				calls++
				context.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set(authclient.PlatformHeader, tt.platform)
			request.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus || calls != tt.wantCalls {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
			}
			if tt.wantStatus == http.StatusForbidden && recorder.Body.String() != `{"code":10003,"data":null,"message":"无权执行该操作"}` {
				t.Fatalf("body=%s", recorder.Body.String())
			}
		})
	}
}
```

The production change that makes this test pass is the concrete `RequireAdminPlatform` Middleware; do not mock `FromContext` or the response writer.

- [ ] **Step 2: Run the test and verify the symbol is missing**

```powershell
cd D:\admin\server
go test ./internal/module/authclient -run TestRequireAdminPlatformAllowsOnlyAdmin -count=1
```

Expected: FAIL to compile because `authclient.RequireAdminPlatform` is undefined.

- [ ] **Step 3: Implement the concrete Middleware**

In `middleware.go`, add:

```go
func RequireAdminPlatform() gin.HandlerFunc {
	return func(context *gin.Context) {
		client, ok := FromContext(context)
		if !ok {
			response.Fail(context, apperror.Internal(fmt.Errorf("authentication client context is missing")))
			return
		}
		if client.Platform != "admin" {
			response.Fail(context, apperror.Forbidden(fmt.Errorf("authentication platform cannot access admin API")))
			return
		}
		context.Next()
	}
}
```

Reuse the file's existing `fmt`, `apperror`, `response` and Gin imports. Do not re-read headers, query Policy, authenticate Tokens or accept a configurable platform argument.

- [ ] **Step 4: Run all authclient tests**

```powershell
cd D:\admin\server
go test ./internal/module/authclient -count=1
```

Expected: PASS. Existing malformed Header cases must remain HTTP 400.

- [ ] **Step 5: Commit only if explicitly authorized**

```powershell
git add server/internal/module/authclient/middleware.go server/internal/module/authclient/middleware_test.go
git commit -m "feat: 增加管理接口平台边界"
```

### Task 2: 迁移后端路由与操作日志规则

**Files:**
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`
- Modify: `server/internal/module/authplatform/handler_test.go`
- Modify: `server/internal/module/menu/handler_test.go`
- Modify: `server/internal/module/role/handler_test.go`
- Modify: `server/internal/module/user/handler_test.go`
- Modify: `server/internal/module/auth/session_admin_handler_test.go`
- Modify: `server/internal/module/operationlog/handler_test.go`
- Modify: `server/internal/module/taskdemo/handler_test.go`
- Modify: `server/internal/module/operationlog/rules.go`
- Modify: `server/internal/module/operationlog/middleware_test.go`
- Modify: `server/internal/module/operationlog/integration_test.go`
- Modify: `server/internal/module/operationlog/task_test.go`
- Modify: `server/cmd/worker/main_test.go`

**Interfaces:**
- Consumes: `authclient.RequireAdminPlatform()` from Task 1
- Produces: shared group `/api/v1`
- Produces: Admin group `/api/admin/v1`
- Produces: audit rules whose management route patterns exactly match the Admin group
- Preserves: legacy old-route fixtures in `database/domain_names_test.go` and `operationlog/schema_test.go`

- [ ] **Step 1: Write failing composition-root route and platform tests**

In `main_test.go`, change the expected route map so Auth and Access remain under `/api/v1`, while every management route from the spec uses `/api/admin/v1`. Add explicit old-path and platform assertions:

```go
want := map[string]int{
	"GET /health":                                   1,
	"GET /ready":                                    1,
	"GET /api/v1/auth/policy":                       1,
	"POST /api/v1/auth/register":                    1,
	"POST /api/v1/auth/login":                       1,
	"POST /api/v1/auth/refresh":                     1,
	"POST /api/v1/auth/logout":                      1,
	"GET /api/v1/auth/me":                           1,
	"GET /api/v1/access":                            1,
	"GET /api/admin/v1/auth-platforms":              1,
	"GET /api/admin/v1/auth-platforms/deployment":   1,
	"POST /api/admin/v1/auth-platforms":             1,
	"PUT /api/admin/v1/auth-platforms/:id":          1,
	"PATCH /api/admin/v1/auth-platforms/:id/status": 1,
	"DELETE /api/admin/v1/auth-platforms/:id":       1,
	"GET /api/admin/v1/menus":                       1,
	"POST /api/admin/v1/menus":                      1,
	"PUT /api/admin/v1/menus/:id":                   1,
	"PATCH /api/admin/v1/menus/:id/status":          1,
	"DELETE /api/admin/v1/menus/:id":                1,
	"GET /api/admin/v1/roles":                       1,
	"POST /api/admin/v1/roles":                      1,
	"PUT /api/admin/v1/roles/:id":                   1,
	"PATCH /api/admin/v1/roles/:id/status":          1,
	"PATCH /api/admin/v1/roles/:id/default":         1,
	"DELETE /api/admin/v1/roles/:id":                1,
	"GET /api/admin/v1/roles/:id/permissions":       1,
	"PUT /api/admin/v1/roles/:id/permissions":       1,
	"GET /api/admin/v1/users":                       1,
	"GET /api/admin/v1/users/role-options":          1,
	"PUT /api/admin/v1/users/:id":                   1,
	"PATCH /api/admin/v1/users/:id/status":          1,
	"DELETE /api/admin/v1/users/:id":                1,
	"GET /api/admin/v1/users/:id/roles":             1,
	"PUT /api/admin/v1/users/:id/roles":             1,
	"GET /api/admin/v1/sessions":                    1,
	"GET /api/admin/v1/sessions/stats":              1,
	"DELETE /api/admin/v1/sessions/:id":             1,
	"DELETE /api/admin/v1/sessions":                 1,
	"GET /api/admin/v1/operation-logs":              1,
	"POST /api/admin/v1/example-tasks":              1,
}

for _, oldRoute := range []struct {
	method string
	path   string
}{
	{method: http.MethodGet, path: "/api/v1/auth-platforms"},
	{method: http.MethodGet, path: "/api/v1/menus"},
	{method: http.MethodGet, path: "/api/v1/roles"},
	{method: http.MethodGet, path: "/api/v1/users"},
	{method: http.MethodGet, path: "/api/v1/sessions"},
	{method: http.MethodGet, path: "/api/v1/operation-logs"},
	{method: http.MethodPost, path: "/api/v1/example-tasks"},
} {
	request := httptest.NewRequest(oldRoute.method, oldRoute.path, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("old route %s %s status=%d", oldRoute.method, oldRoute.path, recorder.Code)
	}
}

request := httptest.NewRequest(http.MethodGet, "/api/admin/v1/menus", nil)
request.Header.Set(authclient.PlatformHeader, "portal")
request.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
recorder := httptest.NewRecorder()
router.ServeHTTP(recorder, request)
if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), `"code":10003`) {
	t.Fatalf("non-admin response status=%d body=%s", recorder.Code, recorder.Body.String())
}
```

Keep the existing missing-client and missing-Bearer assertions, using `/api/admin/v1/example-tasks` and `/api/admin/v1/menus` for Admin cases and `/api/v1/access` for the shared case.

- [ ] **Step 2: Write failing operation-log rule tests**

Change current mutation expectations to new paths and assert an old management rule is absent while shared Auth remains:

```go
if rule, ok := FindRule(http.MethodPut, "/api/admin/v1/users/:id"); !ok || rule.Action != "user.update" {
	t.Fatalf("user update rule = %+v,%v", rule, ok)
}
if _, ok := FindRule(http.MethodPut, "/api/v1/users/:id"); ok {
	t.Fatal("legacy user update rule remains registered")
}
if _, ok := FindRule(http.MethodPost, "/api/v1/auth/login"); !ok {
	t.Fatal("shared login audit rule is missing")
}
```

Update current Middleware request routes in `middleware_test.go` to `/api/admin/v1/users/...`. Do not change the explicit `legacy-request` rows in `schema_test.go`.

- [ ] **Step 3: Run the targeted backend tests and verify old composition fails**

```powershell
cd D:\admin\server
go test ./cmd/api ./internal/module/operationlog -count=1
```

Expected: FAIL because `buildRouter` and `routeRules` still use `/api/v1` for management routes.

- [ ] **Step 4: Split the composition root into shared and Admin groups**

Replace the single group in `buildRouter` with:

```go
sharedRoutes := router.Group("/api/v1")
sharedRoutes.Use(authclient.Require())
auth.RegisterRoutes(sharedRoutes, dependencies.Auth, dependencies.AuthOrigin, dependencies.Authenticate)
authplatform.RegisterPublicRoutes(sharedRoutes, dependencies.AuthPlatform)
access.RegisterRoutes(sharedRoutes, dependencies.Access, dependencies.Authenticate)

adminRoutes := router.Group("/api/admin/v1")
adminRoutes.Use(authclient.Require(), authclient.RequireAdminPlatform())
authplatform.RegisterManagementRoutes(adminRoutes, dependencies.AuthPlatform, dependencies.Authenticate, dependencies.RequirePermission)
menu.RegisterRoutes(adminRoutes, dependencies.Menu, dependencies.Authenticate, dependencies.RequirePermission)
role.RegisterRoutes(adminRoutes, dependencies.Role, dependencies.Authenticate, dependencies.RequirePermission)
user.RegisterRoutes(adminRoutes, dependencies.User, dependencies.Authenticate, dependencies.RequirePermission)
operationlog.RegisterRoutes(adminRoutes, dependencies.OperationLog, dependencies.Authenticate, dependencies.RequirePermission)
auth.RegisterSessionAdminRoutes(adminRoutes, dependencies.SessionAdmin, dependencies.Authenticate, dependencies.RequirePermission)
taskdemo.RegisterRoutes(adminRoutes, dependencies.Task, dependencies.Authenticate)
```

Keep global Middleware order unchanged. Do not introduce a helper that accepts arbitrary route registrars.

- [ ] **Step 5: Migrate active operation-log rules and current event fixtures**

In `rules.go`, retain these shared rules exactly:

```text
/api/v1/auth/register
/api/v1/auth/login
/api/v1/auth/refresh
/api/v1/auth/logout
```

Change every auth-platform, menu, role, user, example-task and session mutation rule to `/api/admin/v1/...`. Change current event payloads in `middleware_test.go`, `integration_test.go`, `task_test.go` and `cmd/worker/main_test.go` to the new route.

Do not edit these historical fixtures:

```text
server/internal/database/domain_names_test.go
server/internal/module/operationlog/schema_test.go
```

- [ ] **Step 6: Align management Handler protocol tests with their real group**

Replace management test groups and request URLs with `/api/admin/v1` in the listed module tests. In `authplatform/handler_test.go`, change only `RegisterManagementRoutes` tests; keep `RegisterPublicRoutes` Policy tests on `/api/v1/auth/policy`. No Handler production file changes are required.

- [ ] **Step 7: Run the complete backend tests**

```powershell
cd D:\admin\server
go test ./... -count=1
```

Expected: PASS, including API composition, Worker operation payload, audit integration, and all module Handler tests.

- [ ] **Step 8: Commit only if explicitly authorized**

```powershell
git add server/cmd/api/main.go server/cmd/api/main_test.go server/cmd/worker/main_test.go server/internal/module/authplatform/handler_test.go server/internal/module/menu/handler_test.go server/internal/module/role/handler_test.go server/internal/module/user/handler_test.go server/internal/module/auth/session_admin_handler_test.go server/internal/module/operationlog/handler_test.go server/internal/module/operationlog/rules.go server/internal/module/operationlog/middleware_test.go server/internal/module/operationlog/integration_test.go server/internal/module/operationlog/task_test.go server/internal/module/taskdemo/handler_test.go
git commit -m "feat!: 区分共享与管理接口命名空间"
```

If committed, the body must state that old `/api/v1` management endpoints are removed and Admin clients must use `/api/admin/v1`; shared Auth and Access paths are unchanged.

### Task 3: 切换 Admin Web 管理 API

**Files:**
- Modify: `web/tests/api/auth-platform.test.ts`
- Modify: `web/tests/api/menu.test.ts`
- Modify: `web/tests/api/role.test.ts`
- Modify: `web/tests/api/user.test.ts`
- Modify: `web/tests/api/taskDemo.test.ts`
- Create: `web/tests/api/session.test.ts`
- Create: `web/tests/api/operation-log.test.ts`
- Verify: `web/tests/api/auth.test.ts`
- Verify: `web/tests/api/access.test.ts`
- Verify: `web/tests/utils/request.test.ts`
- Modify: `web/src/api/auth-platform.ts`
- Modify: `web/src/api/menu.ts`
- Modify: `web/src/api/role.ts`
- Modify: `web/src/api/user.ts`
- Modify: `web/src/api/session.ts`
- Modify: `web/src/api/operation-log.ts`
- Modify: `web/src/api/taskDemo.ts`
- Modify: `web/tests/views/system/operation-logs/index.test.ts`

**Interfaces:**
- Produces: all Admin management requests under `/api/admin/v1`
- Preserves: Auth, Access and Refresh requests under `/api/v1`
- Preserves: request headers `X-Auth-Platform: admin` and `X-Device-ID`

- [ ] **Step 1: Change existing management API tests to the final paths**

Update exact request expectations. Representative assertions:

```ts
expect(requestMock).toHaveBeenCalledWith({ method: 'GET', url: '/api/admin/v1/menus' })
expect(requestMock).toHaveBeenCalledWith({ method: 'PUT', url: '/api/admin/v1/users/7', data: { username: 'alice_new' } })
expect(requestMock).toHaveBeenCalledWith({ method: 'GET', url: '/api/admin/v1/roles/7/permissions' })
expect(requestMock).toHaveBeenCalledWith({ method: 'POST', url: '/api/admin/v1/example-tasks', data: { message: 'foundation-check' } })
```

Update every method in each existing test file, not only the first request.

- [ ] **Step 2: Add exact Session and Operation Log API tests**

Create `session.test.ts` and assert the four current functions use:

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getSessions, getSessionStats, revokeSession, revokeSessions } from '@src/api/session'
import { request } from '@src/utils/request'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)

describe('session API', () => {
  beforeEach(() => requestMock.mockReset())

  it('uses only the Admin session namespace', async () => {
    requestMock.mockResolvedValue({})
    const query = { page: 1, pageSize: 20 }
    await getSessions(query)
    await getSessionStats()
    await revokeSession(7)
    await revokeSessions([7, 8])

    expect(requestMock).toHaveBeenNthCalledWith(1, {
      method: 'GET', url: '/api/admin/v1/sessions', params: query,
    })
    expect(requestMock).toHaveBeenNthCalledWith(2, {
      method: 'GET', url: '/api/admin/v1/sessions/stats',
    })
    expect(requestMock).toHaveBeenNthCalledWith(3, {
      method: 'DELETE', url: '/api/admin/v1/sessions/7',
    })
    expect(requestMock).toHaveBeenNthCalledWith(4, {
      method: 'DELETE', url: '/api/admin/v1/sessions', data: { ids: [7, 8] },
    })
  })
})
```

Create `operation-log.test.ts`:

```ts
import { describe, expect, it, vi } from 'vitest'
import { getOperationLogs } from '@src/api/operation-log'
import { request } from '@src/utils/request'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))

it('uses the Admin operation log namespace', async () => {
  vi.mocked(request).mockResolvedValue({ list: [], total: 0, page: 1, pageSize: 20 })
  const query = { page: 1, pageSize: 20 }
  await getOperationLogs(query)
  expect(request).toHaveBeenCalledWith({
    method: 'GET',
    url: '/api/admin/v1/operation-logs',
    params: query,
  })
})
```

Use the exact exported query type required by the current function if inference does not preserve strict TypeScript checks. Do not use `any` or type assertions.

- [ ] **Step 3: Run the management API tests and verify they fail on old URLs**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/auth-platform.test.ts tests/api/menu.test.ts tests/api/role.test.ts tests/api/user.test.ts tests/api/taskDemo.test.ts tests/api/session.test.ts tests/api/operation-log.test.ts --pool=threads --maxWorkers=1
```

Expected: FAIL because all seven production API modules still request `/api/v1` management paths.

- [ ] **Step 4: Change only the management API URL literals**

In the seven production API files, replace the management prefix with `/api/admin/v1`. Preserve methods, DTOs, params, payloads and return values exactly. Do not edit `auth.ts`, `access.ts` or `utils/request.ts`.

Update the current operation-log page fixture route to `/api/admin/v1/users/:id`; this fixture represents a new current event, not the backend legacy-history test.

- [ ] **Step 5: Run management and shared-boundary frontend tests**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/auth-platform.test.ts tests/api/menu.test.ts tests/api/role.test.ts tests/api/user.test.ts tests/api/taskDemo.test.ts tests/api/session.test.ts tests/api/operation-log.test.ts tests/api/auth.test.ts tests/api/access.test.ts tests/utils/request.test.ts tests/views/system/operation-logs/index.test.ts --pool=threads --maxWorkers=1
```

Expected: PASS. Auth and Access tests must still assert `/api/v1`; request tests must still assert Refresh at `/api/v1/auth/refresh` and `X-Auth-Platform: admin`.

- [ ] **Step 6: Commit only if explicitly authorized**

```powershell
git add web/src/api/auth-platform.ts web/src/api/menu.ts web/src/api/role.ts web/src/api/user.ts web/src/api/session.ts web/src/api/operation-log.ts web/src/api/taskDemo.ts web/tests/api/auth-platform.test.ts web/tests/api/menu.test.ts web/tests/api/role.test.ts web/tests/api/user.test.ts web/tests/api/taskDemo.test.ts web/tests/api/session.test.ts web/tests/api/operation-log.test.ts web/tests/views/system/operation-logs/index.test.ts
git commit -m "feat!: 切换管理端 API 命名空间"
```

If committed, the body must state that Admin Web now requires backend `/api/admin/v1`; shared authentication routes are unchanged.

### Task 4: 更新当前开发文档并完成全量验证

**Files:**
- Modify: `docs/learning/admin-architecture-and-crud.md`
- Modify: `docs/superpowers/specs/2026-08-27-admin-api-namespace-baseline-design.md`
- Verify: all files changed by Tasks 1-3
- Do not modify: historical completed specs/plans or legacy operation-log fixtures

**Interfaces:**
- Consumes: final backend and frontend paths from Tasks 1-3
- Produces: current development guidance that no longer teaches old Admin paths
- Produces: verified prerequisite for `2026-08-27-admin-authentication-baseline.md`

- [ ] **Step 1: Update the active learning document**

In `docs/learning/admin-architecture-and-crud.md`:

- change Admin CRUD examples such as menus and articles from `/api/v1/...` to `/api/admin/v1/...`;
- describe both Router Groups and the Admin platform Middleware order;
- keep `/api/v1/access` as the shared current-platform permission snapshot;
- change frontend Admin API examples to `/api/admin/v1/...`;
- do not rewrite unrelated architecture or historical migration explanations.

- [ ] **Step 2: Mark the namespace spec implemented only after verification**

Do not change the spec status yet. First complete Steps 3-6. If all pass, change:

```text
状态：设计已确认，实施计划已编写，等待执行
```

to:

```text
状态：已实施并完成验证
```

- [ ] **Step 3: Format and verify the complete backend**

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

Expected: all commands exit 0. Review `git diff --stat` after `go fmt` and revert no user changes; only fix task-scoped formatting if needed.

- [ ] **Step 4: Verify the complete frontend**

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

Expected: all Vitest files pass and the production build exits 0.

- [ ] **Step 5: Audit the namespace invariants**

Run current-code searches:

```powershell
cd D:\admin
rg -n '/api/v1/(auth-platforms|menus|roles|users|sessions|operation-logs|example-tasks)' server web\src web\tests docs\learning\admin-architecture-and-crud.md
rg -n '/api/admin/v1/(auth|access)' server web\src web\tests
rg -n '/api/v1/auth|/api/v1/access' server\cmd\api web\src\api web\src\utils\request.ts
git diff --check
git status --short
```

Expected:

- the first search returns only explicit old-path 404 assertions and deliberate historical operation-log fixtures; no Router registration, active audit rule, frontend API or current learning example uses an old management path;
- the second search has no output because shared Auth and Access are not under the Admin prefix;
- the third search confirms shared Auth, Access and Refresh paths remain discoverable;
- AnyScript rules remain unchanged because no types were weakened;
- `git diff --check` exits 0;
- status contains only task files plus pre-existing user changes.

- [ ] **Step 6: Inspect immutable audit-history handling**

Confirm from the diff:

```text
operationlog/rules.go                  current management rules use /api/admin/v1
operationlog/middleware_test.go        current request events use /api/admin/v1
operationlog/schema_test.go            explicit legacy rows still use /api/v1
database/domain_names_test.go          historical migrated row still uses /api/v1
```

If a mechanical replacement changed either legacy file, restore only those task-induced edits without touching other user changes.

- [ ] **Step 7: Commit documentation only if explicitly authorized**

```powershell
git add docs/learning/admin-architecture-and-crud.md docs/superpowers/specs/2026-08-27-admin-api-namespace-baseline-design.md
git commit -m "docs: 更新管理接口命名空间说明"
```

Do not create an empty verification commit. Do not amend or squash without separate authorization.

---

## Execution Notes

- Task 1 must finish first because Task 2 uses the concrete Admin platform Middleware.
- Task 2 and Task 3 form one breaking release unit; do not deploy a new backend with an old Admin Web or the reverse.
- Task 4 is the release gate. Only after every verification passes may the namespace spec be marked implemented.
- After this plan is complete, execute `docs/superpowers/plans/2026-08-27-admin-authentication-baseline.md`; do not start RBAC, COS, mail or SMS first.
- Do not use subagents unless the user explicitly authorizes delegation; inline execution remains available.
