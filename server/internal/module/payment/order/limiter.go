package order

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"admin/server/internal/module/payment"
	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"
)

const (
	channelStartLimit = 40
	channelSlotLimit  = 8
	channelLease      = 6 * time.Second
)

var (
	ErrChannelBudget      = errors.New("payment channel budget exhausted")
	ErrChannelLeaseLost   = errors.New("payment channel lease lost")
	ErrChannelUnavailable = errors.New("payment channel budget unavailable")
)

type RedisChannelLimiter struct{ client redis.UniversalClient }

func NewRedisChannelLimiter(client redis.UniversalClient) *RedisChannelLimiter {
	return &RedisChannelLimiter{client: client}
}

// Admission is atomic across all worker instances. Starts remain charged for
// the one-second window after Release; releasing only frees the in-flight slot.
var reserveChannel = redis.NewScript(`
local clock = redis.call('TIME')
local now = tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
redis.call('ZREMRANGEBYSCORE', KEYS[2], '-inf', now - 1000)
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[2]) then return 0 end
if redis.call('ZCARD', KEYS[2]) >= tonumber(ARGV[3]) then return 0 end
redis.call('ZADD', KEYS[1], now + tonumber(ARGV[1]), ARGV[4])
redis.call('ZADD', KEYS[2], now, ARGV[4])
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[1]) + 1000)
redis.call('PEXPIRE', KEYS[2], 2000)
return 1
`)

var releaseChannel = redis.NewScript(`
if redis.call('ZSCORE', KEYS[1], ARGV[1]) == false then return 0 end
redis.call('ZREM', KEYS[1], ARGV[1])
return 1
`)

func (l *RedisChannelLimiter) Reserve(ctx context.Context, environment payment.Environment, appID string) (func(context.Context) error, error) {
	if ctx == nil || l == nil || l.client == nil || !environment.Valid() || !validChannelAppID(appID) {
		return nil, ErrChannelUnavailable
	}
	slots, starts := channelBudgetKeys(environment, appID)
	value, err := uuid.NewRandom()
	if err != nil {
		return nil, ErrChannelUnavailable
	}
	token := value.String()
	result, err := reserveChannel.Run(ctx, l.client, []string{slots, starts}, channelLease.Milliseconds(), channelSlotLimit, channelStartLimit, token).Int()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrChannelUnavailable, err)
	}
	if result != 1 {
		return nil, ErrChannelBudget
	}
	return func(releaseCtx context.Context) error {
		if releaseCtx == nil {
			return ErrChannelUnavailable
		}
		value, releaseErr := releaseChannel.Run(releaseCtx, l.client, []string{slots}, token).Int()
		if releaseErr != nil {
			return fmt.Errorf("%w: %v", ErrChannelUnavailable, releaseErr)
		}
		if value != 1 {
			return ErrChannelLeaseLost
		}
		return nil
	}, nil
}

func channelBudgetKeys(environment payment.Environment, appID string) (string, string) {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s", environment, appID)))
	prefix := "payment:channel:{" + hex.EncodeToString(digest[:]) + "}"
	return prefix + ":slots", prefix + ":starts"
}

func validChannelAppID(value string) bool {
	if value == "" || len(value) > 64 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e || character == ':' || character == '{' || character == '}' {
			return false
		}
	}
	return true
}
