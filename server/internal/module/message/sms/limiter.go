package sms

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type RedisLimiter struct{ client goredis.UniversalClient }

func NewRedisLimiter(client goredis.UniversalClient) *RedisLimiter {
	return &RedisLimiter{client: client}
}

// TAT encoding and epoch match redis_rate/v10 GCRA. Every window is validated
// before any is written, so a rejected request never debits either window.
var allowSMSWindows = goredis.NewScript(`
local clock = redis.call('TIME')
local now = (tonumber(clock[1]) - 1483228800) + tonumber(clock[2]) / 1000000
local windows = {}
local allowed = true
for arg = 1, #ARGV, 2 do
  local rate = tonumber(ARGV[arg])
  local period = tonumber(ARGV[arg + 1])
  local debt = 0
  local value = redis.call('GET', KEYS[(arg + 1) / 2])
  if value then
    local tat = tonumber(value)
    if not tat or tat ~= tat or tat == math.huge or tat == -math.huge or tat > now + 4 * 86400 then
      return redis.error_reply('corrupt sms rate limit state')
    end
    debt = math.max(0, tat - now)
  end
  local increment = period / rate
  if debt + increment > period + 0.000001 then allowed = false end
  windows[#windows + 1] = {(arg + 1) / 2, debt, increment, period}
end
local retry = 0
for _, window in ipairs(windows) do
  local index, debt, increment, period = unpack(window)
  if allowed then debt = debt + increment end
  retry = math.max(retry, debt + increment - period)
  if debt > 0 then
    redis.call('SET', KEYS[index], string.format('%.6f', now + debt), 'PX', math.ceil(debt * 1000))
  end
end
if allowed then return {1, math.ceil(retry)} end
return {0, math.ceil(retry)}
`)

// Reserve atomically checks every SMS window. Keys carry the platform id, the
// phone HMAC and the policy key, never a scene or a plaintext phone.
func (l *RedisLimiter) Reserve(ctx context.Context, requests ...LimitRequest) (LimitResult, error) {
	if l == nil || l.client == nil {
		return LimitResult{}, fmt.Errorf("sms rate limiter unavailable")
	}
	if len(requests) == 0 || len(requests) > 2 {
		return LimitResult{}, fmt.Errorf("invalid sms rate limit windows")
	}
	keys := make([]string, 0, len(requests))
	args := make([]any, 0, len(requests)*2)
	seen := make(map[string]bool, len(requests))
	for _, request := range requests {
		if request.Key == "" || request.Limit < 1 || request.Limit > 100000 || request.Window <= 0 || request.Window > 86400*time.Second {
			return LimitResult{}, fmt.Errorf("invalid sms rate limit request")
		}
		if seen[request.Key] {
			return LimitResult{}, fmt.Errorf("duplicate sms rate limit key")
		}
		seen[request.Key] = true
		keys = append(keys, request.Key)
		args = append(args, request.Limit, request.Window.Seconds())
	}
	result, err := allowSMSWindows.Run(ctx, l.client, keys, args...).Int64Slice()
	if err != nil {
		return LimitResult{}, fmt.Errorf("redis sms rate limit: %w", err)
	}
	if len(result) != 2 || (result[0] != 0 && result[0] != 1) || result[1] < 0 {
		return LimitResult{}, fmt.Errorf("invalid sms reservation result")
	}
	return LimitResult{Allowed: result[0] == 1, RetryAfterSeconds: int(result[1])}, nil
}
