package middleware

import (
	"fmt"
	"net/http"
	"time"

	"admin/server/internal/shared/origins"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS(origin string) gin.HandlerFunc {
	allowed, err := origins.Parse(origin)
	if err != nil {
		panic(fmt.Sprintf("invalid CORS origin allowlist: %v", err))
	}
	return cors.New(cors.Config{
		AllowOrigins: allowed,
		AllowMethods: []string{
			http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
			http.MethodDelete, http.MethodOptions,
		},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Authorization", "Accept-Language",
			"X-Auth-Platform", "X-Device-ID", RequestIDHeader,
		},
		ExposeHeaders:    []string{"Content-Language", RequestIDHeader},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
