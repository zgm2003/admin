# 系统任务队列中文只读监控实施计划

> 执行者说明：严格按任务顺序执行。行为改动遵循“失败测试 -> 最小实现 -> 测试通过 -> 重构”。不要自动提交、推送、回滚其他开发者改动，也不要未经维护者确认执行真实数据库迁移。

**目标：** 在“系统设置”下增加“任务队列”页面，以安全的同源 iframe 嵌入 Asynqmon v0.7.2 中文只读界面。

**核心方案：** 标准 Admin API 签发 60 秒 Redis grant，通过限定路径的 HttpOnly Cookie 授权 iframe；iframe 路由独立校验 grant 后进入官方 Asynqmon Handler。UI 使用官方 v0.7.2 前端源码制作项目内中文构建并通过 Go `embed` 提供，不修改官方队列 API 行为。

**技术栈：** Go、Gin、Redis、Asynq、Asynqmon v0.7.2、Vue 3、TypeScript、Element Plus、Vitest、PostgreSQL forward migration。

---

## 一、固定契约

### 1. 页面与权限

| 节点 | 路径/接口 | 权限 | 说明 |
| --- | --- | --- | --- |
| 页面 | `/system/queueMonitor` | `system:queueMonitor:view` | 动态路由和菜单入口 |
| Grant | `POST /api/admin/v1/system/queuemonitor/grant` | `system:queueMonitor:list` | 标准 Admin 认证和 RBAC |
| UI | `/api/admin/v1/system/queuemonitor/ui/` | 短期 grant Cookie | iframe 不携带自定义 Header |

菜单层级：

```text
系统设置
└── 任务队列
```

Page 字段固定为：

```text
path          = /system/queueMonitor
componentPath = system/queueMonitor
code          = system:queueMonitor:view
i18nKey       = navigation.systemQueueMonitor
isHidden      = 0
```

Action 字段固定为：

```text
code          = system:queueMonitor:list
parentId      = 任务队列 page ID
path          = NULL
componentPath = NULL
icon          = NULL
i18nKey       = NULL
isHidden      = 1
```

Page 与 action 独立授权，不得互相派生。Action 不进入 `menuTree`，page 权限不能访问 Grant API。

### 2. Grant 与 Cookie

- 原始 grant 使用 `crypto/rand` 生成 32 字节并以 `base64.RawURLEncoding` 编码。
- Redis key：`system:queue-monitor:grant:v1:<sha256(rawGrant)>`。
- Redis value 使用严格 lower camel case JSON，至少包含 `userId`、`platformId`、`permissionCode`、`issuedAt`。
- TTL 固定 60 秒；Redis error 与 grant missing 必须区分。
- Redis 故障闭合，不允许进程内或 PostgreSQL 兜底。
- Grant 响应只返回 `expiresAt`，不得返回原始 grant。
- 每次续期都重新经过 Session、Admin 平台和 `system:queueMonitor:list` 校验；权限或会话撤销后不再签发新 grant。
- 已签发 grant 不在 iframe 热路径查询 PostgreSQL 或 Access 快照，权限撤销后的最大残留访问窗口明确为 60 秒。

Cookie 固定属性：

```text
Name     = admin_queue_monitor_grant
Path     = /api/admin/v1/system/queuemonitor/ui
HttpOnly = true
Secure   = settings.AuthCookieSecure
SameSite = Lax
MaxAge   = 60
Domain   = 不设置
```

iframe URL、查询参数、日志和响应体均不得出现 grant、Access Token 或 Authorization Header。

### 3. Asynqmon 与中文 UI

- 固定 `github.com/hibiken/asynqmon v0.7.2`。
- 显式配置 `ReadOnly: true`。
- 复用当前 `REDIS_URL`，不新增 Queue Redis 配置或数据库编号。
- UI 网关只允许 `GET`、`HEAD`、`OPTIONS`；其他方法返回 405。
- 使用官方 v0.7.2 UI 源码翻译用户可见文案，不直接修改压缩产物。
- 队列名、任务类型、Payload、错误码和业务数据保持原文。
- 保留上游版本、仓库地址和 LICENSE。
- 中文 dist 不包含 `.map` 或 `sourceMappingURL`。
- 静态资源通过 Go `embed` 提供，不依赖当前工作目录、绝对路径或 Module Cache。

### 4. 前端生命周期

```text
进入页面 -> 请求 grant -> 成功后挂载 iframe -> 每 45 秒续期
页面隐藏 -> 停止续期
页面恢复可见 -> 立即重新 grant -> 恢复定时器
组件卸载/失活 -> 清理定时器和事件监听
```

续期不得修改 iframe `src` 或触发 iframe 刷新。初次 grant 失败时不加载 iframe；续期失败必须显示可重试状态，不得伪造成功。

### 5. 非目标

- 不实现定时任务业务、任务生产者或新 Worker Handler。
- 不提供暂停、恢复、删除、归档、重试等写操作。
- 不自研 Asynqmon API，不引入 WebSocket。
- 不实现 Asynqmon 动态中英文切换，本轮内嵌 UI 固定中文。
- 不修改全局 Element Plus 主题或无关页面样式。
- 不在 API/Worker 启动时执行 DDL 或 seed。

---

## 二、预计文件

先以仓库真实结构为准核对，禁止为了匹配计划机械创建空抽象。

```text
server/go.mod
server/go.sum
server/cmd/api/main.go
server/internal/module/system/queueMonitor/
  protocol.go
  grant_store.go
  grant_store_test.go
  service.go
  service_test.go
  handler.go
  handler_test.go
  route.go
  route_test.go
  asynqmon.go
  asynqmon_test.go
  ui/
    README.md
    LICENSE
    build.ps1
    upstream/
    dist/
web/src/api/system/queueMonitor.ts
web/src/views/system/queueMonitor/index.vue
web/tests/api/system/queueMonitor.test.ts
web/tests/views/system/queueMonitor/index.test.ts
docs/database/2026-09-14-system-queue-monitor.sql
server/internal/database/*queue_monitor*_test.go
docs/agent/STATUS.md
```

如果官方 Handler 无法注入自定义文件系统，允许增加最小静态资源 Handler，但不得复制或修改官方队列查询 API。目录应根据 v0.7.2 真实接口调整，并在 `ui/README.md` 说明原因。

---

## 三、实施任务

### Task 1：核对基线和真实接口

**只读检查：**

```powershell
cd D:\admin
git status --short
rg -n "asynq|REDIS_URL|AuthCookieSecure|RequirePermission|navigation.system" server web docs/agent
```

读取：

- `server/cmd/api/main.go` 与 `server/internal/queue`；
- 当前 system 模块的 Route、Handler、Service 和 Redis Store 写法；
- Auth Session 身份结构、错误映射、Cookie 配置与 Origin 校验；
- permission menu/access 的路由和测试；
- 前端动态路由、API 严格解析器、`AppPage` 和相邻 system 页面；
- 当前 PostgreSQL 的系统目录、平台和菜单事实，只执行只读 SQL；
- Asynqmon v0.7.2 的 `Config`、`New`、`ServeHTTP`、RootPath、ReadOnly 和静态资源实现。

精确读取旧项目对应实现作为行为参考：

```text
D:\github-project\admin_back_go\internal\module\queuemonitor\asynqmon.go
D:\github-project\admin_back_go\internal\module\auth\browser_grant.go
D:\github-project\admin_front_ts\src\views\Main\system\queueMonitor\index.vue
```

本任务结束时记录：

1. 标准 Admin group 和原始 router 的实际装配点；
2. 当前 Redis client 接口和 Redis URL 转换函数；
3. 当前身份中 user/platform ID 的真实类型；
4. Asynqmon API 与静态资源是否可分离；
5. 系统设置目录在真实 PostgreSQL 中的稳定定位字段；
6. 工作区已有改动，后续不得覆盖。

### Task 2：Grant Store 和 Service

**RED：先新增测试**

覆盖：

- grant 可解码为 32 字节且连续生成不重复；
- Redis key 只包含 SHA-256，不包含原始 grant；
- 写入 TTL 精确为 60 秒；
- value 严格保存 user、platform、权限和签发时间；
- 正常校验、missing、畸形 JSON、Redis error、context cancel；
- 缺失或空 grant 直接拒绝；
- 签发失败不返回 grant 或 `expiresAt` 假成功。

```powershell
cd D:\admin\server
go test ./internal/module/system/queueMonitor -run "TestGrant|TestService" -count=1
```

确认因生产能力缺失而失败，不接受 import、语法或夹具错误。

**GREEN：最小实现**

- `grant_store.go` 只封装 Redis grant 写入和校验；
- `service.go` 负责随机凭据、身份绑定、过期时间和错误语义；
- 所有 I/O 使用传入的 `context.Context`；
- 复用项目错误类型，不向 HTTP 层泄露 Redis 原始错误。

重复同一测试，确认通过后再整理命名和重复代码。

### Task 3：Grant Handler、Cookie 和标准路由

**RED：** 新增 Handler/Route 测试：

- 成功 envelope 只有 `code/data/message`，data 只有合法 `expiresAt`；
- Cookie 名称、Path、HttpOnly、SameSite、MaxAge 正确；
- `Secure` 分别验证 true/false；
- 响应体和 Location 不含 grant/Token；
- Service 错误映射为明确依赖错误；
- request context 被传到 Service；
- Route 显式绑定 `system:queueMonitor:list`；
- 无认证、错误平台、无 list 权限均不能签发。

```powershell
go test ./internal/module/system/queueMonitor -run "TestHandler|TestRoute" -count=1
```

**GREEN：** Handler 只调用 Service、设置 Cookie、输出 envelope；Route 只声明 URL、认证和显式权限。复用已有 Admin Origin/CSRF 约束，不另造第二套策略。重复测试。

### Task 4：UI Grant 中间件和只读 HTTP 边界

**RED：** 覆盖：

- 无 Cookie、空 Cookie、无效或过期 grant 返回未认证；
- Redis error/畸形 value 返回依赖错误，而非未认证或空页面；
- 有效 grant 可进入下游 Handler；
- `GET/HEAD/OPTIONS` 可进入，其他方法返回 405；
- UI 路由无需 iframe 无法提供的 Bearer 和自定义 Header；
- 路由只能覆盖 `/api/admin/v1/system/queuemonitor/ui` 子树；
- 响应设置合适的 `Cache-Control`，错误不含 Token、Redis DSN 或堆栈。

```powershell
go test ./internal/module/system/queueMonitor -run "TestUI|TestGrantMiddleware" -count=1
```

**GREEN：** UI 子树挂在原始 Gin router；专用中间件校验 Cookie/Redis grant；方法限制在服务端执行；不使用内存授权缓存。重复测试。

### Task 5：接入官方 Asynqmon v0.7.2

按当前 `server/go.mod` 状态添加固定依赖。执行前后检查 diff，只接受必要依赖变化；若核心依赖意外升级，使用显式版本调整，禁止重置整个文件。

```powershell
cd D:\admin\server
go get github.com/hibiken/asynqmon@v0.7.2
go mod tidy
```

**RED：** 集成测试覆盖：

- `ReadOnly=true`；
- Redis 配置来自当前 `REDIS_URL`；
- RootPath 与 UI 挂载路径一致；
- 有效 grant 可访问首页、一个静态资源和一个官方只读 API；
- 写方法被 UI 网关拒绝；
- 切换进程 cwd 后仍可读取静态资源；
- 错误响应不暴露 DSN。

```powershell
go test ./internal/module/system/queueMonitor -run "TestAsynqmon|TestEmbeddedUI" -count=1
```

**GREEN：** `cmd/api/main.go` 显式注入 Redis 配置；Redis URL 转换留在 `internal/queue`；官方 Handler 负责官方 API；自定义 Handler 只在官方不支持 embed FS 时提供中文静态资源。不得修改 Go Module Cache。重复测试。

### Task 6：制作可审计的中文 UI 构建

先检查 v0.7.2 UI 的真实包管理器和构建命令，不假定 npm/pnpm/yarn：

1. 同步构建所需官方源码到 `ui/upstream`，或保存可稳定重建的补丁和锁文件；
2. 在 `ui/README.md` 记录上游仓库、tag/commit、中文修改范围和构建命令；
3. 保留 LICENSE；
4. 翻译源码中的全部用户可见固定文案；
5. 不翻译队列名、任务类型、Payload 和服务端数据；
6. 设置与 RootPath 相容的资源/API base path；
7. 禁用 source map 并构建到 `ui/dist`；
8. 删除 `node_modules`、缓存、日志和临时产物；
9. 确认 `.gitignore` 不会排除运行时 dist；
10. 用 Go `embed` 绑定 dist，重新运行 Task 5 测试。

```powershell
cd D:\admin
Get-ChildItem -Recurse server\internal\module\system\queueMonitor\ui\dist -Filter *.map
rg -n "sourceMappingURL|accessToken|authorization|token=" server/internal/module/system/queueMonitor/ui/dist
rg -n "C:\\Users|D:\\admin|go\\pkg\\mod" server/internal/module/system/queueMonitor/ui/dist
```

预期：无 `.map`、无敏感认证硬编码、无本机绝对路径。

### Task 7：前端严格 API

**RED：** `web/tests/api/system/queueMonitor.test.ts` 覆盖 POST 路径、请求不含 Token、严格解析合法 `expiresAt`，并拒绝缺失字段、错误类型、非法日期和畸形 envelope。

```powershell
cd D:\admin\web
pnpm vitest run tests/api/system/queueMonitor.test.ts --pool=threads --maxWorkers=1
```

**GREEN：** 新增 `web/src/api/system/queueMonitor.ts`，从 `unknown` 严格缩小为 `QueueMonitorGrantResponse`。禁止 `any`、`as any`、`@ts-ignore` 和宽泛 ambient declaration。重复测试。

### Task 8：前端任务队列页面

**RED：** `web/tests/views/system/queueMonitor/index.test.ts` 覆盖：

- 初次 grant 成功后才挂载 iframe；
- iframe URL 固定且不含 Token；
- 首次失败显示错误和重试，不重复通知；
- 45 秒续期且不改变 iframe src；
- 页面隐藏停止续期，恢复可见立即 grant；
- `onDeactivated`/`onBeforeUnmount` 清理 timer 和 listener；
- iframe load/error 正确切换状态；
- 无 list 权限时不请求 Grant，并显示无权限状态。

```powershell
pnpm vitest run tests/views/system/queueMonitor/index.test.ts --pool=threads --maxWorkers=1
```

**GREEN：** 新增 `web/src/views/system/queueMonitor/index.vue`：

- 复用 `AppPage` 和现有错误/空态模式；
- 页面以 iframe 为主，不堆叠装饰卡片；
- iframe 使用剩余可用高度和稳定 `min-height`，避免双滚动；
- 不修改全局 CSS 或 Element Plus 主题；
- 中英文增加外层文案，内嵌 Asynqmon 固定中文。

建议 i18n 键：

```text
navigation.systemQueueMonitor
system.queueMonitor.title
system.queueMonitor.loading
system.queueMonitor.loadFailed
system.queueMonitor.grantFailed
system.queueMonitor.forbidden
system.queueMonitor.retry
```

重复页面测试，再联合运行 API + 页面测试。

### Task 9：菜单 forward migration

先用只读 SQL 检查真实 PostgreSQL 的认证平台、系统设置目录、目标 path/code 和当前 `menu_version`，不能从历史 SQL 推断现状。

**RED：** 按当前 `internal/database` PostgreSQL 测试模式覆盖：

- 只给存在系统目录的 Admin 平台新增 page/action；
- parent、path、componentPath、code、i18nKey、hidden 准确；
- Canvas 不受影响；
- 不自动扩大普通角色权限；
- 重复执行不重复插入、不重复递增版本；
- path/code/父节点冲突时整体回滚；
- 只对真实变化的平台将 `menu_version` 增加一次。

```powershell
cd D:\admin\server
go test ./internal/database -run TestSystemQueueMonitorMigration -count=1
```

**GREEN：** 新增 `docs/database/2026-09-14-system-queue-monitor.sql`：单事务、幂等、冲突显式失败；通过稳定业务字段定位父目录，不写死 ID；保留已有菜单 ID 和角色授权；不影响无关平台；启动过程不得调用迁移。重复测试。

**硬门槛：** 本任务只生成和测试迁移文件。除非维护者在执行阶段再次明确授权，否则不得连接真实库执行 migration、备份、刷新 `current.sql` 或清理 Redis。

### Task 10：显式装配和跨层契约

在 `server/cmd/api/main.go` 显式装配：

```text
Redis Grant Store -> Queue Monitor Service -> Grant Handler
REDIS_URL -> Queue Redis Option -> Asynqmon ReadOnly Handler
原始 Router -> UI Grant Middleware -> Asynqmon/UI Handler
Admin Group -> Authenticate -> RequirePermission(list) -> Grant Handler
```

禁止 DI 容器、运行时注册器、`Manager`、`Factory`、通用 `Adapter` 或第二套 Router。

架构测试至少验证：Grant Route 精确绑定 list 权限；page/action 与 migration 一致；`componentPath` 可被前端受限 glob 解析；UI 子树没有公开旁路；Handler 不直接依赖 Redis/GORM/Asynq client。

### Task 11：更新状态文档

更新 `docs/agent/STATUS.md`，记录：

- 菜单、权限和 API 映射；
- Cookie Path、60 秒 TTL 与 45 秒续期；
- Redis namespace 和故障闭合行为；
- Asynqmon v0.7.2、`ReadOnly=true` 和中文构建来源；
- 实际测试命令及结果；
- 真实 migration 是否执行；
- 未运行的前端全量测试和浏览器验收；
- 固定中文 UI 暂不跟随 Admin 英文模式的限制。

不要回填或修改其他历史计划。

---

## 四、最终验证

### 后端

```powershell
cd D:\admin\server
go fmt ./...
go test ./internal/module/system/queueMonitor/... -count=1
go test ./internal/database -run TestSystemQueueMonitorMigration -count=1
go test ./internal/architecture -count=1
go vet ./internal/module/system/queueMonitor/... ./cmd/api/...
go build ./cmd/api
```

如果 Queue 公共配置被修改，追加：

```powershell
go test ./internal/queue/... -count=1
```

### 前端定向检查

```powershell
cd D:\admin\web
pnpm vitest run tests/api/system/queueMonitor.test.ts tests/views/system/queueMonitor/index.test.ts --pool=threads --maxWorkers=1
pnpm typecheck
pnpm check:architecture
```

维护者已说明前端全量测试由其执行。不得把以下命令报告为已运行，除非获得新指令并看到 exit 0：

```powershell
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

### 静态资源与差异

```powershell
cd D:\admin
Get-ChildItem -Recurse server\internal\module\system\queueMonitor\ui\dist -Filter *.map
rg -n "sourceMappingURL|accessToken|authorization|token=" server/internal/module/system/queueMonitor/ui/dist
rg -n "C:\\Users|D:\\admin|go\\pkg\\mod" server/internal/module/system/queueMonitor/ui/dist
git diff --check
git status --short
```

逐文件审查 diff，排除密钥、`.env`、构建缓存、`node_modules`、无关格式化和其他开发者改动。

---

## 五、主 Agent 兜底审查

国产模型执行后，主 Agent 不接受口头结论，必须重新读取 diff 和测试输出。

### 安全

- iframe URL、HTML、JS、日志和响应中没有 grant/Access Token；
- Cookie 为 HttpOnly、限定 Path、Secure 跟随配置、Domain 为空；
- Redis 只使用 grant hash key，TTL 为 60 秒；
- Redis error 不被当成 missing，且没有内存/数据库兜底；
- UI 路由未经 Cookie 无法访问，非只读方法返回 405；
- `ReadOnly=true` 是实际配置，不只是前端隐藏按钮；
- 错误不泄露 DSN、Token、堆栈或 Redis value。

### RBAC 与路由

- `:view` 只控制页面，`:list` 保护 Grant；
- 无 view 不注册动态路由，无 list 不签发 grant；
- page/action 不互相派生，action 不进入 `menuTree`；
- UI 子树只通过短期 Cookie 进入，没有公开旁路；
- 菜单只进入正确 Admin 平台和系统设置目录，Canvas 不变。

### 前端

- DTO 从 `unknown` 严格解析，无 `any`、`as any`、`@ts-ignore`；
- 初次 grant 前 iframe 不发请求；
- 续期不刷新 iframe；
- 页面隐藏、失活、卸载均清理计时器；
- 请求层与页面不重复通知同一错误；
- 没有全局主题/CSS 改动或无意义组件抽象。

### 中文构建

- 源码确实来自 Asynqmon v0.7.2，并保留 LICENSE 和构建说明；
- 翻译发生在源码而非压缩 JS 字符串替换；
- dist 无 source map、绝对路径和临时文件；
- 首页、静态资源和官方 API 在 Windows 不同 cwd 下均可用；
- 队列名、任务类型和 Payload 未被错误翻译。

### 迁移

- 真实 PostgreSQL 是设计依据；
- migration 幂等、冲突回滚、版本只增一次；
- 不写死父 ID、不修改已有菜单 ID、不扩大普通角色权限；
- 没有启动时 migration；
- 未获明确授权时真实数据库保持未执行。

发现任一问题，主 Agent 必须补失败测试后修正，而不是只列风险。

---

## 六、交付格式

```text
功能：系统设置 -> 任务队列

已修改：
- 精确文件列表

权限：
- system:queueMonitor:view
- system:queueMonitor:list

安全：
- Grant Cookie TTL：60 秒
- 续期周期：45 秒
- Cookie Path：/api/admin/v1/system/queuemonitor/ui
- iframe URL：是否携带 Token
- Asynqmon ReadOnly：实际值

中文 UI：
- 上游版本
- 构建命令
- LICENSE 状态
- source map 检查结果

实际执行：
- 命令、exit code、测试数量

未执行：
- 前端全量测试、真实 migration、浏览器验收等

剩余风险：
- 仅列仍客观存在且无法在本轮消除的风险
```

没有新鲜命令证据时，不得使用“通过”“完成”“已修复”等结论。
