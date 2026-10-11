package setting

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
	"context"
)

type settingAdminOptions struct {
	ValueTypes          []settingValueOption `json:"valueTypes"`
	DefaultValueType    int                  `json:"defaultValueType"`
	DefaultPresentation settingPresentation  `json:"defaultPresentation"`
}

type settingValueOption struct {
	option.Option[int]
	Editor     string `json:"editor"`
	AllowEmpty bool   `json:"allowEmpty"`
}

func adminOptions(ctx context.Context) (settingAdminOptions, error) {
	entries := []option.LocalizedEntry[int]{
		{Value: ValueTypeString, Key: i18n.KeySettingValueTypeString},
		{Value: ValueTypeNumber, Key: i18n.KeySettingValueTypeNumber},
		{Value: ValueTypeBool, Key: i18n.KeySettingValueTypeBool},
		{Value: ValueTypeJSON, Key: i18n.KeySettingValueTypeJSON},
		{Value: ValueTypeMedia, Key: i18n.KeySettingValueTypeMedia},
	}
	options, err := option.ManyLocalized(ctx, entries)
	if err != nil {
		return settingAdminOptions{}, err
	}
	valueTypes := make([]settingValueOption, 0, len(options))
	for _, item := range options {
		valueTypes = append(valueTypes, settingValueOption{Option: item, Editor: valueEditor(item.Value), AllowEmpty: validSettingValue("", item.Value)})
	}
	return settingAdminOptions{ValueTypes: valueTypes, DefaultValueType: ValueTypeString, DefaultPresentation: presentationFor(Record{ValueType: ValueTypeString})}, nil
}
