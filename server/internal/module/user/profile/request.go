package profile

import (
	"fmt"
	"time"

	"admin/server/internal/module/auth"
	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
)

type updateRequest struct {
	Username string  `json:"username"`
	Phone    *string `json:"phone"`
	Birthday *string `json:"birthday"`
	Gender   int16   `json:"gender"`
}

func (r updateRequest) input() (account.PersonalProfileInput, error) {
	if r.Username == "" {
		return account.PersonalProfileInput{}, apperror.InvalidRequest(fmt.Errorf("username is required"))
	}
	var birthday *time.Time
	if r.Birthday != nil {
		parsed, err := time.Parse("2006-01-02", *r.Birthday)
		if err != nil {
			return account.PersonalProfileInput{}, apperror.InvalidRequest(fmt.Errorf("birthday is invalid"))
		}
		birthday = &parsed
	}
	return account.PersonalProfileInput{Username: r.Username, Phone: r.Phone, Birthday: birthday, Gender: r.Gender}, nil
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
