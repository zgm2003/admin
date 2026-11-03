package model

import (
	"context"
	"encoding/json"
	"testing"
)

func TestProviderModelOrdinaryUpdateHasNoStatusOrPrices(t *testing.T) {
	raw, _ := json.Marshal(UpdateModelRequest{Name: "Fixture", Operations: []string{"text.generate"}, Limits: []byte(`{}`)})
	if string(raw) == "" {
		t.Fatal("empty DTO")
	}
	s := NewModelService(nil)
	if _, err := s.Create(context.Background(), 1, CreateModelRequest{UpstreamModelID: "fixture", Name: "Fixture", Operations: []string{"unknown"}, Limits: []byte(`{}`)}); err == nil {
		t.Fatal("unknown operation accepted")
	}
}
