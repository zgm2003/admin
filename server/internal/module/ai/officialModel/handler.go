package officialmodel

import (
	"context"
	"net/http"

	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type handlerService interface {
	List(context.Context, ListQuery) (pagination.Result[SafeValue], error)
	Options(context.Context) (Options, error)
	UpdatePrice(context.Context, int64, UpdatePriceInput) error
	ResetPrice(context.Context, int64, ResetPriceInput) error
	SyncCatalog(context.Context) (SyncResult, error)
}

type Handler struct{ service handlerService }

func NewHandler(service handlerService) *Handler { return &Handler{service: service} }

func (h *Handler) Options(c *gin.Context) {
	result, err := h.service.Options(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) List(c *gin.Context) {
	query, err := parseListQuery(c.Request.URL.Query())
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	result, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func modelID(c *gin.Context) (int64, error) {
	return validate.ParsePositiveInt64(c.Param("id"), "id")
}

func (h *Handler) UpdatePrice(c *gin.Context) {
	id, err := modelID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var request updatePriceRequest
	if err = validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	if err = h.service.UpdatePrice(c.Request.Context(), id, input); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

func (h *Handler) ResetPrice(c *gin.Context) {
	id, err := modelID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var request resetPriceRequest
	if err = validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	if err = h.service.ResetPrice(c.Request.Context(), id, input); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

func (h *Handler) SyncCatalog(c *gin.Context) {
	if err := validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.SyncCatalog(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}
