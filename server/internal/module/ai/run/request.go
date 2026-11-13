package run

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"admin/server/internal/shared/pagination"

	"github.com/google/uuid"
)

const (
	maxContentRunes = 20000
	maxAttachments  = 6
)

type AttachmentInput struct {
	ObjectKey string
}

type SendInput struct {
	Content     string
	RequestID   string
	Attachments []AttachmentInput
}

type RunListQuery struct {
	Page       int
	PageSize   int
	UserID     *int64
	Status     *RunStatus
	Keyword    string
	ViewAll    bool
	PlatformID int64
	SelfUserID int64
}

type attachmentRequest struct {
	ObjectKey *string `json:"objectKey"`
}

type sendMessageRequest struct {
	Content     *string             `json:"content"`
	RequestID   *string             `json:"requestId"`
	Attachments []attachmentRequest `json:"attachments"`
}

func (r sendMessageRequest) input() (SendInput, error) {
	if r.RequestID == nil {
		return SendInput{}, fmt.Errorf("requestId is required")
	}
	content := ""
	if r.Content != nil {
		content = strings.TrimSpace(*r.Content)
	}
	if utf8.RuneCountInString(content) > maxContentRunes {
		return SendInput{}, fmt.Errorf("content must be at most %d characters", maxContentRunes)
	}
	if len(r.Attachments) > maxAttachments {
		return SendInput{}, fmt.Errorf("attachments must be at most %d", maxAttachments)
	}
	attachments := make([]AttachmentInput, 0, len(r.Attachments))
	seen := make(map[string]struct{}, len(r.Attachments))
	for _, raw := range r.Attachments {
		if raw.ObjectKey == nil {
			return SendInput{}, fmt.Errorf("attachment objectKey is required")
		}
		objectKey := strings.TrimSpace(*raw.ObjectKey)
		if objectKey == "" || utf8.RuneCountInString(objectKey) > 512 {
			return SendInput{}, fmt.Errorf("attachment objectKey is invalid")
		}
		for _, character := range objectKey {
			if character < 0x20 || character == 0x7f {
				return SendInput{}, fmt.Errorf("attachment objectKey contains control characters")
			}
		}
		if _, exists := seen[objectKey]; exists {
			return SendInput{}, fmt.Errorf("attachment objectKey is duplicated")
		}
		seen[objectKey] = struct{}{}
		attachments = append(attachments, AttachmentInput{ObjectKey: objectKey})
	}
	if content == "" && len(attachments) == 0 {
		return SendInput{}, fmt.Errorf("content or attachments are required")
	}
	requestID := strings.TrimSpace(*r.RequestID)
	if _, err := uuid.Parse(requestID); err != nil {
		return SendInput{}, fmt.Errorf("requestId must be a UUID")
	}
	return SendInput{Content: content, RequestID: requestID, Attachments: attachments}, nil
}

func parseRunQuery(values url.Values, viewAll bool) (RunListQuery, error) {
	request, err := pagination.ParseRequest(values)
	if err != nil {
		return RunListQuery{}, err
	}
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "keyword" &&
			key != "userId" && key != "status") || len(entries) != 1 {
			return RunListQuery{}, fmt.Errorf("query invalid")
		}
	}
	keyword := strings.TrimSpace(values.Get("keyword"))
	if utf8.RuneCountInString(keyword) > 100 {
		return RunListQuery{}, fmt.Errorf("keyword is too long")
	}
	query := RunListQuery{Page: request.Page, PageSize: request.PageSize, Keyword: keyword, ViewAll: viewAll}
	if raw := strings.TrimSpace(values.Get("userId")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			return RunListQuery{}, fmt.Errorf("userId is invalid")
		}
		query.UserID = &id
	}
	if raw := strings.TrimSpace(values.Get("status")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 8)
		if err != nil {
			return RunListQuery{}, fmt.Errorf("status is invalid")
		}
		status := RunStatus(value)
		if status < RunQueued || status > RunTimeout {
			return RunListQuery{}, fmt.Errorf("status is invalid")
		}
		query.Status = &status
	}
	return query, nil
}
