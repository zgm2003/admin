package notificationtask

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/module/message/notification"
	"admin/server/internal/shared/apperror"
)

type draftRequest struct {
	PlatformID   *int64                 `json:"platformId"`
	Title        *string                `json:"title"`
	ContentHTML  *string                `json:"contentHtml"`
	Variant      *notification.Variant  `json:"variant"`
	Priority     *notification.Priority `json:"priority"`
	LinkType     *notification.LinkType `json:"linkType"`
	Link         *string                `json:"link"`
	AudienceType *AudienceType          `json:"audienceType"`
	TargetIDs    *[]int64               `json:"targetIds"`
	ScheduledAt  *time.Time             `json:"scheduledAt"`
}

func (r draftRequest) input() (DraftInput, error) {
	if r.PlatformID == nil || r.Title == nil || r.ContentHTML == nil || r.Variant == nil || r.Priority == nil || r.LinkType == nil || r.Link == nil || r.AudienceType == nil || r.TargetIDs == nil {
		return DraftInput{}, apperror.InvalidRequest(fmt.Errorf("notification task fields are required"))
	}
	return DraftInput{PlatformID: *r.PlatformID, Title: *r.Title, ContentHTML: *r.ContentHTML, Variant: *r.Variant, Priority: *r.Priority, LinkType: *r.LinkType, Link: *r.Link, AudienceType: *r.AudienceType, TargetIDs: *r.TargetIDs, ScheduledAt: r.ScheduledAt}, nil
}

type optionQuery struct {
	Keyword string
	AfterID int64
	Limit   int
}

func parseListQuery(values url.Values) (ListQuery, error) {
	allowed := map[string]bool{
		"page": true, "pageSize": true, "platformId": true, "status": true,
		"audienceType": true, "keyword": true, "from": true, "to": true,
	}
	for key, entries := range values {
		if !allowed[key] || len(entries) != 1 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	query := ListQuery{Page: 1, PageSize: 20}
	var err error
	if value, ok := values["page"]; ok {
		query.Page, err = strconv.Atoi(value[0])
		if err != nil {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("page is invalid"))
		}
	}
	if value, ok := values["pageSize"]; ok {
		query.PageSize, err = strconv.Atoi(value[0])
		if err != nil {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("pageSize is invalid"))
		}
	}
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	if value, ok := values["platformId"]; ok {
		platformID, parseErr := strconv.ParseInt(value[0], 10, 64)
		if parseErr != nil || platformID <= 0 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("platformId is invalid"))
		}
		query.PlatformID = &platformID
	}
	if value, ok := values["status"]; ok {
		query.Status = Status(value[0])
		if !validStatus(query.Status) {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("status is invalid"))
		}
	}
	if value, ok := values["audienceType"]; ok {
		query.AudienceType = AudienceType(value[0])
		if query.AudienceType != AudienceUser && query.AudienceType != AudienceRole && query.AudienceType != AudiencePlatform {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("audienceType is invalid"))
		}
	}
	if value, ok := values["keyword"]; ok {
		query.Keyword = strings.TrimSpace(value[0])
		if len(query.Keyword) > 128 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("keyword too long"))
		}
	}
	for key, target := range map[string]**time.Time{"from": &query.From, "to": &query.To} {
		if value, ok := values[key]; ok {
			parsed, parseErr := time.Parse(time.RFC3339Nano, value[0])
			if parseErr != nil {
				return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("%s is invalid", key))
			}
			parsed = parsed.UTC()
			*target = &parsed
		}
	}
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("from must not be later than to"))
	}
	return query, nil
}

func parseOptionQuery(values url.Values) (optionQuery, error) {
	allowed := map[string]bool{"platformId": true, "keyword": true, "afterId": true, "limit": true}
	for key, entries := range values {
		if !allowed[key] || len(entries) != 1 {
			return optionQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	query := optionQuery{Limit: 50}
	var err error
	if value, ok := values["keyword"]; ok {
		query.Keyword = strings.TrimSpace(value[0])
		if len(query.Keyword) > 128 {
			return optionQuery{}, apperror.InvalidRequest(fmt.Errorf("keyword too long"))
		}
	}
	if value, ok := values["afterId"]; ok {
		query.AfterID, err = strconv.ParseInt(value[0], 10, 64)
		if err != nil || query.AfterID < 0 {
			return optionQuery{}, apperror.InvalidRequest(fmt.Errorf("afterId invalid"))
		}
	}
	if value, ok := values["limit"]; ok {
		query.Limit, err = strconv.Atoi(value[0])
		if err != nil || query.Limit < 1 || query.Limit > 50 {
			return optionQuery{}, apperror.InvalidRequest(fmt.Errorf("limit invalid"))
		}
	}
	return query, nil
}
