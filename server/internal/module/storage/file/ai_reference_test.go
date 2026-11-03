package file

import (
	airun "admin/server/internal/module/ai/run"
	"errors"
	"testing"
	"time"
)

func TestFrozenAIFileRejectsChangedPhysicalIdentity(t *testing.T) {
	m := Model{ID: 1, UserID: 1, PlatformID: 1, Kind: KindOriginal, Status: StatusReady, ObjectKey: "input/a.png", MimeType: "image/png", SizeBytes: 10, Etag: "etag"}
	snapshot := airun.InputFileSnapshot{ID: "1", ObjectKey: m.ObjectKey, MIMEType: m.MimeType, SizeBytes: m.SizeBytes, ETag: m.Etag}
	if err := validateAIFrozen(m, snapshot, 100); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Model){func(m *Model) { m.ObjectKey = "other" }, func(m *Model) { m.Etag = "new" }, func(m *Model) { m.MimeType = "image/jpeg" }, func(m *Model) { m.SizeBytes++ }, func(m *Model) { m.Status = StatusCleanup }} {
		changed := m
		change(&changed)
		if err := validateAIFrozen(changed, snapshot, 100); err == nil {
			t.Fatalf("changed identity accepted %+v", changed)
		}
	}
}

func TestReferencedAIFileCannotBeDeletedOrCleaned(t *testing.T) {
	s, db, ctx, client, owner := fileFixture(t)
	fileID := upload(t, s, ctx, client, owner, "ai-input")
	if err := db.WithContext(ctx).Exec(`UPDATE storage_file SET status=2,etag='etag' WHERE id=?`, fileID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO ai_media_file(storage_file_id) VALUES(?)`, fileID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.repository.Delete(ctx, owner, fileID, time.Now().UTC()); !errors.Is(err, ErrConflict) {
		t.Fatalf("delete reference=%v", err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE storage_file SET status=3,credential_expires_at=now()-interval '1 hour' WHERE id=?`, fileID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.repository.ClaimCleanup(ctx, fileID, time.Now().UTC()); !errors.Is(err, ErrConflict) {
		t.Fatalf("cleanup reference=%v", err)
	}
}
