# COS 上传规则多编码迁移

本迁移在已执行 `2026-08-30-cos-upload-rule-code-unification.sql` 的数据库上执行。它将
`storage_upload_rule.code` 迁移到 `storage_upload_rule_code` 子表，使一条规则可以拥有多个
稳定上传编码；每个编码在同一认证平台内唯一。历史规则的现有编码会自动迁移，规则限制和 COS
配置不变。

```powershell
psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f docs/database/2026-08-30-cos-upload-rule-multiple-codes.sql
```

编码创建后不可修改。删除规则时子编码跟随软删除，因此已删除规则的编码可以在同一平台重新使用。
