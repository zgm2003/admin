// 一次性维护命令：配置缓存代际完成迁移的固定 Redis 清理。
// 只接受 -mode forward|rollback，不接受调用方 pattern，不打印 Redis URL 或缓存内容。
// 逐 pattern 使用 SCAN 小批量删除并复验 remaining=0；禁止 KEYS / FLUSHDB / FLUSHALL。
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

var forwardCleanupPatterns = []string{
	"system:dictionary:generation:v1",
	"system:dictionary:mutation:v1",
	"system:dictionary:options:v1:*",
	"mail:runtime:v2:*",
	"mail:runtime:generation:v2",
	"mail:runtime:mutation:v2",
	"mail:runtime:load-lock:v2",
	"mail:verify-code-readiness:v3:*",
	"mail:verify-code-readiness:load-lock:v3:*",
	"mail:rate-limit:policies:v3:*",
	"mail:rate-limit:policies:load-lock:v3:*",
	"sms:runtime:generation:v1*",
	"sms:runtime:mutation:v1*",
	"sms:runtime:v1:*",
	"sms:readiness:v1:*",
	"sms:rate-limit:policies:v1:*",
	"sms:rate-limit:policies:load-lock:v1:*",
	"sms:rate-limit:policies:mutation:v1:*",
}

var rollbackCleanupPatterns = []string{
	"config-cache:state:v1:system.dictionary:global",
	"config-cache:snapshot:v1:system.dictionary:global:*",
	"config-cache:state:v1:message.mail:global",
	"config-cache:snapshot:v1:message.mail:global:*",
	"config-cache:state:v1:message.sms:global",
	"config-cache:snapshot:v1:message.sms:global:*",
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
	flags := flag.NewFlagSet("config-cache-generation-completion-migration", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", modeForward, "cleanup mode: forward|rollback")
	if err := flags.Parse(args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if flags.NArg() != 0 {
		return "", fmt.Errorf("config cache generation completion migration accepts only -mode")
	}
	if *mode != modeForward && *mode != modeRollback {
		return "", fmt.Errorf("mode must be %s or %s", modeForward, modeRollback)
	}
	return *mode, nil
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

func cleanupPatterns(mode string) ([]string, error) {
	switch mode {
	case modeForward:
		return forwardCleanupPatterns, nil
	case modeRollback:
		return rollbackCleanupPatterns, nil
	default:
		return nil, fmt.Errorf("mode must be %s or %s", modeForward, modeRollback)
	}
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
