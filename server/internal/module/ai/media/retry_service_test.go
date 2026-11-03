package media

import (
	"admin/server/internal/shared/apperror"
	"context"
	"errors"
	"testing"
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
