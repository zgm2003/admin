package cachegeneration

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	projectredis "admin/server/internal/redis"
)

const (
	// MutationLeaseTTL 是 invalidating state 的 Redis TTL；Renew 每次续到该值。
	MutationLeaseTTL = 30 * time.Second
	// MutationRenewInterval 供调用方的续租 goroutine 使用。
	MutationRenewInterval = 10 * time.Second

	leaseFinalizeTimeout = 3 * time.Second
)

var (
	ErrStateMissing          = errors.New("config cache state is missing")
	ErrUpdating              = errors.New("config cache state is updating")
	ErrGenerationChanged     = errors.New("config cache generation changed")
	ErrMutationTokenMismatch = errors.New("config cache mutation token mismatch")
)

// PublishResult 描述一次 Reconcile 对 Redis state 的实际影响。
type PublishResult int

const (
	PublishPublished PublishResult = iota + 1
	PublishRepaired
	PublishAlreadyNewer
	PublishSkippedInvalidating
)

func (r PublishResult) String() string {
	switch r {
	case PublishPublished:
		return "published"
	case PublishRepaired:
		return "repaired"
	case PublishAlreadyNewer:
		return "already-newer"
	case PublishSkippedInvalidating:
		return "invalidating"
	default:
		return "unknown"
	}
}

// Store 只访问 Redis；PostgreSQL 权威代际由 Repository 提供。
type Store struct{ client *projectredis.Client }

func NewStore(client *projectredis.Client) *Store {
	if client == nil {
		return nil
	}
	return &Store{client: client}
}

func (s *Store) Read(ctx context.Context, scope Scope) (State, bool, error) {
	if s == nil || s.client == nil {
		return State{}, false, fmt.Errorf("cache generation store is not configured")
	}
	if err := scope.Validate(); err != nil {
		return State{}, false, err
	}
	raw, found, err := s.client.GetString(ctx, StateKey(scope))
	if err != nil || !found {
		return State{}, found, err
	}
	state, err := decodeState(raw)
	if err != nil {
		return State{}, true, err
	}
	return state, true, nil
}

// Acquire 仅能把相同 generation 的 ready 原子改为带 TTL 的 invalidating。
func (s *Store) Acquire(ctx context.Context, scope Scope, expected int64) (*Lease, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("cache generation store is not configured")
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if expected < 1 {
		return nil, fmt.Errorf("expected cache generation is invalid")
	}
	token, err := newMutationToken()
	if err != nil {
		return nil, err
	}
	invalidating, err := encodeState(State{
		SchemaVersion: SchemaVersion, State: StateInvalidating, BaseGeneration: expected, MutationToken: token,
	})
	if err != nil {
		return nil, err
	}
	result, err := s.client.EvalString(ctx, acquireStateScript, []string{StateKey(scope)},
		expected, invalidating, MutationLeaseTTL.Milliseconds())
	if err != nil {
		return nil, err
	}
	switch result {
	case "acquired":
		return &Lease{store: s, scope: scope, token: token, baseGeneration: expected}, nil
	case "busy":
		return nil, ErrUpdating
	case "stale":
		return nil, ErrGenerationChanged
	case "missing":
		return nil, ErrStateMissing
	case "corrupt":
		return nil, ErrStateCorrupt
	default:
		return nil, fmt.Errorf("acquire config cache mutation lease returned %q", result)
	}
}

// Reconcile 以 PostgreSQL 权威代际单调修复 Redis state：不降级 ready，
// 不覆盖任何活跃 invalidating mutation。
func (s *Store) Reconcile(ctx context.Context, scope Scope, authoritative int64) (PublishResult, error) {
	if s == nil || s.client == nil {
		return 0, fmt.Errorf("cache generation store is not configured")
	}
	if err := scope.Validate(); err != nil {
		return 0, err
	}
	if authoritative < 1 {
		return 0, fmt.Errorf("authoritative cache generation is invalid")
	}
	payload, err := encodeState(State{SchemaVersion: SchemaVersion, State: StateReady, Generation: authoritative})
	if err != nil {
		return 0, err
	}
	result, err := s.client.EvalString(ctx, reconcileStateScript, []string{StateKey(scope)}, payload, authoritative)
	if err != nil {
		return 0, err
	}
	switch result {
	case "published":
		return PublishPublished, nil
	case "repaired":
		return PublishRepaired, nil
	case "already-newer":
		return PublishAlreadyNewer, nil
	case "invalidating":
		return PublishSkippedInvalidating, nil
	default:
		return 0, fmt.Errorf("reconcile config cache state returned %q", result)
	}
}

// Lease 是 token owner 持有的 mutation 租约；所有发布都必须携带同一 token。
type Lease struct {
	store          *Store
	scope          Scope
	token          string
	baseGeneration int64
	mutex          sync.Mutex
	finalized      bool
}

// Renew 仅在 token 相同且仍为 invalidating 时续租。
func (l *Lease) Renew(ctx context.Context) error {
	if err := l.validate(); err != nil {
		return err
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return ErrMutationTokenMismatch
	}
	result, err := l.store.client.EvalString(ctx, renewStateScript, []string{StateKey(l.scope)},
		l.token, MutationLeaseTTL.Milliseconds())
	if err != nil {
		return err
	}
	switch result {
	case "renewed":
		return nil
	case "lost":
		return ErrMutationTokenMismatch
	case "missing":
		return ErrStateMissing
	case "corrupt":
		return ErrStateCorrupt
	default:
		return fmt.Errorf("renew config cache mutation lease returned %q", result)
	}
}

// Commit 仅允许 token owner 发布更高的 ready；收尾使用有界脱离取消的 context。
func (l *Lease) Commit(ctx context.Context, generation int64) error {
	if err := l.validate(); err != nil {
		return err
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return ErrMutationTokenMismatch
	}
	if generation <= l.baseGeneration {
		return fmt.Errorf("commit generation must advance beyond the lease base generation")
	}
	payload, err := encodeState(State{SchemaVersion: SchemaVersion, State: StateReady, Generation: generation})
	if err != nil {
		return err
	}
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), leaseFinalizeTimeout)
	defer cancel()
	result, err := l.store.client.EvalString(finalizeCtx, commitStateScript, []string{StateKey(l.scope)}, l.token, payload)
	if err != nil {
		return err
	}
	switch result {
	case "committed":
		l.finalized = true
		return nil
	case "lost":
		return ErrMutationTokenMismatch
	case "missing":
		return ErrStateMissing
	case "corrupt":
		return ErrStateCorrupt
	default:
		return fmt.Errorf("commit config cache mutation lease returned %q", result)
	}
}

// Rollback 仅允许 token owner 恢复原 ready。
func (l *Lease) Rollback(ctx context.Context) error {
	if err := l.validate(); err != nil {
		return err
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return ErrMutationTokenMismatch
	}
	payload, err := encodeState(State{SchemaVersion: SchemaVersion, State: StateReady, Generation: l.baseGeneration})
	if err != nil {
		return err
	}
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), leaseFinalizeTimeout)
	defer cancel()
	result, err := l.store.client.EvalString(finalizeCtx, rollbackStateScript, []string{StateKey(l.scope)}, l.token, payload)
	if err != nil {
		return err
	}
	switch result {
	case "restored":
		l.finalized = true
		return nil
	case "lost":
		return ErrMutationTokenMismatch
	case "missing":
		return ErrStateMissing
	case "corrupt":
		return ErrStateCorrupt
	default:
		return fmt.Errorf("rollback config cache mutation lease returned %q", result)
	}
}

func (l *Lease) validate() error {
	if l == nil || l.store == nil || l.store.client == nil {
		return fmt.Errorf("cache generation mutation lease is not configured")
	}
	if l.token == "" {
		return fmt.Errorf("cache generation mutation token is missing")
	}
	return nil
}

func newMutationToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate cache generation mutation token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

const acquireStateScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
local ok, decoded = pcall(cjson.decode, current)
if not ok or type(decoded) ~= 'table' or tonumber(decoded.schemaVersion) ~= 1 then return 'corrupt' end
local count = 0
for _ in pairs(decoded) do count = count + 1 end
if decoded.state == 'invalidating' then
  if count ~= 4 then return 'corrupt' end
  return 'busy'
end
if decoded.state ~= 'ready' or count ~= 3 or decoded.generation == nil then return 'corrupt' end
if tonumber(decoded.generation) ~= tonumber(ARGV[1]) then return 'stale' end
redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
return 'acquired'
`

const renewStateScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
local ok, decoded = pcall(cjson.decode, current)
if not ok or type(decoded) ~= 'table' or tonumber(decoded.schemaVersion) ~= 1 then return 'corrupt' end
local count = 0
for _ in pairs(decoded) do count = count + 1 end
if decoded.state == 'ready' then
  if count ~= 3 or tonumber(decoded.generation) == nil then return 'corrupt' end
  return 'lost'
end
if decoded.state ~= 'invalidating' or count ~= 4 or tonumber(decoded.baseGeneration) == nil then return 'corrupt' end
if type(decoded.mutationToken) ~= 'string' or decoded.mutationToken == '' then return 'corrupt' end
if decoded.mutationToken ~= ARGV[1] then return 'lost' end
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 'renewed'
`

const commitStateScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
local ok, decoded = pcall(cjson.decode, current)
if not ok or type(decoded) ~= 'table' or tonumber(decoded.schemaVersion) ~= 1 then return 'corrupt' end
local count = 0
for _ in pairs(decoded) do count = count + 1 end
if decoded.state == 'ready' then
  if count ~= 3 or tonumber(decoded.generation) == nil then return 'corrupt' end
  return 'lost'
end
if decoded.state ~= 'invalidating' or count ~= 4 or tonumber(decoded.baseGeneration) == nil then return 'corrupt' end
if type(decoded.mutationToken) ~= 'string' or decoded.mutationToken == '' then return 'corrupt' end
if decoded.mutationToken ~= ARGV[1] then return 'lost' end
redis.call('SET', KEYS[1], ARGV[2])
return 'committed'
`

const rollbackStateScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
local ok, decoded = pcall(cjson.decode, current)
if not ok or type(decoded) ~= 'table' or tonumber(decoded.schemaVersion) ~= 1 then return 'corrupt' end
local count = 0
for _ in pairs(decoded) do count = count + 1 end
if decoded.state == 'ready' then
  if count ~= 3 or tonumber(decoded.generation) == nil then return 'corrupt' end
  return 'lost'
end
if decoded.state ~= 'invalidating' or count ~= 4 or tonumber(decoded.baseGeneration) == nil then return 'corrupt' end
if type(decoded.mutationToken) ~= 'string' or decoded.mutationToken == '' then return 'corrupt' end
if decoded.mutationToken ~= ARGV[1] then return 'lost' end
redis.call('SET', KEYS[1], ARGV[2])
return 'restored'
`

const reconcileStateScript = `
local current = redis.call('GET', KEYS[1])
local authoritative = tonumber(ARGV[2])
if not current then
  redis.call('SET', KEYS[1], ARGV[1])
  return 'repaired'
end
local ok, decoded = pcall(cjson.decode, current)
if not ok or type(decoded) ~= 'table' or tonumber(decoded.schemaVersion) ~= 1 then
  redis.call('SET', KEYS[1], ARGV[1])
  return 'repaired'
end
local count = 0
for _ in pairs(decoded) do count = count + 1 end
if decoded.state == 'invalidating' then
  if count ~= 4 or tonumber(decoded.baseGeneration) == nil then
    redis.call('SET', KEYS[1], ARGV[1])
    return 'repaired'
  end
  return 'invalidating'
end
if decoded.state ~= 'ready' or count ~= 3 or tonumber(decoded.generation) == nil then
  redis.call('SET', KEYS[1], ARGV[1])
  return 'repaired'
end
if tonumber(decoded.generation) >= authoritative then return 'already-newer' end
redis.call('SET', KEYS[1], ARGV[1])
return 'published'
`
