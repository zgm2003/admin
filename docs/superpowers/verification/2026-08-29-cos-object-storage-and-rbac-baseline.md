# COS 对象存储与 RBAC 基线验证记录

## Environment

- Date: 2026-08-30 (Asia/Shanghai)
- OS: Windows
- Go: `go1.26.5 windows/amd64`
- Node.js: `v24.12.0`
- pnpm: `11.21.0`
- PostgreSQL client: `18.6`
- PostgreSQL: local listener confirmed on `127.0.0.1:5432`
- Redis: local listener confirmed on `127.0.0.1:6379`
- Repository: `D:\admin`

## Commands

### Backend

```text
go test ./... -count=1
PASS: all packages; 30 packages with tests passed, testschema has no test files.

go vet ./...
PASS

go build ./...
PASS
```

The full Go test run exercised the real PostgreSQL and Redis integration paths available in the
local environment; no package reported an integration skip.

### Frontend

```text
pnpm vitest run --pool=threads --maxWorkers=1
PASS: Test Files 56 passed (56), Tests 371 passed (371)
Duration: 330.84s

pnpm exec vue-tsc --noEmit
PASS

pnpm build
PASS (Vite emitted only the existing chunk-size warning)
```

The focused storage tests were also run before the full suite and passed.

### Static checks

- The forbidden-reference scan produced only intentional historical migration/spec/test references
  such as `canvas:test`, `auth_session`, and `foundation_task`; runtime API/Worker wiring has no
  taskdemo route, startup migration, seed, or Redis flush path.
- The TypeScript `any` scan was empty.
- `git diff --check` passed.

## Migration

Executed locally against the `admin` database on 2026-08-30, using `psql -v ON_ERROR_STOP=1`:

1. `docs/database/2026-08-29-admin-rbac-baseline.sql` — `BEGIN`, `DO`, `COMMIT`
2. `docs/database/2026-08-29-cos-object-storage.sql` — `BEGIN`, table/index creation, `DO`, `COMMIT`

The migrations completed successfully without starting or stopping any service. They remain manual,
transactional migrations; API and Worker startup do not invoke them. Production execution still
requires a reviewed backup and an operator-controlled maintenance window.

Post-migration read-only checks:

- `storage_cos_config` and `storage_upload_rule`: present
- Named storage constraints: 7 present
- Storage indexes including `ux_storage_upload_rule_platform_enabled`: 7 present
- Admin menu: `cloud` directory and `storage:object:list` page present
- Storage action menus under the page: 10 present

## Skipped tests

No automated backend or frontend test was skipped in the commands above. Manual browser/API
acceptance was not run because it requires an authenticated running application session; the user
will restart the backend after this migration step.

## Acceptance

- Automated backend, frontend, type-check, build, static-reference, and whitespace checks: **pass**.
- COS configuration and upload-rule API contracts, encryption boundary, platform isolation, atomic
  per-platform enablement, presigned PUT generation, and Admin two-tab rendering are covered by the
  passing focused and full test suites.
- Manual checks requiring an authenticated browser session remain pending: hidden profile access,
  locale persistence, cross-platform
  access snapshots, menu expansion behavior, concurrent default-role switching, COS secret redaction
  in the UI, direct COS upload, and restart-without-schema-change behavior.
