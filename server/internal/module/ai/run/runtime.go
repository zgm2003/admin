package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	projectredis "admin/server/internal/redis"
	cachefill "admin/server/internal/shared/cacheFill"
	cache "admin/server/internal/shared/configCache"
)

var runtimeScope = cache.Scope{Namespace: "ai.runtime", ScopeKey: "global"}
var ErrRuntimeUnavailable = errors.New("AI runtime configuration unavailable")
var errRuntimeChanged = errors.New("AI runtime generation changed")

type runtimeSnapshotRepository interface {
	LoadRuntime(context.Context, int64, time.Time) (RuntimeSnapshot, *time.Time, error)
}
type generationRepository interface {
	Current(context.Context, cache.Scope) (int64, error)
	MarkPublishedIfUnclaimed(context.Context, int64, time.Time) (bool, error)
}
type Runtime struct {
	repository  runtimeSnapshotRepository
	redis       *projectredis.Client
	generations generationRepository
	states      *cache.Store
	scope       cache.Scope
}

func NewRuntime(repository runtimeSnapshotRepository, redis *projectredis.Client, generations generationRepository, states *cache.Store) *Runtime {
	return &Runtime{repository: repository, redis: redis, generations: generations, states: states, scope: runtimeScope}
}

type runtimeEnvelope struct {
	SchemaVersion   int             `json:"schemaVersion"`
	Generation      int64           `json:"generation"`
	AgentID         int64           `json:"agentId"`
	NextEffectiveAt *time.Time      `json:"nextEffectiveAt"`
	Snapshot        RuntimeSnapshot `json:"snapshot"`
}

func (r *Runtime) configured() error {
	if r == nil || r.repository == nil || r.redis == nil || r.generations == nil || r.states == nil {
		return ErrRuntimeUnavailable
	}
	return nil
}
func (r *Runtime) Get(ctx context.Context, id int64, at time.Time) (RuntimeSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeSnapshot{}, err
	}
	if err := r.configured(); err != nil {
		return RuntimeSnapshot{}, err
	}
	if id < 1 || at.IsZero() {
		return RuntimeSnapshot{}, ErrInvalidSnapshot
	}
	admission, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	for changes := 0; changes < 2; {
		if err := admission.Err(); err != nil {
			return RuntimeSnapshot{}, err
		}
		state, found, err := r.states.Read(admission, r.scope)
		if err != nil {
			return RuntimeSnapshot{}, fmt.Errorf("%w: %w", ErrRuntimeUnavailable, err)
		}
		if found && state.State != cache.StateReady {
			return RuntimeSnapshot{}, cache.ErrUpdating
		}
		if found {
			snap, hit, err := r.cached(admission, state.Generation, id, at)
			if err != nil {
				return RuntimeSnapshot{}, err
			}
			if hit {
				confirmed, ok, err := r.states.Read(admission, r.scope)
				if err != nil {
					return RuntimeSnapshot{}, err
				}
				if ok && confirmed.State == cache.StateReady && confirmed.Generation == state.Generation {
					snap.AcceptedAt = at.UTC()
					return NewRuntimeSnapshot(snap)
				}
				changes++
				continue
			}
		}
		variant := fmt.Sprintf("agent:%d", id)
		target := "state"
		if found {
			target, err = cache.FillKey(r.scope, state.Generation, variant)
			if err != nil {
				return RuntimeSnapshot{}, err
			}
		}
		lease, err := cachefill.Try(admission, r.redis.UniversalClient(), cache.StateKey(r.scope), target)
		if err != nil {
			return RuntimeSnapshot{}, err
		}
		if lease == nil {
			if err = cachefill.Wait(admission); err != nil {
				return RuntimeSnapshot{}, err
			}
			continue
		}
		snap, err := r.fill(ctx, lease, state.Generation, found, id, at)
		releaseErr := lease.Release(ctx)
		if releaseErr != nil {
			return RuntimeSnapshot{}, releaseErr
		}
		if errors.Is(err, errRuntimeChanged) {
			changes++
			continue
		}
		return snap, err
	}
	return RuntimeSnapshot{}, ErrRuntimeUnavailable
}
func (r *Runtime) cached(ctx context.Context, generation, id int64, at time.Time) (RuntimeSnapshot, bool, error) {
	key, err := cache.SnapshotKey(r.scope, generation, fmt.Sprintf("agent:%d", id))
	if err != nil {
		return RuntimeSnapshot{}, false, err
	}
	raw, found, err := r.redis.GetString(ctx, key)
	if err != nil || !found {
		return RuntimeSnapshot{}, false, err
	}
	var e runtimeEnvelope
	dec := json.NewDecoder(bytes.NewBufferString(raw))
	dec.DisallowUnknownFields()
	if len(raw) > 256*1024 || dec.Decode(&e) != nil || dec.Decode(&struct{}{}) != io.EOF || e.SchemaVersion != 1 || e.Generation != generation || e.AgentID != id {
		return RuntimeSnapshot{}, false, ErrRuntimeUnavailable
	}
	canonical, err := json.Marshal(e)
	var compact bytes.Buffer
	if err != nil || json.Compact(&compact, []byte(raw)) != nil || !bytes.Equal(canonical, compact.Bytes()) {
		return RuntimeSnapshot{}, false, ErrRuntimeUnavailable
	}
	if e.NextEffectiveAt != nil && !at.Before(*e.NextEffectiveAt) {
		return RuntimeSnapshot{}, false, nil
	}
	snap, err := NewRuntimeSnapshot(e.Snapshot)
	if err != nil {
		return RuntimeSnapshot{}, false, err
	}
	return snap, true, nil
}
func (r *Runtime) fill(ctx context.Context, lease *cachefill.Lease, generation int64, found bool, id int64, at time.Time) (RuntimeSnapshot, error) {
	work, cancel := lease.WorkContext(ctx)
	defer cancel()
	if !found {
		var err error
		generation, err = r.generations.Current(work, r.scope)
		if err != nil {
			return RuntimeSnapshot{}, err
		}
		published, err := r.states.Reconcile(work, r.scope, generation)
		if err != nil {
			return RuntimeSnapshot{}, err
		}
		if published == cache.PublishSkippedInvalidating || published == cache.PublishAlreadyNewer {
			return RuntimeSnapshot{}, errRuntimeChanged
		}
	}
	if cached, hit, err := r.cached(work, generation, id, at); err != nil {
		return RuntimeSnapshot{}, err
	} else if hit {
		if err := r.confirm(work, generation); err != nil {
			return RuntimeSnapshot{}, err
		}
		cached.AcceptedAt = at.UTC()
		return NewRuntimeSnapshot(cached)
	}
	snap, next, err := r.repository.LoadRuntime(work, id, at)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	snap.AcceptedAt = at.UTC()
	snap, err = NewRuntimeSnapshot(snap)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if next != nil && !next.After(at) {
		return RuntimeSnapshot{}, ErrInvalidSnapshot
	}
	if err := r.confirm(work, generation); err != nil {
		return RuntimeSnapshot{}, err
	}
	envelope := runtimeEnvelope{SchemaVersion: 1, Generation: generation, AgentID: id, NextEffectiveAt: next, Snapshot: snap}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if len(raw) > 256*1024 {
		return RuntimeSnapshot{}, ErrInvalidSnapshot
	}
	ttl := 5 * time.Minute
	if next != nil && time.Until(*next) < ttl {
		ttl = time.Until(*next)
	}
	if ttl <= 0 {
		return RuntimeSnapshot{}, errRuntimeChanged
	}
	key, err := cache.SnapshotKey(r.scope, generation, fmt.Sprintf("agent:%d", id))
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if err = r.redis.SetString(work, key, string(raw), ttl); err != nil {
		return RuntimeSnapshot{}, err
	}
	if err = r.confirm(work, generation); err != nil {
		return RuntimeSnapshot{}, err
	}
	return snap, nil
}
func (r *Runtime) confirm(ctx context.Context, generation int64) error {
	s, found, err := r.states.Read(ctx, r.scope)
	if err != nil {
		return err
	}
	if !found || s.State != cache.StateReady || s.Generation != generation {
		return errRuntimeChanged
	}
	return nil
}

// Mutate coordinates each AI runtime change with the existing tokenized
// generation protocol. The repository callback must advance the same scope
// and write its durable outbox event in the business transaction.
func (r *Runtime) Mutate(ctx context.Context, apply func(context.Context, int64) (cache.MutationResult, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.configured(); err != nil {
		return err
	}
	if apply == nil {
		return ErrRuntimeUnavailable
	}
	base, err := r.generations.Current(ctx, r.scope)
	if err != nil {
		return err
	}
	lease, err := r.states.Acquire(ctx, r.scope, base)
	if errors.Is(err, cache.ErrStateMissing) {
		if _, err = r.states.Reconcile(ctx, r.scope, base); err != nil {
			return err
		}
		lease, err = r.states.Acquire(ctx, r.scope, base)
	}
	if err != nil {
		return err
	}
	work, cancel := context.WithCancelCause(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(cache.MutationRenewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-ticker.C:
				if e := lease.Renew(work); e != nil {
					cancel(e)
					return
				}
			}
		}
	}()
	result, applyErr := apply(work, base)
	cause := context.Cause(work)
	cancel(nil)
	<-done
	cleanup, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cleanupCancel()
	if applyErr != nil {
		current, e := r.generations.Current(cleanup, r.scope)
		if e != nil {
			return errors.Join(applyErr, e)
		}
		if current > base {
			return errors.Join(applyErr, lease.Commit(cleanup, current))
		}
		return errors.Join(applyErr, lease.Rollback(cleanup))
	}
	if !result.Changed {
		return errors.Join(cause, lease.Rollback(cleanup))
	}
	if result.Generation <= base || result.OutboxID < 1 {
		return ErrRuntimeUnavailable
	}
	if err = lease.Commit(cleanup, result.Generation); err != nil {
		return fmt.Errorf("%w: AI configuration committed, cache publication pending: %w", ErrRuntimeUnavailable, err)
	}
	if _, err = r.generations.MarkPublishedIfUnclaimed(cleanup, result.OutboxID, time.Now().UTC()); err != nil {
		return fmt.Errorf("%w: AI configuration committed, outbox acknowledgement pending: %w", ErrRuntimeUnavailable, err)
	}
	return cause
}
