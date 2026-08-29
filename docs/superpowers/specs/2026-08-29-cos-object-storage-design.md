# Admin 腾讯云 COS 对象存储设计

状态：设计已按业务讨论落稿，等待用户审阅

日期：2026-08-29

## 1. 目标

为 Admin 建立一套只面向腾讯云 COS 的对象存储管理能力，先解决高频的配置、上传规则和
浏览器直传授权问题，同时把表名、模块路径、权限和删除语义固定下来。设计必须能被未来
业务端复用，但不把 Canvas、AI、支付或文件业务倒灌进 Admin。

本设计是 COS 独立模块的边界，不是多云存储抽象，也不是文件资产管理系统。实现前必须先
按本文件确认数据库、接口和菜单，再编写实施 plan。

## 2. 已确认范围

### 2.1 进入本期

- 仅腾讯云 COS，不支持 OSS、S3 或任何动态 Provider/Driver/Adapter。
- Admin 对 COS 配置的新增、查看、编辑、启停、连接测试和软删除。
- Admin 对上传规则的新增、查看、编辑、启停和软删除。
- 服务端依据上传规则批量生成短时预签名 PUT URL，浏览器直接上传到 COS。
- 配置密钥加密存储，接口、普通日志和操作日志均不得回显密钥或签名内容。
- 所有数据库变更由维护者手工执行事务化 SQL migration，API/Worker 启动不执行迁移或 seed。

### 2.2 不进入本期

- 文件管理页面、文件元数据事实表、文件预览、回收站、孤儿文件清理和生命周期管理。
- 通过 Admin 后端代理文件内容上传；文件不经过 Admin 服务端带宽链路。
- STS 临时密钥、分片上传编排、断点续传和服务端内容安全扫描。
- 业务项目的图片、附件、头像等领域表。业务项目保存返回的 object key，并自行定义引用关系。
- 多云统一接口、存储驱动注册器、万能 StorageManager 或配置驱动页面引擎。

文件管理不是 COS 配置的自然子功能。它需要单独定义对象元数据、业务引用、删除时机、
预览授权和清理策略，未来真实出现需求时另开 spec。

## 3. 菜单位置与权限

### 3.1 菜单树

Admin 平台菜单在模块真实可用后通过人工菜单 migration 增加，不在 API/Worker 启动时创建
占位数据：

```text
云服务                         directory: cloud
└─ 对象存储                     page: storage:object:list
   ├─ COS 配置                  页面内 Tab
   └─ 上传规则                  页面内 Tab
```

页面固定使用：

```text
path:           /cloud/object-storage
componentPath:  storage/object
routeName:      access:storage:object:list  # 由现有动态路由按菜单 code 生成
i18nKey:        navigation.storageObject
icon:           lucide:cloud-upload
```

`云服务` 目录使用 `navigation.cloud` 和 `lucide:cloud`。目录、页面和动作仍遵守当前
`rbac_menu` 的同平台父子关系，菜单的 `platform_id` 只指向 Admin 平台；不为 Canvas 预置
同名菜单。

### 3.2 动作权限

页面权限负责进入对象存储页面；动作权限只控制真实操作，不为两个 Tab 额外制造页面节点：

| 权限码 | 用途 |
| --- | --- |
| `storage:cos-config:create` | 新增 COS 配置 |
| `storage:cos-config:update` | 编辑 COS 配置元数据或替换密钥 |
| `storage:cos-config:status` | 启用或停用 COS 配置 |
| `storage:cos-config:test` | 发起连接测试 |
| `storage:cos-config:delete` | 软删除 COS 配置 |
| `storage:upload-rule:create` | 新增上传规则 |
| `storage:upload-rule:update` | 编辑上传规则 |
| `storage:upload-rule:status` | 启用或停用上传规则 |
| `storage:upload-rule:delete` | 软删除上传规则 |
| `storage:object:upload` | 申请预签名上传 URL |

`storage:object:list` 是页面节点，权限读取时会按照当前平台过滤。未来业务平台若需要上传，
必须在该平台自己的菜单和角色中显式授予 `storage:object:upload`；Admin 不会把 Admin 平台
权限返回给 Canvas 或其他平台。上传规则本身携带目标 `platform_id`，因此同名规则也不会在
不同认证平台之间串用。

### 3.3 前端页面

页面归位为：

```text
web/src/modules/storage/object/index.vue
web/src/api/storage/cosconfig.ts
web/src/api/storage/uploadrule.ts
```

页面使用现有 Element Plus 表格、Tabs、表单、确认框和空态组件。配置和规则是两个页面内部
视图，连接测试、恢复编辑态、启停和删除都是 Tab 内按钮，不单独占左侧菜单。

## 4. 模块和依赖方向

后端按表名对应子模块组织：

```text
server/internal/module/storage/
  cosconfig/       # storage_cos_config
  uploadrule/      # storage_upload_rule；规则校验和上传凭证申请
server/internal/storage/
  cos/             # 腾讯云 COS 具体 SDK 客户端，不是多云抽象
```

普通数据库请求保持：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

COS SDK 只在 `storage/cos` 中出现。`cosconfig.Service` 负责配置生命周期、密钥解密和连接
测试；`uploadrule.Service` 负责规则生命周期、请求校验、对象 key 生成和调用 COS 签名能力。
Handler 不访问 GORM 或 COS SDK，Repository 不编排第三方调用。

上传凭证没有事实表，因此不创建 `storage_upload_token` 或 `uploadtoken` 表。凭证申请属于
上传规则服务的短时能力，签名完成后立即返回，过期由 COS 和 URL 自身保证。

## 5. 数据模型

### 5.1 `storage_cos_config`

COS 配置是一套可被多个上传规则引用的真实 Bucket 凭据。字段固定如下：

```text
id                     BIGINT PRIMARY KEY
name                   VARCHAR(128) NOT NULL
app_id                 VARCHAR(32) NOT NULL
secret_id_ciphertext   TEXT NOT NULL
secret_key_ciphertext  TEXT NOT NULL
bucket                 VARCHAR(128) NOT NULL
region                 VARCHAR(64) NOT NULL
endpoint               VARCHAR(255) NULL
bucket_domain          VARCHAR(255) NULL
is_enabled             SMALLINT NOT NULL DEFAULT 1 CHECK (is_enabled IN (0, 1))
remark                 VARCHAR(512) NOT NULL DEFAULT ''
created_at             TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
updated_at             TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
deleted_at             TIMESTAMPTZ NULL
```

说明：

- `app_id`、`bucket` 和 `region` 是腾讯云 COS 配置事实，不用 `driver` 字段表示提供商。
- `endpoint` 为空时由 COS 客户端按 Bucket/Region 推导；配置自定义域名时使用
  `bucket_domain` 作为访问域名，但签名仍以实际 COS endpoint 为准。
- SecretId/SecretKey 使用现有应用根密钥派生的独立加密用途进行加密。页面只得到
  `hasCredentials`，不返回任何密钥片段、密文或提示值。
- `is_enabled` 和 `deleted_at` 是两种不同语义：停用保留可恢复配置，删除表示不再参与正常
  查询。删除操作会同时把配置置为停用。

### 5.2 `storage_upload_rule`

上传规则描述某类上传的安全边界和对象落点。字段固定如下：

```text
id                    BIGINT PRIMARY KEY
platform_id           BIGINT NOT NULL
code                  VARCHAR(64) NOT NULL
name                  VARCHAR(128) NOT NULL
cos_config_id         BIGINT NOT NULL
path_prefix           VARCHAR(255) NOT NULL
max_file_size_bytes   BIGINT NOT NULL CHECK (max_file_size_bytes > 0)
max_file_count        INTEGER NOT NULL DEFAULT 1 CHECK (max_file_count > 0)
allowed_extensions    TEXT[] NOT NULL
allowed_mime_types    TEXT[] NOT NULL
access_mode           VARCHAR(16) NOT NULL DEFAULT 'private'
                      CHECK (access_mode IN ('private', 'public'))
is_enabled            SMALLINT NOT NULL DEFAULT 1 CHECK (is_enabled IN (0, 1))
remark                VARCHAR(512) NOT NULL DEFAULT ''
created_at            TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
updated_at            TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
deleted_at            TIMESTAMPTZ NULL
```

关系固定为：

```text
storage_upload_rule.cos_config_id
  -> storage_cos_config.id ON DELETE RESTRICT

storage_upload_rule.platform_id
  -> auth_platform.id ON DELETE RESTRICT
```

规则约束：

- `platform_id` 是规则适用的认证平台。凭证申请只查当前认证平台下的规则；不同平台可拥有
  同名 code，不能跨平台使用对方规则。
- `code` 是业务端稳定引用值，例如 `avatar`、`article-attachment`；不允许使用显示名称作为
  引用。有效未删除规则在同一 `platform_id` 下唯一。
- 扩展名统一转小写并去掉前导点后保存，例如 `jpg`、`png`；MIME 类型统一小写。申请上传
  时扩展名和 MIME 都必须满足规则中非空的限制集合；至少配置一个允许扩展名，默认拒绝未知
  扩展名，避免空规则意外开放全部文件。
- `path_prefix` 必填且只能是相对对象 key 前缀，不允许以 `/` 开头、包含 `..` 或覆盖 Bucket
  根路径。客户端不能直接提交最终对象 key。
- `private` 是默认访问模式；`public` 必须由管理员明确选择，并在页面给出风险提示。私有
  对象的读取 URL 不在本期 Admin 页面中自动生成。
- `max_file_count` 表示一次凭证申请允许的最大文件数，不表达业务表单的总附件数；后者由
  未来业务模块定义。数值字段由 Service 做上限、范围和整数校验，数据库负责非零和枚举级别
  的底线约束。单文件大小上限不得超过 COS 单个 PUT 对象的 5 GiB 上限。
- 创建、更新或启用规则时，目标 `platform_id` 必须是未删除且启用的认证平台；`access_mode`
  为 `public` 时，其 COS 配置必须具有有效 `bucket_domain`。

### 5.3 索引

只为真实查询建立索引：

```text
ux_storage_cos_config_name_active
  UNIQUE (LOWER(name)) WHERE deleted_at IS NULL

ix_storage_cos_config_enabled_created_at
  (is_enabled, created_at DESC, id DESC) WHERE deleted_at IS NULL

ux_storage_upload_rule_platform_code_active
  UNIQUE (platform_id, code) WHERE deleted_at IS NULL

ix_storage_upload_rule_config_enabled_created_at
  (cos_config_id, is_enabled, created_at DESC, id DESC)
  WHERE deleted_at IS NULL
```

不为 `is_enabled`、`deleted_at`、`access_mode` 单独建立低选择性索引，不为数组内容建立
GIN 索引；规则读取是管理列表和按 code 查找，不是数组反向搜索。

## 6. 密钥、配置和删除语义

### 6.1 密钥处理

- 创建和替换配置时接收 SecretId/SecretKey 原文，只在 Service 内短暂存在。
- Repository 只写密文；Model、Response、日志和审计 DTO 不包含明文字段。
- 列表和详情只返回 `hasCredentials`，不返回密文、密钥片段或任何 hint。
- `secretkey.KeyRing` 以现有 `APP_SECRET` 通过 HKDF 使用固定用途
  `admin:storage:cos-encryption:v1` 派生独立 32 字节密钥；禁止直接复用 JWT 或 refresh-token
  HMAC key。
- 密文采用 AES-256-GCM、随机 nonce 和版本化 Base64 文本格式；解密失败视为依赖/配置错误，
  不把密文、nonce 或底层错误回传给客户端。
- 编辑时密钥字段采用“缺失表示保持原值，显式新值表示替换”的严格 DTO 语义；不提供把
  密钥改成空字符串的隐式清除操作。
- 连接测试在内存中解密并调用 COS 的最小权限请求，响应只返回成功或公开错误码，第三方
  SDK 的 endpoint、请求签名和详细错误写入受控服务端日志时也必须脱敏。

### 6.2 配置删除

配置使用软删除，不提供物理删除 API。Service 在同一事务内锁定配置后检查：

```sql
storage_upload_rule.deleted_at IS NULL
AND storage_upload_rule.is_enabled = 1
AND storage_upload_rule.cos_config_id = :config_id
```

存在启用中的规则时返回冲突，配置不改变；必须先停用或删除这些规则。停用规则仍保留
历史关联，配置删除后不能再次启用，必须先把规则改绑到未删除且可用的 COS 配置。

删除配置时同时写 `is_enabled = 0`、`deleted_at` 和 `updated_at`，但不删除 COS Bucket 中
任何对象。

### 6.3 规则删除和更新

- 规则也只软删除；删除后从正常列表和按 code 查询中消失。
- 删除启用规则时要求先停用，避免请求方在配置变更窗口中继续申请凭证。
- 启用规则时必须锁定并验证其 COS 配置存在、未删除且已启用；否则返回明确冲突。
- 更新 `cos_config_id`、路径前缀、访问模式或大小限制属于配置变更，必须记录操作审计。

## 7. 上传凭证流程

### 7.1 推荐模式

采用服务端签发短时预签名 PUT URL，浏览器直接上传 COS：

```text
业务前端
  -> POST /api/v1/storage/upload-credentials
  -> 认证、当前平台和 storage:object:upload 权限
  -> uploadrule.Service 校验 ruleCode、文件名、MIME、大小和批量数量
  -> Repository 读取启用规则并锁定关联 COS 配置
  -> storage/cos 解密凭据并生成随机 object key
  -> 返回短时 uploadUrl 和 objectKey
业务前端
  -> PUT uploadUrl 直接上传 COS
  -> 业务端保存 objectKey
```

请求只接受：

```json
{
  "ruleCode": "avatar",
  "files": [
    {
      "fileName": "photo.png",
      "contentType": "image/png",
      "fileSizeBytes": 123456
    }
  ]
}
```

响应只包含：

```json
{
  "items": [
    {
      "uploadUrl": "https://...",
      "objectKey": "avatar/2026/08/29/<random>.png",
      "method": "PUT",
      "headers": {
        "Content-Type": "image/png"
      },
      "expiresAt": "2026-08-29T12:00:00Z"
    }
  ]
}
```

规则校验和 key 生成要求：

- 不信任客户端提交的目录或 object key；服务端使用规则前缀、日期分区和随机 ID 生成。
- 文件扩展名从规范化后的文件名取得，禁止路径分隔符、控制字符和 `..`。
- `contentType` 和 `fileSizeBytes` 必须通过规则限制；MIME 只是请求元数据，不等于服务端
  已检查文件真实内容。本期不宣称提供内容安全扫描。
- COS 签名必须绑定服务端生成的 object key、请求 method、`Content-Type` 和实际文件长度；
  浏览器会自动发送 Content-Length，客户端不能通过请求 DTO 自定义 key 或放宽规则上限。
- `access_mode=public` 时，签名固定携带 `x-cos-acl: public-read`，响应额外返回由
  `bucket_domain + objectKey` 组成的 `publicUrl`；`private` 时不携带 ACL，也不返回读取 URL。
- URL 有效期固定为 10 分钟，不能由客户端指定，也不在本期变成规则字段。有效期作为 COS
  模块常量写入测试；以后出现真实差异需求时再单独设计。
- 预签名 URL、签名 query、临时 headers 和原始错误不进入操作日志、访问日志摘要或 HTTP
  错误 message。凭证接口不是管理变更，不产生 `audit_operation_log`。

### 7.2 当前 Admin 边界

Admin 页面只管理配置和规则，不新增文件浏览器或上传演示区。凭证接口属于经过平台权限
保护的基础能力；未来 Canvas 或其他业务前端接入前，必须在对应平台补齐菜单、权限和业务
对象引用规则，不得凭 Admin 的角色权限直接访问其他平台的数据。

## 8. 管理 API 边界

管理接口统一使用 `/api/admin/v1`，并经过 Admin 平台、认证和权限中间件：

```text
GET    /api/admin/v1/storage/cos-configs
POST   /api/admin/v1/storage/cos-configs
GET    /api/admin/v1/storage/cos-configs/:id
PUT    /api/admin/v1/storage/cos-configs/:id
PATCH  /api/admin/v1/storage/cos-configs/:id/status
POST   /api/admin/v1/storage/cos-configs/:id/test
DELETE /api/admin/v1/storage/cos-configs/:id

GET    /api/admin/v1/storage/upload-rules
GET    /api/admin/v1/storage/upload-rules/page-init
POST   /api/admin/v1/storage/upload-rules
GET    /api/admin/v1/storage/upload-rules/:id
PUT    /api/admin/v1/storage/upload-rules/:id
PATCH  /api/admin/v1/storage/upload-rules/:id/status
DELETE /api/admin/v1/storage/upload-rules/:id
```

接口约束：

- 列表默认过滤 `deleted_at IS NULL`，返回分页结果和明确的启用状态。
- 上传规则列表支持 `platformId`、`cosConfigId`、`keyword`、`isEnabled`、`page` 和
  `pageSize` 筛选；`page-init` 一次返回可选认证平台和未删除 COS 配置的安全摘要，前端不再
  从多个无关页面 API 拼接下拉数据。
- 配置响应只返回元数据、状态和 `hasCredentials`；任何响应都不能出现 `secretId`、
  `secretKey` 明文、ciphertext、密钥片段或 SDK 配置对象。
- 创建、更新和删除使用严格 JSON DTO，拒绝未知字段、重复 key、缺失必填字段和非法数组值。
- 连接测试失败返回 `dependencyUnavailable` 或明确的配置错误，不返回 COS SDK 原始文本。
- 配置被启用规则引用删除时返回 `conflict`，并保证事务回滚。
- 所有成功和失败响应继续使用 `{ code, data, message }` envelope。

上传凭证接口使用共享 `/api/v1`，由当前平台上下文和 `storage:object:upload` 权限保护；它
只读取当前 `platform_id` 的启用规则，不属于 Admin 管理 URL，也不把凭证申请记录成操作审计
事件。接口的业务平台菜单和更细的对象归属必须在业务平台真正接入时另行确认。

## 9. 审计和错误

### 9.1 操作审计

以下 Admin 变更进入 `audit_operation_log`：

- COS 配置创建、编辑、启停、连接测试和软删除；
- 上传规则创建、编辑、启停和软删除；
- 规则重新绑定 COS 配置、访问模式或限制值变化。

审计 payload 只记录配置/规则 ID、code、目标平台、变更前后状态、Bucket/Region 的非敏感
摘要和结果。Secret、ciphertext、连接测试和凭证申请不得把 upload URL、签名 query、
Authorization 或 SDK headers 放入日志。

### 9.2 公开错误

沿用现有错误 envelope 和错误码：

| 情况 | 公开语义 |
| --- | --- |
| 请求字段或数组非法 | `invalidRequest` |
| 未认证或没有当前平台权限 | `unauthorized` / `forbidden` |
| 配置或规则不存在 | `notFound` |
| code/name 冲突、配置仍被启用规则引用、规则绑定不可用配置 | `conflict` |
| PostgreSQL、Redis（如使用）或 COS SDK 不可用 | `dependencyUnavailable` |

内部 cause 仅进入受控服务端日志；客户端不能看到 SQL、堆栈、DSN、Secret、Token 或第三方
签名细节。

## 10. 数据库迁移和启动边界

实现时新增人工 migration，例如：

```text
docs/database/2026-08-29-cos-object-storage.sql
docs/database/2026-08-29-cos-object-storage.md
```

migration 必须：

- 在 `BEGIN`/`COMMIT` 事务内创建两张表、外键、CHECK、部分唯一索引和查询索引；
- 可重复执行，前置冲突显式失败，不覆盖已有业务数据；
- 不插入 Canvas、文件管理或其他未来菜单，不创建上传 token 表；
- 在删除配置的约束上依赖真实外键和 Service 事务检查，不通过 `is_del` 模拟关系。

API 和 Worker 启动只连接既有数据库和 Redis 依赖，不执行 AutoMigrate、seed、菜单
foundation、数据回填或 Redis 清理。COS 只在连接测试和凭证申请时按需调用，菜单基础数据在
模块真正上线时随人工 SQL 明确增加。

## 11. 测试和验收

### 11.1 后端

- Model/Repository 使用真实 PostgreSQL 验证两表、字段、CHECK、外键、部分唯一索引和软删
  查询；验证 `ON DELETE RESTRICT` 阻止物理删除引用配置。
- Service 覆盖：创建和编辑、密钥替换、连接测试成功/失败、规则 code 冲突、无效配置绑定、
  启用规则引用时删除配置冲突、停用规则后删除配置、删除规则后按 code 不可见。
- Handler 覆盖：未知字段、重复 JSON key、缺字段、非法扩展名/MIME/大小、严格 envelope 和
  secret 字段不出现在响应。
- COS 客户端边界使用最小替换接口测试签名参数、有效期、Content-Type 和随机 object key；
  不把真实 Secret 写入测试日志。
- 启动源代码扫描确认 API/Worker 没有 migration、seed、菜单插入或凭证清理调用。

### 11.2 前端

- API DTO 运行时解析拒绝缺字段、额外字段和错误类型；配置响应只能消费 `hasCredentials`
  和非敏感配置元数据。
- 对象存储页面只展示两个 Tab，配置和规则的按钮按 action permission 控制；删除和连接
  测试使用现有 Element Plus 确认/结果交互。
- 中英文文案、主题持久化、RouterTabs 和动态菜单行为遵守既有基线；不引入 Playwright 作为
  默认验证，先使用 Vitest、类型检查、构建和人工验收。

### 11.3 人工验收标准

1. Admin 权限用户能在“云服务 -> 对象存储”页面切换 COS 配置和上传规则两个 Tab。
2. 配置列表不显示任何 Secret 原文；新增、编辑、连接测试、启停和软删除行为清晰可见。
3. 有启用上传规则引用时删除 COS 配置必定失败且数据不改变；停用规则后可以软删除配置。
4. 规则不能绑定已删除或已停用配置；停用配置后其规则不能继续申请新凭证。
5. 凭证申请只返回短时 URL 和 object key，浏览器直接 PUT 到 COS；日志中没有 URL 签名、Secret
   或 Authorization。
6. Admin 平台以外的 `/api/v1/access` 响应不包含 Admin 的 COS 菜单或权限。
7. API/Worker 重启不执行任何 DDL、seed、回填或菜单插入。

## 12. 后续拆分顺序

得到本 spec 书面确认后，实施 plan 按以下顺序拆分：

1. 人工 SQL migration、真实 PostgreSQL schema 测试和回滚/幂等验证；
2. `storage/cosconfig` 表模块、密钥加解密和管理 API；
3. `storage/uploadrule` 表模块、规则校验和管理 API；
4. `storage/cos` 腾讯云客户端与预签名凭证接口；
5. Admin 对象存储页面、两个 Tab、权限菜单和中英文文案；
6. 全量后端/前端验证、日志脱敏扫描和人工验收。

任何一步发现需要文件表、STS、分片上传、业务对象引用或多平台菜单，都必须暂停并另开
spec，不把复杂度偷偷并入本模块。
