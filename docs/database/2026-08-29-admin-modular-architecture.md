# Admin 模块化架构数据库迁移

## 执行

先备份 PostgreSQL，并在维护窗口停止 API/Worker。由维护者执行：

```powershell
psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f docs/database/2026-08-29-admin-modular-architecture.sql
```

脚本使用单一事务；任何前置冲突或映射失败都会抛出异常并自动回滚。API/Worker 不会执行 migration。

## 回滚

脚本不提供猜测性的 down migration。结构回退必须从维护者确认的备份恢复，并在恢复窗口停止 API/Worker。

## 验证

```sql
SELECT to_regclass('user_profile'), to_regclass('user_session'), to_regclass('user_login_log');
SELECT table_name, column_name, data_type, is_nullable FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name IN ('user_profile','user_session','user_login_log','audit_operation_log');
SELECT conname, pg_get_constraintdef(oid) FROM pg_constraint WHERE connamespace = current_schema()::regnamespace;
SELECT tablename, indexname, indexdef FROM pg_indexes WHERE schemaname = current_schema();
SELECT 'user_account' AS table_name, count(*) FROM user_account UNION ALL
SELECT 'user_session', count(*) FROM user_session UNION ALL
SELECT 'user_login_log', count(*) FROM user_login_log UNION ALL
SELECT 'audit_operation_log', count(*) FROM audit_operation_log;
```

执行前后应记录用户、角色、菜单、授权、平台、会话和审计日志的 ID 与计数，确认历史 ID 保留。
