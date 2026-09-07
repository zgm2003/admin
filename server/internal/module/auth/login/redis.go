package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	projectredis "admin/server/internal/redis"
)

const (
	verificationCodeKeyPrefix   = "auth:verify-code:v2:"
	verificationCodeLeaseSuffix = ":delivery"
	verificationCodeMinimumTTL  = time.Minute
	verificationCodeMaximumTTL  = 60 * time.Minute
	verificationAttemptWindow   = 10 * time.Minute
	verificationAccountAttempts = 10
	verificationIPAttempts      = 30
)

// verificationCodeValue is the Redis-stored payload for a pending code. It
// never contains the code itself, only its HMAC digest and the delivery lease
// token that owns it.
type verificationCodeValue struct {
	Digest     string `json:"digest"`
	LeaseToken string `json:"leaseToken"`
}

type verificationCodeStore struct {
	redis   *projectredis.Client
	hmacKey []byte
}

// NewVerificationCodeStore builds the Auth-owned verification code store. The
// HMAC key is used to derive the account identifier that becomes part of the
// Redis key, so no email or phone number is stored in plaintext.
func NewVerificationCodeStore(redis *projectredis.Client, hmacKey []byte) VerificationCodeStore {
	return &verificationCodeStore{redis: redis, hmacKey: append([]byte(nil), hmacKey...)}
}

// VerificationKey derives the PII-free code key from platform, scene, login
// type and an HMAC of the normalized account.
func (s *verificationCodeStore) VerificationKey(platform, scene, loginType, account string) string {
	mac := hmac.New(sha256.New, s.hmacKey)
	_, _ = mac.Write([]byte(account))
	return verificationCodeKeyPrefix + platform + ":" + scene + ":" + loginType + ":" + hex.EncodeToString(mac.Sum(nil))
}

// Digest computes the HMAC of a plaintext code so the code itself is never
// written to Redis.
func (s *verificationCodeStore) Digest(code string) string {
	mac := hmac.New(sha256.New, s.hmacKey)
	_, _ = mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *verificationCodeStore) leaseKey(key string) string {
	return key + verificationCodeLeaseSuffix
}

func (s *verificationCodeStore) AcquireDelivery(ctx context.Context, key, leaseToken string, ttl time.Duration) (bool, error) {
	acquired, err := s.redis.SetStringIfMissing(ctx, s.leaseKey(key), leaseToken, ttl)
	if err != nil {
		return false, fmt.Errorf("acquire verification code delivery lease: %w", err)
	}
	return acquired, nil
}

func (s *verificationCodeStore) ReleaseDelivery(ctx context.Context, key, leaseToken string) error {
	result, err := s.redis.EvalString(ctx, releaseDeliveryScript, []string{s.leaseKey(key)}, leaseToken)
	if err != nil {
		return fmt.Errorf("release verification code delivery lease: %w", err)
	}
	if result != "deleted" && result != "missing" {
		return fmt.Errorf("release verification code delivery lease: %s", result)
	}
	return nil
}

func (s *verificationCodeStore) Put(ctx context.Context, key, digest, leaseToken string, ttl time.Duration) error {
	if ttl < verificationCodeMinimumTTL || ttl > verificationCodeMaximumTTL {
		return fmt.Errorf("verification code TTL must be between 1 and 60 minutes")
	}
	payload, err := json.Marshal(verificationCodeValue{Digest: digest, LeaseToken: leaseToken})
	if err != nil {
		return fmt.Errorf("encode verification code value: %w", err)
	}
	result, err := s.redis.EvalString(ctx, putVerificationCodeScript, []string{key, s.leaseKey(key)}, string(payload), leaseToken, int64(ttl/time.Millisecond))
	if err != nil {
		return fmt.Errorf("put verification code: %w", err)
	}
	if result != "stored" {
		return fmt.Errorf("put verification code: %s", result)
	}
	return nil
}

func (s *verificationCodeStore) Check(ctx context.Context, key, digest string) (bool, error) {
	raw, found, err := s.redis.GetString(ctx, key)
	if err != nil {
		return false, fmt.Errorf("read verification code: %w", err)
	}
	if !found {
		return false, nil
	}
	var value verificationCodeValue
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return false, fmt.Errorf("decode verification code value: %w", err)
	}
	return value.Digest == digest, nil
}

func (s *verificationCodeStore) CheckAttempt(ctx context.Context, key, digest, clientIP string) (bool, bool, error) {
	accountAttemptsKey := key + ":attempts"
	mac := hmac.New(sha256.New, s.hmacKey)
	_, _ = mac.Write([]byte(clientIP))
	ipAttemptsKey := verificationCodeKeyPrefix + "attempt-ip:" + hex.EncodeToString(mac.Sum(nil))
	result, err := s.redis.EvalString(ctx, checkVerificationCodeAttemptScript,
		[]string{key, accountAttemptsKey, ipAttemptsKey}, digest,
		verificationAccountAttempts, verificationIPAttempts, int64(verificationAttemptWindow/time.Millisecond))
	if err != nil {
		return false, false, fmt.Errorf("check verification code attempt: %w", err)
	}
	switch result {
	case "valid":
		return true, false, nil
	case "mismatch":
		return false, false, nil
	case "limited":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("check verification code attempt: %s", result)
	}
}

func (s *verificationCodeStore) Consume(ctx context.Context, key, digest string) (bool, error) {
	result, err := s.redis.EvalString(ctx, consumeVerificationCodeScript, []string{key}, digest)
	if err != nil {
		return false, fmt.Errorf("consume verification code: %w", err)
	}
	if result == "consumed" {
		return true, nil
	}
	if result == "mismatch" || result == "missing" {
		return false, nil
	}
	return false, fmt.Errorf("consume verification code: %s", result)
}

func (s *verificationCodeStore) DeleteIfOwned(ctx context.Context, key, leaseToken string) error {
	result, err := s.redis.EvalString(ctx, deleteIfOwnedScript, []string{key}, leaseToken)
	if err != nil {
		return fmt.Errorf("delete verification code: %w", err)
	}
	if result != "deleted" && result != "missing" {
		return fmt.Errorf("delete verification code: %s", result)
	}
	return nil
}

const consumeVerificationCodeScript = `
local raw = redis.call('GET', KEYS[1])
if not raw then return 'missing' end
local decoded = cjson.decode(raw)
if decoded.digest == ARGV[1] then
  redis.call('DEL', KEYS[1])
  return 'consumed'
end
return 'mismatch'
`

const checkVerificationCodeAttemptScript = `
local account_attempts = tonumber(redis.call('GET', KEYS[2]) or '0')
local ip_attempts = tonumber(redis.call('GET', KEYS[3]) or '0')
if account_attempts >= tonumber(ARGV[2]) or ip_attempts >= tonumber(ARGV[3]) then
  return 'limited'
end
local raw = redis.call('GET', KEYS[1])
if raw then
  local decoded = cjson.decode(raw)
  if decoded.digest == ARGV[1] then return 'valid' end
end
account_attempts = redis.call('INCR', KEYS[2])
if account_attempts == 1 then redis.call('PEXPIRE', KEYS[2], ARGV[4]) end
ip_attempts = redis.call('INCR', KEYS[3])
if ip_attempts == 1 then redis.call('PEXPIRE', KEYS[3], ARGV[4]) end
return 'mismatch'
`

const putVerificationCodeScript = `
local lease = redis.call('GET', KEYS[2])
if not lease or lease ~= ARGV[2] then return 'lease-mismatch' end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[3])
return 'stored'
`

const deleteIfOwnedScript = `
local raw = redis.call('GET', KEYS[1])
if not raw then return 'missing' end
local decoded = cjson.decode(raw)
if decoded.leaseToken ~= ARGV[1] then return 'token-mismatch' end
redis.call('DEL', KEYS[1])
return 'deleted'
`

const releaseDeliveryScript = `
local lease = redis.call('GET', KEYS[1])
if not lease then return 'missing' end
if lease ~= ARGV[1] then return 'token-mismatch' end
redis.call('DEL', KEYS[1])
return 'deleted'
`
