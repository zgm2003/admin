# Agent 任务路由

这是项目的 AI/开发者入口，不是运行时模块，也不是历史方案库。目标是让每次会话只加载完成当前任务所需的
最小上下文。

## 会话启动

每次任务按以下顺序读取：

1. 根 `AGENTS.md`：不可违反的硬规则。
2. 本文件：判断任务范围和额外入口。
3. `STATUS.md`：当前目标、已完成、进行中、延期和后续事项。
4. `design.md`：当前产品和交互约定。
5. `architecture.md`：当前运行时边界和数据流。
6. 目标模块的代码、测试，以及任务触发的 Skill。

不读取整棵仓库来“了解项目”。不相关的页面、模块、旧项目、SQL、spec 和 plan 都留在上下文之外。

## 任务路由

| 任务信号 | 最小额外读取 | 常见验证 |
| --- | --- | --- |
| 解释、定位单点问题 | 目标文件和直接调用方 | 定向搜索或单测 |
| 文案、样式、单文件配置 | 目标文件和相关测试 | `git diff --check`、定向测试 |
| 单模块 bug | 模块完整调用链和相关测试 | 相关 Go/Vitest 测试 |
| 标准列表/详情/创建/编辑/状态/删除 | `$admin-crud`、目标模块、必要的旧项目对应实现 | CRUD 测试和架构检查 |
| 菜单、角色、权限码、动态路由、Access、Redis 失效 | `$admin-rbac`、permission 模块、相关前端 router/store | 无权限、隐藏页、版本失效、跨平台测试 |
| 表、字段、约束、索引、迁移 | `$admin-database`、当前 PostgreSQL、Model/Repository | 真实 PostgreSQL 结构和回滚检查 |
| UI 专项整改 | 当前页面、公共组件和前端设计 Skill | 页面测试、类型检查、构建 |
| 跨模块新功能 | 先写契约和 `STATUS.md` 当前工作项，再按上表加载组合 Skill | 按影响范围扩大验证 |

三个项目 Skill 是固定施工流程，不是每次对话重新解释的提示词：

- `$admin-crud`：标准 CRUD 垂直切片，从 DTO、分层、权限到测试。
- `$admin-rbac`：菜单、权限码、动态路由、Access 快照、Redis 版本和后端 Middleware 的闭环。
- `$admin-database`：以当前 PostgreSQL 为事实来源的结构检查、forward migration、验证和可选快照。

Skill 位于 `C:\Users\IGT\.codex\skills\admin-crud`、`admin-rbac`、`admin-database`。它们引用本项目路径，
不复制历史 spec/plan；Skill 内容只有在对应任务触发时读取。

## 计划与状态

不再要求每个改动创建或维护 `docs/superpowers/specs` 和 `docs/superpowers/plans`。短任务使用当前对话中的
工作清单；需要跨模块协作时，在 `STATUS.md` 的“当前工作”写目标、范围、验收条件和下一步。完成后只更新
“已完成”；未能交付的内容写“延期/阻塞”和原因。不要回头修改历史 plan 来追求同步。

`STATUS.md` 是唯一的项目进度入口，不是详细设计替代品：稳定产品规则写 `design.md`，稳定技术边界写
`architecture.md`，一次性执行记录写在状态条目中。

## 历史档案

`docs/superpowers/**` 保存早期 spec、plan 和验证记录，保留用于审计和必要的历史追溯，但默认不读、不改、
不把其中的勾选状态当作当前进度。`docs/database/2026-*.sql` 同样是一次性历史 migration；数据库事实以当前
PostgreSQL 为准。只有用户明确要求比较历史、恢复决策或审计迁移时，才按文件名精确读取对应档案。

## 交接与完成

开始修改前记录：任务范围、相关模块、数据流、权限映射（如有）和工作区现状。完成时报告：

- 实际修改文件；
- 实际运行的命令及结果；
- 未运行项、环境阻塞和剩余风险；
- `STATUS.md` 是否更新。

不自动提交、推送、重写历史或清理其他开发者的改动。
