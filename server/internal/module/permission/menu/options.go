package menu

import (
	"admin/server/internal/shared/option"
	"context"
	"sort"
)

type menuFormConstraints struct {
	CodePattern          string   `json:"codePattern"`
	I18nKeyPattern       string   `json:"i18nKeyPattern"`
	PathPattern          string   `json:"pathPattern"`
	ComponentPathPattern string   `json:"componentPathPattern"`
	NameMaxLength        int      `json:"nameMaxLength"`
	CodeMaxLength        int      `json:"codeMaxLength"`
	I18nKeyMaxLength     int      `json:"i18nKeyMaxLength"`
	PathMaxLength        int      `json:"pathMaxLength"`
	ReservedPagePaths    []string `json:"reservedPagePaths"`
}
type menuFormOptions struct {
	MenuTypes   []option.Option[Type] `json:"menuTypes"`
	Constraints menuFormConstraints   `json:"constraints"`
}

func formOptions(ctx context.Context) menuFormOptions {
	reserved := make([]string, 0, len(staticPagePaths))
	for path := range staticPagePaths {
		reserved = append(reserved, path)
	}
	sort.Strings(reserved)
	return menuFormOptions{MenuTypes: menuTypeOptions(ctx), Constraints: menuFormConstraints{
		CodePattern: menuCodePattern.String(), I18nKeyPattern: menuI18nKeyPattern.String(), PathPattern: menuPathPattern.String(), ComponentPathPattern: menuComponentPathPattern.String(),
		NameMaxLength: menuTextMaxRunes, CodeMaxLength: menuTextMaxRunes, I18nKeyMaxLength: menuTextMaxRunes, PathMaxLength: menuPathMaxRunes, ReservedPagePaths: reserved,
	}}
}

func menuTypeOptions(ctx context.Context) []option.Option[Type] {
	return []option.Option[Type]{
		option.New(ctx, TypeDirectory, "目录", "Directory"), option.New(ctx, TypePage, "页面", "Page"), option.New(ctx, TypeAction, "按钮权限", "Action permission"),
	}
}
