package model

import (
	"context"
	"testing"
)

func TestSelectorNumericNormalizationPreservesExistingRMB(t *testing.T) {
	repo := newMemoryPriceRepo()
	s := priceTestService(repo)
	r := textRate("1")
	r.Selector = []byte(`{"size":1}`)
	p, err := s.PreviewSync(context.Background(), 1, sourceFor(r), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetBillingPrice(context.Background(), 1, p.ID, UpdatePriceRequest{Rates: []UpdateRate{{RateID: p.Rates[0].ID, BillingUnitPriceUnits: ptr(555)}}}, 1); err != nil {
		t.Fatal(err)
	}
	r.Selector = []byte(`{"size":1.0}`)
	next, err := s.PreviewSync(context.Background(), 1, sourceFor(r), 1)
	if err != nil {
		t.Fatal(err)
	}
	if *next.Rates[0].BillingUnitPriceUnits != 555 {
		t.Fatal("numeric selector representation reset RMB")
	}
}
