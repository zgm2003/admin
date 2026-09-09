package log

import (
	"context"
	"errors"
	"fmt"
	"time"

	logverification "admin/server/internal/module/message/mail/logVerification"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"gorm.io/gorm"
)

type Detail struct {
	Log                   Model
	VerificationCode      string
	VerificationExpiresAt *time.Time
}

type Service struct {
	repository   *Repository
	verification *logverification.Repository
	keys         *secretkey.KeyRing
}

func NewService(repository *Repository, verification *logverification.Repository, keys *secretkey.KeyRing) *Service {
	return &Service{repository: repository, verification: verification, keys: keys}
}

func (s *Service) List(ctx context.Context, platformID int64, page, size int) ([]Model, int64, error) {
	values, total, err := s.repository.List(ctx, platformID, page, size)
	if err != nil {
		return nil, 0, wrapRepository(err)
	}
	return values, total, nil
}

func (s *Service) Get(ctx context.Context, platformID, id int64) (Detail, error) {
	value, err := s.repository.Find(ctx, platformID, id)
	if err != nil {
		return Detail{}, wrapRepository(err)
	}
	result := Detail{Log: value}
	verification, err := s.verification.FindByLog(ctx, platformID, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, nil
	}
	if err != nil {
		return Detail{}, wrapRepository(err)
	}
	if s.keys == nil {
		return Detail{}, apperror.DependencyUnavailable(fmt.Errorf("mail encryption key unavailable"))
	}
	code, err := secretkey.DecryptMailValue(s.keys.MailEncryptionKey(), verification.CodeCiphertext)
	if err != nil {
		return Detail{}, apperror.DependencyUnavailable(err)
	}
	result.VerificationCode = code
	result.VerificationExpiresAt = &verification.ExpiresAt
	return result, nil
}

func (s *Service) Delete(ctx context.Context, platformID, id int64) error {
	return wrapRepository(s.repository.Delete(ctx, platformID, id))
}

func (s *Service) DeleteMany(ctx context.Context, platformID int64, ids []int64) error {
	return wrapRepository(s.repository.DeleteMany(ctx, platformID, ids))
}

func wrapRepository(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	return apperror.DependencyUnavailable(fmt.Errorf("mail log repository: %w", err))
}
