# 任务简报：平台化 API 路由重构 + 支付体系一次性完成

这是重型任务（heavy）：需求有歧义（多平台路由契约需要设计决策），横跨 Go 后端 server/、Vue 管理端 web/、
React 对外前端 canvas/、PostgreSQL 迁移与 RBAC 菜单、资金账本与第三方支付回调；预计修改上百个文件、耗时多小时。
需要前置设计阶段（含设计评审），设计通过后安排且只安排一次维护者确认检查点；之后按阶段实现，每阶段
implement→review 循环（reviewer 永远是循环最后一步并给出裁决）；最后是 fail-closed 的完整验证门禁、整体评审与最终报告。

## 0. 用户原话与解读

第一条：“先做我们在做支付模块，我希望你能一口气帮我做完，顺便你也可以先查一下这个plan有没有什么问题，而且我不希望代码质量太差和破坏架构，而且我发现目前架构就已经存在问题了！现在又两个平台，router有问题，接口地址应该是api/admin/v1 和 api/canvas/v1 这样的，这样未来加多少个前端也不怕，现在就是有问题的！”
第二条：“继续”。

解读：
1. 评审现有支付 spec/plan，找出并修正问题；
2. 把 API 改成按前端/认证平台划分命名空间：`/api/admin/v1`、`/api/canvas/v1`，未来新前端 `/api/<platform>/v1`；
3. 一次性完成支付体系剩余全部阶段（P1–P5）；
4. 代码质量与架构边界不能降级。

支付的个人接口与回调接口依赖新路由契约，所以执行顺序为：基线 → 设计 → 检查点 → 路由重构 → 支付 P1…P4 → 合并迁移 → 最终验收。

## 1. 环境与硬约束

- 仓库 D:\admin，Windows，PowerShell 7（pwsh）。所有路径用绝对路径。
- 直接在主工作区 D:\admin 施工，不创建 git worktree：AGENTS.md 禁止未授权 commit，worktree 无法合回；且工作区里有本任务前序的未提交改动需要延续。
- 禁止 git commit/amend/fetch/pull/push/rebase/reset/restore/stash/clean、`checkout -- <file>` 等任何改历史或丢改动的命令；只用 status/diff/log/show/ls-files。
- 当前 HEAD 9df46ef4（main，比 origin/main 多 1 个提交）。基线未提交改动属于本支付线“支付回调模板默认填入”（STATUS 顶部条目，状态“定向验证中”），是本任务前序成果：先验证、保留、在其上继续，路由变更后相应调整，不得回退：
  - docs/agent/STATUS.md
  - server/internal/module/payment/config/handler_test.go、options.go、request.go、request_test.go
  - server/internal/shared/i18n/catalog.go
  - web/src/api/payment/config.ts
  - web/src/i18n/messages/en-US.ts、zh-CN.ts
  - web/src/views/payment/config/components/ConfigDialog/index.vue
  - web/tests/views/payment/config/index.test.ts
- 若发现与本任务无关的并发改动（例如 canvas/ 或其他文档），保留。编辑任何文件前重新读取它。
- 不启动/停止 API、Worker、Vite 或任何服务（维护者自行启动后端）。不做真实浏览器验收；UI 以 Vitest 组件测试（渲染、交互、权限显隐、i18n、错误态）+ typecheck + build 验证。
- 真实数据库（server/.env 的 POSTGRES_DSN 指向的 admin 库 public schema）只允许只读核对：`psql --no-psqlrc --set=ON_ERROR_STOP=1`，并包在 `BEGIN READ ONLY; ... ROLLBACK;` 内。禁止对真实库执行 DDL/DML、执行任何迁移 runner、写真实 Redis（DB0）。集成测试按仓库现有测试辅助使用隔离 schema/临时库和测试 Redis DB（STATUS 记录为 DB15），结束后清理临时对象。
- 可以把 server/.env 中的连接配置加载进进程环境用于测试或只读 psql，但任何输出、报告、日志中不得出现 DSN、密码、密钥、Token、私钥原文，只写键名。
- 不在 D:\admin 之外写文件（包括 C:\Users\IGT\.codex\skills）。若 Skill 内容需随新路由契约更新，只在最终报告中列出建议改动。
- docs/local-work/** 默认不可读。本任务经维护者明确允许读取并更新：D:\admin\docs\local-work\2026-10-13-payment-spec.md（433 行）、D:\admin\docs\local-work\2026-10-13-payment-plan.md（357 行），并可在该目录新建本任务的设计文档；不得读取该目录其他文件。
- docs/superpowers/** 不读。docs/database/2026-*.sql/.ps1 历史迁移默认不读；仅允许把 D:\admin\docs\database\2026-10-13-payment-config.sql/.ps1 与 2026-10-14-payment-config-remove-code.sql/.ps1 作为新 forward migration + runner 的风格与安全机制模板。当前结构事实以真实库只读查询和 D:\admin\docs\database\current.sql 为准。
- 旧项目参考（只读，只读直接相关文件）：D:\github-project\admin_back_go（internal/module/payment、payment/wallet、payment/redeemcode、internal/infra/payment/alipay），D:\github-project\admin_front_ts（src/views/Main/payment 与个人钱包）。只继承已验证业务行为，不复制旧表名、is_del、旧权限或旧 DTO。
- 资源受限：Go 测试用 `-p 1`；Vitest 用 `--pool=threads --maxWorkers=1`。任务开始后约 1 小时内可能还有 4 个只读调研 Agent 并行运行 go build/vet、pnpm typecheck；出现疑似资源争用的超时，先重试一次再下结论。
- 格式化只作用于本任务触碰的文件（gofmt/Prettier），不整仓重排他人文件。

## 2. 必读（按顺序）

1. D:\admin\AGENTS.md：硬规则全部遵守。要点包括：分层 router→middleware→handler→service→repository→model；异步 task handler→service→repository；依赖在 cmd/api/main.go、cmd/worker/main.go 显式装配，禁止 DI 容器、运行时注册器和通用 Adapter/Manager/Factory/Base*；context 全程贯穿；前端 view→api/<module>.ts→utils/request.ts；envelope 只有 code/data/message；el-select-v2；禁止 any/as any/@ts-ignore；RBAC page/action 独立；Yes/No 为 0/1；显式 created_at/updated_at；deleted_at 仅用于真实删除；迁移只由维护者执行；TDD。
2. D:\admin\docs\agent\README.md、architecture.md、design.md。
3. D:\admin\docs\agent\STATUS.md：共 1423 行，只读顶部约 200 行（含支付各条目与 Canvas 条目），其余按关键字搜索。
4. 上述两个支付 spec/plan 文件全文。
5. Skills（只读，对应阶段必须按其流程与检查项施工）：C:\Users\IGT\.codex\skills\admin-crud\SKILL.md、admin-rbac\SKILL.md、admin-database\SKILL.md；路径不存在时读 D:\admin\.codebuddy\skills\ 下同名副本。
6. 可选上下文：D:\admin\.agents\tasks\project-deep-dive-2026-10-14\ 下的 server-core.md、server-business.md、web.md、canvas-database-tooling.md 是并行只读调研报告，可能尚未生成。若存在，先读“执行摘要”和与路由、认证、请求层、支付、迁移相关的章节，引用前对照代码核实；不存在则不要等待。

## 3. 已知事实（来自文档与 STATUS，实施前须对照代码核实）

### 3.1 路由现状

- `/api/v1` 承载共享的认证、Access 等接口，平台由请求头 `X-Auth-Platform` 决定，Admin 与 Canvas 都调用它；`/api/admin/v1` 额外要求 Admin 平台标识。另有 `X-Device-ID` 头。
- 认证平台 Admin id=1、Canvas id=2，表 permission_auth_platform（含 code、menu_version、policy_version）；Redis 策略键 `auth:policy:v2:*`。
- 已知的 `/api/v1` 接口举例：`/api/v1/auth/login-config`、`/api/v1/auth/policy`、`/api/v1/auth/logout`、refresh、`/api/v1/storage/upload-credential`、`/api/v1/realtime/ws`（WebSocket，先取 Redis 单次 ticket）、匿名的 `/api/v1/system/setting/legal/:document`（与管理端 `/api/admin/v1/system/setting/legal/:document` 同名，合并后会冲突），以及个人中心 user/profile、user/email、user/phone 等。
- 管理 API 举例：`/api/admin/v1/permission/authplatform`、`/api/admin/v1/system/config-cache-status`、`/api/admin/v1/payment/config`。
- API 的 Origin 校验 `RequireOrigin` 与 `CORS_ORIGIN` 只支持单一来源（http://localhost:16300）。Canvas 本地开发靠 Vite 代理把上游 Origin 改写成 16300 才能工作，生产必须另行配置，这也是多前端架构缺陷的一部分。
- refresh 用 HttpOnly cookie，access token 存内存；本机 localhost 不同端口共享 cookie。
- 前端相关文件：web/src/utils/request.ts、web/src/auth/platform.ts（客户端平台标识）、web/src/api/**；canvas/src/utils/request.ts、canvas/src/auth/platform.ts、canvas/src/api/{auth,permission,storage,user}。两端 Vite 都把 `/api`（含 WS）代理到 http://localhost:16301。端口：web 16300、API 16301、canvas 16302。
- server/internal/architecture 是架构门禁测试；web 有 `pnpm check:architecture`。

### 3.2 支付现状

- 已完成：
  - server/internal/shared/money（金额 units）。
  - payment/config 切片：server/internal/module/payment/config；server/internal/payment/alipay（GoPay v1.5.118 证书内容模式）；AES-GCM 派生密钥；payment.config/global 缓存；web/src/views/payment/config；API `/api/admin/v1/payment/config`；page/action `payment:config:view/list/detail/create/update/status/delete/test`。
- 真实库已执行支付配置迁移和 2026-10-14 编码移除迁移（STATUS 10-14 条目确认 `payment_config.code` 已不存在）。docs/agent/architecture.md 对应行仍写“真实库待维护者执行”，已过期，需先用只读查询核实再修正。
- 未完成：充值套餐、钱包、统一订单、流水、回调事件、个人“我的钱包/充值中心”、订单巡检与关单（Scheduler 计划 payment.order.reconcile）、兑换码（批次与码）、全体验收。回调入口尚不存在，目前只有回调 URL 模板与校验。
- 维护者上次试建配置失败的原因是证书过期（2026-03-14 到期）和私钥 base64 损坏，所以无法做真实支付宝沙箱验收。支付通道测试一律用受控的 httptest Provider，最终报告如实列为未运行。

### 3.3 编排者对 spec/plan 的初步问题

设计阶段必须逐条核实，并在此基础上做完整评审：

1. spec §11 的 HTTP 表面用 `/api/v1/payment/...`（个人钱包/充值）和 `/api/v1/payment/callback/alipay/:configId/:version`（回调），与新路由契约冲突。并入 `/api/admin/v1` 后，个人 `payment/wallet` 会与管理端 `GET /api/admin/v1/payment/wallet` 直接冲突。
2. spec §6 仍写 config“有效 code 唯一”，与 10-14 移除编码的决定不一致。recharge_package 也设计了“有效 code 唯一”的内部编码，与维护者“管理员不填内部编码、用 ID 关联”的最新决定是否一致需要拍板。推荐一并去掉，除非存在真实消费方。
3. 回调 URL 里的 `{configId}/{version}`：STATUS 10-14 条目指出旧项目使用固定公共入口，这个坐标设计并非支付宝要求；而且配置创建前 configId 未知，管理员无法手填（当前未提交改动是让管理员只替换 HOST）。路由迁移后模板必须改。需要决定：保留坐标（由服务端填充）还是改为固定入口，并确保管理员只需配置公网 HOST。若真实库已有配置行存了旧路径（只读核实），迁移需要处理。
4. `PAYMENT_RETURN_ORIGINS` 与 `CORS_ORIGIN` 各管各的：回跳地址本应回到发起购买的那个前端，CORS/Origin 也应按平台配置。建议统一按平台建模（具体形式由设计决定），并去掉 Canvas 代理改写 Origin 的权宜做法。新增或改动的环境变量遵循“缺失时明确失败或明确不可用、不猜默认值”，并同步 README 与 .env.example。
5. `PAYMENT_ENVIRONMENT`：API 和 Worker（巡检）都需要；明确必填还是可选，以及缺失时的行为。
6. P1–P4 原计划各自一份迁移。现在一次性交付，建议合并为一个（或尽量少的）可审计、幂等的 forward migration + runner，由维护者停服后一次执行。内容包括表、约束、触发器、菜单与动作、registered_user 个人授权、Scheduler 计划 seed、版本推进和 Redis 有界修复，沿用最新 runner 的机制。
7. §8 的通道预算（每个 App/环境 40 次/秒、8 个在途、Redis 原子令牌）和巡检参数都是“待评审值”。确认是否必要、能否用现有原语实现；禁止为此造通用限流框架。
8. plan 里有些事实已过期：菜单版本 19/1 现在是 20/5；“未提交 shared/money”等说法；`superpowers:executing-plans` 不适用。实现时以真实库和代码为准。
9. spec 规定 Canvas 的钱包/充值页面等 Canvas 真正接入时再交付。新路由下，后端个人支付接口应能挂在每个平台命名空间（由权限控制，Canvas 没有授权即返回 403）。Canvas 页面是否本次一起做，需在检查点确认（推荐：遵循 spec，本次不做）。

### 3.4 既有测试失败

STATUS 记录，需在基线重新核实。全仓 `go test -p 1 ./... -count=1` 未全绿：

- (a) storage/file 的 Service 分层门禁 TestBackendSourceBoundaries 失败。这是生产代码违反架构，属于本任务“不破坏架构”的范围，推荐修复且保持行为不变。
- (b) internal/database 与 architecture 中依赖已退出对象（如 system_config_cache_generation）的历史迁移 fixture 失败。处理方式在检查点由维护者决定；推荐不删文件，除非维护者同意按提交 ef0f61aa 的先例移除过期的一次性迁移测试。
- (c) Canvas auth-upload runner 的 Redis 重启恢复回归失败。维护者已明确停止这项工作，本任务不得处理，只记录为既有失败。

## 4. 路由重构：需求与设计要点

设计阶段需产出完整契约。下列为推荐方向，需先核实可行性。

需求：每个前端/认证平台一个 API 命名空间。Admin 前端只调用 `/api/admin/v1/*`，Canvas 只调用 `/api/canvas/v1/*`。新增前端时只需新增平台（加极少量显式配置），不必重新划分路由结构。

设计必须覆盖以下各项：

- a. 现有路由清单与映射：从 cmd/api/main.go 和各模块 route.go 提取完整清单（方法、路径、中间件链、权限码、平台约束），并逐条给出 old→new 映射表。
- b. 平台身份：
  - 推荐以路径段作为唯一平台来源，删除 X-Auth-Platform（请求层、CORS allowed headers、测试一并同步）。
  - token、session、WS ticket 所属平台必须与路径平台一致，不一致时返回明确错误。
  - 平台解析不得给热路径新增 PostgreSQL 查询，复用现有策略缓存；Redis 故障时失败闭合。
- c. 装配方式：二选一并论证。
  - (i) `/api/:platform/v1` 参数组，由中间件按 code 解析；新平台无需改代码。
  - (ii) 在 main.go 中按平台 code 列表显式注册。
  - 必须用 go.mod 里实际的 Gin 版本验证静态段与参数段能否并存、回溯行为如何（写临时测试，验证后删除）。
  - 遵守 AGENTS.md：显式装配，不用运行时注册器。
- d. 共享能力与管理接口：
  - 共享能力用同一个注册函数挂到每个平台命名空间，包括：认证、Access、个人中心、邮箱/手机、上传、站内通知收件箱、实时、公开协议文档、支付个人能力；“我的文件”是否共享由设计决定。
  - 管理接口只挂在 admin 下。
  - 列出并解决全部同名冲突，已知有 legal 文档和支付 wallet 两处。推荐用已有的页面/权限码资源名区分个人与管理资源（例如 myWallet→`payment/mywallet`、recharge→`payment/recharge`），不另造前缀。
  - 不顺手重命名与本次无关的资源。现有 `authplatform`/`cosconfig` 与 `config-cache-status`/`upload-credential` 风格不一致，只在报告中记录。
- e. 平台无关接口：
  - `/health`、`/ready` 不变。
  - 第三方回调放进不会与平台 code 冲突的保留命名空间（例如 `/api/callback/v1/...`，名称由设计定），并在认证平台 code 校验中保留这些词。
  - 明确平台 code 的格式（小写、URL 安全）以及创建后是否可改。推荐不可改，因为它就是 API 命名空间。
- f. Cookie：refresh cookie 的 Path/名称按平台命名空间隔离（同主机不同端口会共享 cookie）。说明旧 cookie 会失效、用户需重新登录；上线前可以接受，写入报告。
- g. CORS/Origin：改为按平台配置允许来源；去掉 Canvas Vite 代理改写 Origin 的做法；写清环境变量形式和缺失时的行为，并同步 README/.env.example。
- h. 所有依赖路径前缀的逻辑都要梳理：
  - 操作日志的采集范围：保持现有语义，明确管理写操作与个人操作是否记录；
  - RequireOrigin、访问日志、限流、WS；
  - 队列监控 UI：server/internal/module/system/queueMonitor 的 upstream UI 可能带 root path；
  - 前端请求层里匿名、免刷新、logout 等特例清单，以及实时 WS URL 的构建；
  - 测试中的路径断言。
- i. 契约测试：
  - Go 测试枚举 gin 引擎的全部路由，断言每条 API 都位于 `/api/<platform>/v1`、回调保留命名空间或 health/ready 之下；管理路由不出现在 canvas 下；共享路由在每个平台都存在且权限码一致。
  - web/canvas 测试断言 API 模块只使用本平台前缀。
- j. 容量：对比改动前后平台解析与鉴权热路径上的 Redis/PG 访问次数；多实例一致性保持不变。
- k. 破坏性影响与上线步骤：维护者需更新 .env、重启 API/Worker，用户需重新登录；除支付回调 URL 数据可能需要迁移外，无数据库变更。

## 5. 支付范围

以评审修订、并经检查点确认后的 spec/plan 为准。

- P1：支付配置收尾（包括未提交改动，回调模板随新路由调整），以及固定充值套餐管理。
- P2：
  - 后端：钱包、统一订单、资金流水、回调事件；支付宝 PC/H5 下单、验签回调、主动查单、关单。
  - 前端：个人“我的钱包/充值中心”页面；三个管理页（订单、只读钱包、只读流水），订单详情含回调审计。
- P3：`payment.order.reconcile` 周期巡检（复用 system_scheduler_schedule/job/run，不建支付任务表），以及超时关单、迟到付款恢复、租约与通道预算。
- P4：兑换码批次与码，包括幂等生成、授权导出、兑换与作废的竞争处理、等额一次入账。
- P5：全体验收。

关键不变量（见 spec §7、§13）：

- 同一 requestId、同一输入只产生一笔订单。
- 重复或并发回调、人工核实、双 Worker 竞争，都只入账一次。
- 签名有效但 app/seller/金额/订单/版本不匹配时拒绝。
- 入账在单个事务内完成，锁顺序为回调事件→订单→钱包；流水来源有唯一约束。
- 任何失败全部回滚，并保留可重放的证据。
- 余额允许为负，面额必须为正；HTTP 中金额用十进制字符串；枚举的 JSON 只接受数字。
- 回调响应按支付宝协议返回字面量 success/failure，不用 envelope：
  - 不经过用户认证、Origin 和平台中间件；
  - 保留 request ID、访问日志和 Recovery；
  - 操作日志不得记录敏感 payload。
- 浏览器回跳不改变资金事实；生产环境拒绝 sandbox。

前端：

- web 新页面全部走动态路由，使用独立的 :view/action 和 AppTable/AppSearch/AppDialog、el-select-v2。
- 中英文 i18n 与 placeholder 齐全，二次确认遵循 design.md 规范。
- 金额按字符串展示，负余额原样显示。
- Canvas 页面是否本次做，以检查点结论为准。

菜单/权限按 spec §10：

- Admin 新增“支付管理”目录，含 6 个管理 page，另有 2 个个人根 page。
- registered_user 只显式获得个人 page/action，不自动获得管理动作。
- super_admin 沿用现有语义。

代码质量标准：

- 函数短小内聚，没有死代码；admin/canvas 路由注册不重复逻辑。
- 不做占位页、不造假数据；错误映射明确，不吞错。
- 不新建通用框架；注释语言与现有代码库保持一致。

## 6. 流程要求

1. **基线**（写入基线报告）：
   - 记录 git status 和 HEAD。
   - 运行 `go test ./internal/architecture -count=1`。
   - 运行 STATUS 顶部条目列出的支付配置定向测试：后端 payment/config 等包；前端 tests/views/payment/config/index.test.ts、tests/i18n/index.test.ts。据此确认未提交改动的当前状态。
   - 只读查询真实库：payment_* 表及行数；payment_config_version 等表中是否已有旧路径的 notify URL；permission_auth_platform 的 id/code/menu_version/policy_version；支付菜单；Scheduler 计划。

2. **设计阶段**（设计作者 + 设计评审循环，评审为最后一步）产出两份文档：
   - D:\admin\docs\local-work\2026-10-14-platform-routing-design.md：第 4 节全部内容，外加容量假设、热点查询预算、缓存策略和并发验证方式。
   - D:\admin\docs\local-work\2026-10-14-payment-plan-review.md：spec/plan 的完整问题清单（每条写明问题、证据、影响、修改建议）；新路由下完整的支付 HTTP 表面；迁移合并方案；验证策略；需要维护者拍板的决策及推荐项。

3. **唯一检查点**：
   - 设计评审通过后，用 send_message severity=warning 暂停，给维护者发一份简洁的中文摘要（不超过 40 行），内容包括：
     - 计划问题与修改建议；
     - 路由契约要点与破坏性影响；
     - 需要拍板的决策，每项附推荐默认值。至少覆盖：Canvas 支付页是否本次做、recharge_package 是否去掉 code、回调路径形式、历史迁移 fixture 失败如何处理、环境变量方案；
     - 两份文档的路径；
     - 一句说明：回复“按推荐”即全部采用推荐值。
   - 收到答复后：
     - 把决定写入两份设计文档；
     - 把修订同步进 2026-10-13-payment-spec.md 和 2026-10-13-payment-plan.md，使其成为唯一实施契约，文件顶部注明修订日期与来源；
     - 然后继续执行。
   - 此后不再暂停。例外：遇到必须由维护者决定的破坏性或不可逆操作，或与维护者明确指令冲突的情况。其他小决策直接选推荐项并记录。

4. **实现阶段**：按下列顺序推进。每阶段走 implement→review 循环，reviewer 最后给裁决。涉及资金账本、回调、巡检、兑换并发的阶段评审要格外严格，可用独立双评审。
   - a. 平台路由重构：server、web、canvas、文档、契约测试。
   - b. 修复 storage/file 分层门禁，行为不变。
   - c. 支付 P1。
   - d. 支付 P2。
   - e. 支付 P3。
   - f. 支付 P4。
   - g. 合并迁移：SQL + PowerShell runner，命名如 D:\admin\docs\database\2026-10-14-*.sql/.ps1，或按设计命名。只在隔离 schema/临时库验证：首次执行、重复执行、冲突拒绝与回滚、菜单/授权/版本推进。绝不对真实库执行。

   每个行为变更都遵循：失败测试 → 最小实现 → 通过 → 重构。测试不得只镜像源码，也不得只断言 mock 存在。

5. **最终验证门禁**（fail-closed）与整体评审：
   - 运行第 7 节的全部命令。只有第 3.4 节 (c)，以及检查点确认可保留的既有失败允许不通过，其余必须全绿。
   - 对整个 diff 做一次独立语义评审，修复发现的问题后再出报告。

6. **文档**：
   - STATUS.md：在顶部新增本任务条目。编辑前重新读取顶部，只新增、不覆盖他人条目。
   - architecture.md / design.md：写入新路由契约和已实现的支付稳定规则，修正过期描述。
   - README：更新路由、环境变量和 Canvas 代理说明。
   - server/.env.example、web/.env.example、canvas/.env.example：同步新变量，不写真实值。
   - 不重导 current.sql，因为真实库未迁移。

7. **最终报告**：写入 D:\admin\.agents\tasks\platform-routing-payment-2026-10-14\final-report.md（中文），内容包括：
   - 实际修改的文件清单；
   - 实际运行的命令与结果；
   - 未运行项及原因（真实沙箱支付、浏览器验收、真实库迁移等）；
   - 剩余风险；
   - 维护者执行步骤：要更新的 .env 键名与示例、停止 API/Worker、按顺序执行迁移 runner 及参数、重启服务、重新登录、在角色管理中授权；
   - 建议同步的 Skill 改动；
   - STATUS 是否已更新。

   各阶段的中间报告也放在同一目录下。

## 7. 验证命令

server（D:\admin\server）：
- 对本任务触碰的 Go 文件运行 `gofmt -l`
- `go vet ./...`
- `go test ./internal/architecture -count=1`
- 各阶段的定向包测试
- 最终 `go test -p 1 ./... -count=1`
- `go build ./...`

web（D:\admin\web）：
- 先读 package.json 确认脚本名（如 format:check、lint）
- 受影响文件的 Prettier/ESLint
- `pnpm typecheck`
- `pnpm check:architecture`
- `pnpm vitest run --pool=threads --maxWorkers=1`
- `pnpm build`

canvas（D:\admin\canvas）：
- `pnpm test`
- `pnpm build`（含 tsc）

根目录（D:\admin）：
- `git diff --check`
