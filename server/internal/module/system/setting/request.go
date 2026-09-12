package setting

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"admin/server/internal/shared/yesno"
)

type createRequest struct {
	Key         *string `json:"key"`
	Value       *string `json:"value"`
	ValueType   *int    `json:"valueType"`
	Description *string `json:"description"`
}
type updateRequest struct {
	Value       *string `json:"value"`
	ValueType   *int    `json:"valueType"`
	Description *string `json:"description"`
}
type statusRequest struct {
	IsEnabled *yesno.Value `json:"isEnabled"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Key == nil || r.Value == nil || r.ValueType == nil {
		return CreateInput{}, fmt.Errorf("key, value and valueType are required")
	}
	description := ""
	if r.Description != nil {
		description = *r.Description
	}
	return CreateInput{Key: *r.Key, Value: *r.Value, ValueType: *r.ValueType, Description: description}, nil
}
func (r updateRequest) input() (UpdateInput, error) {
	if r.Value == nil || r.ValueType == nil {
		return UpdateInput{}, fmt.Errorf("value and valueType are required")
	}
	description := ""
	if r.Description != nil {
		description = *r.Description
	}
	return UpdateInput{Value: *r.Value, ValueType: *r.ValueType, Description: description}, nil
}
func parseListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if key != "page" && key != "pageSize" && key != "keyword" && key != "isEnabled" || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("invalid query parameter")
		}
	}
	page, pageSize := 1, 20
	var err error
	if raw := values.Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil {
			return ListQuery{}, fmt.Errorf("page is invalid")
		}
	}
	if raw := values.Get("pageSize"); raw != "" {
		pageSize, err = strconv.Atoi(raw)
		if err != nil {
			return ListQuery{}, fmt.Errorf("pageSize is invalid")
		}
	}
	query := ListQuery{Page: page, PageSize: pageSize, Keyword: strings.TrimSpace(values.Get("keyword"))}
	if raw := values.Get("isEnabled"); raw != "" {
		value, err := strconv.Atoi(raw)
		parsed := yesno.Value(value)
		if err != nil || !yesno.IsValid(parsed) {
			return ListQuery{}, fmt.Errorf("isEnabled is invalid")
		}
		query.IsEnabled = &parsed
	}
	return query, nil
}
