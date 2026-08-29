package uploadrule

import (
	"context"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/storage/cosconfig"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	storagecos "admin/server/internal/storage/cos"
)

type recordingSigner struct {
	credentials storagecos.Credentials
	requests    []storagecos.PutRequest
	err         error
}

func (s *recordingSigner) PresignPut(_ context.Context, credentials storagecos.Credentials, request storagecos.PutRequest) (storagecos.PutResult, error) {
	s.credentials = credentials
	s.requests = append(s.requests, request)
	if s.err != nil {
		return storagecos.PutResult{}, s.err
	}
	return storagecos.PutResult{URL: "https://upload.example.com/signed", Headers: map[string]string{"Content-Type": request.ContentType}}, nil
}

func TestIssueCredentialsValidatesAndSignsPlatformRule(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	otherPlatformID := insertPlatform(t, db, ctx, "canvas", yesno.Yes)
	keys, err := secretkey.New(strings.Repeat("k", 64))
	if err != nil {
		t.Fatal(err)
	}
	configService := cosconfig.NewService(cosconfig.NewRepository(db), keys, nil)
	domain := "https://cdn.example.com"
	configID, err := configService.Create(ctx, cosconfig.CreateInput{Name: "Main", AppID: "1250000000", SecretID: "secret-id", SecretKey: "secret-key", Bucket: "assets", Region: "ap-guangzhou", BucketDomain: &domain, IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	signer := &recordingSigner{}
	service := NewService(NewRepository(db), keys, signer)
	rule := validCreate(platformID, configID, "avatar", yesno.Yes)
	rule.AccessMode = "public"
	rule.MaxFileCount = 2
	if _, err := service.Create(ctx, rule); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	result, err := service.IssueCredentials(ctx, auth.Identity{PlatformID: platformID}, CredentialInput{RuleCode: "avatar", Files: []FileInput{{FileName: "photo.PNG", ContentType: "image/png", FileSizeBytes: 100}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || len(signer.requests) != 1 {
		t.Fatalf("result=%+v requests=%+v", result, signer.requests)
	}
	item := result.Items[0]
	if item.Method != "PUT" || !strings.HasPrefix(item.ObjectKey, "uploads/") || strings.Contains(item.ObjectKey, "photo") || !strings.HasSuffix(item.ObjectKey, ".png") || item.PublicURL == nil || !strings.HasPrefix(*item.PublicURL, "https://cdn.example.com/uploads/") {
		t.Fatalf("item=%+v", item)
	}
	if item.ExpiresAt.Before(before.Add(9*time.Minute+50*time.Second)) || item.ExpiresAt.After(time.Now().UTC().Add(10*time.Minute+time.Second)) {
		t.Fatalf("expiresAt=%v", item.ExpiresAt)
	}
	if signer.credentials.SecretID != "secret-id" || signer.credentials.SecretKey != "secret-key" || signer.requests[0].ContentLength != 100 || !signer.requests[0].PublicRead {
		t.Fatalf("credentials=%+v request=%+v", signer.credentials, signer.requests[0])
	}
	if _, err := service.IssueCredentials(ctx, auth.Identity{PlatformID: otherPlatformID}, CredentialInput{RuleCode: "avatar", Files: []FileInput{{FileName: "photo.png", ContentType: "image/png", FileSizeBytes: 100}}}); appCode(err) != apperror.CodeNotFound {
		t.Fatalf("cross-platform error=%v", err)
	}
}

func TestIssueCredentialsRejectsRuleViolations(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	keys, _ := secretkey.New(strings.Repeat("z", 64))
	configService := cosconfig.NewService(cosconfig.NewRepository(db), keys, nil)
	configID, err := configService.Create(ctx, cosconfig.CreateInput{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "assets", Region: "ap-guangzhou", IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(db), keys, &recordingSigner{})
	rule := validCreate(platformID, configID, "avatar", yesno.Yes)
	rule.MaxFileSizeBytes = 100
	if _, err := service.Create(ctx, rule); err != nil {
		t.Fatal(err)
	}
	invalid := []CredentialInput{{RuleCode: "avatar", Files: nil}, {RuleCode: "avatar", Files: []FileInput{{FileName: "../photo.png", ContentType: "image/png", FileSizeBytes: 1}}}, {RuleCode: "avatar", Files: []FileInput{{FileName: "photo.jpg", ContentType: "image/png", FileSizeBytes: 1}}}, {RuleCode: "avatar", Files: []FileInput{{FileName: "photo.png", ContentType: "text/plain", FileSizeBytes: 1}}}, {RuleCode: "avatar", Files: []FileInput{{FileName: "photo.png", ContentType: "image/png", FileSizeBytes: 101}}}}
	for _, input := range invalid {
		if _, err := service.IssueCredentials(ctx, auth.Identity{PlatformID: platformID}, input); appCode(err) != apperror.CodeInvalidRequest {
			t.Fatalf("input=%+v error=%v", input, err)
		}
	}
}
