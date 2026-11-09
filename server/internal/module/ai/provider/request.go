package provider

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
	defaultTimeoutMS = 60000
	maxTimeoutMS     = 300000
	defaultSortOrder = 0
	maxSortOrder     = 1000000
	maxAPIKeyLength  = 8192
)

type ListQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Vendor   Vendor
	Protocol Protocol
	Status   *yesno.Value
}

type CreateInput struct {
	Name      string
	Vendor    Vendor
	Protocol  Protocol
	BaseURL   string
	APIKey    string
	TimeoutMS int32
	SortOrder int32
	Status    yesno.Value
}

type UpdateInput struct {
	Name      string
	Vendor    Vendor
	Protocol  Protocol
	BaseURL   string
	APIKey    *string
	TimeoutMS int32
	SortOrder int32
	Status    yesno.Value
}

type StatusInput struct {
	Status yesno.Value
}

type createRequest struct {
	Name      *string      `json:"name"`
	Vendor    *Vendor      `json:"vendor"`
	Protocol  *Protocol    `json:"protocol"`
	BaseURL   *string      `json:"baseUrl"`
	APIKey    *string      `json:"apiKey"`
	TimeoutMS *int32       `json:"timeoutMs"`
	SortOrder *int32       `json:"sortOrder"`
	Status    *yesno.Value `json:"status"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Name == nil || r.Vendor == nil || r.Protocol == nil ||
		r.BaseURL == nil || r.APIKey == nil {
		return CreateInput{}, fmt.Errorf("name, vendor, protocol, baseUrl and apiKey are required")
	}
	name, err := validateName(*r.Name)
	if err != nil {
		return CreateInput{}, fmt.Errorf("name: %w", err)
	}
	if !r.Vendor.Valid() {
		return CreateInput{}, fmt.Errorf("vendor is invalid")
	}
	if !r.Protocol.Valid() {
		return CreateInput{}, fmt.Errorf("protocol is invalid")
	}
	baseURL, err := validateBaseURL(*r.BaseURL)
	if err != nil {
		return CreateInput{}, fmt.Errorf("baseUrl: %w", err)
	}
	apiKey, err := validateAPIKey(*r.APIKey, true)
	if err != nil {
		return CreateInput{}, fmt.Errorf("apiKey: %w", err)
	}
	timeout, err := optionalTimeout(r.TimeoutMS)
	if err != nil {
		return CreateInput{}, err
	}
	sortOrder, err := optionalSortOrder(r.SortOrder)
	if err != nil {
		return CreateInput{}, err
	}
	status := yesno.No
	if r.Status != nil {
		if !yesno.IsValid(*r.Status) {
			return CreateInput{}, fmt.Errorf("status is invalid")
		}
		status = *r.Status
	}
	return CreateInput{
		Name: name, Vendor: *r.Vendor, Protocol: *r.Protocol,
		BaseURL: baseURL, APIKey: apiKey, TimeoutMS: timeout, SortOrder: sortOrder, Status: status,
	}, nil
}

type updateRequest struct {
	Name      *string      `json:"name"`
	Vendor    *Vendor      `json:"vendor"`
	Protocol  *Protocol    `json:"protocol"`
	BaseURL   *string      `json:"baseUrl"`
	APIKey    *string      `json:"apiKey"`
	TimeoutMS *int32       `json:"timeoutMs"`
	SortOrder *int32       `json:"sortOrder"`
	Status    *yesno.Value `json:"status"`
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Name == nil || r.Vendor == nil || r.Protocol == nil || r.BaseURL == nil ||
		r.TimeoutMS == nil || r.SortOrder == nil || r.Status == nil {
		return UpdateInput{}, fmt.Errorf("name, vendor, protocol, baseUrl, timeoutMs, sortOrder and status are required")
	}
	name, err := validateName(*r.Name)
	if err != nil {
		return UpdateInput{}, fmt.Errorf("name: %w", err)
	}
	if !r.Vendor.Valid() {
		return UpdateInput{}, fmt.Errorf("vendor is invalid")
	}
	if !r.Protocol.Valid() {
		return UpdateInput{}, fmt.Errorf("protocol is invalid")
	}
	baseURL, err := validateBaseURL(*r.BaseURL)
	if err != nil {
		return UpdateInput{}, fmt.Errorf("baseUrl: %w", err)
	}
	var apiKey *string
	if r.APIKey != nil {
		key, err := validateAPIKey(*r.APIKey, false)
		if err != nil {
			return UpdateInput{}, fmt.Errorf("apiKey: %w", err)
		}
		if key != "" {
			apiKey = &key
		}
	}
	if !yesno.IsValid(*r.Status) {
		return UpdateInput{}, fmt.Errorf("status is invalid")
	}
	if *r.TimeoutMS < 1000 || *r.TimeoutMS > maxTimeoutMS {
		return UpdateInput{}, fmt.Errorf("timeoutMs must be between 1000 and %d", maxTimeoutMS)
	}
	if *r.SortOrder < 0 || *r.SortOrder > maxSortOrder {
		return UpdateInput{}, fmt.Errorf("sortOrder must be between 0 and %d", maxSortOrder)
	}
	return UpdateInput{
		Name: name, Vendor: *r.Vendor, Protocol: *r.Protocol, BaseURL: baseURL,
		APIKey: apiKey, TimeoutMS: *r.TimeoutMS, SortOrder: *r.SortOrder, Status: *r.Status,
	}, nil
}

type statusRequest struct {
	Status *yesno.Value `json:"status"`
}

func (r statusRequest) input() (StatusInput, error) {
	if r.Status == nil || !yesno.IsValid(*r.Status) {
		return StatusInput{}, fmt.Errorf("status is required")
	}
	return StatusInput{Status: *r.Status}, nil
}

func parseListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "keyword" && key != "vendor" &&
			key != "protocol" && key != "status") || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("query invalid")
		}
	}
	request, err := pagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	query := ListQuery{
		Page:     request.Page,
		PageSize: request.PageSize,
		Keyword:  strings.TrimSpace(values.Get("keyword")),
	}
	if utf8.RuneCountInString(query.Keyword) > 200 {
		return ListQuery{}, fmt.Errorf("keyword is too long")
	}
	if raw := strings.TrimSpace(values.Get("vendor")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 16)
		if err != nil {
			return ListQuery{}, fmt.Errorf("vendor is invalid")
		}
		query.Vendor = Vendor(value)
		if !query.Vendor.Valid() {
			return ListQuery{}, fmt.Errorf("vendor is invalid")
		}
	}
	if raw := strings.TrimSpace(values.Get("protocol")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 16)
		if err != nil {
			return ListQuery{}, fmt.Errorf("protocol is invalid")
		}
		query.Protocol = Protocol(value)
		if !query.Protocol.Valid() {
			return ListQuery{}, fmt.Errorf("protocol is invalid")
		}
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

func validateName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 128 {
		return "", fmt.Errorf("must be between 1 and 128 characters")
	}
	return value, nil
}

func validateBaseURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 2048 || strings.ContainsAny(value, " \t\r\n") {
		return "", fmt.Errorf("must be a valid https URL")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("must be a valid https URL without query, fragment or credentials")
	}
	return value, nil
}

func validateAPIKey(value string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return "", fmt.Errorf("is required")
		}
		return "", nil
	}
	if len(value) > maxAPIKeyLength {
		return "", fmt.Errorf("is too long")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("contains invalid characters")
	}
	return value, nil
}

func optionalTimeout(value *int32) (int32, error) {
	if value == nil {
		return defaultTimeoutMS, nil
	}
	if *value < 1000 || *value > maxTimeoutMS {
		return 0, fmt.Errorf("timeoutMs must be between 1000 and %d", maxTimeoutMS)
	}
	return *value, nil
}

func optionalSortOrder(value *int32) (int32, error) {
	if value == nil {
		return defaultSortOrder, nil
	}
	if *value < 0 || *value > maxSortOrder {
		return 0, fmt.Errorf("sortOrder must be between 0 and %d", maxSortOrder)
	}
	return *value, nil
}
