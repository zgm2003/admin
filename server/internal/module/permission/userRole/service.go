package userrole

import "context"

type Service struct{ repository *Repository }

func NewService(repository *Repository) *Service { return &Service{repository: repository} }
func (s *Service) FindByUser(ctx context.Context, userID int64) ([]UserRole, error) {
	return s.repository.FindByUser(ctx, userID)
}
