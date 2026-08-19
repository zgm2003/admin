# Menu Management Phase One Design

## 1. 目标

实现系统管理一期的第一个完整业务切片：菜单管理。

菜单不是一张可以套用通用 CRUD 的平面表，而是一棵带有节点类型、父子约束、
权限语义和级联行为的树。本期仍坚持项目已经确定的线性数据流：

```text
前端菜单管理页面
-> menu API
-> router / middleware
-> handler
-> service
-> repository
-> PostgreSQL
```

每一步都对应一个真实职责。出现问题时，可以沿着这条调用链直接定位，不引入
通用 CRUD、BaseService、BaseRepository、TreeService、Manager、Factory、
Adapter、Platform 或运行时注册框架。

## 2. 本期范围

本期包含：

- 菜单管理核心节点的后端幂等初始化；
- 菜单树查询、创建、修改、状态修改和级联软删除 API；
- directory、page、action 三种节点的完整树约束；
- 核心节点保护；
- 菜单相关后端中英文错误；
- `/system/menus` 动态页面和 `system-menus` 视图注册；
- Element Plus 树形表格和按节点类型变化的抽屉表单；
- 菜单标题、视图和图标的显式前端注册表；
- 按钮权限、严格 TypeScript DTO 和前后端测试。

本期完成后，菜单管理具备真实可用的纵向链路。用户管理、角色管理和角色授权
页面在后续切片实现，不借本期顺手扩展。

## 3. 非目标和硬约束

- 不实现用户管理、角色管理或角色授权页面；
- 不实现 Redis 权限缓存或进程级权限缓存；
- 不修改登录、会话或邮箱注册流程；
- 不把 Dashboard、Login、Forgot Password 等固定页面写入 `sys_menu`；
- 不做分页式平面菜单列表；
- 不做通用 CRUD 页面生成器、通用树组件或跨模块 Base 层；
- 不用空页面、默认图标、原始 i18n key 或其他静默兜底掩盖协议错误；
- 不在 TypeScript 业务代码中使用 `any`、`as any` 或类型抑制；
- 不启动或停止用户的服务，不使用 Docker；
- 不自动 commit、pull、fetch 或 push。

Dashboard 继续由前端静态路由提供，作为动态路由加载失败时仍可进入的稳定定位
页面。菜单管理页面本身是 RBAC 动态页面。

## 4. 核心业务模型

### 4.1 节点类型

`sys_menu.menu_type` 只有三种值：

| 类型 | 作用 | 允许的直接子节点 |
| --- | --- | --- |
| `directory` | 组织导航层级，不直接代表业务接口权限 | `directory`、`page` |
| `page` | 页面入口及页面基础权限 | `action` |
| `action` | 新增、修改、删除等细粒度权限 | 无 |

父子规则固定为：

- 根节点必须是 `directory`，并且 `parent_id IS NULL`；
- directory 可以挂在根或另一个 directory 下；
- page 必须挂在 directory 下；
- action 必须挂在 page 下；
- action 永远是叶子节点；
- 不接受 `parentId=0`、`parentId=1` 或其他根节点哨兵值。

API 创建根节点时必须显式提交 `"parentId": null`。缺少 `parentId` 与明确提交
`null` 是不同状态，Handler 不得把缺失字段猜成根节点。

### 4.2 权限语义

page 自身就是页面基础权限。列表页的 page code 使用 `xxx:list`，例如：

```text
system:menu:list
```

不在它下面再创建一个重复的 `list` action。需要更细权限时，增加 action：

```text
system:menu:create
system:menu:update
system:menu:delete
```

权限规则保持现有 RBAC 设计：

- 直接授予 page，只获得该 page，不自动获得它的 action；
- 直接授予 action，运行时向上推导它的 page 和目录祖先；
- directory 只由已授权的 page/action 推导，不直接写入 `sys_role_menu`；
- page 没有 action 时，它仍然可以作为一个只有基础访问权限的完整页面；
- 某个接口存在时，后端路由必须显式绑定 page 或 action code；
- 没有配置 action 不表示自动放行，也不允许 Service 根据菜单数量猜权限。

### 4.3 稳定身份

所有节点都有非空 `code`。code 是节点和权限的稳定身份，不是显示名称：

- 创建时由管理者显式填写；
- 创建成功后不可修改；
- 使用小写冒号分段命名，例如 `system`、`system:menu:list`；
- 不根据标题、path 或父节点自动生成；
- 软删除后允许按照现有部分唯一索引规则重新使用 code，但旧授权不会自动转移
  到新节点。

directory code 用于树节点身份和访问快照去重，但不作为直接授权记录。

## 5. 数据模型和数据库约束

### 5.1 `sys_menu`

继续使用已经建立的字段：

```text
id
parent_id
menu_type
code
i18n_key
path
view_key
icon
sort_order
is_enabled
created_at
updated_at
deleted_at
```

字段规则：

- `parent_id` 是可空自引用外键，物理删除使用 `ON DELETE RESTRICT`；
- `code`、`i18n_key` 非空；
- directory 的 `path`、`view_key` 都为 `NULL`；
- page 的 `path`、`view_key` 都是非空字符串；
- action 的 `path`、`view_key`、`icon` 都为 `NULL`；
- directory/page 的 `icon` 可以为 `NULL`，非空时必须是注册图标 key；
- `sort_order` 是大于或等于 0 的整数；
- `is_enabled` 使用系统 Yes/No 编码：`0=否`、`1=是`；
- `created_at`、`updated_at` 使用非空 `TIMESTAMPTZ`；
- 软删除只使用 `deleted_at TIMESTAMPTZ NULL`，不增加 `is_del`。

数据库继续保留有效 code 的部分唯一索引，并增加有效 page path 的部分唯一
索引，避免并发请求只依靠 Service 查询产生竞态：

```sql
CREATE UNIQUE INDEX ux_sys_menu_code_active
ON sys_menu (code)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX ux_sys_menu_page_path_active
ON sys_menu (path)
WHERE deleted_at IS NULL AND menu_type = 'page';
```

现有类型 CHECK 继续约束 path/viewKey 组合；本期补充 CHECK，确保
`sort_order >= 0`、directory path 为 `NULL`、action icon 为 `NULL`。Service
仍然做可读的业务校验，数据库约束负责阻止并发或绕过 Service 的非法写入。

不要求 `view_key` 唯一：不同页面路径可以在明确配置下复用同一个页面组件。
不为 i18n key、icon 等没有真实查询需求的字段机械增加索引。

### 5.2 `sys_role_menu`

本期不提供角色授权接口，但菜单变更必须维护现有授权关系：

- 禁用节点时保留 `sys_role_menu`，权限暂时失效；
- 重新启用后，原授权可以恢复生效；
- 删除节点时，同一事务软删除整棵子树关联的有效 `sys_role_menu`；
- 如果普通节点要从 page/action 改为 directory，并且仍有有效直接授权，则拒绝
  修改，不静默删除授权；
- 修改父级或在 page/action 之间转换时，直接授权仍指向同一个稳定 menu ID。

### 5.3 内置状态

数据库不增加 `is_builtin`。内置节点由后端固定 code 集合识别，管理查询返回
计算字段 `isBuiltin`。这样不会再引入一列可被误改的状态，也不会把保护规则交给
前端决定。

## 6. 核心菜单初始化

应用在数据库建表和约束检查完成后、开始监听 HTTP 端口前，初始化以下核心树：

```text
system
`-- system:menu:list
    |-- system:menu:create
    |-- system:menu:update
    `-- system:menu:delete
```

初始协议值：

| code | type | parent code | i18nKey | path | viewKey | icon | sortOrder |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `system` | directory | `NULL` | `navigation.system` | `NULL` | `NULL` | `Setting` | 100 |
| `system:menu:list` | page | `system` | `navigation.systemMenus` | `/system/menus` | `system-menus` | `Menu` | 10 |
| `system:menu:create` | action | `system:menu:list` | `permission.menuCreate` | `NULL` | `NULL` | `NULL` | 10 |
| `system:menu:update` | action | `system:menu:list` | `permission.menuUpdate` | `NULL` | `NULL` | `NULL` | 20 |
| `system:menu:delete` | action | `system:menu:list` | `permission.menuDelete` | `NULL` | `NULL` | `NULL` | 30 |

所有核心节点初始 `is_enabled=1`。

初始化必须在一个 PostgreSQL 事务中按父到子的顺序执行，并遵守：

1. 空数据库中创建缺少的核心节点；
2. 所有核心节点已存在且结构正确时不重复插入；
3. 部分节点缺少时，只创建能够与现有正确结构组成完整树的缺少节点；
4. 已有核心 code 的类型、父级、i18nKey、path 或 viewKey 不符合固定协议时，
   启动失败；
5. 核心 code 存在软删除历史或出现无法唯一识别的记录时，启动失败；
6. 不静默恢复、移动、改名或覆盖损坏的核心节点；
7. icon 和 sortOrder 是允许运营调整的字段，初始化只在首次创建时写默认值，
   后续启动不覆盖。

启动失败时日志记录具体损坏的 code 和字段，但 HTTP 响应中不得暴露数据库细节。

## 7. 核心节点保护

核心节点不能：

- 软删除；
- 禁用；
- 修改 `menuType`；
- 修改 `parentId`；
- 修改 `code`；
- 修改 `i18nKey`；
- 修改 `path`；
- 修改 `viewKey`。

核心 directory/page 可以修改已注册的 icon 和 sortOrder；核心 action 只在当前
表单能力内修改 sortOrder。对核心节点提交其他字段变化时，后端返回明确冲突，
不能因为前端把输入框设为只读就省略后端保护。

普通节点的 code 同样不可修改。其他可编辑字段必须按照修改后的节点类型和整棵
树重新校验，不能只校验发生变化的单个字段。

## 8. HTTP API

### 8.1 路由和权限

```http
GET    /api/v1/menus             -> system:menu:list
POST   /api/v1/menus             -> system:menu:create
PUT    /api/v1/menus/:id         -> system:menu:update
PATCH  /api/v1/menus/:id/status  -> system:menu:update
DELETE /api/v1/menus/:id         -> system:menu:delete
```

所有接口先经过已有认证中间件，再经过已有显式权限中间件。Handler 不自行判断
角色名称，Service 也不接收 Gin Context。

### 8.2 严格请求 DTO

创建请求的全部字段都必须出现；可空字段也要显式提交 `null`：

```json
{
  "parentId": null,
  "menuType": "directory",
  "code": "reports",
  "i18nKey": "navigation.reports",
  "path": null,
  "viewKey": null,
  "icon": "Folder",
  "sortOrder": 100,
  "isEnabled": 1
}
```

修改请求不接受 code，也不修改状态：

```json
{
  "parentId": 1,
  "menuType": "page",
  "i18nKey": "navigation.systemMenus",
  "path": "/system/menus",
  "viewKey": "system-menus",
  "icon": "Menu",
  "sortOrder": 10
}
```

状态请求只接受：

```json
{
  "isEnabled": 0
}
```

协议规则：

- JSON 使用 lower camel case；
- `:id` 必须是正整数；
- 请求必须拒绝未知字段、缺少字段、错误类型和 Yes/No 范围外的数字；
- 不接受字符串数字、空字符串代替 null、`msg` 或旧字段别名；
- code、path、viewKey、i18nKey、icon 在保存前去除首尾空白，但不做大小写
  猜测或自动改名；去除后为空按无效参数处理；
- path 必须是绝对路径，不含 query/hash，不允许使用 `/`、`/login`、
  `/dashboard` 这三个当前静态路由；
- 创建启用节点时，其完整父链必须全部启用；创建禁用节点时仍要满足父子结构。

### 8.3 管理树响应

`GET /api/v1/menus` 查询所有未软删除节点，包括启用和禁用节点，不分页，返回
完整树。`data` 是数组，不是平面分页对象：

```json
{
  "code": 0,
  "data": [
    {
      "id": 20,
      "parentId": null,
      "menuType": "directory",
      "code": "reports",
      "i18nKey": "navigation.reports",
      "path": null,
      "viewKey": null,
      "icon": "Folder",
      "sortOrder": 100,
      "isEnabled": 1,
      "isBuiltin": false,
      "createdAt": "2026-08-19T00:00:00Z",
      "updatedAt": "2026-08-19T00:00:00Z",
      "children": []
    }
  ],
  "message": "ok"
}
```

每个 `children` 都必须是数组，叶子节点返回 `[]`，不能返回 `null` 或省略。
`parentId`、`path`、`viewKey`、`icon` 按字段语义明确返回值或 `null`。同级节点
按照 `sortOrder ASC, code ASC` 稳定排序。

创建成功返回 HTTP 201 和 `data: {"id": <newId>}`。修改、状态修改和删除成功
返回 HTTP 200；修改/删除返回 `data: {"id": <id>}`，状态修改返回
`data: {"id": <id>, "isEnabled": 0|1}`。所有响应始终只有 `code`、`data`、
`message` 三个 envelope 字段。

## 9. Service 树算法和事务

### 9.1 查询建树

Repository 用一次查询取得所有有效节点，不对每个节点递归查询数据库。Service：

1. 建立 `id -> node` 映射；
2. 建立 `parentId -> children` 邻接表；
3. 检查重复 ID、孤儿节点、非法父子类型和循环；
4. 对每组同级节点稳定排序；
5. 从 `parentId=nil` 的 directory 生成响应树。

基础建树为 O(n)，同级排序使整体上界为 O(n log n)。这套函数只属于 menu
模块，不提取为全局 TreeService。

如果数据库中存在孤儿、循环或非法形状，接口返回稳定的菜单树损坏错误，不能
丢弃坏节点后返回一棵看似成功的残缺树。

### 9.2 创建

创建在一个事务中完成：

1. 校验请求的字段形状和注册值；
2. 加载并锁定需要的现有菜单行；
3. 校验父节点存在、未删除、类型允许且启用状态符合规则；
4. 校验 code/path 冲突；
5. 插入节点；
6. 提交后返回 ID。

数据库唯一索引是并发冲突的最终保证。唯一索引错误必须转换为稳定的 code/path
冲突业务错误，不能把 PostgreSQL 错误文本返回给客户端。

### 9.3 修改和移动

修改和移动是一次完整的树变换，不是直接更新一行：

1. 在事务中锁定有效菜单树，并找到目标节点；
2. 核心节点先执行保护检查；
3. 建立目标节点修改后的候选树；
4. 拒绝自己挂自己、挂到自己的任意后代、父节点不存在或类型不匹配；
5. 按新类型校验 path、viewKey、icon 和子节点类型；
6. 如果修改后的目标或其启用后代仍为启用状态，检查它们的完整父链全部启用；
7. 检查 code/path 唯一性以及 directory 直接授权冲突；
8. 整棵候选树合法后才写入目标行并提交。

后代集合通过一次已加载树的邻接表遍历得到，访问集合用于检测循环。不能用
递归 N+1 SQL，也不能在检测到问题后保存一半字段。

### 9.4 禁用和启用

禁用普通节点：

- 在一个事务中取得整棵有效子树；
- 把目标及全部后代的 `is_enabled` 更新为 0；
- 保留所有 `sys_role_menu`；
- 任意一步失败则全部回滚。

启用普通节点：

- 只启用当前目标，不自动启用后代；
- 启用前沿 `parent_id` 向上检查完整父链；
- 任一祖先被禁用、软删除、缺失或出现循环时拒绝启用；
- 同状态请求是幂等成功，但对核心节点提交禁用始终拒绝。

这一不对称规则让禁用可以立即切断整块功能，同时避免重新启用父目录时意外
恢复过去有意关闭的子权限。

### 9.5 删除

删除普通节点是整棵子树的级联软删除：

1. 在事务中锁定有效菜单树；
2. 从目标节点沿邻接表收集全部后代 ID，并检测循环；
3. 如果目标或子树包含核心节点，拒绝删除；
4. 使用同一个时间戳软删除这些 menu ID 对应的有效 `sys_role_menu`；
5. 使用同一个时间戳更新整棵子树的 `updated_at` 和 `deleted_at`；
6. 任意一步失败时回滚授权和菜单的全部变更。

不逐个调用“删除单行”方法，也不把数据库外键改成物理 `ON DELETE CASCADE`。

## 10. 注册表和协议完整性

### 10.1 菜单标题

前端建立专门的 `menuTitleKeys` 注册表。抽屉只能从这里选择 i18nKey，不能输入
任意字符串。注册表中的 key 必须同时存在于 `zh-CN` 和 `en-US`，类型上收窄为
已有 `AppMessageKey`。

核心标题至少增加：

```text
navigation.system
navigation.systemMenus
permission.menuCreate
permission.menuUpdate
permission.menuDelete
```

页面自己的按钮、表格、表单、确认框和通知文案也必须同时提供中英文。两个 locale
保持完全相同的 key 集合，不从另一语言静默回退。

### 10.2 页面视图

`routeViews` 显式增加：

```text
system-menus -> web/src/views/system/menus/index.vue
```

页面表单只能选择 `routeViews` 中存在的 viewKey。未知 viewKey 是协议或参数错误，
不能映射为空组件，也不能根据字符串拼接 import 路径。

### 10.3 图标

菜单图标只能使用 `menuIcons` 的显式 key。表单以图标预览和名称选项展示，不接收
任意组件名。`null` 表示明确不显示图标；未知 key 不能回退为 Folder 或 Setting。

后端维护与上述三类前端注册值一致的菜单协议白名单，对非前端调用同样拒绝未知
值。这里的有限常量是跨端数据协议，不是运行时插件注册器。新增菜单页面时必须
同时修改前端注册表、后端白名单和对应契约测试。

## 11. 后端错误和国际化

菜单业务错误使用现有 `14000+` 范围，至少稳定区分：

| code | HTTP | 场景 |
| --- | --- | --- |
| `14000` | 500 | 已有访问快照或菜单树数据损坏 |
| `14001` | 404 | 菜单不存在或已删除 |
| `14002` | 409 | 有效 code 冲突 |
| `14003` | 409 | 有效 page path 冲突 |
| `14004` | 400 | 父节点不存在、类型不匹配或 parentId 非法 |
| `14005` | 400 | 自己挂自己、挂到后代或发现循环 |
| `14006` | 409 | 核心节点保护规则阻止操作 |
| `14007` | 409 | 启用节点时父链未全部启用 |
| `14008` | 409 | 节点类型变化与现有子节点或直接授权冲突 |
| `14009` | 400 | 节点字段形状或注册值无效 |

认证缺失和权限不足继续使用统一 `401/403`。PostgreSQL 查询或事务失败继续转换为
依赖不可用错误。所有菜单错误在后端 `zh-CN`、`en-US` 目录中使用相同 message
key 和参数集合，不把 SQL、表名、堆栈或内部 Cause 返回给客户端。

## 12. 前端页面

### 12.1 页面结构

页面位于：

```text
web/src/views/system/menus/index.vue
```

它使用 Element Plus 树形表格，Main 继续是唯一纵向滚动容器。页面本身不创建
body 滚动，也不引入全局业务组件框架。

顶部只有当前业务需要的操作：标题、新增根目录、刷新。表格列为：

- 翻译后的标题；
- 节点类型；
- 权限码；
- path/viewKey；
- icon；
- sortOrder；
- 启用状态；
- 是否内置；
- 操作。

表格展示所有未删除节点，包括禁用节点；禁用状态必须清楚可见。树形展开由当前
页面本地状态管理，不创建菜单管理 Store。

### 12.2 权限按钮

前端使用已有 `hasPermission` 结果控制命令是否显示或可用：

- 新增根节点、添加子节点：`system:menu:create`；
- 编辑、启用、禁用：`system:menu:update`；
- 删除：`system:menu:delete`。

这只改善界面体验。直接调用 API 时后端仍由权限中间件做真正鉴权。

### 12.3 抽屉表单

创建时所有节点都显式填写 code；编辑时 code 只读且不进入更新 payload。

按类型显示：

| 类型 | 字段 |
| --- | --- |
| directory | parent、code、i18nKey、icon、sortOrder、enabled |
| page | parent、code、i18nKey、path、viewKey、icon、sortOrder、enabled |
| action | 所属 page、code、i18nKey、sortOrder、enabled |

编辑状态通过单独的状态接口完成，因此编辑抽屉不提交 `isEnabled`。创建抽屉需要
提交它。切换类型时必须清空新类型不允许的字段，不能把隐藏输入的旧值带给 API。

父节点选择器：

- 创建 directory 可以选择“根目录”，其值严格为 `null`；
- 创建/编辑时只显示当前类型允许的父节点；
- 编辑时排除自己和自己的全部后代；
- 不使用 0 作为“根目录”的组件值提交给 API。

核心节点的只读字段、删除按钮和禁用控制由 UI 明确禁用；允许编辑的 icon、排序
仍可操作。禁用和删除普通节点前使用确认框说明对子树的影响。

### 12.4 前端数据流

```text
进入菜单页面
-> GET /api/v1/menus
-> 严格校验 DTO
-> 保存为页面本地 ref
-> Element Plus tree table 渲染

提交表单或确认命令
-> 对应 mutation API
-> 成功后仅重新 GET /api/v1/menus
-> 更新当前管理表格
-> 提示全局菜单和权限需手动刷新页面后更新
```

不使用乐观更新。API 响应缺字段、字段多余、类型错误、未知注册 key 或 `children`
为 null 时，严格 DTO 解析失败并交给统一请求错误通知；页面不使用 `?? []`、
原始 key 或默认 icon 掩盖协议错误。

## 13. 管理者和受影响用户的实际表现

菜单操作成功后，系统有两个不同视角：

**当前管理者：** 菜单管理表格立即重新请求并显示数据库最新树；抽屉关闭，页面
提示“管理数据已更新，刷新页面后侧边栏和路由生效”。系统不强制跳转，不让正在
操作的人突然失去页面上下文。

**受影响用户：** 已打开页面和当前侧边栏暂时保持原样；下一次业务 API 请求会
直接按 PostgreSQL 最新权限判断。如果权限已失效，后端立即返回 403。用户手动
刷新后，前端重新请求 `/api/v1/access`，侧边栏、动态路由和按钮权限统一变为
最新状态。

因此菜单 mutation 成功后明确禁止：

- 立即重新请求 `/api/v1/access`；
- 热删除或热替换全局动态路由；
- 强制 `window.location.reload()`；
- 在菜单管理页本地模拟全局权限变化。

当前权限检查不使用 Redis 缓存，所以数据库提交后立即生效。未来引入 Redis 时，
缓存失效策略另写设计，不能在本期提前加入空缓存层。

如果刷新管理表格时权限已被另一个并发的角色操作收回，请求会得到 403。前端按
统一错误流程通知，不伪造成功列表，也不恢复已经提交的菜单事务。

## 14. 预期代码边界

后端在已有 `server/internal/module/menu` 内补齐真实业务文件：

```text
route.go
handler.go
request.go
response.go
service.go
repository.go
builtin.go        # 仅负责固定核心菜单初始化和校验
```

现有 `model.go`、`schema.go` 按本设计补充必要约束和索引。私有建树函数留在 menu
模块内；如果文件过长，可以按真实职责拆为同包文件，但不能导出通用 Tree API。

前端预期增加或修改：

```text
web/src/api/menu.ts
web/src/api/menu.contract.ts
web/src/access/menu-title-keys.ts
web/src/access/menu-icons.ts
web/src/access/route-views.ts
web/src/views/system/menus/index.vue
web/src/i18n/messages/zh-CN.ts
web/src/i18n/messages/en-US.ts
```

页面内部确有复用价值的表单片段可以放在
`web/src/views/system/menus/components/`，不提升为全局组件。

## 15. 测试策略

### 15.1 后端

Repository 和 Service 的关键树行为使用真实 PostgreSQL 事务测试，至少覆盖：

- 核心菜单首次创建、重复启动幂等；
- 核心结构字段损坏、软删除历史和部分错误结构导致启动失败；
- `parentId: null` 创建根目录，缺少 parentId 和 `0` 被拒绝；
- 父节点不存在和 directory/page/action 父子类型不匹配；
- 自己挂自己、挂到任意后代和循环检测；
- code 冲突、page path 冲突及并发唯一索引错误转换；
- 核心节点删除、禁用和结构字段修改被拒绝；
- 普通节点 code 不可修改；
- 按新类型校验字段、子节点和现有直接授权；
- 禁用整棵子树但保留 role-menu；
- 启用只影响当前节点并检查完整父链；
- 删除整棵子树并软删除对应 role-menu；
- 删除和状态事务任一步失败时完整回滚；
- 管理树包含启用/禁用节点、children 永远为数组且排序稳定；
- 数据库存在孤儿或非法树时返回错误，不返回残缺成功数据；
- 五个 API 的 401、403、参数错误、业务错误和成功 envelope；
- `zh-CN`、`en-US` 菜单错误 key、参数和最终文案一致。

### 15.2 前端

前端至少覆盖：

- 菜单管理 DTO 严格字段、null、children 和注册 key 校验；
- 树形表格渲染启用和禁用节点；
- `isBuiltin` 只读、不可删除和不可禁用控制；
- create/update/delete 按钮权限；
- 三种节点表单字段和切换类型后的清理；
- 根节点提交 `parentId: null`，不提交 0；
- parent 选择排除自身、后代和非法类型；
- i18nKey、viewKey、icon 只能选已注册值；
- zh-CN/en-US key 集合一致；
- 成功 mutation 后只刷新 `GET /api/v1/menus`；
- 不重新加载 `/api/v1/access`、不热替换路由、不强制浏览器刷新；
- 协议错误没有空数组、默认图标或原始 key 兜底；
- 新增 TypeScript 没有裸 `any` 或类型抑制。

## 16. 验收标准

本期完成必须同时满足：

1. 后端启动可以幂等建立正确核心菜单，核心结构损坏时明确失败；
2. super_admin 刷新登录态后能从动态侧边栏进入 `/system/menus`；
3. 管理页显示完整、稳定排序的启用/禁用菜单树；
4. directory、page、action 的创建、修改和移动遵守全部树约束；
5. 禁用级联、启用单节点、删除级联及 role-menu 处理都在事务中正确完成；
6. 核心节点保护同时存在于前端交互和后端 Service；
7. 每个 API 都有明确权限码、严格 DTO、稳定 envelope 和双语错误；
8. mutation 后只刷新管理表格，后端权限立即生效，全局 UI 等用户手动刷新；
9. 前端 i18n/view/icon 注册协议无猜测、无静默兜底、无 TypeScript `any`；
10. 后端测试、构建，前端测试、构建和 `git diff --check` 全部通过。

本 spec 经确认后再编写 implementation plan；确认前不修改运行时代码。
