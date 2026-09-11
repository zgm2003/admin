package email

import (
	"context"
	"errors"
	"time"

	messagemail "admin/server/internal/module/message/mail"
)

const (
	PermissionUpdate = "user:email:update"
	TargetCurrent    = "current"
	TargetNext       = "next"
	ActionBind       = "bind"
	ActionChange     = "change"
)

var (
	ErrCurrentEmailChanged = errors.New("current email changed")
	ErrEmailConflict       = errors.New("email is already bound")
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
	Email     *string
	IsEnabled bool
	Deleted   bool
}

type SendCodeInput struct {
	Target      string
	Email       *string
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
	NextEmail          string
	NextChallengeID    string
	NextCode           string
}

type EmailResult struct{ Email string }

type ChangeInput struct {
	UserID     int64
	PlatformID int64
	Action     string
	OldEmail   string
	NewEmail   string
	OldHint    string
	OldHMAC    string
	NewHint    string
	NewHMAC    string
	Now        time.Time
}

type accountStore interface {
	Current(context.Context, int64) (Current, error)
	EmailInUse(context.Context, int64, string) (bool, error)
	Change(context.Context, ChangeInput) error
}

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

type emailCodeSender interface{ messagemail.VerifyCodeSender }

type authorityCoordinator interface {
	Mutate(context.Context, Current, func(context.Context) error) error
}

type handlerService interface {
	SendCode(context.Context, Actor, SendCodeInput) (SendCodeResult, error)
	BindOrChange(context.Context, Actor, BindOrChangeInput) (EmailResult, error)
}
