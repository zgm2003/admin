# Admin API 命名空间基线设计

日期：2026-08-27

状态：已实施并完成验证

## 1. 目的

本设计在继续认证、RBAC、COS 和其他 Admin 功能前，固定共享身份接口与 Admin 管理接口的
URL 边界。当前项目已经确认由一个 Go 身份后端服务多个独立前端；如果所有接口继续混在
`/api/v1`，后续业务前端接入时将同时影响后端路由、前端 API、审计规则、测试和文档。

本基线只调整 HTTP 命名空间和请求级平台边界，不改变现有 Handler、Service、Repository、
数据库模型、DTO、权限码或业务行为。它优先于
`2026-08-27-admin-authentication-baseline-design.md` 及其实施计划执行，后续功能直接使用最终路径。

本设计统一修订既有 spec、plan 和学习文档中的管理接口路径约定：

- 共享身份接口继续使用 `/api/v1`；
- Admin 专属管理接口使用 `/api/admin/v1`；
- 历史文档不逐份重写，由本文件覆盖其中冲突的管理接口路径；
- 当前仍待执行的 spec 和 plan 必须同步到新路径，不能继续生成新的旧路径代码。

## 2. 已确认决策

- `/api/v1` 只承载未来多个前端共同使用的身份认证和当前平台权限快照；
- `/api/admin/v1` 只承载 Admin 管理能力；
- `/api/admin/v1` 必须强制要求 `X-Auth-Platform: admin`，不能只靠路径命名表达边界；
- 合法但非 `admin` 的平台请求 Admin API 时返回 HTTP `403` 和现有 `code=10003`；
- Header 缺失、重复或格式非法时仍由现有客户端 Middleware 返回 HTTP `400`；
- 原 `/api/v1` 下的管理接口直接失效，不提供别名、重定向或双路径兼容；
- 共享认证、Cookie、Token、会话和 `/access` 的现有协议保持不变；
- 历史操作日志不回填或改写，迁移完成后的新日志记录新路由；
- 本次不修改数据库，不引入新的平台表、菜单字段或通用路由注册器。

## 3. 接口归属

### 3.1 共享身份接口

以下接口保留在 `/api/v1`：

```text
GET  /api/v1/auth/policy
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
GET  /api/v1/auth/me
GET  /api/v1/access
```

这些接口的结果都由 `X-Auth-Platform` 指定的当前平台决定。认证 Token 的平台必须继续与请求
平台一致；`GET /api/v1/access` 后续再按 RBAC 平台隔离 spec 完成菜单和权限过滤。

`POST /api/v1/auth/register` 保留不代表 Admin 允许注册。内置 `admin` 平台仍由认证基线固定
`allowRegister=0`，未来业务平台是否开放注册由自己的 Policy 决定。

### 3.2 Admin 管理接口

以下接口迁移到 `/api/admin/v1`：

```text
GET    /api/admin/v1/auth-platforms
GET    /api/admin/v1/auth-platforms/deployment
POST   /api/admin/v1/auth-platforms
PUT    /api/admin/v1/auth-platforms/:id
PATCH  /api/admin/v1/auth-platforms/:id/status
DELETE /api/admin/v1/auth-platforms/:id

GET    /api/admin/v1/menus
POST   /api/admin/v1/menus
PUT    /api/admin/v1/menus/:id
PATCH  /api/admin/v1/menus/:id/status
DELETE /api/admin/v1/menus/:id

GET    /api/admin/v1/roles
POST   /api/admin/v1/roles
PUT    /api/admin/v1/roles/:id
PATCH  /api/admin/v1/roles/:id/status
PATCH  /api/admin/v1/roles/:id/default
DELETE /api/admin/v1/roles/:id
GET    /api/admin/v1/roles/:id/permissions
PUT    /api/admin/v1/roles/:id/permissions

GET    /api/admin/v1/users
GET    /api/admin/v1/users/role-options
PUT    /api/admin/v1/users/:id
PATCH  /api/admin/v1/users/:id/status
DELETE /api/admin/v1/users/:id
GET    /api/admin/v1/users/:id/roles
PUT    /api/admin/v1/users/:id/roles

GET    /api/admin/v1/sessions
GET    /api/admin/v1/sessions/stats
DELETE /api/admin/v1/sessions/:id
DELETE /api/admin/v1/sessions

GET  /api/admin/v1/operation-logs
POST /api/admin/v1/example-tasks
```

认证平台配置、用户治理、角色授权、菜单配置、跨平台会话治理和操作日志查询都是管理动作，
不能因为底层数据会被多个平台使用而暴露为共享客户端 API。`example-tasks` 是当前 Admin 工程的
任务链路示例，也进入 Admin 命名空间；本基线不借机删除它。

### 3.3 健康检查

基础设施探针不属于版本化业务 API，保持现状：

```text
GET /health
GET /ready
```

## 4. 后端路由设计

`server/cmd/api/main.go` 明确创建两个 Router Group：

```text
sharedRoutes = /api/v1
adminRoutes  = /api/admin/v1
```

两个 Group 都先执行现有 `authclient.Require()`，保证平台、设备、客户端 IP 和 User-Agent 按同一
规则进入请求 Context。`adminRoutes` 随后执行一个职责单一的平台约束 Middleware，只允许当前
`authclient.Client.Platform == "admin"` 的请求继续。

执行顺序固定为：

```text
共享接口：全局 middleware -> authclient.Require -> route middleware -> handler
Admin 接口：全局 middleware -> authclient.Require -> require admin platform -> auth -> permission -> handler
```

平台约束属于请求级关注点，放在 `authclient` Middleware，不下沉到 Handler 或 Service。它从
现有 Gin Context 读取已校验的 `authclient.Client`，不重新解析 Header，不访问 PostgreSQL、Redis
或认证平台 Policy。

平台约束失败时使用现有通用 Forbidden envelope：

```json
{
  "code": 10003,
  "data": null,
  "message": "无权执行该操作"
}
```

英文响应继续由现有 i18n 输出 `Permission denied`。不为命名空间新增业务错误码。

模块内的 `RegisterRoutes` 继续只注册资源相对路径；模块不硬编码 `/api/admin/v1`。命名空间归属
由组合根 `server/cmd/api/main.go` 决定，避免每个业务模块复制前缀。

## 5. 前端请求设计

前端数据流保持：

```text
view -> api/<module>.ts -> utils/request.ts -> Go API
```

`web/src/api/auth.ts` 和 `web/src/api/access.ts` 继续请求 `/api/v1`。以下管理 API 文件改用
`/api/admin/v1`：

```text
auth-platform.ts
menu.ts
role.ts
user.ts
session.ts
operation-log.ts
taskDemo.ts
```

`utils/request.ts` 中登录、刷新、退出和公开认证接口仍是共享路径；自动刷新仍请求
`POST /api/v1/auth/refresh`。Admin Web 继续固定发送 `X-Auth-Platform: admin`，不根据 URL 猜测
或动态切换平台。

不增加前端 API 前缀 Factory、运行时注册器或兼容重写拦截器。每个业务 API 保留明确 URL，便于
代码搜索、协议审查和测试精确断言。

## 6. Cookie、认证与 CORS

Refresh Cookie 继续服务共享认证端点，其 `Path` 保持 `/api/v1/auth`。本迁移不能把 Cookie Path
改为 `/api/admin/v1`，否则共享刷新和退出接口将收不到 Cookie。

Access Token、Refresh Token、Session 的 `platform` 语义不变。Admin API 先校验请求平台为
`admin`，随后现有认证 Middleware 继续校验 Token 平台与请求平台一致。两层约束分别防止错误
客户端进入 Admin 路由和错误平台 Token 冒用。

CORS 仍使用当前精确 Origin、credentials 和允许 Header 配置；路径迁移不新增 Origin，不允许
`*`，也不改变 `X-Auth-Platform`、`X-Device-ID` 要求。

## 7. 操作日志

操作日志规则中的共享认证路由保持 `/api/v1/auth/...`，所有管理动作规则改为
`/api/admin/v1/...`。Middleware 必须用迁移后的 Gin route pattern 匹配规则，继续保持原有
request/response 捕获和脱敏行为。

`operation_log.route` 是不可变历史事实：

- 迁移前记录的 `/api/v1/users/:id` 等值保留；
- 迁移后新请求记录 `/api/admin/v1/users/:id` 等新值；
- 不执行数据库回填，不把历史请求伪装成访问过新路径；
- 管理页面必须能同时展示两种历史 route 字符串。

## 8. 迁移与兼容

这是有意的破坏性 HTTP 协议变更。后端路由、前端调用、操作日志规则和测试必须作为同一个发布
单元切换。迁移完成后：

- `/api/v1/users`、`/api/v1/roles`、`/api/v1/menus` 等旧管理路径返回 `404`；
- 不注册临时别名，不返回 `301`、`302`、`307` 或 `308`；
- 不在 Axios 拦截器中把旧路径静默改写为新路径；
- 不保留两套操作日志规则；
- 不修改共享身份端点，也不使现有登录会话失效。

当前项目尚未发布需要兼容的外部 Admin API，直接切换能避免以后承担永久双协议成本。

## 9. 文档治理与实施顺序

本设计生效后，文档处理规则为：

1. 新增独立的 API 命名空间实施计划，并先执行；
2. 同步修正当前待执行的 Admin 认证 spec 和 plan 中的管理路径；
3. 更新仍作为开发入口使用的 `docs/learning/admin-architecture-and-crud.md`；
4. 既有已实施历史 spec 和 plan 保留原文，由本设计声明覆盖冲突路径；
5. 后续所有 Admin 模块从第一天使用 `/api/admin/v1`；
6. API 命名空间完整验证通过后，才执行 Admin 邮箱登录与账号资料基线计划。

历史文档保留旧路径不是兼容承诺，也不能作为新增代码继续使用旧路径的依据。

## 10. 不包含

- 邮箱登录字段、手机号、Admin 注册开关等认证基线业务实现；
- `rbac_menu.platform`、角色平台归属或 `/access` 平台过滤；
- COS、邮件、短信、支付、AI 或 WebSocket；
- OpenAPI、公共 SDK 或业务前端项目；
- 数据库 Schema、数据回填或缓存键迁移；
- 删除现有成熟功能或重构模块分层。

## 11. 测试策略

行为变化遵循失败测试、最小实现、通过、再重构。

### 11.1 后端

至少覆盖：

- 组合根只在 `/api/admin/v1` 注册全部管理接口；
- 旧 `/api/v1` 管理路径为 `404`，共享身份路径仍存在；
- `X-Auth-Platform: admin` 可以进入 Admin 路由的后续 Middleware；
- 合法非 Admin 平台访问 Admin 路由返回 `403/code=10003`，Handler 不执行；
- 缺失、重复、非法平台或设备 Header 仍返回 `400/code=10001`；
- Admin Token 的认证和权限校验仍按原顺序执行；
- 操作日志共享认证规则保持旧路径，管理动作规则全部使用新路径；
- 新管理请求写入新 route，历史日志读取不受影响。

模块 Handler 测试应使用模块真实归属的新完整路径，避免测试继续把错误前缀固化成事实。

### 11.2 前端

至少覆盖：

- Auth 和 Access API 仍精确请求 `/api/v1`；
- 所有管理 API 精确请求 `/api/admin/v1`；
- 自动 Refresh 继续精确请求 `/api/v1/auth/refresh`；
- 所有请求继续携带 `X-Auth-Platform: admin` 和合法设备 ID；
- 页面测试不残留对旧管理路径的断言；
- 生产构建不依赖路径兼容逻辑。

### 11.3 文档与静态审计

定向搜索必须区分允许的共享路径、不可改写的历史文档和运行时代码。运行时代码及当前待执行
计划中不得残留以下旧管理路径：

```text
/api/v1/auth-platforms
/api/v1/menus
/api/v1/roles
/api/v1/users
/api/v1/sessions
/api/v1/operation-logs
/api/v1/example-tasks
```

## 12. 验证命令

实施完成后运行：

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...

cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build

cd D:\admin
git diff --check
git status --short
```

不使用 Playwright；界面没有视觉变化，用户可按现有方式人工验收。

## 13. 验收标准

1. 共享认证和 `/access` 只使用 `/api/v1`；
2. 当前所有 Admin 管理接口只使用 `/api/admin/v1`；
3. Admin API 强制要求请求平台为 `admin`，其他平台无法进入 Handler；
4. 旧管理路径没有别名、重定向或前端静默兼容；
5. Refresh Cookie Path 和自动刷新端点保持 `/api/v1/auth`；
6. 新操作日志记录新路由，历史日志不回填且仍可读取；
7. 当前认证 spec、plan 和开发学习文档不再指导新增旧管理路径；
8. Handler、Service、Repository、DTO、数据库和权限码没有被命名空间迁移无关修改；
9. 后端与前端完整测试和构建通过；
10. 命名空间基线完成后才开始邮箱登录与账号资料基线实施。
