# 用户认证日志数值协议与审计设计

## 1. 目标与范围

统一 `user_login_log` 的事件和登录方式协议，补齐邮箱/手机号验证码自动注册的审计记录，保证登录、注册和登出主流程不因审计写入失败而失败，并让登录日志页面能按新的数值协议展示和筛选。

本次包含：

- `event_type` 从字符串改为 `SMALLINT`，支持注册、登录、登出三种事件。
- `login_type` 从字符串改为可空 `SMALLINT`，支持 password、email、phone 三种方式。
- `login_account` 重命名为 `account`。
- 从 `user_login_log` 删除 `session_id`；认证系统自身的 Session ID、JWT 和撤销流程不删除。
- 新增验证码自动注册成功事件，并保留随后成功登录事件。
- 登出日志使用当前用户的 `username`，不追溯本次会话此前的认证方式。
- 登录、注册、登出日志均为 best-effort 审计写入，不使用 Asynq，不阻断认证主流程。
- 登录日志页面改用整数 DTO、严格解析、事件 Tag 和登录方式筛选/展示。

本次不包含：

- 不新增公开的显式注册接口；用户仍只能通过已验证的 email 或 phone 自动注册。
- 不把 `event_type`、`login_type` 放入系统字典管理。
- 不改变 `user_account.username` 的生成规则；自动注册继续使用 `deterministicUsername(account)`。
- 不把登录日志迁移为 Asynq 任务。可靠异步审计需要 outbox/事件幂等协议，另行设计。

## 2. 已确认的业务规则

### 2.1 事件枚举

```text
event_type:
1 = register
2 = login
3 = logout
```

```text
login_type:
1 = password
2 = email
3 = phone
```

Go 和 TypeScript 均使用带固定值的 typed enum/union，不允许普通字符串或任意整数穿透业务层。数据库使用 `SMALLINT` 与 `CHECK` 约束。

### 2.2 事件字段

| 事件 | `user_id` | `account` | `login_type` | 说明 |
| --- | --- | --- | --- | --- |
| 注册成功 | 新创建用户 ID | 规范化后的邮箱或手机号 | 2 或 3 | 只由验证码自动注册产生 |
| 登录成功 | 已解析用户 ID | 规范化后的认证账号 | 1、2 或 3 | 随后创建 Session |
| 登录失败且账号存在 | 已解析用户 ID | 规范化后的认证账号 | 1、2 或 3 | 例如禁用用户或密码错误 |
| 登录失败且账号不存在 | `NULL` | 规范化后的输入账号 | 1、2 或 3 | 不伪造用户 ID |
| 登出成功 | 当前用户 ID | 当前 `user_account.username` | `NULL` | 不查询历史登录方式 |

`account` 是审计展示快照。注册/登录保存规范化后的 email/phone，登出保存 username。该字段不再命名为 `login_account`，因为它同时承载注册和登出事件的展示账号。

### 2.3 自动注册流程

公开认证路由没有 `/auth/register`。只有邮箱或手机号验证码验证成功后，发现身份不存在时才创建用户：

```text
验证码校验成功
  -> FindCredentialByIdentity
  -> 找不到身份且平台允许注册
  -> CreateVerifiedIdentity(Username = deterministicUsername(account))
  -> 写 register 成功事件（best-effort）
  -> 创建 Session
  -> 写 login 成功事件（best-effort）
```

并发创建遇到唯一键冲突并重新找到已有用户时，不写 register 事件，只写 login 事件。注册事件写入成功与 Session 创建解耦；用户已创建即构成注册事实。

## 3. 数据库协议

最终 `user_login_log` 字段为：

```text
id             BIGINT
user_id        BIGINT NULL
platform_id    BIGINT NOT NULL
account        VARCHAR(254) NOT NULL
event_type     SMALLINT NOT NULL
login_type     SMALLINT NULL
is_success     SMALLINT NOT NULL
reason_code    VARCHAR(64) NOT NULL
client_ip      VARCHAR(64) NOT NULL
user_agent     VARCHAR(512) NOT NULL
created_at     TIMESTAMPTZ NOT NULL
updated_at     TIMESTAMPTZ NOT NULL
```

约束：

```sql
CHECK (event_type IN (1, 2, 3))
CHECK (is_success IN (0, 1))
CHECK (btrim(account) <> '')
CHECK (
  (event_type = 1 AND login_type IN (2, 3))
  OR (event_type = 2 AND login_type IN (1, 2, 3))
  OR (event_type = 3 AND login_type IS NULL)
)
```

保留 `user_id`、`platform_id` 的外键和现有 created/updated 时间字段；删除 `session_id` 外键及字段。保留按 `created_at`、`user_id`、`platform_id`、`account` 的真实查询索引，账号索引改为使用 `account` 列。

迁移必须是一次性、幂等、可审计的 forward migration：

```text
event_type login  -> 2
event_type logout -> 3
login_type password -> 1
login_type email    -> 2
login_type phone    -> 3
```

历史数据没有可恢复的注册事件，不回填虚构的 register 记录。现有登出行的空账号必须在改列名前通过 `user_id -> user_account.username` 回填；如果存在没有可解析用户或空 username 的行，迁移必须预检失败，不得写入伪造值。迁移必须拒绝未知字符串和不满足映射的行；转换失败保持原表结构和数据不变。迁移成功后刷新 `docs/database/current.sql`。

## 4. 后端分层与失败语义

### 4.1 领域值

`server/internal/module/user/loginLog` 定义 `EventType`、`LoginType`、合法值、公开 label 所需的固定 metadata 和事件校验。Repository Model 使用 typed `int16` 字段；Service、Handler、ListQuery 和 JSON Response 使用相同的整数协议。

Handler 将 `eventType`、`loginType` 查询参数解析为整数，拒绝字符串名称、零值、未知值和重复参数。 `PageInit` 保留现有端点，但返回整数数组；标签由前端 i18n 生成，不从字典读取。

### 4.2 日志写入

`recordLoginEvent` 继续直接调用 PostgreSQL Repository，不引入 Asynq。写入错误记录带上下文的 warning 后返回 `nil`，不得覆盖登录/注册结果。

`Logout` 在撤销 Session 的核心动作完成后，使用 `user_id` 查询当前 username，构造登出事件并 best-effort 写入。username 查询失败或日志写入失败只能记录 warning，不能把已经完成的登出转换为依赖错误。认证/Session 撤销自身的 Redis 或 PostgreSQL 错误仍按原有主流程返回。

审计写入及登出 username 查询应使用短的、明确的 context deadline，避免 PostgreSQL 连接池阻塞或故障等待无限延长认证响应；单次登录/注册审计写入上限为 500ms，登出 username 查询和审计写入共享 500ms 总预算，超时按 best-effort 失败处理。首选在认证 Service 内用常量 timeout 与 `context.WithTimeout` 派生请求 context，仍贯穿 GORM，且不得用 `context.Background()`。

username 查询只读取用户账号事实，不调用 HTTP `me` 接口、不查询历史登录日志、不查询本次会话的登录方式。

### 4.3 事务与时序

日志插入不是认证 Session 创建事务的一部分。登录成功的 Session 创建、认证状态发布和 Token 签发继续按现有顺序执行；审计写入失败不得回滚已完成认证。注册事件在用户创建成功后记录；登录事件在 Session 创建成功后记录。

## 5. 前端契约与交互

API DTO：

- `eventType` 为 `1 | 2 | 3`。
- `loginType` 为 `1 | 2 | 3 | null`。
- `PageInit.eventTypes` 和 `loginTypes` 为整数数组。
- 查询参数 `eventType`、`loginType` 只发送整数。
- 解析器拒绝字符串枚举、未知整数、错误的 null 形状和缺失字段。

登录日志页面：

- 事件列显示三个不同 Tag 类型：注册 `primary`、登录 `success`、登出 `danger`。
- 增加登录方式列；password/email/phone 使用 i18n 文案，登出为 `-`。
- 增加登录方式筛选，空值表示全部。
- 状态列继续按 `is_success` 显示 success/danger，不与事件 Tag 混淆。
- 事件和登录方式选项来自固定前端 metadata 与后端整数协议，不调用系统字典。

## 6. 上线切换

这是一个不兼容的 API/数据库协议变更，不保留字符串双读/双写。维护者应在同一维护窗口停止 API 实例和 Worker（Worker 不应访问该表但按项目迁移要求停止），执行 forward migration，再部署匹配版本的 API/Web 并恢复服务。回滚应用二进制不能单独回滚数据库 schema；如需回退，必须另行准备经过审查的数据恢复迁移，不用旧版本直接读新表。

## 7. 系统字典边界

系统字典只负责全平台共享、可由管理员维护的展示型选项和中英文标签，例如地域、文件扩展名、MIME 类型等。`event_type`、`login_type` 会参与后端分支、HTTP 协议和数据库约束，因此固定在代码与数据库协议中。禁止通过禁用字典项改变认证日志的合法值集合。

## 8. 验证要求

必须覆盖：

- migration 字符串到整数的转换、未知值拒绝、约束、索引、删除 `session_id` 和幂等复跑；
- register/login/logout 三类事件的字段组合和 `login_type` 约束；
- 邮箱/手机号自动注册产生 register + login，竞争创建只产生 login；
- 登录日志数据库故障不阻断登录和自动注册；登出日志/username 查询故障不阻断登出；
- `user_id` 来源为凭据查询/创建结果，不调用 `me`；
- Handler 整数查询协议、PageInit 整数 DTO 和错误输入拒绝；
- 前端三种事件 Tag、登录方式列/筛选和 null 展示；
- Go fmt/vet/test/build、前端定向 Vitest/typecheck/build、真实 PostgreSQL migration 结构验证。
