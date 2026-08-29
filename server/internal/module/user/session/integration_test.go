package session_test

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
	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/module/auth/state"
	"admin/server/internal/module/rbac/role"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/module/user/session"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type integrationPolicyStore struct {
	policy authplatform.Policy
}

type invalidationObservingSessionRepository struct {
	repository           *session.Repository
	states               *authstate.Store
	redis                *projectredis.Client
	platform             string
	userID               int64
	sawInvalidating      bool
	dropStateAfterRevoke bool
}

func (r *invalidationObservingSessionRepository) ListAdmin(ctx context.Context, query session.AdminSessionQuery, now time.Time) ([]session.AdminSession, int64, error) {
	return r.repository.ListAdmin(ctx, query, now)
}

func (r *invalidationObservingSessionRepository) StatsAdmin(ctx context.Context, now time.Time) (session.AdminSessionStats, error) {
	return r.repository.StatsAdmin(ctx, now)
}

func (r *invalidationObservingSessionRepository) FindAdminRevokeTargets(ctx context.Context, ids []int64) ([]session.Record, error) {
	return r.repository.FindAdminRevokeTargets(ctx, ids)
}

func (r *invalidationObservingSessionRepository) RevokeAdmin(ctx context.Context, ids []int64, currentSessionID int64, now time.Time) (session.AdminRevokeResult, error) {
	state, found, err := r.states.ReadSessions(ctx, r.platform, r.userID)
	if err != nil {
		return session.AdminRevokeResult{}, err
	}
	if !found || state.State != authstate.StateInvalidating {
		return session.AdminRevokeResult{}, errors.New("sessions state was not invalidating before PostgreSQL revoke")
	}
	r.sawInvalidating = true
	result, err := r.repository.RevokeAdmin(ctx, ids, currentSessionID, now)
	if err == nil && r.dropStateAfterRevoke {
		err = r.redis.Delete(ctx, authstate.SessionsStateKey(r.platform, r.userID))
	}
	return result, err
}

func (s integrationPolicyStore) CurrentPolicy(_ context.Context, platform string) (authplatform.Policy, error) {
	if platform != s.policy.Code {
		return authplatform.Policy{}, errors.New("unexpected authentication platform")
	}
	return s.policy, nil
}

func TestAdminSessionRevokePersistsDeletesSnapshotAndRejectsOldToken(t *testing.T) {
	fixture := newAdminSessionRevokeFixture(t)
	result, err := fixture.service.RevokeSessions(fixture.ctx, fixture.actor, []int64{fixture.targetSession.ID})
	if err != nil || len(result.Revoked) != 1 {
		t.Fatalf("RevokeSessions() = %+v, %v", result, err)
	}
	if !fixture.observingRepository.sawInvalidating {
		t.Fatal("PostgreSQL revoke did not run under an invalidating sessions lease")
	}
	assertPostgreSQLSessionRevoked(t, fixture)
	if _, found, err := fixture.cache.Read(fixture.ctx, "admin", fixture.targetSession.ID); err != nil || found {
		t.Fatalf("revoked session snapshot = found %v, error %v", found, err)
	}
	assertOldAccessTokenRejected(t, fixture)
}

func TestAdminSessionRevokeCommitFailureLeavesOldSnapshotUnreachable(t *testing.T) {
	fixture := newAdminSessionRevokeFixture(t)
	fixture.observingRepository.dropStateAfterRevoke = true

	_, err := fixture.service.RevokeSessions(fixture.ctx, fixture.actor, []int64{fixture.targetSession.ID})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeDependencyUnavailable {
		t.Fatalf("RevokeSessions() error = %v", err)
	}
	assertPostgreSQLSessionRevoked(t, fixture)
	if _, found, err := fixture.states.ReadSessions(fixture.ctx, "admin", fixture.targetSession.UserID); err != nil || found {
		t.Fatalf("sessions state after failed publish = found %v, error %v", found, err)
	}
	if _, found, err := fixture.cache.Read(fixture.ctx, "admin", fixture.targetSession.ID); err != nil || !found {
		t.Fatalf("old snapshot = found %v, error %v; failure injection should leave it in place", found, err)
	}
	assertOldAccessTokenRejected(t, fixture)
}

type adminSessionRevokeFixture struct {
	ctx                 context.Context
	tx                  *gorm.DB
	states              *authstate.Store
	cache               *auth.SessionCache
	service             *session.Service
	authService         *auth.Service
	observingRepository *invalidationObservingSessionRepository
	actor               session.Actor
	targetSession       session.Record
	oldToken            string
	client              authclient.Client
}

func newAdminSessionRevokeFixture(t *testing.T) adminSessionRevokeFixture {
	tx, ctx := openAuthTransaction(t)
	redisClient := openSessionAdminRedis(t)
	actorUser := createAuthUserWithoutRole(t, tx, ctx, "session-admin-actor")
	targetUser := createAuthUserWithoutRole(t, tx, ctx, "session-admin-target")
	policy := updateTestPolicy(t, tx, ctx, "admin", 0)
	repository := session.NewRepository(tx)
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
	authService := auth.NewService(
		user.NewRepository(tx), role.NewRepository(tx), repository, integrationPolicyStore{policy: policy},
		states, authstate.NewInvalidator(states), cache, redisClient, jwt, []byte(strings.Repeat("h", 32)),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	observingRepository := &invalidationObservingSessionRepository{
		repository: repository,
		states:     states,
		redis:      redisClient,
		platform:   targetSession.Platform,
		userID:     targetUser.ID,
	}
	service := session.NewService(observingRepository, states, authstate.NewInvalidator(states), cache)
	return adminSessionRevokeFixture{
		ctx: ctx, tx: tx, states: states, cache: cache, service: service, authService: authService, observingRepository: observingRepository,
		actor:         session.Actor{UserID: actorUser.ID, SessionID: actorSession.ID},
		targetSession: targetSession, oldToken: oldToken,
		client: authclient.Client{Platform: "admin", DeviceID: targetSession.DeviceID, ClientIP: targetSession.ClientIP, UserAgent: targetSession.UserAgent},
	}
}

func assertPostgreSQLSessionRevoked(t *testing.T, fixture adminSessionRevokeFixture) {
	t.Helper()
	var stored session.Session
	if err := fixture.tx.WithContext(fixture.ctx).Take(&stored, fixture.targetSession.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.RevokedAt == nil {
		t.Fatal("PostgreSQL session was not revoked")
	}
}

func assertOldAccessTokenRejected(t *testing.T, fixture adminSessionRevokeFixture) {
	t.Helper()
	_, err := fixture.authService.Authenticate(fixture.ctx, fixture.oldToken, fixture.client)
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
