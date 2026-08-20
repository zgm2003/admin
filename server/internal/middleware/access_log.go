package middleware

import (
	"errors"
	"log/slog"
	"time"

	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
)

const accessLogOperationKey = "access-log-operation"

type accessLogOperation struct {
	operation    string
	actorUserID  int64
	targetUserID int64
}

func SetAccessLogOperation(context *gin.Context, operation string, actorUserID, targetUserID int64) {
	context.Set(accessLogOperationKey, accessLogOperation{operation: operation, actorUserID: actorUserID, targetUserID: targetUserID})
}

func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(context *gin.Context) {
		started := time.Now()
		context.Next()

		route := context.FullPath()
		if route == "" {
			route = "unmatched"
		}

		attributes := []any{
			"requestId", GetRequestID(context),
			"method", context.Request.Method,
			"route", route,
			"status", context.Writer.Status(),
			"latencyMs", float64(time.Since(started).Microseconds()) / 1000,
		}
		operation, hasOperation := context.Get(accessLogOperationKey)
		operationContext, operationValid := operation.(accessLogOperation)
		if hasOperation && operationValid {
			attributes = append(attributes, "operation", operationContext.operation, "actorUserId", operationContext.actorUserID, "targetUserId", operationContext.targetUserID)
		}
		if lastError := context.Errors.Last(); lastError != nil {
			var appErr *apperror.Error
			if errors.As(lastError.Err, &appErr) {
				attributes = append(attributes, "errorCode", appErr.Code)
				if (appErr.HTTPStatus >= 500 || appErr.Code == apperror.CodeDependencyUnavailable || operationValid) && appErr.Cause != nil {
					attributes = append(attributes, "error", appErr.Cause)
				}
			}
		}
		logger.InfoContext(context.Request.Context(), "http request", attributes...)
	}
}
