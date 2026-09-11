package email

import (
	"context"
	"errors"
	"fmt"
	"time"

	authstate "admin/server/internal/module/auth/state"
)

type AuthorityCoordinator struct {
	states      *authstate.Store
	invalidator *authstate.Invalidator
}

func NewAuthorityCoordinator(states *authstate.Store, invalidator *authstate.Invalidator) *AuthorityCoordinator {
	return &AuthorityCoordinator{states: states, invalidator: invalidator}
}

func (c *AuthorityCoordinator) Mutate(ctx context.Context, current Current, mutation func(context.Context) error) error {
	if c == nil || c.states == nil || c.invalidator == nil || current.UserID < 1 || !current.IsEnabled || current.Deleted {
		return fmt.Errorf("email authority mutation dependencies are unavailable")
	}
	fact, err := c.ensureReady(ctx, current)
	if err != nil {
		return err
	}
	lease, err := c.invalidator.Acquire(ctx, authstate.MutationFacts{Users: []authstate.UserFact{fact}})
	if err != nil {
		return err
	}
	mutationCtx, stopRenewal := lease.StartRenewal(ctx)
	mutationErr := mutation(mutationCtx)
	renewalErr := context.Cause(mutationCtx)
	stopRenewal()
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if mutationErr != nil {
		return errors.Join(mutationErr, renewalErr, lease.Rollback(cleanupCtx))
	}
	nextGeneration, err := authstate.NewGeneration()
	if err != nil {
		return errors.Join(renewalErr, err)
	}
	if err := lease.Commit(cleanupCtx, authstate.MutationFacts{Users: []authstate.UserFact{{UserID: current.UserID, Generation: nextGeneration, IsEnabled: current.IsEnabled, Deleted: current.Deleted}}}); err != nil {
		return errors.Join(renewalErr, err)
	}
	return nil
}

func (c *AuthorityCoordinator) ensureReady(ctx context.Context, current Current) (authstate.UserFact, error) {
	state, found, readErr := c.states.ReadUser(ctx, current.UserID)
	if readErr == nil && found {
		if state.State == authstate.StateInvalidating {
			return authstate.UserFact{}, authstate.ErrUpdating
		}
		if state.IsEnabled != current.IsEnabled || state.Deleted != current.Deleted {
			return authstate.UserFact{}, authstate.ErrGenerationChanged
		}
		return state.Fact(), nil
	}
	generation, err := authstate.NewGeneration()
	if err != nil {
		return authstate.UserFact{}, err
	}
	fact := authstate.UserFact{UserID: current.UserID, Generation: generation, IsEnabled: current.IsEnabled, Deleted: current.Deleted}
	installed, _, installErr := c.states.InstallUserReadyIfMissing(ctx, fact)
	if installErr != nil {
		return authstate.UserFact{}, errors.Join(readErr, installErr)
	}
	if installed.State == authstate.StateInvalidating {
		return authstate.UserFact{}, authstate.ErrUpdating
	}
	if installed.IsEnabled != current.IsEnabled || installed.Deleted != current.Deleted {
		return authstate.UserFact{}, authstate.ErrGenerationChanged
	}
	return installed.Fact(), nil
}
