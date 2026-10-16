package transaction

import (
	"fmt"
	"net/url"
	"strconv"

	"admin/server/internal/shared/pagination"
)

type ListQuery struct {
	Page, PageSize int
	UserID         int64
	PlatformID     int64
	Personal       bool
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
	if (paging.Page - 1) > int(^uint(0)>>1)/paging.PageSize {
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
	if (paging.Page - 1) > int(^uint(0)>>1)/paging.PageSize {
		return ListQuery{}, fmt.Errorf("query offset overflow")
	}
	query := ListQuery{Page: paging.Page, PageSize: paging.PageSize}
	for key, target := range map[string]*int64{"userId": &query.UserID, "platformId": &query.PlatformID} {
		if entries, exists := values[key]; exists {
			raw := entries[0]
			if raw == "" {
				return ListQuery{}, fmt.Errorf("%s invalid", key)
			}
			for _, digit := range raw {
				if digit < '0' || digit > '9' {
					return ListQuery{}, fmt.Errorf("%s invalid", key)
				}
			}
			parsed, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil || parsed < 1 {
				return ListQuery{}, fmt.Errorf("%s invalid", key)
			}
			*target = parsed
		}
	}
	return query, nil
}
