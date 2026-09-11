package config

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

const Table = "message_sms_config"

// Model is the global single active SMS configuration. Credentials are stored
// as SMS-domain ciphertext plus a masked hint; the plaintext never leaves memory.
type Model struct {
	ID                  int64          `gorm:"column:id;primaryKey;autoIncrement"`
	SecretIDCiphertext  string         `gorm:"column:secret_id_ciphertext;not null"`
	SecretKeyCiphertext string         `gorm:"column:secret_key_ciphertext;not null"`
	SecretIDHint        string         `gorm:"column:secret_id_hint;type:varchar(32);not null"`
	SecretKeyHint       string         `gorm:"column:secret_key_hint;type:varchar(32);not null"`
	SDKAppID            string         `gorm:"column:sms_sdk_app_id;type:varchar(64);not null"`
	SignName            string         `gorm:"column:sign_name;type:varchar(128);not null"`
	Region              string         `gorm:"column:region;type:varchar(64);not null"`
	Endpoint            *string        `gorm:"column:endpoint;type:varchar(255)"`
	TTLMinutes          int16          `gorm:"column:ttl_minutes;not null"`
	IsEnabled           yesno.Value    `gorm:"column:is_enabled;not null"`
	LastTestAt          *time.Time     `gorm:"column:last_test_at"`
	LastTestError       string         `gorm:"column:last_test_error;type:varchar(512);not null"`
	CreatedAt           time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt           gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (Model) TableName() string { return Table }
