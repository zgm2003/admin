package provider

import (
	"context"
	"testing"
)

func TestProviderMutationRequiresRuntimeCoordinator(t *testing.T) {
	repo := newMemoryRepository()
	s := NewService(repo, testCipher(t))
	if _, err := s.Create(context.Background(), fixtureCreate(), 1); err == nil || len(repo.providers) != 0 {
		t.Fatal("provider configuration committed without runtime coordinator")
	}
}
