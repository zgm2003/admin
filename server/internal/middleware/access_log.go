package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(context *gin.Context) {
		started := time.Now()
		context.Next()

		route := context.FullPath()
		if route == "" {
			route = "unmatched"
		}
		logger.InfoContext(
			context.Request.Context(),
			"http request",
			"requestId", GetRequestID(context),
			"method", context.Request.Method,
			"route", route,
			"status", context.Writer.Status(),
			"latencyMs", float64(time.Since(started).Microseconds())/1000,
		)
	}
}
