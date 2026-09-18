package dictionary

import (
	"bytes"
	"context"
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
	"admin/server/internal/shared/cacheGeneration"
)

const (
	dictionaryCacheSchema = 1
	dictionaryCacheTTL    = 10 * time.Minute
)

var (
	dictionaryGenerationScope = cachegeneration.Scope{Namespace: "system.dictionary", ScopeKey: "global"}
	ErrOptionsSnapshotCorrupt = errors.New("dictionary options snapshot is corrupt")
)

func CacheGenerationScope() cachegeneration.Scope { return dictionaryGenerationScope }

type optionsCache struct {
	redis *projectredis.Client
	store *cachegeneration.Store
	scope cachegeneration.Scope
	ttl   time.Duration
}

type optionsSnapshot struct {
	SchemaVersion int          `json:"schemaVersion"`
	Generation    int64        `json:"generation"`
	Language      string       `json:"language"`
	Codes         []string     `json:"codes"`
	Options       OptionResult `json:"options"`
}

func NewOptionsCache(redis *projectredis.Client) *optionsCache {
	if redis == nil {
		return nil
	}
	return &optionsCache{redis: redis, scope: dictionaryGenerationScope, ttl: dictionaryCacheTTL}
}

func (c *optionsCache) SetStateStore(store *cachegeneration.Store) { c.store = store }

func (c *optionsCache) ReadState(ctx context.Context) (cachegeneration.State, bool, error) {
	if c == nil || c.store == nil {
		return cachegeneration.State{}, false, fmt.Errorf("dictionary cache state store is not configured")
	}
	return c.store.Read(ctx, c.scope)
}

func (c *optionsCache) Get(ctx context.Context, generation int64, codes []string, language string) (OptionResult, bool, error) {
	if c == nil || c.redis == nil {
		return nil, false, fmt.Errorf("dictionary options cache is not configured")
	}
	variant, err := optionsVariant(codes, language)
	if err != nil {
		return nil, false, err
	}
	key, err := cachegeneration.SnapshotKey(c.scope, generation, variant)
	if err != nil {
		return nil, false, err
	}
	raw, found, err := c.redis.GetString(ctx, key)
	if err != nil || !found {
		return nil, found, err
	}
	options, err := decodeOptionsSnapshot(raw, codes, language, generation)
	if err != nil {
		return nil, false, err
	}
	return options, true, nil
}

func (c *optionsCache) Put(ctx context.Context, generation int64, codes []string, language string, options OptionResult) error {
	if c == nil || c.redis == nil {
		return fmt.Errorf("dictionary options cache is not configured")
	}
	canonical := canonicalCodes(codes)
	variant, err := optionsVariant(canonical, language)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(optionsSnapshot{
		SchemaVersion: dictionaryCacheSchema,
		Generation:    generation,
		Language:      language,
		Codes:         canonical,
		Options:       options,
	})
	if err != nil {
		return fmt.Errorf("encode dictionary options snapshot: %w", err)
	}
	key, err := cachegeneration.SnapshotKey(c.scope, generation, variant)
	if err != nil {
		return err
	}
	return c.redis.SetString(ctx, key, string(payload), c.ttl)
}

func (c *optionsCache) TryFill(ctx context.Context, generation int64, variant string) (*cachefill.Lease, error) {
	if c == nil || c.redis == nil {
		return nil, fmt.Errorf("dictionary options cache is not configured")
	}
	target, err := cachegeneration.FillKey(c.scope, generation, variant)
	if err != nil {
		return nil, err
	}
	return cachefill.Try(ctx, c.redis.UniversalClient(), cachegeneration.StateKey(c.scope), target)
}

func (c *optionsCache) TryRepair(ctx context.Context, variant string) (*cachefill.Lease, error) {
	if c == nil || c.redis == nil {
		return nil, fmt.Errorf("dictionary options cache is not configured")
	}
	return cachefill.Try(ctx, c.redis.UniversalClient(), cachegeneration.StateKey(c.scope), "repair:"+variant)
}

func optionsVariant(codes []string, language string) (string, error) {
	canonical := canonicalCodes(codes)
	if len(canonical) == 0 || strings.TrimSpace(language) == "" {
		return "", fmt.Errorf("dictionary options snapshot coordinates are invalid")
	}
	hash := sha256.Sum256([]byte(language + "\x00" + strings.Join(canonical, "\x00")))
	return "options:" + hex.EncodeToString(hash[:]), nil
}

func canonicalCodes(codes []string) []string {
	seen := make(map[string]struct{}, len(codes))
	result := make([]string, 0, len(codes))
	for _, code := range codes {
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	sort.Strings(result)
	return result
}

func decodeOptionsSnapshot(raw string, requestedCodes []string, language string, generation int64) (OptionResult, error) {
	if generation < 1 {
		return nil, corruptOptionsSnapshot("generation is invalid")
	}
	if err := scanOptionsSnapshotKeys(raw); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot optionsSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, corruptOptionsSnapshot("decode payload: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, corruptOptionsSnapshot("payload has trailing data")
	}
	if snapshot.SchemaVersion != dictionaryCacheSchema || snapshot.Generation != generation || snapshot.Language != language {
		return nil, corruptOptionsSnapshot("coordinates do not match")
	}
	want := canonicalCodes(requestedCodes)
	got := canonicalCodes(snapshot.Codes)
	if len(want) != len(requestedCodes) || len(got) != len(snapshot.Codes) || len(want) != len(got) || len(snapshot.Options) != len(want) {
		return nil, corruptOptionsSnapshot("code set does not match")
	}
	for index, code := range want {
		if got[index] != code || snapshot.Codes[index] != code {
			return nil, corruptOptionsSnapshot("code set does not match")
		}
		options, ok := snapshot.Options[code]
		if !ok || options == nil {
			return nil, corruptOptionsSnapshot("code %q is missing", code)
		}
		seenValues := make(map[string]struct{}, len(options))
		for _, option := range options {
			if option.Label == "" || option.Value == "" {
				return nil, corruptOptionsSnapshot("option is empty")
			}
			if _, exists := seenValues[option.Value]; exists {
				return nil, corruptOptionsSnapshot("option value is duplicated")
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
		return corruptOptionsSnapshot("decode payload: %v", err)
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return corruptOptionsSnapshot("payload must be an object")
	}
	seen := map[string]struct{}{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return corruptOptionsSnapshot("decode field: %v", err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return corruptOptionsSnapshot("field name is invalid")
		}
		if _, exists := seen[key]; exists {
			return corruptOptionsSnapshot("field %q is duplicated", key)
		}
		seen[key] = struct{}{}
		var discard json.RawMessage
		if err := decoder.Decode(&discard); err != nil {
			return corruptOptionsSnapshot("decode field %q: %v", key, err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return corruptOptionsSnapshot("decode payload: %v", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return corruptOptionsSnapshot("payload has trailing data")
	}
	return nil
}

func corruptOptionsSnapshot(format string, values ...any) error {
	return fmt.Errorf("%w: %s", ErrOptionsSnapshotCorrupt, fmt.Sprintf(format, values...))
}
