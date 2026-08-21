package menu

import (
	"context"
	"fmt"

	"admin/server/internal/shared/yesno"
)

type builtinMenuDefinition struct {
	code       string
	menuType   Type
	parentCode string
	i18nKey    string
	path       *string
	viewKey    *string
	icon       *string
	sortOrder  int
}

func (s *Service) EnsureBuiltin(ctx context.Context) error {
	if s == nil || s.repository == nil {
		return fmt.Errorf("ensure builtin menus requires a repository")
	}
	definitions := builtinMenuDefinitions()
	return s.repository.Transaction(ctx, func(repository *Repository) error {
		if err := repository.LockMenuTableForBuiltin(ctx); err != nil {
			return err
		}
		records, err := repository.FindBuiltinRecords(ctx, builtinCodesFromDefinitions(definitions))
		if err != nil {
			return err
		}

		byCode := make(map[string][]Menu, len(definitions))
		for _, record := range records {
			byCode[record.Code] = append(byCode[record.Code], record)
		}
		active := make(map[string]Menu, len(definitions))
		for _, definition := range definitions {
			matching := byCode[definition.code]
			if len(matching) > 1 {
				return fmt.Errorf("ensure builtin menu %s: duplicate records", definition.code)
			}
			if len(matching) == 1 {
				if matching[0].DeletedAt.Valid {
					return fmt.Errorf("ensure builtin menu %s: deleted_at contains history", definition.code)
				}
				active[definition.code] = matching[0]
			}
		}

		for _, definition := range definitions {
			parentID, err := builtinParentID(definition, active)
			if err != nil {
				return err
			}
			record, exists := active[definition.code]
			if exists {
				if err := validateBuiltinRecord(record, definition, parentID); err != nil {
					return err
				}
				continue
			}
			record = Menu{
				ParentID: parentID, MenuType: definition.menuType, Code: definition.code,
				I18nKey: definition.i18nKey, Path: definition.path, ViewKey: definition.viewKey,
				Icon: definition.icon, SortOrder: definition.sortOrder, IsEnabled: yesno.Yes,
			}
			if err := repository.Create(ctx, &record); err != nil {
				return fmt.Errorf("ensure builtin menu %s: %w", definition.code, err)
			}
			active[definition.code] = record
		}
		return nil
	})
}

func builtinMenuDefinitions() []builtinMenuDefinition {
	return []builtinMenuDefinition{
		{
			code: BuiltinSystemCode, menuType: TypeDirectory, i18nKey: "navigation.system",
			icon: builtinString("Setting"), sortOrder: 100,
		},
		{
			code: PermissionList, menuType: TypePage, parentCode: BuiltinSystemCode,
			i18nKey: "navigation.systemMenus", path: builtinString("/system/menus"),
			viewKey: builtinString("system-menus"), icon: builtinString("Menu"), sortOrder: 10,
		},
		{
			code: PermissionCreate, menuType: TypeAction, parentCode: PermissionList,
			i18nKey: "permission.menuCreate", sortOrder: 10,
		},
		{
			code: PermissionUpdate, menuType: TypeAction, parentCode: PermissionList,
			i18nKey: "permission.menuUpdate", sortOrder: 20,
		},
		{
			code: PermissionDelete, menuType: TypeAction, parentCode: PermissionList,
			i18nKey: "permission.menuDelete", sortOrder: 30,
		},
		{
			code: PermissionRoleList, menuType: TypePage, parentCode: BuiltinSystemCode,
			i18nKey: "navigation.systemRoles", path: builtinString("/system/roles"),
			viewKey: builtinString("system-roles"), icon: builtinString("UserFilled"), sortOrder: 20,
		},
		{
			code: PermissionRoleCreate, menuType: TypeAction, parentCode: PermissionRoleList,
			i18nKey: "permission.roleCreate", sortOrder: 10,
		},
		{
			code: PermissionRoleUpdate, menuType: TypeAction, parentCode: PermissionRoleList,
			i18nKey: "permission.roleUpdate", sortOrder: 20,
		},
		{
			code: PermissionRoleStatus, menuType: TypeAction, parentCode: PermissionRoleList,
			i18nKey: "permission.roleStatus", sortOrder: 30,
		},
		{
			code: PermissionRoleDefault, menuType: TypeAction, parentCode: PermissionRoleList,
			i18nKey: "permission.roleSetDefault", sortOrder: 40,
		},
		{
			code: PermissionRoleDelete, menuType: TypeAction, parentCode: PermissionRoleList,
			i18nKey: "permission.roleDelete", sortOrder: 50,
		},
		{
			code: PermissionRoleAuthorize, menuType: TypeAction, parentCode: PermissionRoleList,
			i18nKey: "permission.roleAuthorize", sortOrder: 60,
		},
		{
			code: PermissionUserList, menuType: TypePage, parentCode: BuiltinSystemCode,
			i18nKey: "navigation.systemUsers", path: builtinString("/system/users"),
			viewKey: builtinString("system-users"), icon: builtinString("User"), sortOrder: 30,
		},
		{
			code: PermissionUserUpdate, menuType: TypeAction, parentCode: PermissionUserList,
			i18nKey: "permission.userUpdate", sortOrder: 10,
		},
		{
			code: PermissionUserStatus, menuType: TypeAction, parentCode: PermissionUserList,
			i18nKey: "permission.userStatus", sortOrder: 20,
		},
		{
			code: PermissionUserDelete, menuType: TypeAction, parentCode: PermissionUserList,
			i18nKey: "permission.userDelete", sortOrder: 30,
		},
		{
			code: PermissionUserRoles, menuType: TypeAction, parentCode: PermissionUserList,
			i18nKey: "permission.userRoles", sortOrder: 40,
		},
		{
			code: PermissionAuthPlatformList, menuType: TypePage, parentCode: BuiltinSystemCode,
			i18nKey: "navigation.systemAuthPlatforms", path: builtinString("/system/auth-platforms"),
			viewKey: builtinString("system-auth-platforms"), icon: builtinString("Key"), sortOrder: 40,
		},
		{
			code: PermissionAuthPlatformCreate, menuType: TypeAction, parentCode: PermissionAuthPlatformList,
			i18nKey: "permission.authPlatformCreate", sortOrder: 10,
		},
		{
			code: PermissionAuthPlatformUpdate, menuType: TypeAction, parentCode: PermissionAuthPlatformList,
			i18nKey: "permission.authPlatformUpdate", sortOrder: 20,
		},
		{
			code: PermissionAuthPlatformStatus, menuType: TypeAction, parentCode: PermissionAuthPlatformList,
			i18nKey: "permission.authPlatformStatus", sortOrder: 30,
		},
		{
			code: PermissionAuthPlatformDelete, menuType: TypeAction, parentCode: PermissionAuthPlatformList,
			i18nKey: "permission.authPlatformDelete", sortOrder: 40,
		},
	}
}

func builtinCodesFromDefinitions(definitions []builtinMenuDefinition) []string {
	codes := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		codes = append(codes, definition.code)
	}
	return codes
}

func builtinParentID(definition builtinMenuDefinition, active map[string]Menu) (*int64, error) {
	if definition.parentCode == "" {
		return nil, nil
	}
	parent, ok := active[definition.parentCode]
	if !ok {
		return nil, fmt.Errorf("ensure builtin menu %s: parent %s is missing", definition.code, definition.parentCode)
	}
	parentID := parent.ID
	return &parentID, nil
}

func validateBuiltinRecord(record Menu, definition builtinMenuDefinition, parentID *int64) error {
	if record.MenuType != definition.menuType {
		return builtinFieldError(definition.code, "menu_type")
	}
	if !sameBuiltinInt64Pointer(record.ParentID, parentID) {
		return builtinFieldError(definition.code, "parent_id")
	}
	if record.I18nKey != definition.i18nKey {
		return builtinFieldError(definition.code, "i18n_key")
	}
	if !sameBuiltinStringPointer(record.Path, definition.path) {
		return builtinFieldError(definition.code, "path")
	}
	if !sameBuiltinStringPointer(record.ViewKey, definition.viewKey) {
		return builtinFieldError(definition.code, "view_key")
	}
	if record.IsEnabled != yesno.Yes {
		return builtinFieldError(definition.code, "is_enabled")
	}
	return nil
}

func builtinFieldError(code, field string) error {
	return fmt.Errorf("ensure builtin menu %s: field %s is invalid", code, field)
}

func sameBuiltinStringPointer(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func sameBuiltinInt64Pointer(left, right *int64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func builtinString(value string) *string {
	return &value
}
