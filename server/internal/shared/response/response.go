package response

import (
	"errors"

	"admin/server/internal/shared/apperror"
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
	_ = context.Error(appErr)

	context.AbortWithStatusJSON(appErr.HTTPStatus, Envelope[any]{
		Code:    appErr.Code,
		Data:    nil,
		Message: appErr.Message,
	})
}
