package middleware

import (
	"admin/server/internal/module/message/mail"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"fmt"
	"github.com/gin-gonic/gin"
	"time"
)

type RateLimitConfig struct {
	Limiter mail.Limiter
	Limit   int
	Window  time.Duration
	Key     func(*gin.Context) string
}

func RateLimit(config RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.Limiter == nil || config.Key == nil {
			response.Fail(c, apperror.DependencyUnavailable(fmt.Errorf("rate limiter unavailable")))
			c.Abort()
			return
		}
		ok, e := config.Limiter.Allow(c, mail.LimitRequest{Key: config.Key(c), Limit: config.Limit, Window: config.Window})
		if e != nil || !ok {
			response.Fail(c, apperror.RateLimited(fmt.Errorf("mail rate limit unavailable or exceeded")))
			c.Abort()
			return
		}
		c.Next()
	}
}
