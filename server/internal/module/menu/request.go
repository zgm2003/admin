package menu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

type nullableInt64 struct {
	Value   *int64
	Present bool
}

func (value *nullableInt64) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Value = nil
		return nil
	}
	var decoded int64
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value.Value = &decoded
	return nil
}

type nullableString struct {
	Value   *string
	Present bool
}

func (value *nullableString) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Value = nil
		return nil
	}
	var decoded string
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value.Value = &decoded
	return nil
}

type createRequest struct {
	ParentID  nullableInt64  `json:"parentId"`
	MenuType  *Type          `json:"menuType"`
	Code      *string        `json:"code"`
	I18nKey   *string        `json:"i18nKey"`
	Path      nullableString `json:"path"`
	ViewKey   nullableString `json:"viewKey"`
	Icon      nullableString `json:"icon"`
	SortOrder *int           `json:"sortOrder"`
	IsEnabled *yesno.Value   `json:"isEnabled"`
}

func (request createRequest) input() (CreateInput, error) {
	if !request.ParentID.Present || request.MenuType == nil || request.Code == nil || request.I18nKey == nil ||
		!request.Path.Present || !request.ViewKey.Present || !request.Icon.Present || request.SortOrder == nil || request.IsEnabled == nil {
		return CreateInput{}, apperror.InvalidRequest(fmt.Errorf("all create menu fields are required"))
	}
	if request.ParentID.Value != nil && *request.ParentID.Value < 1 {
		return CreateInput{}, apperror.InvalidRequest(fmt.Errorf("parentId must be null or a positive integer"))
	}
	if *request.SortOrder < 0 || !yesno.IsValid(*request.IsEnabled) {
		return CreateInput{}, apperror.InvalidRequest(fmt.Errorf("sortOrder or isEnabled is invalid"))
	}
	return CreateInput{
		ParentID: request.ParentID.Value, MenuType: *request.MenuType, Code: *request.Code,
		I18nKey: *request.I18nKey, Path: request.Path.Value, ViewKey: request.ViewKey.Value,
		Icon: request.Icon.Value, SortOrder: *request.SortOrder, IsEnabled: *request.IsEnabled,
	}, nil
}

type updateRequest struct {
	ParentID  nullableInt64  `json:"parentId"`
	MenuType  *Type          `json:"menuType"`
	I18nKey   *string        `json:"i18nKey"`
	Path      nullableString `json:"path"`
	ViewKey   nullableString `json:"viewKey"`
	Icon      nullableString `json:"icon"`
	SortOrder *int           `json:"sortOrder"`
}

func (request updateRequest) input() (UpdateInput, error) {
	if !request.ParentID.Present || request.MenuType == nil || request.I18nKey == nil ||
		!request.Path.Present || !request.ViewKey.Present || !request.Icon.Present || request.SortOrder == nil {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("all update menu fields are required"))
	}
	if request.ParentID.Value != nil && *request.ParentID.Value < 1 {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("parentId must be null or a positive integer"))
	}
	if *request.SortOrder < 0 {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("sortOrder must not be negative"))
	}
	return UpdateInput{
		ParentID: request.ParentID.Value, MenuType: *request.MenuType, I18nKey: *request.I18nKey,
		Path: request.Path.Value, ViewKey: request.ViewKey.Value, Icon: request.Icon.Value,
		SortOrder: *request.SortOrder,
	}, nil
}

type statusRequest struct {
	IsEnabled *yesno.Value `json:"isEnabled"`
}

func (request statusRequest) value() (yesno.Value, error) {
	if request.IsEnabled == nil || !yesno.IsValid(*request.IsEnabled) {
		return 0, apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
	}
	return *request.IsEnabled, nil
}

func parseMenuID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, apperror.InvalidRequest(fmt.Errorf("menu id must be a positive integer"))
	}
	return id, nil
}
