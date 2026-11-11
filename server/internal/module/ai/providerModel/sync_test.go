package providermodel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/ai/pricing"
	aiprovider "admin/server/internal/module/ai/provider"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type syncFakeRepository struct {
	officials []Official
	items     []SyncItem
	seenAt    time.Time
	chatModel *Model
}

func (f *syncFakeRepository) List(context.Context, int64, ListQuery) ([]Model, int64, error) {
	return nil, 0, nil
}
func (f *syncFakeRepository) FindByID(context.Context, int64, int64) (Model, error) {
	return Model{}, nil
}
func (f *syncFakeRepository) TestChatModel(context.Context, int64) (Model, error) {
	if f.chatModel == nil {
		return Model{}, gorm.ErrRecordNotFound
	}
	return *f.chatModel, nil
}
func (f *syncFakeRepository) PricesByModelIDs(context.Context, []int64) (map[int64][]Price, error) {
	return map[int64][]Price{}, nil
}
func (f *syncFakeRepository) OfficialByIDs(context.Context, []int64) (map[int64]Official, error) {
	return map[int64]Official{}, nil
}
func (f *syncFakeRepository) OfficialByVendor(context.Context, int16) ([]Official, error) {
	return f.officials, nil
}
func (f *syncFakeRepository) Sync(_ context.Context, _ int64, items []SyncItem, seenAt time.Time) (SyncCounts, error) {
	f.items = items
	f.seenAt = seenAt
	return SyncCounts{Inserted: int64(len(items))}, nil
}
func (f *syncFakeRepository) UpdateConfig(context.Context, int64, int64, UpdateValues) error {
	return nil
}

type syncFakeProvider struct{ model aiprovider.Model }

func (f syncFakeProvider) FindByID(context.Context, int64) (aiprovider.Model, error) {
	return f.model, nil
}

func TestSyncMatchesOfficialModelsAndDefaultsUnmatched(t *testing.T) {
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, _, err := secretkey.EncryptProviderValue(keys.ProviderEncryptionKey(), "sk-test")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"doubao-seedance-2-5-260628"},{"id":"unknown-model"}]}`))
	}))
	defer server.Close()

	repo := &syncFakeRepository{officials: []Official{
		{ID: 7, ModelKey: "gpt-4o", DisplayName: "GPT-4o", ModelKind: pricing.KindChat},
		{
			ID: 8, ModelKey: "doubao-seedance-2-5-260628",
			DisplayName: "Seedance 2.5", ModelKind: pricing.KindVideo,
		},
	}}
	service := &Service{
		repo: repo,
		providerSource: syncFakeProvider{model: aiprovider.Model{
			BaseURL: server.URL, Protocol: aiprovider.ProtocolOpenAI,
			APIKeyCiphertext: ciphertext, TimeoutMS: 3000, Vendor: aiprovider.VendorOpenAI,
		}},
		keys: keys, httpClient: server.Client(), now: time.Now,
	}
	result, err := service.Sync(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 || result.Unmatched != 1 || len(repo.items) != 3 {
		t.Fatalf("result = %+v, items = %+v", result, repo.items)
	}
	if repo.items[0].OfficialModelID == nil || *repo.items[0].OfficialModelID != 7 ||
		repo.items[0].PricingMode != pricing.ModeOfficialToken ||
		repo.items[0].PriceUnit != pricing.UnitMillionTokens ||
		repo.items[0].Status != yesno.No {
		t.Fatalf("matched item = %+v", repo.items[0])
	}
	if repo.items[1].OfficialModelID == nil || *repo.items[1].OfficialModelID != 8 ||
		repo.items[1].ModelKind != pricing.KindVideo ||
		repo.items[1].PricingMode != pricing.ModeProviderUnit ||
		repo.items[1].PriceUnit != pricing.UnitSecond ||
		repo.items[1].Status != yesno.No {
		t.Fatalf("video item = %+v", repo.items[1])
	}
	if repo.items[2].OfficialModelID != nil || repo.items[2].PricingMode != pricing.ModeProviderUnit ||
		repo.items[2].Status != yesno.No {
		t.Fatalf("unmatched item = %+v", repo.items[2])
	}
}
