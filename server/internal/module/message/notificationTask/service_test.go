package notificationtask

import (
	"admin/server/internal/module/message/notification"
	"admin/server/internal/shared/apperror"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestDraftSubmitCancelCopyStateTransitions(t *testing.T) {
	repo := &taskRepositoryStub{}
	service := NewService(repo)
	input := DraftInput{PlatformID: 1, Title: "title", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudiencePlatform}
	created, err := service.Create(context.Background(), 7, input)
	if err != nil || created.Status != StatusDraft {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	submitted, err := service.Submit(context.Background(), created.ID, time.Now().UTC())
	if err != nil || submitted.Status != StatusQueued {
		t.Fatalf("submitted=%+v err=%v", submitted, err)
	}
	if _, err := service.Update(context.Background(), created.ID, input); err == nil {
		t.Fatal("submitted task updated")
	} else {
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusConflict || !errors.Is(err, ErrNotDraft) {
			t.Fatalf("update error=%v", err)
		}
	}
	canceled, err := service.Cancel(context.Background(), created.ID, time.Now().UTC())
	if err != nil || canceled.Status != StatusCanceled {
		t.Fatalf("canceled=%+v err=%v", canceled, err)
	}
	copied, err := service.Copy(context.Background(), created.ID, 8, time.Now().UTC())
	if err != nil || copied.Status != StatusDraft || copied.ID == created.ID {
		t.Fatalf("copied=%+v err=%v", copied, err)
	}
}

func TestServiceMapsInvalidDraftToBadRequest(t *testing.T) {
	service := NewService(&taskRepositoryStub{})
	_, err := service.Create(context.Background(), 1, DraftInput{})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("create error=%v", err)
	}
}

type taskRepositoryStub struct {
	task Task
	next int64
}

func (s *taskRepositoryStub) Create(_ context.Context, creator int64, input DraftInput) (Task, error) {
	s.next++
	s.task = taskFromDraft(s.next, creator, input, time.Now().UTC())
	return s.task, nil
}
func (s *taskRepositoryStub) Update(_ context.Context, id int64, input DraftInput) (Task, error) {
	if s.task.Status != StatusDraft {
		return Task{}, ErrNotDraft
	}
	updated := taskFromDraft(id, s.task.CreatedBy, input, s.task.CreatedAt)
	updated.UpdatedAt = time.Now().UTC()
	s.task = updated
	return updated, nil
}
func (s *taskRepositoryStub) Submit(_ context.Context, id int64, now time.Time) (Task, error) {
	if s.task.Status != StatusDraft {
		return Task{}, ErrNotDraft
	}
	s.task.Status = StatusQueued
	s.task.SubmittedAt = &now
	maximum := int64(100)
	s.task.AudienceMaxUserID = &maximum
	return s.task, nil
}
func (s *taskRepositoryStub) Cancel(_ context.Context, id int64, now time.Time) (Task, error) {
	if s.task.Status != StatusQueued && s.task.Status != StatusScheduled && s.task.Status != StatusProcessing {
		return Task{}, ErrInvalidTransition
	}
	s.task.Status = StatusCanceled
	s.task.CanceledAt = &now
	return s.task, nil
}
func (s *taskRepositoryStub) Copy(_ context.Context, id, creator int64, now time.Time) (Task, error) {
	input := DraftInput{PlatformID: s.task.PlatformID, Title: s.task.Title, ContentHTML: s.task.ContentHTML, Variant: s.task.Variant, Priority: s.task.Priority, LinkType: s.task.LinkType, Link: s.task.Link, AudienceType: s.task.AudienceType, TargetIDs: s.task.TargetIDs, ScheduledAt: s.task.ScheduledAt}
	return s.Create(context.Background(), creator, input)
}
