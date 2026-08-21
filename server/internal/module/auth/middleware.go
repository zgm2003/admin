package auth

import (
	"context"
	"fmt"
	"strings"

	projectmiddleware "admin/server/internal/middleware"
	"admin/server/internal/module/authclient"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const identityContextKey = "auth.identity"

type authenticationService interface {
	Register(context.Context, RegisterInput) (Registered, error)
	Login(context.Context, LoginInput) (Credential, error)
	Refresh(context.Context, RefreshInput) (Credential, error)
	Authenticate(context.Context, string, authclient.Client) (Identity, error)
	Logout(context.Context, Identity, authclient.Client) error
	CurrentUser(context.Context, Identity) (user.Current, error)
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

func Authenticate(service authenticationService) gin.HandlerFunc {
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
		projectmiddleware.SetAuthenticationLog(context, identity.Platform, identity.UserID, identity.SessionID)
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
