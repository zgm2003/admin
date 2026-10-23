package order

import (
	"context"
	"errors"
	"math"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/money"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

const maximumRechargeCents = math.MaxInt64 / money.UnitsPerCent

// Settings reads through the existing cross-instance setting generation cache.
func (s *Service) SetSettings(value sharedsetting.Reader) { s.settings = value }

func (s *Service) minimumRechargeCents(ctx context.Context) (int64, error) {
	if s == nil || s.settings == nil {
		return 0, apperror.DependencyUnavailable(errors.New("recharge settings missing"))
	}
	row, err := s.settings.FindByKey(ctx, sharedsetting.PaymentRechargeMinimumAmountKey)
	if err != nil {
		return 0, apperror.DependencyUnavailable(err)
	}
	minimum, err := row.Number()
	if err != nil || row.Key != sharedsetting.PaymentRechargeMinimumAmountKey || row.Status != yesno.Yes || minimum < 1 || int64(minimum) > maximumRechargeCents/100 {
		return 0, apperror.DependencyUnavailable(errors.New("stored recharge minimum is invalid"))
	}
	return int64(minimum) * 100, nil
}

func (s *Service) rechargeLimits(ctx context.Context) (RechargeLimits, error) {
	minimum, err := s.minimumRechargeCents(ctx)
	if err != nil {
		return RechargeLimits{}, err
	}
	minimumAmount, err := money.FormatRMBCents(minimum)
	if err != nil {
		return RechargeLimits{}, apperror.DependencyUnavailable(err)
	}
	maximumAmount, err := money.FormatRMBCents(maximumRechargeCents)
	if err != nil {
		return RechargeLimits{}, apperror.DependencyUnavailable(err)
	}
	return RechargeLimits{MinimumAmount: minimumAmount, MaximumAmount: maximumAmount, AmountScale: 2}, nil
}
