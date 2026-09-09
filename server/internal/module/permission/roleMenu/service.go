package rolemenu

import "context"

type Service struct{ repository *Repository }

func NewService(repository *Repository) *Service { return &Service{repository: repository} }
func (s *Service) FindByRole(ctx context.Context, roleID int64) ([]RoleMenu, error) {
	return s.repository.FindByRole(ctx, roleID)
}
