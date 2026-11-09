package server

import (
	"context"
	"net/http"

	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type snapshotService interface {
	Snapshot(context.Context) (Snapshot, error)
}
type Handler struct{ service snapshotService }

func NewHandler(service snapshotService) *Handler { return &Handler{service: service} }

func (h *Handler) Detail(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	value, err := h.service.Snapshot(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, value)
}
