-- Forward migration for the administrator phone-change audit contract.
-- action: 1=change, 2=bind. Plaintext values are only exposed through the
-- administrator user-management detail action.
BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
BEGIN
  IF to_regclass(current_schema() || '.user_phone_change_log') IS NULL THEN
    RAISE EXCEPTION 'user_phone_change_log table is required';
  END IF;
END $$;

LOCK TABLE user_phone_change_log IN SHARE ROW EXCLUSIVE MODE;

ALTER TABLE user_phone_change_log
  DROP CONSTRAINT IF EXISTS ck_user_phone_change_log_action,
  DROP CONSTRAINT IF EXISTS user_phone_change_log_action_check,
  ADD COLUMN IF NOT EXISTS old_phone VARCHAR(32),
  ADD COLUMN IF NOT EXISTS new_phone VARCHAR(32);

DO $$
DECLARE action_type text;
BEGIN
  SELECT data_type INTO action_type FROM information_schema.columns
   WHERE table_schema=current_schema() AND table_name='user_phone_change_log' AND column_name='action';
  IF action_type IN ('character varying','text') THEN
    IF EXISTS (SELECT 1 FROM user_phone_change_log WHERE action NOT IN ('bind','change')) THEN
      RAISE EXCEPTION 'user_phone_change_log contains an unknown action';
    END IF;
    ALTER TABLE user_phone_change_log ALTER COLUMN action TYPE SMALLINT
      USING CASE action WHEN 'change' THEN 1 WHEN 'bind' THEN 2 END;
  ELSIF action_type = 'smallint' THEN
    IF EXISTS (SELECT 1 FROM user_phone_change_log WHERE action NOT IN (1,2)) THEN
      RAISE EXCEPTION 'user_phone_change_log contains an unknown numeric action';
    END IF;
  ELSE
    RAISE EXCEPTION 'user_phone_change_log.action must be varchar/text or smallint, got %', action_type;
  END IF;
END $$;

-- Historical rows contain only hints/HMACs, so the migration refuses to guess
-- plaintext values. The current database has no rows; future rows are written
-- with both plaintext values by the service transaction.
DO $$
DECLARE unresolved bigint;
BEGIN
  SELECT count(*) INTO unresolved FROM user_phone_change_log
   WHERE btrim(COALESCE(new_phone,'')) = ''
      OR (action = 1 AND btrim(COALESCE(old_phone,'')) = '')
      OR (action = 2 AND old_phone IS NOT NULL);
  IF unresolved > 0 THEN
    RAISE EXCEPTION 'user_phone_change_log contains % rows with unrecoverable phone facts', unresolved;
  END IF;
END $$;

ALTER TABLE user_phone_change_log ALTER COLUMN new_phone SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_phone_change_log'::regclass AND conname='ck_user_phone_change_log_action') THEN
    ALTER TABLE user_phone_change_log ADD CONSTRAINT ck_user_phone_change_log_action CHECK (action IN (1,2));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_phone_change_log'::regclass AND conname='ck_user_phone_change_log_phone_shape') THEN
    ALTER TABLE user_phone_change_log ADD CONSTRAINT ck_user_phone_change_log_phone_shape CHECK (
      new_phone ~ '^[+]861[3-9][0-9]{9}$' AND (old_phone IS NULL OR old_phone ~ '^[+]861[3-9][0-9]{9}$')
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_phone_change_log'::regclass AND conname='ck_user_phone_change_log_old_phone') THEN
    ALTER TABLE user_phone_change_log ADD CONSTRAINT ck_user_phone_change_log_old_phone CHECK (
      (action=1 AND old_phone IS NOT NULL) OR (action=2 AND old_phone IS NULL)
    );
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_user_phone_change_log_user_created_id_desc
  ON user_phone_change_log (user_id, created_at DESC, id DESC);

DO $$
DECLARE admin_platform_id bigint; account_page_id bigint; detail_exists boolean;
BEGIN
  SELECT id INTO admin_platform_id FROM permission_auth_platform
   WHERE code='admin' AND deleted_at IS NULL LIMIT 1;
  IF admin_platform_id IS NULL THEN RAISE EXCEPTION 'admin platform is required'; END IF;
  SELECT id INTO account_page_id FROM permission_menu
   WHERE platform_id=admin_platform_id AND code='user:account:view' AND menu_type='page' AND deleted_at IS NULL LIMIT 1;
  IF account_page_id IS NULL THEN RAISE EXCEPTION 'user:account:view page is required on admin platform'; END IF;
  SELECT EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND code='user:account:detail' AND deleted_at IS NULL) INTO detail_exists;
  IF NOT detail_exists THEN
    INSERT INTO permission_menu
      (platform_id,parent_id,menu_type,code,i18n_key,path,component_path,icon,sort_order,is_enabled,is_hidden,name,remark,created_at,updated_at)
    VALUES (admin_platform_id,account_page_id,'action','user:account:detail',NULL,NULL,NULL,NULL,50,1,1,'查看用户身份变更记录','Read plaintext email and phone change history in user management',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
    UPDATE permission_auth_platform SET menu_version=menu_version+1,updated_at=CURRENT_TIMESTAMP WHERE id=admin_platform_id;
  END IF;
  UPDATE permission_menu
     SET name='查看用户身份变更记录',
         remark='Read plaintext email and phone change history in user management',
         updated_at=CURRENT_TIMESTAMP
   WHERE platform_id=admin_platform_id
     AND code='user:account:detail'
     AND deleted_at IS NULL
     AND (name IS DISTINCT FROM '查看用户身份变更记录'
       OR remark IS DISTINCT FROM 'Read plaintext email and phone change history in user management');
END $$;

COMMIT;
