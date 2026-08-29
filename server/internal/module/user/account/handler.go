package account

import (
	"context"
	"fmt"
	"net/http"
	"time"

	projectmiddleware "admin/server/internal/middleware"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type userService interface {
	List(context.Context, ListQuery) (pagination.Result[ListItem], error)
	RoleOptions(context.Context) ([]RoleSummary, error)
	Update(context.Context, int64, int64, UpdateInput) (UpdatedProfile, error)
	UpdateStatus(context.Context, int64, int64, yesno.Value) error
	Delete(context.Context, int64, int64) error
	Roles(context.Context, int64) (Roles, error)
	UpdateRoles(context.Context, int64, int64, []int64) (int64, error)
}

type Handler struct {
	service     userService
	actorUserID func(*gin.Context) (int64, bool)
}

func NewHandler(service userService, actorUserID func(*gin.Context) (int64, bool)) *Handler {
	return &Handler{service: service, actorUserID: actorUserID}
}

func (h *Handler) List(context *gin.Context) {
	query, err := parseListQuery(context.Request.URL.Query())
	if err != nil {
		response.Fail(context, err)
		return
	}
	result, err := h.service.List(context.Request.Context(), query)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, userListResponse(result.List, result.Total, result.Page, result.PageSize))
}

func (h *Handler) RoleOptions(context *gin.Context) {
	if len(context.Request.URL.Query()) != 0 {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("role options accepts no query parameters")))
		return
	}
	values, err := h.service.RoleOptions(context.Request.Context())
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, roleOptionsResponse{Roles: roleSummaryResponses(values)})
}

func (h *Handler) Update(context *gin.Context) {
	actor, target, ok := h.mutationIDs(context)
	if !ok {
		return
	}
	var request updateRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(context, err)
		return
	}
	projectmiddleware.SetAccessLogOperation(context, "user.profile.update", actor, target)
	updated, err := h.service.Update(context.Request.Context(), actor, target, input)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, updatedProfileResponse{ID: updated.ID, Username: updated.Username, Phone: updated.Phone, UpdatedAt: updated.UpdatedAt.UTC().Format(time.RFC3339Nano)})
}

func (h *Handler) UpdateStatus(context *gin.Context) {
	actor, target, ok := h.mutationIDs(context)
	if !ok {
		return
	}
	var request statusRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	value, err := request.value()
	if err != nil {
		response.Fail(context, err)
		return
	}
	projectmiddleware.SetAccessLogOperation(context, "user.status.update", actor, target)
	if err := h.service.UpdateStatus(context.Request.Context(), actor, target, value); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, statusResponse{ID: target, IsEnabled: int16(value)})
}

func (h *Handler) Delete(context *gin.Context) {
	actor, target, ok := h.mutationIDs(context)
	if !ok {
		return
	}
	if err := validate.RequireEmptyBody(context); err != nil {
		response.Fail(context, err)
		return
	}
	projectmiddleware.SetAccessLogOperation(context, "user.delete", actor, target)
	if err := h.service.Delete(context.Request.Context(), actor, target); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, emptyResponse{})
}

func (h *Handler) Roles(context *gin.Context) {
	target, err := parseUserID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
		return
	}
	value, err := h.service.Roles(context.Request.Context(), target)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, newRolesResponse(value))
}

func (h *Handler) UpdateRoles(context *gin.Context) {
	actor, target, ok := h.mutationIDs(context)
	if !ok {
		return
	}
	var request rolesRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	values, err := request.values()
	if err != nil {
		response.Fail(context, err)
		return
	}
	projectmiddleware.SetAccessLogOperation(context, "user.roles.update", actor, target)
	count, err := h.service.UpdateRoles(context.Request.Context(), actor, target, values)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, roleResultResponse{ID: target, RoleCount: count})
}

func (h *Handler) mutationIDs(context *gin.Context) (int64, int64, bool) {
	target, err := parseUserID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
		return 0, 0, false
	}
	if h.actorUserID == nil {
		response.Fail(context, apperror.Unauthorized(fmt.Errorf("actor identity is missing")))
		return 0, 0, false
	}
	actor, exists := h.actorUserID(context)
	if !exists || actor <= 0 {
		response.Fail(context, apperror.Unauthorized(fmt.Errorf("actor identity is missing")))
		return 0, 0, false
	}
	return actor, target, true
}
