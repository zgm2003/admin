package ratelimitpolicy

import (
	"fmt"
	"net/http"
	"strconv"

	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(ctx *gin.Context) {
	catalogs, err := h.service.ListAll(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	platforms := make([]PlatformResponse, 0, len(catalogs))
	for _, catalog := range catalogs {
		policies := make([]PolicyResponse, 0, len(catalog.Policies))
		for _, policy := range catalog.Policies {
			policies = append(policies, newPolicyResponse(policy))
		}
		platforms = append(platforms, PlatformResponse{PlatformID: catalog.PlatformID, PlatformCode: catalog.PlatformCode, PlatformName: catalog.PlatformName, Version: catalog.Version, Policies: policies})
	}
	response.OK(ctx, http.StatusOK, ListResponse{Platforms: platforms})
}

func (h *Handler) Update(ctx *gin.Context) {
	key := ctx.Param("key")
	platformID, err := strconv.ParseInt(ctx.Param("platformId"), 10, 64)
	if err != nil || platformID < 1 {
		response.Fail(ctx, invalid(fmt.Errorf("rate limit policy platform is invalid")))
		return
	}
	var request UpdateRequest
	if err := validate.BindJSON(ctx, &request); err != nil {
		response.Fail(ctx, err)
		return
	}
	catalog, err := h.service.Update(ctx.Request.Context(), platformID, Input{Key: key, Limit: request.Limit, WindowSeconds: request.WindowSeconds})
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	policy, found := Find(catalog, key)
	if !found {
		response.Fail(ctx, notFound(fmt.Errorf("rate limit policy %q is missing", key)))
		return
	}
	response.OK(ctx, http.StatusOK, UpdateResponse{PlatformID: catalog.PlatformID, Version: catalog.Version, Policy: newPolicyResponse(policy)})
}
