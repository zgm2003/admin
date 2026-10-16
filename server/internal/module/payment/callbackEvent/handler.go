package callbackevent

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

const maxProviderBody = 64 * 1024

type handlerService interface {
	Receive(context.Context, int64, int64, url.Values) (ReceiveResult, error)
	ListByOrder(context.Context, int64, int, int) (pagination.Result[Value], error)
	GetForOrder(context.Context, int64, int64) (Value, error)
}

type Handler struct{ service handlerService }

func NewHandler(service handlerService) *Handler { return &Handler{service: service} }

func providerFailure(c *gin.Context, status int) {
	c.Data(status, "text/plain; charset=utf-8", []byte("failure"))
}

func (h *Handler) Provider(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h == nil || h.service == nil {
		providerFailure(c, http.StatusServiceUnavailable)
		return
	}
	request := c.Request
	if request.URL.RawQuery != "" || request.URL.ForceQuery || (request.Header.Get("Content-Encoding") != "" && !strings.EqualFold(request.Header.Get("Content-Encoding"), "identity")) {
		providerFailure(c, http.StatusBadRequest)
		return
	}
	mediaType, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) {
		providerFailure(c, http.StatusUnsupportedMediaType)
		return
	}
	if request.ContentLength > maxProviderBody {
		providerFailure(c, http.StatusRequestEntityTooLarge)
		return
	}
	if request.Body == nil {
		providerFailure(c, http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxProviderBody+1))
	if err != nil || len(body) == 0 {
		providerFailure(c, http.StatusBadRequest)
		return
	}
	if len(body) > maxProviderBody {
		providerFailure(c, http.StatusRequestEntityTooLarge)
		return
	}
	values, err := url.ParseQuery(string(body))
	if err != nil || len(values) == 0 || len(values) > 64 {
		providerFailure(c, http.StatusBadRequest)
		return
	}
	for name, entries := range values {
		if name == "" || len(name) > 128 || len(entries) != 1 {
			providerFailure(c, http.StatusBadRequest)
			return
		}
	}
	configID, err := validate.ParsePositiveInt64(c.Param("configId"), "configId")
	if err != nil {
		providerFailure(c, http.StatusBadRequest)
		return
	}
	version, err := validate.ParsePositiveInt64(c.Param("version"), "version")
	if err != nil {
		providerFailure(c, http.StatusBadRequest)
		return
	}
	if _, err := h.service.Receive(request.Context(), configID, version, values); err != nil {
		status := http.StatusServiceUnavailable
		var public *apperror.Error
		if errors.As(err, &public) && public.HTTPStatus < 500 {
			status = http.StatusBadRequest
		}
		providerFailure(c, status)
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("success"))
}

func (h *Handler) List(c *gin.Context) {
	orderID, err := validate.ParsePositiveInt64(c.Param("id"), "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	query, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	paging, err := parseListQuery(query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.ListByOrder(c.Request.Context(), orderID, paging.Page, paging.PageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) Get(c *gin.Context) {
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		response.Fail(c, apperror.InvalidRequest(nil))
		return
	}
	orderID, err := validate.ParsePositiveInt64(c.Param("id"), "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	eventID, err := validate.ParsePositiveInt64(c.Param("eventId"), "eventId")
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.GetForOrder(c.Request.Context(), orderID, eventID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}
