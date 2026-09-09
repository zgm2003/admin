package operationlog

import (
	"context"
	"fmt"

	"admin/server/internal/shared/apperror"
)

type repository interface {
	Insert(context.Context, TaskPayload) error
	List(context.Context, ListQuery) ([]Item, int64, error)
}

type Service struct {
	repository repository
}

func NewService(repository repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Process(ctx context.Context, payload TaskPayload) error {
	if err := validateTaskPayload(payload); err != nil {
		return fmt.Errorf("process operation log payload: %w", err)
	}
	return s.repository.Insert(ctx, payload)
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return ListResult{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	items, total, err := s.repository.List(ctx, query)
	if err != nil {
		return ListResult{}, apperror.DependencyUnavailable(err)
	}
	return ListResult{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}
