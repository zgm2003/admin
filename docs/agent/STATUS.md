# 项目状态

> 这是当前唯一的进度入口。它记录现在要做什么、已经交付什么和下一步做什么；不回填历史
> `docs/superpowers` plan。

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

### 当前优先工作：统一命名、邮件最高规则与后端维护（2026-09-08）

以下条目是当前执行入口；后文 `auth-platform-login-types` 与 TTL 收口条目保留前置工作的记录，不代表本轮仍需重新实施。

| 项目 | 内容 |
| --- | --- |
| 目标 | `password-recovery-first-set`：按现有独立契约兜底找回密码和首次设置密码，先完成架构与行为收口 |
| 决策来源 | 维护者本次对话的最终方案；旧 plan/spec 已删除，不恢复、不作为当前依据 |
| 状态 | 方案已确认，实施及全量回归中；尚未宣称整体交付完成 |
| 优先级 | 后端 Handler -> Service -> Repository -> Model 与前端 View -> API -> Request 边界优先；UI 视觉统一整改延期，必要的协议及交互正确性仍需验证 |
| 兜底范围 | 认证接口与测试替身一致性、首次设密原子条件写入、forget readiness 失效、passwordSetRequired 生命周期、验证码消费与会话撤销的并发/部分失败验证 |
| 已定契约 | 所有发邮件场景（包括管理测试）服从 Mail 管理配置；邮箱额度按平台+规范化邮箱共享，验证码内容按 scene 隔离。60 秒/5 分钟不是硬规则，不新增 Auth TTL/cooldown。存量 Redis 窗口必须衔接，不能直接换 key 重置额度 |
| 验收 | 保留重置后不自动登录、首次设密不强制拦截及保留会话的既定行为；定向行为测试、共享接口测试编译与构建通过；真实依赖验证和未运行项单独记录 |
| 下一步 | 前端对齐后端 user 等业务域，所有资源使用单数；闭合 API/菜单/权限/i18n/测试。邮件只保留每分钟与每 10 分钟两条平台+邮箱策略，删除其他五条。UI 改版不在本轮范围 |

本轮数据库迁移：`docs/database/2026-09-08-module-naming-mail-policy.sql`。已在真实 PostgreSQL 的隔离
schema 上验证两次执行幂等、跨平台菜单改名、原菜单 ID/角色授权保留、access version 仅递增一次及邮件
配置次数保留。尚未在业务 schema 执行，遵守维护者执行 forward migration 的规则。

发布约束：停止并排空所有旧 API 实例后执行 SQL，再启动新版前后端；不能新旧版本混跑。Access snapshot
使用 `authz:permission:v5:`，access state 使用 `authz:permission-state:v3:`，Mail policy 使用 v2，旧 key
不回读、不全库清理。邮箱限流新 key 使用 `mail:send:email:v2:` / `email10:v2:`（Redis 实际带 `rate:`
前缀），每次至多读取两组各四个已知旧场景 key，以 GCRA 剩余债务原子归并，拒绝请求不扣任一窗口。
已删除的 IP/场景/管理员专属策略停止参与判断，旧专属 key 自然过期；它们不是新版邮箱策略的新额度来源。
旧 key 无 hash tag，该跨 key 归并针对当前 standalone Redis；不支持直接迁至 Redis Cluster 后再归并。

后端维护验收清单（本轮进行中）：跨层命名约束与三个 Skill；DTO/Handler/Service/Repository/Model/shared
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

## 状态条目模板

复制以下四行到“后续事项”或新建当前工作条目即可：

```text
目标：
范围：
验收：
下一步/阻塞：
```
