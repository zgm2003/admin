package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(context *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			logger.ErrorContext(
				context.Request.Context(),
				"panic recovered",
				"requestId", GetRequestID(context),
				"panic", recovered,
				"stack", string(debug.Stack()),
			)
			response.Fail(context, apperror.Internal(fmt.Errorf("panic: %v", recovered)))
		}()

		context.Next()
	}
}
