package run

import (
	pr "admin/server/internal/module/ai/providerRequest"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
	"time"
)

func mediaResultFixture(t *testing.T) (*Repository, context.Context, Model, string, pr.Fact) {
	t.Helper()
	repo, _, ctx := runDB(t)
	row, _, err := repo.Create(ctx, fixtureInput(), fixtureSnapshot(), completeClaimPolicy(t), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	token := uuid.NewString()
	if _, err := repo.Claim(ctx, row.ID, token, time.Now()); err != nil {
		t.Fatal(err)
	}
	request, err := repo.StartRequest(ctx, row.ID, token, 1, []byte(`{"model":"fixture"}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return repo, ctx, row, token, request
}
func TestFinishMediaResultCommitsUsageEncryptedSourceAndJobAtomically(t *testing.T) {
	r, ctx, row, token, request := mediaResultFixture(t)
	now := time.Now().UTC()
	cipher, key := "encrypted-source", "test-key"
	mime := "image/png"
	file := MediaResultFile{RunID: row.ID, Ordinal: 0, ImportRequestKey: uuid.NewString(), SourceKind: 1, SourceCiphertext: &cipher, EncryptionKeyID: &key, SourceRecoverable: 1, TransferDeadline: now.Add(time.Minute), MIMEType: &mime, Metadata: json.RawMessage(`{}`), TransferStatus: 0, NextTransferAt: &now}
	if err := r.FinishMediaResult(ctx, row.ID, token, request.ID, "provider-id", []MediaResultItem{{Ordinal: 0, RevisedPrompt: "draw"}}, pr.MediaUsage{Known: true, Basis: "image", ImageCount: 1}, nil, []MediaResultFile{file}, now); err != nil {
		t.Fatal(err)
	}
	var requestFact pr.Fact
	if err := r.db.WithContext(ctx).First(&requestFact, request.ID).Error; err != nil {
		t.Fatal(err)
	}
	if requestFact.Status != 2 || requestFact.UsageStatus != 1 || strings.Contains(string(requestFact.ResponseItems), "https://") || strings.Contains(string(requestFact.ResponseItems), "url") {
		t.Fatalf("request=%+v", requestFact)
	}
	var count int64
	if err := r.db.WithContext(ctx).Table("ai_media_file").Where("run_id=? AND source_ciphertext=?", row.ID, cipher).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("files=%d err=%v", count, err)
	}
	if err := r.db.WithContext(ctx).Table("system_scheduler_job").Where("task_type='ai.media.import'").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("jobs=%d err=%v", count, err)
	}
}
func TestFinishMediaResultRollsBackUsageWhenMediaInsertFails(t *testing.T) {
	r, ctx, row, token, request := mediaResultFixture(t)
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).Exec(`CREATE FUNCTION reject_result_file() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected media insert failure'; END $$;CREATE TRIGGER reject_result_file BEFORE INSERT ON ai_media_file FOR EACH ROW EXECUTE FUNCTION reject_result_file()`).Error; err != nil {
		t.Fatal(err)
	}
	file := MediaResultFile{RunID: row.ID, Ordinal: 0, ImportRequestKey: uuid.NewString(), SourceKind: 2, TransferDeadline: now.Add(time.Minute), Metadata: json.RawMessage(`{}`), TransferStatus: 4}
	if err := r.FinishMediaResult(ctx, row.ID, token, request.ID, "", []MediaResultItem{{Ordinal: 0}}, pr.MediaUsage{Known: true, Basis: "image", ImageCount: 1}, nil, []MediaResultFile{file}, now); err == nil {
		t.Fatal("accepted failed media insert")
	}
	var requestFact pr.Fact
	if err := r.db.WithContext(ctx).First(&requestFact, request.ID).Error; err != nil {
		t.Fatal(err)
	}
	if requestFact.Status != 1 || requestFact.UsageStatus != 0 {
		t.Fatalf("partial result committed %+v", requestFact)
	}
	var count int64
	if err := r.db.WithContext(ctx).Table("ai_media_file").Where("run_id=?", row.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("files=%d err=%v", count, err)
	}
}

func TestFinishMediaResultRollsBackResponseAndFileWhenJobInsertFails(t *testing.T) {
	r, ctx, row, token, request := mediaResultFixture(t)
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).Exec(`ALTER TABLE system_scheduler_job ADD CONSTRAINT reject_media_job CHECK(task_type<>'ai.media.import')`).Error; err != nil {
		t.Fatal(err)
	}
	cipher, key := "encrypted", "key"
	file := MediaResultFile{RunID: row.ID, Ordinal: 0, ImportRequestKey: uuid.NewString(), SourceKind: 1, SourceCiphertext: &cipher, EncryptionKeyID: &key, SourceRecoverable: 1, TransferDeadline: now.Add(time.Minute), Metadata: json.RawMessage(`{}`), TransferStatus: 0, NextTransferAt: &now}
	if err := r.FinishMediaResult(ctx, row.ID, token, request.ID, "", []MediaResultItem{{Ordinal: 0}}, pr.MediaUsage{Known: true, Basis: "image", ImageCount: 1}, nil, []MediaResultFile{file}, now); err == nil {
		t.Fatal("accepted failed durable job")
	}
	var saved pr.Fact
	if err := r.db.WithContext(ctx).First(&saved, request.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != 1 || saved.UsageStatus != 0 {
		t.Fatalf("partial request=%+v", saved)
	}
	var count int64
	if err := r.db.WithContext(ctx).Table("ai_media_file").Where("run_id=?", row.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("files=%d err=%v", count, err)
	}
}

func TestFinishMediaResultKeepsKnownUsageWhenSourceIsUnrecoverable(t *testing.T) {
	r, ctx, row, token, request := mediaResultFixture(t)
	now := time.Now().UTC()
	code, message := "SOURCE_UNRECOVERABLE", "媒体来源无法安全保存"
	file := MediaResultFile{RunID: row.ID, Ordinal: 0, ImportRequestKey: uuid.NewString(), SourceKind: 1, SourceRecoverable: 0, TransferDeadline: now.Add(time.Minute), Metadata: json.RawMessage(`{}`), TransferStatus: 4, ErrorCode: &code, ErrorMessage: &message}
	if err := r.FinishMediaResult(ctx, row.ID, token, request.ID, "provider-id", []MediaResultItem{{Ordinal: 0}}, pr.MediaUsage{Known: true, Basis: "image", ImageCount: 1}, nil, []MediaResultFile{file}, now); err != nil {
		t.Fatal(err)
	}
	var saved pr.Fact
	if err := r.db.WithContext(ctx).First(&saved, request.ID).Error; err != nil {
		t.Fatal(err)
	}
	var usage pr.MediaUsage
	if err := json.Unmarshal(saved.Usage, &usage); err != nil {
		t.Fatal(err)
	}
	if saved.Status != 2 || saved.UsageStatus != 1 || !usage.Known || usage.ImageCount != 1 {
		t.Fatalf("request=%+v usage=%+v", saved, usage)
	}
	var output MediaResultFile
	if err := r.db.WithContext(ctx).Where("run_id=? AND role=2", row.ID).First(&output).Error; err != nil {
		t.Fatal(err)
	}
	if output.TransferStatus != 4 || output.SourceRecoverable != 0 || output.SourceCiphertext != nil {
		t.Fatalf("output=%+v", output)
	}
	var count int64
	if err := r.db.WithContext(ctx).Table("system_scheduler_job").Where("task_type='ai.media.import'").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("jobs=%d err=%v", count, err)
	}
}
