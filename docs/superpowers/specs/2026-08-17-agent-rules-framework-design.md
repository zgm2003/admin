# Admin Agent 施工规范框架设计说明

## 1. 背景与目标

项目已经有一套经过实践的线性架构约束，但规则分散在基础设计、加固设计、计划和 `AGENTS.md` 中。下一个 AI 如果只看局部文件，容易重新引入 `any`、Adapter、Manager、兜底逻辑或跨层调用。

本设计建立一个简单的文档驱动 Agent 规则框架，让任何接手项目的 AI 都能从同一入口按任务规模读取必要上下文、设计、施工和验证。它不是运行时框架，不引入 Agent 服务、插件系统、任务调度器、依赖注入或新的代码架构。

目标：

- 用 `AGENTS.md` 作为项目级硬规则入口；
- 用 `docs/agent/README.md` 保存完整施工流程、边界和检查清单；
- 将 TypeScript 类型安全，特别是禁止业务 `any`，提升为硬规则；
- 将 RBAC 页面 `:view`、独立读取/写入动作权限和隐藏页面动态路由提升为硬规则；
- 明确 PostgreSQL、Redis、进程内缓存的 RBAC 三层关系和本地缓存失效边界；
- 让 spec、plan、代码、测试和 Git 操作有明确路由，小任务不加载无关上下文；
- 让 AI 遇到不确定性时停下来报告，而不是猜默认值或静默兜底。

## 2. 文档职责与读取顺序

### 2.1 权威层级

规则按以下优先级解释：

1. 用户当前明确指令；
2. `AGENTS.md` 的项目硬规则；
3. `docs/agent/README.md` 的施工流程和示例；
4. 当前功能的 spec；
5. 当前功能的 plan；
6. 代码、测试和 README 中的现状说明。

如果文档之间冲突，AI 必须停止实现并报告冲突；不能自行选择一个规则继续。用户确认后，先修改权威文档，再修改代码。

### 2.2 固定入口和渐进式读取

`AGENTS.md` 保持短小，是每次任务的固定入口。进入后由 `docs/agent/README.md` 的任务路由决定还需要读取什么，不允许为了一个局部问题全量阅读所有 spec、plan 和模块。

读取范围按任务大小确定：

| 任务类型 | 必须读取 | 不需要读取 |
| --- | --- | --- |
| 解释代码、查看状态、回答单点问题 | `AGENTS.md`、目标文件及直接调用方 | 全量基础 spec、无关 plan、其他模块 |
| 单文件配置、文案、样式或窄测试修改 | `AGENTS.md`、Agent 指南对应章节、目标文件和相关测试 | 全量系统设计和无关业务文档 |
| 单模块 bug 修复 | `AGENTS.md`、相关 Agent 规则、该模块调用链、相关 spec 章节和测试 | 其他模块 spec 和历史 plan |
| 新功能、跨模块契约、数据库结构、认证权限 | 完整 Agent 指南、对应完整 spec、完整 plan 和相关代码 | 与当前功能无关的历史设计 |
| 执行一份已经批准的 plan | 完整当前 plan、其对应 spec、涉及的 Agent 规则和代码 | 其他已完成 plan |

“相关 spec 章节”必须覆盖本次会改变的契约、数据流、错误和验收标准，但不要求从头重读整份基础设计。只有变更触及全局架构或无法判断局部规则时，才扩大阅读范围。

禁止用“上下文可能有用”为理由批量读取全部仓库文档。每读取一个额外文档，AI 都应能说明它与当前决策的直接关系。

### 2.3 文件职责

- `AGENTS.md`：只放必须立即遵守的短规则、目录边界和验证命令，不堆长篇教程；
- `docs/agent/README.md`：解释规则为什么存在、如何施工、如何判断完成，作为所有 AI 的统一操作手册；
- `docs/superpowers/specs/`：描述一次需求的目标、边界、数据流、错误和验收标准；
- `docs/superpowers/plans/`：把已批准的 spec 拆成可执行步骤和测试命令；
- `README.md`：面向人类开发者的启动、环境和项目说明，不替代 Agent 硬规则。

`AGENTS.md` 与 `docs/agent/README.md` 不复制每个业务字段的完整定义。前者只给硬约束，后者给解释和检查方法，业务细节放入具体 spec。

## 3. TypeScript 类型安全规范

### 3.1 硬规则

前端已经使用 TypeScript，因此业务代码禁止使用裸 `any`。以下写法全部禁止：

```ts
let value: any
const values: any[] = []
const result = response as any
type Payload = Record<string, any>
```

禁止通过 `@ts-ignore`、`@ts-expect-error`、关闭 `noImplicitAny` 或扩大公共类型来绕过错误。第三方库缺失类型时，只能在独立声明文件中补充当前调用所需的最小类型，并说明原因。

### 3.2 替代方式

- 不确定的外部数据使用 `unknown`；
- 经过运行时校验后再缩小为具体 interface/type；
- API 请求和响应必须有明确 DTO；
- Vue 组件 Props、Emits、Store 状态和组合式函数返回值必须有明确类型；
- 有限稳定值使用 `as const` 对象或联合类型；
- 动态键值必须定义键和值的真实类型，不使用 `Record<string, any>`；
- JSON 解码结果先校验结构，不能直接断言为业务类型。

现有 HTTP 响应 envelope 仍严格是 `code`、`data`、`message`。Axios 只能把经过协议校验的 `data` 交给业务 API，页面不能重新猜字段或使用 `??` 掩盖必填字段缺失。

### 3.3 RBAC 页面与动作权限命名

页面权限是路由入口的稳定协议，统一使用资源级 `:view` 后缀，不因页面是列表、详情还是
单例资料页而改变。例如个人资料页面固定使用 `account:profile:view`。列表、单条详情、页面内
按钮和对应后端接口使用独立动作权限，例如 `account:profile:detail`、`account:profile:update`
与 `account:password:update`；页面权限不得自动派生这些 API 权限。

隐藏页面的 `is_hidden=1` 只影响侧边菜单展示。动态路由仍须由 Access 快照注册，前端按钮和
后端 Middleware 都必须检查同一权限码；任何静态路由、前端 `v-if` 或强制跳转都不能替代后端
授权。新增或迁移菜单时，计划和测试必须明确页面 `:view`、读取 `:list/:detail`、写动作权限及其 API 对应关系。

### 3.4 类型检查验收

每次前端改动至少运行：

```powershell
cd D:\admin\web
pnpm vitest run
pnpm build
```

需要检查新增业务代码是否出现裸 `any`：

```powershell
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" web\src -g "*.ts" -g "*.vue"
```

命令有输出时，AI 必须逐处解释并移除；测试夹具也不能用 `any` 逃避类型检查。

### 3.5 Element Plus 树表状态

Element Plus 树/表格组件的行 key 统一在进入状态层时规范化为字符串。全部展开、全部收起、
搜索恢复和平台切换必须使用同一 `String(id)` 集合；不得把数字 ID 直接传给
`expand-row-keys`。这条规则属于 UI 协议，不通过额外通用组件或隐式类型转换规避，并且必须
有对应的交互回归测试。

## 4. 线性架构硬规则

### 4.1 后端调用链

普通数据库请求固定为：

```text
router -> middleware -> handler -> service -> repository -> model -> PostgreSQL
```

Handler 不访问 GORM；Service 不依赖 Gin；Repository 只访问 PostgreSQL；Queue 只封装 Asynq；Redis、Storage 和第三方 client 只负责各自真实协议。

禁止引入 `Platform Adapter`、`Adapter`、`infra`、`Manager`、`Factory`、`BaseService`、`BaseRepository`、万能 `common`、运行时注册器、DI 容器或为了未来替换而存在的接口。只有当前已经存在多个真实实现或测试存在真实替换边界时，才创建最小接口。

### 4.2 上下文和错误

HTTP 或 Asynq 入口取得的 `context.Context` 必须原样传过 Service、Repository、GORM、Redis、Queue 和 Storage。禁止把 `*gin.Context` 传到 Handler/Middleware 以外。

错误必须沿原调用链返回，并转换为项目统一错误。禁止：

- 缺配置时使用默认值；
- 依赖不可用时切换内存实现；
- 失败时返回空数组或假成功；
- 捕获错误后只打印日志不返回；
- 为兼容旧代码同时接受 `msg` 和 `message`；
- 把数据库错误、堆栈、Token 或密码写入 HTTP 响应。

### 4.3 RBAC 三层缓存

RBAC 权限事实和失效版本的来源顺序固定为 PostgreSQL -> Redis -> 进程内缓存。进程内缓存是
有界、带 TTL 的不可变快照，只能在本次请求先从 Redis 确认 `accessVersion` 为 `ready` 且版本
匹配后读取；key 至少包括平台 ID、平台 code、policy version、用户 ID 和 access version。
Redis 状态为 `invalidating`、读取失败或版本无法确认时，禁止使用旧的进程快照，按已确认的
降级规则回源 PostgreSQL 并记录缓存结果。不得为了省一次 Redis 访问而引入无界或不可观察的
本地权限缓存。

### 4.4 命名

- Go 导出名称使用 PascalCase，initialism 使用 `ID`、`HTTP`、`API`；
- PostgreSQL 使用 lower snake case；
- JSON、TypeScript 使用 lower camel case；
- URL 使用小写复数资源名；
- 权限码使用冒号分段，例如 `system:user:list`；
- 三方服务按具体技术名命名，例如 `redis`、`queue`、`storage` 或具体 integration，不使用 `platform` 统称。

## 5. 数据库硬规则

- 每张系统维护表显式声明 `created_at TIMESTAMPTZ NOT NULL` 和 `updated_at TIMESTAMPTZ NOT NULL`；
- Go 模型直接声明字段，不嵌入 `gorm.Model` 或项目级 BaseModel；
- 有真实删除语义的表使用 `deleted_at TIMESTAMPTZ NULL`/`gorm.DeletedAt`，不使用 `is_del`、`is_deleted` 或双字段状态；
- `deleted_at IS NULL` 表示有效，非空表示已删除；普通查询排除已删除数据；
- Yes/No 固定 `0=否`、`1=是`，数据库字段使用 `SMALLINT` 和 `CHECK (column IN (0, 1))`；
- Yes/No 代码不可被字典管理页面修改；
- 业务唯一值只要求对有效数据唯一时使用 `WHERE deleted_at IS NULL` 的部分唯一索引；
- 不为没有真实查询需求的公共字段机械加索引；
- 不用 AutoMigrate 删除字段、改类型或做数据回填；生产发布需要版本化 migration 时另写 spec。

## 6. Agent 施工流程

### 6.1 开始前

1. 判断任务是只读问题、局部修改、单模块 bug，还是跨模块/契约变化；
2. 按 2.2 的路由读取最小充分上下文，不全量扫描无关文档；
3. 需要修改时再检查工作区、当前分支、已有改动和相关服务状态；
4. 判断需求是否足够明确；不明确时一次只问一个问题；
5. 对新功能、跨模块或契约变化先写设计并等待确认；简单局部修改只需在执行前说明变更和验证方式；
6. 需要多步骤实施时才写 plan，列出精确文件、失败测试、实现步骤和验证命令。

### 6.2 实现中

每个行为变更都遵循：

```text
写失败测试 -> 确认按预期失败 -> 最小实现 -> 确认通过 -> 再重构
```

如果测试失败原因不是预期功能缺失，先修正测试或调查环境，不修改实现来掩盖问题。遇到和规范冲突的要求、缺失权限、外部服务阻塞或危险操作时停止并报告。

### 6.3 完成前

- 对照本次实际使用的 spec 章节和 plan 步骤逐项核对，不重新加载无关文档，也不“顺手”扩展范围；
- 运行后端 `go fmt ./...`、`go vet ./...`、`go test ./...`、`go build ./...`；
- 运行前端 `pnpm vitest run`、`pnpm build`；
- 检查新增 TypeScript 代码没有裸 `any`；
- 检查 `git diff --check`、服务是否由用户管理、是否产生了未授权的 commit/push；
- 汇报实际命令输出，不用“应该可以”代替证据。

## 7. Git 和交付边界

- Git 初始化、远程地址和分支设置必须按用户明确指令执行；
- AI 不自动 commit、push、pull、fetch 或删除用户改动；
- 发现其他进程产生的提交或改动时只读核查并保留，不回滚；
- 服务启动、停止和端口变更只有用户明确授权时执行；
- 最终报告必须列出修改文件、验证命令、未完成项和剩余风险。

## 8. Agent 交接清单

下一个 AI 接手编码任务时，必须能回答与当前范围相关的问题：

- 当前任务对应的 spec 和 plan 是什么？
- 工作区和 Git 是否有非本次改动？
- 数据流从哪里开始，经过哪些具体文件？
- 必填字段、错误 envelope 和依赖失败行为是什么？
- 是否需要软删除、Yes/No 编码或 `TIMESTAMPTZ`？
- 前端 DTO、Props 和返回值的明确类型在哪里？
- 已运行哪些验证，哪些验证因环境没有运行？
- 当前页面入口是否使用 `:view`，列表/详情/写接口是否使用独立动作权限？
- RBAC 读取是否经过 Redis 版本门控，进程缓存 key 和失效边界是什么？

只回答局部问题时不要求回答全部 7 项。需要修改代码时，如果与本次范围相关的问题无法回答，不能直接写代码，应补读对应内容或向用户提问；不相关的问题不扩展阅读。

## 9. 落地范围

本设计批准后只做以下文档落地：

- 修改根 `AGENTS.md`：加入 `docs/agent/README.md` 入口、TypeScript 禁止 `any` 规则，并把“改代码前全量读取基础 spec/plan”改为 2.2 的渐进式路由；
- 创建 `docs/agent/README.md`：写入任务分类、按需阅读路由、线性架构、RBAC 页面/动作权限、三层缓存、类型、数据库、错误、TDD、Git 和交接清单；
- 修改根 `README.md`：增加 Agent 施工指南入口，方便人类开发者知道 AI 受哪些规则约束；
- 不修改 Go、Vue、数据库模型或运行时行为。

### 9.1 2026-08-29 规则增补

后续实现明确了页面入口、读取 API 必须独立授权，且 RBAC 进程内缓存和隐藏页面路由边界未在统一
入口中明确。因此本框架增补以下不可协商规则，并同步落地到 `AGENTS.md` 和
`docs/agent/README.md`：

- 每个页面节点（包括个人资料等单例页面）必须以 `:view` 作为入口权限；
- 列表、详情、页面内按钮和后端接口必须使用独立动作权限，前端显示控制不能替代后端鉴权；
- 页面 `:view` 不得通过字符串规则自动扩展为 `:list`、`:detail` 或其他 API 权限；
- `is_hidden=1` 只影响菜单展示，路由仍由 Access 快照动态注册；
- 进程内权限快照必须先经过 Redis access version 门控，不能以旧本地值静默放行；
- 菜单展开状态必须统一使用字符串 row key，全部展开不能退化为全部收起。

## 10. 非目标

本框架不实现：

- Agent 运行时、任务队列、插件市场或多 Agent 调度器；
- 自动修改业务代码的脚本；
- 第二套与 `AGENTS.md` 平行的规则入口；
- 通过 ESLint、代码生成器或复杂 CI 强行替代基本的类型设计和人工审查。
