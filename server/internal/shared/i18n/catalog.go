package i18n

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Locale string

type MessageKey string

const (
	ZhCN Locale = "zh-CN"
	EnUS Locale = "en-US"
)

const (
	KeyInternal                    MessageKey = "error.internal"
	KeyInvalidRequest              MessageKey = "error.invalidRequest"
	KeyUnauthorized                MessageKey = "error.unauthorized"
	KeyForbidden                   MessageKey = "error.forbidden"
	KeyNotFound                    MessageKey = "error.notFound"
	KeyConflict                    MessageKey = "error.conflict"
	KeyDependencyUnavailable       MessageKey = "error.dependencyUnavailable"
	KeyUsernameConflict            MessageKey = "auth.usernameConflict"
	KeyEmailConflict               MessageKey = "auth.emailConflict"
	KeySuperAdminExists            MessageKey = "auth.superAdminExists"
	KeyPermissionDenied            MessageKey = "access.permissionDenied"
	KeyAccessSnapshotInvalid       MessageKey = "access.snapshotInvalid"
	KeyMenuTreeInvalid             MessageKey = "menu.treeInvalid"
	KeyMenuNotFound                MessageKey = "menu.notFound"
	KeyMenuCodeConflict            MessageKey = "menu.codeConflict"
	KeyMenuPathConflict            MessageKey = "menu.pathConflict"
	KeyMenuInvalidParent           MessageKey = "menu.invalidParent"
	KeyMenuCycleDetected           MessageKey = "menu.cycleDetected"
	KeyMenuBuiltinProtected        MessageKey = "menu.builtinProtected"
	KeyMenuParentDisabled          MessageKey = "menu.parentDisabled"
	KeyMenuStructureConflict       MessageKey = "menu.structureConflict"
	KeyMenuInvalidFields           MessageKey = "menu.invalidFields"
	KeyRoleNotFound                MessageKey = "role.notFound"
	KeyRoleCodeConflict            MessageKey = "role.codeConflict"
	KeyRoleNameConflict            MessageKey = "role.nameConflict"
	KeyRoleSystemProtected         MessageKey = "role.systemProtected"
	KeyRoleDefaultProtected        MessageKey = "role.defaultProtected"
	KeyRoleUsersAttached           MessageKey = "role.usersAttached"
	KeyRoleInvalidState            MessageKey = "role.invalidState"
	KeyRoleInvalidPermission       MessageKey = "role.invalidPermission"
	KeyRoleSuperAdminAuthorization MessageKey = "role.superAdminAuthorization"
	KeyRoleDataInvalid             MessageKey = "role.dataInvalid"
)

var catalogs = map[Locale]map[MessageKey]string{
	ZhCN: {
		KeyInternal:                    "服务内部错误",
		KeyInvalidRequest:              "请求参数错误",
		KeyUnauthorized:                "未登录或登录已失效",
		KeyForbidden:                   "无权执行该操作",
		KeyNotFound:                    "请求的资源不存在",
		KeyConflict:                    "数据冲突",
		KeyDependencyUnavailable:       "服务暂未就绪",
		KeyUsernameConflict:            "用户名已存在",
		KeyEmailConflict:               "邮箱已存在",
		KeySuperAdminExists:            "超级管理员已存在",
		KeyPermissionDenied:            "无权执行 {{permission}}",
		KeyAccessSnapshotInvalid:       "访问权限数据无效",
		KeyMenuTreeInvalid:             "菜单树数据无效",
		KeyMenuNotFound:                "菜单不存在",
		KeyMenuCodeConflict:            "菜单编码 {{code}} 已存在",
		KeyMenuPathConflict:            "页面路径 {{path}} 已存在",
		KeyMenuInvalidParent:           "父菜单无效",
		KeyMenuCycleDetected:           "菜单层级存在循环",
		KeyMenuBuiltinProtected:        "核心菜单 {{code}} 不允许执行该操作",
		KeyMenuParentDisabled:          "菜单 {{code}} 的父级未全部启用",
		KeyMenuStructureConflict:       "菜单 {{code}} 的结构与现有子节点或授权冲突",
		KeyMenuInvalidFields:           "菜单字段无效",
		KeyRoleNotFound:                "角色不存在",
		KeyRoleCodeConflict:            "角色编码 {{code}} 已存在",
		KeyRoleNameConflict:            "角色名称 {{name}} 已存在",
		KeyRoleSystemProtected:         "系统角色 {{code}} 不允许执行该操作",
		KeyRoleDefaultProtected:        "默认角色 {{code}} 不允许执行该操作",
		KeyRoleUsersAttached:           "角色 {{code}} 仍绑定用户",
		KeyRoleInvalidState:            "角色状态无效",
		KeyRoleInvalidPermission:       "授权菜单无效",
		KeyRoleSuperAdminAuthorization: "超级管理员不允许配置角色授权",
		KeyRoleDataInvalid:             "角色权限数据无效",
	},
	EnUS: {
		KeyInternal:                    "Internal server error",
		KeyInvalidRequest:              "Invalid request",
		KeyUnauthorized:                "Authentication is required or has expired",
		KeyForbidden:                   "Permission denied",
		KeyNotFound:                    "Resource not found",
		KeyConflict:                    "Data conflict",
		KeyDependencyUnavailable:       "Service is temporarily unavailable",
		KeyUsernameConflict:            "Username already exists",
		KeyEmailConflict:               "Email already exists",
		KeySuperAdminExists:            "A super administrator already exists",
		KeyPermissionDenied:            "Permission denied: {{permission}}",
		KeyAccessSnapshotInvalid:       "Access permission data is invalid",
		KeyMenuTreeInvalid:             "Menu tree data is invalid",
		KeyMenuNotFound:                "Menu not found",
		KeyMenuCodeConflict:            "Menu code {{code}} already exists",
		KeyMenuPathConflict:            "Page path {{path}} already exists",
		KeyMenuInvalidParent:           "Invalid parent menu",
		KeyMenuCycleDetected:           "Menu hierarchy contains a cycle",
		KeyMenuBuiltinProtected:        "Builtin menu {{code}} cannot be changed by this operation",
		KeyMenuParentDisabled:          "The parent hierarchy of menu {{code}} is not fully enabled",
		KeyMenuStructureConflict:       "Menu {{code}} conflicts with existing children or grants",
		KeyMenuInvalidFields:           "Invalid menu fields",
		KeyRoleNotFound:                "Role not found",
		KeyRoleCodeConflict:            "Role code {{code}} already exists",
		KeyRoleNameConflict:            "Role name {{name}} already exists",
		KeyRoleSystemProtected:         "System role {{code}} cannot be changed by this operation",
		KeyRoleDefaultProtected:        "Default role {{code}} cannot be changed by this operation",
		KeyRoleUsersAttached:           "Role {{code}} still has assigned users",
		KeyRoleInvalidState:            "Invalid role state",
		KeyRoleInvalidPermission:       "Invalid permission menu",
		KeyRoleSuperAdminAuthorization: "Super administrator permissions cannot be configured",
		KeyRoleDataInvalid:             "Role permission data is invalid",
	},
}

var interpolationPattern = regexp.MustCompile(`\{\{([A-Za-z][A-Za-z0-9_]*)\}\}`)

type localeContextKey struct{}

func ParseLocale(value string) (Locale, error) {
	locale := Locale(value)
	if locale != ZhCN && locale != EnUS {
		return "", fmt.Errorf("unsupported locale %q", value)
	}
	return locale, nil
}

func WithLocale(ctx context.Context, locale Locale) context.Context {
	return context.WithValue(ctx, localeContextKey{}, locale)
}

func LocaleFromContext(ctx context.Context) Locale {
	locale, ok := ctx.Value(localeContextKey{}).(Locale)
	if !ok {
		return ZhCN
	}
	return locale
}

func ValidateCatalogs() error {
	reference, ok := catalogs[ZhCN]
	if !ok {
		return fmt.Errorf("catalog %q is missing", ZhCN)
	}
	for _, locale := range []Locale{ZhCN, EnUS} {
		catalog, exists := catalogs[locale]
		if !exists {
			return fmt.Errorf("catalog %q is missing", locale)
		}
		if err := validateCatalog(locale, reference, catalog); err != nil {
			return err
		}
	}
	return nil
}

func Translate(locale Locale, key MessageKey, params map[string]string) (string, error) {
	catalog, ok := catalogs[locale]
	if !ok {
		return "", fmt.Errorf("unsupported locale %q", locale)
	}
	template, ok := catalog[key]
	if !ok {
		return "", fmt.Errorf("message key %q is not defined for locale %q", key, locale)
	}

	expected := interpolationParameters(template)
	if err := validateParameters(key, expected, params); err != nil {
		return "", err
	}
	message := template
	for name := range expected {
		message = strings.ReplaceAll(message, "{{"+name+"}}", params[name])
	}
	return message, nil
}

func validateCatalog(locale Locale, reference, candidate map[MessageKey]string) error {
	if len(candidate) != len(reference) {
		return fmt.Errorf("catalog %q has %d keys, want %d", locale, len(candidate), len(reference))
	}
	for key, referenceMessage := range reference {
		candidateMessage, ok := candidate[key]
		if !ok {
			return fmt.Errorf("catalog %q is missing message key %q", locale, key)
		}
		if !sameNames(interpolationParameters(referenceMessage), interpolationParameters(candidateMessage)) {
			return fmt.Errorf("catalog %q message key %q has different interpolation parameters", locale, key)
		}
	}
	return nil
}

func interpolationParameters(message string) map[string]struct{} {
	parameters := make(map[string]struct{})
	for _, match := range interpolationPattern.FindAllStringSubmatch(message, -1) {
		parameters[match[1]] = struct{}{}
	}
	return parameters
}

func validateParameters(key MessageKey, expected map[string]struct{}, params map[string]string) error {
	if len(params) != len(expected) {
		return fmt.Errorf("message key %q parameters = %v, want %v", key, sortedMapKeys(params), sortedSetKeys(expected))
	}
	for name := range expected {
		if _, ok := params[name]; !ok {
			return fmt.Errorf("message key %q is missing parameter %q", key, name)
		}
	}
	return nil
}

func sameNames(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for name := range left {
		if _, ok := right[name]; !ok {
			return false
		}
	}
	return true
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
