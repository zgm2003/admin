package menu

import (
	"context"
	"time"

	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
)

type managedMenuResponse struct {
	Presentation  menuPresentation      `json:"presentation"`
	Actions       menuActions           `json:"actions"`
	ID            int64                 `json:"id"`
	PlatformID    int64                 `json:"platformId"`
	PlatformCode  string                `json:"platformCode"`
	PlatformName  string                `json:"platformName"`
	ParentID      *int64                `json:"parentId"`
	MenuType      Type                  `json:"menuType"`
	Name          string                `json:"name"`
	Code          string                `json:"code"`
	I18nKey       *string               `json:"i18nKey"`
	Path          *string               `json:"path"`
	ComponentPath *string               `json:"componentPath"`
	Icon          *string               `json:"icon"`
	Remark        *string               `json:"remark"`
	SortOrder     int                   `json:"sortOrder"`
	Status     int16                 `json:"status"`
	IsHidden      int16                 `json:"isHidden"`
	CreatedAt     string                `json:"createdAt"`
	UpdatedAt     string                `json:"updatedAt"`
	IsProtected   int16                 `json:"isProtected"`
	Children      []managedMenuResponse `json:"children"`
}

type platformOptionResponse struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Status int16  `json:"status"`
}

type menuCatalogResponse struct {
	AllowedRootTypes []Type                   `json:"allowedRootTypes"`
	Platforms        []platformOptionResponse `json:"platforms"`
	MenuTree         []managedMenuResponse    `json:"menuTree"`
}

type menuIDResponse struct {
	ID int64 `json:"id"`
}

type menuStatusResponse struct {
	ID        int64 `json:"id"`
	Status int16 `json:"status"`
}

func newManagedMenuResponses(ctx context.Context, items []ManagedMenu) ([]managedMenuResponse, error) {
	result := make([]managedMenuResponse, 0, len(items))
	for _, item := range items {
		value, err := newManagedMenuResponse(ctx, item, false)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func newManagedMenuResponse(ctx context.Context, item ManagedMenu, ancestorDisabled bool) (managedMenuResponse, error) {
	children := make([]managedMenuResponse, 0, len(item.Children))
	for _, child := range item.Children {
		value, err := newManagedMenuResponse(ctx, child, ancestorDisabled || item.Status != yesno.Yes)
		if err != nil {
			return managedMenuResponse{}, err
		}
		children = append(children, value)
	}
	presentation, err := newMenuPresentation(ctx, item)
	if err != nil {
		return managedMenuResponse{}, err
	}
	if ancestorDisabled && item.Status != yesno.Yes {
		presentation.StatusReason, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyMenuParentDisabledReason, nil)
		if err != nil {
			return managedMenuResponse{}, err
		}
	} else if item.IsProtected && item.Status == yesno.Yes {
		presentation.StatusReason = presentation.ProtectionReason
	}
	return managedMenuResponse{
		Presentation: presentation, Actions: newMenuActions(item, ancestorDisabled),
		ID: item.ID, PlatformID: item.PlatformID, PlatformCode: item.PlatformCode, PlatformName: item.PlatformName,
		ParentID: item.ParentID, MenuType: item.MenuType, Name: item.Name, Code: item.Code,
		I18nKey: item.I18nKey, Path: item.Path, ComponentPath: item.ComponentPath, Icon: item.Icon, Remark: item.Remark,
		SortOrder: item.SortOrder, Status: int16(item.Status), IsHidden: int16(item.IsHidden),
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano), IsProtected: protectedValue(item.IsProtected), Children: children,
	}, nil
}

func newMenuCatalogResponse(ctx context.Context, catalog Catalog) (menuCatalogResponse, error) {
	platforms := make([]platformOptionResponse, 0, len(catalog.Platforms))
	for _, platform := range catalog.Platforms {
		platforms = append(platforms, platformOptionResponse{
			ID: platform.ID, Code: platform.Code, Name: platform.Name, Status: int16(platform.Status),
		})
	}
	tree, err := newManagedMenuResponses(ctx, catalog.MenuTree)
	if err != nil {
		return menuCatalogResponse{}, err
	}
	return menuCatalogResponse{AllowedRootTypes: []Type{TypeDirectory, TypePage}, Platforms: platforms, MenuTree: tree}, nil
}

func protectedValue(isProtected bool) int16 {
	if isProtected {
		return int16(yesno.Yes)
	}
	return int16(yesno.No)
}
