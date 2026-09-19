package realtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	authclient "admin/server/internal/module/auth/client"
	projectredis "admin/server/internal/redis"
)

var ErrTicketUnavailable = errors.New("realtime ticket is unavailable")

type TicketStore struct {
	redis *projectredis.Client
	now   func() time.Time
}

func NewTicketStore(redis *projectredis.Client) *TicketStore {
	return &TicketStore{redis: redis, now: time.Now}
}

func (s *TicketStore) Issue(ctx context.Context, subject TicketSubject) (string, error) {
	now := s.now().UTC()
	if err := validateTicketSubject(subject, now); err != nil {
		return "", err
	}
	ttl := 30 * time.Second
	if remaining := subject.AccessExpiresAt.Sub(now); remaining < ttl {
		ttl = remaining
	}
	if ttl <= 0 {
		return "", ErrTicketUnavailable
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate realtime ticket: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(random)
	payload, err := json.Marshal(subject)
	if err != nil {
		return "", fmt.Errorf("encode realtime ticket: %w", err)
	}
	if err := s.redis.SetString(ctx, ticketKey(raw), string(payload), ttl); err != nil {
		return "", fmt.Errorf("store realtime ticket: %w", err)
	}
	return raw, nil
}

func (s *TicketStore) Consume(ctx context.Context, raw string) (TicketSubject, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		return TicketSubject{}, ErrTicketUnavailable
	}
	payload, found, err := s.redis.GetDeleteString(ctx, ticketKey(raw))
	if err != nil {
		return TicketSubject{}, fmt.Errorf("consume realtime ticket: %w", err)
	}
	if !found {
		return TicketSubject{}, ErrTicketUnavailable
	}
	var subject TicketSubject
	if err := strictDecode([]byte(payload), &subject); err != nil {
		return TicketSubject{}, fmt.Errorf("decode realtime ticket: invalid payload")
	}
	if err := validateTicketSubject(subject, s.now().UTC()); err != nil {
		return TicketSubject{}, ErrTicketUnavailable
	}
	return subject, nil
}

func ticketKey(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return "realtime:ticket:v1:" + hex.EncodeToString(digest[:])
}

func validateTicketSubject(subject TicketSubject, now time.Time) error {
	if subject.SchemaVersion != 1 || subject.PlatformID <= 0 || authclient.ValidatePlatform(subject.PlatformCode) != nil || subject.UserID <= 0 || subject.SessionID <= 0 || subject.SessionVersion <= 0 || subject.IssuedAt.IsZero() || subject.AccessExpiresAt.IsZero() || subject.IssuedAt.After(now.Add(time.Second)) || !subject.AccessExpiresAt.After(now) {
		return errors.New("invalid realtime ticket subject")
	}
	if subject.IssuedAt.Location() != time.UTC || subject.AccessExpiresAt.Location() != time.UTC {
		return errors.New("realtime ticket times must be UTC")
	}
	return nil
}
