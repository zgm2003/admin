package auth_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/server/internal/module/auth"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestCreateReplacingActiveRevokesOldSession(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUser(t, tx, ctx, "replace")
	repository := auth.NewSessionRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)

	first, err := repository.CreateReplacingActive(ctx, sessionInput(createdUser.ID, "a", now.Add(24*time.Hour)), now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.CreateReplacingActive(ctx, sessionInput(createdUser.ID, "b", now.Add(24*time.Hour)), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || second.Version != 1 {
		t.Fatalf("sessions = first %+v second %+v", first, second)
	}
	var storedFirst auth.Session
	if err := tx.WithContext(ctx).Take(&storedFirst, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedFirst.RevokedAt == nil || !storedFirst.RevokedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("old revoked_at = %v", storedFirst.RevokedAt)
	}
}

func TestFindActiveIdentityRequiresVersionUserAndRole(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUser(t, tx, ctx, "identity")
	repository := auth.NewSessionRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	session, err := repository.CreateReplacingActive(ctx, sessionInput(createdUser.ID, "c", now.Add(time.Hour)), now)
	if err != nil {
		t.Fatal(err)
	}

	identity, err := repository.FindActiveIdentity(ctx, session.ID, session.Version, now)
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserID != createdUser.ID || identity.SessionID != session.ID || identity.Version != session.Version {
		t.Fatalf("identity = %+v", identity)
	}
	if _, err := repository.FindActiveIdentity(ctx, session.ID, session.Version+1, now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("wrong version error = %v", err)
	}
	if err := tx.WithContext(ctx).Model(&user.User{}).Where("id = ?", createdUser.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindActiveIdentity(ctx, session.ID, session.Version, now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("disabled user error = %v", err)
	}
	if err := tx.WithContext(ctx).Model(&user.User{}).Where("id = ?", createdUser.ID).Update("is_enabled", yesno.Yes).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&role.Role{}).
		Where("id IN (SELECT role_id FROM sys_user_role WHERE user_id = ? AND deleted_at IS NULL)", createdUser.ID).
		Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindActiveIdentity(ctx, session.ID, session.Version, now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("disabled role error = %v", err)
	}
}

func TestFindCurrentExcludesExpiredSession(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUser(t, tx, ctx, "expired")
	repository := auth.NewSessionRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := repository.CreateReplacingActive(ctx, sessionInput(createdUser.ID, "d", now.Add(-time.Second)), now); err != nil {
		t.Fatal(err)
	}

	if _, err := repository.FindCurrentByUser(ctx, createdUser.ID, now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expired current session error = %v", err)
	}
}

func TestFindByRefreshHashRequiresActiveIdentity(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUser(t, tx, ctx, "refresh-find")
	repository := auth.NewSessionRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, err := repository.CreateReplacingActive(ctx, sessionInput(createdUser.ID, "e", now.Add(time.Hour)), now)
	if err != nil {
		t.Fatal(err)
	}
	found, err := repository.FindByRefreshHash(ctx, created.RefreshTokenHash, now)
	if err != nil || found.ID != created.ID {
		t.Fatalf("FindByRefreshHash() = %+v,%v", found, err)
	}
	if err := repository.Revoke(ctx, created.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindByRefreshHash(ctx, created.RefreshTokenHash, now.Add(time.Minute)); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("revoked refresh hash error = %v", err)
	}
}

func TestRotateByRefreshHashCanWinOnlyOnce(t *testing.T) {
	connection, ctx := openAuthenticationSchema(t)
	roleRepository := role.NewRepository(connection.GORM)
	if err := role.NewService(roleRepository).EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	createdUser := createAuthUser(t, connection.GORM, ctx, "rotate")
	repository := auth.NewSessionRepository(connection.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, err := repository.CreateReplacingActive(ctx, sessionInput(createdUser.ID, "f", now.Add(time.Hour)), now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = connection.GORM.WithContext(cleanupCtx).Where("user_id = ?", createdUser.ID).Delete(&auth.Session{}).Error
		_ = connection.GORM.WithContext(cleanupCtx).Unscoped().Where("user_id = ?", createdUser.ID).Delete(&role.UserRole{}).Error
		_ = connection.GORM.WithContext(cleanupCtx).Unscoped().Delete(&user.User{}, createdUser.ID).Error
	})

	type result struct {
		session auth.Session
		rotated bool
		err     error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	for index, newHashCharacter := range []string{"g", "h"} {
		waitGroup.Add(1)
		go func(index int, newHash string) {
			defer waitGroup.Done()
			<-start
			rotatedSession, rotated, rotateErr := repository.RotateByRefreshHash(
				ctx, created.ID, created.RefreshTokenHash, strings.Repeat(newHash, 64),
				now.Add(time.Minute), fmt.Sprintf("127.0.0.%d", index+1), "test-agent",
			)
			results <- result{session: rotatedSession, rotated: rotated, err: rotateErr}
		}(index, newHashCharacter)
	}
	close(start)
	waitGroup.Wait()
	close(results)

	winners := 0
	for got := range results {
		if got.err != nil {
			t.Errorf("RotateByRefreshHash() error = %v", got.err)
		}
		if got.rotated {
			winners++
			if got.session.Version != created.Version+1 || !got.session.RefreshExpiresAt.Equal(created.RefreshExpiresAt) {
				t.Errorf("rotated session = %+v", got.session)
			}
		}
	}
	if winners != 1 {
		t.Fatalf("rotation winners = %d, want 1", winners)
	}
}

func TestRevokeIsIdempotentForTheSameSession(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	createdUser := createAuthUser(t, tx, ctx, "revoke")
	repository := auth.NewSessionRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, err := repository.CreateReplacingActive(ctx, sessionInput(createdUser.ID, "i", now.Add(time.Hour)), now)
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
	if err := role.NewService(role.NewRepository(tx)).EnsureSystemRoles(ctx); err != nil {
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

func sessionInput(userID int64, hashCharacter string, expiresAt time.Time) auth.SessionCreate {
	return auth.SessionCreate{
		UserID:           userID,
		RefreshTokenHash: strings.Repeat(hashCharacter, 64),
		ClientIP:         "127.0.0.1",
		UserAgent:        "session-test",
		RefreshExpiresAt: expiresAt,
	}
}
