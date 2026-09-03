package mail

import (
	"errors"
	"testing"

	"admin/server/internal/shared/apperror"
)

func TestErrorSummaryUnwrapsApplicationError(t *testing.T) {
	err := apperror.DependencyUnavailable(errors.New("mail config disabled"))
	if got := errorSummary(err); got != "mail config disabled" {
		t.Fatalf("errorSummary() = %q, want %q", got, "mail config disabled")
	}
}
