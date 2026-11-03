package media

import (
	"encoding/json"
	"time"
)

const (
	TaskQueued    int16 = 0
	TaskPolling   int16 = 1
	TaskSucceeded int16 = 2
	TaskFailed    int16 = 3
	TaskCanceled  int16 = 4
	TaskExpired   int16 = 5
)

const (
	FileRoleInput     int16 = 1
	FileRoleGenerated int16 = 2

	SourceURL    int16 = 1
	SourceBase64 int16 = 2
	SourceStream int16 = 3

	TransferQueued int16 = 0
	Transfering    int16 = 1
	TransferReady  int16 = 2
	TransferRetry  int16 = 3
	TransferFailed int16 = 4
)

type Task struct {
	ID                int64           `gorm:"column:id;primaryKey"`
	RunID             int64           `gorm:"column:run_id"`
	ProviderRequestID int64           `gorm:"column:provider_request_id"`
	MediaKind         int16           `gorm:"column:media_kind"`
	UpstreamTaskID    *string         `gorm:"column:upstream_task_id"`
	Status            int16           `gorm:"column:status"`
	Progress          *int16          `gorm:"column:progress"`
	PollAttempts      int             `gorm:"column:poll_attempts"`
	NextPollAt        *time.Time      `gorm:"column:next_poll_at"`
	DeadlineAt        time.Time       `gorm:"column:deadline_at"`
	UpstreamResult    json.RawMessage `gorm:"column:upstream_result;type:jsonb"`
	ErrorCode         *string         `gorm:"column:error_code"`
	ErrorMessage      *string         `gorm:"column:error_message"`
	CreatedAt         time.Time       `gorm:"column:created_at"`
	UpdatedAt         time.Time       `gorm:"column:updated_at"`
}

func (Task) TableName() string { return "ai_media_task" }

type File struct {
	ID                int64           `gorm:"column:id;primaryKey"`
	RunID             int64           `gorm:"column:run_id"`
	MediaTaskID       *int64          `gorm:"column:media_task_id"`
	Role              int16           `gorm:"column:role"`
	Ordinal           int             `gorm:"column:ordinal"`
	StorageFileID     *int64          `gorm:"column:storage_file_id"`
	ImportRequestKey  *string         `gorm:"column:import_request_key"`
	SourceKind        int16           `gorm:"column:source_kind"`
	SourceCiphertext  *string         `gorm:"column:source_ciphertext"`
	EncryptionKeyID   *string         `gorm:"column:encryption_key_id"`
	SourceRecoverable int16           `gorm:"column:source_recoverable"`
	SourceExpiresAt   *time.Time      `gorm:"column:source_expires_at"`
	TransferDeadline  *time.Time      `gorm:"column:transfer_deadline_at"`
	MIMEType          *string         `gorm:"column:mime_type"`
	SizeBytes         *int64          `gorm:"column:size_bytes"`
	Metadata          json.RawMessage `gorm:"column:metadata;type:jsonb"`
	TransferStatus    int16           `gorm:"column:transfer_status"`
	TransferAttempts  int             `gorm:"column:transfer_attempts"`
	NextTransferAt    *time.Time      `gorm:"column:next_transfer_at"`
	ErrorCode         *string         `gorm:"column:error_code"`
	ErrorMessage      *string         `gorm:"column:error_message"`
	CreatedAt         time.Time       `gorm:"column:created_at"`
	UpdatedAt         time.Time       `gorm:"column:updated_at"`
}

func (File) TableName() string { return "ai_media_file" }
