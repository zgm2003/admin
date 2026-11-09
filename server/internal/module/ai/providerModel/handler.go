package providermodel

import (
	"context"
	"net/http"

	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type handlerService interface {
	List(context.Context, int64, ListQuery) (pagination.Result[SafeValue], error)
	Get(context.Context, int64, int64) (SafeValue, error)
	Update(context.Context, int64, int64, UpdateInput) error
	Sync(context.Context, int64) (SyncResult, error)
	Options(context.Context, int64) (Options, error)
}

type Handler struct{ service handlerService }

func NewHandler(service handlerService) *Handler { return &Handler{service: service} }

func (h *Handler) Options(c *gin.Context) {
	id, err := providerID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.Options(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) List(c *gin.Context) {
	id, err := providerID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	query, err := parseListQuery(c.Request.URL.Query())
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	result, err := h.service.List(c.Request.Context(), id, query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) Get(c *gin.Context) {
	providerID, err := providerID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	modelID, err := modelID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.Get(c.Request.Context(), providerID, modelID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) Update(c *gin.Context) {
	providerID, err := providerID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	modelID, err := modelID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var request updateRequest
	if err = validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	if err = h.service.Update(c.Request.Context(), providerID, modelID, input); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

func (h *Handler) Sync(c *gin.Context) {
	id, err := providerID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err = validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.Sync(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func providerID(c *gin.Context) (int64, error) {
	return validate.ParsePositiveInt64(c.Param("id"), "id")
}

func modelID(c *gin.Context) (int64, error) {
	return validate.ParsePositiveInt64(c.Param("modelId"), "modelId")
}
