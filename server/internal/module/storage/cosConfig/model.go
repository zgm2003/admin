package cosconfig

import (
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
	"time"
)

// Model 是全局逻辑 COS 配置：只保存不随物理位置变化的账号与凭据事实。
// 物理位置（Bucket/Region/Endpoint/BucketDomain）在 storage_cos_config_version 中按 version 不可变保存。
type Model struct {
	ID                  int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Name                string         `gorm:"column:name;type:varchar(128);not null"`
	AppID               string         `gorm:"column:app_id;type:varchar(32);not null"`
	SecretIDCiphertext  string         `gorm:"column:secret_id_ciphertext;not null"`
	SecretKeyCiphertext string         `gorm:"column:secret_key_ciphertext;not null"`
	CurrentVersion      int64          `gorm:"column:current_version;not null"`
	IsEnabled           yesno.Value    `gorm:"column:is_enabled;type:smallint;not null"`
	Remark              string         `gorm:"column:remark;type:varchar(512);not null"`
	CreatedAt           time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt           gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (Model) TableName() string { return "storage_cos_config" }

// Version 是不可变的物理位置版本行：创建后不 UPDATE、不软删。
type Version struct {
	CosConfigID  int64     `gorm:"column:cos_config_id;primaryKey"`
	Version      int64     `gorm:"column:version;primaryKey"`
	Bucket       string    `gorm:"column:bucket;type:varchar(128);not null"`
	Region       string    `gorm:"column:region;type:varchar(64);not null"`
	Endpoint     *string   `gorm:"column:endpoint;type:varchar(255)"`
	BucketDomain *string   `gorm:"column:bucket_domain;type:varchar(255)"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (Version) TableName() string { return "storage_cos_config_version" }

// Current 是"逻辑配置 + 当前物理版本"的读模型；版本号与缓存 generation 都不进入管理 DTO。
type Current struct {
	Model
	Bucket       string
	Region       string
	Endpoint     *string
	BucketDomain *string
}

// RuntimeVersion 是运行时快照中的不可变物理版本。
type RuntimeVersion struct {
	Version      int64
	Bucket       string
	Region       string
	Endpoint     *string
	BucketDomain *string
}

// RuntimeConfig 是 (storage.cosconfig/<id>, runtime) 快照的完整事实：逻辑账号 + 当前 Secret 密文 + 全部物理版本。
type RuntimeConfig struct {
	ID                  int64
	AppID               string
	SecretIDCiphertext  string
	SecretKeyCiphertext string
	CurrentVersion      int64
	IsEnabled           yesno.Value
	Deleted             bool
	Versions            []RuntimeVersion
}

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
