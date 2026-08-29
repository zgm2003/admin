package session_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/database/testschema"
	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/module/rbac/access"
	"admin/server/internal/module/rbac/role"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/module/user/session"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestCreateWithinLimitRetainsNewestSessionsPerPlatform(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUserWithoutRole(t, tx, ctx, "limit")
	policy := updateTestPolicy(t, tx, ctx, "admin", 2)
	repository := session.NewRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)

	first, revoked, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "1", now.Add(time.Hour)), policy, now)
	if err != nil || len(revoked) != 0 {
		t.Fatalf("first CreateWithinLimit() = %+v,%+v,%v", first, revoked, err)
	}
	second, revoked, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "2", now.Add(time.Hour)), policy, now.Add(time.Second))
	if err != nil || len(revoked) != 0 {
		t.Fatalf("second CreateWithinLimit() = %+v,%+v,%v", second, revoked, err)
	}
	third, revoked, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "3", now.Add(time.Hour)), policy, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(revoked) != 1 || revoked[0].ID != first.ID {
		t.Fatalf("revoked sessions = %+v", revoked)
	}
	var active []session.Session
	if err := tx.WithContext(ctx).Where("user_id = ? AND platform_id = ? AND revoked_at IS NULL", createdUser.ID, policy.ID).Order("created_at DESC, id DESC").Find(&active).Error; err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 || active[0].ID != third.ID || active[1].ID != second.ID {
		t.Fatalf("active sessions = %+v", active)
	}
}

func TestCreateWithinLimitKeepsPlatformsIndependentAndSupportsUnlimited(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUserWithoutRole(t, tx, ctx, "platform-limit")
	adminPolicy := updateTestPolicy(t, tx, ctx, "admin", 0)
	appPolicy := createTestPolicy(t, tx, ctx, "app", 1)
	repository := session.NewRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)

	for index, hash := range []string{"4", "5", "6"} {
		if _, revoked, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, hash, now.Add(time.Hour)), adminPolicy, now.Add(time.Duration(index)*time.Second)); err != nil || len(revoked) != 0 {
			t.Fatalf("unlimited admin login %d = %+v,%v", index, revoked, err)
		}
	}
	appInput := sessionInput(createdUser.ID, "7", now.Add(time.Hour))
	appInput.Platform = "app"
	if _, revoked, err := repository.CreateWithinLimit(ctx, appInput, appPolicy, now.Add(4*time.Second)); err != nil || len(revoked) != 0 {
		t.Fatalf("app login = %+v,%v", revoked, err)
	}
	var adminCount, appCount int64
	if err := tx.WithContext(ctx).Model(&session.Session{}).Where("user_id = ? AND platform_id = ? AND revoked_at IS NULL", createdUser.ID, adminPolicy.ID).Count(&adminCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&session.Session{}).Where("user_id = ? AND platform_id = ? AND revoked_at IS NULL", createdUser.ID, appPolicy.ID).Count(&appCount).Error; err != nil {
		t.Fatal(err)
	}
	if adminCount != 3 || appCount != 1 {
		t.Fatalf("active counts admin=%d app=%d", adminCount, appCount)
	}
}

func TestCreateWithinLimitSerializesConcurrentLogins(t *testing.T) {
	db, ctx := openAuthenticationSchema(t)
	createdUser := createAuthUserWithoutRole(t, db, ctx, "concurrent-limit")
	policy := updateTestPolicy(t, db, ctx, "admin", 2)
	repository := session.NewRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	start := make(chan struct{})
	errorsChannel := make(chan error, 3)
	var waitGroup sync.WaitGroup
	for index, hash := range []string{"b", "c", "d"} {
		waitGroup.Add(1)
		go func(index int, hash string) {
			defer waitGroup.Done()
			<-start
			_, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, hash, now.Add(time.Hour)), policy, now.Add(time.Duration(index)*time.Microsecond))
			errorsChannel <- err
		}(index, hash)
	}
	close(start)
	waitGroup.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatal(err)
		}
	}
	var activeCount int64
	if err := db.WithContext(ctx).Model(&session.Session{}).
		Where("user_id = ? AND platform_id = ? AND revoked_at IS NULL", createdUser.ID, policy.ID).Count(&activeCount).Error; err != nil {
		t.Fatal(err)
	}
	if activeCount != 2 {
		t.Fatalf("concurrent active sessions = %d", activeCount)
	}
}

func TestFindAuthoritativeDoesNotRequireAnEnabledRole(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUserWithoutRole(t, tx, ctx, "authority")
	policy := updateTestPolicy(t, tx, ctx, "admin", 1)
	repository := session.NewRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "8", now.Add(time.Hour)), policy, now)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := repository.FindAuthoritative(ctx, createdUser.ID, created.ID, "admin", created.Version, now)
	if err != nil {
		t.Fatal(err)
	}
	if authority.Session.ID != created.ID || authority.UserID != createdUser.ID || authority.UserIsEnabled != yesno.Yes || authority.UserDeleted {
		t.Fatalf("authority = %+v", authority)
	}
}

func TestRotateByRefreshHashUsesExactPlatformAndClientMetadata(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUserWithoutRole(t, tx, ctx, "rotate-client")
	policy := updateTestPolicy(t, tx, ctx, "admin", 1)
	repository := session.NewRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "9", now.Add(time.Hour)), policy, now)
	if err != nil {
		t.Fatal(err)
	}
	client := authclient.Client{Platform: "admin", DeviceID: created.DeviceID, ClientIP: "192.0.2.5", UserAgent: "rotated-agent"}
	rotated, won, err := repository.RotateByRefreshHash(ctx, created.ID, "admin", created.RefreshTokenHash, strings.Repeat("a", 64), now.Add(time.Minute), client)
	if err != nil || !won {
		t.Fatalf("RotateByRefreshHash() = %+v,%v,%v", rotated, won, err)
	}
	if rotated.Version != created.Version+1 || rotated.ClientIP != client.ClientIP || rotated.UserAgent != client.UserAgent || rotated.DeviceID != created.DeviceID || !rotated.RefreshExpiresAt.Equal(created.RefreshExpiresAt) {
		t.Fatalf("rotated session = %+v", rotated)
	}
}

func TestRevokeIsIdempotentForTheSameSession(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUserWithoutRole(t, tx, ctx, "revoke")
	repository := session.NewRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	policy := updateTestPolicy(t, tx, ctx, "admin", 1)
	created, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "i", now.Add(time.Hour)), policy, now)
	if err != nil {
		t.Fatal(err)
	}
	revokedAt := now.Add(time.Minute)
	if err := repository.Revoke(ctx, created.ID, revokedAt); err != nil {
		t.Fatal(err)
	}
	if err := repository.Revoke(ctx, created.ID, revokedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var stored session.Session
	if err := tx.WithContext(ctx).Take(&stored, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.RevokedAt == nil || !stored.RevokedAt.Equal(revokedAt) {
		t.Fatalf("revoked_at = %v, want %v", stored.RevokedAt, revokedAt)
	}
}

func TestListAdminSessionsCalculatesStatusFromPostgres(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUserWithoutRole(t, tx, ctx, "admin-list")
	repository := session.NewRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	policy := updateTestPolicy(t, tx, ctx, "admin", 0)
	active, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "a", now.Add(time.Hour)), policy, now)
	if err != nil {
		t.Fatal(err)
	}
	expired, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "b", now.Add(-time.Hour)), policy, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&session.Session{}).Where("id = ?", expired.ID).Updates(map[string]any{"refresh_expires_at": now.Add(-time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	revoked, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "c", now.Add(time.Hour)), policy, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Revoke(ctx, revoked.ID, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	rows, total, err := repository.ListAdmin(ctx, session.AdminSessionQuery{Page: 1, PageSize: 20}, now)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(rows) != 3 {
		t.Fatalf("rows total=%d len=%d", total, len(rows))
	}
	statuses := map[int64]session.SessionStatus{}
	for _, row := range rows {
		statuses[row.ID] = row.Status
		if row.Username != createdUser.Username {
			t.Fatalf("username = %q", row.Username)
		}
	}
	if statuses[active.ID] != session.SessionStatusActive || statuses[expired.ID] != session.SessionStatusExpired || statuses[revoked.ID] != session.SessionStatusRevoked {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func TestAdminSessionRevokeRejectsCurrentSession(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUserWithoutRole(t, tx, ctx, "admin-current")
	repository := session.NewRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	policy := updateTestPolicy(t, tx, ctx, "admin", 0)
	current, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "d", now.Add(time.Hour)), policy, now)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "e", now.Add(time.Hour)), policy, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	result, err := repository.RevokeAdmin(ctx, []int64{current.ID, other.ID}, current.ID, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.SkippedCurrent != 1 || len(result.Revoked) != 1 || result.Revoked[0].ID != other.ID {
		t.Fatalf("revoke result = %+v", result)
	}
	var stored session.Session
	if err := tx.Unscoped().Take(&stored, current.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.RevokedAt != nil {
		t.Fatalf("current session revoked at %v", stored.RevokedAt)
	}
}

func TestBulkAdminSessionRevokeDeduplicatesAndLimits(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUserWithoutRole(t, tx, ctx, "admin-bulk")
	repository := session.NewRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	policy := updateTestPolicy(t, tx, ctx, "admin", 0)
	first, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "f", now.Add(time.Hour)), policy, now)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "g", now.Add(time.Hour)), policy, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Revoke(ctx, first.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	result, err := repository.RevokeAdmin(ctx, []int64{first.ID, first.ID, second.ID}, 0, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.SkippedRevoked != 1 || len(result.Revoked) != 1 || result.Revoked[0].ID != second.ID {
		t.Fatalf("bulk result = %+v", result)
	}
}

func openAuthTransaction(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := openAuthenticationSchema(t)
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	if err := role.NewService(role.NewRepository(tx), nil).EnsureSystemRoles(ctx); err != nil {
		t.Fatalf("EnsureSystemRoles: %v", err)
	}
	return tx, ctx
}

func openAuthenticationSchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_user_session")
	if err := database.AutoMigrate(ctx, db, &user.User{}, &role.Role{}, &role.UserRole{}, &authplatform.Platform{}, &session.Session{}, &access.Version{}); err != nil {
		t.Fatalf("create session test schema: %v", err)
	}
	if err := role.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("ensure role test schema: %v", err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("ensure platform test schema: %v", err)
	}
	return db, ctx
}

func createAuthUser(t *testing.T, db *gorm.DB, ctx context.Context, prefix string) user.User {
	t.Helper()
	roleRepository := role.NewRepository(db)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatalf("FindDefault: %v", err)
	}
	unique := time.Now().UnixNano()
	created, err := user.NewRepository(db).CreateWithRole(ctx, user.CreateInput{
		Username:     fmt.Sprintf("%s-%d", prefix, unique),
		Email:        fmt.Sprintf("%s-%d@example.com", prefix, unique),
		PasswordHash: "$2a$10$placeholder",
		RoleID:       defaultRole.ID,
	})
	if err != nil {
		t.Fatalf("CreateWithRole: %v", err)
	}
	return created
}

func createAuthUserWithoutRole(t *testing.T, db *gorm.DB, ctx context.Context, prefix string) user.User {
	t.Helper()
	unique := time.Now().UnixNano()
	created := user.User{
		Username: fmt.Sprintf("%s-%d", prefix, unique), Email: fmt.Sprintf("%s-%d@example.com", prefix, unique),
		PasswordHash: "$2a$10$placeholder", IsEnabled: yesno.Yes,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond), UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	if err := db.WithContext(ctx).Create(&created).Error; err != nil {
		t.Fatalf("create user without role: %v", err)
	}
	return created
}

func updateTestPolicy(t *testing.T, db *gorm.DB, ctx context.Context, code string, maxSessions int16) authplatform.Policy {
	t.Helper()
	if err := db.WithContext(ctx).Model(&authplatform.Platform{}).Where("code = ?", code).Updates(map[string]any{
		"max_sessions": maxSessions, "updated_at": time.Now().UTC().Truncate(time.Microsecond),
	}).Error; err != nil {
		t.Fatalf("update test policy: %v", err)
	}
	var value authplatform.Platform
	if err := db.WithContext(ctx).Where("code = ?", code).Take(&value).Error; err != nil {
		t.Fatalf("find test policy: %v", err)
	}
	return testRuntimePolicy(value)
}

func createTestPolicy(t *testing.T, db *gorm.DB, ctx context.Context, code string, maxSessions int16) authplatform.Policy {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	value := authplatform.Platform{
		Code: code, Name: code, PolicyVersion: 1, AccessTTLSeconds: 900, RefreshTTLSeconds: 1209600,
		SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800, MaxSessions: maxSessions,
		BindDevice: yesno.No, BindIP: yesno.No, AllowRegister: yesno.Yes, IsEnabled: yesno.Yes, IsBuiltin: yesno.No,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.WithContext(ctx).Create(&value).Error; err != nil {
		t.Fatalf("create test policy: %v", err)
	}
	return testRuntimePolicy(value)
}

func testRuntimePolicy(value authplatform.Platform) authplatform.Policy {
	return authplatform.Policy{
		ID: value.ID, Code: value.Code, Name: value.Name, PolicyVersion: value.PolicyVersion,
		AccessTTL: time.Duration(value.AccessTTLSeconds) * time.Second, RefreshTTL: time.Duration(value.RefreshTTLSeconds) * time.Second,
		SessionCacheTTL: time.Duration(value.SessionCacheTTLSeconds) * time.Second, AccessCacheTTL: time.Duration(value.AccessCacheTTLSeconds) * time.Second,
		BindDevice: value.BindDevice == yesno.Yes, BindIP: value.BindIP == yesno.Yes, MaxSessions: value.MaxSessions,
		AllowRegister: value.AllowRegister == yesno.Yes, IsEnabled: value.IsEnabled == yesno.Yes, IsBuiltin: value.IsBuiltin == yesno.Yes,
	}
}

func sessionInput(userID int64, hashCharacter string, expiresAt time.Time) session.CreateInput {
	return session.CreateInput{
		UserID:           userID,
		Platform:         "admin",
		DeviceID:         "550e8400-e29b-41d4-a716-446655440000",
		RefreshTokenHash: strings.Repeat(hashCharacter, 64),
		ClientIP:         "127.0.0.1",
		UserAgent:        "session-test",
		RefreshExpiresAt: expiresAt,
	}
}
