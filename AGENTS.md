# Admin 项目硬规则

本文件是 `D:\admin` 的最小硬规则入口。用户当前指令优先；其次是本文件；然后按
`docs/agent/README.md` 路由读取 `STATUS.md`、`design.md`、`architecture.md`、任务相关代码、测试和
Skill。`docs/superpowers/**` 与带日期的旧 SQL 是历史档案，除非用户明确要求追溯，否则不得作为当前行为或
设计依据。

## 项目边界

本项目是对以下成熟旧项目的革新式重构：

- 前端参考：`D:\github-project\admin_front_ts`
- 后端参考：`D:\github-project\admin_back_go`

旧项目只提供已验证的业务行为、信息结构和 UI 参考；当前代码、协议和本文件决定最终实现。迁移时保留成熟
功能，适配当前 DTO、权限、主题和组件，不复制历史兼容负担，也不使用占位页、假数据或无业务价值的抽象。

## 架构边界

普通数据库请求固定为：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

异步消费固定为：

```text
task handler -> service -> repository -> model -> PostgreSQL
```

- Router 只绑定 URL、中间件和 Handler；Middleware 只处理请求级认证、权限、日志、CORS、语言和 Recovery。
- Handler 只绑定/校验请求、传递 `Request.Context()`、调用 Service、输出响应；不得访问 GORM、Redis、Queue、
  Storage 或第三方 SDK。
- Service 负责业务规则、状态变化、事务意图和依赖顺序；不得依赖 Gin 或直接编写 GORM 查询。
- Repository 只访问 PostgreSQL，负责查询、写入、锁和事务；不得处理 HTTP 或编排业务。
- Model 只表达 GORM/PostgreSQL 映射；Queue 只封装 Asynq，不导入业务模块。
- 依赖在 `cmd/api/main.go` 和 `cmd/worker/main.go` 显式装配。禁止 DI 容器、运行时注册器、通用
  `Adapter`/`Manager`/`Factory`/`Base*` 或模糊的 `platform`/`infra` 技术目录。
- 所有入口取得的 `context.Context` 必须贯穿 Service、Repository、GORM、Redis、Queue、Storage 和第三方 I/O；
  不得传递 `*gin.Context` 或中途换成 `context.Background()`。

## 前端与协议

前端数据流固定为：

```text
view -> api/<module>.ts -> utils/request.ts -> Go API
```

- HTTP envelope 只有 `code`、`data`、`message`。`utils/request.ts` 统一处理业务、HTTP、网络和协议错误；
  自动刷新期间的中间 401 不弹通知，终态 401/403 由请求层通知一次。登录接口的凭据错误由登录页内联展示，
  请求层不得对同一错误重复通知；页面与组件不得重复弹同一接口错误。
- API 模块从 `unknown` 严格解析 DTO；View、Props、Emits、Pinia 和组合式函数使用明确类型。业务代码禁止
  `any`、`as any`、`@ts-ignore`、宽泛 ambient declaration 和关闭 strict。
- 优先复用现有 `AppDialog`、`AppTable`、`Search`、`DIcon`、`IconSelect`；公共组件只抽取多个真实页面已
  复用的稳定交互。
- JSON/TypeScript 使用 lower camel case；Go 导出名使用 PascalCase，initialism 使用 `ID`/`HTTP`/`API`；
  URL 使用小写复数资源名。

## RBAC 契约

- 页面节点使用资源级 `:view`，只决定能否进入页面；列表、详情和写操作分别使用独立的
  `:list`、`:detail`、`:create`、`:update`、`:status`、`:delete` 或 `:authorize`。
- 页面和 API 不共用权限码，不从 `:view` 字符串自动派生动作权限；每个 API 的后端 Middleware 使用同一
  action code 再校验。
- `is_hidden = 1` 只隐藏侧边菜单，隐藏页面仍由 Access 快照动态注册；`menuTree` 只包含 directory/page，
  action 只进入 `permissionCodes`。
- 权限事实层级固定为 PostgreSQL -> Redis -> 进程内缓存。进程内缓存必须先经 Redis 确认 access version；
  Redis 故障或版本无法确认时不得返回旧缓存、空权限或假成功。
- 个人资料固定使用隐藏 page `account:profile:view`，读取 `account:profile:detail`，保存
  `account:profile:update`，改密 `account:password:update`。

## 数据库与错误

- PostgreSQL 是唯一运行时事实来源。每个维护表和 Go Model 显式声明非空 `created_at`/`updated_at`
  (`TIMESTAMPTZ`)，不嵌入 `gorm.Model` 或 BaseModel。
- 有真实删除行为的表只使用 `deleted_at`/`gorm.DeletedAt`；业务唯一值需要忽略已删除记录时使用
  `WHERE deleted_at IS NULL` 部分唯一索引。没有删除行为的表不增加该字段。
- Yes/No 固定为 `0 = No`、`1 = Yes`，数据库使用 `SMALLINT + CHECK`，后端使用 `shared/yesno`，前端使用
  `enums/yes-no`。
- DDL、数据回填和 seed 只通过维护者执行的幂等、可审计 forward migration；API/Worker 启动不得执行。历史
  `docs/database/*.sql` 不作为默认知识；迁移成功后可从真实 PostgreSQL 导出 `docs/database/current.sql`。
- 错误必须返回并映射为明确公开语义；禁止缺配置猜默认值、内存兜底、空数组/假成功、吞错，或泄露 SQL、
  堆栈、Token、Cookie、密码、密钥和 DSN。

## 工作流与验证

- 先检查 `git status --short`，保留其他开发者的改动；不自动 commit、fetch、pull、push、rebase 或回滚。
- 只读和局部修改按 `docs/agent/README.md` 渐进路由处理，不为每个小改动创建历史 spec/plan。需要长期记录
  的目标、进度、延期和后续事项只写 `docs/agent/STATUS.md`。
- 行为变化遵循失败测试 -> 最小实现 -> 通过 -> 再重构；新功能先完成契约和权限映射，再实现。
- 未经明确授权不执行 commit、fetch、pull、push、amend、rebase 或其他历史重写。授权提交时标题必须使用
  `<type>: <中文摘要>`，不使用 scope；一个提交只处理一个完整目的，正式提交不得使用 `wip`。
- 提交前使用 `git status --short` 和 `git diff --cached` 核对暂存区，排除密钥、环境文件、构建产物、临时文件
  和无关文档，并运行与改动风险匹配的验证。不得为得到干净工作区删除或回退其他开发者改动。
- 完成前运行与风险匹配的定向检查。共享后端改动可运行：

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

共享前端改动可运行：

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

- 不使用 Docker 或 Docker Compose。最终报告列出实际修改、实际命令、未运行项和剩余风险。
