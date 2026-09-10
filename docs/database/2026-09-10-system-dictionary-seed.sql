BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

DO $$
DECLARE
  v_dictionary_id BIGINT;
  dictionary_builtin SMALLINT;
BEGIN
  SELECT id, is_builtin
    INTO v_dictionary_id, dictionary_builtin
    FROM system_dictionary
   WHERE code = 'user.gender'
     AND deleted_at IS NULL
   FOR UPDATE;

  IF v_dictionary_id IS NULL THEN
    INSERT INTO system_dictionary
      (code, name_zh, name_en, description, is_enabled, is_builtin, created_at, updated_at)
    VALUES
      ('user.gender', '性别', 'Gender', 'Example bilingual dictionary for profile options', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    RETURNING id INTO v_dictionary_id;
  ELSIF dictionary_builtin <> 1 THEN
    RAISE EXCEPTION 'reserved dictionary code user.gender already belongs to a non-builtin dictionary';
  END IF;

  INSERT INTO system_dictionary_item
    (dictionary_id, value, label_zh, label_en, sort, is_enabled, is_builtin, created_at, updated_at)
  VALUES
    (v_dictionary_id, '0', '未知', 'Unknown', 0, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (v_dictionary_id, '1', '男', 'Male', 1, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (v_dictionary_id, '2', '女', 'Female', 2, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
  ON CONFLICT (dictionary_id, value) WHERE deleted_at IS NULL DO NOTHING;
END $$;

COMMIT;
