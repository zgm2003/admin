# 操作日志与会话管理设计

> 2026-08-21 加固说明：操作日志幂等标识、摘要上限、panic 审计和会话撤销顺序以 `2026-08-21-operation-log-session-hardening-design.md` 为准；下文已同步最终契约。

## 1. 目标

本切片同时补齐 Admin 的两项安全基础能力：

1. 对明确登记的“操作类路由”进行监控，并通过 Asynq 异步写入可查询的操作日志；
2. 提供用户会话列表、状态筛选、分页和踢出会话能力，踢出操作同步更新 PostgreSQL 和 Redis 会话状态。

本设计继续使用当前单体项目的具体模块和线性数据流，不引入通用 Adapter、Manager、Factory、BaseService、BaseRepository、DI 容器或运行时注册器。

## 2. 已确认的边界

- 操作日志只记录操作类路由，不记录普通只读列表、详情和下拉选项请求；
- 登录、注册、刷新、退出、会话踢出，以及用户、角色、菜单、认证平台等状态变更路由属于操作类路由；
- 操作日志采用异步队列写入，Redis 或 Worker 故障不能导致原业务请求失败；
- 会话列表是普通读取，会话踢出是同步安全操作；
- 会话撤销以后，PostgreSQL 是安全事实，Redis 快照删除失败必须返回明确错误并记录内部原因，但不能恢复已撤销会话；
- 操作日志追加不可变，不提供删除、批量删除、恢复或物理清理接口；
- 操作日志不保存密码、Access Token、Refresh Token、Cookie、Authorization、应用密钥或完整敏感请求头；
- 会话管理页不能踢出当前登录会话，避免管理员把自己踢下线；
- 当前切片不实现邮箱验证码、找回密码、登录限流、登录日志独立模块或监控指标面板。

## 3. 数据流

### 3.1 操作日志异步链路

```text
router
-> request middleware
-> auth middleware
-> permission middleware
-> handler
-> service
-> repository
-> model
-> PostgreSQL
-> operation-log middleware collects route result
-> queue client
-> Redis/Asynq
-> worker task handler
-> operation-log service
-> operation-log repository
-> operation-log model
-> PostgreSQL
```

操作日志不参与原业务事务，也不改变原业务响应。路由 Handler 完成后，中间件根据显式路由规则采集结果并入队。入队设置有界等待时间；入队失败只写结构化 API 日志，原业务响应保持已经产生的 HTTP 状态和 envelope。

Worker 对合法任务执行 PostgreSQL 插入；数据库失败由 Asynq 重试。任务 JSON 不完整、字段类型错误或版本不支持时明确记录并跳过重试，不能构造空日志。

### 3.2 会话管理链路

```text
router
-> authclient middleware
-> authentication middleware
-> permission middleware
-> session handler
-> session service
-> authstate invalidating lease
-> session repository
-> sys_user_session / sys_user
-> PostgreSQL
-> authstate generation publish
-> Redis session snapshot deletion
```

会话踢出必须先获取 `invalidating` lease，再完成 PostgreSQL 的 `revoked_at` 更新，提交后发布新 generation 并删除会话快照。Redis 失败不回滚已经提交的 PostgreSQL；generation 发布失败时不能恢复旧 ready state，后续认证只能遇到 invalidating 或回源 PostgreSQL并拒绝被撤销的会话。

## 4. 操作日志路由监控

### 4.1 显式路由规则

新增具体的操作规则配置，每条规则至少包含：

- HTTP method；
- Gin 完整路由模板，例如 `/api/v1/users/:id`；
- `module`，例如 `user`、`role`、`auth`、`session`；
- `action`，使用稳定的 lower camel case JSON 字段对应的内部操作码，例如 `user.update`、`session.revoke`；
- 前端展示用的 i18n key 或稳定标题 key；
- 是否采集请求体；
- 是否采集响应体。

规则必须在路由注册处或当前请求监控模块的明确列表中登记。不能根据 HTTP method 自动推断所有 POST/PUT/PATCH/DELETE，也不能使用运行时扫描注册器。

只读路由不登记操作日志规则，包括列表、详情、权限上下文、认证平台公开策略和角色/菜单选项读取。

### 4.2 采集字段

中间件在 `context.Next()` 返回后采集：

- `request_id`；
- `user_id`，匿名请求为空；
- `session_id`，没有身份时为空；
- `platform`；
- HTTP method 和完整路由模板；
- `module` 和 `action`；
- 客户端 IP 和 User-Agent；
- HTTP status；
- `is_success`，status 小于 400 为 1，否则为 0；
- handler 执行耗时毫秒数；
- 脱敏后的请求 JSON 摘要；
- 脱敏后的响应 envelope 摘要；
- 任务创建时间。

请求和响应摘要使用明确的 JSON 结构，不把原始 body 直接写入日志。字段名包含 `password`、`confirmPassword`、`accessToken`、`refreshToken`、`authorization`、`cookie`、`secret`、`key` 的值统一替换为固定掩码。单个摘要限制最大字节数，超出时只记录 `{"truncated":true}` 并丢弃内容；响应 writer 只保留固定上限字节，不把完整或未经解析的敏感 body 作为兜底数据保存。

### 4.3 异步任务

使用明确的 Asynq 任务类型 `system:operation-log:v2`，任务 payload 使用 schema version 2 的固定 DTO，至少包含：

- schema version；
- 服务端生成的 event ID；
- request ID；
- actor/session/platform 字段；
- method、route、module、action；
- client IP、User-Agent；
- status、success、latency；
- 已脱敏的 request/response JSON。

API 进程只负责采集和入队，Worker 负责解码、验证和落库。入队失败不得调用 PostgreSQL 同步补写，也不得写内存队列或返回假成功日志。结构化 API 日志必须包含 request ID、route、action 和 enqueue error。

## 5. 操作日志表

新增 `sys_operation_log`，这是追加不可变的审计记录表，不使用 `deleted_at`、`is_del` 或其他删除状态。

字段约束如下：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | `BIGSERIAL` | 主键 |
| `event_id` | `VARCHAR(64)` | 服务端操作事件标识，建立唯一索引，保证任务重试幂等 |
| `request_id` | `VARCHAR(128)` | 链路关联标识，允许客户端复用 |
| `user_id` | `BIGINT NULL` | 操作者，匿名登录失败可为空 |
| `session_id` | `BIGINT NULL` | 当前会话，可为空 |
| `platform` | `VARCHAR(49) NULL` | 认证平台 |
| `method` | `VARCHAR(10)` | HTTP method |
| `route` | `VARCHAR(255)` | Gin 路由模板 |
| `module` | `VARCHAR(64)` | 操作模块 |
| `action` | `VARCHAR(128)` | 稳定操作码 |
| `client_ip` | `VARCHAR(64)` | 客户端 IP |
| `user_agent` | `VARCHAR(512)` | User-Agent |
| `status_code` | `INTEGER` | HTTP status |
| `is_success` | `SMALLINT` | 0 = No，1 = Yes，带 CHECK |
| `latency_ms` | `BIGINT` | handler 耗时，必须大于等于 0 |
| `request_data` | `JSONB NULL` | 脱敏请求摘要 |
| `response_data` | `JSONB NULL` | 脱敏响应摘要 |
| `created_at` | `TIMESTAMPTZ NOT NULL` | 记录创建时间 |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | 记录更新时间 |

`created_at` 和 `updated_at` 由模型和 SQL 显式声明为 `TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP`。日志记录不会更新已存在行，但模型仍然显式声明两个字段以符合项目表规范。

只为真实查询建立索引：

- `ux_sys_operation_log_event_id`：`event_id` 唯一；
- `ix_sys_operation_log_request_id`：按链路标识查询；
- `ix_sys_operation_log_created_at`：按创建时间倒序分页；
- `ix_sys_operation_log_user_created`：按用户和时间筛选；
- `ix_sys_operation_log_action_created`：按操作码和时间筛选。

不建立未经页面需求证明的全文、响应 JSON 或 User-Agent 索引。

## 6. 操作日志查询接口

新增权限 `system:operation-log:list` 和系统菜单 `/system/operation-logs`。

接口：

```text
GET /api/v1/operation-logs
```

查询参数使用 lower camel case：

- `page`，从 1 开始；
- `pageSize`，范围 1 到 100；
- `userId`，可选正整数；
- `action`，可选前缀匹配；
- `route`，可选前缀匹配；
- `isSuccess`，可选 0 或 1；
- `from`、`to`，可选 ISO 日期时间范围。

响应使用项目统一 `code`、`data`、`message` envelope。列表返回操作人、method、route、action、平台、IP、状态、成功状态、耗时、创建时间和脱敏摘要；详情在当前页展开摘要，不新增单独详情接口。查询失败返回明确依赖错误，不能返回空列表伪装成功。

## 7. 会话管理接口

新增权限：

- `system:session:list`：查看会话列表；
- `system:session:revoke`：踢出会话。

新增系统菜单 `/system/sessions`，菜单和权限由内置菜单初始化写入 PostgreSQL，再通过现有 Access Snapshot 下发。

### 7.1 列表

```text
GET /api/v1/sessions
```

查询参数：

- `page`、`pageSize`；
- `username`，可选前缀关键词；
- `platform`，可选认证平台 code；
- `status`：`active`、`expired`、`revoked`。

列表字段：

- session ID、user ID、username；
- platform；
- device ID；
- client IP；
- User-Agent；
- `createdAt`、`updatedAt`、`refreshExpiresAt`、`revokedAt`；
- status。

状态由 PostgreSQL 当前时间计算：

- `active`：`revoked_at IS NULL` 且 `refresh_expires_at > now`；
- `expired`：`revoked_at IS NULL` 且 `refresh_expires_at <= now`；
- `revoked`：`revoked_at IS NOT NULL`。

不依赖 Redis 判断列表状态，Redis 只用于认证快照加速。

### 7.2 统计

新增：

```text
GET /api/v1/sessions/stats
```

统计当前 active 会话总数和按认证平台的数量。统计直接查询 PostgreSQL，不建立单独计数缓存。

### 7.3 单条踢出

```text
DELETE /api/v1/sessions/:id
```

请求体必须为空。Service 按以下顺序执行：

1. 验证 session ID；
2. 查询目标会话，不存在返回 404；
3. 拒绝当前操作者自己的 `session_id`；
4. 已撤销会话返回幂等结果，不重复修改时间；
5. 获取并续租该用户平台的 authstate invalidating lease；
6. 在 PostgreSQL 事务内锁定会话并设置 `revoked_at` 和 `updated_at`；
7. 发布新的 sessions generation；
8. 删除对应 Redis session snapshot；
9. 返回撤销结果。

### 7.4 批量踢出

```text
DELETE /api/v1/sessions
```

请求体严格为：

```json
{"ids":[1,2,3]}
```

ID 去重后最多 100 个。当前会话和已经撤销的会话不执行写入，并在响应中分别返回跳过数量。批量数据库更新必须是一次明确事务；Redis 快照删除失败返回依赖错误，不能伪造全部成功。

单条和批量踢出均由操作日志规则记录，日志异步入队不改变踢出接口的同步安全语义。

## 8. 前端页面

### 8.1 会话页面

页面路径：`web/src/views/system/sessions/index.vue`。

参考旧项目 SessionList，但使用当前项目的 API、严格 DTO 和 i18n：

- 状态统计区：active 总数和平台分布；
- 搜索区：用户名、平台、状态；
- 分页表格：用户、平台、设备、IP、User-Agent、创建/过期时间、状态；
- 单条踢出和批量踢出必须二次确认；
- 当前会话不显示可执行的踢出按钮；
- 所有加载、空数据、请求失败和 mutation 状态明确展示；
- 不使用显式 `any`、`as any` 或 `Record<string, any>`。

### 8.2 操作日志页面

页面路径：`web/src/views/system/operation-logs/index.vue`。

参考旧项目 operationLog，但只保留查询和详情查看：

- 用户、操作码、路由、成功状态、时间范围筛选；
- 分页列表；
- method、route、action、操作者、IP、状态、耗时、时间；
- 请求和响应脱敏摘要详情；
- 不提供删除按钮；
- 页面业务状态留在 view，API DTO 放在 `web/src/api/operation-log.contract.ts` 和 `web/src/api/operation-log.ts`；
- 所有外部数据从 `unknown` 开始解析并收窄到显式 DTO。

## 9. i18n 与错误

后端新增操作日志和会话管理的稳定 message key，并同步 `zh-CN`、`en-US` catalog。公开错误不包含 SQL、Redis key、任务 payload、Token 或内部堆栈。

至少覆盖：

- session ID 无效；
- 会话不存在；
- 当前会话不可踢出；
- 批量 ID 为空或超过上限；
- 会话查询、撤销或快照删除失败；
- 操作日志查询失败；
- 操作日志任务 payload 无效；
- 操作日志入队失败只进入结构化内部日志，不改业务响应。

## 10. 测试与验收

### 后端

- 操作规则只记录登记的操作类路由，普通 GET 查询不产生记录；
- 请求和响应敏感字段被掩码，原始密码和 Token 不进入任务或数据库；
- 操作日志入队成功时 API 响应保持原状态；Redis 不可用时 API 业务响应仍保持原状态并产生结构化 enqueue error；
- Worker 能解码合法任务、插入 PostgreSQL、对数据库错误重试，对非法 payload 跳过重试；
- 服务端 `event_id` 唯一约束保证同一任务重试不会产生重复日志，重复客户端 `request_id` 仍产生独立日志；
- 大请求和大响应只产生有界合法 JSON 摘要，操作路由 panic 产生 status 500 的失败日志；
- 会话 active/expired/revoked 状态和分页筛选正确；
- 单条和批量踢出不能撤销当前会话；
- PostgreSQL 撤销成功后 Redis 删除失败返回明确依赖错误，后续认证仍拒绝旧 Token；
- 权限和菜单初始化包含 session、operation-log 页面及按钮权限；
- 运行真实 PostgreSQL、Redis 和 Asynq Worker 集成测试，不使用内存数据库或内存队列替代。

### 前端

- 严格验证 session 和 operation-log response DTO；
- 列表分页、筛选、空态、错误态和二次确认行为通过 Vitest；
- 踢出成功后重新请求列表和统计，不做乐观删除；
- 操作日志详情只展示脱敏后的服务端数据；
- 动态菜单和按钮权限正确隐藏；
- `rg` 禁止的显式 `any` 检查无输出；
- `pnpm vitest run` 和 `pnpm build` 通过。

## 11. 不在本期实现

- 操作日志删除、归档、恢复或物理清理；
- 独立登录日志统计和登录风险评分；
- 进程级缓存；
- 操作日志全文搜索和导出；
- 邮箱验证码、找回密码、登录限流和注册防滥用；
- 通用 CRUD、全局 Dialog、全局 Table 或无真实重复依据的抽象。
