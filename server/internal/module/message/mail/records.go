package mail

import (
	mailconfig "admin/server/internal/module/message/mail/config"
	maillog "admin/server/internal/module/message/mail/log"
	logverification "admin/server/internal/module/message/mail/logVerification"
	ratelimitpolicy "admin/server/internal/module/message/mail/rateLimitPolicy"
	recipientrule "admin/server/internal/module/message/mail/recipientRule"
	mailtemplate "admin/server/internal/module/message/mail/template"
)

const (
	SceneLogin          = "login"
	SceneForget         = "forget"
	SceneBindEmail      = "bind_email"
	SceneChangePassword = "change_password"
	StatusPending       = "pending"
	StatusSent          = "sent"
	StatusFailed        = "failed"
	RuleScopeEmail      = "email"
	RuleScopeDomain     = "domain"
	RuleActionAllow     = "allow"
	RuleActionDeny      = "deny"
)

type Config = mailconfig.Model
type Template = mailtemplate.Model
type Log = maillog.Model
type Verification = logverification.Model
type RateLimitPolicy = ratelimitpolicy.Model
type RecipientRule = recipientrule.Model
