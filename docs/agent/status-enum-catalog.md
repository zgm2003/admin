# 持久化业务状态枚举目录

本目录是迁移基线。每个业务域独立维护数值语义；相同数字在不同域不代表相同状态。数据库使用 `SMALLINT` 与域内 `CHECK`，Go 使用 typed enum，前端使用对应的数值 union/enum。所有下拉选项都遵循 `{ label, value }`：`value` 是稳定数值协议，`label` 由固定 enum 元数据或字典 options 提供，页面不得自行写裸选项。

## Label / Value 所有权

### 固定状态 enum

状态集合、状态转换和数据库约束属于代码契约，不能由管理员在字典页面修改。后端为每个状态 enum 提供固定的 metadata（数值、i18n key、可选的 tag 类型）；前端 API parser 验证数值后，使用本地 i18n 根据 metadata 生成 `{ label, value }`。后端返回状态数值，不返回可被页面当作事实的自由文本 label。

示例：

```ts
const jobStatusOptions = computed(() =>
  schedulerJobStatuses.map((status) => ({
    value: status.value,
    label: t(status.i18nKey),
  })),
)
```

### 可配置业务字典

展示分类、业务类型、运营标签等允许管理员维护的选项使用 `system_dictionary` / `system_dictionary_item`。数据库保存稳定的数值或代码 value、启用状态、排序和中英文 label；业务 API 通过 options 端点返回严格的 `{ label, value }`。消费页面加载失败或遇到非法 value 必须报协议错误，不能偷偷回退硬编码 options。

### 禁止混用

- 不把 Job/Run/发送状态迁移到字典表，避免管理员改变状态机语义。
- 不把可配置选项复制成前端 enum，避免数据库与页面两份事实。
- 不让后端根据数字临时拼接 label，也不让前端根据数字猜文案。
- 同一个数字只在所属域内有意义；Mail 的 `1` 不得被通用状态组件解释成 Scheduler 的 `scheduled`。

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
2. API 新契约只传数值；旧字符串不在运行时双读，迁移 runner 负责一次性转换。
3. 前端 API parser 拒绝未知数值，展示文案集中在域 presentation 文件；不在页面散落裸数字。
4. 状态数字不能作为跨域通用字典；新增状态必须同步 Go、SQL、API、TypeScript、i18n 和状态转换测试。
5. 普通编辑保持 last-write-wins；状态转换的并发保护只使用数据库条件更新/事务，不新增管理员 revision。
