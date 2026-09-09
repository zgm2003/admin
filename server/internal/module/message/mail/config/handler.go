package config

import (
	"net/http"

	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Get(ctx *gin.Context) {
	value, err := h.service.Get(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, value)
}

func (h *Handler) Save(ctx *gin.Context) {
	var input Input
	if err := validate.BindJSON(ctx, &input); err != nil {
		response.Fail(ctx, err)
		return
	}
	value, err := h.service.Save(ctx.Request.Context(), input)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, value)
}

func (h *Handler) Delete(ctx *gin.Context) {
	if err := validate.RequireEmptyBody(ctx); err != nil {
		response.Fail(ctx, err)
		return
	}
	if err := h.service.Delete(ctx.Request.Context()); err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, map[string]any{})
}
