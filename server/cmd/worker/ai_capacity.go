package main

import (
	airun "admin/server/internal/module/ai/run"
	"context"
)

// Keep the shared two-second observation alive even when no new Run arrives.
// The existing Worker supervisor owns cancellation and propagates failures.
type aiCapacityRunner struct {
	gate     *airun.Gate
	policies *airun.PolicyReader
}

func (r aiCapacityRunner) Run(ctx context.Context) error { return r.gate.Run(ctx, r.policies) }
