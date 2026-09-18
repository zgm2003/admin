package setting

import (
	"time"

	"admin/server/internal/shared/cacheGeneration"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

// settingGenerationScope 是 system/setting 固定使用的 generation scope；
// 集成测试可替换为隔离 scope，不随请求变化。
var settingGenerationScope = cachegeneration.Scope{Namespace: "system.setting", ScopeKey: "global"}

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

const (
	BrandTitleZhCNKey     = "app.brand.title_zh_cn"
	BrandTitleEnUSKey     = "app.brand.title_en_us"
	BrandDefaultAvatarKey = "app.brand.default_avatar"
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

type BrandSettings struct {
	TitleZhCN     string `json:"titleZhCN"`
	TitleEnUS     string `json:"titleEnUS"`
	DefaultAvatar string `json:"defaultAvatar"`
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
