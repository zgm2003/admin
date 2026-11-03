package provider

import (
	configcache "admin/server/internal/shared/configCache"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"time"
)

type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context, int64) (configcache.MutationResult, error)) error
}

func (s *Service) SetRuntimeCoordinator(runtime RuntimeCoordinator) { s.runtime = runtime }
func (s *Service) mutate(ctx context.Context, fn func(Repository) error) error {
	if s.runtime == nil {
		return errors.New("AI runtime mutation coordinator unavailable")
	}
	return s.runtime.Mutate(ctx, func(ctx context.Context, base int64) (configcache.MutationResult, error) {
		return s.repository.Mutate(ctx, base, fn)
	})
}
func (r *GormRepository) Mutate(ctx context.Context, expected int64, fn func(Repository) error) (configcache.MutationResult, error) {
	var result configcache.MutationResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := fn(NewRepository(tx)); err != nil {
			return fmt.Errorf("%w: %w", configcache.ErrMutationRolledBack, err)
		}
		event, err := configcache.NewRepository(tx).AdvanceTx(ctx, tx, configcache.Scope{Namespace: "ai.runtime", ScopeKey: "global"}, expected, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("%w: %w", configcache.ErrMutationRolledBack, err)
		}
		result = configcache.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	return result, err
}
