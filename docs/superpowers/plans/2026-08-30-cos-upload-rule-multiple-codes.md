# COS Upload Rule Multiple Codes Implementation Plan

> **For agentic workers:** Execute inline in the current workspace. Do not use subagents for this task.

**Goal:** Allow one COS upload rule to own multiple stable upload codes entered with Element Plus `el-input-tag`, while preserving per-platform code uniqueness and using the requested code as the COS object-key prefix.

**Architecture:** Keep shared rule limits on `storage_upload_rule` and move stable codes into `storage_upload_rule_code`. The child table stores `rule_id`, `platform_id`, and `code`, with a unique `(platform_id, code)` constraint. Management APIs expose `codes: string[]`; credential issuance still accepts one `ruleCode`, joins the child table, and uses that exact code as the object prefix.

**Tech Stack:** PostgreSQL 18, Go/Gin/GORM, Vue 3, Element Plus, TypeScript, Vitest.

**Spec:** `docs/superpowers/specs/2026-08-29-cos-object-storage-design.md`

## Global Constraints

- Migration is manual, transactional, idempotent, and preserves every existing scalar code.
- Rule codes are normalized to lower-case trimmed values, are unique within one request, and remain immutable after creation.
- The database must reject duplicate codes within the same platform under concurrent requests.
- Credential requests remain `{ ruleCode, files }` and object keys start with the requested code.
- Frontend uses Element Plus `el-input-tag`; it does not emulate multiple codes by issuing multiple create requests.
- Existing unrelated worktree changes must remain intact.

---

### Task 1: Transactional Schema Migration

**Files:**
- Create: `docs/database/2026-08-30-cos-upload-rule-multiple-codes.sql`
- Create: `docs/database/2026-08-30-cos-upload-rule-multiple-codes.md`
- Modify: `server/internal/database/cos_object_storage_migration_test.go`

- [x] Add a failing migration test that expects `storage_upload_rule_code`, migrated historical codes, `ux_storage_upload_rule_code_platform_code`, and removal of `storage_upload_rule.code`.
- [x] Run `go test ./internal/database -run COSObjectStorageMigration -count=1` and confirm failure.
- [x] Add the transaction migration: create child table, copy existing codes, add constraints/indexes, then drop the old index and column.
- [x] Re-run the migration tests and confirm pass.

### Task 2: Backend Multi-Code Contract

**Files:**
- Modify: `server/internal/module/storage/uploadrule/model.go`
- Modify: `server/internal/module/storage/uploadrule/request.go`
- Modify: `server/internal/module/storage/uploadrule/repository.go`
- Modify: `server/internal/module/storage/uploadrule/service.go`
- Modify: `server/internal/module/storage/uploadrule/model_test.go`
- Modify: `server/internal/module/storage/uploadrule/handler_test.go`
- Modify: `server/internal/module/storage/uploadrule/service_test.go`
- Modify: `server/internal/module/storage/uploadrule/credential_test.go`

- [x] Add a failing DTO tests for `codes: [" avatar ", "article-cover", "avatar"]` normalizing to two values and for empty/unsafe codes being rejected.
- [x] Add failing service tests for atomic creation of one rule plus multiple code rows and conflict on any duplicate platform code.
- [x] Add failing credential tests proving an alias code resolves the same rule and becomes the object-key prefix.
- [x] Change create DTO/input/value from `Code string` to `Codes []string`; keep update DTO without codes.
- [x] Create the rule and child code rows in one transaction; aggregate codes in list/get queries; search by child code.
- [x] Join `storage_upload_rule_code` in `FindUploadTarget` and return the matched code.
- [x] Run `go test ./internal/module/storage/uploadrule -count=1` and confirm pass.

### Task 3: Frontend Element Plus InputTag

**Files:**
- Modify: `web/src/api/storage/uploadrule.ts`
- Modify: `web/src/views/cloud/storage-object/index.vue`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Modify: `web/tests/api/storage/uploadrule.test.ts`
- Modify: `web/tests/views/cloud/storage-object/index.test.ts`

- [x] Add failing tests expecting `codes: string[]`, `el-input-tag`, required validation, normalized create payload, immutable edit display, and codes rendered in the table.
- [x] Replace `ruleForm.code` with `ruleForm.codes`; use `el-input-tag` with clear placeholders and a required error.
- [x] Submit `codes` only on create, preserve immutable codes on update, and render codes as compact tags in the table.
- [x] Run the focused API/view tests and `npm run build`.

### Task 4: Migration Execution and Verification

**Files:**
- Execute: `docs/database/2026-08-30-cos-upload-rule-multiple-codes.sql`

- [x] Verify the existing table and scalar code column before migration.
- [x] Execute with `psql -v ON_ERROR_STOP=1` using the project DSN after removing the GORM-only `TimeZone` option.
- [x] Verify every active rule has at least one child code, the old column/index are absent, the child unique index exists, and no duplicate `(platform_id, code)` rows exist.

### Task 5: Final Verification

- [x] Run focused migration, upload-rule backend, frontend API/view/component tests.
- [x] Run `go test ./internal/module/storage/uploadrule ./internal/database -count=1`.
- [x] Run `npm run build` and `git diff --check`.
- [ ] Inspect the live rule dialog and confirm `el-input-tag` accepts multiple codes and the table displays them.
