package mail

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	mailtemplate "admin/server/internal/module/message/mail/template"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/yesno"
	"golang.org/x/sync/singleflight"
)

const (
	mailRuntimeSnapshotPrefix  = "mail:runtime:v1:"
	mailRuntimeGenerationKey   = "mail:runtime:generation:v1"
	mailRuntimeLoadLockKey     = "mail:runtime:load-lock:v1"
	mailRuntimeSnapshotTTL     = 10 * time.Minute
	mailRuntimeLoadLockTTL     = 5 * time.Second
	mailRuntimeRetryInterval   = 50 * time.Millisecond
	mailRuntimeRebuildAttempts = 3
)

type runtimeSnapshot struct {
	Generation int64           `json:"generation"`
	Config     runtimeConfig   `json:"config"`
	Templates  []Template      `json:"templates"`
	Rules      []RecipientRule `json:"rules"`
}

// runtimeConfig is deliberately separate from config.Model. The public Model
// hides encrypted credentials with json:"-", but the send hot path must carry
// the ciphertext through Redis so it can decrypt it without a PostgreSQL read.
// This is an internal snapshot DTO and is never exposed by an HTTP response.
type runtimeConfig struct {
	ID                  int64       `json:"id"`
	SecretIDCiphertext  string      `json:"secretIdCiphertext"`
	SecretKeyCiphertext string      `json:"secretKeyCiphertext"`
	Region              string      `json:"region"`
	Endpoint            *string     `json:"endpoint"`
	FromEmail           string      `json:"fromEmail"`
	FromName            string      `json:"fromName"`
	ReplyTo             *string     `json:"replyTo"`
	TTLMinutes          int16       `json:"ttlMinutes"`
	IsEnabled           yesno.Value `json:"isEnabled"`
	CreatedAt           time.Time   `json:"createdAt"`
	UpdatedAt           time.Time   `json:"updatedAt"`
}

func runtimeConfigFromModel(value Config) runtimeConfig {
	return runtimeConfig{
		ID: value.ID, SecretIDCiphertext: value.SecretIDCiphertext,
		SecretKeyCiphertext: value.SecretKeyCiphertext, Region: value.Region,
		Endpoint: value.Endpoint, FromEmail: value.FromEmail, FromName: value.FromName,
		ReplyTo: value.ReplyTo, TTLMinutes: value.TTLMinutes, IsEnabled: value.IsEnabled,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func (c runtimeConfig) valid() error {
	if c.ID < 1 || c.SecretIDCiphertext == "" || c.SecretKeyCiphertext == "" ||
		c.Region == "" || c.FromEmail == "" || c.FromName == "" ||
		c.TTLMinutes < 1 || !yesno.IsValid(c.IsEnabled) || c.UpdatedAt.IsZero() {
		return fmt.Errorf("mail runtime config is invalid")
	}
	return nil
}

type runtimeStore struct {
	stores *Stores
	redis  *projectredis.Client
	group  singleflight.Group
}

func newRuntimeStore(stores *Stores, redis *projectredis.Client) *runtimeStore {
	return &runtimeStore{stores: stores, redis: redis}
}

func NewRuntimeStore(stores *Stores, redis *projectredis.Client) *runtimeStore {
	return newRuntimeStore(stores, redis)
}

func (s *runtimeStore) key(scene string) string { return mailRuntimeSnapshotPrefix + scene }

func (s *runtimeStore) Load(ctx context.Context, scene string) (runtimeSnapshot, error) {
	if s == nil || s.stores == nil || s.redis == nil {
		return runtimeSnapshot{}, fmt.Errorf("mail runtime snapshot dependencies unavailable")
	}
	if snapshot, found, err := s.readCached(ctx, scene); err != nil {
		return runtimeSnapshot{}, err
	} else if found {
		return snapshot, nil
	}
	result := s.group.DoChan(scene, func() (any, error) { return s.rebuild(ctx, scene) })
	select {
	case <-ctx.Done():
		return runtimeSnapshot{}, ctx.Err()
	case shared := <-result:
		if shared.Err != nil {
			return runtimeSnapshot{}, shared.Err
		}
		snapshot, ok := shared.Val.(runtimeSnapshot)
		if !ok {
			return runtimeSnapshot{}, fmt.Errorf("mail runtime snapshot type is invalid")
		}
		return snapshot, nil
	}
}

func (s *runtimeStore) rebuild(ctx context.Context, scene string) (runtimeSnapshot, error) {
	for attempt := 0; attempt < mailRuntimeRebuildAttempts; attempt++ {
		token, err := randomRuntimeToken()
		if err != nil {
			return runtimeSnapshot{}, err
		}
		acquired, err := s.redis.SetStringIfMissing(ctx, mailRuntimeLoadLockKey, token, mailRuntimeLoadLockTTL)
		if err != nil {
			return runtimeSnapshot{}, err
		}
		if !acquired {
			if snapshot, found, readErr := s.readCached(ctx, scene); readErr != nil {
				return runtimeSnapshot{}, readErr
			} else if found {
				return snapshot, nil
			}
			deadline := time.Now().Add(mailRuntimeLoadLockTTL)
			for time.Now().Before(deadline) {
				if err := waitRuntimeRetry(ctx, mailRuntimeRetryInterval); err != nil {
					return runtimeSnapshot{}, err
				}
				if snapshot, found, readErr := s.readCached(ctx, scene); readErr != nil {
					return runtimeSnapshot{}, readErr
				} else if found {
					return snapshot, nil
				}
			}
			continue
		}

		snapshot, loadErr := s.loadFromDatabase(ctx, scene)
		if loadErr != nil {
			_ = s.releaseLoadLock(context.WithoutCancel(ctx), token)
			return runtimeSnapshot{}, loadErr
		}
		payload, marshalErr := json.Marshal(snapshot)
		if marshalErr != nil {
			_ = s.releaseLoadLock(context.WithoutCancel(ctx), token)
			return runtimeSnapshot{}, marshalErr
		}
		published, publishErr := s.redis.EvalString(ctx, publishRuntimeSnapshotScript,
			[]string{s.key(scene), mailRuntimeGenerationKey, mailRuntimeLoadLockKey},
			snapshot.Generation, payload, int64(mailRuntimeSnapshotTTL/time.Millisecond), token)
		_ = s.releaseLoadLock(context.WithoutCancel(ctx), token)
		if publishErr != nil {
			return runtimeSnapshot{}, publishErr
		}
		if published == "published" {
			if currentGeneration, generationErr := s.readGeneration(ctx); generationErr != nil {
				return runtimeSnapshot{}, generationErr
			} else if currentGeneration == snapshot.Generation {
				return snapshot, nil
			}
			continue
		}
		if published != "stale" {
			return runtimeSnapshot{}, fmt.Errorf("mail runtime snapshot publication returned %q", published)
		}
	}
	return runtimeSnapshot{}, fmt.Errorf("mail runtime snapshot changed during rebuild")
}

func (s *runtimeStore) readCached(ctx context.Context, scene string) (runtimeSnapshot, bool, error) {
	generation, err := s.readGeneration(ctx)
	if err != nil {
		return runtimeSnapshot{}, false, err
	}
	raw, found, err := s.redis.GetString(ctx, s.key(scene))
	if err != nil || !found {
		return runtimeSnapshot{}, found, err
	}
	var snapshot runtimeSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return runtimeSnapshot{}, true, fmt.Errorf("decode mail runtime snapshot: %w", err)
	}
	if err := validateRuntimeSnapshot(snapshot, scene); err != nil {
		return runtimeSnapshot{}, true, err
	}
	latestGeneration, err := s.readGeneration(ctx)
	if err != nil {
		return runtimeSnapshot{}, false, err
	}
	if snapshot.Generation != generation || snapshot.Generation != latestGeneration {
		return runtimeSnapshot{}, false, nil
	}
	return snapshot, true, nil
}

func (s *runtimeStore) loadFromDatabase(ctx context.Context, scene string) (runtimeSnapshot, error) {
	generation, err := s.readGeneration(ctx)
	if err != nil {
		return runtimeSnapshot{}, err
	}
	config, err := s.stores.Config.Find(ctx)
	if err != nil {
		return runtimeSnapshot{}, err
	}
	templates, err := s.stores.Template.List(ctx)
	if err != nil {
		return runtimeSnapshot{}, err
	}
	rules, err := s.stores.RecipientRule.List(ctx)
	if err != nil {
		return runtimeSnapshot{}, err
	}
	snapshot := runtimeSnapshot{Generation: generation, Config: runtimeConfigFromModel(config), Templates: templates, Rules: rules}
	if err := validateRuntimeSnapshot(snapshot, scene); err != nil {
		return runtimeSnapshot{}, err
	}
	return snapshot, nil
}

func (s *runtimeStore) Invalidate(ctx context.Context) error {
	if s == nil || s.redis == nil {
		return fmt.Errorf("mail runtime snapshot Redis store unavailable")
	}
	keys := make([]string, 0, len(mailtemplate.FixedCatalog()))
	for _, scene := range mailtemplate.FixedCatalog() {
		keys = append(keys, s.key(scene.Scene))
	}
	_, err := s.redis.EvalString(ctx, invalidateRuntimeSnapshotsScript,
		append([]string{mailRuntimeGenerationKey}, keys...))
	return err
}

func (s *runtimeStore) readGeneration(ctx context.Context) (int64, error) {
	raw, found, err := s.redis.GetString(ctx, mailRuntimeGenerationKey)
	if err != nil {
		return 0, err
	}
	if !found || raw == "" {
		return 0, nil
	}
	generation, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || generation < 0 {
		return 0, fmt.Errorf("mail runtime generation is invalid")
	}
	return generation, nil
}

func validateRuntimeSnapshot(snapshot runtimeSnapshot, scene string) error {
	if snapshot.Generation < 0 {
		return fmt.Errorf("mail runtime generation is invalid")
	}
	if err := snapshot.Config.valid(); err != nil {
		return err
	}
	if _, ok := mailtemplate.FindFixed(scene); !ok {
		return fmt.Errorf("mail runtime scene is invalid")
	}
	foundTemplate := false
	for _, template := range snapshot.Templates {
		if template.Scene == scene {
			foundTemplate = true
			break
		}
	}
	if !foundTemplate {
		return fmt.Errorf("mail runtime template for scene %q is missing", scene)
	}
	return nil
}

const publishRuntimeSnapshotScript = `
local generation = redis.call('GET', KEYS[2])
if generation and tonumber(generation) ~= tonumber(ARGV[1]) then return 'stale' end
local lock = redis.call('GET', KEYS[3])
if lock ~= ARGV[4] then return 'stale' end
redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
return 'published'
`

const releaseRuntimeLoadLockScript = `
local current = redis.call('GET', KEYS[1])
if current == ARGV[1] then redis.call('DEL', KEYS[1]) end
return 1
`

const invalidateRuntimeSnapshotsScript = `
local generation = redis.call('INCR', KEYS[1])
for index = 2, #KEYS do redis.call('DEL', KEYS[index]) end
return tostring(generation)
`

func (s *runtimeStore) releaseLoadLock(ctx context.Context, token string) error {
	_, err := s.redis.EvalString(ctx, releaseRuntimeLoadLockScript, []string{mailRuntimeLoadLockKey}, token)
	return err
}

func waitRuntimeRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func randomRuntimeToken() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate mail runtime lock token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}
