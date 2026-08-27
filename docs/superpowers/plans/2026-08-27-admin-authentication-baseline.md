# Admin 邮箱登录与账号资料基线 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除 Admin 公开注册入口，将密码登录标识改为全局邮箱，固定内置 Admin 平台禁止注册，并为全局用户补齐可空手机号资料链路。

**Architecture:** 后端继续使用 `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`，认证会话与 Redis 流程保持不变。前端继续使用 `view -> api/<module>.ts -> utils/request.ts -> Go API`，Admin 只保留邮箱密码登录；后端注册接口继续由认证平台 Policy 控制，供未来独立业务客户端复用。

**Tech Stack:** Go、Gin、GORM、PostgreSQL、Redis、Vue 3、TypeScript、Pinia、Vue Router、Element Plus、Vitest、pnpm

**Spec:** `docs/superpowers/specs/2026-08-27-admin-authentication-baseline-design.md`

**Prerequisite:** `docs/superpowers/plans/2026-08-27-admin-api-namespace-baseline.md` 必须已经完整实施并通过验证；本计划不再迁移 API 前缀。

## Global Constraints

- 一个规范化邮箱对应一个全局有效 `user_account`，不得按认证平台复制用户。
- Admin 前端不得保留 `/register`、注册链接、注册页面或注册 API 调用。
- 后端 `POST /api/v1/auth/register` 保留，并继续由当前 `X-Auth-Platform` 的 `allow_register` 控制。
- 共享 Auth 和 Access 使用 `/api/v1`；用户、角色、菜单、认证平台、会话和操作日志等 Admin 管理接口使用 `/api/admin/v1`。
- 内置 `admin` 固定 `allow_register=0`；非 Admin 平台仍可配置注册开关。
- 登录请求精确为 `{ email, password }`，旧 `username` 字段必须被严格 JSON 绑定拒绝。
- 邮箱执行 Trim、转小写、普通邮箱格式校验和最多 254 UTF-8 字节限制。
- `user_account.phone` 精确为 `VARCHAR(32) NULL`，不增加索引、唯一性、验证状态或地区字段。
- API 中 `phone` 必须出现，值为 `string | null`；写入 `null` 表示清除，空字符串不进入 Service。
- 用户编辑请求必须同时出现 `username` 和 `phone`；更新在一个 PostgreSQL 事务内完成。
- 不修改 JWT、Refresh Cookie、会话上限、设备/IP 绑定、默认角色、多角色和 Bootstrap 安全语义。
- 不实现邮件、短信、邀请、激活、找回密码、个人中心、COS 或 RBAC 平台菜单隔离。
- 不接入业务前端；RBAC 平台隔离完成前不得声称共享后端已经具备跨平台权限隔离。
- TypeScript 不使用显式 `any`、类型断言兜底、可选链补必填字段或静默默认值。
- Redis/PostgreSQL 错误显式返回；不得使用内存实现、旧缓存、空数组或假成功降级。
- 执行开始前运行 `git status --short`，保留所有非本任务改动。
- 下列 Commit 步骤只在维护者再次明确授权提交时执行；未授权时保留工作区改动并跳过提交。

---

## File Map

### Backend account data

- `server/internal/module/user/model.go`：在全局用户模型中表达可空手机号。
- `server/internal/module/user/protocol.go`：保存手机号规范化和控制字符校验。
- `server/internal/module/user/request.go`：区分缺失 `phone` 与显式 `phone: null`。
- `server/internal/module/user/response.go`：闭合用户列表、角色摘要和编辑响应。
- `server/internal/module/user/service.go`：校验资料并编排原子更新。
- `server/internal/module/user/repository.go`：手机号查询、列表、当前用户读取和原子资料写入。
- `server/internal/module/auth/response.go`、`handler.go`：在 `/auth/me` 返回必填 `phone`。
- `server/internal/module/auth/schema_test.go`：验证真实 PostgreSQL 列和索引契约。

### Backend authentication platform

- `server/internal/module/authplatform/schema.go`：迁移内置 Admin 注册策略并放宽对可编辑策略的种子值比较。
- `server/internal/module/authplatform/service.go`：拒绝把内置 Admin 注册开关改为 Yes。
- `server/internal/module/authplatform/redis.go`：提供只清理内置 Admin Policy 派生快照的具体函数。
- `server/cmd/api/main.go`：Redis 建立后、HTTP 监听前执行 Policy 清理。

### Backend email login

- `server/internal/module/auth/request.go`、`handler.go`、`service.go`：把密码登录输入改为邮箱。
- `server/internal/module/user/repository.go`：以规范化邮箱精确读取凭据。
- `server/internal/module/auth/bootstrap_service.go`：保持创建输入不变，并复用邮箱规范化结果。

### Frontend

- `web/src/api/auth.ts`：邮箱登录 DTO、含手机号的 CurrentUser，删除 Admin 注册和 Policy 调用。
- `web/src/api/user.ts`：在用户 DTO 和编辑请求中加入 `phone`。
- `web/src/store/auth.ts`：同步当前管理员的用户名和手机号。
- `web/src/router/index.ts`、`permission.ts`、`api/menu.ts`、`utils/request.ts`：移除 Admin 注册运行时分支。
- `web/src/views/auth/login/index.vue`：邮箱密码表单。
- `web/src/views/auth/register/index.vue`：删除。
- `web/src/views/account/users/index.vue`：展示、编辑手机号。
- `web/src/views/access/auth-platforms/index.vue`：锁定内置 Admin 的注册开关。
- `web/src/i18n/messages/zh-CN.ts`、`en-US.ts`：认证、手机号和内置限制文案。

---

### Task 1: 建立手机号数据库与 Repository 契约

**Files:**
- Modify: `server/internal/module/user/model.go`
- Modify: `server/internal/module/user/protocol.go`
- Modify: `server/internal/module/user/protocol_test.go`
- Modify: `server/internal/module/user/repository.go`
- Modify: `server/internal/module/user/repository_test.go`
- Modify: `server/internal/module/user/service.go`
- Modify: `server/internal/module/auth/schema_test.go`

**Interfaces:**
- Produces: `User.Phone *string`
- Produces: `NormalizePhone(value *string) (*string, error)`
- Produces: `Repository.UpdateProfile(ctx context.Context, userID int64, username string, phone *string, updatedAt time.Time) error`
- Produces: `Current.Phone *string`, `ListItem.Phone *string`, `Summary.Phone *string`
- Consumes: existing `Repository.Transaction`, `LockUserWriteTable`, `LockUser` and PostgreSQL schema setup

- [ ] **Step 1: Write failing PostgreSQL schema and repository tests**

Add schema assertions that `phone` exists as nullable `character varying(32)` and no index mentions `phone`. Replace username credential tests later in Task 4; this task only extends current-user/list/profile storage.

```go
func TestAuthenticationSchemaCreatesNullablePhoneWithoutIndex(t *testing.T) {
	connection, ctx := openAuthenticationSchema(t)
	var column struct {
		DataType         string
		CharacterMaximumLength int
		IsNullable       string
	}
	if err := connection.GORM.WithContext(ctx).Raw(`
		SELECT data_type, character_maximum_length, is_nullable
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'user_account'
		  AND column_name = 'phone'`).Scan(&column).Error; err != nil {
		t.Fatal(err)
	}
	if column.DataType != "character varying" || column.CharacterMaximumLength != 32 || column.IsNullable != "YES" {
		t.Fatalf("phone column = %+v", column)
	}
}
```

Add repository tests for `NULL`, a stored phone, list keyword matching, `FindCurrent`, role summary source data, and one-statement profile update.

```go
func TestRepositoryProfilePhoneRoundTripAndKeywordSearch(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil { t.Fatal(err) }
	repository := user.NewRepository(tx)
	phone := "+86 138-0000-0000"
	input := newCreateInput("phone", defaultRole.ID)
	created, err := repository.CreateWithRole(ctx, input)
	if err != nil { t.Fatal(err) }
	if err := repository.UpdateProfile(ctx, created.ID, created.Username, &phone, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	current, err := repository.FindCurrent(ctx, created.ID)
	if err != nil || current.Phone == nil || *current.Phone != phone {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	rows, err := repository.List(ctx, user.ListQuery{Page: 1, PageSize: 20, Keyword: "138-0000"})
	if err != nil || len(rows) != 1 || rows[0].Phone == nil {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
}
```

- [ ] **Step 2: Run the new tests and confirm the target behavior is missing**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/auth ./internal/module/user -run 'Test(AuthenticationSchemaCreatesNullablePhoneWithoutIndex|RepositoryProfilePhoneRoundTripAndKeywordSearch)' -count=1
```

Expected: FAIL because `User.Phone` and `UpdateProfile` do not exist, or because the column/query contract is absent. Compilation failures are acceptable only when they point to those intentionally missing symbols.

- [ ] **Step 3: Add the nullable model field and phone normalizer**

Add the model field without `not null`, index or default:

```go
Phone *string `gorm:"column:phone;type:varchar(32)"`
```

Add a concrete domain helper in `user/protocol.go`:

```go
func NormalizePhone(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" || utf8.RuneCountInString(normalized) > 32 {
		return nil, fmt.Errorf("phone must contain 1 to 32 Unicode characters")
	}
	for _, character := range normalized {
		if unicode.IsControl(character) {
			return nil, fmt.Errorf("phone contains a control character")
		}
	}
	return &normalized, nil
}
```

Table-driven tests must cover `nil`, trimming, `+86 138-0000-0000`, empty input, 33 runes and control characters.

- [ ] **Step 4: Extend repository reads and replace the narrow username writer**

Add `Phone *string` to `Current`, `ListItem`, `Summary` and internal scan rows. Include `phone` in `FindCurrent`, list SELECTs and keyword search:

```go
db = db.Where(`(
	app_user.username ILIKE ? ESCAPE E'\\'
	OR app_user.email ILIKE ? ESCAPE E'\\'
	OR app_user.phone ILIKE ? ESCAPE E'\\'
)`, pattern, pattern, pattern)
```

Replace `UpdateUsername` with one atomic writer:

```go
func (r *Repository) UpdateProfile(ctx context.Context, userID int64, username string, phone *string, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
		"username": username,
		"phone": phone,
		"updated_at": updatedAt.UTC(),
	})
	// Preserve existing username constraint mapping and record-not-found behavior.
}
```

Do not add phone conflict mapping because no phone uniqueness exists.

- [ ] **Step 5: Run model, protocol and repository tests**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/auth ./internal/module/user -run 'Test(AuthenticationSchema|NormalizePhone|Repository)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit only if explicitly authorized**

```powershell
git add server/internal/module/user/model.go server/internal/module/user/protocol.go server/internal/module/user/protocol_test.go server/internal/module/user/repository.go server/internal/module/user/repository_test.go server/internal/module/user/service.go server/internal/module/auth/schema_test.go
git commit -m "feat: 增加用户手机号数据库契约"
```

### Task 2: 闭合手机号 Service、HTTP 与当前用户响应

**Files:**
- Modify: `server/internal/module/user/request.go`
- Modify: `server/internal/module/user/response.go`
- Modify: `server/internal/module/user/service.go`
- Modify: `server/internal/module/user/service_test.go`
- Modify: `server/internal/module/user/handler.go`
- Modify: `server/internal/module/user/handler_test.go`
- Modify: `server/internal/module/auth/response.go`
- Modify: `server/internal/module/auth/handler.go`
- Modify: `server/internal/module/auth/handler_test.go`
- Modify: `server/internal/module/auth/service_test.go`

**Interfaces:**
- Consumes: `NormalizePhone`, `Repository.UpdateProfile`, and phone-bearing repository DTOs from Task 1
- Produces: `UpdateInput{Username string; Phone *string}`
- Produces: `UpdatedProfile{ID int64; Username string; Phone *string; UpdatedAt time.Time}`
- Produces: required JSON `phone: string | null` in `/auth/me`, user list, role summary and profile update

- [ ] **Step 1: Write failing request and response contract tests**

Extend handler fixtures with phone and require the key in every response. Add update-body cases that distinguish missing from null:

```go
tests := []struct {
	body       string
	wantStatus int
}{
	{`{"username":"alice","phone":"+86 138-0000-0000"}`, http.StatusOK},
	{`{"username":"alice","phone":null}`, http.StatusOK},
	{`{"username":"alice"}`, http.StatusBadRequest},
	{`{"username":"alice","phone":""}`, http.StatusBadRequest},
	{`{"username":"alice","phone":"123\n456"}`, http.StatusBadRequest},
}
```

For `/auth/me`, assert exact data keys rather than merely decoding into a permissive struct:

```go
want := `{"userId":7,"username":"alice","email":"alice@example.com","phone":null}`
assertEnvelopeDataJSON(t, recorder, want)
```

- [ ] **Step 2: Run handler tests and verify they fail**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/user ./internal/module/auth -run 'Test(HandlerUser|MeReturnsClosedCurrentUserShape)' -count=1
```

Expected: FAIL because requests cannot require nullable `phone` and responses omit it.

- [ ] **Step 3: Implement a request-local nullable phone value**

Do not use `**string`: `encoding/json` cannot reliably distinguish a missing key from explicit `null` through that field alone. Add a local request type in `user/request.go`:

```go
type nullablePhoneRequest struct {
	Present bool
	Value   *string
}

func (value *nullablePhoneRequest) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Value = nil
		return nil
	}
	var phone string
	if err := json.Unmarshal(data, &phone); err != nil {
		return err
	}
	value.Value = &phone
	return nil
}

type updateRequest struct {
	Username *string              `json:"username"`
	Phone    nullablePhoneRequest `json:"phone"`
}
```

`input()` must reject `Username == nil` or `!Phone.Present`, then pass `Phone.Value` to Service. `Validate.BindJSON` continues to reject unknown fields and trailing JSON.

- [ ] **Step 4: Write failing Service tests for atomic and idempotent profile updates**

Rename existing username tests around `UpdatedProfile`. Add assertions for normalization, no-op timestamps and rollback:

```go
func TestServiceUpdateProfileNormalizesPhoneAndPreservesNoOpTimestamp(t *testing.T) {
	phone := "  +86 138-0000-0000  "
	updated, err := service.Update(ctx, actor.ID, target.ID, user.UpdateInput{
		Username: target.Username,
		Phone: &phone,
	})
	if err != nil || updated.Phone == nil || *updated.Phone != "+86 138-0000-0000" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	second, err := service.Update(ctx, actor.ID, target.ID, user.UpdateInput{Username: updated.Username, Phone: updated.Phone})
	if err != nil || !second.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Fatalf("idempotent update=%+v err=%v", second, err)
	}
}
```

Force a transaction failure after a test-scoped profile write and assert neither username nor phone changes.

- [ ] **Step 5: Implement Service and response DTO changes**

Normalize both fields before entering the transaction. Preserve current super-admin protection. Compare nullable strings before deciding whether to write:

```go
phone, err := NormalizePhone(input.Phone)
if err != nil {
	return UpdatedProfile{}, apperror.InvalidRequest(err)
}
if target.Username == username && equalOptionalString(target.Phone, phone) {
	return currentUpdatedProfile(target), nil
}
```

Update Handler operation name from `user.username.update` to `user.profile.update`. Return `phone` explicitly in all response structs. `CurrentUserResponse.Phone` must be `*string` without `omitempty`.

- [ ] **Step 6: Run user and auth package tests**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/user ./internal/module/auth -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit only if explicitly authorized**

```powershell
git add server/internal/module/user/request.go server/internal/module/user/response.go server/internal/module/user/service.go server/internal/module/user/service_test.go server/internal/module/user/handler.go server/internal/module/user/handler_test.go server/internal/module/auth/response.go server/internal/module/auth/handler.go server/internal/module/auth/handler_test.go server/internal/module/auth/service_test.go
git commit -m "feat: 闭合用户手机号资料接口"
```

### Task 3: 固定内置 Admin 平台禁止注册

**Files:**
- Modify: `server/internal/module/authplatform/schema.go`
- Modify: `server/internal/module/authplatform/schema_test.go`
- Modify: `server/internal/module/authplatform/service.go`
- Modify: `server/internal/module/authplatform/service_test.go`
- Modify: `server/internal/module/authplatform/redis.go`
- Modify: `server/internal/module/authplatform/redis_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`

**Interfaces:**
- Produces: new builtin Admin seed with `AllowRegister: yesno.No`
- Produces: idempotent schema migration from Yes to No with `policy_version + 1`
- Produces: `authplatform.ClearBuiltinAdminPolicy(ctx context.Context, redis *projectredis.Client) error`
- Consumes: existing `PolicyKey`, `projectredis.Client.Delete`, auth platform mutation transaction and `CodeInvalidPolicy`

- [ ] **Step 1: Write failing schema migration tests**

Replace the exact-seed comparison test with tests for stable identity plus editable runtime policies:

```go
func TestEnsureSchemaMigratesBuiltinAdminRegistrationOnce(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil { t.Fatal(err) }
	db := connection.GORM.WithContext(ctx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	historical := authplatform.Platform{
		Code: authplatform.BuiltinAdminCode, Name: "Admin", PolicyVersion: 7,
		AccessTTLSeconds: 1200, RefreshTTLSeconds: 1_209_600,
		SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800,
		BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 2,
		AllowRegister: yesno.Yes, IsEnabled: yesno.No, IsBuiltin: yesno.Yes,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&historical).Error; err != nil { t.Fatal(err) }
	if err := authplatform.EnsureSchema(ctx, db); err != nil { t.Fatal(err) }
	first := readAdmin(t, db)
	if first.AllowRegister != yesno.No || first.PolicyVersion != 8 {
		t.Fatalf("first migration = %+v", first)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil { t.Fatal(err) }
	second := readAdmin(t, db)
	if second.PolicyVersion != first.PolicyVersion || !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("migration was not idempotent: first=%+v second=%+v", first, second)
	}
}

func readAdmin(t *testing.T, db *gorm.DB) authplatform.Platform {
	t.Helper()
	var value authplatform.Platform
	if err := db.Unscoped().Where("code = ?", authplatform.BuiltinAdminCode).Take(&value).Error; err != nil {
		t.Fatal(err)
	}
	return value
}
```

Also cover:

- no historical row creates Admin with `allow_register=0` and version 1;
- modified valid runtime settings and `is_enabled=0` are preserved;
- duplicate rows, soft-deleted history, `is_builtin=0` or invalid runtime values fail;
- changing only the display name to another valid non-empty name does not get reset.

- [ ] **Step 2: Run schema tests and verify historical Yes fails the new contract**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/authplatform -run 'Test(AuthenticationPlatformSchema|EnsureSchema)' -count=1
```

Expected: FAIL because the seed and matcher still require `allow_register=1` and exact mutable defaults.

- [ ] **Step 3: Implement the explicit, idempotent schema migration**

Keep the existing table lock and unscoped history query. Replace `matchesBuiltinAdmin` with stable checks:

```go
func validateBuiltinAdmin(value Platform) error {
	if value.Code != BuiltinAdminCode || value.IsBuiltin != yesno.Yes || value.DeletedAt.Valid {
		return fmt.Errorf("builtin authentication platform identity is invalid")
	}
	return ValidatePlatform(value)
}
```

When the single valid row has `AllowRegister == yesno.Yes`, update only these columns in the current transaction:

```sql
UPDATE auth_platform
SET allow_register = 0,
    policy_version = policy_version + 1,
    updated_at = ?
WHERE id = ? AND deleted_at IS NULL
```

No-op when already No. Do not reset name, TTL, binding, session limit or enabled state.

- [ ] **Step 4: Write failing Service and Redis startup tests**

Add a Service test that edits the built-in Admin with `AllowRegister: yesno.Yes` and expects `CodeInvalidPolicy` before Redis/PostgreSQL mutation. Verify a non-builtin platform can still switch to Yes.

Add a Redis test:

```go
func TestClearBuiltinAdminPolicyDeletesOnlyAdminKey(t *testing.T) {
	ctx := context.Background()
	redisClient := openPlatformRedis(t)
	adminKey := authplatform.PolicyKey("admin")
	appKey := authplatform.PolicyKey("app")
	if err := redisClient.SetString(ctx, adminKey, `{"state":"old-admin"}`, 0); err != nil { t.Fatal(err) }
	if err := redisClient.SetString(ctx, appKey, `{"state":"app"}`, 0); err != nil { t.Fatal(err) }
	if err := authplatform.ClearBuiltinAdminPolicy(ctx, redisClient); err != nil { t.Fatal(err) }
	if _, found, err := redisClient.GetString(ctx, adminKey); err != nil || found {
		t.Fatalf("admin key found=%v err=%v", found, err)
	}
	if _, found, err := redisClient.GetString(ctx, appKey); err != nil || !found {
		t.Fatalf("app key found=%v err=%v", found, err)
	}
}
```

`server/cmd/api/main_test.go` must assert the cleanup call appears after `projectredis.Open` and before services/routes are constructed.

- [ ] **Step 5: Implement Service guard and concrete Policy cleanup**

Guard the candidate before no-op detection:

```go
if current.Code == BuiltinAdminCode && current.IsBuiltin && input.AllowRegister != yesno.No {
	return mutationPlan{}, invalidPolicy(fmt.Errorf("builtin admin registration must remain disabled"))
}
```

Add the concrete cleanup function:

```go
func ClearBuiltinAdminPolicy(ctx context.Context, redis *projectredis.Client) error {
	if redis == nil {
		return fmt.Errorf("clear builtin admin policy requires Redis")
	}
	if err := redis.Delete(ctx, PolicyKey(BuiltinAdminCode)); err != nil {
		return fmt.Errorf("clear builtin admin policy: %w", err)
	}
	return nil
}
```

Call it in `main.go` immediately after Redis opens and before HTTP dependencies are assembled. Return a startup error on failure.

- [ ] **Step 6: Run auth platform and API bootstrap tests**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/authplatform ./cmd/api -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit only if explicitly authorized**

```powershell
git add server/internal/module/authplatform/schema.go server/internal/module/authplatform/schema_test.go server/internal/module/authplatform/service.go server/internal/module/authplatform/service_test.go server/internal/module/authplatform/redis.go server/internal/module/authplatform/redis_test.go server/cmd/api/main.go server/cmd/api/main_test.go
git commit -m "fix: 固定管理平台禁止公开注册"
```

### Task 4: 将后端密码登录标识切换为邮箱

**Files:**
- Modify: `server/internal/module/auth/request.go`
- Modify: `server/internal/module/auth/handler.go`
- Modify: `server/internal/module/auth/handler_test.go`
- Modify: `server/internal/module/auth/service.go`
- Modify: `server/internal/module/auth/service_test.go`
- Modify: `server/internal/module/user/repository.go`
- Modify: `server/internal/module/user/repository_test.go`
- Modify: `server/internal/module/auth/bootstrap_service_test.go`

**Interfaces:**
- Produces: `LoginRequest{Email string; Password string}`
- Produces: `LoginInput{Email string; Password string; Client authclient.Client}`
- Produces: `userStore.FindCredentialByEmail(context.Context, string) (user.Credential, error)`
- Produces: `Repository.FindCredentialByEmail(ctx context.Context, email string) (Credential, error)`
- Consumes: existing password comparison, platform Policy, session creation, Redis publication and Token issuance

- [ ] **Step 1: Write failing Handler tests for the exact login body**

Change the success body to email and add explicit old-contract rejection:

```go
func TestLoginAcceptsOnlyEmailAndPassword(t *testing.T) {
	success := serveAuthRequest(t, service, http.MethodPost, "/api/v1/auth/login", `{"email":" Admin@Example.COM ","password":"password"}`)
	assertAuthEnvelope(t, success, http.StatusOK, 0)
	if service.loginInput.Email != " Admin@Example.COM " {
		t.Fatalf("handler changed email before service: %+v", service.loginInput)
	}

	legacy := serveAuthRequest(t, service, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"password"}`)
	assertAuthEnvelope(t, legacy, http.StatusBadRequest, apperror.CodeInvalidRequest)
}
```

- [ ] **Step 2: Write failing Service and Repository email tests**

The fake user store must capture the email passed to `FindCredentialByEmail`. Cover trim/lowercase, invalid format, 255-byte input, missing email, record not found, wrong bcrypt password, disabled user and repository errors.

```go
func TestLoginNormalizesEmailAndKeepsCredentialErrorsUniform(t *testing.T) {
	ctx := context.Background()
	redisClient := openAuthRedis(t)
	passwordHash, err := HashPassword("password")
	if err != nil { t.Fatal(err) }
	users := &fakeUserStore{credential: user.Credential{
		ID: 1, Username: "admin", Email: "admin@example.com",
		PasswordHash: passwordHash, IsEnabled: yesno.Yes,
	}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
	_, err = service.Login(ctx, LoginInput{Email: " Admin@Example.COM ", Password: "wrong", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeUnauthorized { t.Fatalf("wrong password error=%v", err) }
	if users.credentialEmail != "admin@example.com" {
		t.Fatalf("credential email = %q", users.credentialEmail)
	}

	users.credentialErr = gorm.ErrRecordNotFound
	_, missingErr := service.Login(ctx, LoginInput{Email: "missing@example.com", Password: "wrong", Client: testAuthClient()})
	var wrongPublic, missingPublic *apperror.Error
	if !errors.As(err, &wrongPublic) || !errors.As(missingErr, &missingPublic) ||
		wrongPublic.HTTPStatus != missingPublic.HTTPStatus || wrongPublic.Code != missingPublic.Code ||
		wrongPublic.MessageKey != missingPublic.MessageKey {
		t.Fatalf("public errors differ: wrong=%v missing=%v", err, missingErr)
	}
}
```

Repository tests must prove lookup uses exact normalized email, returns disabled state, excludes soft-deleted users and does not accept username.

- [ ] **Step 3: Run the new auth and repository tests and verify they fail**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/auth ./internal/module/user -run 'Test(Login|FindCredential|BootstrapAdmin)' -count=1
```

Expected: FAIL because login DTOs and Repository still use username.

- [ ] **Step 4: Implement one shared email normalization path in Auth Service**

Extract the current registration email logic into a private helper used by registration, Bootstrap validation and login:

```go
func normalizeEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Name != "" || parsed.Address != email || len(email) > 254 {
		return "", fmt.Errorf("email address is invalid")
	}
	return email, nil
}
```

Do not trim or normalize passwords. Login must call `FindCredentialByEmail`; there is no username fallback and no automatic registration.

- [ ] **Step 5: Implement exact Repository email lookup**

Rename the method and query:

```go
func (r *Repository) FindCredentialByEmail(ctx context.Context, email string) (Credential, error) {
	var credential Credential
	if err := r.db.WithContext(ctx).
		Model(&User{}).
		Select("id", "username", "email", "password_hash", "is_enabled").
		Where("email = ?", email).
		Take(&credential).Error; err != nil {
		return Credential{}, fmt.Errorf("find user credential: %w", err)
	}
	return credential, nil
}
```

GORM default scope continues to exclude soft-deleted users. Preserve all existing session, Policy, Redis and Token code after credential verification.

- [ ] **Step 6: Run complete auth and user tests**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/auth ./internal/module/user -count=1
```

Expected: PASS, including registration and Bootstrap tests.

- [ ] **Step 7: Commit only if explicitly authorized**

```powershell
git add server/internal/module/auth/request.go server/internal/module/auth/handler.go server/internal/module/auth/handler_test.go server/internal/module/auth/service.go server/internal/module/auth/service_test.go server/internal/module/user/repository.go server/internal/module/user/repository_test.go server/internal/module/auth/bootstrap_service_test.go
git commit -m "feat!: 将密码登录标识切换为邮箱"
```

If committed, include a body explaining that Admin and API clients must replace `{ username, password }` with `{ email, password }`; Refresh Tokens and existing sessions remain valid.

### Task 5: 在 Admin 用户管理中展示和编辑手机号

**Files:**
- Modify: `web/src/api/auth.ts`
- Modify: `web/src/api/user.ts`
- Modify: `web/src/store/auth.ts`
- Modify: `web/src/views/account/users/index.vue`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Modify: `web/tests/api/user.test.ts`
- Modify: `web/tests/store/auth.test.ts`
- Modify: `web/tests/views/account/users/index.test.ts`
- Modify: `web/tests/layout/index.test.ts`
- Modify: `web/tests/router/index.test.ts`
- Modify: `web/tests/views/auth/login/index.test.ts`

**Interfaces:**
- Produces: `CurrentUser.phone: string | null`
- Produces: `UserListItem.phone: string | null`
- Produces: `UserRolesResponse.user.phone: string | null`
- Produces: `UpdateUserInput{username: string; phone: string | null}`
- Produces: `UpdatedProfile{id: number; username: string; phone: string | null; updatedAt: string}`
- Produces: `auth.updateProfile(userId: number, username: string, phone: string | null): boolean`
- Consumes: exact backend DTOs from Tasks 1 and 2

- [ ] **Step 1: Write failing API and Store tests**

Update fixtures so every user DTO contains `phone`. Assert exact update request:

```ts
await updateUser(7, { username: 'alice', phone: '+86 138-0000-0000' })
expect(requestMock).toHaveBeenCalledWith({
  method: 'PUT',
  url: '/api/admin/v1/users/7',
  data: { username: 'alice', phone: '+86 138-0000-0000' },
})
```

Store test:

```ts
store.setAuthenticated({ userId: 7, username: 'alice', email: 'alice@example.com', phone: null })
expect(store.updateProfile(7, 'alice_new', '+86 138-0000-0000')).toBe(true)
expect(store.user).toEqual({
  userId: 7,
  username: 'alice_new',
  email: 'alice@example.com',
  phone: '+86 138-0000-0000',
})
```

- [ ] **Step 2: Run API and Store tests and verify missing fields fail**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/user.test.ts tests/store/auth.test.ts --pool=threads --maxWorkers=1
```

Expected: FAIL because DTOs and Store do not contain phone/profile update.

- [ ] **Step 3: Update TypeScript DTOs and Auth Store**

Use explicit nullable fields, never optional fields:

```ts
export interface CurrentUser {
  userId: number
  username: string
  email: string
  phone: string | null
}

export interface UpdateUserInput {
  username: string
  phone: string | null
}
```

Replace `UpdatedUsername` and `updateUsername` with `UpdatedProfile` and `updateProfile`. Preserve `email` in Store and update both username and phone atomically in one assigned object.

Update every `setAuthenticated` and `/me` fixture in the listed tests to include explicit `phone: null`.

- [ ] **Step 4: Write failing user page tests for display, validation and clearing**

Extend the existing edit test:

```ts
expect(wrapper.text()).toContain('+86 138-0000-0000')
await findAriaButton(wrapper, '编辑').trigger('click')
const phoneInput = document.body.querySelector<HTMLInputElement>('[data-testid="user-phone"]')
if (phoneInput === null) throw new Error('phone input missing')
phoneInput.value = '  +86 139-0000-0000  '
phoneInput.dispatchEvent(new Event('input'))
await bodyButton('保存').trigger('click')
await flushPromises()
expect(updateUser).toHaveBeenCalledWith(7, {
  username: 'new_name',
  phone: '+86 139-0000-0000',
})
```

Add a second case that clears the input and expects `phone: null`, plus rejected control-character and 33-rune cases where Save is disabled and API is not called.

- [ ] **Step 5: Implement the user management UI**

Extend form state and normalization:

```ts
interface UserFormState {
  username: string
  phone: string
}

const normalizedPhone = computed(() => userForm.value.phone.trim())
const phoneValid = computed(() => {
  const value = normalizedPhone.value
  return value === '' || ([...value].length <= 32 && !/\p{Cc}/u.test(value))
})
const submittedPhone = computed<string | null>(() => normalizedPhone.value === '' ? null : normalizedPhone.value)
```

Add a phone table column next to email and render `row.phone ?? '-'`. Initialize the edit input with `row.phone ?? ''`, set `data-testid="user-phone"`, submit both fields, and call `auth.updateProfile(result.id, result.username, result.phone)`.

Do not add country selectors, verification labels or SMS controls.

- [ ] **Step 6: Run user page and dependent frontend tests**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/user.test.ts tests/store/auth.test.ts tests/views/account/users/index.test.ts tests/layout/index.test.ts tests/router/index.test.ts tests/views/auth/login/index.test.ts --pool=threads --maxWorkers=1
```

Expected: PASS.

- [ ] **Step 7: Commit only if explicitly authorized**

```powershell
git add web/src/api/auth.ts web/src/api/user.ts web/src/store/auth.ts web/src/views/account/users/index.vue web/src/i18n/messages/zh-CN.ts web/src/i18n/messages/en-US.ts web/tests/api/user.test.ts web/tests/store/auth.test.ts web/tests/views/account/users/index.test.ts web/tests/layout/index.test.ts web/tests/router/index.test.ts web/tests/views/auth/login/index.test.ts
git commit -m "feat: 增加用户手机号资料管理"
```

### Task 6: 删除 Admin 注册并切换邮箱登录界面

**Files:**
- Modify: `web/src/api/auth.ts`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/permission.ts`
- Modify: `web/src/api/menu.ts`
- Modify: `web/src/utils/request.ts`
- Modify: `web/src/views/auth/login/index.vue`
- Delete: `web/src/views/auth/register/index.vue`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Modify: `web/tests/api/auth.test.ts`
- Modify: `web/tests/api/menu-fields.test.ts`
- Modify: `web/tests/router/index.test.ts`
- Modify: `web/tests/utils/request.test.ts`
- Modify: `web/tests/views/auth/login/index.test.ts`
- Delete: `web/tests/views/auth/register/index.test.ts`

**Interfaces:**
- Produces: `LoginInput{email: string; password: string}`
- Removes: `RegisterInput`, `RegisteredUser`, `register()`, `AuthPolicy`, `getAuthPolicy()` from Admin Web
- Removes: route name `register`, path `/register`, registration Policy guard and Admin no-Bearer registration path
- Consumes: backend `{ email, password }` contract from Task 4 and `CurrentUser.phone` from Task 5

- [ ] **Step 1: Write failing API and login view tests**

Replace username assertions with exact email behavior and remove Policy mocks:

```ts
await wrapper.get('[data-testid="login-email"]').setValue(' Admin@Example.COM ')
await wrapper.get('[data-testid="login-password"]').setValue('password')
await wrapper.get('form').trigger('submit')
await flushPromises()
expect(loginMock).toHaveBeenCalledWith({
  email: 'Admin@Example.COM',
  password: 'password',
})
expect(wrapper.find('a[href="/register"]').exists()).toBe(false)
```

The API test must assert `login()` sends only `email` and `password`. Remove tests for Admin `register()` and `getAuthPolicy()`.

- [ ] **Step 2: Write failing Router, menu and request tests for complete registration removal**

Assert:

```ts
expect(router.hasRoute('register')).toBe(false)
expect(router.resolve('/register').matched).toHaveLength(0)
expect(isMenuPath('/register')).toBe(true)
```

`/register` is no longer a reserved static Admin route, so the menu field protocol treats it like any other
syntactically valid path. This does not recreate a frontend route or page.

Remove registration Policy guard cases and `getAuthPolicy` mocks. In request tests, remove `/api/v1/auth/register` from Admin no-Bearer expectations; keep backend registration coverage exclusively in Go tests.

- [ ] **Step 3: Run targeted frontend tests and verify old runtime paths fail**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/auth.test.ts tests/api/menu-fields.test.ts tests/router/index.test.ts tests/utils/request.test.ts tests/views/auth/login/index.test.ts --pool=threads --maxWorkers=1
```

Expected: FAIL because the Admin runtime still exposes registration and username login.

- [ ] **Step 4: Remove Admin registration runtime code**

Delete the route, page, page test, API function/types, Policy guard, Policy fetch in Login, static menu path and Admin request exception. Do not delete the Go registration route.

Remove only registration-specific i18n keys. Keep `authPlatform.allowRegister` because the platform management page still manages non-Admin platforms. Use a final usage search before deleting any ambiguous key.

- [ ] **Step 5: Implement the exact email login form**

Use:

```ts
interface LoginForm {
  email: string
  password: string
}

const form = reactive<LoginForm>({ email: '', password: '' })
const rules: FormRules<LoginForm> = {
  email: [
    { required: true, message: t('auth.login.emailRequired'), trigger: 'blur' },
    { type: 'email', message: t('auth.login.emailInvalid'), trigger: 'blur' },
  ],
  password: [{ required: true, message: t('auth.login.passwordRequired'), trigger: 'blur' }],
}
```

The input must use `type="email"`, `inputmode="email"`, `autocomplete="username"` and `data-testid="login-email"`. Submit `form.email.trim()`; never trim the password.

- [ ] **Step 6: Run targeted tests and source audits**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/auth.test.ts tests/api/menu-fields.test.ts tests/router/index.test.ts tests/utils/request.test.ts tests/views/auth/login/index.test.ts --pool=threads --maxWorkers=1
rg -n "auth\.register|auth\.login\.register|policyAllowRegister|getAuthPolicy|login-username|views/auth/register" src tests
```

Expected: Vitest PASS. The search returns no Admin registration/login leftovers. `/api/v1/auth/register` must not remain in Web runtime; `registered_user` is a role code and is not a registration UI leftover.

- [ ] **Step 7: Commit only if explicitly authorized**

```powershell
git add web/src/api/auth.ts web/src/router/index.ts web/src/permission.ts web/src/api/menu.ts web/src/utils/request.ts web/src/views/auth/login/index.vue web/src/views/auth/register/index.vue web/src/i18n/messages/zh-CN.ts web/src/i18n/messages/en-US.ts web/tests/api/auth.test.ts web/tests/api/menu-fields.test.ts web/tests/router/index.test.ts web/tests/utils/request.test.ts web/tests/views/auth/login/index.test.ts web/tests/views/auth/register/index.test.ts
git commit -m "feat!: 移除管理端注册并改用邮箱登录"
```

If committed, include a body stating that the Admin Web no longer exposes registration and password login now requires `email`.

### Task 7: 锁定认证平台页面的 Admin 注册开关

**Files:**
- Modify: `web/src/views/access/auth-platforms/index.vue`
- Modify: `web/tests/views/access/auth-platforms/index.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

**Interfaces:**
- Consumes: `AuthPlatformListItem.code`, `isBuiltin`, `allowRegister` and backend guard from Task 3
- Produces: `isBuiltinAdminEdit: boolean`
- Produces: disabled Admin registration switch fixed at `YesNo.No`
- Preserves: editable `allowRegister` for non-builtin platforms

- [ ] **Step 1: Write failing UI tests for Admin and non-Admin rows**

Add a stable test ID to the switch and test both cases:

```ts
it('locks builtin admin registration off but keeps other platforms editable', async () => {
  setPermissions(['auth:platform:list', 'auth:platform:update'])
  const { wrapper } = await mountPage()
  await wrapper.get('[data-testid="auth-platform-update"]').trigger('click')
  await flushPromises()
  const adminSwitch = wrapper.get('[data-testid="auth-platform-allow-register"]')
  expect(adminSwitch.attributes('disabled')).toBeDefined()

  wrapper.unmount()
  const appRow = { ...adminRow, id: 3, code: 'app', isBuiltin: YesNo.No }
  getAuthPlatformsMock.mockResolvedValue({ list: [appRow], total: 1, page: 1, pageSize: 20 })
  const { wrapper: appWrapper } = await mountPage()
  await appWrapper.get('[data-testid="auth-platform-update"]').trigger('click')
  const appSwitch = appWrapper.get('[data-testid="auth-platform-allow-register"]')
  expect(appSwitch.attributes('disabled')).toBeUndefined()
})
```

Also force a stale fixture with builtin Admin `allowRegister: YesNo.Yes`; `openEdit` must normalize the form to No before submission.

- [ ] **Step 2: Run the page test and verify the switch is currently editable**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/access/auth-platforms/index.test.ts --pool=threads --maxWorkers=1
```

Expected: FAIL because the switch has no Admin-specific lock.

- [ ] **Step 3: Implement the explicit UI lock**

Add:

```ts
const isBuiltinAdminEdit = computed(() => {
  const platform = editingPlatform.value
  return dialogMode.value === 'edit' && platform?.code === 'admin' && platform.isBuiltin === YesNo.Yes
})
```

In `openEdit`, set `allowRegister` to No for the built-in Admin even if a stale response says Yes. Disable the switch with `:disabled="isBuiltinAdminEdit"`. In `updateInput()`, force No again for Admin so DOM manipulation cannot make the frontend send Yes. The backend guard remains authoritative.

Add concise bilingual helper text explaining that Admin does not support public registration.

- [ ] **Step 4: Run the page and API tests**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/access/auth-platforms/index.test.ts tests/api/auth-platform.test.ts --pool=threads --maxWorkers=1
```

Expected: PASS; non-Admin create/edit payloads still preserve selected `allowRegister`.

- [ ] **Step 5: Commit only if explicitly authorized**

```powershell
git add web/src/views/access/auth-platforms/index.vue web/tests/views/access/auth-platforms/index.test.ts web/src/i18n/messages/zh-CN.ts web/src/i18n/messages/en-US.ts
git commit -m "fix: 锁定管理平台公开注册开关"
```

### Task 8: 完整验证与范围审计

**Files:**
- Verify only: all files changed by Tasks 1-7
- Modify only when a verification failure proves a task-scoped defect

**Interfaces:**
- Consumes: all backend and frontend contracts produced by Tasks 1-7
- Produces: verified release candidate with no Admin registration or username-login runtime path

- [ ] **Step 1: Format backend code**

Run:

```powershell
cd D:\admin\server
go fmt ./...
```

Expected: exit 0. Review `git diff --stat` afterward so formatting did not touch unrelated files.

- [ ] **Step 2: Run complete backend verification**

Run:

```powershell
cd D:\admin\server
go vet ./...
go test ./...
go build ./...
```

Expected: all commands exit 0. PostgreSQL/Redis integration tests must run against the configured real services; do not replace them with SQLite, miniredis or Docker.

- [ ] **Step 3: Run complete frontend verification**

Run:

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

Expected: all Vitest files pass and the production build exits 0.

- [ ] **Step 4: Run exact scope and type audits**

Run:

```powershell
cd D:\admin
rg -n "FindCredentialByUsername|login-username|policyAllowRegister|getAuthPolicy|views/auth/register|auth\.register|auth\.login\.register" server web/src web/tests
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" web/src -g "*.ts" -g "*.vue"
rg -n "phone" server/internal/module/user server/internal/module/auth web/src/api web/src/store web/src/views/account/users web/tests
git diff --check
git status --short
```

Expected:

- first search has no runtime username-login or Admin registration UI result;
- explicit backend `POST /api/v1/auth/register` remains discoverable through route/service tests and is not deleted;
- AnyScript search has no output;
- phone search only shows the agreed user/auth DTO, storage and UI paths, not geography or SMS modules;
- `git diff --check` exits 0;
- status contains only task files and pre-existing user changes.

- [ ] **Step 5: Manually inspect the migration invariants from test output and diff**

Confirm all of the following before reporting completion:

```text
user_account.phone = VARCHAR(32) NULL
no phone index
admin.allow_register = 0
historical admin migration increments policy_version once
admin Policy Redis key is deleted before API service construction
login accepts email only
/auth/me and user DTOs always contain phone
explicit backend registration and registered_user remain
no RBAC menu platform work is included
```

- [ ] **Step 6: Commit final verification-only fixes only if explicitly authorized**

If verification required code fixes, stage only those task-scoped files and use the matching earlier commit type. If no fixes were needed, do not create an empty commit. Never amend or squash without separate authorization.

---

## Execution Notes

- 执行本计划前，必须完成 `2026-08-27-admin-api-namespace-baseline.md`；所有管理 API 测试从第一步起直接使用 `/api/admin/v1`，不得回退旧路径。
- Tasks 1-4 establish the backend contract; Tasks 5-7 update the Admin client. They belong to one release because the login request is intentionally breaking.
- Task 3 must complete before Task 7 so the frontend control is never the only registration boundary.
- Task 5 must complete before Task 6 because email-login `/me` fixtures already require `phone`.
- Do not start RBAC platform isolation, COS, mail or SMS while executing this plan.
- Playwright is not part of this plan; user performs the visual acceptance unless they explicitly request browser automation.
