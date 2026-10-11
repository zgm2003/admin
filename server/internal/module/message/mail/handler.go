package mail

import (
	"fmt"
	"net/http"

	"admin/server/internal/authcontext"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

const (
	PermissionView = "message:mail:view"
	PermissionList = "message:mail:list"
	PermissionTest = "message:mail:test"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) PageInit(ctx *gin.Context) {
	result, err := PageInit(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, apperror.DependencyUnavailable(err))
		return
	}
	response.OK(ctx, http.StatusOK, result)
}

func (h *Handler) Options(ctx *gin.Context) {
	result, err := Options(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, apperror.DependencyUnavailable(err))
		return
	}
	response.OK(ctx, http.StatusOK, result)
}

func (h *Handler) Test(ctx *gin.Context) {
	identity, found := authcontext.Get(ctx)
	if !found || identity.UserID < 1 || identity.PlatformID < 1 {
		response.Fail(ctx, apperror.Unauthorized(fmt.Errorf("authenticated mail platform is unavailable")))
		return
	}
	var request AdminTestRequest
	if err := validate.BindJSON(ctx, &request); err != nil {
		response.Fail(ctx, err)
		return
	}
	result, err := h.service.TestForPlatform(ctx.Request.Context(), identity.PlatformID, AdminTestInput{
		AdminUserID: identity.UserID, ClientIP: ctx.ClientIP(), ToEmail: request.ToEmail,
		Scene: request.Scene, Variables: request.Variables,
	})
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, result)
}
