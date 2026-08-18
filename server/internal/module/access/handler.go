package access

import (
	"context"
	"fmt"
	"net/http"

	"admin/server/internal/module/auth"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type currentService interface {
	Current(context.Context, int64) (Snapshot, error)
}

type Handler struct {
	service currentService
}

func NewHandler(service currentService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Current(context *gin.Context) {
	identity, ok := auth.IdentityFromContext(context)
	if !ok {
		response.Fail(context, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	snapshot, err := h.service.Current(context.Request.Context(), identity.UserID)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, newCurrentResponse(snapshot))
}
