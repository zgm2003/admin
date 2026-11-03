package provider

import (
	"context"
	"testing"
)

type fixtureModelReader struct{ items []OpenAIModel }

func (f fixtureModelReader) List(context.Context, string, string) ([]OpenAIModel, error) {
	return f.items, nil
}

func TestCatalogPreviewRequiresExplicitCapabilitiesAndKeepsManualMapping(t *testing.T) {
	repo := newMemoryRepository()
	cipher := testCipher(t)
	s := NewService(repo, cipher, fixtureRuntime{})
	p, err := s.Create(context.Background(), fixtureCreate(), 1)
	if err != nil {
		t.Fatal(err)
	}
	catalog := NewCatalogService(repo, cipher, fixtureModelReader{[]OpenAIModel{{ID: "fixture-model", Object: "model", OwnedBy: "fixture"}}})
	preview, err := catalog.Preview(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Added) != 1 || preview.Added[0].ID != "fixture-model" {
		t.Fatalf("preview: %+v", preview)
	}
	if err := catalog.Confirm(context.Background(), p.ID, preview.Digest, nil); err == nil {
		t.Fatal("catalog invented missing operations")
	}
}
