package order

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/validate"
)

type ListQuery struct {
	Page       int
	PageSize   int
	UserID     int64
	PlatformID int64
	Personal   bool
	Keyword    string
	Status     *Status
	From       *time.Time
	To         *time.Time
}

func parseListQuery(values url.Values, userID, platformID int64, personal bool) (ListQuery, error) {
	for key, entries := range values {
		if !allowedOrderKey(key, false) || len(entries) != 1 {
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
	query := ListQuery{Page: paging.Page, PageSize: paging.PageSize, UserID: userID, PlatformID: platformID, Personal: personal}
	if err := parseOrderFilters(values, &query); err != nil {
		return ListQuery{}, err
	}
	return query, nil
}

func parseAdminListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if !allowedOrderKey(key, true) || len(entries) != 1 {
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
	if err := parseOrderFilters(values, &query); err != nil {
		return ListQuery{}, err
	}
	return query, nil
}

func allowedOrderKey(key string, admin bool) bool {
	switch key {
	case "page", "pageSize", "keyword", "status", "from", "to":
		return true
	case "userId", "platformId":
		return admin
	default:
		return false
	}
}

func parseOrderFilters(values url.Values, query *ListQuery) error {
	if raw := values.Get("keyword"); raw != "" {
		keyword := strings.TrimSpace(raw)
		if utf8.RuneCountInString(keyword) > 64 {
			return fmt.Errorf("keyword too long")
		}
		query.Keyword = keyword
	}
	if raw := values.Get("status"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 16)
		if err != nil {
			return fmt.Errorf("status invalid")
		}
		status := Status(parsed)
		if status != StatusPending && status != StatusPaid && status != StatusClosed {
			return fmt.Errorf("status invalid")
		}
		query.Status = &status
	}
	for key, target := range map[string]**time.Time{"from": &query.From, "to": &query.To} {
		raw := values.Get(key)
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return fmt.Errorf("%s invalid", key)
		}
		*target = &parsed
	}
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return fmt.Errorf("from must not be after to")
	}
	return nil
}
