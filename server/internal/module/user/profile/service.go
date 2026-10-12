package profile

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"admin/server/internal/module/auth/login"
	storagefile "admin/server/internal/module/storage/file"
	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"gorm.io/gorm"
)

type Input struct {
	Username         string
	Birthday         *time.Time
	Gender           int16
	Avatar           string
	PlatformID       int64
	AvatarFileID     *int64
	AvatarRequestKey string
	Identity         auth.Identity
}

type Service struct {
	repository  *Repository
	now         func() time.Time
	avatars     avatarCopier
	permissions avatarPermissionChecker
}

type avatarCopier interface {
	PrepareAvatar(context.Context, auth.Identity, int64, string) (storagefile.AvatarCopy, error)
}

func (s *Service) SetAvatarCopier(copier avatarCopier) { s.avatars = copier }

type avatarPermissionChecker interface {
	Allowed(context.Context, auth.Identity, string) (bool, error)
}

func (s *Service) SetAvatarPermissions(checker avatarPermissionChecker) { s.permissions = checker }

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
	if !validGender(input.Gender) {
		return Value{}, apperror.InvalidRequest(fmt.Errorf("gender is invalid"))
	}
	if input.AvatarFileID != nil {
		if s.avatars == nil || s.permissions == nil || input.PlatformID <= 0 {
			return Value{}, apperror.DependencyUnavailable(fmt.Errorf("avatar copy dependencies unavailable"))
		}
		identity := input.Identity
		if identity.UserID != actorUserID || identity.PlatformID != input.PlatformID {
			return Value{}, apperror.InvalidRequest(fmt.Errorf("avatar identity mismatch"))
		}
		allowed, err := s.permissions.Allowed(ctx, identity, "storage:file:detail")
		if err != nil {
			return Value{}, err
		}
		if !allowed {
			return Value{}, apperror.Forbidden(fmt.Errorf("file detail permission required"))
		}
		copied, err := s.avatars.PrepareAvatar(ctx, identity, *input.AvatarFileID, input.AvatarRequestKey)
		if err != nil {
			return Value{}, err
		}
		updated, err := s.repository.UpdateWithAvatar(ctx, identity, username, input.Birthday, input.Gender, copied, input.AvatarRequestKey, s.now().UTC().Truncate(time.Microsecond))
		if err != nil {
			return Value{}, mapRepositoryError(err)
		}
		return updated, nil
	}
	updated, err := s.repository.UpdatePreservingAvatar(ctx, targetUserID, username, input.Birthday, input.Gender, input.Avatar, s.now().UTC().Truncate(time.Microsecond), input.Identity)
	if err != nil {
		return Value{}, mapRepositoryError(err)
	}
	return updated, nil
}

func mapRepositoryError(err error) error {
	var public *apperror.Error
	if errors.As(err, &public) {
		return public
	}
	if errors.Is(err, storagefile.ErrConflict) {
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	}
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
