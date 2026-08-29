package loginlog

import (
	"context"
	"fmt"

	"admin/server/internal/shared/apperror"
)

type Store interface {
	Insert(context.Context, Event) error
	List(context.Context, ListQuery) ([]Item, int64, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) Record(ctx context.Context, event Event) error {
	if err := ValidateEvent(event); err != nil {
		return apperror.InvalidRequest(err)
	}
	if err := s.store.Insert(ctx, event); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return ListResult{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	items, total, err := s.store.List(ctx, query)
	if err != nil {
		return ListResult{}, apperror.DependencyUnavailable(err)
	}
	return ListResult{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

type ListResult struct {
	List     []Item `json:"list"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}
