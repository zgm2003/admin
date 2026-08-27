package access

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"admin/server/internal/module/accessstate"
	"admin/server/internal/module/auth"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
)

const CodeAccessSnapshotInvalid = 14000

type sourceStore interface {
	FindSourceWithVersion(context.Context, int64, int64) (Source, error)
}

type MenuNode struct {
	Code          string     `json:"code"`
	MenuType      MenuType   `json:"menuType"`
	Path          *string    `json:"path"`
	ComponentPath *string    `json:"componentPath"`
	I18nKey       string     `json:"i18nKey"`
	Icon          *string    `json:"icon"`
	IsHidden      int16      `json:"isHidden"`
	Children      []MenuNode `json:"children"`
}

var (
	accessI18nKeyPattern       = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-zA-Z0-9]*)+$`)
	accessPathPattern          = regexp.MustCompile(`^/[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$`)
	accessComponentPathPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$`)
	staticAccessPagePaths      = map[string]struct{}{
		"/login": {}, "/register": {}, "/dashboard": {},
	}
)

type Snapshot struct {
	RoleCodes       []string
	MenuTree        []MenuNode
	PermissionCodes []string
	Version         int64
	CacheResult     string
}

type Service struct {
	store  sourceStore
	states *accessstate.Store
	cache  *SnapshotCache
	logger *slog.Logger
}

func NewService(store sourceStore, states *accessstate.Store, cache *SnapshotCache, logger *slog.Logger) *Service {
	return &Service{store: store, states: states, cache: cache, logger: logger}
}

func (s *Service) Current(ctx context.Context, identity auth.Identity) (Snapshot, error) {
	if err := validateAccessIdentity(identity); err != nil {
		return Snapshot{}, apperror.InvalidRequest(err)
	}
	return s.loadSnapshot(ctx, identity)
}

func (s *Service) Allowed(ctx context.Context, identity auth.Identity, permissionCode string) (bool, error) {
	if err := validateAccessIdentity(identity); err != nil {
		return false, apperror.InvalidRequest(err)
	}
	if strings.TrimSpace(permissionCode) == "" {
		return false, apperror.InvalidRequest(fmt.Errorf("permission check requires a permission code"))
	}
	snapshot, err := s.loadSnapshot(ctx, identity)
	if err != nil {
		return false, err
	}
	permissions := make(map[string]struct{}, len(snapshot.PermissionCodes))
	for _, code := range snapshot.PermissionCodes {
		permissions[code] = struct{}{}
	}
	_, allowed := permissions[permissionCode]
	return allowed, nil
}

func (s *Service) loadSnapshot(ctx context.Context, identity auth.Identity) (Snapshot, error) {
	cacheResult := "miss"
	state, found, stateErr := s.states.Read(ctx, identity.UserID)
	if stateErr != nil {
		cacheResult = "error"
		s.logCacheError(ctx, "accessState", "error", stateErr)
	} else if found {
		if state.State == accessstate.StateInvalidating {
			return Snapshot{}, accessUpdating(accessstate.ErrUpdating)
		}
		cached, cacheFound, cacheErr := s.cache.Read(ctx, identity.PlatformID, identity.Platform, identity.PolicyVersion, identity.UserID, state.Version)
		if cacheErr != nil {
			cacheResult = "error"
			s.logCacheError(ctx, "accessSnapshot", "error", cacheErr)
		} else if cacheFound {
			return snapshotFromCache(cached, "hit"), nil
		}
	}

	for attempt := 0; attempt < 3; attempt++ {
		source, err := s.store.FindSourceWithVersion(ctx, identity.UserID, identity.PlatformID)
		if err != nil {
			return Snapshot{}, apperror.DependencyUnavailable(err)
		}
		snapshot, err := buildSnapshot(source)
		if err != nil {
			return Snapshot{}, accessSnapshotInvalid(err)
		}
		snapshot.Version = source.Version
		snapshot.CacheResult = cacheResult

		current, currentFound, currentErr := s.states.Read(ctx, identity.UserID)
		if currentErr != nil || !currentFound {
			installed, _, installErr := s.states.InstallReadyIfMissing(ctx, accessstate.Version{UserID: identity.UserID, Version: source.Version})
			if installErr != nil {
				s.logCacheError(ctx, "accessState", "error", errors.Join(currentErr, installErr))
				snapshot.CacheResult = "error"
				return snapshot, nil
			}
			current = installed
		}
		if current.State == accessstate.StateInvalidating {
			return Snapshot{}, accessUpdating(accessstate.ErrUpdating)
		}
		if current.Version != source.Version {
			cacheResult = "miss"
			continue
		}

		published, publishErr := s.cache.PublishIfCurrent(ctx, newCachedSnapshot(identity.UserID, identity.PlatformID, identity.Platform, identity.PolicyVersion, snapshot), identity.AccessCacheTTL)
		if publishErr != nil {
			s.logCacheError(ctx, "accessSnapshot", "error", publishErr)
			snapshot.CacheResult = "error"
			return snapshot, nil
		}
		if !published {
			cacheResult = "miss"
			continue
		}
		return snapshot, nil
	}
	return Snapshot{}, accessUpdating(fmt.Errorf("access version kept changing during snapshot rebuild"))
}

func validateAccessIdentity(identity auth.Identity) error {
	if identity.UserID < 1 || identity.SessionID < 1 || identity.PlatformID < 1 || identity.Platform == "" || identity.PolicyVersion < 1 || identity.AccessCacheTTL <= 0 {
		return fmt.Errorf("access snapshot requires a complete authentication identity")
	}
	return nil
}

func (s *Service) logCacheError(ctx context.Context, kind, result string, err error) {
	s.logger.ErrorContext(ctx, "access cache operation failed", "cacheKind", kind, "cacheResult", result, "error", err)
}

func buildSnapshot(source Source) (Snapshot, error) {
	if source.Version < 1 {
		return Snapshot{}, fmt.Errorf("access source version is invalid")
	}
	menusByID := make(map[int64]SourceMenu, len(source.Menus))
	for _, item := range source.Menus {
		if item.ID <= 0 {
			return Snapshot{}, fmt.Errorf("menu has invalid ID %d", item.ID)
		}
		if _, exists := menusByID[item.ID]; exists {
			return Snapshot{}, fmt.Errorf("menu ID %d is duplicated", item.ID)
		}
		if item.MenuType != MenuDirectory && item.MenuType != MenuPage && item.MenuType != MenuAction {
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
			if item.MenuType == MenuPage || item.MenuType == MenuAction {
				startIDs = append(startIDs, item.ID)
			}
		}
	} else {
		startIDs = append(startIDs, source.GrantedMenuIDs...)
	}

	selected := make(map[int64]SourceMenu)
	for _, startID := range startIDs {
		start, exists := menusByID[startID]
		if !exists {
			return Snapshot{}, fmt.Errorf("direct grant menu %d is missing", startID)
		}
		if start.MenuType == MenuDirectory {
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
	roleCodes, err := sortUniqueStrings(source.RoleCodes)
	if err != nil {
		return Snapshot{}, err
	}
	permissionCodes := make([]string, 0)
	for _, item := range selected {
		if item.MenuType == MenuPage || item.MenuType == MenuAction {
			permissionCodes = append(permissionCodes, item.Code)
		}
	}
	permissionCodes, err = sortUniqueStrings(permissionCodes)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		RoleCodes: roleCodes, MenuTree: buildMenuTree(selected), PermissionCodes: permissionCodes, Version: source.Version,
	}, nil
}

func validateSelectedMenus(selected map[int64]SourceMenu) error {
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
		if !yesno.IsValid(item.IsHidden) {
			return fmt.Errorf("menu %d has an invalid hidden state", id)
		}
		if item.Icon != nil && !validAccessIcon(*item.Icon) {
			return fmt.Errorf("menu %d has an invalid icon", id)
		}

		switch item.MenuType {
		case MenuDirectory:
			if item.I18nKey == nil || !validAccessI18nKey(*item.I18nKey) || item.Path != nil || item.ComponentPath != nil {
				return fmt.Errorf("directory menu %d has render fields", id)
			}
		case MenuPage:
			if item.I18nKey == nil || !validAccessI18nKey(*item.I18nKey) || item.Path == nil || item.ComponentPath == nil || !validAccessPath(*item.Path) || !validAccessComponentPath(*item.ComponentPath) {
				return fmt.Errorf("page menu %d is incomplete", id)
			}
			if existingID, exists := paths[*item.Path]; exists {
				return fmt.Errorf("page menus %d and %d share path %q", existingID, id, *item.Path)
			}
			paths[*item.Path] = id
		case MenuAction:
			if item.I18nKey != nil || item.Path != nil || item.ComponentPath != nil || item.Icon != nil || item.ParentID == nil || item.IsHidden != yesno.Yes {
				return fmt.Errorf("action menu %d shape is invalid", id)
			}
		}

		if item.ParentID == nil {
			if item.MenuType == MenuAction {
				return fmt.Errorf("root menu %d is an action", id)
			}
			continue
		}
		parent, exists := selected[*item.ParentID]
		if !exists {
			return fmt.Errorf("selected menu %d has missing parent %d", id, *item.ParentID)
		}
		switch parent.MenuType {
		case MenuDirectory:
			if item.MenuType != MenuDirectory && item.MenuType != MenuPage {
				return fmt.Errorf("directory menu %d has invalid child type %q", parent.ID, item.MenuType)
			}
		case MenuPage:
			if item.MenuType != MenuAction {
				return fmt.Errorf("page menu %d has invalid child type %q", parent.ID, item.MenuType)
			}
		case MenuAction:
			return fmt.Errorf("action menu %d has child %d", parent.ID, item.ID)
		}
	}
	return nil
}

func buildMenuTree(selected map[int64]SourceMenu) []MenuNode {
	childrenByParent := make(map[int64][]SourceMenu)
	roots := make([]SourceMenu, 0)
	for _, item := range selected {
		if item.MenuType == MenuAction {
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

func buildMenuNode(item SourceMenu, childrenByParent map[int64][]SourceMenu) MenuNode {
	children := make([]MenuNode, 0, len(childrenByParent[item.ID]))
	for _, child := range childrenByParent[item.ID] {
		children = append(children, buildMenuNode(child, childrenByParent))
	}
	return MenuNode{
		Code: item.Code, MenuType: item.MenuType, Path: item.Path, ComponentPath: item.ComponentPath,
		I18nKey: *item.I18nKey, Icon: item.Icon, IsHidden: int16(item.IsHidden), Children: children,
	}
}

func validAccessI18nKey(value string) bool {
	return value == strings.TrimSpace(value) && utf8.RuneCountInString(value) <= 128 && accessI18nKeyPattern.MatchString(value)
}

func validAccessPath(value string) bool {
	_, reserved := staticAccessPagePaths[value]
	return !reserved && value == strings.TrimSpace(value) && utf8.RuneCountInString(value) <= 255 && accessPathPattern.MatchString(value)
}

func validAccessComponentPath(value string) bool {
	return value == strings.TrimSpace(value) && utf8.RuneCountInString(value) <= 255 && accessComponentPathPattern.MatchString(value)
}

func validAccessIcon(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && utf8.RuneCountInString(value) <= 128
}

func sortMenus(menus []SourceMenu) {
	sort.Slice(menus, func(left, right int) bool {
		if menus[left].SortOrder != menus[right].SortOrder {
			return menus[left].SortOrder < menus[right].SortOrder
		}
		if menus[left].Code != menus[right].Code {
			return menus[left].Code < menus[right].Code
		}
		return menus[left].ID < menus[right].ID
	})
}

func accessSnapshotInvalid(cause error) *apperror.Error {
	return &apperror.Error{
		HTTPStatus: http.StatusInternalServerError, Code: CodeAccessSnapshotInvalid,
		MessageKey: i18n.KeyAccessSnapshotInvalid, Cause: cause,
	}
}
