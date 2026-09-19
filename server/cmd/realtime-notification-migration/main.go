package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

const (
	modeForward  = "forward"
	modeRollback = "rollback"
)

var fixedCleanupPatterns = []string{
	"config-cache:state:v1:system.setting:global",
	"config-cache:snapshot:v1:system.setting:global:*",
	"config-cache:fill:v1:system.setting:global:*",
	"authz:permission-state:v3:*",
	"authz:menu-state:v1:*",
	"authz:permission:v8:*",
	"realtime:ticket:v1:*",
}

func main() {
	if err := runMain(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runMain() error {
	mode, err := parseMode(os.Args[1:])
	if err != nil {
		return err
	}
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, settings.RedisURL, mode)
}

func parseMode(args []string) (string, error) {
	flags := flag.NewFlagSet("realtime-notification-migration", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", modeForward, "cleanup mode: forward|rollback")
	if err := flags.Parse(args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if flags.NArg() != 0 {
		return "", fmt.Errorf("realtime notification migration accepts only -mode")
	}
	if *mode != modeForward && *mode != modeRollback {
		return "", fmt.Errorf("mode must be %s or %s", modeForward, modeRollback)
	}
	return *mode, nil
}

func cleanupPatterns(mode string) ([]string, error) {
	if mode != modeForward && mode != modeRollback {
		return nil, fmt.Errorf("mode must be %s or %s", modeForward, modeRollback)
	}
	return append([]string(nil), fixedCleanupPatterns...), nil
}

func run(ctx context.Context, redisURL, mode string) error {
	patterns, err := cleanupPatterns(mode)
	if err != nil {
		return err
	}
	client, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	for _, pattern := range patterns {
		if err := client.ScanDelete(ctx, pattern); err != nil {
			return fmt.Errorf("delete pattern %s: %w", pattern, err)
		}
		remaining, err := countPatternKeys(ctx, client, pattern)
		if err != nil {
			return err
		}
		if remaining != 0 {
			return fmt.Errorf("pattern %s still has %d keys", pattern, remaining)
		}
		_, _ = fmt.Fprintf(os.Stdout, "cleanup mode=%s pattern=%s remaining=0\n", mode, pattern)
	}
	return nil
}

func countPatternKeys(ctx context.Context, client *projectredis.Client, pattern string) (int, error) {
	var cursor uint64
	count := 0
	for {
		keys, next, err := client.UniversalClient().Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return 0, fmt.Errorf("scan pattern %s: %w", pattern, err)
		}
		count += len(keys)
		if next == 0 {
			return count, nil
		}
		cursor = next
	}
}
