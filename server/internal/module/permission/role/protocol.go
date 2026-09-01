package role

import "regexp"

const (
	CodeSuperAdmin     = "super_admin"
	CodeRegisteredUser = "registered_user"

	PermissionView      = "permission:role:view"
	PermissionList      = "permission:role:list"
	PermissionCreate    = "permission:role:create"
	PermissionUpdate    = "permission:role:update"
	PermissionStatus    = "permission:role:status"
	PermissionDefault   = "permission:role:default"
	PermissionDelete    = "permission:role:delete"
	PermissionAuthorize = "permission:role:authorize"
)

var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

func IsSystemCode(code string) bool {
	return code == CodeSuperAdmin || code == CodeRegisteredUser
}

func IsValidCode(code string) bool {
	return roleCodePattern.MatchString(code)
}
