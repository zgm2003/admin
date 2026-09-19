package notificationtask

import (
	"admin/server/internal/module/message/notification"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type taskRepository interface {
	Create(context.Context, int64, DraftInput) (Task, error)
	Update(context.Context, int64, DraftInput) (Task, error)
	Submit(context.Context, int64, time.Time) (Task, error)
	Cancel(context.Context, int64, time.Time) (Task, error)
	Copy(context.Context, int64, int64, time.Time) (Task, error)
}
type taskAdminRepository interface {
	Find(context.Context, int64) (Task, error)
	List(context.Context, Status, int, int) ([]Task, int64, error)
	Delete(context.Context, int64, time.Time) error
	Options(context.Context, string, string, int64, int) ([]Option, error)
}
type Service struct{ repository taskRepository }

func NewService(repository taskRepository) *Service { return &Service{repository: repository} }
func (s *Service) Create(ctx context.Context, creator int64, input DraftInput) (Task, error) {
	if creator <= 0 {
		return Task{}, apperror.InvalidRequest(errors.New("creator is invalid"))
	}
	normalized, err := NormalizeDraft(input)
	if err != nil {
		return Task{}, apperror.InvalidRequest(err)
	}
	result, err := s.repository.Create(ctx, creator, normalized)
	return result, mapTaskError(err)
}
func (s *Service) Update(ctx context.Context, id int64, input DraftInput) (Task, error) {
	if id <= 0 {
		return Task{}, apperror.InvalidRequest(errors.New("task id is invalid"))
	}
	normalized, err := NormalizeDraft(input)
	if err != nil {
		return Task{}, apperror.InvalidRequest(err)
	}
	result, err := s.repository.Update(ctx, id, normalized)
	return result, mapTaskError(err)
}
func (s *Service) Submit(ctx context.Context, id int64, now time.Time) (Task, error) {
	if id <= 0 || now.IsZero() {
		return Task{}, apperror.InvalidRequest(errors.New("submit input is invalid"))
	}
	result, err := s.repository.Submit(ctx, id, now.UTC())
	return result, mapTaskError(err)
}
func (s *Service) Cancel(ctx context.Context, id int64, now time.Time) (Task, error) {
	if id <= 0 || now.IsZero() {
		return Task{}, apperror.InvalidRequest(errors.New("cancel input is invalid"))
	}
	result, err := s.repository.Cancel(ctx, id, now.UTC())
	return result, mapTaskError(err)
}
func (s *Service) Copy(ctx context.Context, id, creator int64, now time.Time) (Task, error) {
	if id <= 0 || creator <= 0 || now.IsZero() {
		return Task{}, apperror.InvalidRequest(errors.New("copy input is invalid"))
	}
	result, err := s.repository.Copy(ctx, id, creator, now.UTC())
	return result, mapTaskError(err)
}
func taskFromDraft(id, creator int64, input DraftInput, now time.Time) Task {
	summary, _ := notification.SummaryFromHTML(input.ContentHTML)
	return Task{ID: id, PlatformID: input.PlatformID, Title: input.Title, ContentHTML: input.ContentHTML, Summary: summary, Variant: input.Variant, Priority: input.Priority, LinkType: input.LinkType, Link: input.Link, AudienceType: input.AudienceType, ScheduledAt: input.ScheduledAt, Status: StatusDraft, CreatedBy: creator, CreatedAt: now, UpdatedAt: now, TargetIDs: append([]int64(nil), input.TargetIDs...)}
}
func (s *Service) Detail(ctx context.Context, id int64) (Task, error) {
	repository, ok := s.repository.(taskAdminRepository)
	if id <= 0 || !ok {
		return Task{}, apperror.InvalidRequest(errors.New("task detail is unavailable"))
	}
	result, err := repository.Find(ctx, id)
	return result, mapTaskError(err)
}
func (s *Service) List(ctx context.Context, status Status, page, size int) ([]Task, int64, error) {
	repository, ok := s.repository.(taskAdminRepository)
	if (status != "" && !validStatus(status)) || page < 1 || size < 1 || size > 100 || !ok {
		return nil, 0, apperror.InvalidRequest(errors.New("task page is invalid"))
	}
	rows, total, err := repository.List(ctx, status, page, size)
	return rows, total, mapTaskError(err)
}

func validStatus(status Status) bool {
	switch status {
	case StatusDraft, StatusScheduled, StatusQueued, StatusProcessing, StatusCompleted, StatusFailed, StatusCanceled:
		return true
	default:
		return false
	}
}
func (s *Service) Delete(ctx context.Context, id int64, now time.Time) error {
	repository, ok := s.repository.(taskAdminRepository)
	if id <= 0 || !ok {
		return apperror.InvalidRequest(errors.New("task delete is unavailable"))
	}
	return mapTaskError(repository.Delete(ctx, id, now))
}
func (s *Service) Options(ctx context.Context, kind, keyword string, after int64, limit int) ([]Option, *int64, error) {
	repository, ok := s.repository.(taskAdminRepository)
	if (kind != "platform" && kind != "user" && kind != "role") || after < 0 || limit < 1 || limit > 50 || !ok {
		return nil, nil, apperror.InvalidRequest(errors.New("option query is invalid"))
	}
	rows, err := repository.Options(ctx, kind, keyword, after, limit+1)
	if err != nil {
		return nil, nil, mapTaskError(err)
	}
	var next *int64
	if len(rows) > limit {
		value := rows[limit-1].ID
		next = &value
		rows = rows[:limit]
	}
	return rows, next, nil
}

func mapTaskError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperror.NotFound(err)
	case errors.Is(err, ErrInvalidFacts):
		return apperror.InvalidRequest(err)
	case errors.Is(err, ErrNotDraft), errors.Is(err, ErrInvalidTransition):
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	default:
		return apperror.DependencyUnavailable(err)
	}
}
