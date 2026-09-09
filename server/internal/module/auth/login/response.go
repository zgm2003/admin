package auth

import (
	"time"

	authplatform "admin/server/internal/module/permission/authPlatform"
)

type Registered struct {
	UserID   int64
	Username string
	Email    string
}

type Credential struct {
	AccessToken      string
	ExpiresIn        int
	RefreshToken     string
	RefreshExpiresAt time.Time
	IsNewUser        bool
	// PasswordSetRequired reports that the account has no password yet
	// (passwordless email-code signup) so the client can guide the user to
	// set one.
	PasswordSetRequired bool
}

type RegisteredResponse struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CredentialResponse struct {
	AccessToken         string `json:"accessToken"`
	ExpiresIn           int    `json:"expiresIn"`
	IsNewUser           bool   `json:"isNewUser"`
	PasswordSetRequired bool   `json:"passwordSetRequired"`
}

type SendCodeResponse struct {
	ChallengeID        string    `json:"challengeId"`
	ExpiresAt          time.Time `json:"expiresAt"`
	ResendAfterSeconds int       `json:"resendAfterSeconds"`
}

type LoginConfigResponse struct {
	LoginTypes    []authplatform.LoginTypeOption `json:"loginTypes"`
	AllowRegister bool                           `json:"allowRegister"`
}

type CurrentUserResponse struct {
	UserID   int64   `json:"userId"`
	Username string  `json:"username"`
	Email    string  `json:"email"`
	Phone    *string `json:"phone"`
	Avatar   string  `json:"avatar"`
	// PasswordSetRequired reports that the account still has no password,
	// so profile UIs can offer the first-time set flow.
	PasswordSetRequired bool `json:"passwordSetRequired"`
}
