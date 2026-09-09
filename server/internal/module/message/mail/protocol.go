package mail

import (
	ratelimitpolicy "admin/server/internal/module/message/mail/rateLimitPolicy"
	recipientrule "admin/server/internal/module/message/mail/recipientRule"
	"context"
	"time"
)

const (
	SendModeBusiness  = recipientrule.SendModeBusiness
	SendModeAdminTest = recipientrule.SendModeAdminTest
)

type SendMode = recipientrule.SendMode
type RuleDecision = recipientrule.Decision
type RuleEvaluator = recipientrule.Evaluator
type LimitRequest struct {
	Key    string
	Limit  int
	Window time.Duration
}
type LimitResult struct {
	Allowed           bool
	RetryAfterSeconds int
}
type Limiter interface {
	Reserve(context.Context, ...LimitRequest) (LimitResult, error)
}
type BusinessSendInput struct {
	PlatformID                            int64
	UserID                                *int64
	ClientIP, ChallengeID, Scene, ToEmail string
	Variables                             map[string]string
	RejectActiveChallenge                 bool
}
type AdminTestInput struct {
	AdminUserID              int64
	ClientIP, ToEmail, Scene string
	Variables                map[string]string
}
type AdminTestRequest struct {
	ToEmail   string            `json:"toEmail"`
	Scene     string            `json:"scene"`
	Variables map[string]string `json:"variables"`
}
type AdminTestResult struct {
	LogID     int64  `json:"logId"`
	Status    string `json:"status"`
	RequestID string `json:"requestId"`
	MessageID string `json:"messageId"`
}
type SendResult struct {
	LogID     int64  `json:"logId"`
	Status    string `json:"status"`
	RequestID string `json:"requestId"`
	MessageID string `json:"messageId"`
}
type RateLimitPolicyInput = ratelimitpolicy.Input
type RateLimitCatalog = ratelimitpolicy.Catalog
type RateLimitPolicyStore = ratelimitpolicy.Store

// VerifyCodeReadiness reports whether a platform can send a verification code
// for a scene and, when ready, the single TTL authority for that channel.
type VerifyCodeReadiness struct {
	Ready      bool
	TTLMinutes int
}

// EmailVerifyCodePrepareInput is the preflight input for a login verification
// email. Prepare performs readiness, recipient-rule and business rate-limit
// checks and returns the per-request decision without sending anything.
type EmailVerifyCodePrepareInput struct {
	PlatformID int64
	ClientIP   string
	Scene      string
	ToEmail    string
}

// EmailVerifyCodePreparation is the per-request decision produced by Prepare:
// the channel TTL and the resend wait, both authoritative for this request.
type EmailVerifyCodePreparation struct {
	TTLMinutes         int
	ResendAfterSeconds int
}

// EmailVerifyCodeInput is the narrow authentication-channel input exposed by
// the Mail module. It carries only the fields needed to send a login
// verification email; it never exposes the repository, GORM or Redis.
type EmailVerifyCodeInput struct {
	PlatformID  int64
	UserID      *int64
	ClientIP    string
	ChallengeID string
	Scene       string
	ToEmail     string
	Code        string
	ExpiresAt   time.Time
	Preparation EmailVerifyCodePreparation
}

type EmailVerifyCodeResult struct {
	LogID       int64
	ChallengeID string
	ExpiresAt   time.Time
}

// VerifyCodeSender is the narrow interface Auth depends on for email
// verification codes. It exposes no repository, GORM, Redis client or model.
type VerifyCodeSender interface {
	VerifyCodeReady(context.Context, string) (VerifyCodeReadiness, error)
	PrepareEmailVerifyCode(context.Context, EmailVerifyCodePrepareInput) (EmailVerifyCodePreparation, error)
	SendPreparedEmailVerifyCode(context.Context, EmailVerifyCodeInput) (EmailVerifyCodeResult, error)
}
