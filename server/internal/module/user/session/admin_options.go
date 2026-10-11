package session

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
	"context"
)

type sessionAdminOptions struct {
	Statuses []option.Option[SessionStatus] `json:"statuses"`
}

func adminOptions(ctx context.Context) (sessionAdminOptions, error) {
	statuses, err := option.ManyLocalized(ctx, []option.LocalizedEntry[SessionStatus]{
		{Value: SessionStatusActive, Key: i18n.KeyUserSessionStatusActive},
		{Value: SessionStatusExpired, Key: i18n.KeyUserSessionStatusExpired},
		{Value: SessionStatusRevoked, Key: i18n.KeyUserSessionStatusRevoked},
	})
	if err != nil {
		return sessionAdminOptions{}, err
	}
	return sessionAdminOptions{Statuses: statuses}, nil
}
