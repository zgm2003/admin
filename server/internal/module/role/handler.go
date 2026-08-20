package role

import (
	"context"
	"net/http"

	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type roleService interface {
	List(context.Context, ListQuery) (pagination.Result[ListItem], error)
	Create(context.Context, CreateInput) (int64, error)
	Update(context.Context, int64, UpdateInput) error
	UpdateStatus(context.Context, int64, yesno.Value) error
	SetDefault(context.Context, int64) error
	Delete(context.Context, int64) error
	Permissions(context.Context, int64) (Permissions, error)
	UpdatePermissions(context.Context, int64, []int64) (int64, error)
}
type Handler struct {
	service roleService
}

func NewHandler(service roleService) *Handler {
	return &Handler{service: service}
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
	response.OK(context, http.StatusOK, roleListResponse(result.List, result.Total, result.Page, result.PageSize))
}

func (h *Handler) Create(context *gin.Context) {
	var request createRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(context, err)
		return
	}
	id, err := h.service.Create(context.Request.Context(), input)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusCreated, idResponse{ID: id})
}

func (h *Handler) Update(context *gin.Context) {
	id, err := parseRoleID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
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
	if err := h.service.Update(context.Request.Context(), id, input); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, emptyResponse{})
}

func (h *Handler) UpdateStatus(context *gin.Context) {
	id, err := parseRoleID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
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
	if err := h.service.UpdateStatus(context.Request.Context(), id, value); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, statusResponse{ID: id, IsEnabled: int16(value)})
}

func (h *Handler) SetDefault(context *gin.Context) {
	id, err := parseRoleID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
		return
	}
	if err := validate.RequireEmptyBody(context); err != nil {
		response.Fail(context, err)
		return
	}
	if err := h.service.SetDefault(context.Request.Context(), id); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, defaultResponse{ID: id, IsDefault: 1})
}

func (h *Handler) Delete(context *gin.Context) {
	id, err := parseRoleID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
		return
	}
	if err := validate.RequireEmptyBody(context); err != nil {
		response.Fail(context, err)
		return
	}
	if err := h.service.Delete(context.Request.Context(), id); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, emptyResponse{})
}

func (h *Handler) Permissions(context *gin.Context) {
	id, err := parseRoleID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
		return
	}
	value, err := h.service.Permissions(context.Request.Context(), id)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, newPermissionsResponse(value))
}

func (h *Handler) UpdatePermissions(context *gin.Context) {
	id, err := parseRoleID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
		return
	}
	var request permissionsRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	menuIDs, err := request.values()
	if err != nil {
		response.Fail(context, err)
		return
	}
	count, err := h.service.UpdatePermissions(context.Request.Context(), id, menuIDs)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, permissionResultResponse{ID: id, PermissionCount: count})
}
