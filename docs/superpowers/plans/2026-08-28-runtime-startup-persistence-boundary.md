# Runtime Startup Persistence Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make API and Worker runtime startup consume existing persistent state without migrating schemas, seeding business data, or clearing Redis state.

**Architecture:** Keep schema and migration helpers available only to isolated tests, but remove every state-mutating helper call from the API composition root. Delete the API-only menu seed catalog so no production startup seed definition remains. Protect the boundary with a source-level entrypoint test and full backend verification.

**Tech Stack:** Go 1.26, PostgreSQL, Redis, Asynq, standard `testing`

**Spec:** `docs/superpowers/specs/2026-08-28-runtime-startup-persistence-boundary-design.md`

## Global Constraints

- API and Worker startup must not execute DDL, data migration, seed, repair, Redis cleanup, or initialization tasks.
- Do not add a migrate, init, seed, or repair command.
- Preserve `cmd/bootstrap-admin` as an explicitly invoked business command.
- Preserve existing online request architecture and business behavior.
- Work directly on `main` as previously authorized; do not commit without a new explicit request.

---

### Task 1: Lock The Runtime Startup Boundary

**Files:**
- Modify: `server/cmd/api/main_test.go`
- Modify: `server/cmd/api/main.go`
- Delete: `server/cmd/api/menu_foundation.go`

**Interfaces:**
- Consumes: existing `database.Open`, `redis.Open`, `queue.NewClient`, repositories, services, handlers, and `buildRouter`.
- Produces: `run(*slog.Logger) error` with read/connect/compose/listen startup behavior only.

- [x] **Step 1: Replace startup seed tests with a failing forbidden-call test**

Read `main.go` and assert that none of these fragments exists:

```go
forbidden := []string{
    "database.PrepareDomainNames(", "auth.PrepareSessionSchema(", "operationlog.PrepareSchema(",
    "menu.PrepareSchema(", "database.AutoMigrate(", "authplatform.EnsureSchema(",
    "authplatform.EnsureCanvasPreset(", "menu.PreparePlatformSchema(", "role.EnsureSchema(",
    "auth.EnsureSchema(", "menu.EnsureSchema(", "access.EnsureSchema(",
    "operationlog.EnsureSchema(", "authplatform.ClearBuiltinPolicies(",
    "auth.CleanupLegacySessionPointers(", "menuService.EnsureFoundation(",
    "menuService.EnsurePlatformFoundation(", "roleService.EnsureSystemRoles(",
}
```

Also assert `database.Open(`, `projectredis.Open(`, `queue.NewClient(`, and `buildRouter(` remain in that order.

- [x] **Step 2: Run the focused test and verify RED**

Run:

```powershell
cd D:\admin\server
go test ./cmd/api -run TestRunDoesNotMutatePersistentStateDuringStartup -count=1
```

Expected: FAIL because `main.go` still contains forbidden migration, seed, and cleanup calls.

- [x] **Step 3: Remove startup mutations and seed assembly**

Delete the forbidden blocks from `run`, retain dependency construction used by online services, remove imports that become unused, and delete `menu_foundation.go`. Do not add replacement startup commands or fallback behavior.

- [x] **Step 4: Run focused tests and verify GREEN**

Run:

```powershell
cd D:\admin\server
go test ./cmd/api -count=1
```

Expected: PASS.

### Task 2: Verify The Complete Backend And Real Startup

**Files:**
- Verify: `server/...`
- Verify: `docs/superpowers/specs/2026-08-28-runtime-startup-persistence-boundary-design.md`
- Verify: `docs/superpowers/plans/2026-08-28-runtime-startup-persistence-boundary.md`

**Interfaces:**
- Consumes: the startup boundary implemented in Task 1.
- Produces: verified API/Worker binaries and evidence that API startup no longer reaches Canvas foundation.

- [x] **Step 1: Run formatting and static verification**

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
```

- [x] **Step 2: Run all backend tests and builds**

```powershell
cd D:\admin\server
go test ./...
go build ./...
```

- [x] **Step 3: Verify the production entrypoints contain no forbidden calls**

```powershell
cd D:\admin
rg -n "PrepareDomainNames|PrepareSessionSchema|PrepareSchema|PreparePlatformSchema|AutoMigrate|EnsureSchema|EnsureCanvasPreset|EnsureFoundation|EnsurePlatformFoundation|EnsureSystemRoles|ClearBuiltinPolicies|CleanupLegacySessionPointers" server/cmd/api/main.go server/cmd/worker/main.go
```

Expected: no matches.

- [x] **Step 4: Build and run a temporary API binary**

Build `./cmd/api` to the operating-system temporary directory, run it from `server` with the existing `.env`, confirm it starts listening instead of returning a Canvas foundation error, then stop only that temporary process and delete the temporary binary.

- [x] **Step 5: Inspect the final diff**

Run `git status --short`, `git diff --check`, and focused `git diff` commands. Confirm no user changes were reverted and no generated binary or environment file is included.
