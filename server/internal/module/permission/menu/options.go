package menu

import (
	"admin/server/internal/shared/i18n"
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

func formOptions(ctx context.Context) (menuFormOptions, error) {
	reserved := make([]string, 0, len(staticPagePaths))
	for path := range staticPagePaths {
		reserved = append(reserved, path)
	}
	sort.Strings(reserved)
	menuTypes, err := menuTypeOptions(ctx)
	if err != nil {
		return menuFormOptions{}, err
	}
	return menuFormOptions{MenuTypes: menuTypes, Constraints: menuFormConstraints{
		CodePattern: menuCodePattern.String(), I18nKeyPattern: menuI18nKeyPattern.String(), PathPattern: menuPathPattern.String(), ComponentPathPattern: menuComponentPathPattern.String(),
		NameMaxLength: menuTextMaxRunes, CodeMaxLength: menuTextMaxRunes, I18nKeyMaxLength: menuTextMaxRunes, PathMaxLength: menuPathMaxRunes, ReservedPagePaths: reserved,
	}}, nil
}

func menuTypeOptions(ctx context.Context) ([]option.Option[Type], error) {
	return option.ManyLocalized(ctx, []option.LocalizedEntry[Type]{
		{Value: TypeDirectory, Key: i18n.KeyMenuTypeDirectory},
		{Value: TypePage, Key: i18n.KeyMenuTypePage},
		{Value: TypeAction, Key: i18n.KeyMenuTypeAction},
	})
}
