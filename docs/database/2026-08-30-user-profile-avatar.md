# 个人资料头像字段迁移

在 `user_profile` 增加 `avatar` 字段，用于保存 COS `objectKey`。字段不保存完整 URL，空字符串
表示未设置头像。

```powershell
psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f docs/database/2026-08-30-user-profile-avatar.sql
```

当前个人资料接口只接受空字符串或 `avatar/` 开头的安全相对对象 key。
