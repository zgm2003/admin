-- 统一二值启用状态列：is_enabled -> status。
-- 只重命名列、约束和相关索引，不改变任何状态值（0=停用，1=启用）或业务数据。
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $migration$
DECLARE
    item record;
    old_exists boolean;
    new_exists boolean;
    target_oid oid;
    target_column smallint;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('schema.status_columns.2026-10-23', 0));
    FOR item IN
        SELECT * FROM (VALUES
            ('user_account'),
            ('permission_auth_platform'),
            ('permission_role'),
            ('permission_menu'),
            ('message_mail_config'),
            ('message_mail_template'),
            ('message_mail_recipient_rule'),
            ('message_sms_config'),
            ('message_sms_template'),
            ('message_sms_recipient_rule'),
            ('storage_cos_config'),
            ('storage_upload_rule'),
            ('system_setting'),
            ('system_scheduler_schedule'),
            ('payment_config'),
            ('payment_recharge_package')
        ) AS tables(table_name)
    LOOP
        target_oid := to_regclass(format('%I.%I',current_schema(),item.table_name));
        IF target_oid IS NULL THEN RAISE EXCEPTION 'required table % is missing',item.table_name; END IF;
        EXECUTE format('LOCK TABLE %I.%I IN ACCESS EXCLUSIVE MODE',current_schema(),item.table_name);
        SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=target_oid AND attname='is_enabled' AND NOT attisdropped),
               EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=target_oid AND attname='status' AND NOT attisdropped)
          INTO old_exists,new_exists;
        IF old_exists AND new_exists THEN RAISE EXCEPTION 'table % contains both is_enabled and status',item.table_name; END IF;
        IF NOT old_exists AND NOT new_exists THEN RAISE EXCEPTION 'table % has no binary status column',item.table_name; END IF;
        SELECT attnum INTO target_column FROM pg_attribute WHERE attrelid=target_oid AND attname=CASE WHEN old_exists THEN 'is_enabled' ELSE 'status' END AND atttypid='smallint'::regtype AND attnotnull AND NOT attisdropped;
        IF target_column IS NULL THEN RAISE EXCEPTION 'table % status must be SMALLINT NOT NULL',item.table_name; END IF;
        IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=target_oid AND contype='c' AND convalidated AND conkey=ARRAY[target_column]) THEN
            RAISE EXCEPTION 'table % requires a validated binary status CHECK',item.table_name;
        END IF;
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = current_schema() AND table_name = item.table_name AND column_name = 'is_enabled'
        ) THEN
            EXECUTE format('ALTER TABLE %I.%I RENAME COLUMN is_enabled TO status', current_schema(), item.table_name);
        ELSIF NOT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = current_schema() AND table_name = item.table_name AND column_name = 'status'
        ) THEN
            RAISE EXCEPTION 'table %.% does not contain is_enabled or status', current_schema(), item.table_name;
        END IF;
    END LOOP;
END
$migration$;

DO $migration$
DECLARE
    item record;
BEGIN
    FOR item IN
        SELECT * FROM (VALUES
            ('user_account', 'ck_user_account_is_enabled', 'ck_user_account_status'),
            ('user_account', 'user_account_is_enabled_not_null', 'user_account_status_not_null'),
            ('permission_auth_platform', 'ck_permission_auth_platform_is_enabled', 'ck_permission_auth_platform_status'),
            ('permission_auth_platform', 'permission_auth_platform_is_enabled_not_null', 'permission_auth_platform_status_not_null'),
            ('permission_role', 'ck_rbac_role_is_enabled', 'ck_rbac_role_status'),
            ('permission_role', 'rbac_role_is_enabled_not_null', 'rbac_role_status_not_null'),
            ('permission_menu', 'ck_rbac_menu_is_enabled', 'ck_rbac_menu_status'),
            ('permission_menu', 'rbac_menu_is_enabled_not_null', 'rbac_menu_status_not_null'),
            ('message_mail_config', 'message_mail_config_is_enabled_check', 'message_mail_config_status_check'),
            ('message_mail_config', 'message_mail_config_is_enabled_not_null', 'message_mail_config_status_not_null'),
            ('message_mail_template', 'message_mail_template_is_enabled_check', 'message_mail_template_status_check'),
            ('message_mail_template', 'message_mail_template_is_enabled_not_null', 'message_mail_template_status_not_null'),
            ('message_mail_recipient_rule', 'message_mail_recipient_rule_is_enabled_check', 'message_mail_recipient_rule_status_check'),
            ('message_mail_recipient_rule', 'message_mail_recipient_rule_is_enabled_not_null', 'message_mail_recipient_rule_status_not_null'),
            ('message_sms_config', 'ck_message_sms_config_is_enabled', 'ck_message_sms_config_status'),
            ('message_sms_config', 'message_sms_config_is_enabled_not_null', 'message_sms_config_status_not_null'),
            ('message_sms_template', 'ck_message_sms_template_is_enabled', 'ck_message_sms_template_status'),
            ('message_sms_template', 'message_sms_template_is_enabled_not_null', 'message_sms_template_status_not_null'),
            ('message_sms_recipient_rule', 'ck_message_sms_recipient_rule_is_enabled', 'ck_message_sms_recipient_rule_status'),
            ('message_sms_recipient_rule', 'message_sms_recipient_rule_is_enabled_not_null', 'message_sms_recipient_rule_status_not_null'),
            ('payment_config', 'ck_payment_config_is_enabled', 'ck_payment_config_status'),
            ('payment_config', 'payment_config_is_enabled_not_null', 'payment_config_status_not_null'),
            ('payment_recharge_package', 'ck_payment_recharge_package_enabled', 'ck_payment_recharge_package_status'),
            ('payment_recharge_package', 'payment_recharge_package_is_enabled_not_null', 'payment_recharge_package_status_not_null'),
            ('storage_cos_config', 'ck_storage_cos_config_is_enabled', 'ck_storage_cos_config_status'),
            ('storage_cos_config', 'storage_cos_config_is_enabled_not_null', 'storage_cos_config_status_not_null'),
            ('storage_upload_rule', 'ck_storage_upload_rule_is_enabled', 'ck_storage_upload_rule_status'),
            ('storage_upload_rule', 'storage_upload_rule_is_enabled_not_null', 'storage_upload_rule_status_not_null'),
            ('system_scheduler_schedule', 'ck_system_scheduler_schedule_enabled', 'ck_system_scheduler_schedule_status'),
            ('system_scheduler_schedule', 'system_scheduler_schedule_is_enabled_not_null', 'system_scheduler_schedule_status_not_null'),
            ('system_setting', 'ck_system_setting_is_enabled', 'ck_system_setting_status'),
            ('system_setting', 'system_setting_is_enabled_not_null', 'system_setting_status_not_null')
        ) AS constraints(table_name, old_name, new_name)
    LOOP
        IF EXISTS (
            SELECT 1 FROM pg_constraint c
            JOIN pg_class t ON t.oid = c.conrelid
            JOIN pg_namespace n ON n.oid = t.relnamespace
            WHERE n.nspname = current_schema() AND t.relname = item.table_name AND c.conname = item.old_name
        ) THEN
            IF EXISTS(SELECT 1 FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname=current_schema() AND t.relname=item.table_name AND c.conname=item.new_name) THEN
                RAISE EXCEPTION 'constraint rename conflict on %.%',item.table_name,item.new_name;
            END IF;
            EXECUTE format('ALTER TABLE %I.%I RENAME CONSTRAINT %I TO %I', current_schema(), item.table_name, item.old_name, item.new_name);
        END IF;
    END LOOP;
END
$migration$;

DO $migration$
DECLARE
    item record;
BEGIN
    FOR item IN
        SELECT * FROM (VALUES
            ('storage_cos_config', 'ix_storage_cos_config_enabled_created_at', 'ix_storage_cos_config_status_created_at'),
            ('storage_upload_rule', 'ix_storage_upload_rule_config_enabled_created_at', 'ix_storage_upload_rule_config_status_created_at'),
            ('storage_upload_rule', 'ux_storage_upload_rule_platform_enabled', 'ux_storage_upload_rule_platform_status'),
            ('system_scheduler_schedule', 'ix_system_scheduler_schedule_due', 'ix_system_scheduler_schedule_due_status'),
            ('system_setting', 'ix_system_setting_enabled_key', 'ix_system_setting_status_key')
        ) AS indexes(table_name, old_name, new_name)
    LOOP
        IF to_regclass(format('%I.%I', current_schema(), item.old_name)) IS NOT NULL THEN
            IF to_regclass(format('%I.%I', current_schema(), item.new_name)) IS NOT NULL THEN
                RAISE EXCEPTION 'index rename conflict: %',item.new_name;
            END IF;
            IF NOT EXISTS(SELECT 1 FROM pg_index WHERE indexrelid=to_regclass(format('%I.%I',current_schema(),item.old_name)) AND indrelid=to_regclass(format('%I.%I',current_schema(),item.table_name)) AND indisvalid) THEN
                RAISE EXCEPTION 'source index % has unexpected ownership or is invalid',item.old_name;
            END IF;
            EXECUTE format('ALTER INDEX %I.%I RENAME TO %I', current_schema(), item.old_name, item.new_name);
        ELSIF NOT EXISTS(SELECT 1 FROM pg_index WHERE indexrelid=to_regclass(format('%I.%I',current_schema(),item.new_name)) AND indrelid=to_regclass(format('%I.%I',current_schema(),item.table_name)) AND indisvalid) THEN
            RAISE EXCEPTION 'required index % is missing or invalid',item.new_name;
        END IF;
    END LOOP;
END
$migration$;
