# 用户认证日志数值协议与审计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `user_login_log` 改为注册/登录/登出三事件的整数协议，删除日志表 `session_id`，补齐邮箱/手机号自动注册审计，并保证审计失败不影响认证主流程。

**Architecture:** `user/loginLog` 以 Go typed enum 和 PostgreSQL `SMALLINT + CHECK` 作为协议事实层，API/前端严格传输整数。认证 Service 继续直接调用 PostgreSQL 写审计，不引入 Asynq；登录、注册、登出均使用 best-effort 记录，Session 撤销保持独立的核心失败语义。系统字典只维护可配置展示选项，不参与认证日志枚举。

**Tech Stack:** Go、Gin、GORM/PostgreSQL、Redis auth state、Vue 3、TypeScript、Element Plus、Vitest、PowerShell forward migration。

**Spec:** `docs/superpowers/specs/2026-09-23-user-login-log-audit-design.md`

## Global Constraints

- `event_type`: `1=register`、`2=login`、`3=logout`；`login_type`: `1=password`、`2=email`、`3=phone`。
- 注册只能来自邮箱或手机号验证码自动注册；`user_account.username` 继续使用 `deterministicUsername(account)`。
- 注册/登录的 `account` 是规范化邮箱或手机号；登出的 `account` 是当前 `username`；登出 `login_type` 为 `NULL`。
- 只删除 `user_login_log.session_id`，不得删除 `user_session`、JWT 或认证撤销流程中的 Session ID。
- 登录、注册、登出日志不使用 Asynq；审计写入和 username 查询失败不得阻断认证主流程。
- 固定枚举不进入系统字典；标签使用代码 metadata 与 i18n，系统字典继续只提供可维护的展示型选项。
- 所有入口的 `context.Context` 必须贯穿 Service、Repository、GORM、Redis 和第三方 I/O。
- 不自动 commit、push、pull、rebase 或回滚工作区中其他开发者的改动。

---

### Task 1: 固化登录日志领域枚举和校验契约

**Files:**
- Modify: `server/internal/module/user/loginLog/event.go`
- Modify: `server/internal/module/user/loginLog/model.go`
- Modify: `server/internal/module/user/loginLog/repository.go`
- Modify: `server/internal/module/user/loginLog/service.go`
- Modify: `server/internal/module/user/loginLog/handler.go`
- Test: `server/internal/module/user/loginLog/model_test.go`
- Test: `server/internal/module/user/loginLog/event_test.go`
- Create: `server/internal/module/user/loginLog/handler_test.go`

**Interfaces:**
- Add typed `EventType int16` with `EventRegister=1`、`EventLogin=2`、`EventLogout=3`。
- Add typed `LoginType int16` with `LoginPassword=1`、`LoginEmail=2`、`LoginPhone=3`。
- Change `Event`、`LoginLog`、`ListQuery`、`Item` and service boundaries from string values to typed integers；`LoginType` remains a pointer because logout is null.
- Rename `LoginAccount`/`login_account` references in Go DTOs to `Account`/`account`。
- Keep `PageInit`，but return `eventTypes: [1,2,3]` and `loginTypes: [1,2,3]`。

- [ ] **Step 1: Write failing domain tests** for all enum values, unknown values, register/login requiring a non-null login type, logout requiring a null login type, account non-empty/length limits, and invalid `is_success`.
- [ ] **Step 2: Run the focused tests** with `go test ./internal/module/user/loginLog -run 'Test(Event|LoginLog|Validate|PageInit|Parse)'`; confirm failures come from the old string contract.
- [ ] **Step 3: Implement minimal typed enums and validation**. Use explicit `IsValid`/metadata helpers; reject zero and unknown integers at the Service/Handler boundary.
- [ ] **Step 4: Convert the list query parser** to `strconv.ParseInt` for `eventType` and `loginType`，reject repeated/unknown values，preserve empty query values as “all”，and use typed values in GORM predicates.
- [ ] **Step 5: Run the focused package tests and `go vet ./internal/module/user/loginLog`**；confirm the package is green before migration work。

### Task 2: Create and verify the forward database migration

**Files:**
- Create: `docs/database/2026-09-23-user-login-log-numeric.sql`
- Create: `docs/database/2026-09-23-user-login-log-numeric.ps1`
- Test: `server/internal/database/user_login_log_numeric_migration_test.go`
- Modify: `docs/database/current.sql` only after the real migration succeeds

**Interfaces:**
- Convert `event_type` and `login_type` from varchar to smallint using explicit known-value mappings。
- Rename `login_account` to `account`。
- Drop `fk_user_login_log_session` and `session_id`。
- Preserve `user_id`、platform foreign key、timestamps、Yes/No check、append-only log semantics and real query indexes。

- [ ] **Step 1: Write migration tests** that create the old schema/data, assert known mappings (`login=2`、`logout=3`、`password=1`、`email=2`、`phone=3`), reject an unknown value, and verify the pre-migration schema remains unchanged after a failed transaction.
- [ ] **Step 2: Write the idempotent forward SQL**. Drop dependent checks before type conversion, reject unknown values before mutation, convert with `CASE`, rename the account column, drop the Session FK/column, and create the new enum/account checks. PostgreSQL automatically keeps the existing account index attached to the renamed column; verify its definition and do not create a duplicate index.
- [ ] **Step 3: Backfill existing logout accounts before enforcing the new check** with `UPDATE user_login_log AS log SET login_account = account.username FROM user_account AS account WHERE log.event_type = 'logout' AND btrim(log.login_account) = '' AND log.user_id = account.id`; preflight must fail if any logout row still has an empty account, missing user, or empty username. Add `CHECK (btrim(account) <> '')` after renaming so future empty strings are rejected without inventing historical values。
- [ ] **Step 4: Implement the PowerShell runner** with DSN loading from `server/.env`/environment, transaction execution, catalog checks, exact rerun verification and a clear non-zero failure report. Do not run API/Worker DDL on startup.
- [ ] **Step 5: Run SQL parser/migration unit tests only**. The maintainer executes the runner against the real PostgreSQL after reviewing the SQL；then export the schema-only snapshot to `docs/database/current.sql` and verify the new columns、checks、FKs and indexes。

### Task 3: Record automatic email/phone registration events

**Files:**
- Modify: `server/internal/module/auth/login/service.go`
- Modify: `server/internal/module/auth/login/service_test.go`
- Modify: `server/internal/module/user/account/repository.go`
- Modify: `server/internal/module/user/account/repository_test.go`

**Interfaces:**
- `loginWithCode` emits a register event only after `CreateVerifiedIdentity` succeeds and before `issueCredential` emits the login event.
- Registration event uses `EventRegister`、`LoginTypeEmail`/`LoginTypePhone`、normalized `account`、created `user_id`、no `session_id`、`IsSuccess=yes` and `ReasonCode=success`.
- A unique-key race that re-queries an existing credential emits no register event.
- `recordLoginEvent` bounds each PostgreSQL insert to 500ms with a derived request context; it logs failures and returns nil.

- [ ] **Step 1: Add a failing test** for a new email user asserting exactly two events in order: register then login, with the same normalized account and user ID but different event types.
- [ ] **Step 2: Add a failing race test** where `CreateVerifiedIdentity` returns an email/phone conflict and the winner is reloaded；assert only the login event is recorded。
- [ ] **Step 3: Implement the minimal registration event call** immediately after a successful user creation，reusing `recordLoginEvent` so database failure remains best-effort. Update `recordLoginEvent` to derive a 500ms `context.WithTimeout` from its incoming context before calling the recorder; do not use `context.Background()`.
- [ ] **Step 4: Run `go test ./internal/module/auth/login -run 'Test(Login|Register|Auto)'`** and verify existing login failure events retain user IDs only when the credential lookup found a user.

### Task 4: Make logout audit username-based and non-blocking

**Files:**
- Modify: `server/internal/module/auth/login/service.go`
- Modify: `server/internal/module/user/account/repository.go`
- Modify: `server/internal/module/auth/login/service_test.go`
- Modify: `server/internal/module/user/account/repository_test.go`

**Interfaces:**
- Add a narrow `FindUsernameByID(context.Context, int64) (string, error)` account repository method and expose it through the auth Service user store, or use an existing account-only repository method without calling HTTP `me`。
- `Logout` keeps `revokeSession` as the core action，then constructs `EventLogout` with `account=username`、`LoginType=nil`、`user_id` and platform ID。
- Username lookup and log insert use best-effort warning paths；they must not return `CodeDependencyUnavailable` after successful Session revocation。
- Derive a bounded audit context from the request context using a package constant and `context.WithTimeout`; use it for username lookup and insert, and never use `context.Background()`.

- [ ] **Step 1: Add failing logout tests** for username in the emitted event, null login type, no session ID in the event, username lookup failure, log repository failure, and deadline expiry；all cases must still return successful logout after Session revocation。
- [ ] **Step 2: Implement the narrow username query** with the request context and a non-empty result requirement；do not query previous login rows or call `me`。
- [ ] **Step 3: Route logout logging through the same best-effort helper** used by login/registration；retain core Redis/PostgreSQL Session-revocation errors as blocking。
- [ ] **Step 4: Run `go test ./internal/module/auth/login -run 'TestLogout'` and the account repository tests**；confirm no auth path imports Asynq for login logging。

### Task 5: Update login-log API DTOs and front-end metadata

**Files:**
- Modify: `web/src/api/user/loginLog.ts`
- Modify: `web/src/views/user/loginLog/index.vue`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Test: `web/tests/api/user/loginLog.test.ts`
- Test: `web/tests/views/user/loginLog/index.test.ts`

**Interfaces:**
- Define `LoginLogEventType = 1 | 2 | 3` and `LoginLogType = 1 | 2 | 3`；`LoginLogItem.loginType` is `LoginLogType | null`。
- `LoginLogPageInit.eventTypes/loginTypes` are `number[]` parsed against the fixed allowed sets。
- `LoginLogItem.account` replaces `loginAccount`；`sessionId` is removed from the accepted DTO keys。
- Query fields `eventType` and `loginType` use numeric select-v2 options；empty means no filter。

- [ ] **Step 1: Write failing parser tests** for integer PageInit arrays, integer row values, null logout login type, renamed `account`, removed `sessionId`, and rejection of old string values。
- [ ] **Step 2: Write failing view tests** for three event tags (`primary`、`success`、`danger`), login type labels, logout dash, login type filter options, and query params using integers。
- [ ] **Step 3: Implement strict API parsers** with exact keys and fixed enum checks；do not coerce numeric strings。
- [ ] **Step 4: Update the view** to use typed metadata, add the login type filter/column, map labels through i18n, render logout login type as `-`，and keep status tag semantics separate。
- [ ] **Step 5: Run `pnpm vitest run tests/api/user/loginLog.test.ts tests/views/user/loginLog/index.test.ts --pool=threads --maxWorkers=1` and `pnpm typecheck`。

### Task 6: Remove old protocol references and refresh integration contracts

**Files:**
- Modify: `server/internal/module/user/loginLog/repository_test.go`
- Modify: `server/internal/module/user/loginLog/model_test.go`
- Modify: `server/internal/module/user/loginLog/repository_test.go`
- Modify: `server/internal/module/user/loginLog/event_test.go`
- Modify: `server/internal/module/user/loginLog/handler_test.go`
- Modify: `web/tests/api/user/loginLog.test.ts`
- Create: `web/tests/views/user/loginLog/index.test.ts`

**Interfaces:**
- No production path may send or parse old string event/login values, `login_account`, or login-log `session_id`。
- Authentication platform configuration remains string-based (`email|phone|password`)；only the audit log protocol is numeric。

- [ ] **Step 1: Run `rg -n 'login_account|sessionId|eventType.*login|loginType.*password' server web --glob '*.{go,ts,vue}'` and classify matches in the listed login-log files versus authentication-platform files; do not change authentication-platform string contracts。
- [ ] **Step 2: Update only audit-protocol consumers and fixtures**；leave auth-platform login configuration strings unchanged。
- [ ] **Step 3: Run the login-log, auth-login, database migration and API contract tests together** to catch cross-layer envelope drift。

### Task 7: Run verification and record the delivery state

**Files:**
- Modify: `docs/agent/STATUS.md`

- [ ] **Step 1: Run backend formatting and focused checks**:

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./internal/module/user/loginLog ./internal/module/auth/login ./internal/module/user/account ./internal/database -count=1
go build ./...
```

- [ ] **Step 2: Run frontend checks**:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/user/loginLog.test.ts tests/views/user/loginLog/index.test.ts --pool=threads --maxWorkers=1
pnpm typecheck
pnpm build
```

- [ ] **Step 3: Verify the real PostgreSQL catalog** after maintainer-approved migration: columns/types/nullability, checks, foreign keys, indexes, row counts and converted distributions; verify no `session_id` remains in `user_login_log`。
- [ ] **Step 4: Run `git diff --check` and `git status --short`**, confirm staged/unstaged work from other developers remains untouched, and inspect the diff for secrets or generated artifacts。
- [ ] **Step 5: Update `docs/agent/STATUS.md`** with actual migration execution, test commands/results, unrun commands and remaining risk. Do not commit unless the maintainer explicitly authorizes it。
