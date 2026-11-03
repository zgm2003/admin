package file

import (
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/storage/cosConfig"
	"admin/server/internal/module/storage/uploadRule"
	"admin/server/internal/shared/apperror"
	storagecos "admin/server/internal/storage/cos"
	"context"
	"errors"
	"gorm.io/gorm"
	"io"
	"strings"
	"testing"
	"time"
)

func TestGeneratedImportRejectsReplayWithDifferentFacts(t *testing.T) {
	key := "123e4567-e89b-12d3-a456-426614174000"
	owner := auth.Identity{UserID: 1, PlatformID: 1}
	model := Model{ID: 7, UserID: 1, PlatformID: 1, Kind: KindOriginal, Status: StatusReady, RequestKey: &key, MimeType: "image/png", SizeBytes: 8, Etag: "etag", ObjectKey: "frozen"}
	if err := generatedImportMatches(model, owner, key, "image/png", 8); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Model){func(m *Model) { m.MimeType = "image/jpeg" }, func(m *Model) { m.SizeBytes++ }, func(m *Model) { m.UserID++ }, func(m *Model) { m.Status = StatusCleanup }, func(m *Model) { m.Etag = "" }} {
		changed := model
		change(&changed)
		if !errors.Is(generatedImportMatches(changed, owner, key, "image/png", 8), ErrConflict) {
			t.Fatalf("accepted %+v", changed)
		}
	}
}

type generatedImportCOS struct {
	*memoryCOS
	beforePut    func(storagecos.PutStreamRequest)
	putCalls     int
	loseResponse bool
}

func (c *generatedImportCOS) PutStream(_ context.Context, _ storagecos.Credentials, in storagecos.PutStreamRequest) (storagecos.ObjectInfo, error) {
	c.putCalls++
	if c.beforePut != nil {
		c.beforePut(in)
	}
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return storagecos.ObjectInfo{}, err
	}
	if int64(len(body)) != in.ContentLength {
		return storagecos.ObjectInfo{}, errors.New("length mismatch")
	}
	remote := storagecos.ObjectInfo{Key: in.ObjectKey, MIME: in.ContentType, Size: in.ContentLength, ETag: "generated-etag"}
	c.mu.Lock()
	c.objects[in.ObjectKey] = remote
	c.mu.Unlock()
	if c.loseResponse {
		c.loseResponse = false
		return storagecos.ObjectInfo{}, errors.New("lost accepted PUT response")
	}
	return remote, nil
}
func TestGeneratedImportFreezesPendingKeyBeforePutAndRecoversAcrossMidnight(t *testing.T) {
	s, db, ctx, original, owner := fileFixture(t)
	client := &generatedImportCOS{memoryCOS: original, loseResponse: true}
	s.cos = client
	now := time.Date(2026, 10, 29, 23, 59, 59, 0, time.UTC)
	s.now = func() time.Time { return now }
	key := "123e4567-e89b-12d3-a456-426614174001"
	var frozen Model
	client.beforePut = func(in storagecos.PutStreamRequest) {
		var err error
		frozen, err = s.repository.ByRequest(ctx, owner, key)
		if err != nil || frozen.Status != StatusPending || frozen.ObjectKey != in.ObjectKey || frozen.RuleCode != "ai/image" {
			t.Fatalf("PUT before durable pending row: %+v err=%v", frozen, err)
		}
	}
	input := GeneratedMediaImport{ImportRequestKey: key, OriginalName: "image.png", MIME: "image/png", SizeBytes: 8, Body: strings.NewReader("PNGbytes")}
	if _, err := s.ImportGeneratedMedia(ctx, owner, input); err == nil {
		t.Fatal("accepted lost response")
	}
	var count int64
	if err := db.WithContext(ctx).Model(&Model{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("rows=%d err=%v", count, err)
	}
	runtime := s.runtime.(fixtureRuntime).value
	runtime.CurrentVersion = 2
	runtime.Versions = append(runtime.Versions, cosconfig.RuntimeVersion{Version: 2, Bucket: "new-bucket", Region: "ap-guangzhou"})
	s.runtime = fixtureRuntime{runtime}
	s.rules = generatedUploadRules{platformID: owner.PlatformID, targets: map[string]uploadrule.UploadTarget{}}
	now = now.Add(2 * time.Second)
	input.Body = strings.NewReader("PNGbytes")
	fileID, err := s.ImportGeneratedMedia(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.repository.ByRequest(ctx, owner, key)
	if err != nil || saved.ID != fileID || saved.Status != StatusReady || saved.ObjectKey != frozen.ObjectKey || saved.ConfigVersion != 1 || client.putCalls != 1 {
		t.Fatalf("saved=%+v putCalls=%d err=%v", saved, client.putCalls, err)
	}
	input.SizeBytes = 9
	input.Body = strings.NewReader("PNGbytesx")
	if _, err := s.ImportGeneratedMedia(ctx, owner, input); err == nil {
		t.Fatal("ready replay accepted different size")
	}
}

type generatedUploadRules struct {
	platformID int64
	targets    map[string]uploadrule.UploadTarget
}

func (r generatedUploadRules) FindFileUploadTarget(_ context.Context, platformID int64, code string) (uploadrule.UploadTarget, error) {
	target, found := r.targets[code]
	if platformID != r.platformID || !found {
		return uploadrule.UploadTarget{}, gorm.ErrRecordNotFound
	}
	return target, nil
}

func (generatedUploadRules) FileUploadOptions(context.Context, int64, string) ([]uploadrule.FileUploadOption, error) {
	return nil, errors.New("unexpected upload options query")
}

func TestGeneratedImportSelectsMediaPurposeCode(t *testing.T) {
	for _, test := range []struct{ name, mime, extension, code string }{
		{"image", "image/png", "png", "ai/image"},
		{"audio", "audio/mpeg", "mp3", "ai/audio"},
		{"video", "video/mp4", "mp4", "ai/video"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _, ctx, original, owner := fileFixture(t)
			client := &generatedImportCOS{memoryCOS: original}
			s.cos = client
			target := uploadrule.UploadTarget{RuleID: 1, PlatformID: owner.PlatformID, CosConfigID: 1, Code: test.code, MaxFileSizeBytes: 1024, AllowedExtensions: uploadrule.StringArray{test.extension}, AllowedMimeTypes: uploadrule.StringArray{test.mime}, AccessMode: "private"}
			s.rules = generatedUploadRules{platformID: owner.PlatformID, targets: map[string]uploadrule.UploadTarget{test.code: target}}
			key := "123e4567-e89b-12d3-a456-426614174002"
			fileID, err := s.ImportGeneratedMedia(ctx, owner, GeneratedMediaImport{ImportRequestKey: key, OriginalName: "result." + test.extension, MIME: test.mime, SizeBytes: 8, Body: strings.NewReader("testdata")})
			if err != nil {
				t.Fatal(err)
			}
			saved, err := s.repository.ByRequest(ctx, owner, key)
			if err != nil || saved.ID != fileID || saved.RuleCode != test.code || client.putCalls != 1 {
				t.Fatalf("saved=%+v putCalls=%d err=%v", saved, client.putCalls, err)
			}
		})
	}
}

func TestGeneratedImportRejectsMissingAICodeWithoutFileFallback(t *testing.T) {
	s, db, ctx, original, owner := fileFixture(t)
	client := &generatedImportCOS{memoryCOS: original}
	s.cos = client
	fileTarget, err := (fixtureRules{}).FindFileUploadTarget(ctx, owner.PlatformID, "file")
	if err != nil {
		t.Fatal(err)
	}
	s.rules = generatedUploadRules{platformID: owner.PlatformID, targets: map[string]uploadrule.UploadTarget{"file": fileTarget}}
	_, err = s.ImportGeneratedMedia(ctx, owner, GeneratedMediaImport{ImportRequestKey: "123e4567-e89b-12d3-a456-426614174003", OriginalName: "image.png", MIME: "image/png", SizeBytes: 8, Body: strings.NewReader("PNGbytes")})
	var app *apperror.Error
	if !errors.As(err, &app) || app.Code != apperror.CodeNotFound {
		t.Fatalf("missing AI code error=%v; must not use file", err)
	}
	var count int64
	if err := db.WithContext(ctx).Model(&Model{}).Count(&count).Error; err != nil || count != 0 || client.putCalls != 0 {
		t.Fatalf("rows=%d putCalls=%d err=%v", count, client.putCalls, err)
	}
}
