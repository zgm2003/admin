package mail

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	"gorm.io/gorm"
)

type failedSender struct{ err error }

func (s failedSender) Send(context.Context, SendInput) (ProviderSendResult, error) {
	return ProviderSendResult{}, s.err
}

func TestSendPreparedRecordsProviderFailureAndReturnsMailError(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		t.Run(fmt.Sprintf("wrapped=%t", wrapped), func(t *testing.T) {
			db, ctx := openMailServiceDatabase(t)
			keys := configuredMailTestKeys(t, db, ctx)
			providerErr := NewProviderError("FailedOperation.EmailAddrInBlacklist", "recipient blocked; RequestId=request-fixture")
			var sendErr error = providerErr
			if wrapped {
				sendErr = fmt.Errorf("send verification: %w", providerErr)
			}
			service := NewService(NewStores(db), keys, failedSender{sendErr}, nil, nil, nil)
			result, err := service.SendPreparedEmailVerifyCode(ctx, EmailVerifyCodeInput{
				PlatformID: 1, ChallengeID: "failed-send", Scene: SceneLogin,
				ToEmail: "recipient@example.com", Code: "123456", ExpiresAt: time.Now().UTC().Add(4 * time.Minute),
				Preparation: EmailVerifyCodePreparation{TTLMinutes: 5},
			})
			var stored Log
			if queryErr := db.WithContext(ctx).Take(&stored).Error; queryErr != nil {
				t.Fatal(queryErr)
			}
			if stored.Status != StatusFailed || stored.ErrorCode != "FailedOperation.EmailAddrInBlacklist" || stored.ErrorSummary != "recipient blocked; RequestId=request-fixture" || stored.SentAt != nil {
				t.Fatalf("failed log=%+v", stored)
			}
			if result != (EmailVerifyCodeResult{}) || !errors.Is(err, providerErr) {
				t.Fatalf("result=%+v error=%v, want failure with diagnostic chain", result, err)
			}
			assertApplicationError(t, err, 503, 18001)
		})
	}
}

func TestFailPendingRetainsBothProviderAndPersistenceErrors(t *testing.T) {
	db, ctx := openMailRepositoryDatabase(t)
	service := NewService(NewStores(db), nil, nil, nil, nil, nil)
	cause := NewProviderError("InternalError", "provider diagnostic")
	result, err := service.failPending(ctx, 1, 999, cause, time.Now())
	assertApplicationError(t, err, 503, apperror.CodeDependencyUnavailable)
	if result.Status != StatusPending || !errors.Is(err, gorm.ErrRecordNotFound) || !errors.Is(err, cause) {
		t.Fatalf("result=%+v error=%v, want both causes without falsely confirming audit persistence", result, err)
	}
}

func TestFailPendingDoesNotMislabelDatabaseFailureAsProviderFailure(t *testing.T) {
	db, ctx := openMailRepositoryDatabase(t)
	stores := NewStores(db)
	now := time.Now().UTC()
	row, err := stores.Log.CreatePending(ctx, &Log{PlatformID: 1, Scene: SceneLogin, TemplateID: 1, ToEmail: "recipient@example.com", Subject: "test", Status: StatusPending, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("verification database unavailable")
	service := NewService(stores, nil, nil, nil, nil, nil)
	_, err = service.failPending(ctx, 1, row.ID, cause, now)
	assertApplicationError(t, err, 503, apperror.CodeDependencyUnavailable)
	if !errors.Is(err, cause) {
		t.Fatal("non-provider cause was lost")
	}
}
