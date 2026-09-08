package mail

import (
	"admin/server/internal/shared/yesno"
	"context"
	"time"
)

type SendMode string

const (
	SendModeBusiness  SendMode = "business"
	SendModeAdminTest SendMode = "admin_test"
)

type RuleDecision struct {
	Allowed bool
	RuleID  int64
	Reason  string
}
type RuleEvaluator interface {
	Evaluate(context.Context, int64, string, SendMode) (RuleDecision, error)
}
type LimitRequest struct {
	Key        string
	Limit      int
	Window     time.Duration
	LegacyKeys []string
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
type ConfigInput struct {
	SecretID   string      `json:"secretId"`
	SecretKey  string      `json:"secretKey"`
	Region     string      `json:"region"`
	FromEmail  string      `json:"fromEmail"`
	FromName   string      `json:"fromName"`
	ReplyTo    string      `json:"replyTo"`
	Endpoint   string      `json:"endpoint"`
	TTLMinutes int         `json:"ttlMinutes"`
	IsEnabled  yesno.Value `json:"isEnabled"`
}
type TemplateUpdateInput struct {
	Scene             string            `json:"scene"`
	Name              string            `json:"name"`
	Subject           string            `json:"subject"`
	TencentTemplateID int               `json:"tencentTemplateId"`
	Variables         map[string]string `json:"variables"`
	ExampleVariables  map[string]string `json:"exampleVariables"`
}
type RuleInput struct {
	Scope     string      `json:"scope"`
	Pattern   string      `json:"pattern"`
	Action    string      `json:"action"`
	Name      string      `json:"name"`
	Remark    string      `json:"remark"`
	IsEnabled yesno.Value `json:"isEnabled"`
}

type RateLimitPolicyInput struct {
	Key           string
	Limit         int
	WindowSeconds int
}

type RateLimitCatalog struct {
	Version  int64
	Policies []RateLimitPolicy
}

type RateLimitPolicyStore interface {
	Load(context.Context) (RateLimitCatalog, error)
	Update(context.Context, RateLimitPolicyInput) (RateLimitCatalog, error)
}

type RateLimitSnapshot struct {
	SchemaVersion int                             `json:"schemaVersion"`
	State         string                          `json:"state"`
	Version       int64                           `json:"version"`
	Policies      map[string]rateLimitPolicyValue `json:"policies,omitempty"`
	MutationToken *string                         `json:"mutationToken"`
}

type rateLimitPolicyValue struct {
	Limit         int       `json:"limit"`
	WindowSeconds int       `json:"windowSeconds"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type RateLimitPolicyUpdateRequest struct {
	Limit         int `json:"limit"`
	WindowSeconds int `json:"windowSeconds"`
}

type RateLimitPolicyListResponse struct {
	Version  int64             `json:"version"`
	Policies []RateLimitPolicy `json:"policies"`
}

type RateLimitPolicyResponse struct {
	Version int64           `json:"version"`
	Policy  RateLimitPolicy `json:"policy"`
}

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
	VerifyCodeReady(context.Context, int64, string) (VerifyCodeReadiness, error)
	PrepareEmailVerifyCode(context.Context, EmailVerifyCodePrepareInput) (EmailVerifyCodePreparation, error)
	SendPreparedEmailVerifyCode(context.Context, EmailVerifyCodeInput) (EmailVerifyCodeResult, error)
}
