package access_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"admin/server/internal/module/access"
	"admin/server/internal/module/menu"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
)

func TestServiceCurrentBuildsActionGrantSnapshot(t *testing.T) {
	store := &serviceStore{source: baseSource()}
	service := access.NewService(store)

	got, err := service.Current(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	want := access.Snapshot{
		RoleCodes: []string{"ai_tester", "registered_user"},
		MenuTree: []access.MenuNode{{
			Code: "system", MenuType: menu.TypeDirectory, TitleKey: "navigation.system",
			Children: []access.MenuNode{{
				Code: "system:user:view", MenuType: menu.TypePage,
				Path: stringPointer("/system/users"), ViewKey: stringPointer("systemUsers"),
				TitleKey: "navigation.systemUsers", Children: []access.MenuNode{},
			}},
		}},
		PermissionCodes: []string{"system:user:create", "system:user:view"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Current() = %#v, want %#v", got, want)
	}
}

func TestServiceCurrentDoesNotExpandPageGrantToActions(t *testing.T) {
	source := baseSource()
	source.GrantedMenuIDs = []int64{2}
	service := access.NewService(&serviceStore{source: source})

	got, err := service.Current(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.PermissionCodes, []string{"system:user:view"}) {
		t.Fatalf("permission codes = %v", got.PermissionCodes)
	}
	if len(got.MenuTree) != 1 || len(got.MenuTree[0].Children) != 1 {
		t.Fatalf("menu tree = %#v", got.MenuTree)
	}
}

func TestServiceCurrentDeduplicatesMultiRoleGrants(t *testing.T) {
	source := baseSource()
	source.GrantedMenuIDs = []int64{3, 3}
	service := access.NewService(&serviceStore{source: source})

	got, err := service.Current(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.PermissionCodes, []string{"system:user:create", "system:user:view"}) {
		t.Fatalf("permission codes = %v", got.PermissionCodes)
	}
}

func TestServiceCurrentIncludesEveryPageAndActionForSuperAdmin(t *testing.T) {
	source := baseSource()
	source.SuperAdmin = true
	source.GrantedMenuIDs = nil
	service := access.NewService(&serviceStore{source: source})

	got, err := service.Current(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"system:user:create", "system:user:delete", "system:user:view"}
	if !reflect.DeepEqual(got.PermissionCodes, want) {
		t.Fatalf("permission codes = %v, want %v", got.PermissionCodes, want)
	}
	if len(got.MenuTree) != 1 || len(got.MenuTree[0].Children) != 1 || len(got.MenuTree[0].Children[0].Children) != 0 {
		t.Fatalf("menu tree = %#v", got.MenuTree)
	}
}

func TestServiceCurrentRejectsCorruptAccessGraphs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*access.Source)
	}{
		{
			name: "missing parent",
			mutate: func(source *access.Source) {
				missing := int64(99)
				source.Menus[1].ParentID = &missing
			},
		},
		{
			name: "parent cycle",
			mutate: func(source *access.Source) {
				pageID := int64(2)
				source.Menus[2].ParentID = &pageID
			},
		},
		{
			name: "directory direct grant",
			mutate: func(source *access.Source) {
				source.GrantedMenuIDs = []int64{1}
			},
		},
		{
			name: "invalid parent type",
			mutate: func(source *access.Source) {
				rootID := int64(1)
				source.Menus[3].ParentID = &rootID
			},
		},
		{
			name: "action with children",
			mutate: func(source *access.Source) {
				createID := int64(3)
				source.Menus = append(source.Menus, menu.Menu{
					ID: 5, ParentID: &createID, MenuType: menu.TypeAction, Code: "system:user:export",
					I18nKey: "permission.userExport", IsEnabled: yesno.Yes,
				})
				source.GrantedMenuIDs = []int64{5}
			},
		},
		{
			name: "duplicate page path",
			mutate: func(source *access.Source) {
				rootID := int64(1)
				source.Menus = append(source.Menus, menu.Menu{
					ID: 5, ParentID: &rootID, MenuType: menu.TypePage, Code: "system:team:view",
					I18nKey: "navigation.systemTeams", Path: stringPointer("/system/users"),
					ViewKey: stringPointer("systemTeams"), IsEnabled: yesno.Yes,
				})
				source.GrantedMenuIDs = []int64{2, 5}
			},
		},
		{
			name: "duplicate selected code",
			mutate: func(source *access.Source) {
				rootID := int64(1)
				source.Menus = append(source.Menus, menu.Menu{
					ID: 5, ParentID: &rootID, MenuType: menu.TypePage, Code: "system:user:view",
					I18nKey: "navigation.systemTeams", Path: stringPointer("/system/teams"),
					ViewKey: stringPointer("systemTeams"), IsEnabled: yesno.Yes,
				})
				source.GrantedMenuIDs = []int64{2, 5}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := baseSource()
			test.mutate(&source)
			service := access.NewService(&serviceStore{source: source})
			_, err := service.Current(context.Background(), 7)
			assertSnapshotInvalid(t, err)
		})
	}
}

func TestServiceValidatesInputsBeforeCallingTheStore(t *testing.T) {
	store := &serviceStore{}
	service := access.NewService(store)

	if _, err := service.Current(context.Background(), 0); appErrorCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("Current(0) error = %v", err)
	}
	if _, err := service.Allowed(context.Background(), 7, " "); appErrorCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("Allowed(empty) error = %v", err)
	}
	if store.findCalls != 0 || store.permissionCalls != 0 {
		t.Fatalf("store calls = find %d permission %d", store.findCalls, store.permissionCalls)
	}
}

func TestServiceMapsStoreFailuresToDependencyUnavailable(t *testing.T) {
	findCause := errors.New("find source failed")
	service := access.NewService(&serviceStore{findErr: findCause})
	if _, err := service.Current(context.Background(), 7); appErrorCode(err) != apperror.CodeDependencyUnavailable || !errors.Is(err, findCause) {
		t.Fatalf("Current() error = %v", err)
	}

	permissionCause := errors.New("permission query failed")
	service = access.NewService(&serviceStore{permissionErr: permissionCause})
	if _, err := service.Allowed(context.Background(), 7, "system:user:view"); appErrorCode(err) != apperror.CodeDependencyUnavailable || !errors.Is(err, permissionCause) {
		t.Fatalf("Allowed() error = %v", err)
	}
}

func TestServiceAllowedPreservesCleanPermissionResult(t *testing.T) {
	store := &serviceStore{allowed: true}
	service := access.NewService(store)
	allowed, err := service.Allowed(context.Background(), 7, "system:user:view")
	if err != nil || !allowed {
		t.Fatalf("Allowed(true) = %v,%v", allowed, err)
	}

	store.allowed = false
	allowed, err = service.Allowed(context.Background(), 7, "system:user:delete")
	if err != nil || allowed {
		t.Fatalf("Allowed(false) = %v,%v", allowed, err)
	}
}

type serviceStore struct {
	source          access.Source
	findErr         error
	allowed         bool
	permissionErr   error
	findCalls       int
	permissionCalls int
}

func (s *serviceStore) FindSource(context.Context, int64) (access.Source, error) {
	s.findCalls++
	return s.source, s.findErr
}

func (s *serviceStore) HasPermission(context.Context, int64, string) (bool, error) {
	s.permissionCalls++
	return s.allowed, s.permissionErr
}

func baseSource() access.Source {
	rootID := int64(1)
	pageID := int64(2)
	return access.Source{
		RoleCodes: []string{"registered_user", "ai_tester"},
		Menus: []menu.Menu{
			{ID: 4, ParentID: &pageID, MenuType: menu.TypeAction, Code: "system:user:delete", I18nKey: "permission.userDelete", SortOrder: 20, IsEnabled: yesno.Yes},
			{ID: 2, ParentID: &rootID, MenuType: menu.TypePage, Code: "system:user:view", I18nKey: "navigation.systemUsers", Path: stringPointer("/system/users"), ViewKey: stringPointer("systemUsers"), SortOrder: 20, IsEnabled: yesno.Yes},
			{ID: 1, MenuType: menu.TypeDirectory, Code: "system", I18nKey: "navigation.system", SortOrder: 10, IsEnabled: yesno.Yes},
			{ID: 3, ParentID: &pageID, MenuType: menu.TypeAction, Code: "system:user:create", I18nKey: "permission.userCreate", SortOrder: 10, IsEnabled: yesno.Yes},
		},
		GrantedMenuIDs: []int64{3},
	}
}

func assertSnapshotInvalid(t *testing.T, err error) {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want apperror.Error", err)
	}
	if appErr.HTTPStatus != http.StatusInternalServerError || appErr.Code != access.CodeAccessSnapshotInvalid || appErr.MessageKey != i18n.KeyAccessSnapshotInvalid || appErr.Cause == nil {
		t.Fatalf("application error = %+v", appErr)
	}
}

func appErrorCode(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

func stringPointer(value string) *string {
	return &value
}
