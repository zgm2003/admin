# Admin Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `D:\admin` 建立可本地运行、可测试的 Vue Admin 外壳、Go API、Go Worker、PostgreSQL、Redis 和 Asynq 基础项目。

**Architecture:** 普通请求固定经过 `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`。Redis、Queue 等按真实技术名称直接组织；Service 负责同步编排，业务任务定义归属业务模块，`internal/queue` 不导入业务模块。API 负责开发期 AutoMigrate，Worker 连接 PostgreSQL 但不迁移。

**Tech Stack:** Go 1.26、Gin、GORM、PostgreSQL、go-redis、Asynq、Vue 3、TypeScript、Vite、Element Plus、Pinia、Axios、ECharts、SCSS、Vitest、Vue Test Utils、pnpm。

---

## 执行约束

- 不使用 Docker 或 Docker Compose；只使用用户已安装的本机 PostgreSQL 和 Redis。
- 不运行 `git init`、`git add` 或 `git commit`；只准备 `.gitignore`，Git 仓库由用户创建。
- 每项行为先写失败测试并确认失败，再写最小实现。
- 禁止静默兜底、替代字段、默认地址、内存队列和伪造健康状态。
- 不创建 `platform`、`infra`、通用 Adapter、Manager、Factory、BaseService、BaseRepository、DI 容器或动态注册器。
- 第一阶段不创建登录、RBAC、文件上传、Storage、Integration、Outbox 或通用 Table。

## 文件结构

```text
admin/
├── .gitignore
├── AGENTS.md
├── README.md
├── docs/
├── server/
│   ├── .env.example
│   ├── .env                         # 本机文件，被 Git 忽略
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/
│   │   ├── api/main.go
│   │   └── worker/main.go
│   └── internal/
│       ├── config/config.go
│       ├── database/postgres.go
│       ├── middleware/{access_log.go,cors.go,request_id.go}
│       ├── module/
│       │   ├── health/{route.go,handler.go,response.go,service.go}
│       │   └── taskdemo/{route.go,handler.go,request.go,response.go,service.go,repository.go,model.go,task.go}
│       ├── queue/{client.go,server.go}
│       ├── redis/redis.go
│       └── shared/
│           ├── apperror/error.go
│           ├── pagination/pagination.go
│           ├── response/response.go
│           └── validate/validate.go
└── web/
    ├── .env.example
    ├── .env                         # 本机文件，被 Git 忽略
    ├── package.json
    ├── pnpm-lock.yaml
    ├── vite.config.ts
    └── src/
        ├── api/{health.ts,taskDemo.ts}
        ├── layout/AdminLayout.vue
        ├── router/index.ts
        ├── store/index.ts
        ├── styles/{index.scss,variables.scss}
        ├── types/{http.ts,pagination.ts}
        ├── utils/request.ts
        ├── views/dashboard/{index.vue,components/ReadinessChart.vue}
        ├── App.vue
        ├── main.ts
        └── permission.ts
```

只创建有当前用途的文件。`assets`、`components`、`enums`、`hooks`、`locales`、`storage` 和 `integration` 没有实际代码前不创建空目录。

### Task 1: 工作区约束与最小项目

**Files:**
- Create: `.gitignore`
- Create: `AGENTS.md`
- Create: `README.md`
- Create: `server/go.mod`
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig*.json`
- Create: `web/index.html`

- [x] **Step 1: 建立仓库忽略规则**

`.gitignore` 必须包含：

```gitignore
.idea/
.vscode/
*.log
*.exe
*.out
.env
server/bin/
server/tmp/
web/node_modules/
web/dist/
web/coverage/
```

- [x] **Step 2: 固化 Agent 施工纪律**

`AGENTS.md` 必须要求所有 Agent 先读最终 spec，遵守线性调用链、命名规范和禁止兜底规则，并明确禁止 Docker、禁止主动初始化 Git、禁止创建未使用抽象。

- [x] **Step 3: 初始化 Go module 和依赖**

在 `server` 中创建 module `admin/server`，安装 Gin、Gin CORS、GORM PostgreSQL、go-redis、Asynq、godotenv 和 validator。运行：

```powershell
cd server
go mod tidy
```

Expected: 命令退出 0，`go.mod` 和 `go.sum` 存在。

- [x] **Step 4: 初始化最小 Vue 项目和依赖**

使用 pnpm 建立 Vue 3 + TypeScript + Vite 项目，安装 Vue Router、Pinia、Element Plus、Element Plus Icons、Axios、ECharts、SCSS、Vitest、Vue Test Utils 和 jsdom。不得添加 Vite proxy。

```powershell
cd web
pnpm install
```

Expected: 只生成 `pnpm-lock.yaml`，不存在 `package-lock.json`。

- [x] **Step 5: 验证干净基线**

```powershell
cd server
go test ./...
cd ..\web
pnpm vitest run --passWithNoTests
```

Expected: 两条命令退出 0；不执行任何 Git 命令。

### Task 2: 严格后端配置

**Files:**
- Create: `server/.env.example`
- Create: `server/.env`
- Create: `server/internal/config/config.go`
- Test: `server/internal/config/config_test.go`

- [x] **Step 1: 写失败测试**

测试以下公开 API：

```go
type API struct {
	HTTPAddr    string
	PostgresDSN string
	RedisURL    string
	CORSOrigin  string
}

type Worker struct {
	PostgresDSN string
	RedisURL    string
}

type LookupEnv func(string) (string, bool)

func LoadAPI(LookupEnv) (API, error)
func LoadWorker(LookupEnv) (Worker, error)
```

测试必须覆盖：API 四项齐全时成功；Worker 只读取 `POSTGRES_DSN` 和 `REDIS_URL`；每个缺失或空字符串都返回包含准确变量名的错误；非法 `HTTP_ADDR`、PostgreSQL DSN、Redis URL 和 CORS Origin 都失败；不生成默认值。

```powershell
cd server
go test ./internal/config -run TestLoad -v
```

Expected: FAIL，原因是 `LoadAPI`/`LoadWorker` 尚不存在。

- [x] **Step 2: 实现最小配置解析**

使用 `net.SplitHostPort` 校验监听地址、pgx 配置解析器校验 DSN、go-redis `ParseURL` 校验 Redis URL、`net/url` 校验单一 HTTP/HTTPS Origin。错误使用 `%w` 保留原因，不读取替代变量。

- [x] **Step 3: 添加环境模板**

`server/.env.example` 使用中文注释并包含：

```dotenv
HTTP_ADDR=:16301
POSTGRES_DSN=host=127.0.0.1 user=postgres password=postgres dbname=admin port=5432 sslmode=disable TimeZone=Asia/Shanghai
REDIS_URL=redis://127.0.0.1:6379/0
CORS_ORIGIN=http://localhost:16300
```

创建被忽略的 `server/.env` 供本机联调；优先使用用户现有环境值，若实际凭据不同则只修改 `.env`，不修改模板或代码。入口调用 `godotenv.Load()`；文件不存在不算错误，但所需变量缺失必须失败。

- [x] **Step 4: 验证绿色**

```powershell
cd server
go test ./internal/config -v
```

Expected: PASS。

### Task 3: 统一错误、响应、校验和分页

**Files:**
- Create: `server/internal/shared/apperror/error.go`
- Create: `server/internal/shared/response/response.go`
- Create: `server/internal/shared/pagination/pagination.go`
- Create: `server/internal/shared/validate/validate.go`
- Test: `server/internal/shared/response/response_test.go`
- Test: `server/internal/shared/pagination/pagination_test.go`
- Test: `server/internal/shared/validate/validate_test.go`

- [x] **Step 1: 写响应失败测试**

锁定唯一成功 JSON：

```json
{"code":0,"data":{"value":"ok"},"message":"ok"}
```

锁定参数错误 HTTP 400：

```json
{"code":10001,"data":null,"message":"请求参数错误"}
```

测试 JSON key 只能是 `code`、`data`、`message`，不存在 `msg`。

- [x] **Step 2: 写分页和绑定失败测试**

测试以下类型及 JSON 字段：

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

严格 JSON 绑定必须拒绝未知字段、空 body、尾随第二个 JSON 值和校验失败。

```powershell
cd server
go test ./internal/shared/... -v
```

Expected: FAIL，因为公共包尚未实现。

- [x] **Step 3: 实现最小公共契约**

`apperror.Error` 固定包含 `HTTPStatus`、`Code`、`Message`、`Cause`；定义参数错误 `10001`、未认证 `10002`、无权限 `10003`、未找到 `10004`、冲突 `10005`、依赖未就绪 `10006`、内部错误 `10000`。`response.OK` 与 `response.Fail` 只输出唯一 Envelope。`validate.BindJSON` 使用 `json.Decoder.DisallowUnknownFields()` 并验证 EOF。

- [x] **Step 4: 验证绿色**

```powershell
cd server
go test ./internal/shared/... -v
```

Expected: PASS。

### Task 4: PostgreSQL、Redis 与健康检查

**Files:**
- Create: `server/internal/database/postgres.go`
- Create: `server/internal/redis/redis.go`
- Create: `server/internal/module/health/route.go`
- Create: `server/internal/module/health/handler.go`
- Create: `server/internal/module/health/response.go`
- Create: `server/internal/module/health/service.go`
- Test: `server/internal/module/health/service_test.go`
- Test: `server/internal/module/health/handler_test.go`

- [x] **Step 1: 写健康检查失败测试**

固定行为：

```text
GET /health -> 200，只证明 API 存活，不访问 PostgreSQL/Redis
GET /ready  -> PostgreSQL 与 Redis 都成功时 200
GET /ready  -> 任一失败时 503，code=10006，data=null
```

Service 测试使用以下真实测试边界：

```go
type Probe interface {
	Ping(context.Context) error
}
```

Handler 必须向 Service 传递 `c.Request.Context()`。

```powershell
cd server
go test ./internal/module/health -v
```

Expected: FAIL，因为 health 模块不存在。

- [x] **Step 2: 实现数据库与 Redis 连接**

`database.Open(ctx, dsn)` 返回包含 `*gorm.DB` 和 `*sql.DB` 的具体连接，立即 `PingContext`；`database.AutoMigrate(ctx, db, models...)` 只执行 GORM AutoMigrate。`redis.Open(ctx, redisURL)` 解析唯一 URL、创建具体 client 并立即 Ping。两者不重试、不切换地址、不吞错误，并实现 Health `Probe` 所需的 `Ping(context.Context) error`。

- [x] **Step 3: 实现 Health Service、Handler 和 Route**

成功 readiness data：

```go
type Readiness struct {
	PostgreSQL string `json:"postgresql"`
	Redis      string `json:"redis"`
}
```

值固定为 `up`。失败只通过统一错误返回，不伪造部分成功数据。路由只注册 `/health`、`/ready`。

- [x] **Step 4: 验证绿色**

```powershell
cd server
go test ./internal/database ./internal/redis ./internal/module/health -v
```

Expected: PASS，取消的 Context 能到达 Probe。

### Task 5: PostgreSQL 任务记录与 Asynq

**Files:**
- Create: `server/internal/queue/client.go`
- Create: `server/internal/queue/server.go`
- Create: `server/internal/module/taskdemo/model.go`
- Create: `server/internal/module/taskdemo/repository.go`
- Create: `server/internal/module/taskdemo/service.go`
- Create: `server/internal/module/taskdemo/task.go`
- Create: `server/internal/module/taskdemo/request.go`
- Create: `server/internal/module/taskdemo/response.go`
- Create: `server/internal/module/taskdemo/handler.go`
- Create: `server/internal/module/taskdemo/route.go`
- Test: `server/internal/queue/client_test.go`
- Test: `server/internal/module/taskdemo/service_test.go`
- Test: `server/internal/module/taskdemo/task_test.go`
- Test: `server/internal/module/taskdemo/handler_test.go`

- [x] **Step 1: 写 Service 失败测试**

任务模型只保留当前流程真实使用的字段：

```go
type Task struct {
	ID        string
	Message   string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

状态固定为 `pending`、`running`、`completed`、`failed`。测试必须证明：Service 先创建 pending 记录，Repository 返回后才调用 Queue；Queue 失败时显式标记 failed 并返回错误；成功返回非空业务 task ID；不创建 Outbox。

- [x] **Step 2: 写 Task Handler 和 HTTP Handler 失败测试**

HTTP 只接受：

```json
{"message":"foundation-check"}
```

缺失、空白、超过 200 字符或未知字段返回 400；成功返回 202 和：

```json
{"code":0,"data":{"taskId":"..."},"message":"ok"}
```

Asynq payload 只包含：

```go
type Payload struct {
	TaskID string `json:"taskId"`
}
```

Task Handler 严格拒绝未知/缺失字段，调用 Service 将任务更新为 running，读取数据库中的 Message，记录日志，再更新 completed；错误返回给 Asynq 重试。

```powershell
cd server
go test ./internal/queue ./internal/module/taskdemo -v
```

Expected: FAIL，因为模块尚未实现。

- [x] **Step 3: 实现 Queue 封装**

`internal/queue` 只导入 Asynq 和配置解析代码，提供具体 Client 与 Server；Client 使用 `EnqueueContext`，不在 HTTP 请求内自行重试，不使用内存 fallback。Server 接受调用方传入的 `*asynq.ServeMux`。

- [x] **Step 4: 实现 TaskDemo 线性流程**

Repository 只使用 GORM；Service 依赖一个仅含 `Enqueue(context.Context, string) error` 的窄接口用于真实 Queue 边界和测试；`task.go` 负责 Asynq task 创建、严格解码和 Register。ID 使用 `crypto/rand` 生成 128-bit 十六进制字符串，不增加 UUID 依赖。

- [x] **Step 5: 验证绿色和依赖方向**

```powershell
cd server
go test ./internal/queue ./internal/module/taskdemo -v
go list -deps ./internal/queue | Select-String 'admin/server/internal/module'
```

Expected: 测试 PASS；第二条没有输出，证明 `queue -> module` 不存在。

### Task 6: Middleware、API 与 Worker 显式组装

**Files:**
- Create: `server/internal/middleware/request_id.go`
- Create: `server/internal/middleware/cors.go`
- Create: `server/internal/middleware/access_log.go`
- Test: `server/internal/middleware/request_id_test.go`
- Test: `server/internal/middleware/cors_test.go`
- Test: `server/internal/middleware/access_log_test.go`
- Create: `server/cmd/api/main.go`
- Create: `server/cmd/worker/main.go`
- Test: `server/cmd/api/main_test.go`

- [x] **Step 1: 写 Middleware 和路由失败测试**

测试每个响应都有 `X-Request-ID`；合法传入值被保留，缺失或非法值生成 32 位十六进制值。CORS 只允许配置中的精确 Origin，开启 credentials，不对其他 Origin 返回 allow-origin。结构化日志只包含 request ID、method、route、status、latency，不包含 Authorization 和 body。Router 精确注册 `/health`、`/ready`、`POST /api/v1/example-tasks`。

```powershell
cd server
go test ./internal/middleware ./cmd/api -v
```

Expected: FAIL，因为 Middleware 和组装函数不存在。

- [x] **Step 2: 实现 Middleware**

Request ID 使用 `crypto/rand`；CORS 使用单一 `CORS_ORIGIN`；Access Log 使用 `log/slog` 并在 Gin dispatch 后读取 `c.FullPath()`。不记录 headers、Token、密码或完整请求体。

- [x] **Step 3: 显式组装 API**

`cmd/api/main.go` 按可见顺序执行：

```text
godotenv.Load
-> LoadAPI
-> logger
-> PostgreSQL
-> Redis
-> AutoMigrate(taskdemo.Task)
-> Asynq Client
-> Repository
-> Service
-> Handler
-> Routes
-> HTTP Server
-> signal
-> graceful shutdown and close resources
```

只为测试提取 `buildRouter`，不创建容器或运行时注册。

- [x] **Step 4: 显式组装 Worker**

`cmd/worker/main.go` 按可见顺序执行：

```text
godotenv.Load
-> LoadWorker(POSTGRES_DSN + REDIS_URL)
-> logger
-> PostgreSQL（不 AutoMigrate）
-> Task Repository/Service
-> Asynq ServeMux
-> taskdemo.Register
-> Asynq Server
-> signal and graceful shutdown
```

- [x] **Step 5: 后端全量绿色**

```powershell
cd server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

Expected: 全部退出 0，无 panic、竞态提示或未处理测试失败。

### Task 7: 前端严格 HTTP 契约

**Files:**
- Create: `web/src/types/http.ts`
- Create: `web/src/types/pagination.ts`
- Create: `web/src/utils/request.ts`
- Create: `web/src/api/health.ts`
- Create: `web/src/api/taskDemo.ts`
- Test: `web/src/utils/request.test.ts`

- [x] **Step 1: 写失败测试**

测试响应只有 `{code,data,message}` 时才接受；多字段、缺字段、`msg`、非数字 code、非字符串 message 都抛 `ProtocolError`；`code !== 0` 抛包含业务 code/message 的 `ApiError`；网络错误原样抛出，不转换为空数组或对象。

```powershell
cd web
pnpm vitest run src/utils/request.test.ts
```

Expected: FAIL，因为请求封装不存在。

- [x] **Step 2: 实现唯一类型和 Axios 实例**

```ts
export interface ApiResponse<T> {
  code: number
  data: T
  message: string
}

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

Axios 只读取必填 `VITE_API_BASE_URL`，为空时立即抛错，设置 `withCredentials: true`，不配置 proxy，不读取其他变量，不兼容替代字段。

- [x] **Step 3: 实现模块 API**

`health.ts` 导出 `getHealth()` 和 `getReadiness()`；`taskDemo.ts` 导出 `createExampleTask({message})`。Vue 页面不得直接导入 Axios。

- [x] **Step 4: 验证绿色**

```powershell
cd web
pnpm vitest run src/utils/request.test.ts
```

Expected: PASS。

### Task 8: Vue Admin 外壳与基础仪表盘

**Files:**
- Create: `web/.env.example`
- Create: `web/.env`
- Create: `web/src/store/index.ts`
- Create: `web/src/router/index.ts`
- Create: `web/src/permission.ts`
- Create: `web/src/layout/AdminLayout.vue`
- Create: `web/src/views/dashboard/index.vue`
- Create: `web/src/views/dashboard/components/ReadinessChart.vue`
- Create: `web/src/styles/variables.scss`
- Create: `web/src/styles/index.scss`
- Create: `web/src/App.vue`
- Create: `web/src/main.ts`
- Test: `web/src/views/dashboard/index.test.ts`
- Test: `web/src/router/index.test.ts`

- [x] **Step 1: 确定视觉实现**

面向国内后台开发与运维人员，页面唯一工作是查看 API/依赖状态并投递示例任务。视觉令牌固定为：`#18212A` 墨黑侧栏、`#F4F6F7` 工作区、`#FFFFFF` 内容面、`#16845B` 正常、`#C97822` 等待、`#C33C3C` 失败、`#2F6FED` 操作色；字体使用 `Inter, PingFang SC, Microsoft YaHei, sans-serif`。布局为 224px 侧栏、56px 顶栏、桌面双栏/移动单栏。标志性元素是顶栏中的实时状态轨道，不使用渐变、营销文案、装饰图片、嵌套卡片或圆角药丸堆叠。

- [x] **Step 2: 写 Dashboard 和 Router 失败测试**

测试 `/` 重定向 `/dashboard`，路由显式 `requiresAuth:false`；Dashboard 调用模块 API，成功显示 API/PostgreSQL/Redis 的真实状态；readiness 失败显示“检查失败”而非假健康；空消息不能提交；成功投递显示真实 task ID；失败显示明确错误。

```powershell
cd web
pnpm vitest run src/views/dashboard/index.test.ts src/router/index.test.ts
```

Expected: FAIL，因为页面和 Router 不存在。

- [x] **Step 3: 实现 Router、Pinia 和权限入口**

只创建 `/dashboard`。`permission.ts` 只检查每条路由是否明确声明 `requiresAuth`；首期不模拟用户、Token、角色或动态菜单。

- [x] **Step 4: 实现 AdminLayout 和 Dashboard**

使用 Element Plus 与其 Icons。Dashboard 的状态区域和任务控制台是两个同级工具区域，不互相嵌套。ECharts 在页面私有 `ReadinessChart.vue` 内按需引入 CanvasRenderer、PieChart、TooltipComponent，不全局注册全部图表。所有按钮使用图标和明确命令，提供键盘焦点、移动端无重叠和 reduced-motion 处理。

- [x] **Step 5: 添加环境和全局样式**

`web/.env.example` 与被忽略的 `.env` 包含：

```dotenv
VITE_API_BASE_URL=http://localhost:16301
```

不得定义 Vite proxy。SCSS 不使用 viewport 字号、不使用负 letter-spacing、不使用装饰渐变。

- [x] **Step 6: 前端全量绿色**

```powershell
cd web
pnpm vitest run
pnpm build
```

Expected: 测试 PASS，TypeScript 和 Vite 构建退出 0。

### Task 9: 本机联调、文档和最终验证

**Files:**
- Modify: `README.md`

- [x] **Step 1: 验证本机依赖**

使用 `server/.env` 中的实际 DSN/URL 验证 PostgreSQL 和 Redis；失败时报告准确连接错误，不启动替代服务，不使用 Docker。

- [x] **Step 2: 启动 API、Worker 和前端**

使用隐藏后台进程分别运行：

```powershell
cd server; go run ./cmd/api
cd server; go run ./cmd/worker
cd web; pnpm dev
```

若端口被占用，先识别现有进程；不自动终止用户进程。

- [x] **Step 3: 验证真实链路**

```powershell
Invoke-RestMethod http://localhost:16301/health
Invoke-RestMethod http://localhost:16301/ready
Invoke-RestMethod -Method Post -Uri http://localhost:16301/api/v1/example-tasks -ContentType 'application/json' -Body '{"message":"foundation-check"}'
```

Expected: health/ready `code=0`；任务接口 HTTP 202 且 taskId 非空；PostgreSQL 中记录经历 pending/running/completed；Worker 日志包含该 task ID 和 message。

- [x] **Step 4: 浏览器验证**

在桌面 1440x900 和移动端 390x844 检查无重叠、无横向滚动、状态数据真实、任务提交可用、控制台无错误；检查 ECharts canvas 存在非空像素。保持前端开发服务器运行并提供本地 URL。

- [x] **Step 5: 最终自动化验证**

```powershell
cd server
go vet ./...
go test ./...
go build ./...
cd ..\web
pnpm vitest run
pnpm build
```

Expected: 全部退出 0。

- [x] **Step 6: 完成 README**

README 必须说明本机 PostgreSQL/Redis 前提、环境变量、启动顺序、URL、健康检查语义、示例任务为尽力投递、API 执行 AutoMigrate、Worker 不迁移，以及用户自行创建 Git 仓库。不得出现 Docker 命令。

## 计划自检

- Spec 第一阶段的 Vue、API、Worker、PostgreSQL、Redis、Asynq、响应、错误、校验、分页、健康检查、环境模板和测试均有对应任务。
- 登录、RBAC、文件、字典、Outbox 和业务 CRUD 明确不在本计划中。
- Queue 不导入业务模块；Repository 不调用 Queue；Worker 读取 PostgreSQL 和 Redis；只有 API AutoMigrate。
- 所有新增行为都有先失败后实现的测试步骤。
- 不包含 Docker、Git 初始化、静默默认值或未使用扩展点。
