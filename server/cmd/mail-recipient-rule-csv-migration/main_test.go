package main

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	permissionstate "admin/server/internal/module/permission/state"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func TestCSVStateSyncRequiresExplicitStoppedFlag(t *testing.T) {
	for _, args := range [][]string{{"-mode", "sync"}, {"-mode", "cleanup"}, {"extra"}} {
		if _, err := parseMode(args); err == nil {
			t.Fatalf("unexpectedly accepted %v", args)
		}
	}
	for _, args := range [][]string{nil, {"-mode", "sync", "-old-api-stopped"}} {
		if _, err := parseMode(args); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCSVMenuStateSyncIsMonotonicAndNeverOverridesMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("Redis integration")
	}
	_ = godotenv.Load("../../.env")
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/13"
	client, err := projectredis.Open(context.Background(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	platformID := time.Now().UnixNano()
	key := permissionstate.MenuStateKey(platformID)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
	menus := permissionstate.NewMenuStore(client)
	if err := syncMenuState(ctx, menus, platformID, 14); err != nil {
		t.Fatal(err)
	}
	if err := syncMenuState(ctx, menus, platformID, 15); err != nil {
		t.Fatal(err)
	}
	before, _, err := client.GetString(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncMenuState(ctx, menus, platformID, 15); err != nil {
		t.Fatal(err)
	}
	if err := syncMenuState(ctx, menus, platformID, 14); err == nil {
		t.Fatal("state downgrade accepted")
	}
	if after, _, err := client.GetString(ctx, key); err != nil || after != before {
		t.Fatalf("unexpected state change: %q err=%v", after, err)
	}
	lease, err := menus.Acquire(ctx, platformID, 15)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncMenuState(ctx, menus, platformID, 16); err == nil {
		t.Fatal("active lease overridden")
	}
	if err := lease.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}
