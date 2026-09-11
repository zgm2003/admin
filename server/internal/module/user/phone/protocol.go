package phone

import (
	"context"
	"errors"
	"time"

	messagesms "admin/server/internal/module/message/sms"
)

const (
	PermissionUpdate = "user:phone:update"
	TargetCurrent    = "current"
	TargetNext       = "next"
	ActionBind       = "bind"
	ActionChange     = "change"
)

var (
	ErrCurrentPhoneChanged = errors.New("current phone changed")
	ErrPhoneConflict       = errors.New("phone is already bound")
)

type Actor struct {
	UserID     int64
	SessionID  int64
	PlatformID int64
	Platform   string
	ClientIP   string
}

type Current struct {
	UserID    int64
	Phone     *string
	IsEnabled bool
	Deleted   bool
}

type SendCodeInput struct {
	Target      string
	Phone       *string
	ChallengeID string
}

type SendCodeResult struct {
	ChallengeID        string
	ExpiresAt          time.Time
	ResendAfterSeconds int
}

type BindOrChangeInput struct {
	CurrentChallengeID string
	CurrentCode        string
	NextPhone          string
	NextChallengeID    string
	NextCode           string
}

type PhoneResult struct {
	Phone string
}

type ChangeInput struct {
	UserID     int64
	PlatformID int64
	Action     string
	OldPhone   string
	NewPhone   string
	OldHint    string
	OldHMAC    string
	NewHint    string
	NewHMAC    string
	Now        time.Time
}

type accountStore interface {
	Current(context.Context, int64) (Current, error)
	PhoneInUse(context.Context, int64, string) (bool, error)
	Change(context.Context, ChangeInput) error
}

// VerificationCodeStore is the narrow Auth-owned verification contract used
// by the authenticated phone workflow. Implementations must keep keys and
// values free of plaintext phone numbers and codes.
type VerificationCodeStore interface {
	VerificationKey(platform, scene, loginType, account string) string
	ProofDigest(challengeID, code string) string
	AcquireDelivery(context.Context, string, string, time.Duration) (bool, error)
	Put(context.Context, string, string, string, time.Duration) error
	CheckAttempt(context.Context, string, string, string) (bool, bool, error)
	ConsumeMany(context.Context, []string, []string) (bool, error)
	DeleteIfOwned(context.Context, string, string) error
	ReleaseDelivery(context.Context, string, string) error
}

type authorityCoordinator interface {
	Mutate(context.Context, Current, func(context.Context) error) error
}

type phoneCodeSender interface {
	messagesms.VerifyCodeSender
}

type handlerService interface {
	SendCode(context.Context, Actor, SendCodeInput) (SendCodeResult, error)
	BindOrChange(context.Context, Actor, BindOrChangeInput) (PhoneResult, error)
}
