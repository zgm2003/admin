package authplatform

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	projectredis "admin/server/internal/redis"
)

const policySchemaVersion = 1

type policyState struct {
	SchemaVersion int     `json:"schemaVersion"`
	State         string  `json:"state"`
	MutationToken *string `json:"mutationToken"`
	Policy        *Policy `json:"policy"`
}

type PolicyStore struct {
	redis *projectredis.Client
}

func NewPolicyStore(redis *projectredis.Client) *PolicyStore {
	return &PolicyStore{redis: redis}
}

func PolicyKey(code string) string {
	return "auth:policy:" + code
}

func ClearBuiltinPolicies(ctx context.Context, redis *projectredis.Client) error {
	if redis == nil {
		return fmt.Errorf("clear builtin authentication policies requires Redis")
	}
	keys := []string{PolicyKey(BuiltinAdminCode), PolicyKey(BuiltinCanvasCode)}
	if err := redis.DeleteMany(ctx, keys); err != nil {
		return fmt.Errorf("clear builtin authentication policies: %w", err)
	}
	return nil
}

func (s *PolicyStore) read(ctx context.Context, code string) (policyState, bool, error) {
	raw, found, err := s.redis.GetString(ctx, PolicyKey(code))
	if err != nil || !found {
		return policyState{}, found, err
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var state policyState
	if err := decoder.Decode(&state); err != nil {
		return policyState{}, true, fmt.Errorf("decode authentication policy state: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return policyState{}, true, fmt.Errorf("decode authentication policy state trailing data: %w", err)
	}
	if err := validatePolicyState(code, state); err != nil {
		return policyState{}, true, err
	}
	return state, true, nil
}

func validatePolicyState(code string, state policyState) error {
	if state.SchemaVersion != policySchemaVersion {
		return fmt.Errorf("authentication policy schema version is invalid")
	}
	switch state.State {
	case "ready":
		if state.MutationToken != nil || state.Policy == nil || state.Policy.Code != code {
			return fmt.Errorf("ready authentication policy state is invalid")
		}
		if err := validateRuntimePolicy(*state.Policy); err != nil {
			return err
		}
	case "invalidating":
		if state.MutationToken == nil || *state.MutationToken == "" || state.Policy != nil {
			return fmt.Errorf("invalidating authentication policy state is invalid")
		}
	default:
		return fmt.Errorf("authentication policy state is invalid")
	}
	return nil
}

func validateRuntimePolicy(policy Policy) error {
	if policy.ID < 1 || policy.PolicyVersion < 1 || policy.Code == "" || policy.Name == "" {
		return fmt.Errorf("authentication policy identity is invalid")
	}
	if policy.AccessTTL < time.Minute || policy.AccessTTL > 30*24*time.Hour ||
		policy.RefreshTTL < time.Minute || policy.RefreshTTL > 365*24*time.Hour ||
		policy.SessionCacheTTL < time.Minute || policy.SessionCacheTTL > 24*time.Hour ||
		policy.AccessCacheTTL < time.Minute || policy.AccessCacheTTL > 24*time.Hour ||
		policy.MaxSessions < 0 || policy.MaxSessions > MaximumSessions {
		return fmt.Errorf("authentication policy values are invalid")
	}
	return nil
}

func (s *PolicyStore) installReadyIfMissing(ctx context.Context, policy Policy) (policyState, bool, error) {
	payload, err := encodePolicyState(policyState{SchemaVersion: policySchemaVersion, State: "ready", Policy: &policy})
	if err != nil {
		return policyState{}, false, err
	}
	installed, err := s.redis.SetStringIfMissing(ctx, PolicyKey(policy.Code), payload, 0)
	if err != nil {
		return policyState{}, false, err
	}
	if installed {
		return policyState{SchemaVersion: policySchemaVersion, State: "ready", Policy: &policy}, true, nil
	}
	current, found, err := s.read(ctx, policy.Code)
	if err != nil {
		return policyState{}, false, err
	}
	if !found {
		return policyState{}, false, fmt.Errorf("authentication policy disappeared during publication")
	}
	return current, false, nil
}

func (s *PolicyStore) readyForMutation(ctx context.Context, authority Policy) (Policy, error) {
	state, found, err := s.read(ctx, authority.Code)
	if err != nil {
		return Policy{}, err
	}
	if !found {
		state, _, err = s.installReadyIfMissing(ctx, authority)
		if err != nil {
			return Policy{}, err
		}
	}
	if state.State == "invalidating" {
		return Policy{}, ErrUpdating
	}
	if state.Policy == nil {
		return Policy{}, fmt.Errorf("authentication policy ready state is missing its policy")
	}
	return *state.Policy, nil
}

func (s *PolicyStore) acquire(ctx context.Context, code string, prior *Policy) (*policyLease, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	invalidating, err := encodePolicyState(policyState{SchemaVersion: policySchemaVersion, State: "invalidating", MutationToken: &token})
	if err != nil {
		return nil, err
	}
	if prior == nil {
		installed, setErr := s.redis.SetStringIfMissing(ctx, PolicyKey(code), invalidating, 30*time.Second)
		if setErr != nil {
			return nil, setErr
		}
		if !installed {
			return nil, ErrUpdating
		}
		return &policyLease{store: s, code: code, token: token}, nil
	}
	const beginScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
local decoded = cjson.decode(current)
if decoded.state ~= 'ready' or not decoded.policy or decoded.policy.policyVersion ~= tonumber(ARGV[1]) then
  return 'changed'
end
redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
return 'acquired'
`
	result, err := s.redis.EvalString(ctx, beginScript, []string{PolicyKey(code)}, prior.PolicyVersion, invalidating, int64((30*time.Second)/time.Millisecond))
	if err != nil {
		return nil, err
	}
	if result != "acquired" {
		return nil, ErrUpdating
	}
	copyPrior := *prior
	return &policyLease{store: s, code: code, token: token, prior: &copyPrior}, nil
}

type policyLease struct {
	store     *PolicyStore
	code      string
	token     string
	prior     *Policy
	mutex     sync.Mutex
	finalized bool
}

func (l *policyLease) StartRenewal(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Second)
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
	return ctx, func() { cancel(nil); <-done }
}

func (l *policyLease) renew(ctx context.Context) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return nil
	}
	result, err := l.store.redis.EvalString(ctx, renewPolicyScript, []string{PolicyKey(l.code)}, l.token, int64((30*time.Second)/time.Millisecond))
	if err != nil {
		return err
	}
	if result != "renewed" {
		return fmt.Errorf("renew authentication policy lease: %w", ErrUpdating)
	}
	return nil
}

func (l *policyLease) commit(ctx context.Context, policy Policy) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return ErrUpdating
	}
	ready, err := encodePolicyState(policyState{SchemaVersion: policySchemaVersion, State: "ready", Policy: &policy})
	if err != nil {
		return err
	}
	result, err := l.store.redis.EvalString(ctx, publishPolicyScript, []string{PolicyKey(l.code)}, l.token, ready)
	if err != nil {
		return err
	}
	if result != "published" {
		return fmt.Errorf("publish authentication policy: %w", ErrUpdating)
	}
	l.finalized = true
	return nil
}

func (l *policyLease) rollback(ctx context.Context) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.finalized {
		return ErrUpdating
	}
	if l.prior == nil {
		result, err := l.store.redis.EvalString(ctx, deletePolicyScript, []string{PolicyKey(l.code)}, l.token)
		if err != nil {
			return err
		}
		if result != "deleted" {
			return fmt.Errorf("rollback authentication policy creation: %w", ErrUpdating)
		}
		l.finalized = true
		return nil
	}
	ready, err := encodePolicyState(policyState{SchemaVersion: policySchemaVersion, State: "ready", Policy: l.prior})
	if err != nil {
		return err
	}
	result, err := l.store.redis.EvalString(ctx, publishPolicyScript, []string{PolicyKey(l.code)}, l.token, ready)
	if err != nil {
		return err
	}
	if result != "published" {
		return fmt.Errorf("rollback authentication policy: %w", ErrUpdating)
	}
	l.finalized = true
	return nil
}

const renewPolicyScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
local decoded = cjson.decode(current)
if decoded.state ~= 'invalidating' or decoded.mutationToken ~= ARGV[1] then return 'token-mismatch' end
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 'renewed'
`

const publishPolicyScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
local decoded = cjson.decode(current)
if decoded.state ~= 'invalidating' or decoded.mutationToken ~= ARGV[1] then return 'token-mismatch' end
redis.call('SET', KEYS[1], ARGV[2])
return 'published'
`

const deletePolicyScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
local decoded = cjson.decode(current)
if decoded.state ~= 'invalidating' or decoded.mutationToken ~= ARGV[1] then return 'token-mismatch' end
redis.call('DEL', KEYS[1])
return 'deleted'
`

func encodePolicyState(state policyState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("encode authentication policy state: %w", err)
	}
	return string(payload), nil
}

func randomToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate authentication policy mutation token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func isUpdating(err error) bool {
	return errors.Is(err, ErrUpdating)
}
