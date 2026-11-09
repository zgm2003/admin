package officialmodel

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"admin/server/internal/shared/pagination"
)

type ListQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Vendor   string
}

type UpdatePriceInput struct {
	ExpectedVersion int64
	InputCNYMicros  int64
	OutputCNYMicros int64
}

type ResetPriceInput struct {
	ExpectedVersion int64
}

type updatePriceRequest struct {
	ExpectedVersion *int64  `json:"expectedVersion"`
	InputCNY        *string `json:"inputCnyPerMillion"`
	OutputCNY       *string `json:"outputCnyPerMillion"`
}

func (r updatePriceRequest) input() (UpdatePriceInput, error) {
	if r.ExpectedVersion == nil || r.InputCNY == nil || r.OutputCNY == nil {
		return UpdatePriceInput{}, fmt.Errorf("expectedVersion, inputCnyPerMillion and outputCnyPerMillion are required")
	}
	if *r.ExpectedVersion < 1 {
		return UpdatePriceInput{}, fmt.Errorf("expectedVersion must be positive")
	}
	input, err := parseMicros(*r.InputCNY)
	if err != nil {
		return UpdatePriceInput{}, fmt.Errorf("inputCnyPerMillion: %w", err)
	}
	output, err := parseMicros(*r.OutputCNY)
	if err != nil {
		return UpdatePriceInput{}, fmt.Errorf("outputCnyPerMillion: %w", err)
	}
	return UpdatePriceInput{ExpectedVersion: *r.ExpectedVersion, InputCNYMicros: input, OutputCNYMicros: output}, nil
}

type resetPriceRequest struct {
	ExpectedVersion *int64 `json:"expectedVersion"`
}

func (r resetPriceRequest) input() (ResetPriceInput, error) {
	if r.ExpectedVersion == nil || *r.ExpectedVersion < 1 {
		return ResetPriceInput{}, fmt.Errorf("expectedVersion must be positive")
	}
	return ResetPriceInput{ExpectedVersion: *r.ExpectedVersion}, nil
}

func parseListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "keyword" && key != "vendor") || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("query invalid")
		}
	}
	request, err := pagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	query := ListQuery{
		Page: request.Page, PageSize: request.PageSize,
		Keyword: strings.TrimSpace(values.Get("keyword")),
		Vendor:  strings.TrimSpace(values.Get("vendor")),
	}
	if utf8.RuneCountInString(query.Keyword) > 200 {
		return ListQuery{}, fmt.Errorf("keyword is too long")
	}
	if query.Vendor != "" && !validCode(query.Vendor, 64) {
		return ListQuery{}, fmt.Errorf("vendor is invalid")
	}
	return query, nil
}
