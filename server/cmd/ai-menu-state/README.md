# AI 菜单维护

`ai-menu-state` 只读取 PostgreSQL 中的 Admin 菜单事实；显式 apply 时通过既有 `MenuStore.RebuildReadyState` 发布对应 Redis 菜单版本。它不执行 SQL、不改角色授权、不启用任务，也不枚举用户或删除快照。

先完成 AI core/seed、payment 流水扩展，并部署具备六个 AI 页面和对应 API/Worker 的版本。停止 API、Worker 及其他权限写入入口后，在仓库根目录由维护者执行：

```powershell
.\docs\database\2026-10-28-ai-menu.ps1 -ServicesStopped
```

runner 要求 Go、`psql`、`pg_dump`、`pg_restore` 可用，读取 `server/.env` 中的 `POSTGRES_DSN` 和 `REDIS_URL`。它检查无其他数据库客户端，保存受限目录中的完整备份与审计，单事务执行菜单 SQL、核对并幂等复跑，最后只发布 Admin 菜单版本。可用 `-EnvFile` 和 `-AuditDirectory` 指定环境文件与新的审计目录；不能覆盖旧审计。

从 `server` 目录预览当前数据库与 Redis 状态，不修改任一状态：

```powershell
go run ./cmd/ai-menu-state --env-file .env
```

若 PostgreSQL 已提交但 Redis 发布失败，保持服务停止。先读取审计与上面的预览结果；修复连接问题后可重新运行完整 runner，SQL 重放不会递增版本。也可以只重试已完成 SQL 核验后的发布：

```powershell
go run ./cmd/ai-menu-state --env-file .env --apply --services-stopped
```

处于 `invalidating`、损坏或版本领先的 Redis 状态不会被覆盖；先核对对应修改流程，不能删除状态来绕过检查。提交状态为 `unknown` 时必须先只读确认数据库事实，不当作已回滚，也不恢复旧写入程序。确认审计 `stage=completed`、Admin Redis 版本与 PostgreSQL 一致后，由维护者恢复服务并为所需角色显式授权页面与动作。
