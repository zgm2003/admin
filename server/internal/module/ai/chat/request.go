package chat

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"admin/server/internal/shared/pagination"
)

const maxTitleRunes = 128

type CreateConversationInput struct {
	AgentID int64
	Title   string
}

type RenameConversationInput struct {
	Title string
}

type ListConversationsQuery struct {
	Page     int
	PageSize int
	Keyword  string
}

type createConversationRequest struct {
	AgentID *int64  `json:"agentId"`
	Title   *string `json:"title"`
}

func (r createConversationRequest) input() (CreateConversationInput, error) {
	if r.AgentID == nil || *r.AgentID < 1 {
		return CreateConversationInput{}, fmt.Errorf("agentId is invalid")
	}
	title := ""
	if r.Title != nil {
		title = strings.TrimSpace(*r.Title)
	}
	if title != "" && utf8.RuneCountInString(title) > maxTitleRunes {
		return CreateConversationInput{}, fmt.Errorf("title is too long")
	}
	return CreateConversationInput{AgentID: *r.AgentID, Title: title}, nil
}

type renameConversationRequest struct {
	Title *string `json:"title"`
}

func (r renameConversationRequest) input() (RenameConversationInput, error) {
	if r.Title == nil {
		return RenameConversationInput{}, fmt.Errorf("title is required")
	}
	title := strings.TrimSpace(*r.Title)
	if title == "" || utf8.RuneCountInString(title) > maxTitleRunes {
		return RenameConversationInput{}, fmt.Errorf("title must be between 1 and %d characters", maxTitleRunes)
	}
	return RenameConversationInput{Title: title}, nil
}

func parseConversationQuery(values url.Values) (ListConversationsQuery, error) {
	request, err := pagination.ParseRequest(values)
	if err != nil {
		return ListConversationsQuery{}, err
	}
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "keyword") || len(entries) != 1 {
			return ListConversationsQuery{}, fmt.Errorf("query invalid")
		}
	}
	keyword := strings.TrimSpace(values.Get("keyword"))
	if utf8.RuneCountInString(keyword) > 100 {
		return ListConversationsQuery{}, fmt.Errorf("keyword is too long")
	}
	return ListConversationsQuery{Page: request.Page, PageSize: request.PageSize, Keyword: keyword}, nil
}

func parseMessageLimit(values url.Values) (int32, int, error) {
	for key, entries := range values {
		if (key != "beforeSeq" && key != "limit") || len(entries) != 1 {
			return 0, 0, fmt.Errorf("query invalid")
		}
	}
	var before int32
	if raw := strings.TrimSpace(values.Get("beforeSeq")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || value < 0 {
			return 0, 0, fmt.Errorf("beforeSeq is invalid")
		}
		before = int32(value)
	}
	limit := 50
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, fmt.Errorf("limit is invalid")
		}
		limit = normalizeLimit(value)
	}
	return before, limit, nil
}
