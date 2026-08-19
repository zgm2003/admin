package menu

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

type CreateInput struct {
	ParentID  *int64
	MenuType  Type
	Code      string
	I18nKey   string
	Path      *string
	ViewKey   *string
	Icon      *string
	SortOrder int
	IsEnabled yesno.Value
}

type UpdateInput struct {
	ParentID  *int64
	MenuType  Type
	I18nKey   string
	Path      *string
	ViewKey   *string
	Icon      *string
	SortOrder int
}

type ManagedMenu struct {
	ID        int64
	ParentID  *int64
	MenuType  Type
	Code      string
	I18nKey   string
	Path      *string
	ViewKey   *string
	Icon      *string
	SortOrder int
	IsEnabled yesno.Value
	IsBuiltin bool
	CreatedAt time.Time
	UpdatedAt time.Time
	Children  []ManagedMenu
}

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context) ([]ManagedMenu, error) {
	if s == nil || s.repository == nil {
		return nil, apperror.DependencyUnavailable(fmt.Errorf("list menus requires a repository"))
	}
	menus, err := s.repository.FindActiveMenus(ctx)
	if err != nil {
		return nil, apperror.DependencyUnavailable(err)
	}
	index, err := buildMenuIndex(menus)
	if err != nil {
		return nil, menuTreeInvalid(err)
	}
	if err := index.validateEnabledAncestors(); err != nil {
		return nil, menuTreeInvalid(err)
	}
	tree, err := index.buildManagedTree()
	if err != nil {
		return nil, menuTreeInvalid(err)
	}
	return tree, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	if s == nil || s.repository == nil {
		return 0, apperror.DependencyUnavailable(fmt.Errorf("create menu requires a repository"))
	}
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return 0, menuInvalidFields(err)
	}
	var createdID int64
	err = s.repository.Transaction(ctx, func(repository *Repository) error {
		menus, err := repository.LockActiveMenus(ctx)
		if err != nil {
			return apperror.DependencyUnavailable(err)
		}
		index, err := buildMenuIndex(menus)
		if err != nil {
			return menuTreeInvalid(err)
		}
		if err := validateCreateParent(index, normalized); err != nil {
			return err
		}
		if err := validateCreateEnabledParentChain(index, normalized); err != nil {
			return err
		}
		if menuCodeExists(menus, normalized.Code) {
			return menuCodeConflict(normalized.Code, ErrMenuCodeConflict)
		}
		if normalized.Path != nil && menuPathExists(menus, *normalized.Path, nil) {
			return menuPathConflict(*normalized.Path, ErrMenuPathConflict)
		}
		created := Menu{
			ParentID: normalized.ParentID, MenuType: normalized.MenuType, Code: normalized.Code,
			I18nKey: normalized.I18nKey, Path: normalized.Path, ViewKey: normalized.ViewKey,
			Icon: normalized.Icon, SortOrder: normalized.SortOrder, IsEnabled: normalized.IsEnabled,
		}
		if err := repository.Create(ctx, &created); err != nil {
			return mapServiceRepositoryError(err, normalized.Code, stringValue(normalized.Path))
		}
		createdID = created.ID
		return nil
	})
	if err != nil {
		return 0, mapTransactionError(err)
	}
	return createdID, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	if s == nil || s.repository == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("update menu requires a repository"))
	}
	if id < 1 {
		return menuNotFound(fmt.Errorf("menu id %d is invalid", id))
	}
	err := s.repository.Transaction(ctx, func(repository *Repository) error {
		menus, err := repository.LockActiveMenus(ctx)
		if err != nil {
			return apperror.DependencyUnavailable(err)
		}
		index, err := buildMenuIndex(menus)
		if err != nil {
			return menuTreeInvalid(err)
		}
		target, exists := index.byID[id]
		if !exists {
			return menuNotFound(fmt.Errorf("menu id %d is not active", id))
		}
		if IsBuiltinCode(target.Code) {
			if err := validateBuiltinUpdate(target, input); err != nil {
				return err
			}
		}
		normalized, err := normalizeUpdateInput(input)
		if err != nil {
			return menuInvalidFields(err)
		}
		if err := validateUpdateParent(index, target, normalized); err != nil {
			return err
		}
		if err := validateUpdateChildren(index, target, normalized.MenuType); err != nil {
			return err
		}
		if normalized.MenuType == TypeDirectory && target.MenuType != TypeDirectory {
			hasGrant, err := repository.HasActiveDirectGrant(ctx, target.ID)
			if err != nil {
				return apperror.DependencyUnavailable(err)
			}
			if hasGrant {
				return menuStructureConflict(target.Code, fmt.Errorf("menu has a direct role grant"))
			}
		}
		if normalized.Path != nil && menuPathExists(menus, *normalized.Path, &target.ID) {
			return menuPathConflict(*normalized.Path, ErrMenuPathConflict)
		}
		candidate := target
		candidate.ParentID = normalized.ParentID
		candidate.MenuType = normalized.MenuType
		candidate.I18nKey = normalized.I18nKey
		candidate.Path = normalized.Path
		candidate.ViewKey = normalized.ViewKey
		candidate.Icon = normalized.Icon
		candidate.SortOrder = normalized.SortOrder
		candidateIndex, err := replaceMenuInIndex(index, candidate)
		if err != nil {
			return menuTreeInvalid(err)
		}
		if err := candidateIndex.validateEnabledAncestors(); err != nil {
			var parentDisabled *menuParentDisabledViolation
			if errors.As(err, &parentDisabled) {
				return menuParentDisabled(parentDisabled.code, err)
			}
			return menuTreeInvalid(err)
		}
		if err := repository.UpdateMenu(ctx, id, UpdateValues{
			ParentID: normalized.ParentID, MenuType: normalized.MenuType, I18nKey: normalized.I18nKey,
			Path: normalized.Path, ViewKey: normalized.ViewKey, Icon: normalized.Icon,
			SortOrder: normalized.SortOrder,
		}, time.Now().UTC()); err != nil {
			return mapServiceRepositoryError(err, target.Code, stringValue(normalized.Path))
		}
		return nil
	})
	return mapTransactionError(err)
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, value yesno.Value) error {
	if s == nil || s.repository == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("update menu status requires a repository"))
	}
	if id < 1 || !yesno.IsValid(value) {
		if id < 1 {
			return menuNotFound(fmt.Errorf("menu id %d is invalid", id))
		}
		return menuInvalidFields(fmt.Errorf("is_enabled value %d is invalid", value))
	}
	err := s.repository.Transaction(ctx, func(repository *Repository) error {
		menus, err := repository.LockActiveMenus(ctx)
		if err != nil {
			return apperror.DependencyUnavailable(err)
		}
		index, err := buildMenuIndex(menus)
		if err != nil {
			return menuTreeInvalid(err)
		}
		target, exists := index.byID[id]
		if !exists {
			return menuNotFound(fmt.Errorf("menu id %d is not active", id))
		}
		if IsBuiltinCode(target.Code) && value == yesno.No {
			return menuBuiltinProtected(target.Code, fmt.Errorf("builtin menu cannot be disabled"))
		}
		if target.IsEnabled == value {
			return nil
		}
		if value == yesno.Yes {
			ancestors, err := index.ancestors(id)
			if err != nil {
				return mapTreeMutationError(err)
			}
			for _, ancestorID := range ancestors {
				if index.byID[ancestorID].IsEnabled != yesno.Yes {
					return menuParentDisabled(target.Code, fmt.Errorf("ancestor %s is disabled", index.byID[ancestorID].Code))
				}
			}
			if err := repository.UpdateMenuStatus(ctx, []int64{id}, value, time.Now().UTC()); err != nil {
				return apperror.DependencyUnavailable(err)
			}
			return nil
		}
		descendants, err := index.descendants(id)
		if err != nil {
			return mapTreeMutationError(err)
		}
		ids := append([]int64{id}, descendants...)
		if err := repository.UpdateMenuStatus(ctx, ids, value, time.Now().UTC()); err != nil {
			return apperror.DependencyUnavailable(err)
		}
		return nil
	})
	return mapTransactionError(err)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if s == nil || s.repository == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("delete menu requires a repository"))
	}
	if id < 1 {
		return menuNotFound(fmt.Errorf("menu id %d is invalid", id))
	}
	err := s.repository.Transaction(ctx, func(repository *Repository) error {
		menus, err := repository.LockActiveMenus(ctx)
		if err != nil {
			return apperror.DependencyUnavailable(err)
		}
		index, err := buildMenuIndex(menus)
		if err != nil {
			return menuTreeInvalid(err)
		}
		_, exists := index.byID[id]
		if !exists {
			return menuNotFound(fmt.Errorf("menu id %d is not active", id))
		}
		descendants, err := index.descendants(id)
		if err != nil {
			return mapTreeMutationError(err)
		}
		ids := append([]int64{id}, descendants...)
		for _, menuID := range ids {
			if IsBuiltinCode(index.byID[menuID].Code) {
				return menuBuiltinProtected(index.byID[menuID].Code, fmt.Errorf("builtin menu cannot be deleted"))
			}
		}
		deletedAt := time.Now().UTC()
		if err := repository.SoftDeleteRoleMenus(ctx, ids, deletedAt); err != nil {
			return apperror.DependencyUnavailable(err)
		}
		if err := repository.SoftDeleteMenus(ctx, ids, deletedAt); err != nil {
			return apperror.DependencyUnavailable(err)
		}
		return nil
	})
	return mapTransactionError(err)
}

func validateCreateParent(index menuIndex, input CreateInput) error {
	if input.ParentID == nil {
		if input.MenuType != TypeDirectory {
			return menuInvalidParent(fmt.Errorf("root menu must be a directory"))
		}
		return nil
	}
	if *input.ParentID < 1 {
		return menuInvalidParent(fmt.Errorf("parent id %d is invalid", *input.ParentID))
	}
	parent, exists := index.byID[*input.ParentID]
	if !exists || !allowedMenuChild(parent.MenuType, input.MenuType) {
		return menuInvalidParent(fmt.Errorf("parent does not accept menu type"))
	}
	return nil
}

func validateCreateEnabledParentChain(index menuIndex, input CreateInput) error {
	if input.IsEnabled != yesno.Yes || input.ParentID == nil {
		return nil
	}
	parent, exists := index.byID[*input.ParentID]
	if !exists {
		return menuInvalidParent(fmt.Errorf("parent is missing"))
	}
	if parent.IsEnabled != yesno.Yes {
		return menuParentDisabled(input.Code, fmt.Errorf("parent %s is disabled", parent.Code))
	}
	ancestors, err := index.ancestors(parent.ID)
	if err != nil {
		return menuTreeInvalid(err)
	}
	for _, ancestorID := range ancestors {
		if index.byID[ancestorID].IsEnabled != yesno.Yes {
			return menuParentDisabled(input.Code, fmt.Errorf("ancestor %s is disabled", index.byID[ancestorID].Code))
		}
	}
	return nil
}

func validateUpdateParent(index menuIndex, target Menu, input UpdateInput) error {
	if input.ParentID == nil {
		if input.MenuType != TypeDirectory {
			return menuInvalidParent(fmt.Errorf("root menu must be a directory"))
		}
		return nil
	}
	if *input.ParentID < 1 {
		return menuInvalidParent(fmt.Errorf("parent id %d is invalid", *input.ParentID))
	}
	if *input.ParentID == target.ID {
		return menuCycleDetected(fmt.Errorf("menu cannot be its own parent"))
	}
	descendants, err := index.descendants(target.ID)
	if err != nil {
		return mapTreeMutationError(err)
	}
	for _, descendantID := range descendants {
		if descendantID == *input.ParentID {
			return menuCycleDetected(fmt.Errorf("menu parent is a descendant"))
		}
	}
	parent, exists := index.byID[*input.ParentID]
	if !exists || !allowedMenuChild(parent.MenuType, input.MenuType) {
		return menuInvalidParent(fmt.Errorf("parent does not accept menu type"))
	}
	return nil
}

func validateUpdateChildren(index menuIndex, target Menu, menuType Type) error {
	for _, childID := range index.children[target.ID] {
		if !allowedMenuChild(menuType, index.byID[childID].MenuType) {
			return menuStructureConflict(target.Code, fmt.Errorf("child %s is incompatible", index.byID[childID].Code))
		}
	}
	return nil
}

func validateBuiltinUpdate(target Menu, input UpdateInput) error {
	if input.MenuType != target.MenuType || !sameInt64Pointer(input.ParentID, target.ParentID) ||
		stringsTrim(input.I18nKey) != target.I18nKey || !sameStringPointerTrim(input.Path, target.Path) ||
		!sameStringPointerTrim(input.ViewKey, target.ViewKey) {
		return menuBuiltinProtected(target.Code, fmt.Errorf("builtin immutable field changed"))
	}
	if target.MenuType == TypeAction && input.Icon != nil {
		return menuBuiltinProtected(target.Code, fmt.Errorf("builtin action icon is immutable"))
	}
	return nil
}

func replaceMenuInIndex(index menuIndex, replacement Menu) (menuIndex, error) {
	index.byID[replacement.ID] = replacement
	return buildMenuIndex(menuIndexRows(index))
}

func menuIndexRows(index menuIndex) []Menu {
	rows := make([]Menu, 0, len(index.byID))
	for _, item := range index.byID {
		rows = append(rows, item)
	}
	return rows
}

func menuCodeExists(menus []Menu, code string) bool {
	for _, item := range menus {
		if item.Code == code {
			return true
		}
	}
	return false
}

func menuPathExists(menus []Menu, path string, excludedID *int64) bool {
	for _, item := range menus {
		if item.Path != nil && *item.Path == path && (excludedID == nil || item.ID != *excludedID) {
			return true
		}
	}
	return false
}

func mapTreeMutationError(err error) error {
	switch {
	case errors.Is(err, errMenuCycle):
		return menuCycleDetected(err)
	case errors.Is(err, errMenuParent):
		return menuInvalidParent(err)
	default:
		return menuTreeInvalid(err)
	}
}

func mapServiceRepositoryError(err error, code, path string) error {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	switch {
	case errors.Is(err, ErrMenuCodeConflict):
		return menuCodeConflict(code, err)
	case errors.Is(err, ErrMenuPathConflict):
		return menuPathConflict(path, err)
	default:
		return apperror.DependencyUnavailable(err)
	}
}

func mapTransactionError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.DependencyUnavailable(err)
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}

func sameStringPointerTrim(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return strings.TrimSpace(*left) == *right
}

func sameInt64Pointer(left, right *int64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
