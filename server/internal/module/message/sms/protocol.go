package sms

import (
	"context"
	"time"
)

const (
	PermissionView = "message:sms:view"
	PermissionList = "message:sms:list"
	PermissionTest = "message:sms:test"
)

// PageInitResult is the admin aggregate page bootstrap: the four fixed scenes.
type PageInitResult struct {
	Scenes []SceneOption `json:"scenes"`
}

type SceneOption struct {
	Scene         string   `json:"scene"`
	Name          string   `json:"name"`
	ParameterKeys []string `json:"parameterKeys"`
}

// VerifyCodeReadiness reports whether one scene is able to send right now.
type VerifyCodeReadiness struct {
	Ready      bool
	TTLMinutes int
}

type PhoneVerifyCodePrepareInput struct {
	PlatformID int64
	Scene      string
	ToPhone    string
}

type PhoneVerifyCodePreparation struct {
	TTLMinutes         int
	ResendAfterSeconds int
}

type PhoneVerifyCodeInput struct {
	PlatformID  int64
	UserID      *int64
	ChallengeID string
	Scene       string
	ToPhone     string
	Code        string
	ExpiresAt   time.Time
	Preparation PhoneVerifyCodePreparation
}

type PhoneVerifyCodeResult struct {
	LogID     int64  `json:"logId"`
	RequestID string `json:"requestId"`
	SerialNo  string `json:"serialNo"`
}

// VerifyCodeSender is the narrow interface exposed to the Auth module. Auth never
// sees SMS tables, providers, credentials or its own stores.
type VerifyCodeSender interface {
	VerifyCodeReady(context.Context, string) (VerifyCodeReadiness, error)
	PreparePhoneVerifyCode(context.Context, PhoneVerifyCodePrepareInput) (PhoneVerifyCodePreparation, error)
	SendPreparedPhoneVerifyCode(context.Context, PhoneVerifyCodeInput) (PhoneVerifyCodeResult, error)
}

// LimitRequest describes one sending window. The key already contains the
// platform, the phone HMAC and the policy key, never a scene or plaintext phone.
type LimitRequest struct {
	Key    string
	Limit  int
	Window time.Duration
}

// AdminTestInput and AdminTestResult describe the administrator test send. The
// administrator test is a mode, never a fifth scene.
type AdminTestInput struct {
	PlatformID int64
	Scene      string
	ToPhone    string
}

type AdminTestResult struct {
	LogID     int64  `json:"logId"`
	Status    string `json:"status"`
	RequestID string `json:"requestId"`
	SerialNo  string `json:"serialNo"`
}

type LimitResult struct {
	Allowed           bool
	RetryAfterSeconds int
}

type Limiter interface {
	Reserve(context.Context, ...LimitRequest) (LimitResult, error)
}

// RuntimeCoordinator runs one write in a tokenized mutation so the runtime
// snapshot is invalidated only after the database change committed.
type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context) error) error
}
