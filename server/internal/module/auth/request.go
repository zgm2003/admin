package auth

type RegisterInput struct {
	Username        string
	Email           string
	Password        string
	ConfirmPassword string
}

type LoginInput struct {
	Username  string
	Password  string
	ClientIP  string
	UserAgent string
}

type RefreshInput struct {
	RefreshToken string
	ClientIP     string
	UserAgent    string
}

type RegisterRequest struct {
	Username        string `json:"username" binding:"required,max=64"`
	Email           string `json:"email" binding:"required,max=254"`
	Password        string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required,max=64"`
	Password string `json:"password" binding:"required"`
}
