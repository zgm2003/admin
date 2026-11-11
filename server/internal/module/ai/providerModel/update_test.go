package providermodel

import (
	"context"
	"testing"
	"time"

	"admin/server/internal/module/ai/pricing"
	aiprovider "admin/server/internal/module/ai/provider"
	"admin/server/internal/shared/yesno"
)

type updateFakeRepository struct {
	updated *UpdateValues
}

func (f *updateFakeRepository) List(context.Context, int64, ListQuery) ([]Model, int64, error) {
	return nil, 0, nil
}
func (f *updateFakeRepository) FindByID(context.Context, int64, int64) (Model, error) {
	return Model{ID: 9, ProviderID: 3}, nil
}
func (f *updateFakeRepository) TestChatModel(context.Context, int64) (Model, error) {
	return Model{}, nil
}
func (f *updateFakeRepository) PricesByModelIDs(context.Context, []int64) (map[int64][]Price, error) {
	return map[int64][]Price{}, nil
}
func (f *updateFakeRepository) OfficialByIDs(_ context.Context, ids []int64) (map[int64]Official, error) {
	values := make(map[int64]Official, len(ids))
	for _, id := range ids {
		values[id] = Official{ID: id, Vendor: 1, ModelKey: "gpt-4o", ModelKind: pricing.KindChat}
	}
	return values, nil
}
func (f *updateFakeRepository) OfficialByVendor(context.Context, int16) ([]Official, error) {
	return nil, nil
}
func (f *updateFakeRepository) Sync(context.Context, int64, []SyncItem, time.Time) (SyncCounts, error) {
	return SyncCounts{}, nil
}
func (f *updateFakeRepository) UpdateConfig(_ context.Context, _ int64, _ int64, values UpdateValues) error {
	f.updated = &values
	return nil
}

func TestUpdateAllowsIncompleteDisabledTokenDraft(t *testing.T) {
	repo := &updateFakeRepository{}
	service := &Service{
		repo:           repo,
		providerSource: syncFakeProvider{model: aiprovider.Model{Vendor: aiprovider.VendorOpenAI}},
		now:            func() time.Time { return time.Unix(100, 0) },
	}
	input := UpdateInput{
		DisplayName: "draft", ModelKind: pricing.KindChat, PricingMode: pricing.ModeOfficialToken,
		PriceUnit: pricing.UnitMillionTokens, Status: yesno.No,
	}
	if err := service.Update(context.Background(), 3, 9, input); err != nil {
		t.Fatalf("disabled draft rejected: %v", err)
	}
	if repo.updated == nil || repo.updated.Status != yesno.No {
		t.Fatalf("updated = %+v", repo.updated)
	}
	input.Status = yesno.Yes
	if err := service.Update(context.Background(), 3, 9, input); err == nil {
		t.Fatal("enabled token model without official mapping was accepted")
	}
}
