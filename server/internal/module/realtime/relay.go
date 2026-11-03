package realtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

const (
	relayClaimLimit = 200
	relayLeaseTTL   = 30 * time.Second
	relayIdle       = time.Second
	relayMaxBackoff = 5 * time.Minute
)

type relayRepository interface {
	ClaimPending(context.Context, int, string, time.Time, time.Duration) ([]OutboxEvent, error)
	MarkPublished(context.Context, int64, string, time.Time) error
	Reschedule(context.Context, int64, string, string, time.Time, time.Time) error
}

type relayPublisher interface {
	Publish(context.Context, string, []byte) error
}

type relayBudget interface {
	EvalString(context.Context, string, []string, ...any) (string, error)
}

// A single global key and the Redis clock serialize publication across all
// workers. Each member is one actual publication, so the sliding one-second
// window contains at most 16000 members. Budget consumption and PUBLISH are
// atomic; an uncertain Redis reply may cause a later at-least-once redelivery.
const relayBudgetScript = `
local clock = redis.call('TIME')
local now = tonumber(clock[1]) * 1000000 + tonumber(clock[2])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - 1000000)
if redis.call('ZCARD', KEYS[1]) >= 16000 then
  return 'limited'
end
redis.call('ZADD', KEYS[1], now, ARGV[1])
redis.call('PEXPIRE', KEYS[1], 2000)
redis.call('PUBLISH', ARGV[2], ARGV[3])
return 'published'
`

type Relay struct {
	repository relayRepository
	publisher  relayPublisher
	logger     *slog.Logger
	now        func() time.Time
	budgetKey  string
}

func NewRelay(repository relayRepository, publisher relayPublisher, logger *slog.Logger) *Relay {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Relay{repository: repository, publisher: publisher, logger: logger, now: time.Now, budgetKey: "realtime:relay:budget:v1"}
}

func (r *Relay) Run(ctx context.Context) error {
	if r == nil || r.repository == nil || r.publisher == nil {
		return errors.New("realtime relay is not configured")
	}
	for {
		count, err := r.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			r.logger.Error("realtime relay iteration failed", "errorClass", relayErrorClass(err))
		}
		if count > 0 && ctx.Err() == nil {
			continue
		}
		timer := time.NewTimer(relayIdle)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	if r == nil || r.repository == nil || r.publisher == nil {
		return 0, errors.New("realtime relay is not configured")
	}
	if _, ok := r.publisher.(relayBudget); !ok {
		return 0, errors.New("realtime relay Redis publication budget is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	now := r.now().UTC()
	token := uuid.NewString()
	events, err := r.repository.ClaimPending(ctx, relayClaimLimit, token, now, relayLeaseTTL)
	if err != nil {
		return 0, fmt.Errorf("claim realtime outbox: %w", err)
	}
	published := 0
	for _, event := range events {
		if ctx.Err() != nil {
			return published, nil
		}
		if err := r.publish(ctx, event, token, now); err != nil {
			r.logger.Error("realtime outbox remains pending", "outboxId", event.ID, "errorClass", relayErrorClass(err))
			continue
		}
		published++
	}
	return published, nil
}

func (r *Relay) publish(ctx context.Context, outbox OutboxEvent, token string, now time.Time) error {
	event := outbox.Event
	envelope := Envelope{EventID: event.EventID, Type: event.EventType, Sequence: event.Sequence, OccurredAt: event.OccurredAt.UTC(), Durability: DurabilityDurable, Data: event.Payload}
	payload := PubSubPayload{SchemaVersion: 1, PlatformID: event.PlatformID, TargetType: event.TargetType, TargetUserID: event.TargetUserID, AudienceMaxUserID: event.AudienceMaxUserID, Envelope: envelope}
	raw, err := EncodePubSubPayload(payload)
	if err != nil {
		return r.reschedule(ctx, outbox, token, "payload-invalid", now)
	}
	channel := ""
	switch event.TargetType {
	case TargetUser:
		channel = UserChannel(event.PlatformID, *event.TargetUserID)
	case TargetPlatform:
		channel = PlatformChannel(event.PlatformID)
	default:
		return r.reschedule(ctx, outbox, token, "payload-invalid", now)
	}
	if err := r.publishBudgeted(ctx, channel, raw); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return r.reschedule(ctx, outbox, token, relayErrorClass(err), now)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.repository.MarkPublished(ctx, outbox.ID, token, now); err != nil {
		return err
	}
	return nil
}

func (r *Relay) publishBudgeted(ctx context.Context, channel string, raw []byte) error {
	budget, ok := r.publisher.(relayBudget)
	if !ok {
		return errors.New("realtime relay Redis publication budget is required")
	}
	publicationID := uuid.NewString()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		result, err := budget.EvalString(ctx, relayBudgetScript, []string{r.budgetKey}, publicationID, channel, string(raw))
		if err != nil {
			return err
		}
		switch result {
		case "published":
			return nil
		case "limited":
			timer := time.NewTimer(25 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		default:
			return errors.New("invalid realtime Redis publication budget response")
		}
	}
}

func (r *Relay) reschedule(ctx context.Context, outbox OutboxEvent, token, errorClass string, now time.Time) error {
	errorClass = normalizedError(errorClass)
	if err := r.repository.Reschedule(ctx, outbox.ID, token, errorClass, now.Add(relayBackoff(outbox.Attempts)), now); err != nil {
		return err
	}
	return errors.New(errorClass)
}

func relayBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := time.Second
	for step := 1; step < attempts && delay < relayMaxBackoff; step++ {
		delay *= 2
		if delay > relayMaxBackoff {
			return relayMaxBackoff
		}
	}
	return delay
}

func relayErrorClass(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrClaimLost):
		return "claim-lost"
	default:
		return "dependency-unavailable"
	}
}
