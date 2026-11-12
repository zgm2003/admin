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

const maxContentRunes = 20000

type SendInput struct {
	Content   string
	RequestID string
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

type sendMessageRequest struct {
	Content   *string `json:"content"`
	RequestID *string `json:"requestId"`
}

func (r sendMessageRequest) input() (SendInput, error) {
	if r.Content == nil || r.RequestID == nil {
		return SendInput{}, fmt.Errorf("content and requestId are required")
	}
	content := strings.TrimSpace(*r.Content)
	if content == "" || utf8.RuneCountInString(content) > maxContentRunes {
		return SendInput{}, fmt.Errorf("content must be between 1 and %d characters", maxContentRunes)
	}
	requestID := strings.TrimSpace(*r.RequestID)
	if _, err := uuid.Parse(requestID); err != nil {
		return SendInput{}, fmt.Errorf("requestId must be a UUID")
	}
	return SendInput{Content: content, RequestID: requestID}, nil
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
