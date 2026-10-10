package database_test

import (
	"context"
	"gorm.io/gorm"
	"testing"
)

// Fixture for historical SMS migration only; not a runtime dictionary contract.
func createDictionarySeedFixture(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE system_dictionary(
 id BIGSERIAL PRIMARY KEY,
 code VARCHAR(128) NOT NULL,
 name_zh VARCHAR(128) NOT NULL,
 name_en VARCHAR(128) NOT NULL,
 description VARCHAR(512) NOT NULL DEFAULT '',
 is_enabled SMALLINT NOT NULL DEFAULT 1 CHECK (is_enabled IN (0,1)),
 is_builtin SMALLINT NOT NULL DEFAULT 0 CHECK (is_builtin IN (0,1)),
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_system_dictionary_code ON system_dictionary(code) WHERE deleted_at IS NULL;
CREATE TABLE system_dictionary_item(
 id BIGSERIAL PRIMARY KEY,
 dictionary_id BIGINT NOT NULL REFERENCES system_dictionary(id) ON DELETE RESTRICT,
 value VARCHAR(128) NOT NULL,
 label_zh VARCHAR(256) NOT NULL,
 label_en VARCHAR(256) NOT NULL,
 sort INTEGER NOT NULL DEFAULT 0 CHECK (sort >= 0),
 is_enabled SMALLINT NOT NULL DEFAULT 1 CHECK (is_enabled IN (0,1)),
 is_builtin SMALLINT NOT NULL DEFAULT 0 CHECK (is_builtin IN (0,1)),
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_system_dictionary_item_value_active
ON system_dictionary_item(dictionary_id,value) WHERE deleted_at IS NULL;`).Error; err != nil {
		t.Fatal(err)
	}
}
