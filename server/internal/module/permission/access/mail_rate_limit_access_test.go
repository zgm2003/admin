package permission_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/config"
	login "admin/server/internal/module/auth/login"
	permissionaccess "admin/server/internal/module/permission/access"
	"admin/server/internal/module/permission/menu"
	"admin/server/internal/module/permission/role"
	permissionstate "admin/server/internal/module/permission/state"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
)

func TestMailRateLimitActionRequiresExplicitGrantAndRespectsPlatformScope(t *testing.T) {
	fixture := openAccessRepositoryFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	var root menu.Menu
	if err := fixture.db.WithContext(fixture.ctx).Where("platform_id = ? AND code = ?", fixture.adminPlatformID, "account").Take(&root).Error; err != nil {
		t.Fatal(err)
	}
	mailI18nKey := "navigation.mail"
	mailPath, mailComponentPath := "/message/mail", "message/mail"
	mailPage := menu.Menu{
		PlatformID: fixture.adminPlatformID, ParentID: &root.ID, MenuType: menu.TypePage,
		Name: "邮件服务", Code: "message:mail:view", I18nKey: &mailI18nKey,
		Path: &mailPath, ComponentPath: &mailComponentPath, SortOrder: 20,
		IsEnabled: yesno.Yes, IsHidden: yesno.No, CreatedAt: now, UpdatedAt: now,
	}
	if err := fixture.db.WithContext(fixture.ctx).Create(&mailPage).Error; err != nil {
		t.Fatal(err)
	}
	mailAction := menu.Menu{
		PlatformID: fixture.adminPlatformID, ParentID: &mailPage.ID, MenuType: menu.TypeAction,
		Name: "修改邮件限流策略", Code: "message:mail:rate-limit:update", SortOrder: 10,
		IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now,
	}
	if err := fixture.db.WithContext(fixture.ctx).Create(&mailAction).Error; err != nil {
		t.Fatal(err)
	}

	canvasAction := menu.Menu{
		PlatformID: fixture.canvasPlatformID, ParentID: &fixture.canvasPage.ID, MenuType: menu.TypeAction,
		Name: "Canvas action", Code: "canvas:mail:rate-limit:update", SortOrder: 20,
		IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now,
	}
	if err := fixture.db.WithContext(fixture.ctx).Create(&canvasAction).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.WithContext(fixture.ctx).Create(&menu.RoleMenu{
		RoleID: fixture.assignedRole.ID, MenuID: canvasAction.ID, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	redisClient := openMailRateLimitAccessRedis(t)
	keys := []string{
		permissionstate.StateKey(fixture.user.ID),
		permissionaccess.SnapshotKey("admin", 4, fixture.user.ID, 3), permissionaccess.SnapshotKey("admin", 4, fixture.user.ID, 4), permissionaccess.SnapshotKey("admin", 4, fixture.user.ID, 5),
		permissionaccess.SnapshotKey("canvas", 4, fixture.user.ID, 3), permissionaccess.SnapshotKey("canvas", 4, fixture.user.ID, 4), permissionaccess.SnapshotKey("canvas", 4, fixture.user.ID, 5),
	}
	if err := redisClient.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = redisClient.DeleteMany(context.Background(), keys) })

	stateStore := permissionstate.NewStore(redisClient)
	roleService := role.NewService(role.NewRepository(fixture.db), permissionstate.NewInvalidator(stateStore))
	accessService := permissionaccess.NewService(
		fixture.repository, stateStore, permissionaccess.NewSnapshotCache(redisClient),
		permissionaccess.NewLocalSnapshotCache(8), slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	adminIdentity := accessIdentityForMailRateLimit(fixture.user.ID)
	canvasIdentity := adminIdentity
	canvasIdentity.PlatformID = fixture.canvasPlatformID
	canvasIdentity.Platform = "canvas"

	if _, err := roleService.UpdatePermissions(fixture.ctx, fixture.assignedRole.ID, []int64{mailPage.ID, canvasAction.ID}); err != nil {
		t.Fatal(err)
	}
	adminSnapshot, err := accessService.Current(fixture.ctx, adminIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(adminSnapshot.PermissionCodes, "message:mail:view") || containsString(adminSnapshot.PermissionCodes, mailAction.Code) {
		t.Fatalf("ungranted Admin mail action leaked into permissions: %+v", adminSnapshot.PermissionCodes)
	}
	if containsMenuCode(adminSnapshot.MenuTree, mailAction.Code) || containsString(adminSnapshot.PermissionCodes, canvasAction.Code) {
		t.Fatalf("Admin snapshot leaked hidden or cross-platform action: %+v", adminSnapshot)
	}

	canvasSnapshot, err := accessService.Current(fixture.ctx, canvasIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(canvasSnapshot.PermissionCodes, canvasAction.Code) || containsMenuCode(canvasSnapshot.MenuTree, canvasAction.Code) {
		t.Fatalf("Canvas action access shape = %+v", canvasSnapshot)
	}

	beforeState, found, err := stateStore.Read(fixture.ctx, fixture.user.ID)
	if err != nil || !found {
		t.Fatalf("read access state before grant = %+v found=%v err=%v", beforeState, found, err)
	}
	if _, err := roleService.UpdatePermissions(fixture.ctx, fixture.assignedRole.ID, []int64{mailAction.ID, canvasAction.ID}); err != nil {
		t.Fatal(err)
	}
	afterState, found, err := stateStore.Read(fixture.ctx, fixture.user.ID)
	if err != nil || !found || afterState.Version <= beforeState.Version {
		t.Fatalf("access version after explicit action grant = %+v found=%v err=%v", afterState, found, err)
	}

	adminSnapshot, err = accessService.Current(fixture.ctx, adminIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(adminSnapshot.PermissionCodes, mailAction.Code) || containsMenuCode(adminSnapshot.MenuTree, mailAction.Code) {
		t.Fatalf("granted Admin action access shape = %+v", adminSnapshot)
	}
	if containsString(adminSnapshot.PermissionCodes, canvasAction.Code) {
		t.Fatalf("Admin snapshot contains Canvas permission: %+v", adminSnapshot.PermissionCodes)
	}

	canvasSnapshot, err = accessService.Current(fixture.ctx, canvasIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(canvasSnapshot.PermissionCodes, canvasAction.Code) || !reflect.DeepEqual(canvasSnapshot.RoleCodes, adminSnapshot.RoleCodes) {
		t.Fatalf("Canvas access changed unexpectedly: %+v", canvasSnapshot)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsMenuCode(nodes []permissionaccess.MenuNode, want string) bool {
	for _, node := range nodes {
		if node.Code == want || containsMenuCode(node.Children, want) {
			return true
		}
	}
	return false
}

func openMailRateLimitAccessRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	client, err := projectredis.Open(context.Background(), settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func accessIdentityForMailRateLimit(userID int64) login.Identity {
	return login.Identity{
		UserID: userID, SessionID: userID + 1000, PlatformID: 1, Platform: "admin",
		Version: 1, PolicyVersion: 4, AccessCacheTTL: time.Hour,
	}
}
