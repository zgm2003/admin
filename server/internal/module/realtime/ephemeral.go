package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type RawPublisher interface {
	Publish(context.Context, string, []byte) error
}

// PublishEphemeral sends a transient event directly through Redis Pub/Sub.
// It deliberately bypasses the durable outbox: token deltas must never be
// written as individual realtime events. Clients recover missed deltas by
// fetching a chat snapshot and continuing after its delivery sequence.
func PublishEphemeral(
	ctx context.Context,
	publisher RawPublisher,
	platformID, userID int64,
	eventType string,
	data any,
	occurredAt time.Time,
) error {
	if publisher == nil || platformID < 1 || userID < 1 || occurredAt.IsZero() {
		return errors.New("ephemeral event metadata is invalid")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	envelope := Envelope{
		EventID: uuid.NewString(), Type: eventType, Sequence: 0,
		OccurredAt: occurredAt.UTC(), Durability: DurabilityEphemeral, Data: payload,
	}
	target := userID
	raw, err := EncodePubSubPayload(PubSubPayload{
		SchemaVersion: 1, PlatformID: platformID, TargetType: TargetUser,
		TargetUserID: &target, Envelope: envelope,
	})
	if err != nil {
		return err
	}
	return publisher.Publish(ctx, UserChannel(platformID, userID), raw)
}
