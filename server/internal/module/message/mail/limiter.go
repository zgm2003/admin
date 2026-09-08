package mail

import (
	"context"
	"fmt"
	goredis "github.com/redis/go-redis/v9"
)

type RedisLimiter struct{ client goredis.UniversalClient }

func NewRedisLimiter(client goredis.UniversalClient) *RedisLimiter {
	return &RedisLimiter{client: client}
}

// TAT encoding and epoch match redis_rate/v10 GCRA. Validate every key before
// writing; reject without debit and merge legacy scene debt only once.
var allowMailWindows = goredis.NewScript(`
local clock = redis.call('TIME')
local now = (tonumber(clock[1]) - 1483228800) + tonumber(clock[2]) / 1000000
local index = 1
local windows = {}
local allowed = true
for arg = 1, #ARGV, 3 do
  local rate = tonumber(ARGV[arg])
  local period = tonumber(ARGV[arg + 1])
  local count = tonumber(ARGV[arg + 2])
  local debt = 0
  for key = index, index + count do
    local value = redis.call('GET', KEYS[key])
    if value then
      local tat = tonumber(value)
      if not tat or tat ~= tat or tat == math.huge or tat == -math.huge or tat > now + 4 * 86400 then
        return redis.error_reply('corrupt mail rate limit state')
      end
      debt = debt + math.max(0, tat - now)
    end
  end
  local increment = period / rate
  if debt + increment > period + 0.000001 then allowed = false end
  windows[#windows + 1] = {index, count, debt, increment, period}
  index = index + count + 1
end
local retry = 0
for _, window in ipairs(windows) do
  local index, count, debt, increment, period = unpack(window)
  if allowed then debt = debt + increment end
  retry = math.max(retry, debt + increment - period)
  if debt > 0 then
    redis.call('SET', KEYS[index], string.format('%.6f', now + debt), 'PX', math.ceil(debt * 1000))
  end
  for key = index + 1, index + count do redis.call('DEL', KEYS[key]) end
end
if allowed then return {1, math.ceil(retry)} end
return {0, math.ceil(retry)}
`)

func (l *RedisLimiter) Allow(ctx context.Context, requests ...LimitRequest) (bool, error) {
	result, err := l.Reserve(ctx, requests...)
	return result.Allowed, err
}

func (l *RedisLimiter) Reserve(ctx context.Context, requests ...LimitRequest) (LimitResult, error) {
	if l == nil || l.client == nil {
		return LimitResult{}, fmt.Errorf("mail rate limiter unavailable")
	}
	if len(requests) == 0 || len(requests) > 2 {
		return LimitResult{}, fmt.Errorf("invalid mail rate limit windows")
	}
	keys := make([]string, 0, 10)
	args := make([]any, 0, 6)
	seen := make(map[string]bool)
	for _, request := range requests {
		if request.Key == "" || request.Limit < 1 || request.Limit > 100000 || request.Window <= 0 || request.Window.Seconds() > 86400 || len(request.LegacyKeys) > 4 {
			return LimitResult{}, fmt.Errorf("invalid mail rate limit request")
		}
		for _, key := range append([]string{request.Key}, request.LegacyKeys...) {
			if key == "" || seen[key] {
				return LimitResult{}, fmt.Errorf("duplicate or empty mail rate limit key")
			}
			seen[key] = true
			keys = append(keys, "rate:"+key)
		}
		args = append(args, request.Limit, request.Window.Seconds(), len(request.LegacyKeys))
	}
	result, err := allowMailWindows.Run(ctx, l.client, keys, args...).Int64Slice()
	if err != nil {
		return LimitResult{}, fmt.Errorf("redis mail rate limit: %w", err)
	}
	if len(result) != 2 || (result[0] != 0 && result[0] != 1) || result[1] < 0 {
		return LimitResult{}, fmt.Errorf("invalid mail reservation result")
	}
	return LimitResult{Allowed: result[0] == 1, RetryAfterSeconds: int(result[1])}, nil
}
