package menu

import (
	"reflect"
	"testing"
	"time"

	"admin/server/internal/shared/yesno"
)

func TestMenuProtocolV2UsesComponentPathAndHiddenState(t *testing.T) {
	tests := []struct {
		name     string
		value    reflect.Type
		required []string
	}{
		{name: "model", value: reflect.TypeOf(Menu{}), required: []string{"ComponentPath", "IsHidden"}},
		{name: "create input", value: reflect.TypeOf(CreateInput{}), required: []string{"ComponentPath", "IsHidden"}},
		{name: "update input", value: reflect.TypeOf(UpdateInput{}), required: []string{"ComponentPath", "IsHidden"}},
		{name: "managed menu", value: reflect.TypeOf(ManagedMenu{}), required: []string{"ComponentPath", "IsHidden"}},
		{name: "repository update", value: reflect.TypeOf(UpdateValues{}), required: []string{"ComponentPath", "IsHidden"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, field := range test.required {
				if _, exists := test.value.FieldByName(field); !exists {
					t.Errorf("%s lacks %s", test.name, field)
				}
			}
		})
	}
}

func TestMenuProtocolV2AcceptsCustomI18nKeyAndIconName(t *testing.T) {
	icon := "mdi:shield"
	input := CreateInput{
		MenuType: TypeDirectory, Code: "reports", I18nKey: "reports.orders.list",
		Icon: &icon, SortOrder: 10, IsEnabled: yesno.Yes,
	}
	if _, err := normalizeCreateInput(input); err != nil {
		t.Fatalf("custom i18n key and icon were rejected: %v", err)
	}
}

func TestMenuProtocolV2RejectsStaticAndNonKebabPagePaths(t *testing.T) {
	for _, value := range []string{"/login", "/register", "/dashboard", "/system/menus", "/System/users", "/system/users/", "/system/:id"} {
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
		ID: 7, ParentID: int64Pointer(3), MenuType: TypeAction,
		Code: "system:role:update", I18nKey: "permission.roleUpdate",
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
