package media

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"context"
	"errors"
	"gorm.io/gorm"
	"time"
)

type TransferRetryService struct {
	repository *Repository
	now        func() time.Time
}

func NewTransferRetryService(repository *Repository) *TransferRetryService {
	return &TransferRetryService{repository: repository, now: time.Now}
}
func (s *TransferRetryService) Retry(ctx context.Context, runID, userID, platformID int64) error {
	if runID < 1 {
		return apperror.InvalidRequest(nil)
	}
	if userID < 1 || platformID < 1 {
		return apperror.Unauthorized(nil)
	}
	if s == nil || s.repository == nil {
		return apperror.DependencyUnavailable(nil)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := s.repository.RetryRunImports(ctx, runID, userID, platformID, s.now().UTC())
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	if errors.Is(err, ErrConflict) {
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	}
	return apperror.DependencyUnavailable(err)
}
