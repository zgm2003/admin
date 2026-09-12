package captcha

import (
	"context"
	"net/http"

	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type handlerService interface {
	Generate(context.Context) (Challenge, error)
}
type Handler struct{ service handlerService }

func NewHandler(service handlerService) *Handler { return &Handler{service: service} }
func (h *Handler) Generate(c *gin.Context) {
	challenge, err := h.service.Generate(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, challenge)
}
