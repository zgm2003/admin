BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

INSERT INTO system_setting (setting_key, value, value_type, description, is_enabled, is_builtin, created_at, updated_at)
SELECT 'app.brand.title_zh_cn', '智澜', 1, 'Admin 中文品牌标题', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM system_setting WHERE setting_key = 'app.brand.title_zh_cn' AND deleted_at IS NULL);

INSERT INTO system_setting (setting_key, value, value_type, description, is_enabled, is_builtin, created_at, updated_at)
SELECT 'app.brand.title_en_us', 'ZHILAN', 1, 'Admin English brand title', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM system_setting WHERE setting_key = 'app.brand.title_en_us' AND deleted_at IS NULL);

INSERT INTO system_setting (setting_key, value, value_type, description, is_enabled, is_builtin, created_at, updated_at)
SELECT 'app.brand.default_avatar', '', 1, 'Default avatar COS object key using the avatar upload rule', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM system_setting WHERE setting_key = 'app.brand.default_avatar' AND deleted_at IS NULL);

COMMIT;
