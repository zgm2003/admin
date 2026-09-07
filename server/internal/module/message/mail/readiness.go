package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/yesno"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

const (
	verifyCodeReadinessSchemaVersion = 1
	verifyCodeReadinessStateReady    = "ready"
	verifyCodeReadinessInvalidating  = "invalidating"
	verifyCodeReadinessKeyPrefix     = "mail:verify-code-readiness:v1:"
	verifyCodeReadinessLockPrefix    = "mail:verify-code-readiness:load-lock:v1:"
	verifyCodeReadinessLoadTimeout   = 5 * time.Second
	verifyCodeReadinessRetryInterval = 25 * time.Millisecond
	verifyCodeReadinessMutationTTL   = 30 * time.Second
)

type verifyCodeReadinessRepository interface {
	FindConfig(context.Context, int64) (Config, error)
	FindTemplateByScene(context.Context, int64, string) (Template, error)
}

type verifyCodeReadinessSnapshot struct {
	SchemaVersion int     `json:"schemaVersion"`
	State         string  `json:"state"`
	Ready         *bool   `json:"ready,omitempty"`
	MutationToken *string `json:"mutationToken,omitempty"`
}

// VerifyCodeReadinessMutation is an opaque ownership token for one readiness
// change. Only the store that successfully began a mutation can publish or
// roll it back.
type VerifyCodeReadinessMutation struct {
	platformID          int64
	scene               string
	priorPayload        string
	invalidatingPayload string
}

// VerifyCodeReadinessStore keeps the public login-config path on Redis while
// PostgreSQL remains the authoritative source rebuilt on a missing key only.
type VerifyCodeReadinessStore interface {
	Current(context.Context, int64, string) (bool, error)
	BeginMutation(context.Context, int64, string) (VerifyCodeReadinessMutation, error)
	PublishMutation(context.Context, VerifyCodeReadinessMutation) error
	RollbackMutation(context.Context, VerifyCodeReadinessMutation) error
}

type verifyCodeReadinessStore struct {
	repository verifyCodeReadinessRepository
	redis      *projectredis.Client
	group      singleflight.Group
}

func NewVerifyCodeReadinessStore(repository verifyCodeReadinessRepository, redis *projectredis.Client) *verifyCodeReadinessStore {
	return &verifyCodeReadinessStore{repository: repository, redis: redis}
}

func verifyCodeReadinessKey(platformID int64, scene string) string {
	return verifyCodeReadinessKeyPrefix + strconv.FormatInt(platformID, 10) + ":" + scene
}

func verifyCodeReadinessLoadLockKey(platformID int64, scene string) string {
	return verifyCodeReadinessLockPrefix + strconv.FormatInt(platformID, 10) + ":" + scene
}

func (s *verifyCodeReadinessStore) Current(ctx context.Context, platformID int64, scene string) (bool, error) {
	if err := validateVerifyCodeReadinessCoordinates(platformID, scene); err != nil {
		return false, err
	}
	if s == nil || s.redis == nil || s.repository == nil {
		return false, fmt.Errorf("mail verification readiness store dependencies unavailable")
	}
	snapshot, _, found, err := s.readSnapshot(ctx, platformID, scene)
	if err != nil {
		return false, err
	}
	if found {
		return *snapshot.Ready, nil
	}

	key := verifyCodeReadinessKey(platformID, scene)
	result := s.group.DoChan(key, func() (any, error) {
		sharedContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), verifyCodeReadinessLoadTimeout)
		defer cancel()
		return s.rebuild(sharedContext, platformID, scene)
	})
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case sharedResult := <-result:
		if sharedResult.Err != nil {
			return false, sharedResult.Err
		}
		ready, ok := sharedResult.Val.(bool)
		if !ok {
			return false, fmt.Errorf("mail verification readiness rebuild returned an invalid value")
		}
		return ready, nil
	}
}

func (s *verifyCodeReadinessStore) readSnapshot(ctx context.Context, platformID int64, scene string) (verifyCodeReadinessSnapshot, string, bool, error) {
	if s == nil || s.redis == nil {
		return verifyCodeReadinessSnapshot{}, "", false, fmt.Errorf("mail verification readiness Redis store unavailable")
	}
	raw, found, err := s.redis.GetString(ctx, verifyCodeReadinessKey(platformID, scene))
	if err != nil {
		return verifyCodeReadinessSnapshot{}, "", false, err
	}
	if !found {
		return verifyCodeReadinessSnapshot{}, "", false, nil
	}
	snapshot, err := decodeVerifyCodeReadinessSnapshot(raw)
	if err != nil {
		return verifyCodeReadinessSnapshot{}, raw, true, err
	}
	if snapshot.State == verifyCodeReadinessInvalidating {
		return verifyCodeReadinessSnapshot{}, raw, true, fmt.Errorf("mail verification readiness is invalidating")
	}
	return snapshot, raw, true, nil
}

func (s *verifyCodeReadinessStore) rebuild(ctx context.Context, platformID int64, scene string) (bool, error) {
	if s == nil || s.redis == nil || s.repository == nil {
		return false, fmt.Errorf("mail verification readiness rebuild dependencies unavailable")
	}
	token, err := randomVerifyCodeReadinessToken()
	if err != nil {
		return false, err
	}
	lockKey := verifyCodeReadinessLoadLockKey(platformID, scene)
	acquired, err := s.redis.SetStringIfMissing(ctx, lockKey, token, verifyCodeReadinessLoadTimeout)
	if err != nil {
		return false, err
	}
	if acquired {
		defer func() {
			releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			_, _ = s.redis.EvalString(releaseContext, releaseVerifyCodeReadinessLoadLockScript, []string{lockKey}, token)
		}()
		ready, loadErr := s.loadFromRepository(ctx, platformID, scene)
		if loadErr != nil {
			return false, loadErr
		}
		payload, encodeErr := encodeVerifyCodeReadinessSnapshot(newReadyVerifyCodeReadinessSnapshot(ready))
		if encodeErr != nil {
			return false, encodeErr
		}
		result, publishErr := s.redis.EvalString(ctx, installVerifyCodeReadinessSnapshotScript,
			[]string{verifyCodeReadinessKey(platformID, scene)}, payload)
		if publishErr != nil {
			return false, publishErr
		}
		switch result {
		case "published":
			return ready, nil
		case "exists":
			current, _, found, readErr := s.readSnapshot(ctx, platformID, scene)
			if readErr != nil {
				return false, readErr
			}
			if !found {
				return false, fmt.Errorf("mail verification readiness disappeared during rebuild")
			}
			return *current.Ready, nil
		default:
			return false, fmt.Errorf("mail verification readiness rebuild returned %q", result)
		}
	}

	deadline := time.Now().Add(verifyCodeReadinessLoadTimeout)
	for time.Now().Before(deadline) {
		if err := waitVerifyCodeReadinessRetry(ctx, verifyCodeReadinessRetryInterval); err != nil {
			return false, err
		}
		snapshot, _, found, readErr := s.readSnapshot(ctx, platformID, scene)
		if readErr != nil {
			return false, readErr
		}
		if found {
			return *snapshot.Ready, nil
		}
	}
	return false, fmt.Errorf("mail verification readiness rebuild timed out")
}

func (s *verifyCodeReadinessStore) loadFromRepository(ctx context.Context, platformID int64, scene string) (bool, error) {
	if s == nil || s.repository == nil {
		return false, fmt.Errorf("mail verification readiness repository unavailable")
	}
	config, err := s.repository.FindConfig(ctx, platformID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if config.IsEnabled != yesno.Yes || config.SecretIDCiphertext == "" || config.SecretKeyCiphertext == "" {
		return false, nil
	}
	template, err := s.repository.FindTemplateByScene(ctx, platformID, scene)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return template.IsEnabled == yesno.Yes, nil
}

func (s *verifyCodeReadinessStore) BeginMutation(ctx context.Context, platformID int64, scene string) (VerifyCodeReadinessMutation, error) {
	if s == nil || s.redis == nil || s.repository == nil {
		return VerifyCodeReadinessMutation{}, fmt.Errorf("mail verification readiness mutation dependencies unavailable")
	}
	if _, err := s.Current(ctx, platformID, scene); err != nil {
		return VerifyCodeReadinessMutation{}, err
	}
	_, priorPayload, found, err := s.readSnapshot(ctx, platformID, scene)
	if err != nil {
		return VerifyCodeReadinessMutation{}, err
	}
	if !found {
		return VerifyCodeReadinessMutation{}, fmt.Errorf("mail verification readiness is unavailable")
	}
	token, err := randomVerifyCodeReadinessToken()
	if err != nil {
		return VerifyCodeReadinessMutation{}, err
	}
	invalidatingPayload, err := encodeVerifyCodeReadinessSnapshot(verifyCodeReadinessSnapshot{
		SchemaVersion: verifyCodeReadinessSchemaVersion,
		State:         verifyCodeReadinessInvalidating,
		MutationToken: &token,
	})
	if err != nil {
		return VerifyCodeReadinessMutation{}, err
	}
	result, err := s.redis.EvalString(ctx, beginVerifyCodeReadinessMutationScript,
		[]string{verifyCodeReadinessKey(platformID, scene)}, priorPayload, invalidatingPayload, int64(verifyCodeReadinessMutationTTL/time.Millisecond))
	if err != nil {
		return VerifyCodeReadinessMutation{}, err
	}
	if result != "acquired" {
		return VerifyCodeReadinessMutation{}, fmt.Errorf("mail verification readiness mutation returned %q", result)
	}
	return VerifyCodeReadinessMutation{
		platformID: platformID, scene: scene, priorPayload: priorPayload, invalidatingPayload: invalidatingPayload,
	}, nil
}

func (s *verifyCodeReadinessStore) PublishMutation(ctx context.Context, mutation VerifyCodeReadinessMutation) error {
	if s == nil || s.redis == nil || s.repository == nil {
		return fmt.Errorf("mail verification readiness publication dependencies unavailable")
	}
	if err := validateVerifyCodeReadinessMutation(mutation); err != nil {
		return err
	}
	ready, err := s.loadFromRepository(ctx, mutation.platformID, mutation.scene)
	if err != nil {
		return err
	}
	payload, err := encodeVerifyCodeReadinessSnapshot(newReadyVerifyCodeReadinessSnapshot(ready))
	if err != nil {
		return err
	}
	return s.replaceMutation(ctx, mutation, payload)
}

func (s *verifyCodeReadinessStore) RollbackMutation(ctx context.Context, mutation VerifyCodeReadinessMutation) error {
	if err := validateVerifyCodeReadinessMutation(mutation); err != nil {
		return err
	}
	return s.replaceMutation(ctx, mutation, mutation.priorPayload)
}

func (s *verifyCodeReadinessStore) replaceMutation(ctx context.Context, mutation VerifyCodeReadinessMutation, payload string) error {
	if s == nil || s.redis == nil {
		return fmt.Errorf("mail verification readiness Redis store unavailable")
	}
	result, err := s.redis.EvalString(ctx, publishVerifyCodeReadinessMutationScript,
		[]string{verifyCodeReadinessKey(mutation.platformID, mutation.scene)}, mutation.invalidatingPayload, payload)
	if err != nil {
		return err
	}
	if result != "published" {
		return fmt.Errorf("mail verification readiness publication returned %q", result)
	}
	return nil
}

func validateVerifyCodeReadinessCoordinates(platformID int64, scene string) error {
	if platformID < 1 || scene != SceneLogin {
		return fmt.Errorf("mail verification readiness coordinates are invalid")
	}
	return nil
}

func validateVerifyCodeReadinessMutation(mutation VerifyCodeReadinessMutation) error {
	if err := validateVerifyCodeReadinessCoordinates(mutation.platformID, mutation.scene); err != nil {
		return err
	}
	if mutation.priorPayload == "" || mutation.invalidatingPayload == "" {
		return fmt.Errorf("mail verification readiness mutation is invalid")
	}
	return nil
}

func newReadyVerifyCodeReadinessSnapshot(ready bool) verifyCodeReadinessSnapshot {
	return verifyCodeReadinessSnapshot{SchemaVersion: verifyCodeReadinessSchemaVersion, State: verifyCodeReadinessStateReady, Ready: &ready}
}

func encodeVerifyCodeReadinessSnapshot(snapshot verifyCodeReadinessSnapshot) (string, error) {
	if err := validateVerifyCodeReadinessSnapshot(snapshot); err != nil {
		return "", err
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode mail verification readiness: %w", err)
	}
	return string(payload), nil
}

func decodeVerifyCodeReadinessSnapshot(raw string) (verifyCodeReadinessSnapshot, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot verifyCodeReadinessSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return verifyCodeReadinessSnapshot{}, fmt.Errorf("decode mail verification readiness: %w", err)
	}
	if err := ensureVerifyCodeReadinessJSONEnd(decoder); err != nil {
		return verifyCodeReadinessSnapshot{}, err
	}
	if err := validateVerifyCodeReadinessSnapshot(snapshot); err != nil {
		return verifyCodeReadinessSnapshot{}, err
	}
	return snapshot, nil
}

func ensureVerifyCodeReadinessJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode mail verification readiness: multiple JSON values")
		}
		return fmt.Errorf("decode mail verification readiness: %w", err)
	}
	return nil
}

func validateVerifyCodeReadinessSnapshot(snapshot verifyCodeReadinessSnapshot) error {
	if snapshot.SchemaVersion != verifyCodeReadinessSchemaVersion {
		return fmt.Errorf("mail verification readiness schema version is invalid")
	}
	switch snapshot.State {
	case verifyCodeReadinessStateReady:
		if snapshot.Ready == nil || snapshot.MutationToken != nil {
			return fmt.Errorf("ready mail verification snapshot is invalid")
		}
	case verifyCodeReadinessInvalidating:
		if snapshot.Ready != nil || snapshot.MutationToken == nil || *snapshot.MutationToken == "" {
			return fmt.Errorf("invalidating mail verification snapshot is invalid")
		}
	default:
		return fmt.Errorf("mail verification readiness state is invalid")
	}
	return nil
}

func waitVerifyCodeReadinessRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func randomVerifyCodeReadinessToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate mail verification readiness token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

const releaseVerifyCodeReadinessLoadLockScript = `
local current = redis.call('GET', KEYS[1])
if current == ARGV[1] then redis.call('DEL', KEYS[1]) end
return 'released'
`

const installVerifyCodeReadinessSnapshotScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then return 'exists' end
redis.call('SET', KEYS[1], ARGV[1])
return 'published'
`

const beginVerifyCodeReadinessMutationScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
if current ~= ARGV[1] then return 'changed' end
redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
return 'acquired'
`

const publishVerifyCodeReadinessMutationScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
if current ~= ARGV[1] then return 'changed' end
redis.call('SET', KEYS[1], ARGV[2])
return 'published'
`
