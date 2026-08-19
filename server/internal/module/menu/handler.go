package menu

import (
	"context"
	"net/http"

	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type menuService interface {
	List(context.Context) ([]ManagedMenu, error)
	Create(context.Context, CreateInput) (int64, error)
	Update(context.Context, int64, UpdateInput) error
	UpdateStatus(context.Context, int64, yesno.Value) error
	Delete(context.Context, int64) error
}

type Handler struct {
	service menuService
}

func NewHandler(service menuService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(context *gin.Context) {
	items, err := h.service.List(context.Request.Context())
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, newManagedMenuResponses(items))
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
	response.OK(context, http.StatusCreated, menuIDResponse{ID: id})
}

func (h *Handler) Update(context *gin.Context) {
	id, err := parseMenuID(context.Param("id"))
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
	response.OK(context, http.StatusOK, menuIDResponse{ID: id})
}

func (h *Handler) UpdateStatus(context *gin.Context) {
	id, err := parseMenuID(context.Param("id"))
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
	response.OK(context, http.StatusOK, menuStatusResponse{ID: id, IsEnabled: int16(value)})
}

func (h *Handler) Delete(context *gin.Context) {
	id, err := parseMenuID(context.Param("id"))
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
	response.OK(context, http.StatusOK, menuIDResponse{ID: id})
}
