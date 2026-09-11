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
	{http.MethodPost, "/api/admin/v1/permission/authplatform", "authPlatform", "authPlatform.create", true, true},
	{http.MethodPut, "/api/admin/v1/permission/authplatform/:id", "authPlatform", "authPlatform.update", true, true},
	{http.MethodPatch, "/api/admin/v1/permission/authplatform/:id/status", "authPlatform", "authPlatform.status", true, true},
	{http.MethodDelete, "/api/admin/v1/permission/authplatform/:id", "authPlatform", "authPlatform.delete", false, true},
	{http.MethodPost, "/api/admin/v1/permission/menu", "menu", "menu.create", true, true},
	{http.MethodPut, "/api/admin/v1/permission/menu/:id", "menu", "menu.update", true, true},
	{http.MethodPatch, "/api/admin/v1/permission/menu/:id/status", "menu", "menu.status", true, true},
	{http.MethodDelete, "/api/admin/v1/permission/menu/:id", "menu", "menu.delete", false, true},
	{http.MethodPost, "/api/admin/v1/permission/role", "role", "role.create", true, true},
	{http.MethodPut, "/api/admin/v1/permission/role/:id", "role", "role.update", true, true},
	{http.MethodPatch, "/api/admin/v1/permission/role/:id/status", "role", "role.status", true, true},
	{http.MethodPatch, "/api/admin/v1/permission/role/:id/default", "role", "role.default", false, true},
	{http.MethodDelete, "/api/admin/v1/permission/role/:id", "role", "role.delete", false, true},
	{http.MethodPut, "/api/admin/v1/permission/role/:id/permission", "role", "role.permissions.update", true, true},
	{http.MethodPut, "/api/admin/v1/user/account/:id", "user", "user.update", true, true},
	{http.MethodPatch, "/api/admin/v1/user/account/:id/status", "user", "user.status", true, true},
	{http.MethodDelete, "/api/admin/v1/user/account/:id", "user", "user.delete", false, true},
	{http.MethodPut, "/api/admin/v1/user/account/:id/role", "user", "user.roles.update", true, true},
	{http.MethodDelete, "/api/admin/v1/user/session/:id", "session", "session.revoke", false, true},
	{http.MethodDelete, "/api/admin/v1/user/session", "session", "session.revoke.bulk", true, true},
	{http.MethodPut, "/api/admin/v1/user/profile", "user", "user.profile.update", true, true},
	{http.MethodPost, "/api/admin/v1/user/password", "user", "user.password.update", true, true},
	{http.MethodPost, "/api/admin/v1/user/password/send-code", "user", "user.password.update", false, false},
	{http.MethodPut, "/api/admin/v1/user/password/by-code", "user", "user.password.update", true, true},
	{http.MethodPost, "/api/admin/v1/user/phone/send-code", "user", "user.phone.update", false, false},
	{http.MethodPut, "/api/admin/v1/user/phone", "user", "user.phone.update", false, false},
	{http.MethodPost, "/api/admin/v1/user/email/send-code", "user", "user.email.update", false, false},
	{http.MethodPut, "/api/admin/v1/user/email", "user", "user.email.update", false, false},
	{http.MethodPost, "/api/admin/v1/storage/cosconfig", "storage", "storage.cos-config.create", true, true},
	{http.MethodPut, "/api/admin/v1/storage/cosconfig/:id", "storage", "storage.cos-config.update", true, true},
	{http.MethodPatch, "/api/admin/v1/storage/cosconfig/:id/status", "storage", "storage.cos-config.status", true, true},
	{http.MethodPost, "/api/admin/v1/storage/cosconfig/:id/test", "storage", "storage.cos-config.test", false, true},
	{http.MethodDelete, "/api/admin/v1/storage/cosconfig/:id", "storage", "storage.cos-config.delete", false, true},
	{http.MethodPost, "/api/admin/v1/storage/uploadrule", "storage", "storage.upload-rule.create", true, true},
	{http.MethodPut, "/api/admin/v1/storage/uploadrule/:id", "storage", "storage.upload-rule.update", true, true},
	{http.MethodPatch, "/api/admin/v1/storage/uploadrule/:id/status", "storage", "storage.upload-rule.status", true, true},
	{http.MethodDelete, "/api/admin/v1/storage/uploadrule/:id", "storage", "storage.upload-rule.delete", false, true},
	{http.MethodPut, "/api/admin/v1/message/mail/config", "mail", "mail.config.update", true, false},
	{http.MethodDelete, "/api/admin/v1/message/mail/config", "mail", "mail.config.delete", false, false},
	{http.MethodPost, "/api/admin/v1/message/mail/test", "mail", "mail.test", false, false},
	{http.MethodPut, "/api/admin/v1/message/mail/template/:id", "mail", "mail.template.update", true, false},
	{http.MethodPatch, "/api/admin/v1/message/mail/template/:id/status", "mail", "mail.template.status", true, false},
	{http.MethodGet, "/api/admin/v1/message/mail/log/:id", "mail", "mail.log.detail", false, false},
	{http.MethodPost, "/api/admin/v1/message/mail/recipient-rule", "mail", "mail.rule.create", true, false},
	{http.MethodPut, "/api/admin/v1/message/mail/recipient-rule/:id", "mail", "mail.rule.update", true, false},
	{http.MethodPatch, "/api/admin/v1/message/mail/recipient-rule/:id/status", "mail", "mail.rule.status", true, false},
	{http.MethodDelete, "/api/admin/v1/message/mail/recipient-rule/:id", "mail", "mail.rule.delete", false, false},
	{http.MethodPut, "/api/admin/v1/message/mail/rate-limit-policy/:platformId/:key", "mail", "mail.rate-limit.update", true, true},
	{http.MethodPut, "/api/admin/v1/message/sms/config", "sms", "sms.config.update", false, false},
	{http.MethodDelete, "/api/admin/v1/message/sms/config", "sms", "sms.config.delete", false, false},
	{http.MethodPost, "/api/admin/v1/message/sms/test", "sms", "sms.test", false, false},
	{http.MethodPut, "/api/admin/v1/message/sms/template/:id", "sms", "sms.template.update", true, false},
	{http.MethodPatch, "/api/admin/v1/message/sms/template/:id/status", "sms", "sms.template.status", true, false},
	{http.MethodGet, "/api/admin/v1/message/sms/log/:id", "sms", "sms.log.detail", false, false},
	{http.MethodPost, "/api/admin/v1/message/sms/recipient-rule", "sms", "sms.rule.create", true, false},
	{http.MethodPut, "/api/admin/v1/message/sms/recipient-rule/:id", "sms", "sms.rule.update", true, false},
	{http.MethodPatch, "/api/admin/v1/message/sms/recipient-rule/:id/status", "sms", "sms.rule.status", true, false},
	{http.MethodDelete, "/api/admin/v1/message/sms/recipient-rule/:id", "sms", "sms.rule.delete", false, false},
	{http.MethodPut, "/api/admin/v1/message/sms/rate-limit-policy/:platformId/:key", "sms", "sms.rate-limit.update", true, true},
}

func FindRule(method, route string) (RouteRule, bool) {
	for _, rule := range routeRules {
		if rule.Method == method && rule.Route == route {
			return rule, true
		}
	}
	return RouteRule{}, false
}
