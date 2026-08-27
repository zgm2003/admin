package auth

import "admin/server/internal/module/authclient"

type RegisterInput struct {
	Username        string
	Email           string
	Password        string
	ConfirmPassword string
	Client          authclient.Client
}

type LoginInput struct {
	Email    string
	Password string
	Client   authclient.Client
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
	Email    string `json:"email" binding:"required,max=254"`
	Password string `json:"password" binding:"required"`
}
