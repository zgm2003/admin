# 前端表格、搜索与图标基础能力实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 对齐旧项目的前端公共基线，完成 `AppTable` 刷新/居中能力、统一按钮、`Search`、`DIcon`、`IconSelect` 和全量 `el-select-v2` 迁移。

**Architecture:** 公共组件只负责展示和交互事件，页面继续持有查询状态、权限判断和 API 调用。请求链保持 `view -> api/<module>.ts -> utils/request.ts -> Go API`；`AppTable` 的刷新按钮只发出 `refresh` 事件，不直接请求。所有组件使用显式 TypeScript 类型，不引入业务层通用抽象。

**Tech Stack:** Vue 3、TypeScript strict、Element Plus、Pinia、Vitest、`@iconify/vue`、pnpm。

---

## 工作区约束

- 保留工作区中上一轮布局和 UI 改造的未提交修改，不回滚、不覆盖无关文件。
- 不自动执行 `git commit`、`fetch`、`pull` 或 `push`；每个任务完成后用 `git diff` 检查变更范围，由仓库所有者决定提交。
- 不使用 Docker，不修改后端、数据库、认证或 RBAC 代码。
- 所有页面业务 TypeScript 禁止 `any`、`as any`、`any[]`、`Record<string, any>` 和 `@ts-ignore`。

### Task 1: 准备图标依赖与公共组件出口

**Files:**
- Modify: `web/package.json`
- Modify: `web/pnpm-lock.yaml`
- Create or modify: `web/src/components/DIcon/index.ts`
- Create or modify: `web/src/components/Search/index.ts`
- Create or modify: `web/src/components/IconSelect/index.ts`

- [ ] **Step 1: 确认当前依赖和出口模式**

运行：

```powershell
cd D:\admin\web
Get-Content package.json
Get-Content src\components\AppTable\index.ts
```

确认公共组件使用命名导出，且当前项目没有 `@iconify/vue`。

- [ ] **Step 2: 安装 Iconify 依赖**

运行：

```powershell
pnpm add @iconify/vue
```

预期：`package.json` 增加 `@iconify/vue`，锁文件只增加该依赖及其必要解析，不引入无关包。

- [ ] **Step 3: 建立严格的公共出口**

三个 `index.ts` 只导出对应组件和公开类型，例如：

```ts
export { default as DIcon } from './src/index.vue'
export type { DIconProps } from './src/types'
```

如果组件类型由 Vue 生成类型直接推导，则只导出真实存在的类型，不用宽泛声明掩盖类型错误。

- [ ] **Step 4: 检查依赖和出口变更**

运行：

```powershell
git diff -- web/package.json web/pnpm-lock.yaml web/src/components
```

预期：只包含依赖和三个公共组件的出口准备，不包含业务页面改动。

### Task 2: 扩展 AppTable 的刷新和列对齐契约

**Files:**
- Modify: `web/src/components/AppTable/src/types.ts`
- Modify: `web/src/components/AppTable/src/index.vue`
- Modify: `web/src/components/AppTable/src/index.test.ts`
- Modify: `web/src/components/AppTable/src/types.test.ts`

- [ ] **Step 1: 写默认居中和列级覆盖的失败测试**

在 `index.test.ts` 增加断言：挂载两列，一列无 `elementProps`，另一列配置 `{ elementProps: { align: 'left', headerAlign: 'right' } }`；断言渲染出的 `ElTableColumn` props 分别为默认 center 和显式覆盖值。

```ts
expect(defaultColumn.props('align')).toBe('center')
expect(defaultColumn.props('headerAlign')).toBe('center')
expect(overriddenColumn.props('align')).toBe('left')
expect(overriddenColumn.props('headerAlign')).toBe('right')
```

- [ ] **Step 2: 写内置刷新按钮的失败测试**

挂载默认 `AppTable`，断言存在刷新按钮；触发点击后断言 `wrapper.emitted('refresh')` 有一次。设置 `loading: true` 时断言按钮处于 loading 状态。测试必须通过组件事件验证，不调用任何 API。

- [ ] **Step 3: 实现列属性合并**

在 `columnBindings` 中先设置：

```ts
align: 'center',
headerAlign: 'center',
```

再展开 `column.elementProps`，确保显式列配置覆盖默认值。选择列和展开列的对齐属性显式设置为 center。

- [ ] **Step 4: 实现 AppTable 内置刷新工具栏**

引入 `Refresh` 图标和 i18n 文案，在工具栏右侧始终渲染一个实体 `el-button`。按钮点击只执行 `emit('refresh')`，`:loading` 使用表格 loading 状态；页面 toolbar slot 不再创建刷新按钮。工具栏显示条件必须包含内置刷新按钮，不能因为没有 slot 而隐藏整个 toolbar。

- [ ] **Step 5: 运行 AppTable 测试**

运行：

```powershell
pnpm vitest run src/components/AppTable/src/index.test.ts src/components/AppTable/src/types.test.ts
```

预期：全部通过；若 Element Plus 测试选择器无法定位列，使用组件实例和 `props()` 验证，不改生产代码来迎合测试。

### Task 3: 实现 DIcon

**Files:**
- Create: `web/src/components/DIcon/src/types.ts`
- Create: `web/src/components/DIcon/src/index.vue`
- Create: `web/src/components/DIcon/src/index.test.ts`
- Modify: `web/src/components/DIcon/index.ts`

- [ ] **Step 1: 写 DIcon 类型和行为失败测试**

覆盖三种情况：Element Plus 图标渲染、Iconify 字符串渲染、两种来源同时传入时显示明确错误状态。测试只传真实图标组件或字符串，不使用 `any`。

- [ ] **Step 2: 定义互斥 Props**

使用联合类型表达两种合法输入：

```ts
type ElementIconProps = { component: Component; icon?: never }
type IconifyProps = { component?: never; icon: string }
export type DIconProps = (ElementIconProps | IconifyProps) & {
  size?: string | number
  color?: string
}
```

运行时对缺失/同时存在的来源做明确检查，不能静默 fallback。

- [ ] **Step 3: 实现渲染分支**

Element Plus 图标使用动态组件；Iconify 图标使用 `Icon`；统一包装元素负责 size、color 和 class 透传。非法输入显示可测试的空状态，并在开发环境输出明确错误。

- [ ] **Step 4: 运行 DIcon 测试和类型检查**

运行：

```powershell
pnpm vitest run src/components/DIcon/src/index.test.ts
pnpm exec vue-tsc --noEmit
```

预期：测试和类型检查通过。

### Task 4: 实现 Search

**Files:**
- Create: `web/src/components/Search/src/types.ts`
- Create: `web/src/components/Search/src/index.vue`
- Create: `web/src/components/Search/src/index.test.ts`
- Modify: `web/src/components/Search/index.ts`

- [ ] **Step 1: 写 Search 的失败测试**

覆盖 input、`select-v2`、查询、重置和折叠字段：挂载字段配置，断言选择字段渲染为 `ElSelectV2`，点击查询/重置分别发出对应事件，折叠后隐藏超过 `collapseCount` 的字段。

- [ ] **Step 2: 定义字段类型**

至少定义：

```ts
type SearchOptionValue = string | number
type SearchOption = { label: string; value: SearchOptionValue }
type SearchFieldType = 'input' | 'select-v2'
interface SearchField { key: string; label: string; type: SearchFieldType; options?: SearchOption[]; placeholder?: string; width?: string | number }
```

表单值使用 `Record<string, SearchOptionValue | undefined>` 或调用方明确的泛型约束，禁止 `Record<string, any>`。

- [ ] **Step 3: 实现 Search 渲染和事件**

使用 `el-input` 和 `el-select-v2`；查询和重置通过 emits 交给页面；重置只清理当前字段模型，不调用 API。保留移动端布局和字段折叠，但不引入请求逻辑。

- [ ] **Step 4: 运行 Search 测试**

运行：

```powershell
pnpm vitest run src/components/Search/src/index.test.ts
```

预期：所有字段类型、事件和折叠断言通过。

### Task 5: 实现 IconSelect 并接入菜单表单

**Files:**
- Create: `web/src/components/IconSelect/src/types.ts`
- Create: `web/src/components/IconSelect/src/index.vue`
- Create: `web/src/components/IconSelect/src/index.test.ts`
- Modify: `web/src/components/IconSelect/index.ts`
- Modify: `web/src/views/system/menus/index.vue`
- Modify: `web/src/views/system/menus/index.test.ts`

- [ ] **Step 1: 写 IconSelect 的失败测试**

覆盖关键字过滤、空结果和选择回传：点击一个图标后断言 `select-icon` 事件携带字符串名称；无匹配结果时断言空状态；打开/关闭使用受控 `modelValue`。

- [ ] **Step 2: 实现 IconSelect**

组件内部维护搜索关键字和静态/明确类型的图标目录，使用 `DIcon` 渲染网格；选择后 emit 图标名称并关闭；不让菜单页面知道 Iconify 组件细节。

- [ ] **Step 3: 替换菜单页面图标选择**

移除菜单页面原生图标选择 `el-select`，改用 `IconSelect`；表单只保存图标名称字符串；保留现有菜单新增、编辑、校验和提交逻辑。

- [ ] **Step 4: 运行菜单和组件测试**

运行：

```powershell
pnpm vitest run src/components/IconSelect/src/index.test.ts src/views/system/menus/index.test.ts
```

预期：菜单表单原有行为不变，图标选择断言通过。

### Task 6: 迁移系统页面筛选和 AppTable 刷新

**Files:**
- Modify: `web/src/views/system/users/index.vue`
- Modify: `web/src/views/system/roles/index.vue`
- Modify: `web/src/views/system/menus/index.vue`
- Modify: `web/src/views/system/sessions/index.vue`
- Modify: `web/src/views/system/auth-platforms/index.vue`
- Modify: `web/src/views/system/operation-logs/index.vue`
- Modify: `web/src/layout/components/SettingDrawer.vue`
- Modify: corresponding `*.test.ts` files when selectors or behavior assertions change

- [ ] **Step 1: 迁移 users、roles 筛选**

将原生 `<el-select>` 替换为 `<el-select-v2>`，把 `el-option` 转换为显式 `options`；保留现有 `statusFilter`、`roleFilter` 和查询函数。移除页面刷新按钮，让 `AppTable @refresh` 调用原有 `loadUsers`/`loadRoles`。

- [ ] **Step 2: 迁移 sessions、auth-platforms 筛选**

同样转换状态选项和 loading/clearable 行为；删除重复刷新按钮，保留页面原有刷新函数作为 `@refresh` 处理器。

- [ ] **Step 3: 迁移 menus、operation-logs、SettingDrawer**

替换全部原生选择器；菜单页面的图标字段使用 `IconSelect`；操作日志和菜单页面的刷新入口改为 AppTable 内置按钮；设置抽屉的选择器使用 `el-select-v2`。

- [ ] **Step 4: 按旧项目规则统一操作列按钮**

将迁移页面的编辑、启用/禁用、默认、授权、删除操作改为 `el-button text`，保留权限 `v-if`、disabled 条件、tooltip 和点击函数；顶部新增/批量按钮保留实体按钮。

- [ ] **Step 5: 更新页面测试**

只调整因 DOM 组件从 `ElSelect` 变为 `ElSelectV2` 或刷新按钮移动到 AppTable 导致的选择器；继续断言查询参数、权限隐藏、按钮点击和 API 调用，不把测试改成只检查 HTML 文本。

- [ ] **Step 6: 扫描残留原生选择器和重复刷新按钮**

运行：

```powershell
rg -n "<el-select(\s|>)|<el-option|:icon=\"Refresh\"|t\('.*refresh'\)" web/src/views web/src/layout -g "*.vue"
```

预期：业务页面和布局中没有原生 `<el-select>`/`<el-option>`；AppTable 页面不再单独渲染刷新按钮。路由 tabs 等非 AppTable 场景的刷新不在本次迁移范围内。

### Task 7: 全量验证和工作区审查

**Files:**
- No new files; inspect all changed files and existing tests

- [ ] **Step 1: 运行前端测试**

运行：

```powershell
cd D:\admin\web
pnpm vitest run
```

预期：所有现有和新增测试通过。

- [ ] **Step 2: 运行严格类型检查和构建**

运行：

```powershell
pnpm exec vue-tsc --noEmit
pnpm build
```

预期：无 TypeScript 错误，生产构建成功。

- [ ] **Step 3: 检查 AnyScript 和 diff**

运行：

```powershell
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" web/src -g "*.ts" -g "*.vue"
git diff --check
git status --short
```

预期：本次新增或修改代码没有违规类型写法；`git diff --check` 无输出；工作区只包含本次公共能力、依赖、页面迁移和对应测试。

- [ ] **Step 4: 人工验收关键交互**

启动前端开发服务后检查：

1. 每张 `AppTable` 默认有刷新按钮，点击后页面发起原有列表请求；
2. 普通列表头和内容居中，特殊列按 `elementProps` 覆盖；
3. 操作列按钮统一为文字按钮，颜色语义与旧项目一致；
4. 所有筛选选择器使用 `el-select-v2`；
5. 菜单图标选择器可以搜索、预览、选择并回显。

不创建提交；完成后向仓库所有者报告验证结果和未运行项目。
