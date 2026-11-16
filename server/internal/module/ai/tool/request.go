package tool

import (
	"encoding/json"
	"fmt"
)

type statusRequest struct {
	Status *int16 `json:"status"`
}

func (r statusRequest) input() (Status, error) {
	if r.Status == nil {
		return 0, fmt.Errorf("status is required")
	}
	status := Status(*r.Status)
	if !status.Valid() {
		return 0, fmt.Errorf("status is invalid")
	}
	return status, nil
}

type createRequest struct {
	Code        *string         `json:"code"`
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Category    *string         `json:"category"`
	Parameters  json.RawMessage `json:"parameters"`
	Method      *string         `json:"method"`
	URL         *string         `json:"url"`
	TimeoutMS   *int32          `json:"timeoutMs"`
	Status      *int16          `json:"status"`
	SortOrder   *int32          `json:"sortOrder"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Code == nil || r.Name == nil {
		return CreateInput{}, fmt.Errorf("code and name are required")
	}
	status := StatusEnabled
	if r.Status != nil {
		status = Status(*r.Status)
	}
	input := CreateInput{
		Code: *r.Code, Name: *r.Name, Parameters: r.Parameters,
		Status: status,
	}
	input.Description = stringValue(r.Description)
	input.Category = stringValue(r.Category)
	input.Method = stringValue(r.Method)
	input.URL = stringValue(r.URL)
	if input.Method == "" {
		input.Method = "POST"
	}
	if r.TimeoutMS != nil {
		input.TimeoutMS = *r.TimeoutMS
	}
	if r.SortOrder != nil {
		input.SortOrder = *r.SortOrder
	}
	return input, nil
}

type updateRequest struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Category    *string         `json:"category"`
	Parameters  json.RawMessage `json:"parameters"`
	Method      *string         `json:"method"`
	URL         *string         `json:"url"`
	TimeoutMS   *int32          `json:"timeoutMs"`
	Status      *int16          `json:"status"`
	SortOrder   *int32          `json:"sortOrder"`
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Name == nil || r.Status == nil {
		return UpdateInput{}, fmt.Errorf("name and status are required")
	}
	input := UpdateInput{
		Name: *r.Name, Parameters: r.Parameters, Status: Status(*r.Status),
	}
	input.Description = stringValue(r.Description)
	input.Category = stringValue(r.Category)
	input.Method = stringValue(r.Method)
	input.URL = stringValue(r.URL)
	if input.Method == "" {
		input.Method = "POST"
	}
	if r.TimeoutMS != nil {
		input.TimeoutMS = *r.TimeoutMS
	}
	if r.SortOrder != nil {
		input.SortOrder = *r.SortOrder
	}
	return input, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
