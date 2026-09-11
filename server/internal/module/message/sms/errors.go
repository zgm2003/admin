package sms

import (
	"errors"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

var (
	ErrConfigurationMissing = errors.New("sms configuration is missing")
	ErrRecipientDenied      = errors.New("sms recipient is denied by rule")
	ErrRateLimited          = errors.New("sms rate limited")
	ErrInvalidScene         = errors.New("sms scene is invalid")
	ErrTemplateDisabled     = errors.New("sms template is disabled")
)

func invalidRequest(err error) error { return apperror.InvalidRequest(err) }

func notFound(err error) error { return apperror.NotFound(err) }

func conflict(err error) error { return apperror.Conflict(i18n.KeyConflict, nil, err) }

func dependency(err error) error { return apperror.DependencyUnavailable(err) }

func rateLimited(err error) error { return apperror.RateLimited(err) }

func forbidden(err error) error { return apperror.Forbidden(err) }
