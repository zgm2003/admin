package operationlog

import (
	"context"
	"net/http"

	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service interface {
		List(context.Context, ListQuery) (ListResult, error)
	}
}

func NewHandler(service interface {
	List(context.Context, ListQuery) (ListResult, error)
}) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(context *gin.Context) {
	query, err := parseListQuery(context.Request.URL.Query())
	if err != nil {
		response.Fail(context, err)
		return
	}
	result, err := h.service.List(context.Request.Context(), query)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, result)
}
