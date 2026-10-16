package callbackevent

import (
	"errors"
	"net/url"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
)

func parseListQuery(values url.Values) (pagination.Request, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize") || len(entries) != 1 {
			return pagination.Request{}, apperror.InvalidRequest(errors.New("callback pagination query invalid"))
		}
	}
	paging, err := pagination.ParseRequest(values)
	if err != nil {
		return pagination.Request{}, err
	}
	if paging.Page-1 > int(^uint(0)>>1)/paging.PageSize {
		return pagination.Request{}, apperror.InvalidRequest(errors.New("callback pagination offset overflow"))
	}
	return paging, nil
}
