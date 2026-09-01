package menu

import (
	"reflect"
	"testing"
	"time"

	"admin/server/internal/shared/yesno"
)

func TestMenuProtocolV2UsesNameNullableI18nAndRenderState(t *testing.T) {
	tests := []struct {
		name     string
		value    reflect.Type
		required []string
	}{
		{name: "model", value: reflect.TypeOf(Menu{}), required: []string{"Name", "I18nKey", "ComponentPath", "IsHidden", "Remark"}},
		{name: "create input", value: reflect.TypeOf(CreateInput{}), required: []string{"Name", "I18nKey", "ComponentPath", "IsHidden", "Remark"}},
		{name: "update input", value: reflect.TypeOf(UpdateInput{}), required: []string{"Name", "I18nKey", "ComponentPath", "IsHidden", "Remark"}},
		{name: "managed menu", value: reflect.TypeOf(ManagedMenu{}), required: []string{"Name", "I18nKey", "ComponentPath", "IsHidden", "Remark"}},
		{name: "repository update", value: reflect.TypeOf(UpdateValues{}), required: []string{"Name", "I18nKey", "ComponentPath", "IsHidden", "Remark"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, field := range test.required {
				value, exists := test.value.FieldByName(field)
				if !exists {
					t.Errorf("%s lacks %s", test.name, field)
					continue
				}
				if field == "I18nKey" && value.Type.Kind() != reflect.Pointer {
					t.Errorf("%s I18nKey kind = %s, want pointer", test.name, value.Type.Kind())
				}
			}
		})
	}
}

func TestMenuProtocolV2AcceptsCustomI18nKeyAndIconName(t *testing.T) {
	icon := "lucide:shield-check"
	input := CreateInput{
		PlatformID: 1, MenuType: TypeDirectory, Name: "报表", Code: "reports", I18nKey: stringPointer("reports.orders.list"),
		Icon: &icon, SortOrder: 10, IsEnabled: yesno.Yes,
	}
	if _, err := normalizeCreateInput(input); err != nil {
		t.Fatalf("custom i18n key and icon were rejected: %v", err)
	}
}

func TestMenuProtocolV2RejectsStaticAndNonKebabPagePaths(t *testing.T) {
	for _, value := range []string{"/login", "/register", "/dashboard", "/System/users", "/system/users/", "/system/:id"} {
		if validMenuPath(value) {
			t.Errorf("validMenuPath(%q) = true", value)
		}
	}
	for _, value := range []string{"/system/users", "/reports/order-items"} {
		if !validMenuPath(value) {
			t.Errorf("validMenuPath(%q) = false", value)
		}
	}
}

func TestMenuProtocolV2RequiresActionsToBeHidden(t *testing.T) {
	now := time.Now().UTC()
	action := Menu{
		ID: 7, PlatformID: 1, ParentID: int64Pointer(3), MenuType: TypeAction,
		Name: "修改角色", Code: "permission:role:update", I18nKey: nil,
		IsEnabled: yesno.Yes, IsHidden: yesno.No, CreatedAt: now, UpdatedAt: now,
	}
	if err := validateStoredMenu(action); err == nil {
		t.Fatal("visible action was accepted")
	}
	action.IsHidden = yesno.Yes
	if err := validateStoredMenu(action); err != nil {
		t.Fatalf("hidden action was rejected: %v", err)
	}
}
