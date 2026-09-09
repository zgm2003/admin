package config

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

const Table = "message_mail_config"

type Model struct {
	ID                  int64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SecretIDCiphertext  string         `gorm:"column:secret_id_ciphertext;not null" json:"-"`
	SecretKeyCiphertext string         `gorm:"column:secret_key_ciphertext;not null" json:"-"`
	SecretIDHint        string         `gorm:"column:secret_id_hint;type:varchar(32);not null" json:"secretIdHint"`
	SecretKeyHint       string         `gorm:"column:secret_key_hint;type:varchar(32);not null" json:"secretKeyHint"`
	Region              string         `gorm:"column:region;type:varchar(64);not null" json:"region"`
	Endpoint            *string        `gorm:"column:endpoint;type:varchar(255)" json:"endpoint"`
	FromEmail           string         `gorm:"column:from_email;type:varchar(254);not null" json:"fromEmail"`
	FromName            string         `gorm:"column:from_name;type:varchar(128);not null" json:"fromName"`
	ReplyTo             *string        `gorm:"column:reply_to;type:varchar(254)" json:"replyTo"`
	TTLMinutes          int16          `gorm:"column:ttl_minutes;not null" json:"ttlMinutes"`
	IsEnabled           yesno.Value    `gorm:"column:is_enabled;type:smallint;not null" json:"isEnabled"`
	LastTestAt          *time.Time     `gorm:"column:last_test_at;type:timestamptz" json:"lastTestAt"`
	LastTestError       string         `gorm:"column:last_test_error;type:varchar(512);not null" json:"lastTestError"`
	CreatedAt           time.Time      `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt           time.Time      `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt           gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz" json:"-"`
}

func (Model) TableName() string { return "message_mail_config" }
