# 通用定时任务管理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立 PostgreSQL 权威的通用 schedule/job/run 调度系统，接管通知批次和两类历史清理，提供可观测的管理页面，并删除通知专用 dispatch outbox 与临时触发器。

**Architecture:** `system/scheduler` 提供静态任务目录、Cron Scanner、PostgreSQL Job Publisher、Worker 执行状态机和管理 API；Asynq/Redis 只承担运输。业务模块通过显式注入的事务接口创建一次性 Job，不能访问调度器内部数据库查询。API 与 Worker 在 `cmd/api/main.go`、`cmd/worker/main.go` 显式装配全部依赖。

**Tech Stack:** Go 1.26.5、Gin 1.12、GORM/PostgreSQL、Asynq 0.26、Redis、Vue 3.5、TypeScript 6、Element Plus、Vitest 4、PowerShell migration runner。

**Spec:** `docs/superpowers/specs/2026-09-19-generic-scheduler-design.md`

## Global Constraints

- PostgreSQL 是 schedule、job、run 的唯一运行时事实来源；Redis/Asynq 只负责运输和唤醒。
- Cron 只接受五段表达式，最小周期为每分钟；计划保存 IANA 时区，默认 `Asia/Shanghai`。
- 同一 Schedule 最多一个活动 Job；漏跑时间点合并为一次恢复执行，不批量补跑。
- 管理 CRUD 使用 last-write-wins；不得新增 scheduler revision、万能 Manager、Adapter、Factory 或隐式注册器。
- 任务 Handler、队列、超时、重试、参数 schema 由代码静态目录决定；管理员不得输入 Handler、Asynq 类型或任意 JSON。
- 不保留旧调度协议、双读双写或兼容分支；通知 dispatch outbox 迁移完成后必须删除。
- 所有请求和任务入口的 `context.Context` 必须贯穿 Service、Repository、GORM、Redis、Queue 和第三方 I/O。
- 不自动 commit、push、pull、rebase 或修改历史；执行 AI 用 `git diff` 作为阶段检查，不创建提交。
- 未经维护者明确要求，不由 Agent 运行前端全量测试；计划中列出命令供执行侧和维护者验收。

---

### Task 1: 建立调度领域契约与静态任务目录

**Files:**
- Create: `server/internal/module/system/scheduler/model.go`
- Create: `server/internal/module/system/scheduler/protocol.go`
- Create: `server/internal/module/system/scheduler/catalog.go`
- Create: `server/internal/module/system/scheduler/cron.go`
- Create: `server/internal/module/system/scheduler/errors.go`
- Test: `server/internal/module/system/scheduler/catalog_test.go`
- Test: `server/internal/module/system/scheduler/cron_test.go`

**Interfaces:**
- Produces `TaskDefinition`, `TaskCatalog`, `ScheduleStatus`, `JobStatus`, `RunStatus`, `TriggerSource` and validated request/value types for later tasks.
- `TaskDefinition` must expose `Type`, `DisplayName`, `AdminCreatable`, `BuiltinKey`, `DefaultParams`, `ValidateParams`, `Queue`, `Timeout`, `MaxAttempts`, `Backoff` and an execution callback; it must not expose a user-supplied Handler string.
- `TaskCatalog.Lookup(taskType string) (TaskDefinition, bool)` and `TaskCatalog.ValidatePayload(taskType string, payload json.RawMessage) error` are the only task-type lookup APIs.
- `ParseCron(expression string) (cron.Schedule, error)` accepts exactly five fields, rejects seconds and expressions faster than one minute; `LoadTimezone(name string) (*time.Location, error)` accepts only `time.LoadLocation` IANA names.

- [ ] **Step 1: Write failing contract tests** for five-field acceptance, six-field rejection, sub-minute rejection, invalid IANA timezone, unknown task type, invalid payload and duplicate static task definitions.
- [ ] **Step 2: Run the focused package tests** with `go test ./internal/module/system/scheduler -run 'Test(ParseCron|TaskCatalog|LoadTimezone)'`; confirm the new package has failing tests because the contracts are not implemented.
- [ ] **Step 3: Implement the minimal domain types and static catalog**. Construct definitions explicitly in `NewTaskCatalog(definitions ...TaskDefinition)` and reject duplicate type/builtin keys; do not use `init`, reflection or package-global registration.
- [ ] **Step 4: Implement Cron/timezone validation** with `robfig/cron/v3` standard five-field parser and a minimum-next-fire interval check.
- [ ] **Step 5: Re-run the focused package tests** and `go vet ./internal/module/system/scheduler`; all contract tests must pass.

### Task 2: Add PostgreSQL schema, seed settings, built-in schedules and RBAC migration

**Files:**
- Create: `docs/database/2026-09-19-system-scheduler.sql`
- Create: `docs/database/2026-09-19-system-scheduler.ps1`
- Create: `server/cmd/system-scheduler-migration/main.go`
- Test: `server/internal/database/system_scheduler_migration_test.go`
- Modify: `docs/database/current.sql` only after the real migration succeeds

**Interfaces:**
- Creates exactly `system_scheduler_schedule`, `system_scheduler_job` and `system_scheduler_run`.
- Creates `system.scheduler.history_retention_days` as a built-in numeric setting with default `30` and validation range `7..3650`; its generation/outbox update must use the existing cache-generation migration helpers.
- Seeds three non-deletable schedules: `realtime.retention.cleanup` (`*/1 * * * *`), `message.notification.retention.cleanup` (`*/1 * * * *`) and `system.scheduler.history.cleanup` (`30 3 * * *`), all in `Asia/Shanghai`.
- Seeds page/action menu permissions `system:scheduler:view/list/detail/create/update/status/delete/execute/retry` using the current lower-camel menu path and icon whitelist conventions.

- [ ] **Step 1: Write migration tests** asserting table columns, `created_at/updated_at`, JSONB defaults, Yes/No checks, status checks, partial unique indexes for active schedule name/builtin key, one active Job per schedule, `(task_type, source_key)` uniqueness, `(job_id, attempt_no)` uniqueness, foreign keys and retention setting bounds.
- [ ] **Step 2: Implement one transactional forward SQL migration**. Define explicit `BIGINT` identity/sequence keys, `deleted_at` only on schedules, `JSONB NOT NULL DEFAULT '{}'::jsonb`, UTC `TIMESTAMPTZ`, and indexes for due schedules, publishable jobs, expired leases and terminal history cleanup. Do not leave `message_notification_dispatch_outbox` in the final schema.
- [ ] **Step 3: Implement the PowerShell runner and Go probe** with `-OldAPIStopped`, transactional SQL execution, exact structure fingerprint checks, idempotent seed checks and a preflight that fails if the old API/Worker is still running.
- [ ] **Step 4: Add Redis cleanup to the runner**. After SQL commit, use Asynq `Inspector` to enumerate every queue's pending/scheduled/retry tasks, delete only tasks whose type is `message:notificationtask:dispatch:v1`, and print per-queue deleted/remaining counts; never delete unrelated queues or use a broad Redis `KEYS`/`FLUSHDB`.
- [ ] **Step 5: Add migration tests for rollback and rerun**: SQL failure leaves all three tables/seeds unchanged and does not run cleanup; a second complete run leaves structure, setting generation, menus and cleanup counts unchanged.
- [ ] **Step 6: Do not execute the migration yet.** Run only `powershell.exe -NoProfile -Command "[System.Management.Automation.Language.Parser]::ParseFile('docs/database/2026-09-19-system-scheduler.ps1',[ref]$null,[ref]$null)"` and the migration package unit tests.

### Task 3: Implement schedule/job/run repositories and transactional state transitions

**Files:**
- Create: `server/internal/module/system/scheduler/repository.go`
- Create: `server/internal/module/system/scheduler/repository_test.go`
- Create: `server/internal/module/system/scheduler/query_test.go`

**Interfaces:**
- `Repository` owns only PostgreSQL and exposes transaction-safe methods:
  - `CreateSchedule`, `UpdateSchedule`, `SetScheduleEnabled`, `DeleteSchedule`, `ListSchedules`, `GetSchedule`.
  - `ClaimDueSchedules(ctx, now, limit) ([]DueSchedule, error)` where each returned row is locked with `FOR UPDATE SKIP LOCKED`.
  - `CreateJobTx`, `FindActiveJobTx`, `ClaimPublishableJobs`, `MarkQueued`, `ReschedulePublish`, `ClaimRun`, `CompleteRun`, `FailRun`, `CancelQueuedJobs`, `CreateRetryJob`, `ListJobs`, `GetJob`, `ListRuns`.
  - `CleanupTerminalHistory(ctx, cutoff, limit) (deletedJobs, deletedRuns int, err error)` deletes only terminal Jobs whose Runs are terminal and uses small batches.
- Every update uses conditional status/token predicates and returns a typed lost-claim/not-found/conflict error.

- [ ] **Step 1: Add repository tests against the existing PostgreSQL test harness** for create/list filters, active schedule uniqueness, one active Job per schedule, source-key idempotency, run attempt uniqueness and terminal cleanup not touching active rows.
- [ ] **Step 2: Add concurrency tests** with two GORM sessions claiming the same due schedule, publishing the same Job, and starting the same attempt; assert exactly one winner and no duplicate Run.
- [ ] **Step 3: Implement SQL queries** with `WithContext(ctx)`, `FOR UPDATE SKIP LOCKED`, partial-index-compatible predicates and explicit `RowsAffected` checks; no repository method may read Redis or Asynq.
- [ ] **Step 4: Implement schedule update semantics**: enabled Cron/timezone edits recompute `next_run_at` from `now`, disabling sets no new due point, re-enabling computes from current time, and existing Jobs are untouched.
- [ ] **Step 5: Run `go test ./internal/module/system/scheduler -run 'TestRepository|TestConcurrent'` and `go vet ./internal/module/system/scheduler`.

### Task 4: Implement Cron Scanner and missed-run recovery

**Files:**
- Create: `server/internal/module/system/scheduler/scanner.go`
- Create: `server/internal/module/system/scheduler/scanner_test.go`

**Interfaces:**
- `Scanner.Run(ctx context.Context) error` performs bounded polling and exits cleanly on cancellation.
- `Scanner.RunOnce(ctx context.Context, now time.Time) (int, error)` claims due schedules, creates at most one Job per schedule and advances `next_run_at` to the next future Cron occurrence.
- `ScheduleJobBuilder` validates `TaskDefinition.DefaultParams` and writes a frozen payload snapshot.

- [ ] **Step 1: Write deterministic tests** for one due schedule, two concurrent scanners, multiple missed Cron points collapsing to one Job, disabled schedule, active Job conflict and timezone-aware next occurrence.
- [ ] **Step 2: Implement one-transaction schedule claim**: lock due schedules, check active Job, insert a Job with `trigger_source='cron'`, then advance `next_run_at`; use the unique constraints as the final duplicate guard.
- [ ] **Step 3: Implement bounded polling** with a one-second idle wait, batch size 50, immediate next batch when full, and no unbounded in-memory schedule list.
- [ ] **Step 4: Run focused scanner tests and verify a schedule whose next time is 10 years away is not deleted or treated as expired.**

### Task 5: Implement durable Job Publisher

**Files:**
- Create: `server/internal/module/system/scheduler/publisher.go`
- Create: `server/internal/module/system/scheduler/publisher_test.go`

**Interfaces:**
- `Publisher.Run(ctx context.Context) error` and `Publisher.RunOnce(ctx context.Context, now time.Time) (int, error)`.
- Asynq payload is `{schemaVersion, jobId, attempt, dispatchToken}`; Task ID is `system-scheduler:<jobId>:<attempt>`.
- The publisher depends on a narrow `QueueEnqueuer.Enqueue(ctx, taskType, payload, options)` interface; it does not import business modules.

- [ ] **Step 1: Write tests** for successful enqueue, Redis failure/backoff, Asynq `ErrTaskIDConflict`, lost publish claim, and enqueue-success/database-update failure followed by lease recovery.
- [ ] **Step 2: Implement claim/publish/mark flow** using a token lease and conditional PostgreSQL updates; keep Jobs `scheduled` on dependency failure and cap exponential backoff at five minutes.
- [ ] **Step 3: Ensure queue options come only from the static TaskDefinition** and include timeout/max retry; administrator payload cannot change them.
- [ ] **Step 4: Run `go test ./internal/module/system/scheduler -run 'TestPublisher'` and a Redis-backed queue client unit test without starting the Worker.

### Task 6: Implement Worker execution, retries and stale-message handling

**Files:**
- Create: `server/internal/module/system/scheduler/worker.go`
- Create: `server/internal/module/system/scheduler/worker_test.go`
- Create: `server/internal/module/system/scheduler/task.go`

**Interfaces:**
- `WorkerHandler.ProcessTask(ctx context.Context, task *asynq.Task) error` decodes the scheduler envelope and calls `Executor.Execute`.
- `Executor` claims the Job/Run in PostgreSQL, invokes the catalog definition, then conditionally completes or fails the attempt.
- `Register(mux *asynq.ServeMux, handler *WorkerHandler)` registers only the scheduler envelope TaskType; business task definitions are selected from `TaskCatalog` after database lookup.

- [ ] **Step 1: Write tests** for scheduled/queued/running transitions, unknown task type, invalid payload, duplicate delivery, stale attempt token, canceled Job, successful completion, retryable failure, final failure and context cancellation.
- [ ] **Step 2: Implement transactional Run creation and Job claim** with an execution lease and Worker instance ID; never call a business Handler while holding a database transaction.
- [ ] **Step 3: Implement failure classification** from the catalog retry policy, persist a bounded sanitized error, and return an Asynq error only when the scheduler Job state still requires transport retry.
- [ ] **Step 4: Implement lease recovery**: an expired running lease closes the Run as `failed/worker-lost` and requeues or fails the Job exactly once.
- [ ] **Step 5: Run focused Worker tests and `go test ./internal/module/system/scheduler -run 'TestWorker'`.

### Task 7: Wire built-in retention handlers and scheduler history cleanup

**Files:**
- Create: `server/internal/module/system/scheduler/builtin.go`
- Create: `server/internal/module/system/scheduler/builtin_test.go`
- Read: `server/internal/module/realtime/retention.go` and `server/internal/module/message/notification/retention.go` for their existing bounded `Cleanup` service contracts
- Modify: `server/internal/shared/setting/setting.go`
- Modify: `server/internal/module/system/setting/service.go`
- Modify: `server/internal/module/system/setting/service_test.go`

**Interfaces:**
- `BuiltinDefinitions(realtimeCleaner, notificationCleaner, historyCleaner) []TaskDefinition` returns the three Cron definitions with fixed task types and no administrator parameters.
- Realtime cleanup calls the existing bounded retention service and never deletes notification tables.
- Notification cleanup calls the existing bounded retention service and never deletes schedule tables.
- Scheduler history cleanup reads `system.scheduler.history_retention_days` through the existing setting reader and calls `CleanupTerminalHistory` in bounded batches.

- [ ] **Step 1: Write tests** proving each handler is idempotent, respects its own retention setting, does not cross module tables, and treats PostgreSQL failure as a failed Job.
- [ ] **Step 2: Implement the three definitions** with fixed queue/timeout/retry policies and no editable parameter form.
- [ ] **Step 3: Add a history cleanup test** for 6-day/7-day/3650-day boundaries, active Job preservation and retry after a database error.
- [ ] **Step 4: Run `go test ./internal/module/system/scheduler ./internal/module/realtime ./internal/module/message/notification`.

### Task 8: Replace notification dispatch outbox with Scheduler Jobs

**Files:**
- Modify: `server/internal/module/message/notificationTask/model.go`
- Modify: `server/internal/module/message/notificationTask/repository.go`
- Modify: `server/internal/module/message/notificationTask/service.go`
- Modify: `server/internal/module/message/notificationTask/task.go`
- Modify: `server/internal/module/message/notificationTask/task_handler_test.go`
- Modify: `server/internal/module/message/notificationTask/task_test.go`
- Delete: `server/internal/module/message/notificationTask/dispatch.go`
- Delete: `server/internal/module/message/notificationTask/dispatch_test.go`

**Interfaces:**
- Inject a narrow `BatchJobWriter` into notification service/processor with `CreateBatchJobTx(ctx, tx, taskID, batchNo, availableAt) error` and `CancelBatchJobsTx(ctx, tx, taskID) error`.
- The implementation is the scheduler repository; notification code does not import scheduler SQL or call Asynq directly.
- Scheduler payload remains `{taskId, batchNo}` and source key is `notificationTask:<taskId>:<batchNo>`.

- [ ] **Step 1: Write failing notification tests** for immediate submit creating batch 0, future submit preserving `scheduled_at`, each additional 500-user batch creating exactly one next Job, cancellation canceling only not-started Jobs and duplicate processing being a no-op.
- [ ] **Step 2: Implement transactionally frozen Job creation** in submit and batch completion paths; remove `DispatchOutbox` model and all claim/mark/reschedule methods.
- [ ] **Step 3: Make the notification task Handler a Scheduler catalog callback** that receives the frozen Job payload and calls the existing Processor; preserve platform/role/user audience semantics.
- [ ] **Step 4: Run all notificationTask tests and the realtime event uniqueness tests before deleting the old database table.

### Task 9: Assemble API/Worker dependencies and remove temporary runners

**Files:**
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/worker/main.go`
- Modify: `server/internal/module/message/notificationTask/task.go`
- Delete: `server/internal/module/realtime/retentionTrigger.go`
- Delete: `server/internal/module/message/notification/retentionTrigger.go`
- Modify or delete their focused tests

**Interfaces:**
- API assembly creates scheduler Repository, Catalog and Service, then injects the `BatchJobWriter` into notification Task Service.
- Worker assembly creates scheduler Repository, Catalog, Scanner, Publisher, Executor and built-in definitions; it registers one scheduler envelope Handler plus existing operation-log Handler.
- Worker Runners become `config-generation`, `realtime-outbox`, `scheduler-scanner`, `scheduler-publisher`; no notification dispatch relay or retention trigger remains.

- [ ] **Step 1: Add assembly tests** that reject nil scheduler dependencies and assert all named runners are present exactly once.
- [ ] **Step 2: Wire API and Worker explicitly** without a DI container or runtime module registration.
- [ ] **Step 3: Delete old relay/trigger files and remove all references with `rg -n "DispatchRelay|RetentionTrigger|message_notification_dispatch_outbox" server`.
- [ ] **Step 4: Run `go test ./cmd/api ./cmd/worker ./internal/module/message/notificationTask ./internal/module/system/scheduler` and `go vet ./...`.

### Task 10: Implement scheduler admin API and RBAC enforcement

**Files:**
- Create: `server/internal/module/system/scheduler/handler.go`
- Create: `server/internal/module/system/scheduler/route.go`
- Create: `server/internal/module/system/scheduler/request.go`
- Create: `server/internal/module/system/scheduler/response.go`
- Create: `server/internal/module/system/scheduler/service.go`
- Create: `server/internal/module/system/scheduler/handler_test.go`
- Create: `server/internal/module/system/scheduler/service_test.go`
- Create: `server/internal/module/system/scheduler/route_test.go`

**Interfaces:**
- Routes under `/api/admin/v1/system/scheduler`:
  - `GET /schedule`, `GET /schedule/:id`, `POST /schedule`, `PUT /schedule/:id`, `PATCH /schedule/:id/status`, `DELETE /schedule/:id`, `POST /schedule/:id/execute`.
  - `GET /job`, `GET /job/:id`, `GET /job/:id/run`, `POST /job/:id/retry`.
- Each route uses the exact matching `system:scheduler:*` action; `view` never implies an action.
- API returns the existing `{code,data,message}` envelope and strictly parses `unknown` request JSON.

- [ ] **Step 1: Write handler/route tests** for exact paths, permission middleware, malformed query/body, unknown task type, invalid Cron/timezone, built-in delete rejection, active-job execute conflict and retry authorization.
- [ ] **Step 2: Implement Service rules**: admin-editable fields are name/description/Cron/timezone/allowed params; task type and queue policy are catalog-owned; manual execute leaves `next_run_at` unchanged; retry clones the frozen payload into a new Job.
- [ ] **Step 3: Implement cursor/pagination queries** for schedules, Jobs and Runs with stable `(created_at,id)` ordering and bounded page size; do not load all history into memory.
- [ ] **Step 4: Run focused scheduler API tests and `go test ./internal/module/system/scheduler`.

### Task 11: Build the Admin Vue scheduler pages

**Files:**
- Create: `web/src/api/system/scheduler.ts`
- Create: `web/src/views/system/scheduler/index.vue`
- Create: `web/src/views/system/scheduler/components/ScheduleFormDialog/index.vue`
- Create: `web/src/views/system/scheduler/components/JobDetailDialog/index.vue`
- Create: `web/src/views/system/scheduler/components/RunTimeline/index.vue`
- Create: `web/src/views/system/scheduler/index.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

**Interfaces:**
- API DTOs use lower camel case and strict `unknown` parsing; no `any`, `as any` or broad declaration.
- Schedule page uses `AppTable`, `AppDialog`, `el-tabs` for active/terminal Job filters, `el-select-v2` for task type/timezone/status options, and action visibility from explicit permission codes.
- “新建” is hidden when the API returns no `adminCreatable` task definition; built-in plans show a non-deletable marker.
- Schedule form displays only catalog-declared parameter fields; no raw JSON editor or Handler input.
- Job detail shows status, trigger source, schedule, payload summary, and Run timeline; retry button is shown only for final failures and `system:scheduler:retry`.

- [ ] **Step 1: Write Vitest tests** for schedule list, built-in delete disabled, Cron/timezone validation display, manual execute success/conflict notifications, Job status tabs, Run timeline and retry refresh.
- [ ] **Step 2: Implement the typed API module** with list/detail/create/update/status/delete/execute/job/run/retry methods and response validation.
- [ ] **Step 3: Implement the page and dialogs** using existing shared components, explicit button `type`, loading/empty/error states and request-layer single notifications.
- [ ] **Step 4: Add Chinese/English copy and route/menu labels; ensure no hard-coded user-facing status text remains.
- [ ] **Step 5: Run targeted tests with `pnpm vitest run tests/views/system/scheduler/index.test.ts --pool=threads --maxWorkers=1` and run `pnpm typecheck`.

### Task 12: Add menu catalog, permission fixtures and API contract coverage

**Files:**
- Modify: `server/internal/module/permission/menu/schema_migration_test.go`
- Modify: `server/internal/module/permission/menu/tree_test.go`
- Modify: `server/internal/module/permission/access/service_test.go`
- Modify: `server/internal/module/permission/access/middleware_test.go`

**Interfaces:**
- Page path/component path use `system/scheduler`, resource code uses `system:scheduler:view`, and action codes are the exact list from the migration.
- Menu seed must preserve page/action independence, Admin menu version increment semantics and Canvas menu version stability.

- [ ] **Step 1: Add permission tests** for page-only, action-only, hidden route, and full-role combinations; assert action permission does not create a page route.
- [ ] **Step 2: Verify the migration menu tree uses a valid Lucide icon and lower-camel module path.
- [ ] **Step 3: Run `go test ./internal/module/permission/...` and the existing frontend architecture checker.

### Task 13: Add cross-module failure and capacity tests

**Files:**
- Create: `server/internal/module/system/scheduler/integration_test.go`
- Modify: `server/internal/module/message/notificationTask/integration_test.go`
- Modify: `server/internal/module/realtime/retention_test.go`
- Modify: `server/internal/module/message/notification/retention_test.go`

**Interfaces:**
- Tests use the existing PostgreSQL/Redis test harness and query counter; no fake in-memory database is used for transaction/concurrency claims.
- Test fixtures must model one million-user behavior through 500-user batches without inserting one million rows.

- [ ] **Step 1: Test two Worker instances** scanning/publishing/executing the same schedule and assert one Job, one Run per attempt and no duplicated notification event.
- [ ] **Step 2: Test Redis failure** during publish, Redis data loss after enqueue, PostgreSQL failure after enqueue and Worker crash lease recovery; assert no false success and bounded retry.
- [ ] **Step 3: Test notification flows** for immediate, future scheduled, role snapshot, cancellation before due, batch continuation and final failure/retry.
- [ ] **Step 4: Test each retention handler** with no expired rows, one expired batch, more than one batch, setting boundary and repeated invocation.
- [ ] **Step 5: Add SQL budget assertions**: hot schedule list uses bounded indexed queries, one due scan does not query per user, and job detail loads one Job plus bounded Run rows.
- [ ] **Step 6: Run the backend focused integration command `go test -p 1 ./internal/module/system/scheduler ./internal/module/message/notificationTask ./internal/module/realtime ./internal/module/message/notification -count=1`.

### Task 14: Remove old schema and perform the real migration

**Files:**
- Modify: `docs/database/current.sql` after migration
- Do not create compatibility files or fallback readers

**Interfaces:**
- Runner must stop on any active API/Worker process, require `-OldAPIStopped`, apply the forward SQL transaction, seed three schedules/settings/menus, remove `message_notification_dispatch_outbox`, delete only old notification Asynq task types, and verify zero old task types remain.

- [ ] **Step 1: Stop API/Worker outside the migration command and record the process check output.
- [ ] **Step 2: Execute `powershell.exe -NoProfile -File docs/database/2026-09-19-system-scheduler.ps1 -OldAPIStopped` from `D:\admin`.
- [ ] **Step 3: Re-run the same command and compare structure/settings/menu/generation fingerprints; every fingerprint and seed count must remain unchanged.
- [ ] **Step 4: Run the runner's cleanup-only mode and verify old Asynq task type count is zero without deleting unrelated queue tasks.
- [ ] **Step 5: Export the real schema-only snapshot to `docs/database/current.sql` and run `git diff --check`.

### Task 15: Final verification and handoff

**Files:**
- No new implementation files; update `docs/agent/STATUS.md` with actual completion, migration timestamp, commands and remaining risks.

- [ ] **Step 1: Run backend formatting/build gates:** `cd D:\admin\server; go fmt ./...; go test -p 1 ./... -count=1; go vet ./...; go build ./...`.
- [ ] **Step 2: Provide frontend verification commands to the maintainer:** `cd D:\admin\web; pnpm format:check; pnpm lint; pnpm check:architecture; pnpm typecheck; pnpm vitest run --pool=threads --maxWorkers=1; pnpm build`.
- [ ] **Step 3: Run browser/E2E checks with API and Worker started by the maintainer**: create/edit/disable/enable built-in schedule, manual execute conflict, inspect active/completed Job, view Run timeline, retry final failure, submit immediate/future notification and verify every batch appears in execution records.
- [ ] **Step 4: Verify Redis failure behavior** by stopping Redis only after creating a Job, confirm PostgreSQL Job remains recoverable, restart Redis, and confirm one eventual execution.
- [ ] **Step 5: Verify no old protocol remains:** `rg -n "DispatchRelay|message_notification_dispatch_outbox|realtime.*RetentionTrigger|notification.*RetentionTrigger|message:notificationtask:dispatch:v1" server web docs/database/current.sql` must return only migration/test audit text explicitly documenting deletion, never runtime code.
- [ ] **Step 6: Update `docs/agent/STATUS.md` with actual files, migration result, Redis cleanup counts, tests run by whom, and any deferred UI/E2E item. Do not commit.

## Self-Review Checklist

- Spec coverage: schema/state machine (Tasks 1–3), scanner/publisher/worker recovery (Tasks 4–6), built-ins (Task 7), notification takeover and old relay removal (Tasks 8–9), API/RBAC/UI (Tasks 10–12), failure/capacity verification (Task 13), migration/cleanup/snapshot (Task 14), final gates/status (Task 15).
- No legacy compatibility path is planned; the old notification outbox is deleted only after the new Job path and integration tests pass.
- The plan never gives administrators control over Handler, queue, timeout, retry or arbitrary payload shape.
- Schedule lifetime and Job/Run history lifetime are separate; `3650` only bounds terminal history retention.
- No task introduces optimistic-lock revision or a global cross-module cache manager.
