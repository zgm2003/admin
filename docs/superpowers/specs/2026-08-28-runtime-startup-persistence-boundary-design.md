# 运行时启动与持久化边界设计

## 1. 定位

本设计固定 Admin 项目的运行时启动边界：PostgreSQL、Redis 和 Queue 中已经存在的数据是持久化事实，API 与 Worker 的正常启动不得把数据库迁移、基础数据初始化、历史兼容修复或缓存清理混入启动流程。

本文替代所有旧 spec 和 plan 中以下设计：

- API 启动时执行 `PrepareSchema`、`AutoMigrate` 或 `EnsureSchema`；
- API 启动时迁移、重命名、回填或修复 PostgreSQL 对象和数据；
- API 启动时预设或修复认证平台、菜单、角色和其他基础记录；
- API 启动时清理、重建或主动失效 Redis 状态。

旧文档的其他业务契约继续有效。发生冲突时，以本文为准。

## 2. 核心原则

正常启动只负责加载配置、连接并验证必需依赖、组装服务、注册路由或任务处理器以及开始监听。启动流程不得主动改变 PostgreSQL、Redis 或 Queue 中的业务状态。

项目不新增 `migrate`、`init`、自动 seed 或自动 repair 命令。数据库结构和基础数据只在维护者明确安排具体变更时单独处理，不由运行时进程猜测、补齐或修复。

## 3. API 启动边界

API 启动允许执行：

1. 校验 i18n、环境变量和密钥配置；
2. 创建进程级 `context.Context`；
3. 打开并验证 PostgreSQL、Redis 和 Queue 连接；
4. 创建 Repository、Service、Handler 和 Middleware；
5. 注册路由并启动 HTTP Server；
6. 在退出时关闭自身打开的连接并优雅停止服务。

API 启动禁止执行：

- 创建、删除、重命名或修改表、列、索引、约束和序列；
- `AutoMigrate`、`Prepare*Schema`、`Ensure*Schema` 或任何等价 DDL；
- 字段回填、协议迁移、历史数据修复或兼容转换；
- 插入、恢复、修改或删除认证平台、菜单、角色、用户及其他业务记录；
- 因基础数据缺失而自动补建记录；
- 清理、重建或主动失效 Redis key；
- 发布用于初始化、迁移或修复状态的 Queue 任务。

数据库缺表、缺列、缺约束或缺少业务基础数据时，启动过程不修改数据。连接本身可用时 API 正常启动；具体请求访问到缺失依赖时沿现有错误链明确失败并记录内部原因。

## 4. Worker 启动边界

Worker 继续只加载配置、验证 PostgreSQL 和 Redis、注册任务处理器并启动 Asynq Server。Worker 不执行 DDL、数据初始化、历史修复、缓存清理或初始化任务发布。

## 5. 显式业务命令

现有 `bootstrap-admin` 是维护者主动执行的首个管理员创建命令，不属于 API/Worker 启动流程，本次保留。它只在命令被明确执行时写入业务数据，不得被 API 或 Worker 自动调用。

本次不增加其他迁移、初始化或修复命令。

## 6. 代码收敛

API composition root 删除所有启动写状态调用及其不再需要的依赖。仅被已取消启动流程使用、且没有真实运行时用途的菜单 foundation 组装代码应删除。

Schema 和历史迁移代码不再从生产入口调用。测试仍可在隔离的测试 schema 中建立夹具，但测试准备逻辑不得重新进入 API 或 Worker 的运行时调用链。本次实现只处理启动边界及直接产生的死代码，不顺带重构业务 Service、Repository 或在线接口。

## 7. 验收标准

1. API 在现有数据库上能够越过原 Canvas foundation 失败点并开始监听；
2. API 启动前后 PostgreSQL 业务表内容和 Access Version 完全不变；
3. API 启动前后项目 Redis key 不因启动流程被删除、创建或改写；
4. API 入口不再引用任何 schema prepare/ensure、AutoMigrate、平台预设、菜单 foundation、系统角色预设或启动缓存清理函数；
5. Worker 保持无迁移、无初始化启动；
6. `bootstrap-admin` 仍只能由维护者显式执行；
7. 后端测试、静态检查和构建通过。

