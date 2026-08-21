package authstate

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	defaultLeaseTTL      = 30 * time.Second
	defaultRenewInterval = 10 * time.Second
)

type Invalidator struct {
	store         *Store
	leaseTTL      time.Duration
	renewInterval time.Duration
}

type mutationEntry struct {
	key          string
	priorPayload string
	token        string
	user         *UserFact
	sessions     *SessionsFact
}

type MutationLease struct {
	store         *Store
	entries       []mutationEntry
	leaseTTL      time.Duration
	renewInterval time.Duration
	mutex         sync.Mutex
	finalized     bool
}

func NewInvalidator(store *Store) *Invalidator {
	return &Invalidator{store: store, leaseTTL: defaultLeaseTTL, renewInterval: defaultRenewInterval}
}

func (i *Invalidator) Acquire(ctx context.Context, candidates MutationFacts) (*MutationLease, error) {
	normalized, err := normalizeMutationFacts(candidates)
	if err != nil {
		return nil, err
	}
	entries, err := makeMutationEntries(normalized)
	if err != nil {
		return nil, err
	}
	acquired := make([]mutationEntry, 0, len(entries))
	for _, entry := range entries {
		invalidatingPayload, encodeErr := entry.invalidatingPayload()
		if encodeErr != nil {
			_ = i.restore(ctx, acquired)
			return nil, encodeErr
		}
		result, evalErr := i.store.redis.EvalString(ctx, acquireStateScript, []string{entry.key}, entry.priorPayload, invalidatingPayload, i.leaseTTL.Milliseconds())
		if evalErr != nil {
			return nil, errors.Join(evalErr, i.restore(ctx, acquired))
		}
		if result != "acquired" {
			restoreErr := i.restore(ctx, acquired)
			if result == "updating" {
				return nil, errors.Join(ErrUpdating, restoreErr)
			}
			return nil, errors.Join(ErrGenerationChanged, restoreErr)
		}
		acquired = append(acquired, entry)
	}
	return &MutationLease{store: i.store, entries: acquired, leaseTTL: i.leaseTTL, renewInterval: i.renewInterval}, nil
}

func (i *Invalidator) restore(ctx context.Context, entries []mutationEntry) error {
	if len(entries) == 0 {
		return nil
	}
	return transitionEntries(ctx, i.store, entries, func(entry mutationEntry) (string, error) {
		return entry.priorPayload, nil
	})
}

func (l *MutationLease) StartRenewal(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(l.renewInterval)
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

func (l *MutationLease) Commit(ctx context.Context, next MutationFacts) error {
	normalized, err := normalizeMutationFacts(next)
	if err != nil {
		return err
	}
	nextEntries, err := makeMutationEntries(normalized)
	if err != nil {
		return err
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return ErrMutationTokenMismatch
	}
	if len(nextEntries) != len(l.entries) {
		return fmt.Errorf("authentication state commit key set changed")
	}
	nextByKey := make(map[string]mutationEntry, len(nextEntries))
	for _, entry := range nextEntries {
		nextByKey[entry.key] = entry
	}
	err = transitionEntries(ctx, l.store, l.entries, func(entry mutationEntry) (string, error) {
		nextEntry, ok := nextByKey[entry.key]
		if !ok || nextEntry.generation() == entry.generation() {
			return "", fmt.Errorf("authentication state commit generation is not fresh")
		}
		return nextEntry.priorPayload, nil
	})
	if err == nil {
		l.finalized = true
	}
	return err
}

func (l *MutationLease) Rollback(ctx context.Context) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return ErrMutationTokenMismatch
	}
	err := transitionEntries(ctx, l.store, l.entries, func(entry mutationEntry) (string, error) {
		return entry.priorPayload, nil
	})
	if err == nil {
		l.finalized = true
	}
	return err
}

func (l *MutationLease) renew(ctx context.Context) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return nil
	}
	keys := make([]string, len(l.entries))
	args := make([]any, 0, len(l.entries)+1)
	args = append(args, l.leaseTTL.Milliseconds())
	for index, entry := range l.entries {
		keys[index] = entry.key
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

func makeMutationEntries(facts MutationFacts) ([]mutationEntry, error) {
	entries := make([]mutationEntry, 0, len(facts.Users)+len(facts.Sessions))
	for _, fact := range facts.Users {
		payload, err := encodeState(userStateFromFact(fact, StateReady, nil))
		if err != nil {
			return nil, err
		}
		copyFact := fact
		entries = append(entries, mutationEntry{key: UserStateKey(fact.UserID), priorPayload: payload, user: &copyFact})
	}
	for _, fact := range facts.Sessions {
		payload, err := encodeState(sessionsStateFromFact(fact, StateReady, nil))
		if err != nil {
			return nil, err
		}
		copyFact := fact
		entries = append(entries, mutationEntry{key: SessionsStateKey(fact.Platform, fact.UserID), priorPayload: payload, sessions: &copyFact})
	}
	sort.Slice(entries, func(left, right int) bool {
		leftUserID, leftPlatform := entries[left].identity()
		rightUserID, rightPlatform := entries[right].identity()
		if leftUserID != rightUserID {
			return leftUserID < rightUserID
		}
		return leftPlatform < rightPlatform
	})
	for index := range entries {
		token, err := NewGeneration()
		if err != nil {
			return nil, err
		}
		entries[index].token = token
	}
	return entries, nil
}

func (e mutationEntry) invalidatingPayload() (string, error) {
	if e.user != nil {
		return encodeState(userStateFromFact(*e.user, StateInvalidating, &e.token))
	}
	return encodeState(sessionsStateFromFact(*e.sessions, StateInvalidating, &e.token))
}

func (e mutationEntry) identity() (int64, string) {
	if e.user != nil {
		return e.user.UserID, ""
	}
	return e.sessions.UserID, e.sessions.Platform
}

func (e mutationEntry) generation() string {
	if e.user != nil {
		return e.user.Generation
	}
	return e.sessions.Generation
}

func transitionEntries(ctx context.Context, store *Store, entries []mutationEntry, payload func(mutationEntry) (string, error)) error {
	keys := make([]string, len(entries))
	args := make([]any, 0, len(entries)*2)
	for index, entry := range entries {
		keys[index] = entry.key
		value, err := payload(entry)
		if err != nil {
			return err
		}
		args = append(args, entry.token, value)
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

const acquireStateScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'changed' end
local decoded = cjson.decode(current)
if decoded.state == 'invalidating' then return 'updating' end
if current ~= ARGV[1] then return 'changed' end
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
