package menu

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type testRole struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Code      string         `gorm:"column:code;type:varchar(64);not null"`
	Name      string         `gorm:"column:name;type:varchar(64);not null"`
	IsDefault yesno.Value    `gorm:"column:is_default;type:smallint;not null;default:0"`
	IsEnabled yesno.Value    `gorm:"column:is_enabled;type:smallint;not null;default:1"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (testRole) TableName() string { return "sys_role" }

func TestEnsureBuiltinCreatesExactCoreTreeAndIsIdempotent(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	service := NewService(NewRepository(tx))

	if err := service.EnsureBuiltin(ctx); err != nil {
		t.Fatalf("EnsureBuiltin() first call error = %v", err)
	}
	first := loadBuiltinMenus(t, tx, ctx)
	assertExactBuiltinMenus(t, first)

	if err := service.EnsureBuiltin(ctx); err != nil {
		t.Fatalf("EnsureBuiltin() second call error = %v", err)
	}
	second := loadBuiltinMenus(t, tx, ctx)
	if len(second) != len(first) {
		t.Fatalf("second call stored %d core rows, want %d", len(second), len(first))
	}
	for code, firstItem := range first {
		secondItem := second[code]
		if secondItem.ID != firstItem.ID || !secondItem.CreatedAt.Equal(firstItem.CreatedAt) {
			t.Errorf("second call rewrote %s: before=%+v after=%+v", code, firstItem, secondItem)
		}
	}
}

func TestEnsureBuiltinRecreatesOnlyMissingChild(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	service := NewService(NewRepository(tx))
	if err := service.EnsureBuiltin(ctx); err != nil {
		t.Fatal(err)
	}
	before := loadBuiltinMenus(t, tx, ctx)
	missing := before[PermissionCreate]
	if err := tx.WithContext(ctx).Unscoped().Delete(&missing).Error; err != nil {
		t.Fatalf("physically delete core child: %v", err)
	}

	if err := service.EnsureBuiltin(ctx); err != nil {
		t.Fatalf("EnsureBuiltin() after missing child error = %v", err)
	}
	after := loadBuiltinMenus(t, tx, ctx)
	assertExactBuiltinMenus(t, after)
	if after[PermissionCreate].ID == missing.ID {
		t.Fatal("missing child was not recreated as a new row")
	}
	for _, code := range []string{BuiltinSystemCode, PermissionList, PermissionUpdate, PermissionDelete} {
		if after[code].ID != before[code].ID || !after[code].CreatedAt.Equal(before[code].CreatedAt) {
			t.Errorf("recreating one child rewrote existing core menu %s", code)
		}
	}
}

func TestEnsureBuiltinRejectsCorruptCoreRecords(t *testing.T) {
	tests := []struct {
		name       string
		wantDetail string
		mutate     func(*testing.T, *gorm.DB, context.Context, map[string]Menu)
	}{
		{
			name: "type",
			mutate: func(t *testing.T, tx *gorm.DB, ctx context.Context, items map[string]Menu) {
				updateMenuColumns(t, tx, ctx, items[PermissionCreate].ID, map[string]interface{}{
					"menu_type": TypePage, "path": "/corrupt-create", "view_key": "system-menus",
				})
			},
			wantDetail: "menu_type",
		},
		{
			name: "parent",
			mutate: func(t *testing.T, tx *gorm.DB, ctx context.Context, items map[string]Menu) {
				updateMenuColumns(t, tx, ctx, items[PermissionList].ID, map[string]interface{}{"parent_id": nil})
			},
			wantDetail: "parent_id",
		},
		{
			name: "i18n key",
			mutate: func(t *testing.T, tx *gorm.DB, ctx context.Context, items map[string]Menu) {
				updateMenuColumns(t, tx, ctx, items[PermissionUpdate].ID, map[string]interface{}{"i18n_key": "permission.wrong"})
			},
			wantDetail: "i18n_key",
		},
		{
			name: "path",
			mutate: func(t *testing.T, tx *gorm.DB, ctx context.Context, items map[string]Menu) {
				updateMenuColumns(t, tx, ctx, items[PermissionList].ID, map[string]interface{}{"path": "/wrong"})
			},
			wantDetail: "path",
		},
		{
			name: "view key",
			mutate: func(t *testing.T, tx *gorm.DB, ctx context.Context, items map[string]Menu) {
				updateMenuColumns(t, tx, ctx, items[PermissionList].ID, map[string]interface{}{"view_key": "wrong-view"})
			},
			wantDetail: "view_key",
		},
		{
			name: "disabled",
			mutate: func(t *testing.T, tx *gorm.DB, ctx context.Context, items map[string]Menu) {
				updateMenuColumns(t, tx, ctx, items[PermissionDelete].ID, map[string]interface{}{"is_enabled": yesno.No})
			},
			wantDetail: "is_enabled",
		},
		{
			name: "soft-delete history",
			mutate: func(t *testing.T, tx *gorm.DB, ctx context.Context, items map[string]Menu) {
				item := items[PermissionUpdate]
				if err := tx.WithContext(ctx).Delete(&item).Error; err != nil {
					t.Fatalf("soft delete core row: %v", err)
				}
			},
			wantDetail: "deleted_at",
		},
		{
			name: "duplicate history",
			mutate: func(t *testing.T, tx *gorm.DB, ctx context.Context, _ map[string]Menu) {
				now := time.Now().UTC()
				duplicate := Menu{
					MenuType: TypeDirectory, Code: BuiltinSystemCode, I18nKey: "navigation.system",
					SortOrder: 100, IsEnabled: yesno.Yes, DeletedAt: gorm.DeletedAt{Time: now, Valid: true},
				}
				if err := tx.WithContext(ctx).Unscoped().Create(&duplicate).Error; err != nil {
					t.Fatalf("create duplicate history: %v", err)
				}
			},
			wantDetail: "duplicate",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx, ctx := openMenuTransaction(t)
			service := NewService(NewRepository(tx))
			if err := service.EnsureBuiltin(ctx); err != nil {
				t.Fatalf("initial EnsureBuiltin() error = %v", err)
			}
			test.mutate(t, tx, ctx, loadBuiltinMenus(t, tx, ctx))

			err := service.EnsureBuiltin(ctx)
			if err == nil {
				t.Fatal("EnsureBuiltin() accepted corrupt core data")
			}
			if !strings.Contains(err.Error(), test.wantDetail) {
				t.Fatalf("EnsureBuiltin() error = %q, want detail %q", err, test.wantDetail)
			}
		})
	}
}

func TestEnsureBuiltinPreservesOperatorIconAndSortChanges(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	service := NewService(NewRepository(tx))
	if err := service.EnsureBuiltin(ctx); err != nil {
		t.Fatal(err)
	}
	items := loadBuiltinMenus(t, tx, ctx)
	updateMenuColumns(t, tx, ctx, items[BuiltinSystemCode].ID, map[string]interface{}{"icon": "Folder", "sort_order": 77})
	updateMenuColumns(t, tx, ctx, items[PermissionList].ID, map[string]interface{}{"icon": "Cpu", "sort_order": 55})
	updateMenuColumns(t, tx, ctx, items[PermissionCreate].ID, map[string]interface{}{"sort_order": 44})

	if err := service.EnsureBuiltin(ctx); err != nil {
		t.Fatalf("EnsureBuiltin() after operator changes error = %v", err)
	}
	after := loadBuiltinMenus(t, tx, ctx)
	if value(after[BuiltinSystemCode].Icon) != "Folder" || after[BuiltinSystemCode].SortOrder != 77 {
		t.Fatalf("system operator fields were overwritten: %+v", after[BuiltinSystemCode])
	}
	if value(after[PermissionList].Icon) != "Cpu" || after[PermissionList].SortOrder != 55 {
		t.Fatalf("list operator fields were overwritten: %+v", after[PermissionList])
	}
	if after[PermissionCreate].SortOrder != 44 {
		t.Fatalf("action sort was overwritten: %+v", after[PermissionCreate])
	}
}

func openMenuTransaction(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	connection, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := database.AutoMigrate(ctx, connection.GORM, &testRole{}, &Menu{}, &RoleMenu{}); err != nil {
		t.Fatalf("AutoMigrate menu test schema: %v", err)
	}
	if err := EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	tx := connection.GORM.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	clearBuiltinMenus(t, tx, ctx)
	return tx, ctx
}

func clearBuiltinMenus(t *testing.T, tx *gorm.DB, ctx context.Context) {
	t.Helper()
	var rows []struct {
		ID    int64
		Depth int
	}
	if err := tx.WithContext(ctx).Raw(`
		WITH RECURSIVE builtin_tree AS (
			SELECT id, 0 AS depth
			FROM sys_menu
			WHERE code IN ?
			UNION
			SELECT child.id, parent.depth + 1
			FROM sys_menu child
			JOIN builtin_tree parent ON child.parent_id = parent.id
		)
		SELECT id, max(depth) AS depth
		FROM builtin_tree
		GROUP BY id`, builtinMenuCodes()).Scan(&rows).Error; err != nil {
		t.Fatalf("find existing builtin descendants: %v", err)
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	if len(ids) == 0 {
		return
	}
	if err := tx.WithContext(ctx).Unscoped().Where("menu_id IN ?", ids).Delete(&RoleMenu{}).Error; err != nil {
		t.Fatalf("clear builtin role-menu rows: %v", err)
	}
	sort.Slice(rows, func(left, right int) bool { return rows[left].Depth > rows[right].Depth })
	for _, row := range rows {
		if err := tx.WithContext(ctx).Unscoped().Where("id = ?", row.ID).Delete(&Menu{}).Error; err != nil {
			t.Fatalf("clear builtin menu %d: %v", row.ID, err)
		}
	}
}

func loadBuiltinMenus(t *testing.T, tx *gorm.DB, ctx context.Context) map[string]Menu {
	t.Helper()
	var rows []Menu
	if err := tx.WithContext(ctx).Unscoped().Where("code IN ?", builtinMenuCodes()).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("load builtin menus: %v", err)
	}
	result := make(map[string]Menu, len(rows))
	for _, row := range rows {
		if _, exists := result[row.Code]; exists {
			t.Fatalf("duplicate builtin code %q", row.Code)
		}
		result[row.Code] = row
	}
	return result
}

func assertExactBuiltinMenus(t *testing.T, items map[string]Menu) {
	t.Helper()
	if len(items) != 12 {
		t.Fatalf("builtin menu count = %d, want 12: %+v", len(items), items)
	}
	system := items[BuiltinSystemCode]
	list := items[PermissionList]
	create := items[PermissionCreate]
	update := items[PermissionUpdate]
	deleteItem := items[PermissionDelete]
	roles := items[PermissionRoleList]
	assertBuiltinMenu(t, system, TypeDirectory, nil, "navigation.system", nil, nil, stringPointer("Setting"), 100)
	assertBuiltinMenu(t, list, TypePage, &system.ID, "navigation.systemMenus", stringPointer("/system/menus"), stringPointer("system-menus"), stringPointer("Menu"), 10)
	assertBuiltinMenu(t, create, TypeAction, &list.ID, "permission.menuCreate", nil, nil, nil, 10)
	assertBuiltinMenu(t, update, TypeAction, &list.ID, "permission.menuUpdate", nil, nil, nil, 20)
	assertBuiltinMenu(t, deleteItem, TypeAction, &list.ID, "permission.menuDelete", nil, nil, nil, 30)
	assertBuiltinMenu(t, roles, TypePage, &system.ID, "navigation.systemRoles", stringPointer("/system/roles"), stringPointer("system-roles"), stringPointer("UserFilled"), 20)
	for index, item := range []struct {
		code string
		key  string
	}{
		{PermissionRoleCreate, "permission.roleCreate"},
		{PermissionRoleUpdate, "permission.roleUpdate"},
		{PermissionRoleStatus, "permission.roleStatus"},
		{PermissionRoleDefault, "permission.roleSetDefault"},
		{PermissionRoleDelete, "permission.roleDelete"},
		{PermissionRoleAuthorize, "permission.roleAuthorize"},
	} {
		assertBuiltinMenu(t, items[item.code], TypeAction, &roles.ID, item.key, nil, nil, nil, (index+1)*10)
	}
}

func assertBuiltinMenu(t *testing.T, item Menu, menuType Type, parentID *int64, i18nKey string, path, viewKey, icon *string, sortOrder int) {
	t.Helper()
	if item.ID < 1 || item.MenuType != menuType || !equalInt64Pointers(item.ParentID, parentID) ||
		item.I18nKey != i18nKey || !equalStringPointers(item.Path, path) ||
		!equalStringPointers(item.ViewKey, viewKey) || !equalStringPointers(item.Icon, icon) ||
		item.SortOrder != sortOrder || item.IsEnabled != yesno.Yes || item.DeletedAt.Valid ||
		item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		t.Fatalf("builtin menu %q = %+v", item.Code, item)
	}
}

func updateMenuColumns(t *testing.T, tx *gorm.DB, ctx context.Context, id int64, values map[string]interface{}) {
	t.Helper()
	if err := tx.WithContext(ctx).Model(&Menu{}).Where("id = ?", id).Updates(values).Error; err != nil {
		t.Fatalf("update menu %d: %v", id, err)
	}
}

func builtinMenuCodes() []string {
	return []string{
		BuiltinSystemCode, PermissionList, PermissionCreate, PermissionUpdate, PermissionDelete,
		PermissionRoleList, PermissionRoleCreate, PermissionRoleUpdate, PermissionRoleStatus,
		PermissionRoleDefault, PermissionRoleDelete, PermissionRoleAuthorize,
	}
}

func stringPointer(value string) *string {
	return &value
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}

func equalStringPointers(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func equalInt64Pointers(left, right *int64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}
