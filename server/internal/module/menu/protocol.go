package menu

const (
	PermissionList               = "system:menu:list"
	PermissionCreate             = "system:menu:create"
	PermissionUpdate             = "system:menu:update"
	PermissionDelete             = "system:menu:delete"
	PermissionRoleList           = "system:role:list"
	PermissionRoleCreate         = "system:role:create"
	PermissionRoleUpdate         = "system:role:update"
	PermissionRoleStatus         = "system:role:status"
	PermissionRoleDefault        = "system:role:default"
	PermissionRoleDelete         = "system:role:delete"
	PermissionRoleAuthorize      = "system:role:authorize"
	PermissionUserList           = "system:user:list"
	PermissionUserUpdate         = "system:user:update"
	PermissionUserStatus         = "system:user:status"
	PermissionUserDelete         = "system:user:delete"
	PermissionUserRoles          = "system:user:roles"
	PermissionAuthPlatformList   = "system:auth-platform:list"
	PermissionAuthPlatformCreate = "system:auth-platform:create"
	PermissionAuthPlatformUpdate = "system:auth-platform:update"
	PermissionAuthPlatformStatus = "system:auth-platform:status"
	PermissionAuthPlatformDelete = "system:auth-platform:delete"

	BuiltinSystemCode   = "system"
	BuiltinMenuListCode = PermissionList
)

var menuTitleKeys = map[string]struct{}{
	"navigation.system":              {},
	"navigation.systemMenus":         {},
	"permission.menuCreate":          {},
	"permission.menuUpdate":          {},
	"permission.menuDelete":          {},
	"navigation.systemRoles":         {},
	"permission.roleCreate":          {},
	"permission.roleUpdate":          {},
	"permission.roleStatus":          {},
	"permission.roleSetDefault":      {},
	"permission.roleDelete":          {},
	"permission.roleAuthorize":       {},
	"navigation.systemUsers":         {},
	"permission.userUpdate":          {},
	"permission.userStatus":          {},
	"permission.userDelete":          {},
	"permission.userRoles":           {},
	"navigation.systemAuthPlatforms": {},
	"permission.authPlatformCreate":  {},
	"permission.authPlatformUpdate":  {},
	"permission.authPlatformStatus":  {},
	"permission.authPlatformDelete":  {},
}

var menuViewKeys = map[string]struct{}{
	"system-menus":          {},
	"system-roles":          {},
	"system-users":          {},
	"system-auth-platforms": {},
}

var menuIconKeys = map[string]struct{}{
	"Cpu":        {},
	"Folder":     {},
	"Key":        {},
	"Menu":       {},
	"Setting":    {},
	"User":       {},
	"UserFilled": {},
}

var builtinCodes = map[string]struct{}{
	BuiltinSystemCode:            {},
	BuiltinMenuListCode:          {},
	PermissionCreate:             {},
	PermissionUpdate:             {},
	PermissionDelete:             {},
	PermissionRoleList:           {},
	PermissionRoleCreate:         {},
	PermissionRoleUpdate:         {},
	PermissionRoleStatus:         {},
	PermissionRoleDefault:        {},
	PermissionRoleDelete:         {},
	PermissionRoleAuthorize:      {},
	PermissionUserList:           {},
	PermissionUserUpdate:         {},
	PermissionUserStatus:         {},
	PermissionUserDelete:         {},
	PermissionUserRoles:          {},
	PermissionAuthPlatformList:   {},
	PermissionAuthPlatformCreate: {},
	PermissionAuthPlatformUpdate: {},
	PermissionAuthPlatformStatus: {},
	PermissionAuthPlatformDelete: {},
}

func IsMenuTitleKey(value string) bool {
	_, ok := menuTitleKeys[value]
	return ok
}

func IsMenuViewKey(value string) bool {
	_, ok := menuViewKeys[value]
	return ok
}

func IsMenuIconKey(value string) bool {
	_, ok := menuIconKeys[value]
	return ok
}

func IsBuiltinCode(value string) bool {
	_, ok := builtinCodes[value]
	return ok
}
