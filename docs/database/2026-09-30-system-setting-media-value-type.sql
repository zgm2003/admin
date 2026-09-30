-- Offline forward migration. value stays TEXT; only its metadata type changes.
BEGIN;
DO $migration$
DECLARE
    definition text;
    changed boolean := false;
    affected bigint;
    next_generation bigint;
    changed_at timestamptz := clock_timestamp();
    row record;
    parts text[];
    y integer;
    m integer;
    d integer;
    max_day integer;
BEGIN
    LOCK TABLE system_setting IN ACCESS EXCLUSIVE MODE;
    SELECT pg_get_constraintdef(oid) INTO definition FROM pg_constraint
        WHERE conrelid='system_setting'::regclass AND conname='ck_system_setting_value_type';
    IF definition IS NULL OR definition NOT IN (
        'CHECK ((value_type = ANY (ARRAY[1, 2, 3, 4])))',
        'CHECK ((value_type = ANY (ARRAY[1, 2, 3, 4, 5])))'
    ) THEN RAISE EXCEPTION 'Unexpected system_setting value_type constraint'; END IF;

    IF (SELECT count(*) FROM system_setting WHERE deleted_at IS NULL AND setting_key IN
        ('app.brand.default_avatar','message.mail.recipient_rule.import_template_object_key')) <> 2 THEN
        RAISE EXCEPTION 'Both builtin media settings must already exist';
    END IF;
    FOR row IN SELECT * FROM system_setting WHERE deleted_at IS NULL AND setting_key IN
        ('app.brand.default_avatar','message.mail.recipient_rule.import_template_object_key') FOR UPDATE
    LOOP
        IF row.value_type NOT IN (1,5) OR row.is_enabled<>1 OR row.is_builtin<>1 THEN
            RAISE EXCEPTION 'Builtin media setting metadata is invalid';
        END IF;
        IF row.value <> '' THEN
            IF length(row.value)>1024 OR position('..' in row.value)>0 OR row.value ~ '[[:cntrl:]]'
               OR row.value !~ '^([a-z0-9][a-z0-9._-]*/)*[a-z0-9][a-z0-9._-]*/[.]admin-storage/v2/p[1-9][0-9]*/r[1-9][0-9]*/c[1-9][0-9]*/v[1-9][0-9]*/[0-9]{4}/[0-9]{2}/[0-9]{2}/[0-9a-f]{32}[.][a-z0-9]{1,16}$'
               OR length(split_part(row.value,'/.admin-storage/v2/',1))>64 THEN
                RAISE EXCEPTION 'Media value must be a standard object key, not a URL';
            END IF;
            parts := string_to_array(split_part(row.value,'/.admin-storage/v2/',2),'/');
            PERFORM substring(parts[1] from 2)::bigint, substring(parts[2] from 2)::bigint,
                    substring(parts[3] from 2)::bigint, substring(parts[4] from 2)::bigint;
            y := parts[5]::integer; m := parts[6]::integer; d := parts[7]::integer;
            max_day := CASE WHEN m IN (1,3,5,7,8,10,12) THEN 31 WHEN m IN (4,6,9,11) THEN 30
                WHEN m=2 THEN CASE WHEN y%400=0 OR (y%4=0 AND y%100<>0) THEN 29 ELSE 28 END ELSE 0 END;
            IF d<1 OR d>max_day THEN RAISE EXCEPTION 'Media object key date is invalid'; END IF;
            IF (row.setting_key='app.brand.default_avatar' AND row.value !~ '[.](jpg|jpeg|png|gif|webp)$')
               OR (row.setting_key='message.mail.recipient_rule.import_template_object_key' AND row.value !~ '[.]csv$') THEN
                RAISE EXCEPTION 'Builtin media object extension is invalid';
            END IF;
        END IF;
    END LOOP;

    IF definition='CHECK ((value_type = ANY (ARRAY[1, 2, 3, 4])))' THEN
        ALTER TABLE system_setting DROP CONSTRAINT ck_system_setting_value_type;
        ALTER TABLE system_setting ADD CONSTRAINT ck_system_setting_value_type CHECK(value_type IN (1,2,3,4,5));
        changed := true;
    END IF;
    UPDATE system_setting SET value_type=5, updated_at=changed_at
        WHERE deleted_at IS NULL AND value_type=1 AND setting_key IN
        ('app.brand.default_avatar','message.mail.recipient_rule.import_template_object_key');
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF changed OR affected>0 THEN
        UPDATE system_config_cache_generation SET generation=generation+1,updated_at=changed_at
            WHERE namespace='system.setting' AND scope_key='global' RETURNING generation INTO next_generation;
        IF next_generation IS NULL THEN RAISE EXCEPTION 'system.setting/global generation missing'; END IF;
        INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,attempts,available_at,created_at,updated_at)
            VALUES ('system.setting','global',next_generation,0,changed_at,changed_at,changed_at);
    END IF;
END
$migration$;
COMMENT ON COLUMN system_setting.value_type IS '配置值类型：1=字符串，2=数字，3=布尔，4=JSON，5=媒体对象键';
COMMIT;
