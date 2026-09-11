package email

import (
	"context"
	"os"
	"testing"

	"admin/server/internal/config"
	authstate "admin/server/internal/module/auth/state"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func TestAuthorityCoordinatorCommitsFreshGenerationAfterSuccessfulMutationWhenRequestIsCanceled(t *testing.T) {
	client := openEmailAuthorityRedis(t)
	store := authstate.NewStore(client)
	coordinator := NewAuthorityCoordinator(store, authstate.NewInvalidator(store))
	const userID int64 = 73101
	ctx := context.Background()
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), []string{authstate.UserStateKey(userID)}) })
	_ = client.DeleteMany(ctx, []string{authstate.UserStateKey(userID)})
	prior := authstate.UserFact{UserID: userID, Generation: "email-prior", IsEnabled: true}
	if _, _, err := store.InstallUserReadyIfMissing(ctx, prior); err != nil {
		t.Fatal(err)
	}
	requestCtx, cancel := context.WithCancel(ctx)
	err := coordinator.Mutate(requestCtx, Current{UserID: userID, IsEnabled: true}, func(context.Context) error {
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("successful committed mutation returned %v", err)
	}
	state, found, err := store.ReadUser(ctx, userID)
	if err != nil || !found || state.State != authstate.StateReady || state.Generation == prior.Generation {
		t.Fatalf("authority state=%+v found=%v err=%v", state, found, err)
	}
}

func openEmailAuthorityRedis(t *testing.T) *projectredis.Client {
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
