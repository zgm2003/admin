# 前端 AppDialog 与 AppTable 基础组件设计

## 1. 状态

状态：设计已确认，等待用户审阅。

## 2. 背景

当前 `web/src/views/system` 下的用户、角色、认证平台、操作日志和会话页面都直接使用 Element Plus 的 `el-table`、`el-pagination` 和 `el-dialog`。这些页面已经出现稳定的重复行为：

- 表格加载、空数据、错误状态和内部滚动处理方式重复；
- 分页布局、移动端分页密度和表格底部布局重复；
- 弹窗宽度、销毁策略、内容区滚动、header/footer 插槽和关闭行为重复；
- 页面自身同时承担组件布局和业务请求，后续 UI 调整改动面大。

旧项目 `D:\github-project\admin_front_ts` 已有成熟的 `AppDialog` 和泛型 `AppTable`，本切片吸收其经过验证的边界，但按当前项目的目录、i18n、Element Plus 版本和 lower camel case 契约重新实现，不直接复制旧项目的请求层、状态管理或业务模块。

## 3. 目标

1. 建立严格 TypeScript 的 `AppDialog`，统一系统表单/详情弹窗的结构和响应式行为。
2. 建立泛型 `AppTable<Row>`，统一真实页面的表格列、分页、选择、加载、空态和错误态展示。
3. 迁移五个已有页面，验证公共组件确实消除了重复而没有吞掉业务逻辑：
   - `system/users`
   - `system/roles`
   - `system/auth-platforms`
   - `system/operation-logs`
   - `system/sessions`
4. 保持页面请求流：

   ```text
   view -> api/<module>.ts -> utils/request.ts -> Go API
   ```

   公共组件不发请求、不读权限 Store、不处理业务错误码。

## 4. 设计决策

### 4.1 AppDialog

文件边界：

- `web/src/components/AppDialog/index.ts`
- `web/src/components/AppDialog/src/index.vue`
- `web/src/components/AppDialog/src/dialog.ts`
- 对应的 Vitest 测试文件

核心 props：

- `modelValue: boolean`
- `title?: string`
- `ariaLabel?: string`
- `description?: string`
- `width?: string | number`
- `mobileWidth?: string | number`
- `height?: string | number`
- `bodyPadding?: string | number`
- `showHeader?: boolean`
- `appendToBody?: boolean`
- `destroyOnClose?: boolean`
- `draggable?: boolean`
- `showClose?: boolean`
- `alignCenter?: boolean`
- `closeOnPressEscape?: boolean`

插槽只提供结构能力：`header`、默认内容和 `footer`。组件不拥有提交、取消、loading、权限或业务错误状态。

行为约束：

- 桌面端和移动端使用不同默认宽度；
- 指定 `height` 后只让内容区滚动，禁止弹窗整体产生横向滚动；
- 默认销毁关闭内容，页面可以按需要显式关闭；
- 关闭后恢复打开前的触发元素焦点；
- `ariaLabel` 和 `description` 用于无可见标题或复杂内容的可访问性；
- 其余 Element Plus 属性通过受控 attrs 透传，但不允许业务代码依赖隐藏的 `fullscreen` 兼容行为。

### 4.2 AppTable

文件边界：

- `web/src/components/AppTable/index.ts`
- `web/src/components/AppTable/src/index.vue`
- `web/src/components/AppTable/src/types.ts`
- 对应的 Vitest 测试文件

组件使用 Vue 泛型 SFC，业务行类型由调用方传入，禁止显式 `any`。列定义支持：

- 使用 `prop` 的行字段列；
- 只有 `key` 的派生列；
- `label`、`width`、`minWidth`、`fixed`、`hidden`、溢出提示；
- 当前页面需要的 Element Plus 列属性；
- `cell-<key>` 命名插槽覆盖默认值格式化。

核心 props：

- `columns: TableColumn<Row>[]`
- `data: Row[]`
- `loading?: boolean`
- `rowKey?: string`
- `selectable?: boolean`
- `selectionSelectable?: (row: Row, index: number) => boolean`
- `pagination?: TablePaginationState | null`
- `resultState?: 'idle' | 'loading' | 'refreshing' | 'success' | 'empty' | 'error'`
- `statusMessage?: string`
- `ariaLabel?: string`

事件只描述表格交互：

- `refresh`
- `row-click`
- `selection-change`
- `update:pagination`
- `column-change`

工具栏通过 `toolbar-left`、`toolbar-right` 插槽提供。刷新按钮、权限按钮、搜索条件和 API 调用由页面自行决定；公共表格不创建业务操作。

### 4.3 状态和滚动

- 表格组件只负责表面状态和表格内部滚动，不改变现有页面的请求状态机；
- 页面主滚动仍由 `admin-layout__main` 持有；
- `fixedFooter` 场景下表格占据可用高度，分页固定在表格底部；
- 移动端分页只显示必要控件，桌面端保留完整分页控件；
- 空态和错误态通过插槽或 `statusMessage` 定制，不返回假数据。

## 5. 首批迁移

迁移顺序固定为：

1. `system/roles`：验证普通表格、表单弹窗和权限弹窗；
2. `system/users`：验证选择、角色弹窗和分页筛选；
3. `system/auth-platforms`：验证表单字段较多的弹窗和状态操作；
4. `system/operation-logs`：验证展开行、只读表格和错误/空态；
5. `system/sessions`：验证选择、批量操作和会话分页。

`RolePermissionMatrix` 保持专用组件，因为它是权限树/矩阵，不是通用 CRUD 表格。

## 6. 非目标

- 不实现 `usePagedList` 或通用请求 composable；
- 不把搜索表单、权限判断、确认框、业务按钮放进 `AppTable`；
- 不做列配置服务端持久化；
- 不引入 `BaseTable`、`BaseDialog`、Manager、Factory 或运行时组件注册表；
- 不修改后端 API、数据库、RBAC、认证和操作日志行为；
- 不在本切片同时重做所有页面视觉样式，统一 UI 基线另行验收。

## 7. 测试与验收

组件测试至少覆盖：

- AppDialog 的 v-model、默认/命名插槽、移动宽度、内容滚动和关闭焦点恢复；
- AppTable 的泛型列渲染、派生列、隐藏列、loading、empty、error、分页和 selection；
- 组件不产生显式 `any`、`as any` 或 `Record<string, any>`。

页面迁移测试保持原有业务断言，并增加：

- 页面使用公共组件而不是直接重复的 `el-table`/`el-dialog`；
- 分页和弹窗业务行为不改变；
- 业务 API 请求仍由页面 API 模块触发。

验证命令：

```powershell
cd D:\admin\web
pnpm vitest run
pnpm build
```

## 8. 后续技术债

本切片完成后再评估：

- `usePagedList`：至少两个页面仍有完整重复的加载、筛选、分页和错误状态后再提取；
- 通用筛选区：确认用户、角色和认证平台的筛选字段结构真实稳定后再提取；
- 全局 UI token：基于迁移后的实际页面统一间距、密度和响应式细节。
