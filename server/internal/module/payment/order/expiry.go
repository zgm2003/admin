package order

import (
	"context"
	"errors"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

// orderLifetime reads the current setting only when creating a new order.
// Replays and later payment checks use the immutable expires_at instead.
func (s *Service) orderLifetime(ctx context.Context) (time.Duration, error) {
	if s.settings == nil {
		return 0, apperror.DependencyUnavailable(errors.New("payment order expiry setting missing"))
	}
	row, err := s.settings.FindByKey(ctx, sharedsetting.PaymentOrderExpiryMinutesKey)
	if err != nil {
		return 0, apperror.DependencyUnavailable(err)
	}
	minutes, err := row.Number()
	if err != nil || row.Key != sharedsetting.PaymentOrderExpiryMinutesKey || row.Status != yesno.Yes ||
		len(row.Value) > 4 || strings.IndexFunc(row.Value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 ||
		minutes < sharedsetting.PaymentOrderExpiryMinutesMinimum || minutes > sharedsetting.PaymentOrderExpiryMinutesMaximum {
		return 0, apperror.DependencyUnavailable(errors.New("stored payment order expiry is invalid"))
	}
	return time.Duration(minutes) * time.Minute, nil
}
