# 当前系统架构

本文描述当前仓库可观察到的运行时边界。它是 Agent 的快速导航，不替代目标模块中的接口、测试和实现。

## 组件与入口

```text
web (Vue 3) -> Go API (Gin/GORM) -> PostgreSQL
                    |       |
                    v       v
                  Redis   Asynq -> Go Worker -> PostgreSQL
```

| 组件 | 入口/目录 | 责任 |
| --- | --- | --- |
| Web | `web/src/main.ts`、`web/src/views` | 页面、动态路由、状态和 API 调用 |
| API | `server/cmd/api/main.go` | 配置、依赖装配、全局 Middleware、HTTP 服务 |
| 业务模块 | `server/internal/module/*` | 各域的 Handler、Service、Repository、Model、Schema 和测试 |
| 数据库 | `server/internal/database` | PostgreSQL 连接与结构准备边界 |
| Redis | `server/internal/redis`、模块内 Redis 文件 | 连接、会话/权限缓存和失效协调 |
| Queue | `server/internal/queue` | Asynq 客户端/服务端封装，不导入业务模块 |
| Worker | `server/cmd/worker/main.go` | 注册任务并按任务 Handler -> Service -> Repository 消费 |

当前模块目录包括：`auth`、`health`、`permission`、`user`、`message`、`storage`、`system`。目录存在不代表
所有业务都已完成，状态以 `STATUS.md` 和测试为准。

## 业务身份与跨层定位

业务命名是跨层导航契约：先确认业务归属并校正后端模块，前端 Views/API 再统一使用该业务域与单数资源名，
不再使用业务域倒置或复数资源别名；现有后端目录本身不是业务归属的最终依据。
前后端复合模块目录使用 lower camel case，数据库使用 snake_case；集合字段仍按实际语义使用复数。

| Views（省略 index.vue） | API / 后端 module | PostgreSQL 主表 |
| --- | --- | --- |
| `user/account` | `user/account` | `user_account` |
| `user/profile` | `user/profile`（账户写入协作 `user/account`） | `user_profile`、`user_account` |
| `user/session` | `user/session` | `user_session` |
| `user/loginLog` | `user/loginLog` | `user_login_log` |
| `permission/authPlatform` | `permission/authPlatform` | `permission_auth_platform` |
| `permission/menu` | `permission/menu` | `permission_menu` |
| `permission/role` | `permission/role` | `permission_role` |
| `message/mail` | `message/mail` | `message_mail_*` |
| `storage/object` | `storage/cosConfig`、`storage/uploadRule`、`storage/upload` | `storage_cos_config`、`storage_upload_rule*` |
| `system/operationLog` | `system/operationLog` | `system_operation_log` |
| `system/dictionary` | `system/dictionary` | `system_dictionary`、`system_dictionary_item` |

复合代码模块统一使用 lower camel case；数据库仍使用 snake_case，HTTP API 仍使用小写资源段。Mail 是聚合
页面，后端按表资源拆为 `message/mail/config`、`template`、`log`、`logVerification`、`rateLimitPolicy`、
`recipientRule`；根 `message/mail` 只保留发送编排、Provider、Limiter、Readiness、管理测试和路由聚合。
`logVerification` 是 `log` 详情的下属持久化模块，不单独创建页面或公开 CRUD。
字典管理 CRUD 位于 `/api/admin/v1/system/dictionary` 并使用独立 action 权限；业务消费只读端点位于
`/api/v1/system/dictionary/options`，要求有效登录态但不要求字典管理权限，只返回已启用字典及选项的本地化
`label/value`。options 经过 Redis generation/mutation/snapshot 和有界冷回源租约，PostgreSQL 仍是事实来源。

菜单 page 的 `componentPath` 精确对应 Views 页面，`path = "/" + componentPath`；菜单 `code` 与页面和动作
权限各自对应，`i18nKey` 必须在中英文翻译中可解析。公共登录/找回密码页按静态认证路由处理，不强行创建
菜单或独立数据库表。关联表、聚合页和无表模块允许一对多/多对一映射，不制造占位模块满足表面一致。

认证平台归属“权限与认证”：管理 API 为 `/api/admin/v1/permission/authplatform`，页面/动作权限为
`permission:authPlatform:view/list/create/update/status/delete`（各自独立声明，不自动推导）。导航翻译使用
`navigation.permissionAuthPlatform`，页面文案使用 `permission.authPlatform.*`。`auth/login` 消费平台策略，
公共策略端点 `/api/v1/auth/policy` 不变；前端 `src/auth/platform.ts` 只是当前客户端平台标识，不是管理模块。
本次归属改名不改变平台 ID、策略内容、策略版本或 `auth:policy:v2:*` Redis key，不重置现有会话。

修改业务名称时沿 Views -> API -> Module -> TableName -> 菜单/权限 -> i18n 核对全部引用；数据库中的菜单
和表名以当前 PostgreSQL 为准，文件/测试夹具不能证明实际数据库已同步。未经明确需求不改变既有标识。

## 层内责任

- `request.go` / `response.go` 定义 HTTP DTO 与序列化边界；Service 输入输出表达用例，不接收 Gin Context。
- Handler 的消费接口只包含入口需要的能力；认证 Middleware 只依赖 Authenticate，不能因新增找回密码
  用例而迫使所有 Middleware 测试替身实现完整认证业务接口。
- Repository 方法和查询行结构归属 repository 文件；事务由 Service 表达意图、Repository 执行 SQL/锁。
- Model 保存持久化映射，业务校验、模板目录和事件输入放回模块的业务/协议文件，不混成表模型。
- Redis、Storage、邮件 Provider 等 I/O 留在对应技术实现或业务模块，由 Service 编排，不塞进 PG Repository。
- `shared` 只容纳跨域稳定、无业务模块依赖的协议/工具；不下沉某个模块的业务规则来消除循环依赖。

## 邮件规则所有权

腾讯云 SES 是全系统唯一邮件通道。`message_mail_config` 是全局唯一活动配置，`message_mail_template` 按
`scene` 全局唯一，`message_mail_recipient_rule` 是全局收件策略；三者不持有认证平台 `platform_id`。只有
`message_mail_log` 和 `message_mail_log_verification` 保留来源平台，用于审计、验证码隔离和发送状态更新。
这两张表是 append-only 审计事实，不含 `deleted_at`，不注册管理员删除 API、权限动作或前端删除控件；平台软删
不得从审计 JOIN 中排除历史记录，平台 code 通过 `LEFT JOIN` 保留显示。
Mail readiness 只按场景缓存，配置或模板变更一次性失效全局场景状态；认证平台只参与登录方式开关、发送日志
归属和发送额度 key。缺失认证上下文不得回退到固定平台。当前只有 COS 上传配置允许按认证平台维护独立配置。

Mail 管理的限流策略与 `message_mail_config.ttl_minutes` 是所有邮件发送场景的共同规则来源，Auth 不新增
固定重发间隔、验证码 TTL 或独立 cooldown。邮箱额度按同一平台、同一规范化邮箱共享，验证码内容仍按场景
隔离；只保留每分钟、每 10 分钟两条邮箱策略，管理测试发送直接共用，不叠加场景、IP 或管理员专属额度。
次数与窗口以 Mail 配置为准，Auth 不新增时间配置；验证码有效期同样只来自 Mail 配置。
切换旧的按场景 Redis 限流状态时必须保留已消耗额度，并明确旧进程退出顺序、原子迁移和故障行为；不能只
换 key 后声称窗口连续。该契约的实现与验证进度见 STATUS。

## 容量与一致性

项目默认按百万级用户、多实例和高并发访问设计。共享请求热路径不得把 PostgreSQL 当作每请求配置中心，
不得依赖单进程唯一状态或无界缓存；缓存、限流、队列和失效协议必须明确跨实例一致性、故障闭合和回源上限。
跨模块 spec/plan 必须记录容量假设、热点查询预算、缓存更新策略和并发验证方式。

### 本轮收口的具体约束

- 冷缓存重建使用 `shared/cacheFill`：按 Auth Session、Access、Menu Version 三个 scope 独立限制，
  每目标仅一个跨实例持有者、每 scope 32 并发/128 次启动每秒；数据库工作期限从申请租约起算 4 秒，
  Redis 租约 6 秒。缓存命中不走源加载；失败闭合，有界等待，不提供内存或 PostgreSQL 故障兜底。
  “启动次数”不等于 SQL 数量，一次权限重建可能执行多条查询；尚无百万用户实测结论。
- 平台菜单版本存在 `permission_auth_platform.menu_version`，Redis 使用 `authz:menu-state:v1:<platformID>`。
  菜单写入只锁定平台版本行和该平台菜单，在同一数据库事务递增菜单版本，沿 token lease 发布/回滚；
  不扫描用户表，不逐用户递增授权版本，不更改认证 `policy_version` 或撤销会话。用户/角色授权仍用用户版本。
- Access 每次使用本地缓存前确认用户授权状态和平台菜单状态；Redis v8 快照键同时包含两种版本，
  发布 Lua 同时校验两者，旧发布者不能把旧菜单写成当前快照。重建接口返回 `rebuiltPlatforms`。
- Mail 的 `resendAfterSeconds` 来自两窗口原子预占后的额度，0 为合法值；不再把短窗口长度当独立冷却时间。
  返回值是额度快照，其他并发发送仍可能改变可用性，最终以服务端共同额度判断为准。
- 上述菜单版本机制依赖 `2026-09-08-menu-catalog-version.sql`，业务库执行状态以 STATUS 为准；不得启动时迁移。
- Access 的 page/action 授权不互相派生：直接 action 只进入 `permissionCodes`；直接 page 才进入 `menuTree`，
  且只为页面渲染补 directory 祖先。当前快照 namespace v8 隔离旧的 action 自动补 page 语义和旧模块标识。
- `user:profile:view` 是 `parent_id=NULL` 的隐藏根 page。Admin 邮箱验证码登录在注册开启时创建
  `registered_user`；该角色直接授权 profile page、三个 profile/password action 与 upload action。

## 同步请求

```text
HTTP -> global middleware -> route middleware -> Handler
     -> Service -> Repository -> GORM Model -> PostgreSQL
```

- 全局 Middleware 在 API 入口统一处理 request ID、CORS、访问日志、操作日志、Recovery 和语言。
- `/api/v1` 承载共享认证/Access 接口；`/api/admin/v1` 额外要求 Admin 平台标识。
- Router 只声明路径、中间件和 Handler。Handler 将 `c.Request.Context()` 传入 Service，并输出统一 envelope。
- Service 编排业务规则、状态变化、事务意图和外部依赖顺序。Repository 通过 `WithContext(ctx)` 访问
  PostgreSQL；Model 只映射表结构。
- 依赖在 `cmd/api/main.go` 显式创建，例如 Repository -> Service -> Handler；不使用容器、运行时注册或万能
  基类。

## 异步任务

```text
Queue client -> Redis/Asynq -> Worker task handler -> Service -> Repository -> PostgreSQL
```

实体状态任务只携带最小稳定标识，消费时重新从 PostgreSQL 读取事实；不可变操作事件可携带闭合、脱敏、版本化
DTO。入队和消费边界都必须校验载荷，任务 Handler 不直接写数据库。

## 前端数据流

```text
web/src/views/** -> web/src/api/<module>.ts -> web/src/utils/request.ts -> Go API
```

Router 和 Access Store 根据后端快照动态注册业务页面。API 模块负责 DTO 解析，不把请求放进公共组件；页面
只编排视图状态。Element Plus 树/表格行 key 统一为字符串，不能混用数字 ID。

## RBAC 与缓存

```text
PostgreSQL (权限事实) -> Redis (access version/快照) -> 进程内有界快照
```

每次使用进程内快照前先向 Redis 确认当前 access version；版本不一致时重建或读取 Redis 快照。Redis 故障、
版本无法确认或 payload 损坏时显式返回依赖错误，不进行逐请求 PostgreSQL 故障回源。
正在失效时返回更新中错误；正常 missing 才允许重建。发布或状态确认失败不返回未确认的快照。
菜单树只包含 directory/page，动作权限只进入 `permissionCodes`，页面隐藏状态不改变路由和 API 保护。

## 数据库事实与迁移

- 当前 PostgreSQL 是唯一权威来源；连接、健康检查和就绪检查走 `server/internal/database`。
- API/Worker 启动不执行 DDL、seed、回填或 Redis 全库清理。结构变更由维护者执行幂等、可审计的 forward SQL。
- `docs/database/2026-*.sql` 是历史执行记录，不是当前 schema 的默认知识。需要快照时从真实 PostgreSQL 导出
  `docs/database/current.sql`，发现不一致以数据库为准。
- 维护表显式使用 `TIMESTAMPTZ` 的 `created_at`/`updated_at`；真实删除才使用 `deleted_at`；索引和约束只
  为真实查询及业务不变量创建。

## 代码定位

| 要查的内容 | 先看 |
| --- | --- |
| API 路由和依赖装配 | `server/cmd/api/main.go`、目标模块 `route.go` |
| 认证/权限 | `server/internal/module/auth`、`server/internal/module/permission` |
| 用户与账户 | `server/internal/module/user` |
| 前端路由和 Access | `web/src/router`、`web/src/store`、`web/src/api/permission` |
| 公共请求和错误 | `web/src/utils/request.ts` |
| 页面行为 | 对应 `web/src/views/<domain>` 及镜像测试 `web/tests` |
