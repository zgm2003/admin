# 通用定时任务管理设计

## 1. 目标与边界

为 Admin 项目提供统一的 PostgreSQL 权威调度协议，支持长期 Cron 计划、一次性业务 Job、执行尝试审计、多实例恢复和管理端观察。

本次不保留旧调度协议、双读双写或兼容分支。完成迁移后删除通知专用 dispatch outbox、通知 dispatch relay，以及 realtime/notification 的临时 retention trigger。

PostgreSQL 是 schedule、job、run 的唯一事实来源；Redis/Asynq 只负责运输和 Worker 唤醒。

## 2. 已确认的产品规则

- Cron 只支持五段表达式，最小周期为每分钟一次。
- 每个计划保存 IANA 时区，默认 `Asia/Shanghai`；不接受任意 UTC 偏移字符串。
- 修改 Cron 或时区时，从修改时刻重新计算 `next_run_at`，不补跑旧时间点。
- 停用计划停止生成新 Job；重新启用时从当前时刻重新计算，不追赶停用期间的时间点。
- 同一计划最多一个活动 Job；执行超过 Cron 周期时跳过重叠时间点，不积压补跑。
- 手动立即执行创建独立 Job，不修改原计划时间轴；已有活动 Job 时返回明确冲突。
- 失败重试创建新的 Job，原失败 Job 保持不变，并记录 `retry_of_job_id`。
- 任务核心行为由代码静态 Registry 决定；管理员只能修改代码明确开放的计划字段和参数。
- 不允许管理员输入任意 Handler、Asynq 类型或未经 Registry 校验的 JSON 参数。
- 内置计划可以调整 Cron、时区和启停，但不能删除。
- 执行页面展示活动和终态记录，但不提供通用强制终止运行中 Job 的按钮。

## 3. 数据模型

### 3.1 `system_scheduler_schedule`

长期存在的 Cron 计划，包含名称、任务类型、Cron、IANA 时区、经过 Registry 校验的参数 JSONB、启用状态、`next_run_at`、内置计划标识、创建/更新审计字段和 `deleted_at`。

不增加管理员乐观锁 revision，普通管理操作采用 last-write-wins。内置计划通过唯一 `builtin_key` 和服务层规则保护。

### 3.2 `system_scheduler_job`

一次需要完成的任务实例，包含可空 `schedule_id`、静态 `task_type`、校验后的 payload 快照、`trigger_source`、业务幂等 `source_key`、计划/可用时间、业务状态、尝试次数、退避时间、发布/执行租约、错误分类、`retry_of_job_id`、操作人和时间字段。

状态为：

```text
scheduled -> queued -> running -> completed
                             -> scheduled (等待重试)
                             -> failed
scheduled/queued -> canceled
```

发布租约和执行租约是恢复字段，不是额外业务状态。活动 Job 为 `scheduled/queued/running`，终态为 `completed/failed/canceled`。

约束：同一 Schedule 只能有一个活动 Job；业务 `source_key` 在对应任务类型内唯一；所有 due、租约恢复和历史清理查询使用部分索引。

### 3.3 `system_scheduler_run`

Job 的一次实际执行尝试，使用 `(job_id, attempt_no)` 唯一约束，记录 Worker 实例、状态、起止时间、耗时、脱敏错误分类/摘要和有界结果摘要。

Worker 崩溃导致租约过期时，Run 记录为 `failed/worker-lost`，Job 按 Registry 重试策略重新排队或进入最终失败。

## 4. 调度与故障协议

### 4.1 Scanner

所有 Worker 可运行 Scanner。Scanner 用 `FOR UPDATE SKIP LOCKED` 分批锁定到期 Schedule，在同一事务中创建 Job 并推进 `next_run_at`。`(schedule_id, scheduled_at)` 唯一约束是多实例最终兜底。

停机期间漏掉多个 Cron 点只生成一个恢复 Job，然后直接推进到下一个未来时间点。

### 4.2 Publisher

Publisher 从 PostgreSQL claim `scheduled` 且 `available_at <= now()` 的 Job，使用 token lease 投递 Asynq。Asynq Task ID 固定由 Job ID 和 attempt 组成。

- 投递成功后条件更新为 `queued`。
- Task ID 冲突视为已投递并安全确认。
- Redis/Asynq 失败时 Job 保留在 PostgreSQL，按有界退避重试。
- 入队成功但数据库更新失败时，租约恢复后再次投递，固定 Task ID 防止重复运输。
- Redis 丢失后，未确认的 Job 从 PostgreSQL 重新发布。

### 4.3 Worker

Asynq payload 仅包含 Job ID、attempt 和 dispatch token。Worker 先在 PostgreSQL 条件 claim Job，在事务中创建 Run 并进入 `running`，提交后才调用 Registry Handler。Handler 完成后以 token 条件更新 Run 和 Job。

旧消息、迟到消息或 token 不匹配时只确认并忽略，不得覆盖新代际状态。取消后的迟到消息为空操作。

业务 Handler 必须幂等。通知批次依赖批次键、recipient 和 realtime event 唯一约束；清理任务按条件重复执行。

## 5. 静态任务 Registry

Registry 由 API/Worker 显式装配，不使用反射、`init()`、数据库 Handler 字符串或运行时隐式注册。每个定义固定任务类型、文案、可创建性、内置属性、参数校验、队列、超时、最大尝试次数、退避策略和 Handler。

首批定义：

1. `message.notificationTask.dispatch`：通知批次一次性 Job，仅由通知业务创建。
2. `realtime.retention.cleanup`：清理 realtime 事件历史的内置 Cron。
3. `message.notification.retention.cleanup`：清理通知中心历史的内置 Cron。
4. `system.scheduler.history.cleanup`：清理调度器终态 Job/Run 历史的内置 Cron。

后面三个清理计划由 migration 幂等创建；默认 Cron 为 realtime 每分钟、notification 每分钟、scheduler history 每日 `03:30`，默认时区 `Asia/Shanghai`。通知 dispatch 不创建人工 Cron 计划。

## 6. 通知迁移

通知提交事务冻结业务事实并创建 batch 0 Scheduler Job：立即发送使用当前时间，定时发送使用 `scheduled_at`。每批最多 500 个用户；当前批次完成且仍有用户时，在同一事务创建下一批 Job。业务幂等键为 `notificationTask:<taskId>:<batchNo>`。

通知取消事务同时取消尚未开始的关联 Job；已完成批次不撤回。迁移完成后删除 `message_notification_dispatch_outbox` 及其模型、Repository、relay、Worker wiring 和旧测试，不保留旧表读取或双写。

## 7. 保留与清理

`system.scheduler.history_retention_days` 默认 30，允许 7–3650。只清理终态 Job/Run；活动 Job 不按保留期删除。计划停用或删除不立即删除历史。清理采用小批量、可重入方式，PostgreSQL 故障明确失败，下一次继续。

Realtime 和 notification 清理任务只清理各自业务历史：前者不删除通知中心数据，后者不删除调度计划；调度历史清理不删除业务通知或 realtime 事件。

## 8. 管理页面与权限

页面分为：

- **定时计划**：名称、任务类型、Cron、时区、下次执行、状态、最近结果；支持编辑、启停、立即执行、查看执行记录、删除（内置计划后端拒绝删除）。
- **执行记录**：展示所有 Job，默认可按活动/终态筛选；详情中以时间线展示 Run 尝试。失败 Job 可在权限允许时重新执行，创建新 Job，不修改原记录。

权限码保持页面/动作独立：

```text
system:scheduler:view
system:scheduler:list
system:scheduler:detail
system:scheduler:create
system:scheduler:update
system:scheduler:status
system:scheduler:delete
system:scheduler:execute
system:scheduler:retry
```

当前首批任务全部是内置或业务内部任务；当 Registry 没有可人工创建的任务类型时，前端隐藏“新建”按钮。

## 9. 验证边界

必须覆盖：多实例 Scanner 单次 claim、活动 Job 互斥、漏跑合并、Cron/时区修改、停用/重启、手动执行冲突、参数快照、Redis 丢失、提交后投递失败、Worker 崩溃租约恢复、迟到消息、自动重试、人工重试、通知批次唯一性、三类清理任务幂等、历史保留边界和内置计划不可删除。

迁移验证必须确认新三表、内置计划、通知旧 outbox 删除、旧触发器/relay 不再装配，以及 forward migration 幂等。未获维护者批准前不写实现代码、不创建 migration、不执行 Redis 清理。
