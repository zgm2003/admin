# 后端边界与可靠性审计

审计范围：`server/internal/module/**`、`server/cmd/api`、`server/cmd/worker`。本文件记录已由代码阅读或真实测试确认的问题；“建议”不是已完成修复。

## P0/P1

### Scheduler 删除竞态（已修复）

- 证据：`server/internal/module/system/scheduler/repository.go` 原 `DeleteSchedule` 先 COUNT 活动 Job，再独立软删；`CreateDueJobs` 会并发锁定 schedule 并创建 Job。
- 影响：已删除 schedule 可能残留 `scheduled/queued/running` Job。
- 当前修复：删除与启停均在事务内先锁 schedule 行，再读取和更新；手动 Execute 在锁内读取 schedule 并创建 Job；Retry 对有关联计划的 Job 同时锁定失败 Job 和未删除 schedule。
- 验证：`schedule_concurrency_test.go` 使用真实 PostgreSQL 阻塞关系覆盖删除/启停/Execute/Retry；Scheduler 包测试和后端全仓验证通过。

### Scheduler Publisher 故障假存活

- 证据：`server/internal/module/system/scheduler/publisher.go:58-64` 对 `RunOnce` 错误只记录日志并继续循环；Worker 监督逻辑无法获知持续故障。
- 影响：Worker 进程存活但 Job 长期不发布，运维误判健康。
- 当前修复：入队失败即使 `ReschedulePublish` 成功也会返回迭代错误；连续 5 次非取消迭代失败后 Publisher 返回健康错误，Worker 监督逻辑会停止进程；TaskID 冲突仍按已发布处理，单次故障仍使用既有任务退避。
- 验证：`go test ./internal/module/system/scheduler -count=1` 与后端全量 `go test -p 1 ./... -count=1` 通过。

### notificationTask Repository 越界读取权限事实

- 证据：`server/internal/module/message/notificationTask/repository.go:240-305` 直接读取 `permission_auth_platform`、`permission_menu` 并硬编码权限码和父子关系。
- 影响：消息模块依赖权限表结构，权限契约变更时跨模块静默失配。
- 修复方向：由 Service 注入 capability reader；Repository 只访问任务、目标和通知表。

### notificationTask 目标平台隔离风险

- 证据：`notificationTask/repository.go:240-258` 和 `task.go:125-142` 对用户/角色目标执行全局存在性/成员展开，需继续核对平台授权过滤。
- 影响：跨认证平台用户可能被写入错误平台通知收件箱。
- 修复方向：提交时按 `(platform_id,user_id)` 校验有效平台身份和权限，冻结平台范围内受众。

## P2/P3

- Scheduler `SetScheduleEnabled` 原先分离读取 cron/timezone 与更新，已纳入事务修复。
- Scheduler Service 的 `CreateSchedule`、`UpdateSchedule`、`SetStatus`、`Execute`、`Retry` 均拒绝 `actor <= 0`。
- Mail/SMS 不存在平台的错误语义不一致：Mail 可能映射为依赖不可用，SMS 映射为 NotFound。
- Scheduler schedule/job/run 状态已同步迁移为 SMALLINT、Go typed enum 和前端数字协议；触发来源仍是有语义的固定字符串 enum，不按状态列处理。
- Asynq `ErrTaskIDConflict` 路径把 Job 标记为 queued 但 token 与队列载荷可能不一致，通常可自愈但会产生 stale/retry 噪声，应补协议测试。
