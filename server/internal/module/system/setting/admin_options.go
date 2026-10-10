package setting

import (
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

func adminOptions(ctx context.Context) settingAdminOptions {
	return settingAdminOptions{
		ValueTypes: []settingValueOption{
			{option.New(ctx, ValueTypeString, "字符串", "String"), valueEditor(ValueTypeString), validSettingValue("", ValueTypeString)},
			{option.New(ctx, ValueTypeNumber, "数字", "Number"), valueEditor(ValueTypeNumber), validSettingValue("", ValueTypeNumber)},
			{option.New(ctx, ValueTypeBool, "布尔", "Boolean"), valueEditor(ValueTypeBool), validSettingValue("", ValueTypeBool)},
			{option.New(ctx, ValueTypeJSON, "JSON", "JSON"), valueEditor(ValueTypeJSON), validSettingValue("", ValueTypeJSON)},
			{option.New(ctx, ValueTypeMedia, "媒体", "Media"), valueEditor(ValueTypeMedia), validSettingValue("", ValueTypeMedia)},
		},
		DefaultValueType: ValueTypeString, DefaultPresentation: presentationFor(Record{ValueType: ValueTypeString}),
	}
}
