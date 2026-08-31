package authplatform_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/database"
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/module/permission/menu"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestRepositoryFiltersOrdersLocksAndRollsBackPlatforms(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	repository := authplatform.NewRepository(connection.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)
	first := testPlatform(fmt.Sprintf("app_%d", now.UnixNano()), "First", now.Add(time.Minute))
	second := testPlatform(fmt.Sprintf("web_%d", now.UnixNano()), "Second", now.Add(2*time.Minute))
	deleted := testPlatform(fmt.Sprintf("deleted_%d", now.UnixNano()), "Deleted", now.Add(3*time.Minute))
	for _, value := range []*authplatform.Platform{&first, &second, &deleted} {
		if err := repository.Create(ctx, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repository.SoftDelete(ctx, deleted.ID, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	enabled := yesno.Yes
	query := authplatform.ListQuery{Page: 1, PageSize: 20, Keyword: fmt.Sprintf("%d", now.UnixNano()), IsEnabled: &enabled}
	total, err := repository.Count(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	list, err := repository.List(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 || list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("total=%d list=%+v", total, list)
	}
	found, err := repository.FindPolicy(ctx, first.Code)
	if err != nil || found.ID != first.ID {
		t.Fatalf("FindPolicy() = %+v,%v", found, err)
	}
	if _, err := repository.FindPolicy(ctx, deleted.Code); err == nil {
		t.Fatal("deleted platform was returned as active")
	}

	forced := errors.New("forced rollback")
	err = repository.Transaction(ctx, func(scoped *authplatform.Repository) error {
		locked, lockErr := scoped.LockByID(ctx, first.ID)
		if lockErr != nil || locked.ID != first.ID {
			return fmt.Errorf("lock platform: %w", lockErr)
		}
		if _, updateErr := scoped.UpdateStatus(ctx, first.ID, yesno.No, now.Add(5*time.Minute)); updateErr != nil {
			return updateErr
		}
		return forced
	})
	if !errors.Is(err, forced) {
		t.Fatalf("Transaction() error = %v", err)
	}
	found, err = repository.FindPolicy(ctx, first.Code)
	if err != nil || found.IsEnabled != yesno.Yes {
		t.Fatalf("rolled back platform = %+v,%v", found, err)
	}
}

func TestRepositoryMapsActiveCodeConflictAndAdvancesPolicyVersion(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	repository := authplatform.NewRepository(connection.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)
	value := testPlatform(fmt.Sprintf("conflict_%d", now.UnixNano()), "Conflict", now)
	if err := repository.Create(ctx, &value); err != nil {
		t.Fatal(err)
	}
	duplicate := testPlatform(value.Code, "Duplicate", now)
	if err := repository.Create(ctx, &duplicate); !errors.Is(err, authplatform.ErrCodeConflict) {
		t.Fatalf("duplicate error = %v", err)
	}
	version, err := repository.UpdatePolicy(ctx, value.ID, authplatform.UpdateValues{
		Name: "Changed", AccessTTLSeconds: 901, RefreshTTLSeconds: value.RefreshTTLSeconds,
		SessionCacheTTLSeconds: value.SessionCacheTTLSeconds, AccessCacheTTLSeconds: value.AccessCacheTTLSeconds,
		BindDevice: value.BindDevice, BindIP: value.BindIP, MaxSessions: value.MaxSessions, AllowRegister: value.AllowRegister,
	}, now.Add(time.Minute))
	if err != nil || version != 2 {
		t.Fatalf("UpdatePolicy() = %d,%v", version, err)
	}
}

func TestRepositoryEnforcesPlatformSessionLimitAndReturnsExactRefs(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	preparePlatformSessionSchema(t, connection.GORM, ctx)
	repository := authplatform.NewRepository(connection.GORM)
	firstUser := createPlatformUser(t, connection.GORM, ctx, "first")
	secondUser := createPlatformUser(t, connection.GORM, ctx, "second")
	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	firstSessions := createPlatformSessions(t, connection.GORM, ctx, firstUser.ID, "admin", base, 3)
	secondSessions := createPlatformSessions(t, connection.GORM, ctx, secondUser.ID, "admin", base.Add(time.Minute), 2)
	appSessions := createPlatformSessions(t, connection.GORM, ctx, firstUser.ID, "app", base, 2)

	candidates, err := repository.FindSessionLimitCandidates(ctx, "admin", 1)
	if err != nil || !reflect.DeepEqual(candidates, []int64{firstUser.ID, secondUser.ID}) {
		t.Fatalf("FindSessionLimitCandidates() = %v,%v", candidates, err)
	}
	err = repository.Transaction(ctx, func(scoped *authplatform.Repository) error {
		locked, lockErr := scoped.LockActiveSessionUsers(ctx, "admin")
		if lockErr != nil || !reflect.DeepEqual(locked, []int64{firstUser.ID, secondUser.ID}) {
			return fmt.Errorf("LockActiveSessionUsers() = %v,%v", locked, lockErr)
		}
		revoked, revokeErr := scoped.EnforcePlatformLimit(ctx, "admin", 1, base.Add(2*time.Hour))
		want := []authplatform.SessionRef{
			{UserID: firstUser.ID, SessionID: firstSessions[0].ID},
			{UserID: firstUser.ID, SessionID: firstSessions[1].ID},
			{UserID: secondUser.ID, SessionID: secondSessions[0].ID},
		}
		if revokeErr != nil || !reflect.DeepEqual(revoked, want) {
			return fmt.Errorf("EnforcePlatformLimit() = %+v,%v", revoked, revokeErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertActivePlatformSessionIDs(t, connection.GORM, ctx, "admin", []int64{firstSessions[2].ID, secondSessions[1].ID})
	assertActivePlatformSessionIDs(t, connection.GORM, ctx, "app", []int64{appSessions[0].ID, appSessions[1].ID})
}

func TestRepositoryRevokesOnlyTargetPlatformSessions(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	preparePlatformSessionSchema(t, connection.GORM, ctx)
	createdUser := createPlatformUser(t, connection.GORM, ctx, "revoke")
	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	adminSessions := createPlatformSessions(t, connection.GORM, ctx, createdUser.ID, "admin", base, 2)
	appSessions := createPlatformSessions(t, connection.GORM, ctx, createdUser.ID, "app", base, 1)
	repository := authplatform.NewRepository(connection.GORM)
	revoked, err := repository.RevokePlatformSessions(ctx, "admin", base.Add(2*time.Hour))
	want := []authplatform.SessionRef{{UserID: createdUser.ID, SessionID: adminSessions[0].ID}, {UserID: createdUser.ID, SessionID: adminSessions[1].ID}}
	if err != nil || !reflect.DeepEqual(revoked, want) {
		t.Fatalf("RevokePlatformSessions() = %+v,%v", revoked, err)
	}
	assertActivePlatformSessionIDs(t, connection.GORM, ctx, "admin", []int64{})
	assertActivePlatformSessionIDs(t, connection.GORM, ctx, "app", []int64{appSessions[0].ID})
}

func testPlatform(code, name string, now time.Time) authplatform.Platform {
	return authplatform.Platform{
		Code: code, Name: name, PolicyVersion: 1,
		AccessTTLSeconds: 900, RefreshTTLSeconds: 1209600,
		SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800,
		BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 1,
		AllowRegister: yesno.Yes, IsEnabled: yesno.Yes, IsBuiltin: yesno.No,
		CreatedAt: now, UpdatedAt: now,
	}
}

func preparePlatformSessionSchema(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	if err := database.AutoMigrate(ctx, db, &user.User{}, &auth.Session{}, &authplatform.Platform{}, &menu.Menu{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	var existing int64
	if err := db.WithContext(ctx).Model(&authplatform.Platform{}).Where("code = ?", "app").Count(&existing).Error; err != nil {
		t.Fatal(err)
	}
	if existing == 0 {
		value := testPlatform("app", "App", time.Now().UTC().Truncate(time.Microsecond))
		if err := db.WithContext(ctx).Create(&value).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func createPlatformUser(t *testing.T, db *gorm.DB, ctx context.Context, prefix string) user.User {
	t.Helper()
	unique := time.Now().UnixNano()
	created := user.User{
		Username: fmt.Sprintf("%s_%d", prefix, unique), Email: fmt.Sprintf("%s_%d@example.com", prefix, unique),
		PasswordHash: "hash", IsEnabled: yesno.Yes,
	}
	if err := db.WithContext(ctx).Create(&created).Error; err != nil {
		t.Fatal(err)
	}
	return created
}

func createPlatformSessions(t *testing.T, db *gorm.DB, ctx context.Context, userID int64, platform string, base time.Time, count int) []auth.Session {
	t.Helper()
	var platformRow authplatform.Platform
	if err := db.WithContext(ctx).Where("code = ?", platform).Take(&platformRow).Error; err != nil {
		t.Fatalf("find test platform %s: %v", platform, err)
	}
	sessions := make([]auth.Session, count)
	for index := range sessions {
		createdAt := base.Add(time.Duration(index) * time.Minute)
		refreshHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d:%d", platform, userID, index, base.UnixNano())))
		sessions[index] = auth.Session{
			UserID: userID, PlatformID: platformRow.ID, Platform: platform, DeviceID: fmt.Sprintf("550e8400-e29b-41d4-a716-%012d", userID*100+int64(index)),
			RefreshTokenHash: fmt.Sprintf("%x", refreshHash), Version: 1,
			ClientIP: "127.0.0.1", UserAgent: "test", RefreshExpiresAt: base.Add(24 * time.Hour),
			CreatedAt: createdAt, UpdatedAt: createdAt,
		}
	}
	if err := db.WithContext(ctx).Create(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	return sessions
}

func assertActivePlatformSessionIDs(t *testing.T, db *gorm.DB, ctx context.Context, platform string, want []int64) {
	t.Helper()
	got := make([]int64, 0)
	if err := db.WithContext(ctx).Table("user_session AS session").Joins("JOIN auth_platform AS platform_ref ON platform_ref.id = session.platform_id").Where("platform_ref.code = ? AND session.revoked_at IS NULL", platform).Order("session.id ASC").Pluck("session.id", &got).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("active %s sessions = %v, want %v", platform, got, want)
	}
}
