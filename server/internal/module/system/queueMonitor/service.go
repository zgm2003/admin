package queuemonitor

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type Service struct {
	store grantStore
	now   func() time.Time
}

func NewService(store grantStore) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Issue(ctx context.Context, subject Subject) (IssuedGrant, error) {
	if s == nil || s.store == nil || subject.UserID < 1 || subject.PlatformID < 1 {
		return IssuedGrant{}, fmt.Errorf("queue monitor grant dependencies or subject are unavailable")
	}
	credential, err := newCredential()
	if err != nil {
		return IssuedGrant{}, fmt.Errorf("generate queue monitor grant: %w", err)
	}
	now := s.now().UTC()
	payload, err := json.Marshal(GrantRecord{
		UserID: subject.UserID, PlatformID: subject.PlatformID, PermissionCode: PermissionList, IssuedAt: now,
	})
	if err != nil {
		return IssuedGrant{}, fmt.Errorf("encode queue monitor grant: %w", err)
	}
	if err := s.store.Put(ctx, grantKey(credential), string(payload), GrantTTL); err != nil {
		return IssuedGrant{}, fmt.Errorf("store queue monitor grant: %w", err)
	}
	return IssuedGrant{credential: credential, ExpiresAt: now.Add(GrantTTL)}, nil
}

func (s *Service) Validate(ctx context.Context, credential string) (GrantRecord, error) {
	if s == nil || s.store == nil || strings.TrimSpace(credential) == "" {
		return GrantRecord{}, ErrGrantInvalid
	}
	payload, found, err := s.store.Get(ctx, grantKey(credential))
	if err != nil {
		return GrantRecord{}, err
	}
	if !found || payload == "" {
		return GrantRecord{}, ErrGrantInvalid
	}
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	var record GrantRecord
	if err := decoder.Decode(&record); err != nil || record.UserID < 1 || record.PlatformID < 1 || record.PermissionCode != PermissionList || record.IssuedAt.IsZero() {
		return GrantRecord{}, ErrGrantInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return GrantRecord{}, ErrGrantInvalid
	}
	return record, nil
}

func newCredential() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
