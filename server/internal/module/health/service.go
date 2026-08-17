package health

import (
	"context"
	"errors"
	"fmt"

	"admin/server/internal/shared/apperror"
)

type Probe interface {
	Ping(context.Context) error
}

type Service struct {
	postgres Probe
	redis    Probe
}

func NewService(postgres, redis Probe) *Service {
	return &Service{postgres: postgres, redis: redis}
}

func (s *Service) Readiness(ctx context.Context) (Readiness, error) {
	postgresErr := s.postgres.Ping(ctx)
	redisErr := s.redis.Ping(ctx)

	var failures []error
	if postgresErr != nil {
		failures = append(failures, fmt.Errorf("PostgreSQL: %w", postgresErr))
	}
	if redisErr != nil {
		failures = append(failures, fmt.Errorf("Redis: %w", redisErr))
	}
	if len(failures) > 0 {
		return Readiness{}, apperror.DependencyUnavailable(errors.Join(failures...))
	}

	return Readiness{PostgreSQL: "up", Redis: "up"}, nil
}
