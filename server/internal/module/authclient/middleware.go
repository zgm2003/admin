package authclient

import (
	"fmt"
	"strings"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const clientContextKey = "authclient.client"

func Require() gin.HandlerFunc {
	return func(context *gin.Context) {
		platform, err := exactHeader(context, PlatformHeader)
		if err == nil {
			err = ValidatePlatform(platform)
		}
		deviceID := ""
		if err == nil {
			deviceID, err = exactHeader(context, DeviceIDHeader)
		}
		if err == nil {
			err = ValidateDeviceID(deviceID)
		}
		if err != nil {
			response.Fail(context, apperror.InvalidRequest(err))
			return
		}
		userAgent := context.GetHeader("User-Agent")
		if len(userAgent) > 512 {
			userAgent = userAgent[:512]
		}
		context.Set(clientContextKey, Client{
			Platform: platform, DeviceID: deviceID,
			ClientIP: context.ClientIP(), UserAgent: userAgent,
		})
		context.Next()
	}
}

func FromContext(context *gin.Context) (Client, bool) {
	value, exists := context.Get(clientContextKey)
	if !exists {
		return Client{}, false
	}
	client, ok := value.(Client)
	return client, ok
}

func RequireAdminPlatform() gin.HandlerFunc {
	return func(context *gin.Context) {
		client, ok := FromContext(context)
		if !ok {
			response.Fail(context, apperror.Internal(fmt.Errorf("authentication client context is missing")))
			return
		}
		if client.Platform != "admin" {
			response.Fail(context, apperror.Forbidden(fmt.Errorf("authentication platform cannot access admin API")))
			return
		}
		context.Next()
	}
}

func exactHeader(context *gin.Context, name string) (string, error) {
	values := make([]string, 0, 1)
	for key, entries := range context.Request.Header {
		if strings.EqualFold(key, name) {
			values = append(values, entries...)
		}
	}
	if len(values) != 1 || values[0] == "" || values[0] != strings.TrimSpace(values[0]) {
		return "", fmt.Errorf("%s must have exactly one canonical value", name)
	}
	return values[0], nil
}
