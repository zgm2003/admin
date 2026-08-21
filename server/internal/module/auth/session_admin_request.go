package auth

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"admin/server/internal/module/authclient"
	"admin/server/internal/shared/apperror"
)

type sessionAdminRequest struct {
	IDs []int64 `json:"ids"`
}

func parseSessionAdminQuery(values url.Values) (AdminSessionQuery, error) {
	allowed := map[string]struct{}{`page`: {}, `pageSize`: {}, `username`: {}, `platform`: {}, `status`: {}}
	for key, entries := range values {
		if _, ok := allowed[key]; !ok || len(entries) != 1 {
			return AdminSessionQuery{}, apperror.InvalidRequest(fmt.Errorf(`invalid or repeated query parameter`))
		}
	}
	query := AdminSessionQuery{Page: 1, PageSize: 20}
	var err error
	if value, ok := values[`page`]; ok {
		query.Page, err = strconv.Atoi(value[0])
		if err != nil {
			return AdminSessionQuery{}, apperror.InvalidRequest(fmt.Errorf(`page is invalid`))
		}
	}
	if value, ok := values[`pageSize`]; ok {
		query.PageSize, err = strconv.Atoi(value[0])
		if err != nil {
			return AdminSessionQuery{}, apperror.InvalidRequest(fmt.Errorf(`pageSize is invalid`))
		}
	}
	if value, ok := values[`username`]; ok {
		query.Username = strings.TrimSpace(value[0])
		if len(query.Username) > 254 {
			return AdminSessionQuery{}, apperror.InvalidRequest(fmt.Errorf(`username is too long`))
		}
	}
	if value, ok := values[`platform`]; ok {
		query.Platform = strings.TrimSpace(value[0])
		if err := authclient.ValidatePlatform(query.Platform); err != nil {
			return AdminSessionQuery{}, apperror.InvalidRequest(err)
		}
	}
	if value, ok := values[`status`]; ok {
		query.Status = SessionStatus(value[0])
	}
	if err := validateAdminSessionQuery(query); err != nil {
		return AdminSessionQuery{}, err
	}
	return query, nil
}

func (r sessionAdminRequest) ids() ([]int64, error) {
	if r.IDs == nil || len(r.IDs) == 0 || len(r.IDs) > 100 {
		return nil, apperror.InvalidRequest(fmt.Errorf(`ids must contain 1 to 100 items`))
	}
	for _, id := range r.IDs {
		if id < 1 {
			return nil, apperror.InvalidRequest(fmt.Errorf(`ids must be positive integers`))
		}
	}
	return append([]int64(nil), r.IDs...), nil
}

func parseSessionID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, apperror.InvalidRequest(fmt.Errorf(`session id is invalid`))
	}
	return id, nil
}
