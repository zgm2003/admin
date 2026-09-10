BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS system_dictionary (
  id BIGSERIAL PRIMARY KEY,
  code VARCHAR(128) NOT NULL,
  name_zh VARCHAR(128) NOT NULL,
  name_en VARCHAR(128) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  is_enabled SMALLINT NOT NULL DEFAULT 1,
  is_builtin SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ,
  CONSTRAINT ck_system_dictionary_code CHECK (code ~ '^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$'),
  CONSTRAINT ck_system_dictionary_is_enabled CHECK (is_enabled IN (0, 1)),
  CONSTRAINT ck_system_dictionary_is_builtin CHECK (is_builtin IN (0, 1))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_system_dictionary_code ON system_dictionary (code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_system_dictionary_enabled_code ON system_dictionary (is_enabled, code) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS system_dictionary_item (
  id BIGSERIAL PRIMARY KEY,
  dictionary_id BIGINT NOT NULL REFERENCES system_dictionary(id) ON DELETE RESTRICT,
  value VARCHAR(128) NOT NULL,
  label_zh VARCHAR(256) NOT NULL DEFAULT '',
  label_en VARCHAR(256) NOT NULL DEFAULT '',
  sort INTEGER NOT NULL DEFAULT 0,
  is_enabled SMALLINT NOT NULL DEFAULT 1,
  is_builtin SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ,
  CONSTRAINT ck_system_dictionary_item_sort CHECK (sort >= 0),
  CONSTRAINT ck_system_dictionary_item_is_enabled CHECK (is_enabled IN (0, 1)),
  CONSTRAINT ck_system_dictionary_item_is_builtin CHECK (is_builtin IN (0, 1))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_system_dictionary_item_value_active
  ON system_dictionary_item (dictionary_id, value) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_system_dictionary_item_enabled_sort
  ON system_dictionary_item (dictionary_id, is_enabled, sort, id);

COMMIT;
