# 统一配置缓存代际完成态设计

> 状态：设计已获维护者方向批准，实施按同日总计划分阶段落地。本文只定义配置型缓存协议；COS 对象 key、物理版本、
> 上传规则和私有读取语义见 `2026-09-17-storage-upload-rule-design.md`。

## 1. 目标

项目已有 `system_config_cache_generation`、`system_config_cache_outbox`、Redis `ready/invalidating` state、tokenized mutation
lease、generation 不可变 snapshot、`shared/cacheFill` 和 durable outbox relay。第一阶段只有 `system.setting/global` 使用完整协议；
Dictionary、Mail、SMS 仍各自维护 Redis generation、mutation key、load lock 或 snapshot state，故障语义和回源预算不一致。

本次完成以下收口：

1. COS 配置接入统一协议，并保留对象物理 `version`。
2. Dictionary、Mail runtime、SMS runtime、Mail/SMS 限流策略接入同一 generation/outbox 协议。
3. `system.setting` 写竞争改为服务端内部有界重试，不再把内部 generation 竞争当管理员表单过期。
4. 删除 Mail/SMS 限流策略的管理 CRUD `revision/version`；普通管理配置统一 last-write-wins。
5. 删除上述模块各自的 Redis-only generation/mutation 协议和旧 key；不新增通用 CacheManager、业务注册表或隐式 hook。

## 2. Scope 清单

| Namespace | Scope key | PostgreSQL 事实 | Snapshot variants | 写入频率 |
| --- | --- | --- | --- | --- |
| `system.setting` | `global` | `system_setting` | setting key、brand | 极低 |
| `system.dictionary` | `global` | `system_dictionary`、`system_dictionary_item` | language + sorted codes hash | 低 |
| `message.mail` | `global` | mail config、template、recipient rule、rate-limit policy | scene runtime、scene readiness、platform policy catalog | 低 |
| `message.sms` | `global` | sms config、template、recipient rule、rate-limit policy | runtime、scene readiness、platform policy catalog | 低 |
| `storage.cosconfig` | config ID | COS logical config + physical versions | runtime | 极低 |

namespace 必须满足现有全小写点分校验；scope key 使用 `global` 或 COS 配置 ID 的十进制正整数。Mail/SMS 认证平台数量很小且
配置写入低频，因此每个渠道共用一个 global generation；策略更新使该渠道其它旧 variant 一并不可达，但不会增加热路径查询，
并显著减少 scope、lease 与平台生命周期复杂度。现有只读
`system/cacheGeneration` 页面自然展示全部 scope，不新增页面、权限、编辑按钮或清理控件。

## 3. 明确排除

以下状态不迁入通用 generation 表：

- `permission_auth_platform.menu_version`、`policy_version`；
- `permission_access_version.version`；
- Auth/Session/authority generation；
- 验证码、发送限流计数、队列数据；
- COS 对象物理 `version` 与不可变 rule route；
- 普通分页管理列表缓存（本次只处理共享运行时热读）。

限流策略配置进入 generation；已经消耗的 Redis 限流计数不迁移、不清零、不改 key，策略发布与计数窗口是两件事。

## 4. 管理并发语义

普通配置管理统一 last-write-wins。HTTP/前端 DTO 不返回或提交 generation、revision、version、expected 字段；`updated_at` 不参与
CAS。Mail/SMS 限流策略表的 `revision` 列、Mail catalog `version` 和 SMS policy `revision` 从数据库、Go、JSON、TypeScript 和 UI
中删除。

generation 仍是服务端缓存正确性协议。每次 mutation 尝试固定为：

```text
读取 PostgreSQL authoritative generation
-> 获取相同 generation 的 Redis mutation lease
-> 启动 lease renewal
-> PostgreSQL 事务锁业务事实并重新计算 no-op
-> 有真实变化才在同事务 AdvanceTx(base generation) + outbox
-> 提交后 token owner 发布新 ready generation
-> MarkPublishedIfUnclaimed
```

`ErrUpdating` 或 `ErrGenerationChanged` 表示并发 mutation，不是管理员旧表单。Service 在 500ms 总写入预算内以固定 20ms wait hook
重新开始完整尝试；测试使用 fake clock/wait hook，不真实 sleep。预算耗尽返回依赖繁忙错误。Redis 不可用、lease 丢失或 state 无法
确认时失败闭合，禁止绕过 lease 直接写 PostgreSQL。

no-op 不推进 generation、不写 outbox，并以原 token 恢复 base ready。PostgreSQL 提交成功后 Redis 发布或 mark-published 失败时，
接口返回业务成功并记录不含密钥/载荷的结构化日志，由 outbox relay 恢复。提交结果不确定时重新读取权威 generation；只有确认仍为
base 才 rollback lease，绝不恢复可能过期的 ready。

## 5. 读协议与预算

每个模块保留自己的 snapshot 结构、codec、Repository query 和业务校验；只共享
`cacheGeneration.Repository/Store/Relay` 与 `cacheFill.Lease`。不得新增接受任意业务 loader 的全局 Manager、Adapter、Factory、
运行时注册表或跨模块反射。

读流程固定为：

```text
读取 Redis state
-> ready: 按 generation + variant 读不可变 snapshot
-> miss/corrupt: cacheFill 单 leader 查询 PostgreSQL
-> 发布前再次确认相同 ready generation
-> invalidating: 有界等待，不读 PostgreSQL
-> Redis I/O error: 失败闭合，不用 PostgreSQL 充当每请求兜底
```

正常热读零 PostgreSQL。单个冷 scope/variant 跨实例最多一个 leader 回源。单次 API/发送热路径的配置缓存等待预算为 500ms，
每次等待 20ms；快照 TTL 为 10 分钟，COS runtime 为 5 分钟，旧代际只依靠不可达 key + TTL 回收，不 pattern delete。

严格 snapshot codec 必须拒绝未知字段、重复字段、trailing data、scope/generation/variant 不匹配、缺失固定 catalog、重复业务键、
非法枚举与空密文。Secret 只以 ciphertext 进入 Redis，明文只在实际调用 Provider/COS 前解密。

## 6. 模块完成态

### 6.1 System Setting

保留 `system.setting/global`、现有 snapshot 和 outbox。只补写竞争有界重试、确定性预算测试和共性故障矩阵；管理 DTO 不增加
generation。既有真实 migration 不重做。

### 6.2 Dictionary

使用 `system.dictionary/global`。任何 dictionary 或 item 的真实创建、修改、状态、删除都在同一业务事务推进 generation/outbox。
选项 snapshot variant 由 language 与排序后的 codes 计算；回源开始捕获 generation，发布前再次确认相同 ready state，彻底消除旧请求
向新代际写回旧 options 的竞态。删除 Redis-only `system:dictionary:generation:v1`、`mutation:v1` 和旧 options key 协议。

### 6.3 Mail

使用 `message.mail/global`。config、template、recipient rule 和全部平台 rate-limit policy 共用一个业务事实代际。scene runtime、
scene readiness 和 platform policy catalog 是同一 generation 下的 variants；readiness 和 policy snapshot 不再维护独立 tokenized
state。一次管理写只取得一条 lease、执行一个 PostgreSQL 事务、推进一次 generation/outbox。发送路径仍按 scene/platform 读取，
正常命中零 PostgreSQL。

### 6.4 SMS

使用 `message.sms/global`。config、template、recipient rule 和全部平台 rate-limit policy 共用一个代际；runtime、per-scene
readiness 和 platform policy catalog 作为不同 variants，全部由权威 PostgreSQL generation 驱动。删除 Redis-only
generation/mutation key，保留业务 snapshot codec 和 recipient-rule/模板校验。

### 6.5 Mail/SMS Rate Limit Policy

Mail/SMS 分别复用 `message.mail/global` 与 `message.sms/global`。每个平台 catalog 含固定两条策略；snapshot 不携带管理
revision/version，只携带渠道 generation、platform ID 与策略事实。更新某一策略时锁同平台固定 catalog，no-op 不推进代际。

认证平台创建/软删事务会改变两组策略事实，因此必须分别推进 Mail/SMS 已存在的 global generation，使旧 snapshot variants 不可达。
跨 scope 操作按 namespace 字典序取得 lease，并按同顺序推进，避免死锁。若任一 lease、事务或发布前置步骤失败，平台业务事务不得
部分提交。

## 7. 迁移与部署

分两个独立 migration runner：

1. COS runner：保留现有 `2026-09-17-storage-object-versioning.*`，只负责 COS schema、COS scopes、旧对象引用清空和对应 Redis cleanup。
2. 配置协议完成 runner：新增 `2026-09-17-config-cache-generation-completion.*`，负责 Dictionary/Mail/SMS 三个固定 global scopes、
   删除 Mail/SMS rate-limit revision 列，以及旧 Dictionary/Mail/SMS 配置缓存 key 的定向 cleanup。

两个 runner 都必须要求 `-OldAPIStopped`，自动备份，使用幂等 forward SQL，SQL 成功后执行固定 pattern Redis cleanup 两次并做只读
验证。不得接受任意 pattern，不得使用 `KEYS`、`FLUSHDB` 或 `FLUSHALL`。SQL 已提交而 Redis cleanup 失败时非零退出并允许
`-CleanupOnly` 重试。回滚必须停止新进程、恢复对应 PostgreSQL 备份、运行固定 rollback cleanup，再部署旧二进制。

旧 API/Worker 与新 schema/key 不兼容；项目尚未上线，不做双读、双写或 legacy 兼容。每个 runner 成功后刷新真实
`docs/database/current.sql` 并更新 `STATUS.md`，但必须在对应阶段全部验证完成后才进入下一 runner。

## 8. 验收

1. 全部配置热读零 PostgreSQL；同一冷 key/variant 跨实例只有一个 leader 回源。
2. 旧请求不能向新 generation 写入旧 snapshot；Redis 丢失可从 PostgreSQL 有界修复；Redis 整体故障失败闭合。
3. PostgreSQL 已提交但同步 Redis 发布失败时业务返回成功，pending outbox 可由 Worker relay 恢复。
4. 同 scope 并发写最终 last-write-wins，客户端不见内部 generation；写入预算有硬上限且没有真实 sleep 的 flaky 测试。
5. Dictionary、Mail、SMS 不再存在 Redis-only generation/mutation 协议；Mail readiness 不再是独立写协调器。
6. Mail/SMS rate-limit revision/version 从数据库和前后端协议消失，限流计数及剩余窗口不被清理或重置。
7. 新建/删除认证平台时 Mail/SMS global generation 与两组策略事实处于同一事务结果，不出现单边失效。
8. COS 仍能按物理 version 读取新协议历史对象；管理 DTO 不暴露物理 version 或缓存 generation。
9. migration 可重复执行且 generation/outbox 不重复推进；Redis cleanup 只命中固定旧配置缓存 key。
10. 权限、Access、Session、验证码、限流计数、Queue 的版本或运行语义完全不变。
