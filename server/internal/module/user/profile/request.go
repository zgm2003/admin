package profile

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
)

type updateRequest struct {
	Username string  `json:"username"`
	Phone    *string `json:"phone"`
	Birthday *string `json:"birthday"`
	Gender   int16   `json:"gender"`
	Avatar   string  `json:"avatar"`
}

func (r updateRequest) input() (Input, error) {
	if r.Username == "" {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("username is required"))
	}
	var birthday *time.Time
	if r.Birthday != nil {
		parsed, err := time.Parse("2006-01-02", *r.Birthday)
		if err != nil {
			return Input{}, apperror.InvalidRequest(fmt.Errorf("birthday is invalid"))
		}
		birthday = &parsed
	}
	avatar := strings.TrimSpace(r.Avatar)
	if avatar != "" && (len(avatar) > 512 || !strings.HasPrefix(avatar, "avatar/") || strings.Contains(avatar, "..") || strings.ContainsAny(avatar, "\\\r\n\t") || strings.IndexFunc(avatar, unicode.IsControl) >= 0) {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("avatar is invalid"))
	}
	return Input{Username: r.Username, Phone: r.Phone, Birthday: birthday, Gender: r.Gender, Avatar: avatar}, nil
}

type passwordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

func (r passwordRequest) input() (auth.ChangePasswordInput, error) {
	if r.CurrentPassword == "" || r.NewPassword == "" || r.ConfirmPassword == "" {
		return auth.ChangePasswordInput{}, apperror.InvalidRequest(fmt.Errorf("password fields are required"))
	}
	return auth.ChangePasswordInput{CurrentPassword: r.CurrentPassword, NewPassword: r.NewPassword, ConfirmPassword: r.ConfirmPassword}, nil
}
