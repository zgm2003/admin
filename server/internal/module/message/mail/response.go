package mail

import (
	"admin/server/internal/shared/pagination"
	"time"
)

type logResponse struct {
	ID           int64   `json:"id"`
	PlatformID   int64   `json:"platformId"`
	UserID       *int64  `json:"userId"`
	Scene        string  `json:"scene"`
	TemplateID   int     `json:"templateId"`
	ToEmail      string  `json:"toEmail"`
	Subject      string  `json:"subject"`
	Status       string  `json:"status"`
	RequestID    string  `json:"requestId"`
	MessageID    string  `json:"messageId"`
	ErrorCode    string  `json:"errorCode"`
	ErrorSummary string  `json:"errorSummary"`
	LatencyMs    int64   `json:"latencyMs"`
	SentAt       *string `json:"sentAt"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

type logDetailResponse struct {
	Log                   logResponse `json:"log"`
	VerificationCode      string      `json:"verificationCode"`
	VerificationExpiresAt *string     `json:"verificationExpiresAt"`
}

func formatResponseTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalResponseTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatResponseTime(*value)
	return &formatted
}

func logResponseFromModel(value Log) logResponse {
	return logResponse{
		ID: value.ID, PlatformID: value.PlatformID, UserID: value.UserID, Scene: value.Scene,
		TemplateID: value.TemplateID, ToEmail: value.ToEmail, Subject: value.Subject, Status: value.Status,
		RequestID: value.RequestID, MessageID: value.MessageID, ErrorCode: value.ErrorCode,
		ErrorSummary: value.ErrorSummary, LatencyMs: value.LatencyMs, SentAt: formatOptionalResponseTime(value.SentAt),
		CreatedAt: formatResponseTime(value.CreatedAt), UpdatedAt: formatResponseTime(value.UpdatedAt),
	}
}

func logResponsesFromModels(values []Log) []logResponse {
	result := make([]logResponse, 0, len(values))
	for _, value := range values {
		result = append(result, logResponseFromModel(value))
	}
	return result
}

func logListResponseFromModels(values []Log, total int64, page, pageSize int) pagination.Result[logResponse] {
	return pagination.Result[logResponse]{
		List: logResponsesFromModels(values), Total: total, Page: page, PageSize: pageSize,
	}
}

func logDetailResponseFromModel(value LogDetail) logDetailResponse {
	return logDetailResponse{
		Log: logResponseFromModel(value.Log), VerificationCode: value.VerificationCode,
		VerificationExpiresAt: formatOptionalResponseTime(value.VerificationExpiresAt),
	}
}
