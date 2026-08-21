package pagination

import (
	"fmt"
	"net/url"

	"admin/server/internal/shared/apperror"
)

type Request struct {
	Page     int `form:"page" binding:"required,min=1"`
	PageSize int `form:"pageSize" binding:"required,min=1,max=100"`
}

// ParseRequest parses only the required pagination parameters. Callers keep
// ownership of their module-specific query allowlist and filters.
func ParseRequest(values url.Values) (Request, error) {
	page, err := parsePositiveInt(values, "page")
	if err != nil {
		return Request{}, err
	}
	pageSize, err := parsePositiveInt(values, "pageSize")
	if err != nil {
		return Request{}, err
	}
	if pageSize > 100 {
		return Request{}, apperror.InvalidRequest(fmt.Errorf("pageSize must be between 1 and 100"))
	}
	return Request{Page: page, PageSize: pageSize}, nil
}

func parsePositiveInt(values url.Values, key string) (int, error) {
	entries, exists := values[key]
	if !exists || len(entries) != 1 || entries[0] == "" {
		return 0, apperror.InvalidRequest(fmt.Errorf("%s is required once", key))
	}
	value := 0
	for _, character := range entries[0] {
		if character < '0' || character > '9' {
			return 0, apperror.InvalidRequest(fmt.Errorf("%s must be a positive base-10 integer", key))
		}
		value = value*10 + int(character-'0')
		if value < 0 {
			return 0, apperror.InvalidRequest(fmt.Errorf("%s is too large", key))
		}
	}
	if value < 1 {
		return 0, apperror.InvalidRequest(fmt.Errorf("%s must be a positive integer", key))
	}
	return value, nil
}

type Result[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}
