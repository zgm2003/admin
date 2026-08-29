package uploadrule

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"errors"
)

var ErrConflict = errors.New("upload rule conflict")

func invalid(e error) error    { return apperror.InvalidRequest(e) }
func notFound(e error) error   { return apperror.NotFound(e) }
func conflict(e error) error   { return apperror.Conflict(i18n.KeyConflict, nil, e) }
func dependency(e error) error { return apperror.DependencyUnavailable(e) }
