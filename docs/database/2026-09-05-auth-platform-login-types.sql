BEGIN;

-- 认证平台登录方式策略列：JSONB 数组，至少 1 个、至多 3 个，只允许 email/phone/password。
-- 既有行回填为 ["email","password"]，不改变既有邮箱密码登录行为。
ALTER TABLE auth_platform
  ADD COLUMN IF NOT EXISTS login_types JSONB NOT NULL DEFAULT '["email","password"]';

UPDATE auth_platform
SET login_types = '["email","password"]'::jsonb
WHERE CASE
  WHEN login_types IS NULL OR jsonb_typeof(login_types) <> 'array' THEN TRUE
  ELSE
    jsonb_array_length(login_types) NOT BETWEEN 1 AND 3
    OR NOT login_types <@ '["email","phone","password"]'::jsonb
    OR jsonb_array_length(login_types) <>
      (CASE WHEN login_types @> '["email"]'::jsonb THEN 1 ELSE 0 END
       + CASE WHEN login_types @> '["phone"]'::jsonb THEN 1 ELSE 0 END
       + CASE WHEN login_types @> '["password"]'::jsonb THEN 1 ELSE 0 END)
END;

ALTER TABLE auth_platform
  ALTER COLUMN login_types SET DEFAULT '["email","password"]'::jsonb,
  ALTER COLUMN login_types SET NOT NULL;

ALTER TABLE auth_platform DROP CONSTRAINT IF EXISTS ck_auth_platform_login_types;
ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_login_types CHECK (
  jsonb_typeof(login_types) = 'array'
  AND jsonb_array_length(login_types) BETWEEN 1 AND 3
  AND login_types <@ '["email","phone","password"]'::jsonb
  AND jsonb_array_length(login_types) =
    (CASE WHEN login_types @> '["email"]'::jsonb THEN 1 ELSE 0 END
     + CASE WHEN login_types @> '["phone"]'::jsonb THEN 1 ELSE 0 END
     + CASE WHEN login_types @> '["password"]'::jsonb THEN 1 ELSE 0 END)
);

-- 允许手机号用户使用 email=''：邮箱部分唯一索引只约束非空邮箱。
DROP INDEX IF EXISTS ux_user_account_email_active;
CREATE UNIQUE INDEX ux_user_account_email_active
  ON user_account (lower(email))
  WHERE email <> '' AND deleted_at IS NULL;

COMMIT;
