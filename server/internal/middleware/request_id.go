package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const RequestIDHeader = "X-Request-ID"

const requestIDKey = "requestID"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func RequestID() gin.HandlerFunc {
	return func(context *gin.Context) {
		requestID := context.GetHeader(RequestIDHeader)
		if !validRequestID.MatchString(requestID) {
			generated, err := newRequestID()
			if err != nil {
				response.Fail(context, apperror.Internal(err))
				return
			}
			requestID = generated
		}

		context.Set(requestIDKey, requestID)
		context.Header(RequestIDHeader, requestID)
		context.Next()
	}
}

func GetRequestID(context *gin.Context) string {
	requestID, _ := context.Get(requestIDKey)
	value, _ := requestID.(string)
	return value
}

func newRequestID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate request ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
