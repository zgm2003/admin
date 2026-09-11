package rateLimitPolicy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/shared/apperror"
	"gorm.io/gorm"
)

type Service struct {
	repository repository
	store      Store
}

func NewService(repository repository, store Store) *Service {
	return &Service{repository: repository, store: store}
}

// List is the administrative control-plane read: it always reflects PostgreSQL
// and therefore includes the current revision of every active platform.
func (s *Service) List(ctx context.Context) ([]PlatformResponse, error) {
	catalogs, err := s.repository.ListPlatforms(ctx)
	if err != nil {
		return nil, apperror.DependencyUnavailable(fmt.Errorf("sms rate limit repository: %w", err))
	}
	responses := make([]PlatformResponse, 0, len(catalogs))
	for _, catalog := range catalogs {
		responses = append(responses, platformResponseOf(catalog))
	}
	return responses, nil
}

func (s *Service) Update(ctx context.Context, platformID int64, key string, limit, windowSeconds int) (PlatformResponse, error) {
	if err := ValidateInput(key, limit, windowSeconds); err != nil {
		return PlatformResponse{}, apperror.InvalidRequest(err)
	}
	if s.store == nil {
		return PlatformResponse{}, apperror.DependencyUnavailable(fmt.Errorf("sms rate limit store is unavailable"))
	}
	if _, err := s.repository.FindPlatform(ctx, platformID); errors.Is(err, ErrPlatformNotFound) {
		return PlatformResponse{}, apperror.NotFound(err)
	} else if err != nil {
		return PlatformResponse{}, apperror.DependencyUnavailable(fmt.Errorf("sms rate limit repository: %w", err))
	}

	if err := s.store.Mutate(ctx, platformID, func(writeContext context.Context) error {
		if _, err := s.repository.UpdatePolicy(writeContext, platformID, key, limit, windowSeconds, time.Now().UTC()); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperror.NotFound(err)
			}
			return err
		}
		return nil
	}); err != nil {
		var appError *apperror.Error
		if errors.As(err, &appError) {
			return PlatformResponse{}, err
		}
		return PlatformResponse{}, apperror.DependencyUnavailable(err)
	}

	catalog, err := s.repository.FindPlatform(ctx, platformID)
	if err != nil {
		return PlatformResponse{}, apperror.DependencyUnavailable(fmt.Errorf("sms rate limit repository: %w", err))
	}
	return platformResponseOf(catalog), nil
}

// ProvisionDefaults and DeleteForPlatform keep the policy facts inside the
// authentication platform lifecycle transaction.
func (s *Service) ProvisionDefaults(ctx context.Context, platformID int64) error {
	if err := s.repository.ProvisionDefaults(ctx, platformID, time.Now().UTC()); err != nil {
		return fmt.Errorf("provision sms rate limit policies: %w", err)
	}
	return nil
}

func (s *Service) DeleteForPlatform(ctx context.Context, platformID int64) error {
	if err := s.repository.DeleteForPlatform(ctx, platformID); err != nil {
		return fmt.Errorf("delete sms rate limit policies: %w", err)
	}
	return nil
}

// Catalog returns the runtime catalog for the sending path through the store.
func (s *Service) Catalog(ctx context.Context, platformID int64) (Catalog, error) {
	load := func(loadContext context.Context) (Catalog, error) {
		catalog, err := s.repository.FindPlatform(loadContext, platformID)
		if errors.Is(err, ErrPlatformNotFound) {
			return Catalog{}, ErrPlatformNotFound
		}
		if err != nil {
			return Catalog{}, fmt.Errorf("sms rate limit repository: %w", err)
		}
		if len(catalog.Policies) != len(FixedPolicies()) {
			return Catalog{}, fmt.Errorf("sms rate limit policies are incomplete for platform %d", platformID)
		}
		return catalog, nil
	}
	if s.store == nil {
		return Catalog{}, apperror.DependencyUnavailable(fmt.Errorf("sms rate limit store is unavailable"))
	}
	catalog, err := s.store.Load(ctx, platformID, load)
	if err != nil {
		return Catalog{}, apperror.DependencyUnavailable(err)
	}
	return catalog, nil
}

func platformResponseOf(catalog Catalog) PlatformResponse {
	response := PlatformResponse{
		PlatformID:   catalog.PlatformID,
		PlatformCode: catalog.PlatformCode,
		PlatformName: catalog.PlatformName,
		Policies:     make([]PolicyResponse, 0, len(catalog.Policies)),
	}
	for _, policy := range catalog.Policies {
		response.Policies = append(response.Policies, policyResponseOf(policy))
	}
	return response
}
