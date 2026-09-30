// Offline maintenance: inspect or advance only the Admin menu and global
// system-setting Redis state to PostgreSQL facts. Never deletes Redis data.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"admin/server/internal/config"
	permissionstate "admin/server/internal/module/permission/state"
	projectredis "admin/server/internal/redis"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

var settingScope = cachegeneration.Scope{Namespace: "system.setting", ScopeKey: "global"}

func main() {
	if err := runMain(); err != nil {
		// Do not print underlying connection errors/DSNs from a maintenance command.
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseMode(args []string) (string, error) {
	flags := flag.NewFlagSet("mail-recipient-rule-csv-migration", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", "inspect", "inspect|sync")
	stopped := flags.Bool("old-api-stopped", false, "API and Worker are stopped")
	if err := flags.Parse(args); err != nil {
		return "", fmt.Errorf("invalid migration arguments")
	}
	if flags.NArg() != 0 || (*mode != "inspect" && *mode != "sync") {
		return "", fmt.Errorf("only -mode inspect|sync and -old-api-stopped are accepted")
	}
	if *mode == "sync" && !*stopped {
		return "", fmt.Errorf("sync requires API/Worker stopped and -old-api-stopped")
	}
	return *mode, nil
}

func runMain() error {
	mode, err := parseMode(os.Args[1:])
	if err != nil {
		return err
	}
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot load server environment")
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("invalid PostgreSQL/Redis configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := pgx.Connect(ctx, settings.PostgresDSN)
	if err != nil {
		return fmt.Errorf("PostgreSQL connection failed")
	}
	defer func() { _ = db.Close(ctx) }()
	var platformID, menuVersion, settingGeneration int64
	if err := db.QueryRow(ctx, `SELECT id,menu_version FROM permission_auth_platform
 WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL`).Scan(&platformID, &menuVersion); err != nil {
		return fmt.Errorf("active Admin menu version unavailable")
	}
	if err := db.QueryRow(ctx, `SELECT generation FROM system_config_cache_generation
 WHERE namespace='system.setting' AND scope_key='global'`).Scan(&settingGeneration); err != nil {
		return fmt.Errorf("system.setting/global generation unavailable")
	}
	client, err := projectredis.Open(ctx, settings.RedisURL)
	if err != nil {
		return fmt.Errorf("Redis connection failed")
	}
	defer func() { _ = client.Close() }()
	menus := permissionstate.NewMenuStore(client)
	states := cachegeneration.NewStore(client)
	if mode == "sync" {
		if err := syncMenuState(ctx, menus, platformID, menuVersion); err != nil {
			return err
		}
		previous, found, err := states.Read(ctx, settingScope)
		if err != nil || (found && previous.State != cachegeneration.StateReady) {
			return fmt.Errorf("setting state corrupt or currently mutating; rerun after the writer finishes")
		}
		if found && previous.Generation > settingGeneration {
			return fmt.Errorf("setting Redis generation is ahead of PostgreSQL; refusing downgrade")
		}
		result, err := states.Reconcile(ctx, settingScope, settingGeneration)
		if err != nil || result == cachegeneration.PublishSkippedInvalidating {
			return fmt.Errorf("setting state publication failed; rerun this maintenance command")
		}
	}
	menuState, menuFound, menuErr := menus.Read(ctx, platformID)
	settingState, settingFound, settingErr := states.Read(ctx, settingScope)
	if menuErr != nil || settingErr != nil {
		return fmt.Errorf("Redis state inspection failed")
	}
	if mode == "sync" && (!menuFound || !settingFound || menuState.State != permissionstate.StateReady || menuState.Version != menuVersion || settingState.State != cachegeneration.StateReady || settingState.Generation != settingGeneration) {
		return fmt.Errorf("PostgreSQL/Redis state verification failed")
	}
	// Omit tokens and cache payloads from the audit output.
	return json.NewEncoder(os.Stdout).Encode(struct {
		MenuVersion        int64 `json:"menuVersion"`
		MenuStateFound     bool  `json:"menuStateFound"`
		MenuStateVersion   int64 `json:"menuStateVersion"`
		SettingGeneration  int64 `json:"settingGeneration"`
		SettingStateFound  bool  `json:"settingStateFound"`
		SettingStateNumber int64 `json:"settingStateGeneration"`
	}{menuVersion, menuFound, menuState.Version, settingGeneration, settingFound, settingState.Generation})
}

func syncMenuState(ctx context.Context, menus *permissionstate.MenuStore, platformID, authoritative int64) error {
	if platformID < 1 || authoritative < 1 {
		return fmt.Errorf("invalid Admin menu version")
	}
	previous, found, err := menus.Read(ctx, platformID)
	if err != nil {
		return fmt.Errorf("Admin menu state inspection failed")
	}
	if !found {
		version, err := menus.Current(ctx, platformID, func(context.Context, int64) (int64, error) { return authoritative, nil })
		if err != nil || version != authoritative {
			return fmt.Errorf("Admin menu state initialization failed")
		}
		return nil
	}
	if previous.State != permissionstate.StateReady {
		return fmt.Errorf("Admin menu mutation is active; rerun after the writer finishes")
	}
	if previous.Version > authoritative {
		return fmt.Errorf("Admin menu Redis version is ahead of PostgreSQL; refusing downgrade")
	}
	if previous.Version == authoritative {
		return nil
	}
	lease, err := menus.Acquire(ctx, platformID, previous.Version)
	if err != nil {
		return fmt.Errorf("Admin menu state changed during publication; rerun")
	}
	if err := lease.Commit(ctx, authoritative); err != nil {
		return fmt.Errorf("Admin menu state publication failed; rerun after lease expiry")
	}
	return nil
}
