BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';
LOCK TABLE message_sms_template, message_mail_template IN ACCESS EXCLUSIVE MODE;

ALTER TABLE message_sms_template
  ADD COLUMN IF NOT EXISTS content text NOT NULL DEFAULT '';
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_sms_template' AND column_name='parameter_keys')
     AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_sms_template' AND column_name='variable_keys') THEN
    ALTER TABLE message_sms_template RENAME COLUMN parameter_keys TO variable_keys;
  END IF;
END $$;

ALTER TABLE message_mail_template
  ADD COLUMN IF NOT EXISTS content text NOT NULL DEFAULT '';
ALTER TABLE message_mail_template
  ADD COLUMN IF NOT EXISTS variable_keys jsonb;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_mail_template' AND column_name='variables') THEN
    UPDATE message_mail_template
    SET variable_keys = COALESCE(variable_keys,
      (SELECT jsonb_agg(key ORDER BY CASE key WHEN 'code' THEN 0 WHEN 'ttl_minutes' THEN 1 ELSE 2 END, key)
       FROM jsonb_object_keys(variables) AS keys(key)));
  END IF;
END $$;
ALTER TABLE message_mail_template ALTER COLUMN tencent_template_id DROP NOT NULL;
ALTER TABLE message_mail_template ALTER COLUMN variable_keys SET NOT NULL;

UPDATE message_sms_template
SET content = CASE scene
  WHEN 'login' THEN '您的登录验证码为 {1}，有效期 {2} 分钟。'
  WHEN 'forget' THEN '您的找回密码验证码为 {1}，有效期 {2} 分钟。'
  WHEN 'bind_phone' THEN '您的绑定手机验证码为 {1}，有效期 {2} 分钟。'
  WHEN 'change_password' THEN '您的修改密码验证码为 {1}，有效期 {2} 分钟。'
  ELSE content
END
WHERE content = '';
ALTER TABLE message_sms_template ALTER COLUMN content DROP DEFAULT;

UPDATE message_mail_template
SET content = '<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><title>' || name || '</title></head><body style="margin:0;padding:32px;background:#f5f7fb;font-family:Arial,sans-serif;color:#1f2937"><div style="max-width:640px;margin:0 auto;background:#fff;border:1px solid #e5e7eb;border-radius:16px;overflow:hidden"><div style="padding:28px 32px;background:linear-gradient(135deg,#2563eb,#14b8a6);color:#fff"><h1 style="margin:0;font-size:24px">' || name || '</h1></div><div style="padding:32px"><p style="font-size:16px;line-height:1.8">您好，请使用下面的信息完成操作：</p><div style="padding:22px;text-align:center;background:#f8fafc;border:1px dashed #93c5fd;border-radius:14px"><strong style="font-size:36px;letter-spacing:8px;color:#1d4ed8">{{code}}</strong></div><p style="color:#4b5563">验证码有效期为 {{ttl_minutes}} 分钟，请勿泄露给他人。</p></div><div style="padding:18px 32px;background:#f9fafb;border-top:1px solid #e5e7eb;color:#6b7280;font-size:13px">本邮件由系统自动发送，请勿直接回复。</div></div></body></html>'
WHERE content = '';
UPDATE message_mail_template
SET content = replace(
  replace(content,
    '<p style="font-size:16px;line-height:1.8">',
    '<div data-mail-editor="content" style="font-size:16px;line-height:1.8"><p style="margin:0">'),
  '</p><div style="padding:22px;text-align:center',
  '</p></div><div style="padding:22px;text-align:center')
WHERE content NOT LIKE '%data-mail-editor="content"%'
  AND content LIKE '%<p style="font-size:16px;line-height:1.8">%';
ALTER TABLE message_mail_template ALTER COLUMN content DROP DEFAULT;

ALTER TABLE message_sms_template DROP CONSTRAINT IF EXISTS ck_message_sms_template_parameter_keys;
ALTER TABLE message_sms_template DROP CONSTRAINT IF EXISTS ck_message_sms_template_variable_keys;
ALTER TABLE message_sms_template ADD CONSTRAINT ck_message_sms_template_variable_keys
  CHECK (jsonb_typeof(variable_keys) = 'array' AND jsonb_array_length(variable_keys) >= 2);
ALTER TABLE message_mail_template DROP CONSTRAINT IF EXISTS ck_message_mail_template_variable_keys;
ALTER TABLE message_mail_template ADD CONSTRAINT ck_message_mail_template_variable_keys
  CHECK (jsonb_typeof(variable_keys) = 'array' AND jsonb_array_length(variable_keys) >= 2);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM message_sms_template WHERE content = '' OR variable_keys IS NULL)
     OR EXISTS (SELECT 1 FROM message_mail_template WHERE content = '' OR variable_keys IS NULL) THEN
    RAISE EXCEPTION 'message template backfill left empty content or variables';
  END IF;
  IF EXISTS (
    SELECT 1 FROM (
      SELECT variable_keys, example_variables FROM message_sms_template
      UNION ALL SELECT variable_keys, example_variables FROM message_mail_template
    ) t
    WHERE NOT (variable_keys @> '["code","ttl_minutes"]'::jsonb)
       OR jsonb_typeof(example_variables) <> 'object'
       OR EXISTS (SELECT 1 FROM jsonb_array_elements(variable_keys) k WHERE jsonb_typeof(k) <> 'string')
       OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(variable_keys) k WHERE k !~ '^[a-z][a-z0-9_]{1,63}$' OR NOT example_variables ? k)
       OR jsonb_array_length(variable_keys) <> (SELECT count(DISTINCT k) FROM jsonb_array_elements_text(variable_keys) k)
       OR jsonb_array_length(variable_keys) <> (SELECT count(*) FROM jsonb_object_keys(example_variables))
  ) THEN
    RAISE EXCEPTION 'template keys and examples are inconsistent; migration aborted';
  END IF;
END $$;

ALTER TABLE message_mail_template DROP COLUMN IF EXISTS variables;

COMMIT;
