package operationlog

import (
	"context"

	"admin/server/internal/shared/option"
)

// Action codes stay in audit facts. Labels are derived for the current request,
// never written to PostgreSQL or kept in a localized cache.
func actionLabel(ctx context.Context, action string) string {
	labels, found := actionLabels[action]
	if !found {
		return action
	}
	return option.New(ctx, action, labels.zh, labels.en).Label
}

var actionLabels = map[string]struct{ zh, en string }{
	"authPlatform.create":        {"新增认证平台", "Create authentication platform"},
	"authPlatform.update":        {"编辑认证平台", "Edit authentication platform"},
	"authPlatform.status":        {"修改认证平台状态", "Change authentication platform status"},
	"authPlatform.delete":        {"删除认证平台", "Delete authentication platform"},
	"menu.create":                {"新增菜单", "Create menu"},
	"menu.update":                {"编辑菜单", "Edit menu"},
	"menu.status":                {"修改菜单状态", "Change menu status"},
	"menu.delete":                {"删除菜单", "Delete menu"},
	"role.create":                {"新增角色", "Create role"},
	"role.update":                {"编辑角色", "Edit role"},
	"role.status":                {"修改角色状态", "Change role status"},
	"role.default":               {"设为默认角色", "Set default role"},
	"role.delete":                {"删除角色", "Delete role"},
	"role.permissions.update":    {"更新角色权限", "Update role permissions"},
	"user.update":                {"编辑用户", "Edit user"},
	"user.status":                {"修改用户状态", "Change user status"},
	"user.delete":                {"删除用户", "Delete user"},
	"user.roles.update":          {"更新用户角色", "Update user roles"},
	"session.revoke":             {"踢出会话", "Revoke session"},
	"session.revoke.bulk":        {"批量踢出会话", "Revoke sessions in bulk"},
	"user.profile.update":        {"修改个人资料", "Update personal profile"},
	"user.password.update":       {"修改密码", "Change password"},
	"user.phone.update":          {"修改手机号", "Change phone number"},
	"user.email.update":          {"修改邮箱", "Change email address"},
	"storage.cos-config.create":  {"新增 COS 配置", "Create COS configuration"},
	"storage.cos-config.update":  {"编辑 COS 配置", "Edit COS configuration"},
	"storage.cos-config.status":  {"修改 COS 配置状态", "Change COS configuration status"},
	"storage.cos-config.test":    {"测试 COS 配置", "Test COS configuration"},
	"storage.cos-config.delete":  {"删除 COS 配置", "Delete COS configuration"},
	"storage.upload-rule.create": {"新增上传规则", "Create upload rule"},
	"storage.upload-rule.update": {"编辑上传规则", "Edit upload rule"},
	"storage.upload-rule.status": {"修改上传规则状态", "Change upload rule status"},
	"storage.upload-rule.delete": {"删除上传规则", "Delete upload rule"},
	"mail.config.update":         {"编辑邮件配置", "Edit mail configuration"},
	"mail.config.delete":         {"删除邮件配置", "Delete mail configuration"},
	"mail.test":                  {"发送测试邮件", "Send test mail"},
	"mail.template.update":       {"编辑邮件模板", "Edit mail template"},
	"mail.template.status":       {"修改邮件模板状态", "Change mail template status"},
	"mail.log.detail":            {"查看发送日志详情", "View delivery log details"},
	"mail.rule.create":           {"新增收件规则", "Create recipient rule"},
	"mail.rule.update":           {"编辑收件规则", "Edit recipient rule"},
	"mail.rule.status":           {"修改收件规则状态", "Change recipient rule status"},
	"mail.rule.delete":           {"删除收件规则", "Delete recipient rule"},
	"mail.rate-limit.update":     {"修改邮件限流策略", "Change mail rate limit policy"},
	"sms.config.update":          {"编辑短信配置", "Edit SMS configuration"},
	"sms.config.delete":          {"删除短信配置", "Delete SMS configuration"},
	"sms.test":                   {"发送测试短信", "Send test SMS"},
	"sms.template.update":        {"编辑短信模板", "Edit SMS template"},
	"sms.template.status":        {"修改短信模板状态", "Change SMS template status"},
	"sms.log.detail":             {"查看短信发送日志详情", "View SMS delivery log details"},
	"sms.rule.create":            {"新增短信收件规则", "Create SMS recipient rule"},
	"sms.rule.update":            {"编辑短信收件规则", "Edit SMS recipient rule"},
	"sms.rule.status":            {"修改短信收件规则状态", "Change SMS recipient rule status"},
	"sms.rule.delete":            {"删除短信收件规则", "Delete SMS recipient rule"},
	"sms.rate-limit.update":      {"修改短信限流策略", "Change SMS rate limit policy"},
}
