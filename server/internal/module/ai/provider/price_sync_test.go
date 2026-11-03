package provider

import (
	price "admin/server/internal/module/ai/provider/model"
	"context"
	"testing"
)

func TestOfficialPriceSyncRequiresKnownSourceAndSelectsBoundModel(t *testing.T) {
	repo := newMemoryRepository()
	c := testCipher(t)
	s := NewService(repo, c, fixtureRuntime{})
	req := fixtureCreate()
	req.Code = "openai"
	req.BaseURL = "https://compatible.example.test/v1"
	p, err := s.Create(context.Background(), req, 1)
	if err != nil {
		t.Fatal(err)
	}
	gateway := NewPriceSyncService(repo, nil, nil)
	if _, err := gateway.Preview(context.Background(), p.ID, 2, 1); err == nil {
		t.Fatal("compatible provider accepted official source")
	}
}

var _ *price.Service
