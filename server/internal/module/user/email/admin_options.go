package email

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
	"context"
)

type emailAdminOptions struct {
	Actions []option.Option[ChangeAction] `json:"actions"`
}

func adminOptions(ctx context.Context) (emailAdminOptions, error) {
	actions, err := option.ManyLocalized(ctx, []option.LocalizedEntry[ChangeAction]{
		{Value: ActionChange, Key: i18n.KeyUserContactActionChange},
		{Value: ActionBind, Key: i18n.KeyUserContactActionBind},
	})
	if err != nil {
		return emailAdminOptions{}, err
	}
	return emailAdminOptions{Actions: actions}, nil
}
