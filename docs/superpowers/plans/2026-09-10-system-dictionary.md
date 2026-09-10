# 系统字典管理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在系统设置下交付一个全局、多语言、可缓存、受 RBAC 保护的字典管理基础能力，并提供严格的批量 options 查询接口；第一期不迁移现有业务页面。

**Architecture:** 新增 `system/dictionary` 领域，按 Router -> Middleware -> Handler -> Service -> Repository -> Model -> PostgreSQL 分层。字典类型和字典项是两个维护实体，业务 options 通过语言选择和 Redis 版本化快照读取；PostgreSQL 是唯一事实来源，写入使用现有 mutation lease/version 协议完成跨实例失效。前端新增严格 DTO API 和 Store，管理页只编排状态，业务页迁移留到后续计划。

**Tech Stack:** Go、Gin、GORM、PostgreSQL、Redis、Vue 3、TypeScript、Pinia、Element Plus `el-select-v2`、Vitest。

**Spec:** `docs/superpowers/specs/2026-09-10-system-dictionary-design.md`

## Global Constraints

- 字典全局共享，不增加 `platform_id`；平台拆分只适用于 COS 上传策略、邮件限流策略和登录 Session。
- 稳定程序协议继续使用 Go/TypeScript enum；字典只管理可配置业务选项。
- PostgreSQL 是唯一运行时事实来源；不得用空数组、默认对象、内存兜底或旧快照掩盖依赖错误。
- 维护表显式声明非空 `created_at`/`updated_at`；真实删除才使用 `deleted_at`，并使用部分唯一索引保护有效值。
- 前端 API 必须从 `unknown` 严格解析；业务代码禁止 `any`、`as any`、`@ts-ignore`。
- 下拉选择只能使用 `el-select-v2` 和显式强类型 `options`。
- page/action 权限独立；每个受保护 API 的 Middleware 使用与动作一致的权限码。
- 不自动执行 migration，不提交，不回滚其他开发者改动。

---

### Task 1: 固定工作区和契约基线

**Files:**
- Modify: `docs/agent/STATUS.md`
- Reference: `docs/agent/architecture.md`, `docs/agent/design.md`, `AGENTS.md`
- Test: `server/internal/module/permission` existing menu/access tests

**Interfaces:**
- Produces: 当前工作条目，记录全局字典范围、权限映射、缓存失效策略和本期不迁移现有页面的边界。

- [ ] **Step 1: Check worktree and current database connection configuration**

Run `git status --short` and inspect the existing PostgreSQL/Redis test configuration without changing it. Record unrelated changes and avoid touching them.

- [ ] **Step 2: Add the current-work entry**

在 `STATUS.md` 当前区域记录：目标、范围、验收、下一步；明确不按认证平台拆分，`isBuiltin` 只用于删除保护，第一期不改已有业务下拉。

- [ ] **Step 3: Verify the baseline architecture checks**

Run `cd D:\admin\web; pnpm check:architecture` and the targeted permission test package. Expected result is the existing baseline, with no new findings.

### Task 2: 设计并验证 PostgreSQL schema migration

**Files:**
- Create: `docs/database/2026-09-10-system-dictionary.sql`
- Modify: `docs/database/current.sql` only after the real migration succeeds
- Test: `server/internal/database` migration/structure test location used by recent migrations

**Interfaces:**
- Produces: `system_dictionary` and `system_dictionary_item` tables, constraints, indexes, and stable migration assumptions consumed by Models and Repositories.

- [ ] **Step 1: Inspect the live schema**

使用 PostgreSQL 只读查询确认 `system_*` 命名冲突、时间字段类型、现有 migration 记录/约定、菜单表约束和可用索引；不要以历史 SQL 代替真实结构。

- [ ] **Step 2: Write failing migration assertions**

覆盖表存在、非空时间列、`is_enabled`/`is_builtin` 的 `SMALLINT + CHECK`、字典 code 唯一、字典项 `(dictionary_id, value)` 有效记录唯一、外键删除行为和查询索引。

- [ ] **Step 3: Implement an idempotent forward migration**

创建 `system_dictionary` 与 `system_dictionary_item`；使用显式 `TIMESTAMPTZ`、`SMALLINT` 检查约束、外键和只服务真实列表/options 查询的索引。禁止启动时 AutoMigrate，禁止写入演示数据。

- [ ] **Step 4: Verify rollback behavior in an isolated PostgreSQL transaction**

故意制造重复 code、重复有效 value 或违反 `is_builtin` 规则的失败，确认整体回滚且原结构/数据保持不变；重复执行成功且不产生重复对象。

- [ ] **Step 5: Execute only with maintainer-controlled database procedure**

维护者执行 migration 后检查表、约束、索引和行数，再按项目流程刷新 `docs/database/current.sql`。Agent 不在 API/Worker 启动中执行 DDL。

### Task 3: 建立后端 Model、协议和 Repository

**Files:**
- Create: `server/internal/module/system/dictionary/model.go`
- Create: `server/internal/module/system/dictionary/protocol.go`
- Create: `server/internal/module/system/dictionary/repository.go`
- Create: `server/internal/module/system/dictionary/repository_test.go`

**Interfaces:**
- Consumes: migration tables from Task 2.
- Produces: typed model and repository methods for paginated types, detail/items, options source reads, create/update/status/delete and duplicate/dependency errors.

- [ ] **Step 1: Write repository contract tests**

覆盖分页排序、启用字典只读启用项、语言无关的 raw labels、有效 value 唯一、内置删除拒绝、删除前 item 依赖检查、context 贯穿 GORM，以及事务内更新时间维护。

- [ ] **Step 2: Define explicit models and request/response structs**

Model 只声明 GORM 映射和 `CreatedAt/UpdatedAt`；协议使用 `nameZh/nameEn/labelZh/labelEn/isEnabled/isBuiltin`，禁止暴露 GORM model 作为 API DTO。

- [ ] **Step 3: Implement repository methods with `WithContext(ctx)`**

所有查询、锁和写入仅访问 PostgreSQL；更新只允许标签、名称、描述、排序和状态，不允许改变 `code/value`；查询明确排除停用项和软删（若最终 schema 使用删除字段则按部分唯一规则处理）。

- [ ] **Step 4: Run repository tests against real PostgreSQL fixtures**

Run `cd D:\admin\server; go test ./internal/module/system/dictionary -count=1`. Expected: all repository contract tests pass and no SQLite/in-memory database is used.

### Task 4: 实现 Service 业务规则和 options 语言选择

**Files:**
- Create: `server/internal/module/system/dictionary/service.go`
- Create: `server/internal/module/system/dictionary/service_test.go`
- Modify: `server/internal/module/system/dictionary/repository.go` only for service-required narrow interfaces

**Interfaces:**
- Consumes: repository contracts from Task 3; request language from middleware context; existing Redis mutation/version interfaces.
- Produces: `List`, `Get`, `Create`, `Update`, `UpdateStatus`, `Delete`, `CreateItem`, `UpdateItem`, `UpdateItemStatus`, `DeleteItem`, `GetOptions(ctx, codes, language)`.

- [ ] **Step 1: Write failing service tests**

覆盖 code/value 格式和长度、不可改 code/value、内置删除拒绝、禁用字典/项查询拒绝、当前语言优先、中文回退、双语缺失错误、批量 code 全量返回、重复值错误和 context cancellation。

- [ ] **Step 2: Implement pure validation and label selection**

将语言归一化为 `zh`/`en`；`en` 缺失回退 `zh`，`zh` 仍缺失返回明确公开错误；未知 code、停用字典、无启用项和重复 code 不返回空成功。

- [ ] **Step 3: Implement write orchestration**

Service 负责事务意图、内置保护、依赖检查和 mutation lease 顺序；Repository 执行 SQL。写事务失败释放 lease；提交后 CAS 发布新字典版本，发布失败返回依赖错误并保留可恢复 invalidating 状态。

- [ ] **Step 4: Run service tests**

Run `go test ./internal/module/system/dictionary -run Service -count=1`. Expected: PASS with explicit errors for all invalid cases.

### Task 5: 接入 Redis options 快照和跨实例失效

**Files:**
- Create: `server/internal/module/system/dictionary/cache.go`
- Create: `server/internal/module/system/dictionary/cache_test.go`
- Modify: existing Redis state/version package only through its narrow public interface
- Reference: `server/internal/module/permission/state`, `shared/cacheFill` patterns

**Interfaces:**
- Consumes: `GetOptions` source data and existing Redis lease/version primitives.
- Produces: versioned options snapshot read/publish/invalidate behavior used by Service and API.

- [ ] **Step 1: Write failure-first Redis tests**

覆盖 cache hit 前 version/state confirmation、missing 冷重建单持有者、Redis error/corrupt/invalidating、旧 publisher CAS 拒绝、语言/code/schema 隔离和 bounded rebuild。

- [ ] **Step 2: Define key and payload schema**

使用包含 schema version、dictionary version、language 和 normalized code set 的 key/payload；payload 必须包含完整 labels/options，不能依赖 `json:"-"` 字段或进程内隐含状态。

- [ ] **Step 3: Implement bounded read/rebuild/publish**

命中前先确认 Redis `ready + version`；missing 才申请租约并回源 PostgreSQL；Redis 故障、快照损坏或失效中显式失败；发布同时校验版本，旧 writer 不得覆盖新快照。

- [ ] **Step 4: Run race-oriented package tests**

Run `go test ./internal/module/system/dictionary -run Cache -count=1` and, where the environment permits, `go test -race ./internal/module/system/dictionary`. Expected: concurrent rebuild has one owner and no stale publish.

### Task 6: 实现 Handler、Route 和显式依赖装配

**Files:**
- Create: `server/internal/module/system/dictionary/handler.go`
- Create: `server/internal/module/system/dictionary/route.go`
- Create: `server/internal/module/system/dictionary/handler_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/internal/module/system/route.go` or system route aggregator used by the repository

**Interfaces:**
- Consumes: Service methods from Task 4 and cache-aware options path from Task 5.
- Produces: HTTP envelope endpoints from the spec with request context propagation and precise input limits.

- [ ] **Step 1: Write handler binding tests**

覆盖重复 query 参数、空/超长 code、超过批量上限、非法 ID、缺失字段、无效 `isEnabled`、未知语言和统一 error envelope；断言 Service 收到 `c.Request.Context()` 而不是 `*gin.Context`。

- [ ] **Step 2: Implement handlers only for binding, service calls and output**

Handler 不访问 GORM、Redis 或 Storage；分页、body 和 query 经过白名单解析；options 请求按稳定排序规范化 code，重复 code 拒绝。

- [ ] **Step 3: Register routes with exact permission codes**

分别绑定 `system:dictionary:list/detail/create/update/status/delete`；options 使用 `system:dictionary:list`；不新增无权限公开路由。

- [ ] **Step 4: Assemble dependencies explicitly**

在 `cmd/api/main.go` 创建 Model/Repository/Service/Handler/Route，禁止容器、Manager、Factory 或运行时注册器。

- [ ] **Step 5: Run backend package and build checks**

Run `go fmt ./...; go vet ./...; go test ./internal/module/system/dictionary ./internal/module/system/...; go build ./...`. Expected: PASS.

### Task 7: 增加菜单、权限、i18n 和 Access 失效迁移

**Files:**
- Create: `docs/database/2026-09-10-system-dictionary-menu.sql`
- Modify: permission seed/fixture source used by current menu migrations
- Modify: `web/src/i18n/locales/zh-CN.ts`, `web/src/i18n/locales/en-US.ts` (actual locale paths after inspection)
- Test: permission migration and Access snapshot tests

**Interfaces:**
- Consumes: route permission codes from Task 6.
- Produces: hidden/visible menu node, action permissions, dynamic component path and updated menu version behavior.

- [ ] **Step 1: Write RBAC failure tests**

覆盖无 `view` 不能进入/注册页面、有 `view` 无 `list` 不能加载数据、action 不进入 `menuTree`、隐藏规则、角色授权保持、跨平台 menu version 只递增受影响平台、旧 snapshot namespace 不可复用。

- [ ] **Step 2: Add idempotent menu/permission migration**

新增系统设置下 `system/dictionary` page 和实际动作节点，保留现有 ID/授权关系；迁移失败整体回滚，重复运行不重复插入。

- [ ] **Step 3: Add Chinese and English i18n keys**

覆盖导航、页面标题、字段、状态、错误和确认文案；不把业务字典运行时 label 写入静态 i18n。

- [ ] **Step 4: Verify Access and menu version behavior**

Run targeted permission tests plus migration idempotency/conflict tests;确认 Redis Access namespace/version 按当前项目协议升级或兼容策略处理。

### Task 8: 开发前端严格 API、Store 和管理页面

**Files:**
- Create: `web/src/api/system/dictionary.ts`
- Create: `web/src/store/systemDictionary.ts` or the project’s existing store naming location
- Create: `web/src/views/system/dictionary/index.vue`
- Create: `web/src/views/system/dictionary/dictionaryRules.ts`
- Create: `web/src/views/system/dictionary/components/DictionaryDialog/index.vue`
- Create: `web/src/views/system/dictionary/components/DictionaryItemDialog/index.vue`
- Create: `web/tests/api/system/dictionary.test.ts`
- Create: `web/tests/views/system/dictionary/index.test.ts`

**Interfaces:**
- Consumes: API envelope and permissions from Tasks 6-7.
- Produces: typed `Dictionary`, `DictionaryItem`, `DictionaryOptions` and `useDictionaryOptions(codes)` consumer API.

- [ ] **Step 1: Write failing DTO parser tests**

覆盖 exact keys、numeric ID、`YesNo` flags、nullable/required fields、options map 不丢 code、label/value 类型错误和 envelope 错误。

- [ ] **Step 2: Implement strict API module**

所有请求经过 `utils/request.ts`；每个 response 从 `unknown` 解析，拒绝多余/缺失关键字段和未知值；不使用 `any` 或默认空数组掩盖协议错误。

- [ ] **Step 3: Implement Store/composable states**

按 code + language 管理 ready/loading/error；重复请求合并且失败可重试；Redis/API 错误不保留旧 options；提供明确的 `options(code)` 和加载状态类型。

- [ ] **Step 4: Implement management page**

使用 `AppSearch`、`AppTable`、`AppDialog` 和 `el-select-v2`；字典类型列表支持分页、创建、编辑、启停、删除；详情区管理字典项，支持排序、双语标签、启停、删除确认；按钮按精确 action 权限控制。

- [ ] **Step 5: Run frontend targeted checks**

Run `cd D:\admin\web; pnpm vitest run tests/api/system/dictionary.test.ts tests/views/system/dictionary/index.test.ts --pool=threads --maxWorkers=1; pnpm typecheck; pnpm lint; pnpm check:architecture`. Expected: PASS and zero architecture findings.

### Task 9: 端到端契约验证和交付收口

**Files:**
- Modify: `docs/agent/STATUS.md`
- Test: backend dictionary/RBAC/migration tests, frontend dictionary tests, optional Playwright scenario

**Interfaces:**
- Consumes: all prior tasks.
- Produces: verified first-release capability with explicit residual risks and no existing-page migration.

- [ ] **Step 1: Run backend shared verification**

在 `server` 运行 `go fmt ./...; go vet ./...; go test ./...; go build ./...`；若环境无 C 编译器，明确记录未运行 `-race`，不得声称已完成竞态验证。

- [ ] **Step 2: Run frontend shared verification**

在 `web` 运行 `pnpm vitest run --pool=threads --maxWorkers=1; pnpm build`，记录既有 chunk warning 与实际失败项。

- [ ] **Step 3: Exercise the critical workflow**

用 Playwright 或 API 集成测试验证：进入系统设置 -> 字典管理、无权限拒绝、创建双语字典、创建/停用项、中文/英文 options、删除保护、并发修改后的新版本可见。

- [ ] **Step 4: Update STATUS with evidence**

记录实际修改文件、migration 执行时间/结果、缓存与 RBAC 验证、未运行项和剩余风险；明确现有业务硬编码 options 尚未迁移。

- [ ] **Step 5: Review diff without committing**

Run `git status --short`, `git diff --check` and inspect changed files for secrets, generated artifacts, unrelated edits, incorrect platform scope, `el-select` usage, `any`, or stale permission codes.

## Self-Review Checklist

- [ ] 设计覆盖了全局字典、多语言回退、内置保护、稳定 value/code、权限、缓存和第一期不迁移页面。
- [ ] 计划包含 migration、Model/Repository/Service/Handler/Route、Redis、RBAC、前端 API/Store/View 和验证。
- [ ] 没有把字典误写成动态字段系统，也没有把 YesNo 等协议 enum 改成可配置数据。
- [ ] 所有任务都有具体文件、接口、测试和命令；未使用 TBD/TODO/占位步骤。
