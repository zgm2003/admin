package transaction

import (
	"context"
	"net/http"
	"net/url"

	"admin/server/internal/authcontext"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type handlerService interface {
	Get(context.Context, int64) (Value, error)
	List(context.Context, ListQuery) (pagination.Result[Value], error)
	PersonalList(context.Context, ListQuery) (pagination.Result[PersonalValue], error)
}

type Handler struct{ service handlerService }

func NewHandler(service handlerService) *Handler { return &Handler{service: service} }

func (h *Handler) List(c *gin.Context) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	query, err := parseAdminListQuery(values)
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	result, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := validate.ParsePositiveInt64(c.Param("id"), "id")
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

func (h *Handler) PersonalList(c *gin.Context) {
	identity, ok := authcontext.Get(c)
	if !ok || identity.UserID < 1 || identity.PlatformID < 1 {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	query, err := parseListQuery(values, identity.UserID, identity.PlatformID, true)
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	result, err := h.service.PersonalList(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}
