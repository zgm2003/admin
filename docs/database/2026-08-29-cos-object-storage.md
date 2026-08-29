# 腾讯云 COS 配置与上传规则迁移

执行前完成 PostgreSQL 备份并进入维护窗口。先执行 `2026-08-29-admin-rbac-baseline.sql`，再执行本文件；回滚使用备份恢复，不执行猜测性的 down migration。

```powershell
psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f docs/database/2026-08-29-cos-object-storage.sql
```

本迁移创建 `storage_cos_config`、`storage_upload_rule`、命名检查约束和索引，并在 Admin 平台创建 `cloud -> storage:object:list` 菜单及十个隐藏动作。`ux_storage_upload_rule_platform_enabled` 保证每个平台最多一个启用规则，也允许全部停用。API 和 Worker 启动不会执行本迁移。

验证：

```sql
SELECT column_name, data_type FROM information_schema.columns
WHERE table_name IN ('storage_cos_config', 'storage_upload_rule') ORDER BY table_name, ordinal_position;
SELECT conname FROM pg_constraint WHERE conname LIKE '%storage_%' ORDER BY conname;
SELECT indexname, indexdef FROM pg_indexes WHERE indexname LIKE '%storage_%' ORDER BY indexname;
SELECT id, parent_id, platform_id, code, menu_type, path, component_path FROM rbac_menu
WHERE code IN ('cloud', 'storage:object:list') OR code LIKE 'storage:%' ORDER BY id;
```

Execute twice in the maintenance test or staging rehearsal and confirm counts, IDs, menu parents, and role grants do not change.
