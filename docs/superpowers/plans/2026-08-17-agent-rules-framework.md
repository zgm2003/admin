# Admin Agent Rules Framework Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用一个短 `AGENTS.md` 入口和一个完整 `docs/agent/README.md` 指南固化项目规则，同时禁止前端业务 TypeScript 使用裸 `any`。

**Architecture:** 规则框架完全由文档组成，不创建运行时 Agent 系统。`AGENTS.md` 保存硬规则和按需阅读入口，`docs/agent/README.md` 保存任务路由、施工流程和交接清单，根 README 只提供人类可见入口。

**Tech Stack:** Markdown、PowerShell、ripgrep。

---

## Task 1: 将 AGENTS.md 改为短入口和硬规则

**Files:**
- Modify: `AGENTS.md`

- [ ] **Step 1: 运行定向检查确认当前规则缺失**

运行：

```powershell
cd D:\admin
$content = Get-Content -Raw AGENTS.md
if (-not $content.Contains('docs/agent/README.md')) { Write-Output 'missing agent guide link' }
if (-not $content.Contains('Do not use explicit `any`')) { Write-Output 'missing TypeScript any rule' }
if ($content.Contains('Before changing code, read `docs/superpowers/specs/2026-08-17-admin-foundation-design.md`')) { Write-Output 'full-read rule still present' }
```

预期：输出以上三个缺口，证明测试针对当前问题。

- [ ] **Step 2: 替换全量阅读要求**

将 `AGENTS.md` 顶部说明替换为：

```markdown
Start with this file. Then use `docs/agent/README.md` to select only the rules, spec sections, plan, code, and tests relevant to the current task. Do not read unrelated project documents for a small or local task.
```

不得继续要求每次改代码都完整读取基础 spec 和全部 active plan。

- [ ] **Step 3: 增加 TypeScript 类型硬规则**

在命名规则之后增加：

```markdown
- Frontend business TypeScript must not use explicit `any`, `any[]`, `as any`, or `Record<string, any>`. Use `unknown` for untrusted data, validate it, and narrow it to an explicit DTO, Props, Emits, Store, or function type.
- Do not suppress TypeScript errors with `@ts-ignore`, broad ambient declarations, or disabled strict compiler options. A third-party type gap may use one minimal isolated declaration that does not leak `any` into business code.
```

保留已有线性架构、命名、错误、数据库、Git 和验证规则，不复制长篇教程。

- [ ] **Step 4: 运行 AGENTS.md 定向验证**

运行：

```powershell
cd D:\admin
$content = Get-Content -Raw AGENTS.md
@('docs/agent/README.md', 'Do not read unrelated project documents', 'must not use explicit `any`', 'Use `unknown`') | ForEach-Object {
    if (-not $content.Contains($_)) { throw "missing rule: $_" }
}
if ($content.Contains('Before changing code, read `docs/superpowers/specs/2026-08-17-admin-foundation-design.md`')) {
    throw 'obsolete full-read rule remains'
}
Write-Output 'agents_rules_ok=true'
```

预期：输出 `agents_rules_ok=true`。

## Task 2: 创建统一 Agent 施工指南

**Files:**
- Create: `docs/agent/README.md`

- [ ] **Step 1: 确认指南尚不存在**

运行：

```powershell
cd D:\admin
if (Test-Path docs\agent\README.md) { throw 'agent guide already exists; inspect before editing' }
Write-Output 'agent_guide_missing=true'
```

预期：输出 `agent_guide_missing=true`。

- [ ] **Step 2: 创建指南并写明权威层级**

文件开头必须明确：

```markdown
# Admin Agent 施工指南

本指南解释 `AGENTS.md` 的项目硬规则。用户当前明确指令优先，其次是 `AGENTS.md`，再其次是本指南、当前 spec 和当前 plan。规则冲突时停止实现并报告，不自行猜测。
```

不得创建 `CLAUDE.md`、`GEMINI.md` 或第二套平行权威入口。

- [ ] **Step 3: 写入渐进式任务路由**

指南必须包含下表，明确小需求不全量阅读：

```markdown
| 任务类型 | 读取范围 |
| --- | --- |
| 解释代码、状态或单点问题 | `AGENTS.md`、目标文件、直接调用方 |
| 单文件配置、文案、样式、窄测试 | 对应 Agent 规则、目标文件、相关测试 |
| 单模块 bug | 模块调用链、相关 spec 章节、相关测试 |
| 新功能、跨模块契约、数据库、认证权限 | 对应完整 spec、完整 plan、相关代码 |
| 执行已批准 plan | 当前 plan、对应 spec、涉及的规则和代码 |
```

紧接表格写明：禁止以“可能有用”为理由读取全部历史 spec、plan 或仓库文档；每个额外文档必须与当前决策有直接关系。

- [ ] **Step 4: 写入线性架构和边界**

指南必须保留以下固定链路和边界：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

并明确 Handler/GORM、Service/Gin、Repository/PostgreSQL、Queue/Asynq、Redis、Storage、Integration 的具体职责。禁止 `Platform Adapter`、`infra`、Manager、Factory、BaseService、BaseRepository、DI 容器、运行时注册器和投机接口。

- [ ] **Step 5: 写入前端类型和协议规范**

指南必须列出：

- 禁止 `any`、`any[]`、`as any`、`Record<string, any>`；
- 外部数据使用 `unknown`，校验后缩小；
- API DTO、Props、Emits、Store、组合式函数返回值使用明确类型；
- 有限值使用 `as const` 或联合类型；
- HTTP envelope 精确为 `code`、`data`、`message`；
- 不用类型断言、可选链或默认值掩盖协议错误；
- 第三方类型缺口只能补一个最小隔离声明。

- [ ] **Step 6: 写入数据库、错误和命名规范**

指南必须包含：

- `created_at`、`updated_at` 使用 `TIMESTAMPTZ NOT NULL` 并在模型中显式声明；
- 有删除行为时只用 `deleted_at`/`gorm.DeletedAt`；
- Yes/No 为 `0=否`、`1=是`，数据库使用 `SMALLINT + CHECK`；
- 有效数据唯一性使用 `WHERE deleted_at IS NULL` 的部分唯一索引；
- 不做默认配置、空数组、假成功、内存替代或兼容字段兜底；
- Go/JSON/TypeScript/PostgreSQL/URL/权限码的既有命名规范；
- `context.Context` 沿真实 I/O 链路传递。

- [ ] **Step 7: 写入按规模施工流程和交接清单**

指南必须区分：

- 只读问题：只做必要检查并回答；
- 简单局部修改：说明修改和验证方式，不强制新建完整 spec/plan；
- 新功能、跨模块、契约变化：设计获批后写 spec 和 plan；
- 行为变化和 bug：失败测试、确认失败、最小实现、确认通过；
- 文档修改：运行定向链接和内容检查，不跑无关全量业务测试；
- 完成前：运行与实际改动风险相称的验证并报告未执行项。

交接清单包含当前 spec/plan、工作区已有改动、调用链、错误协议、数据库规则、前端类型和已运行验证；只要求回答与当前任务相关的项目。

- [ ] **Step 8: 验证指南章节完整且没有占位符**

运行：

```powershell
cd D:\admin
$guide = Get-Content -Raw docs\agent\README.md
@('渐进式任务路由', 'TypeScript 类型安全', '线性数据流', '数据库规范', '错误与禁止兜底', '施工流程', '交接清单') | ForEach-Object {
    if (-not $guide.Contains($_)) { throw "missing section: $_" }
}
if ($guide -match '(?i)\b(T[B]D|TO[D]O|PLACEHOLD[E]R)\b') { throw 'unfinished marker found' }
Write-Output 'agent_guide_ok=true'
```

预期：输出 `agent_guide_ok=true`。

## Task 3: 增加人类入口并做范围验证

**Files:**
- Modify: `README.md`
- Verify: `AGENTS.md`
- Verify: `docs/agent/README.md`
- Verify: `docs/superpowers/specs/2026-08-17-agent-rules-framework-design.md`

- [ ] **Step 1: 确认 README 尚未链接 Agent 指南**

运行：

```powershell
cd D:\admin
if ((Get-Content -Raw README.md).Contains('docs/agent/README.md')) { throw 'README already links agent guide; inspect before editing' }
Write-Output 'readme_agent_link_missing=true'
```

预期：输出 `readme_agent_link_missing=true`。

- [ ] **Step 2: 在 README 增加简短入口**

在项目介绍后、环境说明前增加：

```markdown
## AI 施工规范

AI 和开发者共同遵守根 [`AGENTS.md`](AGENTS.md) 的硬规则。完整任务路由、线性架构、TypeScript 类型、数据库、错误和交接规范见 [`docs/agent/README.md`](docs/agent/README.md)。小型局部任务按需读取相关内容，不要求全量阅读历史 spec 和 plan。
```

- [ ] **Step 3: 检查前端业务代码没有已存在的显式 any**

运行：

```powershell
cd D:\admin
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" web\src -g "*.ts" -g "*.vue"
```

预期：没有输出。若发现既有问题，只报告具体位置；本次文档任务不顺手修改业务代码。

- [ ] **Step 4: 运行定向文档验证**

运行：

```powershell
cd D:\admin
$agents = Get-Content -Raw AGENTS.md
$guide = Get-Content -Raw docs\agent\README.md
$readme = Get-Content -Raw README.md
if (-not $agents.Contains('docs/agent/README.md')) { throw 'AGENTS guide link missing' }
if (-not $agents.Contains('must not use explicit `any`')) { throw 'AGENTS any rule missing' }
if (-not $guide.Contains('禁止以“可能有用”为理由')) { throw 'progressive reading rule missing' }
if (-not $readme.Contains('docs/agent/README.md')) { throw 'README guide link missing' }
git diff --check
Write-Output 'agent_framework_ok=true'
```

预期：`git diff --check` 没有空白错误，最后输出 `agent_framework_ok=true`。

- [ ] **Step 5: 核对本次范围，不运行无关全量测试**

运行 `git status --short`，确认本次新增变更只涉及 `AGENTS.md`、`README.md`、`docs/agent/README.md`、当前 spec 和当前 plan。工作区原本存在的 Go/TypeScript 改动必须保留，不归入本次 Agent 文档任务。

因为本次不修改运行时代码，不运行 Go、Vitest 或生产构建；定向文档检查就是本次风险相称的验证。不得自动 commit 或 push。
