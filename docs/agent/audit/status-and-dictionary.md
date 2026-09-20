# 状态、字典与 options 审计

## 结论

固定状态机和可运营选项必须分开：

- 固定状态：数据库 `SMALLINT + CHECK`、Go typed enum、API 数值、前端 enum metadata + i18n。
- 可配置选项：`system_dictionary` / `system_dictionary_item`，后端 options API 返回严格 `{label,value}`。
- 事件类型、第三方协议值：保留字符串。
- Yes/No 继续项目约定 `0/1`，不与业务状态混用。

## 待迁移状态

- `message_mail_log.status`
- `message_sms_log.status`
- `message_notification_task.status`
- `system_scheduler_job.status`
- `system_scheduler_run.status`

已建立编码目录：`docs/agent/status-enum-catalog.md`。Scheduler 先迁，其他域按同一契约分批迁移。

