package log

import (
	"time"

	"admin/server/internal/shared/pagination"
)

type Response struct {
	ID           int64   `json:"id"`
	PlatformID   int64   `json:"platformId"`
	Platform     string  `json:"platform"`
	UserID       *int64  `json:"userId"`
	Username     string  `json:"username"`
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

type DetailResponse struct {
	Log                   Response `json:"log"`
	VerificationCode      string   `json:"verificationCode"`
	VerificationExpiresAt *string  `json:"verificationExpiresAt"`
}

func ListResponse(values []ListRow, total int64, page, pageSize int) pagination.Result[Response] {
	rows := make([]Response, 0, len(values))
	for _, value := range values {
		rows = append(rows, fromRow(value))
	}
	return pagination.Result[Response]{List: rows, Total: total, Page: page, PageSize: pageSize}
}

func NewDetailResponse(value Detail) DetailResponse {
	return DetailResponse{
		Log: fromRow(value.Log), VerificationCode: value.VerificationCode,
		VerificationExpiresAt: optionalTime(value.VerificationExpiresAt),
	}
}

func fromRow(value ListRow) Response {
	return Response{
		ID: value.ID, PlatformID: value.PlatformID, Platform: value.Platform, UserID: value.UserID,
		Username: value.Username, Scene: value.Scene, TemplateID: value.TemplateID, ToEmail: value.ToEmail,
		Subject: value.Subject, Status: value.Status, RequestID: value.RequestID, MessageID: value.MessageID,
		ErrorCode: value.ErrorCode, ErrorSummary: value.ErrorSummary, LatencyMs: value.LatencyMs,
		SentAt: optionalTime(value.SentAt), CreatedAt: formatTime(value.CreatedAt),
		UpdatedAt: formatTime(value.UpdatedAt),
	}
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func optionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}
