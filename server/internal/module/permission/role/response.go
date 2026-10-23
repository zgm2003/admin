package role

import "time"

type emptyResponse struct{}

type idResponse struct {
	ID int64 `json:"id"`
}

type statusResponse struct {
	ID        int64 `json:"id"`
	Status int16 `json:"status"`
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
	Actions         Actions      `json:"actions"`
	ActionLabels    ActionLabels `json:"actionLabels"`
	ID              int64        `json:"id"`
	Code            string       `json:"code"`
	Name            string       `json:"name"`
	IsDefault       int16        `json:"isDefault"`
	Status       int16        `json:"status"`
	UserCount       int64        `json:"userCount"`
	PermissionCount int64        `json:"permissionCount"`
	CreatedAt       string       `json:"createdAt"`
	UpdatedAt       string       `json:"updatedAt"`
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
	Status int16  `json:"status"`
}

type permissionTreeResponse struct {
	ID        int64                    `json:"id"`
	ParentID  *int64                   `json:"parentId"`
	MenuType  string                   `json:"menuType"`
	Code      string                   `json:"code"`
	Name      string                   `json:"name"`
	Status int16                    `json:"status"`
	Children  []permissionTreeResponse `json:"children"`
}

type permissionPlatformResponse struct {
	ID        int64                    `json:"id"`
	Code      string                   `json:"code"`
	Name      string                   `json:"name"`
	Status int16                    `json:"status"`
	MenuTree  []permissionTreeResponse `json:"menuTree"`
}

type permissionsResponse struct {
	Role      summaryResponse              `json:"role"`
	Platforms []permissionPlatformResponse `json:"platforms"`
	MenuIDs   []int64                      `json:"menuIds"`
}

func roleListResponse(items []ListItem, total int64, page, pageSize int) listResponse {
	rows := make([]listItemResponse, 0, len(items))
	for _, item := range items {
		rows = append(rows, listItemResponse{
			Actions: item.Actions, ActionLabels: item.ActionLabels,
			ID:              item.ID,
			Code:            item.Code,
			Name:            item.Name,
			IsDefault:       int16(item.IsDefault),
			Status:       int16(item.Status),
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
			Status: int16(value.Role.Status),
		},
		Platforms: permissionPlatformResponses(value.Platforms),
		MenuIDs:   append(make([]int64, 0, len(value.MenuIDs)), value.MenuIDs...),
	}
}

func permissionPlatformResponses(platforms []PermissionPlatform) []permissionPlatformResponse {
	result := make([]permissionPlatformResponse, 0, len(platforms))
	for _, platform := range platforms {
		result = append(result, permissionPlatformResponse{
			ID: platform.ID, Code: platform.Code, Name: platform.Name, Status: int16(platform.Status),
			MenuTree: permissionTreeResponses(platform.MenuTree),
		})
	}
	return result
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
			Status: int16(node.Status),
			Children:  permissionTreeResponses(node.Children),
		})
	}

	return result
}
