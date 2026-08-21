package menu

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/module/accessstate"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestRepositoryFindActiveMenusIncludesDisabledExcludesDeletedAndSorts(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	repository := NewRepository(tx)
	prefix := fmt.Sprintf("repository:%d", time.Now().UnixNano())
	items := []Menu{
		{MenuType: TypeDirectory, Code: prefix + ":z", I18nKey: "navigation.system", SortOrder: 20, IsEnabled: yesno.Yes},
		{MenuType: TypeDirectory, Code: prefix + ":b", I18nKey: "navigation.system", SortOrder: 10, IsEnabled: yesno.No},
		{MenuType: TypeDirectory, Code: prefix + ":a", I18nKey: "navigation.system", SortOrder: 10, IsEnabled: yesno.Yes},
		{MenuType: TypeDirectory, Code: prefix + ":deleted", I18nKey: "navigation.system", SortOrder: 0, IsEnabled: yesno.Yes},
	}
	for index := range items {
		if err := repository.Create(ctx, &items[index]); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.WithContext(ctx).Delete(&items[3]).Error; err != nil {
		t.Fatal(err)
	}

	rows, err := repository.FindActiveMenus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	filtered := filterMenusByPrefix(rows, prefix)
	if len(filtered) != 3 {
		t.Fatalf("active rows = %+v, want three", filtered)
	}
	wantCodes := []string{prefix + ":a", prefix + ":b", prefix + ":z"}
	for index, wantCode := range wantCodes {
		if filtered[index].Code != wantCode {
			t.Fatalf("sorted code[%d] = %q, want %q", index, filtered[index].Code, wantCode)
		}
	}
	if filtered[1].IsEnabled != yesno.No {
		t.Fatal("disabled active menu was not returned")
	}
}

func TestRepositoryLockActiveMenusRunsInsideTransaction(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	repository := NewRepository(tx)
	code := fmt.Sprintf("repository:lock:%d", time.Now().UnixNano())
	created := Menu{MenuType: TypeDirectory, Code: code, I18nKey: "navigation.system", SortOrder: 1, IsEnabled: yesno.Yes}
	if err := repository.Create(ctx, &created); err != nil {
		t.Fatal(err)
	}

	called := false
	if err := repository.Transaction(ctx, func(locked *Repository) error {
		called = true
		rows, err := locked.LockActiveMenus(ctx)
		if err != nil {
			return err
		}
		if !containsMenu(rows, created.ID) {
			return fmt.Errorf("locked rows do not contain menu %d", created.ID)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("transaction callback was not called")
	}
}

func TestRepositoryCreateWritesNullableFieldsAndTimestamps(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	repository := NewRepository(tx)
	unique := time.Now().UnixNano()
	root := Menu{MenuType: TypeDirectory, Code: fmt.Sprintf("repository:create:%d", unique), I18nKey: "navigation.system", SortOrder: 1, IsEnabled: yesno.Yes}
	if err := repository.Create(ctx, &root); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/repository-create-%d", unique)
	viewKey := "system-menus"
	icon := "Menu"
	page := Menu{
		ParentID: &root.ID, MenuType: TypePage, Code: fmt.Sprintf("repository:create:%d:list", unique),
		I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &viewKey, Icon: &icon,
		SortOrder: 2, IsEnabled: yesno.No,
	}
	if err := repository.Create(ctx, &page); err != nil {
		t.Fatal(err)
	}

	var stored Menu
	if err := tx.WithContext(ctx).First(&stored, page.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ParentID == nil || *stored.ParentID != root.ID || value(stored.Path) != path ||
		value(stored.ViewKey) != viewKey || value(stored.Icon) != icon || stored.IsEnabled != yesno.No ||
		stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() || stored.DeletedAt.Valid {
		t.Fatalf("stored page = %+v", stored)
	}
}

func TestRepositoryUpdateMenuWritesExplicitSQLNulls(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	repository := NewRepository(tx)
	unique := time.Now().UnixNano()
	root := createRepositoryDirectory(t, repository, ctx, fmt.Sprintf("repository:update:%d", unique), 1)
	path := fmt.Sprintf("/repository-update-%d", unique)
	viewKey := "system-menus"
	icon := "Menu"
	page := Menu{
		ParentID: &root.ID, MenuType: TypePage, Code: fmt.Sprintf("repository:update:%d:list", unique),
		I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &viewKey, Icon: &icon,
		SortOrder: 2, IsEnabled: yesno.Yes,
	}
	if err := repository.Create(ctx, &page); err != nil {
		t.Fatal(err)
	}
	updatedAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := repository.UpdateMenu(ctx, page.ID, UpdateValues{
		ParentID: nil, MenuType: TypeDirectory, I18nKey: "navigation.system",
		Path: nil, ViewKey: nil, Icon: nil, SortOrder: 9,
	}, updatedAt); err != nil {
		t.Fatal(err)
	}

	var stored Menu
	if err := tx.WithContext(ctx).First(&stored, page.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ParentID != nil || stored.Path != nil || stored.ViewKey != nil || stored.Icon != nil ||
		stored.MenuType != TypeDirectory || stored.I18nKey != "navigation.system" || stored.SortOrder != 9 ||
		!stored.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("updated row = %+v", stored)
	}
}

func TestRepositoryBulkStatusAndMenuSoftDeleteUseSuppliedTimestamps(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	repository := NewRepository(tx)
	unique := time.Now().UnixNano()
	first := createRepositoryDirectory(t, repository, ctx, fmt.Sprintf("repository:bulk:%d:a", unique), 1)
	second := createRepositoryDirectory(t, repository, ctx, fmt.Sprintf("repository:bulk:%d:b", unique), 2)
	statusAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := repository.UpdateMenuStatus(ctx, []int64{first.ID, second.ID}, yesno.No, statusAt); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{first.ID, second.ID} {
		var stored Menu
		if err := tx.WithContext(ctx).First(&stored, id).Error; err != nil {
			t.Fatal(err)
		}
		if stored.IsEnabled != yesno.No || !stored.UpdatedAt.Equal(statusAt) {
			t.Fatalf("status row %d = %+v", id, stored)
		}
	}

	deletedAt := statusAt.Add(time.Minute)
	if err := repository.SoftDeleteMenus(ctx, []int64{first.ID, second.ID}, deletedAt); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{first.ID, second.ID} {
		var stored Menu
		if err := tx.WithContext(ctx).Unscoped().First(&stored, id).Error; err != nil {
			t.Fatal(err)
		}
		if !stored.DeletedAt.Valid || !stored.DeletedAt.Time.Equal(deletedAt) || !stored.UpdatedAt.Equal(deletedAt) {
			t.Fatalf("deleted row %d = %+v", id, stored)
		}
	}
}

func TestRepositoryRoleMenuSoftDeleteTouchesOnlyActiveTargetLinks(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	repository := NewRepository(tx)
	unique := time.Now().UnixNano()
	createdRole := testRole{
		Code: fmt.Sprintf("repository_role_%d", unique), Name: "Repository Role",
		IsDefault: yesno.No, IsEnabled: yesno.Yes,
	}
	if err := tx.WithContext(ctx).Create(&createdRole).Error; err != nil {
		t.Fatal(err)
	}
	first := createRepositoryDirectory(t, repository, ctx, fmt.Sprintf("repository:grant:%d:a", unique), 1)
	second := createRepositoryDirectory(t, repository, ctx, fmt.Sprintf("repository:grant:%d:b", unique), 2)
	unrelated := createRepositoryDirectory(t, repository, ctx, fmt.Sprintf("repository:grant:%d:c", unique), 3)
	links := []RoleMenu{
		{RoleID: createdRole.ID, MenuID: first.ID},
		{RoleID: createdRole.ID, MenuID: second.ID},
		{RoleID: createdRole.ID, MenuID: unrelated.ID},
	}
	for index := range links {
		if err := tx.WithContext(ctx).Create(&links[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	oldDeletedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	history := RoleMenu{
		RoleID: createdRole.ID, MenuID: second.ID,
		DeletedAt: gorm.DeletedAt{Time: oldDeletedAt, Valid: true}, UpdatedAt: oldDeletedAt,
	}
	if err := tx.WithContext(ctx).Unscoped().Create(&history).Error; err != nil {
		t.Fatal(err)
	}

	hasGrant, err := repository.HasActiveDirectGrant(ctx, first.ID)
	if err != nil || !hasGrant {
		t.Fatalf("HasActiveDirectGrant() = %v,%v", hasGrant, err)
	}
	deletedAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := repository.SoftDeleteRoleMenus(ctx, []int64{first.ID, second.ID}, deletedAt); err != nil {
		t.Fatal(err)
	}

	var stored []RoleMenu
	if err := tx.WithContext(ctx).Unscoped().Where("role_id = ?", createdRole.ID).Order("id").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	byID := make(map[int64]RoleMenu, len(stored))
	for _, item := range stored {
		byID[item.ID] = item
	}
	for _, link := range links[:2] {
		item := byID[link.ID]
		if !item.DeletedAt.Valid || !item.DeletedAt.Time.Equal(deletedAt) || !item.UpdatedAt.Equal(deletedAt) {
			t.Errorf("target link %d = %+v", link.ID, item)
		}
	}
	if byID[links[2].ID].DeletedAt.Valid {
		t.Fatal("unrelated active role-menu link was deleted")
	}
	if item := byID[history.ID]; !item.DeletedAt.Valid || !item.DeletedAt.Time.Equal(oldDeletedAt) {
		t.Fatalf("existing deleted history was changed: %+v", item)
	}
	hasGrant, err = repository.HasActiveDirectGrant(ctx, first.ID)
	if err != nil || hasGrant {
		t.Fatalf("grant after deletion = %v,%v", hasGrant, err)
	}
}

func TestRepositoryGlobalAccessVersionsAreSortedLockedAndAdvanced(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	repository := NewRepository(tx)
	first := createMenuAccessUser(t, tx, ctx, yesno.Yes, false)
	second := createMenuAccessUser(t, tx, ctx, yesno.Yes, false)
	_ = createMenuAccessUser(t, tx, ctx, yesno.No, false)
	_ = createMenuAccessUser(t, tx, ctx, yesno.Yes, true)
	want := []accessstate.Version{{UserID: first.ID, Version: 1}, {UserID: second.ID, Version: 1}}
	candidates, err := repository.FindActiveAccessVersions(ctx)
	if err != nil || !reflect.DeepEqual(candidates, want) {
		t.Fatalf("FindActiveAccessVersions() = %+v,%v", candidates, err)
	}
	if err := repository.LockUserMutationTables(ctx); err != nil {
		t.Fatal(err)
	}
	locked, err := repository.LockActiveAccessVersions(ctx)
	if err != nil || !reflect.DeepEqual(locked, want) {
		t.Fatalf("LockActiveAccessVersions() = %+v,%v", locked, err)
	}
	advanced, err := repository.IncrementAccessVersions(ctx, []int64{second.ID, first.ID, first.ID}, time.Now().UTC().Truncate(time.Microsecond))
	if err != nil || !reflect.DeepEqual(advanced, map[int64]int64{first.ID: 2, second.ID: 2}) {
		t.Fatalf("IncrementAccessVersions() = %+v,%v", advanced, err)
	}
}

func TestRepositoryGlobalAccessVersionsRejectMissingVersion(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	unique := time.Now().UnixNano()
	created := testUser{Username: fmt.Sprintf("missing_%d", unique), Email: fmt.Sprintf("missing_%d@example.com", unique), IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&created).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewRepository(tx).FindActiveAccessVersions(ctx); err == nil {
		t.Fatal("active user without access version was accepted")
	}
}

func TestRepositoryGlobalMenuLockBlocksUserWrites(t *testing.T) {
	db, ctx := openMenuDatabase(t)
	menuTx := db.WithContext(ctx).Begin()
	if menuTx.Error != nil {
		t.Fatal(menuTx.Error)
	}
	t.Cleanup(func() { _ = menuTx.Rollback().Error })
	if err := NewRepository(menuTx).LockUserMutationTables(ctx); err != nil {
		t.Fatal(err)
	}
	userTx := db.WithContext(ctx).Begin()
	if userTx.Error != nil {
		t.Fatal(userTx.Error)
	}
	t.Cleanup(func() { _ = userTx.Rollback().Error })
	waitCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	if err := userTx.WithContext(waitCtx).Exec("LOCK TABLE sys_user IN ROW EXCLUSIVE MODE").Error; err == nil {
		t.Fatal("user write table lock bypassed global menu mutation lock")
	}
}

func TestRepositoryConvertsActiveUniqueViolations(t *testing.T) {
	t.Run("code on create", func(t *testing.T) {
		tx, ctx := openMenuTransaction(t)
		repository := NewRepository(tx)
		code := fmt.Sprintf("repository:conflict:%d", time.Now().UnixNano())
		first := Menu{MenuType: TypeDirectory, Code: code, I18nKey: "navigation.system", IsEnabled: yesno.Yes}
		second := first
		if err := repository.Create(ctx, &first); err != nil {
			t.Fatal(err)
		}
		if err := repository.Create(ctx, &second); !errors.Is(err, ErrMenuCodeConflict) {
			t.Fatalf("duplicate code error = %v", err)
		}
	})

	t.Run("page path on create", func(t *testing.T) {
		tx, ctx := openMenuTransaction(t)
		repository := NewRepository(tx)
		unique := time.Now().UnixNano()
		root := createRepositoryDirectory(t, repository, ctx, fmt.Sprintf("repository:path:%d", unique), 1)
		path := fmt.Sprintf("/repository-path-%d", unique)
		view := "system-menus"
		first := Menu{ParentID: &root.ID, MenuType: TypePage, Code: fmt.Sprintf("repository:path:%d:a", unique), I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &view, IsEnabled: yesno.Yes}
		second := first
		second.Code = fmt.Sprintf("repository:path:%d:b", unique)
		if err := repository.Create(ctx, &first); err != nil {
			t.Fatal(err)
		}
		if err := repository.Create(ctx, &second); !errors.Is(err, ErrMenuPathConflict) {
			t.Fatalf("duplicate path error = %v", err)
		}
	})

	t.Run("page path on update", func(t *testing.T) {
		tx, ctx := openMenuTransaction(t)
		repository := NewRepository(tx)
		unique := time.Now().UnixNano()
		root := createRepositoryDirectory(t, repository, ctx, fmt.Sprintf("repository:update-path:%d", unique), 1)
		firstPath := fmt.Sprintf("/repository-update-path-%d-a", unique)
		secondPath := fmt.Sprintf("/repository-update-path-%d-b", unique)
		view := "system-menus"
		first := Menu{ParentID: &root.ID, MenuType: TypePage, Code: fmt.Sprintf("repository:update-path:%d:a", unique), I18nKey: "navigation.systemMenus", Path: &firstPath, ViewKey: &view, IsEnabled: yesno.Yes}
		second := Menu{ParentID: &root.ID, MenuType: TypePage, Code: fmt.Sprintf("repository:update-path:%d:b", unique), I18nKey: "navigation.systemMenus", Path: &secondPath, ViewKey: &view, IsEnabled: yesno.Yes}
		if err := repository.Create(ctx, &first); err != nil {
			t.Fatal(err)
		}
		if err := repository.Create(ctx, &second); err != nil {
			t.Fatal(err)
		}
		err := repository.UpdateMenu(ctx, second.ID, UpdateValues{
			ParentID: &root.ID, MenuType: TypePage, I18nKey: second.I18nKey,
			Path: &firstPath, ViewKey: &view, SortOrder: second.SortOrder,
		}, time.Now().UTC().Truncate(time.Microsecond))
		if !errors.Is(err, ErrMenuPathConflict) {
			t.Fatalf("duplicate update path error = %v", err)
		}
	})
}

func createMenuAccessUser(t *testing.T, tx *gorm.DB, ctx context.Context, enabled yesno.Value, deleted bool) testUser {
	t.Helper()
	unique := time.Now().UnixNano()
	created := testUser{
		Username: fmt.Sprintf("menu_access_%d", unique), Email: fmt.Sprintf("menu_access_%d@example.com", unique),
		IsEnabled: enabled,
	}
	if err := tx.WithContext(ctx).Create(&created).Error; err != nil {
		t.Fatal(err)
	}
	if enabled == yesno.No {
		if err := tx.WithContext(ctx).Model(&testUser{}).Where("id = ?", created.ID).Update("is_enabled", yesno.No).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.WithContext(ctx).Create(&testAccessVersion{UserID: created.ID, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if deleted {
		if err := tx.WithContext(ctx).Delete(&created).Error; err != nil {
			t.Fatal(err)
		}
	}
	return created
}

func createRepositoryDirectory(t *testing.T, repository *Repository, ctx context.Context, code string, sortOrder int) Menu {
	t.Helper()
	item := Menu{MenuType: TypeDirectory, Code: code, I18nKey: "navigation.system", SortOrder: sortOrder, IsEnabled: yesno.Yes}
	if err := repository.Create(ctx, &item); err != nil {
		t.Fatal(err)
	}
	return item
}

func filterMenusByPrefix(rows []Menu, prefix string) []Menu {
	filtered := make([]Menu, 0)
	for _, row := range rows {
		if len(row.Code) >= len(prefix) && row.Code[:len(prefix)] == prefix {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func containsMenu(rows []Menu, id int64) bool {
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}
