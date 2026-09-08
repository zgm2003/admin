package permissionstate

import (
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cachefill"
	"context"
	"errors"
	"fmt"
)

const menuStatePrefix = "authz:menu-state:v1:"

func MenuStateKey(platformID int64) string { return fmt.Sprintf("%s%d", menuStatePrefix, platformID) }

// MenuStore owns platform catalog revisions, independent from user grants and
// authentication policy. The private token/CAS mechanics are shared with Store.
type MenuStore struct{ state *Store }

func (m *MenuStore) Read(ctx context.Context, platformID int64) (State, bool, error) {
	return m.state.Read(ctx, platformID)
}

type MenuLease struct {
	lease      *MutationLease
	platformID int64
}

func NewMenuStore(r *projectredis.Client) *MenuStore {
	return &MenuStore{state: &Store{redis: r, keyPrefix: menuStatePrefix}}
}

func (m *MenuStore) Current(ctx context.Context, platformID int64, load func(context.Context, int64) (int64, error)) (versionResult int64, resultErr error) {
	if m == nil || m.state == nil || m.state.redis == nil || platformID < 1 {
		return 0, fmt.Errorf("menu version dependencies missing")
	}
	var fill *cachefill.Lease
	defer func() { resultErr = errors.Join(resultErr, fill.Release(ctx)) }()
	for attempt := 0; attempt < 25; attempt++ {
		state, found, err := m.state.Read(ctx, platformID)
		if err != nil {
			return 0, err
		}
		if found {
			if state.State != StateReady {
				return 0, ErrUpdating
			}
			return state.Version, nil
		}
		if fill == nil {
			fill, err = cachefill.Try(ctx, m.state.redis.UniversalClient(), "menu-version", fmt.Sprint(platformID))
			if err != nil {
				return 0, err
			}
			if fill == nil {
				if err := cachefill.Wait(ctx); err != nil {
					return 0, err
				}
				continue
			}
			continue
		}
		work, cancel := fill.WorkContext(ctx)
		version, err := load(work, platformID)
		if err != nil {
			cancel()
			return 0, err
		}
		state, _, err = m.state.InstallReadyIfMissing(work, Version{UserID: platformID, Version: version})
		cancel()
		if err != nil {
			return 0, err
		}
		if state.State != StateReady {
			return 0, ErrUpdating
		}
		return state.Version, nil
	}
	return 0, fmt.Errorf("menu version rebuild busy")
}

func (m *MenuStore) Acquire(ctx context.Context, platformID, version int64) (*MenuLease, error) {
	if m == nil || m.state == nil || m.state.redis == nil {
		return nil, fmt.Errorf("menu version dependencies missing")
	}
	lease, err := NewInvalidator(m.state).Acquire(ctx, []Version{{UserID: platformID, Version: version}})
	if err != nil {
		return nil, err
	}
	return &MenuLease{lease: lease, platformID: platformID}, nil
}
func (l *MenuLease) StartRenewal(ctx context.Context) (context.Context, func()) {
	return l.lease.StartRenewal(ctx)
}
func (l *MenuLease) Rollback(ctx context.Context) error { return l.lease.Rollback(ctx) }
func (l *MenuLease) Commit(ctx context.Context, version int64) error {
	return l.lease.Commit(ctx, map[int64]int64{l.platformID: version})
}
