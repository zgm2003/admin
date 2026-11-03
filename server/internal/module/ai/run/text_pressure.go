package run

import (
	"context"
	"encoding/json"
	"time"
)

// StreamPressure reads only the shared admission observation. Implementations
// must not start per-stream PostgreSQL scans or use stale local fallbacks.
type StreamPressure interface {
	StreamBacklog(context.Context) (int64, error)
}

const streamPressureScript = `
local raw=redis.call('GET',KEYS[1])
if not raw then return 'invalid' end
local ok,o=pcall(cjson.decode,raw)
if not ok or type(o)~='table' or o.schemaVersion~=1 then return 'invalid' end
for _,field in ipairs({'queued','active','backlog','queuedLimit','activeLimit','observedAtMicros'}) do
 local v=o[field]
 if type(v)~='number' or v<0 or v>9007199254740991 or v%1~=0 then return 'invalid' end
end
local clock=redis.call('TIME')
local now=tonumber(clock[1])*1000000+tonumber(clock[2])
local ttl=redis.call('PTTL',KEYS[1])
if ttl<1 or ttl>2000 or o.observedAtMicros>now or now-o.observedAtMicros>=2000000 or o.backlog>100001 then return 'invalid' end
return cjson.encode({backlog=o.backlog})
`

func (g *Gate) StreamBacklog(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if g == nil || g.redis == nil {
		return 0, gateUnavailable(nil)
	}
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	raw, err := g.redis.EvalString(ctx, streamPressureScript, []string{g.prefix + "capacity"})
	if err != nil {
		return 0, gateUnavailable(err)
	}
	var result struct {
		Backlog *int64 `json:"backlog"`
	}
	if json.Unmarshal([]byte(raw), &result) != nil || result.Backlog == nil || *result.Backlog < 0 || *result.Backlog > 100001 {
		return 0, gateUnavailable(nil)
	}
	return *result.Backlog, nil
}

func (e *TextEngine) SetStreamPressure(reader StreamPressure) { e.pressure = reader }
