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
	KeyInternal                     MessageKey = "error.internal"
	KeyInvalidRequest               MessageKey = "error.invalidRequest"
	KeyUnauthorized                 MessageKey = "error.unauthorized"
	KeyForbidden                    MessageKey = "error.forbidden"
	KeyNotFound                     MessageKey = "error.notFound"
	KeyConflict                     MessageKey = "error.conflict"
	KeyDependencyUnavailable        MessageKey = "error.dependencyUnavailable"
	KeyRateLimited                  MessageKey = "error.rateLimited"
	KeyUsernameConflict             MessageKey = "auth.usernameConflict"
	KeyEmailConflict                MessageKey = "auth.emailConflict"
	KeySuperAdminExists             MessageKey = "auth.superAdminExists"
	KeyPermissionDenied             MessageKey = "permission.permissionDenied"
	KeyPermissionSnapshotInvalid    MessageKey = "permission.snapshotInvalid"
	KeyAccessUpdating               MessageKey = "permission.updating"
	KeyMenuTreeInvalid              MessageKey = "menu.treeInvalid"
	KeyMenuNotFound                 MessageKey = "menu.notFound"
	KeyMenuCodeConflict             MessageKey = "menu.codeConflict"
	KeyMenuPathConflict             MessageKey = "menu.pathConflict"
	KeyMenuInvalidParent            MessageKey = "menu.invalidParent"
	KeyMenuCycleDetected            MessageKey = "menu.cycleDetected"
	KeyMenuProtected                MessageKey = "menu.protected"
	KeyMenuParentDisabled           MessageKey = "menu.parentDisabled"
	KeyMenuStructureConflict        MessageKey = "menu.structureConflict"
	KeyMenuInvalidFields            MessageKey = "menu.invalidFields"
	KeyMenuPlatformUnavailable      MessageKey = "menu.platformUnavailable"
	KeyRoleNotFound                 MessageKey = "role.notFound"
	KeyRoleCodeConflict             MessageKey = "role.codeConflict"
	KeyRoleNameConflict             MessageKey = "role.nameConflict"
	KeyRoleSystemProtected          MessageKey = "role.systemProtected"
	KeyRoleDefaultProtected         MessageKey = "role.defaultProtected"
	KeyRoleUsersAttached            MessageKey = "role.usersAttached"
	KeyRoleInvalidState             MessageKey = "role.invalidState"
	KeyRoleInvalidPermission        MessageKey = "role.invalidPermission"
	KeyRoleSuperAdminAuthorization  MessageKey = "role.superAdminAuthorization"
	KeyRoleDataInvalid              MessageKey = "role.dataInvalid"
	KeyUserNotFound                 MessageKey = "user.notFound"
	KeyUserUsernameConflict         MessageKey = "user.usernameConflict"
	KeyUserPhoneConflict            MessageKey = "user.phoneConflict"
	KeyUserSelfOperation            MessageKey = "user.selfOperation"
	KeyUserSuperAdminProtected      MessageKey = "user.superAdminProtected"
	KeyUserLastSuperAdmin           MessageKey = "user.lastSuperAdmin"
	KeyUserInvalidRoles             MessageKey = "user.invalidRoles"
	KeyUserRoleNotFound             MessageKey = "user.roleNotFound"
	KeyUserDataInvalid              MessageKey = "user.dataInvalid"
	KeyAuthPlatformNotFound         MessageKey = "authPlatform.notFound"
	KeyAuthPlatformCodeConflict     MessageKey = "authPlatform.codeConflict"
	KeyAuthPlatformBuiltinProtected MessageKey = "authPlatform.builtinProtected"
	KeyAuthPlatformInvalidPolicy    MessageKey = "authPlatform.invalidPolicy"
	KeyAuthPlatformMenusAttached    MessageKey = "authPlatform.menusAttached"
	KeyAuthPlatformDisabled         MessageKey = "authPlatform.disabled"
	KeyAuthSessionUpdating          MessageKey = "auth.sessionUpdating"
	KeyAuthPlatformUnavailable      MessageKey = "authPlatform.unavailable"
	KeySessionInvalidID             MessageKey = "session.invalidId"
	KeySessionNotFound              MessageKey = "session.notFound"
	KeySessionCurrentProtected      MessageKey = "session.currentProtected"
	KeySessionQueryFailed           MessageKey = "session.queryFailed"
	KeySessionRevokeFailed          MessageKey = "session.revokeFailed"
	KeyOperationLogQueryFailed      MessageKey = "operationLog.queryFailed"
	KeyMailRecipientDenied          MessageKey = "mail.recipientDenied"
	KeyMailRateLimitInvalid         MessageKey = "mail.rateLimitInvalid"
	KeyMailRateLimitNotFound        MessageKey = "mail.rateLimitNotFound"
	KeyMailRateLimitUnavailable     MessageKey = "mail.rateLimitUnavailable"
)

var catalogs = map[Locale]map[MessageKey]string{
	ZhCN: {
		KeyInternal:                     "服务内部错误",
		KeyInvalidRequest:               "请求参数错误",
		KeyUnauthorized:                 "未登录或登录已失效",
		KeyForbidden:                    "无权执行该操作",
		KeyNotFound:                     "请求的资源不存在",
		KeyConflict:                     "数据冲突",
		KeyDependencyUnavailable:        "服务暂未就绪",
		KeyRateLimited:                  "请求过于频繁",
		KeyUsernameConflict:             "用户名已存在",
		KeyEmailConflict:                "邮箱已存在",
		KeySuperAdminExists:             "超级管理员已存在",
		KeyPermissionDenied:             "无权执行 {{permission}}",
		KeyPermissionSnapshotInvalid:    "访问权限数据无效",
		KeyAccessUpdating:               "访问权限正在更新",
		KeyMenuTreeInvalid:              "菜单树数据无效",
		KeyMenuNotFound:                 "菜单不存在",
		KeyMenuCodeConflict:             "菜单编码 {{code}} 已存在",
		KeyMenuPathConflict:             "页面路径 {{path}} 已存在",
		KeyMenuInvalidParent:            "父菜单无效",
		KeyMenuCycleDetected:            "菜单层级存在循环",
		KeyMenuProtected:                "基础菜单 {{code}} 不允许执行该操作",
		KeyMenuParentDisabled:           "菜单 {{code}} 的父级未全部启用",
		KeyMenuStructureConflict:        "菜单 {{code}} 的结构与现有子节点或授权冲突",
		KeyMenuInvalidFields:            "菜单字段无效",
		KeyMenuPlatformUnavailable:      "菜单所属平台不存在或已删除",
		KeyRoleNotFound:                 "角色不存在",
		KeyRoleCodeConflict:             "角色编码 {{code}} 已存在",
		KeyRoleNameConflict:             "角色名称 {{name}} 已存在",
		KeyRoleSystemProtected:          "系统角色 {{code}} 不允许执行该操作",
		KeyRoleDefaultProtected:         "默认角色 {{code}} 不允许执行该操作",
		KeyRoleUsersAttached:            "角色 {{code}} 仍绑定用户",
		KeyRoleInvalidState:             "角色状态无效",
		KeyRoleInvalidPermission:        "授权菜单无效",
		KeyRoleSuperAdminAuthorization:  "超级管理员不允许配置角色授权",
		KeyRoleDataInvalid:              "角色权限数据无效",
		KeyUserNotFound:                 "用户不存在",
		KeyUserUsernameConflict:         "用户名已存在",
		KeyUserPhoneConflict:            "手机号已存在",
		KeyUserSelfOperation:            "不能对自己的账号执行该操作",
		KeyUserSuperAdminProtected:      "只有超级管理员可以操作超级管理员账号",
		KeyUserLastSuperAdmin:           "系统必须保留至少一个有效超级管理员",
		KeyUserInvalidRoles:             "用户角色集合无效",
		KeyUserRoleNotFound:             "角色不存在",
		KeyUserDataInvalid:              "用户角色数据无效",
		KeyAuthPlatformNotFound:         "认证平台不存在",
		KeyAuthPlatformCodeConflict:     "认证平台编码已存在",
		KeyAuthPlatformBuiltinProtected: "内置认证平台不允许删除",
		KeyAuthPlatformInvalidPolicy:    "认证平台策略无效",
		KeyAuthPlatformMenusAttached:    "认证平台仍有关联菜单，请先删除菜单",
		KeyAuthPlatformDisabled:         "认证平台已停用",
		KeyAuthSessionUpdating:          "会话状态正在更新",
		KeyAuthPlatformUnavailable:      "认证平台服务暂不可用",
		KeySessionInvalidID:             "会话 ID 无效",
		KeySessionNotFound:              "会话不存在",
		KeySessionCurrentProtected:      "当前登录会话不能被踢出",
		KeySessionQueryFailed:           "会话查询失败",
		KeySessionRevokeFailed:          "会话撤销失败",
		KeyOperationLogQueryFailed:      "操作日志查询失败",
		KeyMailRecipientDenied:          "收件邮箱被收件规则拒绝",
		KeyMailRateLimitInvalid:         "邮件限流策略参数无效",
		KeyMailRateLimitNotFound:        "邮件限流策略不存在",
		KeyMailRateLimitUnavailable:     "邮件限流策略暂不可用",
	},
	EnUS: {
		KeyInternal:                     "Internal server error",
		KeyInvalidRequest:               "Invalid request",
		KeyUnauthorized:                 "Authentication is required or has expired",
		KeyForbidden:                    "Permission denied",
		KeyNotFound:                     "Resource not found",
		KeyConflict:                     "Data conflict",
		KeyDependencyUnavailable:        "Service is temporarily unavailable",
		KeyRateLimited:                  "Too many requests",
		KeyUsernameConflict:             "Username already exists",
		KeyEmailConflict:                "Email already exists",
		KeySuperAdminExists:             "A super administrator already exists",
		KeyPermissionDenied:             "Permission denied: {{permission}}",
		KeyPermissionSnapshotInvalid:    "Access permission data is invalid",
		KeyAccessUpdating:               "Access permissions are updating",
		KeyMenuTreeInvalid:              "Menu tree data is invalid",
		KeyMenuNotFound:                 "Menu not found",
		KeyMenuCodeConflict:             "Menu code {{code}} already exists",
		KeyMenuPathConflict:             "Page path {{path}} already exists",
		KeyMenuInvalidParent:            "Invalid parent menu",
		KeyMenuCycleDetected:            "Menu hierarchy contains a cycle",
		KeyMenuProtected:                "Protected menu {{code}} cannot be changed by this operation",
		KeyMenuParentDisabled:           "The parent hierarchy of menu {{code}} is not fully enabled",
		KeyMenuStructureConflict:        "Menu {{code}} conflicts with existing children or grants",
		KeyMenuInvalidFields:            "Invalid menu fields",
		KeyMenuPlatformUnavailable:      "The menu platform does not exist or has been deleted",
		KeyRoleNotFound:                 "Role not found",
		KeyRoleCodeConflict:             "Role code {{code}} already exists",
		KeyRoleNameConflict:             "Role name {{name}} already exists",
		KeyRoleSystemProtected:          "System role {{code}} cannot be changed by this operation",
		KeyRoleDefaultProtected:         "Default role {{code}} cannot be changed by this operation",
		KeyRoleUsersAttached:            "Role {{code}} still has assigned users",
		KeyRoleInvalidState:             "Invalid role state",
		KeyRoleInvalidPermission:        "Invalid permission menu",
		KeyRoleSuperAdminAuthorization:  "Super administrator permissions cannot be configured",
		KeyRoleDataInvalid:              "Role permission data is invalid",
		KeyUserNotFound:                 "User not found",
		KeyUserUsernameConflict:         "Username already exists",
		KeyUserPhoneConflict:            "Phone number already exists",
		KeyUserSelfOperation:            "This operation cannot be performed on your own account",
		KeyUserSuperAdminProtected:      "Only a super administrator can operate on a super administrator account",
		KeyUserLastSuperAdmin:           "At least one effective super administrator must remain",
		KeyUserInvalidRoles:             "Invalid user role set",
		KeyUserRoleNotFound:             "Role not found",
		KeyUserDataInvalid:              "User role data is invalid",
		KeyAuthPlatformNotFound:         "Authentication platform not found",
		KeyAuthPlatformCodeConflict:     "Authentication platform code already exists",
		KeyAuthPlatformBuiltinProtected: "Builtin authentication platform cannot be deleted",
		KeyAuthPlatformInvalidPolicy:    "Invalid authentication platform policy",
		KeyAuthPlatformMenusAttached:    "Authentication platform still has active menus",
		KeyAuthPlatformDisabled:         "Authentication platform is disabled",
		KeyAuthSessionUpdating:          "Authentication session state is updating",
		KeyAuthPlatformUnavailable:      "Authentication platform service is temporarily unavailable",
		KeySessionInvalidID:             "Invalid session ID",
		KeySessionNotFound:              "Session not found",
		KeySessionCurrentProtected:      "The current login session cannot be revoked",
		KeySessionQueryFailed:           "Failed to query sessions",
		KeySessionRevokeFailed:          "Failed to revoke session",
		KeyOperationLogQueryFailed:      "Failed to query operation logs",
		KeyMailRecipientDenied:          "Recipient rejected by mail rule",
		KeyMailRateLimitInvalid:         "Mail rate limit policy parameters are invalid",
		KeyMailRateLimitNotFound:        "Mail rate limit policy not found",
		KeyMailRateLimitUnavailable:     "Mail rate limit policy is unavailable",
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
