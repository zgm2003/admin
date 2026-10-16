package rechargepackage

import (
	"admin/server/internal/shared/money"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

type CreateInput struct {
	Code        string
	Name        string
	AmountCents int64
	Badge       *string
	SortOrder   int32
	IsEnabled   yesno.Value
}

type UpdateInput struct {
	Name        string
	AmountCents int64
	Badge       *string
	SortOrder   int32
}

type ListQuery struct {
	Page, PageSize int
	Keyword        string
	IsEnabled      *yesno.Value
}

type createRequest struct {
	Code      *string      `json:"code"`
	Name      *string      `json:"name"`
	Amount    *string      `json:"amount"`
	Badge     *string      `json:"badge"`
	SortOrder *int32       `json:"sortOrder"`
	IsEnabled *yesno.Value `json:"isEnabled"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Code == nil || r.Name == nil || r.Amount == nil || r.SortOrder == nil || r.IsEnabled == nil {
		return CreateInput{}, fmt.Errorf("code, name, amount, sortOrder and isEnabled are required")
	}
	cents, err := parseAmount(*r.Amount)
	if err != nil {
		return CreateInput{}, err
	}
	input := CreateInput{Code: strings.TrimSpace(*r.Code), Name: strings.TrimSpace(*r.Name), AmountCents: cents, Badge: r.Badge, SortOrder: *r.SortOrder, IsEnabled: *r.IsEnabled}
	if err := validateCreate(input); err != nil {
		return CreateInput{}, err
	}
	input.Badge, err = normalizeBadge(input.Badge)
	return input, err
}

type updateRequest struct {
	Name      *string         `json:"name"`
	Amount    *string         `json:"amount"`
	Badge     json.RawMessage `json:"badge"`
	SortOrder *int32          `json:"sortOrder"`
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Name == nil || r.Amount == nil || r.Badge == nil || r.SortOrder == nil {
		return UpdateInput{}, fmt.Errorf("name, amount, badge and sortOrder are required")
	}
	cents, err := parseAmount(*r.Amount)
	if err != nil {
		return UpdateInput{}, err
	}
	badge, err := parseBadge(r.Badge)
	if err != nil {
		return UpdateInput{}, err
	}
	input := UpdateInput{Name: strings.TrimSpace(*r.Name), AmountCents: cents, Badge: badge, SortOrder: *r.SortOrder}
	if err := validateUpdate(input); err != nil {
		return UpdateInput{}, err
	}
	input.Badge, err = normalizeBadge(input.Badge)
	return input, err
}

func parseAmount(value string) (int64, error) {
	cents, err := money.ParseRMBCents(value)
	if err != nil || cents < 1 || validateAmountCents(cents) != nil {
		return 0, fmt.Errorf("amount must be a positive RMB amount with at most two decimal places")
	}
	return cents, nil
}

func parseBadge(raw json.RawMessage) (*string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("badge must be a string or null")
	}
	return normalizeBadge(&value)
}

type statusRequest struct {
	IsEnabled *yesno.Value `json:"isEnabled"`
}

func validateCreate(input CreateInput) error {
	if err := validateCode(input.Code); err != nil {
		return err
	}
	if err := validateUpdate(UpdateInput{input.Name, input.AmountCents, input.Badge, input.SortOrder}); err != nil {
		return err
	}
	if err := validateFlags(input.IsEnabled); err != nil {
		return err
	}
	return nil
}

func validateUpdate(input UpdateInput) error {
	if err := validateName(input.Name); err != nil {
		return err
	}
	if err := validateAmountCents(input.AmountCents); err != nil {
		return err
	}
	if _, err := normalizeBadge(input.Badge); err != nil {
		return err
	}
	return nil
}

func parseListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "keyword" && key != "isEnabled") || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("query invalid")
		}
	}
	p, err := pagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	query := ListQuery{Page: p.Page, PageSize: p.PageSize, Keyword: strings.TrimSpace(values.Get("keyword"))}
	if !utf8.ValidString(query.Keyword) || utf8.RuneCountInString(query.Keyword) > MaxNameLength {
		return ListQuery{}, fmt.Errorf("keyword invalid")
	}
	if raw, exists := values["isEnabled"]; exists {
		switch raw[0] {
		case "0":
			value := yesno.No
			query.IsEnabled = &value
		case "1":
			value := yesno.Yes
			query.IsEnabled = &value
		default:
			return ListQuery{}, fmt.Errorf("isEnabled invalid")
		}
	}
	return query, nil
}
