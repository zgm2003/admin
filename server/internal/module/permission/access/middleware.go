package permission

import (
	"context"
	"fmt"
	"strings"

	projectmiddleware "admin/server/internal/middleware"
	"admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type permissionService interface {
	Allowed(context.Context, auth.Identity, string) (bool, error)
}

func RequirePermission(service permissionService, permissionCode string) gin.HandlerFunc {
	if strings.TrimSpace(permissionCode) == "" {
		panic("permission code is required")
	}
	return func(context *gin.Context) {
		identity, ok := auth.IdentityFromContext(context)
		if !ok {
			response.Fail(context, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
			return
		}
		allowed, err := service.Allowed(context.Request.Context(), identity, permissionCode)
		if err != nil {
			response.Fail(context, err)
			return
		}
		if !allowed {
			response.Fail(context, apperror.ForbiddenWithParams(
				i18n.KeyPermissionDenied,
				map[string]string{"permission": permissionCode},
				fmt.Errorf("permission %s is required", permissionCode),
			))
			return
		}
		projectmiddleware.SetCacheLog(context, "access", "checked", 0)
		context.Next()
	}
}
