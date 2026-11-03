package media

import (
	"admin/server/internal/shared/apperror"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestTransferRetryServiceRejectsInvalidIdentityAndMissingDependency(t *testing.T) {
	s := NewTransferRetryService(nil)
	for _, tc := range []struct {
		run, user, platform int64
		code                int
	}{{0, 1, 1, apperror.CodeInvalidRequest}, {1, 0, 1, apperror.CodeUnauthorized}, {1, 1, 1, apperror.CodeDependencyUnavailable}} {
		err := s.Retry(context.Background(), tc.run, tc.user, tc.platform)
		var app *apperror.Error
		if !errors.As(err, &app) || app.Code != tc.code {
			t.Fatalf("err=%v want=%d", err, tc.code)
		}
	}
}

func TestImportTimeoutUsesFrozenDeadlineAndSourceExpiry(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	deadline := now.Add(15 * time.Minute)
	expires := now.Add(4 * time.Minute)
	file := File{TransferStatus: TransferQueued, TransferDeadline: &deadline, SourceExpiresAt: &expires}
	service := &Service{repository: nil}
	got, err := service.importTimeout(file, now)
	if err != nil || got != 4*time.Minute+30*time.Second {
		t.Fatalf("timeout=%s err=%v want=4m30s", got, err)
	}
	file.SourceExpiresAt = nil
	got, err = service.importTimeout(file, now)
	if err != nil || got != 15*time.Minute+30*time.Second {
		t.Fatalf("timeout=%s err=%v want=15m30s", got, err)
	}
	got, err = service.importTimeout(file, deadline.Add(time.Second))
	if err != nil || got != 30*time.Second {
		t.Fatalf("expired timeout=%s err=%v want=30s", got, err)
	}
	if id, ok := mediaFileID(json.RawMessage(`{"mediaFileId":8}`)); !ok || id != 8 {
		t.Fatalf("id=%d ok=%v", id, ok)
	}
}
