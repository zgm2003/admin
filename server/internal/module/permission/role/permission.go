package role

import (
	"fmt"
	"sort"
	"strings"

	"admin/server/internal/module/permission/menu"
	"admin/server/internal/shared/yesno"
)

type Summary struct {
	ID        int64
	Code      string
	Name      string
	IsDefault yesno.Value
	IsEnabled yesno.Value
}

type PermissionTreeNode struct {
	ID        int64
	ParentID  *int64
	MenuType  menu.Type
	Code      string
	Name      string
	IsEnabled yesno.Value
	Children  []PermissionTreeNode
}

type PermissionPlatform struct {
	ID        int64
	Code      string
	Name      string
	IsEnabled yesno.Value
	MenuTree  []PermissionTreeNode
}

type Permissions struct {
	Role      Summary
	Platforms []PermissionPlatform
	MenuIDs   []int64
}

type permissionIndex struct {
	byID         map[int64]menu.Menu
	children     map[int64][]int64
	roots        []int64
	pageByAction map[int64]int64
}

type platformCodeKey struct {
	PlatformID int64
	Code       string
}

func buildPermissionIndex(rows []menu.Menu) (permissionIndex, error) {
	index := permissionIndex{
		byID: make(map[int64]menu.Menu, len(rows)), children: make(map[int64][]int64),
		roots: make([]int64, 0), pageByAction: make(map[int64]int64),
	}
	codes := make(map[platformCodeKey]struct{}, len(rows))
	for _, row := range rows {
		if row.ID < 1 || row.PlatformID < 1 || row.DeletedAt.Valid || strings.TrimSpace(row.Code) == "" || strings.TrimSpace(row.Name) == "" || !yesno.IsValid(row.IsEnabled) {
			return permissionIndex{}, fmt.Errorf("menu %d has invalid stored values", row.ID)
		}
		if _, exists := index.byID[row.ID]; exists {
			return permissionIndex{}, fmt.Errorf("menu id %d is duplicated", row.ID)
		}
		codeKey := platformCodeKey{PlatformID: row.PlatformID, Code: row.Code}
		if _, exists := codes[codeKey]; exists {
			return permissionIndex{}, fmt.Errorf("menu code %s is duplicated", row.Code)
		}
		codes[codeKey] = struct{}{}
		index.byID[row.ID] = row
	}
	for _, row := range rows {
		if row.ParentID == nil {
			if row.MenuType == menu.TypeAction {
				return permissionIndex{}, fmt.Errorf("root menu %d is an action", row.ID)
			}
			index.roots = append(index.roots, row.ID)
			continue
		}
		parent, exists := index.byID[*row.ParentID]
		if !exists {
			return permissionIndex{}, fmt.Errorf("menu %d has missing parent", row.ID)
		}
		if parent.PlatformID != row.PlatformID {
			return permissionIndex{}, fmt.Errorf("menu %d and parent use different platforms", row.ID)
		}
		validChild := (parent.MenuType == menu.TypeDirectory && (row.MenuType == menu.TypeDirectory || row.MenuType == menu.TypePage)) ||
			(parent.MenuType == menu.TypePage && row.MenuType == menu.TypeAction)
		if !validChild {
			return permissionIndex{}, fmt.Errorf("menu %d has an illegal parent type", row.ID)
		}
		if row.MenuType == menu.TypeAction {
			index.pageByAction[row.ID] = parent.ID
		}
		index.children[parent.ID] = append(index.children[parent.ID], row.ID)
	}
	for id := range index.byID {
		visited := make(map[int64]struct{})
		current := id
		for {
			if _, exists := visited[current]; exists {
				return permissionIndex{}, fmt.Errorf("menu tree has a cycle at %d", current)
			}
			visited[current] = struct{}{}
			row := index.byID[current]
			if row.ParentID == nil {
				break
			}
			current = *row.ParentID
		}
	}
	index.sortIDs(index.roots)
	for parentID := range index.children {
		index.sortIDs(index.children[parentID])
	}
	return index, nil
}

func (index permissionIndex) sortIDs(ids []int64) {
	sort.Slice(ids, func(left, right int) bool {
		a, b := index.byID[ids[left]], index.byID[ids[right]]
		if a.SortOrder != b.SortOrder {
			return a.SortOrder < b.SortOrder
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.ID < b.ID
	})
}

func (index permissionIndex) tree(platformID int64) ([]PermissionTreeNode, error) {
	var build func(int64) (PermissionTreeNode, error)
	build = func(id int64) (PermissionTreeNode, error) {
		row, exists := index.byID[id]
		if !exists {
			return PermissionTreeNode{}, fmt.Errorf("menu %d is missing", id)
		}
		children := make([]PermissionTreeNode, 0, len(index.children[id]))
		for _, childID := range index.children[id] {
			child, err := build(childID)
			if err != nil {
				return PermissionTreeNode{}, err
			}
			children = append(children, child)
		}
		return PermissionTreeNode{ID: row.ID, ParentID: row.ParentID, MenuType: row.MenuType, Code: row.Code, Name: row.Name, IsEnabled: row.IsEnabled, Children: children}, nil
	}
	result := make([]PermissionTreeNode, 0, len(index.roots))
	for _, rootID := range index.roots {
		if index.byID[rootID].PlatformID != platformID {
			continue
		}
		root, err := build(rootID)
		if err != nil {
			return nil, err
		}
		result = append(result, root)
	}
	return result, nil
}

func (index permissionIndex) normalizeRequested(ids []int64) ([]int64, error) {
	selected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, duplicate := selected[id]; duplicate {
			return nil, fmt.Errorf("menu id %d is duplicated", id)
		}
		row, exists := index.byID[id]
		if !exists || (row.MenuType != menu.TypePage && row.MenuType != menu.TypeAction) {
			return nil, fmt.Errorf("menu id %d is not a grantable menu", id)
		}
		selected[id] = struct{}{}
	}
	for actionID, pageID := range index.pageByAction {
		if _, actionSelected := selected[actionID]; actionSelected {
			delete(selected, pageID)
		}
	}
	result := make([]int64, 0, len(selected))
	for id := range selected {
		result = append(result, id)
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result, nil
}

func (index permissionIndex) validateStored(grants []menu.RoleMenu) ([]int64, error) {
	ids := make([]int64, 0, len(grants))
	seen := make(map[int64]struct{}, len(grants))
	for _, grant := range grants {
		if grant.ID < 1 || grant.DeletedAt.Valid {
			return nil, fmt.Errorf("stored role grant is invalid")
		}
		if _, duplicate := seen[grant.MenuID]; duplicate {
			return nil, fmt.Errorf("stored menu grant %d is duplicated", grant.MenuID)
		}
		row, exists := index.byID[grant.MenuID]
		if !exists || (row.MenuType != menu.TypePage && row.MenuType != menu.TypeAction) {
			return nil, fmt.Errorf("stored menu grant %d is corrupt", grant.MenuID)
		}
		seen[grant.MenuID] = struct{}{}
		ids = append(ids, grant.MenuID)
	}
	sort.Slice(ids, func(left, right int) bool { return ids[left] < ids[right] })
	return ids, nil
}
