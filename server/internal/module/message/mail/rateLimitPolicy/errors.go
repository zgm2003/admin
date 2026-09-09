package ratelimitpolicy

import (
	"errors"
	"fmt"
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"gorm.io/gorm"
)

func invalid(err error) error    { return apperror.InvalidRequest(err) }
func dependency(err error) error { return apperror.DependencyUnavailable(err) }

func wrapRepo(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound(err)
	}
	return dependency(fmt.Errorf("mail rate limit policy repository: %w", err))
}

func notFound(err error) error {
	return &apperror.Error{HTTPStatus: http.StatusNotFound, Code: apperror.CodeNotFound, MessageKey: i18n.KeyMailRateLimitNotFound, Cause: err}
}

func unavailable(err error) error {
	return &apperror.Error{HTTPStatus: http.StatusServiceUnavailable, Code: apperror.CodeDependencyUnavailable, MessageKey: i18n.KeyMailRateLimitUnavailable, Cause: err}
}

func rateLimitNotFound(err error) error    { return notFound(err) }
func rateLimitUnavailable(err error) error { return unavailable(err) }
