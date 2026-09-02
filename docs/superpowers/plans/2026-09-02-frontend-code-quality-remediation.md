# 前端代码质量与组件结构整改 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变现有业务功能、接口 URL、权限语义和成熟交互的前提下，统一 Vue 组件目录、Element Plus 布局、公共表格/搜索使用、严格 DTO 解析、错误展示所有权和前端质量门禁。

**Architecture:** 先建立可重复执行的格式、Lint、类型和架构门禁，再迁移组件目录并收紧动态路由；随后将所有 API 固定为 `request<unknown>() -> 严格 parser -> DTO`，移除请求层 UI 副作用，最后以邮件页为样板逐页拆分和清理。页面入口只保留权限、顶层查询和组件协调，独立表单、专用表格与纯算法进入页面私有组件或明确逻辑文件，不新增万能 CRUD、Schema Renderer 或 Base 组件。

**Tech Stack:** Vue 3、TypeScript 6、Pinia、Vue Router、Element Plus、Vue I18n、Vitest、Vite、pnpm、ESLint、Prettier、vue-tsc

**Spec:** `docs/superpowers/specs/2026-09-02-frontend-code-quality-remediation-design.md`

## Global Constraints

- 除 `web/src/App.vue` 外，所有 Vue 页面和组件必须使用 `<ComponentName>/index.vue`；页面私有组件放在 `views/<domain>/<page>/components/<ComponentName>/index.vue`。
- 简单表单优先使用 `el-form`、`el-form-item`、`label-width`、`inline` 和控件宽度；能用 Element Plus 公开属性完成的布局不新增 CSS。
- `el-row/el-col` 只承担真实 24 栅格、响应式分栏或规则网格；禁止单个 `span=24`、每字段包裹和无语义嵌套。
- 无必要样式不写 `<style>`；少量局部样式使用普通 scoped CSS，只有实际使用 Sass 能力时才保留 SCSS。
- 普通列表默认使用 `AppTable`，列表筛选默认使用 `AppSearch`；菜单树表和权限矩阵保留原生 `el-table` 时必须在组件内写明稳定例外原因。
- HTTP 响应固定为 `request<unknown>() -> API 模块严格 parser -> DTO`；必填字段缺失、类型错误或未知字段必须抛出 `ProtocolError`，不得使用断言或 `?? []` 修复。
- `request.ts` 只处理 Envelope、HTTP、Refresh 和错误归一化，不调用 Element Plus 通知；同一失败只由发起动作的页面或组件展示一次。
- 登录页、Dashboard 和应用壳保持静态；其余业务页从 Access 快照动态注册，菜单页不得保留静态绑定特例。
- 不修改 Go API、数据库、API URL、权限码、路由 path、分页规则、确认语义、国际化、暗色主题、移动端和可访问性行为。
- 不创建 `BasePage`、`BaseForm`、`BaseCRUD`、Manager、Factory、通用 Schema Renderer 或万能 `useCRUD`。
- 每个任务先运行失败测试或门禁并确认失败原因，再做最小实现；不得为了干净工作区回退用户改动。
- 本计划中的 Git 命令仅表示建议提交边界；项目硬规则禁止自动 commit、fetch、pull 或 push，只有用户明确授权后才可执行提交。

## Dependency Order

```text
格式基线
  -> 静态门禁与迁移清单
  -> 组件目录与动态路由
  -> DTO parser 基础
  -> 启动边界 API
  -> User / System / Storage API
  -> 请求错误展示所有权
  -> 邮件样板页
  -> 存储与认证平台
  -> 用户、会话与日志
  -> 角色
  -> 菜单
  -> Login / Profile / Dashboard / Layout
  -> 清空迁移清单并全量验证
```

---

### Task 1: 建立 Prettier 机械格式基线

**Files:**
- Create: `web/prettier.config.mjs`
- Create: `web/.prettierignore`
- Modify: `web/package.json`
- Modify: `web/pnpm-lock.yaml`
- Modify mechanically: all tracked `web/src/**/*.{ts,vue,css,scss}`, `web/tests/**/*.ts`, `web/*.json`, `web/*.ts`, `web/index.html`

**Interfaces:**
- Consumes: 当前 pnpm 工程和所有已跟踪前端源码。
- Produces: `pnpm format` 与 `pnpm format:check`；后续所有任务以同一格式为输入，避免结构整改混入风格噪声。

- [ ] **Step 1: 记录格式门禁尚不存在的失败**

Run:

```powershell
cd D:\admin\web
pnpm format:check
```

Expected: FAIL，明确提示 `format:check` script 或 `prettier` 命令不存在；失败不能来自 TypeScript 或 Vitest。

- [ ] **Step 2: 增加固定格式配置和脚本**

`web/prettier.config.mjs` 使用唯一配置：

```js
/** @type {import('prettier').Config} */
export default {
  printWidth: 100,
  semi: false,
  singleQuote: true,
  trailingComma: 'all',
}
```

`web/.prettierignore` 固定排除生成物与本地工具状态：

```text
dist
node_modules
coverage
.playwright-cli
pnpm-lock.yaml
```

安装并增加脚本：

```powershell
cd D:\admin\web
pnpm add -D prettier
```

```json
{
  "format": "prettier --write .",
  "format:check": "prettier --check ."
}
```

- [ ] **Step 3: 单独完成一次机械格式化**

Run:

```powershell
cd D:\admin\web
pnpm format
```

Expected: 只产生引号、分号、缩进、换行和尾逗号等机械差异；不得在本步骤移动文件、改逻辑或改文案。

- [ ] **Step 4: 验证格式化不改变行为**

Run:

```powershell
cd D:\admin\web
pnpm format:check
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
git diff --check
```

Expected: 四条命令全部通过。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/package.json web/pnpm-lock.yaml web/prettier.config.mjs web/.prettierignore web/src web/tests web/*.json web/*.ts web/index.html
git commit -m "style: 统一前端机械格式"
```

---

### Task 2: 建立 ESLint、typecheck、架构检查和迁移清单

**Files:**
- Create: `web/eslint.config.mjs`
- Create: `web/scripts/check-frontend-architecture.mjs`
- Create: `web/scripts/frontend-architecture-baseline.mjs`
- Modify: `web/package.json`
- Modify: `web/pnpm-lock.yaml`
- Modify: `web/tsconfig.app.json`
- Modify: `web/vite.config.ts`
- Modify: `web/tests/App.test.ts`
- Modify: `web/tests/auth/platform.test.ts`
- Modify: `web/tests/enums/yes-no.test.ts`
- Modify: `web/tests/api/health.test.ts`
- Modify: `web/tests/auth/device-id.test.ts`
- Modify: `web/tests/api/auth/platform.test.ts`
- Modify: `web/tests/api/auth/login.test.ts`
- Modify: `web/tests/i18n/index.test.ts`
- Modify: `web/tests/api/storage/uploadrule.test.ts`
- Modify: `web/tests/api/storage/upload.test.ts`
- Modify: `web/tests/components/AppSearch/src/index.test.ts`
- Modify: `web/tests/api/storage/cosconfig.test.ts`
- Modify: `web/tests/components/AppDialog/src/index.test.ts`
- Modify: `web/tests/components/AppDIcon/src/index.test.ts`
- Modify: `web/tests/components/AppDialog/src/dialog.test.ts`
- Modify: `web/tests/components/IconSelect/src/index.test.ts`
- Modify: `web/tests/api/user/session.test.ts`
- Modify: `web/tests/api/user/profile.test.ts`
- Modify: `web/tests/components/UpMedia/src/index.test.ts`
- Modify: `web/tests/api/user/loginlog.test.ts`
- Modify: `web/tests/api/user/account.test.ts`
- Modify: `web/tests/components/LocaleSwitch/src/index.test.ts`
- Modify: `web/tests/api/system/operationlog.test.ts`
- Modify: `web/tests/router/permission-routes.test.ts`
- Modify: `web/tests/components/AppTable/src/types.test.ts`
- Modify: `web/tests/components/AppTable/src/index.test.ts`
- Modify: `web/tests/api/permission/role.test.ts`
- Modify: `web/tests/router/index.test.ts`
- Modify: `web/tests/api/permission/permission.test.ts`
- Modify: `web/tests/layout/breadcrumbs.test.ts`
- Modify: `web/tests/main.test.ts`
- Modify: `web/tests/api/permission/menu.test.ts`
- Modify: `web/tests/layout/index.test.ts`
- Modify: `web/tests/api/permission/menu-fields.test.ts`
- Modify: `web/tests/views/system/operation-logs/index.test.ts`
- Modify: `web/tests/views/dashboard/index.test.ts`
- Modify: `web/tests/views/auth/login/index.test.ts`
- Modify: `web/tests/utils/request.test.ts`
- Modify: `web/tests/utils/ui-preferences.test.ts`
- Modify: `web/tests/views/cloud/storage-object/index.test.ts`
- Modify: `web/tests/views/account/profile/index.test.ts`
- Modify: `web/tests/views/account/users/index.test.ts`
- Modify: `web/tests/layout/components/AppAside.test.ts`
- Modify: `web/tests/views/account/sessions/index.test.ts`
- Modify: `web/tests/layout/components/PermissionMenuNode.test.ts`
- Modify: `web/tests/layout/components/SettingDrawer.test.ts`
- Modify: `web/tests/utils/theme.test.ts`
- Modify: `web/tests/views/permission/auth-platforms/index.test.ts`
- Modify: `web/tests/layout/components/RouteTabs.test.ts`
- Modify: `web/tests/views/permission/menus/index.test.ts`
- Modify: `web/tests/views/permission/menus/filter-menu-tree.test.ts`
- Modify: `web/tests/views/permission/roles/index.test.ts`
- Modify: `web/tests/views/permission/roles/components/RolePermissionMatrix.test.ts`
- Modify: `web/tests/views/permission/roles/role-permission-matrix.test.ts`
- Modify: `web/tests/store/auth.test.ts`
- Modify: `web/tests/store/permission.test.ts`
- Modify: `web/tests/store/ui-preferences.test.ts`

**Interfaces:**
- Consumes: Task 1 的稳定格式。
- Produces: `pnpm lint`、`pnpm typecheck`、`pnpm check:architecture`、`pnpm verify:frontend`；`frontend-architecture-baseline.mjs` 只记录尚未迁移的已知违规，并拒绝未知违规和失效条目。

- [ ] **Step 1: 写出会暴露当前遗留的架构扫描器**

`check-frontend-architecture.mjs` 输出稳定违规 ID：

```js
const rules = [
  'vue-component-path',
  'component-src-directory',
  'broad-view-glob',
  'static-business-route',
  'legacy-import-alias',
  'cross-module-relative-import',
  'explicit-ts-import',
  'oversized-sfc',
  'raw-el-table',
  'unnecessary-scss',
  'meaningless-grid-wrapper',
  'api-unparsed-response',
  'required-array-fallback',
  'request-ui-side-effect',
  'source-shape-test',
]
```

每条发现使用 `{ rule, file, detail }`，路径统一为相对 `web/` 的 `/` 分隔形式。扫描器必须同时失败于：未登记的新违规、已经不存在但仍留在 baseline 的过期条目。`cross-module-relative-import` 检查跨模块的 `../` 相对 import（同目录只能使用 `./`）；`explicit-ts-import` 检查业务 import 中显式 `.ts` 后缀；`unnecessary-scss` 检查没有 Sass 变量、mixin、函数或浅层嵌套理由的 `lang="scss"`；`meaningless-grid-wrapper` 检查单列 `el-row/el-col` 和表单字段的无语义网格包裹。这些规则不能由页面测试通过读取源码来承担。当前 `src/styles/index.scss` 仅保留可访问性所需的 `a:focus-visible`，不存在计划原先假设的裸 `button/input/textarea:focus-visible` 违规，因此不登记 `global-style-rule`。

初次将 `frontend-architecture-baseline.mjs` 写为：

```js
export const architectureBaseline = Object.freeze([])
```

- [ ] **Step 2: 确认空迁移清单会因真实遗留失败**

Run:

```powershell
cd D:\admin\web
node scripts/check-frontend-architecture.mjs
```

Expected: FAIL，至少报告 `components/*/src/index.vue`、9 个独立命名 `.vue`、`../views/**/index.vue`、菜单静态路由、跨模块相对 import、显式 `.ts` import、超大页面、所有尚未严格解析的 API（含 permission menu/role）、`?? []`、`request.ts` 的 Element Plus 依赖、2 个源码形态测试、无 Sass 理由的 SCSS 和无意义 row/col；不得报告生成目录或不存在的全局 focus 规则。

- [ ] **Step 3: 增加工具链并登记精确遗留**

安装：

```powershell
cd D:\admin\web
pnpm add -D eslint @eslint/js typescript-eslint eslint-plugin-vue vue-eslint-parser eslint-plugin-promise eslint-config-prettier globals
```

Flat config 必须覆盖 `src/**/*.{ts,vue}`、`tests/**/*.ts`、`scripts/**/*.mjs`，忽略 `dist`、`node_modules`、`coverage` 和 `.playwright-cli`，启用 TypeScript/Vue 推荐规则、Promise 规则、未使用代码和 `no-restricted-imports`。业务与测试 import 禁止 `@src/*`、跨模块 `../` 相对路径和显式 `.ts` 后缀，全部改为 `@/*` 或同目录 `./`，随后从 `vite.config.ts` 和 `tsconfig.app.json` 删除 `@src` alias。

baseline 逐文件登记当前遗留，条目必须包含原因，例如：

```js
export const architectureBaseline = Object.freeze([
  { rule: 'component-src-directory', file: 'src/components/AppTable/src/index.vue', detail: '公共组件目录待迁移' },
  { rule: 'vue-component-path', file: 'src/layout/components/AppAside.vue', detail: '布局组件目录待迁移' },
  { rule: 'oversized-sfc', file: 'src/views/permission/menus/index.vue', detail: '菜单页待职责拆分' },
  { rule: 'api-unparsed-response', file: 'src/api/storage/uploadrule.ts', detail: '存储 DTO 待严格解析' },
  { rule: 'required-array-fallback', file: 'src/api/storage/uploadrule.ts', detail: 'page-init 必填数组待拒绝 null' },
  { rule: 'request-ui-side-effect', file: 'src/utils/request.ts', detail: '错误所有权迁移前保留' },
  { rule: 'source-shape-test', file: 'tests/views/permission/menus/index.test.ts', detail: '改为可观察行为测试' },
  { rule: 'source-shape-test', file: 'tests/components/AppSearch/src/index.test.ts', detail: '迁移到架构样式检查' },
])
```

实际清单必须覆盖扫描器本次输出的全部精确文件，不能使用目录通配符或规则级跳过。

`package.json` 增加：

```json
{
  "typecheck": "vue-tsc -b",
  "lint": "eslint . --max-warnings 0",
  "check:architecture": "node scripts/check-frontend-architecture.mjs",
  "verify:frontend": "pnpm format:check && pnpm lint && pnpm check:architecture && pnpm typecheck && pnpm vitest run --pool=threads --maxWorkers=1 && pnpm build"
}
```

- [ ] **Step 4: 验证门禁可通过且能拒绝未知违规**

Run:

```powershell
cd D:\admin\web
pnpm lint
pnpm check:architecture
pnpm typecheck
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

Expected: 全部通过。随后临时在未登记测试夹具中创建 `BadComponent.vue`，`pnpm check:architecture` 必须因 `vue-component-path` 失败；立即删除该夹具并再次确认通过，不把夹具留在工作区。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/eslint.config.mjs web/scripts web/package.json web/pnpm-lock.yaml web/tsconfig.app.json web/vite.config.ts web/src web/tests
git commit -m "chore: 增加前端静态质量门禁"
```

---

### Task 3: 迁移公共组件到单层目录

**Files:**
- Move: `web/src/components/AppDialog/src/index.vue` -> `web/src/components/AppDialog/index.vue`
- Move: `web/src/components/AppDialog/src/dialog.ts` -> `web/src/components/AppDialog/dialog.ts`
- Move: `web/src/components/AppDIcon/src/index.vue` -> `web/src/components/AppDIcon/index.vue`
- Move: `web/src/components/AppDIcon/src/types.ts` -> `web/src/components/AppDIcon/types.ts`
- Move: `web/src/components/AppSearch/src/index.vue` -> `web/src/components/AppSearch/index.vue`
- Move: `web/src/components/AppSearch/src/types.ts` -> `web/src/components/AppSearch/types.ts`
- Move: `web/src/components/AppTable/src/index.vue` -> `web/src/components/AppTable/index.vue`
- Move: `web/src/components/AppTable/src/types.ts` -> `web/src/components/AppTable/types.ts`
- Move: `web/src/components/IconSelect/src/index.vue` -> `web/src/components/IconSelect/index.vue`
- Move: `web/src/components/IconSelect/src/types.ts` -> `web/src/components/IconSelect/types.ts`
- Move: `web/src/components/LocaleSwitch/src/index.vue` -> `web/src/components/LocaleSwitch/index.vue`
- Move: `web/src/components/UpMedia/src/index.vue` -> `web/src/components/UpMedia/index.vue`
- Modify: all seven `web/src/components/*/index.ts` barrels
- Move/update: `web/tests/components/{AppDialog,AppDIcon,AppSearch,AppTable,IconSelect,LocaleSwitch,UpMedia}/src/*.test.ts` to the same component test directory without `src/`
- Modify: `web/tests/views/account/profile/index.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: 原公共组件 props、emits、slots 和 barrel exports。
- Produces: `@/components/AppTable` 等稳定 barrel 保持不变；直接实现路径统一为 `@/components/AppTable/index.vue`，类型路径统一为 `@/components/AppTable/types`。

- [ ] **Step 1: 先收缩公共组件目录 baseline**

删除 baseline 中七个 `component-src-directory` 条目，不移动文件。

- [ ] **Step 2: 确认架构检查按预期失败**

Run:

```powershell
cd D:\admin\web
pnpm check:architecture
```

Expected: FAIL，且只新增报告七个公共组件的中间 `src/` 目录。

- [ ] **Step 3: 移动实现、类型、测试并保持 barrel 契约**

每个 barrel 只保留一层导出，例如：

```ts
export { default as AppTable } from './index.vue'
export {
  formatTableColumnValue,
  tableColumnKey,
  tableColumnProp,
  tableColumnValue,
} from './types'
export type {
  DerivedTableColumn,
  TableColumn,
  TableColumnElementProps,
  TableColumnKey,
  TablePaginationState,
  TableRow,
} from './types'
```

不得修改公共组件 props、emits、slot 名、默认分页和错误状态语义。

同时处理 `AppSearch` 的样式门禁测试：删除 `tests/components/AppSearch/src/index.test.ts` 中通过 `readFileSync` 读取 `src/styles/index.scss` 的源码断言；AppSearch 行为测试只保留 Element Plus focus/spacing/emit 的可观察断言，实际全局 CSS 约束由架构检查中确实存在的规则负责，当前没有对应的全局 focus 违规时不新增 baseline 条目。

- [ ] **Step 4: 运行公共组件定向验证**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/components tests/views/account/profile/index.test.ts --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过，`rg -n '/src/' src/components tests/components` 不再找到旧组件路径，且 `rg -n 'readFileSync|source\.toContain|source\.not\.toContain' tests/components -g '*.test.ts'` 无输出。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/components web/tests/components web/tests/views/account/profile/index.test.ts web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 统一公共组件目录结构"
```

---

### Task 4: 迁移布局与私有组件目录并收紧动态路由

**Files:**
- Move: `web/src/layout/components/AppAside.vue` -> `web/src/layout/components/AppAside/index.vue`
- Move: `web/src/layout/components/AppFooter.vue` -> `web/src/layout/components/AppFooter/index.vue`
- Move: `web/src/layout/components/AppHeader.vue` -> `web/src/layout/components/AppHeader/index.vue`
- Move: `web/src/layout/components/PermissionMenuNode.vue` -> `web/src/layout/components/PermissionMenuNode/index.vue`
- Move: `web/src/layout/components/RouteTabs.vue` -> `web/src/layout/components/RouteTabs/index.vue`
- Move: `web/src/layout/components/SettingDrawer.vue` -> `web/src/layout/components/SettingDrawer/index.vue`
- Move: `web/src/views/dashboard/components/ReadinessChart.vue` -> `web/src/views/dashboard/components/ReadinessChart/index.vue`
- Move: `web/src/views/permission/roles/components/RolePermissionMatrix.vue` -> `web/src/views/permission/roles/components/RolePermissionMatrix/index.vue`
- Move: `web/src/views/permission/roles/components/RolePermissionMatrix.css` -> `web/src/views/permission/roles/components/RolePermissionMatrix/styles.css`
- Move: `web/src/views/permission/roles/components/RolePermissionDiffDialog.vue` -> `web/src/views/permission/roles/components/RolePermissionDiffDialog/index.vue`
- Modify: `web/src/layout/index.vue`
- Modify: `web/src/views/dashboard/index.vue`
- Modify: `web/src/views/permission/roles/index.vue`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/router/permission-routes.ts`
- Modify: `web/tests/router/index.test.ts`
- Modify: `web/tests/router/permission-routes.test.ts`
- Modify imports in `web/tests/layout/**/*.test.ts`, `web/tests/views/dashboard/index.test.ts`, `web/tests/views/permission/roles/**/*.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: `PageModuleMap`、Access `componentPath` 映射和现有布局组件契约。
- Produces: `pageModules` 只包含 `views/*/index.vue` 与 `views/*/*/index.vue`；菜单页使用动态路由名 `access:permission:menu:view`，不再存在 `access-menus` 静态业务路由。

- [ ] **Step 1: 修改路由测试以要求菜单动态注册**

在 `permission-routes.test.ts` 固定以下行为：

```ts
it('registers the menu page from the access snapshot', () => {
  const router = testRouter()
  const cleanup = registerPermissionRoutes(router, [
    directory('access', [page('permission:menu:view', '/access/menus', 'access/menus')]),
  ], testViews)

  expect(router.resolve('/access/menus').name).toBe('access:permission:menu:view')
  cleanup()
  expect(router.resolve('/access/menus').matched).toHaveLength(0)
})
```

同时删除原先针对 `staticPageBinding` 的 code/path/component mismatch 和错误静态 route name 测试；把 `testRouter(true)` helper 改为只创建 `admin-layout` 与 Dashboard，避免测试继续要求已删除的业务特例。在 `router/index.test.ts` 断言静态 children 只有 Dashboard，不含 `/access/menus`。

- [ ] **Step 2: 确认新路由契约失败**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/router/index.test.ts tests/router/permission-routes.test.ts --pool=threads --maxWorkers=1
```

Expected: FAIL，当前菜单仍解析为 `access-menus`，且静态 children 仍包含菜单页。

- [ ] **Step 3: 完成目录移动和受限页面注册**

将动态 glob 改为固定深度合并：

```ts
const pageModules: PageModuleMap = {
  ...import.meta.glob<PageModule>('../views/*/index.vue'),
  ...import.meta.glob<PageModule>('../views/*/*/index.vue'),
}
```

删除 `staticPageBinding`、`isStaticBindingCandidate`、`validateStaticBinding` 和 `router/index.ts` 的菜单 child；保留现有 `componentPathMap`，使数据库 `access/menus` 继续唯一映射到源码 `permission/menus`。移动组件并更新全部 import；`RolePermissionMatrix/index.vue` 使用：

```vue
<style scoped src="./styles.css"></style>
```

从 baseline 删除全部已迁移路径、`broad-view-glob` 和 `static-business-route` 条目。

- [ ] **Step 4: 验证目录、路由和既有交互**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/router tests/layout tests/views/dashboard tests/views/permission/roles --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过；`Get-ChildItem src -Recurse -Filter *.vue | Where-Object { $_.Name -ne 'index.vue' -and $_.Name -ne 'App.vue' }` 无输出。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/layout web/src/views/dashboard web/src/views/permission/roles web/src/router web/tests/layout web/tests/router web/tests/views/dashboard web/tests/views/permission/roles web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 统一组件目录并收紧动态路由"
```

---

### Task 5: 建立窄小的 DTO parser 基础守卫

**Files:**
- Create: `web/src/api/protocol.ts`
- Create: `web/tests/api/protocol.test.ts`

**Interfaces:**
- Consumes: `ProtocolError`、`PageResult<T>`。
- Produces: `expectRecord`、`expectExactKeys`、`expectString`、`expectInteger`、`expectBoolean`、`expectArray`、`expectEmptyRecord`、`parsePageResult`；不包含业务字段、Schema DSL 或运行时注册器。

- [ ] **Step 1: 写基础守卫失败测试**

```ts
import { describe, expect, it } from 'vitest'

import {
  expectExactKeys,
  expectInteger,
  expectRecord,
  parsePageResult,
} from '@/api/protocol'
import { ProtocolError } from '@/types/http'

describe('API protocol guards', () => {
  it('rejects arrays, unknown fields, and non-integers', () => {
    expect(() => expectRecord([], 'sample')).toThrow(ProtocolError)
    expect(() => expectExactKeys({ id: 1, extra: true }, ['id'], 'sample')).toThrow(ProtocolError)
    expect(() => expectInteger(1.5, 'sample.id', { min: 1 })).toThrow(ProtocolError)
  })

  it('parses every page item and rejects malformed page metadata', () => {
    const parseItem = (value: unknown) => expectInteger(value, 'sample.item', { min: 1 })
    expect(parsePageResult({ list: [1, 2], total: 2, page: 1, pageSize: 20 }, parseItem, 'sample')).toEqual({
      list: [1, 2], total: 2, page: 1, pageSize: 20,
    })
    expect(() => parsePageResult({ list: [], total: 0, page: 1, pageSize: null }, parseItem, 'sample')).toThrow(ProtocolError)
  })
})
```

- [ ] **Step 2: 确认测试因模块缺失失败**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/protocol.test.ts --pool=threads --maxWorkers=1
```

Expected: FAIL，原因是 `@/api/protocol` 不存在。

- [ ] **Step 3: 实现无业务语义的最小守卫**

核心签名固定为：

```ts
export interface IntegerOptions { min?: number; max?: number }
export function expectRecord(value: unknown, context: string): Record<string, unknown>
export function expectExactKeys(value: Record<string, unknown>, keys: readonly string[], context: string): void
export function expectString(value: unknown, context: string): string
export function expectInteger(value: unknown, context: string, options?: IntegerOptions): number
export function expectBoolean(value: unknown, context: string): boolean
export function expectArray(value: unknown, context: string): unknown[]
export function expectEmptyRecord(value: unknown, context: string): Record<string, never>
export function parsePageResult<T>(value: unknown, parseItem: (item: unknown) => T, context: string): PageResult<T>
```

所有失败统一抛出包含 `context` 的 `ProtocolError`；`parsePageResult` 精确接受 `list/total/page/pageSize`，检查 `total >= 0`、`page >= 1`、`pageSize >= 1`，并返回新对象和新数组。

- [ ] **Step 4: 运行基础守卫验证**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/protocol.test.ts --pool=threads --maxWorkers=1
pnpm typecheck
```

Expected: 两条命令通过。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/api/protocol.ts web/tests/api/protocol.test.ts
git commit -m "refactor: 增加前端协议解析基础守卫"
```

---

### Task 6: 严格解析启动、认证和权限 API

**Files:**
- Modify: `web/src/api/health.ts`
- Modify: `web/src/api/auth/login.ts`
- Modify: `web/src/api/auth/platform.ts`
- Modify: `web/src/api/permission/permission.ts`
- Modify: `web/src/api/permission/menu.ts`
- Modify: `web/src/api/permission/role.ts`
- Modify: `web/tests/api/health.test.ts`
- Modify: `web/tests/api/auth/login.test.ts`
- Modify: `web/tests/api/auth/platform.test.ts`
- Modify: `web/tests/api/permission/permission.test.ts`
- Modify: `web/tests/api/permission/menu.test.ts`
- Modify: `web/tests/api/permission/role.test.ts`
- Modify: `web/tests/store/auth.test.ts`
- Modify: `web/tests/store/permission.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: Task 5 的基础守卫、现有 `YesNo` 守卫和原 API URL/请求体。
- Produces: `parseHealthStatus`、`parseReadiness`、`parseAccessCredential`、`parseCurrentUser`、认证平台分页/状态/创建/空对象 parser、`parsePermissionSnapshot`、`parseMenuCatalog`、菜单节点/平台/写结果 parser、角色分页/角色权限/状态/默认值/写结果 parser；View/Store 只收到严格 DTO。

- [ ] **Step 1: 为每个边界增加未知字段和缺字段失败用例**

至少增加以下契约用例：

```ts
it('rejects an access snapshot with an unknown nested field', async () => {
  requestMock.mockResolvedValue({
    roleCodes: [],
    permissionCodes: [],
    menuTree: [{
      code: 'account:user:view', menuType: 'page', path: '/account/users',
      componentPath: 'account/users', i18nKey: 'navigation.accountUsers',
      icon: null, isHidden: YesNo.No, children: [], extra: true,
    }],
  })
  await expect(getPermission()).rejects.toBeInstanceOf(ProtocolError)
})
```

Health 覆盖额外字段和非 `'up'` 状态；Login 覆盖 credential 缺 `expiresIn`、logout 非空对象；Auth Platform 覆盖分页项未知字段、非法 `YesNo` 和 mutation 结果畸形；Menu 覆盖平台/树节点未知字段、父子 ID 不一致、空平台和全部写结果；Role 覆盖列表项未知字段、权限矩阵重复/非法 `menuIds`、分页元数据和全部写结果。

- [ ] **Step 2: 确认测试揭示直接信任泛型**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/health.test.ts tests/api/auth tests/api/permission --pool=threads --maxWorkers=1
```

Expected: 新增的畸形响应用例 FAIL，因为当前 `request<T>` 直接返回或 parser 未检查 exact keys。

- [ ] **Step 3: 将所有响应改为 unknown 后严格解析**

统一模式：

```ts
export async function getHealth(): Promise<HealthStatus> {
  const value = await request<unknown>({ method: 'GET', url: '/health' })
  return parseHealthStatus(value)
}
```

递归菜单节点必须精确检查 `code/menuType/path/componentPath/i18nKey/icon/isHidden/children`；`directory` 与 `page` 的既有业务关系保持不变。写操作的 `{ id }`、状态结果和空对象也逐一解析。移除这些文件内重复的 `isExactRecord`，但业务状态守卫保留在对应 API 文件。

从 baseline 删除本任务六个 API 的 `api-unparsed-response` 条目。

- [ ] **Step 4: 运行启动边界完整验证**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/health.test.ts tests/api/auth tests/api/permission tests/store/auth.test.ts tests/store/permission.test.ts --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过，API URL、请求 body 和 Store 可观察状态不变。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/api/health.ts web/src/api/auth web/src/api/permission web/tests/api/health.test.ts web/tests/api/auth web/tests/api/permission web/tests/store web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 严格解析认证与权限响应"
```

---

### Task 7: 严格解析用户与会话 API

**Files:**
- Modify: `web/src/api/user/account.ts`
- Modify: `web/src/api/user/profile.ts`
- Modify: `web/src/api/user/session.ts`
- Modify: `web/src/api/user/loginlog.ts`
- Modify: `web/tests/api/user/account.test.ts`
- Modify: `web/tests/api/user/profile.test.ts`
- Modify: `web/tests/api/user/session.test.ts`
- Modify: `web/tests/api/user/loginlog.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: Task 5 parser 基础和现有用户 DTO。
- Produces: 四个用户模块的所有查询、详情、page-init、状态、角色和撤销结果均经过 exact parser；导出的 DTO 名和函数签名保持不变。

- [ ] **Step 1: 补齐分页、嵌套角色、nullable 和 mutation 契约测试**

```ts
it('rejects a session page with a malformed nested item', async () => {
  requestMock.mockResolvedValue({
    list: [{
      id: 1, userId: 7, username: 'alice', platform: 'admin', deviceId: 'device',
      clientIp: '127.0.0.1', userAgent: 'test', createdAt: '2026-09-02T00:00:00Z',
      updatedAt: '2026-09-02T00:00:00Z', refreshExpiresAt: '2026-09-03T00:00:00Z',
      revokedAt: null, status: 'active', isCurrent: false, extra: true,
    }],
    total: 1, page: 1, pageSize: 20,
  })
  await expect(getSessions({ page: 1, pageSize: 20 })).rejects.toBeInstanceOf(ProtocolError)
})
```

Account 覆盖角色数组、`phone: null` 和全部写结果；Profile 覆盖空对象密码结果；Login Log 覆盖 page-init 与日志分页；Session Stats 覆盖动态平台计数字典的非负整数。

- [ ] **Step 2: 确认用户模块仍存在未解析路径**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/user --pool=threads --maxWorkers=1
```

Expected: 新增畸形响应测试 FAIL；现有请求形状测试继续通过。

- [ ] **Step 3: 逐函数改为 unknown -> parser**

分页使用 `parsePageResult`，嵌套角色、用户摘要、状态结果和空对象使用模块私有 parser。动态 `SessionStats.platforms` 必须逐键检查值为非负整数，并返回新 `Record<string, number>`。禁止 `as UserDTO` 和必填字段默认值。

从 baseline 删除四个用户模块的 `api-unparsed-response` 条目。

- [ ] **Step 4: 验证用户 API 和直接消费者**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/user tests/views/account/users tests/views/account/sessions tests/views/account/profile --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
```

Expected: 全部通过。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/api/user web/tests/api/user web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 严格解析用户与会话响应"
```

---

### Task 8: 严格解析系统邮件与操作日志 API

**Files:**
- Modify: `web/src/api/system/mail.ts`
- Modify: `web/src/api/system/operationlog.ts`
- Modify: `web/tests/api/system/mail.test.ts`
- Modify: `web/tests/api/system/operationlog.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: Task 5 parser 基础和现有 Mail/OperationLog DTO。
- Produces: 邮件配置、模板、日志详情、收件规则及所有写结果严格解析；操作日志保留 `requestData/responseData: unknown`，但外层字段和分页严格解析。

- [ ] **Step 1: 增加全部写响应和 unknown payload 外壳测试**

```ts
it('keeps operation payloads unknown but rejects unknown wrapper fields', async () => {
  requestMock.mockResolvedValue({
    list: [{
      id: 1, requestId: 'req-1', userId: null, userName: '', sessionId: null,
      platform: 'admin', method: 'POST', route: '/api/test', module: 'test', action: 'create',
      clientIp: '127.0.0.1', userAgent: 'vitest', statusCode: 200, isSuccess: YesNo.Yes,
      latencyMs: 3, requestData: { arbitrary: true }, responseData: ['opaque'],
      createdAt: '2026-09-02T00:00:00Z', updatedAt: '2026-09-02T00:00:00Z', extra: true,
    }], total: 1, page: 1, pageSize: 20,
  })
  await expect(getOperationLogs({ page: 1, pageSize: 20 })).rejects.toBeInstanceOf(ProtocolError)
})
```

Mail 测试必须覆盖 `save/delete/test/update/status/deleteMany` 的明确结果，不允许无类型 `request()`；列表与详情继续拒绝秘密字段和未知字段。

- [ ] **Step 2: 确认系统 API 的直接泛型和无返回类型路径失败**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/system --pool=threads --maxWorkers=1
```

Expected: 新增 mutation 和操作日志畸形响应测试 FAIL。

- [ ] **Step 3: 完成系统模块严格解析**

所有调用显式使用 `request<unknown>`。Mail 每个函数声明完整 `Promise<...>` 返回类型，数组先 `expectArray` 再逐项 parser；删除和测试接口按真实后端契约解析空对象或明确结果。Operation Log 对 `requestData`、`responseData` 原样保留 unknown，不对内部业务数据猜测结构。

从 baseline 删除两个系统模块的 `api-unparsed-response` 条目。

- [ ] **Step 4: 验证系统 API 与页面**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/system tests/views/message/mail tests/views/system/operation-logs --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
```

Expected: 全部通过。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/api/system web/tests/api/system web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 严格解析邮件与操作日志响应"
```

---

### Task 9: 严格解析对象存储 API 并拒绝必填数组兜底

**Files:**
- Modify: `web/src/api/storage/cosconfig.ts`
- Modify: `web/src/api/storage/uploadrule.ts`
- Modify: `web/src/api/storage/upload.ts`
- Modify: `web/tests/api/storage/cosconfig.test.ts`
- Modify: `web/tests/api/storage/uploadrule.test.ts`
- Modify: `web/tests/api/storage/upload.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: Task 5 parser 基础、`YesNo` 和存储 DTO。
- Produces: COS 列表/详情实际调用 `parseCosConfigResponse`；Upload Rule page-init 的 `platforms/configs` 必须为数组；上传凭证、对象 URL 和全部 mutation 结果严格解析。

- [ ] **Step 1: 把 null 兼容测试改成协议拒绝测试**

```ts
it('rejects null page-init collections instead of inventing empty arrays', async () => {
  requestMock.mockResolvedValue({ platforms: null, configs: [] })
  await expect(getUploadRulePageInit()).rejects.toBeInstanceOf(ProtocolError)
})

it('rejects a COS list item containing a secret', async () => {
  requestMock.mockResolvedValue({
    list: [{ ...config, secretId: 'leak' }], total: 1, page: 1, pageSize: 20,
  })
  await expect(listCosConfigs({ page: 1, pageSize: 20 })).rejects.toBeInstanceOf(ProtocolError)
})
```

上传测试再覆盖 method 只能为 `'PUT'`、headers 字符串字典、可选 `publicUrl` 和 exact object URL。

- [ ] **Step 2: 确认当前静默兜底和绕过 parser 会失败**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/storage --pool=threads --maxWorkers=1
```

Expected: null page-init 当前被改成空数组，测试 FAIL；COS 列表秘密字段和上传畸形 DTO 未被拒绝，测试 FAIL。

- [ ] **Step 3: 统一存储 API parser**

将压缩单行实现展开。COS 列表使用 `parsePageResult(value, parseCosConfigResponse, 'COS config page')`，详情直接 parser；Upload Rule 逐项解析 codes/extensions/MIME 数组；page-init 精确检查两个数组；Upload Credential 对 headers 每个 value 做字符串检查。所有写结果使用 `{ id }`、`{ id, isEnabled }` 或 exact empty object parser。

删除 baseline 中三个存储模块的 `api-unparsed-response`、`required-array-fallback` 条目。

- [ ] **Step 4: 验证存储 API 与上传组件**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/api/storage tests/components/UpMedia tests/views/cloud/storage-object --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
```

Expected: 全部通过，`rg -n 'request<[^>]*(DTO|Result|Response|Config|Rule)|\?\? \[\]' src/api/storage -g '*.ts'` 无输出。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/api/storage web/tests/api/storage web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 严格解析对象存储响应"
```

---

### Task 10: 移除请求层 UI 副作用并补齐错误展示所有权

**Files:**
- Modify: `web/src/utils/request.ts`
- Modify: `web/tests/utils/request.test.ts`
- Modify: `web/src/layout/index.vue`
- Modify: `web/tests/layout/index.test.ts`
- Modify: `web/src/layout/components/AppAside/index.vue`
- Modify: `web/tests/layout/components/AppAside.test.ts`
- Modify: `web/src/components/UpMedia/index.vue`
- Modify: `web/tests/components/UpMedia/index.test.ts`
- Modify: `web/src/views/account/profile/index.vue`
- Modify: `web/tests/views/account/profile/index.test.ts`
- Modify: `web/src/views/cloud/storage-object/index.vue`
- Modify: `web/tests/views/cloud/storage-object/index.test.ts`
- Modify: `web/src/views/message/mail/components/config/index.vue`
- Modify: `web/src/views/message/mail/components/template/index.vue`
- Modify: `web/src/views/message/mail/components/log/index.vue`
- Modify: `web/src/views/message/mail/components/rule/index.vue`
- Modify: `web/tests/views/message/mail/index.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: 严格 `ApiError/ProtocolError` 和各页面现有 inline error / success toast。
- Produces: `request(config: AxiosRequestConfig): Promise<unknown>` 和 `unwrapEnvelope(value: unknown): unknown` 只 reject 归一化错误；列表失败进入 inline/AppTable error，表单和 mutation 失败进入所属组件 error；认证最终 401 仍执行清理与跳转但不弹全局通知。

- [ ] **Step 1: 先把 request 测试改成无通知副作用契约**

删除 `ElNotification` mock，增加：

```ts
it('rejects a normalized network error without owning UI feedback', async () => {
  const networkError = new AxiosError('connection refused', AxiosError.ERR_NETWORK)
  const adapter: AxiosAdapter = async () => Promise.reject(networkError)
  const client = createRequestClient('http://localhost:16301')

  await expect(client.get('/health', { adapter })).rejects.toBe(networkError)
})
```

最终 401 只断言一次 refresh、一次 `onUnauthorized` 和所有请求 rejected。Profile、Layout、Storage Connection Test、Mail mutation 增加“失败文本可见且不产生第二次 toast”的行为断言。

- [ ] **Step 2: 确认新契约与当前通知依赖冲突**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/utils/request.test.ts tests/layout/index.test.ts tests/views/account/profile/index.test.ts tests/views/cloud/storage-object/index.test.ts tests/views/message/mail/index.test.ts --pool=threads --maxWorkers=1
```

Expected: FAIL，原因包括 request 仍调用 `ElNotification`，以及调用方存在依赖“request.ts 已提示”的吞错路径。

- [ ] **Step 3: 删除传输层通知并逐调用方接管错误**

从 `request.ts` 删除 `ElNotification`、`notifyRequestError` 及相关调用。将公开 `request` 和 `unwrapEnvelope` 的业务数据返回类型收窄为 `unknown`，使 API 模块必须显式解析；保留 `normalizeResponseError`、Refresh 并发协调、auth store 状态和 unauthorized callback。

将以下吞错路径改为明确状态：

```ts
async function testConfigConnection(row: CosConfig): Promise<void> {
  try {
    await testCosConfig(row.id)
    ElNotification.success({ title: t('storage.testSuccess') })
  } catch (error: unknown) {
    mutationError.value = errorMessage(error)
  }
}
```

Layout logout、Profile 保存/改密、Mail 四个私有区和 UpMedia 同样由所属组件保存可见错误；取消确认仍不作为错误。成功通知继续由页面拥有。删除 baseline 的 `request-ui-side-effect` 条目。

- [ ] **Step 4: 全量验证所有失败路径没有重复或静默**

Run:

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过；`rg -n 'ElNotification|request\.ts.*notification|request\.ts.*提示' src/utils/request.ts src -g '*.ts' -g '*.vue'` 不在 `request.ts` 找到通知调用或依赖注释。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/utils/request.ts web/tests/utils/request.test.ts web/src/layout web/tests/layout web/src/components/UpMedia web/tests/components/UpMedia web/src/views/account/profile web/tests/views/account/profile web/src/views/cloud/storage-object web/tests/views/cloud/storage-object web/src/views/message/mail web/tests/views/message/mail web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 统一前端请求错误展示所有权"
```

---

### Task 11: 以邮件管理页建立页面整改样板

**Files:**
- Move: `web/src/views/message/mail/components/config/index.vue` -> `web/src/views/message/mail/components/ConfigPanel/index.vue`
- Move: `web/src/views/message/mail/components/template/index.vue` -> `web/src/views/message/mail/components/TemplateTable/index.vue`
- Move: `web/src/views/message/mail/components/log/index.vue` -> `web/src/views/message/mail/components/LogTable/index.vue`
- Move: `web/src/views/message/mail/components/rule/index.vue` -> `web/src/views/message/mail/components/RuleTable/index.vue`
- Modify: `web/src/views/message/mail/index.vue`
- Create: `web/tests/views/message/mail/components/ConfigPanel.test.ts`
- Create: `web/tests/views/message/mail/components/TemplateTable.test.ts`
- Create: `web/tests/views/message/mail/components/LogTable.test.ts`
- Create: `web/tests/views/message/mail/components/RuleTable.test.ts`
- Modify: `web/tests/views/message/mail/index.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: Mail API、`AppTable`、`AppDialog`、页面权限计算和 Task 10 错误状态。
- Produces: PascalCase 私有组件目录；父页只协调 tab、权限与 authoritative reload；四个区域保留现有 props/emits，Dialog 统一使用 `AppDialog`。

- [ ] **Step 1: 为样板结构写行为测试**

```ts
it('keeps the parent as tab coordinator and reloads after a child save', async () => {
  const wrapper = mountMailPage(['system:mail:list', 'system:mail:config:update'])
  await flushPromises()
  wrapper.getComponent({ name: 'ConfigPanel' }).vm.$emit('saved')
  await flushPromises()
  expect(getMailConfigMock).toHaveBeenCalledTimes(2)
})
```

私有组件测试分别覆盖：Config 的 secret 永不回填；Template/Rule 使用 AppDialog 并在失败时保留表单；Log 选择、详情和分页 emits；所有普通表格仍为 AppTable。

- [ ] **Step 2: 确认新组件边界尚不存在**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/message/mail --pool=threads --maxWorkers=1
```

Expected: FAIL，找不到 PascalCase 私有组件或 `AppDialog` 边界。

- [ ] **Step 3: 移动组件并按 Element Plus 规则清理布局**

给四个组件加与目录一致的 `defineOptions({ name: 'ConfigPanel' })` 等名称。Template/Rule 的原生 `el-dialog` 改为 AppDialog；Dialog 单列表单移除 row/col，真实双列字段保留一层 `el-row :gutter` 与两个响应式 col。删除只服务固定宽度、普通 gap 和单 `span=24` 的 class；无必要 style block 直接删除，剩余少量样式改为 `<style scoped>`。

父页保持：

```vue
<ConfigPanel v-if="activeTab === 'config'" :config="config" @saved="loadConfig" @deleted="loadConfig" />
<TemplateTable v-else-if="activeTab === 'templates'" :templates="templates" @refresh="loadTemplates" />
<LogTable v-else-if="activeTab === 'logs'" :logs="logs" @refresh="loadLogs" @page-change="changeLogPage" />
<RuleTable v-else :rules="rules" @refresh="loadRules" />
```

从 baseline 删除邮件页和组件的已整改 style/目录条目。

- [ ] **Step 4: 验证邮件样板页**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/message/mail tests/api/system/mail.test.ts --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过；邮件权限、tab 可见性、配置秘密、日志详情和 mutation 行为不变。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/views/message/mail web/tests/views/message/mail web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 按统一规范整改邮件管理页"
```

---

### Task 12: 拆分对象存储页面的配置与规则职责

**Files:**
- Create: `web/src/views/cloud/storage-object/components/CosConfigPanel/index.vue`
- Create: `web/src/views/cloud/storage-object/components/CosConfigDialog/index.vue`
- Create: `web/src/views/cloud/storage-object/components/UploadRulePanel/index.vue`
- Create: `web/src/views/cloud/storage-object/components/UploadRuleDialog/index.vue`
- Create: `web/src/views/cloud/storage-object/cos-config-form.ts`
- Create: `web/src/views/cloud/storage-object/upload-rule-form.ts`
- Modify: `web/src/views/cloud/storage-object/index.vue`
- Create: `web/tests/views/cloud/storage-object/cos-config-form.test.ts`
- Create: `web/tests/views/cloud/storage-object/upload-rule-form.test.ts`
- Create: `web/tests/views/cloud/storage-object/components/CosConfigDialog.test.ts`
- Create: `web/tests/views/cloud/storage-object/components/UploadRuleDialog.test.ts`
- Modify: `web/tests/views/cloud/storage-object/index.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: 严格 Storage API、AppSearch、AppTable、AppDialog。
- Produces: `CosConfigDialog` emits `submit: [{ id: number | null; input: CreateCosConfigInput | UpdateCosConfigInput }]`；`UploadRuleDialog` emits `submit: [{ id: number | null; input: UploadRuleInput }]`；两个 Panel 只接收已解析数据并 emit 查询、分页和行命令；父页拥有 API 调用与 tab 协调。

- [ ] **Step 1: 先锁定纯 Payload 和 Dialog 边界**

```ts
it('omits immutable and empty secret fields from a COS update', () => {
  expect(toUpdateCosConfigInput(cosForm({ secretId: '', secretKey: '', isEnabled: YesNo.No }))).toEqual({
    name: 'Main', appId: '1250000000', bucket: 'admin', region: 'ap-guangzhou',
    endpoint: null, bucketDomain: null, remark: '',
  })
})

it('converts upload size from MB to bytes without adding immutable update fields', () => {
  expect(toUploadRuleInput(ruleForm({ maxFileSizeMB: 8 }), 'update')).toMatchObject({
    maxFileSizeBytes: 8 * 1024 * 1024,
  })
})
```

Dialog 测试断言表单失败不 emit、成功只 emit 精确 payload、public 模式警告和 creatable 多选仍存在。

- [ ] **Step 2: 确认逻辑模块和组件边界尚不存在**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/cloud/storage-object --pool=threads --maxWorkers=1
```

Expected: FAIL，缺少新逻辑文件和私有组件。

- [ ] **Step 3: 按资源拆分页面并简化表单布局**

`index.vue` 只保留权限、active tab、两类查询数据、load/mutation 协调和四个 Dialog/Panel 的组装。URL 校验、blank form、MB/bytes 转换、全选集合和 payload 映射进入两个纯逻辑文件。Panel 使用 AppSearch/AppTable，不访问 API。Dialog 使用 `el-form` 和 AppDialog，并以 `submit: [{ id, input }]` 把创建/更新意图交给父页；配置与规则中的真实响应式双列保留一层 row/col，单列字段直接为 form-item，控件固定宽度使用 `style` 或公开 width。

删除页面的压缩单行函数和无必要 SCSS；页面目标低于 400 行，私有组件目标低于 300 行。从 baseline 删除存储页 `oversized-sfc` 条目。

- [ ] **Step 4: 验证对象存储完整行为**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/cloud/storage-object tests/api/storage tests/components/UpMedia --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过；初始只加载 COS、page-init 权威数据、secret 省略、MB 转换、状态/删除/测试确认和错误状态保持不变。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/views/cloud/storage-object web/tests/views/cloud/storage-object web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 拆分对象存储配置与规则页面"
```

---

### Task 13: 拆分认证平台表单并保留真实栅格

**Files:**
- Create: `web/src/views/permission/auth-platforms/components/AuthPlatformFormDialog/index.vue`
- Create: `web/src/views/permission/auth-platforms/auth-platform-form.ts`
- Modify: `web/src/views/permission/auth-platforms/index.vue`
- Create: `web/tests/views/permission/auth-platforms/auth-platform-form.test.ts`
- Create: `web/tests/views/permission/auth-platforms/components/AuthPlatformFormDialog.test.ts`
- Modify: `web/tests/views/permission/auth-platforms/index.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: Auth Platform API、AppSearch/AppTable/AppDialog 和现有权限码。
- Produces: `AuthPlatformForm`、`defaultAuthPlatformForm()`、`toCreateAuthPlatformInput()`、`toUpdateAuthPlatformInput()`、`securityChanged()`；Dialog emits `submit`，父页负责 API 和 reload。

- [ ] **Step 1: 将 TTL、builtin admin 和 payload 规则下沉为失败测试**

```ts
it('restores exact system TTL defaults and locks builtin admin registration', () => {
  expect(defaultAuthPlatformForm()).toMatchObject({
    accessTTLSeconds: 900,
    refreshTTLSeconds: 604800,
    sessionCacheTTLSeconds: 300,
    accessCacheTTLSeconds: 300,
  })
  expect(canEditRegistration({ code: 'admin', isBuiltin: YesNo.Yes })).toBe(false)
})
```

Dialog 测试继续覆盖四个 TTL help、创建 code 可编辑、编辑 code 只读、恢复默认不提交、builtin admin 强制关闭注册。

- [ ] **Step 2: 确认新模块缺失**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/permission/auth-platforms --pool=threads --maxWorkers=1
```

Expected: FAIL，缺少 `auth-platform-form.ts` 和 Dialog 组件。

- [ ] **Step 3: 抽取表单生命周期并清理布局**

父页保留 AppSearch/AppTable、权限、查询、状态/删除和 Dialog submit。Dialog 内 basic 两字段、四 TTL、三安全开关和 session 设置都是实际同行网格，可以保留一层 row/col；删除同时承担同一布局的 flex/grid CSS、单 `span=24` col 和只为 input-number 宽度创建的 class。仍需响应式滚动或状态 tag 的样式改为普通 scoped CSS。

父页目标低于 400 行，Dialog 低于 300 行。从 baseline 删除认证平台 `oversized-sfc` 条目。

- [ ] **Step 4: 验证认证平台页面**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/permission/auth-platforms tests/api/auth/platform.test.ts --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过；权限独立、表格密度、TTL 文案、builtin 限制、分页和 payload 不变。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/views/permission/auth-platforms web/tests/views/permission/auth-platforms web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 拆分认证平台表单职责"
```

---

### Task 14: 整改用户、会话和日志页面

**Files:**
- Create: `web/src/views/account/users/components/UserEditDialog/index.vue`
- Create: `web/src/views/account/users/components/UserRoleDialog/index.vue`
- Create: `web/src/views/account/users/user-rules.ts`
- Modify: `web/src/views/account/users/index.vue`
- Modify: `web/src/views/account/sessions/index.vue`
- Modify: `web/src/views/account/login-logs/index.vue`
- Modify: `web/src/views/system/operation-logs/index.vue`
- Create: `web/tests/views/account/users/user-rules.test.ts`
- Create: `web/tests/views/account/users/components/UserEditDialog.test.ts`
- Create: `web/tests/views/account/users/components/UserRoleDialog.test.ts`
- Modify: `web/tests/views/account/users/index.test.ts`
- Modify: `web/tests/views/account/sessions/index.test.ts`
- Create: `web/tests/views/account/login-logs/index.test.ts`
- Modify: `web/tests/views/system/operation-logs/index.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: User/System API、AppSearch/AppTable/AppDialog 和现有保护规则。
- Produces: `isProtectedUser(row: UserListItem, currentUserID: number): boolean`、`normalizePhone(value: string): string | null`、`normalizeRoleIDs(selected: readonly number[], protectedRoleID: number | null, isSuperAdminActor: boolean): number[]` 等纯规则；两个用户 Dialog 只管理各自表单并 emit 精确 submit；四个页面继续保留各自 authoritative query/error state。

- [ ] **Step 1: 锁定用户保护和缺失的登录日志页面行为**

```ts
it('preserves protected super-admin roles while normalizing selected ids', () => {
  expect(normalizeRoleIDs([4, 2, 4], 1, false)).toEqual([1, 2, 4])
  expect(normalizeRoleIDs([4, 2, 4], 1, true)).toEqual([2, 4])
})
```

新增 Login Log 页面测试：初次请求带 `{ page: 1, pageSize: 20 }`；AppSearch 提交 account/event/success/time range；AppTable 分页更新；失败展示 error 而不是 fake empty。

- [ ] **Step 2: 确认新职责边界和登录日志测试失败**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/account/users tests/views/account/login-logs --pool=threads --maxWorkers=1
```

Expected: FAIL，缺少用户规则模块、Dialog 组件和 Login Log 行为测试所需的明确 error result-state。

- [ ] **Step 3: 拆分用户 Dialog 并逐页清理布局/CSS**

用户父页保留 list/query/permissions/mutations；编辑和角色授权分别进入独立 AppDialog。会话统计的两列为真实响应式网格，保留一层 row/col；普通按钮间距改用 el-space。Operation Log 的 meta 三列与 request/response 两列是真实网格，保留；去除与 row/col 重复的 grid CSS。Login Log 继续使用 AppSearch/AppTable，并加入显式 error state。四页无 Sass 能力时改普通 scoped CSS或删除 style。

用户页目标低于 400 行；从 baseline 删除用户页 `oversized-sfc` 和本任务已审查样式条目。

- [ ] **Step 4: 运行账户与日志验证**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/account/users tests/views/account/sessions tests/views/account/login-logs tests/views/system/operation-logs tests/api/user tests/api/system/operationlog.test.ts --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过；self/super-admin 保护、角色排序、当前 Session 禁撤销、批量撤销、日志只读详情和分页筛选保持不变。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/views/account/users web/src/views/account/sessions web/src/views/account/login-logs web/src/views/system/operation-logs web/tests/views/account/users web/tests/views/account/sessions web/tests/views/account/login-logs web/tests/views/system/operation-logs web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 整改用户会话与日志页面"
```

---

### Task 15: 拆分角色资料、授权矩阵和权限差异

**Files:**
- Create: `web/src/views/permission/roles/components/RoleFormDialog/index.vue`
- Create: `web/src/views/permission/roles/components/RolePermissionDialog/index.vue`
- Create: `web/src/views/permission/roles/permission-matrix.ts`
- Create: `web/src/views/permission/roles/permission-selection.ts`
- Create: `web/src/views/permission/roles/permission-diff.ts`
- Delete after migration: `web/src/views/permission/roles/role-permission-matrix.ts`
- Modify: `web/src/views/permission/roles/components/RolePermissionMatrix/index.vue`
- Modify: `web/src/views/permission/roles/components/RolePermissionDiffDialog/index.vue`
- Modify: `web/src/views/permission/roles/index.vue`
- Split/update: `web/tests/views/permission/roles/role-permission-matrix.test.ts`
- Create: `web/tests/views/permission/roles/permission-selection.test.ts`
- Create: `web/tests/views/permission/roles/permission-diff.test.ts`
- Create: `web/tests/views/permission/roles/components/RoleFormDialog.test.ts`
- Create: `web/tests/views/permission/roles/components/RolePermissionDialog.test.ts`
- Modify: `web/tests/views/permission/roles/components/RolePermissionMatrix.test.ts`
- Modify: `web/tests/views/permission/roles/index.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: Role API、现有 matrix 算法和 AppDialog/AppTable/AppSearch。
- Produces: `permission-matrix.ts` 只构建平台/组/行并导出 `buildRolePermissionMatrix(platform: RolePermissionPlatform): RoleMatrixPlatform[]`；`permission-selection.ts` 只展开、切换和规范化选择并保留 `expandDirectMenuIDs`、`normalizeDirectMenuIDs`、`toggleMatrixPage`、`toggleMatrixAction`、`toggleMatrixGroup`；`permission-diff.ts` 导出 `RolePermissionDiff` 与 `diffMenuIDs(before: readonly number[], after: readonly number[]): RolePermissionDiff`；父页只协调列表和 Dialog。

- [ ] **Step 1: 先按职责重写纯逻辑 import 测试**

```ts
import { buildRolePermissionMatrix } from '@/views/permission/roles/permission-matrix'
import { expandDirectMenuIDs, normalizeDirectMenuIDs } from '@/views/permission/roles/permission-selection'
import { diffMenuIDs } from '@/views/permission/roles/permission-diff'

it('round-trips minimal direct grants through the effective matrix', () => {
  const groups = buildRolePermissionMatrix(platformsFixture())
  const effective = expandDirectMenuIDs(groups, [13])
  expect(normalizeDirectMenuIDs(groups, effective)).toEqual([13])
  expect(diffMenuIDs([13], [13, 21])).toEqual({ added: [21], removed: [] })
})
```

RolePermissionDialog 测试覆盖平台 tab、全选/清空、取消 diff 后保留选择、空授权可提交、保存失败保留双 Dialog。

- [ ] **Step 2: 确认新模块与 Dialog 边界尚不存在**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/permission/roles --pool=threads --maxWorkers=1
```

Expected: FAIL，缺少三个纯逻辑模块和两个 Dialog 组件。

- [ ] **Step 3: 按职责拆分并保留权限矩阵例外**

移动现有导出而不改变算法结果：build/getters 进入 matrix，selection state 与 toggle/normalize 进入 selection，diff 类型与函数进入 diff。`RolePermissionMatrix` 保留原生 `el-table`，在模板旁加入明确注释：

```vue
<!-- AppTable exception: this is an interactive page/action permission matrix, not a paginated data list. -->
```

RoleFormDialog 管理 code/name；RolePermissionDialog 管理 load/selection/diff 生命周期并通过 emits 通知父页保存成功。父页保留 API、权限和列表 reload。页面低于 400 行，各组件低于 300 行，纯逻辑文件低于 250 行。从 baseline 删除角色页 `oversized-sfc` 条目。

- [ ] **Step 4: 验证角色完整行为**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/permission/roles tests/api/permission/role.test.ts --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过；权限矩阵有效选择、最小 direct grants、默认角色保护、确认文案、滚动所有权和错误恢复不变。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/views/permission/roles web/tests/views/permission/roles web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 拆分角色资料与授权矩阵职责"
```

---

### Task 16: 拆分菜单树、表单和纯逻辑

**Files:**
- Create: `web/src/views/permission/menus/components/MenuTreeTable/index.vue`
- Create: `web/src/views/permission/menus/components/MenuFormDialog/index.vue`
- Create: `web/src/views/permission/menus/menu-tree.ts`
- Create: `web/src/views/permission/menus/menu-form.ts`
- Modify: `web/src/views/permission/menus/filter-menu-tree.ts`
- Modify: `web/src/views/permission/menus/index.vue`
- Create: `web/tests/views/permission/menus/menu-tree.test.ts`
- Create: `web/tests/views/permission/menus/menu-form.test.ts`
- Create: `web/tests/views/permission/menus/components/MenuTreeTable.test.ts`
- Create: `web/tests/views/permission/menus/components/MenuFormDialog.test.ts`
- Modify: `web/tests/views/permission/menus/filter-menu-tree.test.ts`
- Modify: `web/tests/views/permission/menus/index.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: Menu API、IconSelect/AppDIcon、现有 tree filtering 和权限码。
- Produces: `MenuTreeTable` emits `create-child: [parent: ManagedMenuNode | null]`、`edit: [node: ManagedMenuNode]`、`status: [node: ManagedMenuNode]`、`delete: [node: ManagedMenuNode]`、`expand-change: [keys: readonly string[]]`；`MenuFormDialog` emits `submit: [{ id: number | null; input: CreateMenuInput | UpdateMenuInput }]`；`menu-tree.ts` 导出 `menuRowKey(id: number): string`、`flattenWithChildren`、`collectSubtreeIDs` 并统一字符串 row keys；`menu-form.ts` 负责 form state、parent options、类型切换和 payload。

- [ ] **Step 1: 用行为测试替代源码字符串断言**

删除 `readFileSync` 和“computed 源码形态”断言，增加真实 locale 切换：

```ts
it('updates menu type labels when the application locale changes', async () => {
  const wrapper = mountPage(pinia, ['permission:menu:list'])
  await flushPromises()
  expect(wrapper.get('[data-testid="menu-table"]').text()).toContain('页面')
  setLocale('en-US')
  await nextTick()
  expect(wrapper.get('[data-testid="menu-table"]').text()).toContain('Page')
})
```

纯逻辑测试锁定：字符串 expanded keys、搜索前后恢复、编辑子树不得成为父节点、page code 必须 `:view`、action code 不得 `:view`、action 强制 hidden 且清空 path/component/i18n。

- [ ] **Step 2: 确认新组件和纯逻辑模块缺失**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/permission/menus --pool=threads --maxWorkers=1
```

Expected: FAIL，缺少 `MenuTreeTable`、`MenuFormDialog`、`menu-tree.ts` 和 `menu-form.ts`。

- [ ] **Step 3: 抽取菜单职责并保留树表专用实现**

`index.vue` 只保留平台 tab、权限、顶层 load/mutation/rebuild 和两个私有组件协调。MenuTreeTable 管理展示、expand/collapse 控件和行命令，不调用 API。其原生 `el-table` 前加入：

```vue
<!-- AppTable exception: Element Plus tree rows and controlled expand-row-keys are required here. -->
```

MenuFormDialog 管理 Element Plus form、IconSelect 和字段显隐；只有 name/code 与真实同行字段使用一层 row/col，其余 `span=24` col 全部移除。`menu-form.ts` 返回新对象，不修改 API node。保留 protected menu 的结构锁定与展示字段可编辑规则。

页面低于 400 行，两个组件低于 300 行，纯逻辑文件低于 250 行。删除 baseline 中菜单 `oversized-sfc` 与 `source-shape-test` 条目。

- [ ] **Step 4: 验证最高风险菜单页**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/views/permission/menus tests/api/permission/menu.test.ts tests/api/permission/menu-fields.test.ts tests/router --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过；平台切换、搜索展开恢复、protected 节点、CRUD payload、access cache rebuild、错误保留旧树和动态路由均不变。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/views/permission/menus web/tests/views/permission/menus web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 拆分菜单树与表单职责"
```

---

### Task 17: 收尾 Login、Profile、Dashboard 和 Layout

**Files:**
- Create: `web/src/views/auth/login/components/LoginForm/index.vue`
- Create: `web/src/views/auth/login/types.ts`
- Modify: `web/src/views/auth/login/index.vue`
- Create: `web/src/views/account/profile/components/ProfileForm/index.vue`
- Create: `web/src/views/account/profile/components/PasswordForm/index.vue`
- Modify: `web/src/views/account/profile/index.vue`
- Modify: `web/src/views/dashboard/index.vue`
- Modify: `web/src/views/dashboard/components/ReadinessChart/index.vue`
- Create: `web/src/layout/components/RouteTabs/route-tabs.ts`
- Modify: `web/src/layout/components/RouteTabs/index.vue`
- Modify: `web/src/layout/components/SettingDrawer/index.vue`
- Modify: `web/src/layout/components/AppAside/index.vue`
- Modify: `web/src/layout/components/AppHeader/index.vue`
- Modify: `web/src/layout/components/AppFooter/index.vue`
- Modify: `web/src/layout/components/PermissionMenuNode/index.vue`
- Modify: `web/src/layout/index.vue`
- Create: `web/tests/layout/components/RouteTabs/route-tabs.test.ts`
- Modify: `web/tests/layout/components/RouteTabs.test.ts`
- Modify: `web/tests/layout/components/SettingDrawer.test.ts`
- Modify: `web/tests/layout/index.test.ts`
- Create: `web/tests/views/auth/login/components/LoginForm.test.ts`
- Modify: `web/tests/views/auth/login/index.test.ts`
- Create: `web/tests/views/account/profile/components/ProfileForm.test.ts`
- Create: `web/tests/views/account/profile/components/PasswordForm.test.ts`
- Modify: `web/tests/views/account/profile/index.test.ts`
- Modify: `web/tests/views/dashboard/index.test.ts`
- Modify: `web/scripts/frontend-architecture-baseline.mjs`

**Interfaces:**
- Consumes: 现有 Auth/Profile/Health API、UI preference store、Access tree。
- Produces: Login page 负责 route/auth 协调，LoginForm emits `submit: [input: LoginInput]`；Profile 两表单各自 emit `submit: [input: UpdateAccountProfileInput]` 和 `submit: [input: ChangePasswordInput]`；RouteTabs 纯逻辑导出 `closeRouteTab`、`closeOtherRouteTabs`、`closeAllRouteTabs`、`findMenuPage` 等 tab 增删与 Access tree title 解析函数，SFC 保留 DOM scroll/context menu。

- [ ] **Step 1: 为 RouteTabs 和表单组件写独立行为测试**

```ts
it('closes the active tab and chooses the nearest remaining tab', () => {
  const tabs = [dashboardTab(), routeTab('/account/users'), routeTab('/access/roles')]
  expect(closeRouteTab(tabs, '/account/users', '/account/users')).toEqual({
    tabs: [dashboardTab(), routeTab('/access/roles')],
    nextPath: '/access/roles',
  })
})
```

LoginForm 测试 required validation、pending 锁定和原密码不 trim；ProfileForm 测 avatar object key 与 nullable phone/birthday；PasswordForm 测确认密码和失败保留输入。

- [ ] **Step 2: 确认新表单与 RouteTabs 逻辑模块缺失**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/layout tests/views/auth/login tests/views/account/profile tests/views/dashboard --pool=threads --maxWorkers=1
```

Expected: FAIL，缺少新组件和 `route-tabs.ts`。

- [ ] **Step 3: 拆分组件并完成样式必要性审查**

LoginForm 不访问 Router；父页执行 safe redirect 和 auth bootstrap。Profile 两个表单不访问权限 Store，父页按权限决定是否渲染并执行 API。RouteTabs 把 `findMenuPage`、tab 去重、close/closeOthers/closeAll 变为纯函数，SFC 保留 wheel scroll、focus、context menu 和 refresh/fullscreen emits，使 SFC 低于 400 行。

SettingDrawer 删除单个 `span=24` row/col，使用 el-form/el-space；Profile 外层双栏与表单内部字段拆到组件后各自只有一层网格。逐个 style block 审查：应用壳滚动、响应式侧栏、fullscreen、焦点和 transition 保留；普通 gap、固定宽度和 Element Plus 已有状态删除；普通 CSS 能表达时去掉 `lang="scss"`。

从 baseline 删除这些页面和 Layout 的剩余 oversize/style 审查条目。

- [ ] **Step 4: 验证应用壳和入口页面**

Run:

```powershell
cd D:\admin\web
pnpm vitest run tests/layout tests/views/auth/login tests/views/account/profile tests/views/dashboard --pool=threads --maxWorkers=1
pnpm check:architecture
pnpm typecheck
pnpm build
```

Expected: 全部通过；登录 redirect、防重复提交、Profile 权限、Dashboard 真实依赖状态、route tabs、移动侧栏、fullscreen、主题和语言行为不变。

- [ ] **Step 5: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/src/views/auth/login web/src/views/account/profile web/src/views/dashboard web/src/layout web/tests/views/auth/login web/tests/views/account/profile web/tests/views/dashboard web/tests/layout web/scripts/frontend-architecture-baseline.mjs
git commit -m "refactor: 收尾前端入口页面与应用壳"
```

---

### Task 18: 清空迁移清单并执行全量验收

**Files:**
- Modify: `web/scripts/frontend-architecture-baseline.mjs`
- Modify if a real false positive is found: `web/scripts/check-frontend-architecture.mjs`
- Modify only for verified defects: files reported by the final commands below

**Interfaces:**
- Consumes: Tasks 1-17 的全部产物。
- Produces: 空迁移 baseline、无遗留扫描结果和可重复执行的 `verify:frontend`；原生树表例外只依赖组件内明确注释，不依赖迁移 allowlist。

- [ ] **Step 1: 将迁移 baseline 清空**

```js
export const architectureBaseline = Object.freeze([])
```

不得把真实剩余违规改名后重新登记；菜单树表和权限矩阵仅允许扫描器识别同文件中的 `AppTable exception:` 注释。

- [ ] **Step 2: 确认没有被 baseline 掩盖的遗留**

Run:

```powershell
cd D:\admin\web
pnpm check:architecture
```

Expected: PASS。若失败，逐项回到所属文件修复；只有扫描器把合规 AST/路径误判时才修改扫描器，并增加可复现 fixture 检查，不能新增 allowlist。

- [ ] **Step 3: 执行定向残留扫描**

Run:

```powershell
cd D:\admin\web
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" src tests -g "*.ts" -g "*.vue"
rg -n "request<" src/api -g "*.ts" | rg -v "request<unknown>";
rg -n "\brequest\(" src/api -g "*.ts"
rg -n "\?\? \[\]" src/api -g "*.ts"
rg -n "readFileSync|source\.toContain|source\.not\.toContain" tests -g "*.test.ts"
rg -n "@src/|from ['\"]\.\./|import\(['\"]\.\./|views/\*\*/index\.vue|staticPageBinding" src tests -g "*.ts" -g "*.vue"
Get-ChildItem src -Recurse -Filter *.vue | Where-Object { $_.Name -ne "index.vue" -and $_.Name -ne "App.vue" }
```

Expected: 五组命令均无输出。`readFileSync` 若用于非源码 fixture，必须改用结构化 fixture API，不能恢复读取 `.vue` 源码断言。

- [ ] **Step 4: 执行最终完整验证**

Run:

```powershell
cd D:\admin\web
pnpm format:check
pnpm lint
pnpm check:architecture
pnpm typecheck
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
git diff --check
```

Expected: 七条命令全部通过。再运行 `git status --short`，确认没有 `.env`、`dist/`、coverage、本地浏览器状态、临时 fixture 或其他无关文件进入变更集。

- [ ] **Step 5: 对照 Spec 完成最终审查**

逐项确认：

```text
所有 Vue（除 App.vue）均为目录/index.vue
所有业务页面动态注册且菜单无静态特例
所有 API 响应为 unknown -> exact parser -> DTO
request.ts 无 Element Plus UI 副作用
普通列表/筛选使用 AppTable/AppSearch
原生 el-table 仅菜单树和权限矩阵且有原因注释
简单表单无无意义 row/col，真实网格无重复 CSS
无必要 style block 已删除，普通 CSS 优先
页面与组件退出超大文件迁移清单
现有权限、路由、错误、国际化、移动端和主题测试全部通过
```

- [ ] **Step 6: 建议提交边界（仅用户授权后执行）**

```powershell
git add web/scripts/frontend-architecture-baseline.mjs web/scripts/check-frontend-architecture.mjs web/src web/tests
git commit -m "chore: 完成前端整改全量验收"
```

---

## Execution Notes

- 推荐按 Task 顺序执行，不并行修改共享的 `package.json`、架构 baseline、路由、`request.ts` 或同一页面。
- 每个 Task 完成后先查看 `git status --short` 与 `git diff --check`；若用户未授权提交，只保留已验证工作区并进入下一任务，不执行计划中的 commit 命令。
- 页面拆分期间不修改后端或数据库来迎合前端测试；DTO 不匹配时以真实数据库/API 契约和已确认 Spec 为准，协议违规必须显式失败。
- 任一定向测试显示行为变化时，先恢复原行为再继续结构整改；不得通过删除测试、放宽 parser 或扩大 baseline 让门禁变绿。
