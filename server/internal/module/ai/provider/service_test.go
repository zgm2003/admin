package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/yesno"
)

type fakeRepository struct {
	created  Model
	updated  UpdateValues
	status   yesno.Value
	deleted  int64
	row      Model
	createID int64
}

func (f *fakeRepository) Create(_ context.Context, row Model) (int64, error) {
	f.created = row
	return f.createID, nil
}
func (f *fakeRepository) FindByID(context.Context, int64) (Model, error) { return f.row, nil }
func (f *fakeRepository) List(context.Context, ListQuery) ([]Model, int64, error) {
	return []Model{f.row}, 1, nil
}
func (f *fakeRepository) Update(_ context.Context, _ int64, values UpdateValues) error {
	f.updated = values
	return nil
}
func (f *fakeRepository) UpdateStatus(_ context.Context, _ int64, value yesno.Value, _ time.Time) error {
	f.status = value
	return nil
}
func (f *fakeRepository) Delete(_ context.Context, id int64) error {
	f.deleted = id
	return nil
}

func testKeyRing(t *testing.T) *secretkey.KeyRing {
	t.Helper()
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func TestCreateEncryptsAPIKeyAndReturnsConfiguredValue(t *testing.T) {
	repo := &fakeRepository{createID: 9}
	service := NewService(repo, testKeyRing(t))
	id, err := service.Create(context.Background(), CreateInput{
		Name: "OpenAI official", Vendor: VendorOpenAI, Protocol: ProtocolOpenAI,
		BaseURL: "https://api.openai.com/v1", APIKey: "sk-plain-secret",
		TimeoutMS: 60000, SortOrder: 0, Status: yesno.Yes,
	})
	if err != nil || id != 9 {
		t.Fatalf("Create() = %d, %v", id, err)
	}
	if repo.created.APIKeyCiphertext == "sk-plain-secret" || !strings.HasPrefix(repo.created.APIKeyCiphertext, "ai-provider:v1:") {
		t.Fatalf("ciphertext = %q", repo.created.APIKeyCiphertext)
	}
	plain, err := secretkey.DecryptProviderValue(testKeyRing(t).ProviderEncryptionKey(), repo.created.APIKeyCiphertext)
	if err != nil || plain != "sk-plain-secret" {
		t.Fatalf("decrypt = %q, %v", plain, err)
	}
	repo.row = repo.created
	repo.row.ID = 9
	safe, err := service.Get(context.Background(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if !safe.APIKeyConfigured || safe.ID != 9 || safe.Name != "OpenAI official" {
		t.Fatalf("SafeValue = %+v", safe)
	}
}

func TestUpdateWithoutKeyKeepsCiphertext(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, testKeyRing(t))
	name := "proxy"
	err := service.Update(context.Background(), 3, UpdateInput{
		Name: name, Vendor: VendorOpenAI, Protocol: ProtocolOpenAI,
		BaseURL: "https://proxy.example.com/v1", TimeoutMS: 60000, SortOrder: 0,
		Status: yesno.Yes,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.updated.APIKeyCiphertext != nil || repo.updated.EncryptionKeyID != nil {
		t.Fatal("blank update rotated the credential")
	}
}

func TestUpdateWithNewKeyRotatesCiphertext(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, testKeyRing(t))
	key := "new-secret"
	if err := service.Update(context.Background(), 3, UpdateInput{
		Name: "proxy", Vendor: VendorOpenAI, Protocol: ProtocolOpenAI,
		BaseURL: "https://proxy.example.com/v1", APIKey: &key,
		TimeoutMS: 60000, SortOrder: 0, Status: yesno.Yes,
	}); err != nil {
		t.Fatal(err)
	}
	if repo.updated.APIKeyCiphertext == nil || *repo.updated.APIKeyCiphertext == key {
		t.Fatal("new API key was not encrypted")
	}
}
