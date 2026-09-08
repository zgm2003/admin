-- Offline forward migration. Drain all old API / Worker / maintenance writers before execution.
-- Preserve menu IDs / role grants and configured mail counts/windows.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';
-- Rename the existing relation, never copy it. PostgreSQL keeps OID-bound FKs
-- and sequence defaults attached. Ambiguous or missing source state is an error.
DO $$
DECLARE
  schema_name text := current_schema();
  old_table regclass := to_regclass(format('%I.auth_platform', current_schema()));
  new_table regclass := to_regclass(format('%I.permission_auth_platform', current_schema()));
  item record;
BEGIN
  IF old_table IS NOT NULL AND new_table IS NOT NULL THEN
    RAISE EXCEPTION 'Both platform tables exist; refusing to merge identities';
  END IF;
  IF old_table IS NULL AND new_table IS NULL THEN
    RAISE EXCEPTION 'Platform table missing';
  END IF;
  IF old_table IS NOT NULL THEN
    EXECUTE format('ALTER TABLE %I.auth_platform RENAME TO permission_auth_platform', schema_name);
  END IF;
  EXECUTE format('LOCK TABLE %I.permission_auth_platform IN ACCESS EXCLUSIVE MODE', schema_name);
  FOR item IN SELECT conname FROM pg_constraint
    WHERE conrelid=to_regclass(format('%I.permission_auth_platform', schema_name))
      AND (starts_with(conname,'auth_platform_') OR starts_with(conname,'ck_auth_platform_'))
  LOOP
    EXECUTE format('ALTER TABLE %I.permission_auth_platform RENAME CONSTRAINT %I TO %I',
      schema_name, item.conname, replace(item.conname,'auth_platform','permission_auth_platform'));
  END LOOP;
  FOR item IN SELECT indexname FROM pg_indexes WHERE schemaname=schema_name
    AND tablename='permission_auth_platform' AND starts_with(indexname,'ux_auth_platform_')
  LOOP
    EXECUTE format('ALTER INDEX %I.%I RENAME TO %I', schema_name, item.indexname,
      replace(item.indexname,'auth_platform','permission_auth_platform'));
  END LOOP;
  IF to_regclass(format('%I.auth_platform_id_seq',schema_name)) IS NOT NULL THEN
    IF to_regclass(format('%I.auth_platform_id_seq',schema_name)) IS DISTINCT FROM
      pg_get_serial_sequence(format('%I.permission_auth_platform',schema_name),'id')::regclass THEN
      RAISE EXCEPTION 'Unexpected platform sequence ownership';
    END IF;
    EXECUTE format('ALTER SEQUENCE %I.auth_platform_id_seq RENAME TO permission_auth_platform_id_seq',schema_name);
  END IF;
END $$;
LOCK TABLE permission_menu, permission_access_version, message_mail_rate_limit_policy IN SHARE ROW EXCLUSIVE MODE;

CREATE TEMP TABLE naming_prefix (old_value text PRIMARY KEY, new_value text NOT NULL) ON COMMIT DROP;
INSERT INTO naming_prefix VALUES
  ('auth:platform:', 'permission:authplatform:'),
  ('account:user:loginlog:', 'user:loginlog:'),
  ('account:user:roles', 'user:account:authorize'),
  ('account:user:', 'user:account:'),
  ('account:profile:', 'user:profile:'),
  ('account:password:', 'user:password:'),
  ('auth:session:', 'user:session:'),
  ('system:operation-log:', 'system:operationlog:'),
  ('storage:cos-config:', 'storage:cosconfig:'),
  ('storage:upload-rule:', 'storage:uploadrule:');
CREATE TEMP TABLE naming_path (old_value text PRIMARY KEY, new_value text NOT NULL) ON COMMIT DROP;
INSERT INTO naming_path VALUES
  ('account/users', 'user/account'),
  ('account/profile', 'user/profile'),
  ('account/sessions', 'user/session'),
  ('account/login-logs', 'user/loginlog'),
  ('permission/auth-platforms', 'permission/authplatform'),
  ('auth/platform', 'permission/authplatform'),
  ('permission/menus', 'permission/menu'),
  ('permission/roles', 'permission/role'),
  ('cloud/storage-object', 'storage/object'),
  ('system/operation-logs', 'system/operationlog');
CREATE TEMP TABLE naming_i18n (old_value text PRIMARY KEY, new_value text NOT NULL) ON COMMIT DROP;
INSERT INTO naming_i18n VALUES
  ('navigation.accountUsers', 'navigation.userAccount'),
  ('navigation.accountSessions', 'navigation.userSession'),
  ('navigation.accountLoginLogs', 'navigation.userLoginlog'),
  ('navigation.account', 'navigation.user'),
  ('navigation.accessAuthPlatforms', 'navigation.permissionAuthplatform'),
  ('navigation.authPlatform', 'navigation.permissionAuthplatform'),
  ('navigation.accessMenus', 'navigation.permissionMenu'),
  ('navigation.accessRoles', 'navigation.permissionRole'),
  ('navigation.access', 'navigation.permission'),
  ('navigation.systemOperationLogs', 'navigation.systemOperationlog'),
  ('navigation.cloud', 'navigation.storage'),
  ('layout.account.profile', 'layout.user.profile');
CREATE TEMP TABLE naming_changed (id bigint) ON COMMIT DROP;

-- Longest prefix wins for the former account:user:loginlog and :roles codes.
WITH candidates AS (
  SELECT m.id, (SELECT p.new_value || substr(m.code, length(p.old_value)+1)
    FROM naming_prefix p WHERE starts_with(m.code,p.old_value)
    ORDER BY length(p.old_value) DESC LIMIT 1) AS next_code
  FROM permission_menu m
), updated AS (
  UPDATE permission_menu m SET code=c.next_code, updated_at=CURRENT_TIMESTAMP
  FROM candidates c WHERE m.id=c.id AND c.next_code IS NOT NULL AND m.code IS DISTINCT FROM c.next_code
  RETURNING m.id
) INSERT INTO naming_changed SELECT id FROM updated;

WITH updated AS (
  UPDATE permission_menu SET code=CASE code WHEN 'account' THEN 'user' WHEN 'cloud' THEN 'storage' WHEN 'access' THEN 'permission' END,
    updated_at=CURRENT_TIMESTAMP
  WHERE code IN ('account','cloud','access') RETURNING id
) INSERT INTO naming_changed SELECT id FROM updated;

WITH updated AS (
  UPDATE permission_menu m SET component_path=p.new_value, path='/'||p.new_value, updated_at=CURRENT_TIMESTAMP
  FROM naming_path p WHERE m.component_path=p.old_value RETURNING m.id
) INSERT INTO naming_changed SELECT id FROM updated;

WITH updated AS (
  UPDATE permission_menu m SET i18n_key=p.new_value, updated_at=CURRENT_TIMESTAMP
  FROM naming_i18n p WHERE m.i18n_key=p.old_value RETURNING m.id
) INSERT INTO naming_changed SELECT id FROM updated;

UPDATE permission_access_version SET version=version+1, updated_at=CURRENT_TIMESTAMP
WHERE EXISTS (SELECT 1 FROM naming_changed);

DO $$
BEGIN
  IF (SELECT count(*) FROM message_mail_rate_limit_policy
      WHERE policy_key IN ('business_email_minute','business_email_10m')) <> 2 THEN
    RAISE EXCEPTION 'Both existing email policies are required; refusing to invent quotas';
  END IF;
  IF EXISTS (SELECT 1 FROM message_mail_rate_limit_policy WHERE policy_key NOT IN
    ('business_email_minute','business_email_10m','business_ip_minute','business_scene_minute',
     'admin_test_user_10m','admin_test_ip_minute','admin_test_email_10m')) THEN
    RAISE EXCEPTION 'Unexpected mail policy requires manual review';
  END IF;
END $$;
ALTER TABLE message_mail_rate_limit_policy DROP CONSTRAINT IF EXISTS ck_message_mail_rate_limit_policy_shape;
DELETE FROM message_mail_rate_limit_policy WHERE policy_key IN
  ('business_ip_minute','business_scene_minute','admin_test_user_10m','admin_test_ip_minute','admin_test_email_10m');
UPDATE message_mail_rate_limit_policy
SET dimension='platform_email', revision=revision+1, updated_at=CURRENT_TIMESTAMP
WHERE dimension IS DISTINCT FROM 'platform_email';
ALTER TABLE message_mail_rate_limit_policy ADD CONSTRAINT ck_message_mail_rate_limit_policy_shape
  CHECK (policy_key IN ('business_email_minute','business_email_10m') AND mode='business' AND dimension='platform_email');
COMMIT;
