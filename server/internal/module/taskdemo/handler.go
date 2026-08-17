package taskdemo

import (
	"context"
	"net/http"

	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type submissionService interface {
	Create(context.Context, string) (Created, error)
}

type Handler struct {
	service submissionService
}

func NewHandler(service submissionService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(context *gin.Context) {
	var request CreateRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}

	created, err := h.service.Create(context.Request.Context(), request.Message)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusAccepted, created)
}
