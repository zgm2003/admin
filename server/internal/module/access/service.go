package access

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"admin/server/internal/module/menu"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
)

const CodeAccessSnapshotInvalid = 14000

type store interface {
	FindSource(context.Context, int64) (Source, error)
	HasPermission(context.Context, int64, string) (bool, error)
}

type MenuNode struct {
	Code     string
	MenuType menu.Type
	Path     *string
	ViewKey  *string
	TitleKey string
	Icon     *string
	Children []MenuNode
}

type Snapshot struct {
	RoleCodes       []string
	MenuTree        []MenuNode
	PermissionCodes []string
}

type Service struct {
	store store
}

func NewService(store store) *Service {
	return &Service{store: store}
}

func (s *Service) Current(ctx context.Context, userID int64) (Snapshot, error) {
	if userID <= 0 {
		return Snapshot{}, apperror.InvalidRequest(fmt.Errorf("access snapshot requires a positive user ID"))
	}
	source, err := s.store.FindSource(ctx, userID)
	if err != nil {
		return Snapshot{}, apperror.DependencyUnavailable(err)
	}
	snapshot, err := buildSnapshot(source)
	if err != nil {
		return Snapshot{}, accessSnapshotInvalid(err)
	}
	return snapshot, nil
}

func (s *Service) Allowed(ctx context.Context, userID int64, permissionCode string) (bool, error) {
	if userID <= 0 {
		return false, apperror.InvalidRequest(fmt.Errorf("permission check requires a positive user ID"))
	}
	if strings.TrimSpace(permissionCode) == "" {
		return false, apperror.InvalidRequest(fmt.Errorf("permission check requires a permission code"))
	}
	allowed, err := s.store.HasPermission(ctx, userID, permissionCode)
	if err != nil {
		return false, apperror.DependencyUnavailable(err)
	}
	return allowed, nil
}

func buildSnapshot(source Source) (Snapshot, error) {
	menusByID := make(map[int64]menu.Menu, len(source.Menus))
	for _, item := range source.Menus {
		if item.ID <= 0 {
			return Snapshot{}, fmt.Errorf("menu has invalid ID %d", item.ID)
		}
		if _, exists := menusByID[item.ID]; exists {
			return Snapshot{}, fmt.Errorf("menu ID %d is duplicated", item.ID)
		}
		if item.MenuType != menu.TypeDirectory && item.MenuType != menu.TypePage && item.MenuType != menu.TypeAction {
			return Snapshot{}, fmt.Errorf("menu %d has invalid type %q", item.ID, item.MenuType)
		}
		if item.IsEnabled != yesno.Yes {
			return Snapshot{}, fmt.Errorf("menu %d is not enabled", item.ID)
		}
		menusByID[item.ID] = item
	}

	startIDs := make([]int64, 0, len(source.GrantedMenuIDs))
	if source.SuperAdmin {
		for _, item := range source.Menus {
			if item.MenuType == menu.TypePage || item.MenuType == menu.TypeAction {
				startIDs = append(startIDs, item.ID)
			}
		}
	} else {
		startIDs = append(startIDs, source.GrantedMenuIDs...)
	}

	selected := make(map[int64]menu.Menu)
	for _, startID := range startIDs {
		start, exists := menusByID[startID]
		if !exists {
			return Snapshot{}, fmt.Errorf("direct grant menu %d is missing", startID)
		}
		if start.MenuType == menu.TypeDirectory {
			return Snapshot{}, fmt.Errorf("directory menu %d was directly granted", startID)
		}

		visited := make(map[int64]struct{})
		currentID := startID
		for {
			if _, exists := visited[currentID]; exists {
				return Snapshot{}, fmt.Errorf("menu parent cycle contains ID %d", currentID)
			}
			visited[currentID] = struct{}{}
			current, exists := menusByID[currentID]
			if !exists {
				return Snapshot{}, fmt.Errorf("menu parent %d is missing", currentID)
			}
			selected[currentID] = current
			if current.ParentID == nil {
				break
			}
			currentID = *current.ParentID
		}
	}

	if err := validateSelectedMenus(selected); err != nil {
		return Snapshot{}, err
	}

	roleCodes := append(make([]string, 0, len(source.RoleCodes)), source.RoleCodes...)
	sort.Strings(roleCodes)
	permissionCodes := make([]string, 0)
	for _, item := range selected {
		if item.MenuType == menu.TypePage || item.MenuType == menu.TypeAction {
			permissionCodes = append(permissionCodes, item.Code)
		}
	}
	sort.Strings(permissionCodes)

	return Snapshot{
		RoleCodes:       roleCodes,
		MenuTree:        buildMenuTree(selected),
		PermissionCodes: permissionCodes,
	}, nil
}

func validateSelectedMenus(selected map[int64]menu.Menu) error {
	codes := make(map[string]int64, len(selected))
	paths := make(map[string]int64)
	for id, item := range selected {
		if strings.TrimSpace(item.Code) == "" {
			return fmt.Errorf("menu %d has an empty code", id)
		}
		if existingID, exists := codes[item.Code]; exists {
			return fmt.Errorf("menus %d and %d share code %q", existingID, id, item.Code)
		}
		codes[item.Code] = id
		if strings.TrimSpace(item.I18nKey) == "" {
			return fmt.Errorf("menu %d has an empty i18n key", id)
		}

		switch item.MenuType {
		case menu.TypeDirectory:
			if item.ViewKey != nil {
				return fmt.Errorf("directory menu %d has a view key", id)
			}
		case menu.TypePage:
			if item.Path == nil || strings.TrimSpace(*item.Path) == "" {
				return fmt.Errorf("page menu %d has no path", id)
			}
			if item.ViewKey == nil || strings.TrimSpace(*item.ViewKey) == "" {
				return fmt.Errorf("page menu %d has no view key", id)
			}
			if existingID, exists := paths[*item.Path]; exists {
				return fmt.Errorf("page menus %d and %d share path %q", existingID, id, *item.Path)
			}
			paths[*item.Path] = id
		case menu.TypeAction:
			if item.Path != nil || item.ViewKey != nil {
				return fmt.Errorf("action menu %d has a path or view key", id)
			}
			if item.ParentID == nil {
				return fmt.Errorf("action menu %d has no parent page", id)
			}
		default:
			return fmt.Errorf("menu %d has invalid type %q", id, item.MenuType)
		}

		if item.ParentID == nil {
			continue
		}
		parent, exists := selected[*item.ParentID]
		if !exists {
			return fmt.Errorf("selected menu %d has missing parent %d", id, *item.ParentID)
		}
		switch parent.MenuType {
		case menu.TypeDirectory:
			if item.MenuType != menu.TypeDirectory && item.MenuType != menu.TypePage {
				return fmt.Errorf("directory menu %d has invalid child type %q", parent.ID, item.MenuType)
			}
		case menu.TypePage:
			if item.MenuType != menu.TypeAction {
				return fmt.Errorf("page menu %d has invalid child type %q", parent.ID, item.MenuType)
			}
		case menu.TypeAction:
			return fmt.Errorf("action menu %d has child %d", parent.ID, item.ID)
		default:
			return fmt.Errorf("parent menu %d has invalid type %q", parent.ID, parent.MenuType)
		}
	}
	return nil
}

func buildMenuTree(selected map[int64]menu.Menu) []MenuNode {
	childrenByParent := make(map[int64][]menu.Menu)
	roots := make([]menu.Menu, 0)
	for _, item := range selected {
		if item.MenuType == menu.TypeAction {
			continue
		}
		if item.ParentID == nil {
			roots = append(roots, item)
			continue
		}
		childrenByParent[*item.ParentID] = append(childrenByParent[*item.ParentID], item)
	}
	sortMenus(roots)
	for parentID := range childrenByParent {
		sortMenus(childrenByParent[parentID])
	}

	result := make([]MenuNode, 0, len(roots))
	for _, root := range roots {
		result = append(result, buildMenuNode(root, childrenByParent))
	}
	return result
}

func buildMenuNode(item menu.Menu, childrenByParent map[int64][]menu.Menu) MenuNode {
	children := make([]MenuNode, 0, len(childrenByParent[item.ID]))
	for _, child := range childrenByParent[item.ID] {
		children = append(children, buildMenuNode(child, childrenByParent))
	}
	return MenuNode{
		Code:     item.Code,
		MenuType: item.MenuType,
		Path:     item.Path,
		ViewKey:  item.ViewKey,
		TitleKey: item.I18nKey,
		Icon:     item.Icon,
		Children: children,
	}
}

func sortMenus(menus []menu.Menu) {
	sort.Slice(menus, func(left, right int) bool {
		if menus[left].SortOrder != menus[right].SortOrder {
			return menus[left].SortOrder < menus[right].SortOrder
		}
		return menus[left].Code < menus[right].Code
	})
}

func accessSnapshotInvalid(cause error) *apperror.Error {
	return &apperror.Error{
		HTTPStatus: http.StatusInternalServerError,
		Code:       CodeAccessSnapshotInvalid,
		MessageKey: i18n.KeyAccessSnapshotInvalid,
		Cause:      cause,
	}
}
