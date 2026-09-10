BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

CREATE TEMP TABLE display_option_dictionary_seed (
  code VARCHAR(128) NOT NULL,
  name_zh VARCHAR(128) NOT NULL,
  name_en VARCHAR(128) NOT NULL,
  description VARCHAR(512) NOT NULL,
  value VARCHAR(128) NOT NULL,
  label_zh VARCHAR(256) NOT NULL,
  label_en VARCHAR(256) NOT NULL,
  sort INTEGER NOT NULL,
  PRIMARY KEY (code, value)
) ON COMMIT DROP;

INSERT INTO display_option_dictionary_seed
  (code, name_zh, name_en, description, value, label_zh, label_en, sort)
VALUES
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'ap-guangzhou', '广州（ap-guangzhou）', 'Guangzhou (ap-guangzhou)', 10),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'ap-shanghai', '上海（ap-shanghai）', 'Shanghai (ap-shanghai)', 20),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'ap-nanjing', '南京（ap-nanjing）', 'Nanjing (ap-nanjing)', 30),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'ap-beijing', '北京（ap-beijing）', 'Beijing (ap-beijing)', 40),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'ap-chengdu', '成都（ap-chengdu）', 'Chengdu (ap-chengdu)', 50),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'ap-chongqing', '重庆（ap-chongqing）', 'Chongqing (ap-chongqing)', 60),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'ap-hongkong', '中国香港（ap-hongkong）', 'Hong Kong, China (ap-hongkong)', 70),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'ap-singapore', '新加坡（ap-singapore）', 'Singapore (ap-singapore)', 80),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'ap-tokyo', '东京（ap-tokyo）', 'Tokyo (ap-tokyo)', 90),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'ap-seoul', '首尔（ap-seoul）', 'Seoul (ap-seoul)', 100),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'eu-frankfurt', '法兰克福（eu-frankfurt）', 'Frankfurt (eu-frankfurt)', 110),
  ('storage.cos.region', 'COS 地域', 'COS Regions', 'Tencent Cloud COS region options', 'na-siliconvalley', '硅谷（na-siliconvalley）', 'Silicon Valley (na-siliconvalley)', 120),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'jpg', 'jpg', 'jpg', 10),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'jpeg', 'jpeg', 'jpeg', 20),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'png', 'png', 'png', 30),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'gif', 'gif', 'gif', 40),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'webp', 'webp', 'webp', 50),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'pdf', 'pdf', 'pdf', 60),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'doc', 'doc', 'doc', 70),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'docx', 'docx', 'docx', 80),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'xls', 'xls', 'xls', 90),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'xlsx', 'xlsx', 'xlsx', 100),
  ('storage.file.extension', '文件扩展名', 'File Extensions', 'Common upload file extension options', 'zip', 'zip', 'zip', 110),
  ('storage.mime.type', 'MIME 类型', 'MIME Types', 'Common upload MIME type options', 'image/jpeg', 'image/jpeg', 'image/jpeg', 10),
  ('storage.mime.type', 'MIME 类型', 'MIME Types', 'Common upload MIME type options', 'image/png', 'image/png', 'image/png', 20),
  ('storage.mime.type', 'MIME 类型', 'MIME Types', 'Common upload MIME type options', 'image/gif', 'image/gif', 'image/gif', 30),
  ('storage.mime.type', 'MIME 类型', 'MIME Types', 'Common upload MIME type options', 'image/webp', 'image/webp', 'image/webp', 40),
  ('storage.mime.type', 'MIME 类型', 'MIME Types', 'Common upload MIME type options', 'application/pdf', 'application/pdf', 'application/pdf', 50),
  ('storage.mime.type', 'MIME 类型', 'MIME Types', 'Common upload MIME type options', 'application/zip', 'application/zip', 'application/zip', 60),
  ('message.mail.region', '邮件地域', 'Mail Regions', 'Tencent Cloud SES region options', 'ap-guangzhou', '广州（ap-guangzhou）', 'Guangzhou (ap-guangzhou)', 10),
  ('message.mail.region', '邮件地域', 'Mail Regions', 'Tencent Cloud SES region options', 'ap-hongkong', '中国香港（ap-hongkong）', 'Hong Kong, China (ap-hongkong)', 20);

DO $$
DECLARE
  dictionary_seed RECORD;
  v_dictionary_id BIGINT;
  dictionary_builtin SMALLINT;
BEGIN
  FOR dictionary_seed IN
    SELECT DISTINCT code, name_zh, name_en, description
      FROM display_option_dictionary_seed
     ORDER BY code
  LOOP
    v_dictionary_id := NULL;
    dictionary_builtin := NULL;

    SELECT id, is_builtin
      INTO v_dictionary_id, dictionary_builtin
      FROM system_dictionary
     WHERE code = dictionary_seed.code
       AND deleted_at IS NULL
     FOR UPDATE;

    IF v_dictionary_id IS NULL THEN
      INSERT INTO system_dictionary
        (code, name_zh, name_en, description, is_enabled, is_builtin, created_at, updated_at)
      VALUES
        (dictionary_seed.code, dictionary_seed.name_zh, dictionary_seed.name_en,
         dictionary_seed.description, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
      RETURNING id INTO v_dictionary_id;
    ELSIF dictionary_builtin <> 1 THEN
      RAISE EXCEPTION 'reserved dictionary code % already belongs to a non-builtin dictionary', dictionary_seed.code;
    END IF;

    INSERT INTO system_dictionary_item
      (dictionary_id, value, label_zh, label_en, sort, is_enabled, is_builtin, created_at, updated_at)
    SELECT v_dictionary_id, value, label_zh, label_en, sort, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
      FROM display_option_dictionary_seed
     WHERE code = dictionary_seed.code
     ORDER BY sort
    ON CONFLICT (dictionary_id, value) WHERE deleted_at IS NULL DO NOTHING;
  END LOOP;
END $$;

COMMIT;
