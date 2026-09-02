package operationlog

import "net/http"

type RouteRule struct {
	Method          string
	Route           string
	Module          string
	Action          string
	CaptureRequest  bool
	CaptureResponse bool
}

var routeRules = []RouteRule{
	{http.MethodPost, "/api/admin/v1/auth-platforms", "authPlatform", "authPlatform.create", true, true},
	{http.MethodPut, "/api/admin/v1/auth-platforms/:id", "authPlatform", "authPlatform.update", true, true},
	{http.MethodPatch, "/api/admin/v1/auth-platforms/:id/status", "authPlatform", "authPlatform.status", true, true},
	{http.MethodDelete, "/api/admin/v1/auth-platforms/:id", "authPlatform", "authPlatform.delete", false, true},
	{http.MethodPost, "/api/admin/v1/menus", "menu", "menu.create", true, true},
	{http.MethodPut, "/api/admin/v1/menus/:id", "menu", "menu.update", true, true},
	{http.MethodPatch, "/api/admin/v1/menus/:id/status", "menu", "menu.status", true, true},
	{http.MethodDelete, "/api/admin/v1/menus/:id", "menu", "menu.delete", false, true},
	{http.MethodPost, "/api/admin/v1/roles", "role", "role.create", true, true},
	{http.MethodPut, "/api/admin/v1/roles/:id", "role", "role.update", true, true},
	{http.MethodPatch, "/api/admin/v1/roles/:id/status", "role", "role.status", true, true},
	{http.MethodPatch, "/api/admin/v1/roles/:id/default", "role", "role.default", false, true},
	{http.MethodDelete, "/api/admin/v1/roles/:id", "role", "role.delete", false, true},
	{http.MethodPut, "/api/admin/v1/roles/:id/permissions", "role", "role.permissions.update", true, true},
	{http.MethodPut, "/api/admin/v1/users/:id", "user", "user.update", true, true},
	{http.MethodPatch, "/api/admin/v1/users/:id/status", "user", "user.status", true, true},
	{http.MethodDelete, "/api/admin/v1/users/:id", "user", "user.delete", false, true},
	{http.MethodPut, "/api/admin/v1/users/:id/roles", "user", "user.roles.update", true, true},
	{http.MethodDelete, "/api/admin/v1/sessions/:id", "session", "session.revoke", false, true},
	{http.MethodDelete, "/api/admin/v1/sessions", "session", "session.revoke.bulk", true, true},
	{http.MethodPut, "/api/admin/v1/account/profile", "account", "account.profile.update", true, true},
	{http.MethodPost, "/api/admin/v1/account/password", "account", "account.password.change", true, true},
	{http.MethodPost, "/api/admin/v1/storage/cos-configs", "storage", "storage.cos-config.create", true, true},
	{http.MethodPut, "/api/admin/v1/storage/cos-configs/:id", "storage", "storage.cos-config.update", true, true},
	{http.MethodPatch, "/api/admin/v1/storage/cos-configs/:id/status", "storage", "storage.cos-config.status", true, true},
	{http.MethodPost, "/api/admin/v1/storage/cos-configs/:id/test", "storage", "storage.cos-config.test", false, true},
	{http.MethodDelete, "/api/admin/v1/storage/cos-configs/:id", "storage", "storage.cos-config.delete", false, true},
	{http.MethodPost, "/api/admin/v1/storage/upload-rules", "storage", "storage.upload-rule.create", true, true},
	{http.MethodPut, "/api/admin/v1/storage/upload-rules/:id", "storage", "storage.upload-rule.update", true, true},
	{http.MethodPatch, "/api/admin/v1/storage/upload-rules/:id/status", "storage", "storage.upload-rule.status", true, true},
	{http.MethodDelete, "/api/admin/v1/storage/upload-rules/:id", "storage", "storage.upload-rule.delete", false, true},
	{http.MethodPut, "/api/admin/v1/mail/config", "mail", "mail.config.update", true, false},
	{http.MethodDelete, "/api/admin/v1/mail/config", "mail", "mail.config.delete", false, false},
	{http.MethodPost, "/api/admin/v1/mail/test", "mail", "mail.test", false, false},
	{http.MethodPut, "/api/admin/v1/mail/templates/:id", "mail", "mail.template.update", true, false},
	{http.MethodPatch, "/api/admin/v1/mail/templates/:id/status", "mail", "mail.template.status", true, false},
	{http.MethodDelete, "/api/admin/v1/mail/logs/:id", "mail", "mail.log.delete", false, false},
	{http.MethodDelete, "/api/admin/v1/mail/logs", "mail", "mail.log.delete.bulk", false, false},
	{http.MethodGet, "/api/admin/v1/mail/logs/:id", "mail", "mail.log.detail", false, false},
	{http.MethodPost, "/api/admin/v1/mail/recipient-rules", "mail", "mail.rule.create", true, false},
	{http.MethodPut, "/api/admin/v1/mail/recipient-rules/:id", "mail", "mail.rule.update", true, false},
	{http.MethodPatch, "/api/admin/v1/mail/recipient-rules/:id/status", "mail", "mail.rule.status", true, false},
	{http.MethodDelete, "/api/admin/v1/mail/recipient-rules/:id", "mail", "mail.rule.delete", false, false},
}

func FindRule(method, route string) (RouteRule, bool) {
	for _, rule := range routeRules {
		if rule.Method == method && rule.Route == route {
			return rule, true
		}
	}
	return RouteRule{}, false
}
