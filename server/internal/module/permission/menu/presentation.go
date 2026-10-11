package menu

import (
	"admin/server/internal/shared/i18n"
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

func newMenuPresentation(ctx context.Context, item ManagedMenu) (menuPresentation, error) {
	value := menuPresentation{TypeLabel: string(item.MenuType), TypeTone: "info", VisibilityLabel: "隐藏", VisibilityTone: "info", StatusLabel: "已禁用", StatusTone: "info"}
	typeOptions, err := menuTypeOptions(ctx)
	if err != nil {
		return menuPresentation{}, err
	}
	for _, candidate := range typeOptions {
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
	value.VisibilityLabel, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyMenuVisibilityHidden, nil)
	if err != nil {
		return menuPresentation{}, err
	}
	if item.IsHidden == yesno.No {
		value.VisibilityLabel, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyMenuVisibilityVisible, nil)
		if err != nil {
			return menuPresentation{}, err
		}
		value.VisibilityTone = "success"
	}
	value.StatusLabel, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyMenuStatusDisabled, nil)
	if err != nil {
		return menuPresentation{}, err
	}
	value.StatusActionLabel, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyMenuActionEnable, nil)
	if err != nil {
		return menuPresentation{}, err
	}
	if item.IsEnabled == yesno.Yes {
		value.StatusLabel, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyMenuStatusEnabled, nil)
		if err != nil {
			return menuPresentation{}, err
		}
		value.StatusTone = "success"
		value.StatusActionLabel, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyMenuActionDisable, nil)
		if err != nil {
			return menuPresentation{}, err
		}
	}
	if item.IsProtected {
		value.ProtectionReason, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyMenuProtectedReason, nil)
		if err != nil {
			return menuPresentation{}, err
		}
	}
	return value, nil
}
