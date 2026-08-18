# Backend i18n and RBAC Foundation Design

## 1. 目标

为当前单体 Admin 系统建立一套可直接落地的后端国际化和 RBAC 基础，
并让前端能够根据当前登录用户加载菜单、动态路由和按钮级权限。

设计坚持项目已有的线性数据流：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

不引入通用 Adapter、Manager、Factory、DI 容器、运行时注册器、策略引擎或
为了未来替换而存在的抽象层。

## 2. 范围

本期包含：

- 后端 `zh-CN`、`en-US` 错误国际化；
- `sys_menu`、`sys_role_menu` 及相关 RBAC 数据约束；
- 多角色并集权限计算；
- `super_admin` 全量授权规则；
- `GET /api/v1/access` 访问快照接口；
- 后端显式声明的接口权限中间件；
- 前端 access 状态、动态菜单、动态路由和按钮权限；
- 相关单元测试、集成测试和迁移顺序。

紧接着的下一期再实现用户、角色、菜单的管理 API 和页面。本期不做这些
CRUD 页面，也不插入用于展示的假生产菜单。

## 3. 不做的事情

- 不做注册页面；邮箱登录和现有认证流程保持不变；
- 不做多租户、组织树、数据范围权限或策略引擎；
- 不做会话管理改造，当前保持单会话；
- 不做 Redis 权限缓存和进程级缓存；
- 不做菜单运行时组件猜测或空组件兜底；
- 不把 Dashboard、Login、未来的 Forgot Password 等公共或定位页面放入
  `sys_menu`；
- 不使用 Docker 或 Docker Compose；
- 不自动创建 Git commit、fetch、pull 或 push。

## 4. 后端国际化

### 4.1 语言协商

支持语言固定为 `zh-CN` 和 `en-US`。

- 缺少 `Accept-Language` 时使用 `zh-CN`；
- 请求明确指定不支持的语言时返回 `400`，不静默回退；
- 成功或失败响应都返回 `Content-Language`；
- 语言解析在请求进入 Handler 前完成，后续错误响应使用解析后的语言。

### 4.2 错误模型

业务错误使用具体类型：

```go
type Error struct {
    HTTPStatus int
    Code       int
    MessageKey string
    Params     map[string]string
    Cause      error
}
```

只有统一响应层负责把 `MessageKey` 和 `Params` 翻译为最终 `message`，HTTP
响应 envelope 始终只有：

```json
{
  "code": 0,
  "data": {},
  "message": "ok"
}
```

保留已有 `10000-10006` 错误码；新增范围为：

- `11000+`：认证和会话；
- `12000+`：用户；
- `13000+`：角色；
- `14000+`：菜单和访问权限。

中文和英文目录必须拥有完全一致的 key、参数名和参数数量。目录缺少 key
时服务启动失败，不在运行时使用空字符串或另一语言猜测替代。

## 5. RBAC 数据模型

### 5.1 `sys_menu`

字段如下：

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

约束：

- 根节点 `parent_id` 为 SQL `NULL`，不使用 `0`、`1` 或其他哨兵值；
- API 创建根节点时必须显式传递 `"parentId": null`；缺少字段、`0`、负数
  或不存在的父节点均为参数错误；
- `parent_id` 是自引用外键，`ON DELETE RESTRICT`；
- `menu_type` 只有 `directory`、`page`、`action`；
- directory 可以包含 directory/page；
- page 必须有 `path` 和 `view_key`，可以包含 action；
- action 是叶子节点，不允许 `path` 或 `view_key`；
- `is_enabled` 使用项目 Yes/No 编码，数据库类型为 `SMALLINT`，并有
  `CHECK (is_enabled IN (0, 1))`；
- `created_at`、`updated_at` 使用非空 `TIMESTAMPTZ`；
- 软删除唯一使用 `deleted_at`，不增加 `is_del` 或其他删除字段；
- 只对真实查询创建索引，至少覆盖 `parent_id`、有效节点查询和排序查询。

### 5.2 `sys_role_menu`

字段如下：

```text
id
role_id
menu_id
created_at
updated_at
deleted_at
```

它只保存**直接授权**的 page/action 节点。目录节点和自动推导的祖先不落库。

- 有效的 `(role_id, menu_id)` 使用 `deleted_at IS NULL` 部分唯一索引；
- 角色或菜单关联删除使用 `deleted_at`；
- 恢复授权时恢复已有关联并维护 `updated_at`，不插入重复记录；
- 只有启用且未删除的 menu 才能被直接授权。

### 5.3 角色和用户关系

保留现有 `sys_user_role` 多角色关系。用户的有效权限是所有启用角色的并集。
`super_admin` 是明确的内置角色规则：拥有所有启用的 page/action，不依赖
`sys_role_menu` 中的行。

## 6. 权限语义与计算

权限码使用冒号分段，例如：

```text
system:user:view
system:user:create
system:user:update
system:user:delete
```

权限常量定义在实际拥有该业务接口的模块中，不能集中到一个万能权限文件。

- page code 表示页面入口以及基础查看/列表访问；
- action code 表示新增、修改、删除或其他具体操作；
- 直接授予 action 时，自动获得其 page 和目录祖先的有效访问；
- 直接授予 page 不会获得该 page 下的任何 action；
- directory 只负责导航组织，不作为按钮权限返回；
- 菜单树向上遍历 `parent_id` 生成祖先，结果在内存中去重并排序；
- 发现循环父子链、失效父节点或类型关系损坏时，返回明确内部错误并拒绝
  生成访问快照。

角色权限管理阶段使用完整替换语义：提交最终的直接授权节点集合，后端在
一个 PostgreSQL 事务中校验、恢复、软删除和提交。取消目录或页面时，前端
树同步取消其后代，提交集合中不再包含这些节点。

## 7. 访问快照接口

接口：

```http
GET /api/v1/access
```

只要求登录，不要求额外 RBAC 权限。响应结构：

```json
{
  "roleCodes": ["registered_user", "ai_tester"],
  "menuTree": [
    {
      "code": "ai",
      "menuType": "directory",
      "path": "/ai",
      "viewKey": null,
      "titleKey": "navigation.ai",
      "icon": "Cpu",
      "children": []
    }
  ],
  "permissionCodes": [
    "ai:model:create",
    "ai:model:view"
  ]
}
```

协议规则：

- `menuTree` 只包含 directory/page，action 只进入 `permissionCodes`；
- `roleCodes`、`permissionCodes` 和所有 `children` 都是数组，不返回 `null`；
- 三类数组均排序并去重；
- `viewKey` 只在 page 上出现，directory/action 为 `null`；
- 后端返回的 `viewKey`、icon 必须是前端已知协议值；未知值是协议错误，
  不生成空路由或默认图标；
- 菜单标题只返回稳定的 `i18n_key`/`titleKey`，翻译由前端 locale 文件负责；
- `/access` 不返回数据库内部错误、SQL、堆栈或敏感数据。

## 8. 后端请求和鉴权流程

普通业务请求固定为：

```text
router
→ authenticate
→ requirePermission(模块权限常量)
→ handler
→ service
→ repository
→ model
→ PostgreSQL
```

权限在路由注册处显式声明，例如：

```go
users.GET("", authenticate, requirePermission(user.PermissionView), handler.List)
```

Handler 不访问 GORM，Service 不接收 `*gin.Context`，Repository 只负责
PostgreSQL。当前阶段每次鉴权直接查询 PostgreSQL，鉴权查询失败默认拒绝，
不使用旧快照放行。

## 9. 前端访问初始化和动态路由

Dashboard、Login、未来 Forgot Password 和错误页都是静态路由。Dashboard 是
受保护的固定定位页，绕过 RBAC 菜单配置。

登录成功或浏览器刷新时只初始化一次访问快照：

```text
恢复登录状态
→ 请求 /api/v1/access
→ 校验 DTO
→ 保存 access store
→ 生成侧边栏
→ 从 menuTree 提取 page
→ 通过显式 routeViews 映射 viewKey
→ 注册动态路由
→ router.replace(原目标地址)
```

点击菜单只做前端路由跳转，并请求目标页面自己的业务 API，不再次请求
`/access`。并发守卫共用一个加载 Promise，避免重复请求和时序竞争。

目录只用于侧边栏树，不注册 Vue 组件。page 路由统一注册在静态认证 layout
下面。动态路由完成前不放行目标业务地址；动态注册失败时保留静态 Dashboard，
不生成空白组件或静默 fallback。

前端 access store 使用具体类型，至少包含：

```text
roleCodes
menuTree
permissionCodes
status: idle | loading | ready | error
```

退出登录时清除上述状态并移除动态路由，防止账号切换后残留旧权限。

## 10. 错误和失败状态

- 无登录、Token 无效或过期：`401`，前端清理状态并返回登录页；
- 已登录但无权限：`403`，前端通知并停留当前页面；
- 参数错误、冲突和服务异常使用稳定 code/messageKey，不暴露内部 cause；
- `/access` 加载失败时，Dashboard 仍可显示，但动态菜单和路由不注册；
- 直接访问业务地址且 `/access` 失败时，跳转 Dashboard 并提示访问数据加载
  失败，不伪装成普通 `404`；
- 未知 viewKey、重复路径、循环父子关系等协议错误 fail closed；
- PostgreSQL、Redis 或其他依赖失败时返回显式错误，不返回假成功、空权限或
  默认权限。

## 11. 测试和迁移顺序

### 11.1 后端测试

必须测试语言协商、目录 key 一致性、错误 envelope、数据库约束、软删除过滤、
多角色并集、super_admin、直接授权到祖先推导、action 与兄弟 action 隔离、
禁用菜单、访问快照排序去重、协议损坏和鉴权失败关闭。

### 11.2 前端测试

必须测试登录/刷新只加载一次 `/access`、并发守卫复用 Promise、动态路由注册
时序、点击菜单不重复加载、未知 viewKey、access 失败 Dashboard、401/403
处理和退出后的权限清理。

### 11.3 迁移顺序

```text
sys_menu
→ sys_role
→ sys_user_role
→ sys_role_menu
→ 索引、外键和软删除约束
→ 后端 i18n
→ access 查询
→ 权限中间件
→ 前端 access store
→ 前端动态菜单和路由
```

生产环境不插入假菜单。集成测试使用事务内 fixture 并在测试结束回滚。

## 12. Redis 后续边界

一期不实现权限缓存。未来只在有性能证据时加入 Redis L2，缓存已经计算完成的
访问快照和鉴权结果。缓存身份包含 `userID + permissionVersion`，角色、菜单或
用户授权变化在同一事务中递增受影响用户的权限版本，使旧 key 自然失效。

Redis 不可用时回退 PostgreSQL；PostgreSQL 不可用时鉴权失败，绝不 fail open。
只有确认数据库访问成为瓶颈后，才考虑短 TTL 的进程级 L1；即使增加 L1，缓存
身份和失效规则也必须继续使用权限版本，不能依赖单实例本地过期。

## 13. 验收标准

完成后必须满足：

1. 未登录用户只能访问公共静态路由。
2. 登录或刷新时 `/access` 只请求一次，并能恢复原始业务地址。
3. 后端每个受保护接口都在路由处显式声明权限。
4. 直接授权节点可正确推导祖先菜单和页面访问，不能获得兄弟 action。
5. Dashboard 在 access 失败时仍可定位和显示，动态路由不会静默生成。
6. 中英文错误目录完整一致，响应 envelope 只包含 `code`、`data`、`message`。
7. 权限查询失败、协议数据损坏和依赖不可用时均明确失败且不放行。
8. 相关后端、前端和 PostgreSQL 集成测试通过。
