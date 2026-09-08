package authplatform

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
)

type atomicPolicyRepository struct {
	findCalls atomic.Int64
	platform  Platform
	delay     time.Duration
}

func (r *atomicPolicyRepository) FindPolicy(ctx context.Context, _ string) (Platform, error) {
	r.findCalls.Add(1)
	timer := time.NewTimer(r.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return Platform{}, ctx.Err()
	case <-timer.C:
		return r.platform, nil
	}
}

func TestCurrentPolicyTwoInstancesShareOneMissingKeyRecovery(t *testing.T) {
	firstClient := openPolicyRedis(t)
	secondClient := openPolicyRedis(t)
	firstStore := NewPolicyStore(firstClient)
	secondStore := NewPolicyStore(secondClient)
	ctx := context.Background()
	code := fmt.Sprintf("concurrent_%d", time.Now().UnixNano())
	keys := []string{PolicyKey(code), firstStore.loadLockKey(code)}
	if err := firstClient.DeleteMany(ctx, keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = firstClient.DeleteMany(context.Background(), keys) })

	platform := validReadPlatform()
	platform.Code = code
	repository := &atomicPolicyRepository{platform: platform, delay: 300 * time.Millisecond}
	services := []*Service{
		{policyReader: repository, policies: firstStore},
		{policyReader: repository, policies: secondStore},
	}
	start := make(chan struct{})
	errorsFound := make(chan error, 64)
	var wait sync.WaitGroup
	for index := 0; index < 64; index++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			<-start
			policy, err := services[worker%len(services)].CurrentPolicy(ctx, code)
			if err != nil {
				errorsFound <- err
				return
			}
			if policy.PolicyVersion != 1 {
				errorsFound <- fmt.Errorf("policy version = %d", policy.PolicyVersion)
			}
		}(index)
	}
	close(start)
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatalf("CurrentPolicy: %v", err)
	}
	if repository.findCalls.Load() != 1 {
		t.Fatalf("PostgreSQL policy reads = %d, want 1", repository.findCalls.Load())
	}

	for index := 0; index < 100; index++ {
		if _, err := services[index%len(services)].CurrentPolicy(ctx, code); err != nil {
			t.Fatal(err)
		}
	}
	if repository.findCalls.Load() != 1 {
		t.Fatalf("ready policy caused PostgreSQL reads = %d", repository.findCalls.Load())
	}
}

func TestCurrentPolicyCanceledLeaderDoesNotCancelSharedRecovery(t *testing.T) {
	redisClient := openPolicyRedis(t)
	store := NewPolicyStore(redisClient)
	ctx := context.Background()
	code := fmt.Sprintf("canceled_leader_%d", time.Now().UnixNano())
	keys := []string{PolicyKey(code), store.loadLockKey(code)}
	if err := redisClient.DeleteMany(ctx, keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = redisClient.DeleteMany(context.Background(), keys) })

	platform := validReadPlatform()
	platform.Code = code
	repository := &atomicPolicyRepository{platform: platform, delay: 300 * time.Millisecond}
	service := &Service{policyReader: repository, policies: store}
	leaderContext, cancelLeader := context.WithCancel(ctx)
	leaderResult := make(chan error, 1)
	go func() {
		_, err := service.CurrentPolicy(leaderContext, code)
		leaderResult <- err
	}()
	deadline := time.Now().Add(time.Second)
	for repository.findCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if repository.findCalls.Load() == 0 {
		t.Fatal("leader did not start PostgreSQL policy recovery")
	}

	waiterResult := make(chan error, 1)
	go func() {
		_, err := service.CurrentPolicy(ctx, code)
		waiterResult <- err
	}()
	time.Sleep(25 * time.Millisecond)
	cancelLeader()

	if err := <-leaderResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled leader error = %v, want context canceled", err)
	}
	if err := <-waiterResult; err != nil {
		t.Fatalf("healthy waiter failed after leader cancellation: %v", err)
	}
	if repository.findCalls.Load() != 1 {
		t.Fatalf("PostgreSQL policy reads = %d, want 1", repository.findCalls.Load())
	}
}

func TestCurrentPolicyClosedClientFailsWithoutPostgresFallback(t *testing.T) {
	firstClient := openPolicyRedis(t)
	secondClient := openPolicyRedis(t)
	ctx := context.Background()
	secondStore := NewPolicyStore(secondClient)
	if _, _, err := secondStore.installReadyIfMissing(ctx, validReadPolicy()); err != nil {
		t.Fatal(err)
	}
	repository := &atomicPolicyRepository{platform: validReadPlatform()}
	closedService := &Service{policyReader: repository, policies: NewPolicyStore(firstClient)}
	healthyService := &Service{policyReader: repository, policies: secondStore}
	if err := firstClient.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := closedService.CurrentPolicy(ctx, "admin"); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("closed-client CurrentPolicy error = %v", err)
	}
	if repository.findCalls.Load() != 0 {
		t.Fatalf("closed-client fallback PostgreSQL reads = %d", repository.findCalls.Load())
	}
	if _, err := healthyService.CurrentPolicy(ctx, "admin"); err != nil {
		t.Fatalf("second client could not read ready policy: %v", err)
	}
}

func appErrorCode(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}
