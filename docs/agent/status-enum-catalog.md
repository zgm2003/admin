# 持久化业务状态枚举目录

每个业务域独立拥有稳定数值语义；相同数字在不同域不代表相同状态。数据库使用 `SMALLINT + CHECK`，Go 使用
所属域 typed enum。前端 DTO 保留实际数字/字符串类型，候选项与本地化 label 由后端提供，不另维护合法值名单。

## Label / Value 所有权

- 枚举常量、合法值校验、默认值、限制和状态机均属于后端代码；选项通过所属模块认证 `/options` 返回
  `{value,label}`，必要时带表单 constraints/展示 tone。HTTP Middleware 提供请求语言，模块负责本地化。
- 静态 options 不读 PostgreSQL/Redis。未来动态分类或运营数据建立具名业务资源；不恢复通用字典表、管理页、
  DictService 或运行时注册器。地域、扩展名和 MIME 的预设同样归业务代码；可创建值由后端验证。
- 前端 API 仅类型和调用；页面用 options 或列表/详情 label 展示原值，不自行拼接 `status - 1` 文案。
  未知展示值显示原值/中性样式，禁止映射为已知业务状态。options 加载失败清空并提示，不偷偷硬编码兜底。
- 可操作结论由后端 DTO 的 actions 提供，前端与独立 Access action 交叉检查；动作结论不是安全边界，后端写入
  仍检查最新事实、权限、锁和事务。列表 actions 基于已有行与一次 actor 上下文，不引入逐行附加查询。
- 前端保留交互状态、即时表单 rules、浏览器文件和路由/实时运输；浏览器展示能力 union 不等于业务合法值名单。

## Scheduler

### Job status

| code | Go constant | 语义 | 终态 |
|---:|---|---|---|
| 1 | `JobScheduled` | 等待发布 | 否 |
| 2 | `JobQueued` | 已进入队列 | 否 |
| 3 | `JobRunning` | Worker 执行中 | 否 |
| 4 | `JobCompleted` | 成功完成 | 是 |
| 5 | `JobFailed` | 执行失败且不再重试 | 是 |
| 6 | `JobCanceled` | 被取消 | 是 |

允许转换：`scheduled -> queued -> running -> completed`；`running -> failed`；`running -> scheduled`（可重试）；`scheduled/queued -> canceled`。其他转换由 Service 拒绝。

### Run status

| code | Go constant | 语义 | 终态 |
|---:|---|---|---|
| 1 | `RunRunning` | 本次尝试执行中 | 否 |
| 2 | `RunSucceeded` | 本次尝试成功 | 是 |
| 3 | `RunFailed` | 本次尝试失败 | 是 |

### Schedule enabled state

计划当前使用 `is_enabled SMALLINT`，继续遵循项目 Yes/No：`0=disabled`、`1=enabled`，不额外创建 status 字段。

## Mail / SMS log status

两种渠道各自使用相同业务语义但独立类型：`1=pending`、`2=sent`、`3=failed`。Mail/SMS 的 `scene` 保持固定协议常量，不迁入字典；Mail 额外补数据库 scene CHECK。

## Notification task status

`1=draft`、`2=scheduled`、`3=queued`、`4=processing`、`5=completed`、`6=failed`、`7=canceled`。草稿可编辑；提交后内容与受众冻结；终态为 completed/failed/canceled。

## 迁移规则

1. 每个域先增加 typed enum 与失败契约测试，再执行数据回填和 `SMALLINT` 类型迁移。
2. 持久化状态字段保持数字，展示 options/label/actions 可同时返回；旧状态字符串不运行时双读，迁移 runner 一次转换。
3. 后端拒绝非法写值；前端不再以 parser 拒绝新增状态，展示只消费所属域 options/label/actions，未知值显示原值。
4. 状态数字不能作为跨域通用字典；新增状态同步 Go、SQL、后端 options/本地化与转换测试，DTO形状变更才同步前端类型。
5. 普通编辑保持 last-write-wins；状态转换的并发保护只使用数据库条件更新/事务，不新增管理员 revision。
