# Admin RBAC 菜单基线迁移

这是维护期手工执行的 forward migration。执行前备份 PostgreSQL，停止 API/Worker 或按运维窗口隔离写入；先执行 Admin 模块基线，再执行本文件。

```powershell
psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f docs/database/2026-08-29-admin-rbac-baseline.sql
```

脚本在单事务中校验 Admin 平台、`account` 目录、Canvas 旧码/新码冲突和个人资料节点形状。它保留既有菜单 ID、父子关系和 `rbac_role_menu` 授权；Canvas 页面只改权限码为 `canvas:test:list`，动作仍为 `canvas:test:button`。首次发生菜单变化时，会在同一事务推进活跃用户的 `rbac_access_version`。

SQL 成功后，在维护窗口内仅删除本项目的 Redis 键 `authz:access-state:*` 和 `authz:access:*`，不得使用 `FLUSHDB`。API 和 Worker 启动不会执行迁移、插入菜单、回填版本或清理 Redis。

回滚使用备份恢复，不执行猜测性的 down migration。完成后核对：

```sql
SELECT id, parent_id, platform_id, menu_type, path, component_path, is_hidden
FROM rbac_menu
WHERE code IN ('account:profile:list', 'account:profile:update', 'account:password:update', 'canvas:test:list')
ORDER BY id;

SELECT count(*) AS old_canvas_code_count FROM rbac_menu WHERE code = 'canvas:test' AND deleted_at IS NULL;
SELECT count(*) AS role_grants FROM rbac_role_menu rm JOIN rbac_menu m ON m.id = rm.menu_id WHERE m.code IN ('canvas:test:list', 'canvas:test:button') AND rm.deleted_at IS NULL;
```

应确认旧 Canvas 页面码为 0、个人资料页面为隐藏 page、两个 action 为隐藏子节点，既有角色授权数量未减少。
