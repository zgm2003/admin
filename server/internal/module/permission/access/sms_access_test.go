package permission

import (
	"reflect"
	"testing"

	"admin/server/internal/shared/yesno"
)

func TestBuildSnapshotKeepsSMSPageActionsAndIdentityActionsIndependent(t *testing.T) {
	messageID, smsPageID := int64(1), int64(2)
	profilePageID, smsActionID := int64(3), int64(4)
	emailActionID, phoneActionID := int64(5), int64(6)
	smsPath, smsComponent := "/message/sms", "message/sms"
	profilePath, profileComponent := "/user/profile", "user/profile"
	source := Source{
		Version: 1,
		Menus: []SourceMenu{
			{ID: messageID, MenuType: MenuDirectory, Code: "message", I18nKey: accessStringPointer("navigation.message"), IsEnabled: yesno.Yes, IsHidden: yesno.No},
			{ID: smsPageID, ParentID: &messageID, MenuType: MenuPage, Code: "message:sms:view", I18nKey: accessStringPointer("navigation.sms"), Path: &smsPath, ComponentPath: &smsComponent, IsEnabled: yesno.Yes, IsHidden: yesno.No},
			{ID: profilePageID, MenuType: MenuPage, Code: "user:profile:view", I18nKey: accessStringPointer("navigation.userProfile"), Path: &profilePath, ComponentPath: &profileComponent, IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
			{ID: smsActionID, ParentID: &smsPageID, MenuType: MenuAction, Code: "message:sms:list", IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
			{ID: emailActionID, ParentID: &profilePageID, MenuType: MenuAction, Code: "user:email:update", IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
			{ID: phoneActionID, ParentID: &profilePageID, MenuType: MenuAction, Code: "user:phone:update", IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
		},
	}

	actionOnly := source
	actionOnly.GrantedMenuIDs = []int64{smsActionID, emailActionID, phoneActionID}
	snapshot, err := buildSnapshot(actionOnly)
	if err != nil {
		t.Fatal(err)
	}
	wantActions := []string{"message:sms:list", "user:email:update", "user:phone:update"}
	if !reflect.DeepEqual(snapshot.PermissionCodes, wantActions) || len(snapshot.MenuTree) != 0 {
		t.Fatalf("action-only snapshot = %+v, want permissions %v and no routes", snapshot, wantActions)
	}

	pageOnly := source
	pageOnly.GrantedMenuIDs = []int64{smsPageID, profilePageID}
	snapshot, err = buildSnapshot(pageOnly)
	if err != nil {
		t.Fatal(err)
	}
	wantPages := []string{"message:sms:view", "user:profile:view"}
	if !reflect.DeepEqual(snapshot.PermissionCodes, wantPages) {
		t.Fatalf("page-only permissions = %v, want %v", snapshot.PermissionCodes, wantPages)
	}
	for _, action := range wantActions {
		if snapshotContainsMenuCode(snapshot.MenuTree, action) {
			t.Fatalf("action %q leaked into menu tree: %+v", action, snapshot.MenuTree)
		}
	}
}

func snapshotContainsMenuCode(nodes []MenuNode, code string) bool {
	for _, node := range nodes {
		if node.Code == code || snapshotContainsMenuCode(node.Children, code) {
			return true
		}
	}
	return false
}
