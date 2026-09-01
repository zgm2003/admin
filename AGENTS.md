# Admin Project Instructions

本文件是 `D:\admin` 的项目级硬规则入口。开始任务时先读本文件，再使用
`docs/agent/README.md` 的任务路由，只读取当前任务相关的规则、spec、plan、代码和测试。
小型或局部任务不得全量读取历史文档和无关模块。

## Project Positioning

本项目不是从零重做 Admin，而是对两个成熟旧项目进行革新式重构：

- 前端参考：`D:\github-project\admin_front_ts`
- 后端参考：`D:\github-project\admin_back_go`

统一原则：功能和体验以老项目成熟实现为基线；架构、协议、数据模型和工程质量以当前项目
规则为准。能直接复用的成熟功能、业务规则、页面结构、交互和 UI 应优先复用；存在历史负担
的部分在保持业务行为的前提下革新，不把老项目的重型架构和兼容包袱复制回来。

- 迁移或重构已有功能前，按任务范围查找老项目对应实现。纯前端任务只需读取相关老前端，
  纯后端任务只需读取相关老后端，只有跨端契约或完整功能切片才同时读取两端。
- 未经用户或已确认 spec 授权，不得删减老项目已有成熟功能，不得用占位页、简化流程、假数据
  或重新设计替代可迁移的完整实现。
- 老项目 UI 能适配当前 Element Plus、主题和公共组件时，保留其信息结构、工作密度和操作习惯；
  API、DTO、Store、Router、权限和 i18n 按当前项目边界接入。
- PostgreSQL、`deleted_at`、严格 DTO、显式错误和线性数据流等当前项目改进必须保留。

## Rule Priority

用户当前明确指令优先，其次是本文件、`docs/agent/README.md`、当前 spec、当前 plan、代码和测试。
老项目只作为成熟功能与体验的参考。发现规则、spec、plan 或代码互相冲突时，不得猜测并继续
实现；先核实哪个文档已被明确更新或向用户报告冲突。

## Non-Negotiable Architecture

- 普通数据库请求固定为：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

- Router 只声明 URL、中间件和 Handler 绑定；Middleware 只处理请求级关注点。
- Handler 只绑定和校验请求、传递 request context、调用 Service、输出统一响应；不得访问 GORM、
  Redis、Queue、Storage 或第三方 SDK。
- Service 负责业务规则、状态变化、事务意图和依赖调用顺序；不得依赖 Gin 或直接编写 GORM 查询。
- Repository 只访问 PostgreSQL，负责查询、写入、锁和事务；不得处理 HTTP、调用 Queue 或编排业务。
- Model 只表达 GORM 与 PostgreSQL 映射；Schema 负责明确约束、索引和可审计迁移。
- 异步消费固定为 `task handler -> service -> repository -> model -> PostgreSQL`；Queue 只封装 Asynq，
  不导入业务模块。
- 实体状态类任务只携带最小稳定标识，并从 PostgreSQL 读取业务事实。操作日志等不可变事件类任务
  可以携带完成落库所需的闭合、脱敏、版本化 DTO；任务载荷仍须严格校验且不得携带秘密。
- 从 HTTP 或 Asynq 入口取得的 `context.Context` 必须继续传给 Service、Repository、GORM、Redis、
  Queue、Storage 和第三方 I/O；不得把 `*gin.Context` 传到 Handler 或 Middleware 以下，也不得在
  调用链中途换成 `context.Background()`。
- 使用 `database`、`redis`、`queue`、`storage` 等具体名称。禁止把 `platform`、`infra` 用作模糊
  技术分组、万能目录或第三方服务统称；`authplatform` 等明确业务资源，以及认证、会话、权限和审计
  协议中含义明确的 `platform` 字段不受此限制。禁止引入通用 Adapter、Manager、Factory、BaseService、
  BaseRepository、BaseModel、DI 容器、运行时注册器或为未来假设创建的接口。

## Frontend And Protocol Rules

- 前端 HTTP 数据流固定为：

```text
view -> api/<module>.ts -> utils/request.ts -> Go API
```

- HTTP envelope 精确为 `code`、`data`、`message`；不接受或输出 `msg` 等兼容字段。
- Axios 先严格校验 envelope，业务 API 再把 `unknown` 解析并缩小为明确 DTO。页面不得猜字段、
  静默补默认值或用类型断言掩盖必填字段缺失。
- 前端业务 TypeScript 禁止显式 `any`、`any[]`、`as any`、`Record<string, any>`、`@ts-ignore`、
  宽泛 ambient declaration 和关闭 strict 选项。
- Vue Props、Emits、Pinia 状态、API 请求响应和组合式函数返回值必须使用明确类型。
- 公共组件只抽取多个真实页面已经复用的稳定交互；优先使用当前已有的 `AppDialog`、`AppTable`、
  `Search`、`DIcon` 和 `IconSelect`，不创建万能组件。
- RBAC 页面节点固定使用资源级 `:view` 后缀，只控制能否进入页面或查看资源。读取列表和单条
  数据分别使用独立 action `:list`、`:detail`，写操作使用 `:create`、`:update`、`:status`、
  `:delete` 或 `:authorize`；页面权限不得通过字符串规则自动派生任何 API 权限。
- 个人资料不是静态路由：固定使用隐藏 page `account:profile:view`，GET 使用独立 action
  `account:profile:detail`，资料保存按钮/API 使用 `account:profile:update`，修改密码按钮/API 使用
  `account:password:update`；页面权限、读取权限、动作权限和对应后端 Middleware 必须一一对应。
- `is_hidden = 1` 只表示不显示在侧边菜单，不表示路由或接口公开。隐藏页面仍由 Access 快照
  动态注册，前端入口和按钮按权限码显示，后端路由必须用同一页面/动作权限再次校验。
- Access 快照的 `menuTree` 只放 directory/page，action 只进入 `permissionCodes`；快照只能包含
  PostgreSQL 中实际存在且已授权的权限码，禁止把 `:view` 自动扩展成 `:list`、`:detail` 或其他权限。
- Element Plus 树/表格的行 key 统一按字符串处理；展开状态、搜索恢复和平台切换不得混用数字
  ID 与字符串 ID。
- RBAC Access 的权威层级固定为 PostgreSQL -> Redis -> 进程内缓存。进程内缓存只能在 Redis
  校验当前 access version 后读取；Redis 故障或版本无法确认时不得使用旧的进程缓存兜底。

## Database And Error Rules

- 每个 PostgreSQL Model 及其项目维护表都显式声明 `CreatedAt`/`created_at` 和
  `UpdatedAt`/`updated_at`，数据库类型为非空 `TIMESTAMPTZ`；不得隐藏在 `gorm.Model` 或 BaseModel。
- 有真实删除行为的表只使用 `gorm.DeletedAt`/`deleted_at`，不得增加 `is_del`、`is_deleted`
  或其他重复删除状态；没有删除行为的记录不虚构 `deleted_at`。
- Yes/No 固定为 `0 = No`、`1 = Yes`，后端只使用 `shared/yesno`，前端只使用 `enums/yes-no`；
  数据库字段使用 `SMALLINT` 和 `CHECK` 约束。
- 业务唯一值只要求有效数据唯一时，使用 `WHERE deleted_at IS NULL` 的部分唯一索引；只为真实查询
  创建索引。
- AutoMigrate 不负责删除字段、修改既有类型或回填数据；协议迁移必须显式、幂等、可审计并失败回滚。
- 缺配置、请求畸形、协议违规、依赖不可用和任务载荷不完整时返回明确错误。禁止内存实现兜底、
  空数组或假成功、吞错、静默兼容和泄露 SQL、堆栈、Token、Cookie、密码、密钥或 DSN。
- 只有当前已确认设计明确允许的降级才可执行，并且必须可观察、可测试，不能静默发生。

## Naming And Scope

- JSON 和 TypeScript 使用 lower camel case；Go 导出名使用 PascalCase，initialism 使用 `ID`、
  `HTTP`、`API`；PostgreSQL 使用 lower snake case。
- URL 使用小写复数资源；权限码使用冒号分段，例如 `system:user:list`。
- 只创建当前功能真实使用的文件、字段、表、索引、状态和抽象；不做无关重构或格式化。
- 不使用 Docker 或 Docker Compose。

## Workflow, Git And Verification

- 行为变化和缺陷遵循：失败测试 -> 确认因目标行为缺失而失败 -> 最小实现 -> 通过 -> 再重构。
- 新功能、跨模块契约、数据库或认证权限变化遵循：理解需求与相关老项目实现 -> 方案 -> 用户确认
  -> spec -> plan -> 实现。简单局部修改按 `docs/agent/README.md` 的渐进规则处理。
- 涉及菜单、路由或权限时，写代码前必须在 spec/plan 中列出页面 `:view`、读取 `:list/:detail`、
  写操作权限、隐藏状态和对应 API；发现页面与 API 共用一个权限码或存在自动扩权时先修正文档和迁移设计。
- 修改前检查 `git status --short`，保留用户和其他开发者的现有改动，不为干净工作区删除或回退它们。
- 不自动 commit、fetch、pull 或 push；除非维护者明确要求，不得 amend、rebase 或以其他方式重写
  既有提交历史。
- 用户明确授权提交时，标题使用 `<type>: <中文摘要>`，不使用 scope，一个提交只处理一个完整目的。
- 完成前运行与风险匹配的定向测试；共享或跨模块修改运行完整验证并如实报告未运行项与剩余风险。

Backend:

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

Frontend:

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```
