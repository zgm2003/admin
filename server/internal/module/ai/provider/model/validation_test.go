package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func sourceFor(rates ...SourceRate) SourceSnapshot {
	raw := []byte(`{"official":"fixture"}`)
	sum := sha256.Sum256(raw)
	return SourceSnapshot{Digest: hex.EncodeToString(sum[:]), Payload: raw, SyncedAt: time.Unix(1, 0), Rates: rates}
}
func textRate(price string) SourceRate {
	return SourceRate{Operation: "text.generate", UsageKey: "input", Selector: []byte(`{}`), Unit: "token", UnitQuantity: "1000000", SourceUnitPrice: price}
}

func TestSourceRejectsMalformedDecimalAndOverlappingSelectors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rates []SourceRate
	}{
		{"fraction syntax", []SourceRate{textRate("1/2")}},
		{"exponent syntax", []SourceRate{textRate("1e2")}},
		{"negative zero", []SourceRate{textRate("-0")}},
		{"too many source decimals", []SourceRate{textRate("0.0000000000001")}},
		{"wrong token quantity", []SourceRate{func() SourceRate { r := textRate("1"); r.UnitQuantity = "1"; return r }()}},
		{"same predicate different units", []SourceRate{textRate("1"), func() SourceRate { r := textRate("2"); r.Unit = "request"; r.UnitQuantity = "1"; return r }()}},
		{"overlap", []SourceRate{func() SourceRate { r := textRate("1"); r.Selector = []byte(`{"tier":"standard"}`); return r }(), func() SourceRate {
			r := textRate("2")
			r.Selector = []byte(`{"tier":"standard","size":"large"}`)
			return r
		}()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := priceTestService(newMemoryPriceRepo()).PreviewSync(context.Background(), 1, sourceFor(tc.rates...), 1); err == nil {
				t.Fatal("invalid source accepted")
			}
		})
	}
}
func TestExactInitialRMBPreservesUnrepresentableSourceAsUnset(t *testing.T) {
	for _, price := range []string{"0.000000001", "92233720368.54775808"} {
		p, err := priceTestService(newMemoryPriceRepo()).PreviewSync(context.Background(), 1, sourceFor(textRate(price)), 1)
		if err != nil {
			t.Fatal(err)
		}
		if p.Rates[0].BillingUnitPriceUnits != nil {
			t.Fatal("unrepresentable source was rounded")
		}
		if p.Rates[0].SourceUnitPrice != price {
			t.Fatal("source lost")
		}
	}
}
func TestPreviewUpdateCopiesPublishedSourceAndPublishIsIdempotent(t *testing.T) {
	repo := newMemoryPriceRepo()
	s := priceTestService(repo)
	p, err := s.PreviewSync(context.Background(), 1, sourceFor(textRate("1")), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(context.Background(), 1, p.ID, p.SnapshotDigest, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(context.Background(), 1, p.ID, p.SnapshotDigest, 1); err != nil {
		t.Fatalf("repeat publish: %v", err)
	}
	next, err := s.PreviewUpdate(context.Background(), 1, p.ID, UpdatePriceRequest{Rates: []UpdateRate{{RateID: p.Rates[0].ID, BillingUnitPriceUnits: ptr(222)}}, EffectiveAt: ptrTime(time.Unix(5, 0))}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if next.ID == p.ID || next.SourceDigest != p.SourceDigest || next.Rates[0].SourceUnitPrice != p.Rates[0].SourceUnitPrice || *p.Rates[0].BillingUnitPriceUnits != 100000000 {
		t.Fatal("immutable historical price changed")
	}
	if *next.Rates[0].BillingUnitPriceUnits != 222 || next.SnapshotDigest == p.SnapshotDigest {
		t.Fatal("updated preview missing new price digest")
	}
}
func ptrTime(v time.Time) *time.Time { return &v }
