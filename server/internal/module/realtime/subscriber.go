package realtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	projectredis "admin/server/internal/redis"
	goredis "github.com/redis/go-redis/v9"
)

const AuthInvalidationChannel = "auth:realtime-invalidation:v1"

type UserSubscription struct {
	PlatformID int64
	UserID     int64
}

type DesiredSubscriptions struct {
	Users         []UserSubscription
	Platforms     []int64
	PlatformCodes map[string]int64
}

type subscriptionCommand struct {
	desired DesiredSubscriptions
	done    chan error
}

type pubSubReceive struct {
	value any
	err   error
}

type pubSubConnection interface {
	Receive(context.Context) (any, error)
	Subscribe(context.Context, ...string) error
	Unsubscribe(context.Context, ...string) error
	Close() error
}

type Subscriber struct {
	redis          *projectredis.Client
	connections    *ConnectionSet
	reportError    func(string)
	commands       chan subscriptionCommand
	ready          chan struct{}
	readyOnce      sync.Once
	observedMu     sync.Mutex
	observed       DesiredSubscriptions
	observedSignal chan struct{}
	open           func(context.Context, ...string) pubSubConnection
}

func NewSubscriber(redis *projectredis.Client, connections *ConnectionSet, reportError func(string)) *Subscriber {
	subscriber := &Subscriber{
		redis: redis, connections: connections, reportError: reportError,
		commands: make(chan subscriptionCommand), ready: make(chan struct{}), observedSignal: make(chan struct{}, 1),
	}
	subscriber.open = func(ctx context.Context, channels ...string) pubSubConnection {
		return redis.UniversalClient().Subscribe(ctx, channels...)
	}
	if connections != nil {
		connections.SetSubscriptionObserver(subscriber.observeDesiredSubscriptions)
	}
	return subscriber
}

func UserChannel(platformID, userID int64) string {
	return fmt.Sprintf("realtime:user:v1:%d:%d", platformID, userID)
}

func PlatformChannel(platformID int64) string {
	return fmt.Sprintf("realtime:platform:v1:%d", platformID)
}

func (s *Subscriber) Ready() <-chan struct{} { return s.ready }

func (s *Subscriber) SetDesiredSubscriptions(ctx context.Context, desired DesiredSubscriptions) error {
	normalized, err := normalizeDesiredSubscriptions(desired)
	if err != nil {
		return err
	}
	command := subscriptionCommand{desired: normalized, done: make(chan error, 1)}
	select {
	case s.commands <- command:
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	select {
	case err := <-command.done:
		return err
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (s *Subscriber) Sync(ctx context.Context) error {
	return s.SetDesiredSubscriptions(ctx, s.currentObservedSubscriptions())
}

func (s *Subscriber) Run(ctx context.Context) error {
	if s.redis == nil || s.connections == nil {
		return errors.New("realtime subscriber dependencies are required")
	}
	desired := DesiredSubscriptions{PlatformCodes: map[string]int64{}}
	for {
		pubsub := s.open(ctx, AuthInvalidationChannel)
		if _, err := pubsub.Receive(ctx); err != nil {
			_ = pubsub.Close()
			if ctx.Err() != nil {
				return nil
			}
			s.connections.CloseAll()
			if err := waitSubscriberRetry(ctx); err != nil {
				return nil
			}
			continue
		}
		received := receivePubSub(ctx, pubsub)
		if err := subscribeDesired(ctx, pubsub, DesiredSubscriptions{}, desired, received, func(message *goredis.Message) {
			s.handleMessage(desired, message)
		}); err != nil {
			_ = pubsub.Close()
			s.connections.CloseAll()
			if ctx.Err() != nil {
				return nil
			}
			if err := waitSubscriberRetry(ctx); err != nil {
				return nil
			}
			continue
		}
		s.readyOnce.Do(func() { close(s.ready) })
		for {
			select {
			case <-ctx.Done():
				_ = pubsub.Close()
				return nil
			case command := <-s.commands:
				err := subscribeDesired(ctx, pubsub, desired, command.desired, received, func(message *goredis.Message) {
					s.handleMessage(command.desired, message)
				})
				if err == nil {
					desired = command.desired
				}
				command.done <- err
			case <-s.observedSignal:
				next := s.currentObservedSubscriptions()
				if err := subscribeDesired(ctx, pubsub, desired, next, received, func(message *goredis.Message) {
					s.handleMessage(next, message)
				}); err != nil {
					s.report("subscription_update_failed")
					s.connections.CloseAll()
					_ = pubsub.Close()
					if err := waitSubscriberRetry(ctx); err != nil {
						return nil
					}
					goto reconnect
				}
				desired = next
			case result := <-received:
				if result.err != nil {
					s.connections.CloseAll()
					_ = pubsub.Close()
					if err := waitSubscriberRetry(ctx); err != nil {
						return nil
					}
					goto reconnect
				}
				if message, ok := result.value.(*goredis.Message); ok {
					s.handleMessage(desired, message)
				}
			}
		}
	reconnect:
	}
}

func (s *Subscriber) observeDesiredSubscriptions(desired DesiredSubscriptions) {
	s.observedMu.Lock()
	s.observed = desired
	s.observedMu.Unlock()
	select {
	case s.observedSignal <- struct{}{}:
	default:
	}
}

func (s *Subscriber) currentObservedSubscriptions() DesiredSubscriptions {
	s.observedMu.Lock()
	defer s.observedMu.Unlock()
	return s.observed
}

func subscribeDesired(ctx context.Context, pubsub pubSubConnection, current, next DesiredSubscriptions, received <-chan pubSubReceive, handleMessage func(*goredis.Message)) error {
	currentChannels := desiredChannels(current)
	nextChannels := desiredChannels(next)
	var additions, removals []string
	for channel := range nextChannels {
		if _, exists := currentChannels[channel]; !exists {
			additions = append(additions, channel)
		}
	}
	for channel := range currentChannels {
		if _, exists := nextChannels[channel]; !exists {
			removals = append(removals, channel)
		}
	}
	sort.Strings(additions)
	sort.Strings(removals)
	if len(additions) > 0 {
		if err := pubsub.Subscribe(ctx, additions...); err != nil {
			return fmt.Errorf("subscribe realtime channels: %w", err)
		}
		if err := awaitSubscriptionAcks(ctx, received, "subscribe", additions, handleMessage); err != nil {
			return err
		}
	}
	if len(removals) > 0 {
		if err := pubsub.Unsubscribe(ctx, removals...); err != nil {
			return fmt.Errorf("unsubscribe realtime channels: %w", err)
		}
		if err := awaitSubscriptionAcks(ctx, received, "unsubscribe", removals, handleMessage); err != nil {
			return err
		}
	}
	return nil
}

func receivePubSub(ctx context.Context, pubsub pubSubConnection) <-chan pubSubReceive {
	results := make(chan pubSubReceive, 1)
	go func() {
		defer close(results)
		for {
			value, err := pubsub.Receive(ctx)
			select {
			case results <- pubSubReceive{value: value, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return results
}

func awaitSubscriptionAcks(ctx context.Context, received <-chan pubSubReceive, kind string, channels []string, handleMessage func(*goredis.Message)) error {
	pending := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		pending[channel] = struct{}{}
	}
	for len(pending) > 0 {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case result, open := <-received:
			if !open {
				return errors.New("Redis Pub/Sub receiver closed")
			}
			if result.err != nil {
				return fmt.Errorf("receive Redis Pub/Sub subscription: %w", result.err)
			}
			switch value := result.value.(type) {
			case *goredis.Subscription:
				if value.Kind == kind {
					delete(pending, value.Channel)
				}
			case *goredis.Message:
				handleMessage(value)
			}
		}
	}
	return nil
}

func desiredChannels(desired DesiredSubscriptions) map[string]struct{} {
	result := make(map[string]struct{}, len(desired.Users)+len(desired.Platforms))
	for _, user := range desired.Users {
		result[UserChannel(user.PlatformID, user.UserID)] = struct{}{}
	}
	for _, platformID := range desired.Platforms {
		result[PlatformChannel(platformID)] = struct{}{}
	}
	return result
}

func normalizeDesiredSubscriptions(desired DesiredSubscriptions) (DesiredSubscriptions, error) {
	result := DesiredSubscriptions{Users: append([]UserSubscription(nil), desired.Users...), Platforms: append([]int64(nil), desired.Platforms...), PlatformCodes: make(map[string]int64, len(desired.PlatformCodes))}
	for _, user := range result.Users {
		if user.PlatformID <= 0 || user.UserID <= 0 {
			return DesiredSubscriptions{}, errors.New("invalid realtime user subscription")
		}
	}
	for _, platformID := range result.Platforms {
		if platformID <= 0 {
			return DesiredSubscriptions{}, errors.New("invalid realtime platform subscription")
		}
	}
	for code, id := range desired.PlatformCodes {
		if strings.TrimSpace(code) == "" || id <= 0 {
			return DesiredSubscriptions{}, errors.New("invalid realtime platform mapping")
		}
		result.PlatformCodes[code] = id
	}
	sort.Slice(result.Users, func(i, j int) bool {
		if result.Users[i].PlatformID != result.Users[j].PlatformID {
			return result.Users[i].PlatformID < result.Users[j].PlatformID
		}
		return result.Users[i].UserID < result.Users[j].UserID
	})
	result.Users = compactUsers(result.Users)
	sort.Slice(result.Platforms, func(i, j int) bool { return result.Platforms[i] < result.Platforms[j] })
	result.Platforms = compactInt64(result.Platforms)
	return result, nil
}

func compactUsers(values []UserSubscription) []UserSubscription {
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func compactInt64(values []int64) []int64 {
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func (s *Subscriber) handleMessage(desired DesiredSubscriptions, message *goredis.Message) {
	if message.Channel == AuthInvalidationChannel {
		invalidation, err := decodeAuthInvalidation([]byte(message.Payload))
		if err != nil {
			s.report("invalid_auth_invalidation")
			return
		}
		s.applyInvalidation(desired.PlatformCodes, invalidation)
		return
	}
	payload, err := DecodePubSubPayload([]byte(message.Payload))
	if err != nil {
		s.report("invalid_pubsub_payload")
		return
	}
	switch payload.TargetType {
	case TargetUser:
		if message.Channel != UserChannel(payload.PlatformID, *payload.TargetUserID) {
			s.report("pubsub_channel_mismatch")
			return
		}
		s.connections.PublishUser(payload.PlatformID, *payload.TargetUserID, []byte(message.Payload))
	case TargetPlatform:
		if message.Channel != PlatformChannel(payload.PlatformID) {
			s.report("pubsub_channel_mismatch")
			return
		}
		s.connections.PublishPlatform(payload.PlatformID, *payload.AudienceMaxUserID, []byte(message.Payload))
	}
}

type authInvalidation struct {
	SchemaVersion int    `json:"schemaVersion"`
	TargetType    string `json:"targetType"`
	PlatformCode  string `json:"platformCode,omitempty"`
	UserID        int64  `json:"userId,omitempty"`
}

func decodeAuthInvalidation(raw []byte) (authInvalidation, error) {
	var payload authInvalidation
	if err := strictDecode(raw, &payload); err != nil {
		return authInvalidation{}, err
	}
	if payload.SchemaVersion != 1 {
		return authInvalidation{}, errors.New("invalid auth invalidation schema")
	}
	switch payload.TargetType {
	case "user":
		if payload.UserID <= 0 || payload.PlatformCode != "" {
			return authInvalidation{}, errors.New("invalid user invalidation")
		}
	case "platformUser":
		if payload.PlatformCode == "" || payload.UserID <= 0 {
			return authInvalidation{}, errors.New("invalid platform user invalidation")
		}
	case "platform":
		if payload.PlatformCode == "" || payload.UserID != 0 {
			return authInvalidation{}, errors.New("invalid platform invalidation")
		}
	default:
		return authInvalidation{}, errors.New("invalid auth invalidation target")
	}
	return payload, nil
}

func (s *Subscriber) applyInvalidation(platformCodes map[string]int64, payload authInvalidation) {
	switch payload.TargetType {
	case "user":
		s.connections.CloseUser(payload.UserID)
	case "platformUser":
		if platformID := platformCodes[payload.PlatformCode]; platformID > 0 {
			s.connections.ClosePlatformUser(platformID, payload.UserID)
		}
	case "platform":
		if platformID := platformCodes[payload.PlatformCode]; platformID > 0 {
			s.connections.ClosePlatform(platformID)
		}
	}
}

func (s *Subscriber) report(errorClass string) {
	if s.reportError != nil {
		s.reportError(errorClass)
	}
}

func waitSubscriberRetry(ctx context.Context) error {
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return nil
	}
}
