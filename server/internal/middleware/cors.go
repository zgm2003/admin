package middleware

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS(origin string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     []string{origin},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", RequestIDHeader},
		ExposeHeaders:    []string{RequestIDHeader},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
