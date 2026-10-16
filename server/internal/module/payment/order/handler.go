package order

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"admin/server/internal/authcontext"
	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type handlerService interface {
	Options(context.Context) (Options, error)
	Create(context.Context, auth.Identity, CreateInput) (Checkout, error)
	Pay(context.Context, auth.Identity, int64) (Checkout, error)
	List(context.Context, ListQuery) (pagination.Result[SafeValue], error)
	Get(context.Context, int64) (SafeValue, error)
	GetForIdentity(context.Context, auth.Identity, int64) (SafeValue, error)
}

type Handler struct{ service handlerService }

func NewHandler(service handlerService) *Handler { return &Handler{service: service} }

func identity(c *gin.Context) (auth.Identity, bool) {
	value, ok := authcontext.Get(c)
	return auth.Identity{UserID: value.UserID, SessionID: value.SessionID, PlatformID: value.PlatformID, Platform: value.Platform}, ok && value.UserID > 0 && value.PlatformID > 0
}

func (h *Handler) Options(c *gin.Context) {
	if err := rejectAlternativeTransport(c); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.Options(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) Create(c *gin.Context) {
	if err := rejectAlternativeTransport(c); err != nil {
		response.Fail(c, err)
		return
	}
	actor, ok := identity(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	var request createRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	result, err := h.service.Create(c.Request.Context(), actor, input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusCreated, result)
}

func (h *Handler) Pay(c *gin.Context) {
	if err := rejectAlternativeTransport(c); err != nil {
		response.Fail(c, err)
		return
	}
	actor, ok := identity(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	id, err := validate.ParsePositiveInt64(c.Param("id"), "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.Pay(c.Request.Context(), actor, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

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
	if err := rejectAlternativeTransport(c); err != nil {
		response.Fail(c, err)
		return
	}
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
	actor, ok := identity(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	query, err := parseListQuery(values, actor.UserID, actor.PlatformID, true)
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

func (h *Handler) PersonalGet(c *gin.Context) {
	if err := rejectAlternativeTransport(c); err != nil {
		response.Fail(c, err)
		return
	}
	actor, ok := identity(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	id, err := validate.ParsePositiveInt64(c.Param("id"), "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.GetForIdentity(c.Request.Context(), actor, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func rejectAlternativeTransport(c *gin.Context) error {
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		return apperror.InvalidRequest(fmt.Errorf("query parameters are not accepted"))
	}
	return nil
}
