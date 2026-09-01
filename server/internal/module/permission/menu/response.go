package menu

import (
	"time"

	"admin/server/internal/shared/yesno"
)

type managedMenuResponse struct {
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
	IsEnabled     int16                 `json:"isEnabled"`
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
	IsEnabled int16  `json:"isEnabled"`
}

type menuCatalogResponse struct {
	Platforms []platformOptionResponse `json:"platforms"`
	MenuTree  []managedMenuResponse    `json:"menuTree"`
}

type menuIDResponse struct {
	ID int64 `json:"id"`
}

type menuStatusResponse struct {
	ID        int64 `json:"id"`
	IsEnabled int16 `json:"isEnabled"`
}

func newManagedMenuResponses(items []ManagedMenu) []managedMenuResponse {
	result := make([]managedMenuResponse, 0, len(items))
	for _, item := range items {
		result = append(result, newManagedMenuResponse(item))
	}
	return result
}

func newManagedMenuResponse(item ManagedMenu) managedMenuResponse {
	children := make([]managedMenuResponse, 0, len(item.Children))
	for _, child := range item.Children {
		children = append(children, newManagedMenuResponse(child))
	}
	return managedMenuResponse{
		ID: item.ID, PlatformID: item.PlatformID, PlatformCode: item.PlatformCode, PlatformName: item.PlatformName,
		ParentID: item.ParentID, MenuType: item.MenuType, Name: item.Name, Code: item.Code,
		I18nKey: item.I18nKey, Path: item.Path, ComponentPath: item.ComponentPath, Icon: item.Icon, Remark: item.Remark,
		SortOrder: item.SortOrder, IsEnabled: int16(item.IsEnabled), IsHidden: int16(item.IsHidden),
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano), IsProtected: protectedValue(item.IsProtected), Children: children,
	}
}

func newMenuCatalogResponse(catalog Catalog) menuCatalogResponse {
	platforms := make([]platformOptionResponse, 0, len(catalog.Platforms))
	for _, platform := range catalog.Platforms {
		platforms = append(platforms, platformOptionResponse{
			ID: platform.ID, Code: platform.Code, Name: platform.Name, IsEnabled: int16(platform.IsEnabled),
		})
	}
	return menuCatalogResponse{Platforms: platforms, MenuTree: newManagedMenuResponses(catalog.MenuTree)}
}

func protectedValue(isProtected bool) int16 {
	if isProtected {
		return int16(yesno.Yes)
	}
	return int16(yesno.No)
}
