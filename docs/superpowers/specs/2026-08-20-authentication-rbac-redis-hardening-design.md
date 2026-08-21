# Admin 认证平台与 Redis 加固设计

日期：2026-08-20

状态：设计已确认，实施计划已完成，等待执行

## 1. 目的

本设计一次回收两个已经确认的技术债，并落地多平台认证策略管理：

1. 认证请求不再在 Redis 命中后仍查询 PostgreSQL 会话；
2. RBAC 不再为每个权限请求直接查询 PostgreSQL；
3. 管理员可以在“认证平台”页面维护真实生效的认证、Token、会话和缓存策略；
4. PostgreSQL 始终保存认证会话、认证平台和 RBAC 的权威数据；
5. Redis 只保存可重建的派生快照和短期一致性状态，不成为保真层；
6. 数据流继续保持具体、显式和线性，不复制旧项目的历史架构。

本设计参考 `D:\github-project\admin_back_go` 的 Session Snapshot、Principal Snapshot 和
认证平台实现，以及 `D:\github-project\admin_front_ts` 的认证平台页面。参考只用于提取
已经验证过的行为，不原样复制旧项目的单角色、多平台主体模型、通用基础设施或兼容负担。

本设计替代以下旧契约：

- `2026-08-17-admin-authentication-foundation-design.md` 中固定 Access/Refresh TTL、全局单会话、
  单值 Redis 当前会话指针和 Redis 错误直接失败的读取语义；
- `2026-08-18-backend-i18n-rbac-foundation-design.md` 中 RBAC 每次直接读取 PostgreSQL 的临时实现；
- `2026-08-20-user-management-design.md` 中只删除 `auth:current-session:<userID>` 的临时会话
  清理方式。

未被本设计明确替代的密码摘要、根密钥派生、Refresh Token 轮换、Origin、Cookie 安全、
超级管理员保护、RBAC 权限推导和前端严格 TypeScript 契约继续有效。

## 2. 已确认决策

- 本切片同时实施认证 Session Redis 加固、RBAC Access Redis 加固和认证平台管理；
- 认证平台从第一期开始支持多条平台记录，不先做单例再迁移；
- `admin` 是内置认证平台，code 不可修改且不可删除；
- 认证平台只管理认证与会话策略，不绑定用户或角色；
- 用户登录后可见菜单和接口权限仍完全由 RBAC 决定；
- 请求使用 `X-Auth-Platform` 明确选择平台，后端不猜平台；
- 前端生成并持久化 UUID，通过 `X-Device-ID` 发送客户端实例标识；
- IP 绑定开启时，登录、刷新和受保护请求都要求精确 IP 匹配；
- 会话上限按 `platform + user_id` 独立计算；
- 超过上限时保留最新会话并撤销最旧的超额会话；
- PostgreSQL 是会话、平台策略、用户角色和权限的最终权威数据；
- Redis miss、读取错误或缓存损坏时，明确记录原因并读取 PostgreSQL；
- Redis 可用且缓存命中时，认证和权限判断不查询 PostgreSQL；
- 影响缓存事实的写操作必须先建立 Redis 失效状态，失败时不得进入 PostgreSQL 写事务；
- 权限变更后，目标用户的下一次权限请求不得接受旧权限；
- 不增加进程级认证或权限缓存；
- `AUTH_COOKIE_SECURE`、`CORS_ORIGIN`、可信代理和 Redis 连接属于部署配置，不由数据库热修改；
- 登录方式、验证码类型和邮箱免注册登录作为明确技术债，不创建无运行时用途的字段。

## 3. 范围

### 3.1 包含

- 多平台认证策略表、模型、约束、初始化和管理接口；
- 内置 `admin` 认证平台；
- 平台级 Access TTL、Refresh TTL、Session Cache TTL 和 Access Cache TTL；
- 平台级设备绑定、IP 绑定、最大会话数、注册开关和启停状态；
- 会话表的平台和设备字段、多会话索引及既有会话安全迁移；
- JWT platform claim、平台 Cookie、请求平台和设备 Header；
- Redis Policy、User State、Session State、Session Snapshot、Access State 和 Access Snapshot；
- Redis 失效状态、版本栅栏、写入租约和中断恢复；
- RBAC mutation 的精确版本推进范围；
- 认证平台动态菜单、权限码、分页列表和 Element Plus 管理页面；
- 中英文后端错误、前端文案和严格 TypeScript DTO；
- 真实 PostgreSQL、Redis 并发和故障恢复测试。

### 3.2 不包含

- 邮箱验证码登录、短信验证码登录、手机号登录和第三方登录；
- 验证码类型、验证码挑战和登录方式选择；
- 免注册邮箱登录；
- 认证平台与角色、用户的绑定关系；
- 管理员会话列表和踢出用户页面；
- 登录日志、持久化操作日志和安全事件中心；
- APP_SECRET、Cookie Secure、CORS、Redis URL 或可信代理的在线修改；
- 进程级 L1 缓存；
- 通用 Cache Manager、Adapter、BaseService、BaseRepository 或事件总线；
- 前端全局 AppDialog、AppTable 或通用 CRUD 回收。

## 4. 线性数据流

### 4.1 认证读取

```text
router
-> auth middleware 读取 Header、Bearer Token 和客户端 IP
-> auth service 本地校验 JWT
-> auth Redis 读取 Policy、User State、Session State 和 Session Snapshot
-> Redis 命中且状态一致：返回 Identity
-> Redis miss/error：auth repository 查询 PostgreSQL 权威会话和用户状态
-> auth service 重建可用 Redis 快照
-> middleware 将 Identity 写入 Gin Context
```

Handler 和 Middleware 不访问 GORM。Service 不依赖 Gin。Auth Repository 只访问 PostgreSQL。
Auth Redis 文件只实现本设计使用的具体 key 和原子操作，不引入通用缓存接口包。

### 4.2 RBAC 读取

```text
router
-> auth middleware
-> access middleware
-> access service 读取 Redis Access State
-> 按当前 version 读取完整 Access Snapshot
-> Redis 命中：在内存中的 permissionCodes 集合判断当前 permission code
-> Redis miss/error：access repository 查询 PostgreSQL 权威角色、菜单和授权
-> access service 构建并回填完整 Snapshot
-> 允许或拒绝当前请求
```

`GET /api/v1/access` 与权限 Middleware 必须调用同一份 Access Snapshot 读取逻辑，不能一个
读缓存、另一个直接查库。这样前端显示的菜单/按钮和后端接口判断来自同一事实。

### 4.3 前端

```text
view
-> api/<module>.ts
-> utils/request.ts 添加 X-Auth-Platform、X-Device-ID、Accept-Language 和 Bearer Token
-> Go API
```

认证平台页面继续把业务状态放在 `views/system/auth-platforms` 中，不增加全局 CRUD 框架。

## 5. PostgreSQL 数据模型

所有 PostgreSQL 名称使用 lower snake case。所有 Go 模型显式声明 `CreatedAt` 和
`UpdatedAt`，字段为非空 `TIMESTAMPTZ`。有真实删除行为的认证平台只使用
`gorm.DeletedAt`/`deleted_at`，不增加 `is_del`。

### 5.1 `sys_auth_platform`

| 字段 | 类型 | 约束与用途 |
| --- | --- | --- |
| `id` | `BIGINT` | 主键，自增 |
| `code` | `VARCHAR(49)` | 平台稳定编码，有效记录唯一 |
| `name` | `VARCHAR(64)` | 展示名称 |
| `policy_version` | `BIGINT` | 必填，默认 1，必须大于等于 1 |
| `access_ttl_seconds` | `INTEGER` | Access Token TTL，60 到 2592000 |
| `refresh_ttl_seconds` | `INTEGER` | Refresh Token TTL，60 到 31536000 |
| `session_cache_ttl_seconds` | `INTEGER` | Session Snapshot TTL，60 到 86400 |
| `access_cache_ttl_seconds` | `INTEGER` | Access Snapshot TTL，60 到 86400 |
| `bind_device` | `SMALLINT` | 0=否，1=是 |
| `bind_ip` | `SMALLINT` | 0=否，1=是 |
| `max_sessions` | `SMALLINT` | 0=不限，1=单会话，2 到 100=最新 N 个 |
| `allow_register` | `SMALLINT` | 0=否，1=是 |
| `is_enabled` | `SMALLINT` | 0=否，1=是 |
| `is_builtin` | `SMALLINT` | 0=否，1=是 |
| `created_at` | `TIMESTAMPTZ` | 必填，默认当前时间 |
| `updated_at` | `TIMESTAMPTZ` | 必填，默认当前时间 |
| `deleted_at` | `TIMESTAMPTZ` | 可空，软删除时间 |

平台 code 使用正则 `^[a-z][a-z0-9_]{1,48}$`，保存后不可修改。有效 code 使用部分唯一索引：

```sql
CREATE UNIQUE INDEX ux_sys_auth_platform_code_active
ON sys_auth_platform (code)
WHERE deleted_at IS NULL;
```

所有 Yes/No 字段使用项目 `0=No, 1=Yes`，并具有明确 CHECK 约束。`max_sessions`、四个 TTL
和 `policy_version` 同样具有数据库 CHECK 约束。

API 启动时确保以下内置记录存在并严格校验：

```text
code=admin
name=Admin
accessTTL=900
refreshTTL=1209600
sessionCacheTTL=1800
accessCacheTTL=1800
bindDevice=0
bindIP=0
maxSessions=1
allowRegister=1
isEnabled=1
isBuiltin=1
```

初始化只补缺失的内置记录，不静默覆盖已有值。已有 `admin` 记录违反内置约束时 API 启动
失败。普通平台可以新增、编辑、禁用和软删除；`admin` 不可删除且 code 不可修改。

`policy_version` 在每次真实平台 mutation 中递增。它用于防止旧 Policy 或 Session Snapshot
复活，不是面向 API 客户端的乐观锁字段。

### 5.2 `sys_user_session`

在现有字段基础上增加：

| 字段 | 类型 | 约束与用途 |
| --- | --- | --- |
| `platform` | `VARCHAR(49)` | 必填，签发会话的平台 code |
| `device_id` | `VARCHAR(36)` | 新会话必填，规范 UUID 文本 |

继续使用现有 `version`、`client_ip`、`user_agent`、`refresh_expires_at`、`revoked_at`、
`created_at` 和 `updated_at`。不增加 `access_expires_at`，因为 Access Token 的绝对过期时间
已经由 JWT claim 表达；不增加 `deleted_at`，因为会话历史由 `revoked_at` 表达安全生命周期。

删除原来的全局单会话部分唯一索引：

```text
ux_sys_user_session_current ON (user_id) WHERE revoked_at IS NULL
```

新增真实查询所需索引：

```sql
CREATE INDEX ix_sys_user_session_user_platform_active
ON sys_user_session (user_id, platform, created_at DESC, id DESC)
WHERE revoked_at IS NULL;
```

多会话上限通过锁定用户行后的 PostgreSQL 事务计算和执行，不使用数据库唯一索引假装表达
动态 N 值。事务按 `created_at DESC, id DESC` 保留最新 N 个，其余设置 `revoked_at`。

### 5.3 `sys_access_version`

| 字段 | 类型 | 约束与用途 |
| --- | --- | --- |
| `user_id` | `BIGINT` | 主键，外键到 `sys_user` |
| `version` | `BIGINT` | 必填，默认 1，必须大于等于 1 |
| `created_at` | `TIMESTAMPTZ` | 必填 |
| `updated_at` | `TIMESTAMPTZ` | 必填 |

该表没有真实删除行为，不增加 `deleted_at`。现有用户在 schema 初始化时以幂等 SQL 补齐
version=1；新用户在用户创建事务中创建对应版本记录。

`access_version` 是权限版本栅栏，不是传统乐观锁。客户端不提交该值，Repository 不使用它
解决一般 CRUD 并发覆盖。

### 5.4 迁移语义

当前 JWT 不包含 platform，当前会话也没有可信 device ID。本设计不接受兼容 Header、猜测
平台或伪造设备标识。部署本变更时执行一次明确迁移：

1. 创建并校验 `sys_auth_platform` 与内置 `admin`；
2. 为历史 `sys_user_session` 增加 `platform='admin'` 和 `device_id=''`；
3. 撤销所有迁移前仍未撤销的会话；
4. 删除旧单会话唯一索引并创建多平台索引；
5. 创建并回填 `sys_access_version`；
6. 清理旧 `auth:current-session:*` Redis key；
7. 用户重新登录后获得带 platform claim 和真实 UUID 的新会话。

历史会话允许保存空 device ID 仅用于审计，Service 不允许创建空 device ID 的新会话。旧
Access/Refresh Token 在部署后全部失效，这是一次明确的安全迁移，不提供兼容读取路径。

AutoMigrate 不负责删除旧索引或执行数据回填。上述迁移使用相邻、幂等、可测试的明确 DDL
和数据变更，不引入通用 migration 框架。

## 6. 认证平台运行时策略

### 6.1 平台选择

所有认证接口和受保护 API 请求都携带：

```text
X-Auth-Platform: admin
X-Device-ID: 550e8400-e29b-41d4-a716-446655440000
```

平台 Header 必须符合平台 code 规则。Device ID 必须是规范 UUID 文本。后端使用 Header 的
完整 values 列表要求每个字段恰好出现一次；缺失、重复、空值或格式错误都明确拒绝，不从
Origin、URL、Cookie 名或 User-Agent 猜测。公开的 `/health` 和 `/ready` 不要求这两个 Header，
保证负载均衡和本机探针可以直接检查服务状态。

JWT 增加 `platform` claim，只包含 issuer、user ID、session ID、platform、session version、
issued-at、not-before 和 expiry。请求 Header 与 JWT platform 不一致时返回未认证。Token 不能
跨平台使用。

### 6.2 Token TTL

- Access Token 初次登录和每次刷新时使用当前平台 `access_ttl_seconds`；
- Refresh Token 初次登录时使用当前平台 `refresh_ttl_seconds`；
- Refresh 轮换不延长原始 `refresh_expires_at`；
- 修改 Access/Refresh TTL 只影响新签发的 Token，不重写现有绝对过期时间；
- JWT 签名和时间校验始终由 Go 本地执行，Redis 不负责验证 JWT 签名。

### 6.3 设备与 IP

前端第一次启动时使用 `crypto.randomUUID()` 生成 Device ID，并存入专用 localStorage key。
只持久化该随机 ID，不持久化 Access Token。存储值缺失或损坏时明确生成新 UUID；生成能力
不可用时前端明确失败，不使用时间戳或固定字符串代替随机 ID。

登录始终记录 Device ID 和服务端解析后的客户端 IP：

- `bind_device=1` 时，刷新和每次认证要求 Header Device ID 与会话完全一致；
- `bind_ip=1` 时，刷新和每次认证要求当前客户端 IP 与会话完全一致；
- 不匹配且 Redis 可建立 Session 失效状态时，撤销当前会话并返回未认证；
- 不匹配但 Redis 无法建立失效状态时，当前请求仍被拒绝并返回依赖不可用，不执行不安全的
  PostgreSQL 单边撤销；Redis 恢复后再次发生不匹配会完成撤销；
- `bind_device=0` 或 `bind_ip=0` 时仍记录数据，但不执行对应阻断。

浏览器 UUID 是客户端实例标识，不宣称为不可伪造的硬件指纹。

### 6.4 会话数量

`max_sessions` 按 `user_id + platform` 独立计算：

- 0：不限；
- 1：只保留最新一次登录；
- 2 到 100：只保留最新 N 个有效会话。

新登录事务锁定用户，重新验证用户和平台，查询当前平台有效会话，撤销最旧的超额会话，
最后创建新会话。同一用户并发登录也不能突破上限。`admin` 登录不会撤销未来 `app` 会话。

### 6.5 注册

当前 `POST /api/v1/auth/register` 继续执行用户名、邮箱和密码注册，但必须按请求平台读取
`allow_register`。关闭时明确拒绝注册。当前 Web 的注册页是否展示也由 `admin` 平台策略
决定，不显示入口不能替代后端校验。登录页通过公开的当前平台 Policy 接口读取
`allowRegister`；直接访问 `/register` 时也先读取该接口，关闭注册则返回登录页。

未来“邮箱登录，不存在则创建用户”的免注册流程实施后，独立注册页面和当前注册接口可以
删除；届时 `allow_register` 控制邮箱首次登录是否允许创建用户。该未来行为不在本切片中。

### 6.6 平台变更的精确影响

- 修改名称：更新 Policy，用户不下线；
- 修改 Access/Refresh TTL：只影响新签发 Token；
- 修改缓存 TTL：立即使对应派生缓存不可达，按新 TTL 重建；
- 开启设备/IP 绑定：现有会话下一次请求立即使用已有记录校验；
- 降低最大会话数：同一事务撤销每个用户在该平台最旧的超额会话；
- 提高或取消会话上限：不恢复已撤销会话；
- 关闭注册：下一次注册立即拒绝；
- 禁用或删除平台：同一事务撤销该平台全部有效会话；
- 重新启用平台：不恢复旧会话，用户必须重新登录。

每次平台 mutation 都推进 `policy_version` 并使派生缓存按新版本重建。名称变化可能造成一次
无用户感知的缓存重建，但不会撤销会话。

## 7. Redis 数据

Redis key 使用固定前缀和具体技术含义，不增加可配置 key 模板。

### 7.1 Policy

```text
auth:policy:<platform>
```

值为 `ready` Policy Snapshot 或带 mutation token 的 `invalidating` 状态。Snapshot 包含平台
ID、code、policyVersion、启用状态和本设计全部运行时策略，不包含部署配置。

### 7.2 User State

```text
auth:user-state:<userID>
```

保存用户是否有效、启用以及当前随机 generation。用户禁用、启用和删除使用该 key 建立
失效状态。认证必须先验证 User State，再验证具体 Session。角色状态不属于认证平台；有效、
启用用户即使当前没有启用角色也可以完成身份认证，但 RBAC Snapshot 为其返回零有效权限。
这条规则替代旧认证基础中“至少一个启用角色才能认证”的临时耦合。

### 7.3 Session State 与 Snapshot

```text
auth:sessions-state:<platform>:<userID>
auth:session:<platform>:<sessionID>
```

Session State 保存 `ready + generation` 或 `invalidating + mutationToken`。Session Snapshot 至少
包含 userID、sessionID、platform、sessionVersion、policyVersion、userGeneration、
sessionsGeneration、deviceID、clientIP、refreshExpiresAt 和 revoked 状态。

JWT 已携带 sessionID，因此多会话模式不再使用单值“当前会话指针”。认证只有在 User State、
Session State、Policy 和 Session Snapshot 的 generation/version 全部一致时才命中缓存。

Session Snapshot TTL 为以下值的最小值：

```text
平台 session_cache_ttl_seconds
会话 refresh_expires_at 剩余时间
```

### 7.4 Access State 与 Snapshot

```text
authz:access-state:<userID>
authz:access:<platform>:<policyVersion>:<userID>:<version>
```

Access State 保存：

```text
ready + version
或
invalidating + mutationToken + baseVersion
```

Access Snapshot 保存：

```text
userId
platform
policyVersion
version
roleCodes
menuTree
permissionCodes
```

平台不改变 RBAC 内容。Snapshot key 包含 platform 和 policyVersion，只用于应用该平台自己的
`access_cache_ttl_seconds`、使缓存 TTL 变更后旧 Snapshot 立即不可达，以及隔离不同客户端
缓存；所有平台都从相同的 userID + access version PostgreSQL 权威数据构建权限并集。

权限码在 Snapshot 内按字典序排序且唯一。Service 可以在一次请求内构造具体 `map[string]struct{}`
完成判断，但不得把该集合保存为进程级缓存。

### 7.5 缓存写入校验

从 PostgreSQL 回源后，Service 在返回前重新检查对应 Redis 状态：

- 状态仍缺失时，只能通过原子 `SET NX`/Lua 安装新的 ready generation/version；
- 状态已经进入 invalidating 时，不发布也不返回可能过时的结果；
- 状态已经是另一 generation/version 时，按新状态重试；
- Redis 仍不可用时直接返回已经读取到的 PostgreSQL 权威结果，不尝试内存缓存。

这个二次检查防止“读请求先查到旧 PostgreSQL，写请求随后提交，旧读最后覆盖新缓存”。

## 8. 写入一致性与失效

### 8.1 通用顺序

影响认证或权限事实的写操作使用同一顺序，但各模块实现自己的具体操作，不创建通用
Coordinator 或 Cache Manager：

```text
Redis 原子建立 invalidating 状态并返回 mutationToken
-> 操作期间续租失效状态
-> PostgreSQL 事务锁定、校验、修改并推进持久版本
-> 事务提交
-> Redis 使用 mutationToken 原子发布新 ready 状态
-> 旧 Snapshot 因 generation/version 不匹配变为不可达
```

事务回滚后，使用同一个 token 原子恢复原 ready 状态。无法恢复时返回依赖不可用并保留
失效状态，不能伪造成功。

失效状态使用有限租约并在事务活动期间续租：

- 进程在 PostgreSQL 提交前退出，数据库事务回滚；租约到期后读取从 PostgreSQL 重建旧事实；
- 进程在 PostgreSQL 提交后、Redis 发布前退出，租约到期后读取从 PostgreSQL 重建新事实；
- 旧 Snapshot 没有匹配的 ready generation/version 时永远不可使用；
- 不依赖扫描 Redis key 判断正确性。

### 8.2 认证与会话 mutation

以下操作必须使用 Policy、User State、Session State 或具体 Session 的失效状态：

- 注册；
- 登录和 Refresh；
- Logout；
- 管理员未来踢出会话；
- 用户启用、禁用和删除；
- 认证平台新增、更新、启停和删除。

注册创建的是新的 user ID，不存在旧 User/Session/Access Snapshot，因此不虚构一个待失效的
用户 key。注册在进入 PostgreSQL 事务前必须成功读取 ready Policy 并执行 `Redis PING`；Redis
不可用时不创建用户。用户、默认角色关系和 `sys_access_version=1`
在同一 PostgreSQL 事务中创建，提交后第一次认证或 `/access` miss 再从 PostgreSQL 建立快照。

用户禁用/删除先建立 User State 失效状态，再在 PostgreSQL 事务中改变用户状态并撤销全部
平台会话。发布后的 User State 直接拒绝旧 Token。重新启用生成新的 user generation，旧
Session Snapshot 无法匹配，且 PostgreSQL 中旧会话已经撤销。

### 8.3 RBAC mutation

受影响用户和版本推进范围：

| mutation | `sys_access_version` 推进范围 |
| --- | --- |
| 用户角色集合变化 | 目标用户 |
| 角色权限集合变化 | 当前有效绑定该角色的全部用户 |
| 角色启停或删除 | 当前有效绑定该角色的全部用户 |
| 菜单新增、父子结构/路由/展示字段更新、启停或删除 | 全部有效用户 |

流程：

```text
在 PostgreSQL 外预读并排序候选 user IDs
-> 为每个 user 建立 Access invalidating 状态
-> PostgreSQL 事务执行原有 RBAC mutation
-> 事务按既有锁顺序重新查询并锁定真实受影响 user IDs
-> 真实集合必须与候选集合完全一致，否则回滚、释放状态并重试
-> 同一事务将每个受影响用户 version + 1
-> 提交后发布每个用户的新 ready version
```

受影响用户 ID 去重并稳定升序处理。角色、菜单和用户原有锁顺序继续有效。任何用户无法建立
失效状态时，释放本次已经建立的状态并且不进入 PostgreSQL mutation。候选集合复核限制并发
用户角色变化造成的漏失效；有限次数重试后仍变化则返回明确冲突，不无限循环。

菜单新增也推进全部有效用户版本，因为超级管理员自动拥有全部有效页面和操作权限，其他角色
的可分配权限树也发生变化。

菜单全局 mutation 在事务内使用固定、明确的 PostgreSQL 表锁阻止 `sys_user` 新增或状态写入，
然后重新查询全部有效用户并推进版本；普通用户列表读取不被阻塞。这样新用户不能在候选集合
复核后、菜单提交前插入并建立旧菜单 Snapshot。锁顺序写入 Repository 测试，其他用户 mutation
不得以相反顺序获取同一组锁。

## 9. Redis 与依赖故障

### 9.1 普通读取

```text
Redis hit   -> 使用当前有效 Snapshot，0 次 PostgreSQL
Redis miss  -> PostgreSQL 权威查询 -> Redis 可用时回填
Redis error -> 记录错误 -> PostgreSQL 权威查询 -> 返回真实结果
```

缓存 JSON 损坏、字段缺失、generation/version 不匹配都按显式缓存错误或 miss 处理，记录
key 类别和内部 cause，不记录缓存原文。PostgreSQL 查询失败返回 Dependency Unavailable，不能
使用旧缓存、空权限或假成功。

### 9.2 写入

无法建立 Redis invalidating 状态时：

```text
返回 Dependency Unavailable
PostgreSQL mutation 不执行
```

PostgreSQL 已提交但 Redis 发布失败时：

- 返回 Dependency Unavailable；
- 保持旧 Snapshot 不可达；
- 失效租约到期后由下一次读取从 PostgreSQL 重建；
- 不尝试把 PostgreSQL 回滚到旧业务事实；
- 操作日志后续必须能够区分“数据库已提交、缓存发布失败”。

Redis 故障期间，普通读取仍可由 PostgreSQL 保持真实可用；登录、刷新、退出、权限分配、
角色/菜单变更、用户状态变更和平台策略变更暂停。这符合 Redis 已经承担会话、队列和缓存
协调职责的当前部署事实。

写操作遇到 Policy 或 Snapshot miss 可以先从 PostgreSQL 重建，但必须在进入业务事务前成功
写入对应 Redis ready/invalidating 状态。Redis 连接错误不能按普通读取规则继续执行 mutation。

## 10. HTTP 与 Cookie 契约

### 10.1 现有认证接口

路径保持：

```text
GET  /api/v1/auth/policy
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
GET  /api/v1/auth/me
GET  /api/v1/access
```

这些认证接口和所有受保护业务接口都要求 `X-Auth-Platform` 和 `X-Device-ID`。Policy、登录、
注册和 Refresh 没有 Bearer Token，但仍必须明确平台和设备。Refresh、Logout 继续要求精确
Origin 和空 body。公开健康检查不要求认证平台或设备 Header。

公开 Policy 响应只返回当前 Header 平台登录前需要的安全字段：

```text
code
name
allowRegister
```

不返回 Token/缓存 TTL、设备/IP 策略、会话数量、部署配置或其他平台。平台不存在、已删除或
停用时明确拒绝。该接口是只读请求，Redis error 时按普通读取规则回源 PostgreSQL。

### 10.2 平台 Refresh Cookie

Cookie 名按已严格校验的平台 code 生成：

```text
admin_refresh_<platform>
```

例如：

```text
admin_refresh_admin
admin_refresh_app
```

属性继续固定为：

```text
HttpOnly=true
SameSite=Lax
Path=/api/v1/auth
Domain omitted
Secure=AUTH_COOKIE_SECURE
Max-Age=remaining seconds until refresh_expires_at
Expires=refresh_expires_at
```

Refresh 只读取当前 Header 平台对应的 Cookie。Logout 只过期当前平台 Cookie。这样同一浏览器
登录多个平台时 Refresh Token 不互相覆盖。

### 10.3 认证平台管理接口

```text
GET    /api/v1/auth-platforms?page=1&pageSize=20&keyword=&isEnabled=
POST   /api/v1/auth-platforms
PUT    /api/v1/auth-platforms/:id
PATCH  /api/v1/auth-platforms/:id/status
DELETE /api/v1/auth-platforms/:id
```

列表响应沿用现有分页契约：

```text
list
total
page
pageSize
```

时间使用 RFC3339Nano。JSON 使用 lower camel case。Handler 使用严格 query allowlist、严格 URL
ID 和 `validate.BindJSON`，拒绝未知字段、重复字段、多个 JSON 值和尾随内容。

新增与更新请求只接受本设计真实生效字段。Update 不接受 `code`、`isBuiltin` 或
`policyVersion`。状态接口只接受 `isEnabled`。DELETE 请求体必须为空。

部署状态使用单独只读接口：

```text
GET /api/v1/auth-platforms/deployment
```

只返回：

```text
cookieSecure
corsOrigin
trustedProxyMode
trustedProxyCount
redisStatus
```

不返回 APP_SECRET、Redis URL/密码、PostgreSQL DSN、完整可信代理 CIDR 或其他环境变量。

## 11. 路由权限与内置菜单

新增内置 Page：

```text
code=system:auth-platform:list
path=/system/auth-platforms
viewKey=system-auth-platforms
parent=system
```

新增 Action：

```text
system:auth-platform:create
system:auth-platform:update
system:auth-platform:status
system:auth-platform:delete
```

部署状态 GET 复用 `system:auth-platform:list`。平台列表、部署状态、创建、更新、状态和删除路由
分别绑定准确权限。超级管理员按现有规则自动拥有全部有效 Page/Action。普通角色由管理员明确
分配，不自动扩权。

内置菜单初始化要求已有记录结构正确，不静默改写用户修改。页面和 Action 使用后端、前端、
数据库一致的 `auth-platform` 命名，不使用旧项目的下划线权限码。

## 12. 前端设计

### 12.1 请求平台与设备

`web/src/utils/request.ts` 的 raw client 和 authenticated client 都统一添加：

```text
X-Auth-Platform: admin
X-Device-ID: <persistent UUID>
```

当前 Web 是 Admin 客户端，platform code 由一个具体的前端常量定义，不从 URL 或用户输入读取。
未来其他前端使用自己的构建配置或具体常量。Device ID 存储使用独立模块和固定 key，不与 Auth
Store 混合。

CORS 显式允许 `X-Auth-Platform`、`X-Device-ID`、`Accept-Language`、`Authorization` 和
`Content-Type`，继续只允许精确配置的 Origin 和 credentials，不使用 `*`。

Access Token 继续只存 Pinia 内存。刷新重试继续只允许一个 Refresh 请求在途，每个原请求最多
重试一次。

登录页启动时读取当前平台公开 Policy：`allowRegister=1` 才显示注册链接；注册路由直接访问时
重新读取并校验。Policy 读取失败显示明确认证服务错误，不把失败当作关闭注册或空配置。

### 12.2 认证平台页面

主要文件：

```text
web/src/api/auth-platform.contract.ts
web/src/api/auth-platform.ts
web/src/views/system/auth-platforms/index.vue
```

页面结构：

```text
搜索区：关键词、启用状态、查询、重置、刷新
工具栏：新增
表格：平台、Token TTL、缓存 TTL、安全策略、会话策略、状态、更新时间、操作
分页
部署状态只读区
新增/编辑 Dialog
```

交互规则：

- code 只在新增时填写，编辑时只读；
- `admin` 显示“内置”标记，不显示删除操作；
- TTL 使用秒数数字输入，同时显示可读的分钟、小时或天；
- `maxSessions=0` 显示“不限”，1 显示“单会话”，其他显示“最多 N 个”；
- Yes/No 使用 Switch 或 Select，不使用自由文本；
- 修改设备/IP、安全 TTL、会话上限或平台状态时显示具体影响确认；
- 降低会话上限提示将撤销最旧超额会话；
- 禁用平台提示该平台全部用户立即下线；
- 保存失败保留 Dialog 和输入；成功后关闭并刷新当前页；
- Main 仍是唯一页面纵向滚动容器，Dialog body 独立滚动；
- 不提前抽取全局 AppDialog、AppTable 或通用 CRUD composable。

所有外部数据在 contract 文件中从 `unknown` 严格校验。业务 TypeScript 禁止显式 `any`、
`any[]`、`as any`、`Record<string, any>` 和错误抑制。中英文 i18n key 集合必须一致。

## 13. 配置与可信代理

继续保留：

```text
APP_SECRET
AUTH_COOKIE_SECURE
CORS_ORIGIN
REDIS_URL
```

新增 API 必填配置：

```text
TRUSTED_PROXIES=none
```

本地直连明确使用 `none`，并调用 Gin 的可信代理配置关闭转发 Header 信任。反向代理部署使用
逗号分隔的明确 IP 或 CIDR；逐项 TrimSpace、解析、去重后交给 Gin。缺失、空字符串、非法值、
`0.0.0.0/0` 或 `::/0` 使 API 启动失败。Gin 不保留默认信任所有代理行为。客户端 IP 只由直接
对端地址和经过这些可信代理验证的转发链得到。

`AUTH_COOKIE_SECURE` 必须继续与 `CORS_ORIGIN` 的 HTTP/HTTPS 协议一致。认证平台页面只读
展示这些部署状态，不允许热修改。运行中的前端不能通过管理接口改变 Cookie 安全、根密钥、
CORS、数据库或 Redis 连接。

## 14. 错误、日志和可观测性

新增稳定错误码：

| HTTP | code | 场景 |
| ---: | ---: | --- |
| 404 | `17000` | 认证平台不存在 |
| 409 | `17001` | 有效平台 code 冲突 |
| 409 | `17002` | 内置认证平台不可删除 |
| 400 | `17003` | 平台策略字段或组合无效 |
| 403 | `17004` | 认证平台已停用 |
| 503 | `17005` | 会话状态正在更新 |
| 503 | `17006` | 权限状态正在更新 |

Update DTO 不接收 `code`。客户端尝试提交 `code` 或 `isBuiltin` 属于未知字段，直接使用项目
Invalid Request code `10001`，因此不创建永远无法触发的“code 不可修改”专用错误码。

Redis 连接、读取、写入、续租或发布失败继续使用项目通用 Dependency Unavailable code
`10006`，并通过稳定双语 message key 区分具体内部场景，不重复创造同义业务码。

设备/IP 不匹配、会话撤销/过期和 platform claim 不匹配统一返回未认证，不向客户端泄露哪一项
安全检查失败。平台管理参数错误可以返回具体双语字段错误。

请求日志增加以下非敏感结构化字段：

```text
authPlatform
userID
sessionID
cacheKind
cacheResult=hit|miss|error|invalidating|rebuilt
accessVersion
```

禁止记录密码、密码摘要、Access Token、Refresh Token、Cookie、Authorization Header、完整
Device ID、缓存原文、APP_SECRET、Redis URL、DSN 或请求体。Redis error 和 PostgreSQL error
保留内部 cause，HTTP 只返回稳定 code/message。

现有 `/ready` 继续要求 PostgreSQL 和 Redis 都正常。普通业务读取在 Redis 故障时可以回源
PostgreSQL，不改变 readiness 对完整服务依赖的严格定义。

## 15. 测试策略

### 15.1 PostgreSQL 与模型

真实 PostgreSQL 测试至少覆盖：

- 认证平台字段、CHECK、部分唯一索引和时间字段；
- 内置 `admin` 初始化、重复执行和损坏数据启动失败；
- 旧会话撤销、旧索引删除和 access version 回填；
- 多平台会话查询、稳定排序和并发上限；
- 0、1、N 会话策略保留正确的最新记录；
- 平台禁用/删除、用户禁用/删除与会话撤销同事务；
- RBAC mutation 与受影响用户 version 推进同事务；
- 任一步失败完整回滚。

### 15.2 Redis 与并发

使用用户本机真实 Redis 的集成测试至少覆盖：

- Policy、User、Session 和 Access key 格式；
- Snapshot TTL 上限和绝对会话剩余时间；
- 健康命中时认证和 RBAC 的 PostgreSQL 调用次数为 0；
- miss、Redis 读取错误和损坏 payload 回源 PostgreSQL；
- PostgreSQL 失败时不使用旧 Snapshot；
- mutation token 不匹配不能发布或回滚其他写入；
- 失效租约续租、事务回滚恢复和租约到期惰性重建；
- 读回源与并发写提交时，旧读不能覆盖新 generation/version；
- 权限变更后下一次请求不能接受旧权限；
- Redis 写协调失败时 PostgreSQL mutation 调用次数为 0；
- PostgreSQL 已提交、Redis 发布失败时旧 Snapshot 不可达；
- Redis 清空后由 PostgreSQL 安全重建，不要求启动时扫描全部用户。

测试不使用内存 Redis、SQLite 或 Docker 替代真实依赖。

### 15.3 Auth

至少覆盖：

- Header 缺失、重复、空值和格式错误；
- JWT platform 与 Header 精确匹配；
- 不同平台 Cookie 和会话互不覆盖；
- 动态 Access/Refresh TTL；
- Refresh 轮换不延长绝对过期时间；
- Device ID 持久 UUID 和严格绑定；
- 可信代理与严格 IP 绑定；
- 平台禁用后 Access 和 Refresh 立即失效；
- 平台重新启用不恢复旧会话；
- 用户有效但无启用角色时可认证，RBAC 返回零权限；
- 密码、Token、Cookie、设备标识不进入日志。

### 15.4 RBAC

至少覆盖：

- 多角色启用权限并集；
- 禁用角色不贡献权限；
- 超级管理员动态拥有全部有效页面和操作；
- `/access` 与 RequirePermission 使用相同 Snapshot；
- 用户角色、角色权限、角色状态和菜单 mutation 的精确版本范围；
- 菜单全局 mutation 推进全部有效用户版本；
- Access Snapshot 的稳定排序、非空数组和树完整性；
- Redis 故障时 PostgreSQL 权威权限仍正确；
- PostgreSQL 故障时失败关闭。

### 15.5 前端

至少覆盖：

- raw/normal/Policy/refresh/retry 请求全部带平台和设备 Header；
- 公开 Policy 严格解析，注册入口和直接注册路由遵守 `allowRegister`；
- Device ID 首次生成、复用、损坏重建和生成能力缺失错误；
- 平台 DTO、分页、部署状态和 mutation 响应严格解析；
- 权限码控制查看、创建、编辑、状态和删除；
- `admin` 内置保护；
- TTL、会话策略和安全策略展示；
- 降低上限、禁用和删除的确认文案；
- 保存失败保留状态；
- 中英文文案完整；
- 页面滚动边界和 forbidden any 定向检查。

### 15.6 完整验证

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...

cd D:\admin\web
pnpm vitest run
pnpm build
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src -g "*.ts" -g "*.vue"
```

不启动或停止用户的 API、Worker、Web、PostgreSQL 或 Redis 服务，除非用户在执行阶段另行明确
授权。

## 16. 技术债

以下技术债被明确登记，但本切片不创建字段、接口、空实现或兼容分支：

| 技术债 | 后续边界 |
| --- | --- |
| 登录方式配置 | 实现第二种真实登录方式时，为认证平台增加严格枚举并接入完整登录链路 |
| 验证码类型 | 实现真实验证码挑战、校验、限流和过期后再增加字段 |
| 邮箱免注册登录 | 邮箱存在则登录，不存在且平台允许注册则创建用户；完成后删除独立注册页 |
| 短信和第三方登录 | 每种协议使用具体模块和客户端，不创建通用 Platform Adapter |
| 设备风险识别 | 浏览器 UUID 只做实例标识；复杂指纹和风险评分独立设计 |
| 会话管理页面 | 展示多平台会话并支持踢出用户，复用本设计的 PostgreSQL/Redis 撤销路径 |
| 登录日志与操作日志 | 记录平台、设备摘要、IP、操作者和结果，不记录凭据 |
| 缓存指标面板 | 有真实监控系统后展示 hit/miss/error/rebuild，不在业务表伪造计数 |

本设计完成后再开始 UI 与格式基线回收。不得在实现本设计时同时引入全局 Dialog/Table、
通用 CRUD、主题重做或无关页面改版。

## 17. 预期代码边界

后端主要修改或新增：

```text
server/internal/module/authplatform/
  model.go
  schema.go
  protocol.go
  repository.go
  redis.go
  service.go
  handler.go
  route.go
  *_test.go

server/internal/module/auth/
  token.go
  session_model.go
  session_repository.go
  redis.go
  service.go
  handler.go
  middleware.go
  *_test.go

server/internal/module/access/
  model.go
  schema.go
  redis.go
  repository.go
  service.go
  middleware.go
  *_test.go

server/internal/module/menu/
server/internal/module/role/
server/internal/module/user/
server/internal/config/
server/internal/redis/
server/cmd/api/
```

模块名使用具体 `authplatform` 表达业务资源，不使用泛化 Platform Adapter。`internal/redis`
只扩展当前实际需要的基础命令或原子脚本执行能力；认证和权限 key、payload、失效语义分别留在
所属业务模块。

前端主要修改或新增：

```text
web/src/api/auth-platform.contract.ts
web/src/api/auth-platform.ts
web/src/auth/device-id.ts
web/src/access/menu-title-keys.ts
web/src/access/route-views.ts
web/src/i18n/messages/zh-CN.ts
web/src/i18n/messages/en-US.ts
web/src/utils/request.ts
web/src/views/system/auth-platforms/index.vue
```

## 18. 验收标准

1. 管理员可以维护多条真实生效的认证平台策略；
2. `admin` 平台存在、不可改 code、不可删除；
3. `AUTH_COOKIE_SECURE` 等部署配置只读展示且不能从页面修改；
4. 每个认证接口和受保护请求明确携带 `X-Auth-Platform` 和规范 `X-Device-ID`；
5. JWT、Refresh Cookie 和会话均按平台隔离；
6. 会话上限按平台和用户独立计算，并发登录只保留最新 N 个；
7. 设备/IP 绑定开启后对登录、刷新和认证真实生效；
8. 平台禁用或删除立即撤销该平台会话，重新启用不恢复；
9. Redis 健康命中时认证和 RBAC 请求不查询 PostgreSQL；
10. Redis miss、读取故障或损坏时读取 PostgreSQL 权威数据；
11. PostgreSQL 故障时不使用旧缓存或假成功；
12. Redis 写协调失败时不执行 PostgreSQL mutation；
13. PostgreSQL 已提交但缓存发布失败时旧 Snapshot 仍不可达；
14. RBAC 多角色并集、`/access` 和权限 Middleware 使用同一 Snapshot；
15. 权限 mutation 在同一事务推进正确用户的 access version；
16. 权限变更后的下一次请求不接受旧权限；
17. 不存在进程级权限缓存、通用 Adapter、Manager 或缓存框架；
18. 前端 DTO 严格校验且业务 TypeScript 不出现 forbidden any；
19. 旧无平台会话在迁移时明确撤销，用户重新登录成功；
20. 后端和前端完整测试、构建及真实 PostgreSQL/Redis 集成验证通过。
