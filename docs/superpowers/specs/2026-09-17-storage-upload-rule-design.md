# COS 配置版本化、上传规则与对象读取设计

> 状态：设计已批准，待按实施计划落地。本文记录 2026-09-17 与维护者确认的最终产品语义，取代本文早期
> “`codes TEXT[]` + 当前启用规则推断历史桶”的草稿结论。数据库事实仍以执行时的真实 PostgreSQL 为准。

## 1. 目标与边界

本次改造同时解决四类问题：

1. COS 是全局逻辑配置，可以维护多条；同一逻辑配置可以修改 Bucket、Region、Endpoint 或 Bucket Domain，且新协议上线后
   上传的对象在后续修改物理位置时仍能定位到原位置。
2. 上传规则按认证平台隔离；一个平台可以有多条规则，但最多一条启用。启用新规则自动停用同平台旧规则。
3. 上传编码属于规则，可编辑、删除，并可跨平台、跨规则任意重复；同一规则内的活动编码不得重复。
4. 公开对象持有最终 URL 即可直接访问；私有对象只向同认证平台的已登录用户签发短期 GET URL。

项目尚未上线，本次采用破坏性切换：迁移前的 `<code>/YYYY/...` 对象 key、已保存头像和品牌默认头像均不兼容、不搬迁；
`object-url` 只接受本文的新 key。迁移清空 `user_profile.avatar` 与 `app.brand.default_avatar` 的旧引用，但不调用 COS API 批量删除
Bucket 中的对象字节。不新增逐对象登记表，不代理对象字节流，不新增 RBAC 菜单或权限码，不修改
`permission_auth_platform.menu_version`。`POST /api/v1/storage/object-url` 只要求登录；上传仍要求
`storage:object:upload`。

## 2. 只保留两类内部版本

| 名称 | PostgreSQL 位置 | 用途 | 是否进入管理端 DTO | 是否进入 object key |
| --- | --- | --- | --- | --- |
| 物理 `version` | `storage_cos_config.current_version`、`storage_cos_config_version.version` | 标识对象上传时实际使用的 Bucket 等物理位置 | 否 | 是 |
| 缓存 `generation` | `system_config_cache_generation` 的 `storage.cosconfig/<configId>` | Redis state、不可变快照、mutation lease 与 outbox 发布恢复 | 否 | 否 |

COS 配置和上传规则管理 CRUD 都采用 last-write-wins，不新增 `revision`，也不接受 `expectedRevision` 或
`expectedGeneration`。管理员后提交的有效请求覆盖先提交结果。`updated_at` 只记录最后修改时间，不参与 CAS。

每次真实 COS 配置写入，包括名称、备注、Secret、状态和物理位置修改，都由服务端内部推进 `generation`；只有物理位置字段
变化时才新增 `storage_cos_config_version` 并推进 `current_version`。规范化后没有实际变化的 no-op 不推进 generation，也不写
outbox。generation 冲突只表示另一个服务端 mutation 已先推进缓存代际，Service 必须在有界写入预算内重新读取权威 generation、
重新取得对应 lease 并重试，不能把它作为“表单过期”409 返回给管理端。物理 version 不是缓存版本或编辑锁。

## 3. PostgreSQL 模型

### 3.1 `storage_cos_config`：逻辑账号

该表删去会随物理位置变化的 `bucket`、`region`、`endpoint`、`bucket_domain`，最终保留：

- `id`
- `name`
- `app_id`
- `secret_id_ciphertext`
- `secret_key_ciphertext`
- `current_version`
- `is_enabled`
- `remark`
- `created_at`、`updated_at`、`deleted_at`

`app_id` 创建后不可修改。切换腾讯云账号必须新建逻辑配置。Secret 可以轮换，当前 Secret 用于同一账号下所有历史
Bucket 的私有 GET 签名，避免每个物理版本复制敏感凭据。

### 3.2 `storage_cos_config_version`：不可变物理位置

字段固定为：

- `cos_config_id BIGINT`
- `version BIGINT`
- `bucket VARCHAR(128)`
- `region VARCHAR(64)`
- `endpoint VARCHAR(255) NULL`
- `bucket_domain VARCHAR(255) NULL`
- `created_at TIMESTAMPTZ`
- `updated_at TIMESTAMPTZ`

主键为 `(cos_config_id, version)`，`version >= 1`。版本行创建后不 UPDATE、不软删。修改任一物理字段时，在同一事务
插入 `current_version + 1`，再推进逻辑配置指针。`storage_cos_config(id, current_version)` 使用
`DEFERRABLE INITIALLY DEFERRED` 复合外键指向版本表，使创建逻辑配置与 version 1 可以在一个事务完成。

### 3.3 `storage_upload_rule` 与编码子表

不采用 `storage_upload_rule.codes TEXT[]`。保留 `storage_upload_rule_code`，原因是它能直接表达规则内唯一、按输入差集
软删/新增、稳定排序和未来编码元数据，且不需要数组包含查询或 GIN 索引。

`storage_upload_rule` 不增加 revision。`platform_id`、`cos_config_id`、`access_mode` 创建后不可修改；可修改字段只有
`codes`、`name`、`max_file_size_bytes`、`allowed_extensions`、`allowed_mime_types`、`remark`。

`storage_upload_rule_code`：

- 删除冗余 `platform_id` 及对应外键；平台从 `rule_id` 唯一确定。
- 增加非空 `updated_at`。
- 删除 `(platform_id, code)` 活动唯一索引。
- 新增 `UNIQUE (rule_id, code) WHERE deleted_at IS NULL`。
- 保留 `(rule_id, id) WHERE deleted_at IS NULL`，按插入顺序读取。
- 编码规范化后同一规则内去重并保持首次输入顺序；跨规则、跨平台不限制重复。
- code 规范固定为：去除首尾空白并转小写后总长度 `1..64` 个 ASCII 字节；允许 `/` 分隔多个非空段；每段必须匹配
  `[a-z0-9][a-z0-9._-]*`。不允许空段、反斜杠、空白、控制字符、Unicode、`..`，也不得占用内部段
  `.admin-storage`。因此允许 `article/cover`，不允许以 `.` 开头。
- PostgreSQL CHECK、Go 校验和前端 tag 校验执行同一契约；数据库与后端是权威，前端校验只提供即时反馈。三层各有契约测试，
  不通过复制一个运行时“万能校验器”跨语言共享。

`storage_upload_rule` 新增 `UNIQUE (platform_id) WHERE is_enabled = 1 AND deleted_at IS NULL`。创建时直接启用与状态启用
都先锁 `permission_auth_platform` 平台事实行，再停用同平台其他活动规则，最后创建或启用目标；部分唯一索引负责最终
并发兜底。允许平台暂时没有启用规则。

### 3.4 不保留旧对象路由

不创建 `storage_object_legacy_route`，不按 code 最长前缀猜测配置，也不实现旧 key 的 Redis route cache。不同规则即使在同一平台
复用相同 code 并指向不同配置或不同访问模式，也不构成迁移歧义，因为新 key 直接携带 rule/config/physical version 坐标。
迁移前旧 key 在切换后统一返回无效请求；已知数据库对象引用在 migration 中置空，Bucket 内遗留对象不参与应用读取。

## 4. 对象 key 与读取协议

### 4.1 新对象 key

新上传固定生成；`.admin-storage` 只在 `uploadRule` 模块定义一个 Go 常量，生成器、解析器和后端 code 校验共同引用：

```text
<code>/.admin-storage/v2/p<platformId>/r<ruleId>/c<configId>/v<version>/YYYY/MM/DD/<uuid>.<ext>
```

`code` 只保留人类可读前缀，不参与物理位置推断。路由信息来自受校验的 `p/r/c/v`。修改或删除编码、停用或软删规则、
切换配置当前 Bucket，都不会使旧 v2 key 失效。

### 4.2 路由校验

v2 读取解析并严格校验 key 语法，然后读取不可变 rule route：

- route 的 `platform_id` 必须等于当前登录身份平台；
- route 的 `cos_config_id` 必须等于 key 中 `c<configId>`；
- 配置快照必须存在 key 中 `v<version>`；
- 规则停用或软删不影响历史读取；配置停用不影响历史读取；
- 配置若曾被任何规则引用就不得删除，只能停用。

不符合 v2 结构的 key，包括迁移前 `<code>/YYYY/...` 格式，统一返回明确的无效请求；不回退到当前启用规则、code 前缀或任意
全局配置。

### 4.3 公开与私有

- `public`：使用对应物理 version 的 `bucket_domain` 生成普通 URL，响应 `expiresAt: null`；上传继续设置
  `x-cos-acl: public-read`。缺少域名时显式失败。
- `private`：使用逻辑配置的当前 Secret 与历史 version 的 Bucket/Region/Endpoint 生成约 10 分钟 Presigned GET，
  响应携带 UTC `expiresAt`。
- `POST /api/v1/storage/object-url` 请求只含 `{ objectKey }`，响应固定为 `{ url, expiresAt }`。
- 该接口只要求认证，不要求上传权限。私有 URL 在有效期内是 bearer URL，持有者可使用至过期；应用通过短 TTL，并只在首次
  展示、实际再次使用时已过期或加载失败时按需刷新，以降低暴露窗口和后台请求量。

## 5. 缓存与多实例协议

COS 配置接入已存在的统一协议：

- namespace：`storage.cosconfig`（必须同时满足 PostgreSQL `ck_system_config_cache_generation_namespace` 与 Go
  `namespacePattern`：全小写点分且禁止大写，因此不能写成 `storage.cosConfig`）
- scope key：COS 配置 ID 的十进制字符串
- Redis state：`ready` / 带 token 的 `invalidating`
- snapshot：generation 不可变 key，TTL 5 分钟，严格 JSON schema
- mutation：先取 PostgreSQL generation，再取得 Redis lease；事务内同时修改业务事实、推进 generation、写 outbox；提交后
  CAS 发布 ready。发布失败不把已提交业务写入伪装成失败，由 durable outbox relay 恢复。
- read：正常命中零 PostgreSQL；missing/corrupt 通过 `shared/cacheFill` 单 leader 回源，并在发布前再次确认 generation；
  Redis error、活跃 invalidating、预算耗尽时失败闭合。

快照包含逻辑配置 ID、generation、AppID、加密后的当前 Secret、启停/删除状态和全部物理 version。敏感明文不进入
Redis。解密只发生在需要 COS 签名的 Service 内。

对象读取还需要 rule ID 路由缓存。其 platform/config/access 字段创建后不再变化，因此不创建第二套 generation；使用带 schema
的 Redis 快照、TTL 和 `cacheFill` 有界回源即可。Redis 不可用时不直接无界访问 PostgreSQL。

热路径预算：正常 object-url 请求最多读取一个 rule route cache、一个 generation state、一个 COS snapshot；零 PostgreSQL。
单个冷 key 只允许一个跨实例 leader 回源，单次 API 读预算 500ms，超时返回依赖错误。

## 6. 管理端契约

COS 配置列表与详情仍显示当前 Bucket、Region、Endpoint、Bucket Domain；物理字段来自 current version join，但管理端不展示
`currentVersion` 或 generation。更新、状态和删除请求不携带任何 expected/version/generation 字段。

编辑 COS 配置时 `appId` 只读，Bucket 等物理字段可编辑。Secret ID 与 Secret Key 必须同时留空或同时填写；填写表示轮换。

上传规则 DTO 不含 revision。编辑态禁用平台、COS 配置和访问模式；更新请求不再包含这三个字段。启用一条规则前显示确认，
明确同平台已有启用规则将自动停用。编码继续使用可创建的 tag 输入。

现有 `system/cacheGeneration` 页面会自动显示 `storage.cosconfig/<id>`，无需新增页面、菜单、权限或 i18n 路由。

`UpMedia` 与侧栏头像不再传 `ruleCode` 给 object-url。公开 URL 可长期复用；私有图片在首次绑定、URL 已过期后再次使用或图片
加载失败时按需重新解析，不运行周期刷新定时器。图片成功加载后，签名过期不会移除浏览器已经解码的内容。对象 key 变化和
请求竞态必须忽略旧响应；下载动作在点击时确保 URL 未过期。

GET 签名有效期使用独立常量，初始值 10 分钟，不与 PUT 常量隐式共用。该值是安全窗口与签名请求量之间的可调参数；签名接口
不访问 COS、只返回小型 URL。后端代理则会承担全部对象带宽、Range、缓存、长连接和背压，当前不采用。

## 7. 删除、切换与失败语义

- COS 配置从未被任何规则引用时才允许软删；判断包含已软删规则。已引用配置只能停用，以保留历史读取和 Secret 轮换能力。
- 上传规则仍要求先停用再软删；软删不删除新协议对象依赖的历史物理版本或 rule route。
- PostgreSQL 提交成功、Redis 同步发布失败：接口返回业务成功，记录安全日志，outbox relay 最终发布。
- PostgreSQL 事务回滚：使用原 token 恢复旧 ready；提交结果不确定时读取权威 generation，绝不恢复可能过期的 ready。
- 同 scope 并发写入：后到达请求在有界预算内等待并重试内部 generation/lease；最终按数据库锁顺序 last-write-wins。预算耗尽时返回
  依赖繁忙错误，不返回管理员表单过期冲突，也不绕过 lease 直接写 PostgreSQL。
- 进程在提交后崩溃：outbox 保留；Worker relay 恢复 Redis state。
- Redis 丢失：首次读由有界 repair 从 PostgreSQL 重建；Redis 整体不可用时失败闭合。
- 快照损坏：不返回损坏内容，单 leader 重建；重建前后 generation 不一致则丢弃旧结果并重试。

## 8. 迁移与部署边界

唯一入口为 `docs/database/2026-09-17-storage-object-versioning.ps1 -OldAPIStopped`，串联：

1. 真实 PostgreSQL 只读预检与备份提示；
2. 幂等 forward SQL；
3. 固定旧 storage Redis patterns，以及因清空品牌默认头像而失效的 `system.setting/global` patterns 定向清理，禁止
   `FLUSHDB`/`FLUSHALL`；
4. 数据、约束、索引、generation/outbox 和 Redis key-count 验证；
5. 重复执行不新增版本、不推进 generation、不复制 outbox。

SQL 已提交但 Redis 清理失败时 runner 非零退出并明确报告，可安全重跑。迁移会删除旧逻辑表中的物理字段并删除编码子表的
`platform_id`，旧 API/Worker 与新 schema 不兼容；回滚必须停止新进程、恢复迁移前 PostgreSQL 备份并定向清理新缓存，不能只
切回旧二进制。

迁移不改 RBAC，不递增 `menu_version`。成功后从真实 PostgreSQL 刷新 `docs/database/current.sql`，并把执行时间、备份、
PostgreSQL 验证、Redis 清理和剩余风险写入 `docs/agent/STATUS.md`。

## 9. 验收标准

1. 修改 Bucket 后，新上传使用新 version；修改前的公开和私有对象仍按旧 version 读取。
2. 私有对象只有同平台已登录用户能取得短期 GET URL；跨平台请求失败；公开 URL 无签名且可直接访问。
3. 一个逻辑配置可连续创建多个物理版本；AppID 不可修改；Secret 轮换不复制版本行。
4. 同平台最多一条启用规则；创建启用规则和并发启用都自动停用旧规则，终态唯一。
5. 编码可跨规则、跨平台重复；同规则重复被规范化去重；编码可增删改且新协议历史对象不受影响。
6. COS 配置和上传规则 DTO 均不暴露 revision/generation/expected 字段；并发有效写入按最后一次提交生效，内部代际竞争有界重试。
7. 正常 object-url 热读不访问 PostgreSQL；跨实例冷回源有界；旧请求不能向新 generation 写入旧快照。
8. Redis 发布失败、进程崩溃、Redis 丢失和损坏快照均按本文语义恢复或失败闭合。
9. migration 重复执行幂等；固定 Redis pattern 清理可重试；不修改任何菜单事实或 `menu_version`。
10. migration 后用户头像和品牌默认头像的旧引用均为空；旧格式 object key 被明确拒绝，系统不存在 legacy route 表或运行时兼容分支。
