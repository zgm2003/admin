package setting

import (
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	"encoding/json"
	"testing"
)

func TestSettingResponseOwnsEditorConstraintsAndActions(t *testing.T) {
	tests := []struct {
		key       string
		valueType int
		minimum   int
		maximum   int
		editor    string
		locked    bool
	}{
		{sharedsetting.MessageNotificationRetentionDaysKey, ValueTypeNumber, 30, 3650, "number", true},
		{sharedsetting.RealtimeEventRetentionDaysKey, ValueTypeNumber, 1, 30, "number", true},
		{sharedsetting.SchedulerHistoryRetentionDaysKey, ValueTypeNumber, 7, 3650, "number", true},
		{BrandDefaultAvatarKey, ValueTypeMedia, 0, 0, "media", true},
		{"custom.key", ValueTypeJSON, 0, 0, "json", false},
	}
	for _, test := range tests {
		data, err := json.Marshal(itemResponse(Record{Key: test.key, ValueType: test.valueType, IsBuiltin: yesno.Yes}))
		if err != nil {
			t.Fatal(err)
		}
		var row struct {
			Presentation struct {
				Editor          string
				ValueTypeLocked bool
				Minimum         *int
				Maximum         *int
				Actions         struct {
					Status bool
					Delete bool
				}
			}
		}
		if err = json.Unmarshal(data, &row); err != nil {
			t.Fatal(err)
		}
		p := row.Presentation
		if p.Editor != test.editor || p.ValueTypeLocked != test.locked {
			t.Fatalf("key=%s presentation=%s", test.key, data)
		}
		if test.minimum != 0 && (p.Minimum == nil || p.Maximum == nil || *p.Minimum != test.minimum || *p.Maximum != test.maximum) {
			t.Fatalf("key=%s ranges=%s", test.key, data)
		}
		if p.Actions.Delete || (test.locked && p.Actions.Status) {
			t.Fatalf("builtin/required actions=%s", data)
		}
	}
}

func TestCreateResponseOwnsPresentation(t *testing.T) {
	row := createdResponse(12, CreateInput{Key: "  " + BrandTitleZhCNKey + "  ", ValueType: ValueTypeString})
	if row.ID != 12 || !row.Presentation.RefreshBrand || row.Presentation.AllowEmpty {
		t.Fatalf("incorrect create response %#v", row)
	}
}
