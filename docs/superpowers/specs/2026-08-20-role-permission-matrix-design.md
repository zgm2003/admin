# Role Permission Matrix Design

## 1. 目标

将角色授权 Dialog 中的权限树替换为权限矩阵，使管理员可以同时看清页面访问权限和页面内操作权限，并在保存前确认新增、移除的权限。

交互和布局以旧项目以下实现为直接参考：

- `D:/github-project/admin_front_ts/src/views/Main/permission/role/components/RolePermissionMatrix.vue`；
- `D:/github-project/admin_front_ts/src/views/Main/permission/role/components/RolePermissionMatrix.styles.css`；
- `D:/github-project/admin_front_ts/src/views/Main/permission/role/components/RolePermissionDiffDialog.vue`；
- `D:/github-project/admin_front_ts/src/views/Main/permission/role/role-matrix.ts`。

本项目只做当前 RBAC 契约要求的必要适配，不引入平台 Tabs，不改变后端接口、数据库结构、直接授权规则或运行时权限推导。

## 2. 线性数据流

授权读取和编辑：

```text
GET /api/v1/roles/:id/permissions
-> web/src/api/role.ts 严格解析
-> 角色页面构建权限矩阵
-> 最小直接授权展开为有效展示权限
-> 管理员调整页面和操作权限
```

授权保存：

```text
当前有效展示权限
-> 与打开 Dialog 时的原有效展示权限比较
-> 权限差异确认 Dialog
-> 规范化为最小直接授权 menuIds
-> PUT /api/v1/roles/:id/permissions
-> 成功后关闭 Dialog 并刷新角色列表
```

页面不直接调用 Axios，不新增 Store、Adapter、Manager 或通用权限服务。

## 3. 本期范围

本期包含：

- directory 分组下的页面/操作权限矩阵；
- 全局全选和清空；
- 分组复选框、全选、清空、收起和展开；
- 分组的选中数量和半选状态；
- page 与 action 的联动选择；
- 禁用菜单的明确展示；
- 保存前新增/移除权限差异确认；
- 保存失败时保留选择并展示明确错误；
- 中英文前端文案；
- 矩阵纯函数测试和角色页面交互测试。

## 4. 非目标和硬约束

本期不包含：

- Cascader 或 Tree 两套并存；
- 多平台权限 Tabs；
- 修改角色授权 API 请求或响应结构；
- 修改 `sys_role_menu` 或其他数据库结构；
- 修改 access 权限推导或 `super_admin` 固定全权限规则；
- 将权限矩阵做成全局通用组件；
- 重构角色列表、查询、资料 Dialog 或分页；
- 修改后端及后端国际化文案。

业务 TypeScript 不得使用显式 `any`、宽泛断言、`@ts-ignore` 或协议兜底。不得为了视觉全选而向后端提交 directory 或冗余 page 直接授权。

## 5. 继承的 RBAC 契约

本设计继承现有角色管理规则：

- directory 只组织权限，不写入 `sys_role_menu`；
- 直接授予 page，只获得页面基础访问，不获得 action；
- 直接授予 action，运行时自动获得所属 page 和 directory 祖先；
- page 和其 action 同时存在时，保存前移除冗余 page 直接授权；
- 没有 action 的 page 仍可独立授权；
- 禁用 page/action 可以保留和调整授权，但运行时暂不生效；
- 后端继续验证并规范化提交 ID，不信任前端算法。

旧项目的选择数组同时保留 page 和 action。本项目在 UI 中使用这种有效展示集合，在保存边界转换成当前后端要求的最小直接授权集合。这是展示阶段和持久化阶段的区别，不是两套 RBAC 算法。

## 6. 文件职责

角色授权按旧项目边界拆成局部文件：

| 文件 | 职责 |
| --- | --- |
| `web/src/views/system/roles/role-permission-matrix.ts` | 菜单树到矩阵的转换、有效权限展开、选择联动、统计、差异和最小授权规范化纯函数 |
| `web/src/views/system/roles/components/RolePermissionMatrix.vue` | 权限矩阵展示和用户选择事件 |
| `web/src/views/system/roles/components/RolePermissionMatrix.css` | 权限矩阵局部样式 |
| `web/src/views/system/roles/components/RolePermissionDiffDialog.vue` | 保存前新增/移除权限确认 |
| `web/src/views/system/roles/index.vue` | API 调用、Dialog 状态、错误状态和保存编排 |
| `web/src/views/system/roles/role-permission-matrix.test.ts` | 纯函数规则测试 |
| `web/src/views/system/roles/index.test.ts` | 页面加载、选择、确认和保存集成测试 |

不得创建 composable、全局组件或新的 API DTO。局部组件只通过明确 Props、Emits 和 `defineModel<number[]>` 通信。

## 7. 权限矩阵结构

接口返回的合法菜单树固定为 directory -> page -> action。矩阵转换为：

```text
directory 分组
└─ page 行
   ├─ page 访问复选框
   └─ action 复选框列表
```

示例：

```text
系统管理                       权限 8/8  页面 2/2  操作 6/6
┌──────────────────┬────────────────────────────────────┐
│ 页面权限          │ 操作权限                            │
├──────────────────┼────────────────────────────────────┤
│ [x] 菜单管理      │ [x] 新增 [x] 修改 [x] 删除          │
│ [x] 角色管理      │ [x] 新增 [x] 修改 [x] 授权          │
└──────────────────┴────────────────────────────────────┘
```

规则：

- 每个根 directory 对应一个分组；
- 嵌套 directory 不创建新的视觉分组，其后代 page 继续归入当前最外层 directory 分组；
- 每个 page 对应一行，page 本身就是左侧页面访问权限；
- page 的 action 在右侧平铺；
- 没有 action 的 page 仍显示页面访问复选框，并显示“无操作权限”；
- 当前协议不允许根 page 和根 action，因此不复制旧项目的合成根节点分组；本项目也没有多平台权限，因此不复制平台分支；
- 分组顺序、页面顺序和 action 顺序完全沿用接口树顺序；
- 节点标签使用现有 `i18nKey` 翻译并同时展示 permission code；
- `isEnabled=0` 的 page/action 继续可选，并显示“已禁用”标签。

## 8. 展示选择与直接授权

### 8.1 加载回显

接口 `menuIds` 是最小直接授权集合。页面构造原有效展示权限：

1. 加入所有直接授权 page/action ID；
2. 对每个直接授权 action，加入其所属 page ID；
3. 去重并按数字升序保存为 `originalEffectiveMenuIDs`；
4. 复制为矩阵当前选择 `selectedEffectiveMenuIDs`。

因此，数据库只保存 action 时，矩阵仍会同时勾选 action 和所属 page。全权限角色重新打开时，页面、action、分组和全局状态都正确显示为全选。

### 8.2 用户调整

- 选择 page：只加入 page ID，不自动加入 action；
- 取消 page：同时移除该 page 及其全部 action；
- 选择 action：同时加入该 action 和所属 page；
- 取消 action：只移除该 action，保留 page；
- 选择分组：加入分组内全部 page/action；
- 清空分组：移除分组内全部 page/action；
- 全局全选：加入全部 page/action；
- 全局清空：清空全部 page/action。

当前选择始终是一个合法的有效权限集合，不允许出现选中 action 但未选中所属 page。

### 8.3 统计状态

全局工具栏提供“全选”和“清空”按钮，行为与旧项目当前平台的全选和清空一致；本项目没有平台维度，因此作用范围是完整权限矩阵。

每个 directory 分组显示：

- 权限：已选 page/action 数 / page/action 总数；
- 页面：已选 page 数 / page 总数；
- 操作：已选 action 数 / action 总数；
- 分组复选框的全选、半选和未选状态；
- 收起/展开、全选和清空命令。

统计包含禁用但未软删除的节点，与授权查询返回范围一致。

## 9. 权限差异确认

点击主授权 Dialog 的“保存授权”时，不立即发送请求：

1. 比较 `originalEffectiveMenuIDs` 和 `selectedEffectiveMenuIDs`；
2. 没有变化时不发送 PUT，关闭主 Dialog；
3. 存在变化时打开局部 `RolePermissionDiffDialog`；
4. 新增权限使用 success `el-tag`，移除权限使用 danger `el-tag`；
5. 两个区域没有对应变化时分别显示空状态；
6. 标签使用菜单翻译名称和 permission code，ID 必须存在于当前严格解析的菜单树，不使用 `#ID` 兜底；
7. 管理员取消时只关闭差异 Dialog，矩阵选择保持不变；
8. 管理员确认后才执行保存。

差异确认 Dialog 复刻旧项目 560px 双区域布局，但使用本项目现有 `el-dialog`，不提前引入全局 `AppDialog` 技术债回收。

## 10. 保存规范化和错误处理

确认保存时，将当前有效展示权限转换为最小直接授权：

1. 只保留合法 page/action ID；
2. 若 page 下至少选择一个 action，移除该 page ID；
3. 没有已选 action 的 page 保留 page ID；
4. 去重并按数字升序提交 `{ menuIds }`。

保存期间禁用重复确认并显示 loading。保存成功后：

- 关闭差异 Dialog；
- 关闭主授权 Dialog；
- 保持当前角色列表筛选和分页并重新查询；
- 显示现有授权成功通知。

保存失败时：

- 保持两个 Dialog 和当前选择；
- 在差异确认 Dialog 中显示明确错误；
- 解除 loading，允许再次确认；
- 不关闭主 Dialog，不清空选择，不伪造成功。

授权初次加载失败和重试继续使用现有主 Dialog 行为。

## 11. 视觉和滚动约束

主授权 Dialog 延续旧项目权限编辑器布局：

- 桌面宽度调整为约 1040px，移动端不超过 `94vw`；
- 顶部工具栏放置全选和清空；
- 每个分组使用 8px 圆角边框和 Element Plus 主题变量；
- 分组标题栏使用 `--el-fill-color-lighter`；
- 页面和操作使用带边框的 `el-table` 两列布局；
- action 使用可换行的 `el-space`，不挤压页面列；
- 默认展开所有分组，管理员可以逐组收起；
- Dialog 内容区域内部滚动，不产生全局滚动条；
- 不新增自定义调色板、装饰卡片或说明性功能文案。

移动端允许矩阵内容区横向滚动以保留两列语义，页面不得整体横向滚动。

## 12. 国际化

新增文案同时加入 `zh-CN` 和 `en-US`，至少覆盖：

- 页面权限、操作权限；
- 全部权限、全选、清空；
- 权限/页面/操作统计；
- 无操作权限；
- 展开、收起；
- 新增权限、移除权限；
- 权限差异确认标题；

页面名称和 action 名称继续使用后端返回的已注册 `i18nKey`，不在矩阵中硬编码业务名称。

## 13. 测试要求

纯函数测试至少覆盖：

- directory/page/action 转换为稳定矩阵；
- 直接 page 展开为 page 有效权限；
- 直接 action 展开为 action 加所属 page；
- 选择 action 自动加入 page；
- 取消 page 同时移除 action；
- 取消 action 保留 page；
- 分组和全局全选、清空；
- 分组全选、半选和未选统计；
- 新增/移除权限差异；
- page/action 有效集合规范化为最小直接授权；
- 禁用节点仍参与矩阵、统计和选择。

页面测试至少覆盖：

- 打开授权 Dialog 后正确回显全部有效权限；
- 页面列、操作列、分组统计和禁用标签正确展示；
- 全局和分组选择命令更新矩阵；
- 点击保存先打开差异确认框，不提前调用 PUT；
- 差异确认框正确展示新增和移除权限；
- 取消差异确认保留矩阵选择；
- 确认后只提交最小直接授权 ID；
- 无变化时不发送 PUT；
- 空选择确认后提交空数组；
- 保存失败保留 Dialog、选择和错误；
- 保存成功关闭 Dialog 并刷新当前角色列表。

行为变更必须遵循失败测试 -> 最小实现 -> 通过测试。完成后运行角色管理定向测试、前端全量测试、生产构建、forbidden-any 检查和 `git diff --check`。

## 14. 验收标准

1. 角色授权使用与旧项目一致的页面/操作权限矩阵；
2. 支持全局和分组全选、清空、半选、统计及收起；
3. page/action 联动始终生成合法有效权限集合；
4. 最小直接授权在重新打开时正确显示为有效全选状态；
5. 保存前明确展示新增和移除权限并要求二次确认；
6. 保存请求仍是排序后的最小直接授权 `menuIds`；
7. 只授予 page 的能力得到保留；
8. 禁用权限继续可见、可保留和可调整；
9. 保存失败不丢失选择且错误明确；
10. 后端、数据库和 API 契约没有变化；
11. 中英文文案完整，业务 TypeScript 无 forbidden any；
12. 相关测试、前端全量测试和生产构建通过。
