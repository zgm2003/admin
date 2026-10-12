package profile

import (
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
	"unicode"

	"admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
)

type updateRequest struct {
	Username         string  `json:"username"`
	Birthday         *string `json:"birthday"`
	Gender           int16   `json:"gender"`
	Avatar           string  `json:"avatar"`
	AvatarFileID     *int64  `json:"avatarFileId"`
	AvatarRequestKey string  `json:"avatarRequestKey"`
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
	if r.AvatarFileID != nil {
		if *r.AvatarFileID <= 0 || avatar != "" {
			return Input{}, apperror.InvalidRequest(fmt.Errorf("avatar selection is invalid"))
		}
		if _, err := uuid.Parse(r.AvatarRequestKey); err != nil || len(r.AvatarRequestKey) != 36 {
			return Input{}, apperror.InvalidRequest(fmt.Errorf("avatar request key is invalid"))
		}
	} else if r.AvatarRequestKey != "" {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("avatar file is required"))
	}
	if avatar != "" && (len(avatar) > 512 || !strings.HasPrefix(avatar, "avatar/") || strings.Contains(avatar, "..") || strings.ContainsAny(avatar, "\\\r\n\t") || strings.IndexFunc(avatar, unicode.IsControl) >= 0) {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("avatar is invalid"))
	}
	return Input{Username: r.Username, Birthday: birthday, Gender: r.Gender, Avatar: avatar, AvatarFileID: r.AvatarFileID, AvatarRequestKey: r.AvatarRequestKey}, nil
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

type setPasswordRequest struct {
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

func (r setPasswordRequest) input() (auth.SetPasswordInput, error) {
	if r.NewPassword == "" || r.ConfirmPassword == "" {
		return auth.SetPasswordInput{}, apperror.InvalidRequest(fmt.Errorf("password fields are required"))
	}
	return auth.SetPasswordInput{NewPassword: r.NewPassword, ConfirmPassword: r.ConfirmPassword}, nil
}
