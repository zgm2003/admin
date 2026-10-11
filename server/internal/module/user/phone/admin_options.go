package phone

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
	"context"
)

type phoneAdminOptions struct {
	Actions []option.Option[ChangeAction] `json:"actions"`
}

func adminOptions(ctx context.Context) (phoneAdminOptions, error) {
	actions, err := option.ManyLocalized(ctx, []option.LocalizedEntry[ChangeAction]{
		{Value: ActionChange, Key: i18n.KeyUserContactActionChange},
		{Value: ActionBind, Key: i18n.KeyUserContactActionBind},
	})
	if err != nil {
		return phoneAdminOptions{}, err
	}
	return phoneAdminOptions{Actions: actions}, nil
}
