package phone

import (
	"admin/server/internal/shared/option"
	"context"
)

type phoneAdminOptions struct {
	Actions []option.Option[ChangeAction] `json:"actions"`
}

func adminOptions(ctx context.Context) phoneAdminOptions {
	return phoneAdminOptions{
		Actions: []option.Option[ChangeAction]{option.New(ctx, ActionChange, "换绑", "Change"), option.New(ctx, ActionBind, "绑定", "Bind")},
	}
}
