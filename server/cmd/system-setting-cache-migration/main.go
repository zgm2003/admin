// 一次性维护命令：清理统一配置缓存代际协议上线前遗留的 system:setting:v1:* 缓存键。
// 固定 pattern，不接受调用方传入 pattern；不打印 Redis URL 或任何连接字符串。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

const legacySystemSettingKeyPattern = "system:setting:v1:*"

func main() {
	if err := runMain(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runMain() error {
	if len(os.Args) > 1 {
		return fmt.Errorf("system setting cache migration does not accept arguments")
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
	return run(ctx, settings.RedisURL)
}

// run 是固定入口：连接 Redis，按固定 pattern 清理旧键并确认无残留。
func run(ctx context.Context, redisURL string) error {
	client, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if err := client.ScanDelete(ctx, legacySystemSettingKeyPattern); err != nil {
		return fmt.Errorf("delete legacy system setting cache keys: %w", err)
	}
	remaining, err := countLegacyKeys(ctx, client)
	if err != nil {
		return err
	}
	if remaining != 0 {
		return fmt.Errorf("legacy system setting cache keys remain: %d", remaining)
	}
	_, _ = fmt.Fprintf(os.Stdout, "legacy system setting cache cleared: pattern=%s remaining=0\n", legacySystemSettingKeyPattern)
	return nil
}

func countLegacyKeys(ctx context.Context, client *projectredis.Client) (int, error) {
	var cursor uint64
	count := 0
	for {
		keys, next, err := client.UniversalClient().Scan(ctx, cursor, legacySystemSettingKeyPattern, 100).Result()
		if err != nil {
			return 0, fmt.Errorf("scan legacy system setting cache keys: %w", err)
		}
		count += len(keys)
		if next == 0 {
			return count, nil
		}
		cursor = next
	}
}
