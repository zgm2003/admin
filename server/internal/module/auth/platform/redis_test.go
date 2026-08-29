package authplatform_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/auth/platform"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"github.com/joho/godotenv"
)

func TestCurrentPolicyUsesRedisHitAndPostgreSQLFallbacks(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	redisClient := openPlatformRedis(t)
	key := authplatform.PolicyKey("admin")
	t.Cleanup(func() { _ = redisClient.Delete(context.Background(), key) })
	if err := redisClient.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := authplatform.NewService(authplatform.NewRepository(connection.GORM), authplatform.NewPolicyStore(redisClient), redisClient, nil, nil, nil, logger, authplatform.Deployment{})
	policy, err := service.CurrentPolicy(ctx, "admin")
	if err != nil || policy.Code != "admin" || policy.PolicyVersion != 1 {
		t.Fatalf("initial CurrentPolicy() = %+v,%v", policy, err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	policy, err = service.CurrentPolicy(ctx, "admin")
	if err != nil || policy.Code != "admin" {
		t.Fatalf("cached CurrentPolicy() = %+v,%v", policy, err)
	}
}

func TestCurrentPolicyFallsBackForCorruptionAndRedisReadError(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	redisClient := openPlatformRedis(t)
	key := authplatform.PolicyKey("admin")
	t.Cleanup(func() { _ = redisClient.Delete(context.Background(), key) })
	if err := redisClient.SetString(ctx, key, `{"schemaVersion":1,"state":"ready","unknown":true}`, time.Minute); err != nil {
		t.Fatal(err)
	}
	service := authplatform.NewService(authplatform.NewRepository(connection.GORM), authplatform.NewPolicyStore(redisClient), redisClient, nil, nil, nil, logger, authplatform.Deployment{})
	if policy, err := service.CurrentPolicy(ctx, "admin"); err != nil || policy.Code != "admin" {
		t.Fatalf("corrupt fallback = %+v,%v", policy, err)
	}

	closedRedis := openPlatformRedis(t)
	closedStore := authplatform.NewPolicyStore(closedRedis)
	if err := closedRedis.Close(); err != nil {
		t.Fatal(err)
	}
	service = authplatform.NewService(authplatform.NewRepository(connection.GORM), closedStore, closedRedis, nil, nil, nil, logger, authplatform.Deployment{})
	if policy, err := service.CurrentPolicy(ctx, "admin"); err != nil || policy.Code != "admin" {
		t.Fatalf("Redis error fallback = %+v,%v", policy, err)
	}
}

func TestCurrentPolicyRejectsInvalidatingState(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	redisClient := openPlatformRedis(t)
	key := authplatform.PolicyKey("admin")
	t.Cleanup(func() { _ = redisClient.Delete(context.Background(), key) })
	if err := redisClient.SetString(ctx, key, `{"schemaVersion":1,"state":"invalidating","mutationToken":"token-a","policy":null}`, time.Minute); err != nil {
		t.Fatal(err)
	}
	service := authplatform.NewService(authplatform.NewRepository(connection.GORM), authplatform.NewPolicyStore(redisClient), redisClient, nil, nil, nil, slog.Default(), authplatform.Deployment{})
	_, err := service.CurrentPolicy(ctx, "admin")
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != authplatform.CodeSessionUpdating {
		t.Fatalf("invalidating error = %v", err)
	}
}

func TestClearBuiltinPoliciesDeletesAdminAndCanvasKeysOnly(t *testing.T) {
	ctx := context.Background()
	redisClient := openPlatformRedis(t)
	adminKey := authplatform.PolicyKey(authplatform.BuiltinAdminCode)
	canvasKey := authplatform.PolicyKey("canvas")
	appKey := authplatform.PolicyKey("app")
	t.Cleanup(func() {
		_ = redisClient.Delete(context.Background(), adminKey)
		_ = redisClient.Delete(context.Background(), canvasKey)
		_ = redisClient.Delete(context.Background(), appKey)
	})
	if err := redisClient.SetString(ctx, adminKey, `{"state":"old-admin"}`, 0); err != nil {
		t.Fatal(err)
	}
	if err := redisClient.SetString(ctx, canvasKey, `{"state":"old-canvas"}`, 0); err != nil {
		t.Fatal(err)
	}
	if err := redisClient.SetString(ctx, appKey, `{"state":"app"}`, 0); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.ClearBuiltinPolicies(ctx, redisClient); err != nil {
		t.Fatal(err)
	}
	if _, found, err := redisClient.GetString(ctx, adminKey); err != nil || found {
		t.Fatalf("admin key found=%v err=%v", found, err)
	}
	if _, found, err := redisClient.GetString(ctx, canvasKey); err != nil || found {
		t.Fatalf("Canvas key found=%v err=%v", found, err)
	}
	if _, found, err := redisClient.GetString(ctx, appKey); err != nil || !found {
		t.Fatalf("app key found=%v err=%v", found, err)
	}
}

func TestClearBuiltinPoliciesRequiresRedis(t *testing.T) {
	if err := authplatform.ClearBuiltinPolicies(context.Background(), nil); err == nil {
		t.Fatal("ClearBuiltinPolicies() succeeded without Redis")
	}
}

func openPlatformRedis(t *testing.T) *projectredis.Client {
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
	redisURL, err := url.Parse(settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	redisURL.Path = "/11"
	redisURL.RawPath = ""
	client, err := projectredis.Open(context.Background(), redisURL.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
