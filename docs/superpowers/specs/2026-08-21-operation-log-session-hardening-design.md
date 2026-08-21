# 操作日志与会话撤销加固设计

## 1. 目标

修复会话管理与操作日志首期实现中已经确认的安全和契约缺口，不扩展新的业务能力。

本设计继续遵守线性数据流：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

Redis 只负责认证状态协调、会话快照和异步队列；PostgreSQL 仍是会话撤销与操作日志的持久事实。

## 2. 会话撤销安全顺序

管理端撤销会话固定使用以下顺序：

```text
Service 查询目标会话
-> 为实际可能修改的 user + platform 集合读取当前 sessions generation
-> 一次获取 invalidating lease 并启动续租
-> Repository 在 PostgreSQL 事务中锁定并撤销会话
-> 停止续租并检查续租结果
-> 发布所有 lease key 的新 generation
-> 删除已撤销会话的 Redis snapshot
```

约束：

- Redis state 或 lease 获取失败时，不允许开始 PostgreSQL 撤销；
- PostgreSQL 事务失败或 lease 续租失败时，回滚 lease，并返回明确依赖错误；
- PostgreSQL 提交后 generation 发布失败时，不回滚 PostgreSQL。Redis 中的 `invalidating` state 在租约存在期间拒绝认证，租约过期后 state 缺失，认证回源 PostgreSQL；旧 ready state 不得恢复；
- generation 发布成功后，即使 snapshot 删除失败，旧 snapshot 也因 generation 不匹配而不可达；接口仍返回明确依赖错误；
- 不引入补偿队列、分布式事务、通用 Adapter 或事务框架。

Service 可以在写入前调用 Repository 查询目标会话。会话的 `user_id` 和 `platform` 在创建后不可变，因此查询得到的分组可用于后续 lease；Repository 在真正更新时仍必须重新加行锁。

## 3. 单条与批量撤销契约

单条 `DELETE /api/v1/sessions/:id`：

- ID 不存在：返回 HTTP 404 和稳定的 `session.notFound` message key；
- ID 是当前登录会话：返回 HTTP 403 和既有 `session.currentProtected` message key；
- 已撤销：保持幂等成功，返回 `skippedRevoked = 1`；
- active：同步完成安全撤销后返回成功。

批量 `DELETE /api/v1/sessions`：

- 请求 ID 去重后仍限制为 1 至 100 个；
- 不存在的 ID 忽略，适应管理员页面选择后发生的并发变化；
- 当前会话和已撤销会话分别计入现有跳过数量；
- active 会话在一次 PostgreSQL 事务内撤销；
- 不新增 missing 响应字段。

## 4. 操作日志唯一标识

`requestId` 只用于链路关联，允许客户端传入并允许多条操作共享同一值。新增服务端生成的 `eventId`：

- 每次匹配到操作日志规则时生成一个不可预测且非空的唯一值；
- `eventId` 进入 TaskPayload 和 `sys_operation_log.event_id`；
- Asynq Task ID 使用 `eventId`；
- PostgreSQL 唯一索引使用 `event_id`；
- `request_id` 保留普通非唯一索引，便于按链路查询和展示；
- schema version 从 1 升为 2，Asynq task type 同步升为 `system:operation-log:v2`；Worker 严格拒绝旧字段集合，不做兼容兜底。

现有表迁移由 AutoMigrate 前的 `PrepareSchema` 和 AutoMigrate 后的 `EnsureSchema` 显式完成：增加 `event_id`、为已有行生成唯一值、设为 NOT NULL、删除旧 request_id 唯一索引、创建 event_id 唯一索引和 request_id 普通索引。迁移必须可重复执行。

## 5. 有界摘要

- 响应 writer 最多保留生成合法截断摘要所需的有限字节，不缓存完整响应；
- 未超限的合法 JSON 继续递归脱敏；
- 超限时只输出固定 JSON 对象 `{ "truncated": true }`，不保存未经完整解析和脱敏的原始 prefix；
- 最终摘要必须是合法 UTF-8 JSON，并严格不超过 32 KiB；
- 请求摘要同样遵守最终 JSON 上限，超限时不能因为只读取前缀而退化成 `null`；
- 解析失败时不保存原始 body。

## 6. panic 审计

全局中间件顺序调整为：

```text
RequestID -> CORS -> AccessLog -> OperationLog -> Recovery -> Language -> route handlers
```

Recovery 捕获下游 panic 并写出统一 500 envelope 后返回，外层 OperationLog 继续采集 status、失败状态和响应摘要并异步入队。AccessLog 保持最外层观察完整请求。

## 7. 测试与验收

至少覆盖：

- PostgreSQL 撤销发生前 sessions state 已进入 invalidating；
- generation 发布失败后旧 Token 不命中旧 snapshot；
- 重复客户端 `X-Request-ID` 生成不同 `eventId` 和两条任务；
- schema 将唯一约束迁移到 `event_id`；
- 大请求和大响应的摘要合法且不超过 32 KiB，writer 内存有界；
- 操作路由 panic 后产生 status 500 的审计任务；
- 单条不存在返回 404，单条当前会话返回 403，批量保持既有跳过语义；
- 后端全量 fmt、vet、test、build 通过。
