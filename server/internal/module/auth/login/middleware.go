package auth

import (
	"context"
	"fmt"
	"strings"

	"admin/server/internal/authcontext"
	projectmiddleware "admin/server/internal/middleware"
	"admin/server/internal/module/auth/client"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const identityContextKey = "auth.identity"

type authenticator interface {
	Authenticate(context.Context, string, authclient.Client) (Identity, error)
}

func RequireOrigin(allowedOrigin string) gin.HandlerFunc {
	return func(context *gin.Context) {
		if context.GetHeader("Origin") != allowedOrigin {
			response.Fail(context, apperror.Forbidden(fmt.Errorf("request Origin is not allowed")))
			return
		}
		context.Next()
	}
}

func Authenticate(service authenticator) gin.HandlerFunc {
	return func(context *gin.Context) {
		client, ok := authclient.FromContext(context)
		if !ok {
			response.Fail(context, apperror.InvalidRequest(fmt.Errorf("authentication client metadata is missing")))
			return
		}
		header := context.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Fail(context, apperror.Unauthorized(fmt.Errorf("Bearer Token is required")))
			return
		}
		rawToken := strings.TrimPrefix(header, "Bearer ")
		if rawToken == "" || strings.ContainsAny(rawToken, " \t\r\n") {
			response.Fail(context, apperror.Unauthorized(fmt.Errorf("Bearer Token is malformed")))
			return
		}
		identity, err := service.Authenticate(context.Request.Context(), rawToken, client)
		if err != nil {
			response.Fail(context, err)
			return
		}
		context.Set(identityContextKey, identity)
		authcontext.Set(context, authcontext.Identity{UserID: identity.UserID, SessionID: identity.SessionID, PlatformID: identity.PlatformID, Platform: identity.Platform})
		projectmiddleware.SetAuthenticationLog(context, identity.PlatformID, identity.Platform, identity.UserID, identity.SessionID)
		projectmiddleware.SetCacheLog(context, "session", identity.CacheResult, 0)
		context.Next()
	}
}

func IdentityFromContext(context *gin.Context) (Identity, bool) {
	value, found := context.Get(identityContextKey)
	if !found {
		return Identity{}, false
	}
	identity, ok := value.(Identity)
	return identity, ok
}
