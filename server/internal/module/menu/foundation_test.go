package menu

import (
	"testing"
	"time"

	"admin/server/internal/shared/yesno"
)

func TestEnsureFoundationSeedsFullCatalogAndIsIdempotent(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	activeUser := createMenuAccessUser(t, tx, ctx, yesno.Yes, false)
	definitions := testFoundationDefinitions()

	if err := service.EnsureFoundation(ctx, definitions); err != nil {
		t.Fatalf("EnsureFoundation() error = %v", err)
	}
	var rows []Menu
	if err := tx.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(definitions) {
		t.Fatalf("seeded menus = %d, want %d", len(rows), len(definitions))
	}
	adminPlatformID := testAdminPlatformID(t, tx, ctx)
	for _, row := range rows {
		if row.PlatformID != adminPlatformID {
			t.Fatalf("foundation menu %s platform = %d, want Admin %d", row.Code, row.PlatformID, adminPlatformID)
		}
	}
	if got := readMenuAccessVersion(t, tx, ctx, activeUser.ID); got != 2 {
		t.Fatalf("seed access version = %d, want 2", got)
	}
	updatedAt := make(map[string]time.Time, len(rows))
	for _, row := range rows {
		updatedAt[row.Code] = row.UpdatedAt
	}

	if err := service.EnsureFoundation(ctx, definitions); err != nil {
		t.Fatalf("repeat EnsureFoundation() error = %v", err)
	}
	if got := readMenuAccessVersion(t, tx, ctx, activeUser.ID); got != 2 {
		t.Fatalf("idempotent access version = %d, want 2", got)
	}
	rows = nil
	if err := tx.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if !row.UpdatedAt.Equal(updatedAt[row.Code]) {
			t.Fatalf("idempotent foundation rewrote %s", row.Code)
		}
	}
}

func TestEnsureFoundationDoesNotClaimSameCodeFromAnotherPlatform(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	adminPlatformID := testAdminPlatformID(t, tx, ctx)
	canvas := createRepositoryPlatform(t, tx, ctx, "canvas", "Canvas", yesno.Yes, false)
	canvasAccess := Menu{
		PlatformID: canvas.ID, MenuType: TypeDirectory, Name: "Canvas Access", Code: "access",
		I18nKey: stringPointer("navigation.access"), IsEnabled: yesno.Yes, IsHidden: yesno.No,
	}
	if err := NewRepository(tx).Create(ctx, &canvasAccess); err != nil {
		t.Fatal(err)
	}

	if err := service.EnsureFoundation(ctx, testFoundationDefinitions()); err != nil {
		t.Fatalf("EnsureFoundation() error = %v", err)
	}

	var storedCanvas Menu
	if err := tx.WithContext(ctx).Where("id = ?", canvasAccess.ID).Take(&storedCanvas).Error; err != nil {
		t.Fatal(err)
	}
	if storedCanvas.PlatformID != canvas.ID || storedCanvas.Name != "Canvas Access" {
		t.Fatalf("Canvas menu was claimed by Admin foundation: %+v", storedCanvas)
	}
	var adminProtectedCount int64
	if err := tx.WithContext(ctx).Model(&Menu{}).
		Where("platform_id = ? AND code IN ?", adminPlatformID, []string{"access", PermissionList, PermissionCreate, PermissionUpdate, PermissionDelete}).
		Count(&adminProtectedCount).Error; err != nil {
		t.Fatal(err)
	}
	if adminProtectedCount != 5 {
		t.Fatalf("Admin protected foundation count = %d, want 5", adminProtectedCount)
	}
}

func TestEnsureFoundationRestoresOnlyProtectedNodesInNonEmptyCatalog(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	activeUser := createMenuAccessUser(t, tx, ctx, yesno.Yes, false)
	definitions := testFoundationDefinitions()
	if err := service.EnsureFoundation(ctx, definitions); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := tx.WithContext(ctx).Model(&Menu{}).
		Where("code IN ?", []string{PermissionCreate, "account:user:list"}).
		Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.EnsureFoundation(ctx, definitions); err != nil {
		t.Fatalf("restore foundation error = %v", err)
	}

	var protectedCount, ordinaryCount int64
	if err := tx.WithContext(ctx).Model(&Menu{}).Where("code = ?", PermissionCreate).Count(&protectedCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&Menu{}).Where("code = ?", "account:user:list").Count(&ordinaryCount).Error; err != nil {
		t.Fatal(err)
	}
	if protectedCount != 1 || ordinaryCount != 0 {
		t.Fatalf("active restored counts: protected=%d ordinary=%d", protectedCount, ordinaryCount)
	}
	if got := readMenuAccessVersion(t, tx, ctx, activeUser.ID); got != 3 {
		t.Fatalf("restore access version = %d, want 3", got)
	}
}

func TestProtectedMenusAllowOnlyPresentationUpdates(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	if err := service.EnsureFoundation(ctx, testFoundationDefinitions()); err != nil {
		t.Fatal(err)
	}
	var page Menu
	if err := tx.WithContext(ctx).Where("code = ?", PermissionList).Take(&page).Error; err != nil {
		t.Fatal(err)
	}

	changedI18nKey := "navigation.customMenus"
	changedIcon := "lucide:panel-left"
	if err := service.Update(ctx, page.ID, UpdateInput{
		ParentID: page.ParentID, MenuType: page.MenuType, Name: "自定义菜单中心", I18nKey: &changedI18nKey,
		Path: page.Path, ComponentPath: page.ComponentPath, Icon: &changedIcon, SortOrder: 99, IsHidden: page.IsHidden,
	}); err != nil {
		t.Fatalf("presentation update error = %v", err)
	}

	otherPath := "/access/other-menus"
	if err := service.Update(ctx, page.ID, UpdateInput{
		ParentID: page.ParentID, MenuType: page.MenuType, Name: "自定义菜单中心", I18nKey: &changedI18nKey,
		Path: &otherPath, ComponentPath: page.ComponentPath, Icon: &changedIcon, SortOrder: 99, IsHidden: page.IsHidden,
	}); menuServiceErrorCode(err) != CodeMenuProtected {
		t.Fatalf("protected structure update error = %v", err)
	}
	if err := service.UpdateStatus(ctx, page.ID, yesno.No); menuServiceErrorCode(err) != CodeMenuProtected {
		t.Fatalf("protected disable error = %v", err)
	}
	if err := service.Delete(ctx, page.ID); menuServiceErrorCode(err) != CodeMenuProtected {
		t.Fatalf("protected delete error = %v", err)
	}
}

func testFoundationDefinitions() []FoundationDefinition {
	return []FoundationDefinition{
		{MenuType: TypeDirectory, Name: "权限与认证", Code: "access", I18nKey: stringPointer("navigation.access"), Icon: stringPointer("lucide:shield-check"), SortOrder: 200, IsEnabled: yesno.Yes, IsHidden: yesno.No, Protected: true},
		{ParentCode: "access", MenuType: TypePage, Name: "菜单管理", Code: PermissionList, I18nKey: stringPointer("navigation.accessMenus"), Path: stringPointer("/access/menus"), ComponentPath: stringPointer("access/menus"), Icon: stringPointer("lucide:panel-left"), SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.No, Protected: true},
		{ParentCode: PermissionList, MenuType: TypeAction, Name: "新增菜单", Code: PermissionCreate, SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.Yes, Protected: true},
		{ParentCode: PermissionList, MenuType: TypeAction, Name: "修改菜单", Code: PermissionUpdate, SortOrder: 20, IsEnabled: yesno.Yes, IsHidden: yesno.Yes, Protected: true},
		{ParentCode: PermissionList, MenuType: TypeAction, Name: "删除菜单", Code: PermissionDelete, SortOrder: 30, IsEnabled: yesno.Yes, IsHidden: yesno.Yes, Protected: true},
		{MenuType: TypeDirectory, Name: "用户与账号", Code: "account", I18nKey: stringPointer("navigation.account"), Icon: stringPointer("lucide:users-round"), SortOrder: 100, IsEnabled: yesno.Yes, IsHidden: yesno.No},
		{ParentCode: "account", MenuType: TypePage, Name: "用户管理", Code: "account:user:list", I18nKey: stringPointer("navigation.accountUsers"), Path: stringPointer("/account/users"), ComponentPath: stringPointer("account/users"), Icon: stringPointer("lucide:user-round-cog"), SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.No},
	}
}
