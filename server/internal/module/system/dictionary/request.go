package dictionary

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
)

type createRequest struct {
	Code        *string `json:"code"`
	NameZH      *string `json:"nameZh"`
	NameEN      *string `json:"nameEn"`
	Description *string `json:"description"`
}
type updateRequest struct {
	NameZH      *string `json:"nameZh"`
	NameEN      *string `json:"nameEn"`
	Description *string `json:"description"`
}
type statusRequest struct {
	IsEnabled *yesno.Value `json:"isEnabled"`
}
type createItemRequest struct {
	Value   *string `json:"value"`
	LabelZH *string `json:"labelZh"`
	LabelEN *string `json:"labelEn"`
	Sort    *int    `json:"sort"`
}
type updateItemRequest struct {
	LabelZH *string `json:"labelZh"`
	LabelEN *string `json:"labelEn"`
	Sort    *int    `json:"sort"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Code == nil || r.NameZH == nil || r.NameEN == nil {
		return CreateInput{}, apperror.InvalidRequest(fmt.Errorf("code and bilingual names are required"))
	}
	description := ""
	if r.Description != nil {
		description = *r.Description
	}
	return CreateInput{Code: *r.Code, NameZH: *r.NameZH, NameEN: *r.NameEN, Description: description}, nil
}
func (r updateRequest) input() (UpdateInput, error) {
	if r.NameZH == nil || r.NameEN == nil {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("bilingual names are required"))
	}
	description := ""
	if r.Description != nil {
		description = *r.Description
	}
	return UpdateInput{NameZH: *r.NameZH, NameEN: *r.NameEN, Description: description}, nil
}
func (r statusRequest) value() (yesno.Value, error) {
	if r.IsEnabled == nil || !yesno.IsValid(*r.IsEnabled) {
		return 0, apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
	}
	return *r.IsEnabled, nil
}
func (r createItemRequest) input() (CreateItemInput, error) {
	if r.Value == nil || r.LabelZH == nil || r.LabelEN == nil || r.Sort == nil {
		return CreateItemInput{}, apperror.InvalidRequest(fmt.Errorf("value, bilingual labels and sort are required"))
	}
	return CreateItemInput{Value: *r.Value, LabelZH: *r.LabelZH, LabelEN: *r.LabelEN, Sort: *r.Sort}, nil
}
func (r updateItemRequest) input() (UpdateItemInput, error) {
	if r.LabelZH == nil || r.LabelEN == nil || r.Sort == nil {
		return UpdateItemInput{}, apperror.InvalidRequest(fmt.Errorf("bilingual labels and sort are required"))
	}
	return UpdateItemInput{LabelZH: *r.LabelZH, LabelEN: *r.LabelEN, Sort: *r.Sort}, nil
}

func parseID(value, label string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, apperror.InvalidRequest(fmt.Errorf("%s is invalid", label))
	}
	return id, nil
}
func parseListQuery(values url.Values) (ListQuery, error) {
	allowed := map[string]struct{}{"page": {}, "pageSize": {}, "keyword": {}, "isEnabled": {}}
	for key, entries := range values {
		if _, ok := allowed[key]; !ok || len(entries) != 1 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	p, err := pagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	q := ListQuery{Request: p}
	if v, ok := values["keyword"]; ok {
		q.Keyword = strings.TrimSpace(v[0])
		if len(q.Keyword) > 128 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("keyword is too long"))
		}
	}
	if v, ok := values["isEnabled"]; ok {
		n, e := strconv.ParseInt(v[0], 10, 16)
		x := yesno.Value(n)
		if e != nil || !yesno.IsValid(x) {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
		}
		q.IsEnabled = &x
	}
	return q, nil
}

var _ = validate.RequireEmptyBody
