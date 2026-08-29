package role

import "regexp"

const (
	CodeSuperAdmin     = "super_admin"
	CodeRegisteredUser = "registered_user"

	PermissionList      = "rbac:role:list"
	PermissionCreate    = "rbac:role:create"
	PermissionUpdate    = "rbac:role:update"
	PermissionStatus    = "rbac:role:status"
	PermissionDefault   = "rbac:role:default"
	PermissionDelete    = "rbac:role:delete"
	PermissionAuthorize = "rbac:role:authorize"
)

var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

func IsSystemCode(code string) bool {
	return code == CodeSuperAdmin || code == CodeRegisteredUser
}

func IsValidCode(code string) bool {
	return roleCodePattern.MatchString(code)
}
