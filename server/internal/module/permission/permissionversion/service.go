package permissionversion

import (
	"context"
	"time"
)

type Service struct{ repository *Repository }

func NewService(repository *Repository) *Service { return &Service{repository: repository} }
func (s *Service) Increment(ctx context.Context, userIDs []int64, now time.Time) (map[int64]int64, error) {
	return s.repository.Increment(ctx, userIDs, now)
}
