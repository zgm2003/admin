package server

import (
	"context"
	"errors"
	"sync"
	"time"

	"admin/server/internal/shared/apperror"
)

type collector interface {
	Collect(context.Context) (Snapshot, error)
}
type collection struct {
	done  chan struct{}
	ctx   context.Context
	value Snapshot
	err   error
}

type Service struct {
	collector collector
	mu        sync.Mutex
	value     Snapshot
	err       error
	expiresAt time.Time
	inflight  *collection
}

func NewService(collector collector) *Service { return &Service{collector: collector} }

func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, apperror.DependencyUnavailable(err)
	}
	s.mu.Lock()
	if time.Now().Before(s.expiresAt) {
		value := s.value
		err := s.err
		s.mu.Unlock()
		return value, err
	}
	pending := s.inflight
	if pending == nil {
		work, cancel := context.WithTimeout(ctx, 3*time.Second)
		pending = &collection{done: make(chan struct{}), ctx: work}
		s.inflight = pending
		// An OS call may ignore cancellation. Keep its single slot until it exits,
		// while allowing every HTTP caller to stop waiting within the budget.
		go s.collect(ctx, work, cancel, pending)
	}
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return Snapshot{}, apperror.DependencyUnavailable(ctx.Err())
	case <-pending.done:
		return pending.value, pending.err
	case <-pending.ctx.Done():
		// Successful completion also cancels the work context, but only after
		// publishing the result. Prefer that result over the cleanup cancellation.
		select {
		case <-pending.done:
			return pending.value, pending.err
		default:
			return Snapshot{}, apperror.DependencyUnavailable(pending.ctx.Err())
		}
	}
}

func (s *Service) collect(ctx, work context.Context, cancel context.CancelFunc, pending *collection) {
	defer cancel()
	value, err := s.collector.Collect(work)
	if err == nil {
		err = work.Err()
	}
	if err != nil {
		value = Snapshot{}
		err = apperror.DependencyUnavailable(err)
	}
	s.mu.Lock()
	// Bound retries during OS failures as well as successful sampling.
	s.value, s.err = value, err
	s.expiresAt = time.Now().Add(5 * time.Second)
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		// A disconnected caller must not hold other callers in a failure cooldown.
		s.expiresAt = time.Time{}
	}
	pending.value, pending.err = value, err
	s.inflight = nil
	close(pending.done)
	s.mu.Unlock()
}
