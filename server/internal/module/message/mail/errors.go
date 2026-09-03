package mail

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"errors"
	"fmt"
)

var (
	ErrRecipientDenied = errors.New("mail recipient denied")
	ErrRateLimited     = errors.New("mail rate limited")
)

func invalid(err error) error     { return apperror.InvalidRequest(err) }
func dependency(err error) error  { return apperror.DependencyUnavailable(err) }
func notFound(err error) error    { return apperror.NotFound(err) }
func conflict(err error) error    { return apperror.Conflict(i18n.KeyConflict, nil, err) }
func denied(err error) error      { return apperror.Forbidden(err) }
func rateLimited(err error) error { return apperror.RateLimited(err) }
func providerFailure(err error) error {
	return apperror.DependencyUnavailable(fmt.Errorf("mail provider: %w", err))
}
