package user

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

type updateRequest struct {
	Username *string `json:"username"`
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Username == nil {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("username is required"))
	}
	return UpdateInput{Username: *r.Username}, nil
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

type rolesRequest struct {
	RoleIDs *[]int64 `json:"roleIds"`
}

func (r rolesRequest) values() ([]int64, error) {
	if r.RoleIDs == nil {
		return nil, apperror.InvalidRequest(fmt.Errorf("roleIds is required"))
	}
	for _, id := range *r.RoleIDs {
		if id <= 0 {
			return nil, apperror.InvalidRequest(fmt.Errorf("roleIds must contain positive integers"))
		}
	}
	return append([]int64(nil), (*r.RoleIDs)...), nil
}

func parseUserID(value string) (int64, error) {
	if value == "" {
		return 0, apperror.InvalidRequest(fmt.Errorf("user id is required"))
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, apperror.InvalidRequest(fmt.Errorf("user id must be a positive base-10 integer"))
		}
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.InvalidRequest(fmt.Errorf("user id must be a positive base-10 integer"))
	}
	return id, nil
}

func parseListQuery(values url.Values) (ListQuery, error) {
	allowed := map[string]struct{}{"page": {}, "pageSize": {}, "keyword": {}, "isEnabled": {}, "roleId": {}}
	for key, entries := range values {
		if _, exists := allowed[key]; !exists || len(entries) != 1 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	page, err := parseRequiredUserQueryInt(values, "page")
	if err != nil {
		return ListQuery{}, err
	}
	pageSize, err := parseRequiredUserQueryInt(values, "pageSize")
	if err != nil {
		return ListQuery{}, err
	}
	query := ListQuery{Page: page, PageSize: pageSize}
	if entries, exists := values["keyword"]; exists {
		query.Keyword = strings.TrimSpace(entries[0])
		if utf8.RuneCountInString(query.Keyword) > 254 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("keyword is too long"))
		}
	}
	if entries, exists := values["isEnabled"]; exists {
		parsed, parseErr := strconv.ParseInt(entries[0], 10, 16)
		value := yesno.Value(parsed)
		if parseErr != nil || !yesno.IsValid(value) {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
		}
		query.IsEnabled = &value
	}
	if entries, exists := values["roleId"]; exists {
		roleID, parseErr := parseUserID(entries[0])
		if parseErr != nil {
			return ListQuery{}, parseErr
		}
		query.RoleID = &roleID
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	return query, nil
}

func parseRequiredUserQueryInt(values url.Values, key string) (int, error) {
	entries, exists := values[key]
	if !exists || len(entries) != 1 || entries[0] == "" {
		return 0, apperror.InvalidRequest(fmt.Errorf("%s is required once", key))
	}
	value, err := strconv.Atoi(entries[0])
	if err != nil {
		return 0, apperror.InvalidRequest(err)
	}
	return value, nil
}
