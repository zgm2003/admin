package notification

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetentionTriggerRunsImmediatelyAndSerially(t *testing.T) {
	cleaner := &notificationCleanerStub{started: make(chan struct{}, 1), release: make(chan struct{})}
	trigger := NewRetentionTrigger(cleaner, nil)
	trigger.interval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- trigger.Run(ctx) }()
	select {
	case <-cleaner.started:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not start")
	}
	if cleaner.maximum.Load() != 1 {
		t.Fatalf("maximum=%d", cleaner.maximum.Load())
	}
	cancel()
	close(cleaner.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("trigger did not stop")
	}
}

type notificationCleanerStub struct {
	started chan struct{}
	release chan struct{}
	current atomic.Int32
	maximum atomic.Int32
}

func (s *notificationCleanerStub) Cleanup(context.Context) (CleanupResult, error) {
	value := s.current.Add(1)
	defer s.current.Add(-1)
	for {
		old := s.maximum.Load()
		if value <= old || s.maximum.CompareAndSwap(old, value) {
			break
		}
	}
	select {
	case s.started <- struct{}{}:
	default:
	}
	<-s.release
	return CleanupResult{}, nil
}
