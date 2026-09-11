package log

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/message/sms/logVerification"
	"admin/server/internal/module/message/sms/recipientRule"
	"admin/server/internal/module/message/sms/template"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/phone"
	"gorm.io/gorm"
)

const (
	PermissionList   = "message:sms:list"
	PermissionDetail = "message:sms:detail"
)

// Item is the administrative log shape. It only ever carries the phone hint.
type Item struct {
	ID           int64   `json:"id"`
	PlatformID   int64   `json:"platformId"`
	Platform     string  `json:"platform"`
	UserID       *int64  `json:"userId"`
	Username     string  `json:"username"`
	Scene        string  `json:"scene"`
	TemplateID   int64   `json:"templateId"`
	ToPhoneHint  string  `json:"toPhoneHint"`
	Status       string  `json:"status"`
	RequestID    string  `json:"requestId"`
	SerialNo     string  `json:"serialNo"`
	Fee          int     `json:"fee"`
	ErrorCode    string  `json:"errorCode"`
	ErrorSummary string  `json:"errorSummary"`
	LatencyMS    int64   `json:"latencyMs"`
	SentAt       *string `json:"sentAt"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

type ListResult struct {
	List     []Item `json:"list"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// Detail additionally decrypts the phone and the verification code. It is only
// reachable with the message:sms:detail permission.
type Detail struct {
	Log                   Item    `json:"log"`
	ToPhone               string  `json:"toPhone"`
	VerificationCode      string  `json:"verificationCode"`
	VerificationExpiresAt *string `json:"verificationExpiresAt"`
}

// ListQuery is the validated administrative filter. Phone is the raw input and
// is normalized and HMAC'd before it reaches the repository.
type ListQuery struct {
	Page     int
	PageSize int
	Platform string
	Phone    string
	Scene    string
	Status   string
	From     *time.Time
	To       *time.Time
}

type repository interface {
	List(context.Context, Query) ([]ListRow, int64, error)
	FindByID(context.Context, int64) (ListRow, error)
}

type verificationSource interface {
	FindBySMSLog(context.Context, int64, int64) (logVerification.Model, error)
}

type Service struct {
	repository   repository
	verification verificationSource
	keys         *secretkey.KeyRing
}

func NewService(repository repository, verification verificationSource, keys *secretkey.KeyRing) *Service {
	return &Service{repository: repository, verification: verification, keys: keys}
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return ListResult{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	if query.Scene != "" {
		if err := template.ValidateScene(query.Scene); err != nil {
			return ListResult{}, apperror.InvalidRequest(fmt.Errorf("scene is invalid"))
		}
	}
	if query.Status != "" && query.Status != StatusPending && query.Status != StatusSent && query.Status != StatusFailed {
		return ListResult{}, apperror.InvalidRequest(fmt.Errorf("status is invalid"))
	}

	repositoryQuery := Query{Page: query.Page, PageSize: query.PageSize, Platform: query.Platform, Scene: query.Scene, Status: query.Status, From: query.From, To: query.To}
	if query.Phone != "" {
		if s.keys == nil {
			return ListResult{}, apperror.DependencyUnavailable(fmt.Errorf("sms keys are unavailable"))
		}
		normalized, err := phone.Normalize(query.Phone)
		if err != nil {
			return ListResult{}, apperror.InvalidRequest(fmt.Errorf("phone filter must be a mainland number"))
		}
		hmac := recipientRule.HMACValue(s.keys, normalized)
		repositoryQuery.PhoneToHMAC = &hmac
	}

	rows, total, err := s.repository.List(ctx, repositoryQuery)
	if err != nil {
		return ListResult{}, apperror.DependencyUnavailable(fmt.Errorf("sms log repository: %w", err))
	}
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, itemOf(row))
	}
	return ListResult{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Detail(ctx context.Context, id int64) (Detail, error) {
	if s.keys == nil {
		return Detail{}, apperror.DependencyUnavailable(fmt.Errorf("sms keys are unavailable"))
	}
	row, err := s.repository.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Detail{}, apperror.NotFound(err)
	}
	if err != nil {
		return Detail{}, apperror.DependencyUnavailable(fmt.Errorf("sms log repository: %w", err))
	}
	toPhone, err := decryptWith(s.keys, row.ToPhoneCiphertext)
	if err != nil {
		return Detail{}, apperror.DependencyUnavailable(fmt.Errorf("decrypt sms log phone: %w", err))
	}
	detail := Detail{Log: itemOf(row), ToPhone: toPhone}
	if verification, err := s.verification.FindBySMSLog(ctx, row.PlatformID, row.ID); err == nil {
		code, err := decryptWith(s.keys, verification.CodeCiphertext)
		if err != nil {
			return Detail{}, apperror.DependencyUnavailable(fmt.Errorf("decrypt sms verification code: %w", err))
		}
		expiresAt := verification.ExpiresAt.UTC().Format(time.RFC3339Nano)
		detail.VerificationCode = code
		detail.VerificationExpiresAt = &expiresAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Detail{}, apperror.DependencyUnavailable(fmt.Errorf("sms verification repository: %w", err))
	}
	return detail, nil
}

func itemOf(row ListRow) Item {
	item := Item{
		ID: row.ID, PlatformID: row.PlatformID, Platform: row.Platform,
		UserID: row.UserID, Username: row.Username, Scene: row.Scene,
		TemplateID: row.TemplateID, ToPhoneHint: row.ToPhoneHint, Status: row.Status,
		RequestID: row.RequestID, SerialNo: row.SerialNo, Fee: row.Fee,
		ErrorCode: row.ErrorCode, ErrorSummary: row.ErrorSummary, LatencyMS: row.LatencyMS,
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if row.SentAt != nil {
		sentAt := row.SentAt.UTC().Format(time.RFC3339Nano)
		item.SentAt = &sentAt
	}
	if item.Platform == "" {
		item.Platform = "-"
	}
	if item.Username == "" {
		item.Username = "-"
	}
	return item
}

func decryptWith(keys *secretkey.KeyRing, ciphertext string) (string, error) {
	return secretkey.DecryptSMSValue(keys.SMSEncryptionKey(), ciphertext)
}
