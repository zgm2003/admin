# Admin 用户管理与用户角色分配设计说明

## 1. 背景与目标

菜单管理、角色管理和角色菜单授权已经落地，但用户模块目前只有认证所需的
`sys_user` 模型与 Repository，没有完整的管理 API、动态菜单、前端页面和用户角色
分配入口。RBAC 因此还缺少最后一条管理链：管理员无法通过系统查看用户、启停用户、
修改用户名或调整一个用户拥有的多个角色。

本期补齐用户管理与用户角色分配，并完成 RBAC 主链联调。实现继续遵守线性数据流：

```text
Vue view
-> web/src/api/user.ts
-> /api/v1/users
-> authenticate
-> require permission
-> user handler
-> user service
-> user repository
-> PostgreSQL
```

目标：

- 提供用户分页列表、关键词搜索、状态筛选和角色筛选；
- 只允许管理员修改用户名，邮箱保持只读，密码不进入用户管理；
- 支持一个用户绑定多个角色，运行时权限继续取所有有效启用角色的并集；
- 提供用户启用、禁用、软删除和独立的角色分配操作；
- 禁用或删除用户时立即撤销 PostgreSQL 当前会话，并清理 Redis 当前会话指针；
- 后端独立执行自己账号和超级管理员保护，前端保护只改善交互；
- 保持数据库、Go、URL、JSON、TypeScript、页面目录和权限码中的核心命名一致；
- 完成后进入 RBAC 联调验收，再开始技术债回收和前端 UI 大改。

## 2. 已确认决策

- 后台不提供新增用户；
- 用户由现有注册流程、未来邮箱首次登录流程或 `bootstrap-admin` 创建；
- 用户管理只允许修改用户名，邮箱只读，密码不进入本模块；
- 每个未删除用户至少绑定一个未删除角色，允许同时拥有多个角色；
- 角色分配允许保留或新增禁用角色，但保存后的集合必须至少包含一个启用角色；
- 角色分配使用独立子资源和独立 Dialog，不与用户名编辑混合；
- 用户禁用或删除后立即撤销会话，重新启用后必须重新登录；
- 主动“踢出用户”不属于本期，后续由会话管理提供；
- 禁止管理员禁用、删除自己或修改自己的角色集合，允许修改自己的用户名；
- 只有当前拥有有效 `super_admin` 角色的操作者可以授予、移除或变更
  `super_admin` 用户；
- 系统始终至少保留一个启用且有效绑定 `super_admin` 的用户；
- 普通用户可以软删除，删除后不提供恢复入口；
- 软删除后原用户名和邮箱可以重新注册，新记录获得新用户 ID 和当前默认角色；
- 用户列表使用“统一关键词 + 启用状态 + 角色 + 分页”；
- 本期不实现 RBAC Redis 缓存、会话配置、会话列表、主动踢出或操作日志；
- PostgreSQL 是用户、角色关系、会话和权限的权威来源，Redis 只允许作为可失效
  加速层。这一原则由后续缓存与会话加固 spec 落地。

## 3. 非目标与硬约束

本期不包含：

- 管理员手工创建用户；
- 修改邮箱、密码或找回密码；
- 用户恢复、回收站或历史身份复活；
- 批量启停、批量删除、批量分配角色或用户导出；
- 用户详情、头像、手机号、性别、地址或个人资料扩展；
- 会话列表、会话统计、主动踢出或批量踢出；
- 登录日志和操作日志；
- RBAC Redis 缓存、权限版本或进程级缓存；
- 修改当前认证 Redis 策略或增加多前端会话配置；
- `AppDialog`、`AppTable`、通用搜索表单或 `usePagedList`；
- BaseService、BaseRepository、Manager、Factory、Adapter 或通用 CRUD 框架；
- UI 主题、颜色、密度和响应式专项整改。

本期不得为了后续功能增加空表、空字段、空缓存接口、空审计 Hook、兼容响应或
未使用抽象。前端按钮隐藏和禁用不构成安全边界，所有规则必须由后端重新校验。

## 4. 统一命名

核心业务词固定使用 `user`、`role` 和 `user role`，不得在不同层改称 `account`、
`member`、`group` 或其他同义词。

| 含义 | PostgreSQL | Go | JSON / TypeScript | URL / view |
| --- | --- | --- | --- | --- |
| 用户 | `sys_user` | `user.User` | `user` | `/users`、`system-users` |
| 角色 | `sys_role` | `role.Role` | `role` | `/roles` |
| 用户角色关系 | `sys_user_role` | `role.UserRole` | `roleIds` | `/users/:id/roles` |
| 启用状态 | `is_enabled` | `IsEnabled` | `isEnabled` | `/status` |
| 创建时间 | `created_at` | `CreatedAt` | `createdAt` | - |
| 更新时间 | `updated_at` | `UpdatedAt` | `updatedAt` | - |
| 软删除时间 | `deleted_at` | `DeletedAt` | 不返回 | - |

命名规则继续保持：

- PostgreSQL 使用 lower snake case；
- Go 导出名称使用 PascalCase，initialism 使用 `ID`；
- JSON 和 TypeScript 使用 lower camel case；
- URL 使用小写复数资源；
- 权限码使用冒号分段；
- i18n key 使用现有 lower camel case 命名。

## 5. 数据模型与约束

### 5.1 `sys_user`

本期不增加字段，继续使用现有模型：

| 字段 | 类型 | 规则 |
| --- | --- | --- |
| `id` | `BIGINT` | 主键、自增 |
| `username` | `VARCHAR(64)` | 非空，有效用户大小写不敏感唯一 |
| `email` | `VARCHAR(254)` | 非空，有效用户唯一，本期只读 |
| `password_hash` | `VARCHAR(255)` | 非空，不通过用户管理读写 |
| `is_enabled` | `SMALLINT` | 非空，`0=否`、`1=是` |
| `created_at` | `TIMESTAMPTZ` | 非空 |
| `updated_at` | `TIMESTAMPTZ` | 非空 |
| `deleted_at` | `TIMESTAMPTZ NULL` | 用户软删除状态 |

现有部分唯一索引继续作为数据库最终保护：

```sql
CREATE UNIQUE INDEX ux_sys_user_username_active
ON sys_user (lower(username))
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX ux_sys_user_email_active
ON sys_user (email)
WHERE deleted_at IS NULL;
```

用户软删除后，原用户名和邮箱可以用于创建全新的用户记录。新记录不继承旧记录的
ID、密码、角色关系或会话。

### 5.2 `sys_user_role`

本期不增加字段，继续使用现有模型和部分唯一索引：

```sql
CREATE UNIQUE INDEX ux_sys_user_role_active
ON sys_user_role (user_id, role_id)
WHERE deleted_at IS NULL;
```

有效关系定义为 `deleted_at IS NULL`。移除角色时软删除当前有效关系；以后重新绑定
同一角色时创建一条新关系，不修改或复活历史关系。每次关系写入必须显式维护
`created_at` 和 `updated_at`。

应用层保证每个未删除用户至少存在一个指向未删除角色的有效关系。角色分配保存时
还要求目标集合至少包含一个启用角色。角色之后被角色管理模块禁用时，关系继续保留，
该用户可能暂时没有任何运行时有效角色并因此无法通过认证；角色重新启用后关系恢复
生效。这是现有角色禁用语义，不属于用户数据损坏。

### 5.3 `sys_user_session`

会话表继续由认证模块定义，不增加 `deleted_at`。用户禁用或删除时，将目标用户所有
`revoked_at IS NULL` 的会话更新为同一 UTC 时间：

```text
revoked_at = operationTime
updated_at = operationTime
```

当前数据库存在单会话唯一索引，因此正常情况下最多影响一条。本操作使用“全部未撤销
会话”的条件，不依赖该数量，也不会在未来会话模型调整后留下旧会话。

## 6. 用户名规则归属

用户名规则必须与注册保持完全一致：

- 去除首尾空白；
- 3 至 64 个 Unicode 字符；
- 只允许 Unicode 字母、数字、下划线和连字符；
- 有效用户之间大小写不敏感唯一。

用户名规范化与校验由 `user` 模块提供一个具体 helper，认证注册流程和用户管理流程
共同调用。`auth` 模块继续负责邮箱与密码规则。不得复制两套用户名正则或引入通用
Validator 框架。

## 7. 内置菜单与权限码

在现有 `system` 目录下增加：

| code | type | parent | i18nKey | path | viewKey | icon | sort |
| --- | --- | --- | --- | --- | --- | --- | ---: |
| `system:user:list` | page | `system` | `navigation.systemUsers` | `/system/users` | `system-users` | `User` | 30 |
| `system:user:update` | action | `system:user:list` | `permission.userUpdate` | `NULL` | `NULL` | `NULL` | 10 |
| `system:user:status` | action | `system:user:list` | `permission.userStatus` | `NULL` | `NULL` | `NULL` | 20 |
| `system:user:delete` | action | `system:user:list` | `permission.userDelete` | `NULL` | `NULL` | `NULL` | 30 |
| `system:user:roles` | action | `system:user:list` | `permission.userRoles` | `NULL` | `NULL` | `NULL` | 40 |

`menu.EnsureBuiltin` 在 HTTP 监听前按现有规则创建或严格校验这些记录，不静默修正
已存在但字段损坏的内置菜单。

页面路由和前端协议同步注册：

```text
system-users -> web/src/views/system/users/index.vue
```

## 8. HTTP 路由与权限

```text
GET    /api/v1/users                      -> system:user:list
GET    /api/v1/users/role-options         -> system:user:list
PUT    /api/v1/users/:id                  -> system:user:update
PATCH  /api/v1/users/:id/status           -> system:user:status
DELETE /api/v1/users/:id                  -> system:user:delete
GET    /api/v1/users/:id/roles            -> system:user:roles
PUT    /api/v1/users/:id/roles            -> system:user:roles
```

静态路由 `/users/role-options` 必须在参数路由前注册。所有路由先认证，再检查明确的
权限码。Handler 从认证上下文读取当前操作者 `userID`，并传给 Service；Service
不能接收 `*gin.Context`。

## 9. API 契约

所有响应继续使用精确 envelope：

```json
{
  "code": 0,
  "data": {},
  "message": "ok"
}
```

请求 JSON 使用严格解码，拒绝未知字段、重复 JSON 值、尾随 JSON 和错误类型。
空请求体接口拒绝任何请求体。时间统一返回 RFC3339 UTC 字符串。

### 9.1 用户列表

```http
GET /api/v1/users?page=1&pageSize=20&keyword=alice&isEnabled=1&roleId=2
```

查询参数 allowlist：

- `page`：必填一次，正整数；
- `pageSize`：必填一次，1 至 100；
- `keyword`：可选一次，去除首尾空白，最多 254 个 Unicode 字符；
- `isEnabled`：可选一次，只能是 `0` 或 `1`；
- `roleId`：可选一次，正整数。

未知参数、重复参数和空的数值参数返回 400。关键词同时匹配 `username` 和 `email`，
其中 `%`、`_` 和反斜线按普通字符搜索。`roleId` 匹配用户任一条有效角色关系，角色
可以处于禁用状态。不存在的正整数角色 ID 返回合法空页，不猜测其他角色。

响应：

```json
{
  "list": [
    {
      "id": 7,
      "username": "alice",
      "email": "alice@example.com",
      "isEnabled": 1,
      "roles": [
        {
          "id": 2,
          "code": "registered_user",
          "name": "普通用户",
          "isEnabled": 1
        },
        {
          "id": 5,
          "code": "ai_tester",
          "name": "AI 测试员",
          "isEnabled": 0
        }
      ],
      "createdAt": "2026-08-20T01:00:00Z",
      "updatedAt": "2026-08-20T02:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 20
}
```

用户稳定按 `created_at ASC, id ASC` 排序；每个用户的角色稳定按 `code ASC, id ASC`
排序。多角色 JOIN 不得让同一用户在列表中重复。空列表必须返回 `[]`。

每个未删除用户必须至少有一条指向未删除角色的有效关系。有效关系指向缺失或已软
删除角色，或者用户没有任何有效角色关系时，返回明确数据状态损坏错误，不静默隐藏
关系或伪造默认角色。只有禁用角色仍是合法关系并正常返回。

### 9.2 角色筛选选项

```http
GET /api/v1/users/role-options
```

返回全部未软删除角色，包括禁用角色：

```json
{
  "roles": [
    {
      "id": 2,
      "code": "registered_user",
      "name": "普通用户",
      "isEnabled": 1
    }
  ]
}
```

稳定按 `code ASC, id ASC` 排序。该接口只服务用户列表筛选，不代替角色管理列表，
不支持分页、搜索或 mutation。

### 9.3 修改用户名

```http
PUT /api/v1/users/7
Content-Type: application/json

{
  "username": "alice_new"
}
```

成功返回服务端最终规范化结果：

```json
{
  "id": 7,
  "username": "alice_new",
  "updatedAt": "2026-08-20T03:00:00Z"
}
```

重复提交相同规范化用户名是幂等成功，不机械修改 `updated_at`。邮箱、密码、角色、
状态和其他未知字段全部拒绝。

### 9.4 更新用户状态

```http
PATCH /api/v1/users/7/status
Content-Type: application/json

{
  "isEnabled": 0
}
```

成功返回：

```json
{
  "id": 7,
  "isEnabled": 0
}
```

启用不创建角色、不恢复历史角色关系、不创建会话。用户重新启用后必须重新登录。
重复启用是幂等成功。重复禁用仍会重新执行“撤销全部未撤销会话 + 删除 Redis 指针”
的安全清理，便于上一次 Redis 删除失败后重试。

### 9.5 删除用户

```http
DELETE /api/v1/users/7
```

请求体必须为空。成功返回 `data: {}`。删除是软删除，不返回旧用户模型。

已存在的同 ID 软删除记录再次删除时，数据库不重复写入，但仍重新删除 Redis 当前
会话指针并返回幂等成功；从未存在过的 ID 返回用户不存在。这保证数据库已经提交、
Redis 删除失败后的请求可以安全重试。

### 9.6 查询用户角色

```http
GET /api/v1/users/7/roles
```

响应：

```json
{
  "user": {
    "id": 7,
    "username": "alice",
    "email": "alice@example.com",
    "isEnabled": 1
  },
  "roles": [
    {
      "id": 2,
      "code": "registered_user",
      "name": "普通用户",
      "isEnabled": 1
    },
    {
      "id": 5,
      "code": "ai_tester",
      "name": "AI 测试员",
      "isEnabled": 0
    }
  ],
  "roleIds": [2, 5]
}
```

`roles` 包含全部未软删除角色，`roleIds` 是目标用户当前有效关系中的角色 ID，按
数值升序、去重返回。有效关系指向已软删除或缺失角色时返回数据状态损坏错误。

### 9.7 保存用户角色

```http
PUT /api/v1/users/7/roles
Content-Type: application/json

{
  "roleIds": [2, 5]
}
```

`roleIds` 必须存在且为数组。每个元素必须是正整数；后端排序去重后再校验和保存。
空数组、只包含禁用角色、包含缺失角色或包含已软删除角色都返回明确错误。

成功返回：

```json
{
  "id": 7,
  "roleCount": 2
}
```

同一规范化角色集合重复保存是幂等成功，不重建关系、不修改 `updated_at`。

## 10. 操作者与超级管理员保护

### 10.1 操作者身份

用户 mutation 的 Service 方法必须同时接收：

```text
actorUserID
targetUserID
input
```

Service 从 PostgreSQL 验证操作者当前是否为未删除、启用且有效绑定启用
`super_admin` 的用户。不得信任前端 Access Store 中的 `roleCodes`。

### 10.2 自己账号保护

- 允许修改自己的用户名；
- 禁止禁用自己；
- 禁止删除自己；
- 禁止修改自己的角色集合；
- 以上规则对 `super_admin` 同样生效。

### 10.3 超级管理员目标保护

- 普通管理员可以查看超级管理员用户和角色关系；
- 普通管理员不能修改超级管理员用户的用户名、状态、角色集合或删除该用户；
- 只有当前 `super_admin` 可以操作另一个超级管理员用户；
- 只有当前 `super_admin` 可以给其他用户授予 `super_admin`；
- 只有当前 `super_admin` 可以从其他用户移除 `super_admin`；
- 权限码 `system:user:*` 本身不能绕过这些身份保护。

### 10.4 最后一个超级管理员

以下操作可能减少有效超级管理员数量：

- 禁用一个绑定 `super_admin` 的用户；
- 删除一个绑定 `super_admin` 的用户；
- 从用户角色集合中移除 `super_admin`。

所有用户 mutation 事务必须先使用 `FOR UPDATE` 锁定固定 `super_admin` 角色行，再
锁定目标用户和关系。这个固定顺序同时串行化操作者、目标用户的超级管理员身份判断，
防止普通目标在检查后被并发授予 `super_admin`，也避免不同 mutation 使用相反顺序
产生死锁。可能减少有效超级管理员数量的操作还要计算操作后的数量，结果必须至少为
1，否则返回冲突错误并回滚。

有效超级管理员用户同时满足：

```text
sys_user.deleted_at IS NULL
AND sys_user.is_enabled = 1
AND sys_user_role.deleted_at IS NULL
AND sys_role.code = 'super_admin'
AND sys_role.deleted_at IS NULL
AND sys_role.is_enabled = 1
```

即使两个管理员并发禁用、删除或移除不同超级管理员的角色，也不能同时通过“还剩
一个”的检查。

## 11. Repository 与事务语义

### 11.1 用户列表

Repository 分别执行 count 和 page query。角色筛选使用 `EXISTS` 或等价去重查询，
不得用普通 JOIN 造成总数和列表重复。角色集合可以使用 PostgreSQL 聚合或定向批量
查询，但必须得到严格类型、稳定排序和完整关系，不执行 N+1 查询。

### 11.2 用户名更新

事务：

1. 锁定固定 `super_admin` 角色行；
2. 锁定目标有效用户；
3. 查询操作者和目标的超级管理员身份；
4. 普通管理员操作超级管理员目标时拒绝；
5. 相同规范化用户名直接返回当前值；
6. 更新用户名和 `updated_at`；
7. 映射 `ux_sys_user_username_active` 并发冲突；
8. 提交。

### 11.3 角色分配

事务：

1. 锁定固定 `super_admin` 角色行；
2. 锁定目标有效用户；
3. 拒绝操作者修改自己的角色集合；
4. 锁定全部未软删除角色；
5. 校验请求 ID 全部存在且未软删除；
6. 校验规范化集合至少包含一个启用角色；
7. 查询并严格校验当前有效 `sys_user_role`；
8. 根据当前集合和目标集合判断是否涉及授予、移除或操作 `super_admin` 用户；
9. 执行操作者超级管理员保护；
10. 若移除 `super_admin`，校验操作后仍至少一个有效超级管理员；
11. 使用同一 UTC 时间软删除移除关系；
12. 为新增角色创建新的 `sys_user_role`；
13. 只有集合变化时更新用户 `updated_at`；
14. 提交。

不变关系不重建。任何一步失败都回滚整个关系变更。

角色变更不撤销会话，不请求 `/api/v1/access`，也不强制刷新目标用户页面。后端权限
查询当前直接读取 PostgreSQL，因此接口权限在事务提交后立即按新角色集合计算；目标
用户手动刷新浏览器后，前端菜单、路由和按钮按新的 `/api/v1/access` 结果重建。

### 11.4 禁用用户

PostgreSQL 事务：

1. 锁定固定 `super_admin` 角色行；
2. 锁定目标有效用户；
3. 拒绝禁用自己；
4. 查询操作者和目标的超级管理员身份；
5. 普通管理员操作超级管理员目标时拒绝；
6. 若目标是有效超级管理员，校验操作后仍至少一个有效超级管理员；
7. 更新 `is_enabled=0` 和 `updated_at`；
8. 撤销目标用户全部未撤销会话；
9. 提交。

提交成功后删除 Redis：

```text
auth:current-session:<targetUserID>
```

PostgreSQL 事务失败时不得删除 Redis 指针。Redis 删除失败时不能回滚已经提交的
PostgreSQL，也不能返回假成功；接口返回依赖错误。此时用户状态和会话撤销已经是安全
真相，旧请求会被 PostgreSQL 状态拒绝。管理员可以重复提交禁用来重试 Redis 清理。

### 11.5 启用用户

事务先锁定固定 `super_admin` 角色行，再锁定目标有效用户，执行自己与超级管理员目标
保护，然后只更新 `is_enabled=1` 和 `updated_at`。启用不创建角色、不恢复关系、不
创建会话，也不写入 Redis。目标必须保留至少一个有效关系；当前角色可以全部处于
禁用状态，此时启用成功，但用户仍无法登录，直到至少一个角色被启用或重新分配。

### 11.6 删除用户

PostgreSQL 事务：

1. 锁定固定 `super_admin` 角色行；
2. Unscoped 查询并锁定目标用户；
3. 从未存在时返回用户不存在；
4. 已软删除时不重复写数据库，进入 Redis 清理；
5. 拒绝删除自己；
6. 查询操作者和目标的超级管理员身份；
7. 普通管理员操作超级管理员目标时拒绝；
8. 若目标是有效超级管理员，校验操作后仍至少一个有效超级管理员；
9. 使用同一 UTC 时间软删除全部有效 `sys_user_role`；
10. 撤销目标用户全部未撤销会话；
11. 更新用户 `updated_at` 和 `deleted_at`；
12. 提交。

提交成功或目标已软删除后，删除 Redis 当前会话指针。Redis 删除失败返回依赖错误，
再次 DELETE 同一个已软删除 ID 会重试 Redis 删除并幂等成功。删除不物理删除用户、
关系或会话历史。

## 12. Redis 与权威数据边界

本期只清理现有认证 Redis key，不改变认证读取逻辑，不实现权限缓存。

当前会话 Redis key 只能有一处构造定义。该具体函数移动到 `user` 模块并按用户 ID
生成 `auth:current-session:<userID>`；现有 `auth.Service` 和新增 `user.Service` 共同
调用，不能各自拼接字符串。`user.Service` 只依赖现有 Redis client 所满足的最小
`Delete(context.Context, string) error` 测试边界，不增加 Adapter 或缓存 Manager。

权威关系明确为：

```text
用户、角色、用户角色关系 -> PostgreSQL
会话有效、撤销和过期     -> PostgreSQL
当前会话 Redis 指针       -> 可失效加速数据
RBAC 权限                 -> 当前直接查询 PostgreSQL
```

后续缓存回收必须以 PostgreSQL 为保真层：Redis 权限缓存未命中或故障时查询
PostgreSQL 权威数据；PostgreSQL 不可用时失败关闭，不能使用旧权限或假成功。认证会话
Redis 故障时安全回到 PostgreSQL 的策略也由后续认证会话加固 spec 设计，本期不混改。

## 13. 错误与 i18n

用户模块固定使用以下独立稳定错误码：

| HTTP | code | 场景 |
| ---: | ---: | --- |
| 404 | `16000` | 用户不存在 |
| 409 | `16001` | 用户名冲突 |
| 409 | `16002` | 禁止对自己执行该操作 |
| 403 | `16003` | 普通管理员禁止操作超级管理员目标 |
| 409 | `16004` | 操作会移除最后一个有效超级管理员 |
| 400 | `16005` | 用户角色集合无效 |
| 404 | `16006` | 请求角色不存在或已软删除 |
| 500 | `16007` | 用户或用户角色数据状态损坏 |

通用 JSON、查询参数和 ID 错误继续使用项目 Invalid Request；PostgreSQL 和 Redis I/O
错误继续映射 Dependency Unavailable。中英文目录必须具有完全一致的 key 和参数。

公开错误不得包含 SQL、约束细节、密码摘要、Token、Cookie、Authorization、DSN 或
内部堆栈。服务端日志保留 request ID、actor user ID、target user ID、操作名和内部
cause，不记录敏感凭据。

## 14. 前端设计

### 14.1 文件与数据流

```text
web/src/views/system/users/index.vue
-> web/src/api/user.ts
-> web/src/utils/request.ts
-> Go API
```

外部响应在 `web/src/api/user.contract.ts` 中从 `unknown` 严格校验。DTO、查询、Props、
Emits 和页面状态使用明确类型，禁止显式 `any`、`as any`、`any[]`、
`Record<string, any>` 或 TypeScript 错误抑制。

### 14.2 页面结构

一期继续直接使用 Element Plus：

```text
搜索区
  关键词
  启用状态
  角色
  查询 / 重置 / 刷新

表格
  ID
  用户名
  邮箱
  角色标签
  状态
  创建时间
  更新时间
  操作

分页
```

角色标签展示角色名称，Tooltip 或相邻文本可展示角色编码；禁用角色使用明确的“已
禁用”状态样式，不隐藏。页面加载失败保留明确错误态，不展示伪造空列表。

### 14.3 权限显示

| 页面行为 | 前端权限 |
| --- | --- |
| 查看、筛选、分页、刷新 | `system:user:list` |
| 编辑用户名 | `system:user:update` |
| 启用、禁用 | `system:user:status` |
| 删除 | `system:user:delete` |
| 查看和保存角色 | `system:user:roles` |

无权限的操作不渲染。前端根据当前 Auth Store 用户 ID 和 Access Store `roleCodes`
提前禁用自己账号或超级管理员目标的危险操作并显示原因，但后端必须独立重验。

### 14.4 编辑用户 Dialog

- 用户名可编辑；
- 邮箱只读展示；
- 保存失败保留输入和 Dialog；
- 保存成功关闭 Dialog 并刷新当前列表；
- 修改当前登录用户自己的用户名成功后，使用服务端响应更新 Auth Store 中的
  `username`，Header 立即同步；
- 不重新请求 `/api/v1/access`。

### 14.5 分配角色 Dialog

分配角色使用扁平多选列表，不使用树或级联组件。每项显示角色名称、编码和启用状态。
提供全选与清空；全选覆盖全部未软删除角色，包括禁用角色。清空是允许的编辑状态，
但保存按钮提示并拒绝提交，因为结果没有启用角色。

- 禁用角色可勾选、取消和保存；
- 非 `super_admin` 操作者看到 `super_admin`，但对应选择不可更改；
- 自己账号不进入角色 Dialog；
- 加载失败显示错误态，不伪造空角色集合；
- 保存失败保留当前选择和 Dialog；
- 保存成功关闭 Dialog 并刷新当前列表；
- 角色 mutation 不请求 `/api/v1/access`、不热更新路由、不刷新浏览器。

### 14.6 确认与滚动

启停和删除使用 Element Plus 确认框：

- 禁用明确说明用户会立即下线，重新启用后必须重新登录；
- 删除明确说明用户、角色关系和旧会话不会恢复，原邮箱可以重新注册为新账号；
- 操作超级管理员目标时明确显示身份影响；
- 前端提示不替代后端并发保护。

Main 继续是页面唯一纵向滚动容器。页面不得撑出 body 全局滚动条。Dialog teleport
到 `document.body`，其 body 自己滚动，header/footer 不被长角色列表推离视口。

## 15. 操作日志后续边界

本期不建操作日志表或空 Hook。RBAC 验收后的审计切片至少覆盖：

```text
user.username.update
user.status.update
user.roles.update
user.delete
role.*
menu.*
session.revoke
```

每条审计记录至少包含操作者、目标资源类型与 ID、动作、成功状态、结构化变更摘要、
client IP、request ID 和发生时间。角色变化记录新增和移除的角色 ID，不只保存整个
HTTP body。禁止记录密码、密码摘要、Access Token、Refresh Token、Cookie、
Authorization Header、DSN 或完整认证请求体。

旧项目 `operationLog` 页面只作为后续筛选、列表和详情交互参考，不复制其完整请求与
响应落库策略。

## 16. 测试策略

### 16.1 后端 Repository 与 PostgreSQL

使用真实 PostgreSQL 事务测试，至少覆盖：

- 用户分页、稳定排序、关键词、状态和角色筛选；
- `%`、`_` 和反斜线按普通字符搜索；
- 多角色用户的 count 和 list 不重复；
- 角色列表和用户角色稳定排序；
- 无角色关系、缺失角色和已软删除角色关系返回数据损坏；
- 禁用角色关系正常返回；
- 用户名更新、大小写冲突和软删除后复用；
- 角色关系最小差异、软删除、新增和不变集合；
- 关系写入失败完整回滚；
- 禁用用户同事务更新状态并撤销全部当前会话；
- 删除用户同事务软删除关系、用户并撤销会话；
- 任一步失败完整回滚；
- 已删除用户再次删除的幂等数据库行为。

### 16.2 后端 Service、Handler 与路由

至少覆盖：

- 用户名复用认证中的同一规范化规则；
- 修改自己用户名允许；
- 禁用、删除和分配自己角色拒绝；
- 普通管理员修改超级管理员用户名、状态、角色或删除均拒绝；
- 只有超级管理员可以授予或移除 `super_admin`；
- 两个并发 mutation 不能移除最后一个有效超级管理员；
- 角色 ID 排序去重；
- 禁用角色允许保存；
- 空集合和只有禁用角色的集合拒绝；
- 重复禁用重试会话和 Redis 清理；
- Redis 删除失败时 PostgreSQL 状态已安全提交并返回依赖错误；
- 已删除 ID 再次 DELETE 重试 Redis 并幂等成功；
- 未知 ID 返回 404；
- 查询 allowlist、必填分页、URL ID、严格 JSON 和空 body；
- 每条路由绑定准确权限；
- 中英文错误 key 和参数完全一致。

完整验证：

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

### 16.3 前端

至少覆盖：

- 用户内置菜单标题、view 和 icon 注册；
- 分页、用户、角色和 mutation 响应严格解析；
- 必填分页参数、关键词、状态和角色请求；
- 初次加载、查询、重置、分页和刷新；
- 多角色及禁用角色标签；
- 五个权限分别控制对应行为；
- 自己账号和超级管理员目标的按钮保护；
- 编辑用户名 Dialog 和邮箱只读；
- 角色 Dialog 加载、全选、清空、禁用角色和 `super_admin` 选择保护；
- 保存失败保留表单、选择和 Dialog；
- 修改自己用户名成功后更新 Auth Store；
- mutation 不请求 `/api/v1/access`、不调用 `window.location.reload()`；
- 页面与 Dialog 滚动边界；
- 中英文文案完整；
- 业务 TypeScript 无 forbidden any。

完整验证：

```powershell
cd D:\admin\web
pnpm vitest run
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src -g "*.ts" -g "*.vue"
```

## 17. RBAC 浏览器联调

```text
超级管理员给普通用户分配两个角色
-> 普通用户手动刷新浏览器
-> 菜单和按钮是两个启用角色权限并集

管理员移除一个角色
-> 后端下一次接口权限检查立即读取 PostgreSQL 新关系
-> 用户手动刷新后前端菜单、路由和按钮同步

管理员禁用用户
-> PostgreSQL 用户状态改变并撤销会话
-> Redis 当前会话指针删除
-> 旧 Access 和 Refresh 均不可继续使用
-> 重新启用后必须重新登录

管理员删除用户
-> 用户、角色关系软删除，会话撤销
-> 旧账号和会话失效
-> 原邮箱重新注册得到新 user ID 和当前默认角色
```

联调同时验证普通管理员不能通过直接构造 HTTP 请求操作自己、修改超级管理员目标或
授予 `super_admin`。

## 18. 技术债与后续顺序

本期完成后施工顺序固定为：

```text
用户管理与用户角色分配
-> RBAC 完整联调验收
-> 技术债回收
-> 前端 UI 与格式基线大改
```

技术债回收必须拆成有明确 spec 的切片：

| 技术债 | 回收原则 |
| --- | --- |
| RBAC 每次读取 PostgreSQL | PostgreSQL 权威，Redis 缓存完整用户 Access Snapshot；未命中或 Redis 故障读取 PostgreSQL，不返回旧权限 |
| 认证当前会话 Redis 强依赖 | 单独设计以 PostgreSQL 会话为权威的安全读取和 Redis 加速语义，支持未来会话配置与多个前端 |
| 无操作日志 | 为用户、角色、菜单和会话安全动作设计结构化审计，不记录敏感数据 |
| 页面直接使用 `el-dialog` | 对角色页和用户页真实重复设计无业务能力的 `AppDialog` |
| 页面直接使用 `el-table`、`el-pagination` | 对两个真实分页页面设计无请求能力的 `AppTable` |
| 分页、筛选、加载状态重复 | 基于真实重复评估 `usePagedList` 和搜索组件，不预设万能 CRUD |
| role/user 后端 CRUD 相似 | 做机械重复审查，只提取具体纯函数或协议 helper，禁止 BaseService/BaseRepository |

不得增加进程级权限缓存。Redis 权限缓存的 key、permission version、角色/菜单/用户
mutation 失效范围、多实例一致性、故障日志和指标必须由缓存 spec 明确定义。

技术债验证完成后再做 UI 专项整改。UI 整改统一颜色、间距、字号、表格密度、搜索区、
工具栏、分页、Dialog、加载/空/错误状态、滚动、响应式、明暗主题以及前端代码格式，
不同时改 RBAC 算法和后端数据流。

## 19. 预期代码边界

后端主要修改或新增：

```text
server/internal/module/user/
  model.go
  protocol.go
  session_pointer.go
  errors.go
  request.go
  response.go
  repository.go
  service.go
  handler.go
  route.go
  *_test.go

server/internal/module/menu/
  builtin.go
  protocol.go
  related tests

server/internal/module/auth/
  username validation reuse and related tests

server/internal/shared/i18n/
  catalog.go
  catalog_test.go

server/cmd/api/
  main.go
  main_test.go
```

前端主要修改或新增：

```text
web/src/access/
  menu-title-keys.ts
  route-views.ts
  related tests

web/src/api/
  user.contract.ts
  user.contract.test.ts
  user.ts
  user.test.ts

web/src/i18n/messages/
  zh-CN.ts
  en-US.ts

web/src/store/
  auth.ts
  auth.test.ts

web/src/views/system/users/
  index.vue
  index.test.ts
```

页面业务状态继续留在 `views/system/users/index.vue`。只有文件规模或测试隔离出现真实
问题时，才在该页面目录内拆分具体组件；本期不创建全局 CRUD 页面框架。

## 20. 验收标准

1. `super_admin` 能通过动态侧边栏进入 `/system/users`；
2. 用户列表严格分页，支持关键词、状态和角色筛选；
3. 多角色用户只显示一行，角色标签和禁用状态完整；
4. 用户名可修改，邮箱只读，后台不存在新增用户和密码入口；
5. 一个用户可以绑定多个角色，保存集合至少包含一个启用角色；
6. 禁用角色可保留绑定，运行时权限只取启用角色并集；
7. 自己账号不能被禁用、删除或修改角色；
8. 普通管理员不能操作超级管理员目标或授予 `super_admin`；
9. 并发 mutation 不能移除最后一个有效超级管理员；
10. 禁用和删除用户会在 PostgreSQL 中撤销会话并删除 Redis 当前指针；
11. 用户重新启用后必须重新登录；
12. 删除是软删除，旧关系和会话不恢复，原邮箱可注册为新用户；
13. 所有 API 使用严格 DTO、稳定 envelope、明确权限码和双语错误；
14. 所有前端外部数据严格校验，业务 TypeScript 不出现 forbidden any；
15. mutation 不主动刷新 `/api/v1/access` 或浏览器；
16. Main 和 Dialog 滚动边界正确，不出现 body 全局滚动条；
17. 后端和前端完整测试、构建及仓库 hygiene 检查通过；
18. RBAC 浏览器联调通过后，项目按“技术债回收 -> UI 与格式基线大改”继续施工。
