package dictionary

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cacheFill"
)

const dictionaryCacheSchema = 1

type optionsCache struct {
	redis *projectredis.Client
	ttl   time.Duration
}

type optionsSnapshot struct {
	SchemaVersion int          `json:"schemaVersion"`
	Generation    string       `json:"generation"`
	Language      string       `json:"language"`
	Codes         []string     `json:"codes"`
	Options       OptionResult `json:"options"`
}

func NewOptionsCache(redis *projectredis.Client) *optionsCache {
	return &optionsCache{redis: redis, ttl: 10 * time.Minute}
}

func (c *optionsCache) acquireFill(ctx context.Context, codes []string, language string) (*cachefill.Lease, error) {
	if c == nil || c.redis == nil {
		return nil, fmt.Errorf("dictionary options cache unavailable")
	}
	sorted := append([]string(nil), codes...)
	sort.Strings(sorted)
	return cachefill.Try(ctx, c.redis.UniversalClient(), "system-dictionary", strings.Join(append([]string{language}, sorted...), "\x00"))
}

func (c *optionsCache) generationKey() string { return "system:dictionary:generation:v1" }
func (c *optionsCache) mutationKey() string   { return "system:dictionary:mutation:v1" }

func (c *optionsCache) snapshotKey(codes []string, language, generation string) string {
	sorted := append([]string(nil), codes...)
	sort.Strings(sorted)
	hash := sha256.Sum256([]byte(strings.Join(append([]string{language, generation}, sorted...), "\x00")))
	return "system:dictionary:options:v1:" + hex.EncodeToString(hash[:])
}

func (c *optionsCache) currentGeneration(ctx context.Context) (string, error) {
	if _, found, err := c.redis.GetString(ctx, c.mutationKey()); err != nil {
		return "", err
	} else if found {
		return "", fmt.Errorf("dictionary options are invalidating")
	}
	value, found, err := c.redis.GetString(ctx, c.generationKey())
	if err != nil {
		return "", err
	}
	if found && value != "" {
		return value, nil
	}
	if _, err := c.redis.SetStringIfMissing(ctx, c.generationKey(), "1", 0); err != nil {
		return "", err
	}
	value, found, err = c.redis.GetString(ctx, c.generationKey())
	if err != nil {
		return "", err
	}
	if !found || value == "" {
		return "", fmt.Errorf("dictionary cache generation is missing")
	}
	if _, mutating, err := c.redis.GetString(ctx, c.mutationKey()); err != nil {
		return "", err
	} else if mutating {
		return "", fmt.Errorf("dictionary options are invalidating")
	}
	return value, nil
}

func (c *optionsCache) Get(ctx context.Context, codes []string, language string) (OptionResult, bool, error) {
	generation, err := c.currentGeneration(ctx)
	if err != nil {
		return nil, false, err
	}
	raw, found, err := c.redis.GetString(ctx, c.snapshotKey(codes, language, generation))
	if err != nil || !found {
		return nil, found, err
	}
	options, err := decodeOptionsSnapshot(raw, codes, language, generation)
	if err != nil {
		return nil, false, err
	}
	return options, true, nil
}

func (c *optionsCache) Set(ctx context.Context, codes []string, language string, options OptionResult) error {
	generation, err := c.currentGeneration(ctx)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(optionsSnapshot{SchemaVersion: dictionaryCacheSchema, Generation: generation, Language: language, Codes: append([]string(nil), codes...), Options: options})
	if err != nil {
		return fmt.Errorf("encode dictionary options cache: %w", err)
	}
	return c.redis.SetString(ctx, c.snapshotKey(codes, language, generation), string(payload), c.ttl)
}

func decodeOptionsSnapshot(raw string, requestedCodes []string, language, generation string) (OptionResult, error) {
	if err := scanOptionsSnapshotKeys(raw); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot optionsSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decode dictionary options cache: %w", err)
	}
	if decoder.More() {
		return nil, fmt.Errorf("decode dictionary options cache: trailing value")
	}
	if snapshot.SchemaVersion != dictionaryCacheSchema || snapshot.Generation != generation || snapshot.Language != language {
		return nil, fmt.Errorf("dictionary options cache is stale")
	}
	want := append([]string(nil), requestedCodes...)
	got := append([]string(nil), snapshot.Codes...)
	sort.Strings(want)
	sort.Strings(got)
	if len(want) != len(got) || len(snapshot.Options) != len(want) {
		return nil, fmt.Errorf("dictionary options cache codes do not match")
	}
	for index, code := range want {
		if got[index] != code {
			return nil, fmt.Errorf("dictionary options cache codes do not match")
		}
		options, ok := snapshot.Options[code]
		if !ok || options == nil {
			return nil, fmt.Errorf("dictionary options cache is missing code %q", code)
		}
		seenValues := make(map[string]struct{}, len(options))
		for _, option := range options {
			if option.Label == "" || option.Value == "" {
				return nil, fmt.Errorf("dictionary options cache contains an empty option")
			}
			if _, exists := seenValues[option.Value]; exists {
				return nil, fmt.Errorf("dictionary options cache contains duplicate values")
			}
			seenValues[option.Value] = struct{}{}
		}
	}
	return snapshot.Options, nil
}

func scanOptionsSnapshotKeys(raw string) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode dictionary options cache: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return fmt.Errorf("dictionary options cache must be an object")
	}
	seen := map[string]struct{}{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("dictionary options cache key is invalid")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("dictionary options cache contains duplicate field %q", key)
		}
		seen[key] = struct{}{}
		var discard json.RawMessage
		if err := decoder.Decode(&discard); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("dictionary options cache contains trailing data")
	}
	return nil
}

func (c *optionsCache) Mutate(ctx context.Context, change func(context.Context) error) error {
	if c == nil || c.redis == nil || change == nil {
		return fmt.Errorf("dictionary mutation dependencies unavailable")
	}
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("create dictionary mutation token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	result, err := c.redis.EvalString(ctx, beginDictionaryMutationScript, []string{c.mutationKey(), c.generationKey()}, token, int64((15*time.Second)/time.Millisecond))
	if err != nil {
		return err
	}
	if result != "acquired" {
		return fmt.Errorf("dictionary mutation returned %q", result)
	}
	if err := change(ctx); err != nil {
		rollbackContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_, releaseErr := c.redis.EvalString(rollbackContext, releaseDictionaryMutationScript, []string{c.mutationKey()}, token)
		if releaseErr != nil {
			return fmt.Errorf("dictionary mutation rollback: %w", errors.Join(err, releaseErr))
		}
		return err
	}
	publishContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	result, err = c.redis.EvalString(publishContext, publishDictionaryMutationScript, []string{c.mutationKey()}, token)
	if err != nil {
		return err
	}
	if result != "published" {
		return fmt.Errorf("dictionary mutation publication returned %q", result)
	}
	return nil
}

const beginDictionaryMutationScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then return 'busy' end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('INCR', KEYS[2])
return 'acquired'
`

const releaseDictionaryMutationScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 'stale' end
redis.call('DEL', KEYS[1])
return 'released'
`

const publishDictionaryMutationScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 'stale' end
redis.call('DEL', KEYS[1])
return 'published'
`
