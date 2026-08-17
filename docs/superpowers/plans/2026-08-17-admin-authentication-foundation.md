# Admin Authentication Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first production-shaped Admin authentication slice: email registration, username/password login, rotating tokens, durable single sessions, current-user bootstrap, protected routing, and the Element Plus admin shell.

**Architecture:** Keep each request linear: Gin route/middleware -> handler -> auth service -> focused user/role/session repositories -> PostgreSQL, with Redis holding only the current-session pointer. The API loads one `APP_SECRET` and derives purpose-separated JWT and Refresh HMAC keys with HKDF; bcrypt password hashes do not use this root key. The frontend keeps the Access Token in Pinia memory, validates every auth DTO, performs one coordinated refresh, and renders protected content through one `layout/index.vue` shell.

**Tech Stack:** Go 1.26, Gin, GORM, PostgreSQL, Redis, bcrypt, `golang-jwt/jwt/v5`, Vue 3, TypeScript 6, Pinia, Vue Router, Axios, Element Plus, Vitest.

---

## Execution Rules

- Read `AGENTS.md`, `docs/agent/README.md`, and `docs/superpowers/specs/2026-08-17-admin-authentication-foundation-design.md` before starting.
- Use TDD for every behavior: add one focused failing test, confirm the intended failure, add the minimum implementation, and rerun the focused test.
- Do not use Docker, SQLite, `any`, compatibility response fields, silent fallbacks, or a second layout entry.
- Do not overwrite the user's untracked `web/src/layout/index.vue` without reading it immediately before the layout task.
- Git history belongs to the repository owner. This plan uses diff checkpoints instead of automatic commit commands. Only commit when the owner explicitly authorizes it.
- If execution is isolated, invoke `superpowers:using-git-worktrees` first and preserve the user's untracked layout file deliberately.

## File Map

### Backend files to create

```text
server/internal/module/user/model.go
server/internal/module/user/repository.go
server/internal/module/user/repository_test.go
server/internal/module/role/model.go
server/internal/module/role/repository.go
server/internal/module/role/repository_test.go
server/internal/module/auth/session_model.go
server/internal/module/auth/schema.go
server/internal/module/auth/session_repository.go
server/internal/module/auth/session_repository_test.go
server/internal/module/auth/password.go
server/internal/module/auth/password_test.go
server/internal/module/auth/token.go
server/internal/module/auth/token_test.go
server/internal/secretkey/secretkey.go
server/internal/secretkey/secretkey_test.go
server/internal/module/auth/request.go
server/internal/module/auth/response.go
server/internal/module/auth/service.go
server/internal/module/auth/service_test.go
server/internal/module/auth/bootstrap_service.go
server/internal/module/auth/bootstrap_service_test.go
server/internal/module/auth/middleware.go
server/internal/module/auth/middleware_test.go
server/internal/module/auth/handler.go
server/internal/module/auth/handler_test.go
server/internal/module/auth/route.go
server/cmd/bootstrap-admin/main.go
server/cmd/bootstrap-admin/main_test.go
```

### Backend files to modify

```text
server/internal/config/config.go
server/internal/config/config_test.go
server/internal/redis/redis.go
server/internal/redis/redis_test.go
server/internal/shared/validate/validate.go
server/internal/shared/validate/validate_test.go
server/cmd/api/main.go
server/cmd/api/main_test.go
server/.env.example
server/go.mod
server/go.sum
```

### Frontend files to create

```text
web/src/api/auth.contract.ts
web/src/api/auth.contract.test.ts
web/src/api/auth.ts
web/src/api/auth.test.ts
web/src/store/auth.ts
web/src/store/auth.test.ts
web/src/views/auth/login/index.vue
web/src/views/auth/login/index.test.ts
web/src/views/auth/register/index.vue
web/src/views/auth/register/index.test.ts
web/src/layout/components/AppAside.vue
web/src/layout/components/AppHeader.vue
web/src/layout/components/AppFooter.vue
web/src/layout/index.test.ts
```

### Frontend files to modify or remove

```text
web/src/utils/request.ts
web/src/utils/request.test.ts
web/src/router/index.ts
web/src/router/index.test.ts
web/src/permission.ts
web/src/main.ts
web/src/layout/index.vue
web/src/styles/index.scss
web/src/styles/variables.scss
web/src/types/http.ts
web/src/views/dashboard/index.vue
web/src/views/dashboard/index.test.ts
delete web/src/layout/AdminLayout.vue
```

## Task 1: Add one application root secret and purpose-separated Auth keys

**Files:**
- Modify: `server/internal/config/config.go`
- Modify: `server/internal/config/config_test.go`
- Create: `server/internal/secretkey/secretkey.go`
- Create: `server/internal/secretkey/secretkey_test.go`
- Modify: `server/.env.example`

- [ ] **Step 1: Write failing configuration tests**

Extend `validAPIValues()` and add table cases that require the one root secret, reject secrets shorter than 64 ASCII characters and the documented placeholder, reject `AUTH_COOKIE_SECURE` values other than `0` or `1`, and reject HTTP/Secure mismatches:

```go
func validAPIValues() map[string]string {
	return map[string]string{
		"HTTP_ADDR":         ":16301",
		"POSTGRES_DSN":      validPostgresDSN,
		"REDIS_URL":         validRedisURL,
		"CORS_ORIGIN":       "http://localhost:16300",
		"APP_SECRET":        strings.Repeat("s", 64),
		"AUTH_COOKIE_SECURE": "0",
	}
}

func TestLoadAPIRejectsInvalidAuthConfiguration(t *testing.T) {
	tests := []struct{ name, key, value string }{
		{name: "short application secret", key: "APP_SECRET", value: "short"},
		{name: "non-ASCII application secret", key: "APP_SECRET", value: strings.Repeat("界", 64)},
		{name: "placeholder application secret", key: "APP_SECRET", value: "replace_with_at_least_64_random_characters_before_running_api_server"},
		{name: "invalid cookie secure", key: "AUTH_COOKIE_SECURE", value: "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := validAPIValues()
			values[tt.key] = tt.value
			_, err := LoadAPI(lookup(values))
			assertErrorContains(t, err, tt.key)
		})
	}
}
```

- [ ] **Step 2: Run the focused config test and confirm failure**

Run:

```powershell
cd D:\admin\server
go test ./internal/config -run 'TestLoadAPI|TestLoadAPIRejectsInvalidAuthConfiguration' -count=1
```

Expected: FAIL because `LoadAPI` does not require or expose the application secret or Auth Cookie configuration.

- [ ] **Step 3: Write failing key-derivation tests**

Create focused tests proving stable derivation, purpose separation, exact key length, unsafe-root rejection, and copy-returning accessors:

```go
func TestNewDerivesStableSeparatedKeys(t *testing.T) {
	first, err := New(strings.Repeat("s", 64))
	if err != nil { t.Fatal(err) }
	second, err := New(strings.Repeat("s", 64))
	if err != nil { t.Fatal(err) }

	if len(first.JWTSigningKey()) != 32 || len(first.RefreshTokenHMACKey()) != 32 {
		t.Fatal("derived keys must be 32 bytes")
	}
	if !bytes.Equal(first.JWTSigningKey(), second.JWTSigningKey()) {
		t.Fatal("JWT derivation is not stable")
	}
	if bytes.Equal(first.JWTSigningKey(), first.RefreshTokenHMACKey()) {
		t.Fatal("different purposes produced the same key")
	}
}

func TestNewRejectsUnsafeRootSecrets(t *testing.T)
func TestAccessorsReturnKeyCopies(t *testing.T)
```

Run:

```powershell
go test ./internal/secretkey -count=1
```

Expected: FAIL because the package does not exist.

- [ ] **Step 4: Implement the exact config model and validation**

Add the root secret at API scope because later concrete features may use it outside Auth:

```go
type Auth struct {
	CookieSecure bool
}

type API struct {
	HTTPAddr    string
	PostgresDSN string
	RedisURL    string
	CORSOrigin  string
	AppSecret   string
	Auth        Auth
}
```

Add `loadAppSecret(lookupEnv)` that requires `APP_SECRET`, accepts ASCII only, and rejects fewer than 64 characters and the documented placeholder. Add `loadAuth(lookupEnv, corsOrigin)` that requires `AUTH_COOKIE_SECURE`, parses only `0/1`, and requires `0` for HTTP or `1` for HTTPS. Keep Worker configuration unchanged because it does not sign or validate Auth tokens.

- [ ] **Step 5: Implement the concrete HKDF key derivation**

Create `internal/secretkey` with only the two keys used by this slice:

```go
const keyLength = 32

type KeyRing struct {
	jwtSigningKey       []byte
	refreshTokenHMACKey []byte
}

func New(rootSecret string) (*KeyRing, error)
func (k *KeyRing) JWTSigningKey() []byte
func (k *KeyRing) RefreshTokenHMACKey() []byte
```

`New` applies the same ASCII and 64-character validation, then uses `crypto/hkdf` with SHA-256, nil salt, and these exact versioned info values:

```text
admin:auth:jwt-signing:v1
admin:auth:refresh-token-hmac:v1
```

Return copies from both accessors. Do not expose the root secret, a generic public `Derive` method, a future credential-encryption key, or rotation support in this slice.

- [ ] **Step 6: Document the new environment keys**

Append comments and placeholders to `server/.env.example`:

```dotenv
# 应用唯一根密钥；至少 64 个随机 ASCII 字符，JWT 和 Refresh 子密钥由它派生
APP_SECRET=replace_with_at_least_64_random_characters_before_running_api_server

# Refresh Cookie 是否仅通过 HTTPS 发送；本地 HTTP 使用 0，HTTPS 使用 1
AUTH_COOKIE_SECURE=0
```

- [ ] **Step 7: Verify and inspect the diff**

Run:

```powershell
go fmt ./internal/config ./internal/secretkey
go test ./internal/config ./internal/secretkey -count=1
git diff --check
git diff -- server/internal/config server/internal/secretkey server/.env.example
```

Expected: config and key-derivation tests PASS; Worker tests still prove it does not read the root secret or Auth settings. Do not commit automatically.

## Task 2: Add the minimal Redis pointer operations and strict empty-body validation

**Files:**
- Modify: `server/internal/redis/redis.go`
- Modify: `server/internal/redis/redis_test.go`
- Modify: `server/internal/shared/validate/validate.go`
- Modify: `server/internal/shared/validate/validate_test.go`

- [ ] **Step 1: Write failing Redis integration tests**

Using `LoadWorker` and the existing local `.env`, test a unique key without logging its value:

```go
func openIntegrationClient(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() { t.Skip("Redis integration test") }
	if err := godotenv.Load("../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil { t.Fatalf("load worker config: %v", err) }
	client, err := projectredis.Open(context.Background(), settings.RedisURL)
	if err != nil { t.Fatalf("open Redis: %v", err) }
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestStringOperationsDistinguishMissingKeys(t *testing.T) {
	client := openIntegrationClient(t)
	ctx := context.Background()
	key := "test:redis:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })

	if value, found, err := client.GetString(ctx, key); err != nil || found || value != "" {
		t.Fatalf("missing GetString() = %q,%v,%v", value, found, err)
	}
	if err := client.SetString(ctx, key, "42", time.Minute); err != nil {
		t.Fatalf("SetString() error = %v", err)
	}
	if value, found, err := client.GetString(ctx, key); err != nil || !found || value != "42" {
		t.Fatalf("stored GetString() = %q,%v,%v", value, found, err)
	}
	if err := client.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
```

- [ ] **Step 2: Write failing empty-body tests**

Add cases proving `RequireEmptyBody` accepts no body and rejects whitespace, `{}`, and JSON:

```go
func TestRequireEmptyBody(t *testing.T) {
	for _, body := range []string{" ", "{}", `{"value":1}`} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = request
		if err := RequireEmptyBody(context); err == nil {
			t.Fatalf("body %q was accepted", body)
		}
	}
}
```

- [ ] **Step 3: Run the focused tests and confirm missing symbols**

Run:

```powershell
go test ./internal/redis ./internal/shared/validate -run 'TestStringOperations|TestRequireEmptyBody' -count=1
```

Expected: FAIL because the methods do not exist.

- [ ] **Step 4: Implement concrete operations**

Add methods to `redis.Client`:

```go
func (c *Client) GetString(ctx context.Context, key string) (string, bool, error) {
	value, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get Redis key: %w", err)
	}
	return value, true, nil
}

func (c *Client) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("set Redis key: %w", err)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete Redis key: %w", err)
	}
	return nil
}
```

Add `RequireEmptyBody` using `http.NoBody`, `ContentLength`, `TransferEncoding`, and one-byte read so whitespace is rejected instead of ignored.

- [ ] **Step 5: Verify the focused packages**

Run:

```powershell
go fmt ./internal/redis ./internal/shared/validate
go test ./internal/redis ./internal/shared/validate -count=1
git diff --check
```

Expected: PASS. Do not commit automatically.

## Task 3: Define the four PostgreSQL models and exact schema

**Files:**
- Create: `server/internal/module/user/model.go`
- Create: `server/internal/module/role/model.go`
- Create: `server/internal/module/auth/session_model.go`
- Create: `server/internal/module/auth/schema.go`
- Create: `server/internal/module/auth/schema_test.go`

- [ ] **Step 1: Write a real PostgreSQL schema test**

Open the configured PostgreSQL connection, call AutoMigrate for the four models, then query `information_schema.columns`, `pg_indexes`, and `pg_constraint`. Assert all four tables, explicit `TIMESTAMPTZ` timestamps, Yes/No checks, foreign keys, and these named indexes:

```go
func openAuthenticationSchema(t *testing.T) (*database.Connection, context.Context) {
	t.Helper()
	if testing.Short() { t.Skip("PostgreSQL integration test") }
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil { t.Fatalf("load worker config: %v", err) }
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	connection, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil { t.Fatalf("open PostgreSQL: %v", err) }
	t.Cleanup(func() { _ = connection.Close() })
	if err := database.AutoMigrate(ctx, connection.GORM,
		&user.User{}, &role.Role{}, &role.UserRole{}, &auth.Session{}); err != nil {
		t.Fatalf("AutoMigrate auth schema: %v", err)
	}
	if err := auth.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return connection, ctx
}

wantIndexes := []string{
	"ux_sys_user_username_active",
	"ux_sys_user_email_active",
	"ux_sys_role_code_active",
	"ux_sys_role_default_active",
	"ux_sys_user_role_active",
	"ux_sys_user_session_refresh_hash",
	"ux_sys_user_session_current",
}
```

The test must fail if an index is non-unique, lacks its `WHERE` clause, or the username index lacks `lower((username)::text)`.

- [ ] **Step 2: Run the schema test and confirm missing packages**

Run:

```powershell
cd D:\admin\server
go test ./internal/module/auth -run TestAuthenticationSchema -count=1
```

Expected: FAIL because the model files and tables do not exist.

- [ ] **Step 3: Add explicit models**

Use concrete fields, explicit types, and table names. The central shapes are:

```go
type User struct {
	ID           int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Username     string         `gorm:"column:username;type:varchar(64);not null"`
	Email        string         `gorm:"column:email;type:varchar(254);not null"`
	PasswordHash string         `gorm:"column:password_hash;type:varchar(255);not null"`
	IsEnabled    yesno.Value    `gorm:"column:is_enabled;type:smallint;not null;default:1;check:is_enabled IN (0,1)"`
	CreatedAt    time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}
```

`role.Role` contains code/name/default/enabled/timestamps/deleted-at. `role.UserRole` contains user/role IDs and timestamps/deleted-at. `auth.Session` contains refresh hash, version, client metadata, refresh expiry, revoked-at, and timestamps without `DeletedAt`.

Do not add `Phone`, `Avatar`, `RoleID` to User, `TokenVersion` to User, a generic `deleted_at` index, or `DeletedAt` to Session.

- [ ] **Step 4: Add exact idempotent PostgreSQL constraints and indexes**

Implement `auth.EnsureSchema(ctx, db)` with fixed SQL statements for the seven named indexes and three named `ON DELETE RESTRICT` foreign keys. Every index statement uses PostgreSQL's `CREATE UNIQUE INDEX IF NOT EXISTS <name> ON <table> ... WHERE <predicate>` form; for constraints, query `pg_constraint` by the exact constraint name before issuing `ALTER TABLE`. This function owns only the four authentication tables and is called immediately after their AutoMigrate.

- [ ] **Step 5: Verify schema behavior against PostgreSQL**

Run:

```powershell
go fmt ./internal/module/user ./internal/module/role ./internal/module/auth
go test ./internal/module/auth -run TestAuthenticationSchema -count=1
```

Expected: PASS and the catalog query confirms PostgreSQL `timestamp with time zone`.

- [ ] **Step 6: Inspect only the schema diff**

Run:

```powershell
git diff --check
git diff -- server/internal/module/user/model.go server/internal/module/role/model.go server/internal/module/auth/session_model.go
```

Do not commit automatically.

## Task 4: Implement system-role initialization and lookup

**Files:**
- Create: `server/internal/module/role/repository.go`
- Create: `server/internal/module/role/repository_test.go`

- [ ] **Step 1: Write repository tests inside rollback transactions**

Cover these exact cases:

```go
func TestEnsureSystemRolesCreatesAndValidatesRoles(t *testing.T)
func TestEnsureSystemRolesRejectsMutatedSystemRole(t *testing.T)
func TestFindDefaultRequiresExactlyOneEnabledRole(t *testing.T)
func TestFindByCodeRejectsDeletedOrDisabledRole(t *testing.T)
func TestHasActiveUserWithRole(t *testing.T)
```

Each test begins a PostgreSQL transaction, builds `role.NewRepository(tx)`, and rolls the transaction back in cleanup.

- [ ] **Step 2: Run the focused test and confirm missing repository**

```powershell
go test ./internal/module/role -count=1
```

Expected: FAIL because `NewRepository` and role methods do not exist.

- [ ] **Step 3: Implement the focused repository**

Expose only methods used by this slice:

```go
const (
	CodeSuperAdmin    = "super_admin"
	CodeRegisteredUser = "registered_user"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository
func (r *Repository) EnsureSystemRoles(ctx context.Context) error
func (r *Repository) FindDefault(ctx context.Context) (Role, error)
func (r *Repository) FindByCode(ctx context.Context, code string) (Role, error)
func (r *Repository) HasActiveUserWithRole(ctx context.Context, roleID int64) (bool, error)
```

`EnsureSystemRoles` creates missing rows by code and validates existing code/name/default/enabled values. It returns an error instead of changing a mismatched row. `FindDefault` requires one effective enabled role and reports zero or multiple matches explicitly.

- [ ] **Step 4: Run role tests**

```powershell
go fmt ./internal/module/role
go test ./internal/module/role -count=1
git diff --check
```

Expected: PASS. Do not commit automatically.

## Task 5: Implement user registration persistence and credential lookup

**Files:**
- Create: `server/internal/module/user/repository.go`
- Create: `server/internal/module/user/repository_test.go`

- [ ] **Step 1: Write failing PostgreSQL repository tests**

Cover atomic creation and lookup:

```go
func TestCreateWithRolePersistsUserAndRoleAtomically(t *testing.T)
func TestCreateWithRoleMapsUsernameConstraint(t *testing.T)
func TestCreateWithRoleMapsEmailConstraint(t *testing.T)
func TestCreateWithRoleRejectsInactiveRole(t *testing.T)
func TestFindCredentialUsesCaseInsensitiveUsername(t *testing.T)
func TestFindCredentialReturnsDisabledStateAndExcludesDeletedUsers(t *testing.T)
func TestFindCurrentUserRequiresAnEnabledRole(t *testing.T)
```

- [ ] **Step 2: Confirm the intended failures**

```powershell
go test ./internal/module/user -count=1
```

Expected: FAIL with missing repository symbols.

- [ ] **Step 3: Implement exact inputs, outputs, and sentinel errors**

```go
var (
	ErrUsernameConflict = errors.New("active username already exists")
	ErrEmailConflict    = errors.New("active email already exists")
)

type CreateInput struct {
	Username     string
	Email        string
	PasswordHash string
	RoleID       int64
}

type Credential struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	IsEnabled    yesno.Value
}

type Current struct {
	ID       int64
	Username string
	Email    string
}

func (r *Repository) CreateWithRole(ctx context.Context, input CreateInput) (User, error)
func (r *Repository) FindCredentialByUsername(ctx context.Context, username string) (Credential, error)
func (r *Repository) FindCurrent(ctx context.Context, userID int64) (Current, error)
```

`CreateWithRole` starts one GORM transaction, locks and revalidates the effective role, creates User, then creates `role.UserRole`. Map PostgreSQL conflicts using `pgconn.PgError.ConstraintName`, never error-message substring matching.

- [ ] **Step 4: Verify the user repository**

```powershell
go fmt ./internal/module/user
go test ./internal/module/user -count=1
git diff --check
```

Expected: PASS, including rollback after a relationship insert failure. Do not commit automatically.

## Task 6: Implement the durable single-session repository

**Files:**
- Create: `server/internal/module/auth/session_repository.go`
- Create: `server/internal/module/auth/session_repository_test.go`

- [ ] **Step 1: Write failing session concurrency tests**

Use real PostgreSQL and cover:

```go
func TestCreateReplacingActiveRevokesOldSession(t *testing.T)
func TestFindActiveIdentityRequiresVersionUserAndRole(t *testing.T)
func TestFindCurrentExcludesExpiredSession(t *testing.T)
func TestRotateByRefreshHashCanWinOnlyOnce(t *testing.T)
func TestRevokeIsIdempotentForTheSameSession(t *testing.T)
```

For the rotate test, launch two goroutines with the same old hash and assert exactly one returns `rotated=true`.

- [ ] **Step 2: Run the session tests and confirm failure**

```powershell
go test ./internal/module/auth -run 'Test(CreateReplacing|FindActiveIdentity|FindCurrent|RotateByRefreshHash|Revoke)' -count=1
```

Expected: FAIL because the repository is absent.

- [ ] **Step 3: Implement the session repository contract**

```go
type SessionCreate struct {
	UserID           int64
	RefreshTokenHash string
	ClientIP         string
	UserAgent        string
	RefreshExpiresAt time.Time
}

type Identity struct {
	UserID    int64
	SessionID int64
	Version   int64
}

func (r *SessionRepository) CreateReplacingActive(ctx context.Context, input SessionCreate, now time.Time) (Session, error)
func (r *SessionRepository) FindActiveIdentity(ctx context.Context, sessionID, version int64, now time.Time) (Identity, error)
func (r *SessionRepository) FindCurrentByUser(ctx context.Context, userID int64, now time.Time) (Session, error)
func (r *SessionRepository) FindByRefreshHash(ctx context.Context, hash string, now time.Time) (Session, error)
func (r *SessionRepository) RotateByRefreshHash(ctx context.Context, sessionID int64, oldHash, newHash string, now time.Time, clientIP, userAgent string) (Session, bool, error)
func (r *SessionRepository) Revoke(ctx context.Context, sessionID int64, now time.Time) error
```

Every active lookup joins `sys_user`, `sys_user_role`, and `sys_role`, requires non-deleted/enabled records, and checks `refresh_expires_at > now`. `CreateReplacingActive` locks the user row, revalidates identity state, revokes all unrevoked sessions, then inserts version 1.

- [ ] **Step 4: Verify deterministic session behavior**

```powershell
go fmt ./internal/module/auth
go test ./internal/module/auth -run 'Test(CreateReplacing|FindActiveIdentity|FindCurrent|RotateByRefreshHash|Revoke)' -count=1
git diff --check
```

Expected: PASS with exactly one refresh rotation winner. Do not commit automatically.

## Task 7: Implement password hashing and JWT claims

**Files:**
- Create: `server/internal/module/auth/password.go`
- Create: `server/internal/module/auth/password_test.go`
- Create: `server/internal/module/auth/token.go`
- Create: `server/internal/module/auth/token_test.go`
- Modify: `server/go.mod`
- Modify: `server/go.sum`

- [ ] **Step 1: Add the approved JWT dependency**

```powershell
cd D:\admin\server
go get github.com/golang-jwt/jwt/v5
```

Expected: `go.mod` contains direct `github.com/golang-jwt/jwt/v5`; no authentication framework is added.

- [ ] **Step 2: Write failing password tests**

```go
func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil { t.Fatal(err) }
	if err := VerifyPassword(hash, "correct horse battery staple"); err != nil { t.Fatal(err) }
	if err := VerifyPassword(hash, "wrong"); err == nil { t.Fatal("wrong password matched") }
}

func TestValidatePasswordRejectsOver72Bytes(t *testing.T) {
	if err := ValidatePassword(strings.Repeat("界", 25)); err == nil {
		t.Fatal("75-byte password was accepted")
	}
}
```

- [ ] **Step 3: Write failing JWT tests**

Cover issue/parse, wrong algorithm, wrong secret, expired token, future not-before, malformed claims, and exact user/session/version round trip.

- [ ] **Step 4: Run focused tests and confirm failure**

```powershell
go test ./internal/module/auth -run 'Test(HashPassword|ValidatePassword|JWT)' -count=1
```

Expected: FAIL because password and JWT functions do not exist.

- [ ] **Step 5: Implement concrete password and JWT code**

```go
const AccessTTL = 15 * time.Minute

type JWT struct {
	signingKey []byte
	now        func() time.Time
}

type Claims struct {
	UserID    int64 `json:"uid"`
	SessionID int64 `json:"sid"`
	Version   int64 `json:"ver"`
	jwt.RegisteredClaims
}

func NewJWT(signingKey []byte) *JWT
func (j *JWT) Issue(identity Identity) (string, time.Time, error)
func (j *JWT) Parse(raw string) (Identity, error)
```

`NewJWT` copies the already-derived signing key; it never receives `APP_SECRET`. `Parse` pins `jwt.SigningMethodHS256`, validates issuer and all positive IDs/version. `HashPassword` uses `bcrypt.DefaultCost`; `ValidatePassword` checks at least 8 runes and at most 72 bytes without trimming. Password hashing never reads or derives from `APP_SECRET`.

- [ ] **Step 6: Verify focused and package tests**

```powershell
go fmt ./internal/module/auth
go test ./internal/module/auth -run 'Test(HashPassword|ValidatePassword|JWT)' -count=1
go mod tidy
git diff --check
```

Expected: PASS and no unrelated dependency appears. Do not commit automatically.

## Task 8: Implement registration and login service flows

**Files:**
- Create: `server/internal/module/auth/service.go`
- Create: `server/internal/module/auth/service_test.go`
- Create: `server/internal/module/auth/request.go`
- Create: `server/internal/module/auth/response.go`

- [ ] **Step 1: Write failing registration service tests**

Use narrow fakes and cover normalization, Unicode username validation, email lowercase, exact password confirmation, default-role lookup, conflict mapping, and no session creation during registration:

```go
func TestRegisterCreatesEnabledUserWithDefaultRole(t *testing.T)
func TestRegisterRejectsInvalidUsernameEmailAndPassword(t *testing.T)
func TestRegisterMapsUsernameAndEmailConflicts(t *testing.T)
func TestRegisterDoesNotCreateSession(t *testing.T)
```

- [ ] **Step 2: Write failing login service tests**

```go
func TestLoginReturnsCredentialAndCurrentSession(t *testing.T)
func TestLoginUsesTheSamePublicErrorForUnknownUserAndWrongPassword(t *testing.T)
func TestLoginRejectsDisabledUser(t *testing.T)
func TestLoginRevokesNewSessionWhenRedisWriteFails(t *testing.T)
```

Assert the Redis key is exactly `auth:current-session:<userID>` and its TTL equals the remaining Refresh lifetime.

- [ ] **Step 3: Run tests and confirm failure**

```powershell
go test ./internal/module/auth -run 'Test(Register|Login)' -count=1
```

Expected: FAIL with missing Service methods.

- [ ] **Step 4: Implement the minimal service boundary**

Define only actual dependencies:

```go
type pointerStore interface {
	GetString(context.Context, string) (string, bool, error)
	SetString(context.Context, string, string, time.Duration) error
	Delete(context.Context, string) error
}

type Service struct {
	users               userStore
	roles               roleStore
	sessions            sessionStore
	pointers            pointerStore
	jwt                 *JWT
	refreshTokenHMACKey []byte
	now                 func() time.Time
}

type RegisterInput struct { Username, Email, Password, ConfirmPassword string }
type LoginInput struct { Username, Password, ClientIP, UserAgent string }
type Registered struct { UserID int64; Username, Email string }
type Credential struct { AccessToken string; ExpiresIn int; RefreshToken string; RefreshExpiresAt time.Time }

func (s *Service) Register(ctx context.Context, input RegisterInput) (Registered, error)
func (s *Service) Login(ctx context.Context, input LoginInput) (Credential, error)
```

Generate Refresh tokens with `crypto/rand`, encode them without bias, and store only `HMAC-SHA-256(refreshTokenHMACKey, token)` as 64 lowercase hex characters. The Service receives only the derived key from `secretkey.KeyRing`, never `APP_SECRET`. If `pointerErr` occurs after PostgreSQL creation, call `revokeErr := s.sessions.Revoke(ctx, session.ID, now)` and return `apperror.DependencyUnavailable(errors.Join(pointerErr, revokeErr))`.

- [ ] **Step 5: Verify registration and login**

```powershell
go fmt ./internal/module/auth
go test ./internal/module/auth -run 'Test(Register|Login)' -count=1
git diff --check
```

Expected: PASS; fake call order proves DB session creation precedes Redis pointer write. Do not commit automatically.

## Task 9: Implement authentication, refresh rotation, logout, and current user

**Files:**
- Modify: `server/internal/module/auth/service.go`
- Modify: `server/internal/module/auth/service_test.go`

- [ ] **Step 1: Write failing authentication tests**

Cover exact JWT/pointer/database behavior:

```go
func TestAuthenticateChecksPointerAndDatabaseVersion(t *testing.T)
func TestAuthenticateRebuildsOnlyAMissingPointer(t *testing.T)
func TestAuthenticateReturns503ForRedisErrors(t *testing.T)
func TestAuthenticateRejectsAReplacedSession(t *testing.T)
func TestAuthenticateRejectsDisabledUserOrRole(t *testing.T)
```

The cache-miss case must call `FindCurrentByUser`, set the pointer with the remaining Refresh TTL, then call `FindActiveIdentity`. A Redis error must not be treated as a miss.

- [ ] **Step 2: Write failing refresh tests**

```go
func TestRefreshRotatesHashAndIncrementsVersion(t *testing.T)
func TestRefreshKeepsAbsoluteRefreshExpiry(t *testing.T)
func TestRefreshRejectsReusedToken(t *testing.T)
func TestRefreshRejectsANonCurrentSession(t *testing.T)
func TestRefreshReturns503BeforeMutationWhenRedisFails(t *testing.T)
```

Assert the second credential has the same SessionID, `Version + 1`, a different Refresh Token, and a Cookie lifetime equal to the original absolute expiry minus the current time.

- [ ] **Step 3: Write failing logout and current-user tests**

```go
func TestLogoutRevokesPostgreSQLBeforeDeletingPointer(t *testing.T)
func TestLogoutReturns503WhenPointerDeleteFails(t *testing.T)
func TestCurrentUserReturnsOnlyIDUsernameAndEmail(t *testing.T)
```

- [ ] **Step 4: Confirm all new tests fail for missing methods**

```powershell
go test ./internal/module/auth -run 'Test(Authenticate|Refresh|Logout|CurrentUser)' -count=1
```

Expected: FAIL with missing Service methods.

- [ ] **Step 5: Implement the remaining service API**

```go
type RefreshInput struct {
	RefreshToken string
	ClientIP     string
	UserAgent    string
}

func (s *Service) Authenticate(ctx context.Context, accessToken string) (Identity, error)
func (s *Service) Refresh(ctx context.Context, input RefreshInput) (Credential, error)
func (s *Service) Logout(ctx context.Context, identity Identity) error
func (s *Service) CurrentUser(ctx context.Context, identity Identity) (user.Current, error)
```

Implement `currentSessionID(ctx, userID, now)` as the only pointer read-through function. It parses positive integer values strictly, rebuilds only on a true missing key, and maps Redis operation errors to code 10006. Refresh performs pointer validation before PostgreSQL mutation, then calls the repository's compare-and-swap rotation.

- [ ] **Step 6: Verify all Auth Service behavior**

```powershell
go fmt ./internal/module/auth
go test ./internal/module/auth -run 'Test(Register|Login|Authenticate|Refresh|Logout|CurrentUser)' -count=1
git diff --check
```

Expected: PASS. Do not commit automatically.

## Task 10: Expose strict HTTP endpoints and authentication middleware

**Files:**
- Create: `server/internal/module/auth/middleware.go`
- Create: `server/internal/module/auth/middleware_test.go`
- Create: `server/internal/module/auth/handler.go`
- Create: `server/internal/module/auth/handler_test.go`
- Create: `server/internal/module/auth/route.go`
- Modify: `server/internal/module/auth/request.go`
- Modify: `server/internal/module/auth/response.go`

- [ ] **Step 1: Write failing middleware tests**

Test exact Origin comparison, missing/malformed Bearer headers, successful identity placement, 401 authentication failures, and 503 dependency failures:

```go
func TestRequireOriginAcceptsOnlyExactConfiguredOrigin(t *testing.T)
func TestAuthenticateMiddlewareRequiresBearerToken(t *testing.T)
func TestAuthenticateMiddlewareStoresIdentity(t *testing.T)
func TestAuthenticateMiddlewarePreservesDependencyUnavailable(t *testing.T)
```

- [ ] **Step 2: Write failing Handler contract tests**

Cover every route and cookie attribute:

```go
func TestRegisterReturns201AndNoCookie(t *testing.T)
func TestLoginReturnsCredentialAndSecureRefreshCookie(t *testing.T)
func TestRefreshRejectsEveryNonEmptyBody(t *testing.T)
func TestRefreshRotatesCookieWithRemainingLifetime(t *testing.T)
func TestLogoutExpiresCookieEvenWhenRedisDeleteFails(t *testing.T)
func TestMeReturnsClosedCurrentUserShape(t *testing.T)
func TestAuthHandlersRejectUnknownJSONFields(t *testing.T)
```

Parse response JSON and assert the only top-level keys are `code`, `data`, `message`; assert success DTOs contain exactly their documented keys.

- [ ] **Step 3: Run focused HTTP tests and confirm failure**

```powershell
go test ./internal/module/auth -run 'Test(RequireOrigin|AuthenticateMiddleware|RegisterReturns|LoginReturns|Refresh|LogoutExpires|MeReturns|AuthHandlers)' -count=1
```

Expected: FAIL with missing middleware/handler/route symbols.

- [ ] **Step 4: Implement exact request and response DTOs**

```go
type RegisterRequest struct {
	Username        string `json:"username" binding:"required,max=64"`
	Email           string `json:"email" binding:"required,max=254"`
	Password        string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required,max=64"`
	Password string `json:"password" binding:"required"`
}

type RegisteredResponse struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CredentialResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
}

type CurrentUserResponse struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Email    string `json:"email"`
}
```

- [ ] **Step 5: Implement middleware and cookie handling**

Keep middleware in the auth package to avoid an import cycle. Expose:

```go
func RequireOrigin(allowedOrigin string) gin.HandlerFunc
func Authenticate(service authenticationService) gin.HandlerFunc
func IdentityFromContext(context *gin.Context) (Identity, bool)
```

The Handler uses `validate.BindJSON`, `validate.RequireEmptyBody`, `context.Request.Context()`, `context.ClientIP()`, and `User-Agent`. A single `setRefreshCookie` helper writes the fixed name/path/HttpOnly/SameSite attributes and remaining Max-Age; a single `expireRefreshCookie` helper uses the same scope with Max-Age `-1`.

- [ ] **Step 6: Register the five routes**

```go
func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, origin gin.HandlerFunc, authenticate gin.HandlerFunc) {
	authRoutes := routes.Group("/auth")
	authRoutes.POST("/register", origin, handler.Register)
	authRoutes.POST("/login", origin, handler.Login)
	authRoutes.POST("/refresh", origin, handler.Refresh)
	authRoutes.POST("/logout", origin, authenticate, handler.Logout)
	authRoutes.GET("/me", authenticate, handler.Me)
}
```

- [ ] **Step 7: Verify HTTP contracts**

```powershell
go fmt ./internal/module/auth
go test ./internal/module/auth -count=1
git diff --check
```

Expected: PASS. Cookie tests confirm HttpOnly, host-only, SameSite Lax, exact Path, Secure mode, Expires, and remaining Max-Age. Do not commit automatically.

## Task 11: Wire schema, roles, authentication, and routes into the API

**Files:**
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/main_test.go`

- [ ] **Step 1: Extend the router test before wiring**

Add expected route counts:

```go
want := map[string]int{
	"GET /health":                  1,
	"GET /ready":                   1,
	"POST /api/v1/example-tasks":   1,
	"POST /api/v1/auth/register":   1,
	"POST /api/v1/auth/login":      1,
	"POST /api/v1/auth/refresh":    1,
	"POST /api/v1/auth/logout":     1,
	"GET /api/v1/auth/me":          1,
}
```

Provide a small fake auth service that satisfies the Handler and middleware methods.

- [ ] **Step 2: Run and confirm missing routes**

```powershell
go test ./cmd/api -run TestBuildRouterRegistersFoundationRoutesOnce -count=1
```

Expected: FAIL for the five missing auth routes.

- [ ] **Step 3: Extend startup wiring explicitly**

In `run`, after opening dependencies:

```go
if err := database.AutoMigrate(
	processContext,
	postgres.GORM,
	&taskdemo.Task{},
	&user.User{},
	&role.Role{},
	&role.UserRole{},
	&auth.Session{},
); err != nil {
	return err
}
if err := auth.EnsureSchema(processContext, postgres.GORM); err != nil {
	return fmt.Errorf("ensure authentication schema: %w", err)
}

roleRepository := role.NewRepository(postgres.GORM)
if err := roleRepository.EnsureSystemRoles(processContext); err != nil {
	return fmt.Errorf("ensure system roles: %w", err)
}
```

Construct `secretkey.KeyRing` once from `cfg.AppSecret`; fail startup on any derivation error. Construct JWT with `keys.JWTSigningKey()` and Auth Service with `keys.RefreshTokenHMACKey()`, then construct the concrete User, Role, Session repositories, Handler, Origin middleware, and Authenticate middleware. The raw `APP_SECRET` must not be passed into Auth Service, JWT, repositories, or handlers. Add dependencies to `routerDependencies`; `buildRouter` calls `auth.RegisterRoutes` exactly once.

- [ ] **Step 4: Run API package and startup tests**

```powershell
go fmt ./cmd/api
go test ./cmd/api -count=1
go test ./internal/config ./internal/secretkey ./internal/module/auth ./internal/module/user ./internal/module/role -count=1
git diff --check
```

Expected: PASS. API tests do not open network listeners. Do not commit automatically.

## Task 12: Add the one-time super-admin bootstrap command

**Files:**
- Create: `server/internal/module/auth/bootstrap_service.go`
- Create: `server/internal/module/auth/bootstrap_service_test.go`
- Create: `server/cmd/bootstrap-admin/main.go`
- Create: `server/cmd/bootstrap-admin/main_test.go`

- [ ] **Step 1: Write failing service tests for bootstrap rules**

```go
func TestBootstrapAdminCreatesUserWithSuperAdminRole(t *testing.T)
func TestBootstrapAdminRejectsExistingActiveSuperAdmin(t *testing.T)
func TestBootstrapAdminRejectsMissingOrDisabledSystemRole(t *testing.T)
func TestBootstrapAdminUsesRegistrationValidationAndConflictMapping(t *testing.T)
```

- [ ] **Step 2: Implement a concrete bootstrap-only service**

```go
type BootstrapAdminInput struct {
	Username string
	Email    string
	Password string
}

type BootstrapService struct {
	users bootstrapUserStore
	roles bootstrapRoleStore
}

func (s *BootstrapService) Create(ctx context.Context, input BootstrapAdminInput) (Registered, error) {
	adminRole, err := s.roles.FindByCode(ctx, role.CodeSuperAdmin)
	if err != nil { return Registered{}, apperror.Internal(err) }
	exists, err := s.roles.HasActiveUserWithRole(ctx, adminRole.ID)
	if err != nil { return Registered{}, apperror.Internal(err) }
	if exists { return Registered{}, apperror.Conflict("超级管理员已存在", nil) }
	normalized, err := validateAccountInput(input.Username, input.Email, input.Password, input.Password)
	if err != nil { return Registered{}, err }
	hash, err := HashPassword(normalized.Password)
	if err != nil { return Registered{}, apperror.Internal(err) }
	created, err := s.users.CreateWithRole(ctx, user.CreateInput{
		Username: normalized.Username, Email: normalized.Email,
		PasswordHash: hash, RoleID: adminRole.ID,
	})
	if err != nil { return Registered{}, mapUserCreateError(err) }
	return Registered{UserID: created.ID, Username: created.Username, Email: created.Email}, nil
}
```

`validateAccountInput` and `mapUserCreateError` are the same concrete helpers used by Register. `BootstrapService` has only User and Role dependencies, so the command cannot accidentally require Redis, JWT, or Session dependencies.

- [ ] **Step 3: Write failing command tests**

Test the command configuration loader with missing `POSTGRES_DSN`, username, email, or password. Test that success output includes only the created user ID/username and never includes the password or hash.

- [ ] **Step 4: Implement the command composition**

`main.go` loads `.env`, requires:

```text
POSTGRES_DSN
BOOTSTRAP_ADMIN_USERNAME
BOOTSTRAP_ADMIN_EMAIL
BOOTSTRAP_ADMIN_PASSWORD
```

It opens PostgreSQL, constructs Role/User repositories and `BootstrapService`, calls `Create`, and exits non-zero on any error. It does not AutoMigrate, connect Redis, issue tokens, or start HTTP.

- [ ] **Step 5: Verify service and command**

```powershell
go fmt ./internal/module/auth ./cmd/bootstrap-admin
go test ./internal/module/auth -run TestBootstrapAdmin -count=1
go test ./cmd/bootstrap-admin -count=1
go build ./cmd/bootstrap-admin
git diff --check
```

Expected: PASS. Do not execute the real bootstrap command until the API has migrated tables and the owner supplies process-only bootstrap credentials. Do not commit automatically.

## Task 13: Add strict frontend auth contracts and the memory-only store

**Files:**
- Create: `web/src/api/auth.contract.ts`
- Create: `web/src/api/auth.contract.test.ts`
- Create: `web/src/api/auth.ts`
- Create: `web/src/api/auth.test.ts`
- Create: `web/src/store/auth.ts`
- Create: `web/src/store/auth.test.ts`
- Modify: `web/src/types/http.ts`

- [ ] **Step 1: Write failing closed-object parser tests**

Test accepted DTOs and reject missing, extra, wrong-type, empty-token, non-positive-ID, and non-positive-expiry values:

```ts
expect(parseCredential({ accessToken: 'jwt', expiresIn: 900 })).toEqual({
  accessToken: 'jwt',
  expiresIn: 900,
})

expect(() => parseCredential({ accessToken: 'jwt', expiresIn: 900, refreshToken: 'forbidden' }))
  .toThrow(ProtocolError)
```

- [ ] **Step 2: Run and confirm missing contract functions**

```powershell
cd D:\admin\web
pnpm vitest run src/api/auth.contract.test.ts
```

Expected: FAIL because the contract module does not exist.

- [ ] **Step 3: Move HTTP error classes to the dependency-free type module**

Move `ProtocolError` and `ApiError` from `utils/request.ts` to `types/http.ts`, then re-export them from `utils/request.ts` so existing imports remain valid. `auth.contract.ts` imports `ProtocolError` from `types/http.ts`; `request.ts` can then import Auth parsers without a circular dependency.

- [ ] **Step 4: Implement exact TypeScript contracts**

```ts
export interface RegisterInput {
  username: string
  email: string
  password: string
  confirmPassword: string
}

export interface LoginInput {
  username: string
  password: string
}

export interface RegisteredUser {
  userId: number
  username: string
  email: string
}

export interface AccessCredential {
  accessToken: string
  expiresIn: number
}

export interface CurrentUser {
  userId: number
  username: string
  email: string
}
```

Implement `parseRegisteredUser`, `parseCredential`, and `parseCurrentUser` from `unknown` using `Record<string, unknown>` and exact key checks. No explicit `any` or type assertion to a DTO.

- [ ] **Step 5: Write and implement API functions**

`api/auth.ts` calls `request<unknown>` and parses the result:

```ts
export async function register(input: RegisterInput): Promise<RegisteredUser>
export async function login(input: LoginInput): Promise<AccessCredential>
export async function refresh(): Promise<AccessCredential>
export async function logout(): Promise<void>
export async function getCurrentUser(): Promise<CurrentUser>
```

Use exact paths from the spec. Refresh and Logout send no JSON body.

- [ ] **Step 6: Write failing store tests**

Assert the four statuses, expiry calculation, user application, error state, and full clearing:

```ts
const store = useAuthStore()
store.setCredential({ accessToken: 'jwt', expiresIn: 900 }, 1_000)
expect(store.accessToken).toBe('jwt')
expect(store.accessExpiresAt).toBe(901_000)
store.setAuthenticated({ userId: 1, username: 'admin', email: 'admin@example.com' })
expect(store.status).toBe('authenticated')
store.setAnonymous()
expect(store.accessToken).toBe('')
expect(store.user).toBeNull()
```

- [ ] **Step 7: Implement a state-only Pinia store**

The store may expose `setCredential`, `setAuthenticated`, `setAnonymous`, and `setError`. It must not import Axios or `api/auth.ts`, and must not use browser persistent storage.

- [ ] **Step 8: Verify contracts, API, and Store**

```powershell
pnpm vitest run src/api/auth.contract.test.ts src/api/auth.test.ts src/store/auth.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: PASS and strict TypeScript compilation succeeds. Do not commit automatically.

## Task 14: Coordinate one refresh in the Axios request layer

**Files:**
- Modify: `web/src/utils/request.ts`
- Modify: `web/src/utils/request.test.ts`
- Modify: `web/src/api/auth.ts`

- [ ] **Step 1: Write failing request tests**

Using Axios adapters, cover:

```ts
it('adds the in-memory bearer token to protected requests')
it('coordinates concurrent 401 responses through one refresh')
it('retries each original request at most once')
it('does not refresh login register refresh or logout requests')
it('sets anonymous after a refresh 401')
it('sets error after a refresh 503 or protocol violation')
```

For concurrency, start three requests, return 401 for each first attempt, resolve one refresh response, and assert exactly one `/api/v1/auth/refresh` call and three successful retries.

- [ ] **Step 2: Run and confirm the current client lacks auth behavior**

```powershell
pnpm vitest run src/utils/request.test.ts
```

Expected: FAIL only for the newly added auth tests; existing envelope tests remain green.

- [ ] **Step 3: Refactor without weakening envelope validation**

Keep `parseEnvelope` as the only envelope parser. Add a separate raw Axios client for Refresh so Refresh cannot enter its own interceptor. Use a typed request config:

```ts
interface AuthRequestConfig extends AxiosRequestConfig {
  authRetried?: boolean
}

let refreshPromise: Promise<string> | null = null
```

On an eligible 401, create one promise that posts with no body and `withCredentials: true`, validates the exact credential DTO, updates `useAuthStore(pinia)`, and returns the Access Token. Await it, clone the original config with `authRetried: true`, set Authorization, and retry once.

- [ ] **Step 4: Expose Refresh through the API module**

Export one `refreshAccessCredential()` function from the request module for cold-start use; `api/auth.ts::refresh()` delegates to it. This function returns the validated DTO and updates only credential state, not current-user state.

- [ ] **Step 5: Verify request behavior and types**

```powershell
pnpm vitest run src/utils/request.test.ts src/api/auth.test.ts src/store/auth.test.ts
pnpm exec vue-tsc -b --pretty false
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" src -g "*.ts" -g "*.vue"
```

Expected: tests and typecheck PASS; `rg` produces no matches. Do not commit automatically.

## Task 15: Add protected routing and simple login/register pages

**Files:**
- Modify: `web/src/router/index.ts`
- Modify: `web/src/router/index.test.ts`
- Modify: `web/src/permission.ts`
- Create: `web/src/views/auth/login/index.vue`
- Create: `web/src/views/auth/login/index.test.ts`
- Create: `web/src/views/auth/register/index.vue`
- Create: `web/src/views/auth/register/index.test.ts`

- [ ] **Step 1: Replace public-dashboard tests with auth-route tests**

Add tests that assert:

```ts
expect(router.resolve('/login').meta.requiresAuth).toBe(false)
expect(router.resolve('/register').meta.requiresAuth).toBe(false)
expect(router.resolve('/dashboard').meta.requiresAuth).toBe(true)
```

Mock `refresh`, `getCurrentUser`, and the Auth Store to cover cold-start success, Refresh 401 redirect to `/login?redirect=/dashboard`, authenticated access, public access, dependency error state, and missing route metadata rejection.

- [ ] **Step 2: Run router tests and confirm old behavior fails**

```powershell
pnpm vitest run src/router/index.test.ts
```

Expected: FAIL because Dashboard is still public and auth routes are absent.

- [ ] **Step 3: Implement explicit routes and guard flow**

Use lazy imports for auth pages and the new layout. Keep `requiresAuth` mandatory on every matched record. The guard algorithm is:

```ts
if (to.matched.some((record) => typeof record.meta.requiresAuth !== 'boolean')) {
  throw new Error(`Route ${to.fullPath} must declare requiresAuth`)
}
if (!to.meta.requiresAuth) {
  if (auth.status === 'authenticated' && (to.name === 'login' || to.name === 'register')) {
    return { name: 'dashboard' }
  }
  return true
}
if (auth.status === 'authenticated') return true
try {
  await refresh()
  const user = await getCurrentUser()
  auth.setAuthenticated(user)
  return true
} catch (error: unknown) {
  if (isUnauthorized(error)) {
    auth.setAnonymous()
  } else {
    auth.setError(errorMessage(error))
  }
  return { name: 'login', query: { redirect: to.fullPath } }
}
```

Use the existing numeric `ApiError` class for 401 detection; do not treat 503 or ProtocolError as anonymous.

- [ ] **Step 4: Write failing Login component tests**

Mock Auth API and Router. Assert required fields, disabled submit while pending, exact username/password payload, credential then `/me` order, Store update, redirect query handling, generic credential error, and explicit Store bootstrap error banner.

- [ ] **Step 5: Implement the simple Login page**

Use an `el-form` with username and password, one primary submit button, and a RouterLink to Register. On success: set credential, request current user, set authenticated, then replace the requested protected route or `/dashboard`. Do not include captcha, forgot password, social login, policy dialogs, background art, or success animation.

- [ ] **Step 6: Write failing Register component tests**

Assert all four fields, password equality, exact API payload, conflict/error display, submit lock, and success redirect to Login without setting Auth Store credential.

- [ ] **Step 7: Implement the simple Register page**

Use username, email, password, confirm-password fields. Do not trim password. On success show one Element Plus success message and replace `/login`; include a link back to Login.

- [ ] **Step 8: Verify routes and auth pages**

```powershell
pnpm vitest run src/router/index.test.ts src/views/auth/login/index.test.ts src/views/auth/register/index.test.ts
pnpm exec vue-tsc -b --pretty false
git diff --check
```

Expected: PASS. Do not commit automatically.

## Task 16: Replace the old layout with the approved Element Plus shell

**Files:**
- Modify: `web/src/layout/index.vue`
- Create: `web/src/layout/index.test.ts`
- Create: `web/src/layout/components/AppAside.vue`
- Create: `web/src/layout/components/AppHeader.vue`
- Create: `web/src/layout/components/AppFooter.vue`
- Delete: `web/src/layout/AdminLayout.vue`
- Modify: `web/src/styles/index.scss`
- Modify: `web/src/styles/variables.scss`
- Modify: `web/src/views/dashboard/index.vue`
- Modify: `web/src/views/dashboard/index.test.ts`

- [ ] **Step 1: Re-read the user's layout file and write failing shell tests**

Immediately before editing, run:

```powershell
Get-Content -Raw D:\admin\web\src\layout\index.vue
git status --short
```

Then test that the shell has one Aside, Header, Main, Footer, RouterView, username, Logout, one static Dashboard item, desktop collapse, and mobile Drawer. Assert it emits/calls logout and routes to Login.

- [ ] **Step 2: Run layout tests and confirm the template is incomplete**

```powershell
pnpm vitest run src/layout/index.test.ts
```

Expected: FAIL because the current untracked template contains only placeholders.

- [ ] **Step 3: Implement focused components**

`AppAside.vue` accepts `collapsed: boolean`, renders the brand and one Element Plus menu item for `/dashboard`. `AppHeader.vue` accepts `username`, emits `toggleMenu` and `logout`, and uses Element Plus icons with titles/ARIA labels. `AppFooter.vue` renders only concise copyright text.

- [ ] **Step 4: Implement the approved container nesting**

The root template must follow:

```vue
<el-container class="admin-layout">
  <el-aside class="admin-layout__aside" :width="asideWidth">
    <AppAside :collapsed="collapsed" />
  </el-aside>
  <el-container class="admin-layout__workspace">
    <el-header class="admin-layout__header">
      <AppHeader :username="username" @toggle-menu="toggleMenu" @logout="handleLogout" />
    </el-header>
    <el-main class="admin-layout__main"><RouterView /></el-main>
    <el-footer class="admin-layout__footer"><AppFooter /></el-footer>
  </el-container>
  <el-drawer v-model="mobileMenuOpen" direction="ltr" :with-header="false" size="240px">
    <AppAside :collapsed="false" />
  </el-drawer>
</el-container>
```

Use stable heights and widths. Do not copy the reference dashboard cards or menu inventory.

- [ ] **Step 5: Remove the duplicate layout and adapt Dashboard**

Delete `AdminLayout.vue` after Router imports `layout/index.vue`. Change Dashboard's page-level `header.admin-header` into a content-local status toolbar so the application has one global Header. Update its test selector while preserving health/task behavior.

- [ ] **Step 6: Verify responsive layout and existing Dashboard behavior**

```powershell
pnpm vitest run src/layout/index.test.ts src/views/dashboard/index.test.ts src/router/index.test.ts
pnpm exec vue-tsc -b --pretty false
rg -n "AdminLayout" src
git diff --check
```

Expected: tests/typecheck PASS; `rg` has no output; only `layout/index.vue` is the shell entry. Do not commit automatically.

## Task 17: Complete integration, security, and browser verification

**Files:**
- Modify only files proven necessary by failing verification from Tasks 1-16
- Review: `docs/superpowers/specs/2026-08-17-admin-authentication-foundation-design.md`

- [ ] **Step 1: Prepare local auth configuration without exposing secrets**

Check key names only:

```powershell
Get-Content server\.env |
  Where-Object { $_ -match '^[A-Z][A-Z0-9_]*=' } |
  ForEach-Object { ($_ -split '=', 2)[0] }
```

The current local file lacks the application root secret and Auth Cookie setting. Generate one 64-byte random value with `System.Security.Cryptography.RandomNumberGenerator`, encode it as Base64, insert it as `APP_SECRET` into the ignored `server/.env` using `apply_patch`, add `AUTH_COOKIE_SECURE=0`, and never print or include the value in status reports. If the owner has supplied a value meanwhile, preserve it. Do not create `AUTH_JWT_SECRET`, `AUTH_REFRESH_PEPPER`, or `APP_SECRET_PREVIOUS`.

- [ ] **Step 2: Run all backend verification**

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

Expected: every command exits 0. PostgreSQL integration tests verify real catalog definitions; no test silently skips due to missing normal local configuration.

- [ ] **Step 3: Run all frontend verification**

```powershell
cd D:\admin\web
pnpm vitest run
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" src -g "*.ts" -g "*.vue"
```

Expected: Vitest and build exit 0; `rg` prints nothing.

- [ ] **Step 4: Start only the services needed for browser verification**

Do not stop or replace user-managed processes. If ports 16300 or 16301 are free, start:

```powershell
cd D:\admin\server
go run ./cmd/api

cd D:\admin\web
pnpm dev
```

Use hidden background processes owned by this task, record their PIDs, and stop only those PIDs after verification. The Worker is not required for Auth.

- [ ] **Step 5: Run the real browser acceptance sequence**

At `http://localhost:16300` verify:

```text
Register -> redirected to Login -> Login -> Dashboard
Reload browser -> Refresh Cookie -> /me -> authenticated Dashboard
Refresh token rotation -> old Access Token rejected
Second browser context Login -> first context rejected
Logout -> Cookie expires -> protected route returns to Login
Redis unavailable -> explicit service error, no fake Login success
```

Inspect browser storage: Access Token must not exist in localStorage/sessionStorage/IndexedDB; `admin_refresh` must be HttpOnly, host-only, SameSite Lax, Path `/api/v1/auth`, and use the remaining absolute lifetime.

- [ ] **Step 6: Review the documented debt boundaries**

Confirm the implementation contains no fake menu API, email-verification fields, auth-platform table, login log, session management page, or placeholder abstractions. Confirm the static Dashboard menu is visibly confined to `AppAside.vue` and listed for deletion in the approved spec.

- [ ] **Step 7: Final worktree and diff inspection**

```powershell
cd D:\admin
git status --short
git diff --check
git diff --stat
```

Expected: no generated build output or secret file is tracked; user-owned unrelated changes remain intact. Report verification results and remaining approved debt. Do not commit, fetch, pull, or push without explicit owner instruction.

## Plan Completion Criteria

- Every task's focused tests passed before moving forward.
- The four PostgreSQL tables, constraints, and indexes match the approved spec.
- Registration, Login, Refresh, Logout, `/me`, single-session replacement, and Bootstrap behavior are implemented without silent fallback.
- Frontend Access Token remains memory-only and the Refresh Cookie remains inaccessible to JavaScript.
- Router uses only `layout/index.vue`; `AdminLayout.vue` is removed.
- Full Go verification, full Vitest, frontend build, TypeScript Any scan, and browser acceptance all pass.
- Approved phase-one debt remains explicit and no additional undocumented debt was introduced.
