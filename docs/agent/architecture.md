# 当前系统架构

本文描述当前仓库可观察到的运行时边界。它是 Agent 的快速导航，不替代目标模块中的接口、测试和实现。

## 组件与入口

```text
web (Vue 3) -> Go API (Gin/GORM) -> PostgreSQL
                    |       |
                    v       v
                  Redis   Asynq -> Go Worker -> PostgreSQL
```

| 组件 | 入口/目录 | 责任 |
| --- | --- | --- |
| Web | `web/src/main.ts`、`web/src/views` | 页面、动态路由、状态和 API 调用 |
| API | `server/cmd/api/main.go` | 配置、依赖装配、全局 Middleware、HTTP 服务 |
| 业务模块 | `server/internal/module/*` | 各域的 Handler、Service、Repository、Model、Schema 和测试 |
| 数据库 | `server/internal/database` | PostgreSQL 连接与结构准备边界 |
| Redis | `server/internal/redis`、模块内 Redis 文件 | 连接、会话/权限缓存和失效协调 |
| Queue | `server/internal/queue` | Asynq 客户端/服务端封装，不导入业务模块 |
| Worker | `server/cmd/worker/main.go` | 注册任务并按任务 Handler -> Service -> Repository 消费 |

当前模块目录包括：`auth`、`health`、`permission`、`user`、`message`、`storage`、`system`。目录存在不代表
所有业务都已完成，状态以 `STATUS.md` 和测试为准。

## 同步请求

```text
HTTP -> global middleware -> route middleware -> Handler
     -> Service -> Repository -> GORM Model -> PostgreSQL
```

- 全局 Middleware 在 API 入口统一处理 request ID、CORS、访问日志、操作日志、Recovery 和语言。
- `/api/v1` 承载共享认证/Access 接口；`/api/admin/v1` 额外要求 Admin 平台标识。
- Router 只声明路径、中间件和 Handler。Handler 将 `c.Request.Context()` 传入 Service，并输出统一 envelope。
- Service 编排业务规则、状态变化、事务意图和外部依赖顺序。Repository 通过 `WithContext(ctx)` 访问
  PostgreSQL；Model 只映射表结构。
- 依赖在 `cmd/api/main.go` 显式创建，例如 Repository -> Service -> Handler；不使用容器、运行时注册或万能
  基类。

## 异步任务

```text
Queue client -> Redis/Asynq -> Worker task handler -> Service -> Repository -> PostgreSQL
```

实体状态任务只携带最小稳定标识，消费时重新从 PostgreSQL 读取事实；不可变操作事件可携带闭合、脱敏、版本化
DTO。入队和消费边界都必须校验载荷，任务 Handler 不直接写数据库。

## 前端数据流

```text
web/src/views/** -> web/src/api/<module>.ts -> web/src/utils/request.ts -> Go API
```

Router 和 Access Store 根据后端快照动态注册业务页面。API 模块负责 DTO 解析，不把请求放进公共组件；页面
只编排视图状态。Element Plus 树/表格行 key 统一为字符串，不能混用数字 ID。

## RBAC 与缓存

```text
PostgreSQL (权限事实) -> Redis (access version/快照) -> 进程内有界快照
```

每次使用进程内快照前先向 Redis 确认当前 access version；版本不一致时重建或读取 Redis 快照。Redis 故障、
版本无法确认或正在失效时，不返回旧进程缓存、空权限或假成功；按当前明确策略显式失败或回源 PostgreSQL。
菜单树只包含 directory/page，动作权限只进入 `permissionCodes`，页面隐藏状态不改变路由和 API 保护。

## 数据库事实与迁移

- 当前 PostgreSQL 是唯一权威来源；连接、健康检查和就绪检查走 `server/internal/database`。
- API/Worker 启动不执行 DDL、seed、回填或 Redis 全库清理。结构变更由维护者执行幂等、可审计的 forward SQL。
- `docs/database/2026-*.sql` 是历史执行记录，不是当前 schema 的默认知识。需要快照时从真实 PostgreSQL 导出
  `docs/database/current.sql`，发现不一致以数据库为准。
- 维护表显式使用 `TIMESTAMPTZ` 的 `created_at`/`updated_at`；真实删除才使用 `deleted_at`；索引和约束只
  为真实查询及业务不变量创建。

## 代码定位

| 要查的内容 | 先看 |
| --- | --- |
| API 路由和依赖装配 | `server/cmd/api/main.go`、目标模块 `route.go` |
| 认证/权限 | `server/internal/module/auth`、`server/internal/module/permission` |
| 用户与账户 | `server/internal/module/user` |
| 前端路由和 Access | `web/src/router`、`web/src/store`、`web/src/api/permission` |
| 公共请求和错误 | `web/src/utils/request.ts` |
| 页面行为 | 对应 `web/src/views/<domain>` 及镜像测试 `web/tests` |

