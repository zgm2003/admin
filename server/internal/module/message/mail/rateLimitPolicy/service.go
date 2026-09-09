package ratelimitpolicy

import (
	"context"
	"errors"
	"fmt"

	"admin/server/internal/shared/apperror"
	"gorm.io/gorm"
)

type Service struct {
	repository *Repository
	store      Store
}

func NewService(repository *Repository, store Store) *Service {
	return &Service{repository: repository, store: store}
}

func (s *Service) List(ctx context.Context, platformID int64) (Catalog, error) {
	catalog, err := s.repository.List(ctx, platformID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Catalog{}, notFound(err)
		}
		return Catalog{}, unavailable(err)
	}
	return catalog, nil
}

func (s *Service) ListAll(ctx context.Context) ([]Catalog, error) {
	catalogs, err := s.repository.ListAll(ctx)
	if err != nil {
		return nil, unavailable(err)
	}
	return catalogs, nil
}

func (s *Service) Update(ctx context.Context, platformID int64, input Input) (Catalog, error) {
	if err := ValidateInput(input); err != nil {
		return Catalog{}, invalid(err)
	}
	if s.store == nil {
		return Catalog{}, unavailable(fmt.Errorf("rate limit policy store is unavailable"))
	}
	catalog, err := s.store.Update(ctx, platformID, input)
	if err != nil {
		var applicationError *apperror.Error
		if errors.As(err, &applicationError) && applicationError.Code == apperror.CodeNotFound {
			return Catalog{}, notFound(err)
		}
		return Catalog{}, unavailable(err)
	}
	return catalog, nil
}

func Find(catalog Catalog, key string) (Model, bool) {
	for _, policy := range catalog.Policies {
		if policy.Key == key {
			return policy, true
		}
	}
	return Model{}, false
}
