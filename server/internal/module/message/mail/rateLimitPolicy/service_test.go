package ratelimitpolicy

import (
	"context"
	"errors"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
)

type serviceRepositoryStub struct {
	catalog      Catalog
	catalogs     []Catalog
	result       cachegeneration.MutationResult
	err          error
	expected     int64
	updatedInput Input
}

func (r *serviceRepositoryStub) List(context.Context, int64) (Catalog, error) {
	return r.catalog, r.err
}

func (r *serviceRepositoryStub) ListAll(context.Context) ([]Catalog, error) {
	return r.catalogs, r.err
}

func (r *serviceRepositoryStub) Update(_ context.Context, _ int64, input Input, expected int64, _ time.Time) (Catalog, cachegeneration.MutationResult, error) {
	r.expected = expected
	r.updatedInput = input
	return r.catalog, r.result, r.err
}

type runtimeCoordinatorStub struct {
	generation int64
	calls      int
	err        error
}

func (r *runtimeCoordinatorStub) Mutate(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	r.calls++
	if r.err != nil {
		return r.err
	}
	if r.generation == 0 {
		r.generation = 1
	}
	result, err := change(ctx, r.generation)
	if err == nil && result.Changed {
		r.generation = result.Generation
	}
	return err
}

func TestServiceUpdateUsesMailGenerationAndLastWriteWins(t *testing.T) {
	input := Input{Key: "business_email_minute", Limit: 2, WindowSeconds: 120}
	repository := &serviceRepositoryStub{
		catalog: Catalog{PlatformID: 1, Policies: fixedPoliciesWithTimestamp(time.Now().UTC())},
		result:  cachegeneration.MutationResult{Changed: true, Generation: 8, OutboxID: 12},
	}
	runtime := &runtimeCoordinatorStub{generation: 7}
	service := NewService(repository)
	service.SetRuntimeCoordinator(runtime)
	catalog, err := service.Update(context.Background(), 1, input)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.PlatformID != 1 || repository.expected != 7 || repository.updatedInput != input || runtime.calls != 1 {
		t.Fatalf("catalog=%+v expected=%d input=%+v calls=%d", catalog, repository.expected, repository.updatedInput, runtime.calls)
	}
}

func TestServiceUpdateFailsClosedWithoutRuntimeCoordinator(t *testing.T) {
	service := NewService(&serviceRepositoryStub{})
	_, err := service.Update(context.Background(), 1, Input{Key: "business_email_minute", Limit: 2, WindowSeconds: 120})
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) || applicationError.Code != apperror.CodeDependencyUnavailable {
		t.Fatalf("Update error=%v, want dependency unavailable", err)
	}
}
