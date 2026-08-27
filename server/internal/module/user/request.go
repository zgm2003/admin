package user

import (
	"bytes"
	"encoding/json"
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

type updateRequest struct {
	Username *string              `json:"username"`
	Phone    nullablePhoneRequest `json:"phone"`
}

type nullablePhoneRequest struct {
	Present bool
	Value   *string
}

func (value *nullablePhoneRequest) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Value = nil
		return nil
	}
	var phone string
	if err := json.Unmarshal(data, &phone); err != nil {
		return err
	}
	value.Value = &phone
	return nil
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Username == nil {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("username is required"))
	}
	if !r.Phone.Present {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("phone is required"))
	}
	return UpdateInput{Username: *r.Username, Phone: r.Phone.Value}, nil
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
	return sharedvalidate.ParsePositiveInt64(value, "user id")
}

func parseListQuery(values url.Values) (ListQuery, error) {
	allowed := map[string]struct{}{"page": {}, "pageSize": {}, "keyword": {}, "isEnabled": {}, "roleId": {}}
	for key, entries := range values {
		if _, exists := allowed[key]; !exists || len(entries) != 1 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	paginationRequest, err := sharedpagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	query := ListQuery{Page: paginationRequest.Page, PageSize: paginationRequest.PageSize}
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
		roleID, parseErr := sharedvalidate.ParsePositiveInt64(entries[0], "role id")
		if parseErr != nil {
			return ListQuery{}, parseErr
		}
		query.RoleID = &roleID
	}
	return query, nil
}
