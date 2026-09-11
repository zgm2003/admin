package rateLimitPolicy

import (
	"context"
	"fmt"
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type service interface {
	List(context.Context) ([]PlatformResponse, error)
	Update(context.Context, int64, string, int, int) (PlatformResponse, error)
}

type Handler struct{ service service }

func NewHandler(service service) *Handler { return &Handler{service: service} }

type updateRequest struct {
	Limit         *int `json:"limit"`
	WindowSeconds *int `json:"windowSeconds"`
}

func (r updateRequest) values() (int, int, error) {
	if r.Limit == nil || r.WindowSeconds == nil {
		return 0, 0, apperror.InvalidRequest(fmt.Errorf("sms rate limit fields are required"))
	}
	return *r.Limit, *r.WindowSeconds, nil
}

func (h *Handler) List(c *gin.Context) {
	platforms, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, ListResponse{Platforms: platforms})
}

func (h *Handler) Update(c *gin.Context) {
	platformID, err := validate.ParsePositiveInt64(c.Param("platformId"), "platform id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var request updateRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	limit, windowSeconds, err := request.values()
	if err != nil {
		response.Fail(c, err)
		return
	}
	platform, err := h.service.Update(c.Request.Context(), platformID, c.Param("key"), limit, windowSeconds)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, platform)
}
