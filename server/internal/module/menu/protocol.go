package menu

const (
	PermissionList   = "system:menu:list"
	PermissionCreate = "system:menu:create"
	PermissionUpdate = "system:menu:update"
	PermissionDelete = "system:menu:delete"

	BuiltinSystemCode   = "system"
	BuiltinMenuListCode = PermissionList
)

var menuTitleKeys = map[string]struct{}{
	"navigation.system":      {},
	"navigation.systemMenus": {},
	"permission.menuCreate":  {},
	"permission.menuUpdate":  {},
	"permission.menuDelete":  {},
}

var menuViewKeys = map[string]struct{}{
	"system-menus": {},
}

var menuIconKeys = map[string]struct{}{
	"Cpu":     {},
	"Folder":  {},
	"Key":     {},
	"Menu":    {},
	"Setting": {},
	"User":    {},
}

var builtinCodes = map[string]struct{}{
	BuiltinSystemCode:   {},
	BuiltinMenuListCode: {},
	PermissionCreate:    {},
	PermissionUpdate:    {},
	PermissionDelete:    {},
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
