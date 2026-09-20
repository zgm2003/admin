# 全项目整改优先级与推进顺序

## P0：先修数据正确性和安全边界

1. Scheduler 删除/启停竞态（已修，待全仓回归）。
2. 通知任务平台受众隔离验证。
3. Publisher 持续依赖故障的健康语义。
4. 数据库重复 FK、空 device_id、access version 自增默认值 migration。
5. Mail/SMS/Notification/Scheduler 状态转换和未知值拒绝。

## P1：协议和职责收敛

1. 各域状态 enum 数值化，API/TS/SQL 同步。
2. 字典 options 与固定 enum label 所有权分离。
3. icon catalog 单一来源与契约测试。
4. notificationTask 权限 capability 从 Repository 下沉 Service。
5. request 层取消未经 parser 的泛型 DTO 逃逸。

## P2：减少复杂度和重复代码

1. Mail/SMS 聚合页稳定逻辑抽取。
2. AppSearch 类型映射收紧。
3. layout/聚合页 watcher 请求去重、取消和身份 key 防抖。
4. 清理重复索引、无效字段和无读写引用的迁移候选。

## 每个工作单元的验收

- 先有失败测试，再有最小实现；
- 运行受影响 Go/Vitest、类型检查和架构检查；
- 数据库变更真实执行前做备份、结构指纹、幂等验证；
- Redis 只做固定前缀 SCAN 探针，不使用 KEYS/FLUSH；
- 更新 `STATUS.md`，明确实际命令、未运行项和剩余风险。
