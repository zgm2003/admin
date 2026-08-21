package role

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	sharedpagination "admin/server/internal/shared/pagination"
	sharedvalidate "admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
)

type createRequest struct {
	Code *string `json:"code"`
	Name *string `json:"name"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Code == nil || r.Name == nil {
		return CreateInput{}, apperror.InvalidRequest(fmt.Errorf("code and name are required"))
	}
	return CreateInput{Code: *r.Code, Name: *r.Name}, nil
}

type updateRequest struct {
	Name *string `json:"name"`
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Name == nil {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("name is required"))
	}
	return UpdateInput{Name: *r.Name}, nil
}

type statusRequest struct {
	IsEnabled *yesno.Value `json:"isEnabled"`
}

func (r statusRequest) value() (yesno.Value, error) {
	if r.IsEnabled == nil || !yesno.IsValid(*r.IsEnabled) {
		return 0, apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
	}
	return *r.IsEnabled, nil
}

type permissionsRequest struct {
	MenuIDs *[]int64 `json:"menuIds"`
}

func (r permissionsRequest) values() ([]int64, error) {
	if r.MenuIDs == nil {
		return nil, apperror.InvalidRequest(fmt.Errorf("menuIds is required"))
	}
	seen := make(map[int64]struct{}, len(*r.MenuIDs))
	for _, id := range *r.MenuIDs {
		if id < 1 {
			return nil, apperror.InvalidRequest(fmt.Errorf("menuIds must contain positive integers"))
		}
		if _, exists := seen[id]; exists {
			return nil, apperror.InvalidRequest(fmt.Errorf("menuIds contains duplicate id"))
		}
		seen[id] = struct{}{}
	}
	return *r.MenuIDs, nil
}

func parseRoleID(value string) (int64, error) {
	return sharedvalidate.ParsePositiveInt64(value, "role id")
}

func parseListQuery(values url.Values) (ListQuery, error) {
	allowed := map[string]struct{}{"page": {}, "pageSize": {}, "keyword": {}, "isEnabled": {}}
	for key, entries := range values {
		if _, ok := allowed[key]; !ok || len(entries) != 1 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	paginationRequest, err := sharedpagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	query := ListQuery{Page: paginationRequest.Page, PageSize: paginationRequest.PageSize}
	if entries, ok := values["keyword"]; ok {
		query.Keyword = strings.TrimSpace(entries[0])
		if utf8.RuneCountInString(query.Keyword) > 64 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("keyword is too long"))
		}
	}
	if entries, ok := values["isEnabled"]; ok {
		parsed, parseErr := strconv.ParseInt(entries[0], 10, 16)
		value := yesno.Value(parsed)
		if parseErr != nil || !yesno.IsValid(value) {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
		}
		query.IsEnabled = &value
	}
	return query, nil
}
