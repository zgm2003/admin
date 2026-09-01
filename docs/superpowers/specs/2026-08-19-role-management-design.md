# Role Management Phase One Design

> 历史设计说明（2026-09-01）：所有 page 菜单入口必须使用 `:view`；列表、详情和写接口使用
> 独立 action permission，页面权限不得自动派生 API 权限。本文中的 `system:role:*` 是迁移前
> 实现记录；早期“不实现进程级缓存”的范围由最新三层 Access 缓存基线替代。

## 1. 目标

本期在现有认证、访问快照和菜单管理基础上，落地角色管理与角色授权，使管理员可以显式维护角色资料、角色状态、默认角色和角色菜单权限。

数据流继续保持线性：

```text
frontend view
-> web/src/api/role.ts
-> web/src/utils/request.ts
-> /api/v1/roles
-> authentication middleware
-> permission middleware
-> role Handler
-> role Service
-> role Repository
-> PostgreSQL
```

角色资料和角色授权是两条独立数据流，使用不同接口和事务。不得把角色字段、状态和授权合并成一个万能保存入口。

## 2. 本期范围

本期包含：

- 角色分页列表、关键词搜索和启用状态筛选；
- 创建自定义角色；
- 修改自定义角色名称；
- 单个角色启用和禁用；
- 切换系统默认角色；
- 单个角色软删除；
- 查询和保存单个角色的菜单权限；
- 角色管理核心菜单和权限节点的幂等初始化；
- 角色相关后端中英文错误和前端双语文案；
- `/system/roles` 动态页面及 `system-roles` 视图注册；
- Element Plus 角色资料 Dialog、角色授权 Dialog、表格和分页；
- 后端真实 PostgreSQL 事务测试及前端严格契约和页面交互测试。

## 3. 非目标和硬约束

本期不包含：

- 用户管理和用户角色分配；
- 批量删除、批量启停或批量授权；
- 复制角色；
- 给角色批量分配用户；
- 本角色切片不单独实现 Redis 权限缓存；统一使用最新 Access 三层缓存基线；
- 本角色切片不单独实现 Access 缓存；统一遵守最新 Redis version 门控和有界进程缓存基线；
- 在线用户强制刷新或强制重新登录；
- 操作审计日志；
- `AppDialog`、`AppTable`、通用搜索表单或分页列表组合式函数；
- 视觉 UI 专项整改；
- 通用 CRUD Handler、BaseService、BaseRepository、Manager、Factory 或 Adapter。

本期不得为了后续用户管理或通用 CRUD 预留空字段、空接口、运行时注册器、兼容响应或未使用抽象。

权限变更在 PostgreSQL 中立即生效。受影响用户当前页面不主动刷新；用户手动刷新浏览器后重新请求 `/api/v1/access`，侧边栏、路由和按钮权限才按新结果重建。角色 mutation 不得主动调用 `/api/v1/access`、热更新路由或调用 `window.location.reload()`。

## 4. 继承的 RBAC 契约

本期直接继承菜单管理设计已经确认的权限语义，不重新定义第二套算法：

- `directory` 只组织菜单树，不直接写入 `sys_role_menu`；
- 直接授予 page，只获得该 page 的基础访问权限，不自动获得 action；
- 直接授予 action，运行时向上推导对应 page 和全部 directory 祖先；
- 同时提交 page 及其 action 时，page 是冗余直接授权，保存前移除；
- page 没有 action 时，page 本身仍是完整的基础权限；
- 禁用菜单保留角色授权但暂时不生效；
- 菜单重新启用后，原授权恢复生效；
- 菜单软删除时，由菜单模块在同一事务软删除关联授权；
- `super_admin` 通过后端固定规则拥有全部权限，不依赖 `sys_role_menu`。

角色模块只负责验证并保存直接授权集合。运行时权限并集、祖先推导和 `super_admin` 旁路继续由现有 access 模块负责。

## 5. 数据模型

### 5.1 `sys_role`

继续使用现有表和字段，不增加 `is_builtin`、`role_type` 或其他状态：

| 字段 | 约束 | 说明 |
| --- | --- | --- |
| `id` | `BIGINT PRIMARY KEY` | 数据库身份 |
| `code` | `VARCHAR(64) NOT NULL` | 稳定技术标识 |
| `name` | `VARCHAR(64) NOT NULL` | 管理端展示名称 |
| `is_default` | `SMALLINT NOT NULL` | `0=否`、`1=是` |
| `is_enabled` | `SMALLINT NOT NULL` | `0=否`、`1=是` |
| `created_at` | `TIMESTAMPTZ NOT NULL` | 创建时间 |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | 最后修改时间 |
| `deleted_at` | `TIMESTAMPTZ NULL` | 软删除时间 |

数据库约束：

```sql
CREATE UNIQUE INDEX ux_sys_role_code_active
ON sys_role (code)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX ux_sys_role_name_active
ON sys_role (name)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX ux_sys_role_default_active
ON sys_role (is_default)
WHERE is_default = 1 AND deleted_at IS NULL;
```

现有 Yes/No CHECK 继续要求 `is_default`、`is_enabled` 只能为 `0` 或 `1`。数据库索引保证最多一个有效默认角色，Service 和启动校验保证业务上恰好存在一个已启用默认角色。

角色编码规则固定为：

```text
^[a-z][a-z0-9_]{2,63}$
```

编码由管理员创建时填写，创建后永久不可修改。角色名称去除首尾空格后必须包含 1 至 64 个 Unicode 字符。有效角色的编码和名称都不可重复；角色软删除后允许复用原编码和名称，但旧角色、旧关系和旧授权不会自动转移到新角色。

### 5.2 `sys_user_role`

本期不改变表结构。角色列表的 `userCount` 和删除保护统计以下绑定：

- `sys_user_role.deleted_at IS NULL`；
- 对应 `sys_user.deleted_at IS NULL`；
- 用户即使被禁用仍然计入绑定数量。

这保证禁用用户不会让角色被意外删除。后续用户软删除时，用户管理模块必须在同一事务软删除其有效 `sys_user_role`，该规则由用户管理 spec 负责。

角色删除不物理删除或迁移任何历史 `sys_user_role`。

### 5.3 `sys_role_menu`

本期不改变表结构。角色列表的 `permissionCount` 统计当前有效的直接授权，包括指向禁用菜单的授权，不计算运行时推导出的祖先节点。

有效直接授权满足：

- `sys_role_menu.deleted_at IS NULL`；
- 对应 `sys_menu.deleted_at IS NULL`；
- 对应菜单类型只能是 page 或 action。

## 6. 数据库约束归属

角色模块正式落地后，`sys_role`、`sys_user_role` 的索引和外键由 `role.EnsureSchema` 负责：

- `ux_sys_role_code_active`；
- `ux_sys_role_name_active`；
- `ux_sys_role_default_active`；
- `ux_sys_user_role_active`；
- `fk_sys_user_role_user`；
- `fk_sys_user_role_role`。

现有认证 Schema 中对应定义迁移到 role 模块，认证模块继续只负责用户、会话和认证直接拥有的约束。迁移必须保持 DDL 幂等，不删除已有索引或外键，不引入 Schema Manager 或通用 migration 框架。

## 7. 系统角色与默认角色

### 7.1 `super_admin`

`super_admin` 是系统身份锚点：

- 固定编码为 `super_admin`；
- 固定名称为 `超级管理员`；
- 必须始终启用；
- 永远不能成为默认注册角色；
- 不可改名、禁用或删除；
- 不显示角色授权入口；
- 授权查询和保存接口都明确拒绝；
- 权限继续由 access 模块的固定旁路规则提供。

### 7.2 `registered_user`

`registered_user` 是初始默认角色，但不是永久唯一可选默认角色：

- 固定编码为 `registered_user`；
- 固定名称为 `普通用户`；
- 不可改名或删除；
- 可以查看和修改菜单授权；
- 初次初始化时 `is_default=1`、`is_enabled=1`；
- 管理员可以把默认身份切换到其他符合条件的角色；
- 不再是默认角色后允许禁用；
- 以后重新设为默认前必须先启用。

### 7.3 自定义角色

自定义角色：

- 创建时由管理员填写编码和名称；
- 创建后编码只读，名称可修改；
- 创建时固定 `is_default=0`、`is_enabled=1`；
- 允许零权限；
- 可启用、禁用、设为默认、删除和授权；
- 成为当前默认角色后不得禁用或删除。

### 7.4 默认角色切换

默认角色用于邮箱首次登录或注册时分配初始角色。切换默认角色：

- 目标必须存在、未删除且已启用；
- 目标不能是 `super_admin`；
- 目标可以是零权限角色；
- 只影响切换之后创建的新用户；
- 不迁移、增加或删除任何已有用户的角色关系；
- 同目标重复设置是幂等成功。

切换事务必须锁定全部有效角色，按照稳定 ID 顺序取得写锁，然后清除旧默认并设置新默认。任何一步失败全部回滚，事务结束时必须恰好有一个已启用默认角色。

## 8. 系统角色启动校验

API 在 HTTP 监听前执行系统角色初始化和校验。

首次启动：

- 创建 `super_admin`，`is_default=0`、`is_enabled=1`；
- 创建 `registered_user`，`is_default=1`、`is_enabled=1`。

“首次启动”严格定义为两个固定编码在 Unscoped 查询中都没有任何历史记录，并在同一事务内创建两个角色。只存在其中一个固定角色不属于首次启动，必须按数据不完整使启动失败，不得自动补建另一个角色。

后续启动：

- 使用 Unscoped 查询检测两个固定编码的全部历史记录；
- 任一固定编码不存在、重复或包含软删除历史时启动失败；
- 校验两个角色的固定编码和固定名称；
- 校验 `super_admin` 始终启用且不是默认角色；
- 不把管理员选择的新默认角色改回 `registered_user`；
- 不把已经允许禁用的非默认 `registered_user` 强制启用；
- 校验全表恰好存在一个未删除且已启用的默认角色；
- 默认角色为 `super_admin`、被禁用或状态值非法时启动失败。

启动校验发现损坏时返回明确错误，不自动改名、清除默认、恢复删除记录或猜测替代角色。

## 9. 核心角色管理菜单

角色管理页面挂在现有 `system` directory 下，与菜单管理并列：

| code | type | parent code | i18nKey | path | viewKey | icon | sortOrder |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `system:role:list` | page | `system` | `navigation.systemRoles` | `/system/roles` | `system-roles` | `UserFilled` | 20 |
| `system:role:create` | action | `system:role:list` | `permission.roleCreate` | `NULL` | `NULL` | `NULL` | 10 |
| `system:role:update` | action | `system:role:list` | `permission.roleUpdate` | `NULL` | `NULL` | `NULL` | 20 |
| `system:role:status` | action | `system:role:list` | `permission.roleStatus` | `NULL` | `NULL` | `NULL` | 30 |
| `system:role:default` | action | `system:role:list` | `permission.roleSetDefault` | `NULL` | `NULL` | `NULL` | 40 |
| `system:role:delete` | action | `system:role:list` | `permission.roleDelete` | `NULL` | `NULL` | `NULL` | 50 |
| `system:role:authorize` | action | `system:role:list` | `permission.roleAuthorize` | `NULL` | `NULL` | `NULL` | 60 |

这些节点进入现有菜单模块的核心菜单定义，继承核心节点保护、幂等初始化、双语标题注册和前端 view/icon 注册规则。本期不创建第二套角色菜单初始化器。

## 10. HTTP API

### 10.1 路由与权限

```text
GET    /api/v1/roles                  -> system:role:list
POST   /api/v1/roles                  -> system:role:create
PUT    /api/v1/roles/:id              -> system:role:update
PATCH  /api/v1/roles/:id/status       -> system:role:status
PATCH  /api/v1/roles/:id/default      -> system:role:default
DELETE /api/v1/roles/:id              -> system:role:delete
GET    /api/v1/roles/:id/permissions  -> system:role:authorize
PUT    /api/v1/roles/:id/permissions  -> system:role:authorize
```

全部路由先认证，再执行权限中间件，最后进入 role Handler。Handler 只解析和校验 HTTP，不直接访问 GORM。

### 10.2 分页列表

请求：

```http
GET /api/v1/roles?page=1&pageSize=20&keyword=ai&isEnabled=1
```

查询参数：

| 参数 | 必填 | 规则 |
| --- | --- | --- |
| `page` | 是 | 大于或等于 1 |
| `pageSize` | 是 | 1 至 100 |
| `keyword` | 否 | TrimSpace 后最多 64 个 Unicode 字符 |
| `isEnabled` | 否 | 只能为 `0` 或 `1` |

`page`、`pageSize` 缺失或越界时返回 400，不猜默认值。未知查询参数返回 400。关键词以转义 `%`、`_` 后的字面子串同时匹配 `code` 和 `name`，不让用户输入隐式改变成通配表达式。

Service 把同一组过滤条件交给 Repository 的 COUNT 和分页查询。列表按照 `created_at ASC, id ASC` 稳定排序。

响应复用现有 `pagination.Result[RoleListItem]`：

```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 2,
        "code": "registered_user",
        "name": "普通用户",
        "isDefault": 1,
        "isEnabled": 1,
        "userCount": 12,
        "permissionCount": 3,
        "createdAt": "2026-08-19T02:00:00Z",
        "updatedAt": "2026-08-19T03:00:00Z"
      }
    ],
    "total": 2,
    "page": 1,
    "pageSize": 20
  },
  "message": "ok"
}
```

`list` 必须是数组；无数据返回 `[]`，不能返回 `null`。时间使用 UTC RFC3339Nano 字符串。数量字段必须为大于或等于 0 的整数。

### 10.3 创建角色

```json
{
  "code": "ai_tester",
  "name": "AI 测试员"
}
```

请求只能包含 `code`、`name`。创建成功返回 HTTP 201：

```json
{
  "code": 0,
  "data": {"id": 8},
  "message": "ok"
}
```

### 10.4 修改角色

```json
{
  "name": "AI 测试工程师"
}
```

修改请求只接受 `name`。不存在修改 `code`、`isDefault` 或 `isEnabled` 的兼容字段。成功返回 `data: {}`。

### 10.5 修改状态

```json
{
  "isEnabled": 0
}
```

`isEnabled` 必须显式存在且只能为 `0` 或 `1`。成功返回：

```json
{
  "code": 0,
  "data": {"id": 8, "isEnabled": 0},
  "message": "ok"
}
```

### 10.6 设置默认角色

`PATCH /api/v1/roles/:id/default` 要求空请求体，任何 JSON 字段都返回 400。成功返回：

```json
{
  "code": 0,
  "data": {"id": 8, "isDefault": 1},
  "message": "ok"
}
```

### 10.7 删除角色

`DELETE /api/v1/roles/:id` 要求空请求体。成功返回 `data: {}`。删除为软删除，不返回被删除模型。

### 10.8 查询角色授权

响应只返回授权页面需要的字段：

```json
{
  "code": 0,
  "data": {
    "role": {
      "id": 8,
      "code": "ai_tester",
      "name": "AI 测试员",
      "isDefault": 0,
      "isEnabled": 1
    },
    "menuTree": [
      {
        "id": 1,
        "parentId": null,
        "menuType": "directory",
        "code": "system",
        "i18nKey": "navigation.system",
        "isEnabled": 1,
        "children": []
      }
    ],
    "menuIds": [12, 15]
  },
  "message": "ok"
}
```

`menuTree` 包含全部未软删除菜单，包括禁用节点；同级排序继续使用 `sort_order ASC, code ASC`。每个 `children` 必须为数组。`menuIds` 是排序、去重后的当前有效 page/action 直接授权 ID，不包含 directory 和运行时推导祖先。

`super_admin` 的授权查询返回明确冲突错误，不返回伪造的全选树。

### 10.9 保存角色授权

```json
{
  "menuIds": [12, 15, 18]
}
```

`menuIds` 必须显式存在；空数组合法，表示清空全部直接授权。重复 ID、非正整数、directory ID、不存在或已软删除的菜单 ID 都返回明确错误。禁用 page/action ID 合法。

成功返回：

```json
{
  "code": 0,
  "data": {"id": 8, "permissionCount": 3},
  "message": "ok"
}
```

`permissionCount` 是规范化后实际保存的直接授权数量，不是请求数组长度。

### 10.10 严格 DTO

所有 JSON body 使用严格解码：

- 拒绝未知字段；
- 拒绝尾随第二个 JSON 值；
- 区分缺失字段与显式零值或空数组；
- URL `:id` 必须是正整数；
- 空 body 接口拒绝非空 body；
- 不接受 snake_case、旧字段或兼容别名。

HTTP envelope 精确为 `code`、`data`、`message`，不接受或返回 `msg`。

## 11. 角色 Service 行为

### 11.1 创建

1. 校验并规范化 code、name；
2. 拒绝系统保留编码；
3. 创建 `is_default=0`、`is_enabled=1` 的角色；
4. 不创建任何 `sys_role_menu`；
5. 将并发唯一索引错误精确映射为编码冲突或名称冲突。

### 11.2 修改名称

1. 锁定目标有效角色；
2. 拒绝系统角色；
3. 校验规范化名称；
4. 同名称请求幂等成功；
5. 更新 `name`、`updated_at`；
6. 唯一索引冲突映射为名称冲突。

### 11.3 启用和禁用

禁用：

- 拒绝 `super_admin`；
- 拒绝当前默认角色；
- `registered_user` 只有不再是默认角色时才允许禁用；
- 允许禁用已经绑定用户的其他角色；
- 只更新角色 `is_enabled`、`updated_at`；
- 保留全部 `sys_user_role` 和 `sys_role_menu`。

启用：

- 目标必须存在且未删除；
- 只更新目标角色；
- 原用户关系和授权自动恢复参与权限计算。

同状态请求幂等成功。用户还有其他启用角色时继续使用其他角色权限；用户没有任何启用角色时，现有认证查询在下一次请求返回 401。

### 11.4 删除

删除事务：

1. 锁定目标有效角色；
2. 拒绝 `super_admin`、`registered_user`；
3. 拒绝当前默认角色；
4. 查询有效用户绑定，只要数量大于 0 就拒绝；
5. 使用同一时间戳软删除有效 `sys_role_menu`；
6. 使用同一时间戳更新角色 `updated_at`、`deleted_at`；
7. 任一步失败全部回滚。

删除不自动把用户迁移到默认角色，也不修改历史用户关系。

## 12. 角色授权事务

### 12.1 查询

授权查询：

1. 查询目标有效角色；
2. 拒绝 `super_admin`；
3. 查询全部未软删除菜单；
4. 使用菜单模块既定树约束验证并建立授权树；
5. 查询目标角色当前有效直接授权；
6. 任一授权指向 directory、缺失菜单或已软删除菜单时返回角色数据状态损坏，不静默过滤；
7. 返回稳定排序树和直接授权 ID。

角色处于禁用状态时仍允许查询和修改授权。

### 12.2 保存

保存使用单个 PostgreSQL 事务：

1. 锁定目标有效角色；
2. 拒绝 `super_admin`；
3. 锁定当前有效菜单树，与菜单删除和类型修改串行；
4. 校验每个请求 ID 存在、未删除并且类型为 page/action；
5. 允许目标菜单被禁用；
6. 按最小直接授权规则规范化请求；
7. 查询并校验当前有效 `sys_role_menu`；
8. 使用同一时间戳软删除本次移除的授权；
9. 为新增授权创建新记录；
10. 保留未变化授权的 ID 和时间字段；
11. 更新角色 `updated_at`；
12. 任一步失败全部回滚。

规范化规则：

- directory 永远不进入结果；
- action 保留；
- action 存在时，移除同一 page 的冗余直接授权；
- page 没有已选 action 时保留；
- 多个 action 分别保留；
- 最终 ID 按升序写入和返回；
- 空集合删除全部有效直接授权。

不采用先物理删除全部记录再重建的实现，避免丢失历史和无意义地改写未变化记录。

## 13. 错误与国际化

角色模块使用独立 `15000` 段：

| code | HTTP | 语义 |
| --- | --- | --- |
| `15000` | 404 | 角色不存在 |
| `15001` | 409 | 角色编码冲突 |
| `15002` | 409 | 角色名称冲突 |
| `15003` | 409 | 系统角色保护规则阻止操作 |
| `15004` | 409 | 默认角色保护规则阻止操作 |
| `15005` | 409 | 角色仍绑定用户，禁止删除 |
| `15006` | 400 | 角色状态或目标状态无效 |
| `15007` | 400 | 授权菜单 ID 或类型无效 |
| `15008` | 409 | 超级管理员禁止配置授权 |
| `15009` | 500 | 角色、默认角色或授权数据状态损坏 |

通用请求绑定错误继续使用共享 `10001`。认证缺失和权限不足继续使用共享 401/403。PostgreSQL 查询或事务失败继续转换为依赖不可用错误。

后端 `zh-CN`、`en-US` 必须拥有相同角色 message key 和参数集合。前端页面、确认框、状态标签、菜单标题和权限标题同步补齐两套语言。任何响应都不得包含 SQL、表名、堆栈、Token、密码、DSN 或内部 Cause。

## 14. 前端协议

### 14.1 注册表

前端增加：

- `system-roles` -> `views/system/roles/index.vue` 视图注册；
- `navigation.systemRoles` 菜单标题 key；
- 六个角色 action 标题 key；
- `UserFilled` 图标继续通过现有图标注册表校验；
- 角色页面和确认框双语文案。

access 响应中的角色页面和权限节点必须通过现有严格协议校验。缺少 view、icon 或 i18n key 时明确失败，不使用未知标题、空组件或默认图标兜底。

### 14.2 API 契约

`web/src/api/role.contract.ts` 定义并严格解析：

- `RoleListQuery`；
- `RoleListItem`；
- `PageResult<RoleListItem>`；
- 创建、修改、状态、默认和授权请求 DTO；
- `RolePermissionTreeNode`；
- `RolePermissionsResponse`。

HTTP 外部数据从 `unknown` 开始校验。有限字段必须精确匹配：

- `isDefault`、`isEnabled` 只接受 `0 | 1`；
- 数量必须是非负整数；
- ID 必须是正整数；
- 时间必须是非空字符串；
- 对象拒绝未知字段；
- 数组拒绝重复 ID；
- 树节点拒绝非法类型、孤儿、重复 ID 和非法 children。

业务 TypeScript 和测试不得使用显式 `any`、`any[]`、`as any` 或 `Record<string, any>`。

## 15. 前端页面

### 15.1 页面结构

`/system/roles` 页面包含：

```text
标题 + 新增 + 刷新
关键词 + 启用状态 + 查询 + 重置
角色表格
分页
角色资料 Dialog
角色授权 Dialog
```

表格列固定为：

- 角色名称；
- 角色编码；
- 默认角色；
- 启用状态；
- 用户数量；
- 直接权限数量；
- 创建时间；
- 更新时间；
- 操作。

首次加载显式发送 `page=1&pageSize=20`。查询和重置回到第一页；编辑、授权、状态和默认操作成功后保持当前筛选和分页；创建成功后回到第一页；删除当前页最后一条后，页码只在确实越界时减一并重新查询。

列表加载失败时展示明确错误态，不把已有数据清空为假空表。mutation 失败时保留当前列表。

### 15.2 按钮权限与保护状态

页面命令分别由以下权限控制：

| 命令 | 权限 |
| --- | --- |
| 查看页面、筛选、刷新 | `system:role:list` |
| 新增 | `system:role:create` |
| 编辑名称 | `system:role:update` |
| 启用、禁用 | `system:role:status` |
| 设为默认 | `system:role:default` |
| 删除 | `system:role:delete` |
| 查看和保存授权 | `system:role:authorize` |

没有权限的命令不显示。拥有命令权限但被系统角色或默认角色规则阻止时，按钮使用明确禁用状态，并通过 Tooltip 说明原因。前端禁用只改善体验，后端 Service 必须独立执行同样保护。

### 15.3 角色资料 Dialog

角色资料使用 Element Plus `el-dialog`，桌面宽度约 520px：

- 创建模式显示可编辑 code、name；
- 编辑模式显示只读 code 和可编辑 name；
- 不放入状态、默认或权限字段；
- 提交时显示 loading 并阻止重复提交；
- 表单校验或 API 失败时保留输入和 Dialog；
- 成功后关闭 Dialog 并按既定分页规则重新查询。

本期直接使用 Element Plus，不提前创建 `AppDialog`。

### 15.4 角色授权 Dialog

角色授权使用独立的大尺寸 `el-dialog`，桌面宽度约 900px：

- header 显示角色名称和编码；
- body 显示完整菜单权限树；
- body 设置稳定高度和内部纵向滚动；
- 禁用菜单保留显示，并显示明确禁用状态；
- 软删除菜单不显示；
- 零权限是合法空选择；
- 加载失败时显示错误态，不展示伪造空树；
- 保存失败时保留当前选择和 Dialog；
- 保存成功后关闭 Dialog，并刷新当前列表的直接权限数量；
- `super_admin` 不显示授权入口。

树以菜单 ID 为 `node-key`。page 与 action 的直接授权选择必须保持独立：选择 page 不自动授予 action，选择 action 不额外创建 page 直接授权。directory 只提供分组和批量勾选交互，提交时被移除。提交前前端可以做同样的最小集合规范化，但后端仍必须重新验证和规范化，不能信任页面算法。

### 15.5 确认交互

状态、默认和删除使用 Element Plus 确认框：

- 禁用：显示 `userCount`，说明该角色将停止为这些用户提供权限，关系和授权会保留；
- 设为默认：说明只影响后续新用户，已有用户不变；
- 零权限角色设为默认：额外说明新用户只能看到固定 Dashboard；
- 删除：说明角色及其授权将软删除；用户仍绑定时后端明确拒绝。

管理员收到明确影响说明并确认后，系统按请求执行，不再增加隐藏审批或猜测管理员意图。

### 15.6 滚动与响应式

Main 继续是页面唯一纵向滚动容器。页面、表格和分页不得撑出 body 全局滚动条。Dialog teleport 到 `document.body`，其 body 自己滚动；固定 header/footer 不被长权限树推离视口。移动端使用 Element Plus 响应式宽度，本期不封装全局移动端 Dialog。

## 16. 测试策略

### 16.1 后端

Repository 和 Service 使用真实 PostgreSQL 事务测试，至少覆盖：

- 角色 Schema 幂等建立及所有索引、CHECK、外键；
- 系统角色首次创建和重复启动；
- 系统角色软删除历史、重复、改名和非法状态导致启动失败；
- 管理员切换默认角色后重启不会被改回；
- 分页必填、边界、稳定排序、关键词和状态筛选；
- 关键词 `%`、`_` 按字面值搜索；
- 角色编码和名称冲突及并发唯一索引映射；
- 创建角色默认启用、非默认、零权限；
- 系统角色改名、禁用、删除和授权保护；
- 默认角色禁用、删除保护；
- 并发默认角色切换及事务回滚；
- 禁用已绑定角色保留 `sys_user_role`、`sys_role_menu`；
- 角色重新启用后原关系恢复参与 access 查询；
- 有效用户绑定阻止删除，禁用用户仍计数；
- 无用户绑定时角色及授权同事务软删除；
- 删除任一步失败全部回滚；
- 授权查询包含禁用菜单并拒绝损坏直接授权；
- 授权拒绝 directory、重复、缺失和已删除菜单 ID；
- 授权允许禁用 page/action；
- page、action 最小集合规范化；
- 空数组清空权限；
- 未变化授权不重建；
- 授权事务失败完整回滚；
- Handler 严格 JSON、查询 allowlist、URL ID 和空 body；
- 每条路由绑定准确权限；
- 中英文角色错误目录 key 和参数完全一致。

完整后端验证：

```powershell
cd server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

### 16.2 前端

前端测试至少覆盖：

- 角色菜单标题、view 和 icon 注册；
- 分页响应、列表项和权限树严格解析；
- 必填分页参数和筛选请求；
- 初次加载、查询、重置、分页和刷新；
- 加载失败不伪造空列表；
- 每个按钮的权限显示规则；
- 系统角色和默认角色按钮禁用状态；
- 创建和编辑 Dialog 字段差异；
- 创建回第一页，其他 mutation 保持分页筛选；
- 状态、默认和删除确认文案；
- 受影响用户数量和零权限默认角色警告；
- 授权加载、禁用节点展示、空选择和保存；
- page/action 独立选择及 directory 不提交；
- API 失败保留列表、表单、树选择和 Dialog；
- mutation 不请求 `/api/v1/access`、不热更新路由、不刷新浏览器；
- 页面和 Dialog 滚动边界；
- 中英文文案完整；
- 业务 TypeScript 无 forbidden any。

完整前端验证：

```powershell
cd web
pnpm vitest run
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src -g "*.ts" -g "*.vue"
```

## 17. 技术债和回收顺序

系统施工顺序固定为：

```text
角色管理
-> 用户管理与用户角色分配
-> RBAC 完整联调验收
-> 技术债回收
-> UI 专项整改
```

角色一期允许的技术债及回收节点：

| 当前实现 | 风险边界 | 回收节点 |
| --- | --- | --- |
| 页面直接使用 `el-dialog` | 两个业务 Dialog 会产生重复配置 | RBAC 验收后设计并实现 `AppDialog` |
| 页面直接使用 `el-table`、`el-pagination` | 角色和后续用户页面存在机械重复 | 用户管理完成后设计无请求能力的 `AppTable` |
| 分页、筛选、加载状态写在页面内 | 可能与用户管理重复 | 两个真实分页页面完成后评估 `usePagedList` |
| 搜索区页面内实现 | 可能与用户管理重复 | 用户管理完成后评估通用搜索表单 |
| role/user 后端会出现相似 CRUD 代码 | 可能存在可提取的机械重复 | RBAC 验收后做重复审查；禁止 BaseService/BaseRepository |
| 权限每次读取 PostgreSQL | 数据量增加后查询压力上升 | RBAC 验收后单独设计 Redis 权限缓存和失效版本 |
| 权限变更无操作审计 | 无法追溯管理员变更 | RBAC 验收后的技术债切片补齐角色、用户、菜单操作日志 |

技术债必须通过独立 spec 明确真实重复、缓存一致性和失败语义后回收。不得在角色一期创建未使用公共组件、空缓存接口或通用 CRUD 框架。

技术债验证完成后，再进入 UI 专项整改。UI 整改负责颜色、间距、密度、响应式、交互细节和视觉一致性，不与数据流、权限算法或公共组件重构混在同一个切片。

Redis 权限缓存只允许作为 PostgreSQL 权威数据的可失效读取缓存。进程内缓存必须遵守最新
三层 Access 基线：先确认 Redis access version，再读取有界不可变本机快照；Redis 不可用时
不得返回旧权限或假成功。具体 key、版本、失效和多实例一致性以最新缓存 spec 为准。

## 18. 预期代码边界

后端主要修改或新增：

```text
server/internal/module/role/
  model.go
  schema.go
  protocol.go
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
  schema.go
  related tests

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
  menu-icons.ts
  related tests

web/src/api/
  role.contract.ts
  role.contract.test.ts
  role.ts
  role.test.ts

web/src/i18n/messages/
  zh-CN.ts
  en-US.ts

web/src/views/system/roles/
  index.vue
  index.test.ts
```

页面业务状态继续留在 `views/system/roles/index.vue`。只有文件规模或测试隔离出现真实问题时，才在该页面目录内拆分具体组件；不得为本期创建全局 CRUD 页面框架。

## 19. 验收标准

1. `super_admin` 能通过动态侧边栏进入 `/system/roles`；
2. 角色列表严格使用公共分页协议，支持关键词和启用状态筛选；
3. 自定义角色可以创建、改名、启停、设默认和在无用户绑定时软删除；
4. 系统角色、当前默认角色和绑定用户角色的保护规则同时存在于前端和后端；
5. 默认角色切换事务安全，只影响后续新用户；
6. `registered_user` 的权限可配置，身份字段和删除保护保持稳定；
7. `super_admin` 不维护 `sys_role_menu` 且无法进入授权操作；
8. 授权 Dialog 显示完整未删除菜单树，包括禁用节点；
9. 授权保存严格遵守 page/action 直接授权和最小集合规则；
10. 零权限角色和零权限默认角色均可按明确规则工作；
11. 禁用角色保留关系和授权，重新启用后恢复；
12. 所有 API 使用严格 DTO、稳定 envelope、明确权限码和双语错误；
13. 所有前端外部数据严格校验，业务 TypeScript 不出现 forbidden any；
14. mutation 后不主动刷新当前用户 access 或浏览器；
15. Main 和 Dialog 滚动边界正确，不出现 body 全局滚动条；
16. 后端和前端完整测试、构建及仓库 hygiene 检查通过；
17. 技术债和“先还债、再改 UI”的回收顺序在本 spec 中明确记录。
