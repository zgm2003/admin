# 字典管理使用说明与业务值边界实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让字典管理明确展示每个字典的业务用途与修改影响，并阻止协议绑定字典被增加或停用不兼容的业务值。

**Architecture:** 字典的 PostgreSQL 表、API 路径、Go 模块、Redis generation scope、菜单 path 和权限码保持不变。前端使用一个与业务代码同步的静态 usage registry 展示说明；后端在 system/dictionary service/repository 边界执行 `user.gender` 的固定值策略，业务消费仍通过现有 options 端点。

**Tech Stack:** Go、Gin、GORM、PostgreSQL、Redis generation、Vue 3、TypeScript、Pinia、Vitest、Element Plus。

**Spec:** `docs/agent/design.md`、`docs/agent/architecture.md` 以及 2026-10-10 对“字典管理减法”的确认：不改数据库结构，不改跨层标识；只补用途说明、风险提示和协议值保护。

## Global Constraints

- 不修改 `system_dictionary`、`system_dictionary_item` 表结构，不执行数据库迁移。
- 不修改 `/api/admin/v1/system/dictionary`、`/api/v1/system/dictionary/options`、菜单 path、componentPath、权限码或 Redis namespace。
- 字典只承载管理员可维护的展示型选项；认证、权限、安全、状态机和后端分支使用的 enum 不迁入字典。
- `user.gender` 只允许 `0/1/2` 三个固定值；允许编辑双语标签和排序，不允许新增、停用或删除协议值。
- `message.*.region` 的值必须是对应腾讯云产品支持的 region；扩展名和 MIME 字典只提供上传规则快捷选项，不自动修改已有规则。
- 不使用 `any`、`as any`、`@ts-ignore`，所有 HTTP DTO 继续严格解析。

---

### Task 1: 建立字典使用场景与修改影响注册表

**Files:**
- Create: `web/src/views/system/dictionary/dictionaryUsage.ts`
- Test: `web/tests/views/system/dictionary/dictionaryUsage.test.ts`

**Interfaces:**
- Produces `DictionaryUsage` 与 `dictionaryUsageByCode`，供字典列表和详情组件读取。
- 每个已知编码提供 `consumerLabelKey`、`impactLabelKey`、`valuePolicy`（`fixed` 或 `extensible`）和 `valueHintKey`。
- 未登记的自定义字典返回明确的“暂无页面使用”默认信息。

- [ ] **Step 1: 写失败测试**：覆盖六个现有编码的页面用途、固定/可扩展策略，以及未知编码的默认结果；断言 `user.gender` 为固定值。
- [ ] **Step 2: 运行测试确认失败**：运行 `pnpm vitest run tests/views/system/dictionary/dictionaryUsage.test.ts --pool=threads --maxWorkers=1`，预期因注册表文件和导出不存在而失败。
- [ ] **Step 3: 实现最小注册表**：仅维护稳定字典编码与 i18n key，不把数据库查询、API 请求或页面逻辑放入注册表；未知编码使用统一 fallback。
- [ ] **Step 4: 运行测试确认通过**：重复运行同一 Vitest 命令，确认所有编码策略断言通过。

### Task 2: 在字典管理页面展示用途与风险

**Files:**
- Modify: `web/src/views/system/dictionary/index.vue`
- Modify: `web/src/views/system/dictionary/components/DictionaryDetailDialog/index.vue`
- Modify: `web/src/views/system/dictionary/components/DictionaryItemDialog/index.vue`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Test: `web/tests/views/system/dictionary/index.test.ts`

**Interfaces:**
- 列表增加“使用场景”或等价的可扫描信息；详情显示“修改影响”和“值策略”。
- 字典项编辑仍只编辑标签和排序，不开放 value 编辑。
- `user.gender` 的详情与新增项入口显示固定值提示；普通管理员不会看到“可以自由新增业务值”的误导文案。

- [ ] **Step 1: 写失败测试**：增加断言，列表渲染六类用途；未知字典显示暂无使用；详情显示影响提示；`user.gender` 显示固定值提示；英文名称字段仍保留。
- [ ] **Step 2: 运行受影响测试确认失败**：运行 `pnpm vitest run tests/views/system/dictionary/index.test.ts tests/views/system/dictionary/dictionaryUsage.test.ts --pool=threads --maxWorkers=1`。
- [ ] **Step 3: 实现页面展示**：复用现有 AppTable/AppDialog，不新增通用组件；使用 i18n 生成用途、影响、值策略文案；保持现有权限按钮和 CRUD 行为。
- [ ] **Step 4: 运行测试确认通过**：重复定向 Vitest，并用 `pnpm exec prettier --check` 检查修改文件。

### Task 3: 后端保护 `user.gender` 固定值

**Files:**
- Create: `server/internal/module/system/dictionary/policy.go`
- Modify: `server/internal/module/system/dictionary/service.go`
- Modify: `server/internal/module/system/dictionary/repository.go`
- Test: `server/internal/module/system/dictionary/service_test.go`
- Test: `server/internal/module/system/dictionary/repository_test.go`

**Interfaces:**
- `policy.go` 暴露按字典 code 查询策略的私有函数，不引入通用 Manager/Factory。
- 创建字典项、停用字典项、删除字典项在 `user.gender` 上拒绝不兼容操作；更新标签和排序继续允许。
- 选项读取继续使用现有严格 DTO；固定值策略在管理写入边界阻止新增、停用和删除，避免给运行时 options 引入不兼容值。

- [ ] **Step 1: 写失败测试**：覆盖新增 `user.gender=3`、停用 `0/1/2`、删除内置项均被拒绝；标签/排序更新仍成功；合法三项正常返回。
- [ ] **Step 2: 运行后端定向测试确认失败**：运行 `go test ./internal/module/system/dictionary -run 'Test.*Gender|Test.*Dictionary' -count=1`，预期新增保护用例失败。
- [ ] **Step 3: 实现策略**：在 service 层做业务校验，在 repository 事务锁内再次确认 code/item，避免绕过 service 的并发写入；错误映射保持现有 400/409 语义。
- [ ] **Step 4: 运行后端测试确认通过**：运行 `go test ./internal/module/system/dictionary -count=1`、`go vet ./internal/module/system/dictionary`。

### Task 4: 审计其他可迁移业务值并更新文档

**Files:**
- Modify: `docs/agent/design.md`
- Modify: `docs/agent/architecture.md`
- Modify: `docs/agent/STATUS.md`

- [ ] **Step 1: 对照所有现有下拉和数据库 CHECK**：确认只有六个展示型字典编码；记录 `scene/status/scope/action/loginType/variant/priority/linkType/accessMode/menuType` 等协议 enum 不进入字典的原因。
- [ ] **Step 2: 更新稳定边界文档**：补充六个编码的消费页面、值策略和 `user.gender` 固定值规则；明确新增字典编码没有消费方时不会产生业务功能。
- [ ] **Step 3: 检查文档格式**：运行 `git diff --check`，确认不改变历史迁移档案。

### Task 5: 全量验证

- [ ] **Step 1:** 在 `server` 运行 `go fmt ./...`、`go vet ./...`、`go test -p 1 ./... -count=1`、`go build ./...`。
- [ ] **Step 2:** 在 `web` 运行 `pnpm vitest run --pool=threads --maxWorkers=1`、`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`、`pnpm build`。
- [ ] **Step 3:** 运行 `git diff --check` 并列出实际修改文件、未执行项和剩余风险；未经维护者另行授权不提交 Git。
