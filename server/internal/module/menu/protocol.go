package menu

const (
	PermissionList          = "system:menu:list"
	PermissionCreate        = "system:menu:create"
	PermissionUpdate        = "system:menu:update"
	PermissionDelete        = "system:menu:delete"
	PermissionRoleList      = "system:role:list"
	PermissionRoleCreate    = "system:role:create"
	PermissionRoleUpdate    = "system:role:update"
	PermissionRoleStatus    = "system:role:status"
	PermissionRoleDefault   = "system:role:default"
	PermissionRoleDelete    = "system:role:delete"
	PermissionRoleAuthorize = "system:role:authorize"

	BuiltinSystemCode   = "system"
	BuiltinMenuListCode = PermissionList
)

var menuTitleKeys = map[string]struct{}{
	"navigation.system":         {},
	"navigation.systemMenus":    {},
	"permission.menuCreate":     {},
	"permission.menuUpdate":     {},
	"permission.menuDelete":     {},
	"navigation.systemRoles":    {},
	"permission.roleCreate":     {},
	"permission.roleUpdate":     {},
	"permission.roleStatus":     {},
	"permission.roleSetDefault": {},
	"permission.roleDelete":     {},
	"permission.roleAuthorize":  {},
}

var menuViewKeys = map[string]struct{}{
	"system-menus": {},
	"system-roles": {},
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
	BuiltinSystemCode:       {},
	BuiltinMenuListCode:     {},
	PermissionCreate:        {},
	PermissionUpdate:        {},
	PermissionDelete:        {},
	PermissionRoleList:      {},
	PermissionRoleCreate:    {},
	PermissionRoleUpdate:    {},
	PermissionRoleStatus:    {},
	PermissionRoleDefault:   {},
	PermissionRoleDelete:    {},
	PermissionRoleAuthorize: {},
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
