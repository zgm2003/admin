package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

type sessionAdminRepositoryStub struct {
	targets     []Session
	revokeCalls int
}

func (s *sessionAdminRepositoryStub) ListAdmin(context.Context, AdminSessionQuery, time.Time) ([]AdminSession, int64, error) {
	return nil, 0, nil
}

func (s *sessionAdminRepositoryStub) StatsAdmin(context.Context, time.Time) (AdminSessionStats, error) {
	return AdminSessionStats{}, nil
}

func (s *sessionAdminRepositoryStub) FindAdminRevokeTargets(context.Context, []int64) ([]Session, error) {
	return append([]Session(nil), s.targets...), nil
}

func (s *sessionAdminRepositoryStub) RevokeAdmin(context.Context, []int64, int64, time.Time) (AdminRevokeResult, error) {
	s.revokeCalls++
	return AdminRevokeResult{}, nil
}

func TestRevokeSessionReturnsNotFoundBeforeMutation(t *testing.T) {
	repository := &sessionAdminRepositoryStub{}
	service := &Service{adminSessions: repository}

	_, err := service.RevokeSession(context.Background(), Identity{UserID: 1, SessionID: 10}, 99)
	assertSessionAdminError(t, err, http.StatusNotFound, apperror.CodeNotFound, i18n.KeySessionNotFound)
	if repository.revokeCalls != 0 {
		t.Fatalf("RevokeAdmin() calls = %d, want 0", repository.revokeCalls)
	}
}

func TestRevokeSessionRejectsCurrentSessionBeforeMutation(t *testing.T) {
	repository := &sessionAdminRepositoryStub{targets: []Session{{ID: 10, UserID: 1, Platform: "admin"}}}
	service := &Service{adminSessions: repository}

	_, err := service.RevokeSession(context.Background(), Identity{UserID: 1, SessionID: 10}, 10)
	assertSessionAdminError(t, err, http.StatusForbidden, apperror.CodeForbidden, i18n.KeySessionCurrentProtected)
	if repository.revokeCalls != 0 {
		t.Fatalf("RevokeAdmin() calls = %d, want 0", repository.revokeCalls)
	}
}

func assertSessionAdminError(t *testing.T, err error, status, code int, key i18n.MessageKey) {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want application error", err)
	}
	if appErr.HTTPStatus != status || appErr.Code != code || appErr.MessageKey != key {
		t.Fatalf("error = %+v, want status=%d code=%d key=%s", appErr, status, code, key)
	}
}
