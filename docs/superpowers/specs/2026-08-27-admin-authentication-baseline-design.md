# Admin 邮箱登录与账号资料基线设计

日期：2026-08-27

状态：设计已确认，实施计划已编写；等待 API 命名空间基线先行实施

## 1. 目的

本设计收紧 Admin 的公开入口，并把当前认证身份从“用户名登录”调整为“全局邮箱账号”。本切片
只处理继续建设 Admin 前必须稳定下来的账号基础：Admin 不公开注册、邮箱密码登录、内置 Admin
平台禁止注册，以及全局用户增加可选手机号。

当前 Go 后端的认证、用户、角色和会话是未来多个独立前端共享的身份后端；当前 Vue 项目仍然
只服务 Admin。一个邮箱对应一个全局用户，用户可以拥有多个角色。角色跨平台菜单的合并、去重
和平台过滤属于下一份 RBAC 平台隔离设计，不在本切片提前实现。

本设计细化 `docs/learning/future-menu-template.md` 的管理员账号方向，并修订以下既有设计：

- 替代 `2026-08-17-admin-authentication-foundation-design.md` 中“用户名密码登录”、Admin 注册页、
  `/register` 前端路由和“不包含手机号”的条款；
- 替代 `2026-08-20-authentication-rbac-redis-hardening-design.md` 中内置 `admin` 平台
  `allowRegister=1`、Admin 根据 Policy 展示注册入口和允许直接进入注册页的条款；
- 保留上述设计未被本文件明确修改的密码摘要、Token、Cookie、会话、Redis、Origin、设备、
  平台 Header、权限缓存和安全边界。

本设计依赖 `2026-08-27-admin-api-namespace-baseline-design.md`。共享认证和 `/access` 继续使用
`/api/v1`；用户、角色、菜单、认证平台、会话与操作日志等 Admin 管理接口使用
`/api/admin/v1`。必须先完成命名空间基线，再执行本设计的实施计划。

## 2. 已确认决策

- 一个规范化邮箱只对应一个全局有效用户账号，不按认证平台复制用户；
- Admin 前端删除公共注册入口、注册路由和注册页面；
- 后端 `POST /api/v1/auth/register` 暂时保留，供未来明确允许注册的业务平台复用；
- 内置 `admin` 认证平台固定 `allow_register=0`，不能通过管理接口改成 `1`；
- Admin 使用邮箱和密码登录，不再接受用户名登录；
- 用户名继续保留，作为展示名和后台可编辑资料，不再承担登录标识；
- 邮箱保存和查询前统一去除首尾空白并转为小写；
- 邮箱不存在与密码错误返回相同的公开认证错误；
- 密码登录不会因为邮箱不存在而自动创建用户；免注册邮箱验证码登录留到邮件能力完成后设计；
- `registered_user` 默认角色、用户多角色模型和 `bootstrap-admin` 创建首个超级管理员的路径保留；
- `user_account` 增加可空手机号；本切片不做手机号验证、唯一约束、短信绑定或地区模型；
- 当前 Admin 完成后再接业务前端。在 RBAC 平台隔离完成前，不接入新的业务前端。

## 3. 老项目参考结论

本切片按革新式重构原则参考旧项目，不照搬其重型认证模块。

### 3.1 直接继承

- 邮箱和手机号都是用户账号资料；
- 密码登录的账号输入、错误提示和登录成功后的会话流程保持成熟交互；
- 用户管理列表展示邮箱和手机号，并允许管理员维护基础资料；
- 个人中心未来仍应提供当前管理员修改密码、邮箱和手机号的能力。

### 3.2 适配迁移

- 登录请求适配为当前 lower camel case DTO、统一 envelope 和平台/设备 Header；
- 邮箱查询、用户资料读写继续走当前 `handler -> service -> repository -> model -> PostgreSQL`；
- 前端继续使用当前 Pinia Auth Store、Axios 请求层、Element Plus、i18n 和严格 TypeScript DTO；
- 用户多角色、平台会话和当前 Token 轮换机制保持当前项目实现。

### 3.3 明确替换

- 不复制旧项目同时承载密码、邮箱验证码、短信验证码、验证码挑战和自动注册的重型登录编排；
- 不引入通用登录类型注册器、认证 Adapter、Manager、Factory 或平台主体复制模型；
- 不把旧项目手机号唯一、短信验证、地址和个人资料整套能力提前迁入；
- 不为尚未建设的业务前端创建路由、菜单、表或占位接口。

## 4. 范围

### 4.1 包含

- Admin 前端删除 `/register` 路由、注册链接、注册页面及其专用 API、i18n 和测试；
- Admin 登录表单和登录 API 请求从 `username` 改为 `email`；
- 后端登录请求、Service 和 Repository 改为按规范化邮箱认证；
- 内置 `admin` 平台注册开关的数据库迁移、基础数据校验、管理接口约束和缓存协调；
- `user_account.phone` 可空字段及数据库契约；
- `/api/v1/auth/me`、Admin 用户列表、用户角色摘要和用户编辑协议中的手机号；
- Admin 用户管理页面展示和编辑手机号；
- 相关后端、前端、真实 PostgreSQL 和 Redis 定向测试。

### 4.2 不包含

- 邮箱验证码发送或登录；
- 腾讯云邮件服务、邮件模板或发送日志；
- 邀请管理员、账号激活和首次设置密码；
- 忘记密码、找回密码、修改密码和个人中心；
- 手机短信发送、手机号验证码、手机号登录或手机号绑定；
- 邮箱或手机号变更后的身份核验和会话撤销策略；
- `rbac_menu.platform`、平台菜单迁移或 `/access` 平台过滤；
- 腾讯云 COS；
- AI、支付或任何业务前端页面和接口；
- 地区、国家区号、省市区、地址、经纬度或地理位置表；
- 通用身份 Provider、认证 Adapter 或多云抽象。

## 5. 身份与平台边界

### 5.1 全局用户

`user_account` 是共享身份后端中的全局账号表。邮箱的有效记录部分唯一索引继续作为账号唯一性
约束：

```sql
CREATE UNIQUE INDEX ux_user_account_email_active
ON user_account (email)
WHERE deleted_at IS NULL;
```

同一个有效邮箱不能因为登录平台不同而创建第二个用户。平台只决定认证策略、会话范围和当前
请求可见的权限，不拥有独立用户副本。

### 5.2 多角色

现有 `rbac_user_role` 多角色关系保持不变。注册成功仍只绑定当前有效默认角色；后续管理员可以
通过用户管理给同一用户分配多个角色。本切片不修改角色合并算法。

### 5.3 平台权限安全边界

当前 `access` 缓存键虽然包含平台，但菜单数据尚未按平台归属过滤。下一切片必须给每个
`rbac_menu` 节点增加平台，并在权限源查询阶段先按当前认证平台过滤，再合并、去重多角色授权。
超级管理员届时也只能得到当前平台菜单。

本切片不得用前端隐藏菜单代替后端过滤，也不得声称已经完成跨平台权限隔离。该迁移完成前，
禁止把新的业务前端接到共享后端。

## 6. PostgreSQL 数据模型

### 6.1 `user_account.phone`

在现有 `user_account` 增加：

| 字段 | 类型 | 约束与用途 |
| --- | --- | --- |
| `phone` | `VARCHAR(32) NULL` | 未验证的可选手机号资料 |

规则固定为：

- 历史用户迁移后为 `NULL`，不回填空字符串或假号码；
- `NULL` 表示未填写；输入 `null` 表示清除；
- 非空输入去除首尾空白后长度为 1 到 32 个 Unicode 字符；
- 拒绝控制字符；不自动补国家区号，不删除 `+`、空格、短横线或括号；
- 不创建唯一索引、普通索引、验证状态、国家区号或短信状态字段；
- 后续短信绑定实现时再确认规范化格式、唯一性、验证状态和历史数据迁移。

Go Model 使用明确的可空类型表达 `NULL`，API 使用 `string | null`，不得把 `NULL` 静默转换为
空字符串。

### 6.2 邮箱

现有 `email VARCHAR(254) NOT NULL` 和有效记录部分唯一索引保持不变。所有创建和登录入口复用
同一邮箱规范化规则：

```text
trim surrounding whitespace
-> lowercase
-> validate as a plain email address
-> enforce maximum 254 UTF-8 bytes
```

不保留大小写不同的显示邮箱，不增加平台联合唯一索引，也不允许按用户名回退查询。

### 6.3 内置 Admin 平台

内置 `admin` 平台的新库基础值改为：

```text
code=admin
allowRegister=0
isEnabled=1
isBuiltin=1
```

其中 `code=admin`、`isBuiltin=1` 和 `allowRegister=0` 是内置身份约束；`isEnabled=1` 只是新库
默认值。其他 Token、缓存、设备、IP、启停和会话策略继续遵守现有平台管理规则，不因本切片
重置。基础数据校验只校验内置身份约束和运行时字段合法性，不把可编辑策略强制恢复为种子默认值。

新数据库直接创建 `allow_register=0`。已有数据库执行显式、幂等迁移：

1. 完全没有 `admin` 历史记录时，按新基础值创建内置记录；
2. 存在一条有效内置 `admin` 记录时锁定并校验身份字段；
3. `allow_register=1` 时改为 `0`，维护 `updated_at` 并递增 `policy_version`；
4. 已为 `0` 时不重复写入或递增版本；
5. 存在重复记录、软删除历史记录、身份字段损坏或非法 Yes/No 值时启动失败，不猜测修复；
6. 不覆盖该平台其他已配置策略。

数据库迁移完成且 Redis 连接建立后，API 在开始监听前删除 `auth:policy:admin` 派生快照；第一次
Policy 读取再按现有 read-through 规则从 PostgreSQL 重建。这样旧的 `allowRegister=1` 不会从
Redis 恢复。删除失败时启动失败，不带旧策略提供服务。

## 7. 后端设计

### 7.1 数据流

登录链路保持：

```text
auth router
-> origin/platform/device middleware
-> auth handler
-> auth service
-> user repository
-> user_account / PostgreSQL
-> existing session and Redis flow
```

用户资料维护保持：

```text
user router
-> auth and permission middleware
-> user handler
-> user service
-> user repository
-> user_account / PostgreSQL
```

Handler 不查询数据库，Service 不接收 `*gin.Context`，Repository 不编排认证或 Redis。请求
`context.Context` 继续传给所有 PostgreSQL 和 Redis I/O。

### 7.2 邮箱密码登录

`LoginInput` 和 `LoginRequest` 使用 `Email`，`userStore` 使用明确的
`FindCredentialByEmail(context.Context, string)`。Repository 对已规范化邮箱做精确查询，不再
使用 `lower(username)`：

```sql
SELECT id, username, email, password_hash, is_enabled
FROM user_account
WHERE email = ? AND deleted_at IS NULL
LIMIT 1;
```

Service 顺序：

1. 读取当前平台 Policy；
2. 规范化并校验邮箱，校验密码非空；
3. 按邮箱读取有效账号；
4. 比较 bcrypt 密码摘要；
5. 继续执行现有用户状态、角色、平台会话上限、Redis 状态和 Token 签发流程。

邮箱不存在和 bcrypt 不匹配都使用现有“凭据无效”公开错误，不在 message、code 或状态码中
暴露差异。数据库、Redis 或策略依赖错误继续明确返回 Dependency Unavailable，不伪装成凭据
错误。停用用户和其他既有安全状态保持当前错误语义。

请求中出现旧字段 `username` 属于未知字段，严格绑定直接拒绝；不提供一段时间的双字段兼容，
也不尝试判断输入是用户名还是邮箱。

### 7.3 后端注册能力

`POST /api/v1/auth/register` 的请求和成功响应暂时保持现状：

```json
{
  "username": "display-name",
  "email": "user@example.com",
  "password": "user supplied password",
  "confirmPassword": "user supplied password"
}
```

注册继续先读取 `X-Auth-Platform` 对应 Policy，并只在 `allowRegister=1` 时创建全局用户、绑定
当前有效默认角色。内置 `admin` 固定返回注册被禁止；该接口不能绕过平台策略。

注册接口不接收手机号。本切片也不把密码登录改成“邮箱不存在则注册”。未来业务平台的邮箱
验证码首次登录落地后，再单独决定是否删除显式注册接口。

### 7.4 Bootstrap

`cmd/bootstrap-admin` 继续读取用户名、邮箱和密码并创建首个超级管理员，不依赖 Admin 注册
开关。创建后的管理员使用规范化邮箱和密码登录。命令不增加手机号输入，不创建默认手机号。

### 7.5 Admin 平台不可开启注册

约束必须同时存在于以下边界：

- 基础数据：内置 `admin.allow_register` 为 `0`；
- Service：更新 `admin` 且请求 `allowRegister=1` 时返回明确的策略参数错误；
- Repository/事务：只写入 Service 已校验的候选值，不自行覆盖；
- 前端认证平台页面：Admin 行的“允许注册”控件禁用并显示“否”；
- 公开 Policy：Admin 平台可用时返回 `allowRegister: 0`；平台停用时继续按现有语义拒绝请求。

前端禁用控件只是交互提示，后端 Service 约束才是业务安全边界。

## 8. HTTP 契约

所有响应继续使用精确的 `code`、`data`、`message` envelope。认证请求继续携带当前项目已经
要求的 `X-Auth-Platform` 和 `X-Device-ID`。

### 8.1 登录

```text
POST /api/v1/auth/login
```

请求：

```json
{
  "email": "admin@example.com",
  "password": "user supplied password"
}
```

成功响应、Refresh Cookie、Access Token、TTL 和会话语义不变。

### 8.2 当前用户

```text
GET /api/v1/auth/me
```

响应数据改为：

```json
{
  "userId": 1,
  "username": "Admin",
  "email": "admin@example.com",
  "phone": null
}
```

`phone` 必须存在，值为字符串或 `null`，不是可省略字段。

### 8.3 用户列表与角色摘要

`GET /api/admin/v1/users` 的每个列表项增加：

```json
{
  "phone": null
}
```

用户角色详情中的 `user` 摘要同样增加 `phone`，避免同一用户在两个管理视图出现不一致 DTO。
用户列表 `keyword` 搜索范围扩展为用户名、邮箱和手机号；手机号为 `NULL` 时自然不匹配。

### 8.4 用户编辑

命名空间基线完成后的管理接口路径固定为：

```text
PUT /api/admin/v1/users/:id
```

请求精确为：

```json
{
  "username": "Admin",
  "phone": "+86 138-0000-0000"
}
```

`username` 和 `phone` 两个键都必须出现；`phone` 可以为 `null`。省略字段、提交空字符串、未知
字段或非法值均返回 Invalid Request，避免 PATCH 式猜测。邮箱在本切片只读，不通过该接口修改。

成功响应改为：

```json
{
  "id": 1,
  "username": "Admin",
  "phone": "+86 138-0000-0000",
  "updatedAt": "2026-08-27T10:00:00Z"
}
```

更新用户名和手机号必须在同一事务内完成。两者均未发生变化时可以返回当前值，但不得伪造新的
`updatedAt`。当前登录用户被管理员修改后，前端同步更新 Auth Store 中的用户名和手机号；不
撤销会话。

## 9. Admin 前端

### 9.1 删除注册入口

删除：

- Router 中 `/register` 和 `register` route name；
- `views/auth/register/index.vue`；
- 登录页注册链接和 `policyAllowRegister` 分支；
- `api/auth.ts` 中只被 Admin 注册页使用的 `RegisterInput`、`RegisteredUser` 和 `register()`；
- 路由守卫中针对 `/register` 的 Policy 判断；
- 动态菜单静态路径集合中的 `/register`；
- 注册专用中英文 i18n 文案和测试。

访问 `/register` 不提供兼容重定向或占位页；它和其他未声明路径遵守当前 Router 的未匹配行为。
后端注册接口保留不等于 Admin 前端保留注册代码。

Admin 登录页不再读取公开 Policy；提交登录时由后端按现有流程校验平台。后端
`GET /api/v1/auth/policy` 继续保留，供未来业务客户端读取自身平台的公开认证策略。

### 9.2 登录表单

- 表单字段改为 `email` 和 `password`；
- 邮箱输入使用 `type="email"`、`autocomplete="username"` 和 `inputmode="email"`；
- 表单执行必填和邮箱格式校验；
- 提交前只去除邮箱首尾空白，不修改密码；
- DTO 只发送 `{ email, password }`；
- 登录成功后的 Auth Store、`/me`、安全 redirect 和路由加载流程不变。

中英文文案统一使用“邮箱”，不再提示“用户名登录”。测试选择器同步改为表达 email，避免保留
误导性的 `login-username`。

### 9.3 用户管理

- 列表在邮箱附近增加“手机号”列；未填写时使用当前表格空值展示规范；
- 编辑 Dialog 增加手机号输入，邮箱继续只读展示；
- 保存时显式提交 `phone` 字符串或 `null`；
- 不增加国家区号选择器、短信验证码按钮或手机号状态标签；
- 前端只做与后端一致的长度和控制字符基础校验，不自行推断手机号格式；
- API 返回缺少 `phone`、类型错误或省略必填字段时按协议错误处理，不补空字符串。

### 9.4 认证平台管理

认证平台列表保留通用 `allowRegister` 字段。创建或编辑非内置业务平台时仍可选择该值；编辑
内置 `admin` 时控件锁定为“否”，并明确展示其为内置安全限制。

## 10. 错误、安全与可观察性

- 邮箱不存在和密码错误统一返回当前未认证 code/message；
- 登录参数格式错误使用 Invalid Request，与凭据错误区分；
- `admin` 尝试开启注册使用稳定、双语的认证平台策略错误，不静默改回 `0` 后返回成功；
- 手机号格式或长度错误使用稳定、双语的用户资料参数错误；
- PostgreSQL 或 Redis 失败显式返回依赖不可用，不返回假成功或旧资料；
- 日志不得记录密码、密码摘要、Token、Cookie、Authorization Header 或完整请求体；
- 登录日志如记录账号，应记录规范化邮箱并遵守现有脱敏规则；本切片不新增登录日志模块；
- 手机号属于个人信息，普通请求日志和操作日志正文不得记录完整值；
- 登录、刷新、退出、会话上限、设备/IP 绑定和 Token 生命周期不因登录标识变化而放宽。

## 11. 迁移与兼容

实施顺序固定为：

1. 完成并完整验证 Admin API 命名空间基线；
2. 为 `user_account` 增加可空 `phone` 并验证数据库类型；
3. 幂等迁移内置 `admin.allow_register`，递增真实变化的策略版本；
4. Redis 可用后删除 Admin Policy 快照，后续读取按 PostgreSQL 权威数据重建；
5. 后端上线邮箱登录契约及手机号 DTO；
6. 同一次交付上线 Admin 前端邮箱表单并删除注册入口；
7. 完整验证后才允许使用新版本。

登录请求是有意的破坏性协议变更：旧 `{ username, password }` 请求会失败，不提供双字段过渡。
因此前后端必须作为同一发布单元部署。现有会话、Refresh Token 和已登录管理员不强制退出；
会话刷新与 `/me` 在新版本继续工作。

手机号是只增加可空字段，不需要数据回填。AutoMigrate 可以负责新增列，但数据库最终类型、可空
性和无多余索引必须由真实 PostgreSQL 测试验证。`allow_register` 数据修订不能只依赖
AutoMigrate，必须使用明确、幂等、失败回滚的迁移逻辑。

## 12. 测试策略

行为变化遵循失败测试、最小实现、通过、再重构。

### 12.1 后端单元与协议测试

至少覆盖：

- Login JSON 只接受 `email` 和 `password`，拒绝旧 `username` 和未知字段；
- 邮箱 trim、小写、格式、长度和空值；
- Repository 只按邮箱查询有效账号；
- 邮箱不存在与密码错误具有相同公开响应；
- 用户停用、无有效角色和依赖失败保持现有语义；
- 注册接口继续按平台 `allowRegister` 拒绝或创建，并绑定默认角色；
- Admin 平台注册始终拒绝；
- `bootstrap-admin` 创建的账号可用邮箱登录；
- `/me`、用户列表、角色摘要和编辑响应始终包含 `phone`；
- `phone=null` 清除，合法字符串保存，空字符串、控制字符和超长值拒绝；
- 同时更新用户名和手机号的事务失败时不产生部分写入；
- 修改当前用户资料后 Auth 相关协议不撤销会话。

### 12.2 PostgreSQL 与 Redis 集成测试

至少覆盖：

- `user_account.phone` 为 `VARCHAR(32) NULL`，历史行保持 `NULL`；
- 邮箱有效记录全局唯一索引仍存在且不包含平台；
- 不存在手机号唯一索引或多余普通索引；
- 新数据库创建 Admin 平台时 `allow_register=0`；
- 历史 `allow_register=1` 只迁移一次，并正确维护版本和时间；
- 重复执行迁移不再次写入；
- 损坏或重复的内置 Admin 数据导致启动失败；
- 迁移后旧 Redis Policy 不可继续返回 `allowRegister=1`；
- Redis 协调失败时 API 不开始监听。

不使用 SQLite、内存数据库或内存 Redis 替代真实依赖。

### 12.3 前端测试

至少覆盖：

- Router 不再声明 `/register`；
- 登录页不显示注册链接，不包含注册 Policy 分支；
- 登录表单校验邮箱并只发送 `{ email, password }`；
- 旧用户名 DTO 在类型和协议测试中不可用；
- Auth Store 的当前用户包含 `phone: string | null`；
- 用户列表正确展示手机号空态和值；
- 编辑用户提交显式 `phone` 或 `null`，并同步当前用户 Store；
- 缺失或类型错误的 `phone` 被严格 DTO 解析拒绝；
- Admin 认证平台的注册开关不可编辑，其他平台仍可编辑；
- 中英文认证和用户管理文案不存在残留注册链接。

## 13. 预期代码边界

后端主要涉及：

```text
server/internal/module/auth/
server/internal/module/authplatform/
server/internal/module/user/
server/cmd/api/
server/cmd/bootstrap-admin/
```

前端主要涉及：

```text
web/src/api/auth.ts
web/src/api/user.ts
web/src/router/index.ts
web/src/permission.ts
web/src/store/auth.ts
web/src/views/auth/login/index.vue
web/src/views/auth/register/index.vue        # 删除
web/src/views/account/users/index.vue
web/src/views/access/auth-platforms/index.vue
web/src/i18n/messages/
web/tests/
```

实施计划应从实际调用点展开文件级任务；不得因为上述目录被列出就顺手重构无关代码。

## 14. 验证命令

文档确认并实施后，运行完整验证：

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

同时执行定向搜索，确认 Admin 注册和用户名登录残留已清理：

```powershell
rg -n "/register|auth\.register|login-username|FindCredentialByUsername" web\src web\tests server
```

搜索结果中只允许出现明确保留的后端注册接口、对应后端测试和历史文档；运行时代码不得残留
Admin 注册路由、注册页或用户名登录查询。

## 15. 验收标准

1. Admin 前端只有登录入口，没有注册入口、注册路由或注册页面；
2. Admin 只用规范化邮箱和密码登录，用户名不能作为兼容登录标识；
3. 邮箱不存在和密码错误不向客户端泄露差异；
4. 内置 Admin 平台在新库、旧库、管理 API 和 Redis Policy 中都固定禁止注册；
5. 后端注册能力仍能被明确允许注册的非 Admin 平台复用，并继续绑定默认角色；
6. 一个邮箱在全局有效用户中唯一，不按平台创建用户副本；
7. 用户名保留为展示资料，用户多角色、会话、JWT、Refresh 和 Bootstrap 行为保持有效；
8. 用户手机号可以为空，并在 `/me`、用户管理列表、角色摘要和编辑协议中保持一致；
9. 手机号没有唯一、验证、短信或地区相关的提前设计；
10. 本切片没有引入业务前端、AI、支付、COS、邮件、短信或 RBAC 平台菜单迁移；
11. 在 RBAC 平台隔离完成前，不把新的业务前端接入共享后端；
12. 后端和前端完整验证通过，协议和迁移测试使用真实 PostgreSQL/Redis。

## 16. 后续顺序

本 spec 审阅并实施完成后，后续按已确认顺序推进：

1. RBAC 菜单平台归属与 `/access` 平台隔离；
2. 腾讯云 COS 对象存储模块；
3. 腾讯云邮件服务；
4. 管理员邀请、激活、邮箱验证码登录和个人中心；
5. 腾讯云短信服务及手机号验证规则。

每个后续模块在开始前单独确认业务规则、编写 spec 和实施计划，不在当前数据库或菜单中提前
创建占位结构。
