package recipientrule

import (
	"fmt"
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(ctx *gin.Context) {
	values, err := h.service.List(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, values)
}

func (h *Handler) Create(ctx *gin.Context) {
	var input Input
	if err := validate.BindJSON(ctx, &input); err != nil {
		response.Fail(ctx, err)
		return
	}
	id, err := h.service.Create(ctx.Request.Context(), input)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusCreated, map[string]any{"id": id})
}

func (h *Handler) Update(ctx *gin.Context) {
	id, err := validate.ParsePositiveInt64(ctx.Param("id"), "id")
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	var input Input
	if err = validate.BindJSON(ctx, &input); err != nil {
		response.Fail(ctx, err)
		return
	}
	if err = h.service.Update(ctx.Request.Context(), id, input); err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, map[string]any{})
}

func (h *Handler) SetStatus(ctx *gin.Context) {
	id, err := validate.ParsePositiveInt64(ctx.Param("id"), "id")
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	var request struct {
		IsEnabled *yesno.Value `json:"isEnabled"`
	}
	if err = validate.BindJSON(ctx, &request); err != nil {
		response.Fail(ctx, err)
		return
	}
	if request.IsEnabled == nil {
		response.Fail(ctx, apperror.InvalidRequest(fmt.Errorf("isEnabled is required")))
		return
	}
	if err = h.service.SetStatus(ctx.Request.Context(), id, *request.IsEnabled); err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, map[string]any{"id": id, "isEnabled": *request.IsEnabled})
}

func (h *Handler) Delete(ctx *gin.Context) {
	id, err := validate.ParsePositiveInt64(ctx.Param("id"), "id")
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	if err = h.service.Delete(ctx.Request.Context(), id); err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, map[string]any{})
}
