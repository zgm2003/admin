package config

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type fakeRepository struct {
	row       Model
	found     bool
	created   Model
	updated   Model
	deletedID int64
	testAt    time.Time
	testError string
	findErr   error
	writeErr  error
}

func (f *fakeRepository) FindActive(context.Context) (Model, error) {
	if f.findErr != nil {
		return Model{}, f.findErr
	}
	if !f.found {
		return Model{}, gorm.ErrRecordNotFound
	}
	return f.row, nil
}

func (f *fakeRepository) Create(_ context.Context, value *Model) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.created = *value
	f.row = *value
	f.row.ID = 7
	f.found = true
	return nil
}

func (f *fakeRepository) Update(_ context.Context, value *Model, now time.Time) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.updated = *value
	f.row = *value
	f.row.UpdatedAt = now
	f.found = true
	return nil
}

func (f *fakeRepository) UpdateTestResult(_ context.Context, id int64, at time.Time, message string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.testAt = at
	f.testError = message
	return nil
}

func (f *fakeRepository) Delete(_ context.Context, id int64) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.deletedID = id
	f.found = false
	return nil
}

type fakeRuntime struct {
	calls int
	err   error
}

func (f *fakeRuntime) Mutate(ctx context.Context, change func(context.Context) error) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return change(ctx)
}

func newTestService(t *testing.T, repository *fakeRepository, runtime RuntimeCoordinator) *Service {
	t.Helper()
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	return NewService(repository, keys, runtime)
}

func validInput() Input {
	return Input{
		SecretID:   "AKIDexampleSecretId",
		SecretKey:  "exampleSecretKeyValue",
		SDKAppID:   "1400006666",
		SignName:   "示例签名",
		Region:     "ap-guangzhou",
		Endpoint:   "",
		TTLMinutes: 5,
		IsEnabled:  yesno.Yes,
	}
}

func appErrorCode(err error) int {
	var appError *apperror.Error
	if errors.As(err, &appError) {
		return appError.Code
	}
	return 0
}

func TestLoadReturnsUnconfiguredWhenNoActiveRow(t *testing.T) {
	safe, err := newTestService(t, &fakeRepository{}, &fakeRuntime{}).Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if safe.Configured || safe.Region != "" || safe.TTLMinutes != 0 {
		t.Fatalf("safe = %+v", safe)
	}
}

func TestUpdateRequiresBothSecretsOnCreate(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Input)
	}{
		{name: "both missing", mutate: func(input *Input) { input.SecretID, input.SecretKey = "", "" }},
		{name: "only secret id", mutate: func(input *Input) { input.SecretKey = "" }},
		{name: "only secret key", mutate: func(input *Input) { input.SecretID = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := validInput()
			test.mutate(&input)
			repository, runtime := &fakeRepository{}, &fakeRuntime{}
			_, err := newTestService(t, repository, runtime).Update(context.Background(), input)
			if appErrorCode(err) != apperror.CodeInvalidRequest {
				t.Fatalf("Update() error = %v", err)
			}
			if len(repository.created.SecretIDCiphertext) != 0 || runtime.calls != 0 {
				t.Fatal("invalid credentials reached the repository")
			}
		})
	}
}

func TestUpdateEncryptsCredentialsAndInvalidatesRuntime(t *testing.T) {
	repository, runtime := &fakeRepository{}, &fakeRuntime{}
	safe, err := newTestService(t, repository, runtime).Update(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !safe.Configured || safe.TTLMinutes != 5 || safe.IsEnabled != yesno.Yes {
		t.Fatalf("safe = %+v", safe)
	}
	if !strings.HasPrefix(repository.created.SecretIDCiphertext, "sms:v1:") ||
		!strings.HasPrefix(repository.created.SecretKeyCiphertext, "sms:v1:") {
		t.Fatalf("credentials were not stored as sms ciphertext: %+v", repository.created)
	}
	if strings.Contains(repository.created.SecretIDCiphertext, validInput().SecretID) {
		t.Fatal("stored ciphertext contains the plaintext secret id")
	}
	if repository.created.SecretIDHint != "AK***Id" || repository.created.SecretKeyHint != "ex***ue" {
		t.Fatalf("hints = %q / %q", repository.created.SecretIDHint, repository.created.SecretKeyHint)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime mutations = %d, want 1", runtime.calls)
	}
}

func TestUpdateKeepsExistingSecretsWhenBothAreEmpty(t *testing.T) {
	repository := &fakeRepository{
		found: true,
		row: Model{
			ID: 3, SecretIDCiphertext: "sms:v1:stored-id", SecretKeyCiphertext: "sms:v1:stored-key",
			SecretIDHint: "AK***Id", SecretKeyHint: "ex***ue", SDKAppID: "1400000000",
			SignName: "旧签名", Region: "ap-guangzhou", TTLMinutes: 10, IsEnabled: yesno.No,
		},
	}
	runtime := &fakeRuntime{}
	input := validInput()
	input.SecretID, input.SecretKey = "", ""
	input.SignName = "新签名"
	input.TTLMinutes = 30

	safe, err := newTestService(t, repository, runtime).Update(context.Background(), input)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repository.updated.SecretIDCiphertext != "sms:v1:stored-id" || repository.updated.SecretKeyCiphertext != "sms:v1:stored-key" {
		t.Fatalf("stored credentials were replaced: %+v", repository.updated)
	}
	if repository.updated.SignName != "新签名" || repository.updated.TTLMinutes != 30 {
		t.Fatalf("editable fields were not saved: %+v", repository.updated)
	}
	if safe.SignName != "新签名" || safe.TTLMinutes != 30 {
		t.Fatalf("safe = %+v", safe)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime mutations = %d, want 1", runtime.calls)
	}
}

func TestUpdateRejectsSingleSecretReplacementAndInvalidFields(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Input)
	}{
		{name: "single secret replacement", mutate: func(input *Input) { input.SecretKey = "" }},
		{name: "ttl below one", mutate: func(input *Input) { input.TTLMinutes = 0 }},
		{name: "ttl above sixty", mutate: func(input *Input) { input.TTLMinutes = 61 }},
		{name: "blank app id", mutate: func(input *Input) { input.SDKAppID = "  " }},
		{name: "overlong app id", mutate: func(input *Input) { input.SDKAppID = strings.Repeat("a", maxSDKAppIDLength+1) }},
		{name: "overlong signature", mutate: func(input *Input) { input.SignName = strings.Repeat("签", maxSignNameLength+1) }},
		{name: "invalid region", mutate: func(input *Input) { input.Region = "ap guangzhou" }},
		{name: "invalid endpoint", mutate: func(input *Input) { input.Endpoint = "https://sms.tencentcloudapi.com" }},
		{name: "invalid enabled", mutate: func(input *Input) { input.IsEnabled = yesno.Value(2) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := validInput()
			test.mutate(&input)
			repository := &fakeRepository{found: true}
			repository.row = Model{ID: 3, Region: "ap-guangzhou", TTLMinutes: 5, IsEnabled: yesno.Yes}
			runtime := &fakeRuntime{}
			_, err := newTestService(t, repository, runtime).Update(context.Background(), input)
			if appErrorCode(err) != apperror.CodeInvalidRequest {
				t.Fatalf("Update() error = %v, want invalid request", err)
			}
			if runtime.calls != 0 {
				t.Fatal("invalid input reached the runtime coordinator")
			}
		})
	}
}

func TestUpdateFailsClosedWithoutRuntimeCoordinator(t *testing.T) {
	_, err := NewService(&fakeRepository{}, mustKeys(t), nil).Update(context.Background(), validInput())
	if appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Update() error = %v, want dependency unavailable", err)
	}
}

func TestUpdateMapsRepositoryFailureToDependencyUnavailable(t *testing.T) {
	repository := &fakeRepository{writeErr: errors.New("database unavailable")}
	_, err := newTestService(t, repository, &fakeRuntime{}).Update(context.Background(), validInput())
	if appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Update() error = %v, want dependency unavailable", err)
	}
}

func TestDeleteRequiresAnActiveConfiguration(t *testing.T) {
	runtime := &fakeRuntime{}
	if err := newTestService(t, &fakeRepository{}, runtime).Delete(context.Background()); appErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("Delete() error = %v, want not found", err)
	}

	repository := &fakeRepository{found: true, row: Model{ID: 9}}
	if err := newTestService(t, repository, runtime).Delete(context.Background()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if repository.deletedID != 9 || runtime.calls != 1 {
		t.Fatalf("deleted id = %d, runtime calls = %d", repository.deletedID, runtime.calls)
	}
}

func TestCredentialsDecryptsStoredSecrets(t *testing.T) {
	keys := mustKeys(t)
	stored, _, err := secretkey.EncryptSMSValue(keys.SMSEncryptionKey(), "AKIDstored")
	if err != nil {
		t.Fatal(err)
	}
	storedKey, _, err := secretkey.EncryptSMSValue(keys.SMSEncryptionKey(), "storedSecretKey")
	if err != nil {
		t.Fatal(err)
	}
	repository := &fakeRepository{found: true, row: Model{
		ID: 1, SecretIDCiphertext: stored, SecretKeyCiphertext: storedKey,
		SDKAppID: "1400006666", SignName: "签名", Region: "ap-guangzhou", TTLMinutes: 5,
	}}
	credentials, err := NewService(repository, keys, &fakeRuntime{}).Credentials(context.Background())
	if err != nil {
		t.Fatalf("Credentials() error = %v", err)
	}
	if credentials.SecretID != "AKIDstored" || credentials.SecretKey != "storedSecretKey" ||
		credentials.SDKAppID != "1400006666" || credentials.SignName != "签名" ||
		credentials.Region != "ap-guangzhou" || credentials.TTLMinutes != 5 {
		t.Fatalf("credentials = %+v", credentials)
	}
}

func TestCredentialsRejectsForeignCiphertext(t *testing.T) {
	repository := &fakeRepository{found: true, row: Model{
		ID: 1, SecretIDCiphertext: "mail:v1:foreign", SecretKeyCiphertext: "mail:v1:foreign",
		SDKAppID: "1400006666", Region: "ap-guangzhou", TTLMinutes: 5,
	}}
	_, err := NewService(repository, mustKeys(t), &fakeRuntime{}).Credentials(context.Background())
	if appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Credentials() error = %v, want dependency unavailable", err)
	}
}

func TestMarkTestResultStoresBoundedMessage(t *testing.T) {
	repository := &fakeRepository{}
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	if err := newTestService(t, repository, &fakeRuntime{}).MarkTestResult(context.Background(), 4, at, strings.Repeat("x", 600)); err != nil {
		t.Fatalf("MarkTestResult() error = %v", err)
	}
	if !repository.testAt.Equal(at) || len(repository.testError) != 512 {
		t.Fatalf("test result = %v / %d", repository.testAt, len(repository.testError))
	}
}

func mustKeys(t *testing.T) *secretkey.KeyRing {
	t.Helper()
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	return keys
}
