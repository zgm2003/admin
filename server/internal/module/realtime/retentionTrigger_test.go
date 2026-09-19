package realtime

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetentionTriggerRunsImmediatelyWithoutOverlap(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	cleaner := &retentionCleanerStub{started: started, release: release}
	trigger := NewRetentionTrigger(cleaner, nil)
	trigger.interval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- trigger.Run(ctx) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("initial cleanup did not start")
	}
	if cleaner.maxConcurrent.Load() != 1 {
		t.Fatalf("max concurrent=%d", cleaner.maxConcurrent.Load())
	}
	cancel()
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("trigger did not stop")
	}
}

type retentionCleanerStub struct {
	started       chan struct{}
	release       chan struct{}
	concurrent    atomic.Int32
	maxConcurrent atomic.Int32
}

func (s *retentionCleanerStub) Cleanup(context.Context) (CleanupResult, error) {
	current := s.concurrent.Add(1)
	defer s.concurrent.Add(-1)
	for {
		maximum := s.maxConcurrent.Load()
		if current <= maximum || s.maxConcurrent.CompareAndSwap(maximum, current) {
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
