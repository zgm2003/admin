package mail

import (
	"context"
	"fmt"
	"github.com/go-redis/redis_rate/v10"
	goredis "github.com/redis/go-redis/v9"
)

type RedisLimiter struct{ limiter *redis_rate.Limiter }

func NewRedisLimiter(client goredis.UniversalClient) *RedisLimiter {
	return &RedisLimiter{limiter: redis_rate.NewLimiter(client)}
}
func (l *RedisLimiter) Allow(ctx context.Context, r LimitRequest) (bool, error) {
	if l == nil || l.limiter == nil {
		return false, fmt.Errorf("mail rate limiter unavailable")
	}
	result, e := l.limiter.Allow(ctx, r.Key, redis_rate.Limit{Rate: r.Limit, Burst: r.Limit, Period: r.Window})
	if e != nil {
		return false, fmt.Errorf("redis mail rate limit: %w", e)
	}
	return result.Allowed > 0, nil
}
