package dictionary

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapRepositoryErrorMapsUniqueViolation(t *testing.T) {
	err := mapRepositoryError(&pgconn.PgError{Code: "23505"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("mapped error = %v, want ErrConflict", err)
	}
}

func TestMapRepositoryErrorPreservesOtherErrors(t *testing.T) {
	original := errors.New("database unavailable")
	if !errors.Is(mapRepositoryError(original), original) {
		t.Fatal("non-unique repository error was changed")
	}
}
