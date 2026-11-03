package provider

import (
	configcache "admin/server/internal/shared/configCache"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func testCipher(t *testing.T) *Cipher {
	t.Helper()
	c, err := NewCipher(strings.Repeat("k", 64), "key-1")
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestUpdateRequestCannotCarryStatusOrPrice(t *testing.T) {
	typ := reflect.TypeOf(UpdateRequest{})
	for _, field := range []string{"Status", "Code", "Rates", "Price", "BillingUnitPriceUnits", "SourceUnitPrice"} {
		if _, ok := typ.FieldByName(field); ok {
			t.Fatalf("ordinary update exposes %s", field)
		}
	}
}
func TestProviderSecretNeverAppearsInDTOOrStoredPlaintext(t *testing.T) {
	repo := newMemoryRepository()
	c := testCipher(t)
	s := NewService(repo, c, fixtureRuntime{})
	p, err := s.Create(context.Background(), fixtureCreate(), 1)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	if !p.APIKeyConfigured || strings.Contains(string(raw), "SECRET") {
		t.Fatal("secret escaped DTO")
	}
	stored := repo.versions[0]
	if stored.APIKeyCiphertext == "SECRET" || strings.Contains(stored.APIKeyCiphertext, "SECRET") {
		t.Fatal("plaintext stored")
	}
	plain, err := c.Open(stored.APIKeyCiphertext, stored.EncryptionKeyID)
	if err != nil || plain != "SECRET" {
		t.Fatal("credential not recoverable")
	}
}
func TestProviderUpdateCreatesVersionAndEmptySecretRetainsCiphertext(t *testing.T) {
	repo := newMemoryRepository()
	s := NewService(repo, testCipher(t), fixtureRuntime{})
	p, err := s.Create(context.Background(), fixtureCreate(), 1)
	if err != nil {
		t.Fatal(err)
	}
	original := repo.versions[0].APIKeyCiphertext
	_, err = s.Update(context.Background(), p.ID, UpdateRequest{Name: "Updated", BaseURL: "https://api.example.test/v1", MediaHostAllowlist: []byte(`[]`), RequestTimeoutMS: 30000}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.versions) != 2 || repo.versions[1].Version != 2 || repo.versions[1].APIKeyCiphertext != original {
		t.Fatal("revision/empty secret contract violated")
	}
}
func TestCipherUsesRandomAuthenticatedDomainSeparatedEnvelope(t *testing.T) {
	c := testCipher(t)
	a, id, err := c.Seal("SECRET")
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := c.Seal("SECRET")
	if err != nil || a == b {
		t.Fatal("nonce reused")
	}
	if _, err := c.Open(a, "wrong"); err == nil {
		t.Fatal("wrong key ID accepted")
	}
	if _, err := c.Open(a[:len(a)-1]+"?", id); err == nil {
		t.Fatal("tampering accepted")
	}
}
func TestProviderValidationRejectsURLAndForbiddenHeaders(t *testing.T) {
	for _, url := range []string{"http://host.test", "https://user:pass@host.test", "https://host.test#fragment", "https://127.0.0.1"} {
		req := fixtureCreate()
		req.BaseURL = url
		if _, err := NewService(newMemoryRepository(), testCipher(t)).Create(context.Background(), req, 1); err == nil {
			t.Fatalf("unsafe URL accepted %s", url)
		}
	}
	req := fixtureCreate()
	req.Headers = []byte(`{"Host":"evil","X-Test":"x"}`)
	if _, err := NewService(newMemoryRepository(), testCipher(t)).Create(context.Background(), req, 1); err == nil {
		t.Fatal("Host override accepted")
	}
}
func fixtureCreate() CreateRequest {
	return CreateRequest{Code: "fixture", Name: "Fixture", BaseURL: "https://api.example.test/v1", APIKey: "SECRET", MediaHostAllowlist: []byte(`[]`), RequestTimeoutMS: 30000}
}

type memoryRepository struct {
	providers map[int64]*Provider
	versions  []*ProviderVersion
	models    []ProviderModel
	next      int64
}
type fixtureRuntime struct{}

func (fixtureRuntime) Mutate(ctx context.Context, fn func(context.Context, int64) (configcache.MutationResult, error)) error {
	_, err := fn(ctx, 1)
	return err
}
func (m *memoryRepository) Mutate(ctx context.Context, base int64, fn func(Repository) error) (configcache.MutationResult, error) {
	err := fn(m)
	return configcache.MutationResult{Changed: err == nil, Generation: base + 1, OutboxID: 1}, err
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{providers: map[int64]*Provider{}, next: 1}
}
func (m *memoryRepository) Create(_ context.Context, p *Provider, v *ProviderVersion) error {
	p.ID = m.next
	m.next++
	v.ProviderID = p.ID
	v.ID = m.next
	m.next++
	m.providers[p.ID] = p
	m.versions = append(m.versions, v)
	return nil
}
func (m *memoryRepository) Find(_ context.Context, id int64) (*Provider, error) {
	p, ok := m.providers[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}
func (m *memoryRepository) CurrentVersion(_ context.Context, id int64) (*ProviderVersion, error) {
	for i := len(m.versions) - 1; i >= 0; i-- {
		if m.versions[i].ProviderID == id {
			return m.versions[i], nil
		}
	}
	return nil, ErrNotFound
}
func (m *memoryRepository) SaveRevision(_ context.Context, p *Provider, v *ProviderVersion) error {
	m.providers[p.ID] = p
	m.versions = append(m.versions, v)
	return nil
}
func (m *memoryRepository) WithProvider(ctx context.Context, id int64, fn func(Repository) error) error {
	return fn(m)
}
func (m *memoryRepository) SetStatus(_ context.Context, id int64, status int16) error {
	p, e := m.Find(context.TODO(), id)
	if e != nil {
		return e
	}
	p.Status = status
	return nil
}
func (m *memoryRepository) Models(context.Context, int64) ([]ProviderModel, error) {
	return m.models, nil
}
func (m *memoryRepository) SaveCatalog(_ context.Context, _ int64, models []ProviderModel) error {
	m.models = append(m.models, models...)
	return nil
}
