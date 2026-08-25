# 菜单 RBAC、动态页面协议与界面回收设计

## 1. 状态

状态：设计已确认，等待用户审阅书面版本。

## 2. 背景与纠偏

菜单管理必须是正规的数据库 RBAC，不能因为 `/system/menus` 需要稳定的 Vue 路由挂载点，
就把菜单管理页面和四个权限从数据库中拆出去。当前实现错误地扩大了“静态注册页面组件”
的职责：

- `AppAside` 手工插入了一级“菜单管理”；
- 迁移软删除了 `system:menu:list` 及其 action 节点和角色关系；
- access service 为超级管理员硬塞四个菜单权限；
- 前端把 `/system/menus` 当成数据库菜单的非法动态路径；
- 菜单没有独立 `name`，管理页和角色授权页使用 `t(i18nKey)` 冒充数据库名称；
- action 也被强制填写 `i18nKey`，混淆了业务名称和界面翻译键。

本设计恢复唯一事实来源：菜单、页面和权限节点都来自 `sys_menu`，角色授权关系来自
`sys_role_menu`。Vue 静态路由只解决页面组件的冷启动挂载时序，不产生菜单、不授予权限，
也不参与业务数据展示。

## 3. 目标

1. “系统管理 -> 菜单管理 -> 新增/修改/删除”是 `sys_menu` 中的真实树节点。
2. 普通角色只能通过 `sys_role_menu` 获得 `system:menu:*` 权限；超级管理员也只从当前有效
   数据库菜单集合推导全量权限，不再硬编码追加四个权限码。
3. 所有节点都有数据库名称 `name`；只有 directory/page 有 `i18nKey`。
4. 菜单管理表和角色授权表展示数据库 `name`，切换语言不改变业务数据。
5. 侧边栏、面包屑、RouteTabs 和 Header 使用 directory/page 的 `i18nKey` 翻译导航文案。
6. page 使用 `path + componentPath` 按固定规则加载页面，不恢复 `viewKey` 或页面注册表。
7. 菜单页沿用老项目紧凑、清楚的树表格与 Dialog 交互，并保持当前 Element Plus 主题能力。
8. 数据流继续保持显式、线性，不引入 Adapter、Manager、Factory、Base 层或运行时注册器。

## 4. 非目标

- 不实现语言包管理页面；管理员填写 key 后仍由开发者在前端语言包中维护翻译。
- 不允许数据库配置任意模块 URL 或执行任意 JavaScript。
- 不改变已经确认的角色叶子授权、祖先闭包和权限缓存算法。
- 不把菜单树改造成通用 CRUD；它仍是有明确父子约束的树状业务页面。
- 不新增自定义色彩系统、字体资源、卡片装饰或页面级动画。
- 不修改认证、会话、操作日志和认证平台业务。
- 不自动 commit、push、fetch 或 pull。

## 5. 数据模型

### 5.1 `sys_menu` 字段

`sys_menu` 最终字段为：

```text
id
parent_id
menu_type
name
code
i18n_key
path
component_path
icon
sort_order
is_enabled
is_hidden
created_at
updated_at
deleted_at
```

字段职责：

| 字段 | 规则 | 用途 |
| --- | --- | --- |
| `name` | 所有类型必填，去除首尾空格后 1-128 字符 | 菜单管理、角色授权等管理界面展示的真实业务名称 |
| `code` | 所有类型必填，活动数据唯一，创建后不可修改 | 节点稳定标识；page/action 同时是后端权限码 |
| `i18n_key` | directory/page 必填，action 必须为 `NULL` | 导航区域的翻译键，不是数据库名称 |
| `path` | 只有 page 必填 | 浏览器和 Vue Router 的绝对路由 |
| `component_path` | 只有 page 必填 | `web/src/views/<componentPath>/index.vue` 的相对页面路径 |
| `icon` | directory/page 可选，action 必须为 `NULL` | `DIcon` 使用的图标名称 |
| `is_hidden` | directory/page 可配置，action 固定为 `1` | 只控制侧边栏是否显示 |
| `is_enabled` | `0/1` | 控制节点及其授权是否有效 |

节点形状：

| `menu_type` | `parent_id` | `name` | `i18n_key` | `path` | `component_path` | `icon` | `is_hidden` |
| --- | --- | --- | --- | --- | --- | --- | --- |
| directory | 根为 `NULL`，否则父级必须是 directory | 必填 | 必填 | `NULL` | `NULL` | 可选 | 可配置 |
| page | 父级必须是 directory | 必填 | 必填 | 必填 | 必填 | 可选 | 可配置 |
| action | 父级必须是 page | 必填 | `NULL` | `NULL` | `NULL` | `NULL` | `1` |

数据库继续使用 `TIMESTAMPTZ`、`deleted_at` 软删除和 `0 = No / 1 = Yes`。新增 `name`
非空约束，并把 `i18n_key` 从全表非空改为按节点类型检查；Service 和 PostgreSQL 两层都
拒绝不符合形状的数据。

### 5.2 名称和翻译键规范

`name` 是人直接输入和管理的业务内容，例如：

```text
系统管理
菜单管理
新增菜单
修改菜单
删除菜单
```

`i18nKey` 只用于 directory/page，例如：

```text
navigation.system
navigation.systemMenus
```

格式为至少两段点号分隔的 lower camel case：

```text
^[a-z][a-z0-9]*(?:\.[a-z][a-zA-Z0-9]*)+$
```

action 不填写、不接收、不返回有效字符串，后端归一化后必须为 `NULL`。不得用 action
名称、权限码或空字符串替代 `NULL`。

### 5.3 路由和页面路径规范

page 同时填写两个不同字段：

```text
path:          /system/users
componentPath: system/users
页面文件:      web/src/views/system/users/index.vue
```

- `path` 必须以 `/` 开头；
- `componentPath` 不能以 `/` 开头，也不包含扩展名；
- 两者都使用小写 kebab-case 路径段；
- 不允许 query、hash、动态参数、`.`、`..`、空段或末尾斜杠；
- 不从其中一个字段静默生成或修正另一个字段；
- 页面文件不存在时，整次动态路由安装明确失败，不加载空页面或 Dashboard 兜底。

### 5.4 基础 RBAC 节点

以下五个节点是菜单系统能够继续管理自身的基础节点，但它们仍是真实数据库数据：

| `code` | 类型 | `name` | 父节点 | `path` | `componentPath` |
| --- | --- | --- | --- | --- | --- |
| `system` | directory | 系统管理 | `NULL` | `NULL` | `NULL` |
| `system:menu:list` | page | 菜单管理 | `system` | `/system/menus` | `system/menus` |
| `system:menu:create` | action | 新增菜单 | `system:menu:list` | `NULL` | `NULL` |
| `system:menu:update` | action | 修改菜单 | `system:menu:list` | `NULL` | `NULL` |
| `system:menu:delete` | action | 删除菜单 | `system:menu:list` | `NULL` | `NULL` |

这是“受保护的数据库节点”，不是静态菜单。后端通过明确的基础 code 集合保护它们：

- 禁止禁用和删除；
- 禁止修改 `code/menuType/parentId/path/componentPath/isEnabled/isHidden`；
- directory/page 允许修改 `name/i18nKey/icon/sortOrder`；
- action 允许修改 `name/sortOrder`；
- 管理接口返回计算字段 `isProtected`，前端据此禁用对应控件和命令并显示原因；
- 后端始终是保护规则的最终执行者，不能依赖前端禁用状态。

角色、用户、认证平台、会话和操作日志等其他页面仍是普通数据库菜单，可以按现有树规则
调整或授权；后续新增菜单也不进入基础保护集合。

## 6. 数据迁移

结构迁移必须可重复、可审计并在同一 PostgreSQL 事务中完成：

1. 新增可空 `name VARCHAR(128)`；
2. 使用明确的当前系统菜单 code -> 中文名称映射回填已有节点；
3. 对无法映射的历史节点列出 code 并让启动失败，不使用 code 或 i18nKey 静默充当名称；
4. 将所有 action 的 `i18n_key` 设为 `NULL`，再移除该列的全表 `NOT NULL`；
5. 增加 `name` 非空/非空白约束和新的按类型形状约束；
6. 保留已经完成的 `view_key -> component_path` 迁移和 `view_key` 删除结果；
7. 删除旧迁移中软删除菜单管理子树的行为；
8. 不再次重置用户已经配置的 directory/page `is_hidden`。

结构迁移完成、Redis access store 和 invalidator 建立后，再由 `menuService.EnsureFoundation`
执行基础数据恢复。该操作必须复用菜单写入的 access invalidating lease：

1. 锁定菜单、用户和 access version；
2. 恢复被错误迁移软删除的 `system:menu:list` 子树；
3. 只恢复与对应菜单错误删除时间相同的 `sys_role_menu` 关系，不复活更早的历史关系；
4. 如果基础节点完全不存在，则按第 5.4 节创建；
5. 如果同 code 存在重复历史或结构冲突，回滚并让 API 启动失败；
6. PostgreSQL 提交后发布新的 Redis generation，发布失败则 API 不启动。

如果基础节点已经正确，`EnsureFoundation` 只校验且不写数据库、不推进 version。它不得重建
其他业务菜单，也不得覆盖允许管理员修改的展示字段。这样可以避免 PostgreSQL 已恢复、
Redis 旧快照却仍可命中的状态分裂。

## 7. 后端线性数据流

### 7.1 菜单管理

```text
router -> auth/access middleware -> menu handler -> menu service
       -> menu repository -> sys_menu/sys_role_menu -> PostgreSQL
```

- Handler 只绑定 DTO、传递 request context 和输出 `code/data/message`；
- Service 校验名称、节点形状、基础保护、树关系和缓存失效顺序；
- Repository 只执行 PostgreSQL 查询、事务、锁和软删除；
- 菜单变更沿用当前 access invalidating lease，事务提交后发布新 generation；
- Redis 失效失败必须显式返回依赖错误，不能跳过保护流程或返回假成功。

### 7.2 当前用户访问快照

```text
GET /api/v1/access
-> auth middleware
-> access handler
-> access service
-> access repository
-> sys_user_role + sys_role_menu + sys_menu + sys_access_version
-> PostgreSQL
-> Redis snapshot cache
-> response
```

普通角色从 `sys_role_menu` 的直接授权出发，按现有规则补齐祖先闭包。page/action 的 code
进入 `permissionCodes`，directory 只负责树结构。超级管理员继续拥有所有有效 page/action，
但权限也必须从有效 `sys_menu` 行推导；删除当前硬编码追加四个 `system:menu:*` 的逻辑。

因此，数据库没有 `system:menu:list` 时，任何人都不应凭空获得该权限；基础节点校验会在
API 启动阶段更早明确失败。

## 8. 前端路由与菜单

### 8.1 静态页面挂载边界

Vue Router 静态注册：

```text
/login
/register
/dashboard
/system/menus
```

`/system/menus` 静态 route record 只提供 `system/menus/index.vue` 的稳定组件挂载点，并在
meta 中声明 `requiredPermission = system:menu:list`。它不负责：

- 向侧边栏插入菜单；
- 向 access store 插入权限；
- 生成面包屑或 Tab 标题；
- 绕过 `/api/v1/access`；
- 让超级管理员在数据库缺失节点时继续访问。

动态路由安装遇到数据库节点 `system:menu:list + /system/menus + system/menus` 时，必须校验
它与静态 route record 完全一致，然后复用该 route record，不重复 `addRoute`。任何其他节点
占用 `/system/menus` 或使用不一致的 componentPath 都是协议错误。

### 8.2 菜单和导航渲染

- `AppAside` 删除手工“菜单管理”项；Dashboard 保持产品明确要求的静态定位入口；
- `AccessMenuNode` 只递归渲染 `/api/v1/access.menuTree`；
- “菜单管理”自然显示在数据库“系统管理”目录下；
- `isHidden = 1` 只过滤侧边栏，不改变 access 快照、动态路由、面包屑或直接访问；
- 面包屑不再为 `/system/menus` 写特例，而是从完整 access 菜单树查找；
- RouteTabs、Header 和侧边栏对 directory/page 使用 `t(i18nKey)`；
- 缺少语言包 key 时明确显示 key，不使用 `name/code/path` 兜底。

## 9. 前端 DTO 与展示来源

### 9.1 菜单管理 DTO

`ManagedMenuNode` 增加：

```ts
name: string
i18nKey: string | null
isProtected: YesNo
```

directory/page 的 `i18nKey` 必须是合法字符串；action 必须为 `null`。Create/Update DTO 使用
相同可空规则，禁止继续用空字符串兼容 action。

### 9.2 access DTO

access 菜单树只包含 directory/page，所以继续使用：

```ts
i18nKey: string
```

不需要把 `name` 加入 access 快照；导航只使用翻译键。这样 Redis 快照不保存未使用字段。

### 9.3 角色授权 DTO

角色授权树使用数据库 `name` 作为节点标签，同时显示稳定 `code`。它不再调用
`t(node.i18nKey)` 显示权限定义。切换中英文只翻译页面控件和列名，不翻译数据库菜单名称。

展示来源固定如下：

| 位置 | 显示字段 |
| --- | --- |
| 菜单管理树表格 | `name` |
| 父节点选择器 | `name + code` |
| 角色授权树/差异确认 | `name + code` |
| 侧边栏 | `t(i18nKey)` |
| Header 面包屑 | `t(i18nKey)` |
| RouteTabs | `t(i18nKey)` |

## 10. 菜单管理界面设计

### 10.1 设计方向

对象是中国开发者高频使用的 Admin RBAC 工作台，页面唯一任务是快速扫描、创建和维护
“目录 -> 页面 -> 权限”树。整体保持紧凑、安静、工具化，不做营销式标题、嵌套卡片或
装饰背景。

实现继续使用 Element Plus 和现有主题变量。默认视觉参考为：主色 `#409EFF`、页面底色
`#F5F7FA`、表面 `#FFFFFF`、正文 `#303133`、边框 `#DCDFE6`、成功色 `#67C23A`；这些
只是默认主题语义，代码必须使用 `--el-*`/现有 `--admin-*` 变量，主题切换后随变量变化。

字体沿用现有中文友好系统字体；权限码、路由和页面路径使用等宽字体栈提高扫描效率，
不引入外部字体。页面的识别点不是装饰，而是清楚的 directory/page/action 类型标签、
树缩进和真实数据库名称。

### 10.2 页面结构

```text
+--------------------------------------------------------------------+
| [新增根目录]                         [全部展开/收起] [刷新]          |
+--------------------------------------------------------------------+
| 名称 | 类型 | 权限码 | 路由 | 页面路径 | 图标 | 排序 | 状态 | 操作 |
| 系统管理                                                           |
|   菜单管理                                                         |
|     新增菜单                                                       |
+--------------------------------------------------------------------+
```

- 不增加重复的“菜单管理”页面标题；布局导航已经说明当前位置；
- 表格使用 Element Plus `border` 和 `header-cell-style` 属性，不为表头重写 CSS；
- 表头和所有普通列默认 `align="center"`、`header-align="center"`；
- 名称列保留树层级缩进，内容仍在单元格内居中；
- 操作列固定右侧并给足宽度，不能再次遮住“编辑”等命令；
- 初次异步加载完成后真实展开全部节点；提供“全部展开/收起”命令；
- 展开实现使用 table ref 递归 `toggleRowExpansion`，监听数据变化，不只依赖
  `default-expand-all` 的初始渲染时机；
- 刷新保留明确按钮和 loading，不增加自动轮询；
- 空态、加载失败、修改失败和重试入口都明确可见。

### 10.3 Dialog 表单

继续使用现有 `AppDialog`，桌面宽度约 800-900px，移动端由公共组件收窄。表单桌面双列、
移动端单列，输入类控件占满可用宽度。

字段按类型显示：

| 类型 | 表单字段 |
| --- | --- |
| directory | 父节点、类型、目录名称、i18nKey、权限码、图标、排序、启用、是否隐藏 |
| page | 父节点、类型、页面名称、i18nKey、权限码、路由、页面路径、图标、排序、启用、是否隐藏 |
| action | 父页面、类型、权限名称、权限码、排序、启用 |

具体交互：

- 名称标签随类型显示“目录名称 / 页面名称 / 权限名称”，字段都写入 `name`；
- action 不渲染 i18nKey、路由、页面路径、图标和是否隐藏控件；
- i18nKey、权限码、路由、页面路径使用全宽输入，并在输入框下显示真实格式提示；
- 父节点和类型使用 `el-select-v2`；图标使用已有 `DIcon + IconSelect`；
- 二元状态使用 Switch/Radio，不用文本下拉；
- 编辑基础节点时锁定结构字段并显示受保护原因，不把控件静默消失；
- `code` 对所有节点创建时可填，编辑时只读；
- 类型切换立即清空不兼容字段，action 的 `i18nKey` 提交为 `null`、`isHidden` 提交为 `1`；
- 不设置固定 Dialog 高度；内容超出视口时由 `AppDialog` 的独立滚动区处理。

帮助文本精确表达协议，例如：

```text
i18nKey：至少两段点号路径，例如 navigation.systemUsers
权限码：小写冒号分段，例如 system:user:list
路由：必须以 / 开头，例如 /system/users
页面路径：不能以 / 开头，页面文件为 web/src/views/<页面路径>/index.vue
```

### 10.4 样式边界

- 优先使用 Element Plus 属性和现有公共组件，不复制老项目的业务代码和旧枚举；
- CSS 只负责网格、间距、全宽控件、帮助文本和必要的响应式布局；
- 不硬编码主题色，不覆盖 `el-select-v2` 官方内部样式；
- 不新增卡片嵌套、渐变、圆形装饰、超大标题或无业务含义动画；
- 键盘焦点继续使用 Element Plus 可见焦点，不移除 outline；
- 任何动态内容不能改变工具栏、表格列或 Dialog 的稳定尺寸。

## 11. 错误处理

- 基础节点缺失、结构冲突或恢复失败：API 启动失败并记录具体 code/cause；
- 非法名称、i18nKey、权限码、路由或页面路径：返回明确的业务错误；
- 普通用户无 `system:menu:list`：后端接口返回 403，前端直接访问路由跳 Dashboard；
- access 快照缺少菜单管理节点：不插入假节点，不授予假权限；
- 页面文件缺失或静态绑定不一致：access 路由安装进入明确错误状态；
- 菜单写入或 Redis generation 发布失败：操作不返回假成功；
- 前端协议字段缺失或类型错误：抛 `ProtocolError`，不使用可选链或默认值继续运行。

## 12. 测试与验收

### 12.1 后端

测试必须覆盖：

1. `name` 和可空 `i18n_key` 的 PostgreSQL 约束；
2. 三种节点 DTO 的字段矩阵；
3. action 的 i18nKey 只能为 `NULL`；
4. 基础节点恢复、重复执行、冲突失败和展示字段可编辑；
5. 基础节点结构修改、禁用和删除被后端拒绝；
6. 错误迁移删除的角色关系只恢复本次关联记录；
7. 普通角色只从 `sys_role_menu` 获得菜单管理权限；
8. 超级管理员权限来自有效数据库菜单，不再硬编码追加；
9. 菜单变更继续正确推进 access version 和 Redis generation。

### 12.2 前端

测试必须覆盖：

1. `AppAside` 没有手工菜单管理项，数据库树把它渲染在系统管理下；
2. `/system/menus` 静态组件绑定与数据库节点一致时复用，不一致时明确失败；
3. 面包屑和 Tab 从 access 树生成，不存在菜单管理特例；
4. 菜单管理和角色授权显示 `name`，语言切换不改变业务名称；
5. directory/page 显示 i18nKey，action 不显示且提交 `null`；
6. 名称标签按节点类型变化，输入框和帮助文本完整；
7. `DIcon + IconSelect` 正常选择、清除和回显；
8. 表格 border、浅色表头、全列居中、固定操作列和异步默认展开真实生效；
9. 受保护节点的前端命令状态与后端错误一致；
10. 不引入显式 `any`、`as any`、`any[]`、`Record<string, any>` 或 `@ts-ignore`。

### 12.3 完整验证

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...

cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

浏览器最终分别使用超级管理员和普通角色验收：

- 菜单管理位于系统管理目录下；
- 普通角色未授权时看不到且不能访问，授权 list 后可见可访问；
- create/update/delete 按各自 action 权限独立控制；
- 切换语言只改变导航和系统控件，菜单管理/角色授权中的数据库名称不变；
- 隐藏菜单不出现在侧边栏，但拥有权限时可直接访问；
- 修改角色或菜单后，操作者收到刷新提示，目标用户下次请求使用新 generation。

## 13. 完成标准

- `sys_menu` 有真实 `name`，action 的 `i18n_key` 为 `NULL`；
- 菜单管理基础节点和角色关系存在于 PostgreSQL；
- 后端不再硬塞 `system:menu:*` 权限；
- 前端不再手工插入菜单管理，也不为其写面包屑特例；
- `/system/menus` 只静态绑定组件，显示和权限都来自 access 数据；
- 菜单管理和角色授权显示数据库名称，导航显示 i18n 翻译；
- 表格和 Dialog 达到第 10 节的紧凑工具化基线；
- 后端与前端完整验证全部通过，未自动创建 Git 提交。
