package email

import (
	"admin/server/internal/shared/option"
	"context"
)

type emailAdminOptions struct {
	Actions []option.Option[ChangeAction] `json:"actions"`
}

func adminOptions(ctx context.Context) emailAdminOptions {
	return emailAdminOptions{
		Actions: []option.Option[ChangeAction]{option.New(ctx, ActionChange, "换绑", "Change"), option.New(ctx, ActionBind, "绑定", "Bind")},
	}
}
