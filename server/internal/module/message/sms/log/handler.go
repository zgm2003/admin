package log

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type service interface {
	List(context.Context, ListQuery) (ListResult, error)
	Detail(context.Context, int64) (Detail, error)
}

type Handler struct{ service service }

func NewHandler(service service) *Handler { return &Handler{service: service} }

func (h *Handler) List(c *gin.Context) {
	query, err := parseListQuery(c.Request.URL.Query())
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) Detail(c *gin.Context) {
	id, err := validate.ParsePositiveInt64(c.Param("id"), "sms log id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	detail, err := h.service.Detail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, detail)
}

func parseListQuery(values url.Values) (ListQuery, error) {
	allowed := map[string]struct{}{
		"page": {}, "pageSize": {}, "platform": {}, "toPhone": {}, "scene": {}, "status": {}, "from": {}, "to": {},
	}
	for key, entries := range values {
		if _, ok := allowed[key]; !ok || len(entries) != 1 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	page, err := pagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	query := ListQuery{Page: page.Page, PageSize: page.PageSize}
	if value, ok := values["platform"]; ok {
		query.Platform = strings.TrimSpace(value[0])
		if len([]rune(query.Platform)) > 49 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("platform is too long"))
		}
	}
	if value, ok := values["toPhone"]; ok && value[0] != "" {
		if len(value[0]) > 32 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("toPhone is too long"))
		}
		query.Phone = value[0]
	}
	if value, ok := values["scene"]; ok {
		query.Scene = value[0]
	}
	if value, ok := values["status"]; ok {
		query.Status = value[0]
	}
	if value, ok := values["from"]; ok && value[0] != "" {
		parsed, err := time.Parse(time.RFC3339Nano, value[0])
		if err != nil {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("from is invalid"))
		}
		query.From = &parsed
	}
	if value, ok := values["to"]; ok && value[0] != "" {
		parsed, err := time.Parse(time.RFC3339Nano, value[0])
		if err != nil {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("to is invalid"))
		}
		query.To = &parsed
	}
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("from must not be after to"))
	}
	return query, nil
}
