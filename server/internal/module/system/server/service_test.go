package server

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testCollector struct {
	calls   atomic.Int32
	collect func(context.Context) (Snapshot, error)
}

func (c *testCollector) Collect(ctx context.Context) (Snapshot, error) {
	c.calls.Add(1)
	return c.collect(ctx)
}

func TestSnapshotCoalescesConcurrentRequestsAndDoesNotServeExpiredDataAfterFailure(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	c := &testCollector{collect: func(ctx context.Context) (Snapshot, error) {
		close(started)
		select {
		case <-release:
			return Snapshot{Hostname: "node-a"}, nil
		case <-ctx.Done():
			return Snapshot{}, ctx.Err()
		}
	}}
	s := NewService(c)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Go(func() {
			value, err := s.Snapshot(context.Background())
			if err == nil && value.Hostname != "node-a" {
				err = errors.New("missing collected host")
			}
			errs <- err
		})
	}
	<-started
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if c.calls.Load() != 1 {
		t.Fatalf("collections = %d", c.calls.Load())
	}
	s.mu.Lock()
	s.expiresAt = time.Now().Add(-time.Second)
	s.mu.Unlock()
	c.collect = func(context.Context) (Snapshot, error) { return Snapshot{}, errors.New("OS unavailable") }
	value, err := s.Snapshot(context.Background())
	if err == nil || value.Hostname != "" {
		t.Fatalf("expired value returned as success: %+v, %v", value, err)
	}
	for range 16 {
		if _, err := s.Snapshot(context.Background()); err == nil {
			t.Fatal("failure cooldown returned success")
		}
	}
	if c.calls.Load() != 2 {
		t.Fatalf("failure retry storm: %d", c.calls.Load())
	}
}

func TestSnapshotPropagatesCancellationAndBoundsCollection(t *testing.T) {
	type key struct{}
	started := make(chan struct{})
	c := &testCollector{collect: func(ctx context.Context) (Snapshot, error) {
		if ctx.Value(key{}) != "request" {
			t.Error("request context lost")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 3*time.Second {
			t.Error("missing collection budget")
		}
		close(started)
		<-ctx.Done()
		return Snapshot{}, ctx.Err()
	}}
	s := NewService(c)
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key{}, "request"))
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := s.Snapshot(ctx); done <- err }()
	<-started
	s.mu.Lock()
	pending := s.inflight
	s.mu.Unlock()
	waiter, cancelWaiter := context.WithCancel(context.Background())
	cancelWaiter()
	if _, err := s.Snapshot(waiter); err == nil {
		t.Fatal("cancelled waiter succeeded")
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled request succeeded")
	}
	<-pending.done
	c.collect = func(context.Context) (Snapshot, error) { return Snapshot{Hostname: "recovered"}, nil }
	if value, err := s.Snapshot(context.Background()); err != nil || value.Hostname != "recovered" {
		t.Fatal("cancelled leader poisoned subsequent samples")
	}
}

func TestNativeCollectorReturnsHostResourcesAndRuntime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	value, err := NewCollector(time.Now().Add(-time.Minute)).Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if value.Hostname == "" || value.OS == "" || value.Arch == "" || value.LogicalCores < 1 || value.Memory.TotalBytes == 0 || value.Disk.TotalBytes == 0 || value.GoVersion == "" || value.ProcessUptimeSeconds < 60 {
		t.Fatalf("incomplete real snapshot: %+v", value)
	}
	for _, percent := range []float64{value.CPUPercent, value.Memory.UsedPercent, value.Disk.UsedPercent} {
		if percent < 0 || percent > 100 {
			t.Fatalf("invalid usage: %v", percent)
		}
	}
	if value.Memory.AvailableBytes > value.Memory.TotalBytes || value.Disk.AvailableBytes > value.Disk.TotalBytes {
		t.Fatal("invalid available capacity")
	}
}

func TestSnapshotCallerDeadlineDoesNotPoisonSubsequentSamples(t *testing.T) {
	started := make(chan struct{})
	c := &testCollector{collect: func(ctx context.Context) (Snapshot, error) {
		close(started)
		<-ctx.Done()
		return Snapshot{}, ctx.Err()
	}}
	s := NewService(c)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := s.Snapshot(ctx); result <- err }()
	<-started
	s.mu.Lock()
	pending := s.inflight
	s.mu.Unlock()
	if err := <-result; err == nil {
		t.Fatal("expired caller succeeded")
	}
	<-pending.done
	c.collect = func(context.Context) (Snapshot, error) { return Snapshot{Hostname: "recovered"}, nil }
	if value, err := s.Snapshot(context.Background()); err != nil || value.Hostname != "recovered" {
		t.Fatal("caller deadline poisoned subsequent samples")
	}
}

func TestSnapshotBoundsWaitingWhenCollectorIgnoresCancellation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	c := &testCollector{collect: func(context.Context) (Snapshot, error) {
		close(started)
		<-release
		return Snapshot{Hostname: "late"}, nil
	}}
	s := NewService(c)
	result := make(chan error, 1)
	go func() { _, err := s.Snapshot(context.Background()); result <- err }()
	<-started
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("late sample succeeded")
		}
	case <-time.After(3500 * time.Millisecond):
		t.Fatal("collector blocked request beyond sampling budget")
	}
	for range 8 {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, err := s.Snapshot(ctx)
		cancel()
		if err == nil {
			t.Fatal("blocked collector returned a successful sample")
		}
	}
	if c.calls.Load() != 1 {
		t.Fatalf("blocked collector spawned additional jobs: %d", c.calls.Load())
	}
}

func TestNativeCollectorSamplesWorkingDirectoryFilesystem(t *testing.T) {
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	value, err := NewCollector(time.Now()).Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if value.DiskPath != directory {
		t.Fatalf("sampled %q instead of working directory %q", value.DiskPath, directory)
	}
}
