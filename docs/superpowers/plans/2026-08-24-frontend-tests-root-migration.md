# Frontend Tests Root Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `web/src` 中全部前端测试迁移到 `web/tests`，保留测试行为并让生产源码目录不再包含测试文件。

**Architecture:** 测试文件按照原 `src` 相对结构镜像到 `tests`，测试通过 `@src/*` 别名导入生产代码；生产代码、API 契约和运行时数据流不改变。Vitest 只扫描 `tests/**/*.test.*` 与 `tests/**/*.spec.*`，测试目录不会进入 Vite 生产构建产物。

**Tech Stack:** Vue 3、TypeScript strict、Vite、Vitest、Vue Test Utils、pnpm。

---

### Task 1: 记录测试清单并建立测试目录

**Files:**
- Create: `web/tests/`
- Modify: `docs/superpowers/plans/2026-08-24-frontend-tests-root-migration.md`

- [ ] **Step 1: 记录迁移前测试数量**

运行 `rg --files web`（排除 `node_modules` 和 `dist`）并筛选 `*.test.ts`, `*.test.tsx`, `*.test.js`, `*.test.jsx`, `*.spec.*`；记录数量和完整路径清单。当前基线为 55 个测试文件，其中包括根目录的 `web/vite.config.test.ts`。

- [ ] **Step 2: 检查测试专用资源**

搜索 `setup`, `fixture`, `mock`, `helper`, `snapshot` 文件和配置引用；若没有独立资源，只迁移测试文件，不创建空目录或兼容层。

- [ ] **Step 3: 创建目标目录**

创建 `web/tests`，后续用 `git mv` 将每个测试文件按 `src` 相对路径迁移到 `tests`。

### Task 2: 配置测试别名和测试扫描范围

**Files:**
- Modify: `web/vite.config.ts`
- Modify: `web/vite.config.test.ts`
- Modify: `web/tsconfig.app.json`
- Modify: `web/tsconfig.node.json`

- [ ] **Step 1: 增加生产源码别名**

在 Vite 配置中将 `@src` 指向 `web/src`，在 TypeScript 配置中增加同路径 `paths`，保证 Vitest、Vite 和 `vue-tsc` 使用同一别名。

- [ ] **Step 2: 限制 Vitest 只扫描 tests**

在 `vite.config.ts` 的 `test` 中设置 `include: ['tests/**/*.{test,spec}.{ts,tsx,js,jsx}']`，保留 `environment: 'jsdom'`；不配置真实服务、网络或数据库连接。

- [ ] **Step 3: 处理配置自测文件**

将 `vite.config.test.ts` 迁移到 `tests/vite.config.test.ts`，并让其通过 `@src` 或明确的配置模块路径导入 `vite.config.ts`；若配置文件不能被别名安全加载，使用从 `tests` 到根配置的单层相对路径且只保留这一处配置导入。

### Task 3: 迁移测试并修正导入路径

**Files:**
- Move: every file matching `web/src/**/*.{test,spec}.{ts,tsx,js,jsx}` to the mirrored path under `web/tests`
- Modify: moved test files whose production imports use relative paths

- [ ] **Step 1: 使用 Git 感知的迁移方式移动 54 个测试文件**

使用 `git mv` 或等价的单文件移动命令，目标路径为 `web/tests/<原 src 相对路径>`；`web/vite.config.test.ts` 移到 `web/tests/vite.config.test.ts`；不得删除或覆盖工作区中其他文件。

- [ ] **Step 2: 替换生产代码导入**

将测试中的生产代码、组件、API、Store 和 i18n 相对导入改为 `@src/...`；测试文件之间的相对导入保留镜像结构可读写法，只有跨目录层级不稳定时才改为 `@src`。

- [ ] **Step 3: 检查资源读取路径**

搜索 `readFileSync`, `resolve`, `__dirname`, fixture、snapshot 和动态 import；按新 `tests` 位置修正路径。当前基线未发现专用 fixture 或 snapshot 文件，若复查仍无结果则不新增兼容路径。

### Task 4: 更新文档和工程检查范围

**Files:**
- Modify: `web/README.md` if present, otherwise root `README.md`
- Modify: `web/tsconfig.app.json`
- Modify: relevant lint/build/package configuration only if old `src` test paths are present

- [ ] **Step 1: 更新测试说明**

明确测试统一位于 `web/tests`，测试命令仍为 `pnpm vitest run`，并说明测试目录镜像 `src` 结构。

- [ ] **Step 2: 检查生产打包边界**

确认 Vite 的入口只来自 `index.html`/`src`，没有把 `tests` 作为构建入口；若打包配置存在包含规则，显式排除 `tests`，不修改无关构建选项。

### Task 5: 验证迁移结果

**Files:**
- No new runtime files; inspect all changed paths

- [ ] **Step 1: 验证数量和目录隔离**

确认 `web/src` 不再有测试文件，`web/tests` 有且只有迁移前的 55 个测试文件。

- [ ] **Step 2: 搜索旧路径和无效配置**

搜索旧 `src/**/*.test`, `src/**/*.spec`, 旧 setup 路径、旧扫描规则、失效相对导入、fixture 路径和禁止的 `any` 写法。

- [ ] **Step 3: 运行项目实际验证**

依次运行：

```powershell
cd D:\admin\web
pnpm exec vue-tsc --noEmit
pnpm vitest run
pnpm build
```

若全量测试单进程超时，拆分测试命令并如实记录；迁移引起的失败必须修复后重新运行对应完整验证。

- [ ] **Step 4: 审查 Git 状态和差异**

运行 `git status --short`、`git diff --check` 和 `git diff --stat`，确认只包含本次测试迁移、配置、文档和 Git 重命名，不包含密钥、环境文件、构建产物或上一项已有改动的回退。
