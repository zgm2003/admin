package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/authclient"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/authstate"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"github.com/joho/godotenv"
)

type integrationPolicyStore struct {
	policy authplatform.Policy
}

func (s integrationPolicyStore) CurrentPolicy(_ context.Context, platform string) (authplatform.Policy, error) {
	if platform != s.policy.Code {
		return authplatform.Policy{}, errors.New("unexpected authentication platform")
	}
	return s.policy, nil
}

func TestAdminSessionRevokePersistsDeletesSnapshotAndRejectsOldToken(t *testing.T) {
	tx, ctx := openAuthTransaction(t)
	redisClient := openSessionAdminRedis(t)
	actorUser := createAuthUserWithoutRole(t, tx, ctx, "session-admin-actor")
	targetUser := createAuthUserWithoutRole(t, tx, ctx, "session-admin-target")
	policy := updateTestPolicy(t, tx, ctx, "admin", 0)
	repository := auth.NewSessionRepository(tx)
	now := time.Now().UTC().Truncate(time.Second)
	actorSession, _, err := repository.CreateWithinLimit(ctx, sessionInput(actorUser.ID, "i", now.Add(time.Hour)), policy, now)
	if err != nil {
		t.Fatal(err)
	}
	targetSession, _, err := repository.CreateWithinLimit(ctx, sessionInput(targetUser.ID, "j", now.Add(time.Hour)), policy, now)
	if err != nil {
		t.Fatal(err)
	}

	states := authstate.NewStore(redisClient)
	cache := auth.NewSessionCache(redisClient)
	userGeneration, err := authstate.NewGeneration()
	if err != nil {
		t.Fatal(err)
	}
	sessionsGeneration, err := authstate.NewGeneration()
	if err != nil {
		t.Fatal(err)
	}
	redisKeys := []string{
		authstate.UserStateKey(targetUser.ID),
		authstate.SessionsStateKey("admin", targetUser.ID),
		auth.SessionKey("admin", targetSession.ID),
	}
	_ = redisClient.DeleteMany(ctx, redisKeys)
	t.Cleanup(func() { _ = redisClient.DeleteMany(context.Background(), redisKeys) })
	if _, _, err := states.InstallUserReadyIfMissing(ctx, authstate.UserFact{UserID: targetUser.ID, Generation: userGeneration, IsEnabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := states.InstallSessionsReadyIfMissing(ctx, authstate.SessionsFact{Platform: "admin", UserID: targetUser.ID, Generation: sessionsGeneration}); err != nil {
		t.Fatal(err)
	}
	published, err := cache.PublishIfCurrent(ctx, auth.SessionSnapshot{
		SchemaVersion: 1, UserID: targetUser.ID, SessionID: targetSession.ID, Platform: "admin",
		SessionVersion: targetSession.Version, PolicyVersion: policy.PolicyVersion,
		UserGeneration: userGeneration, SessionsGeneration: sessionsGeneration,
		DeviceID: targetSession.DeviceID, ClientIP: targetSession.ClientIP,
		RefreshExpiresAt: targetSession.RefreshExpiresAt.UTC(), Revoked: false,
	}, time.Hour)
	if err != nil || !published {
		t.Fatalf("publish target session snapshot = %v, %v", published, err)
	}

	jwt := auth.NewJWT([]byte(strings.Repeat("j", 32)))
	oldToken, _, err := jwt.Issue(auth.TokenIdentity{UserID: targetUser.ID, SessionID: targetSession.ID, Platform: "admin", Version: targetSession.Version}, policy.AccessTTL)
	if err != nil {
		t.Fatal(err)
	}
	service := auth.NewService(
		user.NewRepository(tx), role.NewRepository(tx), repository, integrationPolicyStore{policy: policy},
		states, authstate.NewInvalidator(states), cache, redisClient, jwt, []byte(strings.Repeat("h", 32)),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	service.SetSessionAdminRepository(repository)
	result, err := service.RevokeSessions(ctx, auth.Identity{UserID: actorUser.ID, SessionID: actorSession.ID, Platform: "admin", Version: actorSession.Version}, []int64{targetSession.ID})
	if err != nil || len(result.Revoked) != 1 {
		t.Fatalf("RevokeSessions() = %+v, %v", result, err)
	}
	var stored auth.Session
	if err := tx.WithContext(ctx).Take(&stored, targetSession.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.RevokedAt == nil {
		t.Fatal("PostgreSQL session was not revoked")
	}
	if _, found, err := cache.Read(ctx, "admin", targetSession.ID); err != nil || found {
		t.Fatalf("revoked session snapshot = found %v, error %v", found, err)
	}
	_, err = service.Authenticate(ctx, oldToken, authclient.Client{
		Platform: "admin", DeviceID: targetSession.DeviceID, ClientIP: targetSession.ClientIP, UserAgent: targetSession.UserAgent,
	})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeUnauthorized {
		t.Fatalf("old Access Token error = %v", err)
	}
}

func openSessionAdminRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
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
