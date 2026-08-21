package accessstate

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	defaultLeaseTTL   = 30 * time.Second
	defaultRenewEvery = 10 * time.Second
)

type Invalidator struct {
	store      *Store
	leaseTTL   time.Duration
	renewEvery time.Duration
}

type accessMutationEntry struct {
	userID       int64
	priorVersion int64
	token        string
}

type MutationLease struct {
	store      *Store
	entries    []accessMutationEntry
	leaseTTL   time.Duration
	renewEvery time.Duration
	mutex      sync.Mutex
	finalized  bool
}

func NewInvalidator(store *Store) *Invalidator {
	return &Invalidator{store: store, leaseTTL: defaultLeaseTTL, renewEvery: defaultRenewEvery}
}

func (i *Invalidator) Acquire(ctx context.Context, candidates []Version) (*MutationLease, error) {
	normalized, err := normalizeVersions(candidates)
	if err != nil {
		return nil, err
	}
	entries := make([]accessMutationEntry, 0, len(normalized))
	for _, candidate := range normalized {
		state, _, installErr := i.store.InstallReadyIfMissing(ctx, candidate)
		if installErr != nil {
			return nil, errors.Join(installErr, restoreEntries(ctx, i.store, entries))
		}
		if state.State == StateInvalidating {
			return nil, errors.Join(ErrUpdating, restoreEntries(ctx, i.store, entries))
		}
		if state.Version != candidate.Version {
			return nil, errors.Join(ErrVersionChanged, restoreEntries(ctx, i.store, entries))
		}
		token, tokenErr := newMutationToken()
		if tokenErr != nil {
			return nil, errors.Join(tokenErr, restoreEntries(ctx, i.store, entries))
		}
		invalidating, encodeErr := encodeState(State{
			SchemaVersion: SchemaVersion, State: StateInvalidating, MutationToken: token, BaseVersion: candidate.Version,
		})
		if encodeErr != nil {
			return nil, errors.Join(encodeErr, restoreEntries(ctx, i.store, entries))
		}
		result, beginErr := i.store.redis.EvalString(ctx, beginInvalidationScript, []string{StateKey(candidate.UserID)}, candidate.Version, invalidating, i.leaseTTL.Milliseconds())
		if beginErr != nil {
			return nil, errors.Join(beginErr, restoreEntries(ctx, i.store, entries))
		}
		if result != "acquired" {
			restoreErr := restoreEntries(ctx, i.store, entries)
			if result == "updating" {
				return nil, errors.Join(ErrUpdating, restoreErr)
			}
			return nil, errors.Join(ErrVersionChanged, restoreErr)
		}
		entries = append(entries, accessMutationEntry{userID: candidate.UserID, priorVersion: candidate.Version, token: token})
	}
	return &MutationLease{store: i.store, entries: entries, leaseTTL: i.leaseTTL, renewEvery: i.renewEvery}, nil
}

func newMutationToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate access state mutation token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func (l *MutationLease) UserIDs() []int64 {
	result := make([]int64, len(l.entries))
	for index, entry := range l.entries {
		result[index] = entry.userID
	}
	return result
}

func (l *MutationLease) StartRenewal(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(l.renewEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := l.renew(ctx); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	return ctx, func() {
		cancel(nil)
		<-done
	}
}

func (l *MutationLease) Commit(ctx context.Context, versions map[int64]int64) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return ErrMutationTokenMismatch
	}
	if len(versions) != len(l.entries) {
		return fmt.Errorf("access state commit key set changed")
	}
	payloads := make(map[int64]string, len(l.entries))
	for _, entry := range l.entries {
		version, ok := versions[entry.userID]
		if !ok || version <= entry.priorVersion {
			return fmt.Errorf("access state commit version is invalid")
		}
		payload, err := encodeState(State{SchemaVersion: SchemaVersion, State: StateReady, Version: version})
		if err != nil {
			return err
		}
		payloads[entry.userID] = payload
	}
	if err := publishEntries(ctx, l.store, l.entries, payloads); err != nil {
		return err
	}
	l.finalized = true
	return nil
}

func (l *MutationLease) Rollback(ctx context.Context) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return ErrMutationTokenMismatch
	}
	err := restoreEntries(ctx, l.store, l.entries)
	if err == nil {
		l.finalized = true
	}
	return err
}

func (l *MutationLease) renew(ctx context.Context) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized || len(l.entries) == 0 {
		return nil
	}
	keys := make([]string, len(l.entries))
	args := make([]any, 0, len(l.entries)+1)
	args = append(args, l.leaseTTL.Milliseconds())
	for index, entry := range l.entries {
		keys[index] = StateKey(entry.userID)
		args = append(args, entry.token)
	}
	result, err := l.store.redis.EvalString(ctx, renewStatesScript, keys, args...)
	if err != nil {
		return err
	}
	if result != "renewed" {
		return ErrMutationTokenMismatch
	}
	return nil
}

func restoreEntries(ctx context.Context, store *Store, entries []accessMutationEntry) error {
	payloads := make(map[int64]string, len(entries))
	for _, entry := range entries {
		payload, err := encodeState(State{SchemaVersion: SchemaVersion, State: StateReady, Version: entry.priorVersion})
		if err != nil {
			return err
		}
		payloads[entry.userID] = payload
	}
	return publishEntries(ctx, store, entries, payloads)
}

func publishEntries(ctx context.Context, store *Store, entries []accessMutationEntry, payloads map[int64]string) error {
	if len(entries) == 0 {
		return nil
	}
	keys := make([]string, len(entries))
	args := make([]any, 0, len(entries)*2)
	for index, entry := range entries {
		keys[index] = StateKey(entry.userID)
		args = append(args, entry.token, payloads[entry.userID])
	}
	result, err := store.redis.EvalString(ctx, publishStatesScript, keys, args...)
	if err != nil {
		return err
	}
	if result != "published" {
		return ErrMutationTokenMismatch
	}
	return nil
}

const beginInvalidationScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'changed' end
local decoded = cjson.decode(current)
if decoded.state == 'invalidating' then return 'updating' end
if decoded.state ~= 'ready' or decoded.version ~= tonumber(ARGV[1]) then return 'changed' end
redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
return 'acquired'
`

const publishStatesScript = `
for index, key in ipairs(KEYS) do
  local current = redis.call('GET', key)
  if not current then return 'missing' end
  local decoded = cjson.decode(current)
  local tokenIndex = (index - 1) * 2 + 1
  if decoded.state ~= 'invalidating' or decoded.mutationToken ~= ARGV[tokenIndex] then
    return 'token-mismatch'
  end
end
for index, key in ipairs(KEYS) do
  local payloadIndex = (index - 1) * 2 + 2
  redis.call('SET', key, ARGV[payloadIndex])
end
return 'published'
`

const renewStatesScript = `
for index, key in ipairs(KEYS) do
  local current = redis.call('GET', key)
  if not current then return 'missing' end
  local decoded = cjson.decode(current)
  if decoded.state ~= 'invalidating' or decoded.mutationToken ~= ARGV[index + 1] then
    return 'token-mismatch'
  end
end
for _, key in ipairs(KEYS) do redis.call('PEXPIRE', key, ARGV[1]) end
return 'renewed'
`
