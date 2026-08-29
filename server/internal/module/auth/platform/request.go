package authplatform

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

type createRequest struct {
	Code                   *string      `json:"code"`
	Name                   *string      `json:"name"`
	AccessTTLSeconds       *int         `json:"accessTTLSeconds"`
	RefreshTTLSeconds      *int         `json:"refreshTTLSeconds"`
	SessionCacheTTLSeconds *int         `json:"sessionCacheTTLSeconds"`
	AccessCacheTTLSeconds  *int         `json:"accessCacheTTLSeconds"`
	BindDevice             *yesno.Value `json:"bindDevice"`
	BindIP                 *yesno.Value `json:"bindIP"`
	MaxSessions            *int16       `json:"maxSessions"`
	AllowRegister          *yesno.Value `json:"allowRegister"`
	IsEnabled              *yesno.Value `json:"isEnabled"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Code == nil || r.Name == nil || r.AccessTTLSeconds == nil || r.RefreshTTLSeconds == nil ||
		r.SessionCacheTTLSeconds == nil || r.AccessCacheTTLSeconds == nil || r.BindDevice == nil ||
		r.BindIP == nil || r.MaxSessions == nil || r.AllowRegister == nil || r.IsEnabled == nil {
		return CreateInput{}, apperror.InvalidRequest(fmt.Errorf("every authentication platform field is required"))
	}
	if !yesno.IsValid(*r.BindDevice) || !yesno.IsValid(*r.BindIP) || !yesno.IsValid(*r.AllowRegister) || !yesno.IsValid(*r.IsEnabled) {
		return CreateInput{}, apperror.InvalidRequest(fmt.Errorf("authentication platform Yes/No value is invalid"))
	}
	return CreateInput{
		Code: *r.Code, Name: *r.Name,
		AccessTTLSeconds: *r.AccessTTLSeconds, RefreshTTLSeconds: *r.RefreshTTLSeconds,
		SessionCacheTTLSeconds: *r.SessionCacheTTLSeconds, AccessCacheTTLSeconds: *r.AccessCacheTTLSeconds,
		BindDevice: *r.BindDevice, BindIP: *r.BindIP, MaxSessions: *r.MaxSessions,
		AllowRegister: *r.AllowRegister, IsEnabled: *r.IsEnabled,
	}, nil
}

type updateRequest struct {
	Name                   *string      `json:"name"`
	AccessTTLSeconds       *int         `json:"accessTTLSeconds"`
	RefreshTTLSeconds      *int         `json:"refreshTTLSeconds"`
	SessionCacheTTLSeconds *int         `json:"sessionCacheTTLSeconds"`
	AccessCacheTTLSeconds  *int         `json:"accessCacheTTLSeconds"`
	BindDevice             *yesno.Value `json:"bindDevice"`
	BindIP                 *yesno.Value `json:"bindIP"`
	MaxSessions            *int16       `json:"maxSessions"`
	AllowRegister          *yesno.Value `json:"allowRegister"`
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Name == nil || r.AccessTTLSeconds == nil || r.RefreshTTLSeconds == nil || r.SessionCacheTTLSeconds == nil ||
		r.AccessCacheTTLSeconds == nil || r.BindDevice == nil || r.BindIP == nil || r.MaxSessions == nil || r.AllowRegister == nil {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("every authentication platform policy field is required"))
	}
	if !yesno.IsValid(*r.BindDevice) || !yesno.IsValid(*r.BindIP) || !yesno.IsValid(*r.AllowRegister) {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("authentication platform Yes/No value is invalid"))
	}
	return UpdateInput{
		Name: *r.Name, AccessTTLSeconds: *r.AccessTTLSeconds, RefreshTTLSeconds: *r.RefreshTTLSeconds,
		SessionCacheTTLSeconds: *r.SessionCacheTTLSeconds, AccessCacheTTLSeconds: *r.AccessCacheTTLSeconds,
		BindDevice: *r.BindDevice, BindIP: *r.BindIP, MaxSessions: *r.MaxSessions, AllowRegister: *r.AllowRegister,
	}, nil
}

type statusRequest struct {
	IsEnabled *yesno.Value `json:"isEnabled"`
}

func (r statusRequest) value() (yesno.Value, error) {
	if r.IsEnabled == nil || !yesno.IsValid(*r.IsEnabled) {
		return 0, apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
	}
	return *r.IsEnabled, nil
}

func parsePlatformID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, apperror.InvalidRequest(fmt.Errorf("authentication platform id must be a positive integer"))
	}
	return id, nil
}

func parseListQuery(values url.Values) (ListQuery, error) {
	allowed := map[string]struct{}{"page": {}, "pageSize": {}, "keyword": {}, "isEnabled": {}}
	for key, entries := range values {
		if _, ok := allowed[key]; !ok || len(entries) != 1 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("invalid or repeated query parameter"))
		}
	}
	page, err := requiredQueryInt(values, "page")
	if err != nil {
		return ListQuery{}, err
	}
	pageSize, err := requiredQueryInt(values, "pageSize")
	if err != nil {
		return ListQuery{}, err
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	query := ListQuery{Page: page, PageSize: pageSize}
	if entries, ok := values["keyword"]; ok {
		query.Keyword = strings.TrimSpace(entries[0])
		if utf8.RuneCountInString(query.Keyword) > 64 {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("keyword is too long"))
		}
	}
	if entries, ok := values["isEnabled"]; ok {
		parsed, parseErr := strconv.ParseInt(entries[0], 10, 16)
		value := yesno.Value(parsed)
		if parseErr != nil || !yesno.IsValid(value) {
			return ListQuery{}, apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
		}
		query.IsEnabled = &value
	}
	return query, nil
}

func requiredQueryInt(values url.Values, key string) (int, error) {
	entries, ok := values[key]
	if !ok || len(entries) != 1 {
		return 0, apperror.InvalidRequest(fmt.Errorf("%s is required once", key))
	}
	value, err := strconv.Atoi(entries[0])
	if err != nil {
		return 0, apperror.InvalidRequest(err)
	}
	return value, nil
}
