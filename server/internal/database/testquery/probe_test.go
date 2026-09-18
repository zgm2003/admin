package testquery

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"

	"gorm.io/gorm/logger"
)

func TestCounterCountsOnlySuccessfulSelectsAndCanReset(t *testing.T) {
	base := logger.New(log.New(io.Discard, "", 0), logger.Config{LogLevel: logger.Silent})
	counter := New(base, "storage_cos_config", "storage_cos_config_version", "storage_upload_rule")
	trace := func(sql string, err error) {
		counter.Trace(context.Background(), time.Now(), func() (string, int64) { return sql, 1 }, err)
	}
	trace(" SELECT * FROM storage_cos_config WHERE id = 1", nil)
	trace("SELECT version FROM storage_cos_config_version WHERE cos_config_id = 1", nil)
	trace("SELECT id FROM storage_upload_rule WHERE id = 1", nil)
	trace("SELECT * FROM storage_cos_config", errors.New("query failed"))
	trace("UPDATE storage_cos_config SET name = 'changed'", nil)
	if got := counter.Count("storage_cos_config"); got != 1 {
		t.Fatalf("logical config SELECT count = %d, want 1", got)
	}
	if got := counter.Count("storage_cos_config_version"); got != 1 {
		t.Fatalf("physical version SELECT count = %d, want 1", got)
	}
	if got := counter.Count("storage_upload_rule"); got != 1 {
		t.Fatalf("rule SELECT count = %d, want 1", got)
	}
	if got := counter.Total(); got != 3 {
		t.Fatalf("total SELECT count = %d, want 3", got)
	}
	counter.Reset()
	if got := counter.Total(); got != 0 {
		t.Fatalf("SELECT count after reset = %d, want 0", got)
	}
}
