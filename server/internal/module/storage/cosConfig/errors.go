package cosconfig

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"errors"
	"fmt"
	"net/http"
)

var ErrNameConflict = errors.New("active COS config name conflicts")

func notFound(err error) error   { return apperror.NotFound(err) }
func conflict(err error) error   { return apperror.Conflict(i18n.KeyConflict, nil, err) }
func dependency(err error) error { return apperror.DependencyUnavailable(err) }
func invalidConfig(msg string) error {
	return &apperror.Error{HTTPStatus: http.StatusBadRequest, Code: apperror.CodeInvalidRequest, MessageKey: i18n.KeyInvalidRequest, Cause: fmt.Errorf("%s", msg)}
}
