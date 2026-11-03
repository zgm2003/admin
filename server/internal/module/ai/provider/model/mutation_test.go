package model

import (
	configcache "admin/server/internal/shared/configCache"
	"context"
	"errors"
	"testing"
)

type rejectedRuntime struct{}

func (rejectedRuntime) Mutate(context.Context, func(context.Context, int64) (configcache.MutationResult, error)) error {
	return errors.New("Redis unavailable")
}
func TestPriceMutationFailsClosedBeforeRepositoryWrite(t *testing.T) {
	for _, runtime := range []RuntimeCoordinator{nil, rejectedRuntime{}} {
		repo := newMemoryPriceRepo()
		s := NewService(repo)
		s.SetRuntimeCoordinator(runtime)
		if _, err := s.PreviewSync(context.Background(), 1, sourceFor(textRate("1")), 1); err == nil || len(repo.versions) != 0 {
			t.Fatal("price mutation bypassed runtime coordinator")
		}
	}
}
func TestPriceSyncRejectsCredentialChangeInsideMutation(t *testing.T) {
	repo := newMemoryPriceRepo()
	s := priceTestService(repo)
	source := sourceFor(textRate("1"))
	source.ExpectedProviderVersionID = 2
	if _, err := s.PreviewSync(context.Background(), 1, source, 1); !errors.Is(err, ErrSourceVersionChanged) || len(repo.versions) != 0 {
		t.Fatal("stale provider source saved")
	}
}
