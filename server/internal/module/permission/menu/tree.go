package menu

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"admin/server/internal/shared/yesno"
)

var (
	errMenuTreeInvalid = errors.New("menu tree data is invalid")
	errMenuParent      = errors.New("menu parent is invalid")
	errMenuCycle       = errors.New("menu tree contains a cycle")
	errMenuStructure   = errors.New("menu structure conflicts")
	errMenuFields      = errors.New("menu fields are invalid")
)

var (
	menuCodePattern          = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?::[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$`)
	menuI18nKeyPattern       = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-zA-Z0-9]*)+$`)
	menuPathPattern          = regexp.MustCompile(`^/[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$`)
	menuComponentPathPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$`)
)

var staticPagePaths = map[string]struct{}{
	"/login": {}, "/register": {}, "/dashboard": {},
}

var menuIconNames = map[string]struct{}{
	"lucide:activity": {}, "lucide:bell": {}, "lucide:bot": {}, "lucide:brain-circuit": {},
	"lucide:circle-dollar-sign": {}, "lucide:cloud": {}, "lucide:cloud-upload": {}, "lucide:cpu": {},
	"lucide:database": {}, "lucide:file-stack": {}, "lucide:folder": {}, "lucide:gauge": {},
	"lucide:hard-drive": {}, "lucide:house": {}, "lucide:images": {}, "lucide:key-round": {},
	"lucide:layout-dashboard": {}, "lucide:list-tree": {}, "lucide:lock-keyhole": {},
	"lucide:message-square-more": {}, "lucide:monitor-smartphone": {}, "lucide:panel-left": {},
	"lucide:scroll-text": {}, "lucide:server": {}, "lucide:settings-2": {}, "lucide:shield-check": {},
	"lucide:sparkles": {}, "lucide:user-cog": {}, "lucide:user-round": {}, "lucide:user-round-cog": {},
	"lucide:user-circle": {}, "lucide:users": {}, "lucide:users-round": {}, "lucide:wallet-cards": {},
}

type menuIndex struct {
	byID     map[int64]Menu
	children map[int64][]int64
	roots    []int64
}

type platformValueKey struct {
	PlatformID int64
	Value      string
}

type menuParentDisabledViolation struct {
	code string
}

func (e *menuParentDisabledViolation) Error() string {
	return fmt.Sprintf("menu %s has a disabled ancestor", e.code)
}

func buildMenuIndex(menus []Menu) (menuIndex, error) {
	index := menuIndex{
		byID:     make(map[int64]Menu, len(menus)),
		children: make(map[int64][]int64),
		roots:    make([]int64, 0),
	}
	codeSet := make(map[platformValueKey]struct{}, len(menus))
	pathSet := make(map[platformValueKey]struct{}, len(menus))
	for _, item := range menus {
		if _, exists := index.byID[item.ID]; exists {
			return menuIndex{}, fmt.Errorf("%w: duplicate id %d", errMenuTreeInvalid, item.ID)
		}
		if err := validateStoredMenu(item); err != nil {
			return menuIndex{}, fmt.Errorf("%w: %w", errMenuTreeInvalid, err)
		}
		codeKey := platformValueKey{PlatformID: item.PlatformID, Value: item.Code}
		if _, exists := codeSet[codeKey]; exists {
			return menuIndex{}, fmt.Errorf("%w: duplicate code %s", errMenuTreeInvalid, item.Code)
		}
		codeSet[codeKey] = struct{}{}
		if item.Path != nil {
			pathKey := platformValueKey{PlatformID: item.PlatformID, Value: *item.Path}
			if _, exists := pathSet[pathKey]; exists {
				return menuIndex{}, fmt.Errorf("%w: duplicate page path %s", errMenuTreeInvalid, *item.Path)
			}
			pathSet[pathKey] = struct{}{}
		}
		index.byID[item.ID] = item
	}

	for _, item := range menus {
		if item.ParentID == nil {
			if item.MenuType == TypeAction {
				return menuIndex{}, fmt.Errorf("%w: root %d is an action", errMenuTreeInvalid, item.ID)
			}
			index.roots = append(index.roots, item.ID)
			continue
		}
		parent, exists := index.byID[*item.ParentID]
		if !exists {
			return menuIndex{}, fmt.Errorf("%w: menu %d has an orphan parent %d", errMenuTreeInvalid, item.ID, *item.ParentID)
		}
		if parent.PlatformID != item.PlatformID {
			return menuIndex{}, fmt.Errorf("%w: menu %d and parent %d use different platforms", errMenuTreeInvalid, item.ID, parent.ID)
		}
		if !allowedMenuChild(parent.MenuType, item.MenuType) {
			return menuIndex{}, fmt.Errorf("%w: menu %d cannot be a child of %d", errMenuTreeInvalid, item.ID, parent.ID)
		}
		index.children[parent.ID] = append(index.children[parent.ID], item.ID)
	}

	for id := range index.byID {
		visited := make(map[int64]struct{})
		current := id
		for {
			if _, exists := visited[current]; exists {
				return menuIndex{}, fmt.Errorf("%w: cycle at %d", errMenuTreeInvalid, current)
			}
			visited[current] = struct{}{}
			item := index.byID[current]
			if item.ParentID == nil {
				break
			}
			current = *item.ParentID
		}
	}

	sortMenuIDs(index.roots, index.byID)
	for parentID, children := range index.children {
		sortMenuIDs(children, index.byID)
		index.children[parentID] = children
	}
	return index, nil
}

func (index menuIndex) descendants(id int64) ([]int64, error) {
	if _, exists := index.byID[id]; !exists {
		return nil, fmt.Errorf("%w: menu %d is missing", errMenuTreeInvalid, id)
	}
	result := make([]int64, 0)
	visited := map[int64]struct{}{id: {}}
	stack := append([]int64(nil), index.children[id]...)
	for len(stack) > 0 {
		last := len(stack) - 1
		childID := stack[last]
		stack = stack[:last]
		if _, exists := visited[childID]; exists {
			return nil, fmt.Errorf("%w: cycle at %d", errMenuCycle, childID)
		}
		visited[childID] = struct{}{}
		result = append(result, childID)
		children := index.children[childID]
		for childIndex := len(children) - 1; childIndex >= 0; childIndex-- {
			stack = append(stack, children[childIndex])
		}
	}
	return result, nil
}

func (index menuIndex) ancestors(id int64) ([]int64, error) {
	item, exists := index.byID[id]
	if !exists {
		return nil, fmt.Errorf("%w: menu %d is missing", errMenuTreeInvalid, id)
	}
	result := make([]int64, 0)
	visited := map[int64]struct{}{id: {}}
	for item.ParentID != nil {
		parentID := *item.ParentID
		if _, exists := visited[parentID]; exists {
			return nil, fmt.Errorf("%w: cycle at %d", errMenuCycle, parentID)
		}
		parent, exists := index.byID[parentID]
		if !exists {
			return nil, fmt.Errorf("%w: orphan parent %d", errMenuTreeInvalid, parentID)
		}
		visited[parentID] = struct{}{}
		result = append(result, parentID)
		item = parent
	}
	return result, nil
}

func (index menuIndex) validateEnabledAncestors() error {
	for id, item := range index.byID {
		if item.IsEnabled != yesno.Yes {
			continue
		}
		ancestors, err := index.ancestors(id)
		if err != nil {
			return err
		}
		for _, ancestorID := range ancestors {
			if index.byID[ancestorID].IsEnabled != yesno.Yes {
				return &menuParentDisabledViolation{code: item.Code}
			}
		}
	}
	return nil
}

func (index menuIndex) buildManagedTree() ([]ManagedMenu, error) {
	result := make([]ManagedMenu, 0, len(index.roots))
	var build func(int64) (ManagedMenu, error)
	build = func(id int64) (ManagedMenu, error) {
		item, exists := index.byID[id]
		if !exists {
			return ManagedMenu{}, fmt.Errorf("%w: menu %d is missing", errMenuTreeInvalid, id)
		}
		childrenIDs := index.children[id]
		children := make([]ManagedMenu, 0, len(childrenIDs))
		for _, childID := range childrenIDs {
			child, err := build(childID)
			if err != nil {
				return ManagedMenu{}, err
			}
			children = append(children, child)
		}
		return ManagedMenu{
			ID: item.ID, PlatformID: item.PlatformID, ParentID: item.ParentID, MenuType: item.MenuType, Name: item.Name, Code: item.Code,
			I18nKey: item.I18nKey, Path: item.Path, ComponentPath: item.ComponentPath, Icon: item.Icon,
			SortOrder: item.SortOrder, IsEnabled: item.IsEnabled, IsHidden: item.IsHidden,
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, IsProtected: IsProtectedCode(item.Code), Children: children,
		}, nil
	}
	for _, rootID := range index.roots {
		root, err := build(rootID)
		if err != nil {
			return nil, err
		}
		result = append(result, root)
	}
	return result, nil
}

func validateStoredMenu(item Menu) error {
	if item.ID < 1 || item.PlatformID < 1 || !yesno.IsValid(item.IsEnabled) || !yesno.IsValid(item.IsHidden) || !validMenuName(item.Name) || !validMenuCode(item.Code) {
		return fmt.Errorf("%w: code or stored scalar is invalid", errMenuFields)
	}
	if item.Icon != nil && !validMenuIcon(*item.Icon) {
		return fmt.Errorf("%w: icon is invalid", errMenuFields)
	}
	switch item.MenuType {
	case TypeDirectory:
		if item.I18nKey == nil || !validMenuI18nKey(*item.I18nKey) || item.Path != nil || item.ComponentPath != nil {
			return fmt.Errorf("%w: directory render fields are invalid", errMenuFields)
		}
	case TypePage:
		if item.I18nKey == nil || !validMenuI18nKey(*item.I18nKey) || item.Path == nil || item.ComponentPath == nil || !validMenuPath(*item.Path) || !validMenuComponentPath(*item.ComponentPath) {
			return fmt.Errorf("%w: page render fields are invalid", errMenuFields)
		}
	case TypeAction:
		if item.I18nKey != nil || item.Path != nil || item.ComponentPath != nil || item.Icon != nil || item.IsHidden != yesno.Yes {
			return fmt.Errorf("%w: action render fields are invalid", errMenuFields)
		}
	default:
		return fmt.Errorf("%w: menu type is invalid", errMenuFields)
	}
	return nil
}

func allowedMenuChild(parent, child Type) bool {
	switch parent {
	case TypeDirectory:
		return child == TypeDirectory || child == TypePage
	case TypePage:
		return child == TypeAction
	default:
		return false
	}
}

func sortMenuIDs(ids []int64, byID map[int64]Menu) {
	sort.SliceStable(ids, func(left, right int) bool {
		leftItem := byID[ids[left]]
		rightItem := byID[ids[right]]
		if leftItem.SortOrder != rightItem.SortOrder {
			return leftItem.SortOrder < rightItem.SortOrder
		}
		if leftItem.Code != rightItem.Code {
			return leftItem.Code < rightItem.Code
		}
		return leftItem.ID < rightItem.ID
	})
}

func validMenuCode(value string) bool {
	return value == strings.TrimSpace(value) && utf8.RuneCountInString(value) <= 128 && menuCodePattern.MatchString(value)
}

func validMenuName(value string) bool {
	count := utf8.RuneCountInString(value)
	return value == strings.TrimSpace(value) && count >= 1 && count <= 128
}

func validMenuI18nKey(value string) bool {
	return value == strings.TrimSpace(value) && utf8.RuneCountInString(value) <= 128 && menuI18nKeyPattern.MatchString(value)
}

func validMenuPath(value string) bool {
	_, reserved := staticPagePaths[value]
	return !reserved && value == strings.TrimSpace(value) && utf8.RuneCountInString(value) <= 255 && menuPathPattern.MatchString(value)
}

func validMenuComponentPath(value string) bool {
	return value == strings.TrimSpace(value) && utf8.RuneCountInString(value) <= 255 && menuComponentPathPattern.MatchString(value)
}

func validMenuIcon(value string) bool {
	_, ok := menuIconNames[value]
	return ok
}

func normalizeCreateInput(input CreateInput) (CreateInput, error) {
	if input.PlatformID < 1 || !validMenuName(input.Name) || !validMenuCode(input.Code) || !yesno.IsValid(input.IsEnabled) ||
		!yesno.IsValid(input.IsHidden) || input.SortOrder < 0 {
		return CreateInput{}, fmt.Errorf("%w: create scalar is invalid", errMenuFields)
	}
	if err := validateMenuTypeCode(input.MenuType, input.Code); err != nil {
		return CreateInput{}, err
	}
	if err := validateInputShape(input.MenuType, input.I18nKey, input.Path, input.ComponentPath, input.Icon, input.IsHidden); err != nil {
		return CreateInput{}, err
	}
	return input, nil
}

func validateMenuTypeCode(menuType Type, code string) error {
	switch menuType {
	case TypePage:
		if !strings.HasSuffix(code, ":view") {
			return fmt.Errorf("%w: page permission code must end with :view", errMenuFields)
		}
	case TypeAction:
		if strings.HasSuffix(code, ":view") {
			return fmt.Errorf("%w: action permission code cannot end with :view", errMenuFields)
		}
	}
	return nil
}

func normalizeUpdateInput(input UpdateInput) (UpdateInput, error) {
	if !validMenuName(input.Name) || !yesno.IsValid(input.IsHidden) || input.SortOrder < 0 {
		return UpdateInput{}, fmt.Errorf("%w: update scalar is invalid", errMenuFields)
	}
	if err := validateInputShape(input.MenuType, input.I18nKey, input.Path, input.ComponentPath, input.Icon, input.IsHidden); err != nil {
		return UpdateInput{}, err
	}
	return input, nil
}

func validateInputShape(menuType Type, i18nKey, path, componentPath, icon *string, isHidden yesno.Value) error {
	switch menuType {
	case TypeDirectory:
		if i18nKey == nil || !validMenuI18nKey(*i18nKey) || path != nil || componentPath != nil || (icon != nil && !validMenuIcon(*icon)) || !yesno.IsValid(isHidden) {
			return fmt.Errorf("%w: directory fields are invalid", errMenuFields)
		}
	case TypePage:
		if i18nKey == nil || !validMenuI18nKey(*i18nKey) || path == nil || componentPath == nil || !validMenuPath(*path) || !validMenuComponentPath(*componentPath) ||
			(icon != nil && !validMenuIcon(*icon)) || !yesno.IsValid(isHidden) {
			return fmt.Errorf("%w: page fields are invalid", errMenuFields)
		}
	case TypeAction:
		if i18nKey != nil || path != nil || componentPath != nil || icon != nil || isHidden != yesno.Yes {
			return fmt.Errorf("%w: action fields are invalid", errMenuFields)
		}
	default:
		return fmt.Errorf("%w: menu type is invalid", errMenuFields)
	}
	return nil
}
