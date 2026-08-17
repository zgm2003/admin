# Admin 认证基础设计

日期：2026-08-17  
状态：设计已确认，等待实施计划

## 1. 目的

本设计定义新 Admin 系统第一条真实业务链路：用户注册、用户名密码登录、Access Token 刷新、单会话、退出、当前用户加载，以及登录后的基础布局。

这一期采用纵向切片，先把认证闭环做实，再统一整改 UI，随后实现 RBAC、后端动态菜单和用户、角色、菜单管理。旧项目 `admin_back_go` 与 `admin_front_ts` 只作为已经运行过的经验来源，不复制其历史架构和兼容负担。

本设计细化 `2026-08-17-admin-foundation-design.md` 的认证方向。根 `AGENTS.md` 和 `docs/agent/README.md` 仍是更高优先级的施工规则。

## 2. 已确认决策

- 第一期邮箱注册不发送验证码；后续邮件认证切片再增加邮箱验证。
- 注册表单由用户明确填写用户名、邮箱、密码和确认密码。
- 注册用户立即启用，并自动绑定默认普通用户角色。
- 当前采用单会话；同一用户的新登录撤销旧登录。
- 第一个超级管理员通过一次性 `cmd/bootstrap-admin` 命令创建，不使用默认密码，也不让第一个注册用户自动提权。
- 登录只接受用户名和密码，不接受邮箱、手机号或验证码登录。
- 登录后布局使用全高 Aside；右侧依次为 Header、Main 和 Footer。
- 一期 UI 只要求清楚、可用和响应式，不做品牌化精修。
- PostgreSQL 保存会话真相，Redis 保存当前单会话指针，不使用内存状态兜底。
- Access Token 使用短有效期 JWT，Refresh Token 使用 HttpOnly Cookie，并在每次刷新时轮换。

## 3. 一期范围

### 3.1 包含

- 四张表：`sys_user`、`sys_role`、`sys_user_role`、`sys_user_session`；
- 两个系统角色：`super_admin` 和 `registered_user`；
- 用户注册、登录、刷新、退出和当前用户接口；
- bcrypt 密码摘要；
- JWT Access Token 和旋转式 Refresh Token；
- PostgreSQL 持久会话和 Redis 单会话指针；
- 一次性超级管理员初始化命令；
- 前端登录页、注册页、Auth Store、路由守卫和基础后台布局；
- 受保护的现有工作台页面；
- 后端、前端和真实 PostgreSQL 定向测试。

### 3.2 不包含

- 邮箱验证码、邮箱验证状态和邮件发送；
- 忘记密码、修改密码和找回密码；
- 登录限流、注册防滥用和验证码；
- 用户、角色、会话、菜单的管理页面；
- `sys_menu`、`sys_role_menu`、按钮权限和后端动态菜单；
- 登录日志、操作日志和安全审计页面；
- 手机号、头像、昵称、用户资料、地址和第三方登录；
- 认证平台配置、设备绑定、多平台和多会话策略；
- UI 品牌化和仪表盘内容重做。

## 4. 线性数据流

普通认证数据库请求保持明确链路：

```text
router
-> origin/auth middleware
-> handler
-> auth service
-> user/role/session repository
-> model
-> PostgreSQL
```

Redis 只由 Auth Service 在明确的会话步骤中调用：

```text
login service -> PostgreSQL session transaction -> Redis current-session pointer
auth service  -> JWT parse -> Redis pointer -> PostgreSQL session, user and role check
refresh service -> Redis pointer -> PostgreSQL compare-and-swap rotation
logout service  -> PostgreSQL revoke -> Redis pointer delete
```

前端请求链保持：

```text
view -> api/auth.ts -> utils/request.ts -> Go API
```

不引入 Platform Adapter、认证框架、DI 容器、Manager、Factory、BaseService、BaseRepository、运行时注册器或动态认证平台。

## 5. PostgreSQL 数据模型

所有时间字段使用 `TIMESTAMPTZ`。所有 Go 模型显式声明 `CreatedAt` 和 `UpdatedAt`，不嵌入 `gorm.Model`。有真实删除行为的表只使用 `deleted_at`，不增加 `is_del` 或 `is_deleted`。

### 5.1 `sys_user`

| 字段 | 类型 | 约束与用途 |
| --- | --- | --- |
| `id` | `BIGINT` | 主键，自增 |
| `username` | `VARCHAR(64)` | 必填，登录名 |
| `email` | `VARCHAR(254)` | 必填，注册邮箱 |
| `password_hash` | `VARCHAR(255)` | 必填，bcrypt 摘要 |
| `is_enabled` | `SMALLINT` | 必填，默认 `1`，只允许 `0/1` |
| `created_at` | `TIMESTAMPTZ` | 必填，默认当前时间 |
| `updated_at` | `TIMESTAMPTZ` | 必填，默认当前时间 |
| `deleted_at` | `TIMESTAMPTZ` | 可空，软删除时间 |

用户名规则：

- 去除首尾空白后长度为 3 到 64 个 Unicode 字符；
- 只允许 Unicode 字母、Unicode 数字、下划线和短横线；
- 保留用户输入的显示形式；
- 登录与唯一性比较不区分字母大小写。

邮箱规则：去除首尾空白、校验邮箱格式、保存为小写。密码不执行 `TrimSpace`，按用户原始输入校验和计算摘要。

索引：

```sql
CREATE UNIQUE INDEX ux_sys_user_username_active
ON sys_user (lower(username))
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX ux_sys_user_email_active
ON sys_user (email)
WHERE deleted_at IS NULL;
```

软删除用户后，原用户名和邮箱可以被新有效用户重新使用。Repository 必须根据约束名把冲突转换为明确的用户名或邮箱冲突，不能用字符串包含判断数据库错误。

### 5.2 `sys_role`

| 字段 | 类型 | 约束与用途 |
| --- | --- | --- |
| `id` | `BIGINT` | 主键，自增 |
| `code` | `VARCHAR(64)` | 必填，稳定角色编码 |
| `name` | `VARCHAR(64)` | 必填，展示名称 |
| `is_default` | `SMALLINT` | 必填，只允许 `0/1` |
| `is_enabled` | `SMALLINT` | 必填，只允许 `0/1` |
| `created_at` | `TIMESTAMPTZ` | 必填 |
| `updated_at` | `TIMESTAMPTZ` | 必填 |
| `deleted_at` | `TIMESTAMPTZ` | 可空 |

系统角色：

| `code` | `name` | `is_default` | `is_enabled` |
| --- | --- | ---: | ---: |
| `super_admin` | 超级管理员 | 0 | 1 |
| `registered_user` | 普通用户 | 1 | 1 |

有效角色编码使用部分唯一索引。`is_default = 1 AND deleted_at IS NULL` 使用部分唯一索引保证最多一个默认角色。

API 启动时在 AutoMigrate 之后显式执行系统角色初始化：缺失时创建；已存在时校验编码、默认状态和启用状态。遇到冲突或不完整数据时启动失败，不静默覆盖人工数据。

### 5.3 `sys_user_role`

| 字段 | 类型 | 约束与用途 |
| --- | --- | --- |
| `id` | `BIGINT` | 主键，自增 |
| `user_id` | `BIGINT` | 必填，外键到 `sys_user` |
| `role_id` | `BIGINT` | 必填，外键到 `sys_role` |
| `created_at` | `TIMESTAMPTZ` | 必填 |
| `updated_at` | `TIMESTAMPTZ` | 必填 |
| `deleted_at` | `TIMESTAMPTZ` | 可空 |

有效的 `(user_id, role_id)` 使用部分唯一索引。外键使用 `ON DELETE RESTRICT`；正常业务通过 `deleted_at` 解除关系。

第一期每个用户只有一个有效角色关系，但模型不把 `role_id` 放回用户表。后续 RBAC 可以直接增加多个角色关系，无需重构用户表。

### 5.4 `sys_user_session`

| 字段 | 类型 | 约束与用途 |
| --- | --- | --- |
| `id` | `BIGINT` | 主键，自增，也是 JWT session ID |
| `user_id` | `BIGINT` | 必填，外键到 `sys_user` |
| `refresh_token_hash` | `CHAR(64)` | 必填，Refresh Token 的 HMAC-SHA-256 十六进制摘要 |
| `version` | `BIGINT` | 必填，默认 `1`，必须大于等于 1 |
| `client_ip` | `VARCHAR(64)` | 必填，登录或最后刷新来源 IP |
| `user_agent` | `VARCHAR(512)` | 必填，可保存空字符串 |
| `refresh_expires_at` | `TIMESTAMPTZ` | 必填，Refresh Token 绝对过期时间 |
| `revoked_at` | `TIMESTAMPTZ` | 可空，退出、顶号或安全撤销时间 |
| `created_at` | `TIMESTAMPTZ` | 必填 |
| `updated_at` | `TIMESTAMPTZ` | 必填 |

会话表不使用 `deleted_at`。`revoked_at` 是安全生命周期状态，后续会话管理需要读取撤销记录；再增加软删除会产生两套状态。

索引与约束：

- `refresh_token_hash` 全局唯一；
- `user_id` 外键到 `sys_user`，使用 `ON DELETE RESTRICT`；
- `user_id, created_at DESC` 用于后续会话列表和当前用户会话查询；
- `user_id WHERE revoked_at IS NULL` 使用部分唯一索引，数据库层强制当前单会话；
- 新登录事务先撤销该用户所有未撤销会话，再创建新会话；过期但未撤销的旧记录也在此步骤中撤销。

开发阶段继续由 API 进程执行 GORM AutoMigrate。优先使用带名称的 GORM 约束和索引标签；如果 GORM 不能表达指定的 PostgreSQL 函数或部分索引，只在相邻数据库初始化代码中执行明确、幂等、可测试的 DDL，不引入通用 migration 框架。真实 PostgreSQL 集成测试必须验证最终约束和索引定义。

## 6. 后端模块

```text
server/internal/module/auth/
  route.go
  handler.go
  service.go
  request.go
  response.go
  password.go
  token.go
  session_model.go
  session_repository.go

server/internal/module/user/
  model.go
  repository.go

server/internal/module/role/
  model.go
  repository.go

server/cmd/bootstrap-admin/
  main.go
```

职责：

- Auth Router：注册 URL、中间件和 Handler；
- Auth Handler：严格绑定 JSON、读取 Cookie/Header/Origin、调用 Service、输出 envelope；
- Auth Service：验证业务规则、计算密码摘要、签发 Token、编排事务和 Redis 调用；
- User Repository：用户查询、创建用户和用户角色绑定事务；
- Role Repository：查找默认角色、查找超级管理员角色、初始化系统角色；
- Session Repository：创建、撤销、验证和条件轮换 PostgreSQL 会话；
- Middleware：验证 Bearer Token 和当前会话，只把认证身份放入 Gin Context；
- `bootstrap-admin`：组合已有 Service/Repository 创建首个超级管理员。

Service 不接收 `*gin.Context`。所有 I/O 使用请求或命令入口传入的 `context.Context`。只有单元测试确有替换边界时才定义覆盖当前方法的窄接口，不创建通用认证接口层。

## 7. 密码与 Token

### 7.1 密码

- 密码至少 8 个 Unicode 字符；
- bcrypt 输入最多 72 字节，超过时明确返回参数错误；
- 不要求人为组合大写、小写、数字和特殊字符；
- 使用 `bcrypt.DefaultCost`；
- 不保存明文、独立盐值、确认密码或可逆密文；
- 不记录密码、密码摘要或请求体；
- 登录的用户名不存在和密码错误返回同一个公开错误。

### 7.2 Access Token

- 使用 `github.com/golang-jwt/jwt/v5` 和 HS256；
- 有效期固定为 15 分钟；
- 只包含 issuer、user ID、session ID、session version、issued-at、not-before 和 expiry；
- 不包含用户名、邮箱、角色、菜单和按钮权限；
- 前端只在 Pinia 内存保存，不写入 localStorage、sessionStorage、IndexedDB 或普通 Cookie。

### 7.3 Refresh Token

- 使用 `crypto/rand` 生成至少 32 字节随机值；
- 数据库只保存使用独立 pepper 计算的 HMAC-SHA-256 摘要；
- 有效期固定为 14 天，轮换不延长最初的绝对过期时间；
- 每次刷新生成新 Token、替换摘要并递增 session version；
- Repository 使用旧摘要作为条件更新，同一个旧 Token 并发使用时只能有一个请求成功；
- 失败的旧 Token 不恢复、不兼容，也不返回新 Token。

Refresh Cookie 名为 `admin_refresh`，固定属性：

```text
HttpOnly=true
SameSite=Lax
Path=/api/v1/auth
Domain omitted (host-only)
Secure=AUTH_COOKIE_SECURE
Max-Age=remaining seconds until refresh_expires_at (14 days on initial login)
```

Cookie 同时写入与 `refresh_expires_at` 一致的 `Expires`。Refresh 轮换不把 Max-Age 重置为 14 天，而是写入剩余有效期。

## 8. 配置

认证上线后，API 增加三个必填环境变量：

```text
AUTH_JWT_SECRET       # 至少 32 字节的随机密钥
AUTH_REFRESH_PEPPER  # 至少 32 字节，且不得与 JWT secret 相同
AUTH_COOKIE_SECURE   # 0=本地 HTTP，1=HTTPS
```

三个变量必须加入 `server/.env.example` 的中文说明。本地 `.env` 不提交 Git。缺失、长度不足、两项 secret 相同或 Secure 值不是 0/1 时，API 启动失败。`CORS_ORIGIN` 使用 HTTPS 时 Secure 必须为 1；使用本地 HTTP 时 Secure 必须为 0，配置不一致时启动失败。

Access 与 Refresh TTL 在第一期使用代码常量，不创建动态认证平台表或设置页面。

## 9. HTTP 契约

所有接口继续使用精确 envelope：

```json
{
  "code": 0,
  "data": {},
  "message": "ok"
}
```

请求 JSON 使用 `validate.BindJSON`，拒绝未知字段、多个 JSON 值和尾随内容。Refresh 与 Logout 请求体必须为空，`{}`、空白内容和任意其他 body 都拒绝。

### 9.1 注册

```text
POST /api/v1/auth/register
Origin: exact configured frontend origin
Content-Type: application/json
```

```json
{
  "username": "zhangsan",
  "email": "zhangsan@example.com",
  "password": "user supplied password",
  "confirmPassword": "user supplied password"
}
```

成功返回 HTTP 201：

```json
{
  "code": 0,
  "data": {
    "userId": 1,
    "username": "zhangsan",
    "email": "zhangsan@example.com"
  },
  "message": "ok"
}
```

注册成功不创建会话、不设置 Cookie。前端提示成功后跳转登录页。

### 9.2 登录

```text
POST /api/v1/auth/login
Origin: exact configured frontend origin
Content-Type: application/json
```

```json
{
  "username": "zhangsan",
  "password": "user supplied password"
}
```

成功返回 HTTP 200，并设置 Refresh Cookie：

```json
{
  "code": 0,
  "data": {
    "accessToken": "jwt",
    "expiresIn": 900
  },
  "message": "ok"
}
```

### 9.3 刷新

```text
POST /api/v1/auth/refresh
Origin: exact configured frontend origin
Body: forbidden
Credential: admin_refresh Cookie
```

成功返回与登录相同的 `accessToken` 和 `expiresIn`，并轮换 Refresh Cookie。

### 9.4 退出

```text
POST /api/v1/auth/logout
Origin: exact configured frontend origin
Authorization: Bearer <access-token>
Body: forbidden
```

成功后撤销 PostgreSQL session、删除 Redis 当前指针并过期 Refresh Cookie：

```json
{
  "code": 0,
  "data": {},
  "message": "ok"
}
```

### 9.5 当前用户

```text
GET /api/v1/auth/me
Authorization: Bearer <access-token>
```

```json
{
  "code": 0,
  "data": {
    "userId": 1,
    "username": "zhangsan",
    "email": "zhangsan@example.com"
  },
  "message": "ok"
}
```

第一期 `/me` 不返回角色、菜单或权限。RBAC 切片将增加独立的权限上下文和菜单契约，不把临时静态菜单伪装成后端数据。

## 10. 单会话状态变化

### 10.1 登录

```text
1. 严格校验请求和 Origin
2. 按大小写不敏感用户名查询有效、启用用户
3. bcrypt 比较密码
4. 生成 Refresh Token 和摘要
5. PostgreSQL 事务锁定用户并重新确认用户、默认角色仍有效
6. 撤销全部未撤销旧 session
7. 创建 version=1 的新 session
8. 提交 PostgreSQL 事务
9. Redis 写入 auth:current-session:<userID> = <sessionID>
10. Redis 写入失败时撤销新 session，返回 503
11. 签发 Access Token，设置 Refresh Cookie
```

### 10.2 受保护请求

```text
1. 解析并严格验证 JWT 算法、签名和时间声明
2. Redis 读取当前 session ID；Redis 错误返回 503
3. Redis key 缺失时，从 PostgreSQL 查唯一未撤销、Refresh 未过期且用户和角色有效的 session 并重建指针
4. JWT session ID 必须等于当前 session ID
5. PostgreSQL 查询 session/version、用户启用状态、软删除状态和至少一个有效启用角色
6. 任一身份条件不匹配返回 401
7. 将 user ID 与 session ID 写入 Gin Context
```

Redis key 缺失后的 PostgreSQL 重建是设计明确的 read-through 行为；Redis 连接错误不是 cache miss，必须返回 503，不能切换成无 Redis 模式。

### 10.3 刷新

```text
1. 校验 Origin、空 body 和 Refresh Cookie
2. Redis 验证当前 session 指针
3. PostgreSQL 按旧 Refresh 摘要查找并锁定 session
4. 验证未撤销、未过期、用户和至少一个角色有效，且 session 是当前会话
5. 生成新 Refresh Token 和新摘要
6. 条件更新旧摘要，同时 version + 1
7. 使用新 version 签发 Access Token
8. 设置新 Refresh Cookie
```

刷新不改变 session ID，也不延长 `refresh_expires_at`。刷新成功后，带旧 version 的 Access Token 立即失效。

### 10.4 退出

```text
1. 完整验证当前 Access Token 和 session
2. PostgreSQL 设置 revoked_at
3. Redis 删除当前 session 指针
4. 过期 Refresh Cookie
```

PostgreSQL 撤销成功后即为安全真相。Redis 删除失败必须记录并返回 503，同时仍发送过期 Cookie；后续带旧凭据的请求会因 PostgreSQL session 已撤销而失败。

## 11. 一次性超级管理员初始化

命令：

```powershell
cd D:\admin\server
go run ./cmd/bootstrap-admin
```

命令读取：

```text
POSTGRES_DSN
BOOTSTRAP_ADMIN_USERNAME
BOOTSTRAP_ADMIN_EMAIL
BOOTSTRAP_ADMIN_PASSWORD
```

执行命令前必须至少成功启动一次 API，由 API 完成四张表的 AutoMigrate 和系统角色初始化。Bootstrap 命令自身不执行 AutoMigrate；表或系统角色缺失时明确失败，避免第二个迁移入口。

流程：

```text
bootstrap-admin
-> 校验配置和输入
-> PostgreSQL
-> 校验 super_admin 角色存在且启用
-> 检查是否已有有效超级管理员
-> bcrypt 计算摘要
-> 事务创建用户和用户角色关系
```

任何必填值缺失、表或角色数据异常、用户名或邮箱冲突、已有有效超级管理员时命令明确失败。Bootstrap 专用环境变量只通过当前进程注入，不写入项目 `.env`。命令不连接 Redis，不启动 HTTP，不执行隐式默认账号创建，也不打印密码或摘要。

## 12. 前端结构

```text
web/src/
  api/auth.ts
  store/auth.ts
  views/auth/login/index.vue
  views/auth/register/index.vue
  layout/index.vue
  layout/components/AppAside.vue
  layout/components/AppHeader.vue
  layout/components/AppFooter.vue
  permission.ts
```

### 12.1 路由

- `/login`：公开，只显示登录页；
- `/register`：公开，只显示注册页；
- `/`：受保护，使用 `layout/index.vue`；
- `/dashboard`：受保护，一期唯一静态菜单入口；
- 每一条路由必须显式声明 `requiresAuth`。

`AdminLayout.vue` 迁移完成后删除。Router 只引用 `layout/index.vue`，不保留兼容入口。

### 12.2 布局

```text
el-container (horizontal, min-height: 100vh)
├── el-aside (desktop full height)
└── el-container (vertical)
    ├── el-header
    ├── el-main -> RouterView
    └── el-footer
```

Aside 桌面宽度固定并支持折叠；移动端改为简单 Drawer。Header 只包含菜单按钮、当前位置、用户名和退出命令。Footer 只显示基础版权信息。静态菜单只包含工作台，不复制参考图中的业务菜单和仪表盘数据。

### 12.3 Auth Store

Auth Store 使用明确状态：

```text
unknown
anonymous
authenticated
error
```

Store 保存 Access Token、过期时间和当前用户 DTO。Store 只提供明确状态变更，不导入业务 API，避免请求模块循环依赖。退出或认证失效时一次性清空全部内存认证状态。

### 12.4 请求和路由守卫

- 请求层从 Auth Store 读取 Access Token 并附加 Bearer Header；
- 接口模块对 Auth DTO 做运行时结构校验，未知数据不通过类型断言直接进入 Store；
- 多个并发请求同时收到 401 时，只允许一个 Refresh 请求在途；
- Refresh 成功后，每个原请求最多重试一次；
- Login、Register、Refresh 和 Logout 自身不进入自动刷新循环；
- Refresh 返回 401 时进入 `anonymous` 并跳转登录；
- Refresh 返回 503 或协议错误时进入 `error`，页面明确展示服务或协议错误，不伪装成未登录；
- 应用冷启动时，受保护路由先 Refresh，再调用 `/auth/me`，两步成功后才放行；
- `permission.ts` 只编排认证状态、当前用户加载和跳转，不放页面业务逻辑。

业务 TypeScript 不使用显式 `any`、`as any`、`Record<string, any>`、`@ts-ignore` 或宽泛声明。

## 13. 错误协议与日志

线上 `code` 始终是整数：

| HTTP | `code` | 场景 |
| ---: | ---: | --- |
| 400 | 10001 | JSON、字段、用户名、邮箱或密码规则错误 |
| 401 | 10002 | 用户名或密码错误、Access/Refresh 无效或已过期 |
| 403 | 10003 | Origin 不允许、用户或角色被禁用 |
| 409 | 10005 | 有效用户名或邮箱冲突 |
| 503 | 10006 | PostgreSQL 或 Redis 不可用 |
| 500 | 10000 | 未分类内部错误 |

登录时用户名不存在和密码错误使用相同 HTTP、code 和 message。服务端日志记录 request ID、user ID、session ID、约束名和内部 cause，但响应不暴露 SQL、堆栈、DSN 或内部错误。

禁止记录：密码、密码摘要、Access Token、Refresh Token、Cookie、Authorization Header 和完整 Auth 请求体。

## 14. 测试与验证

### 14.1 后端

- Model/真实 PostgreSQL：表名、字段类型、`TIMESTAMPTZ`、CHECK、外键、函数索引和部分唯一索引；
- User Repository：大小写用户名冲突、邮箱冲突、软删除后重新注册、用户与角色事务回滚；
- Role Repository：幂等初始化、缺失默认角色、多个默认角色和禁用角色明确失败；
- Session Repository：单会话约束、旧会话撤销、版本条件更新、Refresh 摘要只能轮换一次；
- Auth Service：注册、bcrypt、错误凭据统一、禁用/删除用户、第二次登录顶号、刷新、退出和依赖失败；
- Middleware：JWT 算法、签名、时间、session ID、version、Redis pointer 和 PostgreSQL 状态；
- Handler：严格 JSON、空 body、Origin、Cookie 属性、HTTP 状态和精确 envelope；
- Bootstrap：成功创建、已有管理员、冲突和事务失败；
- 不使用 SQLite 替代 PostgreSQL。

### 14.2 前端

- Auth DTO 运行时校验和协议错误；
- 登录、注册表单的输入与错误展示；
- Auth Store 四种状态和清理；
- 多个 401 只触发一次 Refresh，每个请求最多重试一次；
- Refresh 401 与 503 分别进入 anonymous 和 error；
- 冷启动 Refresh + `/me`；
- 公开路由、受保护路由和显式 `requiresAuth`；
- Layout 渲染、桌面折叠和移动 Drawer；
- 显式 AnyScript 扫描保持零匹配。

### 14.3 浏览器验收

```text
注册 -> 登录 -> 进入工作台
刷新浏览器 -> Refresh -> /me -> 恢复登录
保存旧 Access -> Refresh -> 旧 Access 被拒绝
第二个浏览器登录 -> 第一个浏览器会话失效
退出 -> Cookie 过期 -> 受保护页面不可进入
停止 Redis -> 明确显示服务错误，不返回假成功
```

最终运行：

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...

cd D:\admin\web
pnpm vitest run
pnpm build
```

## 15. 明确技术债与回收节点

| 一期暂欠 | 风险边界 | 必须回收的节点 |
| --- | --- | --- |
| 登录/注册只有基础样式 | 不影响认证正确性 | 方案 A 验收后统一 UI 阶段 |
| Aside 只有静态工作台 | 不能代表真实权限 | RBAC 菜单接口完成时删除静态菜单 |
| 无用户、角色、会话管理页 | 只能通过数据库和初始化命令管理 | 对应管理模块实施时补齐 |
| 无邮箱验证和找回密码 | 注册邮箱未经所有权验证 | 邮件认证切片；公网开放注册前必须完成邮箱验证 |
| 无登录限流和注册防滥用 | 不适合直接暴露公网注册 | 公网开放注册前必须完成 |
| 无登录日志和操作日志 | 安全审计不完整 | 用户管理和 RBAC 完整落地前完成 |
| `/me` 只有身份信息 | 不承载菜单和权限 | RBAC 切片增加独立权限上下文契约 |

技术债不通过预留数据库字段、空接口、TODO 分支、兼容响应或假数据解决。到达回收节点时创建对应 spec、明确模型变更并删除临时代码。

以下事项不允许作为技术债延期：密码摘要、Token 轮换、Cookie 安全属性、Origin 校验、会话撤销、数据库约束、精确错误、TypeScript 类型安全和两套布局入口清理。

## 16. 后续顺序

```text
方案 A：认证闭环和基础布局
-> UI 基础统一整改
-> RBAC 表、权限中间件和后端动态菜单
-> 用户管理
-> 角色与菜单管理
-> 会话管理、登录日志和操作日志
-> 邮箱认证、找回密码、限流和注册防滥用
```

每一段继续使用独立 spec、plan 和验证，不在方案 A 中提前铺空目录或抽象。

## 17. 完成标准

- 四张表及其 PostgreSQL 约束与索引真实存在；
- 注册用户绑定 `registered_user`，Bootstrap 用户绑定 `super_admin`；
- 登录、刷新、退出、`/me` 和单会话在浏览器真实工作；
- Refresh 轮换和 session version 能拒绝旧 Token；
- Redis/PostgreSQL 故障不会产生假成功或内存兜底；
- Access Token 不落浏览器持久存储，Refresh Token 不暴露给 JavaScript；
- Router 只使用新的 `layout/index.vue`；
- 前端没有显式 `any`，后端没有越层访问；
- 定向测试、全量测试、构建和浏览器验收均通过；
- 一期技术债被记录，并具有明确回收节点。
