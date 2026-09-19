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

func parseOptionQuery(values url.Values) (optionQuery, error) {
	allowed := map[string]bool{"keyword": true, "afterId": true, "limit": true}
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
