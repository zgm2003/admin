package auth_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/server/internal/module/auth"
	"admin/server/internal/module/authclient"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestCreateWithinLimitRetainsNewestSessionsPerPlatform(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUserWithoutRole(t, tx, ctx, "limit")
	policy := updateTestPolicy(t, tx, ctx, "admin", 2)
	repository := auth.NewSessionRepository(tx)
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
	var active []auth.Session
	if err := tx.WithContext(ctx).Where("user_id = ? AND platform = ? AND revoked_at IS NULL", createdUser.ID, "admin").Order("created_at DESC, id DESC").Find(&active).Error; err != nil {
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
	repository := auth.NewSessionRepository(tx)
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
	if err := tx.WithContext(ctx).Model(&auth.Session{}).Where("user_id = ? AND platform = ? AND revoked_at IS NULL", createdUser.ID, "admin").Count(&adminCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&auth.Session{}).Where("user_id = ? AND platform = ? AND revoked_at IS NULL", createdUser.ID, "app").Count(&appCount).Error; err != nil {
		t.Fatal(err)
	}
	if adminCount != 3 || appCount != 1 {
		t.Fatalf("active counts admin=%d app=%d", adminCount, appCount)
	}
}

func TestCreateWithinLimitSerializesConcurrentLogins(t *testing.T) {
	connection, ctx := openAuthenticationSchema(t)
	createdUser := createAuthUserWithoutRole(t, connection.GORM, ctx, "concurrent-limit")
	policy := updateTestPolicy(t, connection.GORM, ctx, "admin", 2)
	repository := auth.NewSessionRepository(connection.GORM)
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
	if err := connection.GORM.WithContext(ctx).Model(&auth.Session{}).
		Where("user_id = ? AND platform = ? AND revoked_at IS NULL", createdUser.ID, "admin").Count(&activeCount).Error; err != nil {
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
	repository := auth.NewSessionRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, _, err := repository.CreateWithinLimit(ctx, sessionInput(createdUser.ID, "8", now.Add(time.Hour)), policy, now)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := repository.FindAuthoritative(ctx, auth.TokenIdentity{UserID: createdUser.ID, SessionID: created.ID, Platform: "admin", Version: created.Version}, now)
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
	repository := auth.NewSessionRepository(tx)
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
	repository := auth.NewSessionRepository(tx)
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
	var stored auth.Session
	if err := tx.WithContext(ctx).Take(&stored, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.RevokedAt == nil || !stored.RevokedAt.Equal(revokedAt) {
		t.Fatalf("revoked_at = %v, want %v", stored.RevokedAt, revokedAt)
	}
}

func openAuthTransaction(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	connection, ctx := openAuthenticationSchema(t)
	tx := connection.GORM.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	if err := role.NewService(role.NewRepository(tx), nil).EnsureSystemRoles(ctx); err != nil {
		t.Fatalf("EnsureSystemRoles: %v", err)
	}
	return tx, ctx
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

func sessionInput(userID int64, hashCharacter string, expiresAt time.Time) auth.SessionCreate {
	return auth.SessionCreate{
		UserID:           userID,
		Platform:         "admin",
		DeviceID:         "550e8400-e29b-41d4-a716-446655440000",
		RefreshTokenHash: strings.Repeat(hashCharacter, 64),
		ClientIP:         "127.0.0.1",
		UserAgent:        "session-test",
		RefreshExpiresAt: expiresAt,
	}
}
