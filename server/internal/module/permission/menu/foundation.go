package menu

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/module/auth/platform"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

type FoundationDefinition struct {
	ParentCode    string
	MenuType      Type
	Name          string
	Code          string
	I18nKey       *string
	Path          *string
	ComponentPath *string
	Icon          *string
	Remark        *string
	SortOrder     int
	IsEnabled     yesno.Value
	IsHidden      yesno.Value
	Protected     bool
}

func IsProtectedCode(code string) bool {
	return code == "access" || code == PermissionView || code == PermissionCreate ||
		code == PermissionUpdate || code == PermissionDelete || code == PermissionRebuildAccessCache
}

func (s *Service) EnsureFoundation(ctx context.Context, definitions []FoundationDefinition) error {
	return s.ensurePlatformFoundation(ctx, authplatform.BuiltinAdminCode, definitions, false)
}

func (s *Service) EnsurePlatformFoundation(ctx context.Context, platformCode string, definitions []FoundationDefinition) error {
	return s.ensurePlatformFoundation(ctx, platformCode, definitions, true)
}

func (s *Service) ensurePlatformFoundation(ctx context.Context, platformCode string, definitions []FoundationDefinition, ensureAll bool) error {
	if s == nil || s.repository == nil || s.accessInvalidator == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("ensure menu foundation requires a repository"))
	}
	if err := authplatform.ValidateCode(platformCode); err != nil {
		return menuInvalidFields(fmt.Errorf("menu foundation platform code: %w", err))
	}
	ordered, err := validateFoundationDefinitions(definitions)
	if err != nil {
		return menuInvalidFields(err)
	}
	platforms, err := s.repository.FindPlatformOptions(ctx)
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	platformID := int64(0)
	for _, platform := range platforms {
		if platform.Code != platformCode {
			continue
		}
		if platformID != 0 {
			return apperror.DependencyUnavailable(fmt.Errorf("multiple active %s platforms exist", platformCode))
		}
		platformID = platform.ID
	}
	if platformID < 1 {
		return apperror.DependencyUnavailable(fmt.Errorf("authentication platform %s is unavailable", platformCode))
	}
	err = s.mutateAllAccessUsers(ctx, func(mutationCtx context.Context, repository *Repository, activeMenus []Menu, operationTime time.Time) (bool, error) {
		allCount, err := repository.CountAllMenusForPlatform(mutationCtx, platformID)
		if err != nil {
			return false, err
		}
		selected := ordered
		if !ensureAll && allCount != 0 {
			selected = protectedFoundationDefinitions(ordered)
		}
		codes := make([]string, len(selected))
		for index, definition := range selected {
			codes[index] = definition.Code
		}
		stored, err := repository.LockMenusByCodesUnscoped(mutationCtx, platformID, codes)
		if err != nil {
			return false, err
		}
		byCode := make(map[string]Menu, len(stored))
		for _, row := range stored {
			if _, duplicate := byCode[row.Code]; duplicate {
				return false, fmt.Errorf("foundation menu code %s has multiple historical rows", row.Code)
			}
			byCode[row.Code] = row
		}
		activeByCode := make(map[string]Menu, len(activeMenus)+len(selected))
		for _, row := range activeMenus {
			if row.PlatformID != platformID {
				continue
			}
			activeByCode[row.Code] = row
		}

		changed := false
		for _, definition := range selected {
			candidate, err := foundationMenu(platformID, definition, activeByCode)
			if err != nil {
				return false, err
			}
			row, exists := byCode[definition.Code]
			switch {
			case !exists:
				if err := repository.Create(mutationCtx, &candidate); err != nil {
					return false, err
				}
				changed = true
			case row.DeletedAt.Valid:
				if err := repository.RestoreFoundationMenu(mutationCtx, row.ID, candidate, operationTime); err != nil {
					return false, err
				}
				candidate.ID = row.ID
				changed = true
			default:
				candidate.ID = row.ID
				candidate.Name, candidate.I18nKey, candidate.Icon, candidate.SortOrder = row.Name, row.I18nKey, row.Icon, row.SortOrder
				if row.Remark != nil {
					candidate.Remark = row.Remark
				}
				if !sameFoundationStructure(row, candidate) {
					if err := repository.UpdateFoundationStructure(mutationCtx, row.ID, candidate, operationTime); err != nil {
						return false, err
					}
					changed = true
				}
			}
			activeByCode[definition.Code] = candidate
		}
		if !changed {
			return false, nil
		}
		finalMenus, err := repository.LockActiveMenus(mutationCtx)
		if err != nil {
			return false, err
		}
		index, err := buildMenuIndex(finalMenus)
		if err != nil {
			return false, err
		}
		if err := index.validateEnabledAncestors(); err != nil {
			return false, err
		}
		return true, nil
	})
	return mapTransactionError(err)
}

func validateFoundationDefinitions(definitions []FoundationDefinition) ([]FoundationDefinition, error) {
	if len(definitions) == 0 {
		return nil, fmt.Errorf("menu foundation requires definitions")
	}
	ordered := append([]FoundationDefinition(nil), definitions...)
	byCode := make(map[string]FoundationDefinition, len(ordered))
	for index, definition := range ordered {
		if definition.Protected != IsProtectedCode(definition.Code) {
			return nil, fmt.Errorf("foundation menu %s protected flag is invalid", definition.Code)
		}
		if _, duplicate := byCode[definition.Code]; duplicate {
			return nil, fmt.Errorf("foundation menu code %s is duplicated", definition.Code)
		}
		input := CreateInput{
			PlatformID: 1, MenuType: definition.MenuType, Name: definition.Name, Code: definition.Code, I18nKey: definition.I18nKey,
			Path: definition.Path, ComponentPath: definition.ComponentPath, Icon: definition.Icon,
			Remark: definition.Remark, SortOrder: definition.SortOrder, IsEnabled: definition.IsEnabled, IsHidden: definition.IsHidden,
		}
		if _, err := normalizeCreateInput(input); err != nil {
			return nil, fmt.Errorf("foundation menu %s: %w", definition.Code, err)
		}
		if definition.ParentCode == "" {
			if definition.MenuType == TypeAction {
				return nil, fmt.Errorf("foundation root %s is an action", definition.Code)
			}
		} else {
			parent, exists := byCode[definition.ParentCode]
			if !exists {
				return nil, fmt.Errorf("foundation menu %s parent %s must appear first", definition.Code, definition.ParentCode)
			}
			if !allowedMenuChild(parent.MenuType, definition.MenuType) {
				return nil, fmt.Errorf("foundation menu %s has an invalid parent type", definition.Code)
			}
		}
		if definition.Protected && definition.IsEnabled != yesno.Yes {
			return nil, fmt.Errorf("protected foundation menu %s must be enabled", definition.Code)
		}
		byCode[definition.Code] = ordered[index]
	}
	return ordered, nil
}

func protectedFoundationDefinitions(definitions []FoundationDefinition) []FoundationDefinition {
	result := make([]FoundationDefinition, 0, 5)
	for _, definition := range definitions {
		if definition.Protected {
			result = append(result, definition)
		}
	}
	return result
}

func foundationMenu(platformID int64, definition FoundationDefinition, activeByCode map[string]Menu) (Menu, error) {
	var parentID *int64
	if definition.ParentCode != "" {
		parent, exists := activeByCode[definition.ParentCode]
		if !exists {
			return Menu{}, fmt.Errorf("foundation menu %s parent %s is unavailable", definition.Code, definition.ParentCode)
		}
		value := parent.ID
		parentID = &value
	}
	return Menu{
		PlatformID: platformID, ParentID: parentID, MenuType: definition.MenuType, Name: definition.Name, Code: definition.Code,
		I18nKey: definition.I18nKey, Path: definition.Path, ComponentPath: definition.ComponentPath,
		Icon: definition.Icon, Remark: definition.Remark, SortOrder: definition.SortOrder, IsEnabled: definition.IsEnabled, IsHidden: definition.IsHidden,
	}, nil
}

func sameFoundationStructure(left, right Menu) bool {
	return left.PlatformID == right.PlatformID && sameInt64Pointer(left.ParentID, right.ParentID) && left.MenuType == right.MenuType &&
		sameStringPointer(left.Path, right.Path) && sameStringPointer(left.ComponentPath, right.ComponentPath) && sameStringPointer(left.Remark, right.Remark) &&
		left.IsEnabled == right.IsEnabled && left.IsHidden == right.IsHidden
}
