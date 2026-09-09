package operationlog

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

type ListQuery struct {
	Page      int
	PageSize  int
	UserID    *int64
	Action    string
	Route     string
	IsSuccess *yesno.Value
	From      *time.Time
	To        *time.Time
}

func parseListQuery(values url.Values) (ListQuery, error) {
	allowed := map[string]struct{}{"page": {}, "pageSize": {}, "userId": {}, "action": {}, "route": {}, "isSuccess": {}, "from": {}, "to": {}}
	for key, entries := range values {
		if _, ok := allowed[key]; !ok || len(entries) != 1 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	query := ListQuery{Page: 1, PageSize: 20}
	var err error
	if value, ok := values["page"]; ok {
		query.Page, err = strconv.Atoi(value[0])
		if err != nil {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("page is invalid"))
		}
	}
	if value, ok := values["pageSize"]; ok {
		query.PageSize, err = strconv.Atoi(value[0])
		if err != nil {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("pageSize is invalid"))
		}
	}
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	if value, ok := values["userId"]; ok {
		id, parseErr := strconv.ParseInt(value[0], 10, 64)
		if parseErr != nil || id < 1 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("userId is invalid"))
		}
		query.UserID = &id
	}
	if value, ok := values["action"]; ok {
		query.Action = strings.TrimSpace(value[0])
		if len(query.Action) > 128 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("action is too long"))
		}
	}
	if value, ok := values["route"]; ok {
		query.Route = strings.TrimSpace(value[0])
		if len(query.Route) > 255 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("route is too long"))
		}
	}
	if value, ok := values["isSuccess"]; ok {
		parsed, parseErr := strconv.ParseInt(value[0], 10, 16)
		success := yesno.Value(parsed)
		if parseErr != nil || !yesno.IsValid(success) {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("isSuccess must be 0 or 1"))
		}
		query.IsSuccess = &success
	}
	for key, target := range map[string]**time.Time{"from": &query.From, "to": &query.To} {
		if value, ok := values[key]; ok {
			parsed, parseErr := time.Parse(time.RFC3339Nano, value[0])
			if parseErr != nil {
				return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("%s is invalid", key))
			}
			parsed = parsed.UTC()
			*target = &parsed
		}
	}
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("from must not be later than to"))
	}
	return query, nil
}
