package sms

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/module/message/sms/recipientRule"
	"admin/server/internal/module/message/sms/template"
	"admin/server/internal/shared/cacheFill"
	"admin/server/internal/shared/yesno"

	projectredis "admin/server/internal/redis"
)

const (
	runtimeCacheSchemaVersion = 1
	runtimeGenerationKey      = "sms:runtime:generation:v1"
	runtimeMutationKey        = "sms:runtime:mutation:v1"
	runtimeSnapshotPrefix     = "sms:runtime:v1:"
	readinessPrefix           = "sms:readiness:v1:"
	runtimeSnapshotTTL        = 10 * time.Minute
	readinessTTL              = 10 * time.Minute
	runtimeMutationLeaseTTL   = 6 * time.Second
)

// ConfiguredConfig is the encrypted runtime view of the SMS configuration.
// Credentials stay encrypted in Redis and are decrypted only immediately before
// a provider call.
type ConfiguredConfig struct {
	SecretIDCiphertext  string
	SecretKeyCiphertext string
	SDKAppID            string
	SignName            string
	Region              string
	Endpoint            string
	TTLMinutes          int
	IsEnabled           yesno.Value
}

type TemplateFact struct {
	Name              string
	TencentTemplateID string
	ParameterKeys     []string
	ExampleVariables  map[string]string
	IsEnabled         yesno.Value
}

type RuleFact struct {
	ID                int64
	Scope             string
	Action            string
	PatternCiphertext string
	PatternHint       string
	IsEnabled         yesno.Value
}

// RuntimeFacts is everything the sending path needs, loaded from PostgreSQL.
type RuntimeFacts struct {
	Configured bool
	Config     ConfiguredConfig
	Templates  map[string]TemplateFact
	Rules      []RuleFact
}

type runtimeSnapshot struct {
	SchemaVersion       int                    `json:"schemaVersion"`
	Generation          string                 `json:"generation"`
	Configured          bool                   `json:"configured"`
	SecretIDCiphertext  string                 `json:"secretIdCiphertext"`
	SecretKeyCiphertext string                 `json:"secretKeyCiphertext"`
	SDKAppID            string                 `json:"smsSdkAppId"`
	SignName            string                 `json:"signName"`
	Region              string                 `json:"region"`
	Endpoint            string                 `json:"endpoint"`
	TTLMinutes          int                    `json:"ttlMinutes"`
	IsEnabled           int16                  `json:"isEnabled"`
	Templates           map[string]templateRow `json:"templates"`
	Rules               []ruleRow              `json:"rules"`
}

type templateRow struct {
	Name              string            `json:"name"`
	TencentTemplateID string            `json:"tencentTemplateId"`
	ParameterKeys     []string          `json:"parameterKeys"`
	ExampleVariables  map[string]string `json:"exampleVariables"`
	IsEnabled         int16             `json:"isEnabled"`
}

type ruleRow struct {
	ID                int64  `json:"id"`
	Scope             string `json:"scope"`
	Action            string `json:"action"`
	PatternCiphertext string `json:"patternCiphertext"`
	PatternHint       string `json:"patternHint"`
	IsEnabled         int16  `json:"isEnabled"`
}

type readinessPayload struct {
	SchemaVersion int  `json:"schemaVersion"`
	Ready         bool `json:"ready"`
	TTLMinutes    int  `json:"ttlMinutes"`
}

// RuntimeCache caches the runtime facts and per-scene readiness behind one
// cross-instance generation counter. PostgreSQL stays the source of truth.
type RuntimeCache struct {
	redis     *projectredis.Client
	namespace string
}

func NewRuntimeCache(redis *projectredis.Client) *RuntimeCache {
	return &RuntimeCache{redis: redis}
}

func (c *RuntimeCache) generationKey() string { return runtimeGenerationKey + c.namespace }
func (c *RuntimeCache) mutationKey() string   { return runtimeMutationKey + c.namespace }

func (c *RuntimeCache) currentGeneration(ctx context.Context) (string, error) {
	if _, mutating, err := c.redis.GetString(ctx, c.mutationKey()); err != nil {
		return "", err
	} else if mutating {
		return "", fmt.Errorf("sms runtime is invalidating")
	}
	value, found, err := c.redis.GetString(ctx, c.generationKey())
	if err != nil {
		return "", err
	}
	if found && value != "" {
		if generation, parseErr := strconv.ParseInt(value, 10, 64); parseErr != nil || generation < 1 {
			return "", fmt.Errorf("sms runtime generation is invalid")
		}
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
		return "", fmt.Errorf("sms runtime generation is missing")
	}
	if generation, parseErr := strconv.ParseInt(value, 10, 64); parseErr != nil || generation < 1 {
		return "", fmt.Errorf("sms runtime generation is invalid")
	}
	return value, nil
}

func (c *RuntimeCache) snapshotKey(generation string) string {
	return runtimeSnapshotPrefix + generation + c.namespace
}

func (c *RuntimeCache) readinessKey(generation, scene string) string {
	return readinessPrefix + generation + ":" + scene + c.namespace
}

// Load returns the cached runtime facts, rebuilding them at most once per target
// across instances through shared/cacheFill.
func (c *RuntimeCache) Load(ctx context.Context, load func(context.Context) (RuntimeFacts, error)) (RuntimeFacts, error) {
	if c == nil || c.redis == nil || load == nil {
		return RuntimeFacts{}, fmt.Errorf("sms runtime cache is unavailable")
	}
	var lease *cachefill.Lease
	defer func() { _ = lease.Release(ctx) }()
	for admission := 0; admission < 25; admission++ {
		generation, err := c.currentGeneration(ctx)
		if err != nil {
			return RuntimeFacts{}, err
		}
		if facts, found, err := c.read(ctx, generation); err != nil {
			return RuntimeFacts{}, err
		} else if found {
			return facts, nil
		}
		lease, err = cachefill.Try(ctx, c.redis.UniversalClient(), "sms-runtime"+c.namespace, c.snapshotKey(generation))
		if err != nil {
			return RuntimeFacts{}, err
		}
		if lease != nil {
			break
		}
		if err := cachefill.Wait(ctx); err != nil {
			return RuntimeFacts{}, err
		}
	}
	if lease == nil {
		return RuntimeFacts{}, fmt.Errorf("sms runtime rebuild is busy")
	}
	workContext, cancel := lease.WorkContext(ctx)
	defer cancel()

	generation, err := c.currentGeneration(workContext)
	if err != nil {
		return RuntimeFacts{}, err
	}
	if facts, found, err := c.read(workContext, generation); err != nil {
		return RuntimeFacts{}, err
	} else if found {
		return facts, nil
	}
	facts, err := load(workContext)
	if err != nil {
		return RuntimeFacts{}, err
	}
	if err := c.write(workContext, generation, facts); err != nil {
		return RuntimeFacts{}, err
	}
	return facts, nil
}

func (c *RuntimeCache) read(ctx context.Context, generation string) (RuntimeFacts, bool, error) {
	raw, found, err := c.redis.GetString(ctx, c.snapshotKey(generation))
	if err != nil {
		return RuntimeFacts{}, false, err
	}
	if !found {
		return RuntimeFacts{}, false, nil
	}
	snapshot, err := decodeRuntimeSnapshot(raw, generation)
	if err != nil {
		return RuntimeFacts{}, false, err
	}
	return factsOf(snapshot), true, nil
}

func (c *RuntimeCache) write(ctx context.Context, generation string, facts RuntimeFacts) error {
	snapshot := runtimeSnapshot{
		SchemaVersion:       runtimeCacheSchemaVersion,
		Generation:          generation,
		Configured:          facts.Configured,
		SecretIDCiphertext:  facts.Config.SecretIDCiphertext,
		SecretKeyCiphertext: facts.Config.SecretKeyCiphertext,
		SDKAppID:            facts.Config.SDKAppID,
		SignName:            facts.Config.SignName,
		Region:              facts.Config.Region,
		Endpoint:            facts.Config.Endpoint,
		TTLMinutes:          facts.Config.TTLMinutes,
		IsEnabled:           int16(facts.Config.IsEnabled),
		Templates:           make(map[string]templateRow, len(facts.Templates)),
	}
	for scene, value := range facts.Templates {
		snapshot.Templates[scene] = templateRow{
			Name: value.Name, TencentTemplateID: value.TencentTemplateID,
			ParameterKeys: value.ParameterKeys, ExampleVariables: value.ExampleVariables,
			IsEnabled: int16(value.IsEnabled),
		}
	}
	for _, rule := range facts.Rules {
		snapshot.Rules = append(snapshot.Rules, ruleRow{
			ID: rule.ID, Scope: rule.Scope, Action: rule.Action,
			PatternCiphertext: rule.PatternCiphertext, PatternHint: rule.PatternHint,
			IsEnabled: int16(rule.IsEnabled),
		})
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode sms runtime snapshot: %w", err)
	}
	return c.redis.SetString(ctx, c.snapshotKey(generation), string(payload), runtimeSnapshotTTL)
}

// Mutate runs one tokenized mutation. The generation is incremented with the
// lease, so a snapshot written by a stale owner is unreachable afterwards.
func (c *RuntimeCache) Mutate(ctx context.Context, change func(context.Context) error) error {
	if c == nil || c.redis == nil || change == nil {
		return fmt.Errorf("sms runtime cache is unavailable")
	}
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("create sms runtime mutation token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	result, err := c.redis.EvalString(ctx, beginRuntimeMutationScript,
		[]string{c.mutationKey(), c.generationKey()}, token, int64(runtimeMutationLeaseTTL/time.Millisecond))
	if err != nil {
		return err
	}
	if result != "acquired" {
		return fmt.Errorf("sms runtime mutation returned %q", result)
	}

	if err := change(ctx); err != nil {
		releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_, releaseErr := c.redis.EvalString(releaseContext, releaseRuntimeMutationScript, []string{c.mutationKey()}, token)
		if releaseErr != nil {
			return fmt.Errorf("sms runtime mutation rollback: %w", errors.Join(err, releaseErr))
		}
		return err
	}

	publishContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	result, err = c.redis.EvalString(publishContext, publishRuntimeMutationScript, []string{c.mutationKey()}, token)
	if err != nil {
		return err
	}
	if result != "published" {
		return fmt.Errorf("sms runtime mutation publication returned %q", result)
	}
	return nil
}

// LoadReadiness caches the per-scene readiness value under the current
// generation so a runtime change makes every scene entry unreachable.
func (c *RuntimeCache) LoadReadiness(ctx context.Context, scene string, compute func(context.Context) (VerifyCodeReadiness, error)) (VerifyCodeReadiness, error) {
	if c == nil || c.redis == nil || compute == nil {
		return VerifyCodeReadiness{}, fmt.Errorf("sms readiness cache is unavailable")
	}
	var lease *cachefill.Lease
	defer func() { _ = lease.Release(ctx) }()
	var generation string
	for admission := 0; admission < 25; admission++ {
		var err error
		generation, err = c.currentGeneration(ctx)
		if err != nil {
			return VerifyCodeReadiness{}, err
		}
		if readiness, found, err := c.readReadiness(ctx, generation, scene); err != nil {
			return VerifyCodeReadiness{}, err
		} else if found {
			return readiness, nil
		}
		lease, err = cachefill.Try(ctx, c.redis.UniversalClient(), "sms-readiness"+c.namespace, c.readinessKey(generation, scene))
		if err != nil {
			return VerifyCodeReadiness{}, err
		}
		if lease != nil {
			break
		}
		if err := cachefill.Wait(ctx); err != nil {
			return VerifyCodeReadiness{}, err
		}
	}
	if lease == nil {
		return VerifyCodeReadiness{}, fmt.Errorf("sms readiness rebuild is busy")
	}
	workContext, cancel := lease.WorkContext(ctx)
	defer cancel()
	generation, err := c.currentGeneration(workContext)
	if err != nil {
		return VerifyCodeReadiness{}, err
	}
	if readiness, found, err := c.readReadiness(workContext, generation, scene); err != nil {
		return VerifyCodeReadiness{}, err
	} else if found {
		return readiness, nil
	}
	readiness, err := compute(workContext)
	if err != nil {
		return VerifyCodeReadiness{}, err
	}
	if err := validateReadinessValue(readiness); err != nil {
		return VerifyCodeReadiness{}, err
	}
	payload, err := json.Marshal(readinessPayload{SchemaVersion: runtimeCacheSchemaVersion, Ready: readiness.Ready, TTLMinutes: readiness.TTLMinutes})
	if err != nil {
		return VerifyCodeReadiness{}, fmt.Errorf("encode sms readiness: %w", err)
	}
	if err := c.redis.SetString(workContext, c.readinessKey(generation, scene), string(payload), readinessTTL); err != nil {
		return VerifyCodeReadiness{}, err
	}
	return readiness, nil
}

func (c *RuntimeCache) readReadiness(ctx context.Context, generation, scene string) (VerifyCodeReadiness, bool, error) {
	raw, found, err := c.redis.GetString(ctx, c.readinessKey(generation, scene))
	if err != nil || !found {
		return VerifyCodeReadiness{}, false, err
	}
	readiness, err := decodeReadiness(raw)
	if err != nil {
		return VerifyCodeReadiness{}, false, err
	}
	return readiness, true, nil
}

func decodeReadiness(raw string) (VerifyCodeReadiness, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return VerifyCodeReadiness{}, err
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var payload readinessPayload
	if err := decoder.Decode(&payload); err != nil {
		return VerifyCodeReadiness{}, fmt.Errorf("decode sms readiness: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return VerifyCodeReadiness{}, fmt.Errorf("sms readiness contains trailing data")
	}
	if payload.SchemaVersion != runtimeCacheSchemaVersion {
		return VerifyCodeReadiness{}, fmt.Errorf("sms readiness schema is unsupported")
	}
	readiness := VerifyCodeReadiness{Ready: payload.Ready, TTLMinutes: payload.TTLMinutes}
	if err := validateReadinessValue(readiness); err != nil {
		return VerifyCodeReadiness{}, err
	}
	return readiness, nil
}

func validateReadinessValue(readiness VerifyCodeReadiness) error {
	if readiness.Ready && (readiness.TTLMinutes < 1 || readiness.TTLMinutes > 60) {
		return fmt.Errorf("sms readiness ttl is invalid")
	}
	if !readiness.Ready && readiness.TTLMinutes != 0 {
		return fmt.Errorf("sms readiness disabled ttl is invalid")
	}
	return nil
}

func decodeRuntimeSnapshot(raw, generation string) (runtimeSnapshot, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return runtimeSnapshot{}, err
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot runtimeSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return runtimeSnapshot{}, fmt.Errorf("decode sms runtime snapshot: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return runtimeSnapshot{}, fmt.Errorf("sms runtime snapshot contains trailing data")
	}
	if snapshot.SchemaVersion != runtimeCacheSchemaVersion || snapshot.Generation != generation {
		return runtimeSnapshot{}, fmt.Errorf("sms runtime snapshot is stale")
	}
	if err := validateRuntimeSnapshot(snapshot); err != nil {
		return runtimeSnapshot{}, err
	}
	return snapshot, nil
}

func validateRuntimeSnapshot(snapshot runtimeSnapshot) error {
	if snapshot.Configured {
		if snapshot.SecretIDCiphertext == "" || snapshot.SecretKeyCiphertext == "" ||
			strings.TrimSpace(snapshot.SDKAppID) == "" || strings.TrimSpace(snapshot.SignName) == "" ||
			strings.TrimSpace(snapshot.Region) == "" || snapshot.TTLMinutes < 1 || snapshot.TTLMinutes > 60 ||
			!yesno.IsValid(yesno.Value(snapshot.IsEnabled)) {
			return fmt.Errorf("sms runtime config is invalid")
		}
	} else if snapshot.SecretIDCiphertext != "" || snapshot.SecretKeyCiphertext != "" || snapshot.SDKAppID != "" ||
		snapshot.SignName != "" || snapshot.Region != "" || snapshot.Endpoint != "" ||
		snapshot.TTLMinutes != 0 || snapshot.IsEnabled != int16(yesno.No) {
		return fmt.Errorf("sms runtime unconfigured state is invalid")
	}

	catalog := template.FixedCatalog()
	if len(snapshot.Templates) != len(catalog) {
		return fmt.Errorf("sms runtime template catalog is incomplete")
	}
	for _, fixed := range catalog {
		row, found := snapshot.Templates[fixed.Scene]
		if !found || strings.TrimSpace(row.Name) == "" || !yesno.IsValid(yesno.Value(row.IsEnabled)) ||
			!sameStrings(row.ParameterKeys, fixed.ParameterKeys) || !validRuntimeVariables(row.ExampleVariables, fixed.ParameterKeys) {
			return fmt.Errorf("sms runtime template %q is invalid", fixed.Scene)
		}
		if yesno.Value(row.IsEnabled) == yesno.Yes && !isDecimal(row.TencentTemplateID) {
			return fmt.Errorf("sms runtime template %q provider id is invalid", fixed.Scene)
		}
	}

	seenRules := make(map[int64]struct{}, len(snapshot.Rules))
	for _, row := range snapshot.Rules {
		if row.ID < 1 || (row.Scope != recipientRule.ScopePhone && row.Scope != recipientRule.ScopePrefix) ||
			(row.Action != recipientRule.ActionAllow && row.Action != recipientRule.ActionDeny) ||
			row.PatternCiphertext == "" || strings.TrimSpace(row.PatternHint) == "" ||
			!yesno.IsValid(yesno.Value(row.IsEnabled)) {
			return fmt.Errorf("sms runtime recipient rule is invalid")
		}
		if _, exists := seenRules[row.ID]; exists {
			return fmt.Errorf("sms runtime recipient rule is duplicated")
		}
		seenRules[row.ID] = struct{}{}
	}
	return nil
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validRuntimeVariables(values map[string]string, keys []string) bool {
	if len(values) != len(keys) {
		return false
	}
	for _, key := range keys {
		if strings.TrimSpace(values[key]) == "" {
			return false
		}
	}
	return true
}

func isDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func rejectDuplicateJSONKeys(raw string) error {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("sms runtime snapshot contains trailing data")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("sms runtime snapshot key is invalid")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("sms runtime snapshot contains duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected sms runtime snapshot delimiter %q", delimiter)
	}
	_, err = decoder.Token()
	return err
}

func factsOf(snapshot runtimeSnapshot) RuntimeFacts {
	facts := RuntimeFacts{
		Configured: snapshot.Configured,
		Config: ConfiguredConfig{
			SecretIDCiphertext: snapshot.SecretIDCiphertext, SecretKeyCiphertext: snapshot.SecretKeyCiphertext,
			SDKAppID: snapshot.SDKAppID, SignName: snapshot.SignName,
			Region: snapshot.Region, Endpoint: snapshot.Endpoint,
			TTLMinutes: snapshot.TTLMinutes, IsEnabled: yesno.Value(snapshot.IsEnabled),
		},
		Templates: make(map[string]TemplateFact, len(snapshot.Templates)),
		Rules:     make([]RuleFact, 0, len(snapshot.Rules)),
	}
	for scene, row := range snapshot.Templates {
		facts.Templates[scene] = TemplateFact{
			Name: row.Name, TencentTemplateID: row.TencentTemplateID,
			ParameterKeys: row.ParameterKeys, ExampleVariables: row.ExampleVariables,
			IsEnabled: yesno.Value(row.IsEnabled),
		}
	}
	for _, row := range snapshot.Rules {
		facts.Rules = append(facts.Rules, RuleFact{
			ID: row.ID, Scope: row.Scope, Action: row.Action,
			PatternCiphertext: row.PatternCiphertext, PatternHint: row.PatternHint,
			IsEnabled: yesno.Value(row.IsEnabled),
		})
	}
	return facts
}

const beginRuntimeMutationScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then return 'busy' end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('INCR', KEYS[2])
return 'acquired'
`

const releaseRuntimeMutationScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 'stale' end
redis.call('DEL', KEYS[1])
return 'released'
`

const publishRuntimeMutationScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 'stale' end
redis.call('DEL', KEYS[1])
return 'published'
`
