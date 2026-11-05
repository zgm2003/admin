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
	relayClaimLimit = 50
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

type Relay struct {
	repository relayRepository
	publisher  relayPublisher
	logger     *slog.Logger
	now        func() time.Time
}

func NewRelay(repository relayRepository, publisher relayPublisher, logger *slog.Logger) *Relay {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Relay{repository: repository, publisher: publisher, logger: logger, now: time.Now}
}

func (r *Relay) Run(ctx context.Context) error {
	if r == nil || r.repository == nil || r.publisher == nil {
		return errors.New("realtime relay is not configured")
	}
	for {
		if _, err := r.RunOnce(ctx); err != nil && ctx.Err() == nil {
			r.logger.Error("realtime relay iteration failed", "errorClass", relayErrorClass(err))
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
	if err := r.publisher.Publish(ctx, channel, raw); err != nil {
		return r.reschedule(ctx, outbox, token, relayErrorClass(err), now)
	}
	if err := r.repository.MarkPublished(ctx, outbox.ID, token, now); err != nil {
		return err
	}
	return nil
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
