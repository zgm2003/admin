package access

import (
	"errors"
	"testing"

	"admin/server/internal/shared/apperror"
)

func TestAccessUpdatingUsesStableCode(t *testing.T) {
	err := accessUpdating(errors.New("updating"))
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != CodeAccessUpdating {
		t.Fatalf("access updating error = %v", err)
	}
}
