package cosconfig

import (
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
	"time"
)

type Model struct {
	ID                  int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Name                string         `gorm:"column:name;type:varchar(128);not null"`
	AppID               string         `gorm:"column:app_id;type:varchar(32);not null"`
	SecretIDCiphertext  string         `gorm:"column:secret_id_ciphertext;not null"`
	SecretKeyCiphertext string         `gorm:"column:secret_key_ciphertext;not null"`
	Bucket              string         `gorm:"column:bucket;type:varchar(128);not null"`
	Region              string         `gorm:"column:region;type:varchar(64);not null"`
	Endpoint            *string        `gorm:"column:endpoint;type:varchar(255)"`
	BucketDomain        *string        `gorm:"column:bucket_domain;type:varchar(255)"`
	IsEnabled           yesno.Value    `gorm:"column:is_enabled;type:smallint;not null"`
	Remark              string         `gorm:"column:remark;type:varchar(512);not null"`
	CreatedAt           time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt           gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (Model) TableName() string { return "storage_cos_config" }

type SafeValue struct {
	ID             int64       `json:"id"`
	Name           string      `json:"name"`
	AppID          string      `json:"appId"`
	Bucket         string      `json:"bucket"`
	Region         string      `json:"region"`
	Endpoint       *string     `json:"endpoint"`
	BucketDomain   *string     `json:"bucketDomain"`
	IsEnabled      yesno.Value `json:"isEnabled"`
	HasCredentials bool        `json:"hasCredentials"`
	Remark         string      `json:"remark"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}
