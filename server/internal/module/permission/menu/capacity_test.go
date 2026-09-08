package menu

import (
	"admin/server/internal/shared/yesno"
	"context"
	"testing"
	"time"
)

func TestMenuMutationDoesNotReadOrLockUserTables(t *testing.T) {
	db, ctx := openMenuDatabase(t)
	user := createMenuAccessUser(t, db, ctx, yesno.Yes, false)
	service, _, _ := newMenuMutationTestService(t, NewRepository(db))
	platformID := testAdminPlatformID(t, db, ctx)
	blocker := db.WithContext(ctx).Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	defer blocker.Rollback()
	if err := blocker.Exec("LOCK TABLE user_account,permission_access_version IN ACCESS EXCLUSIVE MODE").Error; err != nil {
		t.Fatal(err)
	}
	work, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := service.Create(work, CreateInput{PlatformID: platformID, MenuType: TypeDirectory, Name: "No user scan", Code: "capacity", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatalf("menu mutation blocked on user tables: %v", err)
	}
	if err := blocker.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	if v := readMenuAccessVersion(t, db, ctx, user.ID); v != 1 {
		t.Fatalf("user version changed: %d", v)
	}
	if v, err := NewRepository(db).FindMenuVersion(ctx, platformID); err != nil || v != 2 {
		t.Fatalf("catalog version=%d err=%v", v, err)
	}
}
