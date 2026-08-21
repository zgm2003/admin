package authplatform_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"admin/server/internal/database"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/authstate"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/yesno"
)

func TestServiceUpdateBuildsMissingReadyStateBeforeMutation(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	redisClient := openPlatformRedis(t)
	authStates := authstate.NewStore(redisClient)
	if err := redisClient.Delete(ctx, authplatform.PolicyKey("admin")); err != nil {
		t.Fatal(err)
	}
	service := authplatform.NewService(authplatform.NewRepository(connection.GORM), authplatform.NewPolicyStore(redisClient), redisClient, authStates, authstate.NewInvalidator(authStates), auth.NewSessionCache(redisClient).Delete, slog.New(slog.NewTextHandler(io.Discard, nil)), authplatform.Deployment{})
	input := authplatform.UpdateInput{
		Name: "Admin Updated", AccessTTLSeconds: 900, RefreshTTLSeconds: 1209600,
		SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800,
		BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 1, AllowRegister: yesno.Yes,
	}
	if err := service.Update(ctx, 1, input); err != nil {
		t.Fatal(err)
	}
	policy, err := service.CurrentPolicy(ctx, "admin")
	if err != nil || policy.Name != "Admin Updated" || policy.PolicyVersion != 2 {
		t.Fatalf("updated policy = %+v,%v", policy, err)
	}
}

func TestServiceUpdateDoesNotMutatePostgreSQLWhenRedisIsUnavailable(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	redisClient := openPlatformRedis(t)
	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}
	service := authplatform.NewService(authplatform.NewRepository(connection.GORM), authplatform.NewPolicyStore(redisClient), redisClient, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), authplatform.Deployment{})
	input := authplatform.UpdateInput{
		Name: "Must Not Persist", AccessTTLSeconds: 900, RefreshTTLSeconds: 1209600,
		SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800,
		BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 1, AllowRegister: yesno.Yes,
	}
	if err := service.Update(ctx, 1, input); err == nil {
		t.Fatal("update succeeded with Redis unavailable")
	}
	stored, err := authplatform.NewRepository(connection.GORM).FindPolicy(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "Admin" || stored.PolicyVersion != 1 {
		t.Fatalf("PostgreSQL was mutated: %+v", stored)
	}
}

func TestServicePlatformMutationsApplyExactSessionEffects(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	preparePlatformSessionSchema(t, connection.GORM, ctx)
	redisClient := openPlatformRedis(t)
	authStates := authstate.NewStore(redisClient)
	sessionCache := auth.NewSessionCache(redisClient)
	service := authplatform.NewService(
		authplatform.NewRepository(connection.GORM), authplatform.NewPolicyStore(redisClient), redisClient,
		authStates, authstate.NewInvalidator(authStates), sessionCache.Delete,
		slog.New(slog.NewTextHandler(io.Discard, nil)), authplatform.Deployment{},
	)
	code := fmt.Sprintf("sessions_%d", time.Now().UnixNano())
	platformID, err := service.Create(ctx, authplatform.CreateInput{
		Code: code, Name: "Sessions", AccessTTLSeconds: 900, RefreshTTLSeconds: 1209600,
		SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800,
		BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 3, AllowRegister: yesno.Yes, IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatal(err)
	}
	createdUser := createPlatformUser(t, connection.GORM, ctx, "service_sessions")
	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	sessions := createPlatformSessions(t, connection.GORM, ctx, createdUser.ID, code, base, 3)
	for _, session := range sessions {
		if err := redisClient.SetString(ctx, auth.SessionKey(code, session.ID), "cached", time.Hour); err != nil {
			t.Fatal(err)
		}
	}

	input := authplatform.UpdateInput{
		Name: "Sessions renamed", AccessTTLSeconds: 901, RefreshTTLSeconds: 1209601,
		SessionCacheTTLSeconds: 1801, AccessCacheTTLSeconds: 1801,
		BindDevice: yesno.Yes, BindIP: yesno.Yes, MaxSessions: 3, AllowRegister: yesno.No,
	}
	if err := service.Update(ctx, platformID, input); err != nil {
		t.Fatal(err)
	}
	assertActivePlatformSessionIDs(t, connection.GORM, ctx, code, sessionIDs(sessions))

	input.MaxSessions = 1
	if err := service.Update(ctx, platformID, input); err != nil {
		t.Fatal(err)
	}
	assertActivePlatformSessionIDs(t, connection.GORM, ctx, code, []int64{sessions[2].ID})
	for _, session := range sessions[:2] {
		if _, found, err := redisClient.GetString(ctx, auth.SessionKey(code, session.ID)); err != nil || found {
			t.Fatalf("revoked session snapshot %d: found=%v error=%v", session.ID, found, err)
		}
	}
	state, found, err := authStates.ReadSessions(ctx, code, createdUser.ID)
	if err != nil || !found || state.State != authstate.StateReady || state.Generation == "" {
		t.Fatalf("sessions state = %+v found=%v error=%v", state, found, err)
	}

	input.MaxSessions = 3
	if err := service.Update(ctx, platformID, input); err != nil {
		t.Fatal(err)
	}
	assertActivePlatformSessionIDs(t, connection.GORM, ctx, code, []int64{sessions[2].ID})
	if err := service.UpdateStatus(ctx, platformID, yesno.No); err != nil {
		t.Fatal(err)
	}
	assertActivePlatformSessionIDs(t, connection.GORM, ctx, code, []int64{})
	if err := service.UpdateStatus(ctx, platformID, yesno.Yes); err != nil {
		t.Fatal(err)
	}
	assertActivePlatformSessionIDs(t, connection.GORM, ctx, code, []int64{})
	if err := service.Delete(ctx, platformID); err != nil {
		t.Fatal(err)
	}
	var stored authplatform.Platform
	if err := connection.GORM.WithContext(ctx).Unscoped().Take(&stored, platformID).Error; err != nil {
		t.Fatal(err)
	}
	if !stored.DeletedAt.Valid || stored.PolicyVersion != 7 {
		t.Fatalf("deleted platform = %+v", stored)
	}
}

func TestServicePlatformNoOpDoesNotAdvancePolicyVersion(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	preparePlatformSessionSchema(t, connection.GORM, ctx)
	redisClient := openPlatformRedis(t)
	authStates := authstate.NewStore(redisClient)
	service := authplatform.NewService(
		authplatform.NewRepository(connection.GORM), authplatform.NewPolicyStore(redisClient), redisClient,
		authStates, authstate.NewInvalidator(authStates), auth.NewSessionCache(redisClient).Delete,
		slog.New(slog.NewTextHandler(io.Discard, nil)), authplatform.Deployment{},
	)
	stored, err := authplatform.NewRepository(connection.GORM).FindPolicy(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	input := authplatform.UpdateInput{
		Name: stored.Name, AccessTTLSeconds: stored.AccessTTLSeconds, RefreshTTLSeconds: stored.RefreshTTLSeconds,
		SessionCacheTTLSeconds: stored.SessionCacheTTLSeconds, AccessCacheTTLSeconds: stored.AccessCacheTTLSeconds,
		BindDevice: stored.BindDevice, BindIP: stored.BindIP, MaxSessions: stored.MaxSessions, AllowRegister: stored.AllowRegister,
	}
	if err := service.Update(ctx, stored.ID, input); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateStatus(ctx, stored.ID, stored.IsEnabled); err != nil {
		t.Fatal(err)
	}
	after, err := authplatform.NewRepository(connection.GORM).FindPolicy(ctx, "admin")
	if err != nil || after.PolicyVersion != stored.PolicyVersion || !after.UpdatedAt.Equal(stored.UpdatedAt) {
		t.Fatalf("no-op platform = %+v,%v", after, err)
	}
}

func TestServicePlatformRollbackRestoresPolicyAndSessionState(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	preparePlatformSessionSchema(t, connection.GORM, ctx)
	redisClient := openPlatformRedis(t)
	authStates := authstate.NewStore(redisClient)
	service := authplatform.NewService(
		authplatform.NewRepository(connection.GORM), authplatform.NewPolicyStore(redisClient), redisClient,
		authStates, authstate.NewInvalidator(authStates), auth.NewSessionCache(redisClient).Delete,
		slog.New(slog.NewTextHandler(io.Discard, nil)), authplatform.Deployment{},
	)
	code := fmt.Sprintf("rollback_%d", time.Now().UnixNano())
	platformID, err := service.Create(ctx, authplatform.CreateInput{
		Code: code, Name: "Rollback", AccessTTLSeconds: 900, RefreshTTLSeconds: 1209600,
		SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800, BindDevice: yesno.No, BindIP: yesno.No,
		MaxSessions: 1, AllowRegister: yesno.Yes, IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatal(err)
	}
	createdUser := createPlatformUser(t, connection.GORM, ctx, "rollback_user")
	session := createPlatformSessions(t, connection.GORM, ctx, createdUser.ID, code, time.Now().UTC().Add(-time.Hour), 1)[0]
	if err := connection.GORM.WithContext(ctx).Exec("ALTER TABLE sys_auth_platform ADD CONSTRAINT ck_test_auth_platform_rollback CHECK (is_enabled = 1)").Error; err != nil {
		t.Fatal(err)
	}
	err = service.UpdateStatus(ctx, platformID, yesno.No)
	if err == nil {
		t.Fatal("rollback mutation succeeded")
	}
	stored, err := authplatform.NewRepository(connection.GORM).FindPolicy(ctx, code)
	if err != nil || stored.Name != "Rollback" || stored.PolicyVersion != 1 || stored.IsEnabled != yesno.Yes {
		t.Fatalf("rolled-back policy = %+v,%v", stored, err)
	}
	var storedSession auth.Session
	if err := connection.GORM.WithContext(ctx).Take(&storedSession, session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedSession.RevokedAt != nil {
		t.Fatal("rollback revoked session")
	}
	state, found, err := authStates.ReadSessions(ctx, code, createdUser.ID)
	if err != nil || !found || state.State != authstate.StateReady {
		t.Fatalf("rolled-back sessions state = %+v found=%v error=%v", state, found, err)
	}
}

func TestServicePlatformPublishFailureLeavesCommittedSessionStateWithoutOldPolicy(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	preparePlatformSessionSchema(t, connection.GORM, ctx)
	redisClient := openPlatformRedis(t)
	authStates := authstate.NewStore(redisClient)
	service := authplatform.NewService(
		authplatform.NewRepository(connection.GORM), authplatform.NewPolicyStore(redisClient), redisClient,
		authStates, authstate.NewInvalidator(authStates), auth.NewSessionCache(redisClient).Delete,
		slog.New(slog.NewTextHandler(io.Discard, nil)), authplatform.Deployment{},
	)
	code := fmt.Sprintf("publish_%d", time.Now().UnixNano())
	platformID, err := service.Create(ctx, authplatform.CreateInput{
		Code: code, Name: "Publish", AccessTTLSeconds: 900, RefreshTTLSeconds: 1209600,
		SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800, BindDevice: yesno.No, BindIP: yesno.No,
		MaxSessions: 1, AllowRegister: yesno.Yes, IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatal(err)
	}
	createdUser := createPlatformUser(t, connection.GORM, ctx, "publish_user")
	session := createPlatformSessions(t, connection.GORM, ctx, createdUser.ID, code, time.Now().UTC().Add(-time.Hour), 1)[0]
	if err := redisClient.SetString(ctx, auth.SessionKey(code, session.ID), "cached", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := connection.GORM.WithContext(ctx).Exec(`
		CREATE FUNCTION delay_auth_platform_publish() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.5);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_auth_platform_publish
		BEFORE UPDATE OF is_enabled ON sys_auth_platform
		FOR EACH ROW EXECUTE FUNCTION delay_auth_platform_publish()`).Error; err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- service.UpdateStatus(ctx, platformID, yesno.No) }()
	waitForPolicyInvalidating(t, redisClient, code)
	if err := redisClient.Delete(ctx, authplatform.PolicyKey(code)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("publish failure mutation succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("publish failure mutation did not finish")
	}
	var stored authplatform.Platform
	if err := connection.GORM.WithContext(ctx).Unscoped().Take(&stored, platformID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.IsEnabled != yesno.No || stored.PolicyVersion != 2 {
		t.Fatalf("committed platform = %+v", stored)
	}
	var storedSession auth.Session
	if err := connection.GORM.WithContext(ctx).Take(&storedSession, session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedSession.RevokedAt == nil {
		t.Fatal("committed session was not revoked")
	}
	if _, found, err := redisClient.GetString(ctx, authplatform.PolicyKey(code)); err != nil || found {
		t.Fatalf("old policy remained reachable: found=%v error=%v", found, err)
	}
	if _, found, err := redisClient.GetString(ctx, auth.SessionKey(code, session.ID)); err != nil || found {
		t.Fatalf("old session snapshot remained reachable: found=%v error=%v", found, err)
	}
}

func sessionIDs(sessions []auth.Session) []int64 {
	ids := make([]int64, len(sessions))
	for index, session := range sessions {
		ids[index] = session.ID
	}
	return ids
}

func waitForPolicyInvalidating(t *testing.T, redisClient *projectredis.Client, code string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		raw, found, err := redisClient.GetString(context.Background(), authplatform.PolicyKey(code))
		if err == nil && found && strings.Contains(raw, `"state":"invalidating"`) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("policy %s did not become invalidating", code)
}
