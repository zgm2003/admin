# Admin 模块化架构数据库迁移

## 执行

先备份 PostgreSQL，并在维护窗口停止 API/Worker。由维护者执行：

```powershell
psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f docs/database/2026-08-29-admin-modular-architecture.sql
```

脚本使用单一事务；任何前置冲突或映射失败都会抛出异常并自动回滚。API/Worker 不会执行 migration。
SQL 成功后，在维护窗口内仅删除本项目的 Redis 键 `authz:permission-state:*` 和 `authz:permission:*`，使 RBAC 三层缓存重新从数据库构建；不得使用 `FLUSHDB`。然后再启动 API/Worker。

## 回滚

脚本不提供猜测性的 down migration。结构回退必须从维护者确认的备份恢复，并在恢复窗口停止 API/Worker。

## 验证

```sql
SELECT to_regclass('user_profile'), to_regclass('user_session'), to_regclass('user_login_log');
SELECT table_name, column_name, data_type, is_nullable FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name IN ('user_profile','user_session','user_login_log','system_operation_log');
SELECT conname, pg_get_constraintdef(oid) FROM pg_constraint WHERE connamespace = current_schema()::regnamespace;
SELECT tablename, indexname, indexdef FROM pg_indexes WHERE schemaname = current_schema();
SELECT 'user_account' AS table_name, count(*) FROM user_account UNION ALL
SELECT 'user_session', count(*) FROM user_session UNION ALL
SELECT 'user_login_log', count(*) FROM user_login_log UNION ALL
SELECT 'system_operation_log', count(*) FROM system_operation_log;
```

执行前后应记录用户、角色、菜单、授权、平台、会话和审计日志的 ID 与计数，确认历史 ID 保留。

## 当前实施结果

- `auth_session` 已迁移为 `user_session`，持久化平台使用 `platform_id` 外键。
- 已创建 `user_profile`、`user_login_log`，并删除示例表 `foundation_task`。
- `system_operation_log` 已使用 `platform_id`，历史 ID 与记录保留。
- API 和 Worker 启动路径不执行本 migration，也不执行 DDL、seed 或数据回填。
- 本脚本已在 2026-08-29 对当前本地数据库重复执行并通过，第二次执行未插入重复数据。
