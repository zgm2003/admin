package provider

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
)

type Actions struct {
	Update bool `json:"update"`
	Status bool `json:"status"`
	Delete bool `json:"delete"`
}

type SafeValue struct {
	ID                  int64       `json:"id"`
	Name                string      `json:"name"`
	Vendor              Vendor      `json:"vendor"`
	VendorName          string      `json:"vendorName"`
	Protocol            Protocol    `json:"protocol"`
	ProtocolName        string      `json:"protocolName"`
	BaseURL             string      `json:"baseUrl"`
	TimeoutMS           int32       `json:"timeoutMs"`
	SortOrder           int32       `json:"sortOrder"`
	TokenRateMultiplier string      `json:"tokenRateMultiplier"`
	Status              yesno.Value `json:"status"`
	StatusName          string      `json:"statusName"`
	APIKeyConfigured    bool        `json:"apiKeyConfigured"`
	Actions             Actions     `json:"actions"`
	CreatedAt           time.Time   `json:"createdAt"`
	UpdatedAt           time.Time   `json:"updatedAt"`
}

func safeValue(ctx context.Context, row Model) (SafeValue, error) {
	if row.ID < 1 || !row.Vendor.Valid() || !row.Protocol.Valid() || !yesno.IsValid(row.Status) {
		return SafeValue{}, dependency(fmt.Errorf("provider row has invalid selectable values"))
	}
	if _, err := validateName(row.Name); err != nil {
		return SafeValue{}, dependency(fmt.Errorf("provider row has invalid name: %w", err))
	}
	if _, err := validateBaseURL(row.BaseURL); err != nil {
		return SafeValue{}, dependency(fmt.Errorf("provider row has invalid base URL: %w", err))
	}
	if row.APIKeyCiphertext == "" || row.EncryptionKeyID == "" {
		return SafeValue{}, dependency(fmt.Errorf("provider row has no encrypted credential"))
	}
	if row.TimeoutMS < 1000 || row.TimeoutMS > maxTimeoutMS || row.SortOrder < 0 || row.SortOrder > maxSortOrder {
		return SafeValue{}, dependency(fmt.Errorf("provider row has invalid timeout or sort order"))
	}
	if row.TokenRatePPM < 0 || row.TokenRatePPM > 100000000 {
		return SafeValue{}, dependency(fmt.Errorf("provider row has invalid token rate multiplier"))
	}
	status := option.New(ctx, row.Status, "停用", "Disabled")
	if row.Status == yesno.Yes {
		status = option.New(ctx, row.Status, "启用", "Enabled")
	}
	return SafeValue{
		ID: row.ID, Name: row.Name,
		Vendor: row.Vendor, VendorName: row.Vendor.Label(),
		Protocol:     row.Protocol,
		ProtocolName: option.New(ctx, row.Protocol, protocolLabelZH(row.Protocol), protocolLabelEN(row.Protocol)).Label,
		BaseURL:      row.BaseURL, TimeoutMS: row.TimeoutMS, SortOrder: row.SortOrder,
		TokenRateMultiplier: formatRateMultiplier(row.TokenRatePPM),
		Status:              row.Status, StatusName: status.Label,
		APIKeyConfigured: true,
		Actions:          Actions{Update: true, Status: true, Delete: true},
		CreatedAt:        row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func protocolLabelZH(protocol Protocol) string {
	switch protocol {
	case ProtocolOpenAI:
		return "OpenAI Chat 兼容"
	case ProtocolAnthropic:
		return "Anthropic"
	case ProtocolGemini:
		return "Gemini"
	case ProtocolOpenAIResponses:
		return "OpenAI Responses"
	default:
		return ""
	}
}

func protocolLabelEN(protocol Protocol) string {
	switch protocol {
	case ProtocolOpenAI:
		return "OpenAI Chat compatible"
	case ProtocolAnthropic:
		return "Anthropic"
	case ProtocolGemini:
		return "Gemini"
	case ProtocolOpenAIResponses:
		return "OpenAI Responses"
	default:
		return ""
	}
}
