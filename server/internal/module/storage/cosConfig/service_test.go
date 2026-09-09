package cosconfig

import (
	"context"
	"errors"
	"strings"
	"testing"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"admin/server/internal/storage/cos"
)

type recordingConnectionTester struct {
	credentials cos.Credentials
	calls       int
	err         error
}

func (t *recordingConnectionTester) TestConnection(_ context.Context, credentials cos.Credentials) error {
	t.calls++
	t.credentials = credentials
	return t.err
}

func TestServiceEncryptsNormalizesAndPreservesCredentials(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	keys, err := secretkey.New(strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	tester := &recordingConnectionTester{}
	service := NewService(NewRepository(db), keys, tester)
	endpoint := " https://cos.example.com "
	domain := " https://cdn.example.com "
	id, err := service.Create(ctx, CreateInput{Name: " Main ", AppID: " 1250000000 ", SecretID: " secret-id ", SecretKey: " secret-key ", Bucket: " assets ", Region: " ap-guangzhou ", Endpoint: &endpoint, BucketDomain: &domain, IsEnabled: yesno.Yes, Remark: " primary "})
	if err != nil || id < 1 {
		t.Fatalf("create = %d,%v", id, err)
	}
	var stored Model
	if err := db.WithContext(ctx).Where("id = ?", id).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SecretIDCiphertext == "secret-id" || stored.SecretKeyCiphertext == "secret-key" || !strings.HasPrefix(stored.SecretIDCiphertext, "v1:") || !strings.HasPrefix(stored.SecretKeyCiphertext, "v1:") {
		t.Fatalf("credentials were not encrypted: %+v", stored)
	}
	if stored.Name != "Main" || stored.AppID != "1250000000" || stored.Bucket != "assets" || stored.Region != "ap-guangzhou" || stored.Endpoint == nil || *stored.Endpoint != "https://cos.example.com" || stored.BucketDomain == nil || *stored.BucketDomain != "https://cdn.example.com" || stored.Remark != "primary" {
		t.Fatalf("stored normalization = %+v", stored)
	}
	if _, err := service.Create(ctx, CreateInput{Name: "main", AppID: "1250000000", SecretID: "id", SecretKey: "key", Bucket: "assets", Region: "ap-guangzhou", IsEnabled: yesno.Yes}); appCode(err) != apperror.CodeConflict {
		t.Fatalf("duplicate create error = %v", err)
	}

	oldSecretID, oldSecretKey := stored.SecretIDCiphertext, stored.SecretKeyCiphertext
	if err := service.Update(ctx, id, UpdateInput{Name: "Main Updated", AppID: "1250000000", Bucket: "assets", Region: "ap-guangzhou", Endpoint: stringPointer("https://cos.example.com"), BucketDomain: stringPointer("https://cdn.example.com"), Remark: "updated"}); err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Where("id = ?", id).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SecretIDCiphertext != oldSecretID || stored.SecretKeyCiphertext != oldSecretKey {
		t.Fatal("omitted update secrets changed ciphertext")
	}
	if err := service.Update(ctx, id, UpdateInput{Name: "Main Updated", AppID: "1250000000", Bucket: "assets", Region: "ap-guangzhou", Endpoint: stringPointer("https://cos.example.com"), BucketDomain: stringPointer("https://cdn.example.com"), SecretKey: SecretInput{Present: true, Value: "new-key"}, Remark: "updated"}); err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Where("id = ?", id).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SecretIDCiphertext != oldSecretID || stored.SecretKeyCiphertext == oldSecretKey {
		t.Fatal("secret replacement did not update exactly one ciphertext")
	}
	if err := service.TestConnection(ctx, id); err != nil {
		t.Fatal(err)
	}
	if tester.calls != 1 || tester.credentials.SecretID != "secret-id" || tester.credentials.SecretKey != "new-key" || tester.credentials.Endpoint != "https://cos.example.com" {
		t.Fatalf("connection credentials = %+v calls=%d", tester.credentials, tester.calls)
	}
}

func TestServiceStatusAndDeleteRespectEnabledRules(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	keys, err := secretkey.New(strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(db), keys, &recordingConnectionTester{})
	id, err := service.Create(ctx, CreateInput{Name: "Main", AppID: "1250000000", SecretID: "id", SecretKey: "key", Bucket: "assets", Region: "ap-guangzhou", IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec("INSERT INTO storage_upload_rule (cos_config_id, is_enabled) VALUES (?, 1)", id).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateStatus(ctx, id, yesno.No); appCode(err) != apperror.CodeConflict {
		t.Fatalf("disable error = %v", err)
	}
	if err := service.Delete(ctx, id); appCode(err) != apperror.CodeConflict {
		t.Fatalf("delete error = %v", err)
	}
	var stored Model
	if err := db.WithContext(ctx).Where("id = ?", id).Take(&stored).Error; err != nil || stored.IsEnabled != yesno.Yes || stored.DeletedAt.Valid {
		t.Fatalf("conflict changed config: %+v,%v", stored, err)
	}
	if err := db.WithContext(ctx).Exec("UPDATE storage_upload_rule SET is_enabled = 0 WHERE cos_config_id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Unscoped().Where("id = ?", id).Take(&stored).Error; err != nil || stored.IsEnabled != yesno.No || !stored.DeletedAt.Valid {
		t.Fatalf("soft delete = %+v,%v", stored, err)
	}
	if _, err := service.Get(ctx, id); appCode(err) != apperror.CodeNotFound {
		t.Fatalf("get deleted error = %v", err)
	}
	if err := service.Update(ctx, id, UpdateInput{Name: "Deleted", AppID: "1250000000", Bucket: "assets", Region: "ap-guangzhou"}); appCode(err) != apperror.CodeNotFound {
		t.Fatalf("update deleted error = %v", err)
	}
}

func TestServiceRejectsInvalidInputsAndUnavailableTester(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	keys, err := secretkey.New(strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(db), keys, nil)
	for _, input := range []CreateInput{
		{Name: "", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", IsEnabled: yesno.Yes},
		{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", Endpoint: stringPointer("http://cos.example.com"), IsEnabled: yesno.Yes},
		{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", Endpoint: stringPointer("https://bad_host"), IsEnabled: yesno.Yes},
		{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", BucketDomain: stringPointer("not a url"), IsEnabled: yesno.Yes},
	} {
		if _, err := service.Create(ctx, input); appCode(err) != apperror.CodeInvalidRequest {
			t.Fatalf("invalid create error = %v input=%+v", err, input)
		}
	}
	id, err := service.Create(ctx, CreateInput{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.TestConnection(ctx, id); appCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("nil tester error = %v", err)
	}
	if err := service.UpdateStatus(ctx, 0, yesno.Yes); appCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("invalid id error = %v", err)
	}
}

func appCode(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

func stringPointer(value string) *string { return &value }
