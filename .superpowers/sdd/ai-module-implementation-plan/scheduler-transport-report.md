# Scheduler / Realtime 运输切片报告

## 实际修改
- `system/scheduler.Repository.RenewRun(ctx, jobID, attempt, token, now)`：仅在运行中、attempt/token 匹配且当前续约未过期时，将租约延长固定 5 分钟。
- `CompleteRun` / `FailRun` 增加 attempt/token/未过期租约条件，旧 token 或过期 token 返回 `ErrClaimLost`，不改变 Run/Job 终态。
- `Executor` 每 60 秒（可测试覆盖）续 Scheduler Job lease；每次续约 I/O 使用 60 秒独立上限；续约失败取消执行上下文，阻断完成/失败写入；根 context 取消不再产生终态副作用。
- 新增显式 `AITaskExecutors` 与 `AITaskDefinitions` 静态 AI 任务目录，拒绝 nil 执行函数，包含 retry 次数、预算和结算指数退避。
- realtime relay 每批 claim 上限 200；使用 Redis Lua 以跨实例 Redis 时钟、有界 16000/s 滑动窗口预算执行原子 PUBLISH，Redis 故障/预算失败闭合并回退 outbox 重试。

## 验证
- `go vet ./internal/module/system/scheduler ./internal/module/realtime`：通过。
- `go test -short -p 1 ./internal/module/system/scheduler ./internal/module/realtime -count=1 -timeout=2m`：通过。
- `go test -p 1 ./internal/module/realtime -run '^TestRelayRedisTwoInstancesShareSlidingPublicationBudget$' -count=1 -timeout=30s -v`：通过（真实 Redis，验证双客户端竞争最后预算槽、实际 Pub/Sub、窗口 TTL；不是容量压测）。
- 实跑单测覆盖取消、心跳失权/续约 I/O 超时、原非 AI 执行/失败重试、AI 静态任务目录、realtime 200 批次/Redis 失败闭合/取消和原用户/平台通道重投语义。
- 新增旧 token/过期 token/错误 attempt 与成功续约的真实 PostgreSQL 测试；依根代理指示留待串行执行：`go test -p 1 ./internal/module/system/scheduler -run '^TestRepositoryRunLeaseRejectsExpiredAndReplacedOwners$' -count=1 -timeout=2m`。
- 未运行真实 PostgreSQL lease 集成测试、Redis 多实例容量压测或真实迁移；未声称达到容量验收。

## 根代理整合
- 在 API/Worker 静态任务目录装配 `AITaskDefinitions(AITaskExecutors{RunExecute, MediaPoll, MediaImport, UsageSettle, RunRecover, RunEventCleanup}, generationTimeout, transferTimeout)`；所有字段须传入具名执行函数，业务 payload 严格字段解析归 AI 执行器。
- Run 每 15 秒续业务 lease，并观察 Executor 传入的同一 context；Run 生成终态后的转存阶段不再续业务执行 lease。
- Relay 复用现有 Redis 客户端 `EvalString`，构造签名不变。Redis Lua 已直接执行 PUBLISH，不能在外层重复发送。
- 本代理只修改 scheduler/realtime 及本报告，没有改 AI/cmd/前端/数据库/STATUS，没有 Git 提交、真实迁移或外部消费。
