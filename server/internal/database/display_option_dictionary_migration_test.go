package database_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"gorm.io/gorm"
)

func TestDisplayOptionDictionaryMigrationIsIdempotentAndPreservesExistingItems(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_display_option_dictionary")
	createDictionarySeedFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`
INSERT INTO system_dictionary(code,name_zh,name_en,description,is_enabled,is_builtin)
VALUES ('storage.cos.region','既有 COS 地域','Existing COS Regions','existing',1,1);
INSERT INTO system_dictionary_item(dictionary_id,value,label_zh,label_en,sort,is_enabled,is_builtin)
SELECT id,'ap-guangzhou','自定义广州','Custom Guangzhou',1,1,0
FROM system_dictionary WHERE code='storage.cos.region';`).Error; err != nil {
		t.Fatal(err)
	}

	script := readDisplayOptionDictionaryMigration(t)
	for run := 0; run < 2; run++ {
		if err := db.WithContext(ctx).Exec(script).Error; err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}

	var result struct {
		Dictionaries int64
		Items        int64
		Builtin      int64
		BuiltinItems int64
	}
	if err := db.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM system_dictionary WHERE deleted_at IS NULL) dictionaries,
(SELECT count(*) FROM system_dictionary_item WHERE deleted_at IS NULL) items,
(SELECT count(*) FROM system_dictionary WHERE is_builtin=1 AND is_enabled=1 AND deleted_at IS NULL) builtin,
(SELECT count(*) FROM system_dictionary_item WHERE is_builtin=1 AND is_enabled=1 AND deleted_at IS NULL) builtin_items`).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.Dictionaries != 4 || result.Items != 31 || result.Builtin != 4 || result.BuiltinItems != 30 {
		t.Fatalf("seed result = %+v", result)
	}

	var existing struct {
		LabelZH   string
		LabelEN   string
		Sort      int
		IsBuiltin int16
	}
	if err := db.WithContext(ctx).Raw(`SELECT i.label_zh,i.label_en,i.sort,i.is_builtin
FROM system_dictionary_item i
JOIN system_dictionary d ON d.id=i.dictionary_id
WHERE d.code='storage.cos.region' AND i.value='ap-guangzhou' AND i.deleted_at IS NULL`).Scan(&existing).Error; err != nil {
		t.Fatal(err)
	}
	if existing.LabelZH != "自定义广州" || existing.LabelEN != "Custom Guangzhou" || existing.Sort != 1 || existing.IsBuiltin != 0 {
		t.Fatalf("existing item was overwritten: %+v", existing)
	}

	var mailRegions []string
	if err := db.WithContext(ctx).Raw(`SELECT i.value
FROM system_dictionary_item i
JOIN system_dictionary d ON d.id=i.dictionary_id
WHERE d.code='message.mail.region' AND i.deleted_at IS NULL
ORDER BY i.sort,i.id`).Scan(&mailRegions).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Join(mailRegions, ",") != "ap-guangzhou,ap-hongkong" {
		t.Fatalf("mail regions = %v", mailRegions)
	}
}

func TestDisplayOptionDictionaryMigrationRejectsReservedCodeCollisionAndRollsBack(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_display_option_dictionary_collision")
	createDictionarySeedFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`INSERT INTO system_dictionary
(code,name_zh,name_en,description,is_enabled,is_builtin)
VALUES ('storage.file.extension','自定义','Custom','custom',1,0)`).Error; err != nil {
		t.Fatal(err)
	}
	script := readDisplayOptionDictionaryMigration(t)
	if err := db.WithContext(ctx).Connection(func(connection *gorm.DB) error {
		_, migrationErr := connection.Statement.ConnPool.ExecContext(ctx, script)
		if _, rollbackErr := connection.Statement.ConnPool.ExecContext(ctx, "ROLLBACK"); rollbackErr != nil {
			return rollbackErr
		}
		if migrationErr == nil {
			t.Error("migration accepted a non-builtin reserved dictionary code")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var dictionaries int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_dictionary WHERE deleted_at IS NULL`).Scan(&dictionaries).Error; err != nil {
		t.Fatal(err)
	}
	if dictionaries != 1 {
		t.Fatalf("failed migration left partial dictionaries: %d", dictionaries)
	}
}

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

func readDisplayOptionDictionaryMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-10-display-option-dictionaries.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
