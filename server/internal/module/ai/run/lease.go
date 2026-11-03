package run

import (
	"context"
	"errors"
	"time"
)

var ErrRunLeaseLost = errors.New("AI run execution lease lost")

type LeaseStore interface {
	Renew(context.Context, int64, string, time.Time) error
}

// LeaseHeartbeat keeps the 45 second Run lease alive while the scheduler
// executor owns the parent context. A failed renewal cancels the work before
// another model/tool/COS side effect can begin.
func LeaseHeartbeat(ctx context.Context, store LeaseStore, runID int64, token string, now func() time.Time) (context.Context, context.CancelCauseFunc, <-chan error) {
	work, cancel := context.WithCancelCause(ctx)
	done := make(chan error, 1)
	if store == nil || runID < 1 || token == "" || now == nil {
		cancel(ErrRunLeaseLost)
		done <- ErrRunLeaseLost
		return work, cancel, done
	}
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-work.Done():
				done <- context.Cause(work)
				return
			case <-t.C:
				call, cc := context.WithTimeout(work, time.Second)
				err := store.Renew(call, runID, token, now().UTC())
				cc()
				if err != nil {
					cancel(ErrRunLeaseLost)
					done <- ErrRunLeaseLost
					return
				}
			}
		}
	}()
	return work, cancel, done
}
