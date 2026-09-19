package notification

import (
	"fmt"
	"net/url"
	"strconv"

	"admin/server/internal/shared/apperror"
)

func parseMailboxQuery(values url.Values, platformID, userID int64) (MailboxQuery, error) {
	allowed := map[string]struct{}{"beforeId": {}, "limit": {}, "filter": {}, "variant": {}, "priority": {}}
	for key, entries := range values {
		if _, ok := allowed[key]; !ok || len(entries) != 1 {
			return MailboxQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	query := MailboxQuery{PlatformID: platformID, UserID: userID, Limit: 20, Filter: MailboxAll}
	var err error
	if value, ok := values["beforeId"]; ok {
		query.BeforeID, err = strconv.ParseInt(value[0], 10, 64)
		if err != nil || query.BeforeID < 0 {
			return MailboxQuery{}, apperror.InvalidRequest(fmt.Errorf("beforeId is invalid"))
		}
	}
	if value, ok := values["limit"]; ok {
		query.Limit, err = strconv.Atoi(value[0])
		if err != nil || query.Limit < 1 || query.Limit > 50 {
			return MailboxQuery{}, apperror.InvalidRequest(fmt.Errorf("limit is invalid"))
		}
	}
	if value, ok := values["filter"]; ok {
		query.Filter = MailboxFilter(value[0])
	}
	if value, ok := values["variant"]; ok {
		query.Variant = Variant(value[0])
	}
	if value, ok := values["priority"]; ok {
		query.Priority = Priority(value[0])
	}
	if query.Filter != MailboxAll && query.Filter != MailboxUnread {
		return MailboxQuery{}, apperror.InvalidRequest(fmt.Errorf("filter is invalid"))
	}
	if query.Variant != "" && query.Variant != VariantInfo && query.Variant != VariantSuccess && query.Variant != VariantWarning && query.Variant != VariantError {
		return MailboxQuery{}, apperror.InvalidRequest(fmt.Errorf("variant is invalid"))
	}
	if query.Priority != "" && query.Priority != PriorityNormal && query.Priority != PriorityUrgent {
		return MailboxQuery{}, apperror.InvalidRequest(fmt.Errorf("priority is invalid"))
	}
	return query, nil
}
