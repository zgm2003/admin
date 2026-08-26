package access

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/module/accessstate"
	"admin/server/internal/module/auth"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

func TestCurrentAndAllowedShareWarmSnapshotWithoutPostgreSQL(t *testing.T) {
	redisClient := openAccessRedis(t)
	cleanupAccessKeys(t, redisClient, 93001, "admin", 4, 3)
	repository := &countingSourceStore{sources: []Source{baseSource(3)}}
	service := newAccessTestService(redisClient, repository)
	identity := accessIdentity(93001)

	snapshot, err := service.Current(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 || snapshot.CacheResult != "miss" || snapshot.Version != 3 {
		t.Fatalf("first Current() = %+v calls=%d", snapshot, repository.calls)
	}
	allowed, err := service.Allowed(context.Background(), identity, "account:user:create")
	if err != nil || !allowed {
		t.Fatalf("Allowed(create) = %v,%v", allowed, err)
	}
	denied, err := service.Allowed(context.Background(), identity, "account:user:delete")
	if err != nil || denied {
		t.Fatalf("Allowed(delete) = %v,%v", denied, err)
	}
	if repository.calls != 1 {
		t.Fatalf("warm Current/Allowed PostgreSQL calls = %d", repository.calls)
	}
}

func TestLoadSnapshotFallsBackForMissErrorAndCorruption(t *testing.T) {
	redisClient := openAccessRedis(t)
	cleanupAccessKeys(t, redisClient, 93002, "admin", 4, 3)
	repository := &countingSourceStore{sources: []Source{baseSource(3), baseSource(3)}}
	service := newAccessTestService(redisClient, repository)
	identity := accessIdentity(93002)

	if _, err := service.Current(context.Background(), identity); err != nil {
		t.Fatal(err)
	}
	key := SnapshotKey("admin", 4, 93002, 3)
	if err := redisClient.SetString(context.Background(), key, `{"schemaVersion":1,"unknown":true}`, time.Minute); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Current(context.Background(), identity)
	if err != nil || snapshot.CacheResult != "error" || repository.calls != 2 {
		t.Fatalf("corrupt fallback = %+v,%v calls=%d", snapshot, err, repository.calls)
	}
	warm, err := service.Current(context.Background(), identity)
	if err != nil || warm.CacheResult != "hit" || repository.calls != 2 {
		t.Fatalf("rebuilt snapshot = %+v,%v calls=%d", warm, err, repository.calls)
	}
}

func TestLoadSnapshotReturnsPostgreSQLAuthorityWhenRedisFails(t *testing.T) {
	redisClient := openAccessRedis(t)
	repository := &countingSourceStore{sources: []Source{baseSource(3)}}
	service := newAccessTestService(redisClient, repository)
	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Current(context.Background(), accessIdentity(93003))
	if err != nil || snapshot.CacheResult != "error" || repository.calls != 1 {
		t.Fatalf("Redis failure fallback = %+v,%v calls=%d", snapshot, err, repository.calls)
	}
}

func TestLoadSnapshotFailsClosedForPostgreSQLErrorAndInvalidatingState(t *testing.T) {
	redisClient := openAccessRedis(t)
	cleanupAccessKeys(t, redisClient, 93004, "admin", 4, 3)
	repository := &countingSourceStore{err: errors.New("postgres down")}
	service := newAccessTestService(redisClient, repository)
	if _, err := service.Current(context.Background(), accessIdentity(93004)); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("PostgreSQL failure = %v", err)
	}

	store := accessstate.NewStore(redisClient)
	_, _, _ = store.InstallReadyIfMissing(context.Background(), accessstate.Version{UserID: 93004, Version: 3})
	lease, err := accessstate.NewInvalidator(store).Acquire(context.Background(), []accessstate.Version{{UserID: 93004, Version: 3}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Rollback(context.Background()) })
	repository.calls = 0
	if _, err := service.Current(context.Background(), accessIdentity(93004)); appErrorCode(err) != CodeAccessUpdating {
		t.Fatalf("invalidating access error = %v", err)
	}
	if repository.calls != 0 {
		t.Fatal("invalidating access state queried PostgreSQL")
	}
}

func TestReadThroughRechecksVersionBeforePublishing(t *testing.T) {
	redisClient := openAccessRedis(t)
	cleanupAccessKeys(t, redisClient, 93005, "admin", 4, 3)
	stateStore := accessstate.NewStore(redisClient)
	_, _, _ = stateStore.InstallReadyIfMissing(context.Background(), accessstate.Version{UserID: 93005, Version: 3})
	repository := &countingSourceStore{sources: []Source{baseSource(3), baseSource(4)}}
	repository.afterFind = func(call int) {
		if call != 1 {
			return
		}
		lease, err := accessstate.NewInvalidator(stateStore).Acquire(context.Background(), []accessstate.Version{{UserID: 93005, Version: 3}})
		if err != nil {
			t.Fatal(err)
		}
		if err := lease.Commit(context.Background(), map[int64]int64{93005: 4}); err != nil {
			t.Fatal(err)
		}
	}
	service := newAccessTestService(redisClient, repository)
	snapshot, err := service.Current(context.Background(), accessIdentity(93005))
	if err != nil || snapshot.Version != 4 || repository.calls != 2 {
		t.Fatalf("version recheck = %+v,%v calls=%d", snapshot, err, repository.calls)
	}
	if _, found, err := service.cache.Read(context.Background(), "admin", 4, 93005, 3); err != nil || found {
		t.Fatalf("stale version snapshot published = %v,%v", found, err)
	}
}

func TestBuildSnapshotPreservesLeafGrantSemanticsAndStableArrays(t *testing.T) {
	snapshot, err := buildSnapshot(baseSource(3))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot.RoleCodes, []string{"ai_tester", "registered_user"}) {
		t.Fatalf("role codes = %v", snapshot.RoleCodes)
	}
	if !reflect.DeepEqual(snapshot.PermissionCodes, []string{"account:user:create", "account:user:list"}) {
		t.Fatalf("permission codes = %v", snapshot.PermissionCodes)
	}
	if len(snapshot.MenuTree) != 1 || len(snapshot.MenuTree[0].Children) != 1 {
		t.Fatalf("menu tree = %+v", snapshot.MenuTree)
	}

	pageOnly := baseSource(3)
	pageOnly.GrantedMenuIDs = []int64{2}
	snapshot, err = buildSnapshot(pageOnly)
	if err != nil || !reflect.DeepEqual(snapshot.PermissionCodes, []string{"account:user:list"}) {
		t.Fatalf("page-only snapshot = %+v,%v", snapshot, err)
	}

	superAdmin := baseSource(3)
	superAdmin.SuperAdmin = true
	superAdmin.GrantedMenuIDs = nil
	snapshot, err = buildSnapshot(superAdmin)
	if err != nil || !reflect.DeepEqual(snapshot.PermissionCodes, []string{
		"account:user:create", "account:user:delete", "account:user:list",
	}) {
		t.Fatalf("super-admin snapshot = %+v,%v", snapshot, err)
	}
}

func TestBuildSnapshotDoesNotInventMenuPermissionsForSuperAdminWithoutMenuRows(t *testing.T) {
	snapshot, err := buildSnapshot(Source{
		Version: 1, RoleCodes: []string{"super_admin"}, SuperAdmin: true, Menus: []SourceMenu{},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{}
	if !reflect.DeepEqual(snapshot.PermissionCodes, want) {
		t.Fatalf("permissions = %v, want %v", snapshot.PermissionCodes, want)
	}
}

func TestBuildSnapshotKeepsHiddenMenusAndExcludesActionsFromTree(t *testing.T) {
	source := baseSource(1)
	source.Menus[0].IsHidden = yesno.Yes
	source.Menus[1].IsHidden = yesno.Yes
	snapshot, err := buildSnapshot(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.MenuTree) != 1 || snapshot.MenuTree[0].IsHidden != int16(yesno.Yes) ||
		len(snapshot.MenuTree[0].Children) != 1 || snapshot.MenuTree[0].Children[0].IsHidden != int16(yesno.Yes) {
		t.Fatalf("hidden menu tree = %+v", snapshot.MenuTree)
	}
	if len(snapshot.MenuTree[0].Children[0].Children) != 0 ||
		!reflect.DeepEqual(snapshot.PermissionCodes, []string{"account:user:create", "account:user:list"}) {
		t.Fatalf("action tree/permissions = %+v / %v", snapshot.MenuTree, snapshot.PermissionCodes)
	}
}

func TestBuildSnapshotAllowsSharedComponentPathButRejectsSharedRoutePath(t *testing.T) {
	source := baseSource(1)
	secondPath := "/account/accounts"
	sharedComponentPath := "account/users"
	source.Menus = append(source.Menus, SourceMenu{
		ID: 5, ParentID: int64Pointer(1), MenuType: MenuPage, Code: "account:account:list",
		I18nKey: accessStringPointer("navigation.accountAccounts"), Path: &secondPath, ComponentPath: &sharedComponentPath,
		SortOrder: 20, IsEnabled: yesno.Yes, IsHidden: yesno.No,
	})
	source.GrantedMenuIDs = []int64{2, 5}
	if _, err := buildSnapshot(source); err != nil {
		t.Fatalf("shared component path was rejected: %v", err)
	}
	source.Menus[len(source.Menus)-1].Path = source.Menus[1].Path
	if _, err := buildSnapshot(source); err == nil {
		t.Fatal("shared route path was accepted")
	}
}

func TestBuildSnapshotDoesNotGrantStaticMenuPermissionsToOrdinaryRoles(t *testing.T) {
	snapshot, err := buildSnapshot(Source{
		Version: 1, RoleCodes: []string{"registered_user"}, Menus: []SourceMenu{}, GrantedMenuIDs: []int64{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.PermissionCodes) != 0 {
		t.Fatalf("ordinary permissions = %v", snapshot.PermissionCodes)
	}
}

func TestServiceValidatesIdentityAndPermissionCode(t *testing.T) {
	redisClient := openAccessRedis(t)
	service := newAccessTestService(redisClient, &countingSourceStore{})
	if _, err := service.Current(context.Background(), auth.Identity{}); appErrorCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("invalid identity error = %v", err)
	}
	if _, err := service.Allowed(context.Background(), accessIdentity(1), " "); appErrorCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("empty permission error = %v", err)
	}
}

func baseSource(version int64) Source {
	rootID := int64(1)
	pageID := int64(2)
	path := "/account/users"
	componentPath := "account/users"
	return Source{
		Version: version, RoleCodes: []string{"registered_user", "ai_tester", "registered_user"},
		Menus: []SourceMenu{
			{ID: rootID, MenuType: MenuDirectory, Code: "account", I18nKey: accessStringPointer("navigation.account"), SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.No},
			{ID: pageID, ParentID: &rootID, MenuType: MenuPage, Code: "account:user:list", I18nKey: accessStringPointer("navigation.accountUsers"), Path: &path, ComponentPath: &componentPath, SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.No},
			{ID: 3, ParentID: &pageID, MenuType: MenuAction, Code: "account:user:create", I18nKey: nil, SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
			{ID: 4, ParentID: &pageID, MenuType: MenuAction, Code: "account:user:delete", I18nKey: nil, SortOrder: 20, IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
		},
		GrantedMenuIDs: []int64{3, 3},
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}

func accessStringPointer(value string) *string {
	return &value
}

func accessIdentity(userID int64) auth.Identity {
	return auth.Identity{UserID: userID, SessionID: userID + 1000, Platform: "admin", Version: 1, PolicyVersion: 4, AccessCacheTTL: time.Hour}
}

func newAccessTestService(redisClient *projectredis.Client, repository sourceStore) *Service {
	stateStore := accessstate.NewStore(redisClient)
	return NewService(repository, stateStore, NewSnapshotCache(redisClient), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

type countingSourceStore struct {
	sources   []Source
	err       error
	calls     int
	afterFind func(int)
}

func (s *countingSourceStore) FindSourceWithVersion(context.Context, int64) (Source, error) {
	s.calls++
	if s.err != nil {
		return Source{}, s.err
	}
	if len(s.sources) == 0 {
		return Source{}, errors.New("unexpected PostgreSQL source call")
	}
	index := s.calls - 1
	if index >= len(s.sources) {
		index = len(s.sources) - 1
	}
	result := s.sources[index]
	if s.afterFind != nil {
		s.afterFind(s.calls)
	}
	return result, nil
}

func appErrorCode(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}
