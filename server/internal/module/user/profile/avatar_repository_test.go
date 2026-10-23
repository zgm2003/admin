package profile

import (
	"admin/server/internal/database/testschema"
	"admin/server/internal/module/auth/login"
	storagefile "admin/server/internal/module/storage/file"
	"context"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"os"
	"strings"
	"testing"
	"time"
)

func avatarDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	values, err := godotenv.Read("../../../../.env")
	if err != nil {
		t.Fatal(err)
	}
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = values["POSTGRES_DSN"]
	}
	db, ctx := testschema.Open(t, dsn, "test_profile_avatar")
	schema := `CREATE TABLE user_account(id BIGINT PRIMARY KEY,username TEXT,email TEXT,phone TEXT,status SMALLINT,deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ);
 CREATE TABLE user_profile(user_id BIGINT PRIMARY KEY,avatar TEXT NOT NULL,birthday DATE,gender SMALLINT,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ);
 INSERT INTO user_account VALUES(7,'alice','alice@example.com',NULL,1,NULL,CURRENT_TIMESTAMP);
 INSERT INTO user_profile VALUES(7,'avatar/legacy.png',NULL,0,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);`
	if err := db.WithContext(ctx).Exec(schema).Error; err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../../../../docs/database/2026-10-12-storage-file.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(migration)
	start := strings.Index(text, "CREATE TABLE IF NOT EXISTS storage_file (")
	end := strings.Index(text[start:], "\n);") + start + 3
	if start < 0 || end < start {
		t.Fatal("file schema missing")
	}
	if err := db.WithContext(ctx).Exec(text[start:end]).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func TestAvatarAdoptionIsIndependentAndReplayCannotRestoreOldAvatar(t *testing.T) {
	db, ctx := avatarDatabase(t)
	repo := NewRepository(db)
	identity := auth.Identity{UserID: 7, PlatformID: 1}
	now := time.Now().UTC()
	makeCopy := func(key, request string) storagefile.AvatarCopy {
		var id int64
		err := db.WithContext(ctx).Raw(`INSERT INTO storage_file(user_id,platform_id,kind,cos_config_id,config_version,upload_rule_id,rule_code,object_key,original_name,mime_type,size_bytes,status,completed_at,request_key) VALUES(7,1,2,1,1,1,'avatar',?,'image.png','image/png',12,1,?,?) RETURNING id`, key, now, request).Scan(&id).Error
		if err != nil {
			t.Fatal(err)
		}
		return storagefile.AvatarCopy{ID: id, ObjectKey: key}
	}
	firstKey := "d750d24b-23f8-4c10-a240-cd9865706268"
	first := makeCopy("avatar/copy-one.png", firstKey)
	result, err := repo.UpdateWithAvatar(ctx, identity, "alice", nil, 1, first, firstKey, now)
	if err != nil || result.Avatar != first.ObjectKey {
		t.Fatalf("adopt=%+v err=%v", result, err)
	}
	result, err = repo.UpdateWithAvatar(ctx, identity, "ignored", nil, 0, first, firstKey, now)
	if err != nil || result.Username != "alice" {
		t.Fatalf("replay=%+v err=%v", result, err)
	}
	secondKey := "ca14a18b-c319-4fa3-b9a7-b55aa7bf0d7a"
	second := makeCopy("avatar/copy-two.png", secondKey)
	if _, err = repo.UpdateWithAvatar(ctx, identity, "alice", nil, 1, second, secondKey, now); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.UpdateWithAvatar(ctx, identity, "alice", nil, 1, first, firstKey, now); err == nil {
		t.Fatal("stale avatar replay accepted")
	}
	var status int16
	if err := db.WithContext(ctx).Table("storage_file").Select("status").Where("id=?", first.ID).Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status != storagefile.StatusCleanup {
		t.Fatalf("old avatar status=%d", status)
	}
	current, err := repo.Find(ctx, 7)
	if err != nil || current.Avatar != second.ObjectKey {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	if _, err = repo.UpdatePreservingAvatar(ctx, 7, "alice", nil, 1, "avatar/forged.png", now); err == nil {
		t.Fatal("arbitrary avatar accepted")
	}
	cleared, err := repo.UpdatePreservingAvatar(ctx, 7, "alice", nil, 1, "", now, identity)
	if err != nil || cleared.Avatar != "" {
		t.Fatalf("clear=%+v err=%v", cleared, err)
	}
	if err := db.WithContext(ctx).Table("storage_file").Select("status").Where("id=?", second.ID).Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status != storagefile.StatusCleanup {
		t.Fatalf("cleared avatar cleanup status=%d", status)
	}
}
