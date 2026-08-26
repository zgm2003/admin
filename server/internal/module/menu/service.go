package menu

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/accessstate"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
)

type CreateInput struct {
	ParentID      *int64
	MenuType      Type
	Name          string
	Code          string
	I18nKey       *string
	Path          *string
	ComponentPath *string
	Icon          *string
	SortOrder     int
	IsEnabled     yesno.Value
	IsHidden      yesno.Value
}

type UpdateInput struct {
	ParentID      *int64
	MenuType      Type
	Name          string
	I18nKey       *string
	Path          *string
	ComponentPath *string
	Icon          *string
	SortOrder     int
	IsHidden      yesno.Value
}

type ManagedMenu struct {
	ID            int64
	ParentID      *int64
	MenuType      Type
	Name          string
	Code          string
	I18nKey       *string
	Path          *string
	ComponentPath *string
	Icon          *string
	SortOrder     int
	IsEnabled     yesno.Value
	IsHidden      yesno.Value
	CreatedAt     time.Time
	UpdatedAt     time.Time
	IsProtected   bool
	Children      []ManagedMenu
}

type Service struct {
	repository        *Repository
	accessInvalidator *accessstate.Invalidator
}

func NewService(repository *Repository, accessInvalidator *accessstate.Invalidator) *Service {
	return &Service{repository: repository, accessInvalidator: accessInvalidator}
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
	if s == nil || s.repository == nil || s.accessInvalidator == nil {
		return 0, apperror.DependencyUnavailable(fmt.Errorf("create menu requires a repository"))
	}
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return 0, menuInvalidFields(err)
	}
	var createdID int64
	err = s.mutateAllAccessUsers(ctx, func(mutationCtx context.Context, repository *Repository, menus []Menu, _ time.Time) (bool, error) {
		index, err := buildMenuIndex(menus)
		if err != nil {
			return false, menuTreeInvalid(err)
		}
		if err := validateCreateParent(index, normalized); err != nil {
			return false, err
		}
		if err := validateCreateEnabledParentChain(index, normalized); err != nil {
			return false, err
		}
		if menuCodeExists(menus, normalized.Code) {
			return false, menuCodeConflict(normalized.Code, ErrMenuCodeConflict)
		}
		if normalized.Path != nil && menuPathExists(menus, *normalized.Path, nil) {
			return false, menuPathConflict(*normalized.Path, ErrMenuPathConflict)
		}
		created := Menu{
			ParentID: normalized.ParentID, MenuType: normalized.MenuType, Name: normalized.Name, Code: normalized.Code,
			I18nKey: normalized.I18nKey, Path: normalized.Path, ComponentPath: normalized.ComponentPath,
			Icon: normalized.Icon, SortOrder: normalized.SortOrder, IsEnabled: normalized.IsEnabled, IsHidden: normalized.IsHidden,
		}
		if err := repository.Create(mutationCtx, &created); err != nil {
			return false, mapServiceRepositoryError(err, normalized.Code, stringValue(normalized.Path))
		}
		createdID = created.ID
		return true, nil
	})
	if err != nil {
		return 0, mapTransactionError(err)
	}
	return createdID, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	if s == nil || s.repository == nil || s.accessInvalidator == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("update menu requires a repository"))
	}
	if id < 1 {
		return menuNotFound(fmt.Errorf("menu id %d is invalid", id))
	}
	err := s.mutateAllAccessUsers(ctx, func(mutationCtx context.Context, repository *Repository, menus []Menu, operationTime time.Time) (bool, error) {
		index, err := buildMenuIndex(menus)
		if err != nil {
			return false, menuTreeInvalid(err)
		}
		target, exists := index.byID[id]
		if !exists {
			return false, menuNotFound(fmt.Errorf("menu id %d is not active", id))
		}
		normalized, err := normalizeUpdateInput(input)
		if err != nil {
			return false, menuInvalidFields(err)
		}
		if IsProtectedCode(target.Code) && !allowedProtectedUpdate(target, normalized) {
			return false, menuProtected(target.Code, fmt.Errorf("protected menu structure cannot change"))
		}
		if err := validateUpdateParent(index, target, normalized); err != nil {
			return false, err
		}
		if err := validateUpdateChildren(index, target, normalized.MenuType); err != nil {
			return false, err
		}
		if normalized.MenuType == TypeDirectory && target.MenuType != TypeDirectory {
			hasGrant, err := repository.HasActiveDirectGrant(mutationCtx, target.ID)
			if err != nil {
				return false, apperror.DependencyUnavailable(err)
			}
			if hasGrant {
				return false, menuStructureConflict(target.Code, fmt.Errorf("menu has a direct role grant"))
			}
		}
		if normalized.Path != nil && menuPathExists(menus, *normalized.Path, &target.ID) {
			return false, menuPathConflict(*normalized.Path, ErrMenuPathConflict)
		}
		if sameMenuUpdate(target, normalized) {
			return false, nil
		}
		candidate := target
		candidate.ParentID = normalized.ParentID
		candidate.MenuType = normalized.MenuType
		candidate.Name = normalized.Name
		candidate.I18nKey = normalized.I18nKey
		candidate.Path = normalized.Path
		candidate.ComponentPath = normalized.ComponentPath
		candidate.Icon = normalized.Icon
		candidate.SortOrder = normalized.SortOrder
		candidate.IsHidden = normalized.IsHidden
		candidateIndex, err := replaceMenuInIndex(index, candidate)
		if err != nil {
			return false, menuTreeInvalid(err)
		}
		if err := candidateIndex.validateEnabledAncestors(); err != nil {
			var parentDisabled *menuParentDisabledViolation
			if errors.As(err, &parentDisabled) {
				return false, menuParentDisabled(parentDisabled.code, err)
			}
			return false, menuTreeInvalid(err)
		}
		if err := repository.UpdateMenu(mutationCtx, id, UpdateValues{
			ParentID: normalized.ParentID, MenuType: normalized.MenuType, Name: normalized.Name, I18nKey: normalized.I18nKey,
			Path: normalized.Path, ComponentPath: normalized.ComponentPath, Icon: normalized.Icon,
			SortOrder: normalized.SortOrder, IsHidden: normalized.IsHidden,
		}, operationTime); err != nil {
			return false, mapServiceRepositoryError(err, target.Code, stringValue(normalized.Path))
		}
		return true, nil
	})
	return mapTransactionError(err)
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, value yesno.Value) error {
	if s == nil || s.repository == nil || s.accessInvalidator == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("update menu status requires a repository"))
	}
	if id < 1 || !yesno.IsValid(value) {
		if id < 1 {
			return menuNotFound(fmt.Errorf("menu id %d is invalid", id))
		}
		return menuInvalidFields(fmt.Errorf("is_enabled value %d is invalid", value))
	}
	err := s.mutateAllAccessUsers(ctx, func(mutationCtx context.Context, repository *Repository, menus []Menu, operationTime time.Time) (bool, error) {
		index, err := buildMenuIndex(menus)
		if err != nil {
			return false, menuTreeInvalid(err)
		}
		target, exists := index.byID[id]
		if !exists {
			return false, menuNotFound(fmt.Errorf("menu id %d is not active", id))
		}
		if IsProtectedCode(target.Code) && value != yesno.Yes {
			return false, menuProtected(target.Code, fmt.Errorf("protected menu cannot be disabled"))
		}
		if target.IsEnabled == value {
			return false, nil
		}
		if value == yesno.Yes {
			ancestors, err := index.ancestors(id)
			if err != nil {
				return false, mapTreeMutationError(err)
			}
			for _, ancestorID := range ancestors {
				if index.byID[ancestorID].IsEnabled != yesno.Yes {
					return false, menuParentDisabled(target.Code, fmt.Errorf("ancestor %s is disabled", index.byID[ancestorID].Code))
				}
			}
			if err := repository.UpdateMenuStatus(mutationCtx, []int64{id}, value, operationTime); err != nil {
				return false, apperror.DependencyUnavailable(err)
			}
			return true, nil
		}
		descendants, err := index.descendants(id)
		if err != nil {
			return false, mapTreeMutationError(err)
		}
		ids := append([]int64{id}, descendants...)
		if err := repository.UpdateMenuStatus(mutationCtx, ids, value, operationTime); err != nil {
			return false, apperror.DependencyUnavailable(err)
		}
		return true, nil
	})
	return mapTransactionError(err)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if s == nil || s.repository == nil || s.accessInvalidator == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("delete menu requires a repository"))
	}
	if id < 1 {
		return menuNotFound(fmt.Errorf("menu id %d is invalid", id))
	}
	err := s.mutateAllAccessUsers(ctx, func(mutationCtx context.Context, repository *Repository, menus []Menu, operationTime time.Time) (bool, error) {
		index, err := buildMenuIndex(menus)
		if err != nil {
			return false, menuTreeInvalid(err)
		}
		target, exists := index.byID[id]
		if !exists {
			return false, menuNotFound(fmt.Errorf("menu id %d is not active", id))
		}
		if IsProtectedCode(target.Code) {
			return false, menuProtected(target.Code, fmt.Errorf("protected menu cannot be deleted"))
		}
		descendants, err := index.descendants(id)
		if err != nil {
			return false, mapTreeMutationError(err)
		}
		ids := append([]int64{id}, descendants...)
		if err := repository.SoftDeleteRoleMenus(mutationCtx, ids, operationTime); err != nil {
			return false, apperror.DependencyUnavailable(err)
		}
		if err := repository.SoftDeleteMenus(mutationCtx, ids, operationTime); err != nil {
			return false, apperror.DependencyUnavailable(err)
		}
		return true, nil
	})
	return mapTransactionError(err)
}

var errMenuAffectedUsersChanged = errors.New("menu affected users changed")

const menuMutationAttempts = 3

type menuAccessMutation func(context.Context, *Repository, []Menu, time.Time) (bool, error)

func (s *Service) mutateAllAccessUsers(ctx context.Context, mutate menuAccessMutation) error {
	for attempt := 0; attempt < menuMutationAttempts; attempt++ {
		candidates, err := s.repository.FindActiveAccessVersions(ctx)
		if err != nil {
			return apperror.DependencyUnavailable(err)
		}
		lease, err := s.accessInvalidator.Acquire(ctx, candidates)
		if err != nil {
			return apperror.DependencyUnavailable(err)
		}
		mutationCtx, stopRenewal := lease.StartRenewal(ctx)
		changed := false
		advanced := map[int64]int64{}
		err = s.repository.Transaction(mutationCtx, func(repository *Repository) error {
			menus, lockErr := repository.LockActiveMenus(mutationCtx)
			if lockErr != nil {
				return lockErr
			}
			if lockErr := repository.LockUserMutationTables(mutationCtx); lockErr != nil {
				return lockErr
			}
			actual, lockErr := repository.LockActiveAccessVersions(mutationCtx)
			if lockErr != nil {
				return lockErr
			}
			if !equalMenuAccessVersions(candidates, actual) {
				return errMenuAffectedUsersChanged
			}
			operationTime := time.Now().UTC().Truncate(time.Microsecond)
			changed, lockErr = mutate(mutationCtx, repository, menus, operationTime)
			if lockErr != nil || !changed {
				return lockErr
			}
			advanced, lockErr = repository.IncrementAccessVersions(mutationCtx, menuAccessUserIDs(actual), operationTime)
			return lockErr
		})
		renewalCause := context.Cause(mutationCtx)
		stopRenewal()
		if errors.Is(err, errMenuAffectedUsersChanged) {
			if rollbackErr := lease.Rollback(ctx); rollbackErr != nil {
				return apperror.DependencyUnavailable(errors.Join(err, renewalCause, rollbackErr))
			}
			continue
		}
		if err != nil {
			return errors.Join(err, renewalCause, lease.Rollback(ctx))
		}
		if renewalCause != nil {
			return apperror.DependencyUnavailable(errors.Join(renewalCause, lease.Rollback(ctx)))
		}
		if !changed {
			if err := lease.Rollback(ctx); err != nil {
				return apperror.DependencyUnavailable(err)
			}
			return nil
		}
		if err := lease.Commit(ctx, advanced); err != nil {
			return apperror.DependencyUnavailable(err)
		}
		return nil
	}
	return apperror.Conflict(i18n.KeyConflict, nil, errMenuAffectedUsersChanged)
}

func equalMenuAccessVersions(left, right []accessstate.Version) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func menuAccessUserIDs(versions []accessstate.Version) []int64 {
	userIDs := make([]int64, len(versions))
	for index, version := range versions {
		userIDs[index] = version.UserID
	}
	return userIDs
}

func sameMenuUpdate(stored Menu, input UpdateInput) bool {
	return sameInt64Pointer(stored.ParentID, input.ParentID) &&
		stored.MenuType == input.MenuType && stored.Name == input.Name && sameStringPointer(stored.I18nKey, input.I18nKey) &&
		sameStringPointer(stored.Path, input.Path) && sameStringPointer(stored.ComponentPath, input.ComponentPath) &&
		sameStringPointer(stored.Icon, input.Icon) && stored.SortOrder == input.SortOrder && stored.IsHidden == input.IsHidden
}

func allowedProtectedUpdate(stored Menu, input UpdateInput) bool {
	return sameInt64Pointer(stored.ParentID, input.ParentID) && stored.MenuType == input.MenuType &&
		sameStringPointer(stored.Path, input.Path) && sameStringPointer(stored.ComponentPath, input.ComponentPath) &&
		stored.IsHidden == input.IsHidden
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

func sameStringPointer(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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
