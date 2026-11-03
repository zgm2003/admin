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
	Direction      Direction
	SourceType     SourceType
}

func parseListQuery(values url.Values, userID, platformID int64, personal bool) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "direction" && key != "sourceType") || len(entries) != 1 {
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
	query := ListQuery{Page: paging.Page, PageSize: paging.PageSize, UserID: userID, PlatformID: platformID, Personal: personal}
	if err := parseFilters(values, &query); err != nil {
		return ListQuery{}, err
	}
	return query, nil
}

func parseAdminListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "userId" && key != "platformId" && key != "direction" && key != "sourceType") || len(entries) != 1 {
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
	if err := parseFilters(values, &query); err != nil {
		return ListQuery{}, err
	}
	return query, nil
}

func parseFilters(values url.Values, query *ListQuery) error {
	if raw, exists := values["direction"]; exists {
		switch raw[0] {
		case "1":
			query.Direction = DirectionCredit
		case "2":
			query.Direction = DirectionDebit
		default:
			return fmt.Errorf("direction invalid")
		}
	}
	if raw, exists := values["sourceType"]; exists {
		switch raw[0] {
		case "1":
			query.SourceType = SourcePayment
		case "2":
			query.SourceType = SourceRedeemCode
		case "3":
			query.SourceType = SourceAI
		default:
			return fmt.Errorf("sourceType invalid")
		}
	}
	return nil
}
