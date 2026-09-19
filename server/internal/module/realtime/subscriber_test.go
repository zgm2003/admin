package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
)

func TestSubscriberTwoInstancesRouteUserAndPlatformPubSub(t *testing.T) {
	firstRedis := openSubscriberRedis(t)
	secondRedis := openSubscriberRedis(t)
	firstSet := NewConnectionSet(10, 10)
	secondSet := NewConnectionSet(10, 10)
	first := NewSubscriber(firstRedis, firstSet, nil)
	second := NewSubscriber(secondRedis, secondSet, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	startSubscriber(t, ctx, first)
	startSubscriber(t, ctx, second)

	firstUser := newTestConnection(41, 101, 1001)
	firstExcluded := newTestConnection(41, 900, 1002)
	secondUser := newTestConnection(41, 202, 2001)
	firstUser.PlatformCode = "admin"
	firstExcluded.PlatformCode = "admin"
	secondUser.PlatformCode = "admin"
	if err := firstSet.Attach(firstUser); err != nil {
		t.Fatal(err)
	}
	if err := firstSet.Attach(firstExcluded); err != nil {
		t.Fatal(err)
	}
	if err := secondSet.Attach(secondUser); err != nil {
		t.Fatal(err)
	}
	if err := first.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err := second.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	userPayload := subscriberPayload(t, TargetUser, 41, 101, 0)
	if err := firstRedis.Publish(ctx, UserChannel(41, 101), userPayload); err != nil {
		t.Fatal(err)
	}
	expectPayloadEventually(t, firstUser, userPayload)
	expectNoPayload(t, firstExcluded)
	expectNoPayload(t, secondUser)

	platformPayload := subscriberPayload(t, TargetPlatform, 41, 0, 500)
	if err := firstRedis.Publish(ctx, PlatformChannel(41), platformPayload); err != nil {
		t.Fatal(err)
	}
	expectPayloadEventually(t, firstUser, platformPayload)
	expectNoPayload(t, firstExcluded)
	expectPayloadEventually(t, secondUser, platformPayload)

	firstSet.Detach(firstUser)
	if err := first.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	counts, err := firstRedis.UniversalClient().PubSubNumSub(ctx, UserChannel(41, 101)).Result()
	if err != nil {
		t.Fatal(err)
	}
	if counts[UserChannel(41, 101)] != 0 {
		t.Fatalf("user channel subscribers=%d", counts[UserChannel(41, 101)])
	}
	if err := firstRedis.Publish(ctx, UserChannel(41, 101), userPayload); err != nil {
		t.Fatal(err)
	}
	expectNoPayload(t, firstUser)
}

func TestSubscriberRejectsCorruptPayloadAndAppliesAuthInvalidation(t *testing.T) {
	client := openSubscriberRedis(t)
	set := NewConnectionSet(10, 10)
	errors := make(chan string, 2)
	subscriber := NewSubscriber(client, set, func(errorClass string) { errors <- errorClass })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	startSubscriber(t, ctx, subscriber)
	connection := newTestConnection(51, 303, 3001)
	if err := set.Attach(connection); err != nil {
		t.Fatal(err)
	}
	if err := subscriber.SetDesiredSubscriptions(ctx, DesiredSubscriptions{Users: []UserSubscription{{PlatformID: 51, UserID: 303}}, Platforms: []int64{51}, PlatformCodes: map[string]int64{"admin": 51}}); err != nil {
		t.Fatal(err)
	}
	if err := client.Publish(ctx, UserChannel(51, 303), []byte(`{"schemaVersion":1,"unknown":true}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case errorClass := <-errors:
		if errorClass != "invalid_pubsub_payload" {
			t.Fatalf("errorClass=%q", errorClass)
		}
	case <-time.After(time.Second):
		t.Fatal("missing corrupt payload error class")
	}
	expectNoPayload(t, connection)

	invalidation, err := json.Marshal(map[string]any{"schemaVersion": 1, "targetType": "platformUser", "platformCode": "admin", "userId": 303})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Publish(ctx, AuthInvalidationChannel, invalidation); err != nil {
		t.Fatal(err)
	}
	waitClosed(t, connection)
}

func TestSubscriberDisconnectClosesAllAndRestoresCurrentSubscriptions(t *testing.T) {
	client := openSubscriberRedis(t)
	set := NewConnectionSet(10, 10)
	subscriber := NewSubscriber(client, set, nil)
	firstSession := newFakePubSub()
	secondSession := newFakePubSub()
	opened := make(chan int, 2)
	var openCount int
	subscriber.open = func(context.Context, ...string) pubSubConnection {
		openCount++
		opened <- openCount
		if openCount == 1 {
			return firstSession
		}
		return secondSession
	}
	connection := newTestConnection(61, 404, 4001)
	connection.PlatformCode = "admin"
	if err := set.Attach(connection); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	startSubscriber(t, ctx, subscriber)
	if err := subscriber.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	firstSession.fail(errors.New("connection lost"))
	waitClosed(t, connection)

	replacement := newTestConnection(61, 404, 4002)
	replacement.PlatformCode = "admin"
	if err := set.Attach(replacement); err != nil {
		t.Fatal(err)
	}
	select {
	case count := <-opened:
		if count != 1 {
			t.Fatalf("first open count=%d", count)
		}
	default:
	}
	select {
	case count := <-opened:
		if count != 2 {
			t.Fatalf("second open count=%d", count)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not reconnect")
	}
	if err := subscriber.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	payload := subscriberPayload(t, TargetUser, 61, 404, 0)
	secondSession.message(UserChannel(61, 404), payload)
	expectPayloadEventually(t, replacement, payload)
}

func startSubscriber(t *testing.T, ctx context.Context, subscriber *Subscriber) {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- subscriber.Run(ctx) }()
	select {
	case <-subscriber.Ready():
	case err := <-result:
		t.Fatalf("subscriber exited before ready: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber did not become ready")
	}
}

func subscriberPayload(t *testing.T, target TargetType, platformID, userID, audienceMaxUserID int64) []byte {
	t.Helper()
	envelope := Envelope{EventID: uuid.NewString(), Type: EventNotificationCreated, Sequence: 1, OccurredAt: time.Now().UTC(), Durability: DurabilityDurable, Data: json.RawMessage(`{}`)}
	payload := PubSubPayload{SchemaVersion: 1, PlatformID: platformID, TargetType: target, Envelope: envelope}
	if target == TargetUser {
		payload.TargetUserID = &userID
	} else {
		payload.AudienceMaxUserID = &audienceMaxUserID
	}
	raw, err := EncodePubSubPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func expectPayloadEventually(t *testing.T, connection *Connection, want []byte) {
	t.Helper()
	select {
	case got := <-connection.Send():
		if string(got) != string(want) {
			t.Fatalf("payload=%s want=%s", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscriber payload")
	}
}

func waitClosed(t *testing.T, connection *Connection) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if connection.Closed() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("connection was not closed")
		case <-ticker.C:
		}
	}
}

func openSubscriberRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	client, err := projectredis.Open(context.Background(), settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

type fakePubSub struct {
	messages chan pubSubReceive
	first    bool
}

func newFakePubSub() *fakePubSub {
	return &fakePubSub{messages: make(chan pubSubReceive, 16), first: true}
}

func (f *fakePubSub) Receive(ctx context.Context) (any, error) {
	if f.first {
		f.first = false
		return &goredis.Subscription{Kind: "subscribe", Channel: AuthInvalidationChannel, Count: 1}, nil
	}
	select {
	case result := <-f.messages:
		return result.value, result.err
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	}
}

func (f *fakePubSub) Subscribe(_ context.Context, channels ...string) error {
	for _, channel := range channels {
		f.messages <- pubSubReceive{value: &goredis.Subscription{Kind: "subscribe", Channel: channel, Count: 1}}
	}
	return nil
}

func (f *fakePubSub) Unsubscribe(_ context.Context, channels ...string) error {
	for _, channel := range channels {
		f.messages <- pubSubReceive{value: &goredis.Subscription{Kind: "unsubscribe", Channel: channel}}
	}
	return nil
}

func (f *fakePubSub) Close() error { return nil }

func (f *fakePubSub) fail(err error) {
	f.messages <- pubSubReceive{err: err}
}

func (f *fakePubSub) message(channel string, payload []byte) {
	f.messages <- pubSubReceive{value: &goredis.Message{Channel: channel, Payload: string(payload)}}
}
