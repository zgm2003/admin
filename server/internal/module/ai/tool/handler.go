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
	Get(context.Context, string) (Value, error)
	Create(context.Context, CreateInput) (int64, error)
	Update(context.Context, string, UpdateInput) error
	UpdateStatus(context.Context, string, Status) error
	Delete(context.Context, string) error
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

func (h *Handler) Get(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Fail(c, invalid(validationError("tool code is required")))
		return
	}
	result, err := h.service.Get(c.Request.Context(), code)
	if err != nil {
		response.Fail(c, mapError(err))
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
		response.Fail(c, mapError(err))
		return
	}
	response.OK(c, http.StatusCreated, struct {
		ID int64 `json:"id"`
	}{ID: id})
}

func (h *Handler) Update(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Fail(c, invalid(validationError("tool code is required")))
		return
	}
	var request updateRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	if err := h.service.Update(c.Request.Context(), code, input); err != nil {
		response.Fail(c, mapError(err))
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
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

func (h *Handler) Delete(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Fail(c, invalid(validationError("tool code is required")))
		return
	}
	if err := validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context(), code); err != nil {
		response.Fail(c, mapError(err))
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

type validationError string

func (e validationError) Error() string { return string(e) }
