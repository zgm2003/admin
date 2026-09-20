package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrClaimLost       = errors.New("scheduler claim lost")
	ErrActiveJobExists = errors.New("scheduler active job already exists")
	ErrBuiltinSchedule = errors.New("builtin schedule cannot be deleted")
	ErrInvalidSchedule = errors.New("scheduler schedule is invalid")
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) DB() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}

func (r *Repository) CreateSchedule(ctx context.Context, input ScheduleInput, actor *int64, now time.Time) (Schedule, error) {
	if r == nil || r.db == nil {
		return Schedule{}, errors.New("scheduler repository database is required")
	}
	if err := validateScheduleInput(input); err != nil {
		return Schedule{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	next, err := nextRun(input.CronExpression, input.Timezone, now)
	if err != nil {
		return Schedule{}, err
	}
	status := ScheduleDisabled
	if input.Enabled {
		status = ScheduleEnabled
	}
	var row Schedule
	query := `INSERT INTO system_scheduler_schedule(name,description,task_type,cron_expression,timezone,params,is_enabled,next_run_at,created_by,updated_by,created_at,updated_at)
	VALUES(?,?,?,?,?,?::jsonb,?, ?,?,?,?,?) RETURNING *`
	if err := r.db.WithContext(ctx).Raw(query, input.Name, input.Description, input.TaskType, input.CronExpression, input.Timezone, string(input.Params), boolToInt(input.Enabled), nullableNext(status, next), actor, actor, now, now).Scan(&row).Error; err != nil {
		return Schedule{}, mapRepositoryError(err)
	}
	row.Status = status
	row.IsEnabled = int16(boolToInt(input.Enabled))
	return row, nil
}

func (r *Repository) UpdateSchedule(ctx context.Context, id int64, input ScheduleInput, actor *int64, now time.Time) (Schedule, error) {
	if id <= 0 {
		return Schedule{}, ErrInvalidSchedule
	}
	if err := validateScheduleInput(input); err != nil {
		return Schedule{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	next, err := nextRun(input.CronExpression, input.Timezone, now)
	if err != nil {
		return Schedule{}, err
	}
	var row Schedule
	result := r.db.WithContext(ctx).Model(&Schedule{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{
		"name": input.Name, "description": input.Description, "cron_expression": input.CronExpression, "timezone": input.Timezone,
		"params": string(input.Params), "is_enabled": boolToInt(input.Enabled), "next_run_at": nullableNextBool(input.Enabled, next), "updated_by": actor, "updated_at": now,
	})
	if result.Error != nil {
		return Schedule{}, mapRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return Schedule{}, gorm.ErrRecordNotFound
	}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return Schedule{}, err
	}
	row.normalize()
	return row, nil
}

func (r *Repository) SetScheduleEnabled(ctx context.Context, id int64, enabled bool, actor *int64, now time.Time) error {
	if id <= 0 {
		return ErrInvalidSchedule
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var row struct {
		CronExpression string `gorm:"column:cron_expression"`
		Timezone       string `gorm:"column:timezone"`
	}
	if err := r.db.WithContext(ctx).Table("system_scheduler_schedule").Select("cron_expression,timezone").Where("id=? AND deleted_at IS NULL", id).Take(&row).Error; err != nil {
		return err
	}
	var next *time.Time
	if enabled {
		value, err := nextRun(row.CronExpression, row.Timezone, now)
		if err != nil {
			return err
		}
		next = &value
	}
	result := r.db.WithContext(ctx).Model(&Schedule{}).Where("id=? AND deleted_at IS NULL", id).Updates(map[string]any{"is_enabled": boolToInt(enabled), "next_run_at": next, "updated_by": actor, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) DeleteSchedule(ctx context.Context, id int64, now time.Time) error {
	if id <= 0 {
		return ErrInvalidSchedule
	}
	var row struct {
		BuiltinKey *string `gorm:"column:builtin_key"`
	}
	if err := r.db.WithContext(ctx).Table("system_scheduler_schedule").Select("builtin_key").Where("id=? AND deleted_at IS NULL", id).Take(&row).Error; err != nil {
		return err
	}
	if row.BuiltinKey != nil && strings.TrimSpace(*row.BuiltinKey) != "" {
		return ErrBuiltinSchedule
	}
	var count int64
	if err := r.db.WithContext(ctx).Table("system_scheduler_job").Where("schedule_id=? AND status IN ?", id, []JobStatus{JobScheduled, JobQueued, JobRunning}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrActiveJobExists
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	res := r.db.WithContext(ctx).Model(&Schedule{}).Where("id=? AND deleted_at IS NULL", id).Updates(map[string]any{"deleted_at": now, "updated_at": now, "is_enabled": 0, "next_run_at": nil})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) GetSchedule(ctx context.Context, id int64) (Schedule, error) {
	var row Schedule
	err := r.db.WithContext(ctx).Where("id=? AND deleted_at IS NULL", id).First(&row).Error
	row.normalize()
	return row, err
}

func (r *Repository) ListSchedules(ctx context.Context, query ScheduleQuery) ([]Schedule, error) {
	limit := normalizeLimit(query.Limit)
	db := r.db.WithContext(ctx).Where("deleted_at IS NULL")
	if query.Status != nil {
		db = db.Where("is_enabled=?", boolToInt(*query.Status == ScheduleEnabled))
	}
	if query.TaskType != "" {
		db = db.Where("task_type=?", query.TaskType)
	}
	if query.AfterID > 0 {
		db = db.Where("id > ?", query.AfterID)
	}
	var rows []Schedule
	err := db.Order("id ASC").Limit(limit).Find(&rows).Error
	for index := range rows {
		rows[index].normalize()
	}
	return rows, err
}

func (r *Repository) CreateJobTx(ctx context.Context, tx *gorm.DB, job *Job) error {
	if tx == nil || job == nil || job.TaskType == "" || job.MaxAttempts < 1 || job.Queue == "" || job.TimeoutSeconds < 1 {
		return ErrInvalidSchedule
	}
	if len(job.Payload) == 0 {
		job.Payload = json.RawMessage(`{}`)
	}
	if !json.Valid(job.Payload) {
		return ErrInvalidPayload
	}
	err := tx.WithContext(ctx).Raw(`INSERT INTO system_scheduler_job(
schedule_id,task_type,payload,trigger_source,source_key,scheduled_at,available_at,status,attempt_count,max_attempts,queue,timeout_seconds,retry_of_job_id,triggered_by,error_class,last_error,created_at,updated_at)
VALUES(?,?,?::jsonb,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`, job.ScheduleID, job.TaskType, string(job.Payload), job.TriggerSource, nullableString(job.SourceKey), job.ScheduledAt, job.AvailableAt, job.Status, job.AttemptCount, job.MaxAttempts, job.Queue, job.TimeoutSeconds, job.RetryOfJobID, job.TriggeredBy, job.ErrorClass, job.LastError, job.CreatedAt, job.UpdatedAt).Scan(&job.ID).Error
	if err != nil {
		return mapRepositoryError(err)
	}
	return nil
}

func (r *Repository) CreateJob(ctx context.Context, job *Job) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return r.CreateJobTx(ctx, tx, job) })
}

type BatchJobWriter struct {
	repository *Repository
	definition TaskDefinition
}

func NewBatchJobWriter(repository *Repository, definition TaskDefinition) (*BatchJobWriter, error) {
	if definition.Type != "message.notificationtask.batch" {
		return nil, ErrUnknownTaskType
	}
	if repository == nil || repository.db == nil {
		return nil, errors.New("scheduler batch job repository is required")
	}
	return &BatchJobWriter{repository: repository, definition: definition}, nil
}

// CreateBatchJobTx is the narrow transaction bridge used by notificationTask.
// The notification module supplies only task id/batch number; queue policy and
// scheduler envelope remain catalog-owned.
func (w *BatchJobWriter) CreateBatchJobTx(ctx context.Context, tx *gorm.DB, taskID int64, batchNo int, availableAt, now time.Time) error {
	if taskID <= 0 || batchNo < 0 {
		return ErrInvalidSchedule
	}
	payload, err := json.Marshal(struct {
		SchemaVersion int   `json:"schemaVersion"`
		TaskID        int64 `json:"taskId"`
		BatchNo       int   `json:"batchNo"`
	}{1, taskID, batchNo})
	if err != nil {
		return err
	}
	definition := w.definition
	job := Job{TaskType: definition.Type, Payload: payload, TriggerSource: TriggerBusiness, SourceKey: fmt.Sprintf("notificationTask:%d:%d", taskID, batchNo), ScheduledAt: availableAt, AvailableAt: availableAt, Status: JobScheduled, MaxAttempts: definition.MaxAttempts, Queue: definition.Queue, TimeoutSeconds: int(definition.Timeout / time.Second), CreatedAt: now, UpdatedAt: now}
	return w.repository.CreateJobTx(ctx, tx, &job)
}

func (w *BatchJobWriter) CancelBatchJobsTx(ctx context.Context, tx *gorm.DB, taskID int64, now time.Time) error {
	if taskID <= 0 {
		return ErrInvalidSchedule
	}
	result := tx.WithContext(ctx).Exec(`UPDATE system_scheduler_job SET status='canceled',publish_token=NULL,publish_lease_until=NULL,completed_at=?,updated_at=? WHERE task_type='message.notificationtask.batch' AND source_key LIKE ? AND status IN ('scheduled','queued')`, now, now, fmt.Sprintf("notificationTask:%d:%%", taskID))
	return result.Error
}

type DueJobBuilder func(schedule Schedule, scheduledAt time.Time) (Job, error)

// CreateDueJobs keeps schedule locking, active-job detection, frozen job creation,
// and next-fire advancement in one PostgreSQL transaction.
func (r *Repository) CreateDueJobs(ctx context.Context, now time.Time, limit int, builder DueJobBuilder) (int, error) {
	if r == nil || r.db == nil || builder == nil || now.IsZero() {
		return 0, errors.New("scheduler due scan dependencies are invalid")
	}
	limit = normalizeLimit(limit)
	created := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []Schedule
		if err := tx.WithContext(ctx).Raw(`SELECT * FROM system_scheduler_schedule WHERE deleted_at IS NULL AND is_enabled=1 AND next_run_at IS NOT NULL AND next_run_at<=? ORDER BY next_run_at,id LIMIT ? FOR UPDATE SKIP LOCKED`, now, limit).Scan(&rows).Error; err != nil {
			return err
		}
		for index := range rows {
			row := rows[index]
			row.normalize()
			scheduledAt := *row.NextRunAt
			next, err := nextFutureRun(row.CronExpression, row.Timezone, now)
			if err != nil {
				return err
			}
			var active int64
			if err := tx.WithContext(ctx).Table("system_scheduler_job").Where("schedule_id=? AND status IN ?", row.ID, []JobStatus{JobScheduled, JobQueued, JobRunning}).Count(&active).Error; err != nil {
				return err
			}
			if active == 0 {
				job, buildErr := builder(row, scheduledAt)
				if buildErr != nil {
					return buildErr
				}
				job.ScheduleID = &row.ID
				if err := r.CreateJobTx(ctx, tx, &job); err != nil {
					if errors.Is(err, ErrConflict) {
						active = 1
					} else {
						return err
					}
				} else {
					created++
				}
			}
			if err := tx.WithContext(ctx).Model(&Schedule{}).Where("id=? AND next_run_at=?", row.ID, scheduledAt).Updates(map[string]any{"next_run_at": next, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return created, err
}

func (r *Repository) FindActiveJobTx(ctx context.Context, tx *gorm.DB, scheduleID int64) (Job, error) {
	var row Job
	err := tx.WithContext(ctx).Where("schedule_id=? AND status IN ?", scheduleID, []JobStatus{JobScheduled, JobQueued, JobRunning}).Order("id DESC").First(&row).Error
	return row, err
}

func (r *Repository) ClaimPublishableJobs(ctx context.Context, now time.Time, limit int, token string, lease time.Duration) ([]PublishableJob, error) {
	if token == "" {
		token = newToken()
	}
	limit = normalizeLimit(limit)
	if lease <= 0 {
		lease = 30 * time.Second
	}
	until := now.Add(lease)
	var rows []PublishableJob
	err := r.db.WithContext(ctx).Raw(`WITH candidates AS (
SELECT id FROM system_scheduler_job WHERE status='scheduled' AND available_at<=? AND (publish_lease_until IS NULL OR publish_lease_until<=?) ORDER BY available_at,id LIMIT ? FOR UPDATE SKIP LOCKED
), claimed AS (
UPDATE system_scheduler_job j SET publish_token=?,publish_lease_until=?,updated_at=? FROM candidates c WHERE j.id=c.id RETURNING j.*)
SELECT *, ? AS dispatch_token FROM claimed ORDER BY available_at,id`, now, now, limit, token, until, now, token).Scan(&rows).Error
	for i := range rows {
		rows[i].DispatchToken = token
	}
	return rows, err
}

func (r *Repository) RecoverExpiredQueued(ctx context.Context, now time.Time, limit int) (int, error) {
	limit = normalizeLimit(limit)
	var recovered int
	err := r.db.WithContext(ctx).Raw(`WITH candidates AS (SELECT id FROM system_scheduler_job WHERE status='queued' AND publish_lease_until<=? ORDER BY publish_lease_until,id LIMIT ? FOR UPDATE SKIP LOCKED), recovered AS (UPDATE system_scheduler_job j SET status='scheduled',publish_token=NULL,publish_lease_until=NULL,available_at=?,error_class='transport-lost',last_error='queued task delivery lease expired',updated_at=? FROM candidates c WHERE j.id=c.id AND j.status='queued' RETURNING j.id) SELECT count(*) FROM recovered`, now, limit, now, now).Scan(&recovered).Error
	return recovered, err
}

func (r *Repository) MarkQueued(ctx context.Context, id int64, token string, now time.Time) error {
	res := r.db.WithContext(ctx).Exec(`UPDATE system_scheduler_job SET status='queued',updated_at=? WHERE id=? AND status='scheduled' AND publish_token=?`, now, id, token)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrClaimLost
	}
	return nil
}

func (r *Repository) ReschedulePublish(ctx context.Context, id int64, token, safeError string, availableAt, now time.Time) error {
	if len(safeError) > 1024 {
		safeError = safeError[:1024]
	}
	res := r.db.WithContext(ctx).Exec(`UPDATE system_scheduler_job SET available_at=?,publish_token=NULL,publish_lease_until=NULL,error_class='publish',last_error=?,updated_at=? WHERE id=? AND status='scheduled' AND publish_token=?`, availableAt, safeError, now, id, token)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrClaimLost
	}
	return nil
}

func (r *Repository) ClaimRun(ctx context.Context, jobID int64, expectedAttempt int, dispatchToken, workerID, token string, now time.Time, lease time.Duration) (Run, Job, error) {
	if jobID <= 0 || workerID == "" {
		return Run{}, Job{}, ErrInvalidSchedule
	}
	if token == "" {
		token = newToken()
	}
	if lease <= 0 {
		lease = 5 * time.Minute
	}
	var run Run
	var job Job
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", jobID).Take(&job).Error; err != nil {
			return err
		}
		if job.Status != JobQueued || job.AttemptCount+1 != expectedAttempt || job.PublishToken != dispatchToken {
			return ErrClaimLost
		}
		attempt := job.AttemptCount + 1
		finished := now.Add(lease)
		result := tx.WithContext(ctx).Model(&Job{}).Where("id=? AND status='queued' AND publish_token=?", jobID, dispatchToken).Updates(map[string]any{"status": "running", "attempt_count": attempt, "publish_token": nil, "publish_lease_until": nil, "run_token": token, "run_lease_until": finished, "worker_id": workerID, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrClaimLost
		}
		result = tx.WithContext(ctx).Table("system_scheduler_run").Create(map[string]any{"job_id": jobID, "attempt_no": attempt, "status": "running", "worker_id": workerID, "started_at": now, "created_at": now, "updated_at": now})
		if result.Error != nil {
			return mapRepositoryError(result.Error)
		}
		if err := tx.WithContext(ctx).Where("job_id=? AND attempt_no=?", jobID, attempt).Take(&run).Error; err != nil {
			return err
		}
		job.Status = JobRunning
		job.AttemptCount = attempt
		job.PublishToken = ""
		job.PublishLeaseUntil = nil
		job.RunToken = token
		job.RunLeaseUntil = &finished
		job.WorkerID = workerID
		return nil
	})
	return run, job, err
}

func (r *Repository) CompleteRun(ctx context.Context, jobID int64, attempt int, token string, now time.Time, summary string) error {
	if len(summary) > 1024 {
		summary = summary[:1024]
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.WithContext(ctx).Model(&Run{}).Where("job_id=? AND attempt_no=? AND status='running'", jobID, attempt).Updates(map[string]any{"status": "succeeded", "finished_at": now, "duration_ms": gorm.Expr("GREATEST(0, EXTRACT(EPOCH FROM (? - started_at))*1000)::bigint", now), "result_summary": summary, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrClaimLost
		}
		res = tx.WithContext(ctx).Model(&Job{}).Where("id=? AND status='running' AND run_token=?", jobID, token).Updates(map[string]any{"status": "completed", "run_token": nil, "run_lease_until": nil, "completed_at": now, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrClaimLost
		}
		return nil
	})
}

func (r *Repository) FailRun(ctx context.Context, jobID int64, attempt int, token, class, message string, retryAt *time.Time, now time.Time) error {
	if len(message) > 1024 {
		message = message[:1024]
	}
	if len(class) > 64 {
		class = class[:64]
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.WithContext(ctx).Model(&Run{}).Where("job_id=? AND attempt_no=? AND status='running'", jobID, attempt).Updates(map[string]any{"status": "failed", "finished_at": now, "duration_ms": gorm.Expr("GREATEST(0, EXTRACT(EPOCH FROM (? - started_at))*1000)::bigint", now), "error_class": class, "error_message": message, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrClaimLost
		}
		updates := map[string]any{"status": "failed", "run_token": nil, "run_lease_until": nil, "error_class": class, "last_error": message, "completed_at": now, "updated_at": now}
		if retryAt != nil {
			updates["status"] = "scheduled"
			updates["available_at"] = *retryAt
			updates["completed_at"] = nil
		}
		res = tx.WithContext(ctx).Model(&Job{}).Where("id=? AND status='running' AND run_token=?", jobID, token).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrClaimLost
		}
		return nil
	})
}

func (r *Repository) CancelQueuedJobs(ctx context.Context, scheduleID int64, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&Job{}).Where("schedule_id=? AND status IN ?", scheduleID, []JobStatus{JobScheduled, JobQueued}).Updates(map[string]any{"status": "canceled", "publish_token": nil, "publish_lease_until": nil, "completed_at": now, "updated_at": now})
	return res.Error
}

func (r *Repository) RecoverExpiredRuns(ctx context.Context, now time.Time, limit int) (int, error) {
	limit = normalizeLimit(limit)
	recovered := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var jobs []Job
		if err := tx.WithContext(ctx).Raw(`SELECT * FROM system_scheduler_job WHERE status='running' AND run_lease_until<=? ORDER BY run_lease_until,id LIMIT ? FOR UPDATE SKIP LOCKED`, now, limit).Scan(&jobs).Error; err != nil {
			return err
		}
		for _, job := range jobs {
			result := tx.WithContext(ctx).Model(&Run{}).Where("job_id=? AND attempt_no=? AND status='running'", job.ID, job.AttemptCount).Updates(map[string]any{
				"status": "failed", "finished_at": now,
				"duration_ms": gorm.Expr("GREATEST(0, EXTRACT(EPOCH FROM (? - started_at))*1000)::bigint", now),
				"error_class": "worker-lost", "error_message": "worker execution lease expired", "updated_at": now,
			})
			if result.Error != nil {
				return result.Error
			}
			updates := map[string]any{"status": "failed", "run_token": nil, "run_lease_until": nil, "worker_id": "", "error_class": "worker-lost", "last_error": "worker execution lease expired", "completed_at": now, "updated_at": now}
			if job.AttemptCount < job.MaxAttempts {
				updates["status"] = "scheduled"
				updates["available_at"] = now
				updates["completed_at"] = nil
			}
			result = tx.WithContext(ctx).Model(&Job{}).Where("id=? AND status='running' AND run_token=?", job.ID, job.RunToken).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrClaimLost
			}
			recovered++
		}
		return nil
	})
	return recovered, err
}

func (r *Repository) CreateRetryJob(ctx context.Context, jobID int64, actor *int64, now time.Time) (Job, error) {
	var old Job
	if err := r.db.WithContext(ctx).Where("id=?", jobID).First(&old).Error; err != nil {
		return Job{}, err
	}
	if old.Status != JobFailed {
		return Job{}, ErrConflict
	}
	newJob := old
	newJob.ID = 0
	newJob.Status = JobScheduled
	newJob.AttemptCount = 0
	newJob.RetryOfJobID = &old.ID
	newJob.TriggeredBy = actor
	newJob.SourceKey = fmt.Sprintf("retry:%d:%d", old.ID, now.UnixNano())
	newJob.AvailableAt = now
	newJob.ScheduledAt = now
	newJob.CompletedAt = nil
	newJob.PublishToken = ""
	newJob.PublishLeaseUntil = nil
	newJob.RunToken = ""
	newJob.RunLeaseUntil = nil
	newJob.WorkerID = ""
	newJob.ErrorClass = ""
	newJob.LastError = ""
	newJob.CreatedAt = now
	newJob.UpdatedAt = now
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return r.CreateJobTx(ctx, tx, &newJob) }); err != nil {
		return Job{}, err
	}
	return newJob, nil
}

func (r *Repository) ListJobs(ctx context.Context, query JobQuery) ([]Job, error) {
	db := r.db.WithContext(ctx)
	limit := normalizeLimit(query.Limit)
	if query.ScheduleID != nil {
		db = db.Where("schedule_id=?", *query.ScheduleID)
	}
	if query.Status != nil {
		db = db.Where("status=?", *query.Status)
	}
	if query.TaskType != "" {
		db = db.Where("task_type=?", query.TaskType)
	}
	if query.AfterID > 0 {
		db = db.Where("id>?", query.AfterID)
	}
	var rows []Job
	err := db.Order("id ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *Repository) GetJob(ctx context.Context, id int64) (Job, error) {
	var row Job
	err := r.db.WithContext(ctx).Where("id=?", id).First(&row).Error
	return row, err
}

func (r *Repository) ListRuns(ctx context.Context, query RunQuery) ([]Run, error) {
	var rows []Run
	err := r.db.WithContext(ctx).Where("job_id=? AND id>?", query.JobID, query.AfterID).Order("id ASC").Limit(normalizeLimit(query.Limit)).Find(&rows).Error
	return rows, err
}

func (r *Repository) CleanupTerminalHistory(ctx context.Context, cutoff time.Time, limit int) (deletedJobs, deletedRuns int, err error) {
	limit = normalizeLimit(limit)
	return deletedJobs, deletedRuns, r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []int64
		if err := tx.WithContext(ctx).Raw(`SELECT j.id FROM system_scheduler_job j WHERE j.status IN ('completed','failed','canceled') AND j.completed_at<? AND NOT EXISTS (SELECT 1 FROM system_scheduler_job active WHERE active.id=j.id AND active.status IN ('scheduled','queued','running')) ORDER BY j.completed_at,j.id LIMIT ? FOR UPDATE SKIP LOCKED`, cutoff, limit).Scan(&ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if res := tx.WithContext(ctx).Where("job_id IN ?", ids).Delete(&Run{}); res.Error != nil {
			return res.Error
		} else {
			deletedRuns = int(res.RowsAffected)
		}
		if res := tx.WithContext(ctx).Where("id IN ?", ids).Delete(&Job{}); res.Error != nil {
			return res.Error
		} else {
			deletedJobs = int(res.RowsAffected)
		}
		return nil
	})
}

func validateScheduleInput(input ScheduleInput) error {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.TaskType) == "" || strings.TrimSpace(input.Timezone) == "" || strings.TrimSpace(input.CronExpression) == "" {
		return ErrInvalidSchedule
	}
	if len(input.Params) == 0 {
		input.Params = json.RawMessage(`{}`)
	}
	if !json.Valid(input.Params) {
		return ErrInvalidPayload
	}
	return nil
}
func nextRun(expression, timezone string, now time.Time) (time.Time, error) {
	schedule, err := ParseCron(expression)
	if err != nil {
		return time.Time{}, err
	}
	location, err := LoadTimezone(timezone)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(now.In(location)).UTC(), nil
}
func nextFutureRun(expression, timezone string, now time.Time) (time.Time, error) {
	schedule, err := ParseCron(expression)
	if err != nil {
		return time.Time{}, err
	}
	location, err := LoadTimezone(timezone)
	if err != nil {
		return time.Time{}, err
	}
	next := schedule.Next(now.In(location))
	for !next.After(now) {
		next = schedule.Next(next)
	}
	return next.UTC(), nil
}
func nullableNext(status ScheduleStatus, value time.Time) *time.Time {
	if status != ScheduleEnabled {
		return nil
	}
	return &value
}
func nullableNextBool(enabled bool, value time.Time) *time.Time {
	if !enabled {
		return nil
	}
	return &value
}
func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
func normalizeLimit(value int) int {
	if value < 1 {
		return 50
	}
	if value > 500 {
		return 500
	}
	return value
}
func newToken() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}
func mapRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "duplicate key") {
		return ErrConflict
	}
	return err
}

func (s *Schedule) normalize() {
	if s == nil {
		return
	}
	if s.IsEnabled == 1 {
		s.Status = ScheduleEnabled
	} else {
		s.Status = ScheduleDisabled
	}
}

var ErrConflict = errors.New("scheduler conflict")
