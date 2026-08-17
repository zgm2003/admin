# Admin Foundation Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成基础质量加固，固定数据库字段协议，并让 Git、前端开发服务、HTTP 错误、Worker 启动和 Panic Recovery 的行为可验证。

**Architecture:** 保持现有线性调用链和具体目录边界。数据库字段规则写在每个模型中，不引入 BaseModel；HTTP 错误由现有 `response` 和 `AccessLog` 贯通；Worker 复用现有 Redis 客户端做启动 Ping；Vite 只增加开发模式 `open` 配置。

**Tech Stack:** Go 1.26、Gin、GORM/PostgreSQL、go-redis、Asynq、Vue 3、TypeScript、Axios、Vite、Vitest、pnpm。

---

## Task 1: 更新项目规则并绑定 Git 远程

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`
- Create/modify: `.git/config` through Git commands only

- [ ] **Step 1: 更新过期的 Git 规则和启动说明**

修改 `AGENTS.md` 的 Git 规则为：允许按本计划初始化仓库并绑定远程，但禁止自动 `commit`、`fetch`、`pull`、`push`。保留“用户负责 Git 历史”的边界。

将 `README.md` 中以下两类旧说明改为当前事实：

- 项目已经允许并按本计划初始化 Git；
- `pnpm dev` 会在开发服务启动后打开浏览器。

不要修改端口：前端仍为 `16300`，API 仍为 `16301`。

- [ ] **Step 2: 初始化仓库并设置 origin**

先确认当前目录没有 `.git`，然后运行：

```powershell
cd D:\admin
git init -b main
git remote add origin https://github.com/zgm2003/admin.git
```

本次不运行 `git add`、`git commit`、`git fetch`、`git pull` 或 `git push`。

- [ ] **Step 3: 验证 Git 配置**

运行：

```powershell
cd D:\admin
git branch --show-current
git remote -v
git status --short --branch
```

预期：当前分支为 `main`，fetch/push 的 `origin` 均为指定 URL，工作区文件未被自动提交或推送。

## Task 2: 固化 PostgreSQL 时间和 Yes/No 约定

**Files:**
- Create: `server/internal/shared/yesno/yesno.go`
- Create: `server/internal/shared/yesno/yesno_test.go`
- Modify: `server/internal/module/taskdemo/model.go`
- Modify: `server/internal/module/taskdemo/repository_test.go`
- Create: `web/src/enums/yes-no.ts`
- Create: `web/src/enums/yes-no.test.ts`

- [ ] **Step 1: 写 Go Yes/No 失败测试**

在 `yesno_test.go` 先写测试，要求 `No` 为 `int16(0)`、`Yes` 为 `int16(1)`，并测试 `IsValid` 只接受 `0` 和 `1`：

```go
func TestValues(t *testing.T) {
    if No != 0 || Yes != 1 {
        t.Fatalf("No=%d Yes=%d, want 0 and 1", No, Yes)
    }
    if !IsValid(No) || !IsValid(Yes) || IsValid(-1) || IsValid(2) {
        t.Fatal("unexpected Yes/No validation result")
    }
}
```

- [ ] **Step 2: 运行 Go 失败测试**

运行：

```powershell
cd D:\admin\server
go test ./internal/shared/yesno
```

预期：因 `No`、`Yes` 和 `IsValid` 尚未定义而失败。

- [ ] **Step 3: 实现唯一 Go 编码定义**

在 `server/internal/shared/yesno/yesno.go` 写入：

```go
package yesno

type Value int16

const (
    No  Value = 0
    Yes Value = 1
)

func IsValid(value Value) bool {
    return value == No || value == Yes
}
```

- [ ] **Step 4: 写 TypeScript 编码测试并实现公共值**

在 `web/src/enums/yes-no.test.ts` 验证 `YesNo.No === 0`、`YesNo.Yes === 1`，然后创建 `web/src/enums/yes-no.ts`。项目启用了 `erasableSyntaxOnly`，所以使用类型安全的 `as const` 对象而不是 TypeScript `enum`：

```ts
export const YesNo = {
  No: 0,
  Yes: 1,
} as const

export type YesNo = (typeof YesNo)[keyof typeof YesNo]
```

测试不得允许 `2` 作为 Yes/No 值。

- [ ] **Step 5: 运行编码测试**

运行：

```powershell
cd D:\admin\server
go test ./internal/shared/yesno
cd D:\admin\web
pnpm vitest run src/enums/yes-no.test.ts
```

预期：全部通过。

- [ ] **Step 6: 为现有 Task 模型显式声明 timestamptz**

修改 `server/internal/module/taskdemo/model.go`，保留现有字段和状态约束，将两个时间字段改为：

```go
CreatedAt time.Time `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
UpdatedAt time.Time `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
```

不要给没有删除功能的 `foundation_task` 增加 `deleted_at`。以后具有删除语义的模型显式声明：

```go
DeletedAt gorm.DeletedAt `gorm:"type:timestamptz"`
```

不得嵌入 `gorm.Model` 或创建公共 BaseModel。

- [ ] **Step 7: 增加 PostgreSQL 时间类型集成断言**

在现有 `TestRepositoryPersistsAndUpdatesTask` 的 AutoMigrate 后查询 `information_schema.columns`，对 `foundation_task.created_at` 和 `foundation_task.updated_at` 断言：

- `data_type = 'timestamp with time zone'`；
- `is_nullable = 'NO'`；
- `column_default` 非空。

该测试继续使用现有 `testing.Short()` 跳过规则，并在清理阶段删除本测试创建的任务。

- [ ] **Step 8: 运行模型和后端测试**

运行：

```powershell
cd D:\admin\server
go test ./internal/module/taskdemo -run TestRepositoryPersistsAndUpdatesTask -count=1
```

预期：本机 PostgreSQL 可用时通过；未配置 PostgreSQL 时只按现有集成测试规则跳过，不得添加内存数据库兜底。

## Task 3: Vite 开发服务自动打开浏览器

**Files:**
- Modify: `web/vite.config.ts`
- Create: `web/vite.config.test.ts`

- [ ] **Step 1: 写 Vite 配置失败测试**

创建配置测试，导入 `vite.config.ts` 并断言：

```ts
expect(config.server).toMatchObject({
  host: 'localhost',
  port: 16300,
  strictPort: true,
  open: true,
})
```

- [ ] **Step 2: 运行配置测试确认失败**

运行：

```powershell
cd D:\admin\web
pnpm vitest run vite.config.test.ts
```

预期：因 `open` 尚未设置而失败。

- [ ] **Step 3: 增加最小 Vite 配置**

只在现有 `server` 对象增加 `open: true`，保留 `host`、`port` 和 `strictPort` 原值，不增加代理或端口回退。

- [ ] **Step 4: 验证 Vite 配置和构建**

运行：

```powershell
cd D:\admin\web
pnpm vitest run vite.config.test.ts
pnpm build
```

预期：测试和生产构建通过。开发服务的浏览器打开行为在最终人工验收中通过 `pnpm dev` 检查。

## Task 4: 让 Axios 解析 HTTP 失败 envelope

**Files:**
- Modify: `web/src/utils/request.ts`
- Modify: `web/src/utils/request.test.ts`

- [ ] **Step 1: 写 HTTP 400/503 失败测试**

扩展 `createRequestClient` 测试，使用 Axios adapter 拒绝带 `response` 的 `AxiosError`，分别返回：

```json
{"code":10001,"data":null,"message":"请求参数错误"}
```

和：

```json
{"code":10006,"data":null,"message":"服务暂未就绪"}
```

断言错误类型为 `ApiError`，业务 code/message 和 HTTP status 分别为 `10001/请求参数错误/400`、`10006/服务暂未就绪/503`。

再添加两个失败测试：非 2xx 返回 `code: 0`，以及返回 `msg` 或 HTML，均必须抛出 `ProtocolError`。

- [ ] **Step 2: 运行失败测试确认当前行为不满足**

运行：

```powershell
cd D:\admin\web
pnpm vitest run src/utils/request.test.ts
```

预期：当前 HTTP 失败请求仍为 Axios rejection，新增断言失败。

- [ ] **Step 3: 抽出严格 envelope 校验并扩展 ApiError**

在 `request.ts` 中保留唯一字段校验规则，新增 `ApiError.httpStatus`，并让响应拦截器按以下分支处理：

```ts
client.interceptors.response.use(
  (response) => {
    response.data = unwrapSuccessEnvelope(response.data)
    return response
  },
  (error) => {
    if (!axios.isAxiosError(error) || !error.response) {
      return Promise.reject(error)
    }
    const envelope = parseEnvelope(error.response.data)
    if (envelope.code === 0) {
      return Promise.reject(new ProtocolError('HTTP error response must use a non-zero business code'))
    }
    return Promise.reject(new ApiError(envelope.code, envelope.message, error.response.status))
  },
)
```

`unwrapSuccessEnvelope` 必须拒绝非 `code: 0` 的业务错误；错误拦截器必须拒绝 `code: 0`、`msg`、额外字段和非对象。无 response 的网络错误原样返回，不转为空数据。

- [ ] **Step 4: 运行 Axios 测试和前端全量测试**

运行：

```powershell
cd D:\admin\web
pnpm vitest run src/utils/request.test.ts
pnpm vitest run
```

预期：新增错误分支和现有基础测试全部通过。

## Task 5: 将内部错误写入请求日志

**Files:**
- Modify: `server/internal/shared/response/response.go`
- Modify: `server/internal/middleware/access_log.go`
- Modify: `server/internal/shared/response/response_test.go`
- Modify: `server/internal/middleware/access_log_test.go`

- [ ] **Step 1: 写 response context 登记测试**

扩展 `response_test.go`：调用 `response.Fail` 后断言 Gin context 的 `Errors` 含有 `apperror.Error`，公开 JSON 仍严格只有 `code`、`data`、`message`，且不包含内部 cause 文本。

- [ ] **Step 2: 写 AccessLog 内部 cause 测试**

在 `access_log_test.go` 增加一个返回 `response.Fail(context, apperror.Internal(errors.New("database password leaked")))` 的路由。断言结构化日志包含 `requestId`、`errorCode: 10000` 和内部 `error` 字段，但 HTTP 响应不包含 `database password leaked`。

- [ ] **Step 3: 运行新增测试确认失败**

运行：

```powershell
cd D:\admin\server
go test ./internal/shared/response ./internal/middleware
```

预期：当前 response 没有登记 context，AccessLog 没有 error 字段，新增断言失败。

- [ ] **Step 4: 在 response.Fail 登记标准化错误**

保留现有未知错误包装逻辑，得到 `appErr` 后先执行：

```go
_ = context.Error(appErr)
```

再执行现有 `AbortWithStatusJSON`。不得把 `appErr.Cause` 写入响应。

- [ ] **Step 5: 在 AccessLog 结束阶段读取错误**

在 `context.Next()` 返回后读取 `context.Errors.Last()`，用 `errors.As` 提取 `*apperror.Error`。日志始终包含现有请求字段；错误请求增加 `errorCode`，HTTP 状态大于等于 500 或错误码为依赖不可用时增加 `error` 原因。不要记录请求体、Authorization、Cookie、Token 或 DSN，也不要在其他层重复记录同一错误。

- [ ] **Step 6: 运行后端日志测试**

运行：

```powershell
cd D:\admin\server
go test ./internal/shared/response ./internal/middleware
```

预期：response envelope、request ID、错误码和内部原因日志测试全部通过，敏感内容不出现在 HTTP 响应中。

## Task 6: Worker 启动前显式 Ping Redis

**Files:**
- Modify: `server/cmd/worker/main.go`
- Create: `server/cmd/worker/main_test.go`

- [ ] **Step 1: 提取可测试的 Redis 启动检查并写失败测试**

在 Worker 入口保留线性顺序，提取一个只做解析、连接、Ping 和关闭的函数：

```go
func checkRedis(ctx context.Context, redisURL string) error
```

先写测试：传入已取消的 context 和 `redis://127.0.0.1:1/0`，断言返回错误且包含 `ping Redis`；测试不能启动 Asynq，也不能使用内存替代。

- [ ] **Step 2: 运行 Worker 检查测试**

运行：

```powershell
cd D:\admin\server
go test ./cmd/worker -run TestCheckRedis -count=1
```

预期：因 `checkRedis` 尚未实现而失败。

- [ ] **Step 3: 实现启动检查并接入 run**

复用 `internal/redis.Open`：

```go
func checkRedis(ctx context.Context, redisURL string) error {
    client, err := projectredis.Open(ctx, redisURL)
    if err != nil {
        return err
    }
    return client.Close()
}
```

在 `run` 中完成 PostgreSQL 连接后、创建任务 Repository/Service 和 `queue.NewServer` 前调用 `checkRedis(processContext, settings.RedisURL)`。错误必须带 `check Worker Redis` 上下文返回，不能等待无限重试或启动内存队列。

- [ ] **Step 4: 运行 Worker 测试和后端全量检查**

运行：

```powershell
cd D:\admin\server
go test ./cmd/worker -run TestCheckRedis -count=1
go vet ./...
go test ./...
go build ./...
```

预期：Redis 启动检查测试通过，后端全量验证通过。Worker 运行期断线继续交给 Asynq 自身重连，不增加新的 Adapter 或监控层。

## Task 7: 用统一 envelope 替换 Gin Recovery

**Files:**
- Create: `server/internal/middleware/recovery.go`
- Create: `server/internal/middleware/recovery_test.go`
- Modify: `server/cmd/api/main.go`

- [ ] **Step 1: 写 panic 响应和日志失败测试**

在 `recovery_test.go` 创建 Gin 测试路由，按 `RequestID -> AccessLog -> Recovery` 注册中间件，并让路由执行 `panic("panic-check")`。先断言目标行为：HTTP 500、JSON 精确等于：

```json
{"code":10000,"data":null,"message":"服务内部错误"}
```

同时断言 panic 内容不在响应中，Recovery 日志包含 request ID、panic 原因和 stack，AccessLog 日志包含状态 500 和 `errorCode: 10000`。

- [ ] **Step 2: 运行 Recovery 测试确认当前失败**

运行：

```powershell
cd D:\admin\server
go test ./internal/middleware -run Recovery -count=1
```

预期：当前使用 Gin 默认 Recovery，响应不是项目统一 envelope，测试失败。

- [ ] **Step 3: 实现项目 Recovery**

创建 `middleware.Recovery(logger *slog.Logger) gin.HandlerFunc`。使用 `defer` 捕获 panic，利用 `runtime/debug.Stack()` 记录结构化错误日志，再调用：

```go
response.Fail(context, apperror.Internal(fmt.Errorf("panic: %v", recovered)))
```

日志包含 `requestId`、panic 值和 `stack`；响应不得包含 panic 值。logger 为 nil 时使用 `slog.Default()`，不得静默吞掉 panic。

- [ ] **Step 4: 替换 API 路由中的 Gin Recovery**

在 `server/cmd/api/main.go` 的 `buildRouter` 中删除 `gin.Recovery()`，使用 `projectmiddleware.Recovery(dependencies.Logger)`。顺序保持 `RequestID`、`CORS`、`AccessLog`、`Recovery`，保证 Recovery 可读取 request ID，AccessLog 可记录最终状态。

- [ ] **Step 5: 运行 Recovery 和后端全量测试**

运行：

```powershell
cd D:\admin\server
go test ./internal/middleware -run Recovery -count=1
go vet ./...
go test ./...
go build ./...
```

预期：panic 返回标准 envelope，服务端日志有原因和 stack，访问日志仍有状态 500。

## Task 8: 文档、人工验收和最终验证

**Files:**
- Verify: `docs/superpowers/specs/2026-08-17-admin-foundation-hardening-design.md`
- Verify: `docs/superpowers/plans/2026-08-17-admin-foundation-hardening.md`
- Verify: `AGENTS.md`
- Verify: `README.md`

- [ ] **Step 1: 对照 spec 检查实现覆盖**

逐项核对：Git remote、Vite `open`、`TIMESTAMPTZ`、`deleted_at`、`0/1`、Axios 400/503、内部错误日志、Worker Redis Ping、统一 Recovery。发现缺项时回到对应任务补齐，不添加未在 spec 中批准的架构。

- [ ] **Step 2: 运行后端最终验证**

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

预期：所有命令退出码为 0。

- [ ] **Step 3: 运行前端最终验证**

```powershell
cd D:\admin\web
pnpm vitest run
pnpm build
```

预期：所有 Vitest 测试通过，生产构建成功。

- [ ] **Step 4: 人工验证开发启动行为**

确认 PostgreSQL 和 Redis 已由用户启动后，在三个终端分别运行：

```powershell
cd D:\admin\server; go run ./cmd/api
cd D:\admin\server; go run ./cmd/worker
cd D:\admin\web; pnpm dev
```

确认 `pnpm dev` 使用 `http://localhost:16300` 并自动打开浏览器；API 使用 `http://localhost:16301`；Worker 在 Redis 停止后重新启动会明确失败，而不是启动替代队列。

- [ ] **Step 5: 复核 Git，停止服务，不自动提交**

运行：

```powershell
cd D:\admin
git branch --show-current
git remote -v
git status --short --branch
```

确认没有自动 commit、push 或服务后台残留。不要在本计划中创建提交，Git 历史由项目所有者维护。

## 执行约束

- 每个测试先写失败断言，再写最小实现；
- 不使用 Docker，不添加内存数据库、内存队列或依赖失败兜底；
- 不修改一期已完成业务范围；
- 不为没有删除语义的表添加未使用的 `deleted_at`；
- 所有新增错误必须沿现有调用链返回，不能在 Handler 直接操作 GORM、Redis 或 Asynq；
- 任务完成后必须以命令输出为依据汇报，不凭代码阅读声称测试通过。
