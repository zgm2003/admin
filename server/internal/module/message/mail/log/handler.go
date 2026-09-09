package log

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"

	"github.com/gin-gonic/gin"
)

const PermissionDetail = "message:mail:detail"

type handlerService interface {
	List(context.Context, ListQuery, int, int) ([]ListRow, int64, error)
	Get(context.Context, int64) (Detail, error)
}

type Handler struct{ service handlerService }

func NewHandler(service handlerService) *Handler { return &Handler{service: service} }

func (h *Handler) List(ctx *gin.Context) {
	page, size, err := parsePagination(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	filter, err := parseListFilter(ctx.Request.URL.Query())
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	values, total, err := h.service.List(ctx.Request.Context(), filter, page, size)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, ListResponse(values, total, page, size))
}

func parsePagination(ctx *gin.Context) (int, int, error) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	if page < 1 || size < 1 || size > 100 {
		return 0, 0, apperror.InvalidRequest(fmt.Errorf("invalid pagination"))
	}
	return page, size, nil
}

func parseListFilter(values url.Values) (ListQuery, error) {
	allowed := map[string]bool{
		"page": true, "pageSize": true, "platform": true, "toEmail": true,
		"scene": true, "status": true, "from": true, "to": true,
	}
	for key, entries := range values {
		if !allowed[key] || len(entries) != 1 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	filter := ListQuery{
		Platform: strings.TrimSpace(values.Get("platform")),
		ToEmail:  strings.TrimSpace(values.Get("toEmail")),
		Scene:    strings.TrimSpace(values.Get("scene")),
		Status:   strings.TrimSpace(values.Get("status")),
	}
	limits := []struct {
		name  string
		value string
		max   int
	}{
		{"platform", filter.Platform, 49},
		{"toEmail", filter.ToEmail, 254},
		{"scene", filter.Scene, 32},
		{"status", filter.Status, 16},
	}
	for _, item := range limits {
		if len([]rune(item.value)) > item.max {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("query parameter %s is too long", item.name))
		}
	}
	for key, target := range map[string]**time.Time{"from": &filter.From, "to": &filter.To} {
		if v := values.Get(key); v != "" {
			parsed, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				return ListQuery{}, apperror.InvalidRequest(err)
			}
			*target = &parsed
		}
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("from must not be later than to"))
	}
	return filter, nil
}

func (h *Handler) Get(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	value, err := h.service.Get(ctx.Request.Context(), id)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, NewDetailResponse(value))
}

func parseID(ctx *gin.Context) (int64, error) {
	return validate.ParsePositiveInt64(ctx.Param("id"), "id")
}
