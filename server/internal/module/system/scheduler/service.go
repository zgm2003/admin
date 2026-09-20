package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"gorm.io/gorm"
)

type Service struct {
	repository *Repository
	catalog    TaskCatalog
	now        func() time.Time
}

func NewService(repository *Repository, catalog TaskCatalog) *Service {
	return &Service{repository: repository, catalog: catalog, now: time.Now}
}
func (s *Service) Options() []TaskOption { return s.catalog.Options() }
func (s *Service) ListSchedules(ctx context.Context, q ScheduleQuery) ([]Schedule, error) {
	if q.Limit < 1 || q.Limit > 100 || q.AfterID < 0 {
		return nil, apperror.InvalidRequest(errors.New("scheduler schedule query is invalid"))
	}
	rows, err := s.repository.ListSchedules(ctx, q)
	return rows, mapSchedulerError(err)
}
func (s *Service) GetSchedule(ctx context.Context, id int64) (Schedule, error) {
	if id <= 0 {
		return Schedule{}, apperror.InvalidRequest(errors.New("scheduler id is invalid"))
	}
	row, err := s.repository.GetSchedule(ctx, id)
	return row, mapSchedulerError(err)
}
func (s *Service) CreateSchedule(ctx context.Context, input ScheduleInput, actor int64) (Schedule, error) {
	if actor <= 0 {
		return Schedule{}, apperror.InvalidRequest(errors.New("scheduler actor is invalid"))
	}
	definition, ok := s.catalog.Lookup(input.TaskType)
	if !ok || !definition.AdminCreatable {
		return Schedule{}, apperror.InvalidRequest(ErrUnknownTaskType)
	}
	if _, err := ParseCron(input.CronExpression); err != nil {
		return Schedule{}, apperror.InvalidRequest(err)
	}
	if _, err := LoadTimezone(input.Timezone); err != nil {
		return Schedule{}, apperror.InvalidRequest(err)
	}
	if err := s.catalog.ValidatePayload(input.TaskType, input.Params); err != nil {
		return Schedule{}, apperror.InvalidRequest(err)
	}
	now := s.now().UTC()
	row, err := s.repository.CreateSchedule(ctx, input, &actor, now)
	return row, mapSchedulerError(err)
}
func (s *Service) UpdateSchedule(ctx context.Context, id int64, input ScheduleInput, actor int64) (Schedule, error) {
	if id <= 0 || actor <= 0 {
		return Schedule{}, apperror.InvalidRequest(errors.New("scheduler update input is invalid"))
	}
	current, err := s.GetSchedule(ctx, id)
	if err != nil {
		return Schedule{}, err
	}
	input.TaskType = current.TaskType
	input.Enabled = current.IsEnabled == 1
	if _, err = ParseCron(input.CronExpression); err != nil {
		return Schedule{}, apperror.InvalidRequest(err)
	}
	if _, err = LoadTimezone(input.Timezone); err != nil {
		return Schedule{}, apperror.InvalidRequest(err)
	}
	if err = s.catalog.ValidatePayload(input.TaskType, input.Params); err != nil {
		return Schedule{}, apperror.InvalidRequest(err)
	}
	row, err := s.repository.UpdateSchedule(ctx, id, input, &actor, s.now().UTC())
	return row, mapSchedulerError(err)
}
func (s *Service) SetStatus(ctx context.Context, id int64, enabled bool, actor int64) error {
	if id <= 0 || actor <= 0 {
		return apperror.InvalidRequest(errors.New("scheduler status input is invalid"))
	}
	return mapSchedulerError(s.repository.SetScheduleEnabled(ctx, id, enabled, &actor, s.now().UTC()))
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return apperror.InvalidRequest(errors.New("scheduler delete input is invalid"))
	}
	return mapSchedulerError(s.repository.DeleteSchedule(ctx, id, s.now().UTC()))
}
func (s *Service) Execute(ctx context.Context, id, actor int64) (Job, error) {
	if id <= 0 || actor <= 0 {
		return Job{}, apperror.InvalidRequest(errors.New("scheduler execute input is invalid"))
	}
	schedule, err := s.GetSchedule(ctx, id)
	if err != nil {
		return Job{}, err
	}
	definition, ok := s.catalog.Lookup(schedule.TaskType)
	if !ok {
		return Job{}, apperror.DependencyUnavailable(ErrUnknownTaskType)
	}
	now := s.now().UTC()
	job := Job{ScheduleID: &schedule.ID, TaskType: schedule.TaskType, Payload: append([]byte(nil), schedule.Params...), TriggerSource: TriggerManual, SourceKey: fmt.Sprintf("manual:%d:%d", schedule.ID, now.UnixNano()), ScheduledAt: now, AvailableAt: now, Status: JobScheduled, MaxAttempts: definition.MaxAttempts, Queue: definition.Queue, TimeoutSeconds: int(definition.Timeout / time.Second), TriggeredBy: &actor, CreatedAt: now, UpdatedAt: now}
	if err = s.repository.CreateJob(ctx, &job); err != nil {
		return Job{}, mapSchedulerError(err)
	}
	return job, nil
}
func (s *Service) ListJobs(ctx context.Context, q JobQuery) ([]Job, error) {
	if q.Limit < 1 || q.Limit > 100 || q.AfterID < 0 {
		return nil, apperror.InvalidRequest(errors.New("scheduler job query is invalid"))
	}
	rows, err := s.repository.ListJobs(ctx, q)
	return rows, mapSchedulerError(err)
}
func (s *Service) GetJob(ctx context.Context, id int64) (Job, error) {
	row, err := s.repository.GetJob(ctx, id)
	return row, mapSchedulerError(err)
}
func (s *Service) ListRuns(ctx context.Context, q RunQuery) ([]Run, error) {
	if q.JobID <= 0 || q.Limit < 1 || q.Limit > 100 {
		return nil, apperror.InvalidRequest(errors.New("scheduler run query is invalid"))
	}
	rows, err := s.repository.ListRuns(ctx, q)
	return rows, mapSchedulerError(err)
}
func (s *Service) Retry(ctx context.Context, id, actor int64) (Job, error) {
	if id <= 0 || actor <= 0 {
		return Job{}, apperror.InvalidRequest(errors.New("scheduler retry input is invalid"))
	}
	row, err := s.repository.CreateRetryJob(ctx, id, &actor, s.now().UTC())
	return row, mapSchedulerError(err)
}
func mapSchedulerError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperror.NotFound(err)
	case errors.Is(err, ErrConflict), errors.Is(err, ErrActiveJobExists), errors.Is(err, ErrBuiltinSchedule):
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	default:
		return apperror.DependencyUnavailable(err)
	}
}
