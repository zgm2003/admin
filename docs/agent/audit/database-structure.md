# PostgreSQL 结构审计

事实来源：当前 `public` schema 的只读 catalog 查询；`docs/database/current.sql` 仅作为快照。

## P1/P2

- 当前约 46 张表、519 列、148 索引、668 约束、58 外键；未发现完全无代码引用的表。
- `user_session` 同时存在 `fk_auth_session_user`、`fk_user_session_user` 两个相同外键。
- `user_session.device_id` 有 6 条空字符串；字段虽 NOT NULL，但缺少非空 CHECK。
- `permission_access_version.user_id` 有 `rbac_access_version_user_id_seq` 默认值；该字段同时是用户主键/外键，不应自增。
- `message_mail_log.scene` 缺少固定场景 CHECK；SMS 对应字段已有约束。
- `permission_menu` 存在重复/覆盖索引候选；删除前必须用 `EXPLAIN` 与线上索引统计验证。
- `system_scheduler_job.ux_system_scheduler_job_task_attempt(id,attempt_count)` 被主键覆盖，删除前需验证实际计划依赖。
- `realtime_event.event_type` 代码固定集合与 DB 非空约束不一致；需决定固定 CHECK 或版本化事件目录。
- Mail/SMS `template_id` 语义不同：Mail 保存 Provider Template ID，SMS 保存内部模板 ID，不能直接套同一 FK。
- Mail/SMS log `latency_ms` 缺少非负 CHECK。

## 已生成但未执行的 migration

- `docs/database/2026-09-20-schema-hardening.sql`
- `docs/database/2026-09-20-schema-hardening.ps1`

脚本修复重复 FK、清理空设备会话、增加 device CHECK、移除 access version 自增默认值和无用 sequence。真实执行需要维护者停 API/Worker 并显式运行 runner。

