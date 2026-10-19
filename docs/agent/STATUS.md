# 项目状态

## 支付配置表格两列居中（2026-10-19，定向验证完成）

- 渠道、支付通道两列共用的 `payment-config-page__identity` 移除 `flex-start` 强制左对齐，改为 `align-items: center`；保留原内容、上下排列、间距及其他列，不改通用 AppTable。
- 新增渲染回归，编译真实页面 SCSS 并检查两个容器的 computed style；先复现两处 `flex-start`，修正后 `pnpm vitest run tests/views/payment/config/index.test.ts --pool=threads --maxWorkers=1 -t "centers the channel and payment channel content in their cells"`（1 项通过、29 项跳过）、两个改动文件 ESLint/Prettier 与 `git diff --check` 通过。
- 按维护者要求，本批改动期间只运行必要定向检查，build 与全量测试留待全部修改完成后集中运行；本次未做浏览器几何验收、数据库改动或 Git 提交。

## 支付配置页过期提示与状态操作文案（2026-10-19，定向验证完成）

- 移除支付配置页“充值与支付回调将在后续阶段接入”的过期 info 提示；状态操作按当前值明确显示“启用/停用”，分别使用 success/warning，并在确认框展示目标状态。状态列继续消费后端按语言投影的 `presentation.status`，其值遵循 `0=停用、1=启用`，不由前端重复计算。
- 状态字段统一使用 `payment.config.status.label/enabled/disabled`；补齐中英文回归，覆盖提示移除、启停方向、颜色、目标确认、取消/关闭、权限和请求失败。同步更新过期的支付回调设计描述。
- 验证：`pnpm vitest run tests/views/payment/config/index.test.ts tests/api/payment/config.test.ts tests/i18n/index.test.ts --pool=threads --maxWorkers=1`（3 文件/42 项）通过；`pnpm build`（含 vue-tsc）、受影响文件 ESLint/Prettier、`pnpm check:architecture` 与 `git diff --check` 通过。构建仍有既有 >500 kB chunk 警告；未运行全量前端测试、真实支付/回调或服务联调，未提交 Git。

## 平台 API 显式隔离（2026-10-18，代码与迁移完成，待启动联调）

- 已获维护者批准：取消 `/api/v1`，Admin 使用 `/api/admin/v1`，Canvas 使用 `/api/canvas/v1`；每个业务模块独立声明 `RegisterAdminRoutes` / `RegisterCanvasRoutes`，不得用两个包装函数调用同一份路由清单。新平台必须显式新增注册入口；内部 Handler/Service/Repository 仅按真实业务复用。
- 工作区开始时干净。现状盘点覆盖 `cmd/api/main.go`、模块路由、认证/刷新 Cookie、Access、前端请求与 WebSocket、真实 PostgreSQL 只读查询。平台 Admin/Canvas 均启用；Canvas 当前仅有 workspace/profile 两个隐藏 page，以及 upload、profile detail/update、password、email、phone 六个 action。个人账户和钱包仍遵守既有全局业务归属，不借路由改名复制数据或授权。
- 目标映射：Admin 保留管理、本人钱包/充值/兑换、通知/实时、本人文件等现有能力；Canvas 显式注册登录/策略/验证码、Access、个人资料/改密/邮箱/手机号、上传凭据/对象 URL/上传完成和公开法律文档，不批量开放 Admin 管理或当前无权限事实/调用方的支付、通知、文件管理 API。双方 action 权限码不变，认证/Access 仍绑定当前平台。
- 冲突处理：Admin 本人钱包读取调整为 `GET /api/admin/v1/payment/wallet/me`，管理列表仍为 `GET /api/admin/v1/payment/wallet`；法律文档同路径保留匿名净化 HTML 读取，管理写入继续认证并检查 `system:setting:update`。支付通道由 Admin 管理，Provider 回调归属 `/api/admin/v1/payment/callback/alipay/:configId/:version`，使用支付签名而非客户端身份；不建立 Canvas 支付接口。
- 认证映射：路由组固定平台，`X-Auth-Platform` 只做严格一致性校验，Token 与 Session 继续校验相同平台；Refresh Cookie 按平台隔离名称和 `/api/<platform>/v1/auth` 路径。百万用户/多实例容量契约不变：平台分组不增加 PostgreSQL 查询、缓存来源或无界回源，Redis/PG 故障继续失败闭合。
- 切换影响：旧客户端 Refresh Cookie 的 Path 为 `/api/v1/auth`，新平台路由不会收到旧 Cookie；已签发 Access Token 到期后用户需重新登录。为遵守不保留共享旧接口的决定，不提供旧 Refresh 路由兼容桥。
- 支付切换边界：SQL 只改本地凭据版本，不会改写已发给支付商的旧订单通知地址；旧 `/api/v1` 回调在新服务中返回 404。SQL 和 runner 均拒绝仍绑定旧回调路径的待支付订单；必须先通过可信查单结清或关闭旧单，再停机迁移。前后端应在同一维护窗口切换，不混用旧客户端与新 API。
- 执行清单：路由隔离回归、模块独立注册、认证/实时边界、前端 Admin/Canvas 前缀同步、回调地址幂等迁移及维护者 runner、架构文档更新均已完成。Canvas allowlist 覆盖登录/Access/个人资料/上传/法律文档，明确不继承 Admin 管理能力；业务模块只保留显式平台注册入口，健康检查除外。
- 本轮验证：API 路由测试通过；受影响 Go 模块包的路由定向命令通过（部分包无匹配测试）；路由实现阶段 `go fmt ./...`、`go vet ./...`、`go build ./...` 通过。新增旧订单保护后，受影响的 runner/config 包再次通过 `gofmt`、`go vet -p 1 ./cmd/migrate-platform-routes ./internal/module/payment/config` 和 `go build -p 1 ./cmd/migrate-platform-routes ./internal/module/payment/config`。Web `pnpm build` 和 `pnpm check:architecture` 通过；Canvas `pnpm build` 及修改相关 21 项 Vitest 通过。Web 修改相关 18 个测试文件中 17 个通过，1 个操作日志错误态用例因页面/AppTable 文案预期不一致失败，未改动该页面逻辑。
- 迁移保护补充采用失败测试先行：先复现待支付旧回调订单被错误迁移，再实现 SQL 表锁/事务拒绝与 runner 订单计数预检。最终实际执行 `go test -p 1 ./cmd/migrate-platform-routes ./internal/module/payment/config -run 'TestPreflight|TestPlatformCallbackMigration|TestConfigMigrationConstraintsAndReplay' -count=1 -timeout=3m -v` 通过，覆盖预检拒绝/放行、未结旧单回滚、已支付/已关闭订单迁移、无版本、幂等重放、不可变事实及缓存 refill、outbox 冲突回滚；仅写隔离测试 schema。
- 全量验证仍有失败：此前全量 Go 测试报告含 `architecture.TestBackendSourceBoundaries`、若干历史 schema/migration 集成用例失败，以及 `system/scheduler`、`system/setting` 测试各运行 11 分钟后超时；支付配置和上传规则的路由回归项已由本轮定向测试通过。全量 Web Vitest 为 17 failed、1190 passed，另有 worker 超时；独立 HEAD 前端基线因测试 mock 初始化失败，不能据此判定这些失败为基线问题。未执行全量 Canvas Vitest。
- 收尾环境：一轮迁移复跑因长时间无输出而中止，之后降低 Go 编译并发的最终定向命令已通过。构建生成的 `canvas/tsconfig.tsbuildinfo` 已恢复本轮构建前内容；现有 `.git/index.lock` 未删除。本轮忽略目录 `server/tmp/platform-route-head-baseline`、`server/tmp/canvasRunnerTest-2488559521` 和测试日志 `server/tmp/platform-route-go-tests.log` 的清理被执行策略拦截，尚保留，未纳入交付文件。
- 维护者明确授权“迁移”后，于 2026-10-18 23:26:45 +08:00 对本机 `admin.public` 执行 `docs/database/2026-10-18-platform-api-routes.ps1 -ServicesStopped`。执行前核实本项目 API/Worker 未运行、16301 端口未监听、PostgreSQL 无其他客户端会话、Redis 客户端仅有本次探针；旧回调凭据版本 1 条、旧回调待支付订单 0 笔、PG/Redis 均为 generation 3。
- 首次 runner 成功将 1 条回调切为 `/api/admin/v1/payment/callback/alipay/1/1`，CHECK 已验证、不可变触发器保持启用；generation/outbox 由 3/3 变为 4/4，Redis `payment.config/global` 为 ready/4，固定 state key 数量仍为 1。随后同命令复跑成功，迁移行数为 0，generation/outbox 与受保护摘要均未再次变化。两份审计的 `postgresCommitted`、`redisSynchronized`、`verified` 全为 true，SQL SHA256 与执行文件一致。
- 审计根目录为 `%LOCALAPPDATA%/Admin/maintenance/platform-api-routes/`：首次 `20261018-232434-198-bda54fbd37cb472a8e408de63479b96d/audit.json`，重放 `20261018-232707-106-de80ad9db12a40ddb656b134fcdb303a/audit.json`；首次目录另有 `replay-check.json`。独立只读复核确认旧回调为 0、新回调为 1、旧待支付订单为 0，凭据非 URL 字段、支付配置、订单、菜单和平台事实摘要与迁移前一致。
- 2026-10-18 23:32:45 +08:00 使用真实 PostgreSQL 18.6 的 `pg_dump --schema-only --no-owner --no-privileges --schema=public --no-password` 导出并更新 `docs/database/current.sql`，复制后 SHA256 与审计目录中的原始导出一致；Git diff 仅回调 CHECK 一行变化，无业务数据或其他结构漂移。初次快照导出被探针的空 `PGSERVICEFILE` 拒绝，去掉该空覆盖后导出通过，未影响已成功的迁移。首次审计目录另存 `independent-check.txt`，包含结构、事实、Redis 状态和独立比对通过结果；临时只读探针源码已移除。
- API/Worker 保持停止，未自动启动、未修改订单状态、未删除 Redis key 或执行 Git 提交。新增 outbox 事件 1 条仍待 Worker relay 标记发布，Redis 代际已同步，不需要人工篡改 outbox；启动新代码后可由既有幂等 relay 正常处理。本次不重跑前后端全仓测试/构建，未做浏览器联调，前述全量失败不因迁移而视为解决。

## 钱包入口路由组件纠正（2026-10-17，定向验证完成）

- 个人资料上方“钱包”入口的 `/payment/walletPortal` 曾误加载 `payment/wallet/index.vue` 管理钱包列表；现已改为既有 `payment/walletPortal/index.vue`，通过 `el-tabs` 聚合“我的钱包”和“充值中心”，两个 Tab 沿用各自 page/action 权限。不改管理钱包页、权限协议或数据库。
- 新增真实路由导航回归：`router.push('/payment/walletPortal')` 后必须解析到聚合页组件并保留认证要求；先确认旧实现因加载管理组件失败，再修复。实际运行 `pnpm vitest run tests/router/index.test.ts tests/router/permissionRoutes.test.ts tests/layout/components/AppAside.test.ts tests/views/payment/wallet.test.ts --pool=threads --maxWorkers=1`（4 文件/36 项）、`pnpm typecheck`、两个受影响 TypeScript 文件 ESLint/Prettier 与 `git diff --check` 均通过。
- 本轮未运行全仓测试、生产构建或浏览器联调，未执行迁移或启停服务，未提交 Git；保留前一轮 `STATUS.md` 中的迁移执行记录。

## 钱包入口聚合与个人支付菜单隐藏（2026-10-17，前端实现与菜单迁移完成）

- 侧栏账户菜单新增始终显示的“钱包”入口，固定跳转 `/payment/walletPortal`；入口不使用权限 `v-if`。钱包页使用 `el-tabs` 聚合个人钱包与充值中心，两个 Tab 分别只在 `payment:myWallet:view`、`payment:recharge:view` 存在时显示，原 API/action 权限不变。
- 个人钱包聚合页采用深青绿资金主题、余额/充值/流水分区和窄屏适配；新增中英文文案、面包屑与路由标签支持。forward migration `docs/database/2026-10-17-payment-personal-pages-hidden.sql` 已由维护者授权执行，将充值中心和我的钱包 page 节点设为隐藏并按实际变化递增菜单版本。
- 前端既有定向验证：`pnpm vitest run --pool=threads --maxWorkers=1 tests/views/payment/wallet.test.ts tests/layout/components/AppAside.test.ts tests/router/permissionRoutes.test.ts`（3 文件/21 项）通过；`pnpm typecheck`、受影响文件 ESLint、`pnpm build`、`git diff --check` 通过。构建仍有既有大 chunk 警告；未做浏览器视觉截图验收。
- 数据库执行核验（2026-10-17 22:54 +08:00）：真实 `admin.public` 的 Admin 平台 ID=1；菜单 ID 4334/4335 的 `payment:recharge:view`、`payment:myWallet:view` 均由 `is_hidden=0` 改为 `1`，`menu_version` 由 23 升至 24。首次 `psql -X -w -v ON_ERROR_STOP=1 -f` 执行成功但提示 `SET LOCAL` 缺少事务块；随后使用 `--single-transaction` 复跑无警告，完整目标页面记录与菜单版本均未再次变化，幂等核验通过。
- Redis 原 `authz:menu-state:v1:1` 为 ready/23，已按完整旧值 CAS 定向同步为 ready/24；最终 PostgreSQL 只读事务及 Redis GET 均确认两个启用 page 已隐藏、版本一致。未删除快照、未修改用户授权/认证策略或其他 Redis key，临时探针已移除；本次仅 DML，不更新 schema-only 的 `current.sql`。本轮未重跑前后端全仓测试、未启停服务或做浏览器联调，未提交 Git。

## 支付证书导入文件名布局（2026-10-17，定向验证完成）

- 支付配置新增/换版表单的证书导入文件名已从文本域下方移到“导入证书文件”按钮右侧；标签行可收缩，文件名单行省略并保留完整 `title`，三类证书和中英文/窄屏布局一致。私钥字段不显示导入文件名。
- 新增页面回归覆盖新增与换版三类证书的文件名位置、完整名称和文本域不重复展示；实际验证 `pnpm vitest run tests/views/payment/config/index.test.ts --pool=threads --maxWorkers=1`（17 项）、受影响文件 Prettier/ESLint、`pnpm typecheck`、`pnpm build`、Playwright 桌面/390px 中英文 24 项像素布局检查均通过；构建保留既有大 chunk 提示，未提交 Git。

## 支付运行配置收口与自动回跳（2026-10-17，实现与定向验证完成）

- 按维护者要求移除 API/Worker 的 `PAYMENT_ENVIRONMENT` 与 `PAYMENT_RETURN_ORIGINS` 配置字段、加载校验和装配依赖，同时清理 `.env.example`、本地 `.env` 和 README 的重复配置。支付环境以后台支付配置为唯一来源；完整 HTTPS `notifyUrl` 仍由支付配置页面维护，与前端 Origin/CORS 无关。
- 个人充值页取消手填回跳地址，在创建订单提交时自动取当前页面 URL（保留路径/query、去掉 fragment）作为 `returnUrl`，通过订单快照传给支付宝 `return_url`；后端只校验绝对 HTTP(S) URL 格式，不新增回跳白名单配置，不根据回跳参数入账。
- 订单列表/详情/幂等查询通过原 SQL 联表读取配置环境，不增加逐行配置查询，列表仍为 count + page 两次数据库查询；历史回调、恢复和巡检按订单绑定的配置/凭据版本选择 SDK 环境，Redis 限流坐标继续按真实凭据的 environment/appId 隔离，沿用跨实例状态/快照/租约协议。默认百万用户/多实例容量假设不变；生产下单保护门保留，本次不开放生产支付、不迁移表或菜单、不改变负余额/不退款约定。
- 回归先复现无 env 启动失败、回调/巡检全局环境依赖、挂载时地址固化以及不完整 URL 被接受；修补后 `go test -p 1 ./internal/config ./internal/module/payment/... ./internal/payment/alipay -count=1`（含隔离 PostgreSQL/Redis 集成）通过，前端 `pnpm vitest run tests/api/payment tests/views/payment tests/i18n/index.test.ts --pool=threads --maxWorkers=1`（11 文件/47 项）通过。`go vet ./internal/config ./internal/module/payment/... ./internal/payment/alipay ./cmd/api ./cmd/worker`、`go build ./...`、`pnpm build`（含 vue-tsc）、`pnpm check:architecture`（0 findings）、受影响前端文件 Prettier/ESLint、Go 文件 gofmt 与 `git diff --check` 均通过；独立只读审查无阻塞发现。构建仍有既有大 chunk 提示；本轮未重跑全仓测试，未做真实付款/到账/公网回调或 Worker 消费验收，未提交 Git。

## 支付兑换码与个人充值入口（2026-10-17，实现已接入，部署与到账验收待完成）

- 本期已接入固定套餐等额充值、钱包/统一订单/流水、支付宝 Web/H5 配置与回调事实、订单巡检、兑换码生成/作废/兑换和个人钱包入口；这不代表部署与真实到账验收完成。不做退款、AI 扣费、预扣/冻结或管理员任意加款；钱包余额继续允许完整有符号 `int64`，充值只增加余额。
- P4 兑换码后端、交易来源 DTO、API 装配、管理员兑换码页面与个人钱包兑换入口已接入；管理员支持分页/批次/状态筛选、详情、精确查询、生成、作废和 CSV 导出，注册用户只获得独立钱包兑换动作，不获得管理页面动作。中英文 i18n、API/页面回归已补齐。
- 维护工具为 `docs/database/2026-10-17-payment-redeem-code-menu.sql/.ps1`。SQL 只写 Admin 菜单、动作、个人钱包兑换授权和注册用户 access version，并拒绝重复个人兑换 action。runner 要求显式 `-ServicesStopped`，public 目标额外拒绝其他客户端；支持 `-Schema`，对同一 schema 备份并完整读取校验，提交前持久化 before/after 事实与带 SHA256 的流式用户 manifest。Redis 菜单/用户 ready-state 按 500 条批量发布，精确解析 `int64` 后以原始 payload CAS，拒绝损坏、invalidating、更高版本或竞争更新，整批失败不部分写入。
- PostgreSQL 提交状态、Redis 同步状态、验证结论和失败阶段同步写入 `result.json` 与外层 `audit.json`；Redis 发布失败非零退出。维护者保持停服后，使用新审计目录及 `-RecoverFrom TARGET/result.json` 核对原数据库/schema、Redis endpoint、SQL、备份和版本 manifest，只恢复已提交的 Redis 发布，不重跑 SQL、不再次递增版本。提交结果未知时也按预提交事实核实，无法匹配则拒绝。`-CheckOnly` 在只读事务中验证逐 code 唯一性、目录/page/action 形状、父子关系、路由冲突、授权与 Redis 版本，既不修复数据库也不写 Redis。
- 2026-10-17 已在真实 `admin/public`、API/Worker 停止状态下完成 P1 配置契约复核、P2 七表、P2 支付页面/action 与注册用户个人入口、P3 `payment.order.reconcile`、P4 两张兑换码表和 P4 菜单/授权迁移；所有步骤均保留独立备份、SHA256、before/after 或 result 审计目录。P2 菜单使用 `docs/database/2026-10-17-payment-order-wallet-menu.sql`，P2 schema 审计目录为 `C:\Users\IGT\AppData\Local\Admin\maintenance\payment-deploy\20261017-111509-9ad8b1f0805a4653980e412b1271fd79`；P3/P4 runner 审计目录分别为 `C:\Users\IGT\AppData\Local\Admin\maintenance\payment-reconcile\010b2fafb1af407c8187a84e9b6d76f7`、`C:\Users\IGT\AppData\Local\Admin\maintenance\payment-redeem\3da2c3f0884c4eaa8a3ca92219fbe57e` 和 `C:\Users\IGT\AppData\Local\Admin\maintenance\payment-redeem-code-menu\20261017-032257-931`。P2 菜单提交后先按受影响的 2 个注册用户同步 Redis，P4 runner 再完成菜单版本 23/用户 ready-state 发布。
- 真实库最终只读事实：支付表 9 张、全部业务资金事实为空；Admin `menu_version=23`、支付配置 generation=2、订单巡检计划 1 条且有效、注册用户个人支付授权 9 条。迁移执行期间无其他客户端；收尾检查时发现 3 个维护者 `Navicat` 空闲连接，未强行断开。真实结构快照已从 `admin/public` 刷新至 `docs/database/current.sql`。生产/沙盒真实支付到账、回调验签、订单入账、兑换码发放和百万级容量仍未验收；未读取沙盒凭据文件，未启动/停止业务服务，未执行 Git 提交。
- 定向验证：`PAYMENT_ENVIRONMENT=1 go test -p 1 ./internal/database -run '^TestPaymentRedeemCodeMenu(Runner|Migration)' -count=1 -v` 通过，覆盖菜单 SQL 幂等、冲突回滚、缺失页面授权、重复个人 action，以及 runner 的 PG 提交/Redis 故障恢复、未知提交核实、错误目标/损坏状态拒绝、零用户、1001 用户多批 manifest、非幂等 replay 保护及 PowerShell wrapper 恢复/check-only。新增六项菜单结构损坏的只读拒绝测试先复现误报，修补后全部通过；独立只读复核确认该缺口已解决。PowerShell AST、`go fmt ./...`、`go vet ./...`、`go build ./...`、`git diff --check` 已有本轮通过记录。
- 全量验证：`PAYMENT_ENVIRONMENT=1 go test -p 1 ./... -count=1` 非零退出，77 个包通过、2 个包共 22 项顶层测试失败。支付全部模块、支付宝、金额、密钥及全部 permission 模块通过；剩余为既有 `internal/architecture/TestBackendSourceBoundaries`（`storage/file/service.go` 的 `ValidateDependencies` 分层检查）与 `internal/database` 21 项历史迁移/runner fixture（Canvas、旧配置缓存、邮件、短信、通知、字典、存储、调度、设置）。未修改这些无关模块，未将全仓标为通过；完整本地日志为 `server/tmp/payment-verification-20261017.log`。
- 前轮前端证据：`pnpm vitest run tests/api/payment/redeemCode.test.ts tests/views/payment/redeemCode.test.ts --pool=threads --maxWorkers=1`（2 文件/6 项）、`pnpm typecheck`、`pnpm check:architecture`、`pnpm build`、受影响文件 Prettier/ESLint 均通过；构建保留既有大 chunk 警告。本轮未修改前端，未重复运行这些命令。

## 支付订单与回调后端续建（2026-10-16，进行中）

- 继续维护者指定的 `2026-10-13-payment-plan.md` / `payment-spec.md`，沿当前工作区推进 P2，不自动提交、迁移业务库或启停服务；已有未跟踪订单、钱包、流水、套餐和支付宝适配器均保留。退款、AI 扣费、预扣和管理员任意加款不在本期范围，余额保持有符号精确金额。
- 本轮目标：修正订单/回调编译与事实分类、稳定去重、指定环境隔离、可信证据和成功入账的事务一致性；补 Provider 字面响应和输入边界测试；本人钱包/流水与管理字段隔离。数据流保持 Handler → Service → Repository → PostgreSQL，验签在 Service 调用指定历史凭据完成。当前已补齐本人钱包、充值中心、管理订单、管理钱包和管理流水页面，页面只消费后端投影，个人页不展示管理身份字段。
- 权限映射保持独立：订单管理 `payment:order:list/detail`、本人 `payment:recharge:list/detail/create/pay`，钱包管理 `payment:wallet:list/detail`、本人 `payment:myWallet:detail/list`；回调审计 `payment:callbackEvent:list/detail` 独立于订单读取。公开 Provider 入口 `/api/v1/payment/callback/alipay/:configId/:version` 不使用用户鉴权、平台头或浏览器 Origin。尚未实现的 sync/close 不暴露假能力，待 P3 共享租约/通道预算完成后接入。
- 初始验证：`go test ./internal/module/payment/order ./internal/module/payment/callbackEvent -count=1` 复现回调分页 orderID 的 int/int64 编译错误；订单集成测试连接本地 PostgreSQL 127.0.0.1:5432 被拒。先运行不依赖服务的回归，保留真实隔离事务与并发测试，环境不可用不得标为通过。百万账户容量假设沿批准 Spec；钱包单读 ≤1 SELECT、列表 count+page ≤2 SELECT、Redis 不可确认不回源猜配置；本轮不宣称容量或真实到账验收通过。
- 决策：用户要求沿现有工作区继续，因此不创建新 worktree、不移动未提交改动；长期进度只记本文件，不回写历史计划勾选。生产购买继续等 P3 恢复能力和完整验收后开放。
- 继续工作单元：P3 订单巡检已加入 `payment.order.reconcile` 内置 Scheduler 定义；订单 Repository 使用 `FOR UPDATE SKIP LOCKED`、15 秒 token 租约和 CAS 推进/关单，Reconciler 先查单并优先可信入账，过期未支付才尝试关单，失败保留下次核实时间。API/Worker 已显式注册任务，Worker 读取 `PAYMENT_ENVIRONMENT` 并使用历史凭据版本。尚未接入离线 schedule seed、Redis 通道共享限流和真实 PostgreSQL/支付宝回归；不新增兑换码或退款能力。

## 支付证书导入按钮置于字段标签右侧（2026-10-15，待前端验证）

- 支付配置新增/换版表单的应用公钥证书、支付宝公钥证书和根证书改为“标签右侧导入按钮 + 整行文本域”布局；文件名和错误提示保留在文本域下方。私钥仍保持纯文本域，导入行为、字段名和 `data-testid` 不变。
- 已更新布局回归，确认三个证书字段的导入控件挂载在对应表单标签内；本轮修改后待重新运行前端定向验证。
- 未修改支付 API、数据库、菜单/权限或后端；未做真实浏览器账号联调，未提交 Git。

## 支付配置分页重复校验移除（2026-10-14，定向验证完成）

- 按维护者确认的最小范围删除 `payment/config.Service.List` 的整段重复分页校验，直接编排 Count/List；沿用现有 `request.go` 与公共 `pagination.ParseRequest`。HTTP 分页契约不变：每页 1～100 条、页码可超过 100，offset 溢出仍由支付请求层检查；不新增解析器、字段或抽象，证书有效期逻辑暂不调整。
- 补请求边界回归，覆盖第 101 页、每页上限、缺失/空值/重复/零值/负值/非数字、整数与 offset 溢出。实际验证：`go test -p 1 ./internal/module/payment/... ./internal/shared/pagination -count=1`、同范围 `go vet`、`go build ./...` 及 `git diff --check` 通过。
- 未跑全仓测试、前端验证或浏览器验收；未修改数据库结构、菜单/权限、公共分页实现或服务状态，未提交 Git。运行中的后端需重启加载此改动。

## 支付证书日期校验移除（2026-10-14，定向验证完成）

- 按维护者要求删除证书 `NotBefore`/`NotAfter` 对支付配置新增、凭据换版、本地校验和启用操作的拦截，不再区分沙盒与生产。清理凭据校验函数、客户端/展示函数的无用当前时间参数、证书有效期 sentinel 和双语错误文案；管理动作不再因证书日期禁用。
- 保留证书日期元数据原样展示，以及 PEM/DER 解析、RSA 强度、证书用途、APPID/PID、私钥匹配、SDK 客户端初始化和响应验签行为。回归覆盖沙盒/生产、过期/未来/无效日期区间和支付宝官方历史证书包；新增和本地配置校验不因日期失败。
- 实际验证：凭据日期回归在移除前按预期因有效期错误失败；移除后 `go test -p 1 ./internal/payment/... ./internal/module/payment/... ./internal/shared/i18n ./internal/shared/response -count=1`、`go vet ./internal/payment/... ./internal/module/payment/... ./internal/shared/i18n ./internal/shared/response`、`go build ./...`、`gofmt -l` 与 `git diff --check` 通过。未修改数据库、菜单/权限或外部支付宝数据，未提交 Git。运行中的后端需要重启加载代码。

## 支付沙盒真实创建、查询与验签（2026-10-14，真实网关验证完成）

- 按维护者要求读取其指定的原始沙盒凭据 JSON，用当前 NewClient 和 GoPay v1.5.118 实际请求 `https://openapi-sandbox.dl.alipaydev.com/gateway.do`；探针强制仅访问该沙盒主机，使用标准 HTTPS 服务端校验、禁用重定向和调试内容输出。支付宝公钥叶证书真实到期时间为 `2026-03-14T03:25:43Z`，日期确已过，但本轮真实响应仍使用同一证书 SN，且可以用该证书的公钥通过验签，确认沙盒兼容规则并非只让配置保存。
- 首次 `alipay.trade.precreate` 返回 HTTP 200 / code 10000 和二维码，独立验签通过；未扫码/付款时 query/close 返回已验签的 `40004 / ACQ.TRADE_NOT_EXIST`，只证明预创建与查询接口签名可用，不能据此声称已经形成可查交易。预创建设置 1 分钟超时，没有执行付款。
- 维护者随后提供沙盒买家用户 ID，创建 0.01 元待付款交易 `CERTCHECK20261014133900a4d87630`：`alipay.trade.create` → 10000；首次 `alipay.trade.query` → 10000 / WAIT_BUYER_PAY；`alipay.trade.close` → 10000；再次 query → 10000 / TRADE_CLOSED。四次均 HTTP 200、SDK 自动验签通过、按原始响应单独进行 RSA2 验签通过、返回证书 SN 与本地一致，商户订单号/支付宝交易号前后对应。未调用支付、扣款或退款接口，订单关闭已真实确认。
- 实际命令：临时本地探针 `go test ./internal/payment/alipay -run '^TestLocalSandboxGatewayCertificate$' -count=1 -v` 两次通过，分别验证预创建与买家交易闭环；第二次网关调用耗时约 2.75 秒。凭据只从原文件读取，不复制私钥、买家 ID 或完整请求/响应到仓库和日志；临时探针已删除，仅新增本状态记录，未修改运行时代码、表/字段、业务数据库/Redis或服务状态，未提交 Git。本轮不重新运行全仓测试/构建，未验证支付成功到账、异步回调或生产环境。

## 支付沙盒过期公钥证书兼容（2026-10-14，后续已移除）

- 根因是当前自有 `ValidateCredentials` 对沙盒支付宝公钥证书也做严格到期拦截，早于 SDK 初始化和网关请求；SDK 沙盒/生产开关原本正确，本轮不修改环境映射。按维护者授权，仅兼容沙盒支付宝公钥叶证书过期，仍拒绝尚未生效、无效日期区间，应用/根证书以及生产支付宝证书保持严格有效期校验，格式、RSA 强度、用途、身份、私钥匹配和 SDK 验签不放宽。
- `server/internal/payment/alipay/credentials.go` 提供共享有效期规则，`server/internal/module/payment/config/response.go` 同步启用动作判断；新增、换版、本地校验与启用通过现有 NewClient 入口统一生效。原始证书、包内中间证书与实际到期时间保持不变，不加表、字段、菜单、权限或迁移；稳定规则已记录到 `design.md`。
- 回归先确认失败：沙盒过期公钥新增被拒绝、沙盒启用动作禁用，以及尚未生效证书错误显示可启用；最小实现后通过。补 11 项凭据时间/环境边界、10 项管理动作边界、隔离 PostgreSQL/Redis 的创建→本地校验→启用→换版生命周期；官方公钥双证书包沙盒 SDK 初始化通过，生产仍明确拒绝过期叶证书。保留此前三证书独立导入/完整替换与 PC/H5 网关选择回归。
- 使用维护者指定的原始沙盒 JSON 通过真实 Handler→Service→Repository 在自动清理的隔离 schema 新增，返回 HTTP 201 / code 0；随后详情启用动作、本地校验、启用、凭据新版本和再次校验均通过，同一凭据改为生产仍返回公钥有效期错误。仅从原文件读取，不将私钥或请求内容写入仓库/日志；临时本地探针已删除，未向支付宝发出请求。
- 实际验证：`go test -p 1 ./internal/module/payment/... ./internal/payment/alipay ./internal/shared/i18n ./internal/shared/response -count=1`、同范围 `go vet`、`go build ./...` 通过；`pnpm vitest run tests/views/payment/config/index.test.ts tests/api/payment/config.test.ts --pool=threads --maxWorkers=1`（2 文件/17 项）、受影响前端测试 ESLint/Prettier、Go 文件 gofmt 检查及 `git diff --check` 通过。
- 未跑全仓 Go/Vitest 测试、前端整构建或真实支付/到账/公网回调联调；未修改业务 public schema 或运行时 Redis DB0、未启停服务、未提交 Git，保留原有未跟踪目录。已运行的后端仍需重启加载新代码，本地 HTTPS 回调地址可保存不代表支付宝可从公网访问。

## 支付证书字段与 SDK 环境核对（2026-10-14，诊断与定向测试完成）

- 对维护者指定的本地请求 JSON 只提取公开证书元信息，使用 Windows X509Certificate2 独立解析：`appCertPEM` 为新申请的应用证书，2026-10-14 生效、2027-10-19 到期；`alipayCertPEM` 含支付宝叶证书（2026-03-14 到期）和中间证书（2024-08-01 到期）。截图所示 `appPublicCert.crt` 位于应用公钥证书栏，与请求的 `appCertPEM` 对应；不能把应用证书的有效期套到支付宝证书字段。
- 加强前端诊断回归：新增/换版使用三份不同内容并核对界面标签和提交字段，另测同一文件名重导入单证书完整替换旧双证书包。相关 2 文件/17 项通过，未复现字段互换、自动增加中间证书或拼接旧内容；测试只使用本地假证书结构，不保存维护者的凭据。
- 对照旧项目 `order_service.go` 与 `alipay/gateway.go`：旧版 `IsSandbox=(environment==sandbox)` 后传 `!IsSandbox`，当前 `environment=1` 对 SDK 同样传 `isProd=false`，`2` 传 true。直接通过当前 NewClient 与 GoPay 生成 PC/H5 支付 URL，沙盒均为 `openapi-sandbox.dl.alipaydev.com/gateway.do`，生产均为 `openapi.alipay.com/gateway.do`，四个组合通过；仅本地生成地址，没有发出下单或付款请求。
- 当前错误在自有 `ValidateCredentials` 返回，发生于 SDK 客户端初始化和任何网关请求之前；不是生产网关返回，也没有证据表明 SDK 环境传反。旧版没有同等的本地证书有效期拦截；沙盒策略是否兼容过期支付宝证书仍待维护者确认，本轮未放宽生产或沙盒校验。
- 实际验证：`go test ./internal/payment/alipay -run 'TestLocalValidationAndSDKContentMode|TestSDKPaymentURLsUseConfiguredEnvironment' -count=1 -v`、`go vet ./internal/payment/alipay`、`pnpm vitest run tests/views/payment/config/index.test.ts tests/api/payment/config.test.ts --pool=threads --maxWorkers=1`、测试文件 ESLint/Prettier 与 `git diff --check` 通过。只增加/加强测试与状态记录，未改运行时代码、数据库/Redis、服务状态或 Git 历史；未跑全仓测试/构建、真实浏览器文件选择与支付联调，保留原有未跟踪目录。

## 支付回调模板默认填入（2026-10-14，定向验证中）

- 旧项目实际使用固定公共接口 `POST /api/payment/callbacks/alipay`，管理员填写完整通知地址；当前 `{configId}/{version}` 是历史凭据定位设计，不是支付宝要求。当前项目尚无支付订单与回调处理，模板替换不代表回调已接通。
- 本轮只补现有配置交互：后端静态 options 在 `defaults.notifyUrl` 提供完整 HTTPS 模板，新增表单直接消费默认值，管理员只替换 HOST；语言切换不覆盖草稿，换版沿用当前配置地址。未替换 HOST 的示例地址由前后端校验拦截，中英文提示明确填写后端公网域名。
- 无数据库字段、迁移、权限或回调路由变更；保留原有工作区修改，未启动/停止服务或提交 Git。

## 支付配置新增失败定位（2026-10-14，只读诊断完成）

- 维护者反馈 `10006/认证平台服务暂不可用`；代码确认该错误从 Authenticate 调用认证平台 CurrentPolicy 返回，发生在支付 Handler 前。诊断时直连 API 与经前端代理各 3 次读取 Admin policy 均 HTTP 200；真实 PostgreSQL 只读事务确认 Admin policy_version=3、启用且未删除，Redis v2 policy 同为版本 3、ready、无 mutation token，当前代码读取验证通过。此前 10006 的具体底层原因未复现，无对应失败日志，不能据此断定原因。
- 真实 public 的 `payment_config.code` 已不存在，编码移除迁移已生效。维护者重试新增后返回明确 `10001/alipayCertPEM 已过期或尚未生效`，确认当前阻塞已进入支付凭据校验；此前提交的支付宝公钥叶证书到期日为 2026-03-14，需要重新下载有效证书。
- 本轮仅诊断，未修改运行时代码或认证规则，未写数据库/Redis、启停服务或发送实际支付请求；临时只读探针已删除。回调接口实现及分页代码调整未在本轮展开。

## 支付配置字段错误与证书输入修复（2026-10-14，代码与自动化验证完成）

- 定位创建请求的首个失败为 `notifyUrl=http://abc` 不满足已确认的 HTTPS 回调模板。配置模块对回调地址、APPID/PID、私钥格式、三类证书格式/有效期、应用身份和密钥匹配返回固定双语字段提示；保留 HTTP 400、`code=10001`、`data=null`，不回显输入或第三方解析错误。创建、更新凭据与本地校验共享映射，取消/超时保留依赖错误语义。
- 私钥与三类证书使用 Element Plus 自动高度文本域，三类证书保留本地 CRT/PEM 导入按钮；通过 `el-upload` 关闭自动上传，仅读取浏览器文件，清理临时文件列表。保留空白凭据沿用与导入失败不覆盖草稿，补回调模板即时校验与中英文 placeholder；私钥文案明确支持 PEM/原始 Base64。
- 实际验证：新增后端回归先复现通用错误，前端回归先复现无效回调提交和旧导入控件；修复后 `go test -p 1 ./internal/module/payment/... ./internal/payment/alipay ./internal/shared/i18n ./internal/shared/response -count=1`、同范围 `go vet`、`go build ./...` 通过。`pnpm vitest run tests/views/payment/config/index.test.ts tests/i18n/index.test.ts --pool=threads --maxWorkers=1`（2 文件/22 项）、受影响文件 ESLint/Prettier、`pnpm check:architecture`（0 findings）、`pnpm build`（含 vue-tsc）、`git diff --check` 通过；构建保留大 chunk 提示。
- 用户粘贴的私钥末尾 `=a` 不符合 Base64，支付宝公钥叶证书到期日为 2026-03-14；需重新完整复制私钥、下载有效证书并填写 `https://HOST/api/v1/payment/callback/alipay/{configId}/{version}`。未保存用户私钥、执行真实库迁移或服务启停，未跑全仓测试或浏览器真实账号/支付验收；本轮不新增迁移，不提交 Git。

## Canvas 参考前端迁移收尾（2026-10-14，代码与前端验证完成）

- `canvas` 直接复用 `D:\github-project\infinite-canvas\web` 的画布、项目、素材、图片/视频生成、提示词、插件和本地 AI 渠道；按相对路径核对参考项目 174 个 src 文件与 9 个 public 资源，均无缺失。参考项目未修改，AI 渠道和画布媒体仍保留参考项目的本地配置/IndexedDB 行为，供应商管理和画布媒体 COS 迁移未实施。
- 收尾修正 `pnpm dev` 覆盖成 3000 的问题，使用 Vite 的固定 16302 端口；`pnpm build` 包含真实 `tsc --noEmit`。补齐 Vitest/jsdom/Testing Library，参考项目的三个纯逻辑 Bun 测试改用 Vitest，纯逻辑与浏览器测试分别使用 node/jsdom、单 worker；pnpm 原有未填写的构建策略改为 esbuild=true、parcel watcher=false，安装成功。
- 登录样式恢复为页面范围内的 `login.scss`，不全局导入会覆盖参考主题的旧 index.css；个人中心样式限定在 `.profile-page`，补主题变量、表单颜色和实际内容区域滚动。修正复制后的 Streamdown Tailwind 扫描路径。个人中心入口只有显式 `user:profile:view` page 授权后才显示；退出防重复，失败由请求层通知并清除本地会话，不产生未处理 rejection。登录凭据错误保留内联，其他请求错误不再重复显示。README 补 Canvas 启动与验证命令，architecture 移除已不存在的工作区组件说明。
- 实际验证：新增入口/退出回归先复现 2 项失败和未处理异常，再修复后 3/3 通过；登录错误回归先复现重复内联，修复后 4/4 通过。最终 `pnpm test` 11 文件/55 项全部通过，`node_modules\.bin\tsc.cmd --noEmit` 与 `node_modules\.bin\vite.cmd build` 通过，受影响文件 Prettier 和 `git diff --check` 通过；构建仍保留参考项目的大 chunk、静态/动态混合导入提示。Playwright 检查桌面与 390px 手机宽度：登录样式生效、可滚动、主题变量未污染参考主题，后端不可用时登录按钮禁用。
- 真实账号/权限/COS/AI 联调尚未完成：本轮直连 localhost:16301 被拒绝，确认没有 API listener，Canvas 代理返回空 500；未代维护者启动/停止后端。未改数据库、Redis、菜单、角色或用户授权，未执行后端测试或 Git 提交，保留其他 AI 的支付和文档改动。

## 支付配置编码移除（2026-10-14，代码与隔离验证完成，真实库升级待执行）

- 按维护者确认移除支付配置 `code`：新增表单、DTO、Model、静态 limits、列表/详情、搜索与缓存 catalog 均不再提供内部编码，不改成自动生成字段。配置继续使用 ID 关联；搜索仅匹配配置名称、APPID、PID，API envelope 与菜单权限中的 `code` 保持各自原语义。
- 独立 forward migration `docs/database/2026-10-14-payment-config-remove-code.sql/.ps1` 验证旧/新表的列、CHECK、PK/FK、索引、trigger 和 guard 定义，删除编码列、CHECK 与部分唯一索引，并更新身份 guard；首次升级同事务只推进一次 `payment.config/global` generation 和 outbox，重放不再改变事实。旧 SQL/runner 保持原样；新 runner 备份/哈希、记录提交前预期和数据摘要，只条件同步支付 Redis state，支持 `-RecoverFrom`，不改菜单、授权或用户数据，不清 Redis 全库。
- 实际验证：先运行前端无编码输入与 PostgreSQL 无编码列断言，均按预期失败；实现后 `pnpm vitest run tests/views/payment/config/index.test.ts --pool=threads --maxWorkers=1`（10/10）、`pnpm typecheck`、支付相关 ESLint/Prettier、`pnpm check:architecture`（0 findings）、`pnpm build` 通过。`go test -p 1 ./internal/module/payment/... -count=1`、`go vet ./internal/module/payment/...`、`go build ./...` 通过；迁移测试覆盖保留旧配置/凭据、名称/APPID/PID 搜索、回放、视图依赖与 outbox 冲突回滚、Redis 发布失败及恢复。删除字段后调整缓存边界夹具为 1100 条，仍验证凭据版本增长超限时事务回滚。PowerShell 语法与 `git diff --check` 通过。
- 最后通过 `psql --no-psqlrc --set=ON_ERROR_STOP=1` 的只读事务确认真实 public 的编码列仍存在、payment generation=1；本轮未执行真实 DDL、Redis 同步、服务启停或 Git 提交，未导出 current.sql，未跑全仓测试或浏览器真实账号/支付验收。按 AGENTS.md 由维护者停止 API/Worker 后执行 `./docs/database/2026-10-14-payment-config-remove-code.ps1 -ServicesStopped`，成功后启动新版服务。保留并行 Canvas 改动。

## Canvas 请求层统一响应与错误通知（2026-10-14，代码与自动化验证完成）

- Canvas `src/utils/request.ts` 统一处理精确 `code/data/message` envelope、HTTP/网络/取消/协议错误、401 刷新、匿名探测、登录凭据内联例外和 RBAC 错误通知；不再把代理返回的 HTML/空响应泄露为 `API response must be JSON`。500、网络、协议、403、404、429 等均映射为用户可读语义，取消请求不通知，并对共享刷新失败去重一次。
- 新增 `HttpError` 与 Sonner `Toaster`；请求层支持显式 `successMessage`，成功响应由请求层统一解包并可统一提示。登录、个人资料、身份绑定和改密页面移除接口错误重复展示，只保留登录凭据内联、表单校验和重试/局部状态。
- 实际验证：`pnpm exec vitest run tests/utils/request.test.ts --pool=threads --maxWorkers=1`（13 项）、受影响登录/个人中心测试（3 文件/16 项）、`pnpm typecheck`、`pnpm test`（8 文件/45 项）、`pnpm build` 均通过，`git diff --check` 通过。未做真实浏览器账号验收；未改数据库、权限、上传规则或 Redis，未提交 Git。

## Canvas 个人中心前端与共享接口接入（2026-10-13，代码与自动化验证完成）

- 个人中心位于 `canvas/src/views/user/profile`，按 `views -> api/user -> utils/request` 接入资料读取/保存、后端性别选项、首次设密、原密码改密、验证码改密和邮箱/手机绑定换绑；换绑提交当前身份与新身份两份 proof，不通过资料接口修改身份。
- Access Store 与动态路由接入隐藏根 page `user:profile:view`，工作台仅在获得页面授权后显示个人中心入口；读取、保存、改密及身份动作独立检查，不由 action 推导页面。头像使用共享签名上传/完成/预览接口，资料保存保留 adoption request key 以便失败重试；规则固定 `avatar`，按 Canvas 平台查找，不回退 Admin 或通用 `canvas` 规则。
- 收尾复现并修复原密码改密迟到响应清除新登录的问题：只在组件仍挂载、仍为 authenticated、用户 ID 和 access token 均与请求开始一致时清除会话并跳转登录；回归覆盖离开页面、切换账号、同账号新会话及正常改密跳转。
- 实际验证：`pnpm exec vitest run tests/views/user/profile/flows.test.tsx --pool=threads --maxWorkers=1`（9 项）、`pnpm test`（8 文件/38 项）、`pnpm build`（含 `tsc --noEmit`）均通过；后端 `go test ./cmd/api -run '^TestCanvasPersonalRoutes' -count=1`、`go test ./internal/module/user/profile ./internal/module/user/email ./internal/module/user/phone -count=1`、`go vet ./cmd/api ./internal/module/user/profile ./internal/module/user/email ./internal/module/user/phone`、`go build ./cmd/api` 均通过，`git diff --check` 通过。
- 未做浏览器真实账号验收、真实邮件/SMS 发送或 COS 上传；Canvas avatar 规则与角色授权由维护者自行配置。本轮未改数据库、菜单、用户/角色授权、上传规则或 Redis，未提交 Git，保留并行支付等改动。项目库、无限画布与生成工作流迁移尚未完成，不将个人中心交付等同于参考项目功能全部搬运完成。

## Canvas 个人中心菜单补齐（2026-10-13，仅菜单数据完成）

- 按维护者截图反馈，补 Canvas 隐藏根 page `user:profile:view`（id=4289，`/user/profile`、`user/profile`、`layout.user.profile`），下挂读取资料、修改资料、修改密码、绑定/换绑邮箱、绑定/换绑手机 5 个 action（id=4290–4294）；权限码沿用 Admin 的 `user:profile:detail/update`、`user:password:update`、`user:email:update`、`user:phone:update`。不挂在 Canvas 工作台下。
- 仅写 `permission_menu` 和 Canvas `menu_version`；未新增或修改角色/用户授权、用户 access version、上传规则，不操作 Redis。维护者自行授权并重建缓存。执行时 Canvas menu_version 4→5，Admin 保持20；同事务重复执行无变化，无关事实摘要保持 `41eee28752428e21c46fc151fad2ac69`。
- 新增 `docs/database/2026-10-13-canvas-profile-menu.sql` 与 `server/internal/database/canvas_profile_menu_migration_test.go`；`go test ./internal/database -run '^TestCanvasProfileMenu' -count=1` 通过，覆盖节点形状、幂等、路由/动作父节点冲突回滚及不触碰授权/上传配置。真实菜单备份完整读取通过，事务执行与重复核验成功；审计目录 `%LOCALAPPDATA%/Admin/maintenance/canvas-profile-menu/20261013-165952-008`。
- 本轮未实现 Canvas 个人中心前端或接口联调，菜单存在不代表页面功能已完成。维护者已明确停止 Redis 收尾；之前 auth-upload runner 的提交确认、批量租约和 Redis 重启恢复审查问题尚未修复，不再复跑该 runner。合法多角色校验回归已通过，但重启恢复回归仍失败；不将旧执行成功记录当作维护程序故障边界全部通过。未提交 Git，保留并行支付等改动。

## Canvas 通用权限容器与独立上传规则（2026-10-13，正常迁移完成；Redis 收尾已按维护者要求停止）

- 按维护者确认新增 Canvas 平台隐藏根 page `canvas:workspace:view`，路径 `/canvas/workspace`、componentPath `canvas/workspace`、i18nKey `navigation.canvasWorkspace`；其下只新增现有共享 action `storage:object:upload`。不新增上传页面、不改通用 RBAC 规则，保留原 `canvas:test:view/button` 节点和所有无关授权。Canvas 工作台组件移至对应 `canvas/src/views/canvas/workspace/index.tsx`，补 Admin 中英文目录文案。
- `registered_user` 只授予 Canvas upload action，不授予容器 page；双实例 Access Service/Redis client 验证 action 进入 `permissionCodes`，父 page 不进入 `menuTree`。上传凭证与上传完成接口继续使用既有权限，身份、规则和对象平台由后端校验。个人资料页面、Canvas Access 前端接入与实际上传交互不在本轮交付内。
- 新增 Canvas 独立上传规则（id=5、platform_id=2、编码 `canvas`），首次 seed 复制 Admin 活动 `file` 规则的 COS 配置 id=1、104857600 字节限制、扩展名/MIME 与 public 模式；后续两平台规则独立编辑，不回退到 Admin，不复跑覆盖已编辑规则。无 DDL、无新表、无 COS I/O，schema-only `current.sql` 本轮无需刷新。
- 交付 `docs/database/2026-10-13-canvas-auth-upload.sql/.ps1`：写前完整归档备份/哈希、短锁等待、单 PostgreSQL 事务、菜单和有效用户 tokenized Redis 失效租约与续期、提交后条件发布；部分失败审计明确记录提交状态。`-RecoverFrom` 只接受本迁移审计，校验数据库、Redis 实例与 SQL 哈希，拒绝活跃租约/异常版本，不清 Session、额度或 Redis 全库。
- 真实迁移于 2026-10-13 15:57 +08:00 成功，随后重复执行成功，两个审计均 `PostgresCommitted/RedisSynchronized/Verified=true`。page/action id=4287/4288，Canvas menu_version 1→2，Admin 保持19，policy_version 保持3/1；有效普通用户67/718的 access version 21→22、3→4，复跑不递增。无关菜单/规则/授权全行摘要前后保持一致；未停止或重启 API/Worker。
- 真实备份与审计：`%LOCALAPPDATA%/Admin/maintenance/canvas-auth-upload/20261013-155637-336`、`20261013-155924-541`；首次备份 SHA256 `237685C89AA3CB7A595CA6952837BC69169FE45B578A3F108FA9B43903581623`。临时数据库与 Redis DB15 独立验证完整 runner 首次/重复执行、状态丢失后恢复及双实例 Access，生产 Redis DB0 未清理。
- 实际检查：新增 Go 测试 gofmt；`go test ./internal/database -run '^TestCanvasAuthUpload' -count=1`、`go test ./internal/module/permission/access ./internal/module/permission/state ./internal/module/storage/uploadRule -count=1`、同范围 `go vet`、`go build ./cmd/api ./cmd/worker` 均通过。Canvas `pnpm test`（3文件15项）、`pnpm build`（含 tsc）通过；Admin `pnpm vitest run tests/i18n/index.test.ts --pool=threads --maxWorkers=1`（10项）、`pnpm check:architecture`（0 findings）、`pnpm typecheck` 通过。未运行全仓测试、真实账号上传/头像/COS 验收或 Admin 全构建；未提交 Git，保留并行支付/money/secretkey 文档与代码改动。

## 当前工作：支付体系分步建设（2026-10-13，支付配置代码与定向验证完成，真实迁移待执行）

- 当前仅实施 `payment/config`：逻辑渠道与不可变凭据版本、Admin 配置页面和独立 page/actions、维护者离线迁移；不展开套餐、订单、回调、钱包、AI 或 Canvas。工作区中已有并行 Canvas 改动保留。
- 当前配置切片已完成：`server/internal/module/payment/config` 分层实现、`server/internal/payment/alipay` GoPay v1.5.118 内容模式、独立 payment 派生密钥/AES-GCM、Admin `web/src/views/payment/config` 与 typed API、双语文案、配置缓存状态和脱敏操作日志；菜单路径/组件为 `/payment/config` / `payment/config`，page `payment:config:view` 与 `list/detail/create/update/status/delete/test` actions 独立，新版本发布使用 update，不自动授予注册用户管理动作。
- 按旧项目恢复三证书 CRT/PEM 文件导入和空白凭据保留；支持完整混合根包、含中间证书的叶文件、PKCS1/PKCS8 PEM 或原始 base64 私钥。证书 SN 由 SDK 选择，本地检查只验证输入、有效期、身份与 key-match，不将客户端初始化误报为实时连通/付款成功。保留已批准字段，不引入证书路径、证书管理表或 AI 字段。
- `payment.config/global` 复用 generation/outbox 与 mutation lease；目录和指定凭据版本快照 256KiB、TTL 5 分钟，Redis 命中不读 PG，读取/follower 500ms、获准 leader 4 秒；共享 Redis socket I/O 尊重 context deadline。写前与 Redis 使用同一 envelope 编码预算，版本位数增长超限也事务回滚；损坏/缺失/null/重复字段及错误 notify URL 闭合修复，双实例代际竞争不得返回旧 repair 快照。
- 交付维护者执行的 `docs/database/2026-10-13-payment-config.sql/.ps1`：两表、菜单目录/page/actions 与 generation seed；精确检查 identity/default、列/CHECK、PK/FK、活动唯一索引、trigger/function 漂移；backup/hash 与 pre-COMMIT Expected/outcome 审计，未知提交沿新连接/恢复入口核对，endpoint/DB 身份不绑定 Redis run_id；恢复/回放要求实际支付 schema 存在且禁止临时首次建表假验证。不新增支付定时任务，提交后 outbox 使用既有 Worker relay。
- 最新定向验证 `go test -p 1 ./internal/module/payment/... ./internal/payment/alipay ./internal/secretkey ./internal/redis ./internal/shared/cacheFill ./internal/shared/configCache ./internal/module/permission/state ./internal/module/permission/access ./internal/module/system/operationLog ./internal/module/system/configCacheStatus ./cmd/api -count=1` 全部通过；额外并发发布测试 `-count=3` 通过。集成使用隔离 PG schema/测试 Redis DB，覆盖首次/重放、漂移、回滚、跨实例、committed-pending/通用 relay、本地慢 RESP、runner ACL 发布失败与恢复、模拟提交确认丢失及 run_id/状态丢失、缺 schema 恢复拒绝；没有实际重启 Redis 或故障注入真实 COMMIT 链路。
- 后端 `go vet ./...`、`go build ./...` 通过。`go test -p 1 ./... -count=1` 未全绿：storage/file 既有 Service 分层门禁、architecture/database 历史迁移 fixture 及并行 Canvas runner 失败；支付并发用例在全仓负载下出现固定等待预算耗尽，现只允许明确竞争错误后的幂等重试，定向 3 次及最终相关范围通过，不扩大生产超时。未修无关失败或宣称全仓通过。
- 前端 `pnpm vitest run tests/api/payment/config.test.ts tests/views/payment/config/index.test.ts tests/i18n/index.test.ts --pool=threads --maxWorkers=1` 3 文件 22 项通过；`pnpm typecheck`、受影响 Prettier/ESLint、`pnpm check:architecture`（0 findings）、`pnpm build` 通过，保留既有富文本大 chunk 提示。补证书导入错误不覆盖草稿、确认中权限撤销/重复操作、committed-pending 刷新但不伪报成功；未运行全仓前端测试或真实浏览器账号验收。
- PowerShell 语法检查通过；最终 `psql --no-psqlrc --set=ON_ERROR_STOP=1` 的 `BEGIN READ ONLY` 确认真实 public 无两张支付表、支付菜单为 0、Admin/Canvas menu_version=20/5。未执行真实支付 migration、未导出 current.sql、未启停服务、未 Git 提交；先由维护者执行本配置迁移并验收，再按一步一资源继续固定套餐，尚未实现支付下单/回调或 AI。
- 追加 UI 术语与 CRUD 规范：支付配置按支付宝官方截图使用 `APPID`、`绑定的商家账号（PID）`，不显示含义不准确的“应用 ID/商户 ID”；新增、换版、证书、私钥、URL、筛选关键字均补齐中英文 locale placeholder。`$admin-crud` 已加入 placeholder 与官方协议术语的强制检查项，并要求对应前端回归覆盖。
- 按维护者确认移除支付配置内部编码：创建表单、API DTO、Model、缓存快照、列表搜索和详情均不再使用 `code`；配置以数据库 ID 关联，搜索保留配置名称、APPID、PID。新增 `docs/database/2026-10-14-payment-config-remove-code.sql` 处理已落地旧表的幂等删除列、索引、CHECK 和 guard 更新；已执行的旧迁移与 runner 保持原样。真实库已只读确认当前仍有 `payment_config.code`，尚未执行升级迁移。

- 维护者明确先完成支付体系，再建设 AI 体系；当前施工只覆盖支付，不并行新增 AI 模块或修改 Canvas。
- 参考旧 `admin_back_go/internal/module/payment`、`payment/wallet`、`internal/shared/money` 与 `admin_front_ts` 支付页面。旧钱包以 `user_id` 唯一归属；沿用同一账户一个钱包的业务基础，订单和流水记录来源认证平台，不自行建立每个平台一份余额。
- 维护者已确认支付方案，并补充允许负余额；后续按支付配置/套餐 → 钱包/统一订单/回调/个人入口 → 巡检恢复/关单 → 兑换码 → 全体验收逐步施工。新增或改变已批准字段和参数仍须先说明，不并行扩大到 AI 或 Canvas。
- 当前一步契约：`server/internal/shared/money` 只负责金额表示与精确转换；钱包采用人民币 8 位小数的 int64 units（1 元=100000000 units），支付通道金额采用整数分（1 分=1000000 units）。HTTP 钱包金额使用十进制字符串，不用 float、JavaScript number 或静默舍入传递余额；支付通道字符串固定两位小数。
- 本步接口：`ParseRMBUnits`、`FormatRMBUnits` 现支持有符号余额和完整 int64 范围；`CentsToUnits`、`ParseRMBCents`、`FormatRMBCents` 仍只转换非负通道金额，所属业务模块拒绝零充值面额。非法格式、超出精度及 int64 溢出返回明确错误。模块不写钱包、不持久化、不新增 API/权限或数据库表。
- 容量与验证：沿用百万用户、多实例基线；金额转换不访问 PostgreSQL/Redis，不建立缓存。钱包写入后续必须使用 PostgreSQL 行锁、来源唯一约束及同事务账本，不把 Redis 当余额事实；配置后续复用 generation/outbox 和有界冷回源。每阶段核对查询预算与双实例并发、幂等及失败回滚，本步先覆盖最小金额、最大金额、畸形输入、精度与溢出。
- 工作区原有 `docs/agent/STATUS.md` 修改与未跟踪 `canvas/` 由其他工作产生，保留；不自动提交或启动/停止后端。当前未执行生产数据库迁移，未宣称支付功能已上线。
- 本步已完成金额实现与 7 个定向测试（含表驱动边界用例）；先运行 `go test ./internal/shared/money -count=1`，因金额生产包缺失失败，再实现并通过。实际执行 `gofmt -w internal/shared/money/units.go internal/shared/money/units_test.go`、`go test ./internal/shared/money -count=1`、`go vet ./internal/shared/money`、`go build ./internal/shared/money` 均 exit 0。
- 维护者明确本轮先写计划，并确认固定套餐、等额入账。已创建忽略的本地评审文件 `docs/local-work/2026-10-13-payment-spec.md`、`docs/local-work/2026-10-13-payment-plan.md`，仅本支付任务允许后续读取，不扫描其他本地计划。推荐核心 7 表、兑换码另加 2 表，合并旧充值单/支付订单，保留历史凭据版本；钱包/流水不预加 AI 冻结/消费字段。各表逐列用途、类型/空值、FK/CHECK/索引、权限与任务预算均在草案，尚未创建真实对象。
- 真实 PostgreSQL 已通过 `psql --no-psqlrc --set=ON_ERROR_STOP=1` 的 `BEGIN READ ONLY` 查询核对 pg_catalog、information_schema 与现有配置：`admin/public` 无 payment/ai 表；Admin/Canvas id=1/2、menu_version=19/1，已有 4 个有效 Scheduler 计划。SDK 缓存源码核对 GoPay v1.5.118 支持证书内容初始化/验签，不需要共享本地证书路径；本轮没有 DDL、数据/Redis 写入或服务启停。
- 建议只新增 `payment.order.reconcile` 一个周期巡检计划，复用现有 schedule/job/run；六个支付管理页面加独立“我的钱包/充值中心”，回调详情放订单内。默认角色仅获得个人 page/actions，管理动作不自动授予；首次离线菜单/角色增量和 Redis 版本恢复必须可审计幂等。订单30分钟、巡检每分钟及通道/租约预算是待评审值，不是已启用配置。
- 维护者进一步明确：支付通道最多 Alipay/WeChat Pay 两种，provider、environment、pay_method 等封闭枚举统一数字。已逐列审查九表及 Go/API/选项草案：provider=1支付宝/2微信，environment=1沙箱/2生产，payMethod=1PC网页/2H5；payMethods 改 SMALLINT[]，回调 error_code 改 SMALLINT NULL，流水来源和内部确认来源用具名数值类型。原有订单/回调/兑换码状态保留数值，过期投影固定为4而不写库。外部原始 tradeStatus、订单/商户标识、技术 key 和 ISO币种保留字符串并解释用途；后端统一 options/校验/SDK映射，前端消费数字，不维护第二份值域。微信2只分配稳定标识，首期拒绝未实现配置和调用。本轮仍仅修正文档，未执行迁移或继续业务代码。
- 本轮类型一致性补查覆盖数据库列、Go 输入与 DTO、静态 options、指纹、SDK分支和错误投影；payMethods 以 SMALLINT[] 精确集合 CHECK 拒绝空/NULL/多维/重复/未知值，写前按数字升序规范化。JSON body 枚举只接受数字，GET query/环境变量文本经严格整数解析后校验。SDK 核对 seller_id 是真实收款商户标识，保留其字符串身份字段与回调校验，不因旧适配器未使用就删除。
- 本次验证：支付草案中的旧 VARCHAR/TEXT[] 枚举与 Go string 输入声明已定向核对退出，`git diff --check` 通过；真实 PostgreSQL 只读 VALUES 验证 payMethods 集合约束，11 个合法/非法样例全部匹配预期（含空、重复、NULL、多维、未知值）。仅校验表达式，未建表/约束或运行实现测试，不将文档检查视为业务完成。
- 维护者允许余额为负数：已同步本地 spec/plan 与 design.md 的稳定产品约定，取消钱包/历史余额非负限制，去掉“累计充值等于余额”的推断；充值按原余额加正额，前余额允许负数，不加欠款、授信或透支开关字段。负余额不自动定义未来 AI 消费准入或无限透支。
- 金额代码本轮完成失败测试→修复→通过：`gofmt -w internal/shared/money/units.go internal/shared/money/units_test.go`、`go test ./internal/shared/money -count=1`、`go vet ./internal/shared/money`、`go build ./internal/shared/money` 均 exit 0；新增负余额/MinInt64/负数溢出与往返用例。`go test ./internal/shared/money -fuzz '^FuzzRMBUnitsRoundTrip$' -fuzztime=5s -parallel=1` 通过，执行 33784 次。真实 PostgreSQL READ ONLY VALUES 校验 8 个负余额充值/归零/转正/int64 边界样例，8/8 符合预期，仅表达式验证，无 DDL 或数据写入。
- 下一步按已批准计划实施 P1 配置/套餐。本轮只修改金额包和相关计划/产品/状态文档，保留 Canvas、前端文案及其他现有改动；未建支付表、未执行生产迁移、支付宝真实支付、全仓测试/构建或 Git 提交。此前全仓架构门禁与迁移 fixture 失败仍需核实，不由金额定向验证覆盖。

## Canvas 前端第一阶段 Auth 重建（2026-10-13，代码与联调验证完成）

- 按维护者确认在 `canvas/` 独立重建 React 19 + Vite 前端；本阶段只接入 Canvas 平台认证和工作台入口，不恢复旧 Canvas 代码。后续参考 `D:\github-project\infinite-canvas` 的业务功能将按模块搬运；MCP/本地 Agent 暂不纳入本阶段。
- 新增严格 HTTP envelope 请求层、Canvas `X-Auth-Platform`/`X-Device-ID` 头、内存 access token、HttpOnly refresh cookie、并发 401 refresh 合并、终态会话清理和错误透传。新增密码/邮箱/手机登录策略消费、滑块验证码、验证码倒计时、退出登录和响应式登录页；认证成功后进入工作台占位页。
- AI 渠道当前不接 Admin 供应商接口，后续仍按参考项目本地配置搬运；Admin 供应商功能完成后只替换 Canvas 配置 API/store，不改变认证边界。
- 本地开发默认使用 Vite `/api` 代理；因 API 当前 `RequireOrigin` 白名单仍是 `http://localhost:16300`，代理将上游 Origin 固定为该值。生产部署必须把实际 Canvas Origin 加入 API 的 CORS/Origin 配置，不能依赖本地代理。
- 实际验证：`cd canvas; pnpm test`（3 个文件、15 项通过）、`pnpm typecheck` 通过、`pnpm build` 通过；Playwright 真实浏览器验证 `http://localhost:16302/login` 页面渲染、`/api/v1/auth/login-config` 经代理返回 200、Canvas 请求头正确、未登录 refresh 返回 401 后停留登录页。未使用真实账号完成登录，未修改后端、数据库或权限；开发服务器由 Agent 启动，交付前保持运行于 `http://localhost:16302/`。
- 追加修复登出请求边界：`/api/v1/auth/logout` 现在携带当前 Canvas access token，但仍禁止自动 refresh；服务端会话撤销失败时前端仍在 finally 清理本地状态。回归测试覆盖 Bearer 头、无 refresh 和退出后状态清理。
- 按维护者反馈将 Canvas 前端目录对齐当前 `web` 契约：API 使用 `src/api/auth/login.ts`，认证常量使用 `src/auth`，请求层使用 `src/utils/request.ts`，状态使用 `src/store`，路由使用 `src/router/index.tsx`，页面使用 `src/views`，HTTP 错误类型使用 `src/types/http.ts`；测试镜像到 `tests/utils`、`tests/store`、`tests/views`。删除 `services`、`stores`、`pages`、根级 runtime 目录，未改变认证行为。

## 我的文件预览交互整改（2026-10-13，代码与定向验证完成）

- 图片预览改为与 `UpMedia` 一致的全屏 `el-image-viewer`，提供缩放、适应、旋转和下载工具栏，不再使用居中的 `AppDialog` 图片预览。
- 视频/音频继续使用媒体控件；PDF、文本及其他普通文件不显示预览入口，只保留下载和删除，避免 COS 响应头或浏览器内嵌策略导致空白弹窗。
- 图片查看器回归验证全屏模式与签名 URL；普通文件回归验证隐藏预览、保留下载且不请求 inline 签名。复用文件选择弹窗与上传文件库回归同时通过；未改后端接口、数据库、权限或 COS 协议。
- 实际验证：`pnpm vitest run tests/views/storage/file/index.test.ts tests/components/FileSelectDialog.test.ts tests/components/UpMedia/fileLibrary.test.ts --pool=threads --maxWorkers=1`，3 文件 26 项通过；受影响文件 ESLint、Prettier、`pnpm check:architecture`（0 findings）、`pnpm build`（含 vue-tsc 类型检查）和 `git diff --check` 通过。构建保留既有富文本大 chunk 提示。
- 未启动后端、未运行全仓测试或真实浏览器/COS 视觉验收，未提交 Git；维护者刷新前端后可验收全屏图片查看器，以及普通文件的下载、删除操作。

## 云服务“我的文件”卡片网格（2026-10-13，代码与定向验证完成）

- 按维护者确认将“我的文件”从表格改为卡片网格；保留 `AppSearch`、上传、刷新、预览、下载、删除、真实分页、搜索、权限和文件选择弹窗，不改后端接口、数据库或权限协议。
- 图片/视频使用缩略区域，普通文件使用文件图标；卡片显示文件名、类型、大小和上传时间，操作区桌面端悬停显示、触屏常显，删除继续二次确认。分页、权限撤销、组件卸载和翻页会取消缩略图请求，最多并发 4 个且不让旧请求污染新页面。
- 卡片选择模式支持整卡点击及键盘 Enter/Space；操作按钮阻止选择事件冒泡。刷新按钮使用文件页独立定位符，操作按钮保持纯文字且无装饰图标。样式集中在 `FileBrowser.scss`，使用现有主题变量和响应式布局。
- 实际验证：`pnpm vitest run tests/views/storage/file/index.test.ts tests/components/FileSelectDialog.test.ts tests/components/UpMedia/fileLibrary.test.ts --pool=threads --maxWorkers=1`，3 文件 24 项通过；`pnpm typecheck`、受影响文件 ESLint、`pnpm check:architecture`（0 findings）、`pnpm build` 和 `git diff --check` 通过。构建保留既有富文本大 chunk 提示。
- 未启动后端、未运行真实浏览器/COS 视觉验收、未修改后端/数据库/RBAC，未提交 Git；维护者刷新前端即可查看卡片布局。

## 上传暂存清除、系统设置媒体类型与邮件校验修复（2026-10-12，代码、真实迁移与定向验证完成）

- 按维护者最新要求覆盖上一轮清除约定：UpMedia 删除/清除只同步移除本地待保存选择，不弹二次确认、不调用保存或文件删除接口，不影响“我的文件”原件；文件库真正删除仍保留确认。覆盖单/多值、个人头像、设置媒体及禁用/上传中交互。
- 系统设置值类型拆为 5=媒体-文件（按钮和附件行）、6=媒体-图片/视频（正方形）；默认头像使用 6 且仅允许图片，Excel 模板保持 5。后端拥有类型校验、双语 options、展示形态和格式限制，前端按 DTO 展示；混合媒体由公共组件按格式选择图片/视频预览元素。不增加表、字段、权限或上传热路径查询；沿用百万用户/多实例下既有配置版本与 outbox 失效协议，不清 Redis、不逐行访问 COS。
- 邮件配置误报根因是 Element Plus 在异步 options 或语言变化导致 rules 更新时自动校验尚未回填的表单。已关闭规则变化自动校验，配置回填后清除旧校验，保留保存/失焦规则；真实组件回归验证合法多级域名邮箱、中文别名和 TTL=5，以及无效草稿切换语言后保存仍被拦截。另修复 Vitest 中 async-validator 的 CJS/default 互操作造成无效输入漏检，测试环境内联 Element Plus/async-validator，两个新增用例均完成失败到通过验证。
- 已执行 `& './docs/database/2026-10-12-system-setting-media-kinds.ps1'`：首次迁移 1 条默认头像记录，generation 18→19，保留 ID、对象键及其他业务字段；最终版再次执行为 0 条变化、19→19，runner 内二次执行不变。迁移使用原子 DO block、锁等待上限、写前备份和前后审计；测试覆盖强制失败时 DDL/数据/版本/outbox 回滚，以及迁移后新建的 5 类图片附件不会被复跑误改。真实结构已导出 current.sql。审计备份位于 `%LOCALAPPDATA%\Admin\backups\system-setting-media-kinds\20261012-191943-606` 和 `20261012-192635-234`。
- 实际前端验证：`pnpm vitest run tests/views/message/mail/index.test.ts tests/components/UpMedia/src/index.test.ts tests/components/UpMedia/fileLibrary.test.ts tests/views/user/profile/index.test.ts tests/views/system/setting/index.test.ts tests/views/storage/file/index.test.ts vite.config.test.ts --pool=threads --maxWorkers=1`，7 文件 160 项通过；`pnpm build`（含 vue-tsc）、受影响 Vue/TS/测试 ESLint、Prettier、`pnpm check:architecture`（0 findings）及 `git diff --check` 通过。
- 实际后端验证：受影响 Go 文件已 gofmt；`go test ./internal/module/system/setting ./internal/module/message/mail/recipientRule ./internal/shared/i18n ./internal/shared/setting -count=1`、`go test ./internal/database -run '^TestSettingMediaKinds' -count=1`、`go vet ./internal/module/system/setting ./internal/module/message/mail/recipientRule ./internal/shared/i18n ./internal/shared/setting ./internal/database`、`go build ./...` 均通过。
- 未运行全仓测试、真实浏览器/COS 验收；构建保留既有富文本大 chunk 提示。没有启动/停止后端或提交 Git；维护者自行重启新版后端并刷新前端加载新契约。

## UpMedia 四种展示模式整改（2026-10-12，代码与定向验证完成）

- 维护者确认图片 image / 视频 video 使用正方形，文件 file 使用按钮和附件列表，按钮 button 只提供上传入口并交给页面展示结果；文件库选择独立开关，公共组件不出现头像业务文案。
- 范围：UpMedia、FileBrowser、个人中心、设置展示 DTO 与中英文文案；共用签名直传/完成登记/文件选择流程，沿用既有权限，不增加数据库结构。百万用户/多实例基线下不增加上传热路径查询或缓存，设置展示分类使用代码和既有对象键，不逐行查询 COS。
- 按维护者最新截图：正方形悬停显示预览/删除图标，图片复用 Element Plus 查看器和下载，视频使用播放器；附件行保留图标/文件名/下载/清除，不增加逐项替换入口。删除/清除二次确认只移除当前字段选择，不删除文件库原件。验收包括单/多文件、取消、部分失败、文件库开关和用途、签名失效和下载失败、头像清空与幂等保存；不启动/停止后端，不使用子代理或 computer use。
- 已实现四模式与原始文件名展示，文件库页通过 button 模式和 uploaded 事件刷新，个人中心使用 image；设置展示模式由后端 presentation DTO 返回，不在前端重算。图片查看器下载重新取得签名并通过 Blob 保存，保留预览到签名刷新结束；失败可重试，切换绑定或卸载取消旧下载。视频按需挂载播放器、关闭卸载；清除确认晚到时核对原绑定，避免清除新值。稳定规则已同步 design.md。
- 实际验证：`pnpm vitest run tests/components/UpMedia/src/index.test.ts tests/components/UpMedia/fileLibrary.test.ts tests/components/FileSelectDialog.test.ts tests/views/user/profile/index.test.ts tests/views/system/setting/index.test.ts tests/views/storage/file/index.test.ts --pool=forks --maxWorkers=1`：6 文件 95 项通过；随后补下载网络/HTTP 失败重试、绑定变化取消回归，单独重跑 `pnpm vitest run tests/components/UpMedia/src/index.test.ts --pool=forks --maxWorkers=1`：29 项通过。`pnpm build`（含 vue-tsc）、追加测试后的 `pnpm typecheck`、受影响 Vue/TS/测试 ESLint 和 Prettier、`pnpm check:architecture`（0 findings）、`git diff --check` 通过；后端 `go test ./internal/module/system/setting -count=1` 与同包 `go vet` 通过，Go 文件已 gofmt。
- 未运行全仓测试、真实浏览器视觉验收或真实 COS 下载；Blob 下载需要 COS 的 GET CORS 配置，网络 mock 不代表真实云端验收。构建保留既有富文本大 chunk 提示。本轮无数据库/RBAC 变化，不需要迁移；维护者自行重启后端加载新的设置展示 DTO 并刷新前端。没有启动/停止 API/Worker 或提交 Git。

## 我的文件上传完成请求体修复（2026-10-12，代码与定向验证完成）

- 截图中凭证和 COS PUT 成功、complete 返回 400 的根因是请求体契约错配：后端 RequireEmptyBody 要求完全空体，前端却发送 `{}`，Axios 序列化后在 Handler 校验阶段被拒绝。completeFileUpload 现传 undefined，不发送请求体，保留取消信号；本轮只改前端生产代码，无后端行为、数据库、权限或缓存协议变化。
- 纠正 API mock 的旧断言，新增真实 request.ts/Axios adapter 回归，确认带/不带信号均无序列化请求体且完成 DTO 保真；补 Gin Handler 契约测试，覆盖空体接受、非空/畸形体拒绝且不调用 Service，以及身份、Request Context 和上传权限绑定。
- 实际运行 `pnpm vitest run tests/api/storage/file.test.ts tests/api/storage/fileRequest.test.ts tests/components/UpMedia/src/index.test.ts tests/views/storage/file/index.test.ts tests/utils/request.test.ts --pool=threads --maxWorkers=1`：5 文件 81 项通过；受影响前端文件 Prettier、ESLint、`pnpm build`（含 vue-tsc）通过。Go 新测试 gofmt、`go test ./internal/module/storage/file -run '^TestCompleteHandler' -count=1`、`go test ./internal/module/storage/file ./internal/shared/validate -count=1` 和相同两包 `go vet` 通过；前端回归先失败后修复通过。
- 未运行全仓测试或真实浏览器/COS 上传验收，Axios adapter 测试不代表真实网络上传；构建保留既有富文本大 chunk 提示。没有启动/停止 API/Worker、执行迁移或提交 Git；刷新新版前端后重新选择文件上传即可，本轮不需要重启后端。

## 我的文件页面与菜单收口（2026-10-12，代码、菜单修正与定向验证完成）

- 维护者截图中的三个“默认100MB”来自同一条上传规则的 `avatar/file/setting` 编码，非三个 COS 配置。原 options 查询展开全部编码，页面默认取到 avatar；现后端按普通文件 `file`、头像 `avatar` 精确匹配，不回退到其他用途，未知用途返回明确参数错误。查询移回 uploadRule Repository，当前平台、规则和 COS 配置均需有效启用；每次 options 最多一条 SELECT，不改变凭证/对象访问的缓存热路径。
- `FileBrowser` 保留复用 `AppSearch`、`AppTable`、`UpMedia`、`AppDialog`，删除上传用途下拉及其状态/样式；补文件名和类型搜索中英文 placeholder，消费后端匹配规则并展示文件大小限制，B/KB/MB 不将小限制四舍五入成 0 MB。删除二次确认、真实分页、本人隔离及独立权限保持不变；没有新增表、字段或权限。
- 已直接修正真实 PostgreSQL Admin 菜单 id=4283 的 icon 为 `lucide:file-stack`。一次性维护程序在忽略的 server/tmp 内调用现有 Menu Service，沿 Repository 事务与 token lease 发布；Admin menu_version 18→19，Redis 核对 ready/19。重复执行保持 19；Canvas menu_version 保持 1，认证 policy_version、菜单 ID/路径/授权及非 icon 字段保持不变。仅改数据、无 DDL，不需要重导 schema-only 快照。
- 菜单执行前后 JSON、授权摘要和维护源码已落审计：`%LOCALAPPDATA%\Admin\maintenance\storage-file-menu-icon\20261012-083823.708965300` 与 `20261012-083835.576241800`。只读 check 首先确认 icon 未配置，修正与重复执行后 check 成功；临时源码及目录经审计哈希核对后已删除，没有在 server/cmd 增加一次性入口。
- 先失败后通过：前端缺 placeholder、遗留用途下拉、小限制显示 0 MB；后端同规则多 code 展开、file 缺失却返回其他用途、未知 purpose 未拒绝。实际运行 `go test ./internal/module/storage/uploadRule ./internal/module/storage/file ./internal/module/permission/menu ./internal/module/permission/state -count=1`、相同包 `go vet`、`go test ./cmd/api ./cmd/bootstrap-admin ./cmd/worker -count=1`、`go build ./...` 均通过；三个 cmd 入口清理后的编译与入口测试已收尾。
- 前端 `pnpm vitest run tests/views/storage/file/index.test.ts tests/components/FileSelectDialog.test.ts tests/components/UpMedia/src/index.test.ts tests/api/storage/file.test.ts tests/icons/menuIcons.test.ts --pool=threads --maxWorkers=1` 5 文件 37 项通过；受影响文件 ESLint、Prettier、`pnpm check:architecture`（0 findings）、`pnpm build`（含 vue-tsc）和 `git diff --check` 通过。构建保留既有富文本大 chunk 提示；未运行全仓测试、浏览器验收或真实 COS 上传。没有启动/停止后端，也没有提交 Git；维护者需自行重启新版后端再刷新页面。

## 云服务“我的文件”（2026-10-12，代码与真实数据库迁移已落地）

- 维护者确认：云服务新增“我的文件”；仅当前用户/认证平台本人文件，不做管理员全部文件。本人上传、分页搜索/筛选、预览、下载、删除，以及复用文件选择弹窗选取头像；新增一张 storage_file 表，复用现有 COS 配置/版本、上传规则与 scheduler 任务表。
- 头像采用 COS 服务端复制的独立对象，用户头像字段保存副本 Key；删除文件库原图不影响头像，更换头像不影响原图。副本归属/幂等/清理记录复用同一新表，不增加引用表；本期不改品牌图/模板共享资源，历史对象不猜用户归属。
- 后端拥有归属、上传完成 HEAD 校验、暂存对象提升、图片条件、双语 options、删除及复制故障收敛；前端只负责展示、即时校验和交互，thin API、SCSS、纯文字有 type 操作与删除二次确认。
- 权限：新页面 storage:file:view，独立 storage:file:list/detail/delete；申请/完成上传复用 storage:object:upload，头像更新保持 user:profile:update 并核对本人文件读取权限。菜单与授权经离线幂等 forward migration，平台菜单版本和 Redis 权限协议保持现有机制。
- 容量基线：百万用户/多实例，本人页默认20最多100，COUNT+SELECT最多2条SQL；owner/time部分索引，预览按需签名、不逐行HEAD、不扫描整个Bucket；删除/过期上传和未采用头像副本有持久意图，分批清理和跨实例验证，不引入无界回源缓存。
- 后续已实现 `storage/file` 后端模块、文件浏览组件、文件选择弹窗和头像保存复制链路，并由主 Agent 直接执行真实迁移：`pwsh -NoProfile -File docs/database/2026-10-12-storage-file.ps1 -OldServicesStopped` 连续两次成功，`-CheckOnly` 成功。核对 storage_file 存在、4 个菜单节点（1 page/3 action）、4 项默认授权和 1 个 storage.file.scan 计划；Admin menu_version 17→18，复跑不递增，Canvas 保持 1，原 33 项角色授权及摘要保持不变。schema 快照已刷新；此处不把此前缺少的完整实现验证补写为通过。
- 迁移审计位于 `%LOCALAPPDATA%\Admin\maintenance\storage-file\20261012-153247-585` 和 `20261012-153414-510`。本期仍由维护者自行启动后端；不自动启动 API/Worker。

## 定时任务字段与索引收口（2026-10-12，代码、真实迁移与新版服务启动完成）

- 按维护者确认执行：保留 schedule/job/run 与状态数值，状态文案改为待执行/排队中/执行中/执行成功/执行失败/已取消；退出 job.queue/timeout_seconds/worker_id 与 run.result_summary，Worker 归属保留在 run。
- 契约：run DTO 删除 resultSummary；任务队列/超时仍由代码 TaskDefinition 决定，max_attempts 保留；后端按请求语言生成已知错误分类的安全诊断文案，不透出原始异常。既有 API URL、菜单、权限、Redis/队列 envelope 不变。
- 代码已完成：Model/Repository/Worker/DTO 与前端 Run 类型移除四个字段，CompleteRun 退出空摘要参数；自动重试成功清除 job 旧错误，失败 run 保留历史。中文/英文状态与安全错误提示在所属后端生成；无菜单或权限变更。
- 容量与索引：保留现有索引中的事实唯一约束，删除 job(id,attempt_count)、run(job_id,attempt_no DESC) 和 run(finished_at,id) 三个冗余/无用途索引；补 status=2 的 publish_lease_until/id 部分索引，代表性历史数据 EXPLAIN 验证空队列扫描不会随历史记录增长。管理分页仍 count+page 两条 SQL，不新增缓存或逐行查询。
- 交付 docs/database/2026-10-12-scheduler-cleanup.sql/.ps1。维护者停止 API/Worker 后执行 -OldServicesStopped：拒绝服务连接、活动查询和事务；仅允许无事务、无锁的 Navicat 空闲连接，备份并完整读取校验归档/记录 SHA256，单事务迁移与 5 秒锁等待上限，非空 result_summary 或不兼容新索引拒绝并回滚；核对三表记录数/结构，成功后导出真实 current.sql。数据库已提交但后续校验/快照失败会非零退出并在 audit.json 明示，不伪报成功。
- 验证：go test ./internal/module/system/scheduler ./internal/module/message/notificationTask ./internal/shared/i18n -count=1、相同包 go vet、go build ./cmd/api ./cmd/worker 通过；前端 3 文件 30 项 Vitest、pnpm build、改动源文件 ESLint/Prettier、pnpm check:architecture（0 findings）通过。SQL 在隔离 schema 验证历史/Worker 归属保留、重复执行、两类拒绝回滚、自动/手动重试与双语错误；3 万条历史数据的排队恢复 EXPLAIN 使用新索引，无 Seq Scan。
- 完整 runner 在独立临时数据库首次与重复执行均 exit 0，审计 postgresCommitted/verified/snapshotUpdated=true，记录数保持 1→1；临时数据库已删除，仅在临时目录导出快照。审计目录为 %LOCALAPPDATA%/Admin/maintenance/scheduler-cleanup/20261012-121749-741 与 20261012-121752-087。维护者授权代执行后，2026-10-12 13:27 在真实 admin/public 完成首次迁移及重复执行，两次均 exit 0，审计 postgresCommitted/verified/snapshotUpdated=true；3 个计划、19746 个 job、19750 个 run 前后不变，旧字段/索引均为 0，新索引存在。真实备份与审计目录为 %LOCALAPPDATA%/Admin/maintenance/scheduler-cleanup/20261012-132707-815、20261012-132718-695；current.sql 已从真实库刷新，并保留数据库原有配置状态表约束名称。三个 Navicat 连接经核对为空闲、无事务、无锁，未断开客户端。新版 API PID 38600、Worker PID 31596 从 %LOCALAPPDATA%/Admin/runtime/scheduler-cleanup-20261012-132554 启动，/health 与 /ready 均 HTTP 200，PostgreSQL/Redis up；Worker 启动后 job/run 已分别增长至 19750/19754，未见 fatal/panic 或缺表/缺列错误。
- 维护者要求后端自行启动；已停止本次由 Agent 启动的 API PID 38600 和 Worker PID 31596，并核对进程退出。以后不代启动后端；上述健康与任务增长记录仅代表停止前验证结果。
- 真实迁移后重新运行 go test ./internal/module/system/scheduler ./internal/module/message/notificationTask ./internal/shared/i18n -count=1，三个包均通过；新版 api.exe/worker.exe 构建通过，git diff --check 通过。
- 未运行浏览器自动化和本轮后端全量测试；上一轮扩展全量测试在历史 database migration 用例失败（依赖已退出的 system_config_cache_generation），不宣称全量通过，本轮不恢复旧表。

## 定时任务结构只读审查（2026-10-12，讨论结果，未实施迁移）

- 核对生产调用链与真实 PostgreSQL pg_catalog/pg_stat/EXPLAIN。schedule/job/run 分别承担计划、持久任务和执行尝试；notificationTask 业务批次也使用 job，六个任务状态均有实际分支，三表暂不合并。
- 字段候选：job.queue/timeout_seconds 当前只写入，Publisher/Executor 实际使用 TaskDefinition；run.result_summary 的唯一生产成功调用传空串，真实库全部为空。可按代码配置事实来源退出这些字段；max_attempts 参与业务重试和恢复不可一起删除。错误文本当前多数只传分类码，需改善公开诊断内容，不能直接透出原始异常。
- 确认冗余索引：job(id,attempt_count) 唯一性被 id 主键覆盖；run(job_id,attempt_no DESC) 与唯一索引(job_id,attempt_no) 可合并使用 B-tree 反向扫描；run(finished_at,id) 终态部分索引未被当前按 job_id 清理流程使用。删除前分别核对依赖并使用 forward migration。
- 优先索引问题：RecoverExpiredQueued 每轮查询 status=2 的 publish_lease_until，现有 publish_lease 索引只覆盖 status=1。真实库约 1.96 万条 job 时相同过滤/排序的 EXPLAIN 已走 Seq Scan；需为 status=2 的过期投递恢复补匹配的部分索引，并复查 status=1 旧索引实际必要性。不能凭 idx_scan=0 删除唯一约束或断言所有索引无用。
- 计划+状态分页现有索引只有(schedule_id,created_at DESC,id DESC)，状态靠过滤；大历史量且频繁筛选时评估(schedule_id,status,created_at DESC,id DESC)，由代表性数据的执行计划验证，不因用户总数大就给仅 3 个计划过度加索引。

## 定时任务按计划分页查看与确认框语言（代码与定向验证完成）（2026-10-12）

- 移除全量执行记录页签，主页面仅计划表；计划下拉选择当前已加载计划，可按游标加载更多计划；每行“执行记录”打开该计划的 AppDialog/AppTable，保留真实运行状态筛选、详情和独立 retry/detail 权限。
- 管理列表按 created_at DESC、id DESC 返回，计划使用 beforeId 与时间/ID 元组游标；执行记录使用 page/pageSize 和 list/total/page/pageSize 契约，默认每页 20、上限 100，AppTable 显示总数和页码，状态/计划/每页条数变更回第一页；Job HTTP 查询接入 scheduleId，不在前端裁剪全局记录。百万级用户容量下此接口是低频管理员查询，记录每页 COUNT + SELECT 共 2 条 PostgreSQL 查询，按计划索引缩小扫描范围；深页 OFFSET 和精确 COUNT 随该计划历史量增长，沿用既有终态历史清理限制数据量，不进入用户请求热路径。计划每批 100、1 条 SELECT，既有 schedule/job 列表索引可复用，无请求热路径配置读库、无进程缓存或 Redis 协议变化。
- 程序式 ElMessageBox 独立于组件 Provider，应用入口显式安装服务上下文与响应式 Element Plus locale；避免在页面逐个写中文按钮或增加业务请求工具。
- 定向验证：scheduler/shared pagination 的 Go 测试通过；前端 6 文件 44 项测试通过，覆盖分页、筛选回第一页、旧请求隔离和中文/英文确认按钮；pnpm build、pnpm lint、pnpm check:architecture（0 findings）、改动源码 Prettier 与 git diff --check 通过；go vet ./... 与 go build ./... 通过。未运行真实浏览器自动化验收。扩展 go test -p 1 ./... -count=1 最终未通过，发现 internal/database 的多项历史迁移测试依赖已改名的 system_config_cache_generation 表而失败；不能宣称后端全量测试通过，此项另行收口，本轮不恢复旧业务表。

## 需求来源：COS“我的文件”（2026-10-12，实施前讨论记录，当前交付见上文）

- 当前只有 COS 配置、物理版本、上传规则、PUT 直传凭证和 object-url；没有文件管理表/上传完成登记，object key 包含 platform/rule/config/physical version，不包含 userId。当前 STATUS/design/architecture 没有已确认的文件库待办；不能把已有对象按 key 推断成某个用户的历史文件。
- 下期建立具名 storage/file 模块与 storage_file 表，保存用户/认证平台归属、object key、物理配置版本、原始文件名、MIME、大小、ETag、上传状态及时间。签发凭证登记待完成上传；客户端 PUT 后由后端核对 COS HEAD，再幂等登记为可见文件；失败/未完成不混入“我的文件”。浏览器提交的 userId 或“上传成功”不能作为事实来源。
- 第一阶段提供本人文件分页、筛选、图片预览和从文件库选择头像；后端按登录身份强制 user_id/platform_id 过滤，索引使用 (platform_id,user_id,created_at DESC,id DESC)，单页列表查询最多 2 条 PostgreSQL SELECT，签名 URL 按当前页/选取按需取得，禁止列举整个 Bucket 或逐用户无限加载。沿用现有 object key 与物理版本定位，不另造第二套上传配置。
- 文件页、list/detail 和已有 upload 动作分别映射 RBAC；默认用户是否获得“我的文件”page/list/detail，通过下期幂等菜单/授权迁移明确。头像选取后端重新检查本人归属和图片条件；品牌/系统媒体的共享用途保持各自权限，不能一刀切修改现有公开资源 URL 规则。
- 旧文件不自动猜归属；下期决定可靠回填来源和无主历史对象处理。用户删除文件的范围、已引用头像/模板的保护、COS 删除与 PostgreSQL 部分失败/重试需在开放删除前单独定案；本期不提供删除接口，不改数据库或用户头像。
- 下期验收覆盖跨用户/跨平台访问、伪造完成、HEAD 不存在/类型大小不符、重复完成、过期凭证、配置版本变化、签名 URL 过期、并发分页与头像选择；多实例上传归属和完成事实必须落 PostgreSQL，不能依赖单进程缓存。

## 删除与业务状态变更二次确认（2026-10-12，代码与定向验证完成）

- 全项目从 API 删除/状态接口逐一核对 View、通知 Store 调用方和表单入口；系统设置、定时任务、Mail/SMS 模板与收件规则、菜单启用、通知删除/已读补齐确认。Mail/SMS 已有配置和收件规则通过保存修改状态同样确认；上传规则原子 PUT 停用补确认，新增默认状态不作为状态切换。
- 定时任务立即执行/重试、通知中心单条/全部已读及顶栏铃铛自动已读同样确认；取消/关闭不发送请求，不改列表状态，API 错误仍由 request.ts 通知。系统设置和定时任务已有删除确认增加取消处理。
- 操作列按钮保留纯文字并显式指定语义 type：查看/编辑 primary，停用 warning，启用 success，删除 danger；配置缓存状态详情补 primary。双语确认提示显示对象和目标状态，稳定约定已写入 design.md。
- 验证：系统设置新增 3 项回归先失败后通过；`pnpm vitest run <相关页面> --pool=forks --maxWorkers=1` 分批覆盖 10 文件，最终 178 项通过（首轮其他 6 文件 76 项；Mail/SMS/通知最新 3 文件 77 项；菜单最终 25 项）。覆盖启用/停用、取消/关闭、确认等待期间不发送、开关取消不改原值、表单状态取消与通知删除/已读取消。菜单旧测试原先未确认启用，已补取消及确认断言；统一颜色时遗漏枚举导入，构建发现后已修复并重跑。`pnpm lint`、`pnpm build`（含 vue-tsc）、`pnpm check:architecture`（0 findings）、改动文件 Prettier 检查和 `git diff --check` 通过；操作列 type/icon 扫描 0 findings。未运行全仓测试、后端测试或浏览器人工验收；构建保留既有富文本大 chunk 提示。
- 本轮仅前端交互，无后端/API/数据库迁移，无业务热路径和缓存协议变化；不提交 Git。

## 配置状态身份展示与管理列表规范（2026-10-12，代码与定向验证完成）

- 配置缓存状态查询关联 COS 名称（含软删除行），DTO 明确名称/已删除/不存在；列表显示名称和 ID，已删除或不存在记录有明确标记，关键字支持名称查询。
- 保留缓存状态/outbox 历史事实及待同步数量，不改 Redis 一致性协议，不执行数据清理或新增迁移；管理员低频分页读使用两次 PostgreSQL 查询，不新增请求热路径读库或逐行名称查询，COS 按主键关联。
- 全项目表格操作列移除按钮图标；启用类状态列统一“状态”，值统一“启用/停用”，状态翻译统一归入业务域 `.status.label/enabled/disabled`，其他任务/同步状态保持业务含义。
- 验证：`go test -p 1 ./internal/module/system/configCacheStatus ./internal/module/permission/menu ./internal/module/system/scheduler ./internal/shared/i18n -count=1`、COS 非数字/越界 ID 定向回归和 `go vet ./internal/module/system/configCacheStatus ./internal/shared/i18n` 通过；测试数据库使用隔离 schema。
- 前端 `vitest --pool=forks --maxWorkers=1`：COS 页面 9 项、i18n 10 项、受影响页面 14 文件/201 项通过；`pnpm build`（包含 vue-tsc）、`pnpm lint`、`pnpm check:architecture`（0 findings）和 `git diff --check` 通过。20 个操作插槽扫描无图标；菜单树操作列额外检查无图标。初次页面回归发现 `.status` 文本与 `.status.enabled/disabled` 层级冲突，改为 `.status.label` 后双语回归通过。
- 未运行全仓测试或浏览器人工验收；threads 批量测试长时间无输出，已终止本轮测试进程并使用 forks 完成定向回归。构建仅保留既有富文本大 chunk 提示。未启动/停止业务服务、未执行迁移/Redis 清理、未提交 Git；维护者重启新版 API 后刷新前端验收。

## 配置缓存状态跨层改名（2026-10-11，代码与迁移交付）

- 管理业务名称统一为“配置缓存状态”/`configCacheStatus`：Go 模块、共享包、管理 API、RBAC 权限、菜单 path/componentPath/code/i18nKey、前端 API/页面、测试和双语文案已同步；主表目标为 `system_config_cache_state`。
- `generation` 列、outbox 表、Redis `config-cache:state:v1` key、状态值、快照和租约协议保持原语义，仅作为内部缓存一致性协议字段。
- 新增 `docs/database/2026-10-11-system-config-cache-status.sql/.ps1` 幂等 forward migration；保留菜单 ID 与角色授权，按实际变化每个平台最多递增一次 `menu_version`，runner 二次执行校验 generation/outbox/menu facts 不变。2026-10-12 维护者确认旧服务停止后执行真实迁移，菜单契约/表约束/FK 和二次幂等验证通过；未清理 Redis、未提交 Git。

## 当前工作：全项目后端业务归责与前端减法（2026-10-10，代码整改、全量验证与真实离线迁移完成）

- 维护者已明确要求一口气实施全项目整改，不停留在计划。按所属 Go 模块集中拥有枚举、候选项、本地化标签、默认值、约束、业务保护和状态动作；前端保留明确 DTO、即时表单 rules、交互/展示和浏览器运输。数字/字符串 value 原类型保真；新增业务值不由前端 parser/名单拦截。
- 已实际完成五方法泛型 request 与现有全部业务 API 瘦身，删除 api/protocol.ts 和逐接口 expect/parse；保留 envelope、认证凭据、刷新 single-flight/一次重试、终态通知一次及登录内联错误。Base64 下载和实时帧校验只保留在实际浏览器/运输用途文件。
- 字典生产模块、装配、路由、页面、API、Store、专属 i18n 与专属缓存消费退出。性别、COS region/extension/MIME、Mail/SMS region 回归所属后端代码 `/options`；语言乱序保护、加载失败清空、上传多选/自定义值保留。候选项构建的额外 PostgreSQL/Redis 查询预算均为 0。
- 其他业务 options/actions/constraints 覆盖登录/身份变更日志、会话、通知/任务、Mail/SMS、Scheduler、设置、认证平台、菜单、用户与角色；操作日志 55 种动作标签由 Go 按请求语言生成，未知动作保留原值，列表仍只 count + page 两次 PostgreSQL 查询；操作结论与 Access 交叉检查，后端写入仍校验最新事实/锁/事务。上传规则字段与可选状态通过一次 PUT 原子保存，不再前端 PUT 后 PATCH 制造部分成功。
- 14 个自有 CSS 文件迁移 SCSS，Vue 自有 style 全部 lang=scss，保留 scoped、主题变量与响应式选择器；退出 500/400 行强拆、SCSS 必须特定 Sass 语法及 API parser 门禁。组件按职责/真实复用组织，第三方 CSS 原样保留。AGENTS/design/architecture/status-enum-catalog 已同步责任边界。
- 配置缓存代际产品定位、页面/API/权限/代际协议不改；仅退出字典 scope，并将既有展示映射归后端。权限/Session/限流/队列等独立缓存协议不删除，也不把枚举套入配置中心。
- 交付 `docs/database/2026-10-10-remove-system-dictionary.sql/.ps1` 与 `server/cmd/remove-system-dictionary`：离线、幂等 forward removal、durable manifest、受影响平台 menu_version/token lease 衔接、固定 patterns SCAN/UNLINK 与部分失败非零。先完成真实库只读依赖盘点与隔离 schema 集成测试，再按维护者“迁移”指令执行真实业务库退出，结果见下方执行记录。
- 独立审查发现的授权弹窗串用户/旧保存污染、上传规则 PATCH 未锁 no-op/公共配置空域名、设置媒体必填被绕过与确认期间切换目标均按失败测试→修复→回归收口。Account 新增 13 项生命周期回归、Setting Vue/API 45 项、Menu/AuthPlatform 86 项及 OperationLog 10 项定向通过，独立最终审查无未修 Critical/Important。
- 冻结代码后的整体验证已完成：server 的 `go fmt ./...`、`go vet ./...`、`go test -p 1 ./... -count=1`、`go build ./...` 全部 exit 0；web 的 `pnpm format:check`、`pnpm typecheck`、`pnpm check:architecture`（0 findings）、`pnpm lint`、`pnpm vitest run --pool=threads --maxWorkers=1`（117 文件、1066/1066 项）和 `pnpm build` 全部 exit 0。Build 仅保留既有 rich-editor-core 大 chunk 告警，未放宽限制。退出 PowerShell 脚本通过 Parser 语法检查，`git diff --check` 通过。
- Go 首轮唯一失败是 CacheGeneration 旧封闭 DTO 断言遗漏新增展示字段；补齐字段名单后定向与最终全包通过，保持严格字段数检查。代码整改轮测试使用隔离 PostgreSQL schema 与现有 Redis 测试 DB（运行时 Redis DB0，uploadRule 测试 DB3），未在该测试轮执行业务 Redis 清理或迁移 runner。
- 真实迁移执行于 **2026-10-10 23:32–23:33 +08:00**：核对本项目 API/Worker 未运行、其他 PostgreSQL 客户端连接为 0，无需停止任何进程；备份校验后执行 `2026-10-10-remove-system-dictionary.ps1 -OldServicesStopped`，首次及使用同一 manifest/SQL 的重复执行均 exit 0，审计 `postgresCommitted=true`、`verified=true`。两张字典表删除，原 6 个字典/35 个条目、7 个菜单、字典 global generation/outbox 各 1 条退出，相关角色授权原为 0；平台 1 menu_version **15 → 16**、Redis menu state ready=16，平台 2 menu_version=1 与原缺失 Redis menu state 保持不变，两平台 policy_version 不变。
- PostgreSQL 迁移前备份位于 `%LOCALAPPDATA%\Admin\backups\remove-system-dictionary-20261010-233119\public-before.dump`（1,114,609 字节），SHA256 `3AD5F7D4A77F26FE3075719DAD542CDF2B443B863F4324764D574F2D0318D224`；`pg_restore --list` 及 `pg_restore --no-owner --no-acl --file=NUL` 完整归档读取均通过，执行后原备份哈希未变。同目录保留 before/after/repeat、Redis/menu state 对照、备份及执行元数据；durable manifest 位于 `%LOCALAPPDATA%\Admin\maintenance\remove-system-dictionary\manifest.json`，重跑时必须保留。
- 真实验证：六个固定 Redis pattern 均 remaining=0；无关菜单、角色菜单、配置 generation/outbox 全行哈希前后相同，无孤立 role_menu，重复执行后 PostgreSQL 事实、菜单版本、Redis 定向计数与菜单 state 完全不变。只清理字典缓存，未重置 Session/Access/发送额度或其他缓存。`docs/database/current.sql` 已通过 `pg_dump --schema-only --schema=public --no-owner --no-acl` 从真实库刷新（118,794 字节），SHA256 `DC8804B9C14F9AD91B27C0D70B8EFDE5009754789EE60121ED183422F12381DD`，无字典表或业务数据。
- 本次迁移另运行 `go test ./cmd/remove-system-dictionary ./internal/module/permission/state -count=1`，两包通过；迁移及快照/状态更新的 `git diff --check` 通过。未重复运行上一代码整改轮全量 Go/前端套件，未做浏览器验收或第三方发送，未 commit/fetch/pull/push、停服或启动服务；API/Worker 保持停止，由维护者启动新版。

## 后端 options 文案统一进入 i18n catalog（2026-10-11，全项目迁移完成）

- `server/internal/shared/option` 新增 `NewLocalized`：业务模块以稳定 `MessageKey` 投影 `{value,label}`，翻译 key 缺失时返回错误，不回退为 key 或另一语言。
- 邮件、短信、用户联系/会话/登录日志、系统设置、通知/通知任务、Scheduler、认证平台、菜单和配置缓存代际的 options 及 enum presentation 文案均改为请求语言 catalog 投影；数字/字符串枚举值、tone、默认值、接口路径和 JSON 形状不变。catalog 缺 key 不静默回退，HTTP handler 统一返回依赖错误。
- 操作日志、账号和角色动作标签属于行为描述而非 enum option，已移除 `option.New` 并继续按请求语言生成；未知操作保留原值，既有权限动作和业务保护不变。全仓 `server/internal/module` 已无 `option.New` 业务调用。
- 验证：`gofmt`、`go vet ./...`、`go build ./...`、`go test -p 1 ./... -count=1`（全包通过）、共享 i18n/option/architecture 定向测试和 `git diff --check` 通过；未提交 Git。

### 本轮实施前审查记录（保留）

- 维护者已将目标从删除字典管理扩大为全项目责任边界整改：后端集中拥有业务枚举、合法值、业务限制、状态转换和本地化业务文案，前端消费后端结果并承担交互与展示，不再手工维护第二份业务事实。已进一步明确前端整改：API 只保留类型和接口调用，request 提供 `get/post/put/patch/delete` 泛型门面，取消逐接口 DTO 运行时解析及重复业务规则；表单做好 rules，页面做好组件交互；撤销页面 500 行/组件 400 行强制拆分，自有样式统一 SCSS，按交互职责和复用拆组件，不按行数凑文件。具体逐模块契约与施工分批仍需评审，不凭总体方向直接批量改接口或业务代码。
- 已只读审查 PHP 参考项目 `D:\github-project\admin_back` 的性别与 AI 枚举链：代码枚举复用于校验、页面 init、列表/导出和 Provider 协议映射；`DictService::enumToDict` 保留数字/字符串 value，静态枚举转换不查数据库、不进 Redis。借鉴后端单一维护入口与按业务需要提供选项，不照搬其通用服务、基类或旧数值协议。
- 已完成当前生产代码目录盘点：前端 197 个 TS/Vue 文件（含 97 个 views 文件、27 个 API 文件），后端 8 个顶层域、344 个模块生产 Go 文件；按 enums/API/views、Store/router、业务协议/校验和相关测试定位问题，并核查关键前后端调用链。该数字是盘点范围，不代表所有文件、所有运行分支均已逐行审完；全量逐模块责任清单仍在进行。
- 已确认的重复维护点：`api/message/{sms,notificationTask,notification}.ts`、`enums/{scheduler,smsRecipientRule,mailRecipientRule}.ts` 维护业务值/metadata；用户登录日志、会话和身份变更页各自映射文案；Scheduler presentation 按 `status - 1` 拼状态文案并维护 cron 预设；缓存页维护 namespace/scope/status 显示映射。需逐项区分业务语义与纯 UI 呈现后决定后端契约。
- 已确认的重复业务限制：系统设置 API/弹窗维护保留天数范围、内置 key 和媒体类型规则，后端 `system/setting/service.go` 已有对应校验；认证平台表单写死登录方式、TTL 范围和最大会话数，后端 `permission/authPlatform/protocol.go` 已有对应常量/校验；菜单 API 另维护与后端 `permission/menu/tree.go` 对应的语法规则。Mail/SMS 限流前端另识别固定 policy key/mode/dimension，需要审查是否可直接消费后端事实。
- 已确认的操作归责点：通知任务表格自行枚举草稿可编辑/删除/提交和可取消状态；会话页按状态与当前会话推导撤销按钮；上传规则保存先根据当前列表推断替换提示，再分别保存字段和状态。后续需核对后端动作结论、权限快照、事务原子性、并发变化和部分失败，不把按钮判断当业务权限事实。
- 已有后端驱动链应保留：登录配置接口返回本地化 `loginTypes`，验证码倒计时消费服务端 `resendAfterSeconds`，通知任务用户/角色选项由后端有界查询，邮件 Excel 导入由后端解析/校验。请求错误、加载/提交状态、浏览器路由组件解析、上传运输、实时连接与跨标签协调属于前端/传输责任，不因业务减法一并删除；具体交互简化仍待维护者确认。
- 相关测试也固化了旧归属：`web/tests/api/message/{sms,notificationTask}.test.ts` 明确断言前端拥有固定业务 metadata。确认新契约后，应同步迁移为后端定义/本地化/非法请求/动作规则测试与前端消费测试，不能只移动数组或删除失败断言；`design.md`、`architecture.md`、`status-enum-catalog.md` 的旧归属约定也待随实际实现更新。
- 请求/API 专项核查：当前 `web/src/utils/request.ts` 为 292 行，导出的 `request(config)` 固定返回 `Promise<unknown>`；27 个 API 文件共 6,355 行，Mail 943 行、SMS 693 行（均含必要 DTO 类型，不能把全部行数视为冗余）。`api/protocol.ts` 的 expect/parse 辅助和各域重复 parser 将响应契约再维护一遍；部分 parser/helper 还被实时通道和菜单表单引用，退出 API 时必须核对所有消费方，不原样搬去另一个目录继续逐接口解析。
- 规范根因已确认：`AGENTS.md` 和 `design.md` 要求 API 从 unknown 严格解析，`web/scripts/check-frontend-architecture.mjs` 的 `api-unparsed-response` 拒绝直接响应泛型，`oversized-sfc` 强制页面 500 行/组件 400 行拆分，`unnecessary-scss` 拒绝没有特定 Sass 语法的 SCSS。这些旧要求与维护者本次明确指令冲突，本次指令优先；施工时须同步修改规范、门禁及相关测试，不以新 helper/schema/框架换名延续旧设计。本轮尚未修改这些规范或脚本。
- 已只读对照 `D:\github-project\admin_front_ts\src/utils/request.ts` 及代表性 API：其五个方法的调用门面符合目标；但当前参考目录内部也存在 operations/installed client/Zod/schema/参数规范化，不能声称整套旧实现只有简单调用，也不将这些运行时抽象搬入当前项目。新方案沿用简洁门面和业务 API，而非原样复制旧框架、路径或协议。
- 样式盘点：91 个 Vue 文件中有 67 个 style 块，52 个普通内联块、15 个外部引用块，无内联 SCSS；另有 14 个自有 CSS 文件、3 个全局 SCSS 文件。整改需将自有样式组织为 scoped SCSS 或明确共享的 SCSS，保留主题 CSS 变量、作用域和响应式行为，避免只改扩展名/添加 lang 属性；第三方库原始 CSS 引入不属于自有源码迁移。优先 Element Plus 与现有 AppDialog/AppTable/AppSearch 等真实组件，独立交互按职责组件化，不制造只用于降行数的碎片组件。
- 后端保真与请求边界：后端通过输出 DTO/序列化、业务校验、富文本净化和契约测试保证字段、数值、null/空数组、分页及错误语义，不依赖前端 parser 补救；TypeScript 类型和 strict 保留，泛型不是运行时验证。request 集中负责响应 envelope 解包、公共请求头、鉴权/单次重试/并发刷新、取消/网络错误与一次通知，不承接各模块业务逻辑。已有请求回归测试覆盖这些行为，瘦身不能顺带破坏登录刷新或复制错误通知。
- 原删除字典的本地文件 `docs/local-work/2026-10-10-remove-system-dictionary-spec.md`、`docs/local-work/2026-10-10-remove-system-dictionary-plan.md` 仍存在且由 Git 忽略；其中前端静态候选与本地枚举翻译方案已不符合最新方向，标记待修订，不得继续执行。本轮未读取或改写这些文件。删除字典管理目标保留，配置缓存代际的产品定位仍待后续讨论，尚未决定删除其他配置/权限/会话缓存协议。
- 后续跨模块设计需按百万用户、多实例基线记录热点查询预算、缓存策略和并发验证：代码静态枚举不套 PostgreSQL/Redis 配置缓存，动态关系选项有界分页，动作展示结论不得制造逐行查询；接口展示结论不能替代写入时的后端校验和并发保护。具体契约/实施分批需维护者评审。
- 本轮只更新本状态入口并做静态审查，未修改业务代码、运行测试套件/构建、执行数据库迁移、清理 Redis、停服、启动服务或提交 Git。下一步按已收到的前端整改要求补齐逐域“前端删除/后端归属/契约与测试/保留交互”清单，先评审后端保真与薄 API 设计，再确认完整实施计划；运行时改动前同步处理上述冲突规范/门禁。


## 字典管理与缓存状态页面减法（2026-10-10，代码完成，验证待维护者）

- 已确认不改“配置缓存代际”页面标题，也不改 `system/cacheGeneration` 页面目录、API 路径、Go 模块、数据库表、Redis namespace 或权限码。
- 已落地字典使用场景注册表与管理页提示：六个现有字典显示消费页面和修改影响；`user.gender` 标记为固定 `0/1/2`，扩展名、MIME 和云地域标记为可扩展但需满足外部服务/上传规则约束。
- 已增加 system/dictionary 后端策略文件，内置 `user.gender` 的新增、停用、删除保护，不改表结构；其余字典保持现有管理能力。
- 已完成配置缓存页面减法：保留技术 DTO 和内部标识，主表转为配置项/作用范围/同步状态/待同步事件/最近同步时间/最近错误，generation/outbox 等技术字段放入只读详情；Redis state 缺失显示为可恢复的“等待缓存重建”。页面标题仍为“配置缓存代际”。
- 本轮维护者要求不由 Agent 执行测试命令，验证由维护者在代码完成后执行；Agent 不执行数据库迁移、Redis 清理或 Git 提交。

## 短信手机号明文统一（2026-10-08，代码与真实迁移已完成）

- 维护者确认短信手机号与邮件地址采用同一可维护策略：`message_sms_recipient_rule` 的手机号规则明文保存和返回，`message_sms_log` 的收件手机号明文保存和返回；不再保留手机号密文、脱敏 hint 或数据库 HMAC 字段。
- 规则协议拟统一为数值：`scope: 0=phone, 1=prefix`，`action: 0=deny, 1=allow`；`pattern` 保存规范化 `+861...` 完整号码或 `+86` 前缀。唯一索引改为 `(scope, pattern, action)`，日志精确过滤直接使用 `to_phone`。
- 必须保留的加密：短信验证码 `code_ciphertext`、腾讯云 SMS SecretID/SecretKey；限流 Redis key 暂保留内部 HMAC 摘要，避免在共享缓存键中泄露手机号，不作为业务字段或管理员 DTO。
- 影响面已审计：SMS recipientRule Model/Service/Repository/Runtime snapshot/readiness/matcher；SMS send/log/service/repository/DTO/前端列表和详情；数据库 `message_sms_recipient_rule`、`message_sms_log` 约束/索引/current.sql；Redis `message.sms/global` generation/schema。当前真实库两张 SMS 目标表均为 0 行，旧加密数据迁移仍需显式失败保护，不能写假明文。
- 已实现：SMS recipientRule 的 `scope/action` 为数值 0/1，`pattern` 明文；SMS log 使用 `toPhone` 明文；运行时快照 schema 升为 2 并拒绝旧密文/旧字符串/缺失/null 字段；前端 DTO、规则编辑、日志列表统一数字枚举和明文号码。短信验证码、腾讯云密钥仍加密，限流 Redis key 保留内部 HMAC 摘要。
- 迁移：`docs/database/2026-10-08-message-sms-plaintext.sql/.ps1` 已备份并执行，当前真实库两张目标表均为 0 行，旧密文列安全删除，`message.sms/global` 1 → 2，Redis ready=2；SQL 重复执行、快照/约束/索引验证通过。备份和审计目录见 `%LOCALAPPDATA%\Admin\backups\message-sms-plaintext-20261008-211622-34bc81d8`，未清 Redis、未删除其他业务数据。API/Worker 仍保持停止，由维护者启动新版本。
- 验证：后端 `go test -p 1 ./... -count=1` 全部通过，`go vet ./...` 与 `go build ./...` 通过；前端 SMS API/页面 51 项、`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`（0 findings）和 `pnpm build` 通过。未做浏览器人工验收或真实短信发送。

## 收件规则导入弹窗微调（2026-10-08，已完成）

- `MailRuleImportDialog/index.vue` 文件行号列从 80px 改为 120px；删除底部重复的“预览校验”，仅保留表格上方入口，底部为“取消 / 确认整批导入”。
- 保留已读取文件对应的预览表格，使预览失败或确认失败后仍能用上方按钮重新校验。`web/tests/views/message/mail/index.test.ts` 覆盖单一入口、列宽和失败重试；46 项通过，`pnpm typecheck`、定向 ESLint、Prettier 和 `pnpm check:architecture`（0 findings）通过。
- 本轮仅前端局部调整，未改后端/协议/数据库/COS、未停服、未提交。未重跑全量前端测试/构建或浏览器人工验收，刷新前端即可检查。

## Excel/WPS 保存后导入兼容性（2026-10-08，已修复并回归通过）

- 维护者提供真实 WPS 保存后的文件，现有解析器错误返回 `invalid_xlsx`。根因是把普通 ZIP 空目录项、工作簿 `AlternateContent/absPath` 保存位置元数据、输入邮箱后自动生成的 `mailto:` 展示关系也一概拒绝；此前测试仅覆盖生成器与 Excelize 回写，没有覆盖此编辑器保存形态。
- 最小修复只在 `recipientRule/xlsxArchive.go`：接受限定的空目录（仍检查大小及 CRC）、限定命名空间/层级的保存路径元数据；工作表内自动邮箱链接只作为忽略的展示信息，不读取或访问目标，业务仍只取单元格文本。HTTP/file 链接、外部数据关系、未知兼容性内容、宏/公式及路径穿越继续拒绝；行列与解压上限不变。
- 新增 `xlsxSecurity_test.go` 的 WPS 保存结构回归及安全反例。已用维护者原文件直接调用实际解析器，读取 1 条合法规则（scope=0/action=1/isEnabled=1），文件 SHA256 `EDD19754F464E7FFF87674939A7108277AC0F619DA41397F2313AA4BF4B03CEE` 原样保持；没有改写用户文件、上传 COS、执行业务导入或停止服务。临时诊断测试已移除，永久测试仅使用合成数据、不包含用户邮箱。
- 定向验证：`go test -p 1 ./internal/module/message/mail/... ./internal/architecture -count=1` 303 项通过，`go vet ./internal/module/message/mail/...` 和 `go build ./cmd/api ./cmd/worker` 通过；`git diff --check` 通过。维护者可直接用原下载文件重试。
- 本轮无需配置发布或数据库迁移，前端协议不变。维护者重启 API 后可直接重试同一文件。

## 邮件收件规则 XLSX 模板与导入导出（2026-10-08，代码与真实发布已完成）

- 维护者已确认统一改 `.xlsx`，不再提供 CSV/.xls/.xlsm 导入兼容。业务表继续使用已迁移的 scope/action 0/1，权限、规则优先级、原子导入、重验证及不覆盖已有规则的行为不变。
- 模板契约：固定“填写说明”“导入数据”两页，只读“导入数据”；A1:F1 为“类型,邮箱/域名,动作,名称,备注,启用状态”，第二行起填写，最多 1000 条，原文件上限 2 MiB。文件内下拉固定中文：邮箱/域名、允许/拒绝、启用/停用；后端转换为数字枚举，预览随界面语言显示。说明页含字段解释、限制和不参与导入的示例，数据页不预填示例；导出只有“导入数据”页，允许回导。
- API 契约：现有 import/preview/import/export URL 与 action 不变；导入 JSON 为 `{fileName,contentBase64}`，预览改为 `{rows:[{line,rawValues,data,errors}],errors}`。`rawValues` 保留六格原文，字段合法时 `data` 是 `{scope,pattern,action,name,remark,isEnabled}` 数值枚举/规范化结构，字段错误时为 null；重复行仍保留合法 data。Excel 中文转换只发生在后端，前端严格解析数字 DTO、使用现有枚举+i18n 显示，不识别中文业务值。导出 envelope data 为 `{fileName:'mail-recipient-rule.xlsx',contentBase64}`。
- 维护者讨论后确认边界：Go 使用 Excelize，前端仅原生 FileReader/Blob；模板上传 COS，用户导入文件直传业务 API、不落 COS/数据库，导出直接下载。不新增 Excel 中间件、通用业务框架或异步任务；文件安全、基础读写、业务编排分别留在模块私有 `xlsxArchive.go/xlsxCodec.go/xlsx.go`。保留 2 MiB/1000 行、解压 16 MiB/128 ZIP entry、最多两页/六列的有界检查，拒绝公式、宏、外链和伪装文件。百万用户、多实例基线不变；仅管理员低频动作处理文件，不新增热路径数据库查询，整批事务和既有 generation/outbox 一致性不变。
- 模板与配置：使用现有 `setting` 上传规则和标准 COS v2 objectKey；系统模板配置键和值类型 5 不改，仅把文件限制和对象引用切换为 `.xlsx`。不存完整 URL、不删除旧 COS 文件；新模板上传下载逐字节核验后才绑定。无需改表结构。
- 真实发布：维护者此前已允许本轮停服发布；脚本 `docs/database/2026-10-08-mail-rule-xlsx-template.ps1 -OldAPIStopped` 先检查 API/Worker 未运行、备份并通过 `pg_restore --list`，再构建一次性命令。最终发布于 2026-10-08 16:55（Asia/Shanghai）完成：旧 CSV 对象保留，新对象键为 `setting/.admin-storage/v2/p1/r1/c1/v1/2026/10/08/01b79dd8136197ec8548c1ae99eb41c8.xlsx`，COS 下载逐字节匹配；上传规则仅增加 XLSX MIME，`system.setting/global` 17 → 18，`message.mail/global` 4 不变，Redis ready=18，未清 Redis、未改表结构、未删除旧 COS。备份与审计位于 `%LOCALAPPDATA%\Admin\backups\mail-rule-xlsx-20261008-165422-6454f26a`，SHA256 `D08A35CB907E8E9E51EC6699E109EF9B5F86F8CE7A049C1C2B93F8FF866728A4`；不可变 manifest 位于 `%LOCALAPPDATA%\Admin\maintenance\mail-rule-xlsx-template.json`，重复 publish/verify 后事实快照一致。
- 联合验证：后端 `go test -p 1 ./... -count=1` 1992 项通过，`go vet ./...`、`go build ./...` 通过；前端 `pnpm vitest run --pool=threads --maxWorkers=1` 112 文件/960 项通过，`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`（0 findings）、`pnpm build` 通过；模板经 Artifact Tool inspect/render 及后端解析测试确认两页、空数据页、三组下拉、固定表头和示例不导入。构建仅有既有大 chunk 警告。当前未提交 Git；由维护者启动 API/Worker 后刷新前端验收。
- 实际文件：后端 `message/mail/recipientRule/{xlsx,xlsxCodec,xlsxArchive,handler,protocol,repository,route}.go` 及 XLSX/数值测试，替换原 CSV 实现及对应测试；`system/setting/{service,cache,mail_template_object_key_test}.go`、`shared/i18n/catalog.go`、`go.mod/go.sum`。前端 `api/message/mail.ts`、`api/{storage/upload,system/setting}.ts`、收件规则页及 `MailRuleImportDialog` 私有读文件/CSS、`SettingDialog`、`UpMedia/upload.ts`、`utils/request.ts`、中英文 i18n 和镜像测试。新增模板、一次性 `cmd/mail-rule-xlsx-template-publish`、PowerShell runner，并更新本文件/design/architecture。
- 公共改动边界：请求层仅让主动取消不弹错误通知；存储 URL API 增加可选 AbortSignal；UpMedia 规范 `.xlsx` MIME 并保留混合类型选择器行为；设置只收紧当前模板的 `.xlsx` 媒体对象键。这些改动均有定向及全量回归，不加入前端 Excel 解析依赖。
- 红绿证据/未运行项：结构化预览先复现旧 `values` 文本响应失败，再实现 `rawValues/data`；非法 Sheet/稀疏坐标/损坏共享字符串部分读取、严格 DTO、设置旧格式缓存、取消及发布失败恢复均先失败后通过。发布 runner 两次前置检查失败（DSN 使用关键字格式、pg_dump 不接受 TimeZone）时尚未业务写入，修正后备份/上传/绑定/幂等验证通过。未做浏览器或真实 Excel 应用人工验收、未使用 Computer Use；独立审查代理多次因模型容量错误未能完成，不宣称独立审查通过。此前实施期间误停过一个 go 进程已告知维护者，影响未确认；本次最终全量验证已重新运行，发布阶段无需停止进程。

## 邮件收件规则数值协议（2026-10-08，代码与真实迁移已完成）

- 维护者最后确认的契约：仅 `message_mail_recipient_rule` 的 `scope/action` 改为 SMALLINT；`scope: 0=email, 1=domain`，`action: 0=deny, 1=allow`。Go 使用独立 int16 枚举，API 使用数字，前端常量+i18n label；固定程序协议不迁入可编辑字典。0 是合法值，HTTP 缺字段/null/旧字符串/未知值仍须拒绝。
- 既有规则保持：邮箱优先、同级拒绝优先、默认允许；未删除 `(scope,pattern,action)` 唯一；CSV 仍用 email/domain、allow/deny，模板/COS 对象不变；页面及 CRUD/CSV 动作权限全部不变，不修改 SMS 或身份变更日志。
- 容量与一致性：沿用百万用户/多实例基线、现有 runtime generation 和有界冷回源，不新增热路径数据库查询。数值快照严格拒绝缺枚举/旧字符串/非法枚举；迁移原位转换全表（含软删），同事务推进 `message.mail/global` generation/outbox 一次并定向发布 Redis 状态，不删除业务 Redis key、不重置限流额度。
- 已实现：后端 `recipientRule` 使用独立 Scope/Action int16，HTTP 指针 DTO 区分缺字段/null 与合法 0；Repository 组合键与更新写入数字，CSV 文件边界双向转换原文本 token。Mail runtime schema 升至 2，拒绝旧字符串、规则字段缺失/null/非法值；Model 仍只保留持久化映射。前端 `enums/mailRecipientRule.ts`、Mail API 和规则页面使用封闭数值类型与现有 i18n，默认邮箱/拒绝均为 0，创建/编辑零值不丢失。
- 实际迁移：维护者本轮明确允许停止并迁移；2026-10-08 13:31（Asia/Shanghai）核对本项目 API/Worker 未运行、16301 无监听，无需停止任何进程。执行 `docs/database/2026-10-08-mail-recipient-rule-numeric.ps1 -OldAPIStopped`，exit 0。真实 PostgreSQL 原 1 条 email/allow 转成 scope=0/action=1，所有其余字段、ID、时间和规则数保持，两个 SMALLINT 的 CHECK 都为 0/1，原唯一索引保持；`message.mail/global` 3 → 4，Redis ready=4。重复 SQL 与同步完整审计哈希不变；其他 generation、平台菜单版本不变。
- 备份/审计：`%LOCALAPPDATA%\Admin\backups\mail-rule-numeric-20261008-133140-9550d023\public-before.dump`，SHA256 `BD15AC2C5C2626CA596AA903D4FCBCA23213731878116CAE4544C5E80C346C83`，`pg_restore --list` 通过；同目录 before/after/repeat JSON 的规范化规则哈希一致，schema-only 实库导出已更新 `docs/database/current.sql`。未删除 Redis key、未修改限流额度、COS 文件或模板设置；API/Worker 保持停止，由维护者启动新版本。
- 后端验证：数值/严格绑定/缓存/CSV 红测后通过；`go test ./internal/module/message/mail/... ./internal/database ./cmd/mail-recipient-rule-numeric-migration -count=1`、`go test ./internal/architecture -count=1`、`go test ./... -run '^$'`、`go fmt ./...`、`go vet ./...`、`go build ./...` 均通过。迁移测试在隔离 PostgreSQL schema 覆盖四组映射、软删记录、数据保持、约束、唯一性、幂等及源数据异常整批回滚；helper 的隔离 PG/Redis DB13 测试验证审计哈希与定向单调发布。开发中一次缓存 500ms budget 用例受并发负载失败，单测及最终完整相关包重新运行均通过，未放宽预算。
- 前端验证：`pnpm vitest run tests/api/message/mail.test.ts tests/api/message/mailRuleCSV.test.ts tests/views/message/mail/index.test.ts --pool=threads --maxWorkers=1`，3 文件/54 项通过；`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`（0 findings）、变更文件 Prettier、`pnpm build` 与 `git diff --check` 通过，只有既有富文本大 chunk 提醒。独立最终审查无遗留问题。未跑全量 Go/Vitest、浏览器人工验收或真实邮件发送。
- 工作区：开始时 HEAD `4e90fe75`、工作区干净；未提交。期间出现非本任务产生的 `MailRuleImportDialog/index.vue` 单行格式改动，保留，不计入本轮功能。维护者新反馈：CSV 模板只有表头不易填写，已解释 CSV 与 .xlsx/.xls 差异，建议 .xlsx 的说明页+数据页+下拉；等待确认是否统一改 .xlsx，当前格式/COS 模板未变。

## 登录页后端恢复后的重试（2026-10-08，代码已完成）

- 范围：保留上一轮未提交的邮件修复，只处理登录页初始化失败后的恢复；未新增权限、修改后端/数据库、清理 Redis 或操作运行中的服务，未提交 Git。
- 根因：登录方式失败在页面 `configFailed`，认证初始化失败在 Auth Store；现有按钮只重跑 `getLoginConfig`，即使成功也不会重新确认会话，旧的 `auth.errorMessage` 始终显示。仅认证失败而配置成功时还缺少重试入口。
- 实现：页面私有 `web/src/views/auth/login/useLoginInitialization.ts` 管理配置、失败状态和手动重试；`index.vue` 继续编排表单。重试复用原受保护路由守卫的 refresh → me → Access 流程，不把配置成功当作认证成功；有效会话返回原目标，401 留在登录页，依赖持续失败保留可重试状态。公开或外部 redirect 回落 Dashboard，认证已恢复但路由失败时仅提供跳转重试、不再展示登录表单。
- 请求层：`web/src/utils/request.ts` 的终态 401 仍清理 Auth/Access、通知一次；已处于 `/login` 或 `/login/` 时不再整页跳回自身，避免 `redirect` 嵌套及原目标丢失，受保护页面的跳转行为不变。
- 交互/故障边界：重试期间锁定按钮和表单，refresh 成功而 me 尚未完成时保持加载反馈；离页后不再加载旧配置。配置瞬时失败保留选择/草稿但清空可用选项、隐藏表单，并在发送码、完成 Captcha、提交入口拒绝失败状态；成功新配置不再允许原方式时才切换并清理旧 proof。初始化逻辑拆出后 SFC 为 467 行，未绕过 500 行架构限制。
- 红绿回归：新增 `web/tests/views/auth/login/retry.test.ts`、`web/tests/utils/requestLoginRecovery.test.ts`，通过真实 Login/Router/Pinia、受控 API 和真实 Axios 请求层模拟后端故障及恢复。旧错误残留、缺重试入口、401 嵌套、加载反馈消失、草稿丢失及跳转失败后误展示表单均先观察到失败，再修复通过；未访问真实后端。
- 已运行（`web`）：`pnpm vitest run tests/views/auth/login/index.test.ts tests/views/auth/login/retry.test.ts tests/utils/requestLoginRecovery.test.ts tests/utils/request.test.ts tests/router/index.test.ts tests/api/auth/login.test.ts tests/store/auth.test.ts --pool=threads --maxWorkers=1`，7 文件 / 103 项通过。`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`（0 findings）、变更文件 `pnpm exec prettier --check`、`pnpm build` 与根目录 `git diff --check` 通过。只读审查及最终增量复核无遗留问题；构建仅保留既有 rich-editor 大 chunk 警告。
- 未运行/后续：未跑全量 Vitest、未重跑后端测试，未使用 Computer Use 或进行浏览器人工验收。维护者刷新前端后可按“后端未启动 → 打开受保护页/登录页 → 后端恢复 → 重试”验收；本轮无需迁移或重启后端。邮件提示与登录重试两项代码修复完成，下一步再逐项处理数据库字段。

## 邮件发送失败提示修复（2026-10-08，代码已完成）

- 范围：维护者已确认先修改邮箱时的邮件错误提示；本轮未处理登录重试、字段整改、CSV 或媒体设置。开始时工作区干净；未提交、停服、重启或执行业务库迁移，未清理业务 Redis 数据。
- 根因与修复：SES Client 原先把供应商错误码压成 `ses_error`，Mail Service 再统一映射 `503/10006`“服务暂未就绪”。现在保留腾讯云错误码，发送渠道失败使用 `503/18001`，按明确的供应商代码区分拒收、超时、限频、配额/模板不可用；未知错误使用明确的“邮件发送失败”提示，中英文同时提供。收件规则拒绝仍为 `403/18000`，本地额度限制、数据库/Redis 失败保持原语义。
- 诊断与失败边界：管理员日志保留供应商代码、摘要及可用的 RequestId，遮盖当前 SecretID/SecretKey/验证码；普通用户只收到固定公开文案。代码限长及字符校验、摘要 NUL/非法 UTF-8 修复和安全截断避免坏响应破坏日志；缺少发送回执不再当作成功。包装错误使用 `errors.As`，日志写失败保留供应商与数据库两条错误链。SDK 会压平网络错误，因此在每次 Send 独立的 transport 边界捕获超时类型，不解析错误文案、不新增重试或共享可变请求状态。
- 实际修改：`server/internal/storage/mail/tencent_ses.go`、`server/internal/module/message/mail/{errors,provider,service}.go`、`server/internal/shared/i18n/catalog.go` 及其测试；补充 `user/email` Handler/Service 回归和 `web/tests/utils/request.test.ts`。前端生产代码无需修改，现有请求层保留邮件提示且只通知一次；失败发送清理未发送 proof、不启动前端成功倒计时的既有协议不变。
- 红绿验证：原先公开错误码/文案、SDK 错误码保留、包装错误、日志双错误链、畸形回执、诊断截断/脱敏、网络提前超时均先观察到预期失败，再修复通过。腾讯云请求全由本地 HTTP transport 模拟；数据库集成测试使用隔离 schema，未发送真实邮件。
- 后端验证（`server`）：`go test ./internal/storage/mail -count=1`；`go test ./internal/module/message/mail/... ./internal/module/user/email ./internal/module/auth/login ./internal/shared/i18n ./internal/shared/response ./internal/architecture -count=1`；`go fmt ./...`、`go vet ./...`、`go build ./...` 均通过。
- 前端验证（`web`）：`pnpm vitest run tests/utils/request.test.ts tests/api/user/email.test.ts tests/views/user/profile/index.test.ts --pool=threads --maxWorkers=1` 为 3 文件 / 64 项通过；`pnpm typecheck`、`pnpm exec eslint tests/utils/request.test.ts --max-warnings 0`、`pnpm exec prettier --check tests/utils/request.test.ts` 和根目录 `git diff --check` 通过。只读审查及超时补充复核无遗留问题。
- 未运行与后续：未跑全量 Go/Vitest、前端构建或浏览器验收，未做真实腾讯云发送联调；维护者重启后端后检查实际失败反馈，本轮不需要数据库迁移。下一项仍是登录页后端恢复后的重试 Bug，再恢复字段清理。

## 系统设置媒体值类型（2026-09-30，代码与真实迁移已完成）

- 最终契约：媒体是通用设置编辑器的值类型 `5`，与字符串、数字、布尔值、JSON 并列；没有独立媒体页签。系统设置保留“高级设置 / 协议与隐私”，默认高级设置。默认头像、CSV 模板和任意自定义媒体设置都在高级列表的新增/编辑弹窗中使用 UpMedia，统一上传编码 `setting`，配置值仍保存 TEXT 对象键。
- 代码：`SettingDialog` 的媒体类型显示 UpMedia，上传中锁定类型/保存，关闭或切换类型取消旧上传；上传权限与设置保存权限独立，清除只写空字符串。前端和后端严格接受 v2 对象键；品牌默认头像限定图片扩展名，CSV 模板限定 `.csv`。品牌缓存/设置 record 缓存拒绝 URL、旧路径、错误扩展和损坏媒体快照。
- 真实迁移：维护者允许停止 API/Worker 后，已核验二进制模块路径并停止 PID 15028/30872 及其 api/worker 子进程，于 2026-09-30 20:19 执行 `docs/database/2026-09-30-system-setting-media-value-type.ps1 -OldAPIStopped`，exit 0。`system_setting.value_type` CHECK 从 1–4 扩展为 1–5；仅 ID 5 `app.brand.default_avatar` 和 ID 13 `message.mail.recipient_rule.import_template_object_key` 从类型 1 改为 5，value/objectKey 原样不变；标题保持类型 1，`value` 列仍为 `text`。
- 版本与幂等：`system.setting/global` 从 16 推进至 17，Redis 定向状态同步为 17；Admin 菜单 15、Canvas 1、`message.mail/global` 3 保持不变。SQL/Redis 两次幂等、坏源数据整批回滚、约束和值核验通过；未删除 Redis key、未修改 COS 文件或配置。API/Worker 当前仍停止，由维护者启动新版本。
- 备份：`%LOCALAPPDATA%\Admin\backups\system-setting-media-type-20260930-201958\public-before.dump`，SHA256 `EE2C9428C31CE00A79020558A2064787DA9E04B450BC78165265C2368AF2C78D`，`pg_restore --list` 通过；`docs/database/current.sql` 已刷新。未提交 Git。
- 验证：后端设置/邮件/数据库/架构定向测试、`go fmt`、`go vet`、`go build` 通过；前端设置/API/UpMedia/邮件、用户和布局相关 8 文件/97 项 Vitest 通过，`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`（0 findings）、变更文件 Prettier、`pnpm build` 与 `git diff --check` 通过，仅保留既有 rich-editor 大 chunk 警告。未跑全量 Go/Vitest、未做浏览器人工验收或使用 Computer Use。邮件提示和登录重试 Bug 继续延期。

## CSV 模板统一存储对象键（2026-09-30，代码与真实迁移已完成）

- 当前契约：配置名为 `message.mail.recipient_rule.import_template_object_key`，原配置行 ID 13 保留；值为 `file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/4a3990efabc869c6448c200b525023c5.csv`，不含协议或域名，旧 `_url` 配置已不存在。模板接口只返回 objectKey，前端复用 `/api/v1/storage/object-url` 解析，运行时不保留旧 URL 兼容分支。
- 实现位置：设置和邮件收件规则的后端校验、前端 API/弹窗/i18n 已统一为对象键。原 v2 编码及解析协议原样搬到 `server/internal/storage/objectKey`，供上传、设置和邮件复用，不把带认证依赖的 uploadRule 业务模块反向引入邮件模块。前端 `utils/storageObjectKey.ts` 校验同一格式，存储 URL DTO 只接受无凭据的 HTTPS 地址；模板解析失败和过期响应不会生成猜测链接。
- 容量/权限：保留导入 action、配置 generation、对象物理版本和平台隔离；模板读取与对象解析复用现有 Redis 确认和有界回源，不新增每请求 PostgreSQL 配置查询。下载解析不额外要求文件上传权限。
- 真实迁移：本轮再次取得维护者停止及迁移许可后，停止本项目 API/Worker，于 2026-09-30 16:36（Asia/Shanghai）执行 `docs/database/2026-09-30-mail-rule-template-object-key.ps1 -OldAPIStopped`，exit 0。通过当前 Admin 上传规则的 IssueCredentials 上传到标准 v2 路径，仅为该规则补充 `csv`/`text/csv`，未改文件大小、公私有、规则编码、平台或 COS 配置。配套 SQL 原位改名并推进 `system.setting/global` 15 → 16，Redis 同步为 16；Admin 菜单版本保持 15，Canvas 保持 1，邮件 generation 保持 3，规则和角色授权数量未变。
- 断点与幂等：`server/cmd/mail-rule-template-object-key-migration` 在 `%LOCALAPPDATA%\Admin\maintenance\mail-rule-template-object-key.json` 留下无凭据 manifest；完整写入、Sync/Close 后以同卷链接原子发布，拒绝覆盖。所有新建/复用路径均校验哈希、平台/规则坐标及标准 resolver URL；上传后、改设置前验证实际下载。prepare、SQL、Redis 发布重复执行后完整事实快照不变。旧 COS 对象作为既有资源保留，未自动删除。
- 备份：`%LOCALAPPDATA%\Admin\backups\mail-rule-template-object-key-20260930-163633\public-before.dump`，627086 bytes，SHA256 `EDB036769AB5797986CB04FE5C76ADD475D65E837E947AA095D7D9F7997D66E5`，`pg_restore --list` 通过；同目录保留 before/after JSON 和模板 manifest。此轮无表结构变更，未重复导出 schema 快照。
- 已验证：新 SQL 原位保 ID、来源冲突/畸形键回滚、整批幂等；命令的 manifest 一致性、原子写入、白名单增量和下载错误测试；objectKey/uploadRule/recipientRule/setting/database 六包及架构、邮件根模块定向 Go 测试通过。前端 CSV API、存储 API、对象键、UpMedia、邮件页、设置页共 6 文件/64 项 Vitest 通过，变更文件 Prettier 通过。真实模板经标准存储 resolver 和匿名 HTTPS 下载逐字节验证为原 59-byte CSV，SHA256 `ce013c56fed4fec13fa098f42415017da517991e111664ae9dd187a62ccc47b8`。只读审查的三个问题已修复，复核无 Critical/Important 阻断。
- 收尾验证：`go fmt ./...`、`go vet ./...`、`go build ./...`、`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`（0 findings）、`pnpm build`、`git diff --check` 均通过。URL 控制字符校验改成字符码判断后，存储 API 3 项再次通过；构建仅有既有 rich-editor 大 chunk 警告。本轮未重跑全量 Go/Vitest，未做浏览器人工验收或使用 Computer Use；API/Worker 保持停止，由维护者启动新版本并刷新前端。未提交 Git、未清理 Redis；邮件提示与登录重试 Bug 仍延期。

## 邮件收件规则 CSV 导入/导出（2026-09-30，代码与真实库迁移已完成）

- 范围：保留现有邮箱/域名匹配与字段类型；新增 UTF-8 CSV 模板、预览校验、整批事务导入和同格式导出。模板已按上方后续整改统一为系统设置中的标准 objectKey，为空时明确显示未配置，不虚造 COS 地址。
- 权限：邮件 page 仍为 `message:mail:view`，列表沿用 `message:mail:list`；模板配置读取、CSV 预览与确认导入使用独立 `message:mail:rule:import`，导出使用 `message:mail:rule:export`，后端均接现有认证与权限 Middleware。有 page 与 CSV action、没有 list 时仍显示收件规则 tab 和对应按钮，不请求列表、不显示假空表；普通角色未自动获得新 action。
- 容量/一致性：单次最多 1000 条、CSV 1 MiB；预览一次有界重复键查询，确认重新校验；不覆盖已有规则，任一错误不写入。批量写入复用 Mail runtime mutation，一次事务只推进一次 `message.mail/global` generation/outbox；设置读取复用现有 generation 缓存，Redis 故障不回退旧值。
- 实现：后端 `message/mail/recipientRule` 新增模板链接、预览、确认导入与导出接口；导出包含启用和停用规则，不含软删除，超限明确失败而不截断。前端复用 AppDialog/AppTable 展示文件行号、错误与分页；切换文件、关闭弹窗或权限变化会丢弃旧响应，下载使用原生 Blob，未增加 Excel 依赖。入口严格检查原始 JSON UTF-8 和未配对 surrogate；批量事务回滚恢复生成的 ID，避免重试复用回滚 ID。
- 首次模板发布记录（后续已转为上方标准对象键）：`docs/templates/mail-recipient-rule-import.csv`，59 bytes、UTF-8 BOM、六列中文表头、CRLF，仅表头而无会误导入的样例数据。维护者要求直接上传后，已复用现有 COS 配置 ID 1 发布固定公开模板，未放宽通用上传白名单。下载地址为 `https://cos.zgm2003.cn/system/templates/mail-recipient-rule/ce013c56fed4fec13fa098f42415017da517991e111664ae9dd187a62ccc47b8/mail-recipient-rule-import.csv`，通过现有系统设置 Service 回填 `message.mail.recipient_rule.import_template_url`，保留内置、启用和禁止删除约束。
- 首次模板发布验证：匿名 HTTPS GET 与本地文件逐字节一致，SHA256 `ce013c56fed4fec13fa098f42415017da517991e111664ae9dd187a62ccc47b8`；HEAD 返回 200、`text/csv; charset=utf-8`、59 bytes 和附件下载文件名。邮件 ImportTemplate Service 消费到同一地址；`system.setting/global` 在本次配置更新中从 14 到 15，PostgreSQL/Redis 均为 15。重复执行只验证既有链接，不重复上传或推进 generation。发布使用一次性本地辅助程序，完成后已删除；未新增运行时代码，未停止或重启服务。
- 真实迁移：维护者授权停止本项目 API/Worker 后，2026-09-30 15:34（Asia/Shanghai）完成备份、SQL 与 Redis 版本发布；首轮 schema 导出因 PowerShell 参数拼接失败，修正 runner 后于 15:37 整轮复跑 exit 0，SQL/Redis 两次幂等均通过。迁移文件为 `docs/database/2026-09-30-mail-recipient-rule-csv.{sql,ps1}`，固定状态命令为 `server/cmd/mail-recipient-rule-csv-migration`。Admin `menu_version` 从 14 到 15，Canvas 保持 1；`system.setting/global` 从 13 到 14，`message.mail/global` 保持 3。Redis Admin 菜单状态从陈旧的 13 同步到 15，设置状态同步到 14；未删除 Redis key，规则数量和角色授权数量不变。
- 备份与快照：迁移前原始备份为 `%LOCALAPPDATA%\Admin\backups\mail-rule-csv-20260930-153429\public-before.dump`，624001 bytes，SHA256 `5BE1419694EB3C57CDF33E58572ACD405E7F68773CEDE8B47E2525B9960CB865`；成功复跑备份为同级 `mail-rule-csv-20260930-153710\public-before.dump`，624225 bytes，SHA256 `07082D021FC9FB3D91C727EF430AE2DCE2918F22C0BFCACF6C3EB9610B689A8D`。两份 `pg_restore --list` 均通过。`docs/database/current.sql` 已由真实库刷新，123757 bytes，SHA256 `753ADCC1BC4C29585F28F58F20A161C8439902084852480E5ADEEB812368AF17`。
- 验证：后端 `go fmt ./...`、`go vet ./...`、`go test ./... -p 1 -count=1`、`go build ./...` 全部 exit 0，覆盖 CSV 空文件/编码/表头/引号/重复/超限、公式转义再导入、跨批事务回滚、并发重复、独立动作权限、迁移冲突与幂等。前端 `pnpm vitest run tests/api/message/mailRuleCSV.test.ts tests/views/message/mail/index.test.ts tests/api/system/setting.test.ts tests/views/system/setting/index.test.ts --pool=threads --maxWorkers=1` 共 4 文件/55 项通过；变更文件 Prettier、`pnpm lint`、`pnpm check:architecture`（0 findings）、`pnpm typecheck`、`pnpm build` 和 `git diff --check` 通过。构建仅保留既有 rich-editor 大 chunk 警告；只读审查无 Critical/Important 阻断项。
- 未运行/保留项：未跑全量前端 Vitest，未使用 Computer Use 或做浏览器人工验收。全仓 `pnpm format:check` 在既有 `web/tests/views/user/account/index.test.ts` 格式问题处失败，已确认该文件与 HEAD 一致，未为本任务改动它。首次迁移后 API/Worker 保持停止，随后由维护者启动；对象键后续迁移的当前服务状态以上方条目为准。未提交 Git、未清理 Redis、未操作 Canvas。邮件错误提示与登录重试两个 Bug 按维护者要求延期。

## 用户邮箱/手机号变更记录整改（2026-09-29，代码与真实库迁移已完成）

- `user_email_change_log` 与 `user_phone_change_log` 的 `action` 统一为数值协议：`1=change`、`2=bind`；仅保留明文旧值/新值字段，删除 8 个重复的 hint/HMAC 审计字段，并保留动作与值形状约束。邮箱迁移从成功的绑定邮件日志恢复可验证历史值，无法恢复时事务回滚；手机号历史存在但缺少明文事实时事务回滚，不写假值。
- 新增管理员详情接口：`GET /api/admin/v1/user/account/:id/email-change-log` 与 `GET /api/admin/v1/user/account/:id/phone-change-log`。两个接口均使用独立 `user:account:detail` action，返回明文旧值/新值、数值 action、平台和时间，沿用严格分页协议。
- 用户管理操作列新增身份变更记录入口；弹窗使用 `el-tabs` 分为邮箱和手机号两套分页表格，按 `user:account:detail` 控制按钮，包含空态、加载失败、刷新和明文展示；分页/刷新/切换用户时用请求序号丢弃过期响应。新增 API DTO 严格解析、后端分层测试、页面/协议测试及两个 forward migration。
- 已验证：前端 `pnpm typecheck`、`pnpm check:architecture`、用户管理/API 定向 Vitest（3 文件/30 项）和 `pnpm build` 通过；后端 email/phone/database 定向 Go 测试、`go vet ./...`、`go build ./...` 通过。`go test ./... -count=1` 首轮受既有 Mail/SMS/UploadRule 缓存并发预算用例的环境抖动影响失败，五个失败包逐个串行复跑均通过；本次变更包无失败。
- 真实迁移（删除 8 个 hint/HMAC 字段）：确认 PostgreSQL/Redis 运行且 API/Worker 未运行后，于 2026-09-29 19:03（Asia/Shanghai）重新备份并执行两份 forward SQL；两份 SQL 随后重复执行，幂等通过。`user_email_change_log` 的 1 条历史记录恢复为 `old_email=zgm_2003@qq.com`、`new_email=123456@qq.com`、`action=1`；`user_phone_change_log` 原有历史记录为 0 条。两张表的 `action` 均为 `smallint`，明文字段、约束和 `(user_id, created_at DESC, id DESC)` 索引已核验。
- Admin 平台写入隐藏 action `user:account:detail`，挂在 `user:account:view` 下，`menu_version` 从 13 增至 14；Canvas 保持 1。修正 action 名称为“查看用户身份变更记录”并复跑两份迁移，菜单版本保持 14，幂等通过；未清理 Redis，未重启 API/Worker。
- 迁移前备份：原第一版备份仍保留在 `%LOCALAPPDATA%\Admin\backups\user-identity-change-log-20260929-181445\public-before.dump`；本轮删除字段前新增备份 `%LOCALAPPDATA%\Admin\backups\user-identity-change-log-drop-hints-20260929-190309\public-before.dump`，532050 bytes，SHA256 `4A30B0CEDDFAE47FDFB75A251CE8EDB2A962BCBD820AE2157F2501DFF6053930`，`pg_restore --list` 已通过。真实 schema-only 快照已刷新至 `docs/database/current.sql`，大小 123553 bytes，SHA256 `8ECA581D6E274B457ED9A31B9A3AB802FAF36F6C5609A4D9381F0EAC42A91B21`；代码仍未提交。

## 用户登录日志数值协议与审计（2026-09-24，代码与真实库迁移已完成）

- `user_login_log` 已统一为数值协议：`event_type` 为 `1=register`、`2=login`、`3=logout`；`login_type` 为 `1=password`、`2=email`、`3=phone`。`login_account` 改为 `account`，日志表删除 `session_id`，登出账号写当前 `username`，不回查历史登录方式。
- 邮箱/手机号验证码自动注册在创建用户成功后写 register，再写 login；并发唯一键竞争只写 login。登录、注册、登出审计均为带 500ms deadline 的 best-effort PostgreSQL 写入，不使用 Asynq，日志或登出用户名查询失败不阻断认证主流程。固定枚举保留在代码/数据库约束，不进入系统字典。
- 新增 forward migration `docs/database/2026-09-23-user-login-log-numeric.sql` 与受保护 runner `docs/database/2026-09-23-user-login-log-numeric.ps1`；包含历史登出空账号按 `user_id -> user_account.username` 回填、未知值拒绝、约束、索引和幂等复跑。runner 已改为 ASCII 编码以兼容 Windows PowerShell 5.1，并要求显式传入 `-OldAPIStopped`。
- 前端登录日志 DTO/查询改为严格整数协议，使用 `account`、移除 `sessionId`，增加登录方式列/筛选；注册/登录/登出 Tag 分别为 `primary/success/danger`，登出登录方式显示 `-`。
- 验证：后端 `go vet ./...`、登录日志/认证登录/用户账户/数据库定向 `go test`、`go build ./...` 通过；前端架构检查、登录日志 API/页面定向 Vitest 2 文件、`pnpm prettier --check`、`pnpm lint`、`pnpm typecheck`、`pnpm build` 通过。后端 `go test ./... -count=1` 曾受 Mail/SMS 缓存预算和 Scheduler 健康时序用例的并发负载抖动影响，四个失败用例随后单独复跑均通过；该全量命令不记为全绿。构建仅保留既有大 chunk 警告；未提交代码。
- 真实迁移：2026-09-24 11:14（Asia/Shanghai）确认 API/Worker 未运行后执行 runner；PostgreSQL 事务提交成功，重复执行同一 SQL 通过。只读核验为 `account` 存在且非空、`event_type/login_type` 为 `smallint`、`session_id`/`login_account` 不存在、数值约束和 `account, created_at` 索引存在；历史行 `id=132` 为 `account=admin,event_type=3,login_type=NULL`，`id=133` 为 `account=admin@qq.com,event_type=2,login_type=1`，非法计数为 0。
- 迁移前备份：`%LOCALAPPDATA%\Admin\backups\user-login-log-numeric-20260924-091142\public-before.dump`，478356 bytes，SHA256 `D9D31309A70976AEDDEC0CF6C4F98BE52AAD5B095613BDBF14944675E1E16861`，`pg_restore --list` 已通过。真实 schema-only 快照已于 2026-09-24 11:16 刷新至 `docs/database/current.sql`。本迁移未清理 Redis、未重启 API/Worker。

## 管理页 UI 整改与筛选空值语义（2026-09-23，代码已完成）

- 系统设置「协议与隐私」：删除与页签重复的区块标题与描述，收敛卡片层级（原先页面卡片内还套 workspace / editor-shell 两层），改为页签固定、内容区内部滚动，编辑器高度自适应并自身滚动。
- 通知铃铛：未读项补齐与通知中心一致的视觉（主色浅底 + 左侧 3px 色条 + 标题旁圆点 + 标题加粗），并补 hover 与 `:focus-visible` 交互态。
- 通知中心去掉自定义卡片式设计，回到标准管理页：`AppSearch`（未读 / 类型 / 优先级，空值即全部）+ `AppTable`（展开行展示净化正文与链接动作；列为标题 / 类型 / 优先级 / 状态 / 发布时间 / 操作），保留 `message:notification:list/read/delete` 动作。后端只有 cursor 分页（无 total），因此保留「加载更多」按钮而不是页码分页。删除 5 个自定义组件与展示文件。
- 全项目移除筛选下拉的「全部」空概念选项（通知中心、通知任务受众、用户、登录日志、会话、角色、认证平台、对象存储 4 处、定时任务、操作日志，并补齐系统设置 / 字典 / 缓存代际 / 邮件日志 / 短信日志的占位），空值统一代表全部；占位文案统一为「全部X」，中英文键同步（新增 20 个、删除 7 个失效键）。页签与分段控件的「全部通知 / 未读通知 / 全部」是范围切换，保留。
- `AppSearch` 行为修复：下拉清空时回落到字段 `resetValue`（Element Plus 清空 emit `undefined`，此前 undefined 会进入查询模型）；独立筛选补 `clearable` 与 `value-on-clear`，定时任务状态筛选兼容空值。
- 登录页 `views/auth/login/index.vue` 超过页面 SFC 阈值（521 行）：工具函数与成功通知外移到 `views/auth/login/loginPage.ts`，现 476 行；跨模块相对 import 改为 `@/*` 别名。
- 验证：受影响范围定向 Vitest 共 20 个文件通过（设置页、登录页、通知中心、通知铃铛、通知任务、邮件、短信、存储、权限、用户、系统），其中「全部」清理轮 15 文件 179 项、通知中心重写后 8 项、补充回归 5 文件 54 项；`pnpm format:check`、`lint`、`check:architecture`（0 findings）、`typecheck`、`build` 全部 exit 0。未运行全量 `verify:frontend`，未做浏览器人工验收，未提交代码。

## 系统法律文档与登录页入口（2026-09-22，代码与迁移已完成）

- 系统设置整理为“站点信息 / 协议与隐私 / 高级设置”；用户协议与隐私政策各使用一份富文本正文，不按界面语言拆分，专用配置不再重复出现在高级设置列表。
- 后台读取沿用登录态，保存使用 `system:setting:update`；登录页新增两个匿名只读查看入口，不增加强制勾选。服务端只保留有限格式与 HTTPS 链接，并给外链补 `noopener noreferrer`；空正文、非法类型和异常配置明确失败或展示未配置状态。
- 新增幂等 forward migration `docs/database/2026-09-22-system-legal-documents.sql`，只 seed 两条空的内置配置，并在真实变化时将 `system.setting/global` generation 推进一次。已于 2026-09-22 执行：generation 从 5 推进到 6，两条 `app.legal.*` 配置均为内置启用字符串且正文为空，generation 6 写入 1 条 pending outbox；第二次执行保持 generation 6、配置 2 条和 generation 6 outbox 1 条。迁移前备份保存在 `%TEMP%\admin-postgres-before-legal-20260922-190224.dump`；未清理 Redis，正式正文仍需维护者提供并在后台发布。
- 定向验证：后端法律文档 Handler/Service/Route、专用列表隔离与 `cmd/api` 测试通过；前端系统设置 API、富文本编辑器、设置页和登录页共 38 项通过。全量格式、类型、架构、构建与后端门禁仍待本轮收尾执行。

## Vite 开发环境菜单跳转中断（2026-09-22，已修复）

- 冷缓存复现 Vite 在懒页面加载期间分批发现 Element Plus 深层依赖，输出 `optimized dependencies changed. reloading`，并出现动态模块导入失败；整页刷新后内存凭据丢失，路由守卫调用 refresh，故 refresh 是后续现象而非最初触发者。该恢复逻辑早于七提交整改，保持生产旧 chunk 兜底不变。
- 开发配置明确预构建当前第三方入口并关闭运行中自动发现，避免普通首次菜单跳转因 optimizer 更换版本触发整页刷新；配置断言先失败后通过。独立冷缓存浏览器顺序加载 19 个业务页面模块，全部成功，期间没有额外 document 导航或 refresh 请求。
- 验证：Prettier、ESLint、前端架构检查（0 findings）、`vue-tsc -b` 与 `pnpm build` 通过；Vite 配置、路由和请求定向测试串行 5 文件/69 项通过。`pnpm verify:frontend` 的全量 Vitest 阶段运行约 12 分钟未返回结果，已主动终止；另一次并行构建时有一项路由测试超时，停止并发负载后定向复跑通过。全量 Vitest 尚未完成，不能视为全绿。
- 该实验没有登录用户，也没有证明生产部署或外部网络断连不会导致动态资源加载失败；它针对已复现的 Vite 开发环境问题，不更改认证、RBAC、后端或 Canvas。

## 七提交整改止损复核（2026-09-22，已完成）

- 复核范围为 `2e820f52^..21e674ff` 七个提交及其后的未提交工作区。保留 Mail/SMS、Scheduler、NotificationTask 数值状态迁移，Scheduler 并发/健康与 dispatch-token TaskID 修复，NotificationTask Permission 窄 Reader，以及前端 DTO 解析、HTML 契约、聚合页时序和 AppSearch 类型收紧。
- 确认并修复一项未提交误改：邮箱验证码预检曾被缩减为单分钟窗口，现恢复为同平台、同规范化邮箱的分钟与十分钟双窗口原子预占；普通发送、验证码和管理员测试继续共享额度。
- 独立复核发现的两项前端收尾已修复：SMS 在列表权限异步就绪时同步加载页面目录并复用在途请求；`AppSearch` 字段显式声明与模型同类型的 `resetValue`，重置不再把必填标量伪装成 `undefined`。
- 删除 `2e820f52` 新增但已失效的 `docs/agent/audit/` 七份快照。它们仍把已完成事项写成待办，并包含与当前硬规则冲突的 icon catalog 方向；状态数值契约目录继续保留。
- `docs/local-work/2026-09-21-project-remediation-{spec,plan}.md` 不再作为执行计划：其中已验证的工作已经落地，数据库候选清理没有足够负载证据，不继续扩大整改。当前结论是完成本轮状态迁移与止损收尾后停止，后续只按真实需求创建小范围工作项。
- 验证：Server `go fmt ./...`、`go vet ./...`、`go test -p 1 ./... -count=1`、`go build ./...` 全部 exit 0；Web `pnpm verify:frontend` 全部 exit 0（103 个测试文件、745 项测试、生产构建通过，仅保留既有大 chunk warning）。Canvas 按维护者要求不在本轮范围。未执行数据库迁移、未修改业务数据、未清理 Redis、未 commit/push/rebase。

## Batch 11 AppSearch 字段值类型收口（2026-09-21，代码已完成）

- 将所有生产 `AppSearch` 消费点从默认宽泛 `SearchField[]` / `SearchFormModel` 改为页面自身的明确搜索模型；字段 key、数值状态、字符串 enum、日期范围和动态关联 ID 现在在编译期保持关联。
- Mail/SMS 日志、通知任务、权限平台/角色、对象存储、系统设置/字典/操作日志/缓存代际、用户/会话/登录日志页面均完成类型绑定；运行时筛选值、HTTP 参数和 UI 行为未改变。
- `AppSearch` 内部只保留读取泛型对象字段所需的 `keyof TModel` 收窄；每个字段的 `resetValue` 与模型字段类型关联，不增加 `any`、`as any`、宽泛 ambient declaration 或新公共抽象。
- 验证：`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`（0 findings）通过；受影响 14 个 Vitest 文件/166 项通过；受影响文件 Prettier 通过。未运行本轮前端全量 `verify:frontend` 和生产 build，由维护者执行；未执行数据库迁移、未修改 Canvas、未提交代码。

## Batch 10 消息聚合页权限时序收口（2026-09-21，代码已完成）

- 修复 `useMessageAggregateTabs` 只监听当前 tab、不监听列表权限异步就绪的问题：权限从 loading 变为 ready 后会加载当前 tab，避免页面已有权限但内容不请求。
- 权限被撤销时，进行中的旧请求被视为过期，不再把响应写回已无权页面；组件卸载时原有序列失效保护保持不变。没有新增全局 watcher 或常驻状态。
- 回归测试覆盖权限异步授予、权限撤销中的请求、tab 切换旧响应和组件卸载；SMS 页面目录也随列表权限就绪加载，并复用未完成的目录请求。
- 验证：`pnpm vitest run tests/composables/useMessageAggregateTabs.test.ts tests/views/message/mail/index.test.ts tests/views/message/sms/index.test.ts --pool=threads --maxWorkers=1`（3 文件/38 项）通过；`pnpm typecheck`、受影响文件 ESLint、Prettier 通过。未运行前端全量 `verify:frontend`，由维护者执行；未执行数据库迁移、未修改 Canvas、未提交代码。

## Batch 9 邮箱验证码双窗口限流止损（2026-09-22，代码已修复）

- 撤销未提交代码中把验证码预检缩减为单分钟窗口的错误改动；`PrepareEmailVerifyCode` 重新原子消费所属平台、规范化邮箱共享的 `business_email_minute + business_email_10m` 两个窗口。
- 验证码、普通业务邮件和管理员测试邮件继续共享同平台、同邮箱额度；任一策略缺失都失败闭合，不允许绕过十分钟总量限制。
- 回归测试先证明单窗口实现会失败，再覆盖两个窗口的次数与时长，以及缺少十分钟策略时不消费任何额度。
- 定向验证：相关回归测试、Mail 包连续三轮、Auth login、用户邮箱、Scheduler、NotificationTask 测试通过；一次并行包运行中的 Mail readiness 用例受环境负载瞬态失败，单测和 Mail 包连续三轮均通过，最终以串行全量门禁为准。未执行数据库迁移、未清理 Redis、未修改 Canvas、未提交代码。

## Batch 8 真实 Server/Web 验收与 Scheduler TaskID 修复（2026-09-21，已完成）

- 真实启动新 API、Worker 和 Web Vite 进程完成端到端验收；`/health`、`/ready`、登录、`/api/v1/auth/me`、`/api/v1/access`、字典 options、Mail/SMS 日志、NotificationTask CRUD、Scheduler options/schedule/job/execute/run、未登录 401、Canvas 平台访问 Admin API 403，以及字符串状态拒绝 400 均符合契约。
- 真实 Web 验收覆盖登录页、Dashboard、邮件、短信、通知任务、定时任务和角色授权弹窗；页面加载与交互过程中没有 Console error，数字状态筛选和 enum metadata 展示正常。浏览器必须使用 `http://localhost:16300`，`127.0.0.1` 不在当前 CORS allowlist 内；保留的 `vue-i18n` flatten warning 为既有告警。
- 首轮真实 Scheduler 运行发现旧 TaskID `system-scheduler:<jobId>:<attempt>` 在 Asynq 冲突时被 Publisher 错误视为已发布，旧 payload token 与新 Job 不一致，最终出现“数据库 queued、任务 archived”的假成功。已将 TaskID 改为 `system-scheduler:<jobId>:<attempt>:<dispatchToken>`，并补充冲突安全测试；`go test ./internal/module/system/scheduler -count=1` 通过。
- 修复后内置 Job `1/2/3`、新建 Job `7/8`、手动 Execute Job `27` 以及后续真实定时 Job 均成功完成并产生成功 Run。旧 Job `1` 的详情仍保留修复前的 `queued task delivery lease expired` 历史错误，不回写历史记录；最新 Job `63` 详情无错误。
- 本轮未执行数据库迁移、未清理 Redis、未修改 Canvas；临时 API/Worker/Web 进程在最终验证后停止。未提交代码，改动保留给维护者按提交规范整理。

## Batch 7 全量门禁（2026-09-21，已完成）

- 后端：`go fmt ./...`、`go vet ./...`、`go test -p 1 ./... -count=1`、`go build ./...` 全部 exit 0；全量 Go 测试通过，包含 API、Worker、数据库迁移、Scheduler、NotificationTask、Mail/SMS、RBAC、Session 和 Redis 相关包。
- Web：`pnpm verify:frontend` 全部通过：Prettier、ESLint、架构检查（0 baseline findings）、`vue-tsc -b`、103 个测试文件/742 项测试、生产构建均通过。构建仅保留既有大 chunk warning。
- Canvas：`pnpm build` 通过（TypeScript、Vite，8978 modules transformed）；仅有既有动态导入提示和大 chunk warning。
- 部署边界：`server`、`web`、`canvas` 未发现运行时引用根目录 `shared` 或 `contracts`；架构测试 `TestDeploymentUnitsDoNotUseRootSharedDirectory` 通过。根目录 `shared`/`contracts` 不纳入部署。
- `git diff --check` 通过。当前未提交改动仍由维护者保留，Agent 未执行 commit、push、rebase 或回滚。

## 数据库证据审查（2026-09-21，已完成）

- 真实 PostgreSQL catalog 只读核对确认既有 schema-hardening 已生效：`user_session` 仅保留 `fk_user_session_user` 和 `fk_user_session_platform`，`ck_user_session_device_id_nonempty` 存在，空 `device_id` 为 0；`permission_access_version.user_id` 无默认值，`rbac_access_version_user_id_seq` 不存在。
- 约束名称仍有 `auth_session_*` 前缀，这是历史表重命名残留的命名债务；本轮不改名，避免无业务收益的额外迁移风险。
- 真实统计：`user_session` 86 行、`permission_access_version` 3 行、`message_notification_task` 3 行、`system_scheduler_job` 3 行。`EXPLAIN (ANALYZE, BUFFERS)` 已核对会话列表和 access version 查询；access version 使用主键索引，会话列表在当前 86 行规模下选择顺序扫描并在 0.322ms 内完成，未发现异常计划。
- `pg_stat_user_indexes` 已审查会话、权限版本、邮件/SMS 日志、通知任务和 Scheduler 表。0 次使用的索引包含低频管理列表、定时清理、发布租约和空日志表索引；没有跨部署窗口、查询样本和容量数据，不删除任何索引或字段。后续如需清理，必须先积累完整负载窗口并逐项提供查询证据。
- 本轮未执行 DDL、未修改业务数据、未清理 Redis；只读审查命令均 exit 0。

## Redis 只读容量实测（2026-09-21，已完成）

- 使用 `docs/local-work/2026-09-21-redis-probe.ps1` 从 `server/.env` 读取本地 Redis 配置；脚本只执行 `SCAN`、`TYPE`、`PTTL`、`DUMP` 和 `MEMORY USAGE`，不输出连接信息、key 名或 payload，也未执行任何写入或删除命令。
- 实际命令 `& .\docs\local-work\2026-09-21-redis-probe.ps1` exit 0；单次 `SCAN MATCH * COUNT 200` 用时 39ms，每个固定命名空间最多采样 16 个 key。共观察到 49 个 key：31 个 config generation state、7 个 Asynq key、4 个 auth state、2 个 authz state、1 个 auth policy 和其余状态；读取错误、SCAN 重复、未分类 key 均为 0。
- `config-cache:snapshot:v1:*`、`authz:permission:v8:*` 及所有短期租约/验证码/限流前缀当前均为 0；`config-cache:fill:v1:*` 与 `auth:current-session:*` 两个旧协议前缀也为 0。所有预期持久 state 均无 TTL，Asynq 仅 1 个内部 key 带 TTL；未发现应过期 key 缺少 TTL。
- 当前 31 个配置 key 是小型 generation state，16 个样本的序列化长度均为 62B、Redis 内存样本为 118B 至 144B；没有“重复缓存 payload 过多”的证据，因此本轮不删 key、不改缓存协议。剩余风险是本次 API/Worker 低活动时没有 snapshot 样本，峰值基数和命中率仍需在真实负载窗口复测。
- 后续风险：当前低活动窗口没有 snapshot 样本；Mail/SMS 聚合页、非日志 tab 旧响应和 AppSearch 字段边界整改已在 Batch 5 完成，watcher 创建时机与卸载归属已由对应测试覆盖。

## 前端请求、通知 HTML 与域 metadata 收口（2026-09-21，代码已完成）

- `request` 已移除响应泛型重载，固定返回 `Promise<unknown>`；全部 API 调用不再使用 `request<unknown>` 形式伪装响应类型，仍由各 API 模块严格解析 DTO。架构门禁现在禁止任何 `request<T>`。
- 通知 API 边界新增“后端已净化 HTML”验证：只接受后端通知白名单元素、无属性普通元素和加固后的 HTTPS 锚点；事件属性、危险 URL、SVG/script、图片及不安全锚点均作为协议错误拒绝，两个 `v-html` 消费点不再信任任意字符串。
- Notification、NotificationTask、SMS 和 Scheduler 的固定 variant/priority/audience/link/scene/status 展示值已收敛到所属域 metadata，页面只负责用 i18n key 生成 label；固定数值状态不进入可编辑字典。
- TDD 证据：请求泛型负类型契约先以 `TS2578` 失败；6 个恶意 HTML 样本先被解析器放行后转绿；4 组 metadata 契约先因导出不存在失败后转绿。完整 `pnpm verify:frontend` 通过：100 个测试文件、735 项测试、生产构建成功，仅保留既有大 chunk 警告。
- Redis 固定前缀只读 SCAN 探针已完成，结果见上方“Redis 只读容量实测”；后续只在真实负载窗口复测 snapshot 基数和命中率。

## NotificationTask 状态协议整改（2026-09-21，代码与迁移已落地）

- 后端 `notificationTask.Status` 已改为 `int16` typed enum：`1=draft`、`2=scheduled`、`3=queued`、`4=processing`、`5=completed`、`6=failed`、`7=canceled`；提供固定 metadata/i18n key 和终态标记，查询参数只接受数值。
- 前端 API parser、状态 tabs、表格、详情和测试已切换数值协议；label 由 enum metadata 的 i18n key 生成，不把状态交给可编辑字典。
- 新增 `docs/database/2026-09-21-notification-task-status-numeric.sql` 与 PowerShell runner：转换前拒绝未知值，增加 `SMALLINT + CHECK`、状态机约束、幂等复跑和可读备份校验。
- 本轮验证时误执行了该 runner（API/Worker 当时未运行），真实 PostgreSQL 的 `message_notification_task.status` 已从 `varchar` 转为 `smallint DEFAULT 1`；当前分布为 `draft=1`、`completed=2`，两项 CHECK 存在，runner 已重复执行通过。真实 schema 快照已刷新到 `docs/database/current.sql`，SHA256 `718EF89D28D970D06EC3412BE0628F803DA97420D94F29D625EA5DD3115AA6A4`。
- 真实数据库只读核对后已执行 Scheduler runner：`system_scheduler_job.status` 与 `system_scheduler_run.status` 均已转为 `smallint`，未知值预检查、约束、索引和幂等复跑均通过；首次尝试暴露并修复了默认值未先移除的真实 SQL 缺陷。API/Worker 仍未由 Agent 启动。
- Scheduler 迁移后真实 schema 快照再次刷新到 `docs/database/current.sql`，SHA256 `21A3FABA797108D764D93E68FC5416E1FFF287B4BA8F2D356FE11B36319EA2BD`。
- 菜单 icon 按维护者决定冻结为后端、前端各自白名单，不再建立共享目录、catalog 或生成器。Redis 实测和数据库字段/索引候选证据审查已完成；字段/索引暂不删除，等待完整负载窗口后再评估。

### NotificationTask 权限边界收口（2026-09-21，代码已完成）

- 新增 `permission/notification` 窄 Reader，由 Permission 模块独占平台、菜单、角色和用户角色查询；`message/notificationTask` 不再直接读取权限表。
- API/Worker 入口显式注入 Reader factory；事务和批次处理使用事务连接创建 Permission Reader，保证锁、快照和请求 context 贯穿，不引入 DI 容器或通用 Manager。
- 平台能力、用户/角色目标校验、平台/用户/角色选项、平台代理名称和批次受众展开均收口到 Reader，保持提交时角色冻结、`(platform_id,user_id)` 隔离与 keyset 分页。
- 验证：前端 `pnpm verify:frontend` 全绿（100 文件/725 测试，build 通过）；后端通知任务、API、Worker 定向测试通过，`go vet ./...` 与 `go build ./...` 通过。全量 `go test -p 1 ./...` 曾因 `notificationTask` 包在整仓资源竞争下超过 11 分钟被测试超时终止，单独重跑该包 10.1 秒通过；需在低负载环境重新执行全量测试。

## Scheduler 写路径与 Publisher 健康语义收口（2026-09-21，代码已完成）

- 修复 Publisher 在队列依赖失败且 `ReschedulePublish` 成功时返回 `nil` 的吞错路径：现在保留任务重排，同时把入队错误返回给监督循环。
- Publisher 连续 5 次非取消迭代失败后返回 `scheduler publisher unhealthy ...`，由 Worker runner 监督逻辑感知并退出；单次 Redis 抖动仍按现有 per-job backoff 重试，成功迭代会清零连续失败计数。
- 手动 Execute 现在在同一事务中锁定未删除 schedule、读取最新任务定义并创建 Job；Retry 在同一事务中锁定失败 Job，并对有关联计划的 Job 再锁定未删除 schedule。删除先提交时 Execute/Retry 拒绝创建；Execute/Retry 先提交时删除能看到活动 Job 并拒绝删除。
- 新增回归覆盖正常发布、Redis 失败重排并返回错误、Asynq TaskID 冲突幂等、连续依赖失败触发健康错误，以及 Execute/Retry 与计划并发写入的真实 PostgreSQL 行锁顺序。
- 验证：`go fmt ./...`、`go vet ./...`、`go test -p 1 ./... -count=1`、`go build ./...` 全部 exit 0；未执行数据库迁移、API/Worker 重启或 Redis 清理。
- 下一步：在 Scheduler 数据库迁移完成后，继续处理 notificationTask 权限事实下沉和平台受众隔离。

## 简约架构审计与分批整改（2026-09-20，已结束）

- 本节是已完成批次的历史执行摘要，已由上方“七提交整改止损复核”收口，不再作为进行中计划或后续工作清单。
- 已确认并保留的架构决策：持久化业务状态按业务域使用独立数值编码和强类型 enum；展示文案由所属域 metadata/i18n 管理，可编辑字典不承载状态机；HTTP 状态码和 Yes/No 保持各自协议语义。
- Scheduler 删除/启停并发、Publisher 健康语义、dispatch-token TaskID、NotificationTask 权限 Reader 与平台隔离均已完成；Mail/SMS、Scheduler、NotificationTask 数值状态代码和真实 PostgreSQL 迁移均已落地，具体证据见对应完成条目。
- Redis 容量探针和数据库候选字段/索引审查已完成。当前证据不支持继续删除索引、字段或改造缓存协议；只有出现真实负载证据或具体需求时才建立新的小范围工作项。
- `docs/agent/audit/` 临时清单已在 2026-09-22 删除，菜单 icon catalog 方向已否决。状态契约继续以 `docs/agent/status-enum-catalog.md`、当前代码、测试和真实数据库为准。
- 本批次至此停止，不再继续“全项目整改”；未列入当前状态的新工作不得从本节或旧 remediation plan 自动恢复。

### Mail/SMS 日志状态数值化（2026-09-20，已迁移）

- 已新增 `docs/database/2026-09-20-message-log-status-numeric.sql` 与同名 PowerShell runner；状态编码固定为 `1=pending`、`2=sent`、`3=failed`，迁移拒绝未知状态，保留现有数据、ID、约束和索引，并支持重复执行。
- 真实 PostgreSQL 已于 2026-09-20 21:33（Asia/Shanghai）在 API/Worker 未运行时执行并复跑；Mail 16 条 `sent` 记录已转换为 `2`，SMS 当前 0 条；两列均为 `smallint`，Mail 四场景 CHECK、两张表状态 CHECK 和状态索引存在，非法状态写入被拒绝。
- 迁移前备份：`%LOCALAPPDATA%/Admin/backups/message-log-status-numeric-20260920-213326/public-before.dump`，`pg_restore --list` 可读；加入 Mail scene CHECK 后复跑产生最新可读备份 `%LOCALAPPDATA%/Admin/backups/message-log-status-numeric-20260920-220949/public-before.dump`（241862 字节，SHA256 `D619652BD6727F007B0F31ED40281589F6AFF5BC4AF23D5642F9710A33579AF8`）。真实 schema 快照已刷新到 `docs/database/current.sql`，SHA256 `819CD9410831BD33C6A066E1312127C642BCC66960A0C15907CC5F7CFF0D39A1`。
- 定向后端验证已通过：Mail/SMS 模块、数据库迁移夹具、架构测试。首次并行全量测试受 Redis/并发资源抖动影响出现 5 个非确定性失败；隔离重跑全部通过，随后串行 `go test -p 1 ./... -count=1`、`go vet ./...`、`go build ./...` 和 `go fmt ./...` 均通过。前端 `pnpm verify:frontend` 由维护者执行。

### Schema hardening 迁移（2026-09-20，已迁移）

- `docs/database/2026-09-20-schema-hardening.ps1` / `.sql` 已在 API/Worker 未运行时真实执行并复跑；修复 Windows PowerShell UTF-8 BOM，并放宽首次 6 条/后续 0 条的前置检查后，runner 可重复执行。
- 结果：`user_session` 仅保留 `fk_user_session_user`，6 条空 `device_id` 已删除并增加 `ck_user_session_device_id_nonempty`；`permission_access_version.user_id` 默认值已移除，`rbac_access_version_user_id_seq` 已删除。
- 删除前从归档核对 6 条记录均已有 `revoked_at`（无活动会话），因此没有需要额外清理的 `auth:session:<platform>:<id>` 活跃快照；未执行宽泛 Redis 清理。
- 首次备份：`%LOCALAPPDATA%/Admin/backups/schema-hardening-20260920-220154/public-before.dump`，243709 字节，SHA256 `D221ED29BDCE3814F7591FA38D014F06B83465CC1DB03088014EBDAF36CAD4BD`，`pg_restore --list` 得到 441 个归档条目；幂等复跑备份为 `%LOCALAPPDATA%/Admin/backups/schema-hardening-20260920-222953/public-before.dump`（241862 字节，SHA256 `810B6373DD32ADA36039AD2BC63CAF00D6688E8A0104DF74FA4CB14A14FA5967`）。真实 schema 快照已刷新，SHA256 `819CD9410831BD33C6A066E1312127C642BCC66960A0C15907CC5F7CFF0D39A1`。

> 这是当前唯一的进度入口。它记录现在要做什么、已经交付什么和下一步做什么；不回填历史
> `docs/superpowers` plan。

## 全项目实时通道与站内通知（2026-09-19，已实现并迁移）

- 已交付 PostgreSQL durable realtime event/outbox、Redis Pub/Sub、一次性 WebSocket ticket、cursor resume、个人通知邮箱与通知任务管理。`web/` Admin Vue 是首个端到端消费者；Canvas 未写入 Vue 页面菜单。受众固定为最多 1000 个指定用户、按提交时点冻结的指定角色和 O(1) 平台广播。
- 通知任务支持草稿、last-write-wins 编辑、立即/定时提交、取消、复制和只读详情；提交后内容、平台、受众和时间冻结，取消不撤回已生成通知。通知中心是隐藏 page；铃铛仅在 Access ready 且有 list action 时连接实时通道，展示最近 5 条并按独立 view action决定是否显示“查看全部”。
- Review 已修复：WebSocket 帧按 connection epoch 串行处理，旧会话不能推进新会话 cursor 或触发紧急通知；连接先同步 detach，再由连接 loop 执行网络关闭，Redis Subscriber 不等待关闭握手；Pub/Sub 只向浏览器发送 envelope；多批任务复用数据库中的首次 `published_at`；API Subscriber 与 dispatch relay 不再吞退出/标记错误。
- RBAC 保持 page/action 独立：通知任务 create/update 各自拥有受对应 action 保护的 options，update 另有 draft edit 读取端点，不依赖 detail action；个人通知 page/list/read/delete 作为基础权限迁移给现有有效角色。Admin `menu_version` 由 11 递增到 12，Canvas 菜单与版本未变化。
- migration runner 的完整状态现在包含十表列默认值/长度、约束定义和索引定义的结构指纹；同名错误 CHECK/FK/index、错误长度/default 都会在事务内拒绝。Windows PowerShell runner 已保存为 UTF-8 BOM，`powershell.exe -File` 可直接解析。
- 真实 migration 于 **2026-09-19 12:07 +08:00** 首次执行并于 **12:07 +08:00** 完整复跑：十张表、13 个 Admin 菜单节点、两个必需 setting、角色授权、setting generation/outbox、menu/access version 全部通过；第二次 `structure/setting/generation/menu/grants/access/adminVersion/canvas` 指纹完全不变。固定 Redis patterns `system.setting` state/snapshot/fill、permission/menu state、access v8 snapshot、realtime ticket 每次均清理两轮且 `remaining=0`。
- 本次保留两份可读备份：`realtime-notification-20260919-120719/public-before.dump`（180426 字节，SHA256 `2C8655A5386CBBF4E7B2BBF15E7D860BCAD26E25B2B94CC5E0EFE8B2617BEDED`，327 项）和 `realtime-notification-20260919-120748/public-before.dump`（224922 字节，SHA256 `FCFD85E12CF62EDB0D9B09EE077CCD30F605D3B4A5782CE16A29EF9C27CC216B`，412 项），均位于 `%LOCALAPPDATA%\Admin\backups` 且未删除。
- 全量门禁通过：`go test -p 1 ./... -count=1`、`go vet ./...`、`go build ./...`；前端 `format:check`、`lint`、`check:architecture`、`typecheck`、96 文件 706 项 Vitest、生产 build 全部 exit 0。迁移后指定的 5 包真实 PG/Redis 探针通过；测试 fixture 已移除并发创建全局 `pgcrypto` extension 的竞态。
- `docs/database/current.sql` 已从真实 `public` schema-only 刷新，114300 字节，SHA256 `7BDA894A3FB01EAFF786A4B83D8AAC78FB236F21696378805E38C5A785046D3A`。Agent 精确停止了本项目 API/Worker，未停止 Vite，迁移后未启动或重启 API/Worker。
- 2026-09-19 浏览器全链路验收补齐并修复三项真实前端/协议缺陷：平台草稿 `targetIds` 固定输出空数组，保存成功不再因 DTO 解析失败而静默；通知任务表使用 `AppTable` 的 `cell-actions` 契约；铃铛 Popover 不再把 Tooltip 组件当作 reference，通知中心隐藏路由补齐中英文导航文案。真实 Element Plus 交互测试覆盖铃铛展开，通知表使用真实 `AppTable` 覆盖操作列。
- 本次真实 E2E 已覆盖平台/指定用户/指定角色立即发送、角色提交时点冻结展开、未来定时发送、到期前取消、提交后复制与草稿编辑、dispatch/realtime outbox 发布、WebSocket 在线刷新、铃铛最近 5 条、隐藏通知中心 all/unread/已读/全部已读/删除。PostgreSQL 与 Redis 测试账号及全部 E2E 事实已精确清理，原任务未改动；最新全量门禁为后端 test/vet/build 全部 exit 0，前端 format/lint/typecheck/architecture、98 文件 716 项 Vitest、生产 build 全部 exit 0。
- 下一个唯一真实模块是“通用定时任务管理”：实现 PostgreSQL 权威 schedule/job/run、管理页/RBAC、多实例 claim 与 missed-run recovery；使用静态任务类型分派；接管 notification batch 0/后续 batch 唤醒和 realtime/notification cleanup；回归立即/定时通知、取消、重启、Redis 丢失、提交后 enqueue 失败和跨实例单次 claim；随后删除 `message_notification_dispatch_outbox`、`notificationTask/dispatch.go`、Worker dispatch relay 和两个临时 `retentionTrigger.go`，不得保留双调度、旧表读取或过渡分支。

## COS 对象协议与统一配置缓存代际（2026-09-18，已实现并迁移）

- COS 逻辑配置现为全局多配置，上传规则继续按认证平台隔离并选择 `cos_config_id`；每个平台至多一条活动规则。规则编码继续由规范化子表维护，允许跨规则和跨平台重复。普通管理 CRUD 使用 last-write-wins，不新增乐观锁。
- `storage_cos_config_version` 保存不可变物理 Bucket/Region 坐标，逻辑配置只以 `current_version` 指向当前物理版本；v2 object key 自描述 platform/rule/config/physical version。项目未上线，迁移直接清空旧用户头像和品牌默认头像引用，不保留旧 DTO、旧对象 key、旧 Redis 协议或双读双写分支。
- `POST /api/v1/storage/upload-credential` 继续要求 `storage:object:upload`；`POST /api/v1/storage/object-url` 只要求有效登录态，并校验 key 所属认证平台。public 对象返回普通 URL，private 对象返回短期签名 GET URL；上传权限、菜单和前端上传判断均保留。
- 配置代际现覆盖 `system.setting/global`、`system.dictionary/global`、`message.mail/global`、`message.sms/global`、`storage.cosconfig/<configId>`。各模块共享 generation Repository/Store 与 outbox relay，但保留本模块 codec、Repository 和 Cache；没有通用 CacheManager、隐式注册器或跨模块万能 Adapter。
- generation 只负责缓存代际，不是管理员乐观锁。Mail/SMS rate policy 的 `revision` 已删除；`permission_auth_platform.menu_version/policy_version`、`permission_access_version.version`、Session/authority generation、验证码、限流计数和队列状态保持原业务归属，未迁入配置 generation。
- COS migration 已于 **2026-09-18 10:59 +08:00** 通过 `2026-09-17-storage-object-versioning.ps1 -OldAPIStopped` 执行，并于 **11:07 +08:00** 完整复跑验证幂等。两条逻辑配置均回填 physical version 1 和 `storage.cosconfig/<id>` generation/outbox；1 条用户头像旧引用已清空，品牌默认头像原本为空；Admin/Canvas `menu_version` 保持 `11/1`，编码行保持 2。两轮固定 Redis cleanup 的全部 pattern 均为 `remaining=0`。
- 配置缓存完成 migration 已于 **2026-09-18 11:00 +08:00** 通过 `2026-09-17-config-cache-generation-completion.ps1 -OldAPIStopped` 执行，并于 **11:07 +08:00** 完整复跑验证幂等。Dictionary/Mail/SMS 三个 global scope 与同代 pending outbox 均为 generation 1；Mail/SMS policy 事实与时间戳指纹不变，menu/policy/session/access 版本指纹不变；两轮固定 Redis cleanup 全部为 `remaining=0`。
- 各次迁移前 PostgreSQL 归档均曾通过 `pg_restore --list` 验证。维护者要求清理后，已于 **2026-09-18 12:00 +08:00** 删除 `%LOCALAPPDATA%\Admin\backups` 下全部 20 个项目备份批次并复验剩余文件为 0；当前不再保留可用于恢复旧 schema 的本地归档。
- `docs/database/current.sql` 已从最终真实 `public` schema-only 刷新，SHA256 为 `583A90DA6628D47C0D6E7A66A301B1FA5D22C4364F134FA7A659F4DE72D9EDE4`。精确 SQL 预算探针验证：rule route 冷回源 1 条 SELECT、热命中 0；COS snapshot 两实例并发冷回源合计 1 条逻辑配置 SELECT + 1 条物理版本 SELECT、热命中 0；route 与 snapshot 均 ready 的 object-url 为 0 条 SELECT。
- 兜底修复：上传规则页面初始化的 COS 配置摘要已改为按 `storage_cos_config.current_version` 连接 `storage_cos_config_version`，不再从逻辑配置表读取已迁出的 `bucket/region`；真实 PostgreSQL 回归覆盖当前物理版本和停用配置过滤。
- 上传规则新增与编辑弹窗均在访问模式旁提供同款启用/停用单选组：新增默认启用并将选择值写入创建请求；编辑态有 `storage:uploadRule:status` 时通过独立 PATCH 接口保存状态，无该权限时只读展示当前值，不把状态权限并入普通更新接口。
- 维护者要求最终迁移后，两个 forward runner 已于 **2026-09-18 11:59 +08:00** 再次完整执行并确认幂等：COS generation/outbox、编码行、`menu_version`、Mail/SMS policy 指纹及权限/Session/access 版本均保持不变。随后对旧协议和新协议固定 Redis pattern 各执行两轮清理，system.setting、Dictionary、Mail、SMS、COS state/snapshot/fill 与 v2 object route 均为 `remaining=0`；API/Worker 仍未由 Agent 启动。
- 验证通过：`go test -p 1 ./... -count=1`、`go vet ./...`、`go build ./...`、两个 runner 的 PowerShell Parser 检查、`git diff --check`。前端全量 Vitest 由维护者执行，本轮 Agent 未运行。迁移前只读进程检查未发现业务 API/Worker；迁移后未由 Agent 启动或重启服务。

## 统一配置缓存代际协议（2026-09-16，第一阶段已实现并迁移）

- 背景：`system/setting` 当前写路径为 PostgreSQL 提交后删除 Redis；删除失败会出现“数据库已提交但接口返回依赖错误”，缓存未命中期间的旧请求还可能在删除后回填旧值。该问题不只存在于品牌配置，必须按整个系统设置模块处理。
- 已确认方向：PostgreSQL 继续作为唯一事实来源；为低频写、高频读的配置型缓存提供持久化 generation，并使用 Redis `ready` / `invalidating` 状态、带 generation 的不可变快照 key、tokenized mutation lease 和 `shared/cacheFill` 有界冷回源。旧代际缓存不扫描删除，只通过不可达 key 与 TTL 回收。
- 已采用方案：按 `namespace + scope_key` 管理权威 generation，并以 durable outbox 负责 PostgreSQL 提交后的 Redis 发布恢复。generation 表只保存权威代际，不保存缓存内容；不能退化成全项目共用一个 generation，也不能替代业务实体版本或编辑乐观锁。
- 第一阶段已接入 `system_setting`，scope 固定为 `system.setting/global`；Redis state 使用 `ready/invalidating`、tokenized mutation lease，快照 key 带 generation，`shared/cacheFill` 提供有界冷回源。旧 `system:setting:v1:*` 仅由迁移 runner 定向清理，运行时不再 pattern delete。
- 已新增只读管理资源 `system/cacheGeneration`：页面权限 `system:cacheGeneration:view`，列表 API 权限 `system:cacheGeneration:list`，后端 API 为 `GET /api/admin/v1/system/cachegeneration`；不提供手工修改 generation、删除 outbox 或清 Redis 控件。
- 前后端菜单图标白名单已补齐现有菜单使用的 `lucide:list-checks` 与 `lucide:database-zap`。修复前菜单管理后端会因未知 icon 拒绝整棵菜单树并返回“菜单树数据无效”；数据库中的菜单值无需修改，也不需要再次迁移或清理 Redis。
- Element Plus 按需导入入口已显式加载 `ElMessage`、`ElNotification`、`ElMessageBox` 的程序式组件样式，恢复全项目成功/错误反馈与确认框布局；请求错误仍由 `utils/request.ts` 单次通知，对象存储写操作不再重复渲染页顶错误，未增加 Axios 成功通知或新的全局反馈封装。
- 真实迁移已于 2026-09-16 执行：`docs/database/2026-09-16-system-config-cache-generation.ps1 -OldAPIStopped` exit 0；Admin `menu_version` 10→11，Canvas 保持 1；`system_config_cache_generation`/`system_config_cache_outbox`、初始 generation=1/outbox、page/action 均已核验；`system:setting:v1:*` 两次定向扫描均为 0。迁移 runner 已修复为记录迁移前 generation/outbox 快照，重复执行不再把已发布事件或后续代际误报为失败。
- `docs/database/current.sql` 已从真实 PostgreSQL schema-only 导出刷新，包含两张新表、约束、外键和 pending 索引；没有手工伪造菜单数据。
- 真实定向 Go 回归已通过：`internal/shared/cacheGeneration`、`internal/module/system/cacheGeneration`、`internal/module/system/setting`、`internal/database`、`cmd/system-setting-cache-migration` 均 exit 0。前端 `pnpm typecheck` 与受影响文件 Prettier 此前通过。维护者随后在 Node `v24.12.0` 下执行全量 Vitest：首次运行有 10 项分布在 7 个真实 Element Plus 页面测试文件的默认 5 秒超时，没有业务断言失败。Vitest 现统一使用项目已有的 30 秒有界页面测试预算，并为 SettingDrawer、Session、OperationLog、AuthPlatform、Dictionary 测试补齐 wrapper 卸载，避免只删除 DOM 后遗留响应式作用域和 Teleport；本次 Agent 未运行测试，维护者修正后复跑并确认全量 Vitest 无问题。
- 兜底修复：`system_setting` Repository 现在标记事务回调已确定回滚的错误；Service 不再把明确的唯一冲突、未找到或代际冲突误判为“提交结果不确定”并返回成功。真正的数据库提交确认异常仍保留原有 outbox/Redis 恢复路径。
- 兜底修复：缓存代际管理 API 现在要求 Redis `ready` state 的 generation 与 PostgreSQL 权威 generation 完全一致；旧代际、新代际或异常代际均显示 `corrupt`，避免监控页把 Redis 残留误报为 ready。
- 现有版本必须保持业务归属：`permission_auth_platform.policy_version/menu_version`、`permission_access_version.version`、Session/authority generation 等不迁入通用表。字典、Mail/SMS runtime、COS 等现有配置缓存只能在逐项核对作用域、事务边界和故障语义后决定是否迁移。
- 后续阶段另立计划：Dictionary 先修复“读取开始捕获代际、发布时重新读代际”的竞态，再迁移；Mail/SMS runtime、COS 配置逐项评估后迁移。`menu_version`、`policy_version`、`permission_access_version.version`、Session/authority generation、验证码、限流计数和 Queue 数据不迁入中央表。
- 设计必须明确：容量和热点预算、namespace/scope 模型、读写状态机、数据库提交后 Redis 发布失败语义、进程崩溃与 Redis 丢数据后的自愈、冷回源上限、渐进迁移顺序，以及 generation 与 optimistic revision 的边界。
- 维护者偏好：Node 已为 `v24.12.0`，不要重复执行 `nvm use`；测试命令由维护者自行执行，Agent 未获新指令前不要代跑测试。

## 系统设置品牌配置与收尾修复（2026-09-15，代码已完成）

- COS 上传规则编辑态继续复用 `el-input-tag`，统一上传编码可编辑并随 PUT 请求提交；后端在同一事务中处理编码差集、软删、冲突和回滚，未改变上传大小、扩展名、MIME、路径或访问模式配置。
- Queue Monitor 修复首次进入时 `queue_stats` 重复请求：空队列不请求，首次获得真实队列请求一次，队列增删才刷新；Asynqmon dist 已重新构建并清理旧 hash/source map。Dashboard 回归测试通过。
- 系统设置新增品牌配置：`app.brand.title_zh_cn`、`app.brand.title_en_us`、`app.brand.default_avatar`；新增幂等 forward migration `docs/database/2026-09-15-system-setting-brand.sql`，默认头像只保存 COS object key 并固定复用 `avatar` 上传规则（单张、`image/*`、avatar variant，不传递新的大小限制）。真实 `admin.public` migration 已执行并重复执行验证幂等，三条活动内置配置均为 `value_type=1`、`is_enabled=1`、`is_builtin=1`。
- 新增已登录基础端点 `GET /api/admin/v1/system/setting/brand`（仅认证，不要求设置权限）供管理壳层读取品牌；品牌 Store 共享并发请求、支持 reset 竞态保护；标题按当前语言同步到 `document.title`、Aside/Header，用户无头像时回退默认头像。品牌保存仍使用 `system:setting:update`，管理页读取列表与编辑权限独立。
- 设置编辑器支持 string/number/bool/JSON，JSON 可格式化且严格拒绝非法内容；品牌面板移除解释性可见提示，仅保留字段标签和错误状态。
- 验证：Node `v24.12.0` 下前端相关 5 个测试文件 34/34 通过，`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture` 通过；Go `go test ./internal/module/storage/uploadRule ./internal/module/system/queueMonitor ./internal/module/system/setting ./internal/database -count=1`、定向 `go vet`、`go build ./cmd/api` 通过；新增真实 PostgreSQL 品牌事务回滚测试通过。页面测试因 Element Plus/Teleport 挂载成本将单文件预算设为 30 秒，未产生 unhandled error。
- 全量复核：Go 全量 `go fmt ./...`、`go vet ./...`、`go test ./... -count=1`、`go build ./...` 均通过；前端 `pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`、`pnpm build` 均通过（仅有既有大 chunk 警告）。前端全量 Vitest 为 78 文件/611 项通过、4 项失败；单独复跑确认失败仍是既有 `router/index.test.ts` 动态路由用例 1 项超时，以及 `permission/authPlatform` 滚动容器断言 1 项，不涉及本轮文件；本轮相关定向测试 34/34 通过。
- 迁移前备份位于 `%LOCALAPPDATA%\Admin\backups\system-setting-brand-20260916-091253\public-before.dump`，SHA256 为 `1C9293CDF4B3D59FB83CCE617EE08996C71F04F88B9E8EFA94E24FECCD31B31B`；`docs/database/current.sql` 已从真实数据库刷新。未执行 API/Worker 重启、Redis 清缓存和浏览器人工验收。品牌保存后 Redis 删除失败仍沿用系统设置原有“数据库已提交、缓存失效报错”的残余一致性风险。

## 系统任务队列监控（2026-09-14，代码已完成）

- 在系统设置下新增“任务队列”页面，页面权限为 `system:queueMonitor:view`，Grant API 权限为
  `system:queueMonitor:list`；页面通过同源 iframe 加载 Asynqmon v0.7.2，后端实际配置 `ReadOnly=true`。
- Grant 使用 32 字节随机值和 Redis SHA-256 key `system:queue-monitor:grant:v1:<hash>`，TTL 60 秒；只通过
  HttpOnly、SameSite=Lax、Path=`/api/admin/v1/system/queuemonitor/ui` 的 Cookie 传递，Redis 故障闭合，iframe URL
  不携带 Token。前端每 45 秒续期，隐藏、失活和卸载时停止，恢复可见时立即重新签发。
- 中文 UI 固定基于 Asynqmon v0.7.2 上游资源，保留 LICENSE 和源码；运行时 dist 由 Go embed 提供，不依赖 cwd，
  不进入 Vite 主 chunk。当前中文由项目内集中词典覆盖固定界面文案，不翻译队列名、任务类型、Payload 和业务数据。
- 新增 forward migration `docs/database/2026-09-14-system-queue-monitor.sql`：仅为活动 Admin 平台现有系统目录新增
  page/action，真实变化时只递增一次 `menu_version`，不修改 Canvas 或普通角色授权。隔离 PostgreSQL 重复执行测试通过；
  真实 `admin.public` migration 已于 2026-09-14 执行成功；Admin `menu_version` 已为 8，写入
  `system:queueMonitor:view` page 与 `system:queueMonitor:list` action，随后查询确认两条菜单均启用且层级正确。
  使用 `psql` 执行返回 `BEGIN / DO / COMMIT`，未执行真实回滚演练。
- 定向验证：queueMonitor、queue、cmd/api Go 测试通过；迁移隔离测试通过；queue monitor API/页面 Vitest 4 项通过；
  `pnpm typecheck`、`pnpm check:architecture`、定向 `go vet` 和 `go build ./cmd/api` 通过。前端全量 Vitest 与 build 按
  维护者要求未执行；浏览器、真实 Redis 和队列数据验收待 API 重启后进行。
- 上游依赖 `yarn install --frozen-lockfile` 在 Node `v16.15.1` / Yarn `1.22.19` 下成功，随后 `yarn build`
  返回 `Compiled successfully`；Node 24 下的旧版 CRA/PostCSS `ERR_PACKAGE_PATH_NOT_EXPORTED` 不再是当前构建阻塞。
  本次只验证了源码构建，运行时继续使用已校验且已移除 source map 与 `sourceMappingURL` 的现有 v0.7.2 官方 build，未覆盖
  `ui/dist`。
- 页面验收发现开发环境 iframe 相对地址被 Vite `16300` 的 SPA fallback 返回为管理端首页；现已为 Vite 增加读取
  `VITE_API_BASE_URL` 的 `/api` 代理，保持 iframe 同源。菜单使用的 `lucide:list-checks` 已加入前端注册表，真实 Admin
  菜单同步恢复该图标，`menu_version` 已递增至 10。Vite 已重启，代理路径实测返回后端响应而非管理端 HTML。

## 管理页壳与局部对话框组件化（2026-09-12，已完成）

- 新增全局页面壳 `web/src/components/AppPage`，统一管理页根节点为
  `section.app-page.management-page`，并迁移用户、存储、权限、系统、消息等管理页；页面继续通过
  `class` 传递各自模块样式，公共壳提供统一的 `min-width: 0` 等基础布局约束。
- 系统设置页已采用 `AppPage`、`AppSearch`、`AppTable` 与局部 `SettingDialog`，修正查询事件契约、工具栏新增按钮、
  响应式宽度和表单 placeholder/maxlength。
- 系统字典页进一步抽离 `DictionaryFormDialog`、`DictionaryDetailDialog`、`DictionaryItemDialog`，主页面仅负责编排
  列表查询、权限和提交回调；字典项列定义、状态/操作按钮与表单细节分别收敛到局部组件，保留原有 data-testid 与权限行为。
- 验证：字典页定向 Vitest 6/6，AppPage 测试随管理页回归通过；前端 `pnpm typecheck`、`pnpm lint`、
  `pnpm check:architecture`、`pnpm build` 均通过（build 仅有既有大 chunk 警告）。本轮管理页回归分两组共 12 个文件、
  138 项通过；未执行真实 API/数据库/Redis 操作。

## 系统设置与认证滑块 Captcha（2026-09-12，已迁移）

- 新增 `system/setting` 管理切片：设置键、值类型（string/number/bool/JSON）、启停、软删除、内置保护、严格
  请求字段和分页筛选；运行时读取通过 `shared/setting.Reader`，Redis key 为
  `system:setting:v1:<setting-key>`，变更会主动失效缓存；缓存快照使用严格 lower-camel JSON 解码，拒绝未知字段和
  trailing data。
- forward migration `docs/database/2026-09-12-system-setting-captcha.sql` 已在真实 `admin.public` 执行：创建
  `system_setting`、内置 `auth.captcha.ttl_minutes`/`auth.captcha.slide_padding`，并按已有
  `navigation.system` 目录幂等写入系统设置 page/action 与菜单版本。执行时间为
  **2026-09-12 13:37:24 +08:00**；仅 Admin 平台存在系统目录并写入 1 个 page + 6 个 action，Canvas 跳过。
- 新增 `auth/captcha` 滑块服务：go-captcha 生成图片，Redis `captcha:slide:` 保存短期答案，验证使用 `GETDEL` 单次消费；
  Captcha 配置读取失败、Redis/生成失败显式返回依赖错误，错误答案返回请求错误。公共端点为
  `GET /api/v1/auth/captcha`。
- 系统设置页面已按现有管理页范式收敛：使用 `management-page`、`AppSearch` 的 `query/reset` 契约、`AppTable`
  工具栏与 `row-key`、标准空态，以及 `AppDialog` 响应式宽度和 footer 操作；表单补齐长度限制、占位提示和
  `el-select-v2` 全宽样式。
- 登录验证码和找回密码发码前端已接入滑块弹窗；密码登录不要求滑块。系统设置页、Captcha 弹窗及登录/找回密码回归测试已补齐，
  中英文 i18n 已同步。
- 本轮真实库验证：两个 seed 均为 `value_type=2`、`is_enabled=1`、`is_builtin=1`，活动内置 seed 恰好 2 行；
  `system_setting` 的 CHECK、主键、活动唯一索引和启用查询索引均存在。Admin `menu_version` 6→7，Canvas 保持 1；
  立即重复执行 exit 0 且版本保持 7/1，确认幂等。隔离 schema 迁移测试、后端/前端定向测试此前均已通过。
- 迁移前备份位于
  `%LOCALAPPDATA%\\Admin\\backups\\system-setting-captcha-20260912-133629\\public-before.dump`，大小 159172 字节，
  SHA256 `3810AD2DCC66C961747124278BE436E35258045A26E515921F10C38A01A042C8`，`pg_restore --list` 得到 289 个归档条目。
  `docs/database/current.sql` 已于本次迁移后从真实 `public` schema-only 刷新（85123 字节）。
- 尚未执行：真实短信/邮件发送、腾讯云凭据录入、API/Worker 重启、浏览器验收及受影响 Redis runtime/readiness/menu
  定向重建；未清理 Redis 全库、Session、Access 或 Mail 额度。
- 下一步：由维护者在受控窗口重启新 API/Worker 后验收系统设置菜单、滑块图片和发码流程；定时任务、WebSocket、导出等基础能力
  另行排期，不在本切片内混入。

## 短信（SMS）垂直切片（2026-09-10，执行中）

- 目标：交付 `message/sms` 垂直切片并开放手机验证码登录、找回密码、绑定/换绑手机号与验证码改密；场景固定为
  `login`/`forget`/`bind_phone`/`change_password`，SMS 与 Mail 不共享表、场景常量、Redis namespace、Provider
  或加密/HMAC 密钥。
- 当前进度：Task 1–7 的 SMS 基础设施、数据库隔离迁移、Provider、配置/模板/规则/限流、runtime/readiness、日志、
  Auth 手机登录与忘记密码均已落地并通过对应定向测试。Task 8 的 Email/Phone 对等身份改造也已落地：新增
  `shared/email`、`user/email`，并让 `user/email` 与 `user/phone` 共享双 proof、`ConsumeMany`、事务审计、唯一冲突、
  authority generation 失效和严格 Handler 边界；普通 account/profile PUT 保留身份只读字段且拒绝身份写入。
  Task 9–12 的 RBAC/操作日志/架构门禁、严格前端 DTO、SMS 五 tab 管理页、登录/忘记密码/绑定/验证码改密流程已完成。
- 最近定向证据：用户身份、Auth、Mail/SMS、RBAC、操作日志、架构和 `cmd/api` 定向 Go 测试通过；用户/认证高风险
  `go test -race -p 1 ./internal/module/user/... ./internal/module/auth/login/... -count=1` 通过；前端 Email/Phone/Auth/
  Profile/Account/SMS 关键流程 8 个测试文件共 91 项通过，拆分 Profile 后回归 4 个文件 46 项通过；`pnpm typecheck`、
  `pnpm lint`、`pnpm check:architecture` 与 `pnpm build` 均通过（build 仅有既有大 chunk 警告）。
- Task 13 自动验证已由根 Agent 补跑：后端 `go test ./... -count=1`、`go vet ./...`、`go build ./...`、架构测试、
  用户/认证高风险 `go test -race -p 1 ./internal/module/user/... ./internal/module/auth/login/... -count=1` 全部通过；
  前端全量验证此前已由维护者确认通过。
- 验证证据：`go test ./internal/database -run TestMessageSMSMigration -count=1` 六个用例通过（幂等重复执行、非法旧
  phone 整体回滚、规范化碰撞整体回滚、双国家码整体回滚、软删平台日志仍可查询、七表约束/索引行为覆盖）；
  `go test ./internal/architecture -count=1`、`go vet ./internal/database/...`、`gofmt -l internal/database` 均通过。
- 复审修正（Task 2 第 2 轮）：迁移为活动 `registered_user` 幂等直授 `user:phone:update`（先就地新增 action 再授权，
  授权变化与菜单变化合并为每平台一次 `menu_version` 递增）；SQL 手机号规范化改为只剥离一个可选 `+86/86`，与
  `shared/phone` 语义一致；迁移测试补齐七表 CHECK/FK/唯一索引与查询索引覆盖，并修掉夹具中因缺少 finisher 而
  从未真正执行的授权插入语句。
- 根 Agent 最终复审：迁移拒绝用例现在解析 `pgconn.PgError`，逐项精确断言 SQLSTATE 与约束名；模板和限流策略
  改用更新 seed 行以排除唯一键干扰，verification 按“跨平台复合 FK → 正常写入 → 单日志唯一索引”顺序验证。
  强化断言后旧顺序先以 `23505/ux_message_sms_log_verification_log` 按预期失败，调整顺序后六个迁移测试全部通过。
- 真实 `admin.public` migration 已于 2026-09-11 23:03:33 +08:00 执行成功，并立即重复执行确认幂等；
  八张目标表（含 `user_email_change_log`）已创建，手机号非空 1 条、E.164 非法 0 条、规范化碰撞组 0 条；
  四个 SMS 模板、每个活动平台两条策略、SMS page/12 actions、`user:email:update`/`user:phone:update` 与
  `message.sms.region` 已核验。Admin `menu_version` 5→6，Canvas 保持 1；重复执行版本不再增加。
- 迁移前 `public` 备份位于
  `%LOCALAPPDATA%\Admin\backups\message-sms-20260911-230034\public-before.dump`，大小 124882 字节，
  SHA256 `7CC89B78BE70C2A4ED4EAC5A4DB524DE1DFCCEE05786419E62AFFE94F6963449`，`pg_restore --list` 得到 229 个归档条目。
  `docs/database/current.sql` 已从真实 `public` schema-only 刷新。
- 尚未执行：真实短信发送、腾讯云凭据录入、API/Worker 重启、浏览器验收及受影响 Redis runtime/readiness/menu
  定向重建；未清理 Redis 全库、Session、Access 或 Mail 额度。上述运行态动作需分别受控执行，不能由自动测试替代。
- 下一步/阻塞：先按新二进制重启 API/Worker（需维护者确认窗口）并做 `/health`、`/ready`、权限和页面验收；真实短信
  发送仅在维护者录入凭据和受控号码后进行。

## Mail 审计日志不可删除整改（2026-09-09，当前）

- 按维护者确认的审计契约，`message_mail_log` 与 `message_mail_log_verification` 现在是 append-only：
  后端 Model/Repository/Service/Route、前端 API/按钮/选择列和中英文删除文案均已移除；日志详情仍保留
  管理员解密验证码能力。`message:mail:log:delete` 仅作为历史菜单记录软删，不再进入有效权限。
- 认证平台软删不影响 Mail/操作/登录日志的历史来源显示；登录日志使用 `LEFT JOIN`，Mail 日志的来源平台
  同样不追加 `deleted_at IS NULL`。不存在的关联平台以空编码返回，已有软删平台仍保留原 code。
- 新增 forward migration `docs/database/2026-09-09-mail-log-immutable.sql`，执行前拒绝任一日志表存在
  `deleted_at IS NOT NULL` 的数据；移除两张日志表的 `deleted_at`，重建 challenge/verification 唯一索引，
  增加平台、邮箱前缀、场景、状态和时间分页索引，并按受影响平台递增一次 `menu_version`。真实
  `admin.public` 已于 2026-09-09 22:12:13 +08:00 执行成功：日志 9/验证码 9 保持，Admin 版本 3→4、
  Canvas 仍为 1；备份在 `%LOCALAPPDATA%\\Admin\\backups\\mail-log-immutable-20260909-221155\\public-before.dump`，
  SHA256 `42FBFC71DAA843F1F04615B667CF8F84C4EB559441DF4D458FB6ACB7394A76EB`，`pg_restore --list` 通过；
  `docs/database/current.sql` 已从真实 public schema 刷新。随后真实库再次执行同一 migration，exit 0 且
  `menu_version` 保持 4/1，确认幂等。
- Mail 日志查询补充显式 `ESCAPE '\\'`，并有真实 PostgreSQL 通配符回归测试。以事务内 100,000 行临时夹具执行
  `EXPLAIN (ANALYZE, BUFFERS)`：count 使用 `ix_message_mail_log_to_email_prefix` Bitmap Index Scan，
  list 使用主键倒序 Index Scan + LIMIT；小表直接 Seq Scan 属 PostgreSQL 成本选择，不判为索引失效，事务已回滚。
- 日志页与模板目录解耦：模板目录失败不阻断日志，空目录只尝试一次；日志有独立 loading/error 和请求序号，
  旧响应不能覆盖新筛选。`AppSearch` 仅将 date-range 的 null/undefined 映射为空数组，其他畸形值进入显式
  invalid model 错误。
- 本轮新增 Handler 输入边界、迁移幂等/回滚、登录历史平台和页面竞态测试。完整后端
  `go fmt ./...`、`go vet ./...`、`go test ./...`、`go build ./...` 全部通过；完整前端 65 个测试文件/496 项
  Vitest、`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`（0 findings）和 `pnpm build` 全部通过。
  Mail 全部子包 `go test -race ./internal/module/message/mail/...` 也通过。Build 仅报告既有的大 chunk 警告。
  当前 API 进程未由 Agent 重启，维护者需重启到新二进制后再做人工页面/API 验收；SMS 与 HTML 仍未开始。
- 当前有效 Mail 菜单事实为 13 条（含页面/动作；历史 `message:mail:log:delete` 记录已软删），操作日志规则为
  11 条 Mail 变更/详情规则；`account.password.change` 旧兼容翻译键已按未上线契约移除，仅保留
  `user.password.update`。

## 操作日志密码 action 归属修正（2026-09-09）

- 维护者确认：改密操作的 operationLog action 应归属 user 域（路由 `/api/admin/v1/user/password`、
  权限码 `user:password:update`）。`operationLog/rules.go` 中 profile/password 两条的 `module="account"`
  是 09-08 统一 user 域时漏改的历史遗留。已修正：password 规则改为 `module=user, action=user.password.update`
  （动词与权限码 `:update` 对齐），profile 规则 module 同步改为 `user`。
- 前端 i18n 新增 `operationLog.actions.user.password.update`，**保留** `account.password.change` 旧键——
  数据库已存审计行的 action 仍是旧值，保留键使历史日志继续显示中文；后端测试与前端测试同步断言
  新旧两值。
- 验证：后端 operationLog 包测试全过（含规则匹配断言）、`gofmt`/`vet`/`build` 通过；前端
  operationLog + mail Vitest 19/19、`typecheck`、`lint` 通过。**需重启 API** 后新写入的操作日志才使用
  新 action/module。

## 操作日志翻译补齐与发送日志中文化（2026-09-09）

- 操作日志 action 翻译补齐：后端 `operationLog/rules.go` 共 44 条规则，前端 i18n 缺失 mail 全部
  12 个 action 键，且 `account.password.change`（后端）与 `user.password.change`（旧 i18n 键）不匹配
  ——页面 `actionLabel` 机制本就支持（`te` 回退原始码），只是键不全导致英文直出。已补齐 mail 12 键、
  修正密码键名；测试补 `编辑邮件配置`/`修改密码` 断言。
- 发送日志筛选场景下拉改为接口数据：`loadLogs` 首次进入日志页签时加载模板目录
  （`message/mail/template`），场景下拉与表格/详情场景列均显示模板 `name`（如“登录验证码”）而非
  前端写死的 scene 枚举；删除 4 个前端场景死键。
- 发送日志状态中文化：表格状态 tag 与详情状态统一显示 `已发送/发送中/发送失败`
  （未知状态回退原始码），与筛选下拉一致。
- 验证：mail + operationLog + AppSearch 定向 Vitest 29/29；`typecheck`、`lint`、
  `check:architecture`（0 findings）通过。

## AppSearch 时间范围清空修复与宽度调整（2026-09-09）

- 修复公共组件 `AppSearch` 的 date-range 清空缺陷：`el-date-picker` clearable 点击后 emit `null`，
  `setSearchValue` 的 `normalizeValue` 将 null 判为无效直接 return，旧时间范围永远留在表单里且
  `model-value` 回弹——用户无法清空。现在 date-range 分支把 null/undefined 映射为空数组并 emit。
  该缺陷影响所有使用 date-range 的页面（session/loginLog/operationLog/mail log），修复后清空立即生效。
- 邮件发送日志时间范围筛选宽度 340 → 420（datetimerange 两个日期时间输入显示不全）。
- 新增 `web/tests/components/AppSearch/index.test.ts`（失败测试先行：清空事件映射为 `[]`）。
- 验证：AppSearch + mail 定向 Vitest 3 文件 25/25；AppSearch 消费方回归（session/loginLog/
  operationLog/account）3 文件 22/22；`typecheck`、`lint`、`check:architecture` 通过。

## 邮件发送日志筛选、关联用户与表单提示（2026-09-09）

- 发送日志新增筛选：后端 `message/mail/log` 支持 `platform`（平台编码前缀）、`toEmail`（收件邮箱前缀）、
  `scene`、`status` 精确筛选与 `from`/`to`（created_at，RFC3339Nano）时间范围；Handler 采用 loginLog 同款
  白名单 query 解析（重复参数拒绝、长度上限、from<=to 校验）。LIKE 统一转义并加尾部通配。
- 日志响应新增 `username`（LEFT JOIN `user_account`，软删用户与无关联显示 `-`）：业务邮件为收件用户，
  管理测试为发起测试的操作员——前端表格新增"关联用户"列并在详情展示。
- 前端日志页签接入 `AppSearch`（平台/邮箱/场景/状态/时间范围，场景与状态为 el-select-v2）；筛选值由
  子组件规范化后 emit 给聚合页，聚合页持有筛选状态并在查询时重置页码；翻页保留筛选。
- 模板与收件规则表单补齐 placeholder（名称/主题/变量 JSON/规则值/备注，规则值按 email/domain 动态
  提示）；vue-i18n 消息中的 `@` 与 `{}` 需按字面量转义或避开（本次踩坑：`user@example.com` 触发
  Invalid linked format）。
- 验证：mail 全部子包 `go test -count=1` 通过、log 包 `-race` 通过、`gofmt`/`go vet`/`go build` 通过；
  前端 `typecheck`、`lint`、`check:architecture`（0 findings）、mail 定向 Vitest 2 文件 24/24 通过。
  未跑全量测试与浏览器验收；旧 API 二进制需重启后筛选与 `username` 字段才生效。

## 邮件发送日志全平台化与详情对话框（2026-09-09）

- 管理端邮件发送日志从“当前认证平台”视角改为全平台控制面（与限流管理定位一致）：`message/mail/log`
  的 List/Get/Delete/DeleteMany 不再按 `identity.PlatformID` 过滤，可查看/删除所有平台日志；
  发送热路径（CreatePending/MarkSent/MarkFailed/FindActiveChallenge）的平台作用域不变。
- 日志响应新增 `platform`（`permission_auth_platform.code`，LEFT JOIN + COALESCE 空串，参照
  operationLog 既有模式）；平台被软删时日志仍可见，platform 显示 `-`。无数据库结构变化。
- 前端 `MailLog` DTO 增加 required `platform` 严格解析；日志表格新增平台列；详情从 el-drawer
  迁移为 `AppDialog`（复用公共组件契约，宽度 520px），详情内补充平台项；i18n 新增
  `mail.platform`（中/英）。
- 验证：`go test ./internal/module/message/mail/... -count=1` 全部子包通过（含 log 包
  `-race`）；`gofmt`/`go vet`/`go build` 通过；前端 `pnpm typecheck`、`pnpm lint`、
  `check:architecture`（0 findings）、mail 定向 Vitest 2 文件 23/23 通过。未跑全量测试与
  浏览器验收；运行中的旧 API 需重启后日志接口才返回新 `platform` 字段（旧二进制无此字段时
  严格解析会报协议错误，属预期切换行为）。

## 当前收口执行记录（2026-09-09）

- Mail 限流协议已移除 `LimitRequest.LegacyKeys` 及旧场景额度迁移；Redis Lua 只处理平台 + HMAC
  规范化邮箱的每分钟/每 10 分钟两条窗口，缺少 Mail Recipient HMAC 密钥时显式返回依赖错误，禁止明文邮箱进入 Redis。
- 认证平台生命周期已接入 Mail 策略 Repository：创建平台在同一 PostgreSQL 事务中幂等写入
  `business_email_minute=1/60s` 与 `business_email_10m=5/600s`，删除平台在同一事务中软删平台并清理策略；
  失败会整体回滚。依赖在 `cmd/api/main.go` 显式装配，Service 未接触 GORM。
- 操作日志限流更新规则已对齐实际 `/rate-limit-policy/:platformId/:key` 路由，并脱敏映射平台参数；Runtime
  Snapshot shared rebuild 使用 `context.WithoutCancel` + 15 秒有界超时；配置、模板、收件规则写入改为
  Redis mutation lease（先进入 invalidating、写入成功后 CAS 释放），失效失败不再提交数据库，旧快照不可达。
  缺 Mail 配置时 TTL 返回 `0`，前端不再显示伪默认 10 分钟。
- 历史 RBAC migration 断言已恢复验证当时生成的 `account:user:loginlog:view`；`docs/database/current.sql` 已从真实
  PostgreSQL 重新 schema-only 导出且仅包含 `public` schema，不含测试 schema。
- 本次未执行真实 DDL/Redis 清理、未启动 API/Worker、未提交代码。剩余风险：未运行 `go test -race`（机器无 C 编译器）。

## 当前 Mail 平台限流/运行时收尾（2026-09-09）

- 修复限流策略管理响应 DTO：`platformId` 只保留在 catalog envelope，不再重复写入每个 policy。旧运行中的
  API 会返回重复字段，前端严格协议解析因此显示 `mail rate limit policy has invalid fields`；已修改
  `rateLimitPolicy/handler.go` 使用专用 `PolicyResponse`，重新编译后需重启 API 才生效。
- 修正管理端平台范围：Admin 控制台是全平台控制面，不能只读取当前认证平台。限流管理 GET 现在返回
  `platforms[]`，每个平台包含平台名称、编码、版本及两条策略；PUT 使用
  `/rate-limit-policy/:platformId/:key` 明确修改目标平台。前端展示四行（Admin 两行、Canvas 两行），并以
  `platformId + policyKey` 作为行键。
- 修复 Mail 集成测试夹具：测试密钥辅助函数不再递归；验证码日志表和平台化限流策略夹具均已补齐。
  `go test -p 1 ./internal/module/message/mail/... -count=1` 与 Mail 根包 `-race` 已通过。
- Mail Runtime Snapshot 改为专用内部 DTO，显式携带加密凭据密文；发送热路径命中 Redis 时不再因
  `json:"-"` 丢失密文而回源/失败。新增 generation + Redis 租约，配置、模板、收件规则变更以原子
  generation 失效，避免旧快照在并发重建中回写；Redis 故障、快照损坏和源数据不完整均显式失败。
- **真实数据库迁移已执行**：停止状态下于 **2026-09-09 11:11:12 +08:00** 执行
  `docs/database/2026-09-09-mail-rate-policy-platform.sql`。`admin.public` 已从旧全局两行转换为四行：
  `admin`(platform_id=1) 两行 + `canvas`(platform_id=2) 两行，额度仍为 1/60s 与 5/600s；重复执行由
  隔离测试验证不新增，异常整体回滚。迁移前备份位于
  `%LOCALAPPDATA%\Admin\backups\mail-rate-policy-platform-20260909-111112\public-before.dump`，
  SHA256 `29FCFEDF8B7F8654F159A4728935282502DDFF3AD5E72A994A940CEA78D62E0E`，`pg_restore --list` 通过。
  `docs/database/current.sql` 已按真实 schema 刷新。
- 当前没有 API/Worker 或测试进程由 Agent 运行；未清理 Redis 额度、Session 或 Access。维护者需手动启动
  新 API/Worker 后再做人工验收。

## 模块 lower camel 与 Mail 分层整改（2026-09-09，最新）

- 后端复合模块目录/import path 已统一 lower camel：`authPlatform`、`permissionVersion`、`roleMenu`、
  `userRole`、`loginLog`、`operationLog`、`cosConfig`、`uploadRule`、`cacheFill`。Go package 标识符继续遵循
  全小写惯例；PostgreSQL 表/列/约束/索引继续 snake_case，HTTP API 路径继续使用小写资源段。
- Mail 六张表已拆入 `config`、`template`、`log`、`logVerification`、`rateLimitPolicy`、`recipientRule`，
  各自拥有 Model/Repository，公开管理资源拥有 Handler/Service/Route；`logVerification` 仅供日志详情使用。
  根 Mail 已删除万能 Model/Repository/Schema 和管理 CRUD，只保留发送编排、Provider、Limiter、Readiness、
  管理测试、Stores 显式组合及路由聚合。Mail 密文 envelope 下沉到 `internal/secretkey` 供配置和验证码共用。
- 前端业务目录、API/工具 TypeScript 模块已统一 lower camel；Mail 页签模块不再放在泛化 `components` 下。
  Vue 组件目录保持 PascalCase，CSS class/data-testid 保持 kebab-case。`AppAside` scoped 样式原样移至
  `AppAside.css`，SFC 从 406 行降至 159 行，前端架构门禁恢复 0 findings。
- 菜单/权限/i18n 模块段已同步为 `authPlatform`、`operationLog`、`loginLog`、`cosConfig`、`uploadRule`；
  Access Redis snapshot namespace 升至 v8，避免读取旧标识的 v7 快照。forward migration
  `2026-09-09-module-lower-camel.sql` 已通过隔离 PostgreSQL 幂等、冲突回滚、菜单 ID/角色授权保持及平台
  `menu_version` 单次递增测试。
- **真实数据库已迁移**：维护者停止旧 Worker 后，迁移于 **2026-09-09 07:56:54 +08:00** 在本机
  `admin.public` 提交。19 条权限码与 3 组 `path/componentPath/i18nKey` 已原位转换，旧标识均为 0；菜单仍为
  71 条，角色菜单关系仍为 17 条，菜单 ID 与授权摘要保持。Admin `menu_version` 从 2 增至 3，Canvas 仍为
  1，两平台 `policy_version` 仍为 3/1；未修改表名、用户 access version、Session 或 Mail 发送额度。
- 迁移前备份与执行记录位于仓库外
  `%LOCALAPPDATA%\Admin\backups\20260909-075553-module-lower-camel`；`public-before.dump` 已通过
  `pg_restore --list`，SHA256 为 `E5341CD7C603E47F318F6267EADAA2304653B4808CA5BDF845D4212CB7693664`。
  Admin Redis menu state 已从 `ready/version=2` 精确重建为 `ready/version=3`，只操作
  `authz:menu-state:v1:1`；Canvas、用户 Access、Session、Mail 额度及其他 Redis key 未清理。
- `docs/database/current.sql` 已从迁移后的真实 `public` schema-only 刷新。Agent 未启动 API/Worker；维护者可在
  全量检查后手动启动新代码，不得恢复旧二进制。
- **迁移后首次启动回归修复（2026-09-09）**：维护者启动新 API 后，`GET /api/v1/access` 返回 code `14000`
  （访问权限数据无效），而 API/PostgreSQL/Redis 健康检查均正常。根因是 Access/Menu/前端菜单字段校验器仍
  只接受全小写或 kebab-case，拒绝已迁移的 `permission/authPlatform`、`system/operationLog`、
  `user/loginLog`。现已统一为“段首小写，支持 lower camel；保留既有全小写 kebab action/path；拒绝大写首段、
  下划线和非法路径”的兼容规则，并补充后端菜单、Access 快照和前端表单测试。修复后 Permission Menu/Access
  完整定向包通过，前端相关 Vitest 75/75、lint、architecture、typecheck 通过；`go vet` 与 API 编译检查通过。
  当前运行中的 API 仍是修复前二进制，需维护者手动重启后再刷新页面验证。
- 已验证：`go build ./...`；Mail 全部子包；Permission/Storage/Auth/LoginLog/OperationLog/cacheFill/Worker
  定向测试；两份新 migration 的隔离测试；前端 typecheck、lint、architecture 及此前 17 文件 154 项 Vitest。
  最后一轮文件改名后已复跑完整 Mail 子包，以及前端 lint、architecture、typecheck 和 10 文件 57 项改名
  契约 Vitest，均通过；Git 索引无旧模块路径，`git diff --cached --check` 通过。三个 Admin Skill quick
  validator 通过。全量 Go/Vitest/build 尚未按维护者约定重复运行。

## Mail 全局 SES 归属修复（2026-09-08，最新）

- 维护者确认腾讯云 SES 是全系统唯一邮件通道，只有 COS 上传配置允许按认证平台分区。
  `message_mail_config`、`message_mail_template`、`message_mail_recipient_rule` 已改为全局配置/模板/规则；
  `message_mail_log`、`message_mail_log_verification` 与 Redis 发送额度继续保留来源平台，维持“同平台、同邮箱”隔离。
- Mail Repository/Service/Handler 和前端 DTO 已删除配置、模板、收件规则的认证平台参数与响应字段。
  Auth 的 Mail readiness 改为按场景全局缓存，namespace 升至 v3；旧平台级 v2 readiness 不回读。
  管理测试和平台日志仍要求认证上下文中的正数来源平台，缺失时明确 401，不再静默回退平台 1。
- forward migration `2026-09-08-mail-global-configuration.sql` 会锁定三张目标表，拒绝多活动配置、重复场景模板
  或重复收件规则，删除三列及外键后建立全局部分唯一索引；隔离 PostgreSQL 已验证重复执行、三类歧义拒绝和
  失败整体回滚。
- **实际数据库已迁移**：首次编写的迁移测试脚本错误使用 `public.` 限定名，导致测试连接越过临时 schema，
  于约 **2026-09-08 21:59 +08:00** 将本机 `admin.public` 提交到目标结构。发现后立即停止写操作并修正脚本为
  `current_schema()`/search-path 隔离；未执行回滚。只读核验确认原 1 条配置、4 条模板、1 条收件规则的 ID 和
  业务字段全部保留，9 条发送日志及其 `platform_id` 未变，未操作验证码、额度、Session 或其他表。
- 本次操作前没有创建即时备份。迁移后受限权限归档位于
  `%LOCALAPPDATA%\Admin\backups\20260908-220912-mail-global-postmigration`，归档 SHA256：
  `18A685303623E83997386419B30BCE1B16961A2D5289A958BEE62B86CB9C58DD`；`pg_restore --list` 读取通过。
  `docs/database/current.sql` 已从真实 public schema-only 刷新。
- 当前 API/Worker 未由 Agent 重启；检查时 API 未监听常用端口，迁移前 Worker PID `34520` 仍在运行并使用带
  `platform_id` 的旧 Mail SQL，维护者手动重启新代码前不得发送或消费邮件任务。新 readiness v3 将在新进程
  首次访问时按全局配置重建，不需要清理旧 v2，也不得清理发送额度键。
- 验证通过：`go build ./...`；Mail/Auth/Architecture 三包定向测试；Mail/Auth 完整 `-race -p 1`；迁移重复执行、
  歧义拒绝和回滚；前端 `pnpm typecheck`、Mail API/页面 23 项 Vitest、相关 ESLint。`pnpm check:architecture`
  被既有 `AppAside/index.vue` 406 行（阈值 400）拦截，该文件不属于本次 Mail 改动，未在本轮扩展 UI 重构。

## Admin 注册与权限语义修复（2026-09-08，最新）

- 修复 action 自动赋予 page/view 的错误语义：普通角色的直接 action 只进入 `permissionCodes`，不产生 page、
  动态路由或目录；只有直接 page 才进入 `menuTree` 并补目录祖先。Role Service 与前端授权矩阵均保存
  page/action 的显式独立组合。`super_admin` 的全量有效权限语义不变。
- `user:profile:view` 已从 `user` 目录移动为 `parent_id=NULL` 的隐藏根 page。个人中心可动态访问但不显示菜单，
  也不会再让仅有个人能力的用户看到“用户与账号”目录。
- Admin 允许邮箱验证码自注册；新邮箱验证成功后原子创建无密码账号、Profile、Access version，并绑定唯一
  默认 `registered_user`。公开密码直注册路由已移除，避免绕过邮箱验证。登录页显示普通用户自动注册说明，
  AuthPlatform 管理页允许编辑内置 Admin 的注册开关。
- `registered_user` 当前直接授权固定为 5 项：`user:profile:view/detail/update`、`user:password:update`、
  `storage:object:upload`。上传 action 不赋予 `storage:object:view`；额外管理能力必须分配其他角色。
- Access Redis snapshot namespace 升至 v7，旧 v6 action 扩展快照不回读。Admin policy/menu version 与
  当前 `registered_user` 用户的 Access state 已精确同步；未清 Redis、未重置会话或邮件额度。
- 数据库迁移 `2026-09-08-admin-email-registration.sql` 已于 **2026-09-08 15:40:59 +08:00** 提交。
  结果：Admin `allow_register=1`、`policy_version=3`、`menu_version=2`、默认角色 5 条直授权；
  平台/角色/菜单/用户关系 ID 保持，非目标记录未变。真实 schema-only 快照已刷新。
- 迁移前备份与带校验执行记录位于仓库外
  `%LOCALAPPDATA%\Admin\backups\20260908-153855-admin-registration-rbac`；备份 SHA256：
  `6B2837601B70C055C9EB9BECC83D76EDC9A1136DBF98151C23E82F84DB3C13A1`。
- 后端定向测试通过：Access、Role、AuthPlatform、Auth Handler、迁移及 Router，包含 action-only 不泄露页面、
  profile 隐藏根页面、v6 快照不回读、page/action 独立往返、注册开关、密码直注册 404 和迁移回滚。
  前端 `pnpm typecheck` 通过；Role Matrix、Role 页面、AuthPlatform 页面、登录页 5 个 Vitest 文件
  59/59 通过。未跑全量测试/构建，浏览器注册、上传与密码流程由维护者人工验收。
- Agent 在迁移后保持服务关闭；维护者随后于 15:44 从 IDEA 手动启动 API/Worker。当前观察到 API PID 35736、
  Worker PID 34520，未由 Agent 启动或停止。Admin/RBAC 临时执行清单已删除，不作为未来上下文。

## 当前交接：快速收尾（2026-09-08）

本节优先于下方阶段记录。按维护者最新要求冻结重构范围，停止浏览器 E2E，全量测试与构建交由维护者执行。

- 已完成代码：Mail 原子预占返回两窗口共同决定的下一次等待，`resendAfterSeconds=0` 合法；Auth 不另设 cooldown。
  首次无密码登录只给非阻断提示。菜单变更改为平台 `menu_version`，不再扫描/锁定用户表或逐用户递增版本。
  Access 本地/Redis 缓存同时受用户授权版本和平台菜单版本约束，Redis 快照 namespace 为 v7。
- 冷缓存：Auth Session / Access / Menu Version 使用跨实例单目标租约，每个 scope 最多 32 个重建并发、
  每滑动秒最多 128 次启动；源 I/O 的 4 秒期限从租约申请时起算，早于 6 秒 Redis 租约。忙时有界等待后
  返回依赖错误；已知 Redis 故障/损坏不转为业务源回源。此为代码预算，不等于已完成百万用户压测。
- 已验证：8 个相关 Go 包的关键定向测试带 `-race -p 1` 通过（并发、权限失效、Redis 故障、密码、迁移回滚）；
  最后时限传递修正后，Access/Menu 关键用例复跑通过。`go test ./cmd/api -run '^$' -count=1` 编译通过。
  `pnpm typecheck` 通过；Auth API、登录页、菜单页三个 Vitest 文件 50/50 通过。此次未跑全量测试或打包构建。
- **新增迁移已执行**：维护者明确授权后，`2026-09-08-menu-catalog-version.sql` 于
  **2026-09-08 14:33:43 +08:00** 在本机 `admin.public` 提交。Admin/Canvas 的 `menu_version` 均为 1，
  `policy_version` 仍为 2/1；事务提交前逐项核对平台原有字段、71 个菜单、11 条授权及用户授权版本均未改动。
  新字段为 `BIGINT NOT NULL DEFAULT 1`，正数 CHECK 已验证；`docs/database/current.sql` 已从真实 schema-only 刷新。
- 此次先备份并完成归档读取验证，文件、执行 SQL、校验输出和运行日志在仓库外
  `%LOCALAPPDATA%\Admin\backups\20260908-142940-menu-version`；目录 ACL 限当前用户/SYSTEM。
  `public-before.dump` SHA256：`DE217ABA77367E2D9863D9CA1A1E1BE998E180EAA9ED0A55EE3727D62AFC34D6`。
- 仅失效并重建两个平台的菜单版本元数据键，随后核对缓存值与 PostgreSQL 一致；未清空 Redis、未撤销会话、
  未操作邮件额度键。临时维护程序已移出工作区并归档。
- 为避免恢复旧二进制，仅定向编译 API/Worker 两个启动入口并从命令行后台恢复，未运行全量构建或测试。
  当前启动记录：API PID 32924、Worker PID 34024，2026-09-08 14:36 +08:00；日志在同一备份目录。
  切换回 IDE 启动前，应先检查并停止这两个后台进程，避免重复实例/端口冲突，不要盲用过期 PID。
- 迁移后 `/health`、`/ready`、`/api/v1/auth/policy` 均 200；管理接口带平台头但无登录凭据返回 401。
  执行前观察到一轮已有全量 Go 测试，没有终止或重复启动它；其结果以维护者终端为准。
- 浏览器 E2E 原型及失败记录已归档到仓库外 `%LOCALAPPDATA%\Admin\paused-e2e\20260908-135023`，工作区已移除。
  未将其计为通过，也未修改真实账号密码或发送真实邮件。密码页面交互、登录态浏览器验收交给维护者。
- 最后进程检查无 Go/Vitest/浏览器 E2E 测试进程运行；现有 API/Worker 不是本轮新代码上线验收证据。
  未调整暂存区或提交；存在 AD（已暂存但工作区已删除）旧路径/原型文件，提交前必须核对 `git diff --cached`。

### 维护者执行顺序（新增迁移已完成，迁移命令仅留档）

先停止旧 API/Worker，保持 PostgreSQL/Redis 可用。每一步退出码应为 0；失败先停止，不继续发布；不要同时启动另一轮全量测试。

```powershell
cd D:\admin\server
go vet ./...
go test -p 1 ./... -count=1
go build ./...
cd ..\web
pnpm lint
pnpm check:architecture
pnpm typecheck
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

以下备份/迁移命令留作审计和复核，本次已经执行，不需要再执行一次。读取本地配置时不打印连接串：

```powershell
cd D:\admin
$line = Get-Content server/.env | Where-Object { $_ -match '^POSTGRES_DSN=' } | Select-Object -First 1
if (-not $line) { throw 'POSTGRES_DSN missing' }
$dsn = (($line -split '=', 2)[1].Trim().Trim('"').Trim("'")) -replace '\s+TimeZone=\S+', ''
$backup = Join-Path $env:LOCALAPPDATA ('Admin\backups\menu-version-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Path $backup | Out-Null
pg_dump -d $dsn --format=custom --schema=public --file (Join-Path $backup 'public-before.dump')
if ($LASTEXITCODE -ne 0) { throw 'Backup failed' }
psql -X -d $dsn -v ON_ERROR_STOP=1 -f docs/database/2026-09-08-menu-catalog-version.sql
if ($LASTEXITCODE -ne 0) { throw 'Migration failed; do not start the new API' }
psql -X -d $dsn -v ON_ERROR_STOP=1 -c 'SELECT id, code, policy_version, menu_version FROM public.permission_auth_platform ORDER BY id;'
```

预期每个平台有正整数 `menu_version`（初次执行为 1），`policy_version` 不变。然后启动 IDE 的 Admin API/Worker
及前端；`/ready` 应为 200 且 PostgreSQL/Redis 均 up。最后重新导出 public schema-only 到 `docs/database/current.sql`。

### 人工验收清单（尚未验收）

1. 无密码测试账号邮箱登录：仅温和提示，不强制跳转；首次设置无需旧密码，成功后当前会话保留。
2. 找回密码：验证码过期时间和邮件展示遵守 Mail TTL；成功回登录页不自动登录，旧浏览器会话及刷新凭据失效；
   旧密码失败，新密码成功。不要使用真实重要账号做破坏性验证。
3. 同平台同邮箱的登录/找回密码/管理测试共享两条额度；有余额可立即重发，耗尽后按服务端返回值等待；
   不同场景的验证码不能互相使用。测试完恢复自行调整的邮件策略。
4. 普通菜单变更后，另一浏览器刷新菜单/权限得到新结果；无权限 API 仍拒绝。菜单修改不强制注销用户；
   “重建缓存”反馈的平台数量正确。跨平台目录不串用。
5. 全量测试、构建及上述浏览器验收通过后再结案；尚未执行真实百万用户压力测试，不把预算测试当容量承诺。

## 本轮交付状态（2026-09-08，优先于下方历史记录）

- 最新归属修正已落实到代码：前后端管理模块 `permission/authPlatform`，Model 映射
  `permission_auth_platform`，管理 URL `/api/admin/v1/permission/authPlatform`、权限码
  `permission:authPlatform:*`、导航键 `navigation.permissionAuthPlatform`、页面文案 `permission.authPlatform.*`。
  `auth/login`、公共 `/api/v1/auth/policy` 与现有平台策略 Redis key 不变；不把前端客户端平台常量
  `src/auth/platform.ts` 误当成管理模块。业务表已迁移为 `permission_auth_platform`。
- 当前 forward migration 已同步原位改表、约束/索引/序列改名、页面路径和权限码转换；隔离 PostgreSQL
  验证跨平台 ID、外键、序列绑定与原有授权不变、重复执行和后续冲突整体回滚。必须排空旧 API、Worker
  及维护脚本后，由维护者执行迁移，再启用新版本。此脚本包含前一轮邮件两策略转换，不单独跳过其中步骤。
- Agent/三个 Skill 统一为“先确认业务归属并校正后端，再统一各层”，不再以现有后端目录自动推断归属。
- 认证平台归属调整验证：`go fmt ./...`、`go vet ./...`、`go build ./...`、
  `go test -p 1 ./... -count=1` 全部通过；补充的原位改表后失败回滚测试用
  `go test ./internal/architecture -count=1` 通过。`go test -race -p 1 ./internal/module/permission/authPlatform ./internal/module/auth/login -count=1`
  两包完整通过。IDE build 无错误，三个 Skill validator 通过。
- 此次前端 `pnpm lint`、`pnpm check:architecture`、`pnpm build` 通过；全量 Vitest 为 470/471 通过，
  会话页第一项超过默认 5 秒（全量下 8272ms），并非断言失败。使用原超时独立复跑
  `pnpm vitest run tests/views/user/session/index.test.ts --pool=threads --maxWorkers=1 --reporter=verbose`，
  6/6 通过，原超时项 1230ms。未修改测试超时；未将本次全量记录标为全绿。
- 当前依据仅为维护者本次最终决策；下方旧日期记录中的 plan 链接、7 条邮件策略、固定 60 秒和旧路径
  均为历史执行记录，不是当前契约，不恢复已经删除的 plan/spec。
- 已实现：首次设密条件写入与并发单胜者、重置密码撤销会话、forget readiness 失效、refresh 设密标记；
  后端分层归位及 AST 门禁；Views/API/权限/i18n 按 user 等业务域和单数资源统一；三个 Skill 同步。
- 邮件实现仅两条平台+规范化邮箱共享策略，覆盖业务场景与管理测试；TTL/次数/窗口来自 Mail。
  旧场景额度按 GCRA 剩余债务原子归并，任一窗口拒绝不扣本次请求，损坏状态不产生部分写入。
- 业务数据库迁移已获维护者授权并于 **2026-09-08 12:36:07 +08:00** 提交，目标为本机 `admin.public`。
  执行 `docs/database/2026-09-08-module-naming-mail-policy.sql` 的原始事务体，并在同一事务提交前增加
  前后校验：2 个平台全字段与表 OID、11 个引用外键、序列 OID/当前值、71 个菜单非命名字段、
  11 条角色授权全部不变；两名用户 Access version 各递增一次。原 7 条邮件策略收敛为 2 条，保留
  `business_email_minute=1/60s`、`business_email_10m=5/600s`，未调整邮件 TTL 或发送测试邮件。
- 迁移前停止了项目 API/Worker，并确认没有项目数据库连接或活动事务；业务 schema 使用
  `pg_dump --format=custom --schema=public` 备份，`pg_restore --list` 与完整归档读取通过。
  备份及带校验的执行 SQL/结果在仓库外 `%LOCALAPPDATA%\Admin\backups\20260908-123409-module-naming`，
  目录 ACL 限当前用户与 SYSTEM；归档 SHA256 为 `DC05D64AB938E04DA21DF475DCF36DB6B76E13B0FEAB868E98E1253F6B573C9A`。
- 上线衔接已完成：使用现有 `RebuildReadyState` 将 2 个 Access 缓存状态同步到数据库新版本，
  仅失效并重建 `mail:rate-limit:policies:v2` 配置缓存，校验 catalog version=2 且仅含两条平台邮箱策略；
  未操作 `rate:mail:send:*` 额度键、未清空 Redis、未重置会话。临时维护程序已归档到备份目录并移出仓库。
- IDE `Admin API` / `Admin Worker` 已用新代码启动；HTTP `/health`、`/ready`、`/api/v1/auth/policy` 均 200，
  新管理路由带平台头但无登录凭据返回 401，旧 `/api/admin/v1/auth/platform` 返回 404。
  Admin 平台 10 个真实 page 的路径、Views 与中英文 i18n 全部核对通过；Canvas 的独立测试页保持原状，
  不拿它与 Admin 前端文件匹配。`docs/database/current.sql` 已从真实 public schema-only 导出，无业务数据。
- 上阶段记录的冷缓存回源和菜单全用户失效问题已在快速收尾阶段修改；最新验证范围见顶部。
  Redis error/corrupt 路径现已显式失败且零故障回源，但这不等于冷启动或百万用户写路径已验收。
- 重发等待已改为由 Mail 原子预占结果决定，0 表示仍有额度；页面流程待维护者验收。
  没有新增 Auth 时间配置，契约与定向测试结果见顶部。
- 上轮实际验证（认证平台归属调整前）：`go fmt ./...`、`go vet ./...`、`go build ./...`、第二轮
  `go test -p 1 ./... -count=1` 全部通过；随后新增的迁移冲突回滚测试独立通过。
  `pnpm vitest run --pool=threads --maxWorkers=1 --reporter=verbose` 65 文件/471 项全部通过；
  `pnpm lint`、`pnpm check:architecture`、`pnpm build`（含类型检查）通过，三个 Skill validator 通过。
  首次前端全量运行约九分钟无单文件结果后已停止，以上是重新运行的最终结果。
- `git diff --check` 通过；暂存区 `git diff --cached --check` 仍报 role-view.ts 末尾空行，工作区版本已修正，
  未修改其他会话的暂存状态。保留既有 i18n flatten 警告及构建大包提示；未执行浏览器业务端到端验收。
  上轮开发服务的 HTTP 200 只代表前端可达；本轮迁移后接口和数据库验收见上文。
  未做登录态浏览器端到端验收，也未 commit；不能把本次迁移完成等同于所有架构容量事项已结案。
- 当前环境已具备 GCC 且 `CGO_ENABLED=1`。本轮 `go test -race -p 1` 对 auth/login、user/account、
  message/mail、permission/access 使用 `Test.*(Concurrent|TwoInstances|TwoServices|FirstPassword|SetPassword|Fault|RedisFailure|Corrupt)`
  筛选的定向测试四包通过；未跑全仓 race。下方旧记录中的无编译器阻塞已过时。

## 当前总目标

| 项目 | 内容 |
| --- | --- |
| 目标 | `agent-context-refactor`：让 Agent 使用小而精、渐进式的项目上下文 |
| 状态 | 已完成 |
| 验收 | 硬规则、任务路由、状态、设计、架构和固定 Skill 均有唯一入口；历史 spec/plan/SQL 不进入默认读取 |
| 最近更新 | 2026-09-04 |

## 已完成基线

- 后端使用 `server/cmd/api`、`server/cmd/worker` 两个显式组合入口，业务模块位于
  `server/internal/module`，同步请求遵循线性分层。
- 当前后端模块包括认证、健康检查、权限（菜单/角色/Access）、用户、消息邮件、对象存储和操作日志；
  具体范围以代码和数据库事实为准。
- 前端生产代码位于 `web/src`，测试位于 `web/tests`；页面、API、Router、Store 和公共组件已有独立目录。
- HTTP envelope、严格 DTO、RBAC 页面/动作权限、PostgreSQL/Redis Access 版本协议和显式时间/删除字段规则已
  固化在 `AGENTS.md`、`design.md`、`architecture.md`。
- `$admin-crud`、`$admin-rbac`、`$admin-database` 已安装到
  `C:\Users\IGT\.codex\skills`，分别封装 CRUD、RBAC 和数据库施工流程。
- 旧方案和一次性 migration 保留在 `docs/superpowers/**`、`docs/database/2026-*.sql`，默认读取路径已移除。

## 本次整改记录

| 项目 | 结果 |
| --- | --- |
| 根硬规则 | `AGENTS.md` 收敛为不可违反的边界和验证要求 |
| 渐进路由 | `docs/agent/README.md` 提供启动顺序、任务矩阵和历史档案策略 |
| 当前设计 | `docs/agent/design.md` 提供产品、UI、协议和权限基线 |
| 当前架构 | `docs/agent/architecture.md` 提供组件、数据流和数据库事实边界 |
| 容量基线 | 共享请求路径按百万级用户、多实例高并发设计；spec/plan 必须记录热点查询、缓存一致性、故障行为和并发验证 |
| 固定流程 | 三个全局 Skill 通过 quick validator |
| RBAC Skill 架构 | `$admin-rbac` 按当前 PostgreSQL 权限事实、Redis `ready/invalidating` 状态、版本化快照、本地缓存确认、token lease 失效和动态路由链路重写；新增无界 PostgreSQL 故障回源、旧 publisher/token、跨平台和双实例容量检查 |
| 前端下拉组件 | 全部生产 `el-select`/`el-option` 迁移为 `el-select-v2 + options`；`AGENTS.md`、设计基线及 `check:architecture` 固化禁用规则，保留数值/字符串、多选、可创建与 header 插槽行为 |
| 邮件规则与限流 | 修复 `AppTable` 动态插槽导致收件规则状态请求携带 `undefined`；收件规则明确为默认允许、邮箱优先、拒绝优先；Redis 限流异常返回 `503/10006`，真实额度超限返回 `429/10007` |
| 邮件日期展示 | 邮件日志接口将 `sentAt`、`createdAt`、`updatedAt`、`verificationExpiresAt` 显式输出为 UTC RFC3339Nano，空值为 `null`；页面使用供应商实际发送时间 `sentAt` 和验证码过期时间，并按应用语言和本地时区格式化，空值或非法值显示 `-` |
| 邮件错误反馈 | 收件规则拒绝使用 `403/18000`；终态 `401/403` 由请求层通知一次，登录凭据 `10002` 由登录页内联展示且不重复通知 |
| 后端契约与全量验证 | 邮件日志列表复用 `shared/pagination.Result`；权限菜单测试夹具统一遵循 `path = "/" + componentPath`，完整后端测试、静态检查和构建通过 |
| 邮件限流策略管理 | 核心实现和前端页签已落地；已补迁移跨平台 action 隔离、RBAC Access 授权/版本失效/隐藏 action 集成 fixture、Redis 回源版本保护、失败草稿恢复和严格策略 DTO 校验。2026-09-05 已在本地 `admin` PostgreSQL 执行限流策略迁移并校验 7 条策略与隐藏 action；真实 Redis 双实例并发/故障探针仍未完成 |

## 邮箱验证码 TTL 与重发契约收口（2026-09-07）

> 来源计划：`docs/agent/plans/2026-09-07-mail-verification-contract-closeout.md`。已按失败测试 → 最小实现落地并全量验证，未提交。

**实际接口与 Redis namespace**

- Mail 窄接口改为三段式：`VerifyCodeReady(...) (VerifyCodeReadiness{Ready, TTLMinutes}, error)`、`PrepareEmailVerifyCode(...)`（readiness + 收件规则 + 业务限流各一次）、`SendPreparedEmailVerifyCode(...)`（不重复计数）。
- readiness 快照 schema/key/load-lock 升 v2：`mail:verify-code-readiness:v2:<platform>:<scene>`，ready payload 携带 `ttlMinutes`；v1 不解析、不回退。
- Auth 验证码 key 升 v2：`auth:verify-code:v2:`；Put Lua 只以当前 delivery lease 为写权限，允许原子覆盖旧码；TTL 允许 1–60 分钟。
- 限流 key 未换 namespace，发布期间额度连续。

**5 分钟 TTL 与 60 秒重发验证证据**

- `message_mail_config.ttl_minutes=5` 是唯一 TTL 来源：`loadFromRepository` 读取 `config.TTLMinutes`，`TestPrepareEmailVerifyCodeReturnsConfigTTLAndResendWindow` 断言 TTL=5 且 `business_email_minute.window_seconds=60`。
- Auth 已删除 `verificationCodeTTL = 10 * time.Minute` 常量；`SendCode` 用一次捕获的 `now + preparation.TTLMinutes` 计算 `expiresAt`，同一值写入 Redis/模板/verification log/响应。
- 前端 `SendCodeResult` 增加严格整数 `resendAfterSeconds`（1–86400），登录页倒计时直接取该值，不再由 `expiresAt - Date.now()` 推算。

**双实例查询预算与 winner 数**

- 64 路 v2 missing 只回源一组 PG（config:1/template:1）；100 次 ready 热读零 PG 查询（`TestVerifyCodeReadinessTwoInstancesRecoverMissingSnapshotOnce`）。
- 修复双实例协调竞态：拿到 load-lock 后、回源前复查快照，闭合落后实例的冗余回源。
- 双 Auth Service 同邮箱并发 SendCode 仅一个进入 provider（`TestSendCodeTwoServicesSingleProviderWinner`）。

**各故障点公开错误码**

- 额度超限 `429/10007`；Redis 读取故障 `503/10006`；readiness/依赖不可用 `503/10006`。
- Prepare 失败只释放 lease、不删除旧码；code Put 后发送失败删除本次 code 并释放 lease；任一 owned cleanup 失败提升为 `503/10006`，不返回假成功。

**执行者基线曾运行的命令（终审轮未复跑，最终以维护者执行为准）**

- 后端：`go fmt ./...`、`go vet ./...`、`go test ./...`（36 包全过）、`go build ./...`。
- 前端：`pnpm vitest run --pool=threads --maxWorkers=1`（63 文件 451 用例全过）、`pnpm lint`、`pnpm check:architecture`、`pnpm typecheck`、`pnpm build`。
- 2026-09-07 终审轮：以上全量命令均未运行，由维护者执行最后一轮全量验证。

**终审兜底（2026-09-07，独立审查者）**

| 严重度 | Finding | 修复 | RED/GREEN |
| --- | --- | --- | --- |
| Critical | Auth 验证码 Redis value 解析不严格：`Check` 用普通 `json.Unmarshal` 只比较 digest，接受缺 `leaseToken`、未知字段（含 PII 走私）与 trailing data；`Consume`/`CheckAttempt`/`DeleteIfOwned` Lua 只比较 `decoded.digest`，corrupt payload 可能被当作有效码消费或删除 | `auth/login/redis.go`：新增 `decodeVerificationCodeValue`（token 级重复 key 扫描 + `DisallowUnknownFields` + EOF trailing 检查 + digest/leaseToken 非空校验）用于 `Check`；三个 Lua 脚本统一 `pcall(cjson.decode)` + 字段数恰为 2 + digest/leaseToken 非空字符串校验，异常返回 `corrupt` 并在 Go 侧 fail-closed | `TestVerificationCodeRejectsCorruptValueFields` 先 RED（`Check accepted corrupt value "{"digest":"digest-a"}"`），修复后 GREEN |
| Important | `VerifyCodeReady` 在读取 readiness 前先 `loadRateLimitCatalog`，登录配置热路径因无关限流目录回源 Redis/PostgreSQL，且 catalog 故障会误杀 ready 结果（旧契约残留） | `message/mail/service.go` 删除该调用，ready 热读只读 readiness v2 快照；限流目录归属 Prepare | 新增 `TestVerifyCodeReadyHotReadDoesNotLoadRateLimitCatalog` 先 RED（catalog 错误导致 `ready={false 0}` 依赖错误），修复后 GREEN；`TestVerifyCodeReadyFailsWhenRateLimitPolicyIsUnavailable` 更名为 `...ReadinessStoreIsUnavailable` 修正语义 |

**终审轮实际运行的定向命令（全部通过）**

- `go test ./internal/module/auth/login -run 'TestVerificationCode|TestSendCode|TestHandlerSendCode' -count=1`（25 项）
- `go test ./internal/module/message/mail -run 'Test.*VerifyCodeReadiness|TestVerifyCodeReady|TestPrepareEmailVerifyCode|TestSendPreparedEmailVerifyCode|Test.*TwoInstances|Test.*Concurrent|Test.*Fault' -count=1`（22 项）
- `go test ./internal/module/auth/login ./internal/module/message/mail -run 'Test.*TwoInstances|Test.*Integration|Test.*Concurrent|Test.*Fault' -count=1`
- `gofmt -l` 对 auth/login 与 message/mail 输出为空（redis.go 终审改动已格式化）。
- `pnpm vitest run tests/api/auth/login.test.ts tests/views/auth/login/index.test.ts --pool=threads --maxWorkers=1`（2 文件 24 项）、`pnpm typecheck`、`pnpm check:architecture`（0 基线项）。
- `git diff --check` 通过。

**终审独立验证的关键行为**

- 双实例：`TestSendCodeTwoServicesSingleProviderWinner`（双 Auth Service/双 Redis client 单 provider winner）、`TestVerificationCodeTwoClientConsumeHasSingleWinner`、`TestPrepareEmailVerifyCodeTwoServicesShareRedisRateWindow`（双 Mail Service/双 Redis client 共享真实限流窗口，429 后窗口恢复重发成功）、`TestVerifyCodeReadinessTwoInstancesRecoverMissingSnapshotOnce`（双实例单回源 + 热读零 PostgreSQL）。
- TTL 同源：`TestSendCodeUsesPreparationTTLAndResendWindow`、`TestPrepareEmailVerifyCodeReturnsConfigTTLAndResendWindow`（TTL=5、resend=60 来自 `business_email_minute.window_seconds`）、`TestSendPreparedEmailVerifyCodePersistsPreparationTTL`。
- 429/503：`TestSendCodeReturnsRateLimitedWhenPrepareIsRateLimited`（429 时 `releaseCalls=1`、`deleteCalls=0`，旧码不被烧毁）、`TestVerificationCodePutReplacesPriorCodeUnderCurrentLease`（窗口后新码替换旧码）、`TestSendCodeFailsClosedWhenCleanupFails`、`TestSendCodeGenerationFailureReportsLeaseCleanupFailure`、`TestSendCodeSuccessfulSendFailsClosedWhenLeaseReleaseFails`。
- 静态自查：`rg` 确认无 `verificationCodeTTL` 残留、无 cooldown key、生产链路无 `context.Background()`；phone 登录在 `service.go` 显式 `Forbidden`；前端 `SendCodeResult` 严格三字段 DTO（1–86400 整数校验）、倒计时仅用 `resendAfterSeconds`、无 `el-select`/`any`/`@ts-ignore`。

**未运行项与剩余风险**

- `-race` 未运行（本机无 C 编译器）。
- 旧 v1 readiness/验证码 key 不迁移，按原 TTL 自然过期。
- `SettingDrawer` 因拆出 `SettingDrawer.css` 新增一个文件，属此前 UI 工作，非本计划范围，保留为可 review diff。

## 后续事项

- 新业务模块按任务路由加载 `$admin-crud`、`$admin-rbac` 或 `$admin-database`，交付后只更新本文件对应条目。
- 需要长期跟踪的功能在本表新增一行，不再创建一组必须同步维护的历史 spec/plan。
- 数据库迁移成功并确认需要快照时，从真实 PostgreSQL 导出 `docs/database/current.sql`；当前未创建该文件，
  不用历史 SQL 拼接伪造。
- 2026-09-05 执行 `docs/database/2026-09-04-mail-rate-limit-policies.sql`：创建
  `message_mail_rate_limit_policy` 并插入 7 条固定策略，创建隐藏 action
  `message:mail:rate-limit:update`；校验通过，并定向清理 1 个旧的权限状态 Redis 键，
  页面重载后 Access version 16 快照已包含该 action；未更新 `docs/database/current.sql`。
- 只有用户明确要求历史审计、决策恢复或迁移对比时，才读取 `docs/superpowers` 或旧 migration。

## 当前工作

### 历史工作入口：统一命名、邮件最高规则与后端维护（2026-09-08，已由后续条目覆盖）

以下内容是历史执行记录；后文 `auth-platform-login-types` 与 TTL 收口条目保留前置工作的记录，不代表当前仍需重新实施。

| 项目 | 内容 |
| --- | --- |
| 目标 | `password-recovery-first-set`：按现有独立契约兜底找回密码和首次设置密码，先完成架构与行为收口 |
| 决策来源 | 维护者本次对话的最终方案；旧 plan/spec 已删除，不恢复、不作为当前依据 |
| 状态 | 历史施工入口；具体结果以本文件顶部的后续完成条目和当前代码为准，不再作为当前待办 |
| 优先级 | 后端 Handler -> Service -> Repository -> Model 与前端 View -> API -> Request 边界优先；UI 视觉统一整改延期，必要的协议及交互正确性仍需验证 |
| 兜底范围 | 认证接口与测试替身一致性、首次设密原子条件写入、forget readiness 失效、passwordSetRequired 生命周期、验证码消费与会话撤销的并发/部分失败验证 |
| 已定契约 | 所有发邮件场景（包括管理测试）服从 Mail 管理配置；邮箱额度按平台+规范化邮箱共享，验证码内容按 scene 隔离。60 秒/5 分钟不是硬规则，不新增 Auth TTL/cooldown。存量 Redis 窗口必须衔接，不能直接换 key 重置额度 |
| 验收 | 保留重置后不自动登录、首次设密不强制拦截及保留会话的既定行为；定向行为测试、共享接口测试编译与构建通过；真实依赖验证和未运行项单独记录 |
| 后续 | 新需求按 `docs/agent/README.md` 路由加载对应 Skill；不要从本历史条目重新开启已完成的命名、邮件或密码整改 |

本轮数据库迁移：`docs/database/2026-09-08-module-naming-mail-policy.sql`。已在真实 PostgreSQL 的隔离
schema 上验证两次执行幂等、跨平台菜单改名、原菜单 ID/角色授权保留、access version 仅递增一次及邮件
配置次数保留。尚未在业务 schema 执行，遵守维护者执行 forward migration 的规则。

发布约束：停止并排空所有旧 API 实例后执行 SQL，再启动新版前后端；不能新旧版本混跑。Access snapshot
使用 `authz:permission:v5:`，access state 使用 `authz:permission-state:v3:`，Mail policy 使用 v2，旧 key
不回读、不全库清理。邮箱限流新 key 使用 `mail:send:email:v2:` / `email10:v2:`（Redis 实际带 `rate:`
前缀），每次至多读取两组各四个已知旧场景 key，以 GCRA 剩余债务原子归并，拒绝请求不扣任一窗口。
已删除的 IP/场景/管理员专属策略停止参与判断，旧专属 key 自然过期；它们不是新版邮箱策略的新额度来源。
旧 key 无 hash tag，该跨 key 归并针对当前 standalone Redis；不支持直接迁至 Redis Cluster 后再归并。

后端维护验收清单（历史记录，已结束）：跨层命名约束与三个 Skill；DTO/Handler/Service/Repository/Model/shared
责任归位与自动检查；首次设密原子写入；邮件全场景共享策略与旧 Redis 衔接；readiness 全场景失效；
refresh 首次设密标记；真实 PostgreSQL/Redis 并发与故障回归。另需评估 Access/Auth Redis 故障无界回源、
菜单全用户失效的容量风险，不能用文件移动代替这些问题的结论。

本轮维护原则：根规则只放硬边界，任务路由负责按需加载，三个项目 Skill 分别维护 CRUD、数据库和 RBAC 施工流程；只将可复用的已确认约束沉淀到 Skill，不把本次密码业务细节扩成通用框架。

本轮已执行验证：`go build ./...` 通过；公共 shared/config/secretkey 定向测试通过；auth/login、message/mail、auth/platform、user/profile 的 `-short` 测试通过（真实依赖测试跳过）。`go test ./... -run '^$'` 因 API/Access 测试替身缺少新增认证接口方法而失败。前端 `pnpm check:architecture` 通过；全量 Vitest 已报告一个存储页面用例失败，随后长时间未完成，本轮启动的进程已停止，无完整通过结论。工作区同时存在其他会话改动，以上仅代表执行当时的结果。

### 前置工作记录：认证平台登录方式

| 项目 | 内容 |
| --- | --- |
| 目标 | `auth-platform-login-types`：把认证平台可配置的邮箱/手机号/密码登录方式连接到 Mail 验证码、Auth、User、Session、RBAC 和前端登录链路 |
| 范围 | `docs/agent/plans/2026-09-05-auth-platform-login-types.md`；AuthPlatform JSONB 策略与 Redis 故障闭合、Mail 认证窄接口、Auth 验证码 Redis 原子消费和 delivery lease、自动注册事务、Session/JWT、登录日志、严格前端 DTO、并发/故障探针 |
| 验收 | Redis ready 命中零 PostgreSQL 配置查询；Redis/数据库/Mail 部分失败显式 `503/10006`；验证码只能消费一次且错误码不删除正确码；并发注册由 PostgreSQL 唯一键收敛并补齐角色/Profile/Access version；登录方式变更递增 policy version 且不撤销既有 Session；全量 Go/Vitest/build 与 review checklist 通过 |
| 下一步/阻塞 | 原实现与终审已完成；维护者随后定案邮箱验证码 TTL 必须唯一来自 Mail 配置（当前 5 分钟），重发只复用现有 Redis rate limit。下一工作单元按 `docs/agent/plans/2026-09-07-mail-verification-contract-closeout.md` 收口 Auth/Mail/前端时间语义和 60 秒后重发；SMS 接入前 `phone` 仍不可用。`-race` 仍受 `CGO_ENABLED=0` 且无 C compiler 阻塞。 |

### `auth-platform-login-types` review 与兜底（2026-09-07）

| 严重度 | Finding/检查项 | 修复与文件 | 验证结果 |
| --- | --- | --- | --- |
| Critical | 可绕过平台 `loginTypes`，且 SMS 未接入时 `phone` 路径可被直接请求 | `auth/login/service.go` 在访问 User 前强制校验策略并拒绝不可用 phone；空密码哈希在 bcrypt 前拒绝 | Auth Service/Handler 回归通过 |
| Critical | 验证码在确认账号状态前消费会烧毁正确码，禁止注册时还可能暴露错误语义 | `auth/login/service.go` 改为 `Check -> identity decision -> Consume`；新账号且禁止注册统一 `401/10002` | 错码不删正确码、禁止注册不消费、单次消费测试通过 |
| Critical | 新 delivery lease 可覆盖未过期验证码；provider/Redis 部分失败可能残留 code/lease 或假成功 | `auth/login/redis.go` Lua 拒绝覆盖；`auth/login/service.go` 清理失败闭合为 `503/10006`，成功发送后释放 lease | Redis Put/Consume/Release、Mail 失败清理与重复 challenge 测试通过 |
| Critical | Policy 缺失恢复的 200ms 等待短于 5s load lock；首请求取消会拖累同实例 waiter，并用取消 context 释放锁 | `auth/platform/service.go`、`redis.go` 统一 5s 契约，使用 `DoChan`、有界共享 context、调用者独立取消和有界释放 context | `TestCurrentPolicyTwoInstancesShareOneMissingKeyRecovery`、`TestCurrentPolicyCanceledLeaderDoesNotCancelSharedRecovery` 连续 3 轮通过，PostgreSQL 仅回源 1 次 |
| Important | PostgreSQL 回源期间旧 publisher 可能盖过 Redis 中已发布的新 policy/loginTypes | `auth/platform/redis.go` 安装失败后读取胜出快照；policy 更新递增 version | 新快照胜出、ready 热读零 PostgreSQL、invalidating/corrupt/Redis error fail-closed 测试通过 |
| Important | Mail readiness 未包含 Redis 限流依赖，且 Auth/Mail 验证码 TTL 所有权不清 | `message/mail/protocol.go`、`service.go`：窄接口接收 Auth TTL，并把 Redis 限流纳入 readiness；Auth 固定返回自身 10 分钟有效期 | Mail readiness、TTL、发送失败测试通过；Auth 不依赖 Mail Repository/GORM |
| Important | 自动注册、唯一键竞争和事务回滚必须在高并发下保持完整 identity | `user/account/repository.go` 仅锁 enabled default role，在单事务创建账号/Profile/Role/Access version；唯一冲突 requery winner | 并发 winner、Profile INSERT 强制失败全回滚、phone-only `email=''`/空 hash 测试通过 |
| Important | LoginLog 故障不应改变已成功登录；Session/JWT 必须使用当前 policy version，关闭登录方式不能撤销既有 Session | `auth/login/service.go` 登录日志改 best-effort，凭证统一由当前策略签发 | 真实 PostgreSQL login-log INSERT 故障仍返回凭证；Session/AuthState/JWT 回归通过 |
| Important | 请求 DTO 可同时携带互斥凭据，响应漏出 `isNewUser`，登录失败类型记录不准 | `auth/login/request.go`、`response.go`、`handler.go` 使用指针字段和互斥校验，显式输出 `isNewUser`，邮箱失败记录 `loginType=email` | Handler/协议测试通过 |
| Important | 前端可在配置失败时显示默认密码方式，且 boolean/loginTypes/时间戳解析不严格 | `web/src/api/auth/*.ts`、登录页严格解析并在配置失败时不渲染登录表单；每次发送尝试轮换 challenge | Auth API 与登录页测试通过；无 phone/fake data 展示 |
| Important | 登录页超过 SFC 架构阈值；认证平台 `reactive const form` 触发 Vue 隐式 `let` 编译警告 | 样式迁至 `auth/login/LoginPage.css`；认证平台表单改为 `ref<AuthPlatformForm>` | `check:architecture` 通过；登录页 9 项、认证平台页 11 项测试通过且无该警告 |
| Critical | 六位验证码可被并发枚举，错误请求还会放大 LoginLog 写入 | `auth/login/redis.go` 使用 Lua 原子校验并按账号 10 次/IP 30 次、10 分钟窗口限流；账号/IP 均为 HMAC key，无 PII；达限在 User/LoginLog 前返回 `429/10007` | 账号/IP 阈值、正确码不删除、共享 Redis 可重复运行及 Service 短路测试通过 |
| Important | Policy 新增 `loginTypes` 后仍复用 v1 Redis namespace；仅调整顺序也会错误递增版本 | `auth/platform/redis.go` 使用 schema/key/load-lock v2；`service.go` 比较前规范化登录方式 | 旧快照绕开、反序 no-op、双实例恢复测试通过 |
| Important | `VerifyCodeReady` 每次公开 `login-config` 查询配置和全部模板，发送前又重复查询 | `message/mail/readiness.go` 新增 Redis readiness 状态机；missing 才经 singleflight + 5 秒 token lock 单次回源，corrupt/invalidating/Redis error 闭合；配置/模板状态写入使用 tokenized CAS 发布 | 双实例 64 路仅 1 组 PostgreSQL 查询、100 次热命中零查询、跨实例失效/旧 owner 拒绝测试通过 |
| Important | Mail/provider 失败时复用已取消请求 context，可能残留 Auth code/lease；Mail readiness 写库后的发布亦可能被取消 | Auth 清理使用 1 秒 `WithoutCancel` 有界 context；Mail rollback 1 秒、提交后发布 5 秒且保留 request values | 取消后 code/lease 清理、readiness rollback/publish 测试通过 |
| Important | `challengeId` 超过数据库 128 字符、管理端空 `loginTypes` 仍可保存、登录配置失败后无法恢复 | 请求增加 `max=128`；表单校验 1-3 个合法且不重复；登录页增加 i18n 失败态与重试按钮，成功后只渲染服务端方式 | Handler、认证平台页、登录页 9 项测试通过 |
| Important | GoLand 的 `PACKAGE + admin/server/...` 将 CLI wildcard 当作单个包校验，无法启动后端全量测试 | `.run/Admin Server Tests.run.xml` 改为从 `$PROJECT_DIR$/server` 执行 `go test ./...` 的 Shell Script 配置；`Admin Full Tests` 同步引用新类型 | 3 个 `.run` XML 解析通过；`go list ./...` 识别 39 个包；最终全量仍由维护者执行 |
| Checked | Public routes、管理端 action RBAC、Access version、hidden action 分离 | `cmd/api/main.go`/测试、Access fixture 与现有 Middleware 映射核对 | `login-config`/`send-code`/`login` 路由基线及 RBAC/Access 测试通过 |
| Checked | context 必须贯穿 Service/Repository/Redis/Mail，不得下传 Gin 或换成 `context.Background()` | Auth 链路使用 request context；Mail Handler 全部改传 `c.Request.Context()`；补偿/发布只使用 `context.WithoutCancel(ctx)` 加硬超时，不创建无界后台 context | 取消请求原错误返回 200 的测试先 RED，修复后返回 `503/10006`；生产调用链静态扫描通过 |

迁移与验证记录：

- 已执行 `docs/database/2026-09-05-auth-platform-login-types.sql`，输出为 `BEGIN`、`ALTER TABLE`、回填、约束/索引重建、`COMMIT`，exit 0；未打印 DSN，未更新 `docs/database/current.sql`。
- Task 12 使用两个 Auth Service/两个 Redis client 验证 policy 单次回源、ready 零额外查询、单 client 故障闭合；验证 32 路验证码 Consume 仅一个 winner、Redis Put/Consume/Release 故障，以及 PostgreSQL identity 回滚/login-log best-effort。
- 实现基线曾完成 `go fmt ./...`、`go vet ./...`、`go test ./...`、`go build ./...`；终审兜底后定向执行 `go test ./cmd/api ./internal/module/auth/login ./internal/module/auth/platform ./internal/module/message/mail -count=1`，四包均通过。
- 前端实现基线曾完成 63 文件/446 项 Vitest 与 `pnpm build`；终审兜底后登录/认证平台/API 4 个文件共 36 项 Vitest、受影响文件 Prettier、`pnpm lint`、`pnpm check:architecture`、`pnpm typecheck` 均通过，最终全量复跑交维护者从 `.run` 执行。全仓 `pnpm format:check` 仍因 3 个未被本任务修改的历史文件失败：`role-view.ts`、`mail.test.ts`、`menus/index.test.ts`。
- `go test -race` 未执行：当前 `CGO_ENABLED=0` 且机器没有 `gcc/clang`；Vite build 仍有既有的 >500 kB chunk 提示。本轮终审兜底未 commit，历史计划文件无本轮修改。
- `.run` 提供 `Admin Server Tests`、`Admin Web Tests` 和一键并行的 `Admin Full Tests`；后端使用 Shell Script 在 `server` 工作目录执行真实 `go test ./...`，避免 GoLand `PACKAGE` 模式拒绝 wildcard。3 个 XML 均可解析，`go list ./...` 识别 39 个包；未代维护者执行最后一轮测试。

### 邮箱验证码契约收口（待执行）

| 项目 | 内容 |
| --- | --- |
| 计划 | `docs/agent/plans/2026-09-07-mail-verification-contract-closeout.md` |
| 固定契约 | Mail `message_mail_config.ttl_minutes` 是邮箱验证码唯一 TTL 来源，当前 5 分钟；重发只使用现有 `business_email_minute=1/60s` Redis rate limit；不新增 cooldown 状态。 |
| 实施范围 | Mail readiness v2 携带 TTL、限流前置预检、Auth lease-owner 原子替换、`resendAfterSeconds` HTTP/前端契约、真实 Redis 双实例故障探针。 |
| 不在范围 | 数据库迁移、SMS/phone 开放、系统设置、身份生命周期、AuthPlatform 字段、`registered_user` 授权、并发协议时间配置化。 |
| 验收 | 5 分钟 TTL 全链路同源；60 秒内 429 且旧码可用，60 秒后新码替换旧码；Redis/PG/provider 故障无假成功；readiness 热读零 PostgreSQL，missing 双实例单回源；Go/Vitest 全量通过。 |

## 找回密码与首次设置密码（2026-09-08）

> 用户授权牵头实现找回密码/首次设置密码契约；探索与交互方案已与用户对齐后端到端落地，未提交。

**后端契约**

- 公开路由 `POST /api/v1/auth/password/forgot`（发码，scene=forget，返回 SendCodeResult）与
  `POST /api/v1/auth/password/reset`（email+code+newPassword+confirmPassword，成功 data 为空对象）。
- 登录、refresh、Me 响应新增 `passwordSetRequired` 布尔字段：凭据 `PasswordHash == ""` 时为 true。
- 认证路由 `POST /api/admin/v1/account/password/set`：仅允许无密码账号首次设置（已有密码返回
  `10005` Conflict），使用 `account:password:update` 权限码，成功后不吊销当前会话；改密仍走
  `ChangePasswordAndRevokeSessions` 吊销全部会话。
- Repository 新增 `SetPasswordHash`；`authenticationService` 接口新增 `ForgotPassword`/`ResetPassword`，
  相关测试桩已同步。

**前端契约与页面**

- `AccessCredential`/`CurrentUser` 严格 DTO 纳入 `passwordSetRequired`（`expectExactKeys` + 布尔校验），
  `utils/request.ts` 的 `isAccessCredential` 探测同步。
- 新增 `/forgotPassword` 页面（复用 LoginPage.css 布局）：邮箱 → 发码（60s 倒计时、challenge 轮换）→
  设置新密码 → 成功回登录页并经 `?account=` 预填；登录页新增忘记密码入口链接。
- profile 密码卡片双形态：`passwordSetRequired` 为 true 时隐藏当前密码框、改调 `setPassword`、成功文案
  提示会话保持；store 在 set 成功后清除标记。
- zh-CN/en-US 文案齐备；所有引用 `AccessCredential`/`CurrentUser` 的测试 stub 已同步新字段。

**验证证据**

- 后端：`go fmt`、`go vet`、`go build`、`go test ./...`（36 包）全部通过。
- 前端：全量 Vitest 64 文件 461 项中 460 项通过，唯一失败（store updateProfile 期望形状）修复后定向重跑
  通过；`pnpm build`（含 vue-tsc）通过，仅存既有 >500 kB chunk 提示。
- 首轮全量中 storage-object/roles 4 项 5s 超时为机器负载抖动，隔离重跑全部通过。
- `go test -race` 仍未执行（机器无 gcc/clang，与既有记录一致）。

## 项目规则维护（2026-09-07）

- `$admin-rbac` 已通过 `skill-creator/scripts/quick_validate.py`；其读写路径与当前 permission 模块核对，明确
  Redis state 确认、本地/Redis snapshot、PostgreSQL fresh source、事务内 access version、tokenized
  invalidation lease 和提交后 CAS 发布顺序。现有 Redis 故障 PostgreSQL 回源若后续调整，必须补跨实例有界
  恢复或改为显式依赖失败，禁止无界每请求回源。
- 前端 5 个生产 SFC（个人资料、认证平台、对象存储配置/规则、邮件收件规则）共 10 个旧下拉已迁移为
  `el-select-v2`；架构脚本会拒绝新的原生 `el-select`。定向 4 文件 47 项 Vitest、typecheck、lint、
  `check:architecture` 和生产 build 通过；完整前端 63 文件 449 项 Vitest 全绿，build 仍有既有的 >500 kB
  chunk 提示。

## 前端 UI 整改（2026-09-08）

- 设计基调升级为「现代精致」：`styles/variables.scss` 与 `index.scss` 新增圆角/阴影/渐变令牌
  （`--admin-radius-*`、`--admin-shadow-*`、`--admin-gradient-*`），暗色有独立阴影覆盖；全局面板圆角
  8px → 12px。
- 产品定名「智澜」：`navigation.admin`（zh 智澜 / en Zhilan）、页脚、`index.html` 标题、登录/忘记密码
  与 Aside/Header 品牌位统一；品牌图标使用 `web/src/assets/logo.png`（黄橙四宫格）。
- 登录/忘记密码彻底重构：共享 `views/auth/auth-page.scss`（漂移光斑 + 主色点阵 + 玻璃拟态面板 + 入场
  动效，`prefers-reduced-motion` 降级）；验证码改用 `el-input-otp`（6 位数字 validator）；登录方式保持
  ElSegmented（测试锚点约束）并重做胶囊滑块样式；新组件 `views/auth/components/AuthDock` 提供页内
  语言/明暗主题/主色快捷调整。
- 个人资料重构为身份横幅 + Tab 分区（基本资料/安全设置），首次设密码用户默认落在安全设置 Tab；
  权限锚点、`UpMedia` 头像、`el-select-v2` 性别契约保持不变。
- UI 偏好升级 v3：新增 `layout`（side 侧边 / top 顶部通栏），v1/v2 存储自动迁移；设置抽屉改为
  el-tabs 分组（主题/布局/界面）并新增布局线框预览选择；布局壳统一为 topbar + body 结构，Aside 品牌可
  用 `show-brand` 隐藏，响应式、移动端抽屉菜单与内容全屏行为不变。
- canvas 登录页同步改版（React/Tailwind）：真实 logo、漂移光斑 + 点阵背景、入场动效、渐变主按钮，
  移除无对应概念的「访问密钥」入口；`globals.css` 新增 `canvas-login-orb` 关键帧。

**验证证据**

- 定向 Vitest：认证页、资料页、布局、SettingDrawer、i18n、theme/ui-preferences 等 12 文件 80 项 +
  品牌替换后 8 文件 52 项，全部通过（含忘记密码页 OTP 用例与 v1→v3 偏好迁移用例）。
- `pnpm build`（web，含 vue-tsc）通过，仅存既有 >500 kB chunk 提示；`pnpm build`（canvas，含 tsc）通过。
- 全量 `pnpm verify:frontend` 由维护者执行。

## 展示型硬编码选项全局字典迁移（2026-09-10，完成）

- 目标：按“后台可维护展示选项进入字典，后端状态机/协议 enum、Yes/No、界面偏好和关联资源不进入字典”的边界，迁移现存业务页面中的同类硬编码 `el-select-v2` 选项。
- 本轮范围：`storage.cos.region`、`storage.file.extension`、`storage.mime.type`、`message.mail.region` 四个全局多语言字典；Storage 保留扩展名/MIME 的多选、筛选、可创建、自定义值及全选/半选行为，COS 与 SES 地域使用独立字典，避免错误绑定各自可用地域。
- 已完成：Storage 一次批量加载三个字典，Mail 独立加载 SES 地域；语言切换重新请求，加载中/失败均禁用依赖选择器和保存动作并显示明确错误，不使用硬编码兜底。Storage 的多选、筛选、可创建、自定义值、全选/半选均保留，全选集合随当前启用字典项变化。其余下拉已复核为 Yes/No、状态机/协议 enum、界面偏好或动态关联资源，不迁移。
- 数据库：新增 forward migration `docs/database/2026-09-10-display-option-dictionaries.sql`，包含 4 个内置字典与 31 个基线项；已有同 value 项不覆盖，保留 code 被非内置字典占用时整笔拒绝。真实 `admin.public` 已只读确认当前仅 `user.gender`、新 code 无冲突；migration 未执行到业务库。隔离 PostgreSQL 已验证重复执行幂等、既有项保持和冲突回滚。
- 验收：后端 `go vet ./...`、`go test ./... -count=1`、`go build ./...` 通过；前端定向 Vitest 38/38、最终全量 Vitest 68 文件 517 项、`pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`（0 findings）、`pnpm build` 均通过。Build 仅有既有的大 chunk 警告。
- 迁移事实：真实 `admin.public` 已于 2026-09-10 15:59 +08:00 执行 `2026-09-10-display-option-dictionaries.sql`，并再次执行确认幂等；最终为 4 个启用内置字典、31 个启用内置项，空 value/中英文标签和重复活动值分组均为 0，原 `user.gender` 3 项未变化。迁移前 `public` 备份位于 `%LOCALAPPDATA%\Admin\backups\display-option-dictionaries-20260910-155853\public-before.dump`，大小 123773 字节，SHA256 `C85F6226715AF35ACA9A3FC4F046DC942EF1E28F55525F69F056E836BDAF324D`，`pg_restore --list` 得到 229 个归档条目。此次仅写入业务 seed，未修改 schema，因此不刷新 `current.sql`；未重启 API、未清理 Redis、Session、Access 或 Mail 额度。

## 系统字典管理（2026-09-10）

- 目标：新增系统设置下的全局多语言字典管理；字典 `code/value` 稳定不可修改，`isBuiltin` 仅表示系统保护，不等同于代码 enum。COS 上传策略、邮件限流策略和登录 Session 仍按认证平台隔离，字典不拆平台。
- 范围：后端 `system/dictionary` CRUD、双语标签、options 批量读取、严格 DTO、RBAC action、PostgreSQL migration/menu；前端 API/Pinia 多语言状态、主从管理页和筛选；Redis generation/mutation/snapshot 缓存及跨实例冷回源租约。
- 迁移事实：两份 forward migration 已在真实 `admin` PostgreSQL 执行并重复验证幂等，`docs/database/current.sql` 已由真实库刷新。`canvas` 没有系统设置目录，因此菜单/action 仅写入 `admin` 平台；字典数据仍全局共享。体验 seed 已写入内置 `user.gender` 及 `0/1/2` 三个双语选项。
- 本轮收口：唯一冲突统一映射 409，字典及字典项补 Trim 和长度边界，新增项改为指针写入并正确返回数据库自增 ID；管理页补齐 loading/empty/error、筛选翻页、权限按钮、主从 CRUD、不可变字段及内置删除保护测试。首个业务迁移已完成：个人中心性别通过登录态共享端点 `/api/v1/system/dictionary/options` 和 Pinia Store 加载，严格将字符串值收窄为数字 `0 | 1 | 2`，语言切换重新加载，失败或畸形值不使用硬编码兜底。
- 验收：后端 `go vet ./...`、`go test ./...`、`go build ./...` 全部通过；前端 `pnpm typecheck`、`pnpm lint`、`pnpm check:architecture`（0 findings）、`pnpm build` 全部通过；全量 Vitest 68 文件 512 项通过。Build 仅报告既有的大 chunk 警告。
- 下一步/阻塞：API 未由 Agent 重启，需用新二进制验证共享 options、真实 Redis generation/mutation/snapshot 和个人中心双语切换。后续继续按业务优先级迁移展示型硬编码下拉；登录方式、Session 状态、邮件发送状态、权限节点类型和 Yes/No 等程序协议 enum 保留在代码中。

## 状态条目模板

### 通用 Scheduler 第一轮实现（2026-09-20，已迁移）

- 已完成调度器领域契约、PostgreSQL schedule/job/run schema 与 PowerShell forward runner；runner 定向清理旧通知 Asynq task，修复删除分页收缩跳过任务的问题，并已验证脚本可解析。
- 已实现 Repository 的 due claim、publish/run lease、重试、租约恢复、终态清理，Scanner、Publisher、Worker Executor、内置 realtime/notification/history 清理任务和 Scheduler 管理 API/RBAC。
- 通知批次已改为 Scheduler Job，不再运行时读取 `message_notification_dispatch_outbox`、dispatch relay 或 retention trigger；旧表只在迁移 SQL 中删除，真实迁移尚未执行。
- 新增 Scheduler 管理页、Job/Run 详情和中英文文案；API/Worker 已显式装配 scanner、publisher、scheduler envelope worker。
- 验证：新增 Scheduler 迁移幂等/部分 schema 回滚 fixture，以及真实 PostgreSQL 中可空 lease 字段读取回归用例；`go test ./internal/database ./internal/module/system/scheduler ./cmd/api ./cmd/worker -count=1`、`go vet ./...`、`go build ./...` 和 PowerShell migration parser 均通过。全量后端此前唯一一次失败仍是 `system/setting` 并发 Redis scope flush 用例的抖动，单独重跑通过，未修改既有缓存协议；前端仅执行既有 `pnpm typecheck`、`pnpm build`，全量 Vitest 留给维护者。
- 真实迁移：2026-09-20 09:19（Asia/Shanghai）停止 API/Worker 后执行 `docs/database/2026-09-19-system-scheduler.ps1 -OldAPIStopped`；PostgreSQL 创建三张 Scheduler 表、三个内置计划和 `system.scheduler.history_retention_days=30`，删除 `message_notification_dispatch_outbox`。runner 两轮定向 Redis 清理均为 `deleted=0 remaining=0`，未删除其他队列任务。
- 幂等复验：同一 runner 第二次执行成功，目标 seed 为 `INSERT 0 0`、无新增计划；实库只读核对为 `tables=3`、三个内置计划、`system.setting/global generation=3`、旧 outbox 表数量 `0`、历史保留设置数量 `1`。真实结构快照已刷新至 `docs/database/current.sql`。
- 备份：有效迁移前归档为 `%LOCALAPPDATA%/Admin/backups/system-scheduler-20260920-091903/public-before.dump`（226599 bytes，SHA256 `08296AAA19FF3B7DA2DE611C64A41F105449B6AD87B663678738325B19946EBB`）；幂等复验另生成 `system-scheduler-20260920-092437` 归档。首次 DSN 失败留下的 0 字节目录 `system-scheduler-20260920-091452` 未删除，避免未经明确授权清理备份。
- 下一步/阻塞：API/Worker 当前保持停止，维护者可自行启动新版本做 Scheduler 页面和通知链路验收；前端全量测试、浏览器链路和真实 Worker 执行尚未由 Agent 运行。

复制以下四行到“后续事项”或新建当前工作条目即可：

```text
目标：
范围：
验收：
下一步/阻塞：
```

### 邮件与短信模板正文统一（2026-09-14）

- 已统一两类模板字段：API 使用 `variableKeys`，数据库使用 `variable_keys`；短信新增可上传腾讯云的 `content`，邮件保存完整 HTML，邮件模板 ID 可空但启用时必须为正整数。
- 已完成后端 Service/Repository/Model/Handler、运行时指针 ID 适配和模板校验；短信位置变量使用 `{1}` 到 `{n}`，邮件预览使用 sandbox iframe。
- 前端已改用 `el-input-tag` 管理动态变量，邮件富文本编辑器懒加载至 `web/src/views/message/mail/template/components/MailHtmlEditor/`，并保留 HTML 源码、预览和复制入口。
- 实际验证：后端 `go test ./internal/module/message/sms/... ./internal/module/message/mail/...` 通过；前端 `vue-tsc --noEmit --skipLibCheck`、短信/邮件 API 与 HTML 工具 Vitest（48 项）通过；`vite build` 通过并产出独立 `MailHtmlEditor` chunk。
- 最新执行（维护者已授权）：迁移在真实 PostgreSQL 完成 ROLLBACK 演练、正式 COMMIT、再次执行幂等验证。两表各 4 行，content/variable_keys 非空，Mail provider ID 可空，旧 variables/parameter_keys 已移除。current.sql 已从 public schema 重新导出。
- 迁移备份：`%LOCALAPPDATA%/Admin/backups/template-content-20260914-155047/templates-before.dump`，pg_restore --list 验证可读，SHA256 `2FDC59198FFA3115A121E2034BBFDC17305FBD6942593070F8F1C426F89CA691`。
- 更正前一阶段类型验证结论：根 tsconfig 仅含 references，vue-tsc --noEmit 不代表子项目通过。本轮实际 `pnpm typecheck`（vue-tsc -b）已通过。已更新邮件页面旧 fixture、修复 SMS 实际误用普通 el-input 和 JSON tag 误大写。
- 本轮后端 `go test ./internal/module/message/sms/... ./internal/module/message/mail/...` 通过；SMS 页面定向 Vitest（forks、单 worker）11/11 通过，仍有既有 i18n warning。threads 定向组合运行无结果后已中断，不计为通过。相关前端文件已定向 Prettier 格式化。
- es5-ext postinstall 经检查只输出提示、不生成构建产物，显式设置 allowBuilds=false，pnpm 安装保护问题已解除。前端全量测试由维护者执行，本轮不执行全量。
- 未重启 API/Worker、未清理 Redis、未调用真实腾讯云。新旧字段不兼容，联调必须使用新进程；既有 runtime 缓存与新增变量发送支持仍需专项核对。编辑器独立 chunk 约 670KB，仍有构建警告，不能称为完全解决 chunk 问题。

### 邮件模板编辑体验修复（2026-09-14）

- AppDialog 默认 content height 改为 500px，内部统一使用 el-scrollbar；footer 留在滚动区外并增加顶边框。邮件模板弹窗改用 AppDialog，基础字段和变量压缩在顶部，编辑器与 sandbox 预览左右并排，示例变量同行显示。
- 完整邮件 HTML 新增 `data-mail-editor="content"` 可视化编辑边界；富文本只更新该区域，不再替换 body。编辑器忽略初始化 change，只有用户在编辑区操作后才回写，避免点击即丢失完整 HTML 和邮件卡片样式。
- migration 已再次在真实 PostgreSQL 执行，4 个邮件模板均有编辑边界，current.sql 已刷新。AppDialog/HTML 工具定向测试 7/7、Mail 页面测试 20/20、pnpm typecheck 通过；前端全量测试由维护者执行。
