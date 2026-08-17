# Admin 基础质量加固设计说明

## 1. 背景与目标

一期基础计划中的任务已经全部完成。本设计不改变现有线性架构，也不增加新业务模块，只处理一期完成后确认的基础质量问题，并固化后续建表规范。

本次范围包含：

1. 初始化本地 Git 仓库并绑定指定远程仓库；
2. 前端开发服务启动后自动打开浏览器；
3. 统一 PostgreSQL 时间字段、软删除字段和 Yes/No 编码；
4. 修复 Axios 无法解析 HTTP 400、503 等失败响应中的业务错误；
5. 让后端请求日志记录内部错误原因，但不泄漏到响应；
6. 让 Worker 在启动时无法连接 Redis 时明确退出；
7. 用项目自己的 Recovery 保证 panic 仍返回统一响应结构。

本次继续遵守普通数据库请求的固定调用链：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

不得借本次加固引入 BaseModel、BaseRepository、Manager、Factory、Platform Adapter、依赖注入容器或兼容性兜底。

## 2. Git 仓库配置

当前 `D:\admin` 没有 `.git` 目录。实施时只执行以下仓库初始化和远程绑定：

```powershell
cd D:\admin
git init -b main
git remote add origin https://github.com/zgm2003/admin.git
git remote -v
```

本节是项目所有者的新指令，实施时覆盖 `AGENTS.md` 和 `README.md` 中一期阶段“不得初始化 Git”的旧约束，但不授予自动提交或推送权限。

验收结果必须满足：

- 当前分支名为 `main`；
- `origin` 的 fetch 和 push 地址均为 `https://github.com/zgm2003/admin.git`；
- 根目录现有 `.gitignore` 继续生效；
- 同步修改 `AGENTS.md` 和 `README.md` 中“不得初始化 Git”或“Git 由项目所有者初始化”的过期说明。

本次不执行 `git add`、`git commit`、`git fetch`、`git pull` 或 `git push`。远程仓库内容和首次提交由项目所有者决定。

## 3. 前端开发服务自动打开浏览器

在 `web/vite.config.ts` 的现有 `server` 配置中增加：

```ts
open: true
```

现有开发约束保持不变：

- 前端固定监听 `localhost:16300`；
- `strictPort: true`，端口占用时明确失败，不自动换端口；
- 只有 `pnpm dev` 启动开发服务时打开浏览器；
- 测试和生产构建不得打开浏览器；
- 不增加 Vite Proxy，前端继续通过 `VITE_API_BASE_URL` 直接访问 Go API。

## 4. PostgreSQL 表字段规范

### 4.1 时间类型

PostgreSQL 不使用 MySQL 风格的 `DATETIME`。所有系统维护的表都必须显式包含：

```sql
created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
```

规则如下：

- `created_at` 表示记录创建时间，创建后不再修改；
- `updated_at` 表示最后修改时间，每次真实更新都必须同步更新；
- GORM 创建和更新模型时负责维护两个字段；
- 手写 SQL 时必须显式维护 `updated_at`，不使用隐藏的数据库触发器自动更新；
- `TIMESTAMPTZ` 表示确定的时间点，API 传输使用带时区的 ISO 8601 时间，前端按 `Asia/Shanghai` 展示；
- 不用字符串、Unix 秒数或不带时区的 `TIMESTAMP` 代替业务时间字段。

每个 `model.go` 都直接声明字段，不嵌入 `gorm.Model`，也不创建项目级 `BaseModel`。这样开发者打开模型文件就能看到该表的完整公共字段。

Go 模型使用明确字段和类型：

```go
CreatedAt time.Time `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
UpdatedAt time.Time `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
```

现有 `foundation_task` 已包含 `created_at` 和 `updated_at`，实施时应补充明确的 `TIMESTAMPTZ` GORM 类型约束，并确认 AutoMigrate 后数据库类型正确。

### 4.2 软删除

需要删除语义的业务表默认使用：

```sql
deleted_at TIMESTAMPTZ NULL
```

字段含义只有一套：

- `deleted_at IS NULL`：记录有效；
- `deleted_at IS NOT NULL`：记录已软删除，字段值同时表示删除时间。

禁止再增加 `is_del`、`is_deleted` 或 `delete_flag`。删除状态只由 `deleted_at` 判断，避免两个字段产生不一致。

Go 模型使用 GORM 标准的 `gorm.DeletedAt`。Repository 的普通查询依赖 GORM 默认作用域排除已删除数据；软删除调用最终生成 `UPDATE ... SET deleted_at = ...`，不执行物理 `DELETE`。

```go
DeletedAt gorm.DeletedAt `gorm:"type:timestamptz"`
```

软删除数据流保持线性：

```text
router -> middleware -> handler -> service -> repository
-> GORM 更新 deleted_at -> PostgreSQL
```

只有明确的回收站、恢复或物理清理功能可以使用 `Unscoped()`，并且必须写在具有具体业务名称的 Repository 方法内。Handler 和 Service 不得直接操作 `Unscoped()`。项目不提供通用硬删除入口。

以下表可以不增加 `deleted_at`：

- 产品定义为永久追加且没有删除操作的审计日志、事件记录；
- 仅按保留策略物理清理、不会被业务用户删除的临时或任务记录；
- 当前功能没有删除、恢复或删除态查询需求的表。

因此，本次不为没有删除功能的 `foundation_task` 虚构软删除能力。以后新增删除行为时，必须在同一个业务变更中增加 `deleted_at`、查询规则和测试。其他例外必须在所属模块 spec 中写明原因，不能临时执行物理删除。

### 4.3 软删除与索引

业务唯一值只要求在有效数据中唯一时，必须使用 PostgreSQL 部分唯一索引：

```sql
CREATE UNIQUE INDEX uk_<table>_<column>_active
ON <table> (<column>)
WHERE deleted_at IS NULL;
```

禁止使用 `(column, deleted_at)` 复合唯一索引模拟该规则，因为 PostgreSQL 中多个 `NULL` 不会按普通相等值处理。

普通查询索引按真实查询条件创建。不得给每张表机械增加 `deleted_at`、`created_at`、`updated_at` 索引；常用有效数据查询可以按实际 SQL 使用 `WHERE deleted_at IS NULL` 的部分索引。

## 5. Yes/No 统一编码

系统拥有一套不可变的 Yes/No 技术编码：

| 代码 | 含义 |
| ---: | --- |
| `0` | No / 否 |
| `1` | Yes / 是 |

不使用“`1` 是、`2` 否”。`0/1` 与 Go、TypeScript 和 SQL 的常见零值语义一致，也便于直接添加数据库约束。

数据库中需要以编码传输和查询的 Yes/No 字段使用：

```sql
<column> SMALLINT NOT NULL,
CONSTRAINT chk_<table>_<column>_yes_no CHECK (<column> IN (0, 1))
```

规则如下：

- PostgreSQL 字段使用能表达业务含义的名称，例如 `is_enabled`、`is_system`，不用 `flag1`；
- JSON 和 TypeScript 使用对应的 lower camel case 名称，例如 `isEnabled`；
- Go 公共值只在 `server/internal/shared/yesno` 定义一次，使用 `yesno.No = 0`、`yesno.Yes = 1`；
- 前端公共值只在 `web/src/enums/yes-no.ts` 定义一次，使用 `as const` 对象 `YesNo.No = 0`、`YesNo.Yes = 1`；由于项目启用了 `erasableSyntaxOnly`，不使用 TypeScript `enum` 语法；
- 请求收到其他数值时直接返回参数错误，数据库再以 `CHECK` 约束兜底数据完整性；
- 只有产品明确规定默认值时数据库字段才设置 `DEFAULT`，不得用默认值掩盖调用方漏传；
- `deleted_at` 不使用 Yes/No 编码，它本身就是软删除的唯一事实。

Yes/No 是不可变技术枚举，不写入可由管理员修改的字典表，也不建设管理页面。中文“是/否”展示文本由前端枚举或国际化资源维护；未来的业务字典不能修改 `0/1` 的协议含义。

## 6. Axios 失败响应解析

### 6.1 当前问题

当前 Axios 只注册成功响应拦截器。HTTP 400、503 等状态会直接进入 Axios rejection 分支，响应中的 `code`、`data`、`message` 没有经过协议校验，也不会转换为 `ApiError`。

### 6.2 目标行为

成功和失败响应必须共用同一套严格 envelope 校验，不兼容 `msg`、额外字段或缺失字段。

```text
HTTP 2xx
-> 校验 code/data/message
-> code 必须为 0
-> 返回 data

HTTP 非 2xx 且存在 response
-> 校验 code/data/message
-> code 必须非 0
-> 抛出 ApiError(code, message, httpStatus)

请求没有收到 response
-> 保留原始网络错误

任意响应违反协议
-> 抛出 ProtocolError
```

不得把 400、401、403、409、500、503 转成空数据，也不得从 HTTP 状态猜测业务文案。

### 6.3 验收测试

前端测试至少覆盖：

- HTTP 400 的合法业务失败 envelope 转换为 `ApiError`；
- HTTP 503 的合法业务失败 envelope 转换为 `ApiError`；
- 非 2xx 却返回 `code: 0` 时抛出 `ProtocolError`；
- 非 2xx 返回 `msg` 或 HTML 等错误结构时抛出 `ProtocolError`；
- 没有 response 的连接失败保留原始网络错误。

## 7. 后端内部错误日志

### 7.1 当前问题

`apperror.Error` 已保存内部 `Cause`，但 `response.Fail` 只输出公开消息，`AccessLog` 也只记录请求元数据。发生 PostgreSQL、Redis 或业务内部错误时，客户端看不到敏感细节是正确的，但服务端日志同样没有原因，导致无法根据 request ID 排查。

### 7.2 目标行为

错误仍按原调用链返回：

```text
dependency/repository error
-> service 包装为 apperror
-> handler 调用 response.Fail
-> response.Fail 将错误登记到当前 Gin context
-> AccessLog 在请求结束后记录业务错误码和内部原因
-> 客户端只收到公开 envelope
```

日志规则：

- 每个请求仍只有一条标准访问日志，包含 `requestId`、method、route、status、latency；
- 失败请求增加稳定的业务 `errorCode`；
- 5xx 和依赖不可用错误增加内部 `error` 原因；
- 客户端响应仍只能包含 `code`、`data`、`message`，绝不包含 Cause、SQL、堆栈或配置；
- 不记录密码、Token、Cookie、完整请求体或 PostgreSQL DSN；
- 不在 Handler、Service、Repository 每层重复记录同一个错误。

测试必须证明内部原因出现在服务端日志中，同时没有出现在 HTTP 响应中，并能通过同一个 request ID 关联。

## 8. Worker 启动时检查 Redis

### 8.1 当前问题

Worker 当前连接 PostgreSQL 后直接启动 Asynq Server。Asynq 启动调用不能作为清晰的 Redis 启动就绪检查，因此 Redis 不可用时 Worker 进程可能继续存活并反复报连接错误。

### 8.2 目标行为

Worker 启动顺序固定为：

```text
load config
-> connect and ping PostgreSQL
-> connect and ping Redis
-> build task repositories/services
-> register Asynq handlers
-> start Asynq Server
```

`cmd/worker` 复用现有 `internal/redis.Open` 做显式 Ping，并在进程退出时关闭客户端。Redis URL 解析失败、连接失败或 Ping 失败时，`run` 立即返回带上下文的错误，`main` 记录错误并以非零状态退出。

不得跳过任务、启动内存队列或等待无限重试来伪装启动成功。Worker 成功启动后 Redis 临时中断时，继续使用 Asynq 自身的运行期重连行为；本次不增加新的 Runner、监控协议或替代队列。

测试至少覆盖 Redis 启动检查失败会阻止 Asynq Server 启动，并保留可定位的错误原因。

## 9. 统一 Panic Recovery

### 9.1 当前问题

API 当前直接使用 `gin.Recovery()`。Handler 或中间件 panic 时，它返回 Gin 默认的裸 HTTP 500，不满足项目固定的 `code`、`data`、`message` 协议。

### 9.2 目标行为

使用项目自己的 `middleware.Recovery` 替换 `gin.Recovery()`：

```text
panic
-> Recovery 捕获 panic 和 stack
-> 服务端记录 request ID、panic 原因和 stack
-> response.Fail(apperror.Internal(...))
-> HTTP 500 + 标准 envelope
```

响应固定为：

```json
{
  "code": 10000,
  "data": null,
  "message": "服务内部错误"
}
```

Recovery 不吞掉日志，不暴露 panic 内容，不返回 HTML 或空响应。中间件顺序必须保证 Recovery 能取得 request ID，并且 AccessLog 最终记录真实的 500 状态。

测试必须覆盖 panic 路由的状态码、精确 envelope、request ID、内部 panic 日志和访问日志。

## 10. 文档与最终验收

实施完成后同步更新：

- `AGENTS.md`：加入时间字段、软删除和 Yes/No 编码规则，移除过期 Git 禁令；
- `README.md`：说明仓库远程状态和 `pnpm dev` 会自动打开浏览器；
- 原一期设计不改写历史结论，本 spec 作为后续加固依据。

最终验证命令：

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

```powershell
cd D:\admin\web
pnpm vitest run
pnpm build
```

还必须人工核验：

- `git branch --show-current` 输出 `main`；
- `git remote -v` 显示指定的 `origin`；
- `pnpm dev` 在 `16300` 启动并自动打开浏览器；
- HTTP 400、503 和 panic 都遵守精确响应 envelope；
- Redis 停止时 Worker 启动失败并以非零状态退出；
- PostgreSQL 的公共时间字段为 `TIMESTAMPTZ`；
- 软删除表只以 `deleted_at` 判断删除状态；
- Yes/No 字段拒绝 `0`、`1` 之外的值。

## 11. 非目标

本次不实现：

- 登录、用户、角色、菜单和业务字典管理；
- 回收站页面、通用恢复接口或通用硬删除接口；
- 数据库触发器、通用 BaseModel 或自动建索引框架；
- 版本化 migration 框架；
- Docker、内存数据库、内存队列或依赖不可用时的兜底；
- 自动提交、拉取或推送 Git 历史。
