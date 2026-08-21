# 操作日志与会话管理 Implementation Plan

> 加固覆盖说明：本计划中的初始幂等键、摘要截断和会话撤销顺序已由 `2026-08-21-operation-log-session-hardening.md` 完成修正；下文同步为最终实现口径。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox ( - [ ] ) syntax for tracking.

**Goal:** 为 Admin 增加只记录操作类路由的异步操作日志，以及以 PostgreSQL 为安全事实、Redis 负责认证快照失效的会话列表和踢出能力。

**Architecture:** 普通 HTTP 请求继续使用 router -> middleware -> handler -> service -> repository -> model -> PostgreSQL。操作日志在请求中间件完成原响应后采集并通过 Asynq 入队，Worker 使用 task handler -> service -> repository -> model -> PostgreSQL 追加写入；入队失败只记录结构化错误，不改变业务响应。会话管理复用 auth 的 Session、authstate 和 session snapshot 删除路径，撤销顺序固定为 authstate invalidating lease -> PostgreSQL -> fresh generation -> Redis snapshot delete。

**Tech Stack:** Go, Gin, GORM, PostgreSQL, Redis, Asynq, Vue 3, TypeScript, Element Plus, Pinia, vue-i18n, Vitest。

---

## 文件边界

新增 operationlog 模块只放真实职责文件：

- Create: server/internal/module/operationlog/model.go、schema.go、schema_test.go
- Create: server/internal/module/operationlog/request.go、response.go、repository.go、service.go、handler.go、route.go
- Create: server/internal/module/operationlog/rules.go、middleware.go、middleware_test.go
- Create: server/internal/module/operationlog/task.go、task_test.go、handler_test.go、integration_test.go

会话管理继续属于 auth 模块：

- Create: server/internal/module/auth/session_admin.go、session_admin_request.go、session_admin_response.go
- Create: server/internal/module/auth/session_admin_handler.go、session_admin_route.go、session_admin_test.go、session_admin_handler_test.go、session_admin_integration_test.go
- Modify: server/internal/module/auth/session_repository.go

配套修改：

- Modify: server/cmd/api/main.go、server/cmd/worker/main.go
- Modify: server/internal/module/menu/builtin.go、protocol.go
- Modify: server/internal/shared/i18n/catalog.go 及中英文 catalog
- Create: web/src/api/session.contract.ts、session.ts、operation-log.contract.ts、operation-log.ts 及对应测试
- Create: web/src/views/system/sessions/index.vue、operation-logs/index.vue 及对应测试
- Modify: web/src/access/route-views.ts、menu-title-keys.ts、web/src/i18n/messages/zh-CN.ts、en-US.ts

每个任务都先写失败测试；不自动提交 Git，提交由仓库 owner 按 AGENTS.md 规范执行。

## Task 1: 建立操作日志模型与数据库约束

**Files:** operationlog/model.go、schema.go、schema_test.go、cmd/api/main.go

- [ ] Step 1: 在 schema_test.go 写真实 PostgreSQL 失败测试。PrepareSchema 和 AutoMigrate OperationLog 后查询 information_schema.columns 和 pg_indexes，断言字段齐全、event_id 唯一索引、request_id 普通索引和 created_at/action 查询索引存在；插入 is_success=2 必须被 CHECK 拒绝。
- [ ] Step 2: 运行 cd server; go test ./internal/module/operationlog -run TestEnsureSchema -count=1，预期因 package/model/schema 不存在而失败。
- [ ] Step 3: 实现 OperationLog。字段为 ID、RequestID、UserID *int64、SessionID *int64、Platform *string、Method、Route、Module、Action、ClientIP、UserAgent、StatusCode、IsSuccess yesno.Value、LatencyMs、RequestData/ResponseData datatypes.JSON、CreatedAt、UpdatedAt；显式 timestamptz，不嵌入 gorm.Model，不添加 DeletedAt。
- [ ] Step 4: 实现 PrepareSchema 和 EnsureSchema，回填 event_id，创建 is_success CHECK、event_id 唯一索引、request_id 普通索引及 created_at、user_created、action_created 查询索引；重复执行成功。API 在 AutoMigrate 前调用 PrepareSchema，并在现有 schema 初始化后调用 EnsureSchema。
- [ ] Step 5: 重跑上述测试，预期 PASS。

## Task 2: 实现操作规则、脱敏和请求结束采集

**Files:** operationlog/rules.go、middleware.go、middleware_test.go、middleware/access_log.go

- [ ] Step 1: 写失败测试 TestRulesMatchOnlyExplicitMutations、TestSanitizeSummaryMasksSecrets、TestMiddlewareKeepsBusinessStatusWhenEnqueueFails。分别验证已登记 PUT/DELETE 命中、GET 列表/详情不命中；password、confirmPassword、accessToken、refreshToken、authorization、cookie、secret、key 变成固定掩码；假队列失败时原 status/envelope 不变且结构化日志包含 requestId、route、action、enqueue error。
- [ ] Step 2: 运行 cd server; go test ./internal/module/operationlog -run "TestRules|TestSanitize|TestMiddlewareKeeps" -count=1，预期失败。
- [ ] Step 3: 定义 RouteRule{Method, Route, Module, Action string, CaptureRequest, CaptureResponse bool}，用固定切片登记所有状态变更路由，使用 method + Gin FullPath 精确匹配，不扫描路由表、不按 HTTP method 猜测。
- [ ] Step 4: 实现 JSON 摘要脱敏。递归处理 object/array，敏感字段不区分大小写替换固定掩码；超过固定最大字节数时只保留 truncated 标记，不保存未经完整解析的前缀；解析失败不得保存原始 body。
- [ ] Step 5: 实现 OperationLogMiddleware(logger, enqueuer)。context.Next 后读取 request ID、认证写入的 user/session/platform、FullPath、status、耗时、脱敏 request/response，组装 TaskPayload；用最多 500ms 派生 context 入队；入队失败只 logger.ErrorContext，不写库、不改业务响应。保留现有 AccessLog，仅复用其 context key 读取函数。
- [ ] Step 6: 重跑中间件测试，预期 PASS。

## Task 3: 接入 Asynq 操作日志任务和 Worker

**Files:** operationlog/task.go、task_test.go、cmd/worker/main.go、cmd/api/main.go

- [ ] Step 1: 写失败测试 TestDecodePayloadRejectsUnknownOrMissingFields、TestTaskHandlerMarksMalformedPayloadSkipRetry、TestEnqueueUsesEventIDAsTaskID。覆盖未知字段、缺字段、错误类型、尾随 JSON、SkipRetry、任务类型 system:operation-log:v2、TaskID=event ID、MaxRetry 和 Timeout。
- [ ] Step 2: 运行 cd server; go test ./internal/module/operationlog -run "TestDecodePayload|TestTaskHandler|TestEnqueue" -count=1，预期失败。
- [ ] Step 3: 定义闭合 TaskPayload：schemaVersion、eventId、requestId、userId、sessionId、platform、method、route、module、action、clientIp、userAgent、statusCode、isSuccess、latencyMs、requestData、responseData、createdAt。DisallowUnknownFields、单文档、必填和范围校验；非法 payload 包装 asynq.SkipRetry。
- [ ] Step 4: 合法任务调用 operationlog service 的 event_id 幂等插入；数据库错误返回让 Asynq 重试，event_id 冲突视为已处理。API 创建 QueueEnqueuer；Worker 创建 PostgreSQL repository/service 并调用 operationlog.Register(mux, service)，不使用内存队列。
- [ ] Step 5: 重跑任务测试，预期 PASS。

## Task 4: 实现操作日志查询 API

**Files:** operationlog/request.go、response.go、repository.go、service.go、handler.go、route.go、handler_test.go

- [ ] Step 1: 写失败测试 TestListRejectsInvalidPaginationAndSuccessCode、TestListReturnsTypedEnvelope、TestRegisterRoutesRequiresOperationLogListPermission。验证 page/pageSize、isSuccess、时间范围；响应只含 code/data/message；仅 GET /api/v1/operation-logs 且权限为 system:operation-log:list，无删除路由。
- [ ] Step 2: 运行 cd server; go test ./internal/module/operationlog -run "TestList|TestRegisterRoutes" -count=1，预期失败。
- [ ] Step 3: 实现严格 ListRequest（page、pageSize、userId、action、route、isSuccess、from、to），默认 1/20，最大 100；repository 只用 GORM/PostgreSQL，created_at DESC,id DESC 分页并返回 total。
- [ ] Step 4: Service 负责校验和错误映射；Handler 传 context.Request.Context()，成功返回 list/total，失败走 response.Fail。只提供查询，不提供删除、恢复、清理或导出。
- [ ] Step 5: 重跑查询测试，预期 PASS。

## Task 5: 扩展 auth 会话管理 repository/service

**Files:** auth/session_repository.go、session_admin.go、session_admin_request.go、session_admin_response.go、session_admin_test.go

- [ ] Step 1: 写真实 PostgreSQL 失败测试 TestListAdminSessionsCalculatesStatusFromPostgres、TestAdminSessionRevokeRejectsCurrentSession、TestBulkAdminSessionRevokeDeduplicatesAndLimits。插入 active、expired、revoked 数据，验证 username/platform 筛选、分页、当前会话保护、去重和 100 上限。
- [ ] Step 2: 运行 cd server; go test ./internal/module/auth -run "TestListAdminSessions|TestAdminSessionRevoke|TestBulkAdminSession" -count=1，预期失败。
- [ ] Step 3: repository JOIN sys_user 分页，状态直接按 revoked_at 和 refresh_expires_at > NOW() 计算；stats 查询 active 总数及 platform 分组；单条先 SELECT FOR UPDATE，再 COALESCE 更新 revoked_at/updated_at；批量在一次事务锁定并更新未撤销项，所有方法接收 context.Context。
- [ ] Step 4: Service 接收 actor user/session，单条不存在返回 404、单条当前 session 返回 403、已撤销只计 skipped；写库前获取并续租 authstate invalidating lease，数据库成功后发布新 generation，再调用现有 session snapshot delete。Redis 失败返回 dependency unavailable，不回滚 PostgreSQL，也不恢复旧 ready generation。
- [ ] Step 5: 重跑 auth 测试，预期 PASS。

## Task 6: 暴露会话 API

**Files:** auth/session_admin_handler.go、session_admin_route.go、session_admin_handler_test.go、cmd/api/main.go

- [ ] Step 1: 写失败测试 TestSessionRoutesUseExactPermissions、TestBulkRevokeRequiresExactIDsBody、TestSessionHandlerUsesRequestContext。断言 GET list/stats 使用 system:session:list，DELETE 单条/批量使用 system:session:revoke；批量拒绝空 body、未知字段、空 ids、超过 100；Handler 使用 request context。
- [ ] Step 2: 运行 cd server; go test ./internal/module/auth -run "TestSessionRoutes|TestBulkRevoke|TestSessionHandler" -count=1，预期失败。
- [ ] Step 3: 注册 GET /api/v1/sessions、GET /api/v1/sessions/stats、DELETE /api/v1/sessions/:id、DELETE /api/v1/sessions。单条要求空 body；批量只允许 {"ids":[...]}，去重后最多 100；响应使用 lower camel case，返回 list/total、activeTotal/platforms 和 revoked/skippedCurrent/skippedRevoked。
- [ ] Step 4: 将 handler 放入 routerDependencies，复用现有 auth 数据库/Redis 客户端和管理 service；操作规则登记四个会话管理操作。
- [ ] Step 5: 重跑 API 测试，预期 PASS。

## Task 7: 初始化菜单、权限和后端 i18n

**Files:** menu/builtin.go、menu/protocol.go、shared/i18n/catalog.go、catalog_test.go、cmd/api/main.go

- [ ] Step 1: 写失败 builtin 测试，断言 /system/sessions、/system/operation-logs 和 system:session:list、system:session:revoke、system:operation-log:list 存在，EnsureBuiltin 重复执行不重复插入。
- [ ] Step 2: 运行 cd server; go test ./internal/module/menu ./internal/shared/i18n -run "Test.*Builtin|Test.*Catalog" -count=1，预期失败。
- [ ] Step 3: 沿现有系统管理树插入两个页面及叶子权限，新增 session/operation-log 错误 key 和中英文 catalog，保持 catalog key 集合一致；不把 token、密码、Redis key 或内部堆栈放进公开文案。
- [ ] Step 4: 重跑菜单和 i18n 测试，预期 PASS。

## Task 8: 实现前端严格 contracts 和 API

**Files:** web/src/api/session.contract.ts、session.ts、operation-log.contract.ts、operation-log.ts 及对应测试

- [ ] Step 1: 写失败 Vitest parser 测试，覆盖合法分页/stats/bulk revoke/日志摘要及 null、缺字段、错误 lower camel case、未知字段、非法状态；测试禁止显式 any。
- [ ] Step 2: 运行 cd web; pnpm vitest run src/api/session.test.ts src/api/operation-log.test.ts，预期失败。
- [ ] Step 3: API 只调用 request<unknown>；parser 从 unknown 严格收窄到 SessionListItem、SessionListQuery、SessionStats、RevokeResult、OperationLogItem、OperationLogListQuery 等 DTO；不接受 msg，不使用 any、as any 或 Record<string, any>。
- [ ] Step 4: 运行 parser 测试并执行 rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src -g "*.ts" -g "*.vue"，预期测试 PASS 且 rg 无输出。

## Task 9: 实现两个前端页面、动态 view 和 i18n

**Files:** web/src/views/system/sessions/index.vue、operation-logs/index.vue 及测试；web/src/access/route-views.ts、menu-title-keys.ts、web/src/i18n/messages/zh-CN.ts、en-US.ts

- [ ] Step 1: 写失败页面测试，覆盖 sessions 初次 list/stats、筛选、分页、当前 session 不显示踢出、确认后刷新 list/stats；operation logs 筛选、分页、详情展开且没有删除按钮；错误、空态、loading 有文案节点。
- [ ] Step 2: 运行 cd web; pnpm vitest run src/views/system/sessions/index.test.ts src/views/system/operation-logs/index.test.ts，预期失败。
- [ ] Step 3: sessions 使用 Element Plus statistic/form/table/pagination/message-box，成功踢出后重新请求 list/stats，不做乐观删除；批量选择最多 100；当前 auth store sessionId 匹配时隐藏/禁用按钮。
- [ ] Step 4: operation logs 展示 method、route、action、操作者、IP、status、latency、createdAt 和已脱敏 JSON 摘要，不提供删除、清理、导出。
- [ ] Step 5: routeViews 增加 system-sessions、system-operation-logs；menuTitleKeys 和中英文 catalog 增加页面、筛选、状态、错误文案。
- [ ] Step 6: 重跑页面测试，预期 PASS。

## Task 10: 端到端 wiring 和验收

**Files:** cmd/api/main_test.go、cmd/worker/main_test.go（必要时创建）、operationlog/integration_test.go、auth/session_admin_integration_test.go

- [ ] Step 1: 写失败 wiring/integration 测试：新路由只注册一次且有权限 middleware；Worker 注册 system:operation-log:v2；真实 PostgreSQL、Redis、Asynq Worker 验证登记操作入库、撤销写入 revoked_at、删除 snapshot、旧 token 被拒绝。
- [ ] Step 2: 运行 cd server; go test ./cmd/api ./cmd/worker ./internal/module/operationlog ./internal/module/auth -run "Test.*Route|Test.*Worker|Test.*Integration" -count=1。缺少真实依赖时明确报告阻塞，不能改用内存替代。
- [ ] Step 3: 运行后端全量：cd server; go fmt ./...; go vet ./...; go test ./...; go build ./...。四条命令均成功才报告后端完成。
- [ ] Step 4: 运行前端全量：cd web; pnpm vitest run; pnpm build。
- [ ] Step 5: 运行 git diff --check 和敏感字段 rg 检查；只允许脱敏字段名、测试断言和固定掩码逻辑命中。检查后停止，不自动 commit/push/fetch/pull。

## Spec 覆盖自检

- 操作规则、脱敏、摘要限制、入队失败不影响业务：Task 2。
- Asynq payload、SkipRetry、重试和 event_id 幂等：Task 3。
- 操作日志表、分页查询和不可删除：Task 1、Task 4。
- 会话状态、分页、统计、单/批量踢出、当前会话保护：Task 5、Task 6。
- PostgreSQL 权威及 authstate/Redis 顺序：Task 5、Task 10。
- 菜单、权限、前后端 i18n：Task 7、Task 9。
- 严格 TypeScript DTO、错误/空态、二次确认：Task 8、Task 9。
- 真实 PostgreSQL、Redis、Asynq 和全量验证：Task 10。

## 计划自检结果

- 无 TODO、TBD 或空泛的“适当处理”步骤。
- Task 2 采集字段、Task 3 payload、Task 1 模型字段保持 lower camel case / PascalCase 映射一致。
- 所有数据访问沿线性链路；Queue 不导入业务模块，Repository 不调用 Queue。
- 不引入 Platform Adapter、Manager、Factory、BaseService、BaseRepository、DI 容器、运行时注册器、进程级缓存或删除接口。
