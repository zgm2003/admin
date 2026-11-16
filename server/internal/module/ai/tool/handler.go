package tool

import (
	"context"
	"net/http"

	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"

	"github.com/gin-gonic/gin"
)

type handlerService interface {
	List(context.Context) ([]Value, error)
	UpdateStatus(context.Context, string, Status) error
}

type Handler struct{ service handlerService }

func NewHandler(service handlerService) *Handler { return &Handler{service: service} }

func (h *Handler) List(c *gin.Context) {
	result, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Fail(c, mapError(err))
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Fail(c, invalid(validationError("tool code is required")))
		return
	}
	var request statusRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	status, err := request.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	if err := h.service.UpdateStatus(c.Request.Context(), code, status); err != nil {
		response.Fail(c, mapError(err))
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

type validationError string

func (e validationError) Error() string { return string(e) }
