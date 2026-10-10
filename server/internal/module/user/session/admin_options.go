package session

import (
	"admin/server/internal/shared/option"
	"context"
)

type sessionAdminOptions struct {
	Statuses []option.Option[SessionStatus] `json:"statuses"`
}

func adminOptions(ctx context.Context) sessionAdminOptions {
	return sessionAdminOptions{
		Statuses: []option.Option[SessionStatus]{option.New(ctx, SessionStatusActive, "活跃", "Active"), option.New(ctx, SessionStatusExpired, "已过期", "Expired"), option.New(ctx, SessionStatusRevoked, "已撤销", "Revoked")},
	}
}
