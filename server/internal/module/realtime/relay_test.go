package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRelayPublishesTargetChannelsAndMarksAfterPublish(t *testing.T) {
	now := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	userID := int64(10)
	maxUserID := int64(500)
	repository := &relayRepositoryStub{events: []OutboxEvent{
		{ID: 1, Event: Event{Sequence: 11, EventID: uuid.NewString(), PlatformID: 2, EventType: EventNotificationCreated, TargetType: TargetUser, TargetUserID: &userID, Payload: json.RawMessage(`{"notificationId":1}`), OccurredAt: now}},
		{ID: 2, Event: Event{Sequence: 12, EventID: uuid.NewString(), PlatformID: 2, EventType: EventNotificationCreated, TargetType: TargetPlatform, AudienceMaxUserID: &maxUserID, Payload: json.RawMessage(`{"notificationId":2}`), OccurredAt: now}},
	}}
	publisher := &relayPublisherStub{}
	relay := NewRelay(repository, publisher, nil)
	relay.now = func() time.Time { return now }
	count, err := relay.RunOnce(context.Background())
	if err != nil || count != 2 {
		t.Fatalf("RunOnce()=%d,%v", count, err)
	}
	if got := publisher.channels; len(got) != 2 || got[0] != UserChannel(2, 10) || got[1] != PlatformChannel(2) {
		t.Fatalf("channels=%v", got)
	}
	if len(repository.marked) != 2 || len(repository.rescheduled) != 0 {
		t.Fatalf("marked=%v rescheduled=%v", repository.marked, repository.rescheduled)
	}
	for _, raw := range publisher.payloads {
		if _, err := DecodePubSubPayload(raw); err != nil {
			t.Fatalf("published invalid payload: %v", err)
		}
	}
}

func TestRelayReschedulesPublishAndPayloadFailuresWithBoundedClasses(t *testing.T) {
	now := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	userID := int64(10)
	repository := &relayRepositoryStub{events: []OutboxEvent{
		{ID: 1, Attempts: 3, Event: Event{Sequence: 11, EventID: uuid.NewString(), PlatformID: 2, EventType: EventNotificationCreated, TargetType: TargetUser, TargetUserID: &userID, Payload: json.RawMessage(`{"secret":"` + strings.Repeat("x", 600) + `"}`), OccurredAt: now}},
		{ID: 2, Event: Event{Sequence: 12, EventID: uuid.NewString(), PlatformID: 2, EventType: EventNotificationCreated, TargetType: TargetUser, TargetUserID: &userID, Payload: json.RawMessage(`not-json`), OccurredAt: now}},
	}}
	publisher := &relayPublisherStub{err: errors.New("redis://user:password@host token=PAYLOAD")}
	relay := NewRelay(repository, publisher, nil)
	relay.now = func() time.Time { return now }
	count, err := relay.RunOnce(context.Background())
	if err != nil || count != 0 {
		t.Fatalf("RunOnce()=%d,%v", count, err)
	}
	if len(repository.rescheduled) != 2 {
		t.Fatalf("rescheduled=%v", repository.rescheduled)
	}
	for _, item := range repository.rescheduled {
		if len(item.errorClass) > 512 || strings.Contains(item.errorClass, "redis://") || strings.Contains(item.errorClass, "PAYLOAD") {
			t.Fatalf("unsafe error class=%q", item.errorClass)
		}
	}
	if repository.rescheduled[0].availableAt.Sub(now) != 4*time.Second {
		t.Fatalf("backoff=%s", repository.rescheduled[0].availableAt.Sub(now))
	}
}

func TestRelayRepeatsPublishWhenMarkFailsAfterPublish(t *testing.T) {
	now := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	userID := int64(10)
	repository := &relayRepositoryStub{
		events:  []OutboxEvent{{ID: 1, Event: Event{Sequence: 11, EventID: uuid.NewString(), PlatformID: 2, EventType: EventNotificationCreated, TargetType: TargetUser, TargetUserID: &userID, Payload: json.RawMessage(`{}`), OccurredAt: now}}},
		markErr: errors.New("database connection lost"),
	}
	publisher := &relayPublisherStub{}
	relay := NewRelay(repository, publisher, nil)
	relay.now = func() time.Time { return now }
	if count, err := relay.RunOnce(context.Background()); err != nil || count != 0 {
		t.Fatalf("first RunOnce()=%d,%v", count, err)
	}
	if len(publisher.payloads) != 1 || len(repository.rescheduled) != 0 {
		t.Fatalf("published=%d rescheduled=%v", len(publisher.payloads), repository.rescheduled)
	}
	repository.markErr = nil
	if count, err := relay.RunOnce(context.Background()); err != nil || count != 1 {
		t.Fatalf("second RunOnce()=%d,%v", count, err)
	}
	if len(publisher.payloads) != 2 || len(repository.marked) != 1 {
		t.Fatalf("published=%d marked=%v", len(publisher.payloads), repository.marked)
	}
}

type relayRepositoryStub struct {
	events      []OutboxEvent
	marked      []int64
	rescheduled []relayReschedule
	markErr     error
}

type relayReschedule struct {
	id          int64
	errorClass  string
	availableAt time.Time
}

func (s *relayRepositoryStub) ClaimPending(context.Context, int, string, time.Time, time.Duration) ([]OutboxEvent, error) {
	return append([]OutboxEvent(nil), s.events...), nil
}
func (s *relayRepositoryStub) MarkPublished(_ context.Context, id int64, _ string, _ time.Time) error {
	if s.markErr != nil {
		return s.markErr
	}
	s.marked = append(s.marked, id)
	return nil
}
func (s *relayRepositoryStub) Reschedule(_ context.Context, id int64, _ string, errorClass string, availableAt, _ time.Time) error {
	s.rescheduled = append(s.rescheduled, relayReschedule{id: id, errorClass: errorClass, availableAt: availableAt})
	return nil
}

type relayPublisherStub struct {
	err      error
	channels []string
	payloads [][]byte
}

func (s *relayPublisherStub) Publish(_ context.Context, channel string, payload []byte) error {
	if s.err != nil {
		return s.err
	}
	s.channels = append(s.channels, channel)
	s.payloads = append(s.payloads, append([]byte(nil), payload...))
	return nil
}
