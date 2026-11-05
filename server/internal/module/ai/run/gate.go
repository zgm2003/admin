package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

var (
	ErrGateDenied      = errors.New("AI admission rate or capacity exceeded")
	ErrGateUnavailable = errors.New("AI admission gate is unavailable")
)

type GateError struct {
	RetryAfter time.Duration
	Reason     string
	cause      error
}

func (e *GateError) Error() string { return e.Reason }
func (e *GateError) Unwrap() error { return e.cause }

type GateRedis interface {
	EvalString(context.Context, string, []string, ...any) (string, error)
}
type CapacityCounts struct {
	Queued  int64 `json:"queued"`
	Active  int64 `json:"active"`
	Backlog int64 `json:"backlog"`
}

// CountCapacity must use the Run state indexes and cap each count at its
// supplied limit. The gate calls it at most once per second across instances.
type CapacityCounter interface {
	CountCapacity(context.Context, int64, int64) (CapacityCounts, error)
}
type capacityObservation struct {
	SchemaVersion    int   `json:"schemaVersion"`
	Queued           int64 `json:"queued"`
	Active           int64 `json:"active"`
	Backlog          int64 `json:"backlog"`
	QueuedLimit      int64 `json:"queuedLimit"`
	ActiveLimit      int64 `json:"activeLimit"`
	ObservedAtMicros int64 `json:"observedAtMicros"`
}

// GateLimits is deployment capacity, not a business setting. API and Worker
// read it from environment configuration so every instance shares one contract.
type GateLimits struct {
	GlobalRatePerSecond         int64
	GlobalRateBurst             int64
	QueuedBackpressureThreshold int64
	ActiveBackpressureThreshold int64
}

func (l GateLimits) validate() error {
	for _, value := range []int64{l.GlobalRatePerSecond, l.GlobalRateBurst, l.QueuedBackpressureThreshold, l.ActiveBackpressureThreshold} {
		if value < 1 || value > 1<<53-1 {
			return gateUnavailable(fmt.Errorf("AI gate deployment limits are invalid"))
		}
	}
	return nil
}

type Gate struct {
	redis   GateRedis
	counter CapacityCounter
	limits  GateLimits
	prefix  string
}

func NewGate(redis GateRedis, counter CapacityCounter, limits GateLimits) *Gate {
	return &Gate{redis: redis, counter: counter, limits: limits, prefix: "ai:run:gate:{global}:"}
}

// Capacity freshness and both token buckets are evaluated in one Redis
// transaction. A denied request spends neither bucket; platforms share the
// same user key. Redis TIME prevents divergent application clocks.
const gateCheckScript = `
local clock=redis.call('TIME')
local now=tonumber(clock[1])*1000000+tonumber(clock[2])
local raw=redis.call('GET',KEYS[1])
if not raw then return cjson.encode({code='missing'}) end
local valid,o=pcall(cjson.decode,raw)
if not valid or type(o)~='table' or o.schemaVersion~=1 then return cjson.encode({code='invalid'}) end
for _,field in ipairs({'queued','active','backlog','queuedLimit','activeLimit','observedAtMicros'}) do
 local v=o[field]
 if type(v)~='number' or v<0 or v>9007199254740991 or v%1~=0 then return cjson.encode({code='invalid'}) end
end
local ttl=redis.call('PTTL',KEYS[1])
if o.observedAtMicros>now or ttl<1 or ttl>2000 then return cjson.encode({code='invalid'}) end
if now-o.observedAtMicros>=2000000 then return cjson.encode({code='missing'}) end
local queuedLimit,activeLimit=tonumber(ARGV[1]),tonumber(ARGV[2])
if o.queuedLimit<queuedLimit or o.activeLimit<activeLimit then return cjson.encode({code='missing'}) end
if o.queued>=queuedLimit or o.active>=activeLimit or o.backlog>=100000 then return cjson.encode({code='backpressure',retryAfterMs=1000}) end
local function readBucket(key,rate,burst)
 local state=redis.call('HMGET',key,'tokens','at')
 if not state[1] and not state[2] then return burst end
 local tokens,at=tonumber(state[1]),tonumber(state[2])
 if not tokens or not at or tokens~=tokens or at~=at or tokens<0 or tokens>9007199254740991 or at<0 or at>now then return nil end
 return math.min(burst,tokens+(now-at)/1000000*rate)
end
local userRate,userBurst,globalRate,globalBurst=tonumber(ARGV[3]),tonumber(ARGV[4]),tonumber(ARGV[5]),tonumber(ARGV[6])
local userTokens=readBucket(KEYS[2],userRate,userBurst)
local globalTokens=readBucket(KEYS[3],globalRate,globalBurst)
if not userTokens or not globalTokens then return cjson.encode({code='invalid'}) end
if userTokens<1 or globalTokens<1 then
 local retry=math.max(math.max(0,1-userTokens)/userRate,math.max(0,1-globalTokens)/globalRate)
 return cjson.encode({code='limited',retryAfterMs=math.max(1,math.ceil(retry*1000))})
end
local function spend(key,tokens,rate,burst)
 redis.call('HSET',key,'tokens',string.format('%.17g',math.max(0,tokens-1)),'at',string.format('%.0f',now))
 redis.call('EXPIRE',key,string.format('%.0f',math.ceil(burst/rate)+1))
end
spend(KEYS[2],userTokens,userRate,userBurst)
spend(KEYS[3],globalTokens,globalRate,globalBurst)
return cjson.encode({code='ok'})
`

// Keeping this token for a full second (also after failed work) bounds all
// capacity scans, including request-triggered cold fills, to one start/sec.
// Work has an 800ms deadline, shorter than the 1000ms ownership lease.
const capacityClaimScript = `
local clock=redis.call('TIME')
local now=tonumber(clock[1])*1000000+tonumber(clock[2])
local stamp=string.format('%.0f',now)
if not redis.call('SET',KEYS[1],ARGV[1]..':'..stamp,'PX',1000,'NX') then return 'busy' end
return stamp
`
const capacityPublishScript = `
if redis.call('GET',KEYS[1])~=ARGV[1] then return 'lost' end
local clock=redis.call('TIME')
local now=tonumber(clock[1])*1000000+tonumber(clock[2])
local elapsed=now-tonumber(ARGV[2])
local remaining=2000-math.ceil(elapsed/1000)
if elapsed<0 or remaining<1 then return 'lost' end
redis.call('SET',KEYS[2],ARGV[3],'PX',remaining)
return 'published'
`

func gateUnavailable(err error) *GateError {
	return &GateError{Reason: "AI admission gate is unavailable", cause: errors.Join(ErrGateUnavailable, err)}
}

func (g *Gate) Check(ctx context.Context, userID int64, policy Policy) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if g == nil || g.redis == nil || g.counter == nil || userID < 1 {
		return gateUnavailable(nil)
	}
	if err := policy.Validate(); err != nil {
		return gateUnavailable(err)
	}
	if err := g.limits.validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, err := g.redis.EvalString(ctx, gateCheckScript, []string{g.prefix + "capacity", g.prefix + "user:" + strconv.FormatInt(userID, 10), g.prefix + "rate"}, g.limits.QueuedBackpressureThreshold, g.limits.ActiveBackpressureThreshold, policy.UserRatePerSecond, policy.UserRateBurst, g.limits.GlobalRatePerSecond, g.limits.GlobalRateBurst)
		if err != nil {
			return gateUnavailable(err)
		}
		var result struct {
			Code         string `json:"code"`
			RetryAfterMs int64  `json:"retryAfterMs"`
		}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			return gateUnavailable(err)
		}
		switch result.Code {
		case "ok":
			return nil
		case "limited", "backpressure":
			if result.RetryAfterMs < 1 || result.RetryAfterMs > 1000 {
				return gateUnavailable(nil)
			}
			return &GateError{RetryAfter: time.Duration(result.RetryAfterMs) * time.Millisecond, Reason: result.Code, cause: ErrGateDenied}
		case "missing":
			if attempt > 0 {
				return gateUnavailable(nil)
			}
			refreshed, err := g.RefreshCapacity(ctx, policy)
			if err != nil {
				return err
			}
			if !refreshed {
				return gateUnavailable(nil)
			}
		default:
			return gateUnavailable(nil)
		}
	}
	return gateUnavailable(nil)
}

func (g *Gate) RefreshCapacity(ctx context.Context, policy Policy) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if g == nil || g.redis == nil || g.counter == nil {
		return false, gateUnavailable(nil)
	}
	if err := policy.Validate(); err != nil {
		return false, gateUnavailable(err)
	}
	if err := g.limits.validate(); err != nil {
		return false, err
	}
	workCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	token := uuid.NewString()
	stamp, err := g.redis.EvalString(workCtx, capacityClaimScript, []string{g.prefix + "capacity:leader"}, token)
	if err != nil {
		return false, gateUnavailable(err)
	}
	if stamp == "busy" {
		return false, nil
	}
	observedAt, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil || observedAt < 1 {
		return false, gateUnavailable(err)
	}
	counts, err := g.counter.CountCapacity(workCtx, g.limits.QueuedBackpressureThreshold, g.limits.ActiveBackpressureThreshold)
	if err != nil {
		return false, gateUnavailable(err)
	}
	if err := workCtx.Err(); err != nil {
		return false, gateUnavailable(err)
	}
	if counts.Queued < 0 || counts.Active < 0 || counts.Backlog < 0 || counts.Backlog > 100001 || counts.Queued > 1<<53-1 || counts.Active > 1<<53-1 {
		return false, gateUnavailable(nil)
	}
	raw, err := json.Marshal(capacityObservation{SchemaVersion: 1, Queued: counts.Queued, Active: counts.Active, Backlog: counts.Backlog, QueuedLimit: g.limits.QueuedBackpressureThreshold, ActiveLimit: g.limits.ActiveBackpressureThreshold, ObservedAtMicros: observedAt})
	if err != nil {
		return false, gateUnavailable(err)
	}
	result, err := g.redis.EvalString(workCtx, capacityPublishScript, []string{g.prefix + "capacity:leader", g.prefix + "capacity"}, token+":"+stamp, observedAt, string(raw))
	if err != nil {
		return false, gateUnavailable(err)
	}
	if result != "published" {
		return false, gateUnavailable(nil)
	}
	return true, nil
}

// Transient cache/DB stalls must not kill the supervised capacity loop. The
// policy is still read on every tick and a persistent failure is returned to
// the lifecycle owner after the bounded retries.
const (
	capacityReadAttempts = 5
	capacityRetryDelay   = 200 * time.Millisecond
)

func waitCapacityRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// Run is supervised by the lifecycle owner. A read/Redis/scan error is returned
// explicitly; an expired observation then closes admission until recovery.
func (g *Gate) Run(ctx context.Context, policies interface {
	Read(context.Context) (Policy, error)
}) error {
	if policies == nil {
		return fmt.Errorf("capacity policy reader is required")
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		var (
			policy Policy
			err    error
		)
		for attempt := 0; attempt < capacityReadAttempts; attempt++ {
			policy, err = policies.Read(ctx)
			if err == nil {
				break
			}
			if attempt < capacityReadAttempts-1 && !waitCapacityRetry(ctx, capacityRetryDelay) {
				return nil
			}
		}
		if err != nil {
			return err
		}
		for attempt := 0; attempt < capacityReadAttempts; attempt++ {
			if ctx.Err() != nil {
				return nil
			}
			if _, err = g.RefreshCapacity(ctx, policy); err == nil {
				break
			}
			if attempt < capacityReadAttempts-1 && !waitCapacityRetry(ctx, capacityRetryDelay) {
				return nil
			}
		}
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
