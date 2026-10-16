package wallet

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/money"
	"admin/server/internal/shared/pagination"
)

var ErrBalanceOverflow = errors.New("wallet balance overflow")

type repository interface {
	Find(context.Context, int64) (Model, bool, error)
}

type adminRepository interface {
	Count(context.Context) (int64, error)
	List(context.Context, int, int) ([]Model, error)
}

type adminFinder interface {
	FindAdmin(context.Context, int64) (Model, bool, error)
}

type Service struct{ repository repository }

func NewService(repository repository) *Service { return &Service{repository: repository} }

type PersonalValue struct {
	HasWallet bool   `json:"hasWallet"`
	Currency  string `json:"currency"`
	Balance   string `json:"balance"`
}

type Value struct {
	PersonalValue
	UserID       int64 `json:"-"`
	BalanceUnits int64 `json:"-"`
}

type AdminValue struct {
	UserID    int64     `json:"userId"`
	Username  string    `json:"username"`
	Currency  string    `json:"currency"`
	Balance   string    `json:"balance"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (s *Service) Get(ctx context.Context, userID int64) (Value, error) {
	if userID < 1 {
		return Value{}, apperror.InvalidRequest(errors.New("user id invalid"))
	}
	if s == nil || s.repository == nil {
		return Value{}, apperror.DependencyUnavailable(errors.New("wallet repository missing"))
	}
	row, found, err := s.repository.Find(ctx, userID)
	if err != nil {
		return Value{}, apperror.DependencyUnavailable(err)
	}
	if !found {
		return Value{UserID: userID, PersonalValue: PersonalValue{Currency: "CNY", Balance: "0"}}, nil
	}
	if row.UserID != userID {
		return Value{}, apperror.DependencyUnavailable(errors.New("wallet user mismatch"))
	}
	balance, err := money.FormatRMBUnits(row.BalanceUnits)
	if err != nil {
		return Value{}, apperror.DependencyUnavailable(err)
	}
	return Value{PersonalValue: PersonalValue{HasWallet: true, Currency: "CNY", Balance: balance}, UserID: userID, BalanceUnits: row.BalanceUnits}, nil
}

func (s *Service) AdminGet(ctx context.Context, userID int64) (AdminValue, error) {
	if userID < 1 {
		return AdminValue{}, apperror.InvalidRequest(errors.New("user id invalid"))
	}
	if s == nil {
		return AdminValue{}, apperror.DependencyUnavailable(errors.New("wallet repository missing"))
	}
	repo, ok := s.repository.(adminFinder)
	if !ok {
		return AdminValue{}, apperror.DependencyUnavailable(errors.New("wallet repository missing"))
	}
	row, found, err := repo.FindAdmin(ctx, userID)
	if err != nil {
		return AdminValue{}, apperror.DependencyUnavailable(err)
	}
	if !found {
		return AdminValue{}, apperror.NotFound(nil)
	}
	if row.UserID != userID {
		return AdminValue{}, apperror.DependencyUnavailable(errors.New("wallet user mismatch"))
	}
	balance, err := money.FormatRMBUnits(row.BalanceUnits)
	if err != nil {
		return AdminValue{}, apperror.DependencyUnavailable(err)
	}
	return AdminValue{UserID: row.UserID, Username: row.Username, Currency: "CNY", Balance: balance, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func checkedAdd(balance, amount int64) (int64, error) {
	if amount > 0 && balance > math.MaxInt64-amount {
		return 0, ErrBalanceOverflow
	}
	if amount < 0 && balance < math.MinInt64-amount {
		return 0, ErrBalanceOverflow
	}
	return balance + amount, nil
}

func (s *Service) AdminList(ctx context.Context, page, pageSize int) (pagination.Result[AdminValue], error) {
	if page < 1 || pageSize < 1 || pageSize > 100 || (page-1) > int(^uint(0)>>1)/pageSize {
		return pagination.Result[AdminValue]{}, apperror.InvalidRequest(fmt.Errorf("pagination invalid"))
	}
	if s == nil {
		return pagination.Result[AdminValue]{}, apperror.DependencyUnavailable(errors.New("wallet repository missing"))
	}
	repo, ok := s.repository.(adminRepository)
	if !ok {
		return pagination.Result[AdminValue]{}, apperror.DependencyUnavailable(errors.New("wallet repository missing"))
	}
	total, err := repo.Count(ctx)
	if err != nil {
		return pagination.Result[AdminValue]{}, apperror.DependencyUnavailable(err)
	}
	rows, err := repo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return pagination.Result[AdminValue]{}, apperror.DependencyUnavailable(err)
	}
	values := make([]AdminValue, 0, len(rows))
	for _, row := range rows {
		balance, formatErr := money.FormatRMBUnits(row.BalanceUnits)
		if formatErr != nil || row.UserID < 1 {
			return pagination.Result[AdminValue]{}, apperror.DependencyUnavailable(errors.Join(formatErr, errors.New("stored wallet is invalid")))
		}
		values = append(values, AdminValue{UserID: row.UserID, Username: row.Username, Currency: "CNY", Balance: balance, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	return pagination.Result[AdminValue]{List: values, Total: total, Page: page, PageSize: pageSize}, nil
}
