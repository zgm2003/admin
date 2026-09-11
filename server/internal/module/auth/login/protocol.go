package auth

import (
	"context"
	"time"

	"admin/server/internal/module/auth/client"
	authplatform "admin/server/internal/module/permission/authPlatform"
)

const (
	PermissionSessionList   = "user:session:list"
	PermissionSessionRevoke = "user:session:revoke"
)

// SendCodeInput carries the strict send-code request after binding.
type SendCodeInput struct {
	Account     string
	LoginType   authplatform.LoginType
	Scene       string
	ChallengeID string
	Client      authclient.Client
}

// SendCodeResult returns only challenge metadata, expiry and the resend wait,
// never the code.
type SendCodeResult struct {
	ChallengeID        string
	ExpiresAt          time.Time
	ResendAfterSeconds int
}

// VerificationCodeStore is the Auth-owned Redis store for verification codes.
// Keys never contain plaintext email or phone; values carry only an HMAC
// digest and the delivery lease token. Consume is a single atomic
// compare-and-delete, so exactly one caller can redeem a code.
type VerificationCodeStore interface {
	VerificationKey(platform, scene, loginType, account string) string
	ProofDigest(challengeID, code string) string
	AcquireDelivery(context.Context, string, string, time.Duration) (bool, error)
	Put(context.Context, string, string, string, time.Duration) error
	Check(context.Context, string, string) (bool, error)
	CheckAttempt(context.Context, string, string, string) (bool, bool, error)
	Consume(context.Context, string, string) (bool, error)
	// ConsumeMany validates and deletes several codes in a single atomic Lua
	// call. Any mismatch leaves every code untouched.
	ConsumeMany(context.Context, []string, []string) (bool, error)
	DeleteIfOwned(context.Context, string, string) error
	ReleaseDelivery(context.Context, string, string) error
}
