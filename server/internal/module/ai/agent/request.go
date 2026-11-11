package agent

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
)

const (
	maxNameRunes         = 128
	maxDescriptionRunes  = 500
	maxSystemPromptRunes = 20000
)

type ListQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Status   *yesno.Value
}

type CreateInput struct {
	Name            string
	Description     string
	SystemPrompt    string
	ProviderModelID int64
	Status          yesno.Value
}

type UpdateInput struct {
	Name            string
	Description     string
	SystemPrompt    string
	ProviderModelID int64
	Status          yesno.Value
}

type StatusInput struct {
	Status yesno.Value
}

type createRequest struct {
	Name            *string      `json:"name"`
	Description     *string      `json:"description"`
	SystemPrompt    *string      `json:"systemPrompt"`
	ProviderModelID *int64       `json:"providerModelId"`
	Status          *yesno.Value `json:"status"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Name == nil || r.Description == nil || r.SystemPrompt == nil ||
		r.ProviderModelID == nil || r.Status == nil {
		return CreateInput{}, fmt.Errorf("name, description, systemPrompt, providerModelId and status are required")
	}
	value, err := validateInput(*r.Name, *r.Description, *r.SystemPrompt, *r.ProviderModelID, *r.Status)
	if err != nil {
		return CreateInput{}, err
	}
	return CreateInput{
		Name: value.Name, Description: value.Description, SystemPrompt: value.SystemPrompt,
		ProviderModelID: value.ProviderModelID, Status: value.Status,
	}, nil
}

type updateRequest struct {
	Name            *string      `json:"name"`
	Description     *string      `json:"description"`
	SystemPrompt    *string      `json:"systemPrompt"`
	ProviderModelID *int64       `json:"providerModelId"`
	Status          *yesno.Value `json:"status"`
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Name == nil || r.Description == nil || r.SystemPrompt == nil ||
		r.ProviderModelID == nil || r.Status == nil {
		return UpdateInput{}, fmt.Errorf("name, description, systemPrompt, providerModelId and status are required")
	}
	value, err := validateInput(*r.Name, *r.Description, *r.SystemPrompt, *r.ProviderModelID, *r.Status)
	if err != nil {
		return UpdateInput{}, err
	}
	return UpdateInput{
		Name: value.Name, Description: value.Description, SystemPrompt: value.SystemPrompt,
		ProviderModelID: value.ProviderModelID, Status: value.Status,
	}, nil
}

type inputValues struct {
	Name            string
	Description     string
	SystemPrompt    string
	ProviderModelID int64
	Status          yesno.Value
}

func validateInput(name, description, systemPrompt string, providerModelID int64, status yesno.Value) (inputValues, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	systemPrompt = strings.TrimSpace(systemPrompt)
	if name == "" || utf8.RuneCountInString(name) > maxNameRunes {
		return inputValues{}, fmt.Errorf("name must be between 1 and %d characters", maxNameRunes)
	}
	if utf8.RuneCountInString(description) > maxDescriptionRunes {
		return inputValues{}, fmt.Errorf("description must be at most %d characters", maxDescriptionRunes)
	}
	if systemPrompt == "" || utf8.RuneCountInString(systemPrompt) > maxSystemPromptRunes {
		return inputValues{}, fmt.Errorf("systemPrompt must be between 1 and %d characters", maxSystemPromptRunes)
	}
	if providerModelID < 1 {
		return inputValues{}, fmt.Errorf("providerModelId is invalid")
	}
	if !yesno.IsValid(status) {
		return inputValues{}, fmt.Errorf("status is invalid")
	}
	return inputValues{
		Name: name, Description: description, SystemPrompt: systemPrompt,
		ProviderModelID: providerModelID, Status: status,
	}, nil
}

type statusRequest struct {
	Status *yesno.Value `json:"status"`
}

func (r statusRequest) input() (StatusInput, error) {
	if r.Status == nil || !yesno.IsValid(*r.Status) {
		return StatusInput{}, fmt.Errorf("status is invalid")
	}
	return StatusInput{Status: *r.Status}, nil
}

func parseListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "keyword" && key != "status") || len(entries) != 1 {
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
	}
	if utf8.RuneCountInString(query.Keyword) > 200 {
		return ListQuery{}, fmt.Errorf("keyword is too long")
	}
	if raw := strings.TrimSpace(values.Get("status")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 8)
		if err != nil {
			return ListQuery{}, fmt.Errorf("status is invalid")
		}
		status := yesno.Value(value)
		if !yesno.IsValid(status) {
			return ListQuery{}, fmt.Errorf("status is invalid")
		}
		query.Status = &status
	}
	return query, nil
}
