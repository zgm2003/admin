package setting

import (
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

type settingActions struct {
	Status bool `json:"status"`
	Delete bool `json:"delete"`
}
type settingPresentation struct {
	Editor          string         `json:"editor"`
	ValueTypeLocked bool           `json:"valueTypeLocked"`
	Minimum         *int           `json:"minimum"`
	Maximum         *int           `json:"maximum"`
	MaxLength       *int           `json:"maxLength"`
	AllowEmpty      bool           `json:"allowEmpty"`
	WarnOnDecrease  bool           `json:"warnOnDecrease"`
	RefreshBrand    bool           `json:"refreshBrand"`
	MediaAccept     string         `json:"mediaAccept"`
	MediaVariant    string         `json:"mediaVariant"`
	MediaRuleCode   string         `json:"mediaRuleCode"`
	Actions         settingActions `json:"actions"`
}

const brandTitleMaxRunes = 128

type retentionBounds struct {
	Minimum int
	Maximum int
}

var retentionLimits = map[string]retentionBounds{
	sharedsetting.MessageNotificationRetentionDaysKey: {30, 3650},
	sharedsetting.RealtimeEventRetentionDaysKey:       {1, 30},
	sharedsetting.SchedulerHistoryRetentionDaysKey:    {7, 3650},
}

func valueEditor(valueType int) string {
	switch valueType {
	case ValueTypeString:
		return "text"
	case ValueTypeNumber:
		return "number"
	case ValueTypeBool:
		return "boolean"
	case ValueTypeJSON:
		return "json"
	case ValueTypeMedia:
		return "media"
	default:
		return "text"
	}
}

func presentationFor(row Record) settingPresentation {
	p := settingPresentation{Editor: valueEditor(row.ValueType), AllowEmpty: validSettingValue("", row.ValueType),
		MediaRuleCode: "setting", MediaVariant: "file",
		Actions: settingActions{Status: !isRequiredSetting(row.Key), Delete: row.IsBuiltin == yesno.No && !isRequiredSetting(row.Key)}}
	if bounds, ok := retentionLimits[row.Key]; ok {
		p.Minimum = &bounds.Minimum
		p.Maximum = &bounds.Maximum
		p.ValueTypeLocked = true
		p.WarnOnDecrease = true
	}
	switch row.Key {
	case BrandTitleZhCNKey, BrandTitleEnUSKey:
		p.AllowEmpty = false
		maximum := brandTitleMaxRunes
		p.MaxLength = &maximum
		p.ValueTypeLocked = true
		p.RefreshBrand = true
	case BrandDefaultAvatarKey:
		p.ValueTypeLocked = true
		p.RefreshBrand = true
		p.MediaVariant = "avatar"
		p.MediaAccept = ".png,.jpg,.jpeg,.gif,.webp"
	case sharedsetting.MailRecipientRuleImportTemplateObjectKey:
		p.ValueTypeLocked = true
		p.MediaAccept = ".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	return p
}
