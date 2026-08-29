package profile

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"gorm.io/gorm"
)

type Input struct {
	Username string
	Phone    *string
	Birthday *time.Time
	Gender   int16
}

type Service struct {
	repository *Repository
	now        func() time.Time
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) Current(ctx context.Context, userID int64) (Value, error) {
	if s == nil || s.repository == nil || userID <= 0 {
		return Value{}, apperror.DependencyUnavailable(fmt.Errorf("current profile requires a repository"))
	}
	value, err := s.repository.Find(ctx, userID)
	if err != nil {
		return Value{}, mapRepositoryError(err)
	}
	return value, nil
}

func (s *Service) Update(ctx context.Context, actorUserID, targetUserID int64, input Input) (Value, error) {
	if s == nil || s.repository == nil {
		return Value{}, apperror.DependencyUnavailable(fmt.Errorf("update profile requires a repository"))
	}
	if actorUserID <= 0 || targetUserID <= 0 || actorUserID != targetUserID {
		return Value{}, apperror.InvalidRequest(fmt.Errorf("profile target is invalid"))
	}
	username, err := account.NormalizeUsername(input.Username)
	if err != nil {
		return Value{}, apperror.InvalidRequest(err)
	}
	phone, err := account.NormalizePhone(input.Phone)
	if err != nil {
		return Value{}, apperror.InvalidRequest(err)
	}
	if input.Gender < 0 || input.Gender > 2 {
		return Value{}, apperror.InvalidRequest(fmt.Errorf("gender is invalid"))
	}
	updated, err := s.repository.Update(ctx, targetUserID, username, phone, input.Birthday, input.Gender, s.now().UTC().Truncate(time.Microsecond))
	if err != nil {
		return Value{}, mapRepositoryError(err)
	}
	return updated, nil
}

func mapRepositoryError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &apperror.Error{HTTPStatus: http.StatusNotFound, Code: account.CodeUserNotFound, MessageKey: i18n.KeyUserNotFound, Cause: err}
	}
	if errors.Is(err, account.ErrUsernameConflict) {
		return &apperror.Error{HTTPStatus: http.StatusConflict, Code: account.CodeUserUsernameConflict, MessageKey: i18n.KeyUserUsernameConflict, Cause: err}
	}
	if errors.Is(err, account.ErrPhoneConflict) {
		return &apperror.Error{HTTPStatus: http.StatusConflict, Code: account.CodeUserPhoneConflict, MessageKey: i18n.KeyUserPhoneConflict, Cause: err}
	}
	return apperror.DependencyUnavailable(err)
}
