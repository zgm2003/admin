# 菜单协议与动态页面映射设计

## 1. 状态

状态：设计已确认，等待用户审阅。

## 2. 背景

当前菜单协议同时维护 i18nKey、viewKey 和图标注册表：

- 菜单标题由 menuTitleKeys 限制为前端预先登记的有限集合；
- 页面组件由 routeViews 手动登记，再由数据库保存 viewKey；
- 菜单页面又维护一份 menuIcons，与已经存在的 DIcon、IconSelect 重复；
- 后端还复制了标题、页面和图标白名单。

这造成同一个菜单页面需要多处同步修改，新增业务页面无法沿着一条明确规则完成。
本设计把菜单数据、i18n 文案和页面文件按稳定命名规则连接起来，保留线性数据流，
删除重复注册表和无业务价值的 viewKey。

## 3. 目标

1. 管理员可以直接填写 i18nKey，并在语言包中自行维护对应文案。
2. 删除 viewKey 选择器，改为填写路由和页面路径，页面组件按页面路径加载。
3. 菜单图标只保存名称字符串，显示和选择统一使用已有的 DIcon、IconSelect。
4. 菜单表单输入控件占满所在列，并为 i18nKey、code、路由、页面路径等输入提供格式提示。
5. 保持后端和前端数据流显式、线性，不引入通用注册器、Adapter、Factory 或 Base 层。

## 4. 非目标和硬约束

- 不实现用户自定义语言包管理页面；语言包仍由前端代码维护。
- 不允许后端下发任意 JavaScript 模块地址并直接执行。
- 不保留 viewKey 兼容字段，不同时接受新旧字段；页面字段统一使用 componentPath。
- 不为页面组件增加运行时注册中心；import.meta.glob 只作为构建期文件索引。
- 不为缺失翻译、缺失页面或非法图标提供默认值、空组件或静默跳转。
- 不修改认证、RBAC 授权算法、会话、操作日志和 Redis 行为。
- 不在 TypeScript 业务代码中使用 any、as any 或类型抑制。
- 不使用 Docker，不自动 commit、push、fetch 或 pull。

## 5. 统一命名规则

### 5.1 i18nKey

i18nKey 是语言包中的稳定键，不是最终显示文本。管理员在菜单表单中直接填写，
后端和前端只做格式校验，不再维护“已注册标题列表”。

统一格式为命名空间加点号分隔的 lower camel case：

~~~text
namespace.pageName
namespace.pageName.fieldName
~~~

约束：

- 必须是非空、首尾无空格的字符串；
- 长度不超过 128 个字符；
- 至少包含两个由 . 分隔的段；
- 每段以小写字母开头，只允许字母和数字；后续段可使用 lower camel case；
- 不允许空段、空格、下划线、斜杠、冒号、查询参数或哈希片段。

建议校验正则：

~~~text
^[a-z][a-z0-9]*(?:\.[a-z][a-zA-Z0-9]*)+$
~~~

示例：

~~~text
navigation.systemUsers
navigation.systemAuthPlatforms
permission.roleUpdate
reports.orders.list
~~~

表单在输入框下显示格式提示和示例。保存时不把 key 转成中文，也不自动生成 key。
语言包缺少该 key 时，vue-i18n 显示明确的 key；不得替换成菜单 code、path 或
其他默认标题。

### 5.2 页面路由和页面路径

page 菜单明确填写两个不同字段：

~~~text
路由 path：      /system/users
页面 componentPath：system/users
~~~

两者的职责不同：

- `path` 是浏览器地址和 Vue Router 的路由地址，必须以 `/` 开头；
- `componentPath` 是前端页面文件的相对路径，不能以 `/` 开头，也不包含文件扩展名；
- `componentPath` 映射到 `web/src/views/<componentPath>/index.vue`；
- 默认建议 `path` 与 `componentPath` 保持同一段结构，但允许明确配置不同值，以支持
  路由别名；
- 不根据其中一个字段静默生成、覆盖或修正另一个字段。

示例：

~~~text
path:          /system/users
componentPath: system/users
文件：         web/src/views/system/users/index.vue
~~~

`path` 和 `componentPath` 都使用小写 kebab-case 路径段。两者都不允许 query、hash、
动态参数、`.`、`..`、空段或末尾斜杠。页面文件不存在时，动态路由注册明确返回协议
错误并停止注册该访问快照；不得加载空组件、Dashboard 或其他页面作为兜底。

### 5.3 菜单显示状态

菜单新增 `isHidden` 字段，使用项目统一 YesNo 编码表达“是否隐藏”：

~~~text
0 = 否，显示在侧边栏
1 = 是，不显示在侧边栏
~~~

隐藏只影响菜单树的展示，不影响：

- `/api/v1/access` 返回该节点；
- 动态路由注册；
- 面包屑和直接访问；
- 后端接口认证和 RBAC 权限判断。

隐藏目录时，目录及其子树不显示在侧边栏；隐藏 page 时，页面入口从侧边栏移除，
但拥有权限的用户仍可通过地址或其他业务跳转进入。这个行为与旧项目的菜单渲染规则
一致：父目录隐藏时，不把子节点提升到上一级。action 不出现在侧边栏，其 `isHidden`
固定为 1，表单不展示该字段。

### 5.4 icon

icon 只保存名称字符串或 null：

- Element Plus 图标示例：Setting、Menu、UserFilled；
- Iconify 图标示例：mdi:shield、lucide:users；
- null 表示明确不显示图标。

后端校验非空字符串长度不超过 128 个字符，不复制前端组件白名单。图标解析错误由
DIcon 显示明确空状态，不自动替换为 Folder、Setting 或其他图标。

## 6. 数据库协议

### 6.1 sys_menu

保留字段：

~~~text
id
parent_id
menu_type
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
~~~

删除字段：

~~~text
view_key
~~~

字段形状：

| menu_type | parent_id | path | component_path | icon | is_hidden |
| --- | --- | --- | --- | --- | --- |
| directory | 根或 directory | NULL | NULL | 可选 | 可配置 |
| page | directory | 非空绝对路径 | 非空相对页面路径 | 可选 | 可配置 |
| action | page | NULL | NULL | NULL | 1 |

i18n_key、code、path、component_path、icon、is_hidden 均由 Service 做可读的格式
校验，数据库保留非空、长度、节点类型、`path` 唯一性和 YesNo 范围等约束。继续使用
deleted_at 软删除、显式 TIMESTAMPTZ 时间字段和当前 YesNo 编码。

这是一次明确的协议变更，不通过 AutoMigrate 静默删除字段。实现时必须在数据库 schema
流程中提供可审计的变更步骤：

1. 为现有 page 根据当前页面文件补齐 `component_path`；
2. 为现有 directory/page 补齐 `is_hidden=0`，为 action 补齐 `is_hidden=1`；
3. 删除旧约束中对 `view_key` 的引用；
4. 删除 `sys_menu.view_key`，并保留 `component_path`、`is_hidden` 的新约束。

任一步骤失败或存在无法转换的数据时，启动失败，不返回假成功。实现不得把旧
`view_key` 内容直接当作 `component_path`，除非它经过明确的路径映射校验。

### 6.2 不保留内置菜单数据

本设计删除菜单模块中的固定内置菜单定义，不再通过 EnsureBuiltin 自动创建或校验
以下业务菜单：

~~~text
system:menu:list
system:role:list
system:user:list
system:auth-platform:list
system:session:list
system:operation-log:list
~~~

数据库中的业务菜单全部由菜单管理功能创建、修改、禁用和软删除。菜单模块不再维护
固定 builtin code 集合，也不再返回 isBuiltin 字段或执行内置节点保护。普通菜单的
code、path、componentPath、父子类型、启用状态、隐藏状态和软删除规则仍由 Service
和 PostgreSQL 约束保护。

菜单管理页本身不依赖一条数据库菜单记录：它是前端静态注册的系统定位页，访问权限
使用固定的 system:menu:list API 权限。这个权限是菜单管理接口的安全契约，不代表一
条 sys_menu 记录。后端接口继续执行认证和权限校验，前端路由只负责在无权限时阻止
进入页面，不把静态路由伪装成动态菜单。

删除内置菜单数据后，首次部署仍必须通过明确的授权初始化流程为管理员提供
system:menu:list 权限；初始化流程不得重新插入 sys_menu 菜单记录。

## 7. HTTP 契约

### 7.1 菜单管理 API

现有 URL 和权限保持不变：

~~~text
GET    /api/v1/menus
POST   /api/v1/menus
PUT    /api/v1/menus/:id
PATCH  /api/v1/menus/:id/status
DELETE /api/v1/menus/:id
~~~

创建和更新请求使用 `componentPath` 和 `isHidden`，删除 `viewKey`：

~~~json
{
  "parentId": 1,
  "menuType": "page",
  "code": "system:user:list",
  "i18nKey": "navigation.systemUsers",
  "path": "/system/users",
  "componentPath": "system/users",
  "icon": "User",
  "sortOrder": 30,
  "isEnabled": 1,
  "isHidden": 0
}
~~~

directory/action 按字段形状显式提交 path、componentPath、icon 等可空字段；action
必须提交 `isHidden: 1`，directory/page 可提交 `0` 或 `1`。响应中同样删除 viewKey，
不得接受旧字段别名。

### 7.2 访问快照

GET /api/v1/access 的菜单节点统一使用 i18nKey、componentPath、isHidden，不再返回
titleKey 或 viewKey：

~~~json
{
  "code": "system:user:list",
  "menuType": "page",
  "path": "/system/users",
  "componentPath": "system/users",
  "i18nKey": "navigation.systemUsers",
  "icon": "User",
  "isHidden": 0,
  "children": []
}
~~~

directory 的 path、componentPath 为 null。action 不出现在可见菜单树中，但仍保留在
后端权限计算中，并固定返回 `isHidden: 1`。访问快照必须继续严格校验树结构、路径
唯一性、componentPath 合法性和字段闭合性；多个 page 可以复用同一个
`componentPath`，但 `path` 必须唯一。隐藏 directory/page 仍返回快照，不在
access API 层静默删除。

## 8. 前端动态路由

除菜单管理页外，动态路由仍由 /api/v1/access 返回的菜单树决定，但页面组件不再由
数据库指定。菜单管理页是唯一固定的系统定位页，不需要 sys_menu 记录。

数据流固定为：

~~~text
router guard
-> getAccess()
-> parseAccessSnapshot()
-> 安装静态 /system/menus（要求 system:menu:list）
-> 按 page.componentPath 查找 import.meta.glob 页面文件
-> registerAccessRoutes()
-> router.addRoute('admin-layout', ...)
~~~

页面文件索引只覆盖 web/src/views/**/index.vue。路由注册逻辑必须：

- 将 /system/users 精确映射到 ../views/system/users/index.vue；
- 动态路由的 URL 使用 node.path，动态组件使用 node.componentPath；两者不得互相
  推导或覆盖；
- 将 /system/menus 作为静态路由直接加载 web/src/views/system/menus/index.vue；
- 静态 /system/menus 路由只允许拥有 system:menu:list 权限的用户进入；
- 拒绝根路径、静态登录路径和不存在的页面文件；
- 检查动态路径和路由名称重复；
- 在任意页面解析失败时清理本次已经加入的动态路由并返回明确错误；
- 保留静态 Dashboard 和静态菜单管理页作为固定定位页，但不把它们作为其他动态页面
  缺失时的默认组件。

`isHidden` 不影响动态路由注册。菜单侧边栏组件在渲染时按节点的 `isHidden` 过滤：
隐藏目录不提升子节点，隐藏 page 不渲染菜单项；面包屑和直接访问仍按完整 access
树和当前 route 工作。

删除 routeViews 和 hasRouteViewKey。动态页面加载逻辑放在 web/src/router，不新增
运行时注册中心。

## 9. 前端菜单页面

### 9.1 表单字段

表单字段按节点类型显示：

| 类型 | 字段 |
| --- | --- |
| directory | parent、menuType、code、i18nKey、icon、sortOrder、isEnabled、isHidden |
| page | parent、menuType、code、i18nKey、path、componentPath、icon、sortOrder、isEnabled、isHidden |
| action | parent、menuType、code、i18nKey、sortOrder、isEnabled；isHidden 固定为 1 |

删除页面视图选择器。page 使用两个普通文本输入框：一个填写 path（路由），一个
填写 componentPath（页面路径），页面文件是否存在由动态路由规则在访问快照解析时
验证。

路由输入框约定：

- label 使用“路由”；
- placeholder 使用类似 /system/users 的绝对路径示例；
- 输入框下提示“必须以 / 开头，例如 /system/users”；
- 只允许小写 kebab-case 路径段，禁止 query、hash、动态参数和末尾斜杠；
- 输入值使用整行或较宽的表单列，避免路径被截断。

页面路径输入框约定：

- label 使用“页面路径”；
- placeholder 使用类似 system/users 的相对路径示例；
- 输入框下提示“不能以 / 开头，页面文件为 web/src/views/<页面路径>/index.vue”；
- 只允许小写 kebab-case 路径段，禁止 query、hash、动态参数、文件扩展名和末尾斜杠；
- `componentPath` 不自动从 path 生成，也不因 path 修改而联动变化。

code 编辑时只读；i18nKey、path、componentPath、code 的输入框下显示明确的格式说明。
directory/page 显示“菜单显示”单选或开关，action 不显示该控件。提示内容必须进入
中英文语言包，并与实际校验规则一致。

### 9.2 宽度与布局

- el-input、el-select-v2、el-input-number 在表单列中使用 width: 100%；
- i18nKey、path 和 componentPath 使用整行或可用宽度较大的表单项，避免长值被挤压；
- 桌面端保持双列，移动端切换单列；
- 表单项 label 宽度统一，提示文本在控件下方显示，不占用控件横向空间；
- 图标选择按钮使用完整可用宽度，并展示当前图标预览和名称；
- Dialog 根据内容自适应高度，不设置与表单数量无关的固定高度；
- 不用额外的嵌套卡片或复杂布局抽象。

### 9.3 图标组件

菜单页面不再导入 menuIcons，不再使用 component :is：

~~~vue
<DIcon v-if="row.icon !== null" :icon="row.icon" />

<IconSelect
  v-model="iconSelectVisible"
  @select-icon="selectMenuIcon"
/>
~~~

IconSelect 内部用 DIcon 展示候选项。菜单页面只接收和提交字符串，不感知图标来源的
内部实现。

删除以下重复职责：

- web/src/access/menu-icons.ts；
- 菜单页面中的图标组件索引和白名单判断；
- 任何把未知图标替换成默认图标的逻辑。

## 10. 前端目录调整

删除不再承担协议注册职责的文件：

~~~text
web/src/access/menu-title-keys.ts
web/src/access/route-views.ts
web/src/access/protocol.ts
web/src/access/menu-icons.ts
~~~

如果 web/src/access 没有其他真实业务职责，则删除该目录；访问状态和快照 API 仍
分别归属于 store/access.ts、api/access.ts 和 api/access.contract.ts，不因为目录删除
而合并成一个万能文件。

DIcon、IconSelect 保留在：

~~~text
web/src/components/DIcon
web/src/components/IconSelect
~~~

## 11. 错误处理

- i18nKey 格式错误：返回明确的菜单字段错误；
- page path 或 componentPath 格式错误、前后斜杠错误或重复：返回明确的菜单路径错误；
- 页面文件不存在：访问快照安装动态路由时返回协议错误；
- action 提交 isHidden=0：返回明确的菜单字段错误；
- 隐藏状态不改变路由或权限：不把隐藏菜单误判成不存在；
- 图标名称无法解析：DIcon 显示明确空状态并在开发环境报告错误；
- API 响应缺字段、多字段或旧 viewKey/titleKey 字段：严格 DTO 解析失败；
- 不把错误转换为空树、默认标题、默认图标或 Dashboard 页面。

后端仍遵循：

~~~text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
~~~

前端仍遵循：

~~~text
view -> api/<module>.ts -> utils/request.ts -> Go API
~~~

## 12. 测试与验收

### 12.1 后端

- schema 不再包含 view_key，旧约束引用被清理；
- 不再自动创建内置菜单记录；菜单管理静态路由可用且按 system:menu:list 权限保护；
- 合法自定义 i18nKey 可以创建和更新；
- 空 key、非法分隔符、超长 key 和首尾空格被拒绝；
- page path 唯一性、componentPath 合法性，节点树约束和软删除规则保持不变，不再存在核心节点保护；
- isHidden 只接受 0/1，action 固定为 1，隐藏不影响权限快照；
- 请求和响应严格不包含 viewKey；
- /api/v1/access 返回 i18nKey、componentPath、isHidden，不返回 titleKey。

### 12.2 前端

- 菜单表单使用文本输入而不是标题选择器；
- i18nKey、code、path、componentPath、isHidden 的提示文案和真实校验规则一致；
- 表单 input、select-v2、input-number 宽度填满所在列；
- page 表单显示路由和页面路径两个输入框，不显示 viewKey；
- 隐藏菜单不显示在侧边栏，仍能注册路由、生成面包屑并按权限直接访问；
- 菜单表格和表单都使用 DIcon；
- 图标选择使用 IconSelect，选择后正确回显和提交；
- /system/users 精确加载 views/system/users/index.vue；
- 缺失页面文件、重复路径和非法访问协议明确失败；
- 动态路由注册失败时已加入的路由被清理；
- 全仓搜索不再发现 menuTitleKeys、routeViews、viewKey、titleKey 旧协议；
- 不出现显式 any、as any 或类型抑制。

验证命令：

~~~powershell
cd D:\\admin\\server
go fmt ./...
go vet ./...
go test ./...
go build ./...

cd D:\\admin\\web
pnpm vitest run
pnpm build
~~~

## 13. 实施顺序

1. 先调整数据库 schema、Go Model、菜单协议并移除内置菜单定义；
2. 调整菜单 Service、Repository、Handler 及 access 快照字段；
3. 调整前端 API contract 和菜单管理表单；
4. 用 DIcon、IconSelect 替换页面内图标映射；
5. 按 componentPath + import.meta.glob 重写动态页面加载和路由注册，并按 isHidden 过滤菜单展示；
6. 删除旧 access 注册文件和所有旧字段引用；
7. 按后端、前端顺序执行定向测试、类型检查、全量测试和构建。

本 spec 经用户审阅确认后，再编写 implementation plan；确认前不修改运行时代码。
