package loginlog

import (
	"admin/server/internal/shared/option"
	"context"
)

type loginLogAdminOptions struct {
	EventTypes []option.Option[EventType] `json:"eventTypes"`
	LoginTypes []option.Option[LoginType] `json:"loginTypes"`
}

func adminOptions(ctx context.Context) loginLogAdminOptions {
	return loginLogAdminOptions{
		EventTypes: []option.Option[EventType]{option.New(ctx, EventRegister, "注册", "Registration"), option.New(ctx, EventLogin, "登录", "Login"), option.New(ctx, EventLogout, "退出登录", "Logout")},
		LoginTypes: []option.Option[LoginType]{option.New(ctx, LoginPassword, "密码", "Password"), option.New(ctx, LoginEmail, "邮箱验证码", "Email code"), option.New(ctx, LoginPhone, "手机验证码", "Phone code")},
	}
}
