package rechargepackage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
)

type repository interface {
	Count(context.Context, ListQuery) (int64, error)
	List(context.Context, ListQuery) ([]Model, error)
	FindByID(context.Context, int64) (Model, error)
	Create(context.Context, *Model) error
	Update(context.Context, int64, UpdateInput, time.Time) error
	UpdateStatus(context.Context, int64, yesno.Value, time.Time) error
	MarkDeleted(context.Context, int64, time.Time) error
	Catalog(context.Context) ([]CatalogItem, error)
}

type CatalogItem struct {
	ID          int64
	Code        string
	Name        string
	AmountCents int64
	Badge       *string
	SortOrder   int32
}

type Service struct {
	repository repository
	now        func() time.Time
}

func NewService(repo repository) *Service { return &Service{repository: repo, now: time.Now} }

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[SafeValue], error) {
	if s == nil || s.repository == nil {
		return pagination.Result[SafeValue]{}, dependency(fmt.Errorf("recharge package repository missing"))
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
		value, err := safeValue(ctx, row)
		if err != nil {
			return pagination.Result[SafeValue]{}, dependency(err)
		}
		values = append(values, value)
	}
	return pagination.Result[SafeValue]{List: values, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (SafeValue, error) {
	if id < 1 {
		return SafeValue{}, invalid(fmt.Errorf("id invalid"))
	}
	row, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return SafeValue{}, mapError(err)
	}
	value, err := safeValue(ctx, row)
	if err != nil {
		return SafeValue{}, dependency(err)
	}
	return value, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	if err := validateCreate(input); err != nil {
		return 0, invalid(err)
	}
	badge, err := normalizeBadge(input.Badge)
	if err != nil {
		return 0, invalid(err)
	}
	now := s.now().UTC()
	row := &Model{Code: strings.TrimSpace(input.Code), Name: strings.TrimSpace(input.Name), AmountCents: input.AmountCents, Badge: badge, SortOrder: input.SortOrder, IsEnabled: input.IsEnabled, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.Create(ctx, row); err != nil {
		return 0, mapError(err)
	}
	return row.ID, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	if err := validateUpdate(input); err != nil {
		return invalid(err)
	}
	badge, err := normalizeBadge(input.Badge)
	if err != nil {
		return invalid(err)
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Badge = badge
	return mapError(s.repository.Update(ctx, id, input, s.now().UTC()))
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, value yesno.Value) error {
	if id < 1 || !yesno.IsValid(value) {
		return invalid(fmt.Errorf("status invalid"))
	}
	return mapError(s.repository.UpdateStatus(ctx, id, value, s.now().UTC()))
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	return mapError(s.repository.MarkDeleted(ctx, id, s.now().UTC()))
}

func (s *Service) Catalog(ctx context.Context) ([]CatalogItem, error) {
	if s == nil || s.repository == nil {
		return nil, dependency(fmt.Errorf("recharge package repository missing"))
	}
	rows, err := s.repository.Catalog(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	return rows, nil
}
