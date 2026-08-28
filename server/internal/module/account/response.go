package account

import (
	"time"

	"admin/server/internal/module/user"
)

type profileResponse struct {
	UserID   int64   `json:"userId"`
	Username string  `json:"username"`
	Email    string  `json:"email"`
	Phone    *string `json:"phone"`
	Birthday *string `json:"birthday"`
	Gender   int16   `json:"gender"`
}

type updatedProfileResponse struct {
	UserID    int64   `json:"userId"`
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	Phone     *string `json:"phone"`
	Birthday  *string `json:"birthday"`
	Gender    int16   `json:"gender"`
	UpdatedAt string  `json:"updatedAt"`
}

func newProfileResponse(value user.PersonalProfile) profileResponse {
	var birthday *string
	if value.Birthday != nil {
		formatted := value.Birthday.Format("2006-01-02")
		birthday = &formatted
	}
	return profileResponse{UserID: value.ID, Username: value.Username, Email: value.Email, Phone: value.Phone, Birthday: birthday, Gender: value.Gender}
}

func newUpdatedProfileResponse(value user.PersonalProfile) updatedProfileResponse {
	result := newProfileResponse(value)
	return updatedProfileResponse{UserID: result.UserID, Username: result.Username, Email: result.Email, Phone: result.Phone, Birthday: result.Birthday, Gender: result.Gender, UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}
