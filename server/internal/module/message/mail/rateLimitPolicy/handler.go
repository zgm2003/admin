package ratelimitpolicy

import (
	"fmt"
	"net/http"

	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(ctx *gin.Context) {
	catalog, err := h.service.List(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, ListResponse{Version: catalog.Version, Policies: catalog.Policies})
}

func (h *Handler) Update(ctx *gin.Context) {
	key := ctx.Param("key")
	var request UpdateRequest
	if err := validate.BindJSON(ctx, &request); err != nil {
		response.Fail(ctx, err)
		return
	}
	catalog, err := h.service.Update(ctx.Request.Context(), Input{Key: key, Limit: request.Limit, WindowSeconds: request.WindowSeconds})
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	policy, found := Find(catalog, key)
	if !found {
		response.Fail(ctx, notFound(fmt.Errorf("rate limit policy %q is missing", key)))
		return
	}
	response.OK(ctx, http.StatusOK, UpdateResponse{Version: catalog.Version, Policy: policy})
}
