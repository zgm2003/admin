package menu

import "time"

type managedMenuResponse struct {
	ID            int64                 `json:"id"`
	ParentID      *int64                `json:"parentId"`
	MenuType      Type                  `json:"menuType"`
	Code          string                `json:"code"`
	I18nKey       string                `json:"i18nKey"`
	Path          *string               `json:"path"`
	ComponentPath *string               `json:"componentPath"`
	Icon          *string               `json:"icon"`
	SortOrder     int                   `json:"sortOrder"`
	IsEnabled     int16                 `json:"isEnabled"`
	IsHidden      int16                 `json:"isHidden"`
	CreatedAt     string                `json:"createdAt"`
	UpdatedAt     string                `json:"updatedAt"`
	Children      []managedMenuResponse `json:"children"`
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
		ID: item.ID, ParentID: item.ParentID, MenuType: item.MenuType, Code: item.Code,
		I18nKey: item.I18nKey, Path: item.Path, ComponentPath: item.ComponentPath, Icon: item.Icon,
		SortOrder: item.SortOrder, IsEnabled: int16(item.IsEnabled), IsHidden: int16(item.IsHidden),
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano), Children: children,
	}
}
