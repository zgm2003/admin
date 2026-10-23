-- Run manually with psql -X --set=ON_ERROR_STOP=1 --single-transaction --file=...
-- Only old built-in descriptions change; custom notes and all values survive.
-- The existing Worker outbox relay publishes the new setting generation.
DO $migration$
DECLARE
    changed_count BIGINT;
    previous_generation BIGINT;
    next_generation BIGINT;
BEGIN
    PERFORM set_config('lock_timeout', '5s', true);
    PERFORM id FROM system_setting
    WHERE deleted_at IS NULL AND setting_key IN (
        'app.brand.default_avatar', 'app.brand.title_en_us', 'app.brand.title_zh_cn',
        'app.legal.privacy_policy', 'app.legal.user_agreement',
        'auth.captcha.slide_padding', 'auth.captcha.ttl_minutes'
    ) ORDER BY setting_key FOR UPDATE;

    UPDATE system_setting AS setting
    SET description = correction.description, updated_at = CURRENT_TIMESTAMP
    FROM (VALUES
        ('app.brand.default_avatar', 'Default avatar COS object key using the avatar upload rule', '默认头像 COS 对象键（使用头像上传规则）'),
        ('app.brand.title_en_us', 'Admin English brand title', '管理后台英文品牌标题'),
        ('app.brand.title_zh_cn', 'Admin 中文品牌标题', '管理后台中文品牌标题'),
        ('app.legal.privacy_policy', 'Published privacy policy HTML', '已发布的隐私政策 HTML 内容'),
        ('app.legal.user_agreement', 'Published user agreement HTML', '已发布的用户协议 HTML 内容'),
        ('auth.captcha.slide_padding', 'Captcha slide tolerance in pixels', '滑块验证码允许偏差（像素）'),
        ('auth.captcha.ttl_minutes', 'Captcha challenge lifetime in minutes', '验证码有效期（分钟）')
    ) AS correction(setting_key, previous_description, description)
    WHERE setting.setting_key = correction.setting_key
        AND setting.description = correction.previous_description
        AND setting.is_builtin = 1 AND setting.deleted_at IS NULL;
    GET DIAGNOSTICS changed_count = ROW_COUNT;

    IF changed_count = 0 THEN
        RAISE NOTICE 'Built-in setting descriptions already corrected or customized';
        RETURN;
    END IF;

    SELECT generation INTO STRICT previous_generation FROM system_config_cache_state
    WHERE namespace = 'system.setting' AND scope_key = 'global' FOR UPDATE;
    IF previous_generation < 1 OR previous_generation = 9223372036854775807 THEN
        RAISE EXCEPTION 'Setting cache generation cannot advance';
    END IF;
    next_generation := previous_generation + 1;
    UPDATE system_config_cache_state
    SET generation = next_generation, updated_at = CURRENT_TIMESTAMP
    WHERE namespace = 'system.setting' AND scope_key = 'global';
    INSERT INTO system_config_cache_outbox (
        namespace, scope_key, generation, attempts, available_at, created_at, updated_at
    ) VALUES (
        'system.setting', 'global', next_generation, 0,
        CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
    );
    RAISE NOTICE 'Corrected % built-in descriptions; setting generation % -> %',
        changed_count, previous_generation, next_generation;
END
$migration$;
