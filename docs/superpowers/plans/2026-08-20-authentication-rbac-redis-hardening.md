# Authentication, RBAC, and Redis Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` in the main thread to implement this plan task-by-task. Do not use subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build multi-platform authentication policy management while making PostgreSQL authoritative and Redis a rebuildable zero-PostgreSQL fast path for healthy authentication and RBAC reads.

**Architecture:** Preserve the explicit request flow `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`. Business-owned Redis files hold only concrete Policy, User State, Session, and Access snapshot semantics; low-level Redis exposes only the string, compare-and-set, Lua, scan-delete, and expiry operations those modules actually use. Every cache-affecting write establishes a leased invalidating state before its PostgreSQL transaction, advances a persistent version or generation in that transaction, and atomically publishes the ready state afterward.

**Tech Stack:** Go 1.26, Gin, GORM, PostgreSQL, go-redis, JWT v5, Vue 3, TypeScript 6 strict mode, Pinia, Vue Router, Axios, Element Plus, Vitest.

---

## Execution Rules

- Work in `D:\admin`; do not create a worktree, use Docker, start or stop user services, or create Git commits.
- Before each task, run `git status --short` and preserve every unrelated user change.
- Use TDD: add the named failing test, run the exact targeted command and confirm it fails for the stated missing behavior, implement the smallest complete behavior, then rerun the targeted command.
- PostgreSQL and Redis integration tests use `server/.env`; they may skip only under `go test -short`. A dependency failure in normal test mode is a real failure.
- Do not add Adapter, Manager, Factory, BaseService, BaseRepository, DI containers, process-local caches, compatibility headers, compatibility JWT claims, or fallback in-memory state.
- Do not add online editing for `APP_SECRET`, `AUTH_COOKIE_SECURE`, `CORS_ORIGIN`, `REDIS_URL`, `POSTGRES_DSN`, or trusted proxies.
- Frontend business code must keep external values as `unknown` until strict contract parsing succeeds. Do not add explicit `any`, `any[]`, `as any`, `Record<string, any>`, or TypeScript suppression.

## File Responsibility Map

### Backend: create

- `server/internal/module/authplatform/model.go`: `sys_auth_platform` GORM model and stable runtime policy types.
- `server/internal/module/authplatform/protocol.go`: platform code, TTL, Yes/No, and policy combination validation.
- `server/internal/module/authplatform/schema.go`: platform constraints, partial unique index, and builtin `admin` initialization validation.
- `server/internal/module/authplatform/repository.go`: PostgreSQL pagination, locking, policy reads, platform mutations, session-limit enforcement, and platform-wide revocation.
- `server/internal/module/authplatform/redis.go`: `auth:policy:<platform>` ready/invalidating state and compare-token Lua operations.
- `server/internal/module/authplatform/service.go`: public policy, paged management, deployment status, and policy mutation order.
- `server/internal/module/authplatform/request.go`: strict query, URL ID, create, update, and status request conversion.
- `server/internal/module/authplatform/response.go`: closed lower-camel-case HTTP DTOs.
- `server/internal/module/authplatform/errors.go`: stable platform/session-state codes `17000` through `17005`.
- `server/internal/module/authplatform/handler.go`: HTTP binding and response writing only.
- `server/internal/module/authplatform/route.go`: public policy and permission-protected management routes.
- `server/internal/module/authplatform/*_test.go`: model, schema, repository, Redis, service, handler, route, concurrency, and failure tests.
- `server/internal/module/authclient/client.go`, `middleware.go`, and tests: exact platform/device Header parsing plus trusted client metadata shared by public Policy and authentication without a package cycle.
- `server/internal/module/authstate/state.go`, `redis.go`, `mutation.go`, and tests: concrete User State and platform-scoped Session State keys, payloads, and leased mutation operations shared by auth, authplatform, and user without creating import cycles.
- `server/internal/module/auth/redis.go`: concrete per-session Snapshot key, payload, read-through publication, and legacy-key cleanup semantics.
- `server/internal/module/access/model.go`: `sys_access_version` GORM model.
- `server/internal/module/access/schema.go`: access-version constraint, foreign key, and idempotent user backfill.
- `server/internal/module/accessstate/state.go`, `redis.go`, `mutation.go`, and tests: concrete Access State keys, version payloads, and leased mutation operations shared by access, role, menu, and user without creating import cycles.
- `server/internal/module/access/redis.go`: complete Access Snapshot key, payload, and read-through publication.
- `server/internal/module/access/errors.go`: stable Access State updating code `17006`.
- `server/internal/module/access/redis_test.go`: real Redis snapshot hit, miss, corruption, publication, and race coverage.

### Backend: modify

- `server/.env.example`: require and document `TRUSTED_PROXIES=none`.
- `server/internal/config/config.go`, `config_test.go`: parse a strict trusted-proxy allowlist and expose display-safe deployment metadata.
- `server/internal/redis/redis.go`, `redis_test.go`: add only the low-level atomic and expiry operations required by the three business Redis files.
- `server/internal/middleware/cors.go`, `cors_test.go`: allow the two authentication headers and `PATCH`.
- `server/internal/middleware/access_log.go`, `access_log_test.go`: emit non-sensitive auth/cache fields supplied by auth and access middleware.
- `server/internal/shared/i18n/catalog.go`, `catalog_test.go`: add paired Chinese/English authentication-platform and cache-state messages.
- `server/internal/module/auth/token.go`, `token_test.go`: add the mandatory platform claim and runtime Access TTL.
- `server/internal/module/auth/session_model.go`, `schema.go`, `schema_test.go`: add platform/device columns, safe legacy migration, remove the global-current index, and create the platform/session index.
- `server/internal/module/auth/session_repository.go`, `session_repository_test.go`: remove role coupling, implement stable multi-session transactions, authoritative snapshot reads, rotation, and revocation.
- `server/internal/module/auth/request.go`, `response.go`, `service.go`, `service_test.go`: accept request metadata, dynamic policy, device/IP binding, snapshots, and mutation ordering.
- `server/internal/module/auth/handler.go`, `handler_test.go`, `middleware.go`, `middleware_test.go`, `route.go`: exact headers, platform cookies, public Policy, and request-log context.
- `server/internal/module/access/repository.go`, `repository_test.go`: load authoritative access source plus persistent version; remove the separate `HasPermission` SQL path.
- `server/internal/module/access/service.go`, `service_test.go`, `handler.go`, `handler_test.go`, `middleware.go`, `middleware_test.go`: make `/access` and permission checks consume the same cached snapshot.
- `server/internal/module/user/repository.go`, `repository_test.go`, `service.go`, `service_test.go`: create access version with users and coordinate User/Session/Access state mutations.
- `server/internal/module/role/repository.go`, `repository_test.go`, `service.go`, `service_test.go`: invalidate and advance precisely affected users for role permission/status mutations.
- `server/internal/module/menu/protocol.go`, `builtin.go`, `builtin_test.go`, `repository.go`, `repository_test.go`, `service.go`, `service_test.go`: add auth-platform permissions and global access-version mutation locking.
- `server/cmd/api/main.go`, `main_test.go`: schema order, concrete dependency wiring, trusted proxies, route registration, and legacy Redis cleanup.

### Frontend: create

- `web/src/auth/platform.ts`, `platform.test.ts`: fixed Admin client platform code.
- `web/src/auth/device-id.ts`, `device-id.test.ts`: persistent canonical UUID generation and strict recovery.
- `web/src/api/auth-platform.contract.ts`, `auth-platform.contract.test.ts`: strict DTO and pagination/deployment parsers.
- `web/src/api/auth-platform.ts`, `auth-platform.test.ts`: exact management requests.
- `web/src/views/system/auth-platforms/index.vue`, `index.test.ts`: permission-aware Element Plus platform management page.

### Frontend: modify

- `web/src/utils/request.ts`, `request.test.ts`: attach platform/device headers to raw, normal, refresh, and retry requests.
- `web/src/api/auth.contract.ts`, `auth.contract.test.ts`, `auth.ts`, `auth.test.ts`: strict public Policy contract and API.
- `web/src/router/index.ts`, `index.test.ts`, `web/src/permission.ts`: restore the public registration route and enforce `allowRegister` on direct navigation.
- `web/src/views/auth/login/index.vue`, `index.test.ts`: load Policy, report failures, and conditionally show registration.
- `web/src/views/auth/register/index.vue`, `index.test.ts`: load Policy before form use, redirect when disabled, and replace hard-coded strings with i18n.
- `web/src/access/route-views.ts`, `menu-title-keys.ts`, and their tests: register the new backend-owned route and title/action keys.
- `web/src/i18n/messages/zh-CN.ts`, `en-US.ts`, `web/src/i18n/index.test.ts`: complete paired platform, policy, confirmation, validation, and deployment strings.

---

### Task 1: Trusted Proxy, CORS, and Deployment Configuration

**Files:**
- Modify: `server/.env.example`
- Modify: `server/internal/config/config.go`
- Modify: `server/internal/config/config_test.go`
- Modify: `server/internal/middleware/cors.go`
- Modify: `server/internal/middleware/cors_test.go`

- [ ] **Step 1: Add failing configuration tests**

Add table tests that require `TRUSTED_PROXIES`, accept only `none` or a trimmed/deduplicated list of concrete IP/CIDR values, and reject empty values, invalid entries, `0.0.0.0/0`, and `::/0`:

```go
func TestLoadAPIParsesTrustedProxies(t *testing.T) {
	values := validAPIValues()
	values["TRUSTED_PROXIES"] = "10.0.0.8, 10.20.0.0/16,10.0.0.8"
	got, err := LoadAPI(lookup(values))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10.0.0.8", "10.20.0.0/16"}
	if !reflect.DeepEqual(got.TrustedProxies, want) || got.TrustedProxyMode != "allowlist" {
		t.Fatalf("trusted proxies = %v mode=%q", got.TrustedProxies, got.TrustedProxyMode)
	}
}

func TestLoadAPIRejectsUnsafeTrustedProxies(t *testing.T) {
	for _, value := range []string{"", "bad", "0.0.0.0/0", "::/0", "none,127.0.0.1"} {
		t.Run(value, func(t *testing.T) {
			values := validAPIValues()
			values["TRUSTED_PROXIES"] = value
			_, err := LoadAPI(lookup(values))
			assertErrorContains(t, err, "TRUSTED_PROXIES")
		})
	}
}
```

Also change the CORS test to assert `PATCH`, `X-Auth-Platform`, and `X-Device-ID` are allowed while an unlisted header remains rejected.

- [ ] **Step 2: Run tests and confirm the missing behavior**

Run:

```powershell
cd D:\admin\server
go test ./internal/config ./internal/middleware -run "TestLoadAPI|TestCORS" -count=1
```

Expected: FAIL because `API` has no trusted-proxy fields, the environment key is not required, and CORS omits the new headers or method.

- [ ] **Step 3: Implement strict parsing and CORS allowlisting**

Add these fields and values:

```go
type API struct {
	HTTPAddr        string
	PostgresDSN     string
	RedisURL        string
	CORSOrigin      string
	AppSecret       string
	TrustedProxies  []string
	TrustedProxyMode string
	Auth            Auth
}

func loadTrustedProxies(lookupEnv LookupEnv) ([]string, string, error) {
	raw, err := required(lookupEnv, "TRUSTED_PROXIES")
	if err != nil {
		return nil, "", err
	}
	if raw == "none" {
		return []string{}, "none", nil
	}
	seen := make(map[string]struct{})
	values := make([]string, 0)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" || entry == "none" || entry == "0.0.0.0/0" || entry == "::/0" {
			return nil, "", fmt.Errorf("TRUSTED_PROXIES: unsafe entry %q", entry)
		}
		if net.ParseIP(entry) == nil {
			if _, _, parseErr := net.ParseCIDR(entry); parseErr != nil {
				return nil, "", fmt.Errorf("TRUSTED_PROXIES: invalid entry %q", entry)
			}
		}
		if _, exists := seen[entry]; !exists {
			seen[entry] = struct{}{}
			values = append(values, entry)
		}
	}
	if len(values) == 0 {
		return nil, "", fmt.Errorf("TRUSTED_PROXIES: allowlist is empty")
	}
	return values, "allowlist", nil
}
```

Extend CORS exactly as follows:

```go
AllowMethods: []string{
	http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodDelete, http.MethodOptions,
},
AllowHeaders: []string{
	"Origin", "Content-Type", "Authorization", "Accept-Language",
	"X-Auth-Platform", "X-Device-ID", RequestIDHeader,
},
```

Document `TRUSTED_PROXIES=none` in `server/.env.example` and add it to `validAPIValues()` plus the required-key loop.

- [ ] **Step 4: Rerun targeted tests**

Run the Step 2 command. Expected: PASS.

### Task 2: Concrete Redis Atomic Primitives

**Files:**
- Modify: `server/internal/redis/redis.go`
- Modify: `server/internal/redis/redis_test.go`

- [ ] **Step 1: Add failing real-Redis tests**

Cover conditional set, expiry inspection, compare-token Lua execution, multi-key deletion, and legacy scan deletion:

```go
func TestAtomicStringOperations(t *testing.T) {
	client := openIntegrationClient(t)
	ctx := context.Background()
	key := "test:redis:atomic:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = client.Delete(ctx, key) })

	installed, err := client.SetStringIfMissing(ctx, key, `{"state":"ready"}`, time.Minute)
	if err != nil || !installed {
		t.Fatalf("first SetStringIfMissing = %v,%v", installed, err)
	}
	installed, err = client.SetStringIfMissing(ctx, key, "replacement", time.Minute)
	if err != nil || installed {
		t.Fatalf("second SetStringIfMissing = %v,%v", installed, err)
	}
	ttl, found, err := client.TTL(ctx, key)
	if err != nil || !found || ttl <= 0 || ttl > time.Minute {
		t.Fatalf("TTL = %v,%v,%v", ttl, found, err)
	}
}
```

The Lua test must set JSON containing `mutationToken=token-a`, prove token `token-b` cannot publish, prove `token-a` can publish, and assert the final payload exactly. The scan test must delete only keys under a unique `auth:current-session:<test-prefix>*` prefix.

- [ ] **Step 2: Run the integration test and confirm it fails**

Run:

```powershell
cd D:\admin\server
go test ./internal/redis -run "TestAtomicStringOperations|TestEvalString|TestScanDelete" -count=1
```

Expected: FAIL to compile because the required methods do not exist.

- [ ] **Step 3: Add only the required low-level methods**

Use the exact public surface below; business keys and payloads must not enter this package:

```go
func (c *Client) SetStringIfMissing(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, bool, error)
func (c *Client) EvalString(ctx context.Context, script string, keys []string, args ...any) (string, error)
func (c *Client) DeleteMany(ctx context.Context, keys []string) error
func (c *Client) ScanDelete(ctx context.Context, pattern string) error
```

Implementation requirements:

```go
installed, err := c.client.SetNX(ctx, key, value, ttl).Result()
ttl, err := c.client.TTL(ctx, key).Result()
value, err := c.client.Eval(ctx, script, keys, args...).Text()
_, err := c.client.Del(ctx, keys...).Result()
```

`TTL` maps Redis `-2` to `found=false`, returns Redis `-1` as `ttl=-1, found=true` for a key without expiry, returns positive durations unchanged, and wraps transport/protocol failures. Snapshot writers require positive TTLs; ready state writers may deliberately persist until Redis eviction/flush, while invalidating states always use their finite lease. `ScanDelete` loops `SCAN` with count 100, deletes each returned batch, ends only at cursor zero, and returns the first error without pretending cleanup succeeded.

- [ ] **Step 4: Rerun the targeted Redis tests**

Run the Step 2 command. Expected: PASS with the configured real Redis.

### Task 3: PostgreSQL Models, Safe Session Migration, and Access Versions

**Files:**
- Create: `server/internal/module/authplatform/model.go`
- Create: `server/internal/module/authplatform/protocol.go`
- Create: `server/internal/module/authplatform/schema.go`
- Create: `server/internal/module/authplatform/schema_test.go`
- Create: `server/internal/module/access/model.go`
- Create: `server/internal/module/access/schema.go`
- Create: `server/internal/module/access/schema_test.go`
- Modify: `server/internal/module/auth/session_model.go`
- Modify: `server/internal/module/auth/schema.go`
- Modify: `server/internal/module/auth/schema_test.go`
- Modify: `server/internal/module/user/repository.go`
- Modify: `server/internal/module/user/repository_test.go`
- Modify: `server/cmd/api/main.go`

- [ ] **Step 1: Add failing model and real-PostgreSQL schema tests**

Assert the complete columns and constraints from spec sections 5.1 through 5.3, including `TIMESTAMPTZ`, explicit timestamps, Yes/No checks, partial unique platform code, absence of `deleted_at` on versions, and the replacement session index. Use these core model expectations:

```go
func TestPlatformTableName(t *testing.T) {
	if got := (authplatform.Platform{}).TableName(); got != "sys_auth_platform" {
		t.Fatalf("TableName() = %q", got)
	}
}

func TestAccessVersionTableName(t *testing.T) {
	if got := (access.Version{}).TableName(); got != "sys_access_version" {
		t.Fatalf("TableName() = %q", got)
	}
}
```

Add a migration test that creates the old `sys_user_session` shape and `ux_sys_user_session_current`, inserts one active session, calls `auth.PrepareSessionSchema`, and verifies atomically that:

```text
platform = admin
device_id = empty historical audit value
revoked_at is non-null
ux_sys_user_session_current is absent
ix_sys_user_session_user_platform_active is present after EnsureSchema
```

Call `auth.PrepareSessionSchema` a second time and verify no newer post-migration session is revoked.

Extend `TestCreateWithRolePersistsUserAndRoleAtomically` to require the new user's `sys_access_version` row at version 1, and extend the forced relationship failure test to prove no version row survives a rollback.

- [ ] **Step 2: Run schema tests and confirm the missing tables/columns**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/authplatform ./internal/module/access ./internal/module/auth ./internal/module/user -run "Test.*Schema|Test.*TableName|TestPrepareSessionSchema|TestCreateWithRole" -count=1
```

Expected: FAIL because the new packages/types do not exist and the session still has the global current-session index.

- [ ] **Step 3: Define exact models and validation constants**

Create these models without embedding base structs:

```go
type Platform struct {
	ID                     int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Code                   string         `gorm:"column:code;type:varchar(49);not null"`
	Name                   string         `gorm:"column:name;type:varchar(64);not null"`
	PolicyVersion          int64          `gorm:"column:policy_version;not null;default:1"`
	AccessTTLSeconds       int            `gorm:"column:access_ttl_seconds;not null"`
	RefreshTTLSeconds      int            `gorm:"column:refresh_ttl_seconds;not null"`
	SessionCacheTTLSeconds int            `gorm:"column:session_cache_ttl_seconds;not null"`
	AccessCacheTTLSeconds  int            `gorm:"column:access_cache_ttl_seconds;not null"`
	BindDevice             yesno.Value    `gorm:"column:bind_device;type:smallint;not null"`
	BindIP                 yesno.Value    `gorm:"column:bind_ip;type:smallint;not null"`
	MaxSessions            int16          `gorm:"column:max_sessions;type:smallint;not null"`
	AllowRegister          yesno.Value    `gorm:"column:allow_register;type:smallint;not null"`
	IsEnabled              yesno.Value    `gorm:"column:is_enabled;type:smallint;not null"`
	IsBuiltin              yesno.Value    `gorm:"column:is_builtin;type:smallint;not null"`
	CreatedAt              time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt              time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt              gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

type Version struct {
	UserID    int64     `gorm:"column:user_id;primaryKey"`
	Version   int64     `gorm:"column:version;not null;default:1"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
}
```

Extend `auth.Session` with non-null `Platform string` and `DeviceID string` columns. New-session validation later rejects an empty Device ID; only migrated history may contain it.

- [ ] **Step 4: Implement explicit idempotent DDL and migration order**

`auth.PrepareSessionSchema` must first inspect `to_regclass('sys_user_session')`. If absent, return without work. If both new columns already exist, return without revoking sessions. Otherwise run one PostgreSQL transaction containing:

```sql
ALTER TABLE sys_user_session ADD COLUMN IF NOT EXISTS platform VARCHAR(49) NOT NULL DEFAULT 'admin';
ALTER TABLE sys_user_session ADD COLUMN IF NOT EXISTS device_id VARCHAR(36) NOT NULL DEFAULT '';
UPDATE sys_user_session
SET revoked_at = COALESCE(revoked_at, CURRENT_TIMESTAMP),
    updated_at = CASE WHEN revoked_at IS NULL THEN CURRENT_TIMESTAMP ELSE updated_at END
WHERE revoked_at IS NULL;
DROP INDEX IF EXISTS ux_sys_user_session_current;
ALTER TABLE sys_user_session ALTER COLUMN platform DROP DEFAULT;
ALTER TABLE sys_user_session ALTER COLUMN device_id DROP DEFAULT;
```

After AutoMigrate, `auth.EnsureSchema` creates:

```sql
CREATE INDEX IF NOT EXISTS ix_sys_user_session_user_platform_active
ON sys_user_session (user_id, platform, created_at DESC, id DESC)
WHERE revoked_at IS NULL;
```

`access.EnsureSchema` creates `ck_sys_access_version_version`, its user foreign key, and runs:

```sql
INSERT INTO sys_access_version (user_id, version, created_at, updated_at)
SELECT id, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM sys_user
ON CONFLICT (user_id) DO NOTHING;
```

`authplatform.EnsureSchema` creates all named checks and `ux_sys_auth_platform_code_active`, then locks the table and either inserts the exact builtin `admin` values from spec section 5.1 or fails if an existing active/history row differs. It never overwrites existing values.

Wire startup in this exact order:

```go
auth.PrepareSessionSchema(ctx, db)
database.AutoMigrate(ctx, db, /* existing models */, &authplatform.Platform{}, &auth.Session{}, &access.Version{})
role.EnsureSchema(ctx, db)
authplatform.EnsureSchema(ctx, db)
auth.EnsureSchema(ctx, db)
menu.EnsureSchema(ctx, db)
access.EnsureSchema(ctx, db)
```

- [ ] **Step 5: Create each new user's access version in the existing user transaction**

After creating `sys_user` and `sys_user_role`, `user.Repository.CreateWithRole` inserts the version using explicit SQL so `user` does not import `access` and form a cycle:

```sql
INSERT INTO sys_access_version (user_id, version, created_at, updated_at)
VALUES (?, 1, ?, ?)
```

At transaction entry, before locking the default role or inserting a user, execute `LOCK TABLE sys_user IN ROW EXCLUSIVE MODE`. This is the user-write side of Task 11's fixed global-menu lock order. Use the same UTC microsecond-truncated timestamp for the new user, user-role relation, and access version. Any version insert failure rolls back the user and relation. Both normal registration and `bootstrap-admin` already call `CreateWithRole`, so they share this transaction without a second path.

- [ ] **Step 6: Rerun schema and user creation tests**

Run the Step 2 command. Expected: PASS on the configured PostgreSQL.

### Task 4: Authentication Platform Policy Read and Management HTTP Contract

**Files:**
- Create: `server/internal/module/authplatform/repository.go`
- Create: `server/internal/module/authplatform/redis.go`
- Create: `server/internal/module/authplatform/service.go`
- Create: `server/internal/module/authplatform/request.go`
- Create: `server/internal/module/authplatform/response.go`
- Create: `server/internal/module/authplatform/errors.go`
- Create: `server/internal/module/authplatform/handler.go`
- Create: `server/internal/module/authplatform/route.go`
- Create: `server/internal/module/authplatform/repository_test.go`
- Create: `server/internal/module/authplatform/redis_test.go`
- Create: `server/internal/module/authplatform/service_test.go`
- Create: `server/internal/module/authplatform/handler_test.go`
- Create: `server/internal/module/authclient/client.go`
- Create: `server/internal/module/authclient/middleware.go`
- Create: `server/internal/module/authclient/middleware_test.go`
- Modify: `server/internal/shared/i18n/catalog.go`
- Modify: `server/internal/shared/i18n/catalog_test.go`

- [ ] **Step 1: Add failing protocol, repository, and handler tests**

Tests must prove:

```text
platform code regex is ^[a-z][a-z0-9_]{1,48}$
platform/device Headers each require exactly one canonical value before Policy or auth handlers run
list query is a strict allowlist and page/pageSize are required exactly once
create requires every mutable policy field and rejects unknown JSON keys
update rejects code, isBuiltin, and policyVersion as unknown keys
status accepts only isEnabled
DELETE requires an empty body
list order is updated_at DESC, id DESC and response arrays are non-null
public Policy returns exactly code, name, allowRegister
deployment returns exactly cookieSecure, corsOrigin, trustedProxyMode, trustedProxyCount, redisStatus
```

Use this public contract assertion:

```go
recorder := httptest.NewRecorder()
request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/policy", nil)
request.Header["X-Auth-Platform"] = []string{"admin"}
request.Header["X-Device-ID"] = []string{"550e8400-e29b-41d4-a716-446655440000"}
router.ServeHTTP(recorder, request)
assertJSON(t, recorder.Body.Bytes(), `{
  "code":0,
  "data":{"code":"admin","name":"Admin","allowRegister":1},
  "message":"ok"
}`)
```

The router in this test installs `authclient.Require()` before the public Policy handler. Add table cases for missing, repeated, empty, whitespace-padded, and malformed platform/device values; each fails before the service is called.

Repository integration tests must insert multiple active/deleted platforms and verify filtered count/list, row locking, code conflict mapping, builtin protection, `policy_version + 1`, and transaction rollback.

- [ ] **Step 2: Run tests and confirm the module is absent**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/authclient ./internal/module/authplatform ./internal/shared/i18n -count=1
```

Expected: FAIL because repository, service, HTTP, errors, and cache code are not implemented.

- [ ] **Step 3: Define stable runtime and HTTP types**

Use these domain types:

```go
type Policy struct {
	ID                     int64
	Code                   string
	Name                   string
	PolicyVersion          int64
	AccessTTL              time.Duration
	RefreshTTL             time.Duration
	SessionCacheTTL        time.Duration
	AccessCacheTTL         time.Duration
	BindDevice             bool
	BindIP                 bool
	MaxSessions            int16
	AllowRegister          bool
	IsEnabled              bool
	IsBuiltin              bool
	Deleted                bool
}

type ListQuery struct {
	Page      int
	PageSize  int
	Keyword   string
	IsEnabled *yesno.Value
}

type Deployment struct {
	CookieSecure      bool
	CORSOrigin        string
	TrustedProxyMode  string
	TrustedProxyCount int
	RedisStatus       string
}
```

TTL conversion must reject overflow and retain integer seconds exactly. `ValidatePolicy` enforces exactly the spec ranges and Yes/No validity; it does not invent an ordering relationship between Access and Refresh TTL.

- [ ] **Step 4: Implement authoritative repository reads and strict errors**

Repository methods are concrete and PostgreSQL-only:

```go
func (r *Repository) FindPolicy(ctx context.Context, code string) (Platform, error)
func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error)
func (r *Repository) List(ctx context.Context, query ListQuery) ([]Platform, error)
func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error
func (r *Repository) LockByID(ctx context.Context, id int64) (Platform, error)
func (r *Repository) LockByCodeUnscoped(ctx context.Context, code string) ([]Platform, error)
func (r *Repository) Create(ctx context.Context, value *Platform) error
func (r *Repository) UpdatePolicy(ctx context.Context, id int64, values UpdateValues, updatedAt time.Time) (int64, error)
func (r *Repository) UpdateStatus(ctx context.Context, id int64, value yesno.Value, updatedAt time.Time) (int64, error)
func (r *Repository) SoftDelete(ctx context.Context, id int64, deletedAt time.Time) (int64, error)
```

Map errors to exact codes:

```go
const (
	CodeNotFound         = 17000
	CodeCodeConflict     = 17001
	CodeBuiltinProtected = 17002
	CodeInvalidPolicy    = 17003
	CodeDisabled         = 17004
	CodeSessionUpdating  = 17005
)
```

Add matching bilingual catalog entries and validate the catalog key set remains identical.

Add module-specific dependency message keys while preserving business code `10006`: platform Policy/cache unavailable, authentication session/cache unavailable, and access state/snapshot unavailable. These messages distinguish the failing internal path without exposing Redis keys, PostgreSQL text, or credentials.

- [ ] **Step 5: Implement Policy Redis read-through without hiding failures**

The cache payload is closed JSON with a schema version and explicit state:

```go
type policyState struct {
	SchemaVersion int     `json:"schemaVersion"`
	State         string  `json:"state"`
	MutationToken *string `json:"mutationToken"`
	Policy        *Policy `json:"policy"`
}
```

`CurrentPolicy(ctx, code)` performs:

```text
Get auth:policy:<code>
ready valid payload -> return policy
invalidating valid payload -> return 17005
missing/read error/corrupt payload -> query PostgreSQL
PostgreSQL success -> re-read state, SET NX ready only if still missing
concurrent ready -> return the newly installed ready policy
concurrent invalidating -> return 17005 without returning the stale PostgreSQL row
Redis still unavailable after PostgreSQL success -> return PostgreSQL policy
```

Do not include deployment settings in the cached policy. A disabled active platform returns `17004`; a missing or deleted platform returns `17000`. Give the service the existing concrete `*slog.Logger`; on cache read/corruption/write errors that still return PostgreSQL authority, log `cacheKind=policy`, `cacheResult=error|rebuilt`, and the internal cause, never the payload or connection URL.

- [ ] **Step 6: Implement handlers and routes with exact permissions**

Create the small concrete request-metadata package first:

```go
const (
	PlatformHeader = "X-Auth-Platform"
	DeviceIDHeader = "X-Device-ID"
)

type Client struct {
	Platform  string
	DeviceID  string
	ClientIP  string
	UserAgent string
}

func Require() gin.HandlerFunc
func FromContext(context *gin.Context) (Client, bool)
```

`Require` reads `context.Request.Header.Values`, requires exactly one value for each Header, rejects surrounding whitespace, validates the platform regex and canonical lowercase UUID, then records `context.ClientIP()` and a User-Agent capped at 512 bytes. It stores no Gin Context below Handler/Middleware. The public Policy handler reads only `Client.Platform` and passes that plain string plus `context.Request.Context()` to its service. Package `authclient` imports no auth or authplatform package.

Expose:

```go
func RegisterPublicRoutes(routes *gin.RouterGroup, handler *Handler) {
	routes.GET("/auth/policy", handler.Policy)
}

func RegisterManagementRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/auth-platforms", authenticate, requirePermission("system:auth-platform:list"), handler.List)
	routes.GET("/auth-platforms/deployment", authenticate, requirePermission("system:auth-platform:list"), handler.Deployment)
	routes.POST("/auth-platforms", authenticate, requirePermission("system:auth-platform:create"), handler.Create)
	routes.PUT("/auth-platforms/:id", authenticate, requirePermission("system:auth-platform:update"), handler.Update)
	routes.PATCH("/auth-platforms/:id/status", authenticate, requirePermission("system:auth-platform:status"), handler.UpdateStatus)
	routes.DELETE("/auth-platforms/:id", authenticate, requirePermission("system:auth-platform:delete"), handler.Delete)
}
```

At this task boundary, mutation service tests may use a fake Policy invalidation store and fake platform-session coordinator. The real Redis/session mutation wiring is completed in Tasks 6 and 12; production `cmd/api` wiring is intentionally deferred until those concrete dependencies exist.

- [ ] **Step 7: Rerun module tests**

Run the Step 2 command. Expected: PASS.

### Task 5: Exact Platform/Device Request Metadata, JWT, Cookie, and Client IP

**Files:**
- Modify: `server/internal/module/auth/request.go`
- Modify: `server/internal/module/auth/response.go`
- Modify: `server/internal/module/auth/token.go`
- Modify: `server/internal/module/auth/token_test.go`
- Modify: `server/internal/module/auth/middleware.go`
- Modify: `server/internal/module/auth/middleware_test.go`
- Modify: `server/internal/module/auth/handler.go`
- Modify: `server/internal/module/auth/handler_test.go`
- Modify: `server/internal/module/auth/route.go`
- Modify: `server/internal/middleware/access_log.go`
- Modify: `server/internal/middleware/access_log_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`

- [ ] **Step 1: Add failing auth-client, JWT, and Cookie tests**

Header parsing itself is already covered by `authclient` in Task 4. Add auth middleware/handler tests proving the stored client metadata reaches Login, Register, Refresh, Authenticate, and Logout unchanged, and JWT authentication rejects a valid token when its platform differs from the client selected by the Header:

```go
client := authclient.Client{
	Platform: "admin", DeviceID: "550e8400-e29b-41d4-a716-446655440000",
	ClientIP: "127.0.0.1", UserAgent: "test-agent",
}
```

JWT tests must prove:

```text
platform claim is mandatory and matches ^[a-z][a-z0-9_]{1,48}$
Issue uses the supplied Access TTL rather than a package constant
issuer, uid, sid, platform, version, iat, nbf, and exp are the only claims
old tokens without platform are rejected
Header platform and token platform mismatch is unauthorized
```

Handler tests must prove `admin_refresh_admin` and `admin_refresh_app` do not overwrite each other and Logout expires only the selected platform cookie.

- [ ] **Step 2: Run targeted tests and confirm current permissive behavior**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/auth ./internal/middleware ./cmd/api -run "Test.*Client|Test.*JWT|Test.*Cookie|TestBuildRouter|TestAccessLog" -count=1
```

Expected: FAIL because auth does not consume `authclient.Client`, no platform claim exists, TTL is fixed, and the cookie name is global.

- [ ] **Step 3: Install and consume the explicit request metadata middleware**

Use the Task 4 type directly in auth inputs and identity checks:

```go
type LoginInput struct {
	Username string
	Password string
	Client   authclient.Client
}
```

Apply the same concrete `Client` field to Register and Refresh inputs. Auth handlers call `authclient.FromContext`; the auth middleware passes it beside the raw Bearer Token to `Authenticate`. Register `authclient.Require()` once on `/api/v1` before all auth and business routes; `/health` and `/ready` stay outside that group.

Configure Gin immediately after `gin.New()`:

```go
if err := router.SetTrustedProxies(dependencies.TrustedProxies); err != nil {
	panic(fmt.Sprintf("set trusted proxies: %v", err))
}
```

Pass `nil` for `none`. Never retain Gin's default trust-all behavior.

- [ ] **Step 4: Make JWT and platform cookies runtime-specific**

Replace the fixed token API with:

```go
type TokenIdentity struct {
	UserID    int64
	SessionID int64
	Platform  string
	Version   int64
}

func (j *JWT) Issue(identity TokenIdentity, ttl time.Duration) (string, time.Time, error)
func (j *JWT) Parse(raw string) (TokenIdentity, error)
```

Claims add:

```go
Platform string `json:"platform"`
```

`Issue` rejects TTL outside 60 seconds through 30 days. Cookie naming is:

```go
func refreshCookieName(platform string) string {
	return "admin_refresh_" + platform
}
```

Login, Refresh, Logout, Register, and Authenticate receive the `authclient.Client` selected by middleware. The Task 4 Policy handler uses the same context value. Refresh reads only the selected platform cookie. Cookie Max-Age remains the positive number of whole seconds until the persisted absolute refresh expiry.

- [ ] **Step 5: Add safe authentication log fields**

Extend the existing request log state with:

```go
type authenticationLog struct {
	Platform    string
	UserID      int64
	SessionID   int64
	CacheKind   string
	CacheResult string
	AccessVersion int64
}
```

Expose setter functions called only by auth/access middleware and handlers. The logger may emit `authPlatform`, `userID`, `sessionID`, `cacheKind`, `cacheResult`, and `accessVersion`; tests must prove Authorization, Cookie, token strings, full Device ID, APP_SECRET, Redis URL, DSN, password, and request body never appear.

- [ ] **Step 6: Rerun targeted tests**

Run the Step 2 command. Expected: PASS.

### Task 6: Session Redis Fast Path and Multi-Platform Session Mutations

**Files:**
- Create: `server/internal/module/authstate/state.go`
- Create: `server/internal/module/authstate/state_test.go`
- Create: `server/internal/module/authstate/redis.go`
- Create: `server/internal/module/authstate/redis_test.go`
- Create: `server/internal/module/authstate/mutation.go`
- Create: `server/internal/module/authstate/mutation_test.go`
- Create: `server/internal/module/auth/redis.go`
- Create: `server/internal/module/auth/redis_test.go`
- Modify: `server/internal/module/auth/session_repository.go`
- Modify: `server/internal/module/auth/session_repository_test.go`
- Modify: `server/internal/module/auth/service.go`
- Modify: `server/internal/module/auth/service_test.go`
- Modify: `server/internal/module/auth/handler.go`
- Modify: `server/internal/module/auth/middleware.go`
- Delete: `server/internal/module/user/session_pointer.go`
- Delete: `server/internal/module/user/session_pointer_test.go`

- [ ] **Step 1: Replace pointer-era tests with failing snapshot tests**

Delete tests that require `auth:current-session:<userID>`. Add tests for exact keys:

```go
func TestSessionRedisKeys(t *testing.T) {
	if got := authstate.UserStateKey(7); got != "auth:user-state:7" { t.Fatal(got) }
	if got := authstate.SessionsStateKey("admin", 7); got != "auth:sessions-state:admin:7" { t.Fatal(got) }
	if got := auth.SessionKey("admin", 11); got != "auth:session:admin:11" { t.Fatal(got) }
}
```

Service tests use counting fakes and prove:

```text
healthy matching Policy + User State + Session State + Session Snapshot -> zero PostgreSQL session/user calls
any Redis miss/read error/corrupt closed JSON -> PostgreSQL authoritative read and safe rebuild
Redis error plus PostgreSQL success -> return PostgreSQL identity
Redis error plus PostgreSQL failure -> 10006, never old/empty identity
invalidating Policy/User/Session state -> 17005 and no stale identity
device/IP mismatch -> request rejected; PostgreSQL revoke occurs only after invalidating state exists
valid enabled user with zero enabled roles authenticates
snapshot TTL = min(sessionCacheTTL, remaining refresh lifetime)
```

Repository tests prove 0/1/N session retention per `user_id + platform`, stable newest ordering by `created_at DESC, id DESC`, cross-platform isolation, concurrent limit enforcement, refresh rotation, and transaction rollback.

- [ ] **Step 2: Run targeted tests and confirm pointer behavior fails them**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/authstate ./internal/module/auth ./internal/module/user -run "TestSessionRedis|Test.*State|Test.*Lease|Test.*Snapshot|Test.*Session|Test.*Authenticate|Test.*Refresh|Test.*Logout" -count=1
```

Expected: FAIL because the service still reads the current-session pointer and then always queries PostgreSQL.

- [ ] **Step 3: Define closed shared state payloads and the auth-owned Session Snapshot**

Use JSON schema version 1 and these payloads:

```go
type UserState struct {
	SchemaVersion int     `json:"schemaVersion"`
	State         string  `json:"state"`
	UserID        int64   `json:"userId"`
	Generation    string  `json:"generation"`
	IsEnabled     bool    `json:"isEnabled"`
	Deleted       bool    `json:"deleted"`
	MutationToken *string `json:"mutationToken"`
}

type SessionsState struct {
	SchemaVersion int     `json:"schemaVersion"`
	State         string  `json:"state"`
	Platform      string  `json:"platform"`
	UserID        int64   `json:"userId"`
	Generation    string  `json:"generation"`
	MutationToken *string `json:"mutationToken"`
}

type SessionSnapshot struct {
	SchemaVersion     int       `json:"schemaVersion"`
	UserID            int64     `json:"userId"`
	SessionID         int64     `json:"sessionId"`
	Platform          string    `json:"platform"`
	SessionVersion    int64     `json:"sessionVersion"`
	PolicyVersion     int64     `json:"policyVersion"`
	UserGeneration    string    `json:"userGeneration"`
	SessionsGeneration string   `json:"sessionsGeneration"`
	DeviceID          string    `json:"deviceId"`
	ClientIP          string    `json:"clientIp"`
	RefreshExpiresAt  time.Time `json:"refreshExpiresAt"`
	Revoked           bool      `json:"revoked"`
}
```

`UserState` and `SessionsState` live in package `authstate`, which imports no auth, authplatform, or user package. `SessionSnapshot` lives in package `auth`. All decoders reject unknown/missing fields, invalid enums, empty generations, invalid IDs, non-UTC or invalid time, and mismatched key identity. Random generations and mutation tokens use 32 cryptographically random bytes encoded with raw URL-safe base64.

`authstate.Store` exposes only these concrete read/install operations; compare-token transitions remain private to `authstate` and are used by its `Invalidator`:

```go
type UserFact struct {
	UserID     int64
	Generation string
	IsEnabled  bool
	Deleted    bool
}

type SessionsFact struct {
	Platform   string
	UserID     int64
	Generation string
}

type MutationFacts struct {
	Users    []UserFact
	Sessions []SessionsFact
}

func (s *Store) ReadUser(ctx context.Context, userID int64) (UserState, bool, error)
func (s *Store) ReadSessions(ctx context.Context, platform string, userID int64) (SessionsState, bool, error)
func (s *Store) InstallUserReadyIfMissing(ctx context.Context, fact UserFact) (UserState, bool, error)
func (s *Store) InstallSessionsReadyIfMissing(ctx context.Context, fact SessionsFact) (SessionsState, bool, error)

func NewInvalidator(store *Store) *Invalidator
func (i *Invalidator) Acquire(ctx context.Context, candidates MutationFacts) (*MutationLease, error)
func (l *MutationLease) StartRenewal(parent context.Context) (context.Context, func())
func (l *MutationLease) Commit(ctx context.Context, next MutationFacts) error
func (l *MutationLease) Rollback(ctx context.Context) error
```

The `bool` returned by each install method is `true` only when this call installed the supplied ready fact; the returned state is always the actual current state, including a concurrent ready or invalidating value. Candidates and committed facts require positive IDs, valid platform codes, non-empty generations, unique User State IDs, and unique Session State `(platform, userID)` pairs. They are normalized by user ID and then platform before Redis acquisition. Duplicate keys with different facts are rejected. `Acquire` transitions every exact ready candidate to invalidating, restores earlier transitions if a later one fails, and never enters PostgreSQL itself. `Commit` requires exactly the same key set and freshly generated ready generations; `Rollback` restores the captured prior facts. Both use only the lease's mutation tokens. The concrete lease is fixed at 30 seconds and renews every 10 seconds; tests inject a private clock/interval option rather than exposing runtime configuration.

`auth.SessionCache` implements read, conditional publish, and deletion for `auth:session:<platform>:<sessionID>`. Lua compares the parsed state's `mutationToken`; a token mismatch returns a stable non-success result and never changes the key. This split lets auth, user, and authplatform coordinate generations without importing one another, while the credential-bearing Session Snapshot remains owned by `auth`.

`authstate` returns concrete sentinel errors such as `ErrUpdating`, `ErrGenerationChanged`, and `ErrMutationTokenMismatch`; it does not construct HTTP errors. Auth/user/authplatform services translate `ErrUpdating` to the appropriate `17005` or mutation failure and map Redis transport failures to `10006`.

- [ ] **Step 4: Implement authoritative session repository operations**

Replace role-coupled identity SQL with enabled/non-deleted user plus active session checks. Required methods:

```go
func (r *SessionRepository) CreateWithinLimit(ctx context.Context, input SessionCreate, policy authplatform.Policy, now time.Time) (Session, []Session, error)
func (r *SessionRepository) FindAuthoritative(ctx context.Context, token TokenIdentity, now time.Time) (SessionAuthority, error)
func (r *SessionRepository) FindByRefreshHash(ctx context.Context, platform, hash string, now time.Time) (SessionAuthority, error)
func (r *SessionRepository) RotateByRefreshHash(ctx context.Context, sessionID int64, platform, oldHash, newHash string, now time.Time, client authclient.Client) (Session, bool, error)
func (r *SessionRepository) Revoke(ctx context.Context, sessionID int64, now time.Time) error
```

`CreateWithinLimit` locks the user row, rechecks enabled/non-deleted user and enabled platform inside the transaction, loads active platform sessions in stable descending order, inserts the new session, then revokes every record after the newest N when N is nonzero. It returns revoked sessions so their Redis snapshots can be removed after commit.

- [ ] **Step 5: Implement the read-through authentication algorithm**

`Authenticate(ctx, rawToken, client)` executes in this order:

```text
parse and locally verify JWT
require token platform == client platform
read current Policy
read User State
read Sessions State
read Session Snapshot
validate every ID, platform, session version, policy version, generation, refresh expiry, revoked state, device, and IP
return Identity immediately on full hit
on miss/error/corruption, query FindAuthoritative
re-read Policy/User/Session state before publishing
install or retry against the newest ready generations
return PostgreSQL authority if Redis remains unavailable
```

Identity carries `Platform`, `PolicyVersion`, `AccessCacheTTL`, and non-response cache diagnostics for access/logging. The service uses its concrete `*slog.Logger` for successful-fallback cache errors with `cacheKind=userState|sessionState|sessionSnapshot`; it logs the error cause but never the cache payload, full Device ID, token, Cookie, or Redis URL.

- [ ] **Step 6: Implement coordinated Login, Refresh, Logout, and Register**

- Register reads a ready enabled Policy, rejects `allowRegister=false`, requires Redis `PING`, and creates user + default role + access version in one PostgreSQL transaction.
- Login reads Policy and credentials, begins Sessions State invalidation, starts lease renewal, calls `CreateWithinLimit`, publishes a new generation, writes the current/new snapshots, deletes revoked snapshots, and issues Access/Refresh tokens using dynamic TTLs.
- Refresh finds authority by platform/hash, establishes invalidation before rotation, enforces device/IP, rotates once, preserves `refresh_expires_at`, publishes, and issues only a new Access TTL plus rotated Refresh Token.
- Logout establishes Session State invalidation before PostgreSQL revoke, publishes a new generation, and deletes the snapshot. Redis coordination failure means PostgreSQL is not called.
- Device/IP mismatch follows the same revoke flow; if invalidation cannot be established, return `10006` and do not perform a one-sided PostgreSQL revoke.

For every transaction, start a renewal context before entering PostgreSQL. A renewal failure cancels the transaction context. On rollback, restore the prior ready state with the same token. On commit/publish failure, return `10006` and leave stale snapshots unreachable until lease expiry and read-through rebuild.

- [ ] **Step 7: Rerun auth and session tests**

Run the Step 2 command. Expected: PASS.

### Task 7: Concrete Access State and Mutation Lease Package

**Files:**
- Create: `server/internal/module/accessstate/state.go`
- Create: `server/internal/module/accessstate/state_test.go`
- Create: `server/internal/module/accessstate/redis.go`
- Create: `server/internal/module/accessstate/redis_test.go`
- Create: `server/internal/module/accessstate/mutation.go`
- Create: `server/internal/module/accessstate/mutation_test.go`

- [ ] **Step 1: Add failing lease lifecycle tests**

Use a real Redis test plus deterministic fake clock/renewer tests to prove:

```text
candidate user IDs/versions are positive, deduplicated, and sorted ascending
duplicate candidate IDs with different versions are rejected
Acquire establishes every user's invalidating state before success
failure on user N restores users 1..N-1 and returns no lease
Renew refreshes only matching mutation tokens
Rollback restores every prior ready version only with matching token
Commit publishes supplied new versions only with matching token
token mismatch never overwrites another writer
renewal failure cancels the context used by the PostgreSQL transaction
```

Core test shape:

```go
lease, err := invalidator.Acquire(ctx, []accessstate.Version{
	{UserID: 9, Version: 4}, {UserID: 7, Version: 3}, {UserID: 9, Version: 4},
})
if err != nil { t.Fatal(err) }
if got := lease.UserIDs(); !reflect.DeepEqual(got, []int64{7, 9}) { t.Fatal(got) }

transactionCtx, stop := lease.StartRenewal(ctx)
defer stop()
if err := repository.Transaction(transactionCtx, func(...) error { return forced }); !errors.Is(err, forced) {
	t.Fatal(err)
}
if err := lease.Rollback(ctx); err != nil { t.Fatal(err) }
```

- [ ] **Step 2: Run tests and confirm the coordinator is absent**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/accessstate -count=1
```

Expected: FAIL because the concrete Access State package does not exist.

- [ ] **Step 3: Implement the narrow Access State store and lease**

This is not a generic cache coordinator. Package `accessstate` knows only `authz:access-state:<userID>` and has no PostgreSQL, auth, role, menu, or user imports:

```go
type Version struct {
	UserID  int64
	Version int64
}

type State struct {
	SchemaVersion int    `json:"schemaVersion"`
	State         string `json:"state"`
	Version       int64  `json:"version"`
	MutationToken string `json:"mutationToken,omitempty"`
	BaseVersion   int64  `json:"baseVersion,omitempty"`
}

type Invalidator struct {
	store *Store
	leaseTTL time.Duration
	renewEvery time.Duration
}

type MutationLease struct {
	userIDs []int64
	priorVersions map[int64]int64
	tokens map[int64]string
	// private renewal cancellation/error state
}

func (i *Invalidator) Acquire(ctx context.Context, candidates []Version) (*MutationLease, error)
func (l *MutationLease) StartRenewal(parent context.Context) (context.Context, func())
func (l *MutationLease) Commit(ctx context.Context, versions map[int64]int64) error
func (l *MutationLease) Rollback(ctx context.Context) error
```

`Store` strictly decodes closed version-state JSON and implements read, install-ready-if-missing, begin invalidation, renew by token, publish by token, and restore by token through compare-token Lua. Use a 30-second lease and renew every 10 seconds. `Acquire` uses each caller-supplied PostgreSQL candidate version to install a missing ready state, then transitions ready -> invalidating. If Redis contains a different ready version, the caller must abandon its stale candidate and retry from PostgreSQL; if state is already invalidating, return the stable conflict/update error instead of spinning forever.

`accessstate` returns concrete sentinel errors `ErrUpdating`, `ErrVersionChanged`, and `ErrMutationTokenMismatch`; it imports no HTTP error package. `access.Service` maps `ErrUpdating` to code `17006`, while role/menu/user mutation services treat it as a bounded conflict or dependency failure according to their already documented retry point. Redis transport failures always map to `10006` before PostgreSQL mutation begins.

- [ ] **Step 4: Rerun lease tests**

Run the Step 2 command. Expected: PASS.

### Task 8: Access Snapshot Redis and One Authorization Fact

**Files:**
- Create: `server/internal/module/access/redis.go`
- Create: `server/internal/module/access/redis_test.go`
- Create: `server/internal/module/access/errors.go`
- Create: `server/internal/module/access/errors_test.go`
- Modify: `server/internal/module/access/repository.go`
- Modify: `server/internal/module/access/repository_test.go`
- Modify: `server/internal/module/access/service.go`
- Modify: `server/internal/module/access/service_test.go`
- Modify: `server/internal/module/access/handler.go`
- Modify: `server/internal/module/access/handler_test.go`
- Modify: `server/internal/module/access/middleware.go`
- Modify: `server/internal/module/access/middleware_test.go`
- Modify: `server/internal/module/access/response.go`

- [ ] **Step 1: Add failing shared-snapshot and cache tests**

Tests must prove the exact Snapshot key and payload isolation; Access State keys are tested in Task 7:

```go
func TestAccessRedisKeys(t *testing.T) {
	if got := SnapshotKey("admin", 4, 7, 9); got != "authz:access:admin:4:7:9" { t.Fatal(got) }
}
```

Add counting-fake tests for:

```text
Current and Allowed call the same loadSnapshot method
ready state + matching snapshot -> zero PostgreSQL calls
Allowed tests membership in snapshot.permissionCodes and never calls HasPermission SQL
miss/error/corrupt snapshot -> one FindSourceWithVersion authority read
PostgreSQL error -> fail closed with 10006
invalidating state -> 17006
permission codes and role codes are sorted unique non-null arrays
snapshot key and TTL use Identity platform, policyVersion, and accessCacheTTL
read-through recheck prevents an old authority read from publishing after a version mutation
```

- [ ] **Step 2: Run targeted access tests and confirm dual SQL paths**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/access -count=1
```

Expected: FAIL because `Allowed` calls `HasPermission` independently and no versioned Redis state exists.

- [ ] **Step 3: Make access source local and versioned**

To keep mutation modules acyclic, replace imported menu/role model types inside `access` with local read types:

```go
type MenuType string

const (
	MenuDirectory MenuType = "directory"
	MenuPage      MenuType = "page"
	MenuAction    MenuType = "action"
)

type SourceMenu struct {
	ID int64
	ParentID *int64
	MenuType MenuType
	Code string
	I18nKey string
	Path *string
	ViewKey *string
	Icon *string
	SortOrder int
	IsEnabled yesno.Value
}

type Source struct {
	Version        int64
	RoleCodes      []string
	Menus          []SourceMenu
	GrantedMenuIDs []int64
	SuperAdmin     bool
}
```

`FindSourceWithVersion` requires an active enabled user and exactly one `sys_access_version` row. A user with zero enabled roles is valid: it returns no role codes, no grants, the enabled menu universe, `SuperAdmin=false`, and builds empty `menuTree`/`permissionCodes`. Remove `HasPermission` from repository and its tests after equivalent snapshot tests pass.

- [ ] **Step 4: Implement the closed Access Snapshot payload and consume Access State**

```go
type CachedSnapshot struct {
	SchemaVersion   int        `json:"schemaVersion"`
	UserID          int64      `json:"userId"`
	Platform        string     `json:"platform"`
	PolicyVersion   int64      `json:"policyVersion"`
	Version         int64      `json:"version"`
	RoleCodes       []string   `json:"roleCodes"`
	MenuTree        []MenuNode `json:"menuTree"`
	PermissionCodes []string   `json:"permissionCodes"`
}
```

Decoder checks are closed and recursive, including menu shape, unique IDs/codes/paths, stable sibling order, and sorted unique role/permission strings. `access.Service` reads the ready/invalidating version through the concrete `accessstate.Store` created in Task 7; `access/redis.go` owns only the complete Snapshot. No cached `map[string]struct{}` survives a request.

- [ ] **Step 5: Implement one load path for display and enforcement**

Use these public service signatures:

```go
func (s *Service) Current(ctx context.Context, identity auth.Identity) (Snapshot, error)
func (s *Service) Allowed(ctx context.Context, identity auth.Identity, permissionCode string) (bool, error)
```

Both call:

```go
func (s *Service) loadSnapshot(ctx context.Context, identity auth.Identity) (Snapshot, error)
```

`loadSnapshot` implements state -> versioned snapshot -> PostgreSQL -> state recheck -> conditional publish. `Allowed` constructs a request-local set from `PermissionCodes` and checks the exact code. Handler and middleware set the access cache result/version log fields, but the HTTP response remains exactly `roleCodes`, `menuTree`, and `permissionCodes`.

Give `access.Service` the concrete application `*slog.Logger`. When a Redis error/corrupt Snapshot successfully falls back to PostgreSQL, log `cacheKind=accessState|accessSnapshot`, `cacheResult=error|rebuilt`, and the cause without logging payloads or connection details. PostgreSQL failure still returns the access-specific `10006` message and is recorded by the existing request error log.

- [ ] **Step 6: Rerun access tests**

Run the Step 2 command. Expected: PASS, including proof that healthy hits have zero repository calls.

### Task 9: User Creation, Role Changes, Status, and Deletion Versioning

**Files:**
- Modify: `server/internal/module/user/repository.go`
- Modify: `server/internal/module/user/repository_test.go`
- Modify: `server/internal/module/user/service.go`
- Modify: `server/internal/module/user/service_test.go`

- [ ] **Step 1: Add failing transaction and coordination tests**

Repository integration tests must prove:

```text
Task 3 user creation and bootstrap access-version behavior remains intact
UpdateRoles increments only the target version in the role transaction
UpdateStatus and Delete revoke every platform session and update User State facts in one transaction
forced failure rolls back user, roles, sessions, and access version together
```

Service tests must prove Redis write coordination happens before PostgreSQL:

```go
func TestUpdateRolesDoesNotEnterPostgreSQLWhenAccessInvalidationFails(t *testing.T) {
	repository := &countingUserRepository{}
	service := NewService(repository, failingAccessStateInvalidator())
	_, err := service.UpdateRoles(context.Background(), 1, 7, []int64{2})
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || repository.transactionCalls != 0 {
		t.Fatalf("error=%v calls=%d", err, repository.transactionCalls)
	}
}
```

Also prove user enable/disable/delete coordinates User State, every known platform Session State, and Access State; a partial invalidation failure releases already acquired states without entering PostgreSQL.

- [ ] **Step 2: Run user mutation tests and confirm missing leased invalidation**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/user -run "TestCreateWithRole|Test.*UpdateRoles|Test.*UpdateStatus|Test.*Delete" -count=1
```

Expected: FAIL because user mutations do not use leased invalidation or advance existing access versions.

- [ ] **Step 3: Extend repository transactions with explicit version operations**

Add concrete methods:

```go
func (r *Repository) LockAccessVersion(ctx context.Context, userID int64) (int64, error)
func (r *Repository) IncrementAccessVersion(ctx context.Context, userID int64, now time.Time) (int64, error)
func (r *Repository) FindActiveSessionPlatforms(ctx context.Context, userID int64) ([]string, error)
func (r *Repository) LockUserWriteTable(ctx context.Context) error
```

`LockUserWriteTable` executes `LOCK TABLE sys_user IN ROW EXCLUSIVE MODE`. Task 3's `CreateWithRole` insert remains the sole new-user version path. Every transaction that can write `sys_user` calls this method before `LockUser`/`LockUserUnscoped`: username update, role-set touch, enable/disable, and delete. `UpdateRoles` locks/increments the target version only when the effective role set actually changes. No-op mutations do not increment versions or touch Redis state after validation.

- [ ] **Step 4: Coordinate user mutations in service order**

- `UpdateRoles`: read the target persistent access version -> acquire an `accessstate` lease for that candidate -> run the existing protected transaction -> lock/recheck and increment the target version -> commit -> publish the new version.
- `UpdateStatus`/`Delete`: read and sort active platform codes plus the target persistent access version -> acquire `authstate` User/Session leases -> acquire the `accessstate` lease -> run the existing transaction and revoke all sessions -> lock/recheck and increment Access version -> publish the new User generation, each new Sessions generation, and Access version.
- Re-enable creates a fresh User generation and does not restore sessions.
- User reads/list/username-only updates do not invalidate authentication or access because those fields are not part of authorization facts.

Remove the old pointer dependency and `deleteSessionPointer`; there is no compatibility key path.

- [ ] **Step 5: Rerun user mutation tests**

Run the Step 2 command. Expected: PASS.

### Task 10: Role Mutation Candidate Recheck and Precise Version Advancement

**Files:**
- Modify: `server/internal/module/role/repository.go`
- Modify: `server/internal/module/role/repository_test.go`
- Modify: `server/internal/module/role/service.go`
- Modify: `server/internal/module/role/service_test.go`

- [ ] **Step 1: Add failing candidate/recheck/concurrency tests**

Tests cover the exact scope matrix:

```text
role create/name/default-only changes do not advance access versions
role permission change advances all effective users bound to that role
role enable/disable advances all effective users bound to that role
delete with bound users retains the existing conflict protection, restores the acquired lease, and advances no version
deleting an unbound custom role has an empty affected-user set and advances no user version
candidate IDs are unique sorted ascending
Redis acquisition failure means zero PostgreSQL transaction calls
candidate set changed before lock -> rollback, release, retry
candidate set unstable after bounded retries -> explicit conflict
transaction failure restores old ready states and versions
commit succeeds but Redis publish fails -> 10006 and old snapshot remains unreachable
```

Use two real PostgreSQL connections in the concurrency test: pause after candidate read, change a user-role relation in the other transaction, then assert the first transaction detects the set mismatch rather than committing an unversioned permission fact.

- [ ] **Step 2: Run role tests and confirm current direct writes**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/role -run "TestService.*Status|TestService.*Delete|TestService.*Permissions|TestRepository.*Access|TestConcurrent.*Candidate" -count=1
```

Expected: FAIL because role writes currently have no candidate access invalidation or version increment.

- [ ] **Step 3: Add concrete affected-user repository operations**

```go
func (r *Repository) FindEffectiveAccessVersionsByRole(ctx context.Context, roleID int64) ([]accessstate.Version, error)
func (r *Repository) LockEffectiveAccessVersionsByRole(ctx context.Context, roleID int64) ([]accessstate.Version, error)
func (r *Repository) IncrementAccessVersions(ctx context.Context, userIDs []int64, now time.Time) (map[int64]int64, error)
```

Queries join active/non-deleted users, active user-role relations, and exactly one `sys_access_version` row. Both candidate and locked results use `ORDER BY app_user.id`. The locked form obtains locks in the existing role -> menu where applicable -> user ID ascending -> access version order. The bulk update uses `UPDATE ... SET version=version+1, updated_at=? WHERE user_id IN ? RETURNING user_id, version` and rejects missing/duplicate rows.

- [ ] **Step 4: Wrap only authorization-changing methods**

Inject `*accessstate.Invalidator` into `role.Service`. For `UpdateStatus`, `Delete`, and `UpdatePermissions`:

```text
read candidate user/version pairs outside transaction
acquire the Access State lease for all candidates
start renewal context
run the existing transaction
requery and lock actual affected user/version pairs
require exact equality with candidates
perform original mutation
increment versions in the same transaction
commit and publish versions
on candidate mismatch, rollback/release and retry at most 3 times
```

No-op status/permission requests still validate stored state but then roll back/release without increment and return success. `Create`, name-only `Update`, and `SetDefault` keep their current linear path because they do not change effective permissions.

- [ ] **Step 5: Rerun role tests**

Run the Step 2 command. Expected: PASS.

### Task 11: Menu Global Mutation Locking and Builtin Authentication Platform Permissions

**Files:**
- Modify: `server/internal/module/menu/protocol.go`
- Modify: `server/internal/module/menu/builtin.go`
- Modify: `server/internal/module/menu/builtin_test.go`
- Modify: `server/internal/module/menu/repository.go`
- Modify: `server/internal/module/menu/repository_test.go`
- Modify: `server/internal/module/menu/service.go`
- Modify: `server/internal/module/menu/service_test.go`

- [ ] **Step 1: Add failing builtin and global-version tests**

Add exact builtin definitions:

```text
system:auth-platform:list   page   /system/auth-platforms  system-auth-platforms
system:auth-platform:create action
system:auth-platform:update action
system:auth-platform:status action
system:auth-platform:delete action
```

Tests assert the Page is under `system`, actions are under the Page, codes/title/view/path match across allowlists, all are enabled/protected, and duplicate/damaged records fail startup.

Global mutation tests cover create, structural/display update, status, and delete. Each must:

```text
candidate all active user IDs sorted
invalidate all candidate Access States before PostgreSQL
lock sys_user against inserts/status writes in a documented order
requery exactly the same active IDs
advance every version in the menu transaction
publish after commit
```

Concurrency test: pause menu mutation after candidate acquisition, attempt user creation/status mutation from a second connection, and prove the table lock prevents it from slipping between recheck and commit.

- [ ] **Step 2: Run menu tests and confirm missing global invalidation**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/menu -count=1
```

Expected: FAIL because auth-platform builtins and global access version handling are absent.

- [ ] **Step 3: Add protocol constants and all aligned allowlists**

```go
const (
	PermissionAuthPlatformList   = "system:auth-platform:list"
	PermissionAuthPlatformCreate = "system:auth-platform:create"
	PermissionAuthPlatformUpdate = "system:auth-platform:update"
	PermissionAuthPlatformStatus = "system:auth-platform:status"
	PermissionAuthPlatformDelete = "system:auth-platform:delete"
)
```

Use title keys `navigation.systemAuthPlatforms` plus `permission.authPlatformCreate`, `permission.authPlatformUpdate`, `permission.authPlatformStatus`, and `permission.authPlatformDelete`. Add `system-auth-platforms` to the view-key allowlist and use an existing concrete Element Plus icon key.

- [ ] **Step 4: Implement global affected-user operations with fixed locks**

Add:

```go
func (r *Repository) FindActiveAccessVersions(ctx context.Context) ([]accessstate.Version, error)
func (r *Repository) LockUserMutationTables(ctx context.Context) error
func (r *Repository) LockActiveAccessVersions(ctx context.Context) ([]accessstate.Version, error)
func (r *Repository) IncrementAccessVersions(ctx context.Context, userIDs []int64, now time.Time) (map[int64]int64, error)
```

`LockUserMutationTables` uses one explicit statement:

```sql
LOCK TABLE sys_user IN SHARE ROW EXCLUSIVE MODE;
```

Document and test lock order as menu rows -> `sys_user` table -> active user rows ascending -> access version rows ascending. Task 3 user creation and Task 9 username/role/status/delete writes explicitly acquire `ROW EXCLUSIVE` on `sys_user` before any user row lock; the menu mutation acquires `SHARE ROW EXCLUSIVE` before any user row lock. The incompatible table modes serialize at the table boundary, so neither side waits for the other's row while holding the opposite lock.

Inject `*accessstate.Invalidator` into `menu.Service` and wrap all four mutations with the same three-attempt candidate user/version recheck sequence from Task 10. Even a new disabled menu advances versions because the assignable permission tree changed; an exact no-op Update or Status does not.

- [ ] **Step 5: Rerun menu tests**

Run the Step 2 command. Expected: PASS.

### Task 12: Platform Mutation Effects, Session Limits, and Policy Cache Publishing

**Files:**
- Modify: `server/internal/module/authplatform/repository.go`
- Modify: `server/internal/module/authplatform/repository_test.go`
- Modify: `server/internal/module/authplatform/redis.go`
- Modify: `server/internal/module/authplatform/redis_test.go`
- Modify: `server/internal/module/authplatform/service.go`
- Modify: `server/internal/module/authplatform/service_test.go`
- Modify: `server/internal/module/authplatform/handler_test.go`

- [ ] **Step 1: Add failing mutation-effect tests**

Cover every row from spec section 6.6:

```text
name-only update increments policy version and rebuilds Policy but revokes no sessions
Access/Refresh TTL affects only future issued tokens
cache TTL change makes old policyVersion snapshot keys unreachable
enable bindDevice/bindIP applies on next request using stored values
lower maxSessions revokes oldest excess session per user on this platform
raise/unlimit does not restore revoked sessions
disable/delete revokes every active session on this platform
re-enable does not restore any session
admin code cannot change through DTO and admin cannot delete
every real mutation increments policyVersion exactly once
Redis invalidation failure -> zero PostgreSQL transaction calls
PostgreSQL rollback -> prior ready Policy/Session states restored
publish failure after commit -> 10006 with old generations unreachable
```

Use real PostgreSQL test data with at least two users, three sessions each on `admin`, and one session each on `app` to prove exact scoping.

- [ ] **Step 2: Run authplatform tests and confirm effects are incomplete**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/authplatform -count=1
```

Expected: FAIL because Task 4 provided the HTTP/data boundary but not full session mutation effects.

- [ ] **Step 3: Add platform-scoped repository transaction methods**

```go
func (r *Repository) LockActiveSessionUsers(ctx context.Context, platform string) ([]int64, error)
func (r *Repository) RevokePlatformSessions(ctx context.Context, platform string, now time.Time) ([]SessionRef, error)
func (r *Repository) EnforcePlatformLimit(ctx context.Context, platform string, maxSessions int16, now time.Time) ([]SessionRef, error)
```

Lock user IDs ascending, then sessions by `user_id`, `created_at DESC`, `id DESC`. For each user, keep the newest N when N > 0 and revoke the rest. `maxSessions=0` makes no changes. Returned refs contain only `UserID` and `SessionID`, allowing exact Session State generations/snapshot deletion without exposing refresh hashes.

- [ ] **Step 4: Implement full Policy + Session invalidation order**

For update/status/delete:

```text
load current authoritative platform
begin Policy invalidation
if session facts change, read affected user IDs and acquire their `authstate` platform Session State leases in ascending order
start all lease renewals
transaction locks platform, revalidates request, increments policyVersion, applies row mutation, enforces/revokes sessions
commit
publish new/deleted Policy state
publish each new Sessions generation and delete revoked Session snapshots
```

`authplatform.Service` depends on `authstate.Store` for generation changes and on a narrow session-snapshot deletion function supplied at assembly; it never imports package `auth`, which already imports `authplatform`. Create first verifies code history, then uses `SET NX` to establish an `auth:policy:<newCode>` invalidating lease before inserting `policyVersion=1`; a Redis failure or an existing mutation state prevents the PostgreSQL transaction. After commit it publishes ready Policy with the same token. If publish fails after commit, return `10006`; after lease expiry the next public/management read rebuilds from PostgreSQL.

- [ ] **Step 5: Rerun authplatform tests**

Run the Step 2 command. Expected: PASS.

### Task 13: Backend Assembly, Startup Cleanup, and Cross-Module Failure Verification

**Files:**
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`
- Modify: `server/internal/module/auth/route.go`
- Modify: `server/internal/module/access/route.go`
- Modify: `server/internal/module/menu/route.go`
- Modify: `server/internal/module/role/route.go`
- Modify: `server/internal/module/user/route.go`
- Modify: `server/internal/module/health/service_test.go`

- [ ] **Step 1: Add failing assembly and route tests**

Update the route inventory to require each route exactly once, including Policy and all auth-platform management routes. Capture permission middleware input and assert the exact mapping:

```go
wantPermissions := map[string]string{
	"GET /api/v1/auth-platforms": "system:auth-platform:list",
	"GET /api/v1/auth-platforms/deployment": "system:auth-platform:list",
	"POST /api/v1/auth-platforms": "system:auth-platform:create",
	"PUT /api/v1/auth-platforms/:id": "system:auth-platform:update",
	"PATCH /api/v1/auth-platforms/:id/status": "system:auth-platform:status",
	"DELETE /api/v1/auth-platforms/:id": "system:auth-platform:delete",
}
```

Integration assembly test boots the real repositories/services against configured PostgreSQL and Redis, calls Policy, Login, `/me`, `/access`, and one permission-protected route, then captures SQL query counts to prove warm authentication and warm access checks issue zero PostgreSQL queries.

Route middleware-order tests must distinguish client metadata validation from authentication:

```text
GET /api/v1/auth/policy without either client Header -> HTTP 400, code 10001
GET /api/v1/access without either client Header -> HTTP 400, code 10001
GET /api/v1/access with one canonical platform Header and one canonical Device ID Header but no Bearer Token -> HTTP 401, code 10002
GET /health and GET /ready without client Headers -> retain their existing responses
```

The default `zh-CN` error bodies are exactly:

```json
{"code":10001,"data":null,"message":"请求参数错误"}
{"code":10002,"data":null,"message":"未登录或登录已失效"}
```

Each case asserts the exact `code`, `data`, and `message` envelope and proves the downstream Policy/auth service call count is zero when the earlier middleware rejects the request. This locks `authclient.Require()` before public Policy and authentication, while preserving unauthenticated probes outside `/api/v1`.

Add failure cases:

```text
Redis flushed -> reads rebuild from PostgreSQL
Redis connection forced to error -> Policy/auth/access ordinary reads use PostgreSQL authority
PostgreSQL forced to error after cache invalidation/miss -> request returns 10006, never stale success
Redis mutation operations forced to fail -> platform/user/role/menu/session PostgreSQL mutation call count is zero
```

- [ ] **Step 2: Run assembly tests and confirm dependencies are not yet wired**

Run:

```powershell
cd D:\admin\server
go test ./cmd/api -count=1
```

Expected: FAIL because router dependencies, startup schema/cleanup, and concrete services do not yet include the new modules.

- [ ] **Step 3: Wire concrete modules without a container or registry**

Construct in a readable linear order:

```text
PostgreSQL and Redis clients
schema/migration/init
secret keys and queue
authstate Store and leases; accessstate Store and invalidator
auth SessionCache; authplatform repository + Policy Redis + service(authstate, SessionCache deleter)
auth session repository + service(authstate, SessionCache)
access repository + Snapshot Redis + service(accessstate Store)
menu repository + service(accessstate invalidator)
role repository + service(accessstate invalidator)
user repository + service(authstate leases, accessstate invalidator)
handlers and middleware
router
```

The `authstate` and `accessstate` packages know only their concrete Redis state protocols and import no business module. This prevents `auth -> user/role`, `access -> auth`, and mutation-service dependencies from forming Go import cycles. Complete Session Snapshots remain in `auth`; complete Access Snapshots remain in `access`. Do not introduce a runtime registry or generic interface package.

Before serving traffic, perform:

```go
if err := redisClient.ScanDelete(processContext, "auth:current-session:*"); err != nil {
	return fmt.Errorf("remove legacy current session keys: %w", err)
}
```

This explicit one-time cleanup is safe after the PostgreSQL migration revoked all old tokens. It must not scan or delete new snapshot prefixes.

- [ ] **Step 4: Preserve strict readiness and route middleware order**

Keep `/health` and `/ready` outside `/api/v1`. Keep readiness dependent on both PostgreSQL and Redis. Inside `/api/v1`, order middleware and routes so client metadata is present before auth middleware, auth identity is present before access middleware, and handlers are last.

- [ ] **Step 5: Rerun assembly tests**

Run the Step 2 command. Expected: PASS.

### Task 14: Frontend Platform, Device ID, Request Headers, and Public Policy

**Files:**
- Create: `web/src/auth/platform.ts`
- Create: `web/src/auth/platform.test.ts`
- Create: `web/src/auth/device-id.ts`
- Create: `web/src/auth/device-id.test.ts`
- Modify: `web/src/utils/request.ts`
- Modify: `web/src/utils/request.test.ts`
- Modify: `web/src/api/auth.contract.ts`
- Modify: `web/src/api/auth.contract.test.ts`
- Modify: `web/src/api/auth.ts`
- Modify: `web/src/api/auth.test.ts`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/router/index.test.ts`
- Modify: `web/src/permission.ts`
- Modify: `web/src/views/auth/login/index.vue`
- Modify: `web/src/views/auth/login/index.test.ts`
- Modify: `web/src/views/auth/register/index.vue`
- Modify: `web/src/views/auth/register/index.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

- [ ] **Step 1: Add failing UUID, request, Policy, and route tests**

Device tests cover first generation, reuse, corrupt storage regeneration, canonical lower-case UUID, storage write failure, missing `crypto.randomUUID`, and invalid generated result. Do not accept a timestamp/random fallback.

Request tests capture headers for:

```text
normal public Policy
login and register
raw refresh
authenticated request
retried request after refresh
logout
```

Every request must have exactly:

```ts
expect(headers.get('X-Auth-Platform')).toBe('admin')
expect(headers.get('X-Device-ID')).toBe('550e8400-e29b-41d4-a716-446655440000')
```

With a non-empty Access Token in the Auth Store, the public Policy request must omit `Authorization`. If Policy returns HTTP 401, the request rejects that exact error without calling `/api/v1/auth/refresh`, changing the Auth Store credential, or issuing a retry. This proves Policy remains a pre-login request even when an old authenticated state is still in memory.

Policy parser accepts only `{ code, name, allowRegister }`, requires `code === 'admin'`, trimmed non-empty name, and project `YesNo`. Login tests prove a Policy failure shows an explicit error and does not masquerade as registration disabled. Register navigation tests prove `allowRegister=0` redirects to Login and `allowRegister=1` allows the form.

- [ ] **Step 2: Run targeted frontend tests and confirm missing behavior**

Run:

```powershell
cd D:\admin\web
pnpm vitest run src/auth src/utils/request.test.ts src/api/auth.contract.test.ts src/api/auth.test.ts src/router/index.test.ts src/views/auth/login/index.test.ts src/views/auth/register/index.test.ts
```

Expected: FAIL because platform/device modules and public Policy do not exist, and Register is not currently a route.

- [ ] **Step 3: Implement concrete platform and strict Device ID modules**

```ts
export const authPlatform = 'admin' as const
export const deviceIDStorageKey = 'admin:device-id'

const canonicalUUIDPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

export function readDeviceID(storage: Storage = window.localStorage): string {
  const stored = storage.getItem(deviceIDStorageKey)
  if (stored !== null && canonicalUUIDPattern.test(stored)) return stored
  if (typeof crypto.randomUUID !== 'function') {
    throw new ProtocolError('crypto.randomUUID is required')
  }
  const generated = crypto.randomUUID().toLowerCase()
  if (!canonicalUUIDPattern.test(generated)) {
    throw new ProtocolError('crypto.randomUUID returned an invalid UUID')
  }
  storage.setItem(deviceIDStorageKey, generated)
  return generated
}
```

Storage access errors propagate as explicit bootstrap failures.

- [ ] **Step 4: Attach headers in the shared request interceptor**

Replace locale-only interception with one function used by both `rawClient` and `client`:

```ts
function applyClientHeaders(config: InternalAxiosRequestConfig): InternalAxiosRequestConfig {
  config.headers = AxiosHeaders.from(config.headers)
  config.headers.set('Accept-Language', readLocale())
  config.headers.set('X-Auth-Platform', authPlatform)
  config.headers.set('X-Device-ID', readDeviceID())
  return config
}
```

Do not cache Device ID in Pinia. Reading storage on each request makes storage corruption/replacement explicit while still preserving the stable persisted value.

Keep the anonymous-path sets explicit and add Policy to both:

```ts
const noBearerPaths = new Set([
  '/api/v1/auth/policy',
  '/api/v1/auth/login',
  '/api/v1/auth/register',
  '/api/v1/auth/refresh',
])
const noRefreshPaths = new Set([...noBearerPaths, '/api/v1/auth/logout'])
```

Policy must use the normal envelope/error interceptor so its failure is displayed, but it must never send a Bearer Token or trigger Refresh.

- [ ] **Step 5: Add strict Policy API and registration routing**

```ts
export interface AuthPolicy {
  code: typeof authPlatform
  name: string
  allowRegister: YesNo
}

export async function getAuthPolicy(): Promise<AuthPolicy> {
  const data = await request<unknown>({ method: 'GET', url: '/api/v1/auth/policy' })
  return parseAuthPolicy(data)
}
```

Restore `/register` as a public static route. In the permission guard, when navigating to Register, await Policy; redirect to Login when disabled and propagate/store a clear service error when Policy fails. The Login view also loads Policy on mount, displays the error, and shows the registration link only for `YesNo.Yes`. The Register view calls the same API before showing its form and moves all existing hard-coded strings into both i18n catalogs.

- [ ] **Step 6: Rerun targeted frontend tests**

Run the Step 2 command. Expected: PASS.

### Task 15: Frontend Authentication Platform API and Management Page

**Files:**
- Create: `web/src/api/auth-platform.contract.ts`
- Create: `web/src/api/auth-platform.contract.test.ts`
- Create: `web/src/api/auth-platform.ts`
- Create: `web/src/api/auth-platform.test.ts`
- Create: `web/src/views/system/auth-platforms/index.vue`
- Create: `web/src/views/system/auth-platforms/index.test.ts`
- Modify: `web/src/access/route-views.ts`
- Modify: `web/src/access/route-views.test.ts`
- Modify: `web/src/access/menu-title-keys.ts`
- Modify: `web/src/access/menu-title-keys.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Modify: `web/src/i18n/index.test.ts`

- [ ] **Step 1: Add failing strict contract and API tests**

Define and test these exact DTOs:

```ts
export interface AuthPlatformListItem {
  id: number
  code: string
  name: string
  policyVersion: number
  accessTTLSeconds: number
  refreshTTLSeconds: number
  sessionCacheTTLSeconds: number
  accessCacheTTLSeconds: number
  bindDevice: YesNo
  bindIP: YesNo
  maxSessions: number
  allowRegister: YesNo
  isEnabled: YesNo
  isBuiltin: YesNo
  createdAt: string
  updatedAt: string
}

export interface AuthPlatformDeployment {
  cookieSecure: boolean
  corsOrigin: string
  trustedProxyMode: 'none' | 'allowlist'
  trustedProxyCount: number
  redisStatus: 'up' | 'down'
}
```

Parsers reject extra/missing keys, duplicate IDs/codes, invalid Yes/No, invalid platform code, out-of-range TTL/session values, invalid RFC3339 timestamps, unstable list order, bad page totals, and secrets/deployment fields in platform rows.

API tests assert exact methods, paths, query fields, request body allowlists, and strict parsed responses for list/create/update/status/delete/deployment.

- [ ] **Step 2: Add failing page behavior tests**

Mount with permission sets and assert:

```text
list permission permits page load and deployment status
create/update/status/delete controls are each independently permission-gated
search/reset/refresh and pagination issue exact queries
admin row has builtin tag and no enabled delete command
edit code is read-only; create code is editable
TTL values show seconds plus readable minute/hour/day labels
maxSessions shows unlimited/single/max N
bindDevice/bindIP/allowRegister/isEnabled use switches or selects
lowering maxSessions shows oldest-session revocation confirmation
enabling binding or changing security TTL shows explicit impact confirmation
disable/delete shows all-user logout confirmation
save failure keeps dialog and values; success closes and reloads current page
dialog body scrolls independently and Main remains the page scroll owner
```

- [ ] **Step 3: Run targeted page tests and confirm files are absent**

Run:

```powershell
cd D:\admin\web
pnpm vitest run src/api/auth-platform.contract.test.ts src/api/auth-platform.test.ts src/views/system/auth-platforms/index.test.ts src/access/route-views.test.ts src/access/menu-title-keys.test.ts src/i18n/index.test.ts
```

Expected: FAIL because the new contracts, API, page, route view, and strings do not exist.

- [ ] **Step 4: Implement strict contracts and exact API functions**

Export explicitly typed methods:

```ts
export async function getAuthPlatforms(query: AuthPlatformListQuery): Promise<PageResult<AuthPlatformListItem>>
export async function getAuthPlatformDeployment(): Promise<AuthPlatformDeployment>
export async function createAuthPlatform(input: CreateAuthPlatformInput): Promise<{ id: number }>
export async function updateAuthPlatform(id: number, input: UpdateAuthPlatformInput): Promise<Record<string, never>>
export async function updateAuthPlatformStatus(id: number, isEnabled: YesNo): Promise<AuthPlatformStatusResult>
export async function deleteAuthPlatform(id: number): Promise<Record<string, never>>
```

`UpdateAuthPlatformInput` has no `code`, `isBuiltin`, or `policyVersion`. All API return values pass a strict parser; no view casts response data.

- [ ] **Step 5: Implement the Element Plus page without global abstractions**

Follow the existing roles/users page conventions: local refs, typed form state, permission computed values, explicit load/mutation errors, Element Plus icons, table/pagination, and a local `el-dialog`. Use `max-height: min(68vh, 680px); overflow-y: auto` on the dialog body wrapper. Do not create AppDialog, AppTable, CRUD composables, nested cards, or a second page-level scroll owner.

The page uses these codes verbatim:

```ts
system:auth-platform:create
system:auth-platform:update
system:auth-platform:status
system:auth-platform:delete
```

Register `'system-auth-platforms'` in `routeViews`, add all backend title keys to `menuTitleKeys`, and add paired Chinese/English page strings. The i18n key-set test remains exact.

- [ ] **Step 6: Rerun targeted frontend tests**

Run the Step 3 command. Expected: PASS.

### Task 16: Full Verification, Race Recovery, and Spec Coverage Audit

**Files:**
- Modify as required by failures: only files already named in Tasks 1 through 15
- Verify: `docs/superpowers/specs/2026-08-20-authentication-rbac-redis-hardening-design.md`
- Verify: `docs/superpowers/plans/2026-08-20-authentication-rbac-redis-hardening.md`

- [ ] **Step 1: Run all backend verification**

Run:

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

Expected: all commands exit 0. Normal `go test ./...` must exercise the configured real PostgreSQL and Redis tests; do not replace it with `-short` for final verification.

- [ ] **Step 2: Run race-sensitive modules**

Run:

```powershell
cd D:\admin\server
go test -race ./internal/module/auth ./internal/module/authplatform ./internal/module/access ./internal/module/user ./internal/module/role ./internal/module/menu -count=1
```

Expected: PASS with no Go race reports and no leaked renewal goroutines.

- [ ] **Step 3: Run all frontend verification**

Run:

```powershell
cd D:\admin\web
pnpm vitest run
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src -g "*.ts" -g "*.vue"
```

Expected: Vitest and build exit 0; `rg` prints no matches and exits 1 because the forbidden patterns are absent.

- [ ] **Step 4: Run Redis recovery and consistency scenarios against the assembled service layer**

Use the integration tests from Tasks 6 through 13 to explicitly rerun:

```powershell
cd D:\admin\server
go test ./internal/module/auth ./internal/module/authplatform ./internal/module/access ./internal/module/user ./internal/module/role ./internal/module/menu ./cmd/api -run "Test.*Redis|Test.*Cache|Test.*Invalidat|Test.*Publish|Test.*Concurrent|Test.*Recovery|Test.*Warm" -count=1
```

Expected: PASS, including these assertions in output/test names:

```text
warm auth and RBAC hits use zero PostgreSQL queries
read miss/error/corruption uses PostgreSQL authority
PostgreSQL failure does not accept old snapshots
Redis coordination failure prevents PostgreSQL mutation
post-commit publish failure leaves old snapshots unreachable
lease expiry rebuilds committed truth
permission change rejects old permission on next request
```

- [ ] **Step 5: Audit forbidden architecture and legacy behavior**

Run:

```powershell
cd D:\admin
rg -n "auth:current-session|CreateReplacingActive|FindCurrentByUser|HasPermission\(" server web -g "*.go" -g "*.ts" -g "*.vue"
rg -n "PlatformAdapter|BaseService|BaseRepository|CacheManager|ServiceManager|DIContainer|runtime registry|gorm\.Model|is_del|is_deleted" server web -g "*.go" -g "*.ts" -g "*.vue"
```

Expected: no runtime-code matches. A migration cleanup pattern or plan/spec prose is allowed only outside the searched runtime directories. Concrete business name `authplatform` is expected and is not an Adapter.

- [ ] **Step 6: Review every spec section against the coverage matrix**

Check all 20 acceptance criteria in spec section 18 and record any failed criterion as an implementation failure, not a future task. Confirm technical debt items in section 16 remain absent: no login-mode field, captcha field, email auto-registration, session-management UI, operation-log persistence, or process-level cache.

- [ ] **Step 7: Inspect final changes without committing**

Run:

```powershell
cd D:\admin
git status --short
git diff --check
git diff --stat
```

Expected: only intended implementation/spec/plan files are changed; `git diff --check` exits 0. Do not stage, commit, fetch, pull, or push.

---

## Spec Coverage Matrix

| Spec requirement | Implemented by |
| --- | --- |
| Linear auth/access/frontend data flows and no generic architecture | Tasks 4-15; Execution Rules; Task 16 architecture scan |
| `sys_auth_platform`, builtin `admin`, checks, timestamps, soft delete | Tasks 3-4 |
| Session platform/device columns, safe old-session revocation, new index | Task 3 |
| `sys_access_version` and user backfill | Tasks 3 and 9 |
| Exact platform/device headers and trusted client IP | Tasks 1, 4, and 5 |
| JWT platform claim, dynamic TTL, isolated Cookie | Task 5 |
| Device/IP binding and per-platform 0/1/N session limits | Tasks 6 and 12 |
| Registration policy and explicit future email-login debt | Tasks 6 and 14; plan scope exclusions |
| Policy/User/Session/Access Redis key formats and closed snapshots | Tasks 4, 6, 7, and 8 |
| Redis hit 0 PostgreSQL; miss/error/corruption authoritative fallback | Tasks 4, 6, 8, 13, and 16 |
| Leased invalidation, token compare, rollback/publish/crash recovery | Tasks 2, 6, 7, and 12 |
| Exact RBAC affected-user version scopes | Tasks 9-11 |
| Candidate recheck, stable ordering, menu global locking | Tasks 10-11 |
| Shared `/access` and permission middleware fact | Task 8 |
| Platform management endpoints, codes `17000-17006`, bilingual errors | Tasks 4 and 8 |
| Builtin menu and precise route permissions | Tasks 11 and 13 |
| Deployment status read-only and secrets excluded | Tasks 1, 4, and 15 |
| Frontend strict headers, Device ID, Policy/registration behavior | Task 14 |
| Strict DTOs and Element Plus auth-platform page | Task 15 |
| Logging safety and readiness | Tasks 5 and 13 |
| Real PostgreSQL/Redis, backend/frontend full verification | Tasks 3-16, especially Task 16 |

## Completion Boundary

The feature is complete only when Task 16 passes in full. The next project phase is the separately planned UI/format and global-component debt recovery; do not mix that work into this implementation.
