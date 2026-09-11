package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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

// ProofDigest binds a one-time code to the challenge identifier returned by
// the send-code endpoint. The NUL separator is outside both accepted input
// alphabets, so the encoded tuple is unambiguous without storing either value.
func (s *verificationCodeStore) ProofDigest(challengeID, code string) string {
	mac := hmac.New(sha256.New, s.hmacKey)
	_, _ = mac.Write([]byte(challengeID))
	_, _ = mac.Write([]byte{0})
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
	value, err := decodeVerificationCodeValue(raw)
	if err != nil {
		return false, fmt.Errorf("decode verification code value: %w", err)
	}
	return value.Digest == digest, nil
}

// decodeVerificationCodeValue strictly decodes a stored verification code
// payload. It rejects duplicate JSON keys, unknown fields (including any
// smuggled PII), trailing data, and missing or empty digest/leaseToken so a
// corrupt or hostile value can never be treated as a usable code.
func decodeVerificationCodeValue(raw string) (verificationCodeValue, error) {
	if err := scanVerificationCodeKeys(raw); err != nil {
		return verificationCodeValue{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value verificationCodeValue
	if err := decoder.Decode(&value); err != nil {
		return verificationCodeValue{}, fmt.Errorf("verification code value is invalid: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return verificationCodeValue{}, fmt.Errorf("verification code value contains trailing data")
	}
	if value.Digest == "" || value.LeaseToken == "" {
		return verificationCodeValue{}, fmt.Errorf("verification code value is missing digest or lease token")
	}
	return value, nil
}

// scanVerificationCodeKeys performs a token-level duplicate-key scan over the
// top-level object before structural decoding, so a hostile payload cannot
// smuggle a second key past the decoder.
func scanVerificationCodeKeys(raw string) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("verification code value must be a JSON object")
	}
	seen := map[string]struct{}{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("verification code value key is invalid")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("verification code value contains duplicate key %q", key)
		}
		seen[key] = struct{}{}
		var discard any
		if err := decoder.Decode(&discard); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
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

// ConsumeMany atomically validates every key against its digest and, only when
// all of them match, deletes all of them. A partial match consumes nothing.
func (s *verificationCodeStore) ConsumeMany(ctx context.Context, keys []string, digests []string) (bool, error) {
	if len(keys) == 0 || len(keys) != len(digests) {
		return false, fmt.Errorf("consume many verification codes: invalid argument count")
	}
	args := make([]any, 0, len(digests))
	for _, digest := range digests {
		args = append(args, digest)
	}
	result, err := s.redis.EvalString(ctx, consumeManyVerificationCodeScript, keys, args...)
	if err != nil {
		return false, fmt.Errorf("consume many verification codes: %w", err)
	}
	switch result {
	case "consumed":
		return true, nil
	case "mismatch", "missing":
		return false, nil
	default:
		return false, fmt.Errorf("consume many verification codes: %s", result)
	}
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
local ok, decoded = pcall(cjson.decode, raw)
if not ok or type(decoded) ~= 'table' then return 'corrupt' end
local count = 0
for _ in pairs(decoded) do count = count + 1 end
if count ~= 2 then return 'corrupt' end
if type(decoded.digest) ~= 'string' or #decoded.digest == 0 then return 'corrupt' end
if type(decoded.leaseToken) ~= 'string' or #decoded.leaseToken == 0 then return 'corrupt' end
if decoded.digest == ARGV[1] then
  redis.call('DEL', KEYS[1])
  return 'consumed'
end
return 'mismatch'
`

const consumeManyVerificationCodeScript = `
if #KEYS ~= #ARGV then return 'mismatch' end
for i = 1, #KEYS do
  local raw = redis.call('GET', KEYS[i])
  if not raw then return 'mismatch' end
  local ok, decoded = pcall(cjson.decode, raw)
  if not ok or type(decoded) ~= 'table' then return 'corrupt' end
  local count = 0
  for _ in pairs(decoded) do count = count + 1 end
  if count ~= 2 then return 'corrupt' end
  if type(decoded.digest) ~= 'string' or #decoded.digest == 0 then return 'corrupt' end
  if type(decoded.leaseToken) ~= 'string' or #decoded.leaseToken == 0 then return 'corrupt' end
  if decoded.digest ~= ARGV[i] then return 'mismatch' end
end
for i = 1, #KEYS do redis.call('DEL', KEYS[i]) end
return 'consumed'
`

const checkVerificationCodeAttemptScript = `
local account_attempts = tonumber(redis.call('GET', KEYS[2]) or '0')
local ip_attempts = tonumber(redis.call('GET', KEYS[3]) or '0')
if account_attempts >= tonumber(ARGV[2]) or ip_attempts >= tonumber(ARGV[3]) then
  return 'limited'
end
local raw = redis.call('GET', KEYS[1])
if raw then
  local ok, decoded = pcall(cjson.decode, raw)
  if not ok or type(decoded) ~= 'table' then return 'corrupt' end
  local count = 0
  for _ in pairs(decoded) do count = count + 1 end
  if count ~= 2 then return 'corrupt' end
  if type(decoded.digest) ~= 'string' or #decoded.digest == 0 then return 'corrupt' end
  if type(decoded.leaseToken) ~= 'string' or #decoded.leaseToken == 0 then return 'corrupt' end
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
local ok, decoded = pcall(cjson.decode, raw)
if not ok or type(decoded) ~= 'table' then return 'corrupt' end
local count = 0
for _ in pairs(decoded) do count = count + 1 end
if count ~= 2 then return 'corrupt' end
if type(decoded.digest) ~= 'string' or #decoded.digest == 0 then return 'corrupt' end
if type(decoded.leaseToken) ~= 'string' or #decoded.leaseToken == 0 then return 'corrupt' end
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
