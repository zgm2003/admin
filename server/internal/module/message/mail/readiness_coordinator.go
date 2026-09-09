package mail

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ReadinessCoordinator struct {
	store VerifyCodeReadinessStore
}

func NewReadinessCoordinator(store VerifyCodeReadinessStore) *ReadinessCoordinator {
	return &ReadinessCoordinator{store: store}
}

func (c *ReadinessCoordinator) Mutate(ctx context.Context, change func(context.Context) error) error {
	mutations, err := c.begin(ctx)
	if err != nil {
		return err
	}
	if err := change(ctx); err != nil {
		if rollbackErr := c.rollback(ctx, mutations); rollbackErr != nil {
			return dependency(errors.Join(err, rollbackErr))
		}
		return err
	}
	return c.publish(ctx, mutations)
}

func (c *ReadinessCoordinator) begin(ctx context.Context) ([]VerifyCodeReadinessMutation, error) {
	if c == nil || c.store == nil {
		return nil, dependency(fmt.Errorf("mail verification readiness store unavailable"))
	}
	mutations := make([]VerifyCodeReadinessMutation, 0, 2)
	for _, scene := range []string{SceneLogin, SceneForget} {
		if err := ctx.Err(); err != nil {
			return nil, dependency(errors.Join(err, c.rollback(ctx, mutations)))
		}
		mutation, err := c.store.BeginMutation(ctx, scene)
		if err != nil {
			return nil, dependency(errors.Join(err, c.rollback(ctx, mutations)))
		}
		mutations = append(mutations, mutation)
	}
	return mutations, nil
}

func (c *ReadinessCoordinator) publish(ctx context.Context, mutations []VerifyCodeReadinessMutation) error {
	publishContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), verifyCodeReadinessLoadTimeout)
	defer cancel()
	var result error
	for _, mutation := range mutations {
		result = errors.Join(result, c.store.PublishMutation(publishContext, mutation))
	}
	if result != nil {
		return dependency(result)
	}
	return nil
}

func (c *ReadinessCoordinator) rollback(ctx context.Context, mutations []VerifyCodeReadinessMutation) error {
	rollbackContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	var result error
	for index := len(mutations) - 1; index >= 0; index-- {
		result = errors.Join(result, c.store.RollbackMutation(rollbackContext, mutations[index]))
	}
	return result
}
