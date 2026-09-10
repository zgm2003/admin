package dictionary

import (
	"time"

	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
)

const (
	PermissionView   = "system:dictionary:view"
	PermissionList   = "system:dictionary:list"
	PermissionDetail = "system:dictionary:detail"
	PermissionCreate = "system:dictionary:create"
	PermissionUpdate = "system:dictionary:update"
	PermissionStatus = "system:dictionary:status"
	PermissionDelete = "system:dictionary:delete"
)

type ListQuery struct {
	pagination.Request
	Keyword   string
	IsEnabled *yesno.Value
}

type ListItem struct {
	Dictionary
	ItemCount int64
}

type Detail struct {
	Dictionary Dictionary
	Items      []Item
}

type OptionsItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type CreateInput struct {
	Code        string
	NameZH      string
	NameEN      string
	Description string
}

type UpdateInput struct {
	NameZH      string
	NameEN      string
	Description string
}

type CreateItemInput struct {
	Value   string
	LabelZH string
	LabelEN string
	Sort    int
}

type UpdateItemInput struct {
	LabelZH string
	LabelEN string
	Sort    int
}

type ListResult = pagination.Result[ListItem]

type OptionResult map[string][]OptionsItem

type DictionaryView struct {
	ID          int64       `json:"id"`
	Code        string      `json:"code"`
	NameZH      string      `json:"nameZh"`
	NameEN      string      `json:"nameEn"`
	Description string      `json:"description"`
	IsEnabled   yesno.Value `json:"isEnabled"`
	IsBuiltin   yesno.Value `json:"isBuiltin"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}
