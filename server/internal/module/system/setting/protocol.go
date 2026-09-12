package setting

import (
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

const (
	PermissionView   = "system:setting:view"
	PermissionList   = "system:setting:list"
	PermissionDetail = "system:setting:detail"
	PermissionCreate = "system:setting:create"
	PermissionUpdate = "system:setting:update"
	PermissionStatus = "system:setting:status"
	PermissionDelete = "system:setting:delete"
)

const (
	ValueTypeString = sharedsetting.ValueTypeString
	ValueTypeNumber = sharedsetting.ValueTypeNumber
	ValueTypeBool   = sharedsetting.ValueTypeBool
	ValueTypeJSON   = sharedsetting.ValueTypeJSON
)

var ErrNotFound = errNotFound{}

type errNotFound struct{}

func (errNotFound) Error() string { return "setting not found" }

var ErrConflict = errConflict{}

type errConflict struct{}

func (errConflict) Error() string { return "setting key conflict" }

type ListQuery struct {
	Page      int
	PageSize  int
	Keyword   string
	IsEnabled *yesno.Value
}

type ListResult struct {
	Items    []Record
	Total    int64
	Page     int
	PageSize int
}

type CreateInput struct {
	Key, Value, Description string
	ValueType               int
}
type UpdateInput struct {
	Value, Description string
	ValueType          int
}

type Detail struct{ Record Record }

type listItem struct {
	ID          int64       `json:"id"`
	Key         string      `json:"key"`
	Value       string      `json:"value"`
	ValueType   int         `json:"valueType"`
	Description string      `json:"description"`
	IsEnabled   yesno.Value `json:"isEnabled"`
	IsBuiltin   yesno.Value `json:"isBuiltin"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}
