package scheduler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapRepositoryErrorMapsLocalizedUniqueViolationToConflict(t *testing.T) {
	localized := &pgconn.PgError{Code: "23505", Message: "重复键违反唯一约束"}
	got := mapRepositoryError(fmt.Errorf("insert failed: %w", localized))

	if !errors.Is(got, ErrConflict) {
		t.Fatalf("mapRepositoryError() = %v, want ErrConflict", got)
	}
}
