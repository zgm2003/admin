package transaction

import (
	"context"
	"errors"
	"fmt"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"gorm.io/gorm"
)

func (s *Service) Get(ctx context.Context, id int64) (Value, error) {
	if id < 1 {
		return Value{}, apperror.InvalidRequest(fmt.Errorf("transaction id invalid"))
	}
	row, err := s.repository.Find(ctx, id)
	if err != nil {
		return Value{}, mapError(err)
	}
	value, err := value(ctx, row)
	if err != nil {
		return Value{}, apperror.DependencyUnavailable(err)
	}
	return value, nil
}

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[Value], error) {
	rows, total, err := s.list(ctx, query)
	if err != nil {
		return pagination.Result[Value]{}, err
	}
	projected, err := page(ctx, rows, total, query)
	if err != nil {
		return pagination.Result[Value]{}, apperror.DependencyUnavailable(err)
	}
	return pagination.Result[Value]{List: projected.List, Total: projected.Total, Page: projected.Page, PageSize: projected.PageSize}, nil
}

func (s *Service) PersonalList(ctx context.Context, query ListQuery) (pagination.Result[PersonalValue], error) {
	query.Personal = true
	rows, total, err := s.list(ctx, query)
	if err != nil {
		return pagination.Result[PersonalValue]{}, err
	}
	values := make([]PersonalValue, 0, len(rows))
	for _, row := range rows {
		if row.UserID != query.UserID || row.PlatformID != query.PlatformID {
			return pagination.Result[PersonalValue]{}, apperror.DependencyUnavailable(fmt.Errorf("transaction identity mismatch"))
		}
		projected, err := value(ctx, row)
		if err != nil {
			return pagination.Result[PersonalValue]{}, apperror.DependencyUnavailable(err)
		}
		values = append(values, projected.PersonalValue)
	}
	return pagination.Result[PersonalValue]{List: values, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) list(ctx context.Context, query ListQuery) ([]Model, int64, error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 || (query.Page-1) > int(^uint(0)>>1)/query.PageSize {
		return nil, 0, apperror.InvalidRequest(fmt.Errorf("pagination invalid"))
	}
	if query.Personal && (query.UserID < 1 || query.PlatformID < 1) {
		return nil, 0, apperror.InvalidRequest(fmt.Errorf("identity invalid"))
	}
	if s == nil || s.repository == nil {
		return nil, 0, apperror.DependencyUnavailable(fmt.Errorf("transaction repository missing"))
	}
	total, err := s.repository.Count(ctx, query)
	if err != nil {
		return nil, 0, mapError(err)
	}
	rows, err := s.repository.List(ctx, query)
	if err != nil {
		return nil, 0, mapError(err)
	}
	return rows, total, nil
}

func mapError(err error) error {
	var public *apperror.Error
	switch {
	case errors.As(err, &public):
		return err
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperror.NotFound(err)
	default:
		return apperror.DependencyUnavailable(err)
	}
}
