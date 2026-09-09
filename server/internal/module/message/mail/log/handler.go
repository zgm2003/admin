package log

import (
	"fmt"
	"net/http"
	"strconv"

	"admin/server/internal/authcontext"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

const (
	PermissionDetail = "message:mail:detail"
	PermissionDelete = "message:mail:log:delete"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(ctx *gin.Context) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	if page < 1 || size < 1 || size > 100 {
		response.Fail(ctx, apperror.InvalidRequest(fmt.Errorf("invalid pagination")))
		return
	}
	values, total, err := h.service.List(ctx.Request.Context(), identity.PlatformID, page, size)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, ListResponse(values, total, page, size))
}

func (h *Handler) Get(ctx *gin.Context) {
	identity, id, err := identityAndID(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	value, err := h.service.Get(ctx.Request.Context(), identity.PlatformID, id)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, NewDetailResponse(value))
}

func (h *Handler) Delete(ctx *gin.Context) {
	identity, id, err := identityAndID(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	if err := h.service.Delete(ctx.Request.Context(), identity.PlatformID, id); err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, map[string]any{})
}

func (h *Handler) DeleteMany(ctx *gin.Context) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	var ids []int64
	if err := validate.BindJSON(ctx, &ids); err != nil {
		response.Fail(ctx, err)
		return
	}
	if err := h.service.DeleteMany(ctx.Request.Context(), identity.PlatformID, ids); err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, map[string]any{})
}

func identityAndID(ctx *gin.Context) (authcontext.Identity, int64, error) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		return authcontext.Identity{}, 0, err
	}
	id, err := validate.ParsePositiveInt64(ctx.Param("id"), "id")
	return identity, id, err
}

func requireIdentity(ctx *gin.Context) (authcontext.Identity, error) {
	if identity, found := authcontext.Get(ctx); found && identity.UserID > 0 && identity.PlatformID > 0 {
		return identity, nil
	}
	return authcontext.Identity{}, apperror.Unauthorized(fmt.Errorf("authenticated mail platform is unavailable"))
}
