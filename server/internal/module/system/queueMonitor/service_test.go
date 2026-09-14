package queuemonitor

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordingGrantStore struct {
	key, value string
	ttl        time.Duration
	putErr     error
	getValue   string
	found      bool
	getErr     error
}

func (s *recordingGrantStore) Put(_ context.Context, key, value string, ttl time.Duration) error {
	s.key, s.value, s.ttl = key, value, ttl
	return s.putErr
}

func (s *recordingGrantStore) Get(_ context.Context, _ string) (string, bool, error) {
	return s.getValue, s.found, s.getErr
}

func TestServiceIssueCreatesHashedSixtySecondGrant(t *testing.T) {
	store := &recordingGrantStore{}
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	service := NewService(store)
	service.now = func() time.Time { return now }

	issued, err := service.Issue(context.Background(), Subject{UserID: 11, PlatformID: 22})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	raw, decodeErr := base64.RawURLEncoding.DecodeString(issued.credential)
	if decodeErr != nil || len(raw) != 32 {
		t.Fatalf("credential does not encode 32 bytes: len=%d err=%v", len(raw), decodeErr)
	}
	if store.ttl != GrantTTL {
		t.Fatalf("ttl = %v, want %v", store.ttl, GrantTTL)
	}
	if strings.Contains(store.key, issued.credential) || !strings.HasPrefix(store.key, grantKeyPrefix) {
		t.Fatalf("grant key leaks credential or has wrong prefix: %q", store.key)
	}
	if strings.Contains(store.value, issued.credential) {
		t.Fatal("grant value leaks credential")
	}
	if issued.ExpiresAt != now.Add(GrantTTL) {
		t.Fatalf("expiresAt = %v", issued.ExpiresAt)
	}
}

func TestServiceIssueFailsClosedWhenStoreFails(t *testing.T) {
	service := NewService(&recordingGrantStore{putErr: errors.New("redis unavailable")})
	issued, err := service.Issue(context.Background(), Subject{UserID: 1, PlatformID: 2})
	if err == nil || issued.credential != "" || !issued.ExpiresAt.IsZero() {
		t.Fatalf("Issue() = %+v, %v", issued, err)
	}
}

func TestServiceValidateDistinguishesMissingAndDependencyFailure(t *testing.T) {
	service := NewService(&recordingGrantStore{})
	if _, err := service.Validate(context.Background(), "missing"); !errors.Is(err, ErrGrantInvalid) {
		t.Fatalf("missing error = %v", err)
	}

	service = NewService(&recordingGrantStore{getErr: errors.New("redis unavailable")})
	if _, err := service.Validate(context.Background(), "credential"); err == nil || errors.Is(err, ErrGrantInvalid) {
		t.Fatalf("dependency error = %v", err)
	}
}

func TestServiceValidateRejectsMalformedGrant(t *testing.T) {
	service := NewService(&recordingGrantStore{getValue: `{"userId":0,"platformId":2}`, found: true})
	if _, err := service.Validate(context.Background(), "credential"); !errors.Is(err, ErrGrantInvalid) {
		t.Fatalf("malformed error = %v", err)
	}
}
