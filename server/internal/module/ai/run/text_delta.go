package run

import (
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"time"
	"unicode/utf8"
)

const textDeltaMaxBytes = 8192
const textDeltaInterval = 250 * time.Millisecond

// Backpressure limits both retained memory and durable write frequency. No
// goroutine outlives the request, and no timer emits after lease cancellation.
type textDeltaBatch struct {
	ctx           context.Context
	emit          func(context.Context, string) error
	pending       string
	last          time.Time
	pressure      StreamPressure
	checked       time.Time
	intervalValue time.Duration
	pressureErr   error
	now           func() time.Time
}

func newTextDeltaBatch(ctx context.Context, emit func(context.Context, string) error, pressure StreamPressure) *textDeltaBatch {
	return &textDeltaBatch{ctx: ctx, emit: emit, pressure: pressure, now: time.Now}
}
func (b *textDeltaBatch) interval() (time.Duration, error) {
	now := b.now()
	if b.checked.IsZero() || now.Sub(b.checked) >= time.Second {
		b.checked = now
		if b.pressure == nil {
			b.pressureErr = gateUnavailable(nil)
			return 0, b.pressureErr
		}
		backlog, err := b.pressure.StreamBacklog(b.ctx)
		if err != nil {
			b.pressureErr = err
			return 0, err
		}
		if backlog < 0 || backlog > 100001 {
			b.pressureErr = gateUnavailable(nil)
			return 0, b.pressureErr
		}
		b.pressureErr = nil
		b.intervalValue = textDeltaInterval
		if backlog >= 100000 {
			b.intervalValue = time.Second
		}
	}
	return b.intervalValue, b.pressureErr
}
func (b *textDeltaBatch) Append(text string) error {
	if e := b.ctx.Err(); e != nil {
		return e
	}
	interval, e := b.interval()
	if e != nil {
		return e
	}
	for len(text) > 0 {
		space := textDeltaMaxBytes - len(b.pending)
		if space == 0 {
			if e := b.Flush(); e != nil {
				return e
			}
			space = textDeltaMaxBytes
		}
		take := min(space, len(text))
		if take < len(text) {
			for take > 0 && !utf8.RuneStart(text[take]) {
				take--
			}
		}
		if take == 0 {
			if e := b.Flush(); e != nil {
				return e
			}
			continue
		}
		b.pending += text[:take]
		text = text[take:]
		if len(b.pending) == textDeltaMaxBytes {
			if e := b.Flush(); e != nil {
				return e
			}
		}
	}
	if len(b.pending) > 0 && (b.last.IsZero() || b.now().Sub(b.last) >= interval) {
		return b.Flush()
	}
	return nil
}
func (b *textDeltaBatch) Flush() error {
	if len(b.pending) == 0 {
		return b.ctx.Err()
	}
	if e := b.ctx.Err(); e != nil {
		return e
	}
	for {
		interval, e := b.interval()
		if e != nil {
			return e
		}
		wait := interval - b.now().Sub(b.last)
		if b.last.IsZero() || wait <= 0 {
			break
		}
		timer := time.NewTimer(wait)
		select {
		case <-b.ctx.Done():
			timer.Stop()
			return b.ctx.Err()
		case <-timer.C:
		}
	}
	if e := b.emit(b.ctx, b.pending); e != nil {
		return e
	}
	b.pending = ""
	b.last = b.now()
	return nil
}
func (r *Repository) AppendTextDelta(ctx context.Context, id int64, token, text string, at time.Time) error {
	if len(text) == 0 || len(text) > textDeltaMaxBytes || !utf8.ValidString(text) {
		return ErrInvalidSnapshot
	}
	ctx, c := context.WithTimeout(ctx, 2*time.Second)
	defer c()
	return r.withExecution(ctx, id, token, false, at, func(tx *gorm.DB, row Model) error {
		if row.ExecutionStatus != 1 {
			return ErrRunLeaseLost
		}
		payload, e := json.Marshal(struct {
			Text string `json:"text"`
		}{text})
		if e != nil {
			return e
		}
		_, e = r.events.AppendTx(ctx, tx, id, "textDelta", payload, at)
		return e
	})
}
