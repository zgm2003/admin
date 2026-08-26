package role

import "time"

type emptyResponse struct{}

type idResponse struct {
	ID int64 `json:"id"`
}

type statusResponse struct {
	ID        int64 `json:"id"`
	IsEnabled int16 `json:"isEnabled"`
}

type defaultResponse struct {
	ID        int64 `json:"id"`
	IsDefault int16 `json:"isDefault"`
}

type permissionResultResponse struct {
	ID              int64 `json:"id"`
	PermissionCount int64 `json:"permissionCount"`
}
type listItemResponse struct {
	ID              int64  `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	IsDefault       int16  `json:"isDefault"`
	IsEnabled       int16  `json:"isEnabled"`
	UserCount       int64  `json:"userCount"`
	PermissionCount int64  `json:"permissionCount"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type listResponse struct {
	List     []listItemResponse `json:"list"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

type summaryResponse struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsDefault int16  `json:"isDefault"`
	IsEnabled int16  `json:"isEnabled"`
}

type permissionTreeResponse struct {
	ID        int64                    `json:"id"`
	ParentID  *int64                   `json:"parentId"`
	MenuType  string                   `json:"menuType"`
	Code      string                   `json:"code"`
	Name      string                   `json:"name"`
	IsEnabled int16                    `json:"isEnabled"`
	Children  []permissionTreeResponse `json:"children"`
}

type permissionsResponse struct {
	Role     summaryResponse          `json:"role"`
	MenuTree []permissionTreeResponse `json:"menuTree"`
	MenuIDs  []int64                  `json:"menuIds"`
}

func roleListResponse(items []ListItem, total int64, page, pageSize int) listResponse {
	rows := make([]listItemResponse, 0, len(items))
	for _, item := range items {
		rows = append(rows, listItemResponse{
			ID:              item.ID,
			Code:            item.Code,
			Name:            item.Name,
			IsDefault:       int16(item.IsDefault),
			IsEnabled:       int16(item.IsEnabled),
			UserCount:       item.UserCount,
			PermissionCount: item.PermissionCount,
			CreatedAt:       item.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt:       item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}

	return listResponse{
		List:     rows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}

func newPermissionsResponse(value Permissions) permissionsResponse {
	return permissionsResponse{
		Role: summaryResponse{
			ID:        value.Role.ID,
			Code:      value.Role.Code,
			Name:      value.Role.Name,
			IsDefault: int16(value.Role.IsDefault),
			IsEnabled: int16(value.Role.IsEnabled),
		},
		MenuTree: permissionTreeResponses(value.MenuTree),
		MenuIDs:  append(make([]int64, 0, len(value.MenuIDs)), value.MenuIDs...),
	}
}

func permissionTreeResponses(nodes []PermissionTreeNode) []permissionTreeResponse {
	result := make([]permissionTreeResponse, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, permissionTreeResponse{
			ID:        node.ID,
			ParentID:  node.ParentID,
			MenuType:  string(node.MenuType),
			Code:      node.Code,
			Name:      node.Name,
			IsEnabled: int16(node.IsEnabled),
			Children:  permissionTreeResponses(node.Children),
		})
	}

	return result
}
