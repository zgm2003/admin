# 前端表格、搜索与图标基础能力设计

## 1. 状态

状态：设计已确认，等待用户审阅。

## 2. 背景

当前项目已经有 `AppTable`、`AppDialog` 和部分页面级筛选/操作实现，但不同页面仍存在重复和不一致：

- 表格刷新按钮由页面各自维护；
- 表格列默认对齐规则不统一，无法通过明确列配置覆盖；
- 操作列按钮存在实体按钮、文字按钮和图标按钮混用；
- 页面筛选仍有原生 `el-select`，与旧项目的 `el-select-v2` 基线不一致；
- `DIcon`、`Search`、`IconSelect` 还没有按旧项目的成熟用法在当前项目形成公共能力。

旧项目 `D:\github-project\admin_front_ts` 已有可参考的实现。本次吸收其真实的交互和视觉规则，但按当前项目的目录、lower camel case 契约、i18n 和严格 TypeScript 约束重新实现，不复制旧项目的业务请求、状态管理或模块代码。

## 3. 目标

1. 让 `AppTable` 内置刷新按钮，页面只响应 `refresh` 事件。
2. 让表格表头和单元格默认居中，并允许单列显式覆盖对齐方式。
3. 统一业务操作按钮的视觉和语义，保持与旧项目一致。
4. 建立严格 TypeScript 的 `DIcon`、`Search` 和 `IconSelect` 公共组件。
5. 将当前业务页面中的原生 `el-select` 全部迁移为 `el-select-v2`。
6. 保持页面请求流清晰且线性：

   ```text
   view -> api/<module>.ts -> utils/request.ts -> Go API
   ```

公共组件不访问 API、不读权限 Store、不处理业务错误码。

## 4. 设计决策

### 4.1 AppTable

文件边界：

- `web/src/components/AppTable/index.ts`
- `web/src/components/AppTable/src/index.vue`
- `web/src/components/AppTable/src/types.ts`
- 对应的 Vitest 测试文件

行为约束：

- 刷新按钮由 `AppTable` 统一渲染，默认始终显示；页面不再在 toolbar slot 中重复放置刷新按钮。
- 点击刷新按钮只触发 `refresh` emit，不直接调用请求。
- 刷新按钮的 loading 状态由现有表格 loading 状态或明确的刷新 loading 传入控制。
- 默认 `align: 'center'` 和 `headerAlign: 'center'`。
- 单列的 `elementProps.align` 或 `elementProps.headerAlign` 优先于默认值。
- 选择列、序号列固定居中。
- 保留现有泛型行类型、分页、选择、隐藏列、派生列和 `cell-<key>` 插槽能力。
- 不把搜索条件、权限判断、确认框或业务操作按钮放进 `AppTable`。

列对齐合并顺序必须明确为：

```text
AppTable 默认值 -> 当前列 elementProps 覆盖值
```

组件事件保持最小且明确：

- `refresh`
- `row-click`
- `selection-change`
- `update:pagination`
- `column-change`

### 4.2 按钮规则

按照旧项目的真实实现统一：

- 表格操作列统一使用 `el-button text`；
- 编辑使用 `type="primary"`；
- 启用/禁用使用 `type="warning"`；
- 设为默认使用 `type="success"`；
- 删除使用 `type="danger"`；
- 顶部新增、批量操作、展开收起使用实体 `el-button`；
- 弹窗取消和确认保留普通按钮，不强行改成文字按钮；
- 刷新按钮由 `AppTable` 内置，使用带刷新图标和文字的实体按钮。

本次只调整按钮组件接入、类型和视觉形态，不改变权限判断、确认流程、接口调用和成功/失败提示。

### 4.3 Search

文件边界：

- `web/src/components/Search/index.ts`
- `web/src/components/Search/src/index.vue`
- `web/src/components/Search/src/types.ts`
- 对应的 Vitest 测试文件

职责：

- 根据显式字段定义渲染输入框、选择框等筛选控件；
- 触发 `query` 和 `reset`；
- 支持旧项目已有的字段折叠能力；
- 不访问 API、不持有业务 loading、不处理权限。

字段类型必须使用明确的 TypeScript 联合类型。选择字段统一使用 `el-select-v2`，选项使用显式的 `{ label: string; value: string | number }` 结构。后端返回的未知数据必须在 API 或页面边界先完成类型收窄后再传给组件。

### 4.4 DIcon

文件边界：

- `web/src/components/DIcon/index.ts`
- `web/src/components/DIcon/src/index.vue`
- 对应的类型和测试文件

能力：

- 支持传入 Element Plus 图标组件；
- 支持传入 Iconify 图标名称；
- 两种图标来源互斥；
- 支持 `size`、`color` 和 class 透传；
- 使用显式联合 Props 和 Emits，不使用显式 `any`。

新增 `@iconify/vue` 依赖，仅用于 `DIcon` 和 `IconSelect`，不扩散到业务页面。

缺少图标来源或同时传入两种来源时，开发环境暴露明确错误；运行时显示明确空状态，不静默替换为未知图标。

### 4.5 IconSelect

文件边界：

- `web/src/components/IconSelect/index.ts`
- `web/src/components/IconSelect/src/index.vue`
- `web/src/views/system/menus/components/IconSelect.vue`（若当前菜单页面仍需要页面专用编排，则迁移为公共组件的薄封装）
- 对应的测试文件

行为：

- 支持关键字搜索；
- 使用网格展示图标预览；
- 点击图标后通过明确事件回传图标名称；
- 使用 `DIcon` 渲染预览，不重复实现图标分支；
- 没有匹配项时显示空状态；
- 菜单表单只接收字符串图标名称，不依赖 Iconify 的内部实现。

## 5. 页面迁移范围

迁移当前业务页面中的原生 `<el-select>`，包括：

- `web/src/views/system/users/index.vue`
- `web/src/views/system/sessions/index.vue`
- `web/src/views/system/menus/index.vue`
- `web/src/views/system/auth-platforms/index.vue`
- `web/src/views/system/roles/index.vue`
- `web/src/views/system/operation-logs/index.vue`
- `web/src/layout/components/SettingDrawer.vue`

同时迁移这些页面的表格刷新按钮和表格操作列按钮。迁移完成后，业务模板中不再保留原生 `<el-select>`，也不再为同一张 `AppTable` 单独渲染刷新按钮。

不修改后端 API、数据库、认证、RBAC、操作日志和会话管理行为；不在本切片新增通用请求 composable、列配置持久化、BaseTable、BaseDialog、Manager、Factory 或运行时注册表。

## 6. 错误与边界

- `AppTable` 刷新只触发事件，不吞掉页面查询错误；错误继续由现有请求层和页面错误状态展示。
- `Search` 对字段配置错误不做静默兜底，开发环境应暴露明确错误。
- `DIcon` 对非法或缺失图标显示明确空状态并在开发环境报错。
- `IconSelect` 对空结果显示空状态，回传值始终为明确字符串。
- 公共组件不返回假数据、不隐藏 API 错误、不接受显式 `any`。

## 7. 测试与验收

组件测试至少覆盖：

- `AppTable` 默认居中、列级对齐覆盖、刷新事件、刷新 loading、分页和选择事件；
- `Search` 输入字段、`el-select-v2` 字段、折叠、查询和重置；
- `DIcon` 的 Element Plus/Iconify 两种来源和非法 Props；
- `IconSelect` 搜索、空状态和选择回传；
- 公共组件及业务代码不出现 `any`、`as any`、`any[]` 或 `Record<string, any>`。

页面验收至少包括：

- 用户、角色、菜单、会话、认证平台、操作日志和设置页面的筛选仍能查询和重置；
- 表格刷新只由 `AppTable` 触发，行为与原页面一致；
- 表格操作按钮统一为旧项目约定的文字按钮；
- 菜单图标可以通过 `IconSelect` 选择并正确回显；
- 全量搜索确认不存在业务页面原生 `<el-select>`。

验证命令：

```powershell
cd D:\admin\web
pnpm vitest run
pnpm build
```

## 8. 后续技术债

本切片完成后，再根据真实重复度评估是否提取分页请求 composable、通用筛选布局和全局 UI token；不因未来可能出现的页面提前增加抽象。
