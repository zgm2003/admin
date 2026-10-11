package loginlog

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
	"context"
)

type loginLogAdminOptions struct {
	EventTypes []option.Option[EventType] `json:"eventTypes"`
	LoginTypes []option.Option[LoginType] `json:"loginTypes"`
}

func adminOptions(ctx context.Context) (loginLogAdminOptions, error) {
	eventTypes, err := option.ManyLocalized(ctx, []option.LocalizedEntry[EventType]{
		{Value: EventRegister, Key: i18n.KeyUserLoginEventRegister},
		{Value: EventLogin, Key: i18n.KeyUserLoginEventLogin},
		{Value: EventLogout, Key: i18n.KeyUserLoginEventLogout},
	})
	if err != nil {
		return loginLogAdminOptions{}, err
	}
	loginTypes, err := option.ManyLocalized(ctx, []option.LocalizedEntry[LoginType]{
		{Value: LoginPassword, Key: i18n.KeyLoginTypePassword},
		{Value: LoginEmail, Key: i18n.KeyLoginTypeEmail},
		{Value: LoginPhone, Key: i18n.KeyLoginTypePhone},
	})
	if err != nil {
		return loginLogAdminOptions{}, err
	}
	return loginLogAdminOptions{EventTypes: eventTypes, LoginTypes: loginTypes}, nil
}
