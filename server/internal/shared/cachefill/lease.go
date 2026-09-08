// Package cachefill bounds distributed cold-cache work. It contains no business
// fallback: an unavailable Redis or a full budget rejects database admission.
package cachefill

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

const WorkTimeout = 4 * time.Second

type Lease struct {
	client            redis.UniversalClient
	key, slots, token string
	deadline          time.Time
}

// The deadline starts before Redis admission, not after a slow cache recheck.
func (l *Lease) WorkContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, l.deadline)
}

var acquire = redis.NewScript(`
local clock = redis.call('TIME')
local now = tonumber(clock[1])*1000 + math.floor(tonumber(clock[2])/1000)
if redis.call('EXISTS',KEYS[1]) == 1 then return 0 end
redis.call('ZREMRANGEBYSCORE',KEYS[2],'-inf',now)
redis.call('ZREMRANGEBYSCORE',KEYS[3],'-inf',now-1000)
if redis.call('ZCARD',KEYS[2]) >= 32 or redis.call('ZCARD',KEYS[3]) >= 128 then return 0 end
redis.call('SET',KEYS[1],ARGV[1],'PX',6000)
redis.call('ZADD',KEYS[2],now+6000,ARGV[1])
redis.call('ZADD',KEYS[3],now,ARGV[1])
redis.call('PEXPIRE',KEYS[2],6000)
redis.call('PEXPIRE',KEYS[3],1000)
return 1
`)
var release = redis.NewScript(`
if redis.call('GET',KEYS[1]) == ARGV[1] then redis.call('DEL',KEYS[1]) end
redis.call('ZREM',KEYS[2],ARGV[1])
return 1
`)

// Try admits at most one owner per target, 32 concurrent owners and 128 starts
// per sliding second across all instances of a scope. Every owner must use a
// WorkTimeout context, shorter than the Redis lease, for all source I/O.
func Try(ctx context.Context, client redis.UniversalClient, scope, target string) (*Lease, error) {
	if client == nil || scope == "" || target == "" {
		return nil, fmt.Errorf("cache fill requires client, scope and target")
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(bytes[:])
	scopeHash := sha256.Sum256([]byte(scope))
	targetHash := sha256.Sum256([]byte(target))
	prefix := fmt.Sprintf("cachefill:{%x}:", scopeHash[:8])
	l := &Lease{client: client, key: fmt.Sprintf("%skey:%x", prefix, targetHash), slots: prefix + "slots", token: token, deadline: time.Now().Add(WorkTimeout)}
	n, err := acquire.Run(ctx, client, []string{l.key, l.slots, prefix + "starts"}, token).Int()
	if err != nil {
		return nil, fmt.Errorf("cache fill admission: %w", err)
	}
	if n == 0 {
		return nil, nil
	}
	if n != 1 {
		return nil, fmt.Errorf("invalid cache fill admission")
	}
	return l, nil
}

// Release is token-checked; a delayed owner cannot delete its successor's lease.
// A bounded detached cleanup is necessary when the caller has been canceled.
func (l *Lease) Release(ctx context.Context) error {
	if l == nil {
		return nil
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 250*time.Millisecond)
	defer cancel()
	return release.Run(cleanup, l.client, []string{l.key, l.slots}, l.token).Err()
}

func Wait(ctx context.Context) error {
	timer := time.NewTimer(20 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
