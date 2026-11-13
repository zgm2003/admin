package transaction

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/shared/pagination"
)

type FlowType int16

const (
	FlowAll    FlowType = 0
	FlowCredit FlowType = 1
	FlowDebit  FlowType = 2
)

type ListQuery struct {
	Page, PageSize int
	UserID         int64
	PlatformID     int64
	Personal       bool
	Flow           FlowType
	Keyword        string
	From           *time.Time
	To             *time.Time
}

func parseListQuery(values url.Values, userID, platformID int64, personal bool) (ListQuery, error) {
	for key, entries := range values {
		if !allowedLedgerKey(key, false) || len(entries) != 1 {
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
	if err := parseLedgerFilters(values, &query); err != nil {
		return ListQuery{}, err
	}
	return query, nil
}

func parseAdminListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if !allowedLedgerKey(key, true) || len(entries) != 1 {
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
	if err := parseLedgerFilters(values, &query); err != nil {
		return ListQuery{}, err
	}
	return query, nil
}

func allowedLedgerKey(key string, admin bool) bool {
	switch key {
	case "page", "pageSize", "flow", "keyword", "from", "to":
		return true
	case "userId", "platformId":
		return admin
	default:
		return false
	}
}

func parseLedgerFilters(values url.Values, query *ListQuery) error {
	if raw := values.Get("flow"); raw != "" {
		switch raw {
		case "all":
			query.Flow = FlowAll
		case "credit":
			query.Flow = FlowCredit
		case "debit":
			query.Flow = FlowDebit
		default:
			return fmt.Errorf("flow invalid")
		}
	}
	if raw := values.Get("keyword"); raw != "" {
		keyword := strings.TrimSpace(raw)
		if utf8.RuneCountInString(keyword) > 64 {
			return fmt.Errorf("keyword too long")
		}
		query.Keyword = keyword
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
