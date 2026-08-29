# Admin 模块化架构重构设计

状态：已实施并完成验证

日期：2026-08-29

## 1. 目的

本设计固定 Admin 项目下一阶段的目录、表归属、认证日志、RBAC 和数据库迁移基线。目标是
一次性把当前按大模块平铺的实现，收敛为“领域模块 -> 表对应子模块”的线性结构，使后续
维护者可以通过表名直接定位后端、前端和测试代码。

本次是架构重构，不是新业务开发。现有 Admin 功能、可见 URL、成熟页面信息结构和权限行为
原则上保持不变；变化只来自本设计明确列出的表名、日志语义、模块边界和删除项。

本设计同时吸收以下两个旧项目的成熟经验：

- Go 旧项目 `D:\github-project\admin_back_go` 的 `module/user`、`module/profile`、
  `module/auth`、`module/permission`、`module/systemlog` 以及嵌套子模块组织方式；
- PHP 旧项目 `D:\github-project\admin_back\app\module` 的 User、Permission、System
  模块入口和 AI、Payment 等业务模块的独立边界。

旧项目只提供已验证的业务行为、页面结构和定位方式，不复制旧项目的 `is_del`、重型 AI
分层、万能 Manager/Factory 或其他历史兼容负担。

## 2. 已确认的产品边界

### 2.1 当前项目负责什么

Admin 是专精管理端的独立前端和后端。当前范围包含：

- 管理员登录、登出、刷新会话、个人资料和修改密码；
- 用户账号、用户资料、用户登录日志和会话管理；
- 认证平台管理；
- 菜单、角色、用户角色关系和菜单授权管理；
- 当前平台的权限读取与 Redis 加速；
- 管理操作审计日志；
- 健康检查和现有 Worker 的操作日志消费。

一个邮箱代表一个全局账号。账号可以拥有多个全局角色，当前认证平台的权限由这些角色合并
并去重后返回。角色不是平台专属资源，菜单才拥有平台归属。

### 2.2 当前明确不做

以下内容不进入本次重构，也不在 Admin 中预留假模块、假菜单或占位 API：

- Canvas 业务前端和 Canvas 业务服务；
- AI 对话、Agent、Harness、知识库等业务。未来由独立项目外接开源 Agent/Harness；
- 支付业务；
- 腾讯云 COS 上传和文件管理；
- 邮件发送、短信发送；
- 地理位置和地址大表；
- WebSocket 业务通道；
- Admin 注册页面。

共享认证后端保留未来业务端可复用的注册能力和默认角色，但 Admin 前端不展示注册入口。
未来业务端、AI 项目、支付项目通过独立前端和独立模块接入，不把业务代码倒灌进 Admin。

### 2.3 旧项目模块核对结果

为避免重构时遗漏成熟能力，已核对旧 Go/PHP 项目的主要模块并作如下归类：

| 旧项目模块 | 本项目处理 |
| --- | --- |
| User、Profile、Auth、UsersLoginLog、UserSession | 本次进入 `user/*`、`auth/*` 和登录日志重构 |
| Permission、Role、System、SystemLog | 本次进入 `rbac/*`、`auth/platform` 和 `audit/operationlog` |
| Payment | 未来独立支付业务项目；当前不创建 Admin 支付菜单或表 |
| AI、Agent、Chat、Knowledge 等 | 未来独立 AI 项目，外接开源 Agent/Harness；当前不创建 Admin AI 模块 |
| Mail、SMS | 未来按实际服务接入；当前不实现发送服务，但账号邮箱/手机号字段保持可复用 |
| Upload、UploadConfig、UploadToken | 未来只接腾讯云 COS，不接 OSS；当前不创建上传模块 |
| Realtime、WebSocket | 未来业务端按真实需求接入；当前不创建 WebSocket 模块 |
| CronTask、Export、Notification、QueueMonitor、Book 等 | 当前没有已确认的 Admin 业务范围，另开 spec 后再接入 |

这张表是范围边界，不是要求现在为未来模块创建空目录或占位接口。

## 3. 现状与问题

当前后端 `server/internal/module` 直接平铺 `user`、`auth`、`role`、`menu`、`access`、
`operationlog`、`authplatform`、`account` 和 `taskdemo`。一个包同时承载多张表或多个不同
职责时，表、服务、路由和测试难以按领域定位；例如：

- `user` 同时拥有 `user_account` 和 `user_profile` 模型，并承载管理用户与个人中心；
- `auth` 同时拥有认证编排和 `auth_session` 持久化；
- `authplatform`、`access`、`operationlog` 的表名与包名不在同一领域层级；
- `taskdemo` 仍占据 API、Worker、首页和 `foundation_task` 表；
- `operationlog` 当前仍把登录、刷新和登出列入操作日志规则；
- `auth_session.platform`、`audit_operation_log.platform` 使用平台字符串，不能由外键保证
  平台存在；
- `EnsureSchema`、`PrepareSchema` 和 `AutoMigrate` 代码仍散落在模块中，容易被重新接入启动流程。

这些问题不是通过增加一个通用基础层解决，而是通过明确的领域目录、表级归属和显式迁移解决。

## 4. 不可偏移的工程原则

### 4.1 线性请求链路

普通数据库请求必须保持：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

各层职责固定：

- Router 只绑定 URL、中间件和 Handler；
- Middleware 只处理认证、平台、权限、request ID、CORS、访问日志、审计捕获和 Recovery；
- Handler 只绑定/校验请求、传递 request context、调用 Service 和输出统一响应；
- Service 负责业务规则、事务意图、状态变化和调用顺序；
- Repository 只执行 PostgreSQL 查询、写入、锁和事务；
- Model 只表达 GORM 与 PostgreSQL 映射。

Handler 不访问 GORM、Redis、Queue、Storage 或第三方 SDK；Service 不接收 Gin Context；
Repository 不处理 HTTP，不编排队列。所有入口 context 必须一路传递，不替换为
`context.Background()`。

### 4.2 不制造万能抽象

禁止引入 `BaseService`、`BaseRepository`、`BaseModel`、`Manager`、`Factory`、通用
`Adapter`、`infra` 万能目录、DI 容器、运行时注册器或为未来假设创建的接口。

只有多个真实实现或测试确实需要替换边界时，才定义覆盖当前需求的最小接口。模块内部优先
采用旧项目已经验证的清晰文件分工，但保留当前项目的严格 DTO、显式错误、PostgreSQL 和
`deleted_at` 规则。

### 4.3 运行时只读数据库结构

API 和 Worker 启动只负责加载配置、连接并检查依赖、组装模块、注册路由/任务和监听服务。
启动期间禁止：

- `AutoMigrate`、`PrepareSchema`、`EnsureSchema` 或任何等价 DDL；
- 创建、删除、重命名或修改表、列、索引、约束和序列；
- 回填数据、修复历史数据、预设平台/菜单/角色或补建访问版本；
- 清理、重建或主动失效 Redis key；
- 发布初始化、迁移或修复任务。

数据库变化只通过维护者明确执行的 forward SQL migration 完成。`bootstrap-admin` 仍是
独立的、由维护者显式执行的首个管理员创建命令，不由 API/Worker 调用。

## 5. 目标后端目录

目录只按真实领域和表名拆分；每个子模块按需拥有 `model.go`、`request.go`、`response.go`、
`repository.go`、`service.go`、`handler.go`、`route.go` 和测试文件，不要求每个目录机械生成
全部文件。

```text
server/internal/module/
  user/
    account/       # user_account；管理员用户管理、凭据和账号状态
    profile/       # user_profile；个人资料
    loginlog/      # user_login_log；登录/登出安全事件查询与落库
    session/       # user_session；会话持久化和管理员会话管理
  rbac/
    menu/          # rbac_menu；菜单树和页面/动作节点
    role/          # rbac_role；角色生命周期和默认角色
    userrole/      # rbac_user_role；全局用户-角色关系
    rolemenu/      # rbac_role_menu；角色-菜单授权关系
    accessversion/ # rbac_access_version；用户级权限版本
    access/        # 当前平台权限源、合并去重和快照
    state/         # RBAC 相关 Redis 状态和失效
  auth/
    login/         # 登录、注册编排、密码校验、Token 和登出流程
    platform/      # auth_platform；平台策略和管理接口
    client/        # 请求平台/设备客户端解析
    state/         # 认证 Redis 状态、会话代际和缓存
  audit/
    operationlog/  # audit_operation_log；管理业务变更审计
  health/          # 健康和就绪检查，无业务表
```

### 5.1 后端依赖方向

依赖方向固定为：

```text
auth/login
  -> user/account + user/session + user/loginlog
  -> auth/platform + auth/state
  -> rbac/access

rbac/access
  -> user/account + rbac/accessversion + rbac/userrole
  -> rbac/role + rbac/menu + rbac/rolemenu + auth/platform

audit/operationlog
  <- 管理路由审计中间件和操作日志 Worker
```

`user/*` 不反向依赖 `auth/login`，`rbac/*` 不依赖 HTTP Handler。登录编排可以调用账号和
会话子模块的明确方法，但账号模型不携带一组角色 ID，角色关系仍由 `rbac_user_role` 表维护。

### 5.2 现有包的迁移归属

| 当前包 | 目标归属 | 处理方式 |
| --- | --- | --- |
| `user` | `user/account`、`user/profile` | 按模型和职责拆分，保留现有用户管理行为 |
| `account` | `user/profile` + `auth/login` | 个人资料进入 profile，密码变更由登录编排负责 |
| `auth` | `auth/login`、`user/session` | Token/认证留 login，会话模型/仓储/管理查询进 session |
| `authplatform` | `auth/platform` | 仅改目录和引用，保留平台策略行为 |
| `authclient` | `auth/client` | 保留请求平台、设备和客户端解析 |
| `authstate` | `auth/state` | 保留认证状态键和代际失效 |
| `menu` | `rbac/menu` | 保留菜单协议和树校验，删除启动 foundation |
| `role` | `rbac/role`、`rbac/userrole`、`rbac/rolemenu` | 角色、用户角色、角色菜单关系按表拆分 |
| `access` | `rbac/access`、`rbac/accessversion`、`rbac/state` | 权限计算、版本表和 Redis 状态拆分 |
| `operationlog` | `audit/operationlog` | 只保留管理业务审计和其 Worker 消费 |
| `taskdemo` | 无 | 整体删除 |

## 6. 目标前端目录

前端采用与后端同名的领域目录。HTTP 数据流保持：

```text
view -> api/<module>.ts -> utils/request.ts -> Go API
```

目标结构如下：

```text
web/src/
  modules/
    user/
      account/       # 用户列表、账号状态、角色分配入口
      profile/       # 当前用户资料和密码入口
      loginlog/      # 登录日志页面
      session/       # 会话列表和踢除
    rbac/
      menu/          # 菜单管理
      role/          # 角色和权限矩阵
      access/        # 当前权限读取和路由构建
    auth/
      login/         # 登录页及认证状态
      platform/      # 认证平台管理
    audit/
      operationlog/  # 操作日志页面
  api/               # 业务 API 文件按领域归位，不放 Axios 基础实现
  utils/
    request.ts       # 现有 Axios/envelope 基础，不放业务 API
  layout/            # 现有布局、RouterTabs、主题和个人中心
  components/        # 现有真实复用公共组件
```

迁移可以先移动文件再调整 import，但必须保持用户可见的 route path、route name、权限码、
HTTP URL 和响应 envelope。已有 Element Plus、SCSS、中文/英文持久化、主题和 RouterTabs
行为不在本设计中重新设计；重构只把页面、API、类型和测试按领域归位。

## 7. 表与模型基线

所有项目维护的 PostgreSQL 表都显式声明 `created_at TIMESTAMPTZ NOT NULL` 和
`updated_at TIMESTAMPTZ NOT NULL`。有真实删除行为的表使用 `deleted_at TIMESTAMPTZ NULL`
和 GORM `DeletedAt`；日志、事件和不可变记录不增加删除状态。

### 7.1 `user_account`

`user/account` 是全局账号唯一事实来源，固定包含：

- `id`；
- `username`；
- 规范化的 `email`；
- 可空 `phone`；手机号现在保留在主账号表，为未来短信登录和业务端复用预留；
- `password_hash`；
- `is_enabled`；
- `created_at`、`updated_at`、`deleted_at`。

邮箱是全局账号标识，当前登录使用邮箱和密码。手机号不是本轮业务能力，但有效账号的手机号
必须通过部分唯一索引保证唯一。用户名、邮箱和手机号的唯一性只针对未软删除账号。

### 7.2 `user_profile`

`user/profile` 只保存非认证资料：

```text
user_id BIGINT PRIMARY KEY REFERENCES user_account(id) ON DELETE RESTRICT
birthday DATE NULL
gender SMALLINT NOT NULL DEFAULT 0 CHECK (gender IN (0, 1, 2))
created_at TIMESTAMPTZ NOT NULL
updated_at TIMESTAMPTZ NOT NULL
```

这是严格的一对一关系。资料不存在时由显式 migration 或明确的资料写入流程创建，不在 API
启动时偷偷补行。

### 7.3 `user_session`

当前 `auth_session` 迁移为 `user_session`，由 `user/session` 负责。字段保留现有会话行为，
并将平台字符串替换为外键：

- `user_id -> user_account(id)`；
- `platform_id BIGINT NOT NULL -> auth_platform(id)`；
- `device_id`；
- `refresh_token_hash`，只存 hash，不存 Token 原文；
- `version`；
- `client_ip`、`user_agent`；
- `refresh_expires_at`、`revoked_at`；
- `created_at`、`updated_at`。

刷新只做 hash 原子轮换、会话版本和 Redis 状态更新，不产生登录日志或操作日志。

### 7.4 `user_login_log`

`user/loginlog` 新建并拥有 `user_login_log`。它是认证安全事件事实表，不是业务操作审计表。
字段和约束固定为：

```text
id             BIGINT PRIMARY KEY
user_id        BIGINT NULL REFERENCES user_account(id) ON DELETE RESTRICT
session_id     BIGINT NULL REFERENCES user_session(id) ON DELETE RESTRICT
platform_id    BIGINT NOT NULL REFERENCES auth_platform(id) ON DELETE RESTRICT
login_account  VARCHAR(254) NOT NULL
event_type     VARCHAR(16) NOT NULL CHECK (event_type IN ('login', 'logout'))
login_type     VARCHAR(32) NULL
               CHECK ((event_type = 'login' AND login_type IS NOT NULL)
                   OR (event_type = 'logout' AND login_type IS NULL))
is_success     SMALLINT NOT NULL CHECK (is_success IN (0, 1))
reason_code    VARCHAR(64) NOT NULL
client_ip      VARCHAR(64) NOT NULL
user_agent     VARCHAR(512) NOT NULL
created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
updated_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
```

`login_type` 在 `event_type = 'login'` 时记录当前支持的 `password`，登出时为空；未来新增
方式必须通过协议和 migration 明确加入。`user_id` 和 `session_id` 可以为空，以容纳未知账号
的失败登录；`login_account` 保存规范化后的登录账号候选值，不保存密码、验证码或 Token。
`reason_code` 是机器可读原因码，不写秘密和原始异常。

登录失败可以没有 `user_id` 和 `session_id`，但仍记录平台、账号候选值、结果和原因。成功登录
与登出在会话状态变更成功后写入；写入失败必须显式返回错误或回滚对应会话事务，禁止假装成功。

以下事件明确不写入 `user_login_log`：刷新 Token、Token 轮换、Redis 指针更新、设备状态
重建和其他缓存维护。

### 7.5 RBAC 表

保留并按表拆分以下关系：

```text
rbac_role            -> rbac/role
rbac_user_role       -> rbac/userrole
rbac_menu            -> rbac/menu
rbac_role_menu       -> rbac/rolemenu
rbac_access_version  -> rbac/accessversion
```

`rbac_role` 是全局角色，包含 `code`、`name`、`is_default`、`is_enabled`、时间戳和
`deleted_at`。默认角色仍是全局唯一有效角色，用于未来业务端注册后的初始绑定。

`rbac_user_role` 只含 `user_id`、`role_id` 和记录字段；用户可以拥有多个角色，不在表中添加
冗余 `platform_id`。

`rbac_role_menu` 只含 `role_id`、`menu_id` 和记录字段；菜单已经携带平台，授权关系不重复
保存平台 ID，跨平台错误由菜单外键和访问查询共同阻断。

`rbac_menu` 的 `platform_id` 为非空外键，父菜单使用 `(parent_id, platform_id)` 复合外键，
确保父子菜单属于同一平台。根级 page 合法，Canvas 未来可以拥有无目录的 test page，但
本次不创建 Canvas 业务代码或启动期预设。

`rbac_access_version` 保持 `(user_id)` 单列主键，一行代表一个用户的全局权限代际。它不是
权限数据，也不需要 `platform_id`：任一角色、用户角色或菜单授权变化都递增该用户版本，
一次失效其所有平台快照。这样缓存键仍按 `(user_id, platform_id, version)` 隔离，避免为了
平台粒度引入需要同步多行的复杂度。

### 7.6 `auth_platform`

`auth/platform` 负责 `auth_platform` 的策略、TTL、设备/IP 绑定、最大会话数、注册开关和
启用状态。平台 code 全局唯一，平台表是 `platform_id` 外键的权威字典。

Admin 平台的注册开关继续关闭。共享 `/api/v1/auth/register` 能力保留给未来允许注册的平台，
但 Admin 前端不提供注册 UI。已有平台、Canvas 数据和历史策略由 migration 保留，API/Worker
启动不创建或修复它们。

### 7.7 `audit_operation_log`

`audit/operationlog` 保留不可变操作审计表，字段中的 `platform` 存储列改为可空
`platform_id -> auth_platform(id)`，其余事件 ID、请求 ID、用户/会话、方法、路由、模块、
动作、客户端摘要、状态码、成功标记、耗时、脱敏请求/响应 JSON 和时间字段保留。

操作日志只记录 Admin 管理业务变更，例如：

- 菜单创建、修改、启停和删除；
- 角色创建、修改、启停、默认角色变更和权限分配；
- 用户账号修改、启停、删除和角色分配；
- 认证平台策略变更；
- 管理员踢除其他会话。
- 管理员或用户修改密码时，只记录动作和结果，不记录密码或密码哈希。

登录、注册、刷新、登出、密码校验和 Redis 状态维护不进入操作日志。密码修改是账号业务
变更，可以记录 `account.password.change` 动作，但不得把它当成登录事件，也不得记录任何
认证秘密。

## 8. 外键、索引和删除语义

### 8.1 外键原则

所有真实关系由 PostgreSQL 外键表达，不在 Service 里维护一套容易漂移的手工关系：

```text
user_profile.user_id          -> user_account.id       RESTRICT
user_session.user_id          -> user_account.id       RESTRICT
user_session.platform_id      -> auth_platform.id     RESTRICT
user_login_log.user_id        -> user_account.id       RESTRICT
user_login_log.session_id     -> user_session.id      RESTRICT
user_login_log.platform_id    -> auth_platform.id     RESTRICT
rbac_user_role.user_id        -> user_account.id       RESTRICT
rbac_user_role.role_id        -> rbac_role.id          RESTRICT
rbac_role_menu.role_id        -> rbac_role.id          RESTRICT
rbac_role_menu.menu_id        -> rbac_menu.id          RESTRICT
rbac_menu.platform_id         -> auth_platform.id     RESTRICT
rbac_menu(parent_id,platform_id)
                               -> rbac_menu(id,platform_id) RESTRICT
rbac_access_version.user_id   -> user_account.id       RESTRICT
audit_operation_log.user_id   -> user_account.id       RESTRICT
audit_operation_log.session_id
                               -> user_session.id       RESTRICT
audit_operation_log.platform_id
                               -> auth_platform.id      RESTRICT
```

账号、平台、角色和菜单使用软删除；在存在审计、会话或授权引用时禁止物理删除。这样既保留
历史可追溯性，也避免用重复的 `is_deleted` 字段模拟外键关系。

### 8.2 只为真实查询建立索引

migration 固定验证并建立以下索引，名称固定、定义可审计：

- `user_account`：有效 `lower(username)`、有效 `lower(email)` 和有效非空 `phone` 部分唯一索引；
- `user_session`：`refresh_token_hash` 唯一索引；用户创建时间索引；有效会话的
  `(user_id, platform_id, created_at DESC, id DESC)` 部分索引；
- `user_login_log`：按创建时间倒序分页索引；按 `user_id`、`platform_id` 和
  `login_account` + 创建时间筛选的索引，只保留页面实际使用的组合；
- `auth_platform`：有效 code 部分唯一索引；
- `rbac_role`：有效 code/name 唯一索引，`is_default = 1` 的有效部分唯一索引；
- `rbac_user_role`：有效 `(user_id, role_id)` 部分唯一索引；
- `rbac_menu`：有效 `(platform_id, code)` 唯一索引、页面 path 唯一索引、
  `(platform_id, parent_id, sort_order, id)` 树查询索引；
- `rbac_role_menu`：有效 `(role_id, menu_id)` 部分唯一索引；
- `audit_operation_log`：event ID 唯一索引、request ID 普通索引、创建时间、用户和动作
  查询索引；平台筛选有真实页面需求时建立 `(platform_id, created_at DESC)`。

不为 `is_enabled`、`is_success`、`deleted_at` 单独创建低选择性索引，不复制旧项目无真实查询
依据的索引。

## 9. 认证、权限与日志流程

### 9.1 登录

1. `auth/client` 解析请求平台、设备、IP 和客户端摘要；平台 code 必须解析到
   `auth_platform.id`。
2. `auth/login` 规范化邮箱，调用 `user/account` 读取账号并校验密码哈希与启用状态。
3. 认证策略校验通过后，在事务中创建/轮换 `user_session`，只保存 refresh hash。
4. 成功登录事务写入一条 `user_login_log(event_type=login)`；失败登录单独写入失败事件且不创建会话。
5. 认证成功后发布必要的 Redis 会话/权限状态；Redis 只做加速和代际状态，不成为事实来源。

登录日志写入失败不能被吞掉。依赖不可用时沿统一错误 envelope 返回明确错误，不返回空凭据。

### 9.2 刷新

刷新只验证 refresh hash、原子轮换 `user_session`、更新认证 Redis 状态并返回新凭据。不会
写 `user_login_log`，不会写 `audit_operation_log`，不会把 Token、Cookie 或 hash 写入普通日志。

### 9.3 登出

登出撤销当前会话，并写入一条 `user_login_log(event_type=logout)`。进入 Logout Service 的
有效身份请求按幂等成功处理：会话已经撤销时不重复改变状态，但仍记录一次成功的 logout
事件；撤销和日志写入在同一事务意图中完成。未通过认证的请求在 Middleware 阶段被拒绝，
不产生成功 logout 事件。任何情况下都不产生 refresh 日志。

### 9.4 当前平台权限

`rbac/access` 接收认证身份中的 `user_id` 和 `platform_id`：

1. 读取启用且未删除的用户及其用户级 `rbac_access_version`；
2. 读取所有有效启用角色，按 code 去重；
3. 查询 `rbac_menu` 时第一条件必须是当前 `platform_id`；
4. `super_admin` 只获得当前平台的有效菜单和动作，不获得其他平台节点；
5. 普通角色从 `rbac_role_menu` 读取当前平台菜单授权，补齐必要的有效祖先目录；
6. 按 menu ID/code 合并去重，构建树和 permission code；
7. Redis 快照键包含 `user_id`、`platform_id` 和用户级 version，任何 RBAC 变更递增版本。

角色没有 `platform_id` 并不是漏洞：平台过滤发生在菜单源查询和授权 join 上，菜单与父菜单的
复合外键又阻止跨平台父子关系。`/api/v1/access` 绝不返回当前平台之外的菜单、权限码或角色
权限详情。

## 10. API、路由与 DTO 兼容策略

### 10.1 URL 命名空间

- 共享认证与权限：`/api/v1`；
- Admin 管理资源：`/api/admin/v1`；
- 健康检查：保留现有 `/health`、`/ready`。

本次目录重构不改变现有 URL、HTTP 方法、权限码、响应 envelope（`code`、`data`、`message`）。
Admin 注册入口继续不存在。

### 10.2 platform 字段边界

数据库列、Go Model、Repository 查询和 migration 统一使用 `platform_id`。HTTP DTO 不得
直接泄露数据库内部命名：

- 需要提交平台身份的请求使用 `platformId`；
- 需要展示平台的响应可以按既有页面契约返回平台 code/name，若需要 ID 则使用 `platformId`；
- 不为了兼容错误字段同时输出 `platform` 和 `platformId`，新增或明确修改的协议只保留一个语义
  清晰的字段。

这样既固定了数据库外键设计，又避免仅为改包名而无谓破坏用户已经验收的页面协议。

### 10.3 统一错误与敏感数据

Handler 只输出公开错误 code/message。响应、请求日志、操作日志和登录日志不得包含：

- 密码、密码哈希、验证码；
- Access/Refresh Token、Cookie、Authorization；
- Secret、密钥、DSN、SQL、堆栈；
- 完整第三方请求凭据。

## 11. 显式数据库 migration

实现阶段增加一份人工执行的 forward SQL migration，并在数据库文档中记录执行顺序、回滚
策略和验证 SQL。API/Worker 不调用该文件。migration 必须可重复执行：已经完成的重命名、
列、约束和索引由显式检查识别并跳过；发现新旧对象同时存在、数据不一致或未知映射时直接
失败，不覆盖数据。项目不引入运行时迁移框架或为了记录迁移而新增万能表。

### 11.1 migration 顺序

1. 在事务开始前检查当前 schema、重复邮箱/用户名/手机号、未知平台 code、重复活动约束和
   `foundation_task` 是否仍被其他表引用；任何冲突直接失败，不猜测、不静默修复。
2. 创建 `user_profile`（若已由既有明确 migration 创建，则只验证结构和约束）。
3. 将 `auth_session` 重命名为 `user_session`；若新旧表同时存在则失败，不覆盖任一表。
4. 在 `user_session` 中将 `platform` 字符串映射为 `auth_platform.id`，未知或已损坏的值
   直接回滚；添加 `platform_id` 外键后再删除旧列和旧索引。
5. 创建 `user_login_log`，不把历史 `audit_operation_log` 猜测性改写为登录事件。
6. 将 `audit_operation_log.platform` 映射为 `platform_id`；空值保持空，未知非空值回滚；
   添加外键并删除旧列。
7. 验证并补齐 `rbac_access_version` 缺失行、外键、检查约束和索引。该补齐只在维护者显式
   执行 migration 时发生，不再由 `EnsureSchema` 或启动流程发生。
8. 显式删除 demo 专属 `foundation_task` 表；如果发现非 demo 外键引用则先失败并要求人工处理。
9. 创建新约束和索引，删除已被新定义取代的旧约束/索引，输出表计数、外键和索引验证结果。

### 11.2 数据保留

迁移必须保留现有用户、资料、角色、菜单、授权、平台、会话和操作审计数据及其 ID。历史
操作日志保持原样，即使其中存在旧的认证路由记录，也不把不可验证的历史事件伪造成新的
`user_login_log`。会话平台映射失败时宁可整批回滚，也不能把平台猜成 `admin`。

### 11.3 启动代码收敛

migration 合入后，从生产组合根删除所有 schema prepare/ensure、菜单 foundation、平台
预设、访问版本回填和 Redis 清理调用。只被这些调用使用且无真实运行时用途的死代码删除；
测试 schema builder 可以保留在测试目录，但不得被 API/Worker 引用。

## 12. taskdemo 删除范围

一次性移除以下内容：

- `server/internal/module/taskdemo` 及其测试；
- API 和 Worker 对 `taskdemo` 的依赖、路由、任务类型、队列处理器和 composition wiring；
- `web/src/api/taskDemo.ts`、首页示例任务表单/按钮和相关 i18n；
- `web/tests/api/taskDemo.test.ts`、dashboard 示例任务测试；
- README 和其他文档中的示例任务、`foundation_task`、AutoMigrate 启动说明。

Worker 保留操作日志任务消费链路：

```text
operation log task handler -> operationlog service -> repository -> audit_operation_log
```

不因为删除 demo 而删除 Queue 基础设施；正式业务任务未来另行设计，当前不引入 Outbox。

## 13. 测试与验收

### 13.1 后端

- 每个目标表模块拥有 Model/TableName、Repository、Service、Handler 和路由的定向测试；
- migration 在隔离 PostgreSQL schema 上执行，验证表、列、约束、外键、索引、数据计数和
  历史 ID 保留；
- 登录成功/失败、登出、刷新分别验证日志落点，确认 refresh 不写任何登录/操作日志；
- 操作日志规则只匹配管理业务变更，认证路由不入队；
- 多角色合并、超级管理员当前平台限定、跨平台菜单不可见、缓存版本失效有集成测试；
- API/Worker 启动测试确认不执行 DDL、seed、数据回填、foundation 删除或 Redis 清理；
- `go fmt ./...`、`go vet ./...`、`go test ./...`、`go build ./...` 全部通过。

### 13.2 前端

- 迁移后的模块 API、DTO、Pinia、路由和页面测试保持镜像目录；
- 运行时搜索不得残留 `taskdemo`、`example-tasks`、旧 `auth_session`、错误的 DB `platform`
  查询或旧包 import；
- 保持 `/api/admin/v1`、`/api/v1`、菜单平台 tabs、权限矩阵、个人中心和 RouterTabs 行为；
- `pnpm vitest run --pool=threads --maxWorkers=1` 和 `pnpm build` 通过。

### 13.3 人工验收重点

1. 现有 Admin 数据库执行 migration 后，用户、角色、菜单、会话和操作日志仍可查询；
2. 重启 API/Worker 不产生任何表结构或业务数据变化；
3. Admin 登录、刷新、登出和个人中心可用，登录日志能区分成功/失败/登出；
4. 切换当前平台时 `/access` 只返回该平台的菜单和权限；
5. 超级管理员拥有当前 Admin 全部菜单，但看不到 Canvas 或其他平台权限；
6. 菜单、角色、用户和平台管理变更进入操作日志，认证事件不进入操作日志；
7. 首页不再有示例任务，Worker 仍能消费操作日志任务。

## 14. 实施顺序

本 spec 批准后，另行编写 implementation plan，按以下顺序实施：

1. 先落显式 SQL migration 和迁移验证工具/测试；
2. 移动后端表模块并修复依赖方向；
3. 收敛认证日志、操作日志和启动组合根；
4. 删除 taskdemo、foundation_task 和首页示例任务；
5. 按同一领域树迁移前端 API、类型、页面和测试；
6. 完成全量验证、README/文档更新和人工验收。

任何实现过程中发现的业务行为变化、字段新增或未来 COS/邮件/短信/AI/支付接入，必须另
开对应 spec，不得借本次目录重构顺便扩大范围。

## 15. 实施验证记录

- 人工执行 `docs/database/2026-08-29-admin-modular-architecture.sql` 前已备份本地 PostgreSQL `admin` 数据库；脚本在真实库中幂等执行成功。
- 真实库验证结果：`user_account=2`、`user_profile=1`、`user_session=25`、`user_login_log=0`、`audit_operation_log=120`；账号、会话和审计历史数据保留。
- `go fmt ./...`、`go vet ./...`、`go test ./... -count=1`、`go build ./...` 全部通过。
- `pnpm vitest run --pool=threads --maxWorkers=1 --testTimeout=10000 --hookTimeout=10000` 通过 51 个测试文件、358 个测试；`pnpm build` 通过。
- API/Worker 启动源码未发现 DDL、seed、回填、foundation 删除或 Redis 清理入口；Worker 仅保留操作日志任务消费。
- 当前目录树、`/api/v1` 与 `/api/admin/v1` 路由边界、登录日志、RBAC 平台隔离和前端 RouterTabs 行为按既定清单完成。
