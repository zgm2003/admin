package callbackevent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/payment/alipay"
	"admin/server/internal/shared/money"
)

type pendingRepository interface {
	ListPending(context.Context, time.Time, time.Time, int64, int) ([]Model, error)
}

// RecoverPending consumes only the closed facts persisted after RSA2
// verification. The settlement transaction locks the event, order and wallet;
// concurrent deliveries/recovery jobs therefore need no second event lease.
func (s *Service) RecoverPending(ctx context.Context) error {
	if ctx == nil || s == nil || s.repository == nil || s.runtime == nil || s.confirmer == nil || !s.environment.Valid() {
		return errors.New("payment callback recovery dependencies missing")
	}
	repo, ok := s.repository.(pendingRepository)
	if !ok {
		return errors.New("payment callback pending repository missing")
	}
	work, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cutoff := s.now().UTC().Add(-15 * time.Second)
	var after time.Time
	var afterID int64
	var runErr error
	for range 20 {
		if work.Err() != nil {
			return errors.Join(runErr, ctx.Err())
		}
		rows, err := repo.ListPending(work, cutoff, after, afterID, 100)
		if err != nil {
			return errors.Join(runErr, err)
		}
		for _, row := range rows {
			if work.Err() != nil {
				return errors.Join(runErr, ctx.Err())
			}
			if row.ID <= 0 || row.CreatedAt.After(cutoff) || row.CreatedAt.Before(after) || (row.CreatedAt.Equal(after) && row.ID <= afterID) {
				return errors.Join(runErr, errors.New("payment callback pending cursor invalid"))
			}
			after, afterID = row.CreatedAt, row.ID
			if err := s.recoverEvent(work, row); err != nil {
				if work.Err() != nil {
					return errors.Join(runErr, ctx.Err())
				}
				runErr = errors.Join(runErr, fmt.Errorf("recover payment callback %d: %w", row.ID, err))
			}
		}
		if len(rows) < 100 {
			return runErr
		}
	}
	return runErr
}

func (s *Service) recoverEvent(ctx context.Context, row Model) error {
	stored, err := value(ctx, row)
	if err != nil {
		return err
	}
	if stored.ProcessStatus != ProcessPending {
		return errors.New("payment callback recovery row is not pending")
	}
	credentials, err := s.runtime.Credentials(ctx, row.ConfigID, row.ConfigVersion)
	if err != nil {
		return err
	}
	if credentials.Environment != s.environment || !validFact(credentials.AppID, 64) || !validFact(credentials.SellerID, 64) {
		return errors.New("payment callback recovery environment or identity mismatch")
	}
	payload := stored.Payload
	amount, err := money.ParseRMBCents(payload.TotalAmount)
	if err != nil {
		return err
	}
	notify := alipay.Notify{OrderNo: payload.OrderNo, AppID: payload.AppID, SellerID: payload.SellerID, AmountCents: amount, Status: payload.TradeStatus, PaidAt: payload.PaidAt}
	if payload.TradeNo != nil {
		notify.TradeNo = *payload.TradeNo
	}
	if payload.NotifyID != nil {
		notify.NotifyID = *payload.NotifyID
	}
	_, err = s.process(ctx, row, notify, credentials, true)
	return err
}
