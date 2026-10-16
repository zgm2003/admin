package order

import (
	"fmt"
	"net/url"

	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/validate"
)

type ListQuery struct {
	Page       int
	PageSize   int
	UserID     int64
	PlatformID int64
	Personal   bool
}

func parseListQuery(values url.Values, userID, platformID int64, personal bool) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize") || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("query invalid")
		}
	}
	paging, err := pagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	if userID < 1 || platformID < 1 {
		return ListQuery{}, fmt.Errorf("identity invalid")
	}
	if paging.Page-1 > int(^uint(0)>>1)/paging.PageSize {
		return ListQuery{}, fmt.Errorf("query offset overflow")
	}
	return ListQuery{Page: paging.Page, PageSize: paging.PageSize, UserID: userID, PlatformID: platformID, Personal: personal}, nil
}

func parseAdminListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "userId" && key != "platformId") || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("query invalid")
		}
	}
	paging, err := pagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	query := ListQuery{Page: paging.Page, PageSize: paging.PageSize}
	for key, target := range map[string]*int64{"userId": &query.UserID, "platformId": &query.PlatformID} {
		entries, present := values[key]
		if !present {
			continue
		}
		parsed, parseErr := validate.ParsePositiveInt64(entries[0], key)
		if parseErr != nil {
			return ListQuery{}, fmt.Errorf("%s invalid", key)
		}
		*target = parsed
	}
	if paging.Page-1 > int(^uint(0)>>1)/paging.PageSize {
		return ListQuery{}, fmt.Errorf("query offset overflow")
	}
	return query, nil
}
