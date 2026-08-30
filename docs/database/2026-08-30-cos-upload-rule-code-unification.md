# COS 上传规则编码统一迁移

本迁移在已执行 `2026-08-29-cos-object-storage.sql` 的数据库上执行。`storage_upload_rule.code`
统一作为业务上传权限编码和 COS 对象 key 前缀，删除冗余的 `path_prefix` 与 `max_file_count`，并
移除“每个平台只能有一条启用规则”的唯一索引。多条上传规则可以同时启用，单选/多选由业务代码
和 `UpMedia` 组件控制。

```powershell
psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f docs/database/2026-08-30-cos-upload-rule-code-unification.sql
```

迁移保留现有 `code` 值，因此历史业务调用不会因为字段合并而改变；新对象会使用该 `code` 作为
前缀。迁移不改动 COS 中已有对象。
