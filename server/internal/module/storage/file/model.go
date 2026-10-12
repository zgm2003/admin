package file

import (
	"gorm.io/gorm"
	"time"
)

const (
	KindOriginal  int16 = 1
	KindAvatar    int16 = 2
	StatusPending int16 = 1
	StatusReady   int16 = 2
	StatusCleanup int16 = 3
)

type Model struct {
	ID                                       int64 `gorm:"primaryKey"`
	UserID, PlatformID                       int64
	Kind                                     int16
	CosConfigID, ConfigVersion, UploadRuleID int64
	RuleCode, ObjectKey                      string
	UploadObjectKey                          *string
	OriginalName, MimeType                   string
	SizeBytes                                int64
	Etag                                     string
	Status                                   int16
	CredentialExpiresAt, CompletedAt         *time.Time
	RequestKey                               *string
	SourceFileID                             *int64
	OperationLeaseUntil                      *time.Time
	CreatedAt, UpdatedAt                     time.Time
	DeletedAt                                gorm.DeletedAt
}

func (Model) TableName() string { return "storage_file" }
