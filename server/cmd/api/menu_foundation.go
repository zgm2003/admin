package main

import (
	"admin/server/internal/module/auth"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/menu"
	"admin/server/internal/module/operationlog"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/yesno"
)

func menuFoundation() []menu.FoundationDefinition {
	return []menu.FoundationDefinition{
		menuDirectory("用户与账号", "account", "navigation.account", "lucide:users-round", 100, false),
		menuPage("account", "用户管理", user.PermissionList, "navigation.accountUsers", "/account/users", "account/users", "lucide:user-round-cog", 10, false),
		menuAction(user.PermissionList, "修改用户", user.PermissionUpdate, 10, false),
		menuAction(user.PermissionList, "修改用户状态", user.PermissionStatus, 20, false),
		menuAction(user.PermissionList, "删除用户", user.PermissionDelete, 30, false),
		menuAction(user.PermissionList, "分配用户角色", user.PermissionRoles, 40, false),
		menuPage("account", "会话管理", auth.PermissionSessionList, "navigation.accountSessions", "/account/sessions", "account/sessions", "lucide:monitor-smartphone", 20, false),
		menuAction(auth.PermissionSessionList, "踢出会话", auth.PermissionSessionRevoke, 10, false),

		menuDirectory("权限与认证", "access", "navigation.access", "lucide:shield-check", 200, true),
		menuPage("access", "菜单管理", menu.PermissionList, "navigation.accessMenus", "/access/menus", "access/menus", "lucide:panel-left", 10, true),
		menuAction(menu.PermissionList, "新增菜单", menu.PermissionCreate, 10, true),
		menuAction(menu.PermissionList, "修改菜单", menu.PermissionUpdate, 20, true),
		menuAction(menu.PermissionList, "删除菜单", menu.PermissionDelete, 30, true),
		menuPage("access", "角色管理", role.PermissionList, "navigation.accessRoles", "/access/roles", "access/roles", "lucide:user-cog", 20, false),
		menuAction(role.PermissionList, "新增角色", role.PermissionCreate, 10, false),
		menuAction(role.PermissionList, "修改角色", role.PermissionUpdate, 20, false),
		menuAction(role.PermissionList, "修改角色状态", role.PermissionStatus, 30, false),
		menuAction(role.PermissionList, "设置默认角色", role.PermissionDefault, 40, false),
		menuAction(role.PermissionList, "删除角色", role.PermissionDelete, 50, false),
		menuAction(role.PermissionList, "配置角色权限", role.PermissionAuthorize, 60, false),
		menuPage("access", "认证平台", authplatform.PermissionList, "navigation.accessAuthPlatforms", "/access/auth-platforms", "access/auth-platforms", "lucide:key-round", 30, false),
		menuAction(authplatform.PermissionList, "新增认证平台", authplatform.PermissionCreate, 10, false),
		menuAction(authplatform.PermissionList, "编辑认证平台", authplatform.PermissionUpdate, 20, false),
		menuAction(authplatform.PermissionList, "变更认证平台状态", authplatform.PermissionStatus, 30, false),
		menuAction(authplatform.PermissionList, "删除认证平台", authplatform.PermissionDelete, 40, false),

		menuDirectory("系统管理", "system", "navigation.system", "lucide:settings-2", 300, false),
		menuPage("system", "操作日志", operationlog.PermissionList, "navigation.systemOperationLogs", "/system/operation-logs", "system/operation-logs", "lucide:scroll-text", 10, false),
	}
}

func canvasMenuFoundation() []menu.FoundationDefinition {
	return []menu.FoundationDefinition{
		{
			MenuType: menu.TypePage, Name: "Test", Code: "canvas:test", I18nKey: stringPointer("navigation.test"),
			Path: stringPointer("/test"), ComponentPath: stringPointer("test"),
			IsEnabled: yesno.Yes, IsHidden: yesno.No,
		},
		menuAction("canvas:test", "Test Button", "canvas:test:button", 10, false),
	}
}

func menuDirectory(name, code, i18nKey, icon string, sortOrder int, protected bool) menu.FoundationDefinition {
	return menu.FoundationDefinition{
		MenuType: menu.TypeDirectory, Name: name, Code: code, I18nKey: stringPointer(i18nKey), Icon: stringPointer(icon),
		SortOrder: sortOrder, IsEnabled: yesno.Yes, IsHidden: yesno.No, Protected: protected,
	}
}

func menuPage(parentCode, name, code, i18nKey, path, componentPath, icon string, sortOrder int, protected bool) menu.FoundationDefinition {
	return menu.FoundationDefinition{
		ParentCode: parentCode, MenuType: menu.TypePage, Name: name, Code: code, I18nKey: stringPointer(i18nKey),
		Path: stringPointer(path), ComponentPath: stringPointer(componentPath), Icon: stringPointer(icon),
		SortOrder: sortOrder, IsEnabled: yesno.Yes, IsHidden: yesno.No, Protected: protected,
	}
}

func menuAction(parentCode, name, code string, sortOrder int, protected bool) menu.FoundationDefinition {
	return menu.FoundationDefinition{
		ParentCode: parentCode, MenuType: menu.TypeAction, Name: name, Code: code,
		SortOrder: sortOrder, IsEnabled: yesno.Yes, IsHidden: yesno.Yes, Protected: protected,
	}
}

func stringPointer(value string) *string {
	return &value
}
