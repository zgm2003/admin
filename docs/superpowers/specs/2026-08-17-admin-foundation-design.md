# Admin 基础项目设计说明

## 1. 项目目标

从零搭建一个简单、清晰、适合长期维护的单体 Admin 系统。核心目标不是追求架构模式，而是保证数据流线性、直接、容易追踪：每个请求都能顺着明确的文件和调用关系找到问题发生的位置。后续用 AI 开发模块时不得为满足形式而虚构 Repository、接口或数据库访问。

```text
admin/
├── web/       # Vue 前端
├── server/    # Go 后端
└── docs/      # 项目文档
```

## 2. 第一阶段范围

第一阶段只把基础架构跑起来，包含：

- 可启动的 Vue 3 管理后台外壳；
- 可启动的 Go HTTP 服务；
- 可启动的 Go Worker 服务；
- PostgreSQL、Redis 配置和 GORM 自动建表能力；
- Asynq 队列及一个最小示例任务；
- 统一响应、业务错误、参数校验和分页结构；
- 健康检查，以及 PostgreSQL、Redis（同时覆盖 Asynq 队列存储连通性）就绪检查接口；
- 本地环境配置模板、启动命令和基础测试。

第一阶段不堆用户、角色、菜单、操作日志和 IoT 业务。基础架构确认稳定后，再用一个完整业务模块验证整条链路。

## 3. 技术栈

前端：

- Vue 3、TypeScript、Vite；
- ECharts、Element Plus、Pinia、Axios、SCSS、Vue Router；
- Vitest、Vue Test Utils；
- pnpm。

前端从最小 Vite 项目开始，不继承 Vben、v3-admin-vite 等大型模板的框架代码，最多只参考视觉和交互。

后端：

- Go、Gin、GORM、PostgreSQL；
- Redis、go-redis、Asynq；
- Gin binding 和 go-playground/validator 做参数校验；
- Go 标准库 `log/slog` 输出结构化日志；
- Go 标准测试和少量 PostgreSQL 集成测试。

暂不引入微服务、动态路由注册器、代码生成器、依赖注入框架等额外复杂度。

## 4. 后端架构

每个普通业务模块使用一致的目录：

```text
server/internal/module/<模块名>/
├── route.go
├── handler.go
├── request.go
├── response.go
├── service.go
├── repository.go
├── model.go
├── task.go             # 仅在模块拥有异步任务时存在
├── handler_test.go
├── service_test.go
└── repository_test.go
```

各层职责：

- `route.go`：路由地址、中间件和 Handler 绑定；
- `handler.go`：接收并校验请求、调用 Service、输出响应；
- `request.go`：请求 DTO 和基础格式校验标签；
- `response.go`：响应 DTO；
- `service.go`：业务规则、跨字段校验和事务意图；
- `repository.go`：数据库查询、写入、锁和事务实现；
- `model.go`：GORM 数据表映射。
- `task.go`：模块自己的 Asynq 任务类型、Payload、Handler 和注册函数。

Handler 不直接操作 GORM，Service 不依赖 Gin，Repository 不处理 HTTP 状态码和用户提示。不得为了让健康检查、Redis、队列或对象存储符合数据库链路而创建虚假的 Repository。

项目保留 Handler、Service、Repository 等具有现实职责的分层，但不采用 DDD、Clean Architecture 或端口适配器式抽象，也不创建 Manager、Factory、Base 层或为了未来替换而存在的接口。每一层必须对应当前真实对象和行为，不能只有架构意义。

每种请求使用明确的单向调用链。

普通数据库请求：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

对象存储请求：

```text
handler -> file service -> storage -> COS/OSS/S3
```

认证和权限检查：

```text
auth/permission middleware -> auth/permission service -> repository 或 redis
```

首期尽力投递：

```text
handler -> service
1. service -> repository -> PostgreSQL（创建任务记录）
2. repository 返回 service
3. service -> queue -> Redis/Asynq
```

可靠任务投递的请求阶段：

```text
handler -> service -> repository -> PostgreSQL（业务数据和 Outbox 在同一事务提交）
```

可靠任务投递的投递阶段：

```text
dispatcher -> outbox repository -> PostgreSQL（读取待投递记录）
outbox repository 返回 dispatcher
dispatcher -> queue -> Redis/Asynq
dispatcher -> outbox repository -> PostgreSQL（标记已投递）
```

异步任务消费：

```text
task handler -> service -> repository -> model -> PostgreSQL
```

真正出现微信、短信、支付等第三方业务接口后：

```text
handler 或 task handler -> service -> integration/<具体服务>/client -> 第三方系统
```

跨模块调用只能是当前模块 `Service -> 目标模块 Service`，Handler 和 Repository 不跨模块调用；所有依赖必须保持单向，禁止循环依赖。

各层职责固定：Service 负责业务规则，Repository 只负责 PostgreSQL，Storage 只负责对象存储 SDK，Redis 目录只负责 Redis 连接和基础操作，Queue 只负责 Asynq，Integration Client 只负责对应第三方协议和 SDK。Handler 和 Middleware 不直接调用 GORM、Redis SDK、对象存储 SDK 或第三方 SDK。

Service 决定一个业务操作的事务范围，数据库层负责开启、提交和回滚事务，同一事务内的 Repository 必须使用同一个事务句柄。第一个真实多表事务出现时才增加简单的 `WithTx` 能力；Service 不直接编写 SQL 或执行 GORM 查询，不引入通用 Unit of Work 框架。

不提前设计 `BaseService`、`BaseRepository`、Manager、Factory、运行时注册器或万能实现层。只有当前确实存在多个实现，或者测试中存在真实替换边界时，才提取覆盖当前需求的最小接口。

## 5. 后端公共目录

```text
server/
├── cmd/api/                  # HTTP API 进程入口
├── cmd/worker/               # Asynq Worker 和长驻 Runner 进程入口
├── internal/config/          # 配置读取和校验
├── internal/database/        # 数据库连接和 AutoMigrate
├── internal/redis/           # Redis 客户端、连接和基础操作
├── internal/queue/           # 仅封装 Asynq 客户端、服务端和配置
├── internal/runner/          # 真正长驻任务的生命周期管理
├── internal/storage/         # 当前使用的 COS、OSS 或 S3 客户端
├── internal/middleware/      # 请求日志、恢复、认证等中间件
├── internal/module/          # 业务模块
├── internal/shared/
│   ├── apperror/             # 统一业务错误
│   ├── pagination/           # 分页结构
│   ├── response/             # 统一 HTTP 响应
│   └── validate/             # 公共校验器
```

参数校验分三层：

- Handler：必填、长度、范围、数据格式；
- Service：跨字段规则、状态流转、业务约束；
- PostgreSQL：唯一键、外键、非空和 check 约束。

开发阶段只由 API 进程启动时执行 GORM `AutoMigrate`，适用于建表、新增字段和索引；Worker 连接 PostgreSQL，但不执行 `AutoMigrate`，避免 API 和 Worker 同时迁移数据库。删除字段、修改字段类型和数据回填不交给 `AutoMigrate` 自动处理。项目进入正式生产发布阶段后，再引入版本化 SQL migration。

## 6. 异步任务和长驻进程

项目保持一个代码仓库和一套业务模块，但提供两个独立进程：

```text
API：    接收 HTTP 请求并向 Asynq 投递任务
Worker：消费 Asynq 任务，并管理必要的长驻 Runner
```

首期尽力投递：

```text
handler -> service
1. service -> repository -> PostgreSQL（创建任务记录）
2. repository 返回 service
3. service -> queue -> Redis/Asynq
```

可靠任务投递的请求阶段：

```text
handler -> service -> repository -> PostgreSQL（业务数据和 Outbox 在同一事务提交）
```

可靠任务投递的投递阶段：

```text
dispatcher -> outbox repository -> PostgreSQL（读取待投递记录）
outbox repository 返回 dispatcher
dispatcher -> queue -> Redis/Asynq
dispatcher -> outbox repository -> PostgreSQL（标记已投递）
```

消费阶段：

```text
task handler -> service -> repository -> model -> PostgreSQL
```

Service 负责首期任务投递的顺序编排，Dispatcher 负责 Outbox 投递编排。Repository 和 Outbox Repository 只操作 PostgreSQL，永远不能调用 Queue；Queue 只负责 Asynq。不为这套流程增加 Manager、Factory、通用 Adapter 或 Unit of Work。

Asynq 负责最终会结束的后台工作，例如导入导出、通知发送、AI 处理、文件处理、设备批量指令、重试任务和定时任务。任务需要稳定的任务类型、可序列化参数、超时、有限重试和幂等规则。任务进度、业务状态和最终结果记录在 PostgreSQL，Redis 不作为业务事实来源。

永不主动结束的设备监听、消息订阅和连接维护不包装成 Asynq Task，而是放入 Worker 的 `runner`，由统一的 `context`、启动、健康检查和优雅关闭机制管理。耗时很长但能够结束的工作优先拆分阶段并保存进度，使其可以取消、重试或断点恢复。

首期只搭建队列基础能力和一个最小示例任务，不提前实现完整的任务管理后台。

API `/ready` 检查 PostgreSQL 和 Redis；Redis 连通同时证明 Asynq 队列存储可达，但不代表 Worker 进程一定正在消费。首期不为此增加 Worker 心跳表或额外探测协议，部署层直接检查 Worker 进程；出现业务上的消费存活监控需求后再增加心跳。

`internal/queue` 不导入任何业务模块，也不拥有业务任务注册。任务定义和 Handler 放在所属模块的 `task.go`，`cmd/worker` 作为组合入口导入 Queue 和各任务模块并完成注册，依赖方向固定为 `module -> queue`、`cmd/worker -> queue + module`，禁止出现 `queue -> module`。

首期示例任务采用尽力投递，不对 PostgreSQL 与 Redis/Asynq 提供原子一致性承诺。第一个要求可靠投递的正式业务任务必须使用事务 Outbox：业务数据与待投递记录在同一 PostgreSQL 事务内提交，由 Dispatcher 扫描并投递 Asynq，成功后标记已投递。该机制按真实业务需求引入，首期不提前实现。Outbox 和 Asynq 都可能产生重复投递，因此正式任务 Handler 必须幂等。

HTTP Handler 从 `Request.Context()` 获取 `context.Context`；Service 和 Repository 的每个操作都显式接收并向下传递该 Context；GORM 查询使用 `WithContext`，Redis 和对象存储调用也传入同一 Context。Asynq Handler 传递任务 Context，Runner 使用进程统一的父 Context。禁止将 `*gin.Context` 传入 Service 或 Repository。

## 7. 请求、响应和错误流转

成功响应：

```json
{
  "code": 0,
  "data": {},
  "message": "ok"
}
```

失败响应使用正确的 HTTP 状态码和稳定的业务错误码。响应字段固定为 `code`、`data`、`message`，不得同时兼容 `msg` 等其他名称：

```json
{
  "code": 10001,
  "data": null,
  "message": "请求参数错误"
}
```

错误按调用链原路返回：

```text
PostgreSQL/GORM 错误
-> Repository 识别数据库错误
-> Service 转换为业务错误
-> Handler 输出统一响应
-> Axios 识别 HTTP 状态和业务错误码
-> 页面展示已经确认的提示信息
```

日志记录 request ID、请求方法、路由、状态码和耗时，不记录密码、Token 和完整请求体。

项目禁止静默兜底：必填配置缺失时启动失败，必填请求字段缺失时返回 400，接口响应结构不正确时前端抛出协议错误，依赖不可用和任务参数不完整时明确失败。禁止捕获错误后返回空数组、同时兼容多套响应结构、必填字段缺失时猜测默认值，以及 Redis 失败后未经设计地切换数据来源。

只有 DTO 明确声明为可选、且产品已经规定展示方式的字段，前端才可以使用 `??` 显示占位内容。不得用可选链和默认值掩盖必填字段或接口结构错误。

## 8. 登录和权限方向

基础架构之后依次实现登录/退出、用户、角色、菜单、按钮权限和操作日志。权限关系使用 PostgreSQL 关系表保存，后端是最终权限边界，前端权限只控制展示。第一版使用五张核心表：`sys_user`、`sys_role`、`sys_menu`、`sys_user_role`、`sys_role_menu`。

认证采用 Access Token + Refresh Token：Access Token 使用短有效期 JWT，通过 `Authorization: Bearer <token>` 发送；Refresh Token 使用较长有效期并放入 Cookie。Redis 保存会话、Refresh Token 状态和 Token 版本，刷新时轮换 Refresh Token；退出登录、封禁用户和修改密码时使对应会话失效。JWT 只携带用户 ID、会话 ID、Token 版本和有效期等必要声明，不写入完整角色和权限集合。

Refresh Token Cookie 固定使用 `HttpOnly=true`、`SameSite=Lax`、`Path=/api/v1/auth` 和 host-only Domain；生产环境必须设置 `Secure=true`，只有明确的本地 HTTP 开发环境可以关闭 Secure。Cookie 有效期与 Refresh Token 一致。刷新和退出等使用 Cookie 的写操作必须校验精确 Origin，CORS 只允许配置中的前端 Origin 并显式开启 credentials，禁止 `*`；HttpOnly 不能替代 CSRF 防护。

JWT 使用 `github.com/golang-jwt/jwt/v5`，认证 Service 和 Gin Middleware 由项目以少量明确代码实现，不引入封装完整登录流程的框架。

第一版 RBAC 不使用 Casbin。菜单用类型区分目录、菜单和按钮，按钮保存 `system:user:list` 形式的权限码。路由显式声明所需权限，权限中间件查询或缓存当前用户权限集合。只有未来出现多租户、资源级授权或复杂动态策略时，才评估在权限检查边界内部替换为 Casbin。

```go
users.GET("", permission.Require("system:user:list"), handler.List)
```

## 9. RESTful API 和分页协议

业务接口必须采用 RESTful API，以资源名和 HTTP 方法表达行为：

```text
GET    /api/v1/system/users
GET    /api/v1/system/users/:id
POST   /api/v1/system/users
PUT    /api/v1/system/users/:id
DELETE /api/v1/system/users/:id
```

URL 使用小写复数资源名和必要的短横线，不使用 `/getUserList`、`/createUser` 等动词式路径。登录、刷新 Token 等无法自然表达为 CRUD 的操作使用明确资源语义，例如 `POST /api/v1/auth/login` 和 `POST /api/v1/auth/refresh`。

所有分页列表接口必须返回唯一格式，不允许混用 `rows`、`records` 或直接数组：

```json
{
  "code": 0,
  "data": {
    "list": [],
    "total": 100,
    "page": 1,
    "pageSize": 20
  },
  "message": "ok"
}
```

后端公共分页泛型只定义一次，放在 `internal/shared/pagination`：

```go
type Request struct {
	Page     int `form:"page" binding:"required,min=1"`
	PageSize int `form:"pageSize" binding:"required,min=1,max=100"`
}

type Result[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}
```

前端对应的公共分页泛型也只定义一次，放在 `src/types/pagination.ts`：

```ts
export interface PageRequest {
  page: number
  pageSize: number
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  pageSize: number
}
```

两端关系固定为 `pagination.Result<UserListItem>` 经 JSON 对应 `PageResult<UserListItem>`。分页请求中的 `page` 和 `pageSize` 是必填字段，缺失或越界时返回 400，不自动修改为默认值。

公共分页 DTO 只统一分页外壳。用户、角色、日志等具体列表行仍使用各模块自己的 DTO，不设计万能 `TableRowDTO`。后端使用 `pagination` 命名而不使用 `TableDTO`，因为分页数据也可能被列表或卡片消费，后端不能依赖前端 UI 形态。

前端公共 Table 组件只接收固定分页数据、列配置、加载状态和事件，不自行请求接口，不猜测字段，不承载页面业务。DTO 的前端对应物是 TypeScript `interface` 或 `type`；`enum` 只表达状态、类型等有限且稳定的取值，不能用来描述 DTO 结构。

## 10. 前端架构

```text
web/src/
├── api/             # 按业务模块封装接口
├── assets/          # 图片、字体等静态资源
├── components/      # 真正跨页面复用的组件
├── enums/           # 公共枚举
├── hooks/           # 可复用组合式逻辑
├── layout/          # 后台整体布局
├── locales/         # 国际化资源，先保留基础能力
├── router/          # 路由定义
├── store/           # Pinia 状态
├── styles/          # SCSS 变量、重置和全局样式
├── types/           # 公共 TypeScript 类型
├── utils/           # Axios 等通用工具
├── views/           # 页面及页面私有组件
├── App.vue
├── main.ts
└── permission.ts    # 路由守卫编排
```

固定请求链：

```text
view -> api/<模块名>.ts -> utils/request.ts -> Go API
```

`permission.ts` 只负责登录状态、当前用户与菜单加载、动态路由安装和无权访问跳转，不存放接口地址、角色判断分支和页面业务逻辑。

页面私有组件放在对应 `views` 目录内。只有产生真实复用后，代码才提升到全局 `components`、`hooks`、`types`、`enums` 或 `utils`。

ECharts 通过页面或图表组件按需引入，不全局一次性注册所有图表能力。

`enums` 只保存有限且稳定的枚举值，不存放 DTO。接口 DTO 放在对应 `api` 模块或公共 `types` 中。

## 11. 公共代码和公共服务边界

`internal/shared` 只存放无业务归属、被多个模块稳定复用的错误、分页、响应和校验结构，不创建含义模糊的 `common` 或 `shared/services` 杂项目录。

可管理的字典是独立的 `internal/module/dictionary` 业务模块；仅存在于代码中的固定状态使用枚举。上传能力拆分为 `internal/storage` 中当前真实使用的 COS、OSS 或 S3 客户端，以及 `internal/module/file` 中的上传策略、临时凭证、文件记录和业务校验。其他模块存在后端上传需求时，通过文件模块的 Service 使用该能力，不自行创建对象存储客户端。

不创建通用对象存储接口、万能实现层或统一第三方客户端。具体实现使用容易搜索的现实名称，例如 `storage.COS`、`storage.OSS`、`storage.S3`、`redis.Client`、`sms.Client` 和 `wechat.Client`。只有当前确实需要同时支持多个实现时，才提取最小公共接口。

`internal/integration` 不提前创建。真正接入微信、短信、支付等第三方业务系统时，再按 `internal/integration/<具体服务>/client` 建立对应目录；如果当前只有对象存储，就只创建 `internal/storage`。

问题定位必须与目录职责一致：数据库问题查 Repository，业务规则问题查 Service，请求参数问题查 Handler，对象存储问题查 Storage，Redis 问题查 Redis，队列问题查 Queue 或 Task Handler，第三方接口问题查对应 Integration Client。

## 12. 命名规范

- JSON 字段、TypeScript 变量和函数使用小驼峰，如 `userId`、`pageSize`；
- TypeScript 类型和 Vue 组件使用大驼峰，如 `UserInfo`、`UserTable.vue`；
- Go 导出标识符使用大驼峰，私有标识符使用小驼峰；Go 缩写写作 `UserID`、`HTTPServer`、`APIClient`；
- PostgreSQL 表名和字段名使用小写蛇形，如 `sys_user`、`user_id`、`created_at`；
- URL 使用小写复数资源名和短横线；
- 权限码使用冒号分段，如 `system:user:list`。

## 13. 本地启动方式

环境变量坚持最少且必需。后端只保留四项：

```text
HTTP_ADDR       # Go API 监听地址
POSTGRES_DSN    # PostgreSQL 完整连接串
REDIS_URL       # Redis 地址、密码和 DB 合并后的 URL
CORS_ORIGIN     # 唯一允许访问 API 的前端 Origin
```

前端只保留一项：

```text
VITE_API_BASE_URL # 浏览器直接访问的 Go API 地址
```

`server/.env.example`、`web/.env.example` 和本地对应的 `.env` 都为每个变量提供中文注释。`.env.example` 提交到 Git，`.env` 只供本机使用并被 Git 忽略。Go 启动时读取本地 `.env`，也接受部署环境直接注入同名变量；无论变量来自哪里，必填项缺失或格式错误都立即失败，不增加多层配置文件和隐式默认值。

API 和 Worker 共用这一份后端 `.env`，但各进程只读取并校验自己当前使用的配置。Worker 首期读取 `POSTGRES_DSN` 和 `REDIS_URL`，不读取 `HTTP_ADDR` 和 `CORS_ORIGIN`；Worker 需要连接 PostgreSQL 更新任务状态，但不执行 `AutoMigrate`。

前端不配置 Vite Proxy，项目也不依赖 Nginx 或其他反向代理转发 API。Axios 使用 `VITE_API_BASE_URL` 直接请求 Go API；后端通过单一 Gin CORS 中间件精确允许 `CORS_ORIGIN`、开启 credentials，并拒绝其他 Origin，禁止使用通配符 `*`。

本地开发端口固定为前端 `16300`、Go API `16301`。Vite 开启 `strictPort`，端口被占用时明确失败，不自动切换到其他端口。

```text
PostgreSQL：本机服务或数据库容器
Redis：     本机服务或 Redis 容器
API：       go run ./cmd/api
Worker：    go run ./cmd/worker
前端：      pnpm dev
```

环境变量缺失时明确报错退出，不增加服务发现、网关和分布式配置。

## 14. 测试边界

- Handler 测试：参数绑定、HTTP 状态码和统一响应；
- Service 测试：业务规则和错误转换；
- Repository 集成测试：PostgreSQL 查询和数据库约束；
- Queue 测试：任务编解码、注册、重试和幂等边界；
- Worker 测试：任务消费和长驻 Runner 的停止行为；
- 前端请求测试：响应解包和错误处理；
- Router/permission 测试：受保护页面的跳转；
- 冒烟检查：前端、API 和 Worker 能启动，PostgreSQL、Redis 及 Asynq 实际投递消费正常。

测试跟随模块放置。只有真实跨模块的关键流程才增加端到端测试。

## 15. 反过度设计和 AI 开发约束

后续 AI 生成的业务模块必须遵守固定目录和调用链。新增抽象必须有当前代码中的真实重复或替换需求，不能只因为“以后可能用到”。

永远不为尚未出现的需求预留表、字段、状态、接口和扩展层。每张表、每个字段、每个索引、每个 DTO 属性都必须能指出当前功能中的读取或写入位置；无法指出当前用途的内容不进入实现。数据库不保留“以后可能需要”的备用字段，新增真实需求时再通过 `AutoMigrate` 或明确的数据变更增加。

RESTful API、统一响应、分页 DTO、JSON 命名和错误语义属于固定契约。前后端都不得为了兼容自身错误而添加另一套字段名、响应格式或静默转换。

每个模块都必须能快速回答：

1. 哪个 Route 收到请求？
2. 哪个 Handler 绑定参数？
3. 哪个 Service 处理业务规则？
4. 哪个 Repository 访问数据库？
5. 哪个 Model 和数据表保存数据？
6. 错误在哪一层被识别、转换和返回？

如果这些问题必须依靠隐藏钩子、运行时注册器或生成器才能回答，就说明实现偏离了本设计。
