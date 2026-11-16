package tool

import "time"

type Status int16

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

type Kind int16

const (
	KindBuiltin Kind = 1
	KindHTTP    Kind = 2
)

func (status Status) Valid() bool { return status == StatusDisabled || status == StatusEnabled }

func (status Status) Label() string {
	if status == StatusEnabled {
		return "启用"
	}
	return "停用"
}

func (kind Kind) Valid() bool { return kind == KindBuiltin || kind == KindHTTP }

func (kind Kind) Label() string {
	if kind == KindHTTP {
		return "HTTP 工具"
	}
	return "内置工具"
}

type Tool struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Code        string     `gorm:"column:code;type:varchar(64);not null"`
	APIName     string     `gorm:"column:api_name;type:varchar(64);not null"`
	Name        string     `gorm:"column:name;type:varchar(128);not null"`
	Description string     `gorm:"column:description;type:varchar(500);not null"`
	Category    string     `gorm:"column:category;type:varchar(32);not null"`
	Kind        Kind       `gorm:"column:kind;type:smallint;not null"`
	Parameters  string     `gorm:"column:parameters;type:jsonb;not null"`
	Method      string     `gorm:"column:method;type:varchar(8);not null"`
	URL         string     `gorm:"column:url;type:varchar(2000);not null"`
	TimeoutMS   int32      `gorm:"column:timeout_ms;not null"`
	Status      Status     `gorm:"column:status;type:smallint;not null"`
	SortOrder   int32      `gorm:"column:sort_order;not null"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt   *time.Time `gorm:"column:deleted_at;type:timestamptz"`
}

func (Tool) TableName() string { return "ai_tool" }
