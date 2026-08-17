package health

import (
	"context"
	"net/http"

	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type readinessService interface {
	Readiness(context.Context) (Readiness, error)
}

type Handler struct {
	service readinessService
}

func NewHandler(service readinessService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Health(context *gin.Context) {
	response.OK(context, http.StatusOK, Status{Status: "up"})
}

func (h *Handler) Ready(context *gin.Context) {
	readiness, err := h.service.Readiness(context.Request.Context())
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, readiness)
}
