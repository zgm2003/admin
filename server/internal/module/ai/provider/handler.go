package provider

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
	Get(context.Context, int64) (SafeValue, error)
	Create(context.Context, CreateInput) (int64, error)
	Update(context.Context, int64, UpdateInput) error
	UpdateStatus(context.Context, int64, StatusInput) error
	Delete(context.Context, int64) error
	Options(context.Context) (Options, error)
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

func providerID(c *gin.Context) (int64, error) {
	return validate.ParsePositiveInt64(c.Param("id"), "id")
}

func (h *Handler) Get(c *gin.Context) {
	id, err := providerID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) Create(c *gin.Context) {
	var request createRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	id, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusCreated, struct {
		ID int64 `json:"id"`
	}{ID: id})
}

func (h *Handler) Update(c *gin.Context) {
	id, err := providerID(c)
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
	if err = h.service.Update(c.Request.Context(), id, input); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	id, err := providerID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var request statusRequest
	if err = validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	if err = h.service.UpdateStatus(c.Request.Context(), id, input); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := providerID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err = validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err = h.service.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
