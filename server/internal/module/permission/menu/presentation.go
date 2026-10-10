package menu

import (
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
	"context"
)

type menuActions struct {
	Update            bool   `json:"update"`
	Status            bool   `json:"status"`
	Delete            bool   `json:"delete"`
	AddChild          bool   `json:"addChild"`
	AllowedChildTypes []Type `json:"allowedChildTypes"`
}

type menuPresentation struct {
	TypeLabel         string `json:"typeLabel"`
	TypeTone          string `json:"typeTone"`
	VisibilityLabel   string `json:"visibilityLabel"`
	VisibilityTone    string `json:"visibilityTone"`
	StatusLabel       string `json:"statusLabel"`
	StatusTone        string `json:"statusTone"`
	StatusActionLabel string `json:"statusActionLabel"`
	ProtectionReason  string `json:"protectionReason"`
	StatusReason      string `json:"statusReason"`
}

func newMenuActions(item ManagedMenu, ancestorDisabled bool) menuActions {
	children := make([]Type, 0, 2)
	for _, child := range []Type{TypeDirectory, TypePage, TypeAction} {
		if allowedMenuChild(item.MenuType, child) {
			children = append(children, child)
		}
	}
	canStatus := (!item.IsProtected || item.IsEnabled != yesno.Yes) && (item.IsEnabled == yesno.Yes || !ancestorDisabled)
	return menuActions{Update: true, Status: canStatus, Delete: !item.IsProtected, AddChild: len(children) > 0, AllowedChildTypes: children}
}

func newMenuPresentation(ctx context.Context, item ManagedMenu) menuPresentation {
	value := menuPresentation{TypeLabel: string(item.MenuType), TypeTone: "info", VisibilityLabel: "隐藏", VisibilityTone: "info", StatusLabel: "已禁用", StatusTone: "info"}
	for _, candidate := range menuTypeOptions(ctx) {
		if candidate.Value == item.MenuType {
			value.TypeLabel = candidate.Label
			break
		}
	}
	switch item.MenuType {
	case TypeDirectory:
		value.TypeTone = "primary"
	case TypePage:
		value.TypeTone = "success"
	case TypeAction:
		value.TypeTone = "warning"
	}
	value.VisibilityLabel = option.New(ctx, 0, "隐藏", "Hidden").Label
	if item.IsHidden == yesno.No {
		value.VisibilityLabel = option.New(ctx, 0, "显示", "Visible").Label
		value.VisibilityTone = "success"
	}
	value.StatusLabel = option.New(ctx, 0, "已禁用", "Disabled").Label
	value.StatusActionLabel = option.New(ctx, 0, "启用", "Enable").Label
	if item.IsEnabled == yesno.Yes {
		value.StatusLabel = option.New(ctx, 1, "已启用", "Enabled").Label
		value.StatusTone = "success"
		value.StatusActionLabel = option.New(ctx, 1, "禁用", "Disable").Label
	}
	if item.IsProtected {
		value.ProtectionReason = option.New(ctx, 1, "受保护菜单不可删除或禁用，仍可编辑展示信息和新增子节点", "Protected menus cannot be deleted or disabled; display information and child nodes remain editable").Label
	}
	return value
}
