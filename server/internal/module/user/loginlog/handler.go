package loginlog

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

const (
	PermissionView = "account:user:loginlog:view"
	PermissionList = "account:user:loginlog:list"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) PageInit(context *gin.Context) {
	response.OK(context, http.StatusOK, map[string]any{"eventTypes": []string{EventLogin, EventLogout}, "loginTypes": []string{LoginPassword}})
}

func (h *Handler) List(context *gin.Context) {
	query, err := parseListQuery(context.Request.URL.Query())
	if err != nil {
		response.Fail(context, err)
		return
	}
	result, err := h.service.List(context.Request.Context(), query)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, result)
}

func parseListQuery(values url.Values) (ListQuery, error) {
	allowed := map[string]bool{"page": true, "pageSize": true, "userId": true, "platformId": true, "eventType": true, "loginType": true, "isSuccess": true, "loginAccount": true, "from": true, "to": true}
	for key, entries := range values {
		if !allowed[key] || len(entries) != 1 {
			return ListQuery{}, apperror.InvalidRequest(contextError("invalid or repeated query parameter"))
		}
	}
	query := ListQuery{Page: 1, PageSize: 20}
	var err error
	if v := values.Get("page"); v != "" {
		query.Page, err = strconv.Atoi(v)
		if err != nil {
			return ListQuery{}, apperror.InvalidRequest(err)
		}
	}
	if v := values.Get("pageSize"); v != "" {
		query.PageSize, err = strconv.Atoi(v)
		if err != nil {
			return ListQuery{}, apperror.InvalidRequest(err)
		}
	}
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return ListQuery{}, apperror.InvalidRequest(contextError("pagination is invalid"))
	}
	if v := values.Get("userId"); v != "" {
		id, e := strconv.ParseInt(v, 10, 64)
		if e != nil || id < 1 {
			return ListQuery{}, apperror.InvalidRequest(e)
		}
		query.UserID = &id
	}
	if v := values.Get("platformId"); v != "" {
		id, e := strconv.ParseInt(v, 10, 64)
		if e != nil || id < 1 {
			return ListQuery{}, apperror.InvalidRequest(e)
		}
		query.PlatformID = &id
	}
	query.EventType = strings.TrimSpace(values.Get("eventType"))
	query.LoginType = strings.TrimSpace(values.Get("loginType"))
	query.LoginAccount = strings.TrimSpace(values.Get("loginAccount"))
	if v := values.Get("isSuccess"); v != "" {
		n, e := strconv.ParseInt(v, 10, 16)
		value := yesno.Value(n)
		if e != nil || !yesno.IsValid(value) {
			return ListQuery{}, apperror.InvalidRequest(contextError("isSuccess must be 0 or 1"))
		}
		query.IsSuccess = &value
	}
	for key, target := range map[string]**time.Time{"from": &query.From, "to": &query.To} {
		if v := values.Get(key); v != "" {
			parsed, e := time.Parse(time.RFC3339Nano, v)
			if e != nil {
				return ListQuery{}, apperror.InvalidRequest(e)
			}
			parsed = parsed.UTC()
			*target = &parsed
		}
	}
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return ListQuery{}, apperror.InvalidRequest(contextError("from must not be later than to"))
	}
	return query, nil
}

func contextError(message string) error { return fmt.Errorf("%s", message) }
