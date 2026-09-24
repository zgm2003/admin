-- Forward-only migration for the user login audit contract.
-- event_type: 1=register, 2=login, 3=logout.
-- login_type: 1=password, 2=email, 3=phone.
BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
BEGIN
  IF to_regclass(current_schema() || '.user_login_log') IS NULL THEN
    RAISE EXCEPTION 'user_login_log table is required';
  END IF;
END $$;

LOCK TABLE user_login_log IN SHARE ROW EXCLUSIVE MODE;

ALTER TABLE user_login_log
  DROP CONSTRAINT IF EXISTS ck_user_login_log_event_type,
  DROP CONSTRAINT IF EXISTS ck_user_login_log_login_type,
  DROP CONSTRAINT IF EXISTS ck_user_login_log_is_success,
  DROP CONSTRAINT IF EXISTS ck_user_login_log_account;

-- Validate and backfill the historical logout display account before renaming
-- login_account. Missing usernames are a migration error, never a fake value.
DO $$
DECLARE
  account_column text;
  unresolved bigint;
BEGIN
  SELECT CASE
    WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='login_account') THEN 'login_account'
    WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='account') THEN 'account'
    ELSE NULL
  END INTO account_column;
  IF account_column IS NULL THEN
    RAISE EXCEPTION 'user_login_log account column is required';
  END IF;

  IF account_column = 'login_account' THEN
    EXECUTE format($sql$
      UPDATE user_login_log AS log
         SET login_account = app_user.username
        FROM user_account AS app_user
       WHERE btrim(COALESCE(log.event_type::text, '')) IN ('logout', '3')
         AND btrim(COALESCE(log.login_account, '')) = ''
         AND log.user_id = app_user.id
         AND btrim(app_user.username) <> ''
    $sql$);
  ELSE
    EXECUTE $sql$
      UPDATE user_login_log AS log
         SET account = app_user.username
        FROM user_account AS app_user
       WHERE btrim(COALESCE(log.event_type::text, '')) IN ('logout', '3')
         AND btrim(COALESCE(log.account, '')) = ''
         AND log.user_id = app_user.id
         AND btrim(app_user.username) <> ''
    $sql$;
  END IF;

  EXECUTE format('SELECT count(*) FROM user_login_log WHERE btrim(COALESCE(%I, '''')) = ''''', account_column) INTO unresolved;
  IF unresolved > 0 THEN
    RAISE EXCEPTION 'user_login_log contains % empty account values after backfill', unresolved;
  END IF;
END $$;

DO $$
DECLARE
  event_data_type text;
  login_data_type text;
BEGIN
  SELECT data_type INTO event_data_type
    FROM information_schema.columns
   WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='event_type';
  SELECT data_type INTO login_data_type
    FROM information_schema.columns
   WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='login_type';

  IF event_data_type IN ('character varying', 'text') THEN
    IF EXISTS (SELECT 1 FROM user_login_log WHERE event_type NOT IN ('login', 'logout')) THEN
      RAISE EXCEPTION 'user_login_log contains an unknown event_type';
    END IF;
    ALTER TABLE user_login_log ALTER COLUMN event_type TYPE SMALLINT USING CASE event_type WHEN 'login' THEN 2 WHEN 'logout' THEN 3 END;
  ELSIF event_data_type = 'smallint' THEN
    IF EXISTS (SELECT 1 FROM user_login_log WHERE event_type NOT IN (1, 2, 3)) THEN
      RAISE EXCEPTION 'user_login_log contains an unknown numeric event_type';
    END IF;
  ELSE
    RAISE EXCEPTION 'user_login_log.event_type must be varchar/text or smallint, got %', event_data_type;
  END IF;

  IF login_data_type IN ('character varying', 'text') THEN
    IF EXISTS (SELECT 1 FROM user_login_log WHERE login_type IS NOT NULL AND login_type NOT IN ('password', 'email', 'phone')) THEN
      RAISE EXCEPTION 'user_login_log contains an unknown login_type';
    END IF;
    ALTER TABLE user_login_log ALTER COLUMN login_type TYPE SMALLINT USING CASE login_type WHEN 'password' THEN 1 WHEN 'email' THEN 2 WHEN 'phone' THEN 3 END;
  ELSIF login_data_type = 'smallint' THEN
    IF EXISTS (SELECT 1 FROM user_login_log WHERE login_type IS NOT NULL AND login_type NOT IN (1, 2, 3)) THEN
      RAISE EXCEPTION 'user_login_log contains an unknown numeric login_type';
    END IF;
  ELSE
    RAISE EXCEPTION 'user_login_log.login_type must be varchar/text or smallint, got %', login_data_type;
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='login_account')
     AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='account') THEN
    ALTER TABLE user_login_log RENAME COLUMN login_account TO account;
  END IF;
END $$;

ALTER TABLE user_login_log DROP CONSTRAINT IF EXISTS fk_user_login_log_session;
ALTER TABLE user_login_log DROP COLUMN IF EXISTS session_id;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_login_log'::regclass AND conname='ck_user_login_log_event_type') THEN
    ALTER TABLE user_login_log ADD CONSTRAINT ck_user_login_log_event_type CHECK (event_type IN (1, 2, 3));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_login_log'::regclass AND conname='ck_user_login_log_login_type') THEN
    ALTER TABLE user_login_log ADD CONSTRAINT ck_user_login_log_login_type CHECK (
      (event_type = 1 AND login_type IN (2, 3))
      OR (event_type = 2 AND login_type IN (1, 2, 3))
      OR (event_type = 3 AND login_type IS NULL)
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_login_log'::regclass AND conname='ck_user_login_log_is_success') THEN
    ALTER TABLE user_login_log ADD CONSTRAINT ck_user_login_log_is_success CHECK (is_success IN (0, 1));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_login_log'::regclass AND conname='ck_user_login_log_account') THEN
    ALTER TABLE user_login_log ADD CONSTRAINT ck_user_login_log_account CHECK (btrim(account) <> '');
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_user_login_log_account_created_at ON user_login_log (account, created_at DESC);

COMMIT;
