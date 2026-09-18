package ratelimitpolicy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/shared/apperror"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"gorm.io/gorm"
)

type serviceRepository interface {
	List(context.Context, int64) (Catalog, error)
	ListAll(context.Context) ([]Catalog, error)
	Update(context.Context, int64, Input, int64, time.Time) (Catalog, cachegeneration.MutationResult, error)
}

type Service struct {
	repository serviceRepository
	runtime    RuntimeCoordinator
}

func NewService(repository serviceRepository) *Service {
	return &Service{repository: repository}
}

func (s *Service) SetRuntimeCoordinator(runtime RuntimeCoordinator) {
	s.runtime = runtime
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
	if s.runtime == nil {
		return Catalog{}, unavailable(fmt.Errorf("mail runtime coordinator is unavailable"))
	}
	var catalog Catalog
	err := s.runtime.Mutate(ctx, func(writeContext context.Context, expected int64) (cachegeneration.MutationResult, error) {
		var mutation cachegeneration.MutationResult
		var updateErr error
		catalog, mutation, updateErr = s.repository.Update(writeContext, platformID, input, expected, time.Now().UTC())
		return mutation, updateErr
	})
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
