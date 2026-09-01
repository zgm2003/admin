# 邮件服务模块设计

## 1. 状态与范围

状态：设计草案，等待评审。

本模块为 Admin 提供腾讯云 SES 邮件配置、固定业务模板、发送事实查询、测试发送和收件人黑白名单。邮件是管理员邀请、邮箱验证码和后续邮箱安全流程的发送基础设施。

本模块包含：

- 腾讯云 SES 单配置管理；
- 四个固定模板场景及其腾讯云模板 ID；
- 邮件发送记录和 Admin 诊断详情；
- 收件人精确邮箱/域名黑名单和白名单；
- 公开业务发送与 Admin 测试发送的独立限流策略；
- Admin 页面中的四个内部 Tab。

本模块不包含：

- 短信、站内通知、WebSocket 或通用消息中心；
- 多云、多邮件供应商、Provider/Adapter/Factory 抽象；
- 邮件正文编辑或本地 HTML 模板渲染；
- 公共注册、邮箱验证码登录、管理员邀请流程本身；
- 队列监控、通用事件总线或可配置工作流。

`menu.remark` 是独立的菜单表改动。本模块只依赖最终菜单协议，不修改菜单表字段，也不把该改动混入邮件提交。

## 2. 既有资产继承

老项目已经存在成熟的邮件页面、腾讯云 SES 调用和模板元数据，当前项目只迁移业务事实和交互，不迁移老项目的 MySQL、`is_del=2` 语义或重型模块层。

固定模板场景和 ID 直接沿用：

| scene | 名称 | Tencent Template ID | 变量 |
| --- | --- | ---: | --- |
| `login` | 邮箱验证码登录 | `47941` | `code`, `ttl_minutes` |
| `forget` | 找回密码 | `47942` | `code`, `ttl_minutes` |
| `bind_email` | 绑定/换绑邮箱 | `47943` | `code`, `ttl_minutes` |
| `change_password` | 验证码改密 | `47944` | `code`, `ttl_minutes` |

腾讯云控制台中的模板正文是权威事实。Admin 只维护场景、名称、主题展示名、模板 ID、变量声明和示例变量，不保存或发送本地正文。

## 3. 菜单与权限

新增一个页面菜单，挂在 `消息服务` 目录下：

```text
消息服务
└─ 邮件服务
```

页面权限使用新协议：

```text
system:mail:view
```

页面内部使用四个 Tab，不创建四个子页面或四个左侧菜单：

1. 邮件配置
2. 邮件模板
3. 发送日志
4. 黑白名单

读取和操作权限独立授权，不由 `system:mail:view` 自动扩权：

| 权限码 | 用途 |
| --- | --- |
| `system:mail:list` | 读取配置、模板、黑白名单列表和页面初始化字典 |
| `system:mail:detail` | 读取单条发送日志详情 |
| `system:mail:config:update` | 保存或修改 SES 配置 |
| `system:mail:config:delete` | 删除 SES 配置 |
| `system:mail:test` | 发送 Admin 测试邮件 |
| `system:mail:template:create` | 创建模板元数据 |
| `system:mail:template:update` | 修改模板元数据 |
| `system:mail:template:status` | 启停模板 |
| `system:mail:template:delete` | 删除模板 |
| `system:mail:log:delete` | 删除发送日志 |
| `system:mail:rule:create` | 创建黑白名单规则 |
| `system:mail:rule:update` | 修改黑白名单规则 |
| `system:mail:rule:status` | 启停黑白名单规则 |
| `system:mail:rule:delete` | 删除黑白名单规则 |

Tab 显示规则：页面入口只由 `system:mail:view` 控制；每个 Tab 的数据请求和按钮再由对应的 `list/detail/action` 权限控制。没有日志权限时，发送日志 Tab 不显示；没有写权限时只显示只读状态，不伪造按钮。

## 4. 页面结构

邮件服务页面使用一个 `el-tabs`，默认进入“邮件配置”。四个 Tab 均使用懒加载，切换 Tab 后才请求该 Tab 的数据。

### 4.1 邮件配置

展示：

- 是否已配置、启用状态、地域、Endpoint；
- 发件邮箱、发件名称、Reply-To；
- 验证码有效期，范围 1 到 60 分钟；
- 最近一次测试时间和安全的失败摘要。

操作：

- 保存配置；
- 连接/发送测试邮件；
- 删除配置。

SecretId 和 SecretKey 在数据库中使用项目现有凭据加密机制保存。读取接口只返回 `configured` 和短提示，不返回密钥、密文或可逆线索。编辑时密钥输入框为空，空值表示保留旧密钥。

### 4.2 邮件模板

展示四个固定场景的模板元数据、启用状态、腾讯云模板 ID、变量和示例变量。

场景是稳定业务协议，固定场景只能编辑名称、展示主题、模板 ID、变量和状态；不允许把一个场景改成另一个场景，也不允许删除系统仍依赖的唯一模板。模板必须启用后才能发送业务邮件。

### 4.3 发送日志

展示发送事实：收件邮箱、场景、模板 ID、发送状态、腾讯云 Request ID/Message ID、失败码、失败摘要、耗时、发送时间和创建时间。

Admin 端按用户要求提供完整诊断字段，包括：

- 完整收件邮箱；
- 验证码明文；
- 验证码状态和过期时间；
- 腾讯云错误码和错误消息。

这些字段只允许具备 `system:mail:detail` 或日志读取权限的 Admin API 返回。它们不得出现在公开业务响应、普通应用日志、操作日志请求体或响应体中。数据库中验证码仍以独立密文快照保存，日志查询时在受保护的 Admin service 层解密。

日志状态固定为：

- `pending`：已建立发送记录，尚未得到腾讯云结果；
- `sent`：腾讯云接受发送；
- `failed`：配置、规则、限流或腾讯云调用失败。

失败必须显式返回，不自动切换其他供应商，不伪造成功。日志删除是软删除；删除日志不影响验证码挑战状态或模板配置。

### 4.4 黑白名单

规则类型：

- `email`：完整邮箱，规范化后精确匹配；
- `domain`：域名匹配，`example.com` 匹配 `user@example.com` 及其子域名，边界按 DNS 标签判断，不做字符串后缀误匹配。

规则动作：

- `deny`：禁止发送；
- `allow`：允许通过收件人策略。

匹配顺序固定为：

1. 精确邮箱规则优先于域名规则；
2. 同一匹配级别内 `deny` 优先于 `allow`；
3. 没有匹配规则时按默认策略处理；
4. 默认策略为允许普通业务发送，Admin 测试发送也必须经过规则检查。

白名单只表达“允许该收件人”，不绕过任何限流、模板状态、配置状态或平台权限。黑名单命中返回统一的发送拒绝错误，不向公开调用者泄露是哪个规则命中；Admin 日志可记录规则 ID 和拒绝原因。

规则字段：`id`、`scope`、`pattern`、`action`、`name`、`remark`、`is_enabled`、`created_at`、`updated_at`、`deleted_at`。同一平台、同一 scope、同一规范化 pattern、同一 action 的有效规则唯一。

## 5. 发送链路

业务发送由内部 Mail service 调用，Admin 测试发送走独立 HTTP API；两者共用模板读取、收件人规范化、黑白名单判定、限流判定、腾讯云 SES 客户端和日志落库。

```text
调用方
  -> Mail service
  -> 规范化收件邮箱
  -> 检查配置和模板
  -> 黑白名单判定
  -> 对应限流器
  -> 创建 pending 日志
  -> 腾讯云 SES SendEmail
  -> 完成 sent/failed 日志
  -> 返回明确结果
```

验证码业务必须能同步知道发送是否成功，因此第一期采用带超时的同步 SES 调用，不引入发送队列。单次腾讯云请求超时建议 8 秒；不做跨供应商 fallback。后续若真实发送量需要异步化，另开设计，不在本模块预留 outbox 或通用任务表。

重复请求的幂等键由调用方提供业务 challenge ID；同一 challenge 不重复发送成功邮件。没有 challenge ID 的 Admin 测试发送按独立请求处理，但仍受测试限流约束。

## 6. 双层限流

限流必须保护两个入口：

### 6.1 公开业务发送

至少同时按以下维度限流：

- 收件邮箱；
- 客户端 IP；
- 业务场景。

建议默认窗口：同一邮箱同一场景 60 秒最多 1 次，10 分钟最多 5 次；同一 IP 1 分钟最多 10 次。实际窗口和阈值进入配置常量，禁止由请求参数指定。

### 6.2 Admin 测试发送

至少同时按以下维度限流：

- 当前 Admin 用户；
- 客户端 IP；
- 收件邮箱。

Admin 测试发送额度可以比公开业务更宽，但不能无限制，也不能由白名单绕过。测试发送场景固定为四个模板之一，不能提交任意腾讯云模板 ID 或任意正文。

限流器使用 Redis 原子计数/过期窗口，多个 API 实例共享状态；HTTP 层使用成熟开源 rate-limit middleware 负责快速拒绝，业务 service 再按邮箱和场景执行业务维度限流。Redis 不可用时，公开发送和 Admin 测试发送均失败关闭，不退化为进程内无限放行。

## 7. 数据模型

使用 PostgreSQL、`TIMESTAMPTZ`、软删除和项目统一 `Yes/No` 语义。

### 7.1 `system_mail_config`

单个 Admin 平台一条有效配置，字段包括：加密 SecretId、加密 SecretKey、提示值、地域、Endpoint、发件邮箱、发件名称、Reply-To、验证码 TTL、启用状态、最近测试时间、最近测试错误、审计时间和软删除时间。

### 7.2 `system_mail_template`

固定场景唯一；字段包括：scene、name、subject、Tencent template ID、变量 JSON、示例变量 JSON、启用状态、审计时间和软删除时间。scene 使用数据库 CHECK 和唯一有效索引约束四个场景。

### 7.3 `system_mail_log`

字段包括：scene、template_id、to_email、subject、状态、腾讯云 Request ID、Message ID、错误码、错误摘要、耗时、发送时间、创建时间和软删除时间。日志不保存正文和未加密验证码。

### 7.4 `system_mail_log_verification`

一条日志最多一份验证码诊断快照：mail_log_id、密钥版本、验证码密文、过期时间、创建时间。该表只能由 Mail service 访问，Repository 不把密文直接映射到公开 DTO。

### 7.5 `system_mail_recipient_rule`

字段见黑白名单章节。pattern 入库前统一小写、去除首尾空白并验证邮箱/域名格式；规则查询按 scope/pattern 建索引，所有有效规则按平台隔离。

## 8. API 边界

Admin API 保持 `/api/admin/v1` 和 lower camel case 响应约定，建议路径：

```text
GET    /mail/page-init
GET    /mail/config
PUT    /mail/config
DELETE /mail/config
POST   /mail/test
GET    /mail/templates
POST   /mail/templates
PUT    /mail/templates/:id
PATCH  /mail/templates/:id/status
DELETE /mail/templates/:id
GET    /mail/logs
GET    /mail/logs/:id
DELETE /mail/logs/:id
DELETE /mail/logs
GET    /mail/recipient-rules
POST   /mail/recipient-rules
PUT    /mail/recipient-rules/:id
PATCH  /mail/recipient-rules/:id/status
DELETE /mail/recipient-rules/:id
```

公开业务发送不直接暴露 Mail provider 或模板正文，只由认证/用户模块通过 Go interface 调用。公开接口不返回验证码、腾讯云错误详情、Secret 信息或规则命中细节。

## 9. 安全与日志

- Secret 使用固定用途的 AES-256-GCM 派生密钥，密文带 key ID 和随机 nonce；禁止复用 JWT/Refresh Token 密钥。
- API 响应、普通 slog、操作日志 payload 均不得包含 Secret、Secret 密文、验证码或模板变量原文。
- Admin 邮件日志接口显式关闭请求体/响应体操作日志记录，仅记录“谁在何时查看或删除哪条邮件日志”的元信息。
- 邮箱地址在 Admin 页面按用户要求完整显示；公开错误使用统一文案，避免枚举收件人和规则。
- 腾讯云错误消息只保存经过长度限制的摘要，禁止把完整 SDK 对象或凭据写入日志。
- 配置、模板、规则状态变化写入操作日志；读取邮件日志也写入最小化审计事件。

## 10. 启动、迁移与缓存

数据库迁移只允许在人工执行的 SQL/迁移流程中完成一次。API 和 Worker 启动不得执行迁移、初始化模板、重建菜单或写入默认数据。

邮件配置、模板和规则更新后，通过权限/配置版本或显式失效通知清理进程缓存。Redis 只缓存短期读取结果和限流计数，不缓存 Secret 明文、验证码明文或完整邮件正文。

## 11. 测试与验收

后端单元/集成测试必须覆盖：

- 四个固定模板 ID、变量和场景校验；
- 首次配置、保留旧密钥、删除配置和连接测试；
- 精确邮箱、域名、子域名、边界域名和规则优先级；
- 黑名单拒绝、白名单允许但不能绕过限流；
- 公开发送和 Admin 测试发送分别按邮箱/IP/用户限流；
- Redis 不可用时拒绝发送；
- pending -> sent/failed 状态转换和腾讯云错误映射；
- Admin DTO 可显示完整诊断，公开接口和普通日志无敏感字段；
- 模板停用、配置停用、无模板和 SES 超时均明确失败；
- 多平台数据隔离和规则唯一冲突。

前端测试必须覆盖：

- 页面只显示一个四 Tab 的邮件服务入口；
- 无日志权限时日志 Tab 隐藏，无写权限时按钮隐藏；
- 配置密钥输入框编辑时为空且不渲染旧值；
- 模板 ID、变量、状态和四个场景展示；
- 日志筛选、详情诊断、批量删除和失败空态；
- 黑白名单新增、编辑、启停、删除和匹配类型展示；
- API 响应字段严格校验，拒绝未知或缺失敏感字段。

人工验收至少包括：

1. 腾讯云 SES 配置保存、连接测试和停用；
2. 四个模板 ID 与腾讯云控制台模板一致；
3. Admin 测试发送成功/失败均能看到完整日志诊断；
4. 黑名单阻断虚拟邮箱，白名单可放行指定邮箱但仍受限流；
5. 公开业务发送在邮箱/IP维度触发限流；
6. API/Worker 重启不执行迁移和默认数据写入；
7. 日志、操作日志和 API 响应中无 Secret、密文或公开端验证码泄露。

## 12. 实施顺序

1. 先确认菜单 `remark` 字段改动已独立完成，并确认最终 `:view/:list/:detail` 权限语义；
2. 建立 PostgreSQL 表、约束、索引和一次性 SQL；
3. 实现凭据加密、SES client、模板元数据和同步发送 service；
4. 实现日志与验证码密文快照；
5. 实现 Redis 双层限流与黑白名单规则；
6. 实现 Admin API 和严格 DTO；
7. 实现四 Tab 页面和权限显示；
8. 运行后端、前端测试并进行真实 SES/限流/规则人工验收；
9. 仅在真实可用后写入消息服务与邮件服务菜单基础数据。
