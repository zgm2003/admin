# Admin AI 模块最终设计稿

> 版本：v1.2 · 2026-10-28（维护者确认供应商切换语义；首期具名供应商为 OpenAI）
> 用途：一次读清本期新增哪些菜单、数据表、字段、权限和功能，以及如何生成、存储、计费和恢复。
> 状态：**维护者已确认产品方向，实施方案已整理并开始基础实现；21表结构首版仅在隔离测试schema验证，正式库未建AI表/菜单，未获得执行正式库迁移的授权。**
> 已确认的产品规则见第 1 节；本文对其余工程细节给出明确设计选择，不把工程选择冒充维护者此前已逐项确认。

## 阅读导航

- 想知道做什么、哪些不做：第 1 节。
- 想知道新增哪些菜单、页面长什么样：第 2 节。
- 想知道有哪些按钮和受保护 API：第 3 节。
- 想知道新增与修改哪些表：第 4、5 节。
- 想知道一次 AI 调用怎么完成、怎么扣钱：第 6、7 节。
- 想知道工具、四类媒体、COS 怎么工作：第 8、9 节。
- 想知道多实例、缓存、Worker、失败恢复和验收：第 10～13 节。

## 1. 本期范围与不可变规则

### 1.1 已确认的产品决定

1. **只建设 Admin 内部 AI，Canvas 不动。** 不恢复旧 Canvas transport，也不以 Canvas 联调作为验收。
2. Admin 提供文字、图片、视频、音频四类消费入口，统一使用 Run、用量事实和支付钱包。
3. 配置顺序为「供应商 → 标准模型映射/供应商模型定价 → Agent → 工作台」。**工作台只能选择 Agent**，不选择供应商或模型。
4. 一个 Agent 可保存多个供应商模型绑定，**当前使用项只有一个**。管理员手动切换，失败不自动换供应商、不轮询、不负载均衡。
5. **维护者再次确认：切换 Agent 当前供应商，新 Run 用新的；已开始的任务继续原供应商。** Run 创建时冻结绑定、凭据版本、模型、协议、价格、倍率及工具版本；排队中、执行中或恢复中的已有 Run 均不重新读取 Agent 当前绑定。
6. 对话/工具默认使用 **Responses API**；Chat Completions 只作为管理员显式配置的兼容协议，不做运行失败后的隐式降级。
7. Images、Audio 使用对应能力接口；视频采用独立异步提交/查询/下载流程，不能假设普通 OpenAI 兼容接口一定支持视频。
8. 系统提示词直接由 Agent 配置拥有；Run 保存实际使用内容的快照。**不建独立 Prompt 库及其菜单、API、表。**
9. Tool 支持 HTTP 和具名内置工具；不开放任意代码、Shell、命令执行，也不默认开放上游全部托管工具。
10. 图片、视频、音频输出统一转存现有 COS，保存文件记录、媒体元信息和 Run 关联，不把上游临时 URL 当长期结果。
11. 按实际用量计费：内部人民币分项单价 × 实际 token/媒体用量 ÷ 计价数量 × Agent 倍率；Run 结束后统一结算。LLM 按输入/缓存输入/输出 token 分项，图片按质量/尺寸对应的实际生成张数计费，不另叠加图片 token 费用。
12. 扣当前登录操作者自己的 `payment_wallet`；管理员同样遵守准入和结算规则，不为其他用户代付。
13. 参考 sub2api 的余额准入、在途预留和据实扣费：已准入请求允许结算为负余额，余额不足后拒绝新请求；不承诺负数绝对值永远很小。
14. 官方模型保留标准身份与能力；供应商模型同步保存美元参考价（只读），不修改上游报价。系统内部另维护人民币售价：新计费项首次同步时复制同一计价单位下的美元数值作为初始值，管理员后续可修改人民币售价；再次同步保留已有人民币售价，不强制美元/人民币长期 1:1，也不使用真实汇率。价格经确认生效后不可变，旧 Run 和历史账单不回算。
15. **不做 Content、Context/RAG、文档上传切片、Embedding、Rerank、向量库或会话记忆工程。** 普通对话历史不属于 RAG。
16. **首期具名供应商为 OpenAI。** OpenAI 兼容协议与官方价格来源分别校验；自定义 Base URL 的兼容供应商不能因此自动套用 OpenAI 官方美元报价。

### 1.2 本设计采用的工程取舍

- 使用当前 Go/Gin、PostgreSQL、Redis、Asynq、Scheduler、realtime、COS 和 Vue 3 体系，不引入第二套框架或钱包。
- 新增 **21 张 `ai_*` 表**；修改现有 `payment_transaction`，`payment_wallet` 结构不变。存储、权限、配置缓存和调度复用现有表。
- 不照搬旧项目 33 张 AI 表；不建 Prompt/RAG 表、独立 AI 钱包、独立素材管理菜单或首期统计聚合表。
- 文字、图片、视频、音频共用创建 Run 的消费 API；异步执行复用 Scheduler/Worker，浏览器刷新不重发生成。
- 供应商配置和供应商模型价格来自有版本的缓存；美元参考价只读，人民币内部计费价可维护但按版本冻结；钱包准入、在途记录和扣费使用 PostgreSQL 短事务，避免依赖失效余额缓存。
- 错误、未知用量、转存失败分别表达，不能以空结果、零费用或重新调用模型掩盖失败。

## 2. 菜单与页面功能

### 2.1 固定菜单清单：1 个目录、6 个页面

| 层级 | 中文 / 英文 | path | componentPath | page code | 页面负责什么 |
| --- | --- | --- | --- | --- | --- |
| 目录 | AI / AI | 空 | 空 | `ai` | 仅组织下属页面，不可直接授权 |
| 页面 | 供应商配置 / Providers | `/ai/provider` | `ai/provider` | `ai:provider:view` | 配置真实上游、密钥、模型、协议能力与模型人民币售价 |
| 页面 | 官方模型 / Official Models | `/ai/officialModel` | `ai/officialModel` | `ai:officialModel:view` | 标准模型目录与能力，按关联供应商模型只读展示报价/内部售价 |
| 页面 | 智能体配置 / Agents | `/ai/agent` | `ai/agent` | `ai:agent:view` | 提示词、倍率、工具、多供应商绑定与手动切换当前项 |
| 页面 | 工具管理 / Tools | `/ai/tool` | `ai/tool` | `ai:tool:view` | HTTP/内置工具配置、输入输出约束和启停 |
| 页面 | 运行监控 / Runs | `/ai/run` | `ai/run` | `ai:run:view` | 按授权跨用户查看运行、请求、工具、媒体和计费事实 |
| 页面 | AI 工作台 / AI Workspace | `/ai/chat` | `ai/chat` | `ai:chat:view` | 四类消费、自己的会话和生成记录、结果预览与下载 |

所有页面均为 Admin 平台可见 page，`is_hidden=0`。目录 `i18nKey=navigation.ai`；页面分别为
`navigation.aiProvider/aiOfficialModel/aiAgent/aiTool/aiRun/aiChat`，中英文同时交付。page 的 `path` 必须等于
`/` 加 `componentPath`。action 隐藏挂在对应 page 下，不带 path/component/icon/i18n 渲染字段。

### 2.2 供应商配置

- 列表：名称、启停、模型数量、当前凭据版本、最近连接探测结果、更新时间；密钥仅显示“已配置/未配置”。
- 新增/编辑：名称、HTTPS Base URL、API Key、自定义受限请求头、超时、允许的模型/媒体能力。密钥加密保存，编辑空密钥表示保持，不表示清空。
- 模型页签：上游模型 ID、显示名、支持操作、限制参数、标准模型映射、启停。模型同步先看差异，再确认保存；不自动覆盖人工映射。
- 模型价格页签并列展示「供应商参考价 USD/M token（只读）」与「内部售价 RMB/M token（可编辑）」，输入、缓存输入、输出分别设置。图片则按真实质量/尺寸展示 USD/张与 RMB/张；音视频按其实际计价单位展示，不能统一标成每百万 token。
- 新计费项同步时内部售价自动填入同单位下的美元数值；已有人民币售价再次同步时保持不变。修改人民币售价需独立 updatePrice 权限，同步需独立 syncPrice 权限；均生成预览并确认生效，不修改历史版本。
- 连接测试只做不产生推理消费的配置/连通性/模型列表检查。真实能力验证通过工作台创建付费 Run，不提供绕过钱包的免费 test 接口。
- 配置更新产生不可变凭据版本；新 Run 使用新版本，已有 Run 引用原版本。被引用供应商只可停用，不可硬删。

### 2.3 官方模型

- 列表：厂商、标准模型键、显示名、支持的媒体/操作、关联供应商模型、当前内部人民币价格版本、生效时间、启停。
- 详情：展示供应商同步的美元参考价（只读）与系统内部人民币计费价；覆盖输入/缓存输入/输出 token、图片质量/尺寸、音视频时长等真实计费维度，不把所有模型强行折成“每次请求一个价”。
- 价格仅按关联供应商模型只读聚合展示；同步、人民币改价与确认生效入口位于供应商模型价格页签，并分别检查 Provider 的价格动作权限。标准模型编辑不能修改报价或内部售价。
- Provider 的模型 ID 可以不同，但必须明确映射到一个标准模型；没有完整可用价格的操作不能启用消费。
- 历史价格版本只读；停用标准模型阻止新 Run，不抹掉历史价格、用量和流水。首期不提供删除标准模型或删除已生效价格版本的动作。

### 2.4 智能体配置

- 基本信息：名称、说明、头像文件引用、启停、允许的媒体能力、倍率、系统提示词、生成参数限制。
- 供应商模型绑定：一个 Agent 可配置多行，每行绑定一个 Provider 模型；通过“设为当前”选择唯一当前项。
- 不存在当前项、当前 Provider/Model 已停用、缺少价格或所选操作不受支持时，后端明确拒绝创建 Run；不偷偷选第一项。
- “设为当前”使用事务行锁，更新 Agent 自己的当前项，不修改 Provider 全局状态；普通编辑遵循现有 last-write-wins，最后提交的完整配置生效。
- 工具绑定：从启用工具中选择，限制工具暴露名、允许参数及调用上限；系统提示词与工具配置在 Run 中冻结。
- 同一 Agent 可声明多个媒体能力，但工作台只展示**当前绑定真实支持且价格完整**的能力；不为不同媒体自动选择其他绑定。
- 试用按钮跳转工作台预选 Agent，仍走相同权限、本人钱包和计费链路，不新增管理端免计费生成能力。

### 2.5 工具管理

- 维护名称、暴露给模型的说明、JSON Schema、工具类型、超时、结果大小、启停与副作用/幂等约束。
- HTTP 工具固定受限 URL、方法、鉴权秘密、请求映射和响应映射；模型只提供 schema 内业务参数，不能改目标主机或鉴权头。
- 内置工具首期为 `currentTime` 和 `calculator`：前者输出指定合法时区的时间，后者用专用四则运算解析器，不使用 eval 或执行代码。
- 配置修订产生不可变 Tool Version；已创建 Run 使用冻结版本。默认本地工具不另收工具次数费用，模型处理工具参数/结果的 token 仍属于实际用量。
- 不提供绕过 Agent/Run 的通用工具执行 API，也不恢复旧“AI 自动生成工具”动作。

### 2.6 运行监控

- 按时间、用户、Agent、媒体类型、供应商、运行状态、用量完整性、结算状态、转存状态筛选和分页。
- 详情展示输入/输出摘要、配置快照、每轮模型请求、工具调用、媒体结果、价格版本、分项费用、钱包交易号和可公开错误。
- 请求失败、账单待核对、扣款待重试、媒体保存待恢复分别可见，不统一伪装成“生成失败”。
- 首期为管理只读，不开放替他人生成、人工结算、退款、修改余额或盲目重试生成。查看全局 Run 不改变费用归属。
- 汇总仅统计指定时间窗口的 Run/用量/费用，不新增首期统计表；查询范围和预算见第 10 节。

### 2.7 AI 工作台

| 页签 | 输入与交互 | 输出与记录 |
| --- | --- | --- |
| 文字 | 选 Agent、输入消息、可选自己有权使用的图片附件；流式展示及工具执行进度 | 普通会话、消息、最终文本、各轮用量与费用 |
| 图片 | 选 Agent、提示词、支持的尺寸/质量/数量；编辑能力启用时允许自己的输入图片 | 图片网格、COS 预览/下载、每个结果的保存状态 |
| 视频 | 选 Agent、提示词、支持的时长/分辨率；上游支持时允许参考图片 | 异步进度、已保存的视频、播放/下载及费用 |
| 音频 | 选 Agent；根据能力选择文字转语音或音频转文字，参数由后端 options 输出 | COS 音频或转写文本、播放/下载、时长/token 与费用 |

界面只展示后端声明的可操作结论与参数范围；切换语言重新读取静态 options，值类型保持。用户刷新页面可以恢复
Run 状态及已持久化事件，不能重放生成。停止不保证上游零消费；断开浏览器连接也不等于取消 Run。自己的历史和跨用户
运行监控是两个不同授权面；没有管理 Run 权限的人仍能查看自己的消费。

## 3. 权限与 API 契约

统一前缀 `/api/admin/v1`。页面 `:view` 只决定动态路由；除第 3.3 节纯静态 options 的明确例外外，业务 API 均显式绑定 `authenticate + requirePermission(对应 action)`。
action/page 双向独立；不会因为拥有 generate 自动获得 page，也不会因为拥有 page 自动获得消费权限。
后端通过身份强制指定本人 `user_id`；消费请求不接受可替换扣费用户的字段。

### 3.1 管理操作

| 资源 | METHOD 路径（省略统一前缀） | action |
| --- | --- | --- |
| Provider | `GET /ai/provider`；`GET /ai/provider/:id` | 分别 `ai:provider:list`、`ai:provider:detail` |
| Provider | `POST /ai/provider`；`PUT /ai/provider/:id`；`PATCH /ai/provider/:id/status`；`DELETE /ai/provider/:id` | 分别 `ai:provider:create/update/status/delete`，逐条显式声明 |
| Provider | `POST /ai/provider/:id/test`；`POST /ai/provider/:id/model/sync-preview` | 分别 `ai:provider:test`、`ai:provider:sync` |
| Provider Model | `GET /ai/provider/:id/model`；`PUT /ai/provider/:id/model` | 分别 `ai:provider:detail`、`ai:provider:update` |
| Official Model | `GET /ai/officialmodel`；`GET /ai/officialmodel/:id` | 分别 `ai:officialModel:list`、`ai:officialModel:detail` |
| Official Model | `POST /ai/officialmodel`；`PUT /ai/officialmodel/:id`；`PATCH /ai/officialmodel/:id/status` | 分别 `ai:officialModel:create/update/status`，逐条声明 |
| Provider Model Price | `GET /ai/provider/:id/model/:modelId/price`；`POST /ai/provider/:id/model/:modelId/price/sync` | 分别 `ai:provider:detail`、`ai:provider:syncPrice`；同步保存只读美元参考价并生成预览 |
| Provider Model Price | `PUT /ai/provider/:id/model/:modelId/price`；`POST /ai/provider/:id/model/:modelId/price/:versionId/publish` | `ai:provider:updatePrice`，仅维护人民币售价预览及确认生效，不允许改写美元参考价 |
| Official Model Price View | `GET /ai/officialmodel/:id/price` | `ai:officialModel:detail`，聚合展示供应商美元参考价与内部人民币价格 |
| Agent | `GET /ai/agent`；`GET /ai/agent/:id` | 分别 `ai:agent:list`、`ai:agent:detail` |
| Agent | `POST /ai/agent`；`PUT /ai/agent/:id`；`PATCH /ai/agent/:id/status`；`DELETE /ai/agent/:id` | 分别 `ai:agent:create/update/status/delete`，逐条声明 |
| Agent 当前项 | `PATCH /ai/agent/:id/binding/current` | `ai:agent:switchBinding` |
| Tool | `GET /ai/tool`；`GET /ai/tool/:id` | 分别 `ai:tool:list`、`ai:tool:detail` |
| Tool | `POST /ai/tool`；`PUT /ai/tool/:id`；`PATCH /ai/tool/:id/status`；`DELETE /ai/tool/:id` | 分别 `ai:tool:create/update/status/delete`，逐条声明 |
| Run | `GET /ai/run`；`GET /ai/run/summary` | `ai:run:list` |
| Run | `GET /ai/run/:id`；其 `/request`、`/tool-call`、`/usage-charge` 子路径 | `ai:run:detail` |
| Run 输出 | `GET /ai/run/:id/media/:mediaId/preview` | `ai:run:detail`，另核对该媒体确属此 Run |

表中的 `create/update/status/delete` 是排版缩写，不是可直接使用的权限码或自动派生规则；实现和 migration 逐一写出
完整 code。带 `:id` 的查改操作均验证实体存在、启停/引用状态，不允许用软删除绕过账务引用。

### 3.2 本人消费操作

| METHOD 路径 | action | 数据范围/含义 |
| --- | --- | --- |
| `GET /ai/chat/agent` | `ai:chat:generate` | 可消费 Agent 简要信息、当前能力与允许参数，无上游秘密 |
| `GET /ai/chat/conversation`；`GET /ai/chat/run` | `ai:chat:list` | 仅自己的会话和四类生成历史 |
| `GET /ai/chat/conversation/:id`；其 `/message` 子路径 | `ai:chat:detail` | 仅自己的会话/消息 |
| `POST /ai/chat/conversation` | `ai:chat:create` | 创建文字会话，不等于消费 |
| `PUT /ai/chat/conversation/:id` | `ai:chat:update` | 修改自己的会话标题 |
| `DELETE /ai/chat/conversation/:id` | `ai:chat:delete` | 隐藏自己的会话，不删除 Run/账单/流水 |
| `POST /ai/chat/run` | `ai:chat:generate` | 四类统一创建，必填 Agent、operation、输入和 UUID 请求幂等键 |
| `GET /ai/chat/run/:id`；其 `/event` 子路径 | `ai:chat:detail` | 自己的状态、结果、费用及按 sequence 补拉事件 |
| `POST /ai/chat/run/:id/cancel` | `ai:chat:cancel` | 仅本人，后端状态允许时请求停止 |
| `GET /ai/chat/run/:id/media/:mediaId/preview` | `ai:chat:detail` | 自己的输出，经授权签发短时文件地址 |
| `POST /ai/chat/run/:id/media/retry` | `ai:chat:retryTransfer` | 只重试尚可恢复的 COS 保存，不重新生成、不重复计费 |

上传输入附件复用 `storage:object:upload` 及现有上传接口，另验证所有权、平台和 AI 用途；查余额/充值复用现有
`payment:myWallet:detail` 及个人充值权限，不建第二套钱包入口。缺少钱包展示权限不免除服务器端余额准入。
所有 AI 列表默认 20 条、最大 100 条；详情下的请求、事件、工具和计费项也分页，不一次返回无限历史。

### 3.3 静态 options、动态候选及写入边界

- `GET /ai/provider/options`、`/ai/officialmodel/options`、`/ai/agent/options`、`/ai/tool/options`、
  `/ai/run/options`、`/ai/chat/options` 仅要求当前 Admin authenticate，沿用项目既有静态 options 形状，不新增 options action。
  只返回协议/状态/类型 enum、程序默认值和静态 constraints，按语言输出 `{value,label}`，0 PostgreSQL/Redis。
- 动态候选不放进静态 options。`GET /ai/agent/candidate/create` 使用 `ai:agent:create`，
  `GET /ai/agent/:id/candidate/update` 使用 `ai:agent:update`，按 kind=providerModel/tool 返回窄 DTO：
  ID、名称、有效性、能力和必要限制，不返回凭据或工具鉴权。候选按关键字分页，不读整库或借用工作台 Agent 接口。
- 标准模型映射候选为 `GET /ai/provider/candidate/create` → `ai:provider:create`、
  `GET /ai/provider/:id/candidate/update` → `ai:provider:update`；只返回可映射标准模型身份与能力，不赋予模型价格管理权。
- Create 可指定初始实体 status 和初始当前绑定；普通 Update DTO **排除实体 status 与 Agent.activeBindingId**，禁止
  通过额外字段绕过独立动作。实体启停仅走 status API，已创建 Agent 的当前项仅走 switchBinding API。
- Agent 普通编辑在行锁中保留提交时的当前项，不用陈旧表单覆盖它；要移除/停用当前绑定，先独立切换当前项或停用 Agent，
  然后编辑。普通可编辑配置保持 last-write-wins，不增加 expectedVersion/generation 管理乐观锁。
- 普通 Provider/Provider Model/Official Model 创建和编辑 DTO 不接受价格字段；模型目录同步不隐式执行价格同步。
  美元参考价只能由后端具名供应商同步流程写入；人民币价格预览/确认生效必须检查 ai:provider:updatePrice，
  不能通过普通 update 或模型 sync 绕过。价格路由逐一核对 modelId 属于该 Provider、versionId 属于该模型。

### 3.4 完整 action 清单

下列每项都是独立、隐藏的 action，共 **38 个**，不包括 6 个 page 的 :view；连同目录总共新增 45 个菜单节点。

| 页面 | 完整 action code |
| --- | --- |
| Provider | `ai:provider:list`、`ai:provider:detail`、`ai:provider:create`、`ai:provider:update`、`ai:provider:status`、`ai:provider:delete`、`ai:provider:test`、`ai:provider:sync`、`ai:provider:syncPrice`、`ai:provider:updatePrice` |
| Official Model | `ai:officialModel:list`、`ai:officialModel:detail`、`ai:officialModel:create`、`ai:officialModel:update`、`ai:officialModel:status` |
| Agent | `ai:agent:list`、`ai:agent:detail`、`ai:agent:create`、`ai:agent:update`、`ai:agent:status`、`ai:agent:delete`、`ai:agent:switchBinding` |
| Tool | `ai:tool:list`、`ai:tool:detail`、`ai:tool:create`、`ai:tool:update`、`ai:tool:status`、`ai:tool:delete` |
| Run | `ai:run:list`、`ai:run:detail` |
| Chat | `ai:chat:generate`、`ai:chat:list`、`ai:chat:detail`、`ai:chat:create`、`ai:chat:update`、`ai:chat:delete`、`ai:chat:cancel`、`ai:chat:retryTransfer` |

## 4. 数据结构总览

### 4.1 新增 21 张表

| 序号 | 表名 | 主要职责 | 归属模块 |
| ---: | --- | --- | --- |
| 1 | `ai_provider` | 供应商逻辑配置、当前凭据版本 | `ai/provider` |
| 2 | `ai_provider_version` | 不可变 Base URL/加密凭据/请求策略版本 | `ai/provider/version` |
| 3 | `ai_provider_model` | 上游模型、能力、限制和标准模型映射 | `ai/provider/model` |
| 4 | `ai_official_model` | 标准模型身份和能力目录 | `ai/officialModel` |
| 5 | `ai_provider_model_price_version` | 供应商同步美元参考价与内部人民币价格版本，生效后不可变 | `ai/provider/model/priceVersion` |
| 6 | `ai_provider_model_price_rate` | 价格版本下按 Token/次数/时长及 selector 的计费项 | `ai/provider/model/priceRate` |
| 7 | `ai_agent` | 提示词、倍率、能力、唯一当前绑定 | `ai/agent` |
| 8 | `ai_agent_model_binding` | Agent 的多个供应商模型绑定 | `ai/agent/modelBinding` |
| 9 | `ai_agent_tool` | Agent 与工具的启用绑定 | `ai/agent/tool` |
| 10 | `ai_tool` | 工具逻辑定义和当前版本 | `ai/tool` |
| 11 | `ai_tool_version` | 工具 schema、HTTP/内置执行定义和秘密版本 | `ai/tool/version` |
| 12 | `ai_conversation` | 普通文字会话及用户归属 | `ai/conversation` |
| 13 | `ai_message` | 用户/助手/工具消息和有序上下文 | `ai/message` |
| 14 | `ai_run` | 一次消费、快照、准入预留、执行/计费/保存状态 | `ai/run` |
| 15 | `ai_provider_request` | 每轮真实上游请求与用量，不代表故障切换 | `ai/providerRequest` |
| 16 | `ai_tool_call` | 一次工具请求、结果、错误和副作用幂等 | `ai/toolCall` |
| 17 | `ai_run_event` | 有序可恢复事件和实时发布事实 | `ai/run/event` |
| 18 | `ai_media_task` | 异步媒体任务、上游任务标识和轮询恢复 | `ai/mediaTask` |
| 19 | `ai_media_file` | 输入/输出媒体与存储文件、转存状态的关联 | `ai/mediaFile` |
| 20 | `ai_usage_charge` | 一次 Run 的汇总费用与幂等结算 | `ai/usage/charge` |
| 21 | `ai_usage_charge_item` | 各请求/计费维度的数量、费率和金额快照 | `ai/usage/chargeItem` |

### 4.2 现有对象：复用与修改清单

| 对象 | 本期处理 |
| --- | --- |
| `payment_wallet` | 复用用户唯一钱包，有符号 bigint 余额，**不改结构、不加非负约束** |
| `payment_transaction` | 扩展消费方向、扣前余额、AI charge 来源及相应约束/索引，详见第 5.22 节 |
| `storage_file` | 复用文件事实和对象坐标，输出使用现有 `kind=1`，不新建第二套文件表 |
| `storage_cos_config`、`storage_cos_config_version` | 复用全局 COS 逻辑配置与历史版本，不建立所谓 Admin 专属 COS 配置 |
| `storage_upload_rule*` | 复用 Admin 平台当前有效规则，增加 AI 用途 code；同平台不新增第二条启用规则 |
| `permission_menu`、角色/授权/版本表 | 通过 Admin 平台 forward migration 增加目录、6 page 和明确 actions，不改 Canvas 菜单 |
| `system_scheduler_*` | 复用持久任务、发布、租约和恢复；不新建另一套 AI 调度/outbox 表 |
| `realtime_event*` | 复用实时通知运输；AI Run 事件仍是可恢复的业务事实 |
| `system_setting`、现有配置 generation/outbox 表 | 复用工程限制、缓存版本/变更发布，不创建 AI 配置中心或通用注册器 |

### 4.3 通用字段与数据规则

- 下节字段表加上此处公共列才构成完整目标结构；未标 `NULL` 的列均 `NOT NULL`。
- 除明确列出复合主键者，`id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY`；公开 DTO 将 ID 输出为十进制字符串，避免 JS 精度丢失。
- **每张新增表**均显式有 `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`、`updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`，不嵌入 BaseModel。
- 有真实删除入口的 Provider、Provider Model、Agent、Tool、Conversation 使用 `deleted_at TIMESTAMPTZ NULL`；其余无删除入口的事实/版本/账单表不增加该列。
- Yes/No 固定 `SMALLINT CHECK(value IN(0,1))`；状态和 protocol 使用具名 SMALLINT CHECK，候选由后端 options 输出，不借展示字典维护状态机。
- JSONB 分别约束为 object/array，并由 DTO 和领域校验限定字段、长度、schema 深度，不接受任意不受限 JSON。金额与用量不使用 float/double。
- 普通配置 CRUD 沿用 last-write-wins，不把缓存 generation 当管理员乐观锁；Run/Worker 才使用 state_version 和 lease token 条件。
- 跨表引用使用明确外键及 `ON DELETE RESTRICT`，不级联删除消费事实；表间用户/平台/归属在事务内再次核对。
- 价格发布后、Provider/Tool 版本创建后，以及用量结算后，核心快照不可修改；用具名触发器保护，不靠前端只读。
- 软删业务 code 使用 `UNIQUE ... WHERE deleted_at IS NULL`；被引用对象在业务层拒删并提示停用。绑定移除采用 `status=0` 保留引用。
- 下节索引均是主键、业务唯一、外键关联或实际列表/队列/结算查询所需；不为每个字段机械建索引。

## 5. 逐表结构与约束

下列是目标结构，不是当前已存在表。所有表均包含第 4.3 节的主键和时间字段；未标 NULL 的列必填。
配置状态 0=停用、1=启用，均有 SMALLINT CHECK。各字段表加公共列即构成完整字段清单。

### 5.1 ai_provider

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| code | VARCHAR(64) | 非空稳定编码，创建后不可改 |
| name / description | VARCHAR(100) / VARCHAR(1000) | 名称与说明，说明默认空文本 |
| status | SMALLINT | 0/1，默认 0 |
| current_version | BIGINT | 当前凭据版本号，>0 |
| created_by / updated_by | BIGINT | FK → user_account |
| deleted_at | TIMESTAMPTZ NULL | 软删除 |

唯一 `code WHERE deleted_at IS NULL`；索引 `(status,id DESC) WHERE deleted_at IS NULL`。复合 FK
`(id,current_version)` → `ai_provider_version(provider_id,version)` 延迟检查；首个版本与 Provider 同事务创建。

### 5.2 ai_provider_version

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| provider_id / version | BIGINT / BIGINT | FK → ai_provider；version>0 |
| base_url | VARCHAR(2048) | 经服务端安全校验的 HTTPS API 根地址 |
| api_key_ciphertext | TEXT | 认证加密密文，不能输出到 DTO/日志 |
| encryption_key_id | VARCHAR(64) | 密钥版本，缺失不能执行 |
| headers_ciphertext | TEXT NULL | 受限额外请求头密文，禁止覆盖 Host 等危险头 |
| media_host_allowlist | JSONB | array，上游媒体下载允许域名 |
| request_timeout_ms | INTEGER | 1000～300000，同时受 Run 总 deadline 限制 |
| created_by | BIGINT | FK → user_account |

唯一 `(provider_id,version)`。创建后不可变；历史 Run 引用版本 id，不读取 Provider 当前凭据替代。

### 5.3 ai_provider_model

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| provider_id | BIGINT | FK → ai_provider |
| upstream_model_id / name | VARCHAR(200) / VARCHAR(100) | 真实上游模型 ID 与展示名 |
| official_model_id | BIGINT NULL | FK → ai_official_model；未映射只可停用保存 |
| operations | TEXT[] | 非空、无重复，值见第 6.1 节 |
| limits | JSONB | object，token/尺寸/数量/时长等能力限制 |
| status | SMALLINT | 0/1；启用必须有 official_model_id |
| deleted_at | TIMESTAMPTZ NULL | 引用中只停用 |

唯一 `(provider_id,upstream_model_id) WHERE deleted_at IS NULL`；索引 `(official_model_id)`、
`(provider_id,status,id) WHERE deleted_at IS NULL`。启用消费还校验真实能力和已发布费率，不从模型 ID 猜能力。

### 5.4 ai_official_model

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| vendor_code / model_key | VARCHAR(64) / VARCHAR(200) | 标准厂商与模型键 |
| name | VARCHAR(100) | 展示名 |
| operations | TEXT[] | 标准允许操作集合 |
| source_urls | JSONB | array，公开模型/价格可追溯来源，不自动抓任意 URL |
| status | SMALLINT | 0/1，默认 0 |
| created_by / updated_by | BIGINT | FK → user_account |

唯一 `(vendor_code,model_key)`；索引 `(status,id)`。不提供删除，不加 deleted_at。

### 5.5 ai_provider_model_price_version

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| provider_model_id / version | BIGINT / BIGINT | FK → ai_provider_model；version>0 |
| status | SMALLINT | 0 预览、1 已发布；发布后不可修改 |
| effective_at | TIMESTAMPTZ NULL | 生效时必填，可为未来时间 |
| currency | CHAR(3) | 固定 CNY，内部计费币种 |
| source_currency | CHAR(3) | 固定 USD；仅表达供应商参考价的原始币种 |
| source_digest | CHAR(64) | 供应商同步响应规范化后的 SHA-256 |
| snapshot_digest | CHAR(64) | 来源摘要、所有人民币售价和生效时间的摘要，确认发布所见预览 |
| source_payload | JSONB | 受限、脱敏的供应商价格响应审计，不含密钥 |
| synced_at | TIMESTAMPTZ | 本次供应商同步时间 |
| created_by / published_by | BIGINT / BIGINT NULL | FK → user_account；同步/确认操作者 |
| published_at | TIMESTAMPTZ NULL | 内部人民币售价确认生效时间 |

唯一 `(provider_model_id,version)`、`(provider_model_id,id)`；已发布版本唯一 `(provider_model_id,effective_at) WHERE status=1`；
索引 `(provider_model_id,effective_at DESC) WHERE status=1`，发布时锁供应商模型行并核对 snapshot_digest，重复发布幂等返回。
供应商同步只追加版本并写入美元参考价；首次计费项的人民币数值复制同单位美元数值，后续同步保留已有人民币售价。
管理员改价时从既有版本复制新的预览，沿用其只读美元来源、source_digest 和 synced_at，仅编辑人民币项与生效时间。
美元参考价和来源即使处于预览也不可被管理员改写；整版发布后不可修改。Run 只选择 accepted_at 时已生效的最近版本。

### 5.6 ai_provider_model_price_rate

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| price_version_id | BIGINT | FK → ai_provider_model_price_version |
| operation / usage_key | VARCHAR(32) / VARCHAR(64) | 操作与 input/cacheInput/output token、图片次数、音视频时长等维度 |
| selector / selector_hash | JSONB / CHAR(64) | 规范化尺寸/质量等 object 及其 SHA-256 |
| unit | VARCHAR(16) | token / image / second / character / request |
| unit_quantity | NUMERIC(24,8) | >0；token 统一为 1000000、图片为 1，其他操作按真实计价单位规范化 |
| source_unit_price | NUMERIC(30,12) | >=0，与 unit_quantity 对应的规范化美元参考单价；只读同步保存，原始报价保留在 source_payload |
| billing_unit_price_units | BIGINT NULL | >=0，系统内部人民币售价，已发布版本必填；管理员可在预览版本中调整，未设置不能当作 0 |

唯一 `(price_version_id,operation,usage_key,selector_hash)`。选择器禁止重叠或多重匹配；缺价格不能按零价格处理。
供应商同步不得改写历史版本；人民币售价只能在未生效预览版本中维护，已生效版本不可增删改。
operation/usage_key/selector/unit/unit_quantity 和美元参考价均来自服务端同步；人民币改价 DTO 不接受这些只读字段。
美元数值复制人民币初始值时用精确十进制转换；超过人民币 8 位精度或 bigint 范围时保留只读来源并明确要求管理员填写合法售价，
不得静默截断成免费项。显式售价 0 与未填写有区别，只有完整合法的计费项才能发布。后文简称 Price Rate。

### 5.7 ai_agent

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| code / name / description | VARCHAR(64) / VARCHAR(100) / VARCHAR(1000) | 稳定编码、名称、说明 |
| avatar_file_id | BIGINT NULL | FK → storage_file，复用文件引用保护 |
| capabilities | TEXT[] | text/image/video/audio 非空子集 |
| system_prompt | TEXT | 系统提示词，可空文本，最大 64 KiB |
| multiplier_ppm | BIGINT | 1～100000000；1000000=1 倍 |
| active_binding_id | BIGINT NULL | 唯一当前绑定；Agent 启用时必填 |
| generation_params | JSONB | object，合法生成参数，后端校验 |
| status | SMALLINT | 0/1，默认 0 |
| created_by / updated_by | BIGINT | FK → user_account |
| deleted_at | TIMESTAMPTZ NULL | 被会话/Run 引用时只停用 |

唯一 `code WHERE deleted_at IS NULL`；索引 `(status,id) WHERE deleted_at IS NULL`。复合 FK
`(id,active_binding_id)` → `ai_agent_model_binding(agent_id,id)` 延迟检查，防串绑。编辑/切换锁 Agent 行；
事务完成前校验启用 Agent 的当前绑定启用。Provider 以后停用阻止新 Run，不自动换绑定。普通 CRUD 保持 last-write-wins。

### 5.8 ai_agent_model_binding

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| agent_id / provider_model_id | BIGINT / BIGINT | FK → ai_agent / ai_provider_model |
| text_protocol | SMALLINT | 1 Responses、2 Chat Completions，默认 1；不改变媒体专用接口 |
| parameters | JSONB | object，按模型约束的绑定参数 |
| status | SMALLINT | 0/1，默认 1 |

唯一 `(agent_id,provider_model_id)`、`(agent_id,id)`；索引 `(provider_model_id)`。移除用停用、不加 deleted_at；
停用当前项前必须先通过独立动作切换当前项或停用 Agent，再编辑绑定；不能用普通 PUT 绕过切换/启停权限。多条绑定不是备援列表。

### 5.9 ai_agent_tool

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| agent_id / tool_id | BIGINT / BIGINT | FK → ai_agent / ai_tool |
| exposed_name | VARCHAR(64) | 此 Agent 中模型可见的稳定工具名 |
| parameter_constraints | JSONB | object，只能收窄工具允许参数 |
| status | SMALLINT | 0/1 |

唯一 `(agent_id,tool_id)`、`(agent_id,exposed_name)`；索引 `(tool_id)`。停用解绑保留行；Run 冻结绑定及 Tool Version。

### 5.10 ai_tool

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| code / name | VARCHAR(64) / VARCHAR(100) | 稳定业务编码、名称 |
| type | SMALLINT | 1 HTTP、2 内置 |
| current_version | BIGINT | >0，当前不可变版本号 |
| status | SMALLINT | 0/1，默认 0 |
| created_by / updated_by | BIGINT | FK → user_account |
| deleted_at | TIMESTAMPTZ NULL | 引用中只停用 |

唯一 `code WHERE deleted_at IS NULL`；索引 `(status,id) WHERE deleted_at IS NULL`。复合 FK
`(id,current_version)` → `ai_tool_version(tool_id,version)` 延迟检查。

### 5.11 ai_tool_version

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| tool_id / version | BIGINT / BIGINT | FK → ai_tool；version>0 |
| type | SMALLINT | 冻结的 1 HTTP / 2 内置 |
| description | TEXT | 模型可见说明，最大 16 KiB |
| input_schema / output_schema | JSONB / JSONB | object，受限 JSON Schema，不允许外部 $ref |
| builtin_key | VARCHAR(64) NULL | 内置必填，仅 currentTime / calculator |
| http_definition | JSONB NULL | HTTP 必填，固定 URL/方法/映射/域名策略 |
| secret_ciphertext / encryption_key_id | TEXT NULL / VARCHAR(64) NULL | 秘密密文及密钥版本，有密文则必填 key id |
| timeout_ms | INTEGER | 100～30000，默认 10000 |
| output_limit_bytes | INTEGER | 1～65536，默认 65536 |
| has_side_effect | SMALLINT | Yes/No |
| idempotency_mode | SMALLINT | 0 无副作用、1 无幂等保证、2 下游幂等键、3 具名结果查询 |
| created_by | BIGINT | FK → user_account |

唯一 `(tool_id,version)`；HTTP/内置字段互斥 CHECK，副作用与幂等模式 CHECK。创建后不可变；
Run 固定 version id，不能运行时读取 Tool 当前版本代替。

### 5.12 ai_conversation

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| user_id / platform_id | BIGINT / BIGINT | FK → user_account / permission_auth_platform |
| agent_id | BIGINT | FK → ai_agent，创建后固定 |
| title | VARCHAR(200) | 会话标题，默认后端本地化“新对话” |
| last_message_sequence | BIGINT | >=0，行锁内分配序号 |
| deleted_at | TIMESTAMPTZ NULL | 本人隐藏，不级联删除 Run/账单/消息 |

索引 `(platform_id,user_id,updated_at DESC,id DESC) WHERE deleted_at IS NULL`、`(agent_id)`。同一会话
只允许一个未结束文字 Run；多个会话仍受钱包在途规则限制，不设置全用户对话串行锁。

### 5.13 ai_message

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| conversation_id | BIGINT | FK → ai_conversation |
| run_id | BIGINT NULL | FK → ai_run；会话展示草稿可空 |
| sequence | BIGINT | >0，会话内递增 |
| role | SMALLINT | 1 用户、2 助手、3 工具；客户端不能伪造 system role |
| content | JSONB | array，具名文本/文件引用/工具结果块，不存媒体本体 |
| status | SMALLINT | 1 完整、2 部分、3 失败 |
| provider_item_id / tool_call_id | VARCHAR(200) NULL / VARCHAR(200) NULL | 上游输出项/工具标识 |

唯一 `(conversation_id,sequence)`；索引 `(run_id,sequence)`。系统提示词用 Run 快照，不读取用户伪造系统消息。
上下文仅来自本人当前会话已完成消息，按模型输入预算保留完整尾部消息，不做摘要记忆或 RAG；截断时界面提示。

### 5.14 ai_run

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| user_id / platform_id | BIGINT / BIGINT | FK →实际操作者/来源平台，创建后不可改 |
| request_key | UUID | 客户端稳定幂等键 |
| request_fingerprint | CHAR(64) | 规范化客户端业务输入 SHA-256 |
| conversation_id | BIGINT NULL | FK → ai_conversation，媒体可空 |
| agent_id / binding_id | BIGINT / BIGINT | FK → ai_agent / ai_agent_model_binding |
| provider_model_id / provider_version_id | BIGINT / BIGINT | FK → ai_provider_model / ai_provider_version |
| official_model_id / price_version_id | BIGINT / BIGINT | FK →标准模型/ai_provider_model_price_version 中已发布版本 |
| operation | VARCHAR(32) | 第 6.1 节操作值 |
| execution_status | SMALLINT | 第 6.2 节运行状态 |
| usage_status | SMALLINT | 0 尚未消费、1 已完整确认、2 存在未知用量 |
| billing_status | SMALLINT | 第 7 节账单状态 |
| transfer_status | SMALLINT | 0 无媒体、1 待保存、2 保存中、3 全部就绪、4 部分失败、5 全部失败 |
| input_snapshot | JSONB | object，规范化输入、参数与文件引用，无秘密/二进制 |
| config_snapshot | JSONB | object，完整配置 generation、Agent/绑定/模型/协议/提示词/Tool Version/价格快照 |
| policy_snapshot | JSONB | object，总时限、工具与在途规则快照 |
| multiplier_ppm | BIGINT | >0，冻结 Agent 倍率 |
| estimated_units | BIGINT | >=0，在途估算，不是预扣金额 |
| reservation_status | SMALLINT | 0 无、1 在途保留、2 已释放 |
| state_version | BIGINT | >0，Worker CAS/fencing |
| next_event_sequence | BIGINT | >=0，事务分配事件序号 |
| lease_token | UUID NULL | 执行租约身份 |
| lease_until / heartbeat_at | TIMESTAMPTZ NULL / TIMESTAMPTZ NULL | 租约截止与心跳 |
| accepted_at / deadline_at | TIMESTAMPTZ / TIMESTAMPTZ | 准入时间与总截止时间 |
| started_at / finished_at | TIMESTAMPTZ NULL / TIMESTAMPTZ NULL | 执行起止 |
| cancel_requested_at | TIMESTAMPTZ NULL | 本人停止请求时间，不等于上游已取消 |
| error_code / error_message | VARCHAR(64) NULL / VARCHAR(1000) NULL | 稳定公开错误及脱敏说明 |

唯一 `(platform_id,user_id,request_key)`；同 key 异指纹返回冲突。复合 FK `(agent_id,binding_id)` →
`ai_agent_model_binding(agent_id,id)`；复合 FK `(provider_model_id,price_version_id)` →
`ai_provider_model_price_version(provider_model_id,id)`，禁止引用其他供应商模型的价格。绑定归属及其 Provider/标准模型/价格版本在准入事务核对。
本人历史索引 `(platform_id,user_id,accepted_at DESC,id DESC)`；管理列表 `(accepted_at DESC,id DESC)`、
`(agent_id,accepted_at DESC)`、`(execution_status,accepted_at)`。钱包在途索引
`(user_id,id) INCLUDE(estimated_units,usage_status) WHERE reservation_status=1`。
同会话唯一 `(conversation_id) WHERE conversation_id IS NOT NULL AND execution_status IN(0,1,2)`。
不建立到短期 Scheduler Job 历史行的强外键，持久调度使用稳定 source_key，避免其清理破坏 Run 账务历史。
到期执行租约恢复索引 `(lease_until,id) WHERE execution_status IN(1,2) AND lease_until IS NOT NULL`；
queued 超时恢复使用已有 `(execution_status,accepted_at)` 索引，不扫描全部历史 Run。

### 5.15 ai_provider_request

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| run_id / sequence | BIGINT / INTEGER | FK → ai_run；sequence>0，本 Run 第几轮模型请求 |
| request_key / request_fingerprint | UUID / CHAR(64) | 稳定请求身份与本轮规范化摘要 |
| status | SMALLINT | 0 未发送、1 发送中、2 完成、3 已知失败、4 结果不明 |
| upstream_request_id | VARCHAR(200) NULL | 上游请求/响应标识 |
| request_summary | JSONB | object，脱敏请求摘要 |
| response_items | JSONB | array，后续工具循环必要的完整上游输出项/不透明续接项，非前端展示字段 |
| usage | JSONB | object，规范化计量及原始用量审计，最大 64 KiB |
| usage_status | SMALLINT | 0 尚未发送、1 完整确认、2 未知 |
| started_at / finished_at | TIMESTAMPTZ NULL / TIMESTAMPTZ NULL | 请求时间 |
| error_code / error_message | VARCHAR(64) NULL / VARCHAR(1000) NULL | 脱敏错误 |

唯一 `(run_id,sequence)`、`request_key`；索引 `(status,updated_at) WHERE status IN(1,4)`。
发送前持久化并 CAS 到发送中；已发送/完成/未知的同轮不再次生成。上游累计用量必须转换为每轮差量后再汇总。
不存在“更换模型再试一次”的隐含语义。后文的 Request/Attempt 均指真实请求轮次，不是备用供应商。

### 5.16 ai_tool_call

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| run_id / provider_request_id | BIGINT / BIGINT | FK → Run / 发出工具请求的模型轮次 |
| tool_version_id | BIGINT | FK →冻结 Tool Version |
| call_id / exposed_name | VARCHAR(200) / VARCHAR(64) | 模型工具标识与冻结工具名 |
| arguments | JSONB | object，Schema 校验后参数，敏感项加密/脱敏 |
| result | JSONB NULL | 受限结果块，不超过冻结大小限制 |
| status | SMALLINT | 0 待执行、1 执行中、2 成功、3 已知失败、4 结果不明 |
| idempotency_key | UUID | 稳定副作用身份，Worker 重投不改变 |
| execution_attempts | INTEGER | >=0，恢复/结果查询次数，不是额外模型次数费 |
| started_at / finished_at | TIMESTAMPTZ NULL / TIMESTAMPTZ NULL | 执行时间 |
| error_code / error_message | VARCHAR(64) NULL / VARCHAR(1000) NULL | 脱敏错误 |

唯一 `(run_id,call_id)`、`idempotency_key`；索引 `(provider_request_id,id)`、
`(status,updated_at) WHERE status IN(1,4)`。同身份异参数返回冲突，不执行第二次。逻辑 call_id 与执行次数分开。

### 5.17 ai_run_event

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| run_id / sequence | BIGINT / BIGINT | FK → Run；sequence>0，Run 内严格递增 |
| event_type | VARCHAR(64) | queued/started/textDelta/toolStarted/toolFinished/mediaUpdated/finished/billingUpdated 等具名枚举 |
| payload | JSONB | object，最大 64 KiB，脱敏本人事件 |
| published_at | TIMESTAMPTZ NULL | 已持久交付现有 realtime_event/outbox 的时间，不代表浏览器收到 |

唯一 `(run_id,sequence)`；待发布索引 `(id) WHERE published_at IS NULL`；清理索引 `(created_at,id)`。
这是 Run 业务事件，不是另一套 Scheduler Job；以 run+sequence 为 dedup_key，在写事件的同一短事务经窄接口调用
现有 realtime.Repository.AppendTx 并标记 published_at，Redis 通知交给原有 relay。Run sequence 与 realtime 全局 sequence
是不同坐标，前端按 Run sequence 去重/补拉。文本按不小于 250ms 或
8 KiB 合批写入，不逐 token 写 PostgreSQL。

### 5.18 ai_media_task

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| run_id / provider_request_id | BIGINT / BIGINT | FK → Run / 上游生成或提交请求 |
| media_kind | SMALLINT | 1 图片、2 视频、3 音频 |
| upstream_task_id | VARCHAR(200) NULL | 异步任务 ID，同步图片/音频可空 |
| status | SMALLINT | 0 待提交、1 处理中、2 成功、3 失败、4 已取消、5 结果不明 |
| progress | SMALLINT NULL | 0～100；无上游进度则空，不伪造百分比 |
| poll_attempts | INTEGER | >=0 |
| next_poll_at | TIMESTAMPTZ NULL | 下一次允许查询时间 |
| deadline_at | TIMESTAMPTZ | 冻结截止时间 |
| upstream_result | JSONB | object，仅元数据，不存 base64/二进制 |
| error_code / error_message | VARCHAR(64) NULL / VARCHAR(1000) NULL | 脱敏错误 |

唯一 `run_id`；索引 `(next_poll_at,id) WHERE status=1`。调度发布和租约复用 Scheduler。已提交但未获得任务 ID
的未知结果不能通过重新提交恢复。轮询失败可退避，模型生成提交本身不自动重做。

### 5.19 ai_media_file

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| run_id / media_task_id | BIGINT / BIGINT NULL | FK → Run / 媒体任务，文字输入图片可无媒体任务 |
| role / ordinal | SMALLINT / INTEGER | 1 输入、2 输出；同 role 下 ordinal>=0 |
| storage_file_id | BIGINT NULL | FK → storage_file，转存就绪必填 |
| import_request_key | UUID NULL | 仅输出必填的稳定存储导入身份；现有文件输入不冒称导入 |
| source_kind | SMALLINT | 0 现有 storage_file、1 URL、2 本次响应字节流、3 可下载上游资源 |
| source_ciphertext / encryption_key_id | TEXT NULL / VARCHAR(64) NULL | 临时定位密文/密钥版本，不存媒体本体 |
| source_recoverable | SMALLINT | Yes/No，是否还能独立获取原结果 |
| source_expires_at | TIMESTAMPTZ NULL | 已知上游失效时间 |
| transfer_deadline_at | TIMESTAMPTZ NULL | 输出必填，获得结果后独立冻结的保存截止时间，不使用 Run 生成截止 |
| mime_type / size_bytes | VARCHAR(100) NULL / BIGINT NULL | 校验后真实 MIME/大小，大小>=0 |
| metadata | JSONB | object，尺寸/时长/声道等 |
| transfer_status | SMALLINT | 0 待保存、1 保存中、2 就绪、3 可重试失败、4 不可恢复失败 |
| transfer_attempts | INTEGER | >=0 |
| next_transfer_at | TIMESTAMPTZ NULL | 退避保存时间 |
| error_code / error_message | VARCHAR(64) NULL / VARCHAR(1000) NULL | 保存/校验错误 |

唯一 `(run_id,role,ordinal)`、`import_request_key WHERE import_request_key IS NOT NULL`；索引 `(storage_file_id)`、
`(next_transfer_at,id) WHERE transfer_status=3`。就绪要求有效 storage_file，物理对象坐标以该文件表为准；
不重复存 COS 凭据或签名 URL。输入附件同样验证本人、平台、用途和生命周期。
CHECK：role=1 时 source_kind=0、storage_file_id 非空、transfer_status=2、import_request_key/上游定位字段为空；
role=2 时 source_kind IN(1,2,3)、import_request_key 与 transfer_deadline_at 非空。输出就绪要求关联文件 ready。

### 5.20 ai_usage_charge

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| run_id | BIGINT | FK → ai_run，唯一，每 Run 一个汇总结算 |
| user_id / platform_id | BIGINT / BIGINT | FK →用户/平台，必须与 Run 一致 |
| status | SMALLINT | 0 待用量、1 待扣款、2 已结算、3 待重试、4 待核对 |
| currency | CHAR(3) | 固定 CNY |
| price_version_id | BIGINT | FK → ai_provider_model_price_version，与 Run 冻结的供应商模型价格版本一致 |
| multiplier_ppm | BIGINT | >0，冻结倍率 |
| actual_units | BIGINT NULL | >=0；NULL 是未知，不能用 0 替代 |
| usage_complete | SMALLINT | Yes/No，只有 1 可进入扣款 |
| snapshot_digest | CHAR(64) | 明细/用量/费率/倍率 SHA-256 |
| settlement_attempts | INTEGER | >=0 |
| next_settle_at | TIMESTAMPTZ NULL | 退避恢复时间 |
| settled_at | TIMESTAMPTZ NULL | 结算成功必填 |
| error_code / error_message | VARCHAR(64) NULL / VARCHAR(1000) NULL | 账务依赖错误 |

唯一 `run_id`；索引 `(user_id,created_at DESC)`、`(next_settle_at,id) WHERE status IN(1,3)`。
结算后快照与费用不可改；资金流水从 ai_usage_charge_id 反向唯一引用，避免双向外键循环。零费用可以无流水。

### 5.21 ai_usage_charge_item

| 字段 | 类型 | 含义/约束 |
| --- | --- | --- |
| charge_id / provider_request_id | BIGINT / BIGINT | FK → Charge / 实际模型请求 |
| price_rate_id | BIGINT | FK → ai_provider_model_price_rate，与 Run 的价格版本一致 |
| item_key | VARCHAR(200) | 稳定 operation+usage_key+selector 身份 |
| operation / usage_key | VARCHAR(32) / VARCHAR(64) | 操作及计量维度 |
| selector_snapshot | JSONB | object，实际尺寸/质量等匹配条件 |
| quantity / unit_quantity | NUMERIC(24,8) / NUMERIC(24,8) | 实际数量>=0，计价数量>0 |
| unit | VARCHAR(16) | 冻结单位 |
| billing_unit_price_units / multiplier_ppm | BIGINT / BIGINT | 冻结内部人民币售价>=0、倍率>0，美元参考价不参与扣款 |
| amount_units | BIGINT | >=0，最终分配到此项的整数 units |
| raw_usage_digest | CHAR(64) | 原计量事实 SHA-256 |

唯一 `(charge_id,provider_request_id,item_key)`；索引 `(provider_request_id)`。用量完整后固化明细；结算事务验证
`SUM(amount_units)=charge.actual_units`。已结算明细不可增删改。

### 5.22 修改 payment_transaction，钱包不另建表

保留原 id、transaction_no、user_id、platform_id、amount_units、balance_after_units、order_id、redeem_code_id、
created_at、updated_at 以及原有唯一索引/外键；新增：

| 新增字段 | 类型 | 约束/回填 |
| --- | --- | --- |
| direction | SMALLINT | NOT NULL，1 收入、2 消费；存量回填 1 |
| balance_before_units | BIGINT | NOT NULL，存量使用 numeric 校验后按 after-amount 回填 |
| ai_usage_charge_id | BIGINT NULL | FK → ai_usage_charge，ON DELETE RESTRICT |

1. 保留 `amount_units>0`。来源改为 `num_nonnulls(order_id,redeem_code_id,ai_usage_charge_id)=1`。
2. 订单/兑换码必须 direction=1，AI charge 必须 direction=2；禁止负数 amount 或伪造充值订单。
3. 收入满足 `after::numeric=before::numeric+amount::numeric`，消费满足相减；before/after 为 bigint，
   CHECK 用 numeric 避免表达式溢出，替换原来仅适合充值的 ck_payment_transaction_balance。
4. 新增 `UNIQUE(ai_usage_charge_id) WHERE ai_usage_charge_id IS NOT NULL`，保留原交易号/订单/兑换码唯一保护。
5. 保留不可 UPDATE/DELETE/TRUNCATE 触发器；存量回填在维护窗口的审计 migration 内临时处理触发器并同事务恢复，
   不永久关闭保护。超范围或非法旧来源直接拒绝迁移，不猜测修账。
6. 同步交易 DTO、来源/方向选项、翻译、列表过滤与订单/兑换码入账写入；不再对消费按 after-amount 推算扣前余额。
7. 钱包新增具名 DebitTx：只接受正数 amount，检查 int64 下溢，但不要求 before>=amount；允许已准入请求据实扣负。

`payment_wallet` 仍只有 user_id、balance_units、created_at、updated_at，用户跨平台一个钱包；本期不加 frozen_balance
或第二钱包。在途估算由 Run 事实表达，不等于冻结/预扣钱包余额。

## 6. 统一 Run 与状态流程

### 6.1 固定操作与协议

| operation | 工作台 | 上游能力契约 |
| --- | --- | --- |
| text.generate | 文字 | 当前绑定的 Responses 或显式 Chat Completions，支持时携带 Tool Schema |
| image.generate | 图片 | Images 生成；文本提示词、数量及受支持参数 |
| image.edit | 图片 | 仅声明支持时启用 Images 编辑；输入引用须有权读取 |
| video.generate | 视频 | 具名异步视频协议，提交后持久化任务 ID，再查询和取回输出 |
| audio.speech | 音频 | 文本转语音，使用 Audio speech 接口 |
| audio.transcribe | 音频 | 音频转写，使用 Audio transcription 接口 |

首期视频实现具名 `openaiVideoV1` 协议契约：`POST /videos` 提交、`GET /videos/:id` 查询、
`GET /videos/:id/content` 下载；以供应商文档/真实能力核验通过为启用前提，不把此路径作为任何兼容站已支持的保证。
不支持此契约的视频供应商不能通过填写任意 JSON 脚本伪装成可用，须增加具名实现并补测试；不引入通用 Adapter/Factory。
当前 OpenAI 官方视频/Sora 服务已停用，因此本期不发布 `video.generate` 价格，也不允许新 Run 创建该操作；保留协议客户端与
`ai_media_task` 轮询/转存事实，仅用于后续具名供应商接入和既有任务恢复，不把视频能力伪装成可用。
音频第一期不做 Realtime 双向语音或音频翻译；四媒体范围不等于承诺上游所有衍生端点。

### 6.2 三个独立状态轴

Run `execution_status`：0 queued、1 running、2 waitingUpstream、3 succeeded、4 failed、5 cancelled。
路径为 queued→running→终态，异步视频可 running→waitingUpstream→终态；终态不可重新变回 running。
模型调用失败即 failed，不自动切换/重试生成；传输失败且结果不明时仍以 failed 表示执行失败，usage/billing 单独待核对。

`billing_status` 与 Charge status 一致：0 pendingUsage、1 pendingDebit、2 settled、3 retryPending、4 reconciliationRequired。
`transfer_status` 独立为无媒体/待保存/保存中/全部就绪/部分失败/全部失败。
因此可能出现「生成成功 + 扣费已完成 + 视频保存失败」，界面必须如实展示，不能改成免费失败或再次请求生成。

### 6.3 创建到完成

1. Handler 绑定并校验 DTO，传当前身份和 Request.Context 到 Service；客户端不能传计费用户、真实上游模型或费率。
2. 优先查本人 request_key：同指纹返回同一 Run；异指纹返回 409。已存在 Run 不因 Agent 后续变更而重新准入或执行。
3. 从有版本 Redis 快照确认 Agent 当前绑定、协议、模型、提示词、Tool Version、价格、倍率、输入限制和运行策略；不混读不同 generation。
4. 校验输入文件所有权/用途、操作能力、完整价格和消息上下文；不支持的操作明确拒绝，不改走其他绑定。
5. 在短事务中锁本人钱包、核对在途准入并插入 Run/输入消息/必要媒体引用/初始事件，同时写现有 Scheduler durable Job。
6. 事务成功返回 202 和 Run ID、queued 状态、订阅坐标；Redis 队列暂不可投递时 durable Job 仍在 PostgreSQL，状态不冒充 started。
7. Worker 通过 Scheduler 的发布/租约/重投递保护取得任务，再以 Run/Request 状态机防止重复上游消费。
8. 发请求前创建 ai_provider_request，发送前持久化发送中；每轮用同一 Run 绑定。文字输出合批持久化事件，最终消息单独落库。
9. 遇工具请求执行第 8 节闭环；媒体结果创建媒体关联，异步任务持续查询但不再次提交生成。
10. Run 达到执行终态后，汇总全部已确认用量。完整则固化 Charge/Items 并启动结算；未知则进入待核对，不能按 0 结算。
11. 转存和结算各自通过独立恢复任务执行。刷新浏览器只补拉事实和事件，不创建新的生成。

同一会话的并发回复由数据库唯一约束拒绝并返回明确“当前会话正在生成”；请求幂等键不包括服务器端当前配置，
避免管理员切换后相同请求键被误认成新消费。快照内部另保存本次实际生效配置的 digest。

### 6.4 停止与未知提交

- 本人取消写 cancel_requested_at；Worker 在发新轮次/工具前及心跳时检查。尚未发送的 queued Run 可直接取消并零费用结束。
- 已发送同步请求尝试取消相应 I/O；已提交视频仅在供应商明确支持取消时调用具名取消能力，否则等待终态/截止并标明无法保证停止。
- 浏览器断连只断实时运输，不代表取消持久任务。上游已完成或取消太迟时保留真实完成与费用，不倒改成未消费。
- HTTP 超时、连接中断、Worker 在发送后崩溃，若不能证明未提交，均记录未知；即使 Scheduler 人工重试也不能再次生成。
- 有可信上游请求/任务 ID 且供应商支持查询时执行只读核对；没有查询证据就保持待核对。交付具名维护命令
  `ai-usage-reconcile`：默认 dry-run，只接收 Run ID 和可追溯用量证据，不接受目标扣费用户或任意扣款金额；
  明确 apply 后仍调用同一用量/结算服务，核对请求身份和费率快照并持久记录证据摘要/操作者审计。不是公开管理 API。

## 7. 准入、定价、计费与负余额

### 7.1 金额与费率

- 沿用 `shared/money`：**1 RMB = 100000000 units**，钱包和资金流水始终使用 checked int64；对外金额用精确十进制字符串。
- 内部费率以 `billing_unit_price_units / unit_quantity` 表达；每轮实际数量与冻结倍率用 `math/big` 精确有理数计算，不使用 float。
- Agent 倍率为 multiplier_ppm/1000000。计算各条未取整金额后先求 Run 总和，最后仅向上取整一次到 1 unit。
- 明细先向下取整数 units，剩余 units 按小数余数从大到小分配，余数相同按稳定 item_key 排序；确保分项之和等于总额，
  不逐 token、逐轮向上取整放大账单。全程检查 bigint 范围，溢出保留待结算错误，不截断金额。
- 供应商美元参考价与系统内部人民币售价同时保留在价格项快照中；美元只用于展示、审计和与供应商同步对比，不参与真实汇率换算。
  首次同步时同一计价单位下的人民币初始值复制美元数值，后续由管理员按业务需要维护人民币售价。人民币售价按 `shared/money`
  转为整数 units，发布后历史 Run 不重新定价。
- 缓存输入与普通输入不能重复计费：若上游 inputToken 已含 cachedInputToken，普通输入计量取两者差值；若上游本身分开，则直接用分项。
- 图片按真实质量/尺寸 selector 的人民币单张售价 × 实际生成张数 × Agent 倍率计费；同次图片生成不再叠加图片 token 费用。
  张数取上游确认生成数量，不取提示词要求张数或 COS 转存成功张数；转存失败不抹掉已经发生的生成消费。
- 音视频按真实 selector 和实际完成数量/时长或有效 usage 定价；不得用提示词估算代替最终用量。供应商不提供可靠计量
  且不能按已确认固定维度确定费用时，此操作不能上线收费，不凭经验伪造价格或用量。

### 7.2 价格维护与“同步”的实际边界

首期同步来源改为**供应商实际提供且可追溯的模型报价数据**。模型目录同步与价格同步是独立操作；普通模型列表不能视为已经包含报价。
价格同步使用具名供应商实现和其实际支持的报价接口/官方价格源，按供应商返回的
operation/usageKey/selector/unit/unitQuantity/单位美元价格规范化并保存只读来源；不抓取任意第三方网页、不使用真实汇率，也不允许
管理员改写美元参考价。首期 OpenAI 的 `/v1/models` 只用于模型目录，不当作价格接口；价格适配器绑定官方
`https://developers.openai.com/api/docs/pricing.md`（及模型详情页中同源的计价表）作为可版本化价格源，并保存
source_digest/source_payload。若官方价格源不可访问、结构不完整或无法与模型/计价维度稳定匹配，同步明确失败，不能猜价、沿用旧价冒充新同步，
也不能把自定义 Base URL 的兼容供应商当成 OpenAI 报价。供应商没有提供可靠价格或返回结构不完整时，相关计费项不能进入可消费状态。

OpenAI 来源适配器只获取固定官方文档，不携带供应商 API Key；总超时 20 秒、响应上限 2 MiB，重定向仍需同源校验。
source_payload 记录来源 URL、抓取时间、parserVersion、原始计价性质与必要的原文表片段，source_digest 对实际获取文档做 SHA-256。
Standard/Batch/Flex/Fast、上下文档位、缓存写入策略等计费条件必须保留并与实际请求相匹配；不通过只匹配模型名取到错误档位。
首期未实现或无法可靠计量的计费条件明确阻止相应能力启用，不静默丢弃该条件后宣称价格完整。
图片原文若明确标注为估算每张，则只读保留“估算参考”性质，并在人民币预览中明确展示；若只有 token 单价且没有可追溯的每张报价，
不凭经验造一个官方 USD/张。内部图片仍按质量/尺寸每张收费，不把参考美元价冒充实际供应商扣费。

新计费项首次同步时，内部人民币售价默认复制同计价单位美元数值，作为管理页初始值；管理员可以在人民币价格预览中调整，
并确认生成新的生效版本。再次同步按 operation/usage_key/selector/unit/unit_quantity 的完整计价身份匹配：既有项保留当前已生效人民币售价，
单位或计价数量变化不能继承旧的人民币数值，必须作为新项生成预览并重新确认，避免每千 token 的旧价误当每百万 token。
新增项才复制美元数值为人民币初价。同步只生成预览、不立即改变扣费；失败明确返回，不破坏上一已生效版本。
管理员确认人民币售价和 effective_at 后发布，后端核对 source_digest 与 snapshot_digest；原版本和历史 Run 保持不变。
同一价格版本可同时保存输入/缓存/输出 token、图片按质量/尺寸计次、音视频时长等维度；缺失、冲突或非法计费项阻止发布，
没有完整生效价格的操作不可消费。系统不新增充值汇率或美元余额，充值入账仍以支付域既有人民币金额为准。

下表数值仅为计算示例，不是任何模型的真实报价或 seed：

| 计费项 | 供应商只读参考价 | 首次人民币初价 | 管理员修改后 | Agent 倍率 | 最终收费 |
| --- | --- | --- | --- | --- | --- |
| 输入 token | 5 USD/M token | 5 RMB/M token | 8 RMB/M token | 1.5 | 12 RMB/M token |
| 图片某质量/尺寸 | 0.1 USD/张 | 0.1 RMB/张 | 0.2 RMB/张 | 1.5 | 生成 3 张收取 0.9 RMB |

### 7.3 准入和在途：借鉴机制，不复制故障放行

为避免一套新的余额缓存/失效协议，本期直接在短 PostgreSQL 事务中处理**动态余额事实**，不把数据库当配置中心。
在途估算保存在 ai_run，钱包不冻结、不预扣；Redis 不承担余额或在途的最终事实。这样多个实例天然锁同一个用户钱包。

固定锁序与准入：

1. 校验配置/价格并得到 estimated_units；估算考虑已知输入、最大输出、数量/时长和工具轮数，不用人为小金额截断估算。
2. 锁 payment_wallet 当前用户行（不存在时在事务内创建 0 余额），只把 balance>0 的用户视为余额可准入。
3. 查询该 user_id 的 reservation_status=1 Run，索引读取在途数量与估算和；**跨平台同用户共用**。
4. 如有 usage_status=2 或请求 status=4 的未核对在途 Run，拒绝新消费，不通过过期租约释放它来继续透支。
   正常 status=1 发送中的请求不是未知提交，仍按在途估算/数量规则处理，不能因此把 2 并发退化成全部串行。
5. 对已知固定费用操作，余额必须覆盖该固定报价，存在其他在途时还需覆盖它们的估算。
6. 对变动用量操作，没有其他在途时，balance>0 即允许首笔进入，即使估算高于余额；有其他在途时要求
   `balance - SUM(在途估算) >= 本次估算`，并遵守用户在途数量限制。
7. 同一事务插入 Run 和预留事实及 Scheduler Job，不扣钱包余额。失败全部回滚；不以 Redis/模型故障放行。

预留延续到已确认结算完成或证明未发生消费；Run 已生成终态但账务待重试时仍占预留，不能提前释放。
执行租约只控制谁能执行，不决定资金预留是否作废；过期由恢复服务核对，不按时间直接清除在途金额。
单次输出/轮数/并发限制降低风险，但首笔估算超余额仍可准入，**不构成最大透支金额承诺**。

### 7.4 据实结算事务

1. 锁 Charge，验证 Run 身份、用户/平台、完整用量、明细总和、费率/倍率和 snapshot_digest。
2. 已 settled 则核对唯一流水后幂等返回；相同来源但金额/身份不同返回冲突，不补扣。
3. 锁本人钱包；amount>0 时 checked subtract，**不检查余额必须覆盖金额**，允许余额变负。
4. 插入 direction=2、来源 ai_usage_charge_id 的不可变资金流水，包含扣前/扣后余额；唯一键防重复。
5. 同事务更新钱包、Charge settled、Run billing_status 和 reservation_status=2，并记录账务事件。
6. actual_units=0 时仍结算用量事实并释放预留，但不插 0 元资金流水。
7. 扣款/数据库/下溢失败不改成 0，保留待重试事实；重试只走扣款，不再发起模型或工具。

未知用量通过 `server/cmd/ai-usage-reconcile` 提交与请求身份、冻结价格及审计摘要一致的证据；完整证据才转入结算，
部分证据保留 review 与预留。已知费用重试耗尽通过 `server/cmd/ai-usage-settle` 重试同一 Charge；默认预览，显式
`--apply` 才写入，不能覆盖用户、金额或费率。参数与用法见各命令目录的 README.md。

Run 完成写用量与最终扣款使用不同短事务；取消路径只更新 Run 标志，不在持有 Run 锁时反向等待钱包。
所有消费结算遵守 Charge→Wallet→Run 锁序，准入只锁 Wallet 后插入新 Run，统计在途不锁其他 Run，避免循环等待。
已有充值/兑换码与消费均锁同一钱包并写统一账本，保持金额守恒。

## 8. Responses 与工具调用

1. 从 Run 冻结的提示词、当前会话完整消息及已绑定 Tool Version 构造首轮请求。默认 Responses。
2. 解析模型输出项；纯文本持久化分块，`function_call` 必须按 call_id、已绑定工具名和参数 schema 验证。
3. 创建稳定 ai_tool_call 并核对重复调用身份；服务端执行具名内置函数或固定 HTTP 定义，不直接执行模型提供的 URL/代码。
4. 以 `function_call_output` 和相同 call_id 回传结果；保留续接所需的完整输出项/不透明 reasoning 项，不只拼接纯文本。
5. 后续模型请求沿用同一绑定与费率快照，各轮独立记用量；没有工具请求时得到最终回复，或在总限制/取消/模型失败时结束。

同一轮的多个工具调用按输出顺序串行执行，首期不加入并行工具编排。已知普通工具错误可作为受控错误结果回传模型；
有副作用调用结果不明时停止 Run 并核对，不盲目再执行。无副作用查询可在同次执行预算内重试一次；有副作用只在
下游明确提供稳定幂等保证或结果查询时恢复。模型产生新的 call_id 不足以证明是新业务副作用，危险操作须有业务幂等键。

HTTP 工具固定方法/目标/鉴权，限制协议、域名、每跳重定向与实际连接 IP，阻断 loopback/link-local/metadata/private
地址和 DNS rebinding；凭据只发送到被冻结的目标域，不转发到媒体下载域。无任意执行、无通用网络代理、无隐式托管 RAG。

## 9. 图片、视频、音频与 COS

### 9.1 统一媒体流程

可再次下载的上游结果：生成/提交 → 持久化结果身份 → 收集真实 usage → 标记生成终态并安排结算/转存 → 验证文件 ready → 展示。
只返回一次的字节流/base64 则在接收时有界 spool，并由同一 Worker 立即导入，不能等扣款完成后才开始接收或保存；
生成、保存与扣款状态依然独立。COS 失败不抹掉已经发生的用量，也不能再次生成以获取副本。
图片/音频可同步返回但仍由 Worker 执行；视频提交后只持久化任务 ID 并安排下次查询，不占用 API 请求等待整个生成。
轮询错误退避重查原 task，不能重新提交生成。多张图片逐一记录 ordinal 和保存状态，部分失败不抹掉已成功结果。

### 9.2 明确需要新增的存储服务能力

- 当前存储只有浏览器直传登记/完成、Head/Copy/Read 等能力，**没有现成的服务端上游结果流式转存**。本期新增具名
  `ImportGeneratedMedia` Service 和 COS 流式写入支持，由显式依赖装配，不让 AI Handler 直接碰 COS。
- 下载/解码先落**有界本地临时 spool 文件**，边读取边计数/校验，得到真实大小和 MIME 后才以 import_request_key
  登记/恢复 storage_file；现表要求 size_bytes>0、MIME 非空，不写 0/1 占位，不将 500 MiB 视频整块读入内存。
  再从 spool 流式写 COS 并 Head 校验，最后标 ready；保留 owner/platform/rule/config physical version/object key/ETag。
  单实例 spool 总配额 2 GiB、并发导入 2 个，使用受限权限目录和唯一文件名，结束/取消时 finally 清理。
  临时文件不作为跨实例恢复事实；可下载源重新拉取，纯响应字节由接收 Worker 立即导入，不能把本地路径交给另一实例任务。
- COS 配置全局共享，**上传规则才按平台隔离**。真实数据库同平台只允许一条启用规则，故在 Admin 当前规则增加
  `ai/image`、`ai/video`、`ai/audio`、`ai/input` 用途 code，不额外 seed 第二条启用规则，不改 Canvas 规则。
- AI 类型进一步收窄当前存储规则，不越过其扩展名/MIME/大小上限；缺规则或规则拒绝就失败闭合，不自动选其他平台规则。
- AI 文件引用保护需接入通用 storage/file 删除与 cleanup：仍被未删除 AI 会话或持久 Run/媒体记录引用的文件拒绝直接删除，
  提示引用来源；cleanup 也复查引用。不能只在 AI 页面隐藏删除按钮。历史文件丢失时显示 unavailable，账务仍保留。

### 9.3 转存失败与真实可恢复性

- 上游可重新下载的 URL/资源，在有效期内按原结果重试保存；失败不重新调用生成、不重复扣费。
- 字节流/base64 必须边界解码并写 COS，不将媒体本体写 PostgreSQL。没有可重新下载源时，Worker 崩溃或 COS 持续失败后
  原始结果可能不可恢复；明确标 source_recoverable=0 和保存失败，不能承诺无来源也能后台重试，更不能暗中再次生成。
- 每输出自动转存总共最多 3 次（初次 + 2 次重试，退避 5/30 秒）；到期、MIME/大小非法、源不可恢复则终止。本人 retryTransfer 仅在后端
  判定仍有源且未超业务截止时可用，每次操作幂等；不接受更换来源 URL 或重新生成。
- 独立转存预算为获得输出后 900 秒，transfer_deadline_at 取该值与已知 source_expires_at 的更早者；不复用可能已经
  临近耗尽的 Run 生成 deadline。单次下载最多 300 秒且不越过保存截止，临近生成截止成功仍有独立保存窗口。
- 下载按每跳域名/IP校验，验证 Content-Length 但不信任其值，实际读取超限立即中止；检查 MIME/文件头、时长与结果元信息。
  过期 URL 和鉴权参数不作为可公开错误或事件记录。

### 9.4 预览、播放与访问权限

当前 storage URL 只允许图片 inline，音视频会强制 attachment；本期显式扩展受限安全 MIME：
image/png、image/jpeg、image/webp、image/gif；audio/mpeg、audio/wav、audio/ogg、audio/mp4；video/mp4、video/webm。
HTML、SVG、可执行内容不以内联方式预览。通过存储授权签发短时 GET 地址，私有对象首期有效期 300 秒；签名 URL 不落长期事实。

本人输出按 user/platform+Run 所有权校验。跨用户监控的媒体读取走精确 `ai:run:detail` 授权的 AI 输出读取服务，
再验证媒体确属该 Run；不伪造 owner、不放宽通用 storage 文件 API 的本人边界。公开规则仍服从现有对象公开语义，
不能声称已公开的 Bucket URL 因页面权限变成私密；敏感 AI 输出应由维护者在现有规则选择私有访问模式。

## 10. 多实例、缓存、任务与容量

### 10.1 容量假设与查询预算

设计面向百万注册用户、多 API/Worker 实例；首轮容量验收按峰值 200 新 Run/秒、2000 个在执行 Run 和 10000 条实时连接
进行建模。这是测试目标，不是当前机器已达到的吞吐。超出部署容量以队列背压和明确限流处理，不无界扩 Worker。

| 路径 | 正常命中预算 | 失败/回源上限 |
| --- | --- | --- |
| 静态 options | 0 PostgreSQL、0 Redis | 不用数据库/字典维护协议 enum |
| 新 Run 配置/定价 | Redis generation 确认 + 单 Agent 完整快照，正常 0 PostgreSQL 配置查询 | 复用跨实例 cacheFill lease；单次 leader 总预算 4 秒，follower 500ms，超预算失败闭合 |
| 输入文件校验 | 按本次最多 4 个附件合批读取，禁止每个文件独立 N+1 | 只读本人/平台/用途，缺失立即拒绝 |
| Run 准入 | 一次钱包锁、一次有界在途查询、同事务写 Run/消息/事件/Scheduler Job | 事务不执行 HTTP/Redis/COS；锁等待 1 秒、语句 2 秒、总事务 3 秒 |
| 生成中 | 不回源读取可变配置；每 Run 最快 250ms 合批文本事件 | 执行租约 45 秒、每 15 秒续约；失去租约停止新副作用 |
| 管理普通列表 | count+page 两条索引查询，默认 20/最大 100 | 仅配置小表；超时明确失败 |
| Run/消息/事件历史 | 游标分页，limit+1，按时间/ID/sequence 稳定排序 | 不承诺昂贵的全量 total；时间范围最大 31 天，查询 2 秒超时 |
| Run 汇总 | 默认最近 1 小时、最大 24 小时；有界只读 projection | 先限制最多 100000 条候选，超出返回范围过大，不把部分数据冒充全局统计；同条件缓存 5 秒、分布式全局最多 1 次重建/秒 |
| 结算 | 一个 Charge 短事务，锁 wallet 并写唯一 ledger | 只重试资金事务，不重做模型；冲突和下溢明确可见 |

Run 事实补充索引 `(provider_model_id,accepted_at DESC)` 支持上游筛选；筛选不能省略时间条件。摘要统计不扫描消息或原始
JSON 大字段。没有首期聚合表时就明确查询上限，不假定百万用户场景可以无限 COUNT/GROUP BY。

### 10.2 配置与价格缓存

- 复用现有 config generation/outbox 协议，具名 namespace `ai.runtime`、scope `global`，variant 按 `agent:<id>`
  和不可变凭据/工具版本区分。单 Agent runtime 快照只包含当前绑定和必要工具/定价，不包含整个系统目录。
- 每个快照最大 256 KiB、TTL 最长 300 秒；超容量不能开启 Agent，不能无限缓存。Provider/Tool 凭据在 Redis 中仍为密文。
- Runtime 变更与 generation/outbox 同事务更新；普通编辑 last-write-wins。发布/失效失败保留 durable outbox 并明确状态，
  不返回旧进程缓存或假更新。读操作先确认 Redis ready/version，再读相应 snapshot，不能只看本地 TTL。
- Redis missing 走现有有界冷回源；Redis error/corrupt/invalidating 失败闭合，不退回每请求 PostgreSQL 或过期本地配置。
- 快照覆盖完整当前依赖；前后两次 generation 不一致就重读一次，仍不稳定则返回依赖错误，不拼接新 prompt+旧模型。
- 价格快照记录当前版本及 nextEffectiveAt；TTL 不越过已知的下一生效时刻，到期按同一冷填充协议重建。未来版本不得提前收费。
- Run accepted_at 对应配置快照被完整确认的服务端时间；该时间与价格选择一起冻结。切换已经提交并发布后创建的新 Run
  必须读到新完整快照，执行中的旧 Run 继续用自己快照。
- 本期钱包没有新增余额缓存，不存在只失效 AI 扣款却遗漏充值入账的问题。展示余额读取既有支付 API，不成为准入事实。

### 10.3 工程初始值与硬限制

下表为本设计明确选择的初始工程值，后续由显式维护的 `system_setting` 键和后端 constraints 输出；migration seed，
缺失/非法不在运行时猜默认。不是模型报价或承诺的最大负余额。每个 Run 冻结实际使用的 policy。

| 设置键/范围 | 初始值 |
| --- | --- |
| ai.run.user_inflight_limit | 2，跨平台同一用户合计 |
| ai.run.user_rate_per_second / user_rate_burst | 1 / 5，分布式 token bucket |
| ai.run.global_rate_per_second / global_rate_burst | 200 / 400，跨实例共享 token bucket |
| ai.run.queued_backpressure_threshold / active_backpressure_threshold | 1000 / 2000，1 秒有界观测，非资金准入信号量 |
| ai.run.max_queued_seconds | 60，尚未发送超时直接失败并按已知零用量结束 |
| ai.run.text_timeout_seconds | 180 |
| ai.run.image_timeout_seconds | 300 |
| ai.run.audio_timeout_seconds | 300 |
| ai.run.video_timeout_seconds | 3600 |
| ai.run.max_model_rounds | 8 |
| ai.run.max_tool_calls | 16 |
| ai.run.max_bound_tools | 16，另受快照总大小限制 |
| ai.run.max_input_attachments | 4 |
| ai.run.max_output_tokens | 4096，且不超过当前模型限制 |
| ai.run.max_text_input_bytes | 131072，另经当前模型 token/上下文限制校验 |
| ai.run.max_images | 4，且不超过供应商支持数量 |
| ai.media.image_max_bytes | 10485760（10 MiB） |
| ai.media.audio_max_bytes | 104857600（100 MiB） |
| ai.media.video_max_bytes | 524288000（500 MiB） |
| ai.media.download_timeout_seconds | 300，每次受剩余任务时限约束 |
| ai.media.transfer_timeout_seconds | 900，独立于生成时限，另受源失效时间约束 |
| ai.media.spool_max_bytes_per_worker | 2147483648（2 GiB）；单实例导入并发 2 |
| ai.run.event_retention_days | 7，只清终态、已结算、已发布事件 |

单次 HTTP 工具默认 10 秒、最大 30 秒，输出最大 64 KiB；输入 JSON 请求最大 1 MiB，提示词/附件规则仍单独校验。
音视频请求的秒数、图像尺寸/质量、音色等由启用的 Provider Model 声明并被标准价格覆盖，不虚构统一时长或通用尺寸。
有效文件上限取 AI 限制与平台上传规则的较小值。SDK/HTTP 客户端不自动重试生成 POST。达到模型轮次或工具次数上限时
Run 失败并据实保留费用，不在上限之外再请求一轮“总结”。

新消费在幂等已存在检查后经过 Redis Lua 用户/全局 token bucket，超限返回 429+retryAfter；Redis 不可确认则 503，
不故障放行。队列/活跃量由单个跨实例 lease leader 每秒一次通过 Run 状态索引有界统计，发布 2 秒有效的容量观测；
follower 不逐请求查询 PostgreSQL，缺失/过期/错误拒绝新准入。达到阈值停止新 Run，最多容许观测延迟期间一个全局 burst
的额外负载，明确不是精确全局在途硬上限。恢复扫描将 queued 超过 60 秒且未发送的 Run 结束，释放预留；
已发送/未知结果不能套用 queued 规则归零。限流/容量 gate 不代替钱包事务和跨平台个人在途检查。

### 10.4 Scheduler/Worker 与恢复

复用 `system/scheduler.Repository.CreateJobTx` 及已有具名 BatchJobWriter 模式，新增窄的 AI Job 写入接口，
由 Repository 在 Run/用量/媒体事实事务中调用；Service 不直接写 GORM，Queue 仍不导入业务模块。
Scheduler 静态 task catalog 和 API/Worker 装配显式增加以下任务，不引入动态注册器：

现有 Scheduler Executor 的 Job lease 为 5 分钟且无心跳，**本期必须新增 token 条件续约**，不能只续 AI Run 的
45 秒 lease。生成执行阶段每 15 秒续 Run lease、长任务每 60 秒续所属 Scheduler Job 的 5 分钟 lease，任一续约失败或
所有权丢失立即停止新模型/工具/COS 副作用，并使旧 token 的 Complete/Fail 无效。TaskDefinition.Timeout 覆盖本步骤
实际预算：生成使用 generation deadline，原始字节就地保存使用独立 transfer deadline，外层至多两者预算加 30 秒收尾；
异步视频每次只做短提交/查询，不让一个 Job 睡满一小时。新增心跳要对现有非 AI 任务做回归测试。
生成终态后的保存阶段不再续 Run 执行 lease，仅由当前保存步骤的 Scheduler Job token 保护，避免把已完成 Run 误判为丢失。

| 任务类型 | 工作与幂等边界 |
| --- | --- |
| ai.run.execute | 一个 Run 的生成入口；MaxAttempts=1，稳定 source_key；额外用 Request 状态机抵御人工 Scheduler retry |
| ai.media.poll | 只查询已提交任务并安排下次查询；无再次生成权限，按 task+poll 序号去重 |
| ai.media.import | 只导入已确定输出，按 import_request_key 幂等，自动最多 3 次 |
| ai.usage.settle | 只执行已固化 Charge 扣款，唯一 ledger 防双扣；自动最多 10 次，仍失败则可见待恢复 |
| ai.run.recover | 每分钟扫描最多 100 个到期租约/未知结果，单次 20 秒、最多 8 并发；只核对/结束/恢复幂等步骤 |
| ai.run.event.cleanup | 每日按 1000 行批次清理符合保留条件的事件，不删除 Run/Charge/交易 |

结算重试初始 5 秒、指数退避到最多 300 秒；10 次耗尽后保留 retryPending，维护者修复依赖后使用同一具名结算服务
恢复，不从 Scheduler 重新执行生成。Job 的稳定 source_key 必须带业务身份和逻辑步骤，不能每次换随机键逃过幂等。
恢复任务还按现有索引领取到期 next_poll_at、next_transfer_at、next_settle_at 及未启动 queued Run，各类最多 100 条、
总 20 秒预算；缺失运输补投同一逻辑步骤，已耗尽的结算步骤保留错误并由具名恢复命令重开同一结算，不重开生成。
只有领取到新业务步骤序号时才创建新 source_key，重复扫描同一步返回原 Job，不能用新随机键制造无限重复。

Run 事件与 realtime.AppendTx/outbox 在同一 PostgreSQL 短事务持久化，published_at 只表示已进入现有运输系统，
不是到达浏览器；通过窄 Repository 事务桥，不调用假定存在的通用 Service Publish。Redis 发送/重试由原有 realtime relay 负责。
按 2000×4=8000 事件/秒及一个 burst 的额外负载，现有 relay 需具备每批最多 200、跨实例有界总预算 16000 事件/秒
的能力（需要实际改造/压测，不声称现代码已达标）；60 秒故障恢复按最多 576000 条积压、持续新增 9600 条/秒，
净追平至少 6400 条/秒、120 秒内恢复验收。积压超过 100000 条时文本合批窗口提高到 1 秒，显示运输延迟并背压新 Run，
不丢最终文本或无界堆内存。只复用一个 realtime 交付系统，不新增 AI publisher/outbox。所有后台循环从生命周期 context 派生有界工作上下文。

首期保留 Run、消息、媒体引用、价格版本、用量和资金流水，不默认自动删这些事实；仅事件分块按 7 天策略清理。
过期事件补拉返回空事件页、`resetReason: "EVENTS_EXPIRED"` 与当前 `nextSequence`，客户端清除暂存文本并以持久消息/Run
结果恢复；不得伪装为从未生成。对账和资金流水永不通过通用
Scheduler 历史清理删除。文件历史仍被业务引用时不能被通用 cleanup 误删。

## 11. 后端/前端与共享模块改动范围

### 11.1 新模块及依赖方向

普通入口保持 router→middleware→handler→service→repository→model→PostgreSQL。后台任务保持
task handler→service→repository。主线模块按第 4.1 节声明的业务身份落目录，复合目录名用 lower camel case，Go package
名用简短全小写；不建立 aiManager、通用 Adapter/Factory 或模糊 platform/infra 目录。

- `ai/provider`：供应商维护、不可变凭据版本、上游模型配置与同步预览。
- `ai/officialModel`：标准模型目录与关联供应商报价的只读聚合展示。
- `ai/provider/model/priceVersion`、`ai/provider/model/priceRate`：供应商美元报价同步与只读审计、人民币内部售价预览与版本确认。
- `ai/agent`：多供应商绑定、当前项切换、提示词、工具与能力约束。
- `ai/tool`：定义/版本；具名 HTTP、currentTime、calculator 执行实现。
- `ai/conversation`、`ai/message`：本人普通会话与消息。
- `ai/run`：统一消费编排、快照、状态机、取消、监控与事件。
- `ai/providerRequest`、`ai/toolCall`：请求/工具事实，不承担管理路由。
- `ai/mediaTask`、`ai/mediaFile`：异步媒体和文件关联。
- `ai/usage`：计量归一化、精确计算、Charge/Items 与钱包结算意图。

供应商 I/O 使用具名 ResponsesClient、ChatCompletionsClient、ImagesClient、AudioClient、VideoClient；由 main 显式
装配到服务，按已知 protocol 选择，不用运行时插件/代码字符串。每个 I/O 接收源 Context，不能中途换成 Background。
同理 HTTP Tool、COS 与 Redis I/O 不进入 Handler，也不塞进 PostgreSQL Repository。

### 11.2 必须同步修改的现有模块

| 路径/模块 | 本期实际计划改动 |
| --- | --- |
| payment/wallet | DebitTx、负余额合法性、checked subtract；准入同钱包锁 |
| payment/transaction | direction/before/AI source、幂等约束、DTO/选项/查询/双语显示 |
| payment/order、payment/redeemCode | 入账显式写 direction=1 和 before，保持旧幂等语义 |
| storage/file、internal/storage/cos | 服务端 ImportGeneratedMedia、流式写入/校验/幂等、AI 引用与 cleanup 保护、安全媒体 inline |
| storage/uploadRule | Admin 当前规则新增 AI 用途 code，保留单启用规则约束及现有规则内容 |
| system/scheduler、cmd/api、cmd/worker | 具名 AI task/事务桥、长任务 Job token 续约、AI 阶段 lease、恢复和静态显式装配 |
| realtime | 复用 AppendTx/outbox、按用户交付和去重；调整并验证 relay 运输容量，不扩大跨用户订阅权限 |
| system/setting、config generation | 显式 AI 初始策略 seed 和 ai.runtime namespace/快照 codec，不改通用缓存语义 |
| permission/menu、Access | Admin 目录/page/actions seed、真实授权与菜单版本失效；不隐式给所有用户开 AI |
| web/src/api/ai、web/src/views/ai | 六个真实页面、明确 DTO、四媒体工作台和请求层错误契约 |
| web i18n/导航/页面文案 | 六页面和所有操作中文/英文，模块段与菜单 code 一致 |

前端 view→api→utils/request→Go API。只使用已有 AppDialog/AppTable/Search 等真实复用组件，选择器为强类型
el-select-v2；样式保持 scoped SCSS/主题，API 不加逐接口 DTO runtime parser。输入 rules/即时反馈保留，真实能力和
状态转换由后端 owning module 决定。错误由请求层统一通知，页面不重复弹同一接口错误。

## 12. 迁移与验收门槛

### 12.1 交付顺序及迁移纪律

1. 维护者评审本文后才进入实施：先固定 DTO/权限/状态测试，再按 TDD 增加最小实现。本文不是自动开工或数据库执行授权。
2. 创建 21 张表和 payment_transaction 扩展的 forward migration，写幂等结构校验；不复制旧 MySQL SQL，不 AutoMigrate。
3. 准备 Admin 菜单/actions、工程策略、供应商报价同步/人民币售价维护入口和具名 Scheduler 任务定义；不 seed 密钥、占位 Provider 或假价格。
4. 在隔离真实 PostgreSQL 验证首次/重复执行、非法旧账本/不完整缓存/冲突菜单的整体回滚，再由维护者授权维护窗口。
5. 业务库执行前备份并核对 schema/触发器/余额与 ledger；停止不兼容旧写入入口，迁移同事务处理回填与保护。
6. 变更菜单仅针对 Admin 平台；同事务真实变更时增加对应 menu_version，重复执行不再递增。不清理全库 Redis。
7. 迁移后从真实 PostgreSQL 刷新 current.sql；真实报价/Provider/Key/COS 规则由维护者核验配置，不使用假数据通过验收。
8. Provider/价格/Agent→工具/Run/工作台→统一用量/钱包→媒体/COS/恢复按依赖实现；每个切片同时闭合权限、Worker、幂等和测试。

### 12.2 必须通过的用例

| 范围 | 关键验收 |
| --- | --- |
| 菜单/权限 | Admin 恰好 1 目录+6 page；有 action 无 page 不产生路由，有 page 无 action 不可调 API；Canvas 无变化 |
| Provider/价格 | 密钥不回显；协议能力不猜测；美元报价只读、人民币单独编辑；新项同数值初始化；再次同步保留人民币价；普通更新/目录同步不能绕过价格动作权限；快照防串模型；发布后不可变；无完整价格不能生成 |
| Agent | 多绑定但唯一当前；跨 Agent 串绑失败；并发切换得到一个完整当前项；旧 Run 不改，新 Run 使用切换后项 |
| 本人范围 | 伪造 user/platform/run/conversation/file ID 无法读取/消费他人数据；管理查看不产生代付或停止权限 |
| Run 幂等 | 双实例同 request_key/指纹只一个 Run/一个 Job；异指纹冲突；手动 Scheduler retry 不再生成 |
| 余额/在途 | 正余额首笔变动用量可入；并发估算共享；余额<=0 拒绝；固定费用覆盖检查；未知用量阻止新消费 |
| 资金结算 | 扣成负数合法；重复/乱序结算单流水；并发充值与消费守恒；零费用无流水；unknown 不转 0；下溢不改余额 |
| 多轮计费 | 全部模型轮次去重计量；缓存 token 不重复计费；图片按质量/尺寸及生成张数收费且不叠加图片 token；人民币改价与 Agent 倍率共同生效；selector 匹配唯一；整数分项和总额一致；历史价格不回算 |
| 工具 | 未绑定工具/非法 schema/SSRF 被拒；有副作用未知不盲重放；call_id 重放幂等；超时/上限会停止且保留费用 |
| 四媒体 | 当前绑定支持才显示；视频只提交一次；轮询/转存恢复不重生成；同步字节结果不可恢复时明确失败 |
| COS | 服务端导入/多实例接管/稳定 request key；MIME/大小/重定向约束；引用清理保护；音视频合法 inline；私有预览授权 |
| 故障/缓存 | Redis unavailable 不返回旧配置/不回源风暴；PG 提交后队列故障 durable Job 可恢复；Scheduler/Run 双 lease 续约；旧 token 不产生新副作用；所有 due 恢复索引可用 |
| 实时/取消 | 刷新/断连恢复无新消费；补拉事件有序；过期事件明确；取消太迟仍保留真实消费 |
| 容量 | 至少两套 API/Worker/Redis client 并发验证，计数 PostgreSQL 查询/Redis 操作/锁等待/事件写频率，达不到预算就拒绝声称完成 |

实现后执行风险匹配的 Go fmt/vet/test/build、前端 Vitest/typecheck/lint/architecture/build 和真实浏览器四媒体链路；
单测、隔离 PostgreSQL/Redis、真实供应商消费、COS 及容量验收分开记录。未运行就不能写通过，不使用 Docker。

## 13. 事实依据、验证与评审结论

### 13.1 现状核对

- 既有事实盘点：[ai-module-inventory.md](ai-module-inventory.md)，包括旧 Admin/旧表和本地 sub2api 余额行为；不是本期建表清单。
- 项目规则：[AGENTS.md](../../AGENTS.md)、[architecture.md](architecture.md)、[design.md](design.md)。普通 CRUD 的
  last-write-wins、全局 COS/平台上传规则、配置 generation 与后台分层均遵循当前约定。
- 本轮已对真实 `admin.public` 执行只读 pg_catalog/information_schema 查询，`transaction_read_only=on`：
  现有 AI 表 0、AI 菜单 0；核对钱包/流水/文件字段、资金约束和 Admin 上传规则唯一索引，未写业务库。
- 代码依据：`payment/wallet/repository.go`、`payment/transaction/{model,protocol,response,repository}.go`、
  `storage/file/{model,service,repository,worker}.go`、`internal/storage/cos/types.go`、
  `system/scheduler/{protocol,repository,worker}.go`、`shared/money/units.go`。
- OpenAI 工具协议依据：官方 [Function calling](https://developers.openai.com/api/docs/guides/function-calling)，
  在本次连续会话前序已实际读取并核对 function_call_output/call_id；并未发起真实模型请求。
- OpenAI 模型目录与价格依据：官方 [`GET /models`](https://developers.openai.com/api/reference/resources/models/methods/list)
  仅返回模型身份、所有者和可用性字段；官方 [Pricing](https://developers.openai.com/api/docs/pricing) 与模型详情页提供计价表。
  本轮通过 Python urllib 实际取得上述两页 Markdown 版本 HTTP 200，核对目录字段和 Standard/Batch/Flex/Fast 等计价表；
  普通 HTML 页面获取曾返回 403，后续一次 Markdown 重取曾超时，不把文档站可达性当作稳定接口 SLA。
  因此实施时目录同步与报价同步必须是两个独立适配器，价格源响应保存摘要，不能把 `/models` 当报价接口。
- 视频端点仍按具名协议契约实现，账号和模型是否可用以实施阶段的真实能力/计价核验为准；未执行付费生成或真实四媒体验收。

### 13.2 本次文档交付的实际验证范围

设计编写阶段只创建设计文档并更新状态/盘点导航，检查表数、逐表章节、菜单、38 条 action 与关键产品规则一致性、Markdown 围栏/链接/
尾随空白。没有生成或执行 AI DDL，没有新增运行时代码、启停 API/Worker、调用真实生成/支付或修改 Canvas。
Go/前端构建、模型能力/费用、实际 COS 上传和容量测试尚未执行，属于后续实施验收，不能用文档静态检查替代。

维护者确认后已开始首批基础实现，实际代码/隔离测试/构建与剩余范围以 [STATUS.md](STATUS.md) 顶部当前工作为准；
上述文档阶段记录不是当前全部实施状态，也不能把隔离schema建表误认为正式库迁移完成。

### 13.3 维护者一眼可核对的交付口径

**本期最终交付：6 个 Admin 页面、21 张 AI 业务表、1 张现有资金流水表扩展、复用原钱包与 COS/Scheduler/权限体系，
完成文字/图片/视频/音频的 Agent 消费、真实工具执行、Run 追踪、实际用量计费与允许已准入请求负余额的完整链路。**

没有第二钱包，没有独立 Prompt 库，没有 Content/RAG，没有 Canvas 施工，没有自动供应商切换或免费生成旁路。
多供应商只是 Agent 的多条可配置绑定，每次只使用管理员当前选定的一项。本文评审通过后可据此编写实施计划；
不把本文创建完成等同于上述功能已经上线。
