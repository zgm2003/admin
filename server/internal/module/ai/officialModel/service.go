package officialmodel

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/shared/pagination"
)

type repository interface {
	Count(context.Context, ListQuery) (int64, error)
	List(context.Context, ListQuery) ([]Model, error)
	UpdatePrice(context.Context, int64, int64, int64, int64, time.Time) error
	ResetPrice(context.Context, int64, int64, time.Time) error
	SyncCatalog(context.Context, Catalog, time.Time) (SyncResult, error)
}

type Service struct {
	repository repository
	now        func() time.Time
	catalog    func() (Catalog, error)
}

func NewService(repo repository) *Service {
	return &Service{repository: repo, now: time.Now, catalog: LoadCatalog}
}

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[SafeValue], error) {
	if s == nil || s.repository == nil {
		return pagination.Result[SafeValue]{}, dependency(fmt.Errorf("official model repository missing"))
	}
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 || (query.Page-1) > int(^uint(0)>>1)/query.PageSize {
		return pagination.Result[SafeValue]{}, invalid(fmt.Errorf("pagination invalid"))
	}
	total, err := s.repository.Count(ctx, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, mapError(err)
	}
	rows, err := s.repository.List(ctx, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, mapError(err)
	}
	values := make([]SafeValue, 0, len(rows))
	for _, row := range rows {
		value, err := safeValue(row)
		if err != nil {
			return pagination.Result[SafeValue]{}, dependency(err)
		}
		values = append(values, value)
	}
	return pagination.Result[SafeValue]{List: values, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) UpdatePrice(ctx context.Context, id int64, input UpdatePriceInput) error {
	if s == nil || s.repository == nil {
		return dependency(fmt.Errorf("official model repository missing"))
	}
	if id < 1 || input.ExpectedVersion < 1 || input.InputCNYMicros < 0 || input.OutputCNYMicros < 0 {
		return invalid(fmt.Errorf("price input is invalid"))
	}
	return mapError(s.repository.UpdatePrice(ctx, id, input.ExpectedVersion, input.InputCNYMicros, input.OutputCNYMicros, s.now().UTC()))
}

func (s *Service) ResetPrice(ctx context.Context, id int64, input ResetPriceInput) error {
	if s == nil || s.repository == nil {
		return dependency(fmt.Errorf("official model repository missing"))
	}
	if id < 1 || input.ExpectedVersion < 1 {
		return invalid(fmt.Errorf("reset input is invalid"))
	}
	return mapError(s.repository.ResetPrice(ctx, id, input.ExpectedVersion, s.now().UTC()))
}

func (s *Service) SyncCatalog(ctx context.Context) (SyncResult, error) {
	if s == nil || s.repository == nil || s.catalog == nil {
		return SyncResult{}, dependency(fmt.Errorf("official model repository missing"))
	}
	catalog, err := s.catalog()
	if err != nil {
		return SyncResult{}, dependency(err)
	}
	result, err := s.repository.SyncCatalog(ctx, catalog, s.now().UTC())
	if err != nil {
		return SyncResult{}, mapError(err)
	}
	return result, nil
}
