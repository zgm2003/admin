package event

import (
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"os"
	"testing"
	"time"
)

func TestCleanupPreservesActiveUnsettledAndUnpublishedEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	_ = godotenv.Load("../../../../../.env")
	db, ctx := testschema.Open(t, os.Getenv("POSTGRES_DSN"), "test_ai_event_cleanup")
	ddl := `CREATE TABLE ai_run(id bigint PRIMARY KEY,execution_status smallint NOT NULL,billing_status smallint NOT NULL);CREATE TABLE ai_run_event(id bigint PRIMARY KEY,run_id bigint NOT NULL REFERENCES ai_run(id),published_at timestamptz,created_at timestamptz NOT NULL);INSERT INTO ai_run VALUES(1,1,1),(2,3,3),(3,3,2),(4,3,2),(5,3,2);INSERT INTO ai_run_event VALUES(1,1,now(),now()-interval '10 days'),(2,2,now(),now()-interval '10 days'),(3,3,NULL,now()-interval '10 days'),(4,4,now(),now()-interval '10 days'),(5,5,now(),now());`
	if err := db.WithContext(ctx).Exec(ddl).Error; err != nil {
		t.Fatal(err)
	}
	r := NewRepository(db, nil)
	n, err := r.Cleanup(ctx, time.Now().Add(-7*24*time.Hour), 1)
	if err != nil || n != 1 {
		t.Fatalf("deleted=%d err=%v", n, err)
	}
	var ids []int64
	if err = db.WithContext(ctx).Table("ai_run_event").Select("id").Order("id").Scan(&ids).Error; err != nil {
		t.Fatal(err)
	}
	if len(ids) != 4 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 || ids[3] != 5 {
		t.Fatalf("protected events deleted: %v", ids)
	}
	if n, err = r.Cleanup(ctx, time.Now().Add(-7*24*time.Hour), 1000); err != nil || n != 0 {
		t.Fatalf("replay=%d err=%v", n, err)
	}
}
