# Agent / Tool 后端交付报告

日期：2026-10-28。范围：`server/internal/module/ai/agent`、`server/internal/module/ai/tool`，未提交 Git。

## 已实现

- Agent：提示词、倍率、媒体能力、生成参数、多 Provider Model 绑定、具名工具绑定及每工具 maxCalls 约束；普通 update 不接受 status / activeBindingId / activeProviderModelId / actorId。
- 创建支持 `activeProviderModelId` 选择本次新建绑定，不要求客户端预知数据库 binding ID；已有 Agent 独立 switchBinding 动作，Agent 行锁保护当前项，普通编辑保留事务取得的当前项，启用时移除/停用当前绑定会整事务回滚。
- current binding 的 Provider / Provider Model / Official Model 启停与归属在数据库核验；完整操作与价格能力通过注入 `BindingValidator.ValidateBinding(ctx, providerModelID, textProtocol, capabilities)` 再校验。nil validator 不假可用。
- Tool：HTTP 和具名内置 `currentTime`、`calculator`；更新新建不可变版本，旧版本继续可读。HTTP 固定 HTTPS:443 公共目标、无重定向/代理、DNS解析后再校验并绑定公共地址；输入不能改主机/鉴权头。
- HTTP requestMapping / responseMapping、secretHeaders 独立密钥域 AES-GCM 加密，密文与明文秘密均不在公开响应中。更新未传 secretHeaders 保留旧秘密，传空对象清除。`SetCipher` 显式注入密钥。
- JSON Schema 为明确受支持且有界的子集（拒绝未知关键字及远程引用），输入输出运行时校验；专用四则解析器不执行代码，限制表达式长度/深度/运算次数；超时 100..30000ms、输出至多64KiB。
- 独立 list/detail/create/update/status/delete / switchBinding 权限路由及 Agent 候选 create/update 路由；无通用工具执行 HTTP API。options 纯静态零I/O，使用后端双语 option.Option，数值 enum 保持数值类型。
- 所有配置写入通过注入 `RuntimeCoordinator.Mutate`，同一 PostgreSQL 事务推进 `ai.runtime/global` generation 与 outbox；nil coordinator 明确失败。回调使用续租 context。Run/Redis RuntimeReader 与真实 lease 协调由根任务交付，不在本目录重复实现。

## 接口装配

- `agent.NewService(agent.NewRepository(db), bindingValidator, runtimeCoordinator)`。
- `tool.NewService(tool.NewRepository(db), runtimeCoordinator)`，随后 `SetCipher(toolCipher)`。
- `tool.NewCipher(root, keyID)` 使用独立 HKDF purpose；其 `Headers(ctx, frozenVersion)` 可作为 SecretReader。
- `tool.NewExecutor(nil, toolCipher).Execute(ctx, frozenToolVersion, arguments)`；nil HTTPDoer 使用受限公共 HTTPS transport，测试可注入 HTTPDoer。
- 两模块 `RegisterAdminRoutes(group, handler, authenticate, requirePermission)`；本切片不改 cmd 装配或菜单。

## 实际验证

- RED：protocol / repository / executor / cipher 类型尚不存在时测试编译失败；options数值enum、未知schema、过深JSON、保留工具名前缀等行为分别观察到期望失败，再实施最小修复。
- `go test -short ./internal/module/ai/agent ./internal/module/ai/tool -count=1`：最新通过，Agent 8.616s / Tool 9.517s。
- `go vet ./internal/module/ai/agent ./internal/module/ai/tool`：通过。
- `go test -p 1 ./internal/module/ai/agent ./internal/module/ai/tool -count=1 -timeout=3m`：较早一版真实 PostgreSQL testschema 验证通过，Agent 11.570s / Tool 8.642s（current switch、无价拒绝、不可变 ToolVersion、generation/outbox）。
- 后续新增的 PostgreSQL 原子 create / 普通update保留当前项 / 移除当前项回滚与generation不推进等测试已编译通过；按根任务要求暂停PG，待根串行统一执行同一命令。未使用 SQLite 或内存仓库。

## 待根任务统一验证 / 风险

- 根提供并装配同一 RuntimeCoordinator 与完整价格能力 BindingValidator；不存在 nil runtime / Redis失败后的写入兜底。
- 最新全部 PostgreSQL 集成测试仍需根串行运行；未执行正式库迁移、真实付费调用、外部HTTP工具或Git操作。
- 管理 API 已定义，未改入口、菜单、共享钱包、SQL结构。前端页面按根后续分工继续实现，另在本报告追加结果。


## 前端交付（同日）

- 新增 `web/src/api/ai/agent.ts`、`tool.ts`：明确 DTO、列表/详情/候选、创建/编辑、独立 status 与 switchBinding 请求；Update DTO 不含 status/current binding/秘密回显。
- 新增 `web/src/views/ai/agent/index.vue`、`tool/index.vue`：AppSearch/AppTable/AppDialog、强类型 `el-select-v2`、分页/空态/失败、Access action 独立交叉校验；Agent 多绑定/工具约束/当前切换；Tool HTTP/内置、schema、秘密留空保持/清除、不可变版本。
- 新增 `web/src/i18n/messages/aiAgent.ts`、`aiTool.ts`，导出 flat UTF-8 `zhCN`/`enUS`，根 locale 需显式 spread。
- 前端验证：`pnpm exec vue-tsc -b --pretty false` 通过；`pnpm vitest run tests/api/ai/agentTool.test.ts tests/views/ai/agentTool.test.ts --pool=threads --maxWorkers=1 --reporter=dot` 通过（2 files / 5 tests）。Element Plus 全量表单挂载在 jsdom 下产生递归更新噪声，定向页面测试保留权限/未知值契约并避免把框架噪声误报为业务成功。

## Usage 结算加固（同日）

- `usage/facts.go`：独立冻结事实结构，严格校验 charge/run 归属、终态、快照 digest、price version、倍率、provider request、price rate、selector/usage key/unit、RawUsageDigest、明细总额，并支持 RequiredItemKeys/UsageDigests。
- `usage/repository.go`：WithCharge 锁序改为 Charge → Wallet → Run；读取冻结 requests/rates；MarkSettled 同事务写 `billingUpdated`（可注入 `run/event.Repository`）。
- `usage/service.go`：SettlementTx 强制事实读取，不再可选跳过校验；未知用量/错价/漏项不扣款。
- Usage 验证：`go test -short ./internal/module/ai/usage -count=1`、`go vet ./internal/module/ai/usage` 通过；真实 PostgreSQL 集成依根通知暂停，待串行运行。
