package mail

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// CodeRecipientDenied marks a mail-specific business rejection where the
// recipient matches a deny rule. It intentionally does not reuse the generic
// RBAC forbidden code so clients can tell the two 403 semantics apart.
const CodeRecipientDenied = 18000

var (
	ErrRecipientDenied = errors.New("mail recipient denied")
	ErrRateLimited     = errors.New("mail rate limited")
)

func invalid(err error) error     { return apperror.InvalidRequest(err) }
func dependency(err error) error  { return apperror.DependencyUnavailable(err) }
func notFound(err error) error    { return apperror.NotFound(err) }
func conflict(err error) error    { return apperror.Conflict(i18n.KeyConflict, nil, err) }
func rateLimited(err error) error { return apperror.RateLimited(err) }

func denied(err error) error {
	return &apperror.Error{
		HTTPStatus: http.StatusForbidden,
		Code:       CodeRecipientDenied,
		MessageKey: i18n.KeyMailRecipientDenied,
		Cause:      err,
	}
}
func providerFailure(err error) error {
	return apperror.DependencyUnavailable(fmt.Errorf("mail provider: %w", err))
}

func rateLimitInvalid(err error) error {
	return &apperror.Error{
		HTTPStatus: http.StatusBadRequest,
		Code:       apperror.CodeInvalidRequest,
		MessageKey: i18n.KeyMailRateLimitInvalid,
		Cause:      err,
	}
}

func rateLimitNotFound(err error) error {
	return &apperror.Error{
		HTTPStatus: http.StatusNotFound,
		Code:       apperror.CodeNotFound,
		MessageKey: i18n.KeyMailRateLimitNotFound,
		Cause:      err,
	}
}

func rateLimitUnavailable(err error) error {
	return &apperror.Error{
		HTTPStatus: http.StatusServiceUnavailable,
		Code:       apperror.CodeDependencyUnavailable,
		MessageKey: i18n.KeyMailRateLimitUnavailable,
		Cause:      err,
	}
}

func wrapRepo(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound(err)
	}
	if isUniqueViolation(err) {
		return conflict(err)
	}
	return dependency(fmt.Errorf("mail repository: %w", err))
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
