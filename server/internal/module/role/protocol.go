package role

import "regexp"

const (
	CodeSuperAdmin     = "super_admin"
	CodeRegisteredUser = "registered_user"
)

var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

func IsSystemCode(code string) bool {
	return code == CodeSuperAdmin || code == CodeRegisteredUser
}

func IsValidCode(code string) bool {
	return roleCodePattern.MatchString(code)
}
