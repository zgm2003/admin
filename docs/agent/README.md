# Admin Agent 施工指南

本指南解释根 `AGENTS.md` 的项目硬规则。用户当前明确指令优先，其次是 `AGENTS.md`，再其次是本指南、当前 spec 和当前 plan。规则冲突时停止实现并报告，不自行猜测。

本指南只约束 AI 和开发者如何理解、修改和验证项目，不是运行时 Agent 框架，不改变应用架构。

## 1. 渐进式任务路由

先判断任务规模，再读取最小充分上下文。小型局部任务不全量读取历史 spec、plan 或其他模块。

| 任务类型 | 读取范围 |
| --- | --- |
| 解释代码、状态或单点问题 | `AGENTS.md`、目标文件、直接调用方 |
| 单文件配置、文案、样式、窄测试 | 对应 Agent 规则、目标文件、相关测试 |
| 单模块 bug | 模块调用链、相关 spec 章节、相关测试 |
| 新功能、跨模块契约、数据库、认证权限 | 对应完整 spec、完整 plan、相关代码 |
| 执行已批准 plan | 当前 plan、对应 spec、涉及的规则和代码 |

禁止以“可能有用”为理由读取全部历史 spec、plan 或仓库文档。每个额外文档必须与当前决策有直接关系。

只读问题不需要创建 spec 或 plan。简单局部修改只需说明修改点和验证方式。新功能、跨模块或协议变化必须先确认设计；多步骤实现才编写 plan。

## 2. 线性数据流

普通数据库请求固定为：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

各层职责：

- Router：声明 URL、中间件和 Handler 绑定；
- Middleware：处理请求级认证、权限、request ID、CORS、日志和 Recovery；
- Handler：绑定和校验请求，调用 Service，输出统一响应；
- Service：业务规则、状态变化、调用顺序和事务意图；
- Repository：只处理 PostgreSQL 查询、写入、锁和事务；
- Model：只描述 GORM 与 PostgreSQL 表映射。

边界固定：

- Handler 不直接访问 GORM、Redis、Queue、Storage 或第三方 SDK；
- Service 不依赖 Gin，不接收 `*gin.Context`；
- Repository 不处理 HTTP，不调用 Queue；
- Queue 只封装 Asynq，不导入业务模块；
- Redis 目录只处理 Redis 连接和基础操作；
- Storage 只处理当前对象存储 SDK；
- Integration client 只处理一个具体第三方协议。

异步消费固定为：

```text
task handler -> service -> repository -> model -> PostgreSQL
```

禁止引入 `Platform Adapter`、通用 Adapter、`infra`、Manager、Factory、BaseService、BaseRepository、DI 容器、运行时注册器或为了未来替换而存在的接口。只有当前存在多个真实实现，或者测试确有替换边界时，才定义覆盖当前需求的最小接口。

## 3. TypeScript 类型安全

前端使用 TypeScript，业务代码不得退化为 AnyScript。禁止：

```ts
let value: any
const rows: any[] = []
const payload = input as any
type Values = Record<string, any>
```

同样禁止用 `@ts-ignore`、宽泛 ambient declaration 或关闭严格编译选项绕过错误。

正确方式：

- 外部未知数据使用 `unknown`；
- 运行时校验结构后再缩小类型；
- API 请求和响应声明明确 DTO；
- Vue Props、Emits、Pinia 状态和组合式函数返回值使用明确类型；
- 有限稳定值使用 `as const` 对象或联合类型；
- 动态键和值定义真实类型，不使用 `Record<string, any>`；
- 第三方库缺失类型时，在独立声明文件中补当前调用所需的最小类型，不让 `any` 扩散到业务代码。

HTTP 数据流固定为：

```text
view -> api/<module>.ts -> utils/request.ts -> Go API
```

Axios 必须先严格校验 envelope，再把 `data` 交给业务 API。页面不能猜字段、兼容 `msg`，也不能用可选链、类型断言或默认值掩盖必填字段缺失。

裸 `any` 定向检查：

```powershell
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" web\src -g "*.ts" -g "*.vue"
```

命令有输出时逐处处理。测试夹具也不能用 `any` 逃避类型检查。

## 4. 数据库规范

所有系统维护表直接声明：

```sql
created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
```

Go 模型显式写出 `CreatedAt` 和 `UpdatedAt`，不嵌入 `gorm.Model`，不创建 BaseModel。手写 SQL 更新记录时显式维护 `updated_at`，不使用隐藏触发器。

有真实删除行为的表只使用：

```sql
deleted_at TIMESTAMPTZ NULL
```

- `deleted_at IS NULL` 表示有效；
- 非空表示已软删除，并记录删除时间；
- Go 使用 `gorm.DeletedAt`；
- 禁止增加 `is_del`、`is_deleted` 或另一套删除状态；
- 没有删除行为的日志、事件或任务记录不虚构该字段；
- 业务唯一值只要求有效数据唯一时，使用 `WHERE deleted_at IS NULL` 的部分唯一索引。

Yes/No 固定为：

```text
0 = No / 否
1 = Yes / 是
```

数据库字段使用 `SMALLINT` 和 `CHECK (column IN (0, 1))`。后端只使用 `shared/yesno`，前端只使用 `enums/yes-no`。这组代码不可由字典管理页面修改。

只为真实查询创建索引。AutoMigrate 不负责删除字段、修改类型或数据回填；生产 migration 需求单独设计。

## 5. 错误与禁止兜底

HTTP envelope 精确为：

```json
{
  "code": 0,
  "data": {},
  "message": "ok"
}
```

错误必须沿调用链返回。服务端记录内部 cause，客户端只接收公开 code/message。禁止把 SQL、堆栈、Token、Cookie、密码或 DSN 写入响应或普通请求日志。

禁止以下兜底：

- 缺配置时猜默认值；
- PostgreSQL、Redis 或 Queue 不可用时切换内存实现；
- 失败时返回空数组、假成功或旧缓存；
- 捕获错误后只记录日志不返回；
- 同时接受 `msg` 和 `message`；
- 必填字段缺失时使用 `??`、可选链或类型断言继续运行；
- Worker 依赖不可用时假装启动成功。

只有产品明确声明为可选的字段，才允许使用对应的空态展示。

## 6. 命名规范

- Go 导出名称使用 PascalCase，initialism 使用 `ID`、`HTTP`、`API`；
- JSON 和 TypeScript 使用 lower camel case；
- PostgreSQL 表、列、索引和约束使用 lower snake case；
- URL 使用小写复数资源名；
- 权限码使用冒号分段，例如 `system:user:list`；
- 第三方服务使用具体名称，例如 `redis`、`queue`、`storage`、`wechat`，不用 `platform` 统称；
- 文件和包名表达真实职责，不使用万能 `common`、`utils2` 或无业务含义的缩写。

## 7. Context 与 I/O

HTTP Handler 使用 `context.Request.Context()`，Asynq Handler 使用任务 Context。该 `context.Context` 必须继续传给 Service、Repository、GORM、Redis、Queue、Storage 和第三方 I/O。

禁止：

- 将 `*gin.Context` 传入 Service 或 Repository；
- 在调用链中途换成 `context.Background()`；
- 忽略取消、超时或 I/O 错误；
- 为了少写参数把 Context 放进全局对象。

## 8. 施工流程

### 8.1 只读问题

读取最小相关代码和文档，给出证据支持的回答。不修改文件，不创建 spec 或 plan。

### 8.2 简单局部修改

说明要改的文件、行为和验证方式，然后直接做定向修改。不要求为文案、单个配置值或窄测试创建完整 spec/plan。

### 8.3 新功能和契约变化

涉及新功能、跨模块、数据库结构、认证权限、公共协议或多个步骤时：

```text
理解需求 -> 提出方案 -> 用户确认 -> spec -> plan -> 实现
```

不在设计获批前修改运行时代码。

### 8.4 行为变化和 bug

```text
写失败测试 -> 确认按预期失败 -> 最小实现 -> 确认通过 -> 再重构
```

失败测试必须因目标行为缺失而失败，不能因拼写、导入或环境错误失败。修复后运行相关测试，再根据影响范围决定是否扩大验证。

### 8.5 文档任务

文档任务运行链接、章节、关键词和 `git diff --check` 等定向检查。不修改运行时代码时，不机械运行 Go、Vitest 或生产构建。

## 9. 验证与完成

验证规模与风险匹配：

- Go 变更：相关测试；共享或跨模块变更再运行 `go fmt ./...`、`go vet ./...`、`go test ./...`、`go build ./...`；
- 前端行为变更：相关 Vitest；公共请求、路由或构建契约变更再运行 `pnpm vitest run` 和 `pnpm build`；
- 数据库变更：模型测试和真实 PostgreSQL 集成检查，不用内存数据库代替；
- 文档变更：定向内容检查和 `git diff --check`。

完成报告必须区分：已验证、未运行、环境阻塞和剩余风险。禁止用“应该可以”代替命令输出。

## 10. Git 与服务边界

- 不回滚用户已有改动；
- Git 初始化、remote、分支、commit、fetch、pull 和 push 只按用户明确授权执行；
- 发现其他进程产生的提交或改动时先只读核查并保留；
- 不停止用户启动的服务；
- 需要启动开发服务时先确认授权，结束时说明哪些进程仍在运行；
- 不使用 Docker 或 Docker Compose。

## 11. 交接清单

接手编码任务时，只回答与当前范围相关的项目：

- 当前任务对应哪个 spec、plan 或具体用户指令？
- 工作区和 Git 是否存在其他改动？
- 数据流从哪里开始，经过哪些具体文件？
- 请求、响应、错误和依赖失败行为是什么？
- 是否涉及 `TIMESTAMPTZ`、`deleted_at` 或 Yes/No？
- 前端 DTO、Props、Emits、Store 和返回值类型在哪里？
- 哪些验证已运行，哪些未运行，原因是什么？

只回答局部问题时不要求回答全部项目。需要修改代码时，如果与本次范围相关的问题无法回答，应补读对应内容或向用户提问，不扩大到无关文档。
