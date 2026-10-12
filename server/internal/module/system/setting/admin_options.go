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
	Editor       string `json:"editor"`
	AllowEmpty   bool   `json:"allowEmpty"`
	MediaVariant string `json:"mediaVariant"`
	MediaAccept  string `json:"mediaAccept"`
}

func adminOptions(ctx context.Context) (settingAdminOptions, error) {
	entries := []option.LocalizedEntry[int]{
		{Value: ValueTypeString, Key: i18n.KeySettingValueTypeString},
		{Value: ValueTypeNumber, Key: i18n.KeySettingValueTypeNumber},
		{Value: ValueTypeBool, Key: i18n.KeySettingValueTypeBool},
		{Value: ValueTypeJSON, Key: i18n.KeySettingValueTypeJSON},
		{Value: ValueTypeMediaFile, Key: i18n.KeySettingValueTypeMediaFile},
		{Value: ValueTypeMediaVisual, Key: i18n.KeySettingValueTypeMediaVisual},
	}
	options, err := option.ManyLocalized(ctx, entries)
	if err != nil {
		return settingAdminOptions{}, err
	}
	valueTypes := make([]settingValueOption, 0, len(options))
	for _, item := range options {
		p := presentationFor(Record{ValueType: item.Value})
		valueTypes = append(valueTypes, settingValueOption{Option: item, Editor: p.Editor, AllowEmpty: p.AllowEmpty, MediaVariant: p.MediaVariant, MediaAccept: p.MediaAccept})
	}
	return settingAdminOptions{ValueTypes: valueTypes, DefaultValueType: ValueTypeString, DefaultPresentation: presentationFor(Record{ValueType: ValueTypeString})}, nil
}
