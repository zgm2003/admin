# 配置缓存状态页面减法实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改任何跨层业务标识的前提下，让配置缓存代际页面只展示管理员需要的同步状态，并正确表达 Redis 被清空后的可恢复状态。

**Architecture:** 保留 `system/cacheGeneration` 的页面目录、API 路径、Go 模块、数据库表、Redis namespace 和权限码。后端继续以 PostgreSQL generation 为事实、Redis state 为运行状态；前端把 namespace/scope/status 转换成中文业务文案，将技术计数和代际信息收进详情区域。

**Tech Stack:** Go、Gin、GORM、PostgreSQL、Redis、Vue 3、TypeScript、Vitest、Element Plus。

**Spec:** `docs/agent/design.md`、`docs/agent/architecture.md` 以及 2026-10-10 对“配置缓存代际页面不改内部名称、只做界面减法”的确认。

## Global Constraints

- 页面可见标题“配置缓存代际”保持不变；本轮只优化列名、状态名和详情说明，不把它改成新的数据库、API、模块或权限名称。
- 不修改 `system_config_cache_generation`、`system_config_cache_outbox` 表，不执行迁移，不清理 Redis。
- PostgreSQL generation 是事实来源；Redis 缺少 state 只能标记为可恢复的 `missing`，不能伪造 ready，也不能把数据库错误隐藏成空数据。
- `corrupt`、`unavailable` 仍然是异常状态；`missing` 在页面上使用等待重建语义和 warning/info 样式。
- 技术字段仍保留在 API DTO 中，避免破坏协议；只调整页面主表布局，详情区域继续显示原始值用于排障。
- 不新增手工“重建缓存”按钮；恢复依靠现有有界 cold-fill/relay 协议。

---

### Task 1: 复现并锁定 Redis state 缺失语义

**Files:**
- Inspect/Modify: `server/internal/module/system/cacheGeneration/service.go`
- Test: `server/internal/module/system/cacheGeneration/service_test.go`
- Inspect/Modify: `server/internal/shared/cacheFill/lease.go`
- Test: `server/internal/shared/cacheFill/lease_test.go`

**Interfaces:**
- 保持现有 `CacheGenerationStatus` DTO 和 `missing` 值不变。
- `resolveStatus` 在 Redis state 缺少时继续返回 `StatusMissing`；只有实际 cold-fill 失败才返回 `StatusUnavailable` 或 `StatusCorrupt`。

- [ ] **Step 1: 写失败/回归测试**：增加服务测试，验证 PostgreSQL generation 存在、Redis state 缺失时返回 `missing`，并验证 ready generation 不匹配仍返回 `corrupt`。
- [ ] **Step 2: 运行测试**：运行 `go test ./internal/module/system/cacheGeneration ./internal/shared/cacheFill -count=1`，记录当前行为；如果现有实现已满足契约，只保留回归测试，不改后端逻辑。
- [ ] **Step 3: 仅在测试证明恢复失败时修复**：沿现有 lease/cold-fill 边界补最小修复，禁止增加内存兜底、无限回源或后台无界 goroutine。
- [ ] **Step 4: 重跑定向测试**：确认 missing、corrupt、unavailable 三种状态分界稳定。

### Task 2: 建立 namespace、scope 和状态的中文展示映射

**Files:**
- Create: `web/src/views/system/cacheGeneration/presentation.ts`
- Test: `web/tests/views/system/cacheGeneration/presentation.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

**Interfaces:**
- `displayNamespace(namespace: string): string`：已知 namespace 显示业务名称，未知值显示“其他配置”。
- `displayScope(namespace: string, scopeKey: string): string`：`global` 显示“全局”，平台 scope 显示“认证平台 N”，不改变原始值。
- `statusLabel` 继续使用 API 状态 enum；`missing` 文案改成“等待缓存重建”，英文为 “Waiting for cache rebuild”。

- [ ] **Step 1: 写失败测试**：覆盖五个固定 namespace、`global`、数字 platform scope、未知 namespace 和全部七种状态。
- [ ] **Step 2: 运行测试确认失败**：运行 `pnpm vitest run tests/views/system/cacheGeneration/presentation.test.ts --pool=threads --maxWorkers=1`。
- [ ] **Step 3: 实现纯展示函数**：不在展示函数中访问 API、Store 或数据库；未知值必须保留可识别的 fallback，不显示原始英文 namespace 作为主标题。
- [ ] **Step 4: 运行测试确认通过**：重复定向 Vitest 并运行受影响文件 Prettier 检查。

### Task 3: 压缩主表并增加技术详情抽屉

**Files:**
- Create: `web/src/views/system/cacheGeneration/components/CacheGenerationDetailDialog/index.vue`
- Modify: `web/src/views/system/cacheGeneration/index.vue`
- Test: `web/tests/views/system/cacheGeneration/index.test.ts`

**Interfaces:**
- 主表保留：配置项、作用范围、同步状态、待同步数量、最近同步时间、最近错误。
- 详情抽屉显示：原始 namespace、scopeKey、当前 generation、pendingCount、latestAttempts、latestPublishedGeneration、latestPublishedAt、updatedAt。
- 详情只读，不增加写操作，不增加新权限码。

- [ ] **Step 1: 写失败测试**：更新页面测试，断言主表不再把 `Scope`、`当前代际`、`最近发布代际`等技术字段作为主列；断言中文配置名称和作用范围显示；点击详情后能看到原始技术字段；missing 使用非 danger 样式。
- [ ] **Step 2: 运行测试确认失败**：运行 `pnpm vitest run tests/views/system/cacheGeneration/index.test.ts --pool=threads --maxWorkers=1`。
- [ ] **Step 3: 实现最小页面调整**：复用现有 AppDialog/AppTable；主表通过 presentation 函数显示中文；详情组件接收严格 `CacheGeneration` props；保留搜索、筛选、分页、刷新、权限和 loading/empty/error 状态。
- [ ] **Step 4: 运行测试确认通过**：重复定向 Vitest，确认无路由/API/权限标识变更。

### Task 4: 维护文档与跨层标识回归

**Files:**
- Modify: `docs/agent/design.md`
- Modify: `docs/agent/architecture.md`
- Modify: `docs/agent/STATUS.md`
- Test: `server/internal/module/system/cacheGeneration/route_test.go`
- Test: `server/cmd/api/main_test.go`

- [ ] **Step 1: 增加标识回归断言**：确认 API 路径 `/api/admin/v1/system/cachegeneration`、权限 `system:cacheGeneration:list`、数据库 scope `system.setting/global` 等没有被文案重命名带动。
- [ ] **Step 2: 更新文档**：说明页面是“配置缓存代际”内部模块的中文运维视图，`missing` 是等待重建而非数据丢失；记录 Redis 清空后的恢复边界和不提供手工重建按钮的原因。
- [ ] **Step 3: 检查格式**：运行 `git diff --check`，确认不修改数据库快照或 migration 文件。

### Task 5: 全量验证

- [ ] **Step 1:** 在 `server` 运行 `go fmt ./...`、`go vet ./...`、`go test -p 1 ./... -count=1`、`go build ./...`。
- [ ] **Step 2:** 在 `web` 运行 `pnpm vitest run --pool=threads --maxWorkers=1`、`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`、`pnpm build`。
- [ ] **Step 3:** 人工检查页面：字典列表用途、字典项风险提示、缓存页面中文映射、missing 样式、详情技术字段和刷新恢复；确认页面标题仍为“配置缓存代际”；未经维护者另行授权不提交 Git。
