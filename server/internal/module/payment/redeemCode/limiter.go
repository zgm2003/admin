package redeemcode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"
)

type RedisLimiter struct{ client redis.UniversalClient }

func NewRedisLimiter(client redis.UniversalClient) *RedisLimiter {
	return &RedisLimiter{client: client}
}

var reserveRedeem = redis.NewScript(`
local now=redis.call('TIME'); local ms=tonumber(now[1])*1000+math.floor(tonumber(now[2])/1000)
redis.call('ZREMRANGEBYSCORE',KEYS[1],'-inf',ms); redis.call('ZREMRANGEBYSCORE',KEYS[2],'-inf',ms-600000)
if redis.call('ZCARD',KEYS[1]) >= 1 then return 0 end
if redis.call('ZCARD',KEYS[2]) >= 10 then return 0 end
redis.call('ZADD',KEYS[1],ms+15000,ARGV[1]); redis.call('ZADD',KEYS[2],ms,ARGV[1]); redis.call('PEXPIRE',KEYS[1],16000); redis.call('PEXPIRE',KEYS[2],601000); return 1`)
var finishRedeem = redis.NewScript(`
local now=redis.call('TIME'); local ms=tonumber(now[1])*1000+math.floor(tonumber(now[2])/1000)
local untilMS=redis.call('ZSCORE',KEYS[1],ARGV[1]); if untilMS==false or tonumber(untilMS)<=ms then return -1 end
redis.call('ZREM',KEYS[1],ARGV[1]); if ARGV[2]=='0' then redis.call('ZREM',KEYS[2],ARGV[1]); end; return 1`)

func (l *RedisLimiter) Reserve(ctx context.Context, userID int64) (func(context.Context, bool) error, error) {
	if l == nil || l.client == nil || userID < 1 {
		return nil, ErrLeaseLost
	}
	token := uuid.NewString()
	d := sha256.Sum256([]byte(fmt.Sprintf("%d", userID)))
	prefix := "payment:redeem:{" + hex.EncodeToString(d[:]) + "}"
	slots, attempts := prefix+":lease", prefix+":attempts"
	n, e := reserveRedeem.Run(ctx, l.client, []string{slots, attempts}, token).Int()
	if e != nil {
		return nil, fmt.Errorf("%w: %v", ErrLeaseLost, e)
	}
	if n != 1 {
		return nil, ErrLimit
	}
	return func(done context.Context, invalid bool) error {
		if done == nil {
			return ErrLeaseLost
		}
		n, e := finishRedeem.Run(done, l.client, []string{slots, attempts}, token, fmt.Sprint(boolInt(invalid))).Int()
		if e != nil {
			return fmt.Errorf("%w: %v", ErrLeaseLost, e)
		}
		if n != 1 {
			return ErrLeaseLost
		}
		return nil
	}, nil
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
