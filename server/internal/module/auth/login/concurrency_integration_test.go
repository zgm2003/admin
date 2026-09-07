package auth

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/database/testschema"
	authplatform "admin/server/internal/module/auth/platform"
	"admin/server/internal/module/message/mail"
	"admin/server/internal/module/permission/role"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/module/user/loginlog"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestVerificationCodeTwoClientConsumeHasSingleWinner(t *testing.T) {
	firstClient := openAuthRedis(t)
	secondClient := openAuthRedis(t)
	keyMaterial := []byte(strings.Repeat("v", 32))
	firstStore := NewVerificationCodeStore(firstClient, keyMaterial)
	secondStore := NewVerificationCodeStore(secondClient, keyMaterial)
	key := firstStore.VerificationKey("admin", mail.SceneLogin, string(authplatform.LoginTypeEmail), fmt.Sprintf("concurrent-%d@example.com", time.Now().UnixNano()))
	ctx := context.Background()

	if acquired, err := firstStore.AcquireDelivery(ctx, key, "lease", 10*time.Second); err != nil || !acquired {
		t.Fatalf("AcquireDelivery = %v, %v", acquired, err)
	}
	if err := firstStore.Put(ctx, key, firstStore.Digest("123456"), "lease", time.Minute); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = secondClient.DeleteMany(context.Background(), []string{key, key + verificationCodeLeaseSuffix})
	})

	var winners atomic.Int64
	errorsFound := make(chan error, 32)
	var wait sync.WaitGroup
	for index := 0; index < 32; index++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			store := firstStore
			if worker%2 == 1 {
				store = secondStore
			}
			consumed, err := store.Consume(ctx, key, store.Digest("123456"))
			if err != nil {
				errorsFound <- err
				return
			}
			if consumed {
				winners.Add(1)
			}
		}(index)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatalf("concurrent Consume: %v", err)
	}
	if winners.Load() != 1 {
		t.Fatalf("Consume winners = %d, want 1", winners.Load())
	}
}

func TestVerificationCodeClosedClientFaultProbe(t *testing.T) {
	client := openAuthRedis(t)
	store := NewVerificationCodeStore(client, []byte(strings.Repeat("f", 32)))
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := store.VerificationKey("admin", mail.SceneLogin, string(authplatform.LoginTypeEmail), "fault@example.com")

	if err := store.Put(ctx, key, store.Digest("123456"), "lease", time.Minute); err == nil {
		t.Fatal("Put succeeded with a closed Redis client")
	}
	if consumed, err := store.Consume(ctx, key, store.Digest("123456")); err == nil || consumed {
		t.Fatalf("Consume with closed Redis = %v, %v", consumed, err)
	}
	if err := store.ReleaseDelivery(ctx, key, "lease"); err == nil {
		t.Fatal("ReleaseDelivery succeeded with a closed Redis client")
	}
}

func TestCreateVerifiedIdentityRollsBackWhenProfileInsertFails(t *testing.T) {
	db, ctx := openAuthFaultDatabase(t, "test_auth_identity_fault")
	if err := database.AutoMigrate(ctx, db, &user.User{}, &role.Role{}, &role.UserRole{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE user_profile (user_id BIGINT PRIMARY KEY, avatar VARCHAR(512) NOT NULL DEFAULT '', birthday DATE, gender SMALLINT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE permission_access_version (user_id BIGINT PRIMARY KEY, version BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`ALTER TABLE user_profile ADD CONSTRAINT ck_force_profile_insert_failure CHECK (user_id < 0)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.WithContext(ctx).Create(&role.Role{Code: "default", Name: "Default", IsDefault: yesno.Yes, IsEnabled: yesno.Yes}).Error; err != nil {
		t.Fatal(err)
	}

	_, err := user.NewRepository(db).CreateVerifiedIdentity(ctx, user.VerifiedIdentityInput{
		IdentityKind: "email", Account: "rollback@example.com", Username: "rollback", PasswordHash: "",
	})
	if err == nil {
		t.Fatal("identity creation succeeded despite forced profile failure")
	}
	for _, table := range []string{"user_account", "permission_user_role", "permission_access_version", "user_profile"} {
		var count int64
		if countErr := db.WithContext(ctx).Table(table).Count(&count).Error; countErr != nil {
			t.Fatalf("count %s: %v", table, countErr)
		}
		if count != 0 {
			t.Fatalf("%s retained %d rows after rollback", table, count)
		}
	}
}

func TestPostgresLoginLogFailureDoesNotChangeSuccessfulCredential(t *testing.T) {
	redisClient := openAuthRedis(t)
	userID := time.Now().UnixNano()%1_000_000_000 + 1_000_000
	sessionID := userID + 1
	cleanupAuthRedisKeys(t, redisClient, userID, "admin", sessionID)
	passwordHash, err := HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUserStore{credential: user.Credential{ID: userID, Username: "admin", Email: "admin@example.com", PasswordHash: passwordHash, IsEnabled: yesno.Yes}}
	sessions := &fakeSessionStore{createSession: Session{ID: sessionID, UserID: userID, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: testAuthClient().ClientIP}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: testPolicy()})
	failingDB, _ := openAuthFaultDatabase(t, "test_auth_login_log_fault")
	service.SetLoginLogRecorder(loginlog.NewService(loginlog.NewRepository(failingDB)))
	service.logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	credential, err := service.Login(context.Background(), LoginInput{
		LoginType: authplatform.LoginTypePassword, LoginAccount: "admin@example.com", Password: "password", Client: testAuthClient(),
	})
	if err != nil || credential.AccessToken == "" || credential.RefreshToken == "" {
		t.Fatalf("credential after login-log INSERT failure = %+v, %v", credential, err)
	}
}

func openAuthFaultDatabase(t *testing.T, prefix string) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return testschema.Open(t, settings.PostgresDSN, prefix)
}
