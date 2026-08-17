package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/taskdemo"
	"admin/server/internal/queue"
	projectredis "admin/server/internal/redis"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("Worker stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("load Worker config: %w", err)
	}

	processContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	postgres, err := database.Open(processContext, settings.PostgresDSN)
	if err != nil {
		return err
	}
	defer postgres.Close()
	if err := checkWorkerRedis(processContext, settings.RedisURL); err != nil {
		return err
	}

	repository := taskdemo.NewRepository(postgres.GORM)
	service := taskdemo.NewService(repository, nil, logger)
	mux := asynq.NewServeMux()
	taskdemo.Register(mux, service)
	server, err := queue.NewServer(settings.RedisURL)
	if err != nil {
		return err
	}
	if err := server.Start(mux); err != nil {
		return fmt.Errorf("start Asynq Worker: %w", err)
	}
	<-processContext.Done()
	server.Shutdown()
	return nil
}

func checkRedis(ctx context.Context, redisURL string) error {
	client, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		return err
	}
	return client.Close()
}

func checkWorkerRedis(ctx context.Context, redisURL string) error {
	if err := checkRedis(ctx, redisURL); err != nil {
		return fmt.Errorf("check Worker Redis: %w", err)
	}
	return nil
}
