# Redis 与缓存协议审计

## 当前已确认的协议族

- Auth Session / authority generation
- Access version、menu state、access snapshot
- 配置 generation：`system.setting/global`、`system.dictionary/global`、`message.mail/global`、`message.sms/global`、`storage.cosconfig/<configId>`
- 验证码、邮件/SMS 限流
- Asynq 队列
- 实时 ticket、Pub/Sub、通知摘要/在线连接状态
- COS route/snapshot

## 风险清单

- 不能用 Redis key 数量单独判断设计错误；必须记录每个 key 的 TTL、最大基数、写入来源、事实来源和故障行为。
- 已有 generation/state/snapshot/lease/outbox 协议较多，需逐模块证明其容量收益；禁止再增加通用 CacheManager 或万能 Adapter。
- Access 本地缓存必须先确认 Redis version；Redis 故障不得返回旧权限或无界回源。
- 配置快照旧代际通过不可达 key + TTL 回收，不应每次写入 pattern delete。
- 需要补充运行时采样：`SCAN` 分批统计固定前缀、TTL 分布、无 TTL 比例、最大 namespace 基数；禁止 `KEYS`、`FLUSHDB`、`FLUSHALL`。

## 待执行探针

1. 在维护者允许的本地 Redis DB 上按固定前缀 `SCAN`。
2. 对每个前缀统计 key 数、TTL=-1 数、TTL 分位数和 payload 大小抽样。
3. 将结果与架构容量预算对照，决定保留、合并或删除协议。

