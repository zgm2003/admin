package role

import (
	"context"
	"fmt"

	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
)

type Actions struct {
	Update     bool `json:"update"`
	Status     bool `json:"status"`
	SetDefault bool `json:"setDefault"`
	Delete     bool `json:"delete"`
	Authorize  bool `json:"authorize"`
}
type ActionLabels struct {
	Update     string `json:"update"`
	Status     string `json:"status"`
	SetDefault string `json:"setDefault"`
	Delete     string `json:"delete"`
	Authorize  string `json:"authorize"`
}

func roleActions(ctx context.Context, item ListItem) (Actions, ActionLabels) {
	label := func(zh, en string) string { return option.New(ctx, "", zh, en).Label }
	actions := Actions{Update: !IsSystemCode(item.Code), Status: item.Code != CodeSuperAdmin && !(item.IsDefault == yesno.Yes && item.IsEnabled == yesno.Yes), SetDefault: item.Code != CodeSuperAdmin && item.IsDefault != yesno.Yes && item.IsEnabled == yesno.Yes, Delete: !IsSystemCode(item.Code) && item.IsDefault != yesno.Yes && item.UserCount == 0, Authorize: item.Code != CodeSuperAdmin}
	labels := ActionLabels{Update: label("编辑", "Edit"), Status: label("启用", "Enable"), SetDefault: label("设为默认", "Set as default"), Delete: label("删除", "Delete"), Authorize: label("授权", "Authorize")}
	if item.IsEnabled == yesno.Yes {
		labels.Status = label("禁用", "Disable")
	}
	if IsSystemCode(item.Code) {
		labels.Update = label("系统角色名称不可修改", "System role names cannot be changed")
		labels.Delete = label("系统角色不可删除", "System roles cannot be deleted")
	}
	if item.Code == CodeSuperAdmin {
		labels.Status = label("超级管理员必须保持启用", "The super administrator role must remain enabled")
		labels.SetDefault = label("超级管理员不能设为默认角色", "The super administrator role cannot be the default role")
		labels.Authorize = label("超级管理员拥有全部权限，无需授权", "Super administrators already have all permissions")
	} else {
		if item.IsDefault == yesno.Yes {
			if item.IsEnabled == yesno.Yes {
				labels.Status = label("默认角色不可禁用", "The default role cannot be disabled")
			}
			labels.SetDefault = label("当前已是默认角色", "This is already the default role")
			if !IsSystemCode(item.Code) {
				labels.Delete = label("默认角色不可删除", "The default role cannot be deleted")
			}
		} else if item.IsEnabled == yesno.No {
			labels.SetDefault = label("禁用角色不能设为默认", "Disabled roles cannot be the default role")
		}
		if !IsSystemCode(item.Code) && item.IsDefault != yesno.Yes && item.UserCount > 0 {
			labels.Delete = label(fmt.Sprintf("角色关联 %d 个用户，不可删除", item.UserCount), fmt.Sprintf("The role has %d users and cannot be deleted", item.UserCount))
		}
	}
	return actions, labels
}
