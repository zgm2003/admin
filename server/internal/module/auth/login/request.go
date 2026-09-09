package auth

import (
	"admin/server/internal/module/auth/client"
	authplatform "admin/server/internal/module/permission/authPlatform"
)

type RegisterInput struct {
	Username        string
	Email           string
	Password        string
	ConfirmPassword string
	Client          authclient.Client
}

type LoginInput struct {
	LoginType    authplatform.LoginType
	LoginAccount string
	Password     string
	Code         string
	Client       authclient.Client
}

type RefreshInput struct {
	RefreshToken string
	Client       authclient.Client
}

type RegisterRequest struct {
	Username        string `json:"username" binding:"required,max=64"`
	Email           string `json:"email" binding:"required,max=254"`
	Password        string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

type LoginRequest struct {
	LoginType    *string `json:"loginType" binding:"required"`
	LoginAccount *string `json:"loginAccount" binding:"required"`
	Password     *string `json:"password"`
	Code         *string `json:"code"`
}

type SendCodeRequest struct {
	Account     *string `json:"account" binding:"required"`
	LoginType   *string `json:"loginType" binding:"required"`
	Scene       *string `json:"scene" binding:"required"`
	ChallengeID *string `json:"challengeId" binding:"omitempty,max=128"`
}

// ForgotPasswordRequest is the request body of POST /auth/password/forgot.
type ForgotPasswordRequest struct {
	Email *string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest is the request body of POST /auth/password/reset.
type ResetPasswordRequest struct {
	Email           *string `json:"email" binding:"required,email"`
	Code            *string `json:"code" binding:"required,len=6,numeric"`
	NewPassword     *string `json:"newPassword" binding:"required"`
	ConfirmPassword *string `json:"confirmPassword" binding:"required"`
}
