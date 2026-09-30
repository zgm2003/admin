-- Run offline via the same-name PowerShell runner after verifying the COS object.
BEGIN;
SELECT set_config('migration.template_source_url', :'template_source_url', true),
       set_config('migration.template_object_key', :'template_object_key', true);

DO $migration$
DECLARE
    old_key constant text := 'message.mail.recipient_rule.import_template_url';
    new_key constant text := 'message.mail.recipient_rule.import_template_object_key';
    source_url text := current_setting('migration.template_source_url');
    object_key text := current_setting('migration.template_object_key');
    previous system_setting%ROWTYPE;
    target system_setting%ROWTYPE;
    next_generation bigint;
    changed_at timestamptz := clock_timestamp();
BEGIN
    IF object_key = '' OR length(object_key) > 1024 OR position('..' in object_key) > 0
       OR object_key !~ '^([a-z0-9][a-z0-9._-]*/)*[a-z0-9][a-z0-9._-]*/[.]admin-storage/v2/p[1-9][0-9]*/r[1-9][0-9]*/c[1-9][0-9]*/v[1-9][0-9]*/[0-9]{4}/[0-9]{2}/[0-9]{2}/[0-9a-f]{32}[.]csv$' THEN
        RAISE EXCEPTION 'A verified v2 CSV object key is required';
    END IF;
    LOCK TABLE system_setting IN SHARE ROW EXCLUSIVE MODE;
    SELECT * INTO previous FROM system_setting WHERE setting_key=old_key AND deleted_at IS NULL FOR UPDATE;
    SELECT * INTO target FROM system_setting WHERE setting_key=new_key AND deleted_at IS NULL FOR UPDATE;
    IF target.id IS NOT NULL THEN
        IF previous.id IS NOT NULL OR target.value<>object_key OR target.value_type<>1 OR target.is_builtin<>1 OR target.is_enabled<>1 THEN
            RAISE EXCEPTION 'Conflicting template object-key setting';
        END IF;
        RETURN;
    END IF;
    IF previous.id IS NULL OR previous.value<>source_url OR previous.value_type<>1 OR previous.is_builtin<>1 OR previous.is_enabled<>1 THEN
        RAISE EXCEPTION 'Source template setting changed or is unavailable';
    END IF;
    UPDATE system_setting SET setting_key=new_key, value=object_key,
        description='邮件收件规则 CSV 模板对象键（不含协议与域名）', updated_at=changed_at
        WHERE id=previous.id;
    UPDATE system_config_cache_generation SET generation=generation+1, updated_at=changed_at
        WHERE namespace='system.setting' AND scope_key='global'
        RETURNING generation INTO next_generation;
    IF next_generation IS NULL THEN
        RAISE EXCEPTION 'system.setting/global generation is missing';
    END IF;
    INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,attempts,available_at,created_at,updated_at)
        VALUES ('system.setting','global',next_generation,0,changed_at,changed_at,changed_at);
END
$migration$;
COMMIT;
