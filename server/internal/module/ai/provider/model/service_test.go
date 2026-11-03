package model

import (
	configcache "admin/server/internal/shared/configCache"
	"context"
	"errors"
	"testing"
	"time"
)

func TestSyncPreviewCopiesDollarToRMBOnlyForNewIdentity(t *testing.T) {
	repo := newMemoryPriceRepo()
	svc := priceTestService(repo)
	first, err := svc.PreviewSync(context.Background(), 7, SourceSnapshot{Digest: "source-a", Payload: []byte(`{"models":1}`), SyncedAt: time.Unix(1, 0), Rates: []SourceRate{{Operation: "text.generate", UsageKey: "input", Selector: []byte(`{}`), Unit: "token", UnitQuantity: "1000000", SourceUnitPrice: "5"}}}, 9)
	if err != nil {
		t.Fatal(err)
	}
	if first.Rates[0].BillingUnitPriceUnits == nil || *first.Rates[0].BillingUnitPriceUnits != 500000000 {
		t.Fatalf("initial RMB=%v", first.Rates[0].BillingUnitPriceUnits)
	}
	if err := svc.SetBillingPrice(context.Background(), 7, first.ID, UpdatePriceRequest{Rates: []UpdateRate{{RateID: first.Rates[0].ID, BillingUnitPriceUnits: ptr(int64(700000000))}}}, 9); err != nil {
		t.Fatal(err)
	}
	second, err := svc.PreviewSync(context.Background(), 7, SourceSnapshot{Digest: "source-b", Payload: []byte(`{"models":2}`), SyncedAt: time.Unix(2, 0), Rates: []SourceRate{{Operation: "text.generate", UsageKey: "input", Selector: []byte(`{}`), Unit: "token", UnitQuantity: "1000000", SourceUnitPrice: "6"}, {Operation: "text.generate", UsageKey: "output", Selector: []byte(`{}`), Unit: "token", UnitQuantity: "1000000", SourceUnitPrice: "2"}}}, 9)
	if err != nil {
		t.Fatal(err)
	}
	if *second.Rates[0].BillingUnitPriceUnits != 700000000 {
		t.Fatalf("existing RMB overwritten: %d", *second.Rates[0].BillingUnitPriceUnits)
	}
	if *second.Rates[1].BillingUnitPriceUnits != 200000000 {
		t.Fatalf("new RMB not copied: %d", *second.Rates[1].BillingUnitPriceUnits)
	}
}

func TestPublishRequiresPreviewDigestAndFreezesVersion(t *testing.T) {
	repo := newMemoryPriceRepo()
	svc := priceTestService(repo)
	p, err := svc.PreviewSync(context.Background(), 1, SourceSnapshot{Digest: "abc", Payload: []byte(`{}`), SyncedAt: time.Now(), Rates: []SourceRate{{Operation: "text.generate", UsageKey: "input", Selector: []byte(`{}`), Unit: "token", UnitQuantity: "1000000", SourceUnitPrice: "1"}}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Publish(context.Background(), 1, p.ID, "wrong", 2); err == nil {
		t.Fatal("wrong digest accepted")
	}
	if err := svc.Publish(context.Background(), 1, p.ID, p.SnapshotDigest, 2); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetBillingPrice(context.Background(), 1, p.ID, UpdatePriceRequest{Rates: []UpdateRate{{RateID: p.Rates[0].ID, BillingUnitPriceUnits: ptr(int64(2))}}}, 2); err == nil {
		t.Fatal("published version mutable")
	}
}

func ptr(v int64) *int64 { return &v }

type memoryPriceRepo struct {
	versions []*PriceVersion
	next     int64
}
type fixtureRuntime struct{}

func (m *memoryPriceRepo) CurrentProviderVersionID(context.Context, int64) (int64, error) {
	return 1, nil
}

func (fixtureRuntime) Mutate(ctx context.Context, fn func(context.Context, int64) (configcache.MutationResult, error)) error {
	_, err := fn(ctx, 1)
	return err
}
func priceTestService(repo Repository) *Service { return NewService(repo, fixtureRuntime{}) }
func (m *memoryPriceRepo) Mutate(ctx context.Context, base int64, fn func(Repository) error) (configcache.MutationResult, error) {
	err := fn(m)
	if errors.Is(err, errNoRuntimeChange) {
		return configcache.MutationResult{Changed: false, Generation: base}, nil
	}
	return configcache.MutationResult{Changed: err == nil, Generation: base + 1, OutboxID: 1}, err
}

func newMemoryPriceRepo() *memoryPriceRepo { return &memoryPriceRepo{next: 1} }
func (m *memoryPriceRepo) WithModel(ctx context.Context, _ int64, fn func(Repository) error) error {
	return fn(m)
}
func (m *memoryPriceRepo) Effective(_ context.Context, modelID int64, at time.Time) (*PriceVersion, error) {
	for i := len(m.versions) - 1; i >= 0; i-- {
		v := m.versions[i]
		if v.ProviderModelID == modelID && v.Status == PriceStatusPublished && v.EffectiveAt != nil && !v.EffectiveAt.After(at) {
			return v, nil
		}
	}
	return nil, ErrPriceNotFound
}
func (m *memoryPriceRepo) CreatePreview(_ context.Context, p *PriceVersion) error {
	p.ID = m.next
	m.next++
	for i := range p.Rates {
		p.Rates[i].ID = m.next
		m.next++
	}
	m.versions = append(m.versions, p)
	return nil
}
func (m *memoryPriceRepo) Latest(_ context.Context, _ int64) (*PriceVersion, error) {
	if len(m.versions) == 0 {
		return nil, nil
	}
	return m.versions[len(m.versions)-1], nil
}
func (m *memoryPriceRepo) UpdatePreview(_ context.Context, p *PriceVersion) error { return nil }
func (m *memoryPriceRepo) Publish(_ context.Context, _ int64, _ int64, _ string, _ int64) error {
	for _, v := range m.versions {
		if v.Status == PriceStatusPublished {
			return ErrPublishedImmutable
		}
		v.Status = PriceStatusPublished
	}
	return nil
}
func (m *memoryPriceRepo) Find(_ context.Context, _ int64, id int64) (*PriceVersion, error) {
	for _, v := range m.versions {
		if v.ID == id {
			return v, nil
		}
	}
	return nil, ErrPriceNotFound
}
