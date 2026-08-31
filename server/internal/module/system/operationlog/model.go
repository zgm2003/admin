package operationlog

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"admin/server/internal/shared/yesno"
)

type JSON []byte

func (j *JSON) Scan(value interface{}) error {
	switch typed := value.(type) {
	case nil:
		*j = nil
		return nil
	case []byte:
		if !json.Valid(typed) {
			return fmt.Errorf("operation log JSONB is invalid")
		}
		*j = append((*j)[:0], typed...)
		return nil
	case string:
		if !json.Valid([]byte(typed)) {
			return fmt.Errorf("operation log JSONB is invalid")
		}
		*j = append((*j)[:0], typed...)
		return nil
	default:
		return fmt.Errorf("operation log JSONB has unsupported type %T", value)
	}
}

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("operation log JSONB is invalid")
	}
	return string(j), nil
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte(`null`), nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("operation log JSONB is invalid")
	}
	return j, nil
}

func (j *JSON) UnmarshalJSON(value []byte) error {
	if string(value) == `null` {
		*j = nil
		return nil
	}
	if !json.Valid(value) {
		return fmt.Errorf("operation log JSONB is invalid")
	}
	*j = append((*j)[:0], value...)
	return nil
}

type OperationLog struct {
	ID           int64       `gorm:"column:id;primaryKey;autoIncrement"`
	EventID      string      `gorm:"column:event_id;type:varchar(64);not null"`
	RequestID    string      `gorm:"column:request_id;type:varchar(128);not null"`
	UserID       *int64      `gorm:"column:user_id"`
	SessionID    *int64      `gorm:"column:session_id"`
	PlatformID   *int64      `gorm:"column:platform_id"`
	Method       string      `gorm:"column:method;type:varchar(10);not null"`
	Route        string      `gorm:"column:route;type:varchar(255);not null"`
	Module       string      `gorm:"column:module;type:varchar(64);not null"`
	Action       string      `gorm:"column:action;type:varchar(128);not null"`
	ClientIP     string      `gorm:"column:client_ip;type:varchar(64);not null"`
	UserAgent    string      `gorm:"column:user_agent;type:varchar(512);not null"`
	StatusCode   int32       `gorm:"column:status_code;not null"`
	IsSuccess    yesno.Value `gorm:"column:is_success;type:smallint;not null"`
	LatencyMs    int64       `gorm:"column:latency_ms;not null"`
	RequestData  JSON        `gorm:"column:request_data;type:jsonb"`
	ResponseData JSON        `gorm:"column:response_data;type:jsonb"`
	CreatedAt    time.Time   `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time   `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
}

func (OperationLog) TableName() string {
	return "system_operation_log"
}
