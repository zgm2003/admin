package access

type currentResponse struct {
	RoleCodes       []string           `json:"roleCodes"`
	MenuTree        []menuNodeResponse `json:"menuTree"`
	PermissionCodes []string           `json:"permissionCodes"`
}

type menuNodeResponse struct {
	Code          string             `json:"code"`
	MenuType      MenuType           `json:"menuType"`
	Path          *string            `json:"path"`
	ComponentPath *string            `json:"componentPath"`
	I18nKey       string             `json:"i18nKey"`
	Icon          *string            `json:"icon"`
	IsHidden      int16              `json:"isHidden"`
	Children      []menuNodeResponse `json:"children"`
}

func newCurrentResponse(snapshot Snapshot) currentResponse {
	menuTree := make([]menuNodeResponse, 0, len(snapshot.MenuTree))
	for _, node := range snapshot.MenuTree {
		menuTree = append(menuTree, newMenuNodeResponse(node))
	}
	return currentResponse{
		RoleCodes:       append(make([]string, 0, len(snapshot.RoleCodes)), snapshot.RoleCodes...),
		MenuTree:        menuTree,
		PermissionCodes: append(make([]string, 0, len(snapshot.PermissionCodes)), snapshot.PermissionCodes...),
	}
}

func newMenuNodeResponse(node MenuNode) menuNodeResponse {
	children := make([]menuNodeResponse, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, newMenuNodeResponse(child))
	}
	return menuNodeResponse{
		Code: node.Code, MenuType: node.MenuType, Path: node.Path, ComponentPath: node.ComponentPath,
		I18nKey: node.I18nKey, Icon: node.Icon, IsHidden: node.IsHidden, Children: children,
	}
}
