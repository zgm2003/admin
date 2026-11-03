# AI 模块菜单与数据表盘点

> 盘点日期：2026-10-28
>
> 本文是 AI 项目启动前的事实清单，不是迁移脚本，也不代表所有旧项目对象都应原样恢复。
> 当前项目代码和 PostgreSQL 结构优先；旧项目只用于恢复已经验证过的业务边界。
> 本期完整目标方案见 [Admin AI 模块最终设计稿](ai-module-design.md)：固定 6 个页面、21 张新 AI 表及支付流水扩展。
> 本文保留事实盘点与前期讨论，不能把旧表清单或候选措辞当作最终建表/权限清单。

## 1. 资料范围

本次只读核对：

- 当前项目 `D:\admin`：`AGENTS.md`、`docs/agent/design.md`、`docs/agent/architecture.md`、
  `server/`、`web/`、`canvas/` 和 `docs/database/current.sql`；
- 旧 Go 项目 `D:\github-project\admin_back_go`：AI 模块、Admin 合同、seed 和 schema；
- 旧前端项目 `D:\github-project\admin_front_ts`：AI 页面、API 和生成路由；
- `D:\github-project\sub2api`：当前 HEAD `f178e5ae32659e8901644a9f2378a0439ffa36ae`，重点检查余额准入、在途预留和实际扣费。

本轮数据库结论来自已提交的 `current.sql` 快照与 Model/Repository 交叉核对，**没有连接真实 PostgreSQL**。
旧 `schema.sql` 是旧项目的 MySQL 结构参考，不是当前 PostgreSQL 可直接执行的迁移。下文严格区分
“旧项目事实”“当前代码/结构快照事实”和“待确认的目标候选”。

盘点结果：旧 Admin 有 **1 个 AI 目录、7 个页面**；旧结构有 **33 张 `ai_*` 表、3 张钱包表**。
表的数量不是新项目的建表数量：钱包复用现有支付域，旧复数表名也不原样复制。

### 1.1 维护者已确认的本期边界

- **只做 Admin 系统内部 AI；Canvas 本期不改动、不接入。** 下文 Canvas 内容是现状和边界参考，不是本期施工项。
- 余额消费参考 sub2api 的准入、在途和据实结算机制，不另要求固定最大透支金额；已准入请求可结算成负数，
  余额不满足准入条件后停止新请求，不承诺负数绝对值永远很小。金额表示继续遵循当前整数 units 契约。
- **Admin 消费入口已确认纳入文字、图片、视频、音频四类能力，统一 Run 与计费事实。** 这不等于恢复旧 Admin 图片试玩或
  旧 Canvas transport；本期入口、任务状态和媒体结果都由当前 Admin 后端重新定义。
- **生成的图片、视频、音频统一转存现有 COS**：复用当前存储域保存文件记录，AI 记录关联 Run、媒体元数据和对象坐标，
  不把上游临时链接作为长期结果，也不把媒体二进制写入 PostgreSQL。本期仅建设 Admin 的媒体保存与查看链路。
- **首期 Provider 以 OpenAI 兼容协议为主，默认使用 Responses API**：Agent 的对话与工具编排优先走 Responses，
  Chat Completions 保留为可显式配置的兼容协议；Images、Audio 按对应能力接口调用，视频按独立异步适配设计。
  支持自定义 Base URL；具体上游能力与视频任务协议仍需核对，不因此认定任意兼容站都支持 Responses、工具或四类媒体。
- **维护者补充确认首期具名供应商为 OpenAI**；兼容协议不等于可以使用 OpenAI 官方报价，报价来源按供应商身份独立确认。
  本期报价来源边界已在设计 v1.2 第 7.2 节收口：OpenAI `/v1/models` 只返回目录，美元价从固定官方 Pricing Markdown/模型计价表同步，
  保留计价档位/单位/原文性质与来源摘要；抓取或解析失败明确报错，不把目录同步当报价同步。
- **计费按实际用量结算**：模型价格 × token/媒体用量 × Agent 倍率，Run 结束结算；沿用 sub2api 的准入、在途预留、事务幂等
  和已准入请求允许负余额的机制。价格、用量和倍率都要在 Run/Charge 中保存快照，不能因管理员后续改价重算历史账单。
- **官方模型菜单保留，定位为标准模型身份与能力目录**：供应商模型映射该目录，报价与人民币售价归供应商模型拥有；
  官方模型页只读聚合展示关联价格，Agent 绑定实际供应商模型并配置倍率，应用只选择 Agent。
- **定价以 2026-10-28 评审修订为准**：供应商美元参考价只读同步保存；新计费项的人民币售价先复制同单位美元数值，管理员后续
  可以手动修改人民币售价，再次同步不覆盖已有人民币价。不使用真实汇率或强制长期 1:1；LLM 按分项 token、图片按质量/尺寸
  与实际生成张数计费，最终费用为内部人民币单价 × 实际用量 ÷ 计价数量 × Agent 倍率。生效价格版本与 Run 快照不可回写。
- **AI 费用扣当前登录操作者自己的 `payment_wallet`**：Run、Usage Charge 和资金流水关联实际操作者；管理员查看全局 Run
  或执行管理动作不改变扣费用户，不能将他人的消费记到当前操作员钱包。
- **工作台只能选择 Agent**：Provider 先配置，Agent 再绑定多个供应商及其模型、系统提示词、Tool 和计费倍率；应用不能绕过
  Agent 直接选择 Provider/Model。Run 必须保存 Agent、模型、价格和倍率快照。
- **系统提示词直接在 Agent 内维护**：随 Agent 配置保存，Run 冻结本次实际使用的提示词快照；修改 Agent 不回写历史 Run。
  本期不新增独立 Prompt 库、菜单、管理 API 或提示词库表，不迁移旧 `ai_prompts`。
- **一个 Agent 允许配置多个供应商及其模型绑定**：应用仍只选择 Agent，不把 Agent 限制为单个 Provider/Model。
  **同一 Agent 每次只使用当前选定的一个绑定，模型调用失败即 Run 失败，不自动切换。** 管理员在 Agent 配置中手动切换当前使用项；
  这不是修改供应商的全局启停状态。新 Run 冻结当前绑定及其协议/模型快照，后续切换仅影响新 Run，不改动已创建 Run。
  2026-10-28 再次确认：排队、执行和恢复中的已有 Run 均继续原供应商；同一会话的下一次新 Run 才读取新绑定。
- **Tool 支持真实 HTTP 和内置工具调用**：Agent 可以绑定并执行，调用参数、结果、超时、错误和审计事实必须持久化；
  本期不开放任意代码、Shell 或命令执行，避免把 Admin AI 直接变成通用远程执行入口。
- **本期不做 Context/RAG**：不做文档上传、文档切片、向量索引、Embedding/Rerank、检索增强或会话记忆工程；
  不新增 Context 菜单、向量数据库依赖或 `ai_context_*` 运行时表。普通会话消息仍可持久化，但不属于 RAG。

## 2. 当前项目状态

当前 `D:\admin` 尚未接入 AI 后端：

- `server/internal/module` 没有 `ai` 模块；
- `web/src/views`、`web/src/api` 没有 Admin AI 页面和 API；
- `cmd/api`、`cmd/worker` 没有 AI 依赖装配；
- Canvas 已有图片、视频、文本、音频、Prompt、模型选择和本地 Provider 配置，但主要是浏览器本地配置/参考项目 API，尚未成为当前 Go 后端的统一 AI 运行时；
- 当前钱包 `payment_wallet` 的 `balance_units` 为有符号 `bigint`，Model 使用 `int64`，无非负 CHECK；
- 支付流水 `payment_transaction` 使用正数金额，来源被约束为订单/兑换码二选一，尚无消费方向或 AI 来源，不能直接承接 AI 消费；
- 钱包主键是 `user_id`，同一账户跨认证平台共享余额；流水中的 `platform_id` 记录来源，不代表每个平台单独一个钱包；
- 继续使用现有人民币 8 位小数精度：**1 元 = 100,000,000 units，1 分 = 1,000,000 units**。units 不是分，HTTP 金额继续使用精确十进制字符串。

当前可复用的事实基础：

| 能力 | 当前对象 |
| --- | --- |
| 钱包余额 | `payment_wallet` |
| 资金流水 | `payment_transaction` |
| 文件/COS | `storage_file`、`storage_cos_config`、`storage_cos_config_version`、`storage_upload_rule*` |
| 异步任务 | `system_scheduler_schedule`、`system_scheduler_job`、`system_scheduler_run`、Asynq |
| 实时事件 | realtime 模块 |
| 用户/平台 | `user_account`、权限平台和 Access 快照 |

## 3. 旧项目 Admin 菜单

旧项目的 AI 顶级目录为 `AI助手`，目录 code 为 `ai`。旧菜单事实如下：

| 旧节点 | 路径 | 类型 | 主要权限 |
| --- | --- | --- | --- |
| AI助手 | — | directory | — |
| AI对话 | `/ai/chat` | page | 旧版本未配置独立页面权限 |
| 运行监控 | `/ai/runs` | page | `ai_run_list` |
| AI工具管理 | `/ai/tools` | page | 旧版本未配置独立页面权限 |
| 上下文工程 | `/ai/context` | page | `ai_context_view` 等 |
| 供应商配置 | `/ai/providers` | page | 旧版本未配置独立页面权限 |
| 智能体配置 | `/ai/agents` | page | 旧版本未配置独立页面权限 |
| 官方模型 | `/ai/official-models` | page | `ai_official_model_list` |

旧 action 还包括：

```text
ai_provider_add / edit / test / status / del
ai_agent_add / edit / test / status / del
ai_tool_add / edit / status / del / generate
ai_run_list
ai_official_model_list / price_sync
ai_context_view / manage / document_manage / profile_manage / evaluate
```

上述页面由旧前端真实实现，并由生成路由合同映射至 `src/views/Main/ai/*`，不是占位菜单。
旧 Provider、Agent、Tool、Chat 页面没有独立页面权限，另一些页面复用了读取权限；这套授权形状不能原样迁入当前项目。

**菜单不等于全部 AI API**：旧项目另有登录态会话、消息、生成和 Context 消费接口。新项目必须分别定义
Admin 管理动作和 Canvas 用户侧操作的身份/资源所有权，不能因为用户没有 Admin `:view` 就禁止其正常生成，
也不能因为用户已经登录就开放管理员 Provider 密钥、价格修改或全用户运行记录。

### 3.1 当前项目的目标菜单命名

当前项目要求复合模块使用 lower camel case、资源使用单数、页面和 API 权限分离，因此不直接复制旧的复数路径。建议目标形状为：

| 目标节点 | 建议 componentPath/path | 独立页面权限 | 说明 |
| --- | --- | --- | --- |
| AI 目录 | 不设页面 component；目录 path 按现有菜单契约确定 | 不可直接授权 | 仅聚合子页面，不承载 API 权限 |
| 供应商配置 | `ai/provider` / `/ai/provider` | `ai:provider:view` | Provider、模型同步、连接测试 |
| 官方模型 | `ai/officialModel` / `/ai/officialModel` | `ai:officialModel:view` | 标准模型目录与能力；只读展示关联供应商美元参考价和人民币内部售价 |
| 智能体配置 | `ai/agent` / `/ai/agent` | `ai:agent:view` | Agent 配置和能力绑定 |
| 工具管理 | `ai/tool` / `/ai/tool` | `ai:tool:view` | 工具定义和 Agent 工具绑定 |
| 运行监控 | `ai/run` / `/ai/run` | `ai:run:view` | Run、Attempt、用量和结算详情 |
| AI 工作台 | `ai/chat` / `/ai/chat` | `ai:chat:view` | Admin 文字、图片、视频、音频四个页签；Canvas 不依赖此 page |

读取和写操作使用与 page 分离的 action。下列仅是按旧有能力梳理的候选后缀，
例如 Provider 的 list 必须落成完整、独立的 `ai:provider:list`，不是一条包含斜线的权限码：

```text
Provider：list / detail / create / update / status / delete / test / sync
OfficialModel：list / updatePrice / syncPrice
Agent：list / detail / create / update / status / delete / test
Tool：list / detail / create / update / status / delete / generate
Run：list / detail
```

当前先按推荐草案保留 6 个 Admin 页面，四类生成能力集中在 AI 工作台，不为每种媒体单独增加 page；
实际生成、查询和写操作仍需独立 action 校验，不能从工作台的 `:view` 推导接口权限。
上表仍不是已经写入数据库的菜单；正式迁移前需要固定 page/action 组合和 Admin 平台归属。
Run 重试、取消、人工结算等操作必须先核实真实业务和状态转换，不能为了凑权限提前开放。
page/action 双向独立；每个受保护 API 必须显式绑定自己的 action，不能从 `:view` 或 URL 推导。

### 3.2 菜单与数据表的关系

| 页面/入口 | 主要旧事实表 | 边界 |
| --- | --- | --- |
| 供应商配置 | `ai_providers`、`ai_provider_models` | 管理端维护；服务端持有密钥 |
| 官方模型 | 两张 `ai_official_model_price_override*` 表 | 基础目录来自内嵌 JSON，不是独立官方模型表 |
| 智能体配置 | `ai_agents`、`ai_agent_tools` | 绑定 Provider/Model 和 Tool；工作台只选择 Agent，本期不做 Context/RAG |
| 工具管理 | `ai_tools`、`ai_agent_tools`；执行记录为 `ai_tool_calls` | 支持 HTTP/内置工具；定义、绑定与调用审计分离，不开放任意代码/命令 |
| 运行监控 | `ai_runs`、`ai_provider_attempts`、`ai_run_events`、统计事实、两张用量账单表 | 用量账单是 AI 事实，资金余额归 payment；按实际 token/媒体用量结算，关联实际操作者 |
| 对话/Canvas | 会话、消息、回复命令/分块、文本/图片任务、素材、Prompt | 用户侧 API；不因此新增素材、图片试玩或 Prompt 管理菜单 |

旧项目的 Context Engine 只作为历史表清单保留，本期明确不迁移、不建菜单、不接入向量存储或 RAG Worker。

## 4. 旧项目 AI 数据表

旧项目 `database/schema.sql` 按 `CREATE TABLE` 精确统计有 33 张 `ai_*` 表；加上 3 张钱包表，共 36 张相关表。
下面完整列出，**不代表每张都需要迁移或对应独立菜单**。

| 分组 | AI 表数 | 其他表数 |
| --- | ---: | ---: |
| Provider、模型和定价 | 4 | 0 |
| Agent、Tool 和调用 | 4 | 0 |
| 对话、消息和回复 | 4 | 0 |
| Run、Attempt 和观测 | 5 | 0 |
| 图片、文本、素材和 Prompt | 5 | 0 |
| Context Engine | 9 | 0 |
| 用量计费和钱包 | 2 | 3 |
| 合计 | **33** | **3** |

### 4.1 Provider、模型和定价

| 表 | 用途 |
| --- | --- |
| `ai_providers` | Provider 配置、加密 API Key、Base URL、健康状态 |
| `ai_provider_models` | Provider 下的模型目录、能力类型、官方映射 |
| `ai_official_model_price_overrides` | 官方模型价格覆写版本 |
| `ai_official_model_price_override_rates` | 价格覆写的分项费率 |

旧官方模型基础目录由 `officialmodel/catalog/official_models_v1.json` 内嵌提供，
**没有独立的 `ai_official_models` 表**。目标项目应把模型身份、能力和价格快照分开；不能因存在菜单就虚构旧表，
Provider 真实密钥不得进入浏览器。

本期标准目录统一模型身份与能力，不提供官方美元报价的人工覆写。供应商模型拥有报价同步与内部人民币售价，目标表改为
`ai_provider_model_price_version`、`ai_provider_model_price_rate`；上表两个旧 price_override 表名仅是旧项目事实，不恢复为本期结构。
LLM 按输入/缓存输入/输出 token 分项，图片按质量/尺寸及实际生成张数计费；音视频保留其真实计量维度，不统一套用 token 单位。
新计费项的人民币初价复制同计价单位美元数值，之后管理员可以单独改人民币售价，后续同步保留已有人民币价。
同步形成预览，确认人民币售价和生效时间后发布；Run 保存内部人民币售价、计量单位与 Agent 倍率快照，历史账单不回算。

### 4.2 Agent、Tool 和工具调用

| 表 | 用途 |
| --- | --- |
| `ai_agents` | 旧 Agent 名称、Provider/Model 绑定、场景、系统提示词、头像、计费倍率、Context Profile；本期移除 Context Profile 关联 |
| `ai_tools` | 工具定义、参数 schema、状态 |
| `ai_agent_tools` | Agent 与 Tool 的多对多绑定 |
| `ai_tool_calls` | 一次运行中的工具调用、参数、结果和状态 |

本期一个 Agent 可配置多个供应商及其模型绑定，当前使用项只有一个；工作台只选择 Agent。
管理员可手动切换 Agent 当前使用的绑定，新 Run 选用并冻结该绑定；运行失败不自动轮询、故障切换或负载均衡到其他供应商。
系统提示词由 Agent 配置直接拥有，不引用独立提示词库。创建 Run 时保存实际使用的提示词内容快照，
不能仅依赖 Agent 当前值还原历史；Agent 后续修改不改变已创建 Run 的执行快照。旧 `ai_prompts` 仅保留在旧表盘点中，
本期不恢复其 Model、Repository、管理 API 或菜单，也不新建对应提示词库表。

**本期默认协议与工具执行约定（目标设计草案，不代表旧项目实现）：**

- 默认使用 Responses API 编排 Agent 对话和工具；Provider/Model 需要明确声明并通过连接测试核对协议与工具能力。
  不支持 Responses 的上游可在对应模型绑定中显式选择 Chat Completions；各次实际请求记录 Provider、模型与协议快照。
  Run 固定使用创建时选定的绑定，工具循环的各轮请求继续使用同一供应商、模型和协议；模型调用失败即 Run 失败。
  管理员手动切换只影响后续新 Run，不隐式降级协议、丢弃工具、切换供应商或自动重放整个 Run。
- 工具调用循环为：模型返回 `function_call` → 服务端校验并执行 Agent 已绑定的工具 → 以 `function_call_output`
  和对应 `call_id` 将结果交回模型 → 继续生成，
  直至得到最终结果、失败、取消或达到配置限制；模型输出工具请求本身不等于工具已成功执行。
- 一次用户操作对应一个 Run；同一 Run 可包含多轮模型请求和多次工具调用。分别保存请求轮次、上游请求标识、
  工具调用标识与参数/结果/错误，实际表名按当前单数命名规范另行设计，不直接恢复上面的旧复数表。
- 对各轮已经发生的上游用量去重记账并汇总，在 Run 结束时统一结算；失败或取消不能漏掉前面已产生的费用，
  不能只取最后一次模型响应的用量，也不能把本地工具调用次数自动当作模型 token 或额外收费项。
- 工具执行需要明确轮数、调用数、超时和输出大小限制，以及有副作用调用的幂等与失败恢复；数值在正式设计中冻结。
  工具调用身份与执行重试 Attempt 分开；有副作用调用在执行结果不明时不得盲目重试，需要下游幂等或结果查询保证。
  默认 Responses 不改变 Images/Audio/视频的能力接口边界，也不自动开放上游全部托管工具或被排除的 Context/RAG 能力。

协议依据：[OpenAI 官方 Function calling 指南](https://developers.openai.com/api/docs/guides/function-calling)，
2026-10-28 实际读取（HTTP 200）；官方多步工具流程和 `call_id`/`function_call_output` 示例已核对。

### 4.3 对话、消息和实时回复

| 表 | 用途 |
| --- | --- |
| `ai_conversations` | 用户会话和 Agent 归属 |
| `ai_messages` | 用户、助手、工具消息事实 |
| `ai_reply_commands` | 一次回复命令和终态控制 |
| `ai_reply_delivery_chunks` | 流式回复分块和断点恢复 |

对话、消息、Reply Command 是持久事实；页面刷新不能依赖进程内流缓冲。

### 4.4 Run、Provider Attempt 和观测

| 表 | 用途 |
| --- | --- |
| `ai_runs` | 一次 AI 运行的用户、Agent、Provider/Model 快照、状态、用量和结算状态 |
| `ai_provider_attempts` | 一次 Run 的 Provider 尝试、请求/响应摘要、幂等键、错误和用量 |
| `ai_run_events` | Run 生命周期事件 |
| `ai_run_dashboard_facts` | Run 终态统计事实 |
| `ai_run_dashboard_daily_facts` | 日聚合统计事实 |

Run 和 Attempt 必须保存 Provider/Model 快照，不能因为管理员后来修改配置而改变历史账单。
Agent 可配置的多个供应商绑定与某个 Run 实际使用的绑定分别表达；请求事实记录实际使用的供应商、模型、协议、
调用轮次、错误与用量。一个 Run 冻结一个当前使用绑定，模型调用失败后结束 Run，不自动换用其他绑定；
管理员后续手动切换 Agent 不改变已有 Run，已发生用量始终按已有规则保留并结算。

### 4.5 图片、文本、素材和 Prompt

| 表 | 用途 |
| --- | --- |
| `ai_image_tasks` | 图片生成异步任务、Lease、状态、参数和错误 |
| `ai_image_files` | 图片任务的输入/输出文件及 COS 坐标 |
| `ai_text_tasks` | Canvas 文本生成源任务 |
| `ai_assets` | 当前用户的 AI 素材元数据 |
| `ai_prompts` | 旧 Prompt 目录；中立业务 core 保留，Admin Prompt transport 已退休 |

大文件不进入 PostgreSQL；数据库只保存对象坐标、媒体元信息和结果关联。

**本期媒体保存约定（已确认统一转存 COS；以下为目标设计草案）：**

- 图片、视频、音频生成结果由服务端转存现有 COS，并关联现有存储域文件记录与所属 Run；数据库保存媒体类型、
  文件元信息和对象坐标，Admin 通过现有存储访问机制展示或下载，不以永久保留上游临时 URL 代替转存。
- 上游生成状态与转存状态分开记录；转存失败进入可重试状态，不展示为文件已保存，也不为了重试转存而重新生成媒体。
  转存重试不得重复扣费，已产生的上游用量仍按 Run 的实际消费规则结算。
- 复用 Admin 平台对应的 COS 配置和存储权限边界；文件访问校验所属用户与授权，不暴露存储凭据。
  具体上传规则、文件引用字段及转存重试/恢复协议在正式设计中核对，本期不扩展 Canvas。

### 4.6 旧 Context Engine（本期排除）

| 表 | 用途 |
| --- | --- |
| `ai_context_profiles` | embedding/rerank/memory 模型和索引策略 |
| `ai_context_spaces` | 上下文空间 |
| `ai_context_bindings` | Agent 与 Context Space 的绑定 |
| `ai_context_documents` | 文档事实及来源 |
| `ai_context_document_versions` | 文档版本、处理 Lease、状态和摘要 |
| `ai_context_chunks` | 文档切片和向量索引坐标 |
| `ai_context_plans` | 某次 Run 的上下文编排快照 |
| `ai_context_plan_items` | Plan 中被选中/排除的来源项 |
| `ai_conversation_memories` | 对话记忆的派生事实 |

以上 9 张 Context 表是旧项目事实，本期全部排除：不建表、不迁移、不接入 Qdrant/Embedding/Rerank，
也不把 `ai_conversation_memories` 误当普通聊天历史。普通聊天只保留 `ai_conversations`/`ai_messages` 等对话事实。

### 4.7 AI 计费

| 表 | 用途 |
| --- | --- |
| `ai_usage_charges` | 一次 Run 的报价、实际费用、计费版本和结算状态 |
| `ai_usage_charge_items` | 按 token、图片、音频等类别拆分的实际用量 |
| `user_wallets` | 旧用户级钱包余额、累计充值/消费和冻结金额；新项目复用 `payment_wallet`，不另建第二套钱包 |
| `wallet_holds` | 旧项目的运行预算/冻结事实 |
| `wallet_transactions` | 旧项目钱包流水 |

旧钱包有 `balance_units >= 0`、`held_units <= balance_units` 约束，旧流水也强制前后余额非负，
**不能原样照搬成允许透支的钱包**。仅参考其金额方向、来源幂等和结算事实；并发在途登记与数据库冻结扣款是不同机制，
不能把“预估金额必须全额覆盖”的旧 Hold 规则悄悄变成当前 AI 请求准入规则。

### 4.8 旧项目中不应自动恢复的对象

旧架构已标记退休或不再作为 active runtime 的对象：

- 旧 AI model/tool/prompt/knowledge-base REST 资源；
- Admin AI 图片试玩和旧资产管理交互面；
- 旧知识图谱/工具映射路由；
- 音频和视频生成 active runtime；
- 旧菜单命名和旧 app/ai adapter。

这里退休的是对应的旧 REST/管理入口，不代表 `ai_tools` 等底层业务表不存在。
旧架构测试明确要求 Prompt 的 Model/Repository/Service 继续存在，但不允许恢复 Admin Prompt 路由。
本期已确定系统提示词直接保存在 Agent 配置中；Canvas Prompt 库服务端化不在本期范围，不能因为旧表存在就直接建表。

### 4.9 新项目命名与复用原则

- 上述表名是旧事实；新代码使用单数资源、复合模块 lower camel case，新 PostgreSQL 表使用对应的 snake_case，
  例如候选 `ai/provider` → `ai_provider`、`ai/officialModel/priceOverride` → `ai_official_model_price_override`。
- 钱包和统一资金流水继续归 `payment`，AI 只拥有 Run、用量、价格快照及结算关联，不复制 `user_wallets`。
- 系统提示词已确认在 Agent 内维护，不建独立 Prompt 库；统计聚合方式仍需设计，音频/视频运行时已确认纳入、具体协议需核实；
  Context/RAG/向量存储明确排除。
- 确定模块归属后，再统一 Model、API、Views、菜单和 i18n；本轮没有生成 DDL 或声称最终表结构已获批准。

## 5. sub2api 余额行为核对

sub2api 在标准余额计费模式下允许已准入请求按实际费用结算成负数，余额耗尽后拒绝新的请求；
**源码没有保证透支绝对值一定很小**。订阅模式和简易模式另有分支，不应混入这个结论。

### 5.1 准入阶段

`BillingCacheService.CheckBillingEligibility` 在余额模式下要求余额大于 0，且不低于已配置的正数准入阈值：

- 余额小于等于 0 时拒绝；
- 默认 `minimum_balance_reserve` 为 `0.000001` 美元；这是开始请求的门槛，不是最大透支金额；
- 可配置更高的保留金额；
- 余额低于阈值时不再发起新请求。

应区分两条路径，不能把所有费用都描述成全额预扣：

| 路径 | 准入与在途行为 |
| --- | --- |
| 变动用量费用 `ReserveInflight` | Redis 无在途请求时，首笔允许估算费用超过余额；有在途请求时才要求 `balance - inflight_sum >= estimate` |
| 已知固定费用 `ReserveFixedCharge` | 额外先要求 `balance >= amount`，并叠加在途、订阅/API Key 额度检查 |
| 图片批处理余额冻结 | 数据库原子移入 `frozen_balance`，要求可用余额覆盖 Hold；不是普通文本请求准入规则 |

关键 Lua 条件是 `count > 0 and (balance - sum) < amount` 才拒绝。它减少多个并发请求共享同一余额造成的透支，
但特意保留单独一笔请求的“余额尚有剩余就能开始”体验。默认启用在途预留，TTL 为 900 秒，活跃请求支持续期；
预留延续到计费任务完成，而不是上游刚结束就提前释放。

在途预留还有明确的故障边界：Redis 执行失败、预留阶段余额读取失败或缓存不支持时会 fail-open；
模型无法估价默认也放行且不预留（可配置未定价时拒绝）。不要把这些行为误写成严格的跨实例透支上限。

### 5.2 实际结算阶段

`deductUsageBillingBalance` 的行为是：

1. 先尝试 `balance >= cost` 的原子扣款；
2. 如果余额不足，再执行不带余额条件的原子扣款；
3. 返回新的负余额，并标记 `BalanceOverdrafted=true`；
4. 若扣后余额不再满足准入条件，尝试同步失效余额缓存；失效失败会记录告警，不撤销已经提交的账务；
5. 扣后仍符合条件且启用在途预留时，先同步扣减余额缓存，再释放预留，避免出现“预留已释放但缓存余额尚未扣减”的窗口；
6. 后续请求读到低于阈值或非正余额时被拒绝；已经准入的请求仍需完成实际费用结算，不能抹掉账单。

因此小负数可以由“最后一笔实际费用超过剩余余额”解释，不是长期授信额度；但源码不能证明线上总是小负数。
并发估算偏低、单次大请求、预留失效或故障放行时，多笔已准入请求也可能继续结算。
尤其 `max_output_tokens`、`max_input_tokens` 和 `max_reservation_usd` 是**估算/预留上限**，不是上游实际费用的硬上限。

源码测试也直接覆盖了较大差额：余额 `0.01`、估算 `5` 的独立请求仍可获得首笔预留；
扣费单测模拟扣后余额 `-5` 并认定结算成功。这些用例是行为证据，不是本轮实测的线上消费。

### 5.3 对当前项目的结论

产品规则应拆成两条，而不是简单写成“允许负余额”：

1. **已经准入的 AI 请求按实际用量结算，允许把余额扣到负数；**
2. **余额不满足准入条件后，不再接受新的 AI 请求；并发在途需要统一计入。**

建议目标流程：

```text
读取余额/阈值
  -> 区分变动用量与固定费用，明确首笔放行/并发预留规则
  -> 允许本次请求完成
  -> 按实际用量原子扣费
  -> 允许结果为负，完整记录实际费用，不截账归零
  -> 失效余额缓存
  -> 后续请求按阈值拒绝
```

这不等于复制 sub2api 的浮点美元实现。当前项目继续使用现有人民币整数 units 和精确金额字符串；
供应商美元报价只读保存，内部人民币售价首次复制同数值后可由管理员调整，不引入真实汇率或美元钱包。

维护者已接受参考 sub2api 的机制，不另要求人为固定最大透支金额，本盘点不重新引入
“预估费用必须全部由余额覆盖”的硬预扣方案。正式设计仍需核对单次输出/媒体数量、工具或 Agent 多步调用、
并发数、估算误差、重试和缓存失效失败，并明确整数金额换算、估算参数与依赖故障行为；
“通常只负一点”不是硬金额保证，提高余额门槛或截小预留值也不能证明该保证。

### 5.4 sub2api 相关事实表

sub2api 不是同一套业务模型，但下列对象对应了我们需要关注的边界：

| sub2api 对象 | 作用 |
| --- | --- |
| `users.balance` | 用户余额，数据库注释明确允许为负数 |
| `users.frozen_balance` | 图片批处理等场景的冻结余额 |
| `usage_logs` | 请求和实际用量事实 |
| `usage_billing_dedup` | 请求级扣费幂等键 |
| `usage_billing_dedup_archive` | 幂等键归档，避免清理后重复扣费 |
| `api_keys` | API Key 用量、额度和限速归属 |
| `groups` / `user_subscriptions` | 分组或订阅模式的额度边界 |
| `batch_image_jobs` / `batch_image_items` / `batch_image_events` | 异步图片任务和结算恢复 |

其中 `usage_billing_dedup` 以 `(request_id, api_key_id)` 唯一，并核对 `request_fingerprint`；
同一事务先 claim、检查当前/归档去重事实，再更新余额，失败整体回滚。当前项目应保留同等强度的唯一幂等事实。

## 6. 当前项目需要先解决的资金表问题

当前结构中的 `payment_transaction` 不只有金额约束：

```sql
CHECK (amount_units > 0)
```

同时还有：

- `order_id` 与 `redeem_code_id` 必须恰好一个非空（来源 XOR）；
- 余额算术 CHECK 只保护当前充值入账方向的 int64 下界；
- 交易有来源唯一约束、不可更新/删除保护和钱包外键。

这表示当前流水金额是“绝对金额”，不能直接把 AI 消费写成负数金额；扩展时必须同步调整来源 XOR、
方向相关算术/溢出约束、AI 来源唯一约束、Model/DTO/查询和不可变账务规则。候选方案：

### 6.1 统一资金流水扩展（推荐候选）

- 保留 `amount_units > 0`，增加收入/支出方向；`credit/debit` 是语义，封闭枚举按当前契约使用具名数字值，不直接照搬旧字符串枚举；
- 增加明确 `source_type/source_id` 或 AI Charge/Run 关联，并为同一 AI 结算建立唯一幂等键；
- 充值为 `credit`，AI 消费为 `debit`，余额本身仍可为负；
- 保持用户钱包行锁、来源唯一约束、余额快照和 int64 溢出保护；充值订单/兑换码继续走原有来源模型。

### 6.2 独立消费资金事实（对比候选）

- `ai_usage_charges` 记录 AI 实际费用；
- `payment_transaction` 仍只记录充值；
- 钱包余额更新另外依赖 AI 专用资金事实；
- 两套流水容易造成账务查询和对账分裂，除非明确建立统一投影协议。

当前推荐统一资金流水：同一钱包的所有资金变化使用同一套账本，金额保持正数、方向表达收入/消费，AI 来源通过具名字段或独立关联表达，
不伪造充值订单。这个形状借鉴旧 Go AI 的 `wallet_transactions(amount_units, direction, source_type, source_id)`，
但**不复制旧表的余额非负 CHECK**。正式迁移前需要先冻结 direction/source 字典、唯一键、负余额结算边界和历史流水兼容查询；
在此之前不改 `payment_transaction`，也不新增 AI 迁移。
这只是资金记账结构，与用户此前否定的 AI 请求预扣方案不是同一个维度；零费用 Run 仍保存用量事实，不伪造零元资金流水。

## 7. 当前建议的菜单/表落地顺序

虽然目标是一次完成，但实施仍需按依赖顺序落地：

1. 先确认菜单、平台、页面/动作权限和接口契约，再设计 Provider、Official Model、Pricing 和 Agent；
2. Tool、Conversation、Message、Run、Realtime；
3. AI Usage、钱包消费方向流水、结算恢复；
4. Admin 统一承载文字、图片、视频、音频生成、Run/计费和结果查看；Canvas API 不在本期范围；
5. Admin 菜单、权限、动态路由和中英文 i18n 收口；
6. Worker、Scheduler、Redis 缓存、幂等和容量验证；Context/RAG 不进入实施顺序。

这是候选技术依赖，不是已获批准的实施计划，也不是把功能拆成互相独立的产品；最终仍应形成一套统一 AI 运行时。
Worker、缓存、权限和幂等必须随每个切片验证，不能推迟到最后才接入。

容量按百万用户、多实例、高并发基线评审：配置/定价不做逐请求 PostgreSQL 回源；正常热路径从有版本的 Redis 快照读取，
钱包与结算持久写入保留事务事实。缓存更新沿用当前 generation/outbox 和有界重建思路；
故障时不得退回旧本地配置或形成无界数据库流量。
具体 Redis 操作数、数据库读写预算和单用户在途上限尚需正式设计，不能把本轮盘点当作压测结果。
并发验收至少覆盖双实例同余额准入、估算不足、结算重复/乱序、预留续期/过期、数据库提交后失效失败及恢复；
流式断连和多步 Agent 已发生的上游费用不能因取消而丢失。

## 8. 可追溯源码入口

下列行号对应本次盘点时的本地文件；仓库后续修改可能移动行号。

| 结论 | 源码入口 |
| --- | --- |
| 旧 AI 菜单及 actions | `D:/github-project/admin_back_go/database/seed.sql:11,48-49,71-95,131-138`；`internal/admincontract/views.go:29-35` |
| 旧前端真实页面映射 | `D:/github-project/admin_front_ts/src/modules/routing/generated/views.ts:5-11,47-53`；`local-views.ts:71-88` |
| 全部 33 张 AI 表 | `D:/github-project/admin_back_go/database/schema.sql:29-1049` 的 `CREATE TABLE ai_*`；Prompt 起始行 558 |
| 旧钱包及非负约束 | 同一 `schema.sql:1834-1940`；尤其 1848、1940 的 CHECK |
| Prompt 中立 core 保留、Admin transport 退休 | `D:/github-project/admin_back_go/internal/architecture/admin_only_test.go:108-136` |
| 官方模型基础目录非数据库表 | `D:/github-project/admin_back_go/internal/module/ai/officialmodel/catalog.go:451` |
| 当前钱包与资金流水快照 | [current.sql](../database/current.sql#L1199) 第 1199-1239 行；来源唯一索引 3403/3410，不可变触发器 3690/3697 |
| 当前有符号金额及精度 | [wallet/model.go](../../server/internal/module/payment/wallet/model.go#L5)、[transaction/model.go](../../server/internal/module/payment/transaction/model.go#L5)、[money/units.go](../../server/internal/shared/money/units.go#L13) |
| sub2api 余额门槛与固定费用 | `D:/github-project/sub2api/backend/internal/service/billing_cache_service.go:744,800,1009-1042` |
| 在途首笔放行与并发原子检查 | `D:/github-project/sub2api/backend/internal/repository/billing_inflight_cache.go:43-71` |
| 在途故障放行和预留金额截断 | `D:/github-project/sub2api/backend/internal/service/billing_inflight_reservation.go:248-257,300-347` |
| 默认阈值、预留 TTL 和估算配置 | `D:/github-project/sub2api/backend/internal/config/config.go:929-947,2132-2141` |
| 先去重、再事务扣费与允许负数 | `D:/github-project/sub2api/backend/internal/repository/usage_billing_repo.go:22-109,181-187,245-274` |
| 余额耗尽时失效缓存 | `D:/github-project/sub2api/backend/internal/service/gateway_usage_billing.go:468-493` |
| 首笔费用超过余额仍放行的测试 | `D:/github-project/sub2api/backend/internal/repository/billing_inflight_cache_test.go:92-106` |
| 负余额扣款与下一次拒绝的测试 | `usage_billing_repo_unit_test.go:51-104`；`backend/internal/service/billing_cache_service_balance_test.go:44-109`（均在 sub2api） |

## 9. 本次实际检查

已检查：

- 当前项目 Git 状态、AI 相关目录、钱包/流水结构和架构文档；
- 旧 Go AI 模块、Admin seed、权限合同、旧数据库 schema；
- 旧前端 AI 页面、API 和动态路由生成物；
- Canvas 当前本地 AI 配置、图片/视频 API 和模型选择；
- sub2api 的余额准入、在途预留、实际扣费、缓存失效和相关测试源码。
- OpenAI 官方 Function calling 指南中的 Responses 多步工具流程、调用结果类型与调用标识关联；仅做文档核对，未发起模型请求。

实际验证：

- `git status --short`：开始时仅本盘点文档未跟踪，未清理或覆盖他人改动。
- `git diff --check`：检查已跟踪文档的变更空白；本盘点未跟踪，另用显式文本检查覆盖尾随空格、标题、围栏、表格、表名集合与本地 Markdown 链接。
- 以 PowerShell 正则提取旧 `schema.sql` 的 CREATE TABLE 名称，对比第 4 节主表格：33 张 AI 表和 3 张钱包表全部列入，无重复、无遗漏或虚构表。
- 曾在 sub2api/backend 执行 `$env:GOTOOLCHAIN='local'; go test -tags unit ./internal/repository -run '^TestInflightReservation_FirstRequestAlwaysAdmitted$' -count=1`，
  结果 exit 1：`go.mod requires go >= 1.27.3 (running go 1.26.5; GOTOOLCHAIN=local)`。未执行到测试，未改参考项目版本、未下载/切换工具链。

未运行当前项目全量 Go/Vitest/build、真实 AI 上游/支付或多实例压测；本轮没有运行时代码改动。
没有连接/修改数据库、写菜单/权限、清理 Redis、启停服务或执行 Git 提交。
新增本文并更新 `STATUS.md` 盘点记录；目标候选与未定预算不视为已批准或已实现。
