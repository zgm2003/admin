package response

import (
	"errors"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"github.com/gin-gonic/gin"
)

type Envelope[T any] struct {
	Code    int    `json:"code"`
	Data    T      `json:"data"`
	Message string `json:"message"`
}

func OK[T any](context *gin.Context, status int, data T) {
	context.JSON(status, Envelope[T]{Code: 0, Data: data, Message: "ok"})
}

func Fail(context *gin.Context, err error) {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		appErr = apperror.Internal(err)
	}
	locale := i18n.LocaleFromContext(context.Request.Context())
	message, translateErr := i18n.Translate(locale, appErr.MessageKey, appErr.Params)
	if translateErr != nil {
		appErr = apperror.Internal(errors.Join(appErr, translateErr))
		message, _ = i18n.Translate(locale, appErr.MessageKey, nil)
	}
	_ = context.Error(appErr)

	context.AbortWithStatusJSON(appErr.HTTPStatus, Envelope[any]{
		Code:    appErr.Code,
		Data:    nil,
		Message: message,
	})
}
