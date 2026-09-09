package uploadrule

import (
	"database/sql/driver"
	"fmt"
	"time"

	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgtype"
	"gorm.io/gorm"
)

type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	encoded, err := pgtype.NewMap().Encode(pgtype.TextArrayOID, pgtype.TextFormatCode, pgtype.FlatArray[string](a), nil)
	return string(encoded), err
}
func (a *StringArray) Scan(value any) error {
	var source []byte
	switch typed := value.(type) {
	case string:
		source = []byte(typed)
	case []byte:
		source = typed
	case nil:
		*a = nil
		return nil
	default:
		return fmt.Errorf("invalid postgres text array value %T", value)
	}
	var decoded pgtype.FlatArray[string]
	if err := pgtype.NewMap().Scan(pgtype.TextArrayOID, pgtype.TextFormatCode, source, &decoded); err != nil {
		return err
	}
	*a = StringArray(decoded)
	return nil
}

type Model struct {
	ID                int64          `gorm:"column:id;primaryKey;autoIncrement"`
	PlatformID        int64          `gorm:"column:platform_id;not null"`
	Name              string         `gorm:"column:name;type:varchar(128);not null"`
	CosConfigID       int64          `gorm:"column:cos_config_id;not null"`
	MaxFileSizeBytes  int64          `gorm:"column:max_file_size_bytes;not null"`
	AllowedExtensions StringArray    `gorm:"column:allowed_extensions;type:text[];not null"`
	AllowedMimeTypes  StringArray    `gorm:"column:allowed_mime_types;type:text[];not null"`
	AccessMode        string         `gorm:"column:access_mode;type:varchar(16);not null"`
	IsEnabled         yesno.Value    `gorm:"column:is_enabled;not null"`
	Remark            string         `gorm:"column:remark;not null"`
	CreatedAt         time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
	Codes             []string       `gorm:"-"`
}

func (Model) TableName() string { return "storage_upload_rule" }

type RuleValue struct {
	ID                int64       `json:"id"`
	PlatformID        int64       `json:"platformId"`
	PlatformCode      string      `json:"platformCode"`
	PlatformName      string      `json:"platformName"`
	Codes             []string    `json:"codes"`
	Name              string      `json:"name"`
	CosConfigID       int64       `json:"cosConfigId"`
	CosConfigName     string      `json:"cosConfigName"`
	MaxFileSizeBytes  int64       `json:"maxFileSizeBytes"`
	AllowedExtensions []string    `json:"allowedExtensions"`
	AllowedMimeTypes  []string    `json:"allowedMimeTypes"`
	AccessMode        string      `json:"accessMode"`
	IsEnabled         yesno.Value `json:"isEnabled"`
	Remark            string      `json:"remark"`
	CreatedAt         time.Time   `json:"createdAt"`
	UpdatedAt         time.Time   `json:"updatedAt"`
}

type RuleCode struct {
	ID         int64          `gorm:"column:id;primaryKey;autoIncrement"`
	RuleID     int64          `gorm:"column:rule_id;not null"`
	PlatformID int64          `gorm:"column:platform_id;not null"`
	Code       string         `gorm:"column:code;type:varchar(64);not null"`
	CreatedAt  time.Time      `gorm:"column:created_at;not null"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (RuleCode) TableName() string { return "storage_upload_rule_code" }
