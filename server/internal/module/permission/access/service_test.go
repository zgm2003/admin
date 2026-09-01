package permission

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/permission/state"
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
	if !reflect.DeepEqual(repository.platformIDs, []int64{identity.PlatformID}) {
		t.Fatalf("permission source platform IDs = %v", repository.platformIDs)
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

func TestBuildSnapshotKeepsPageAndReadPermissionsIndependent(t *testing.T) {
	rootID := int64(1)
	pageID := int64(2)
	path := "/account/users"
	componentPath := "account/users"
	snapshot, err := buildSnapshot(Source{
		Version: 1,
		Menus: []SourceMenu{
			{ID: rootID, MenuType: MenuDirectory, Code: "account", I18nKey: accessStringPointer("navigation.account"), IsEnabled: yesno.Yes, IsHidden: yesno.No},
			{ID: pageID, ParentID: &rootID, MenuType: MenuPage, Code: "account:user:view", I18nKey: accessStringPointer("navigation.accountUsers"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No},
		},
		GrantedMenuIDs: []int64{pageID},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"account:user:view"}
	if !reflect.DeepEqual(snapshot.PermissionCodes, want) {
		t.Fatalf("permission codes = %v, want %v", snapshot.PermissionCodes, want)
	}
}

func TestLoadSnapshotUsesRedisStateGateBeforeLocalCache(t *testing.T) {
	serviceRedis := openAccessRedis(t)
	cleanupRedis := openAccessRedis(t)
	cleanupAccessKeys(t, cleanupRedis, 93006, "admin", 4, 3)
	repository := &countingSourceStore{sources: []Source{baseSource(3), baseSource(3)}}
	service := NewService(
		repository,
		permissionstate.NewStore(serviceRedis),
		NewSnapshotCache(serviceRedis),
		NewLocalSnapshotCache(8),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	identity := accessIdentity(93006)

	if _, err := service.Current(context.Background(), identity); err != nil {
		t.Fatal(err)
	}
	if err := cleanupRedis.Delete(context.Background(), SnapshotKey("admin", 4, 93006, 3)); err != nil {
		t.Fatal(err)
	}
	warm, err := service.Current(context.Background(), identity)
	if err != nil || warm.CacheResult != "hit" || repository.calls != 1 {
		t.Fatalf("L1 read = %+v,%v calls=%d", warm, err, repository.calls)
	}

	if err := serviceRedis.Close(); err != nil {
		t.Fatal(err)
	}
	fresh, err := service.Current(context.Background(), identity)
	if err != nil || fresh.CacheResult != "error" || repository.calls != 2 {
		t.Fatalf("Redis gate failure = %+v,%v calls=%d", fresh, err, repository.calls)
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
	service.local = NewLocalSnapshotCache(8)
	snapshot, err := service.Current(context.Background(), identity)
	if err != nil || snapshot.CacheResult != "error" || repository.calls != 2 {
		t.Fatalf("corrupt fallback = %+v,%v calls=%d", snapshot, err, repository.calls)
	}
	warm, err := service.Current(context.Background(), identity)
	if err != nil || warm.CacheResult != "hit" || repository.calls != 2 {
		t.Fatalf("rebuilt snapshot = %+v,%v calls=%d", warm, err, repository.calls)
	}
}

func TestLoadSnapshotBypassesLocalCacheForMissingOrCorruptState(t *testing.T) {
	for _, test := range []struct {
		name        string
		userID      int64
		mutateState func(*testing.T, *projectredis.Client, int64)
		cacheResult string
	}{
		{
			name:   "missing",
			userID: 93007,
			mutateState: func(t *testing.T, client *projectredis.Client, userID int64) {
				t.Helper()
				if err := client.Delete(context.Background(), permissionstate.StateKey(userID)); err != nil {
					t.Fatal(err)
				}
			},
			cacheResult: "miss",
		},
		{
			name:   "corrupt",
			userID: 93008,
			mutateState: func(t *testing.T, client *projectredis.Client, userID int64) {
				t.Helper()
				if err := client.SetString(context.Background(), permissionstate.StateKey(userID), `{"state":"ready","unknown":true}`, time.Minute); err != nil {
					t.Fatal(err)
				}
			},
			cacheResult: "error",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			redisClient := openAccessRedis(t)
			cleanupAccessKeys(t, redisClient, test.userID, "admin", 4, 3)
			repository := &countingSourceStore{sources: []Source{baseSource(3), baseSource(3)}}
			service := newAccessTestService(redisClient, repository)
			identity := accessIdentity(test.userID)
			if _, err := service.Current(context.Background(), identity); err != nil {
				t.Fatal(err)
			}
			if err := redisClient.Delete(context.Background(), SnapshotKey("admin", 4, test.userID, 3)); err != nil {
				t.Fatal(err)
			}
			test.mutateState(t, redisClient, test.userID)

			snapshot, err := service.Current(context.Background(), identity)
			if err != nil || snapshot.CacheResult != test.cacheResult || repository.calls != 2 {
				t.Fatalf("state %s fallback = %+v,%v calls=%d", test.name, snapshot, err, repository.calls)
			}
		})
	}
}

func TestLoadSnapshotBypassesLocalCacheAfterAccessVersionChanges(t *testing.T) {
	redisClient := openAccessRedis(t)
	cleanupAccessKeys(t, redisClient, 93009, "admin", 4, 3)
	t.Cleanup(func() { _ = redisClient.Delete(context.Background(), SnapshotKey("admin", 4, 93009, 4)) })
	repository := &countingSourceStore{sources: []Source{baseSource(3), baseSource(4)}}
	service := newAccessTestService(redisClient, repository)
	identity := accessIdentity(93009)
	if _, err := service.Current(context.Background(), identity); err != nil {
		t.Fatal(err)
	}
	stateStore := permissionstate.NewStore(redisClient)
	lease, err := permissionstate.NewInvalidator(stateStore).Acquire(context.Background(), []permissionstate.Version{{UserID: identity.UserID, Version: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Commit(context.Background(), map[int64]int64{identity.UserID: 4}); err != nil {
		t.Fatal(err)
	}

	snapshot, err := service.Current(context.Background(), identity)
	if err != nil || snapshot.Version != 4 || repository.calls != 2 {
		t.Fatalf("version change = %+v,%v calls=%d", snapshot, err, repository.calls)
	}
}

func TestLoadSnapshotDoesNotUseLocalCacheWhileInvalidating(t *testing.T) {
	redisClient := openAccessRedis(t)
	cleanupAccessKeys(t, redisClient, 93010, "admin", 4, 3)
	repository := &countingSourceStore{sources: []Source{baseSource(3)}}
	service := newAccessTestService(redisClient, repository)
	identity := accessIdentity(93010)
	if _, err := service.Current(context.Background(), identity); err != nil {
		t.Fatal(err)
	}
	lease, err := permissionstate.NewInvalidator(permissionstate.NewStore(redisClient)).Acquire(
		context.Background(),
		[]permissionstate.Version{{UserID: identity.UserID, Version: 3}},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Rollback(context.Background()) })

	if _, err := service.Current(context.Background(), identity); appErrorCode(err) != CodeAccessUpdating {
		t.Fatalf("invalidating access error = %v", err)
	}
	if repository.calls != 1 {
		t.Fatalf("invalidating state PostgreSQL calls = %d, want 1", repository.calls)
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

	store := permissionstate.NewStore(redisClient)
	_, _, _ = store.InstallReadyIfMissing(context.Background(), permissionstate.Version{UserID: 93004, Version: 3})
	lease, err := permissionstate.NewInvalidator(store).Acquire(context.Background(), []permissionstate.Version{{UserID: 93004, Version: 3}})
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
	stateStore := permissionstate.NewStore(redisClient)
	_, _, _ = stateStore.InstallReadyIfMissing(context.Background(), permissionstate.Version{UserID: 93005, Version: 3})
	repository := &countingSourceStore{sources: []Source{baseSource(3), baseSource(4)}}
	repository.afterFind = func(call int) {
		if call != 1 {
			return
		}
		lease, err := permissionstate.NewInvalidator(stateStore).Acquire(context.Background(), []permissionstate.Version{{UserID: 93005, Version: 3}})
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
	if _, found, err := service.cache.Read(context.Background(), 1, "admin", 4, 93005, 3); err != nil || found {
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

func TestBuildSnapshotSupportsCanvasRootPageAndAction(t *testing.T) {
	pageID := int64(41)
	path, componentPath := "/test", "test"
	snapshot, err := buildSnapshot(Source{
		Version:   1,
		RoleCodes: []string{"canvas_user"},
		Menus: []SourceMenu{
			{ID: pageID, MenuType: MenuPage, Code: "canvas:test:list", I18nKey: accessStringPointer("navigation.system"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No},
			{ID: 42, ParentID: &pageID, MenuType: MenuAction, Code: "canvas:test:button", IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
		},
		GrantedMenuIDs: []int64{42},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.MenuTree) != 1 || snapshot.MenuTree[0].Code != "canvas:test:list" || snapshot.MenuTree[0].MenuType != MenuPage ||
		!reflect.DeepEqual(snapshot.PermissionCodes, []string{"canvas:test:button", "canvas:test:list"}) {
		t.Fatalf("Canvas snapshot = %+v", snapshot)
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
	missingPlatformID := accessIdentity(1)
	missingPlatformID.PlatformID = 0
	if _, err := service.Current(context.Background(), missingPlatformID); appErrorCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("missing platform id error = %v", err)
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
	return auth.Identity{UserID: userID, SessionID: userID + 1000, PlatformID: 1, Platform: "admin", Version: 1, PolicyVersion: 4, AccessCacheTTL: time.Hour}
}

func newAccessTestService(redisClient *projectredis.Client, repository sourceStore) *Service {
	stateStore := permissionstate.NewStore(redisClient)
	return NewService(repository, stateStore, NewSnapshotCache(redisClient), NewLocalSnapshotCache(8), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

type countingSourceStore struct {
	sources     []Source
	err         error
	calls       int
	platformIDs []int64
	afterFind   func(int)
}

func (s *countingSourceStore) FindSourceWithVersion(_ context.Context, _ int64, platformID int64) (Source, error) {
	s.calls++
	s.platformIDs = append(s.platformIDs, platformID)
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
